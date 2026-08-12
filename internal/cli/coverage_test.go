package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validConfig is the minimal config body reused across the offline CLI tests;
// it loads cleanly so execution reaches the templates-dir override branch.
const validConfig = "semp:\n  host: h\n  username: u\n  password: p\n  msg_vpn: v\nworkflows:\n  - template: app-inbound.new-seq\n"

// TestRunTemplatesDirNotFound drives the run command's --templates-dir override
// (disabling the bundled fallback) and the NewEngine error branch: the config
// loads but the explicit templates dir is missing, so NewEngine returns a
// TemplateError → classifyExit → exit 2.
func TestRunTemplatesDirNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.yaml", validConfig)
	missing := filepath.Join(dir, "no-such-templates")
	code, _, errOut := runCLI(t, "run", "-c", cfg, "-t", missing)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "Templates directory not found")
}

// TestValidateTemplatesDirNotFound drives the validate command's
// --templates-dir override, the non-bundled LoadTemplatesDir branch, and its
// error handling: the missing dir yields a TemplateError → exit 2.
func TestValidateTemplatesDirNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFile(t, dir, "config.yaml", validConfig)
	missing := filepath.Join(dir, "no-such-templates")
	code, _, errOut := runCLI(t, "validate", "-c", cfg, "-t", missing)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "Templates directory not found")
}

// TestListModulesWriteFailure points --output at an existing directory so
// os.WriteFile fails. list-modules does not set the exit code, so it stays 0
// while the failure is reported to stderr.
func TestListModulesWriteFailure(t *testing.T) {
	dir := t.TempDir() // a directory path is not a writable file target
	code, _, errOut := runCLI(t, "list-modules", "-o", dir)
	assert.Equal(t, 0, code)
	assert.Contains(t, errOut, "Failed to write module reference")
}

// TestInitMkdirAllFailure makes the output dir's parent a regular file, so
// os.MkdirAll cannot create the directory → exit 2.
func TestInitMkdirAllFailure(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "not-a-dir", "x")
	target := filepath.Join(file, "child") // parent component is a file
	code, _, errOut := runCLI(t, "init", "-o", target)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "Failed to create output directory")
}

// TestInitWriteFailure pre-creates a directory whose name collides with the
// first bundled template file, then forces overwrite so the skip branch is
// bypassed and os.WriteFile fails writing over a directory → exit 2.
func TestInitWriteFailure(t *testing.T) {
	dir := t.TempDir()
	// "app-inbound.yaml" sorts first among the bundled templates.
	collide := filepath.Join(dir, "app-inbound.yaml")
	require.NoError(t, os.MkdirAll(collide, 0o755))
	code, _, errOut := runCLI(t, "init", "-o", dir, "--force")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "Failed to write")
}
