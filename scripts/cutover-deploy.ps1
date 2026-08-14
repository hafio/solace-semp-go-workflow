# Build the Linux binary and copy it, with the cutover check, into a broker
# container.
#
# The cutover check needs only two files in the container: the binary and the
# script. Workflow templates are embedded in the binary and the configs are
# generated at run time, so nothing else is copied.
#
#   .\scripts\cutover-deploy.ps1 -Container solace
#   .\scripts\cutover-deploy.ps1 -Container solace -Dir /var/tmp/cutover
#   .\scripts\cutover-deploy.ps1 -Container solace -Arch arm64
#   .\scripts\cutover-deploy.ps1 -Container solace -Run
#
# -Run executes the check inside the container afterwards. Without it the script
# prints the command to run yourself.
#
# The broker connection defaults inside the container to http://localhost:8080
# with admin/admin on the "default" VPN. Override with -SempHost, -SempUsername,
# -SempPassword and -SempMsgVpn, which are passed to the container's environment
# for that one command only.

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Container,

    [string]$Dir = "/tmp",
    [string]$Arch = "amd64",
    [switch]$Run,
    [switch]$Keep,

    [string]$SempHost,
    [string]$SempUsername,
    [string]$SempPassword,
    [string]$SempMsgVpn
)

$ErrorActionPreference = "Stop"

$RepoRoot  = Split-Path -Parent $PSScriptRoot
$DevScript = Join-Path $PSScriptRoot "dev.ps1"
$CheckSrc  = Join-Path $PSScriptRoot "cutover-check.sh"
$BinName   = "semp-workflow-linux-$Arch"
$BinSrc    = Join-Path (Join-Path $RepoRoot "dist") $BinName

function Step  { param([string]$m) Write-Host ""; Write-Host "== $m" -ForegroundColor Cyan }
function Info  { param([string]$m) Write-Host "   $m" }
function Fail  { param([string]$m) Write-Host "ERROR: $m" -ForegroundColor Red; exit 1 }

if (-not (Test-Path $DevScript)) { Fail "dev.ps1 not found at $DevScript" }
if (-not (Test-Path $CheckSrc))  { Fail "cutover-check.sh not found at $CheckSrc" }

# Resolve docker to a concrete .exe once, and call it through that path
# everywhere below. The Docker Desktop bin directory contains BOTH docker.exe and
# an extensionless "docker"; if PowerShell selects the latter, Windows cannot
# execute it directly and falls back to ShellExecute, which opens the ARGUMENT in
# its associated application rather than running docker. With no association for
# .sh that surfaces as an "open with" dialog on cutover-check.sh instead of a
# copy. Pinning the .exe removes the ambiguity.
$Docker = $null
foreach ($c in @(Get-Command docker -All -ErrorAction SilentlyContinue)) {
    if ($c.Source -and $c.Source.ToLower().EndsWith(".exe")) { $Docker = $c.Source; break }
}
if (-not $Docker) { Fail "docker.exe not found on PATH. Is Docker Desktop installed and started?" }

# The container has to exist before anything is built, so a typo in the name
# fails in two seconds rather than after a cross-compile.
Step "Checking container '$Container'"
$state = & $Docker inspect -f '{{.State.Running}}' $Container
if ($LASTEXITCODE -ne 0) { Fail "container '$Container' not found. Is it running? (docker ps)" }
if ("$state".Trim() -ne "true") { Fail "container '$Container' exists but is not running" }
Info "running"

Step "Building $BinName"
# TARGET_OS / TARGET_ARCH are read by dev.ps1. PowerShell has no inline
# VAR=x command prefix, so they are set and then restored: leaving them set
# would make the next plain `dev.ps1 build` silently produce a Linux binary.
$prevOs    = $env:TARGET_OS
$prevArch  = $env:TARGET_ARCH
$buildFrom = Get-Date
try {
    $env:TARGET_OS   = "linux"
    $env:TARGET_ARCH = $Arch
    & $DevScript build
    if ($LASTEXITCODE -ne 0) { Fail "build failed (see scripts\logs\build.log)" }
}
finally {
    if ($null -eq $prevOs)   { Remove-Item Env:TARGET_OS   -ErrorAction SilentlyContinue } else { $env:TARGET_OS = $prevOs }
    if ($null -eq $prevArch) { Remove-Item Env:TARGET_ARCH -ErrorAction SilentlyContinue } else { $env:TARGET_ARCH = $prevArch }
}

