<#
.SYNOPSIS
  dev.ps1 -- developer / CI task runner for the Go implementation of
  SEMP Workflow Automation. Mirrors dev.sh exactly (same tasks, same flags).

.DESCRIPTION
  Tasks:
    build   go build -> dist\semp-workflow-<os>-<arch>[.exe]   (fatal)
    vet     go vet ./...                                       (fatal)
    lint    golangci-lint run                                  (fatal)
    test    go test ./... (unit)                               (fatal)
    integration  go test -tags integration ./...               (fatal; needs live broker)
    cov     coverage profile -> func total + HTML              (fatal)
    scan    govulncheck ./... (image scan N/A -- no Dockerfile)(fatal)
    image   container image build                    (warn-skip: no Dockerfile here)
    graphify  python -m graphify update .          (local only; report-only)
    size    build + go-size-analyzer size breakdown            (report-only)
  Aggregates:
    all     build vet test                       (fast inner loop; CI runs 'all scan')
    full    all + gencheck + cov + image + scan + graphify   (pre-tag sweep)
  (integration is opt-in -- never part of all/full, so broker-free runs pass.)

  Every task closes with a footer line -- echoed to the console and appended to
  that task's log:  <ISO-8601 w/ offset> | <task> | <duration>s | OK|FAILED (exit N)

  Runs from any cwd. Logs land in <scriptdir>\logs\<task>.log (plain text).
#>
[CmdletBinding()]
param([Parameter(ValueFromRemainingArguments = $true)] [string[]] $Tasks)

# NB: do NOT set this to "Stop". In Windows PowerShell 5.1, `2>&1` on a native
# command (go/govulncheck) wraps each stderr line in a NativeCommandError; with
# "Stop" the first such record becomes a terminating error and aborts the script
# before $LASTEXITCODE is read -- e.g. `go vet` writing diagnostics to stderr would
# kill the run with no log written. "Continue" mirrors dev.sh (set -uo pipefail,
# no -e): every task gates explicitly on its captured exit code instead.
$ErrorActionPreference = "Continue"

$ScriptDir    = Split-Path -Parent $MyInvocation.MyCommand.Path
$GoRoot       = (Resolve-Path (Join-Path $ScriptDir "..")).Path
$ProjectRoot  = $GoRoot
$LogDir       = Join-Path $ScriptDir "logs"
$DistDir      = Join-Path $GoRoot "dist"
$BinName      = "semp-workflow"   # base name; build appends -<os>-<arch>[.exe]
$CoverageOut  = Join-Path $GoRoot "coverage.out"
$CoverageHtml = Join-Path $GoRoot "coverage.html"

# Toolchain parity with CI: when go.mod carries a `toolchain` directive it is the
# single pin, and GOTOOLCHAIN makes any go binary honour it exactly. Set only
# when unset, so an exported value wins.
$goMod = Join-Path $GoRoot "go.mod"
if (-not $env:GOTOOLCHAIN -and (Test-Path $goMod)) {
    $m = Select-String -Path $goMod -Pattern '^toolchain (\S+)' | Select-Object -First 1
    if ($m) { $env:GOTOOLCHAIN = $m.Matches[0].Groups[1].Value }
}

$Version = "0.0.0-dev"
if ($env:SEMP_WF_VERSION) {
    $Version = $env:SEMP_WF_VERSION
} else {
    try {
        $d = (& git describe --tags --exact-match 2>$null)
        if ($LASTEXITCODE -eq 0 -and $d) { $Version = "$d".Trim() }
    } catch { }
}

# Tasks that record FAILED in their footer but never abort the run.
$ReportOnly = @("graphify", "size")

# Tools that honor NO_COLOR emit clean text into the logs; static build by default.
$env:NO_COLOR = "1"
if (-not $env:CGO_ENABLED) { $env:CGO_ENABLED = "0" }

if (-not (Test-Path $LogDir)) { New-Item -ItemType Directory -Path $LogDir | Out-Null }

