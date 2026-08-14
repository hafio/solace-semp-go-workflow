package cli

import (
	"bytes"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wferrors "solace-semp-workflow/internal/errors"
	"solace-semp-workflow/internal/modules"
	"solace-semp-workflow/internal/output"
)

// runCLI executes the root command with the given args and returns the exit
// code and captured stdout/stderr. Color is disabled for deterministic output.
func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	origNoColor := color.NoColor
	color.NoColor = true
	origOut, origErr := output.Out, output.ErrOut
	var out, errOut bytes.Buffer
	output.Out, output.ErrOut = &out, &errOut
	t.Cleanup(func() {
		output.Out, output.ErrOut = origOut, origErr
		color.NoColor = origNoColor
	})

	code := 0
	root := newRootCmd("test", &code)
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&errOut)
	if err := root.Execute(); err != nil {
		code = 2 // usage error, matching Execute()
	}
	return code, out.String(), errOut.String()
}

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func TestListModules(t *testing.T) {
	code, out, _ := runCLI(t, "list-modules")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "msg_vpn_queue.add")
	assert.Contains(t, out, "msg_vpn_acl_profile_publish_topic_exception.add")
}

func TestListModulesWritesFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "all-modules.md")
	code, out, _ := runCLI(t, "list-modules", "-o", target)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "Module reference written to:")

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# SEMP Workflow Automation - Module Reference")
}

func TestListModulesWritesSplitReference(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "modules")
	code, out, _ := runCLI(t, "list-modules", "--output-dir", dir)
	require.Equal(t, 0, code)
	assert.Contains(t, out, "Module reference written to:")

	// An index plus one page per family, so the directory is created and every
	// registered family is reachable from the index.
	index, err := os.ReadFile(filepath.Join(dir, "index.md"))
	require.NoError(t, err)
	assert.Contains(t, string(index), "| Family | Actions | Reference |")
	assert.Contains(t, string(index), "[msg_vpn_queue](msg_vpn_queue.md)")

	family, err := os.ReadFile(filepath.Join(dir, "msg_vpn_queue.md"))
	require.NoError(t, err)
	assert.Contains(t, string(family), "# `msg_vpn_queue`")
	assert.Contains(t, string(family), "## `msg_vpn_queue.add`")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	// One page per family plus the index. The exact number tracks the registry,
	// so assert the relationship rather than a literal that would need editing
	// on every spec bump.
	assert.Len(t, entries, len(output.ModuleFamilies(modules.Info()))+1)
}

func TestListModulesRemovesStaleReferencePages(t *testing.T) {
	// A family that disappears from a spec bump must not leave a page behind
	// documenting actions the registry no longer has.
	dir := t.TempDir()
	stale := filepath.Join(dir, "removed_family.md")
	require.NoError(t, os.WriteFile(stale, []byte("# `removed_family`\n"), 0o644))
	keep := filepath.Join(dir, "notes.txt")
	require.NoError(t, os.WriteFile(keep, []byte("not generated\n"), 0o644))

	code, out, _ := runCLI(t, "list-modules", "--output-dir", dir)
	require.Equal(t, 0, code)
	assert.Contains(t, out, "Removed 1 stale reference file(s)")

	assert.NoFileExists(t, stale)
	// Only Markdown is managed; anything else in the directory is left alone.
	assert.FileExists(t, keep)
}

// configWithRequiredInputs passes full validation: it references a bundled
// template and supplies every input that template requires. Distinct from
// coverage_test.go's validConfig, which only needs to LOAD cleanly -- its users
// fail earlier on a missing templates directory and never reach input
// resolution. `validate` now resolves inputs and checks every action's
// arguments, so a config that omits required inputs is no longer valid: it would
// have failed at run time.
const configWithRequiredInputs = `semp:
  host: h
  username: u
  password: p
  msg_vpn: v
workflows:
  - template: app-inbound.new-seq
    inputs:
      domain: D
      system: S
      system_topic: T
      non_service_queue_owner: owner
      aem_client_username: aem
`

func TestValidateBundledTemplateOK(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.yaml", configWithRequiredInputs)
	code, out, _ := runCLI(t, "validate", "-c", cfg)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "Validation passed!")
}

