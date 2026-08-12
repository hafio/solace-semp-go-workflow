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
	assert.Contains(t, out, "queue.add")
	assert.Contains(t, out, "acl_publish_exception.add")
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

func TestValidateBundledTemplateOK(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.yaml",
		"semp:\n  host: h\n  username: u\n  password: p\n  msg_vpn: v\nworkflows:\n  - template: app-inbound.new-seq\n")
	code, out, _ := runCLI(t, "validate", "-c", cfg)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "Validation passed!")
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