# ---- console helpers (colored; not captured to logs) ----------------------
function Step($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Ok($m)   { Write-Host " OK  $m" -ForegroundColor Green }
function Warn($m) { Write-Host " !   $m" -ForegroundColor Yellow }
function Die($m)  { Write-Host " XX  $m" -ForegroundColor Red; exit 1 }

# Invoke-Logged: run a native command in $WorkDir, capture combined output once
# (PS 5.1-safe: flatten ErrorRecords, no Tee-Object), clean it to plain ASCII,
# write a UTF-8 log, echo to console, and return the native exit code
# ($LASTEXITCODE). Cleaning strips CSI escapes (spinner/color) and folds the
# Unicode box-drawing characters and micro sign a tool may emit (e.g.
# go-size-analyzer's table + "us" timings) to ASCII (-,|,+,u), so the log stays
# readable plain text. The folds are inert for ASCII-only tools.
function Invoke-Logged {
    param([string]$Task, [string]$WorkDir, [string]$Exe, [string[]]$CmdArgs)
    $log = Join-Path $LogDir "$Task.log"
    Push-Location $WorkDir
    try {
        $out  = (& $Exe @CmdArgs 2>&1 | ForEach-Object { "$_" } | Out-String -Width 4096)
        $code = $LASTEXITCODE
    } finally {
        Pop-Location
    }
    $out = $out -replace "\x1b\[[0-9;?]*[a-zA-Z]", ""
    # Fold Unicode box-drawing (U+2500-257F) and the micro sign to ASCII so the
    # log stays plain text. The patterns are built from code points via [char]
    # so THIS source file stays ASCII-only (PS 5.1 mis-decodes a BOM-less
    # non-ASCII byte as a parse error). Order matters: replace the two common
    # rules first, then catch any remaining box char with +.
    $out = $out -replace ([char]0x2500), "-"                                  # horizontal rule
    $out = $out -replace ([char]0x2502), "|"                                  # vertical rule
    $out = $out -replace ("[" + [char]0x2500 + "-" + [char]0x257F + "]"), "+" # corners/junctions/other box
    $out = $out -replace ([char]0x00B5), "u"                                  # micro sign (us timings)
    $header = "=== $Task ===`r`ndir: $WorkDir`r`ncmd: $Exe $($CmdArgs -join ' ')`r`n`r`n"
    Set-Content -Path $log -Value ($header + $out) -Encoding utf8
    if ($out) { Write-Host $out }
    return $code
}

function Have($name) { return [bool](Get-Command $name -ErrorAction SilentlyContinue) }

# Invoke-Task runs Task-<Name>, then closes it with the S7 footer -- echoed to the
# console and appended to that task's log -- whatever the outcome. Fatal tasks abort
# the whole run on a non-zero code; report-only tasks ($ReportOnly) record FAILED in
# the footer but let the run continue. Task functions therefore `return` a code
# instead of calling Die, so the footer always lands. Duration is whole seconds and
# the timestamp carries the local UTC offset with no colon (yyyy-MM-ddTHH:mm:ss+HHMM),
# matching dev.sh's `date +%z` so the two scripts write the same footer format.
function Invoke-Task {
    param([string]$Task)
    $fn = "Task-" + $Task.Substring(0, 1).ToUpperInvariant() + $Task.Substring(1)
    $start = Get-Date
    $code = & $fn
    if ($null -eq $code) { $code = 0 }
    $dur = [int][math]::Floor(((Get-Date) - $start).TotalSeconds)
    $now = Get-Date
    $off = ($now.ToString("zzz")) -replace ":", ""
    $ts  = $now.ToString("yyyy-MM-ddTHH:mm:ss") + $off
    $status = if ($code -eq 0) { "OK" } else { "FAILED (exit $code)" }
    $footer = "$ts | $Task | ${dur}s | $status"
    Write-Host $footer
    Add-Content -Path (Join-Path $LogDir "$Task.log") -Value $footer -Encoding utf8
    if ($code -ne 0 -and ($ReportOnly -notcontains $Task)) { exit $code }
}

# Get-BinPath echoes the dist\ output path for a target: the base name plus
# -<os>-<arch> plus the platform extension (.exe on Windows). The target is always
# in the name because the release job merges every cross-compile leg into one
# directory with merge-multiple -- a bare name would let one arch overwrite another
# and ship a single architecture under several names.
function Get-BinPath($os, $arch) {
    $ext = if ($os -eq "windows") { ".exe" } else { "" }
    return (Join-Path $DistDir "$BinName-$os-$arch$ext")
}

# Import-LocalEnv loads KEY=VALUE lines from a gitignored scripts\local.env -- the
# ONLY source of the broker connection for the `integration` task (no environment
# fallback). Each key is set as a process env var so the `go test` child sees it;
# blank lines and `#` comments are skipped, and surrounding single or double
# quotes are stripped (matching what bash's `source` of the same file yields).
# The file is required: absent it we warn and return $false (task then fails with
# its footer) rather than run a skip-only suite that looks like a pass.
function Import-LocalEnv {
    $file = Join-Path $ScriptDir "local.env"
    if (-not (Test-Path $file)) {
        Warn "scripts\local.env not found -- copy scripts\local.env.example to scripts\local.env and fill in the broker connection"
        return $false
    }
    foreach ($line in Get-Content -LiteralPath $file) {
        $t = $line.Trim()
        if ($t -eq "" -or $t.StartsWith("#")) { continue }
        $idx = $t.IndexOf("=")
        if ($idx -lt 1) { continue }
        $key = $t.Substring(0, $idx).Trim()
        $val = $t.Substring($idx + 1).Trim().Trim('"', "'")
        Set-Item -Path "Env:$key" -Value $val
    }
    return $true
}

# ---- tasks ----------------------------------------------------------------
# Task-Build compiles the CLI to dist\<name>-<os>-<arch>[.exe]. TARGET_OS /
# TARGET_ARCH (set by CI's per-leg binaries matrix) select the target; unset means
# the host's own os/arch, so local and CI output are the same shape. The target is
# always in the name -- see Get-BinPath. PowerShell has no inline env prefix, so
# GOOS/GOARCH are set around the build and restored after (same pattern as size's
# GOEXPERIMENT), never leaking into other tasks.
function Task-Build {
    $os   = if ($env:TARGET_OS)   { $env:TARGET_OS }   else { "$(& go env GOOS)".Trim() }
    $arch = if ($env:TARGET_ARCH) { $env:TARGET_ARCH } else { "$(& go env GOARCH)".Trim() }
    $out  = Get-BinPath $os $arch
    Step "build -> $out"
    if (-not (Test-Path $DistDir)) { New-Item -ItemType Directory -Path $DistDir | Out-Null }
    $prevGoos = $env:GOOS; $prevGoarch = $env:GOARCH
    $env:GOOS = $os; $env:GOARCH = $arch
    try {
        $code = Invoke-Logged "build" $GoRoot "go" @(
            "build", "-trimpath", "-ldflags", "-s -w -X main.version=$Version",
            "-o", $out, "./cmd/semp-workflow")
    } finally {
        if ($null -eq $prevGoos)   { Remove-Item Env:GOOS   -ErrorAction SilentlyContinue } else { $env:GOOS = $prevGoos }
        if ($null -eq $prevGoarch) { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue } else { $env:GOARCH = $prevGoarch }
    }
    if ($code -eq 0) { Ok "build -> $out" }
    return $code
}

function Task-Gen {
    Step "gen (module descriptors from specs/)"
    $code = Invoke-Logged "gen" $GoRoot "go" @("generate", "./internal/modules/...")
    if ($code -eq 0) { Ok "gen" }
    return $code
}

function Task-Gencheck {
    Step "gencheck (descriptors match specs/)"
    $genArgs = @(
        "run", "./tools/semp-gen",
        "-software", "specs/software/10.26.0.8755/semp-v2-config.json",
        "-appliance", "specs/appliance/10.26.0.8755/semp-v2-config.json",
        "-out", "internal/modules", "-check"
    )
    $code = Invoke-Logged "gencheck" $GoRoot "go" $genArgs
    if ($code -eq 0) { Ok "gencheck" }
    return $code
}

function Task-Vet {
    Step "vet"
    $code = Invoke-Logged "vet" $GoRoot "go" @("vet", "./...")
    if ($code -eq 0) { Ok "vet" }
    return $code
}

function Task-Lint {
    Step "lint (golangci-lint)"
    if (-not (Have "golangci-lint")) { Warn "golangci-lint not installed; skipping (install: https://golangci-lint.run)"; return 0 }
    # golangci-lint v2 dropped --no-color; NO_COLOR=1 (set above) disables color.
    $code = Invoke-Logged "lint" $GoRoot "golangci-lint" @("run", "./...")
    if ($code -eq 0) { Ok "lint" }
    return $code
}

function Task-Test {
    Step "test (unit)"
    # -count=1 disables Go's test result cache so every run re-executes the whole
    # suite -- a cached PASS can otherwise hide a regression or stale coverage.
    $code = Invoke-Logged "test" $GoRoot "go" @("test", "-count=1", "./...")
    if ($code -eq 0) { Ok "test" }
    return $code
}

# Task-Integration runs the `//go:build integration` suites (module/engine/cli
# lifecycles) against a live broker. Connection values come solely from the
# gitignored scripts\local.env, which Import-LocalEnv loads (and requires -- it
# fails the task if the file is absent). Building with the tag always compiles the
# integration files, so this catches compile errors even against an unreachable
# broker.
function Task-Integration {
    Step "integration (-tags integration; live broker)"
    if (-not (Import-LocalEnv)) { return 1 }
    # -count=1: never cache. -tags integration compiles+runs the broker-gated suites.
    $code = Invoke-Logged "integration" $GoRoot "go" @("test", "-count=1", "-tags", "integration", "./...")
    if ($code -eq 0) { Ok "integration" }
    return $code
}

function Task-Cov {
    Step "cov"
    # -count=1: never cache -- coverage must reflect a real run of every package.
    $code = Invoke-Logged "cov" $GoRoot "go" @(
        "test", "-count=1", "-covermode=atomic", "-coverprofile=$CoverageOut", "./...")
    if ($code -ne 0) { return $code }
    Push-Location $GoRoot
    try {
        $funcOut = (& go tool cover -func="$CoverageOut" 2>&1 | ForEach-Object { "$_" } | Out-String -Width 4096)
        Add-Content -Path (Join-Path $LogDir "cov.log") -Value $funcOut -Encoding utf8
        $total = ($funcOut -split "`n" | Where-Object { $_ -match "total:" } | Select-Object -Last 1)
        if ($total) { Write-Host $total.Trim() }
        & go tool cover -html="$CoverageOut" -o "$CoverageHtml" 2>&1 | Out-Null
    } finally {
        Pop-Location
    }
    Ok "cov -> $CoverageHtml"
    return 0
}

# Task-Scan is the single security-scan task (S7): the dependency scanner always,
# plus an image scan only where the project ships an image. This project ships
# binaries only (no Dockerfile), so scan is govulncheck alone -- fatal on findings.
# Invoked as `go tool govulncheck` so it resolves through go.mod's tool + toolchain
# pins (never `go run pkg@version`, which is module-less and ignores both).
function Task-Scan {
    Step "scan (govulncheck; image scan N/A -- no Dockerfile)"
    $code = Invoke-Logged "scan" $GoRoot "go" @("tool", "govulncheck", "./...")
    if ($code -eq 0) { Ok "scan" }
    return $code
}

# Task-Image builds the project's container image. This repo ships no Dockerfile
# (binaries only), so the task warn-and-skips -- shipping no image is a valid
# project shape (S7), not a misconfiguration. If a Dockerfile is added, the local
# single-arch build below runs; CI's tag.yml owns the multi-arch push.
function Task-Image {
    Step "image"
    if (-not (Test-Path (Join-Path $GoRoot "Dockerfile"))) {
        Warn "no Dockerfile; skipping image build (binaries-only project)"
        return 0
    }
    $code = Invoke-Logged "image" $GoRoot "docker" @("build", "--progress=plain", "-t", "${BinName}:$Version", ".")
    if ($code -eq 0) { Ok "image" }
    return $code
}

# Task-Graphify refreshes the local knowledge graph (AST-only, no API cost). It is
# a developer artifact, so it is local-only: it warn-skips when CI is set (belt and
# braces -- CI runs 'all scan', which never reaches graphify anyway) and rides only
# in `full`, never in `all`.
function Task-Graphify {
    Step "graphify (update graph at $ProjectRoot)"
    if ($env:CI) { Warn "CI detected; skipping graphify (local-only developer artifact)"; return 0 }
    $py = if (Have "python") { "python" } elseif (Have "python3") { "python3" } else { $null }
    if (-not $py) { Warn "python not found; skipping graphify"; return 0 }
    $code = Invoke-Logged "graphify" $ProjectRoot $py @("-m", "graphify", "update", ".")
    if ($code -eq 0) { Ok "graphify" }
    return $code
}

# Task-Size builds a fresh binary (Task-Build; identical flags, so the measured
# artifact equals what ships) and reports its size breakdown by package/module via
# go-size-analyzer (gsa). gsa attributes bytes AFTER dead-code elimination, so it
# reflects the linked binary. Pinned to a release tag and run via `go run` so
# nothing is installed globally. Report-only: the build is fatal to this task (its
# code propagates), but a gsa failure (e.g. no network to fetch the tool) only warns.
# gsa requires GOEXPERIMENT=jsonv2 (it uses encoding/json/v2); set + restore it
# around the call (same save/restore pattern Task-Build uses for GOOS/GOARCH) so it
# never leaks into other tasks. gsa also writes its report (box-drawing table) as
# UTF-8; PS 5.1 decodes a native command's stdout using [Console]::OutputEncoding
# (the OEM code page by default), which turns those bytes into mojibake before the
# ASCII fold in Invoke-Logged can act. Force UTF-8 decoding for this capture (also
# saved/restored) so the box chars decode correctly and the fold can flatten them.
# Fallback without gsa: `go tool nm -size -sort=size <binary>`.
function Task-Size {
    $os   = if ($env:TARGET_OS)   { $env:TARGET_OS }   else { "$(& go env GOOS)".Trim() }
    $arch = if ($env:TARGET_ARCH) { $env:TARGET_ARCH } else { "$(& go env GOARCH)".Trim() }
    $out  = Get-BinPath $os $arch
    $bc = Task-Build
    if ($bc -ne 0) { return $bc }
    Step "size (go-size-analyzer)"
    $prev = $env:GOEXPERIMENT
    $env:GOEXPERIMENT = "jsonv2"
    $prevEnc = [Console]::OutputEncoding
    try { [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false) } catch {}
    try {
        $code = Invoke-Logged "size" $GoRoot "go" @(
            "run", "github.com/Zxilly/go-size-analyzer/cmd/gsa@v1.13.0",
            "-f", "text", $out)
    } finally {
        try { [Console]::OutputEncoding = $prevEnc } catch {}
        if ($null -eq $prev) { Remove-Item Env:GOEXPERIMENT -ErrorAction SilentlyContinue }
        else { $env:GOEXPERIMENT = $prev }
    }
    if ($code -eq 0) { Ok "size" }
    return $code
}

function Task-All  { Invoke-Task "build"; Invoke-Task "vet"; Invoke-Task "test" }
function Task-Full { Task-All; Invoke-Task "gencheck"; Invoke-Task "cov"; Invoke-Task "image"; Invoke-Task "scan"; Invoke-Task "graphify" }

function Show-Usage {
    Write-Host @"
Usage: dev.ps1 <task> [task...]

Tasks:
  build     Compile the CLI to dist\semp-workflow-<os>-<arch>[.exe]
            (TARGET_OS / TARGET_ARCH cross-compile; unset = host)
  gen       Regenerate internal/modules/gen_*.go + manifest from specs/
  gencheck  Fail if the checked-in descriptors no longer match specs/
  vet       go vet ./...
  lint      golangci-lint run
  test      go test ./... (unit)
  integration  go test -tags integration ./...  (needs a live broker; requires scripts\local.env)
  cov       Coverage profile -> printed total + coverage.html
  scan      govulncheck ./...                 (image scan N/A -- no Dockerfile)
  image     Container image build             (warn-skips -- no Dockerfile here)
  graphify  python -m graphify update .        (local only; report-only)
  size      build + go-size-analyzer breakdown (report-only)

Aggregates:
  all       build vet test                    (CI runs 'all scan')
  full      all + gencheck + cov + image + scan + graphify
  (integration is opt-in and never part of all/full)

Every task closes with a footer echoed and appended to logs\<task>.log:
  <ISO-8601 w/ offset> | <task> | <duration>s | OK|FAILED (exit N)

Env:
  SEMP_WF_VERSION   override the stamped version (unset: exact git tag, else 0.0.0-dev)
  TARGET_OS/TARGET_ARCH   cross-compile target for 'build' (default: host)

  The 'integration' task reads the broker connection ONLY from a gitignored
  scripts\local.env (copy scripts\local.env.example) -- SEMP_HOST, SEMP_USERNAME,
  SEMP_PASSWORD, SEMP_MSG_VPN, and optional SEMP_VERIFY_SSL (TLS cert verify,
  default false). The task fails if the file is missing.
"@
}

if (-not $Tasks -or $Tasks.Count -eq 0) { Show-Usage; exit 0 }

foreach ($t in $Tasks) {
    switch ($t) {
        { $_ -in @("-h", "--help", "help") } { Show-Usage; exit 0 }
        { $_ -in @("build", "vet", "lint", "test", "integration", "cov", "scan", "image", "graphify", "size", "gen", "gencheck") } { Invoke-Task $t }
        "all"   { Task-All }
        "full"  { Task-Full }
        default { Die "unknown task: $t (run 'dev.ps1 help')" }
    }
}