func TestValidateCatchesMissingRequiredInput(t *testing.T) {
	// The same template with no inputs at all. This used to pass validation and
	// then fail at run time, which is the worst of both.
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.yaml",
		"semp:\n  host: h\n  username: u\n  password: p\n  msg_vpn: v\nworkflows:\n  - template: app-inbound.new-seq\n")
	code, _, errOut := runCLI(t, "validate", "-c", cfg)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "Required input")
}

func TestValidateCatchesUnknownActionArgument(t *testing.T) {
	// A misspelled attribute is caught locally, naming the workflow, the task and
	// the argument, instead of travelling to the broker to be rejected there.
	dir := t.TempDir()
	writeFile(t, dir, "bad.yaml", `workflow-templates:
  - name: "Flow"
    inputs:
      required:
      - qname
    actions:
    - name: "Create queue"
      module: "queue.add"
      args:
        queueName: "{{ inputs.qname }}"
        acessType: "exclusive"
`)
	cfg := writeFile(t, dir, "config.yaml",
		"semp:\n  host: h\n  username: u\n  password: p\n  msg_vpn: v\nworkflows:\n  - template: bad.Flow\n    inputs:\n      qname: Q\n")

	code, _, errOut := runCLI(t, "validate", "-c", cfg, "-t", dir)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "unknown argument: acessType")
	assert.Contains(t, errOut, "task 'Create queue'")
	assert.Contains(t, errOut, "queue.add")
}

func TestValidateCatchesUnknownModule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.yaml", `workflow-templates:
  - name: "Flow"
    actions:
    - name: "Typo"
      module: "queue.crate"
      args:
        queueName: "Q"
`)
	cfg := writeFile(t, dir, "config.yaml",
		"semp:\n  host: h\n  username: u\n  password: p\n  msg_vpn: v\nworkflows:\n  - template: bad.Flow\n")

	code, _, errOut := runCLI(t, "validate", "-c", cfg, "-t", dir)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "Unknown module 'queue.crate'")
}

func TestValidateTemplateNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.yaml",
		"semp:\n  host: h\n  username: u\n  password: p\n  msg_vpn: v\nworkflows:\n  - template: nope.Missing\n")
	code, _, errOut := runCLI(t, "validate", "-c", cfg)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "not found")
}

func TestValidateConfigError(t *testing.T) {
	code, _, errOut := runCLI(t, "validate", "-c", filepath.Join(t.TempDir(), "missing.yaml"))
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "Config file not found")
}

func TestRunConfigError(t *testing.T) {
	code, _, _ := runCLI(t, "run", "-c", filepath.Join(t.TempDir(), "missing.yaml"))
	assert.Equal(t, 2, code)
}

func TestRunTemplateNotFoundExit1(t *testing.T) {
	// Config loads and templates load (bundled), but the referenced template is
	// absent → engine synthesizes a FAILED task → exit 1. Stays offline (no broker).
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.yaml",
		"semp:\n  host: h\n  username: u\n  password: p\n  msg_vpn: v\nworkflows:\n  - template: nope.Missing\n")
	code, _, _ := runCLI(t, "run", "-c", cfg)
	assert.Equal(t, 1, code)
}

func TestInitCopiesBundledTemplates(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "templates")
	code, out, _ := runCLI(t, "init", "-o", target)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "write")

	entries, err := os.ReadDir(target)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// Second run without --force skips existing files.
	code, out, _ = runCLI(t, "init", "-o", target)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "skip")
}

func TestMissingConfigFlagIsUsageError(t *testing.T) {
	// run requires --config; omitting it is a cobra usage error → exit 2.
	code, _, _ := runCLI(t, "run")
	assert.Equal(t, 2, code)
}

func TestClassifyExit(t *testing.T) {
	assert.Equal(t, 2, classifyExit(wferrors.NewConfigError("bad config")))
	assert.Equal(t, 2, classifyExit(wferrors.NewTemplateError("bad template")))
	assert.Equal(t, 1, classifyExit(wferrors.NewWorkflowError("generic")))
	assert.Equal(t, 1, classifyExit(stderrors.New("plain")))
}