# dev.ps1 only sets an exit code when a task FAILS; on success it simply runs to
# the end, leaving $LASTEXITCODE holding whatever the previous command set. So
# the exit-code check above cannot be trusted on its own: require the binary to
# have been written by this build, which also catches a stale one left by an
# earlier run.
if (-not (Test-Path $BinSrc)) { Fail "expected binary not found at $BinSrc" }
if ((Get-Item $BinSrc).LastWriteTime -lt $buildFrom) {
    Fail "$BinSrc was not rebuilt (it predates this run). The build silently did nothing; check scripts\logs\build.log"
}
$size = [math]::Round((Get-Item $BinSrc).Length / 1MB, 1)
Info "$BinSrc  ($size MB)"

Step "Copying into ${Container}:$Dir"
& $Docker exec $Container mkdir -p $Dir
if ($LASTEXITCODE -ne 0) { Fail "could not create $Dir in the container" }

$BinDst   = "$Dir/semp-workflow"
$CheckDst = "$Dir/cutover-check.sh"

& $Docker cp $BinSrc "${Container}:$BinDst"
if ($LASTEXITCODE -ne 0) { Fail "failed to copy the binary" }
Info "$BinDst"

& $Docker cp $CheckSrc "${Container}:$CheckDst"
if ($LASTEXITCODE -ne 0) { Fail "failed to copy cutover-check.sh" }
Info "$CheckDst"

# docker cp does not preserve the executable bit in every configuration, so set
# it explicitly rather than relying on it.
& $Docker exec $Container chmod +x $BinDst $CheckDst
if ($LASTEXITCODE -ne 0) { Fail "could not make the files executable in the container" }

# Prove the binary actually runs on the container's architecture. A mismatched
# build fails here with "exec format error" rather than midway through the check.
$version = & $Docker exec $Container $BinDst --version
if ($LASTEXITCODE -ne 0) { Fail "the binary does not run in the container. Wrong -Arch? (try arm64)" }
Info "verified: $("$version".Trim())"

# The check finds the binary next to itself, so no --binary argument is needed.
$envArgs = @()
if ($SempHost)     { $envArgs += @("-e", "SEMP_HOST=$SempHost") }
if ($SempUsername) { $envArgs += @("-e", "SEMP_USERNAME=$SempUsername") }
if ($SempPassword) { $envArgs += @("-e", "SEMP_PASSWORD=$SempPassword") }
if ($SempMsgVpn)   { $envArgs += @("-e", "SEMP_MSG_VPN=$SempMsgVpn") }

$checkArgs = ""
if ($Keep) { $checkArgs = " --keep" }

if (-not $Run) {
    Step "Ready"
    Info "Run the check with:"
    Write-Host ""
    Write-Host "  docker exec -it $Container $CheckDst$checkArgs"
    Write-Host ""
    Info "Logs are written inside the container and their path is printed at the end."
    Info "Copy them back with:  docker cp ${Container}:<logs-path> .\scripts\logs\m4-run"
    exit 0
}

Step "Running the cutover check"
$execArgs = @("exec")
$execArgs += $envArgs
$execArgs += @($Container, $CheckDst)
if ($Keep) { $execArgs += "--keep" }

& $Docker @execArgs
$checkCode = $LASTEXITCODE

Write-Host ""
if ($checkCode -eq 0) {
    Write-Host "Cutover check passed." -ForegroundColor Green
}
else {
    Write-Host "Cutover check FAILED (exit $checkCode)." -ForegroundColor Red
    Write-Host "Copy the logs out with:  docker cp ${Container}:<logs-path> .\scripts\logs\m4-run"
}
exit $checkCode
