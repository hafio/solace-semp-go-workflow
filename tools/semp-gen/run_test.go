package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realSpecs locates the committed specifications, skipping the test if absent.
func realSpecs(t *testing.T) (software, appliance string) {
	t.Helper()
	software = filepath.Join("..", "..", "specs", "software", "10.26.0.8755", "semp-v2-config.json")
	appliance = filepath.Join("..", "..", "specs", "appliance", "10.26.0.8755", "semp-v2-config.json")
	for _, p := range []string{software, appliance} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("specification not present at %s", p)
		}
	}
	return software, appliance
}

func genFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "gen_") && strings.HasSuffix(e.Name(), ".go") {
			out = append(out, e.Name())
		}
	}
	return out
}

// The generator's whole contract in one test: generating into an empty directory
// produces the full descriptor set, and checking it immediately afterwards
// reports no drift. If those two disagree, the committed output and the drift
// gate can never both be right.
func TestRunGeneratesThenReportsNoDrift(t *testing.T) {
	sw, ap := realSpecs(t)
	dir := t.TempDir()

	require.NoError(t, run(sw, ap, dir, false))

	assert.Len(t, genFiles(t, dir), 86)
	assert.FileExists(t, filepath.Join(dir, "manifest_generated.go"))

	manifest, err := os.ReadFile(filepath.Join(dir, "manifest_generated.go"))
	require.NoError(t, err)
	assert.Contains(t, string(manifest), "ExpectedModules")
	assert.Equal(t, 225, strings.Count(string(manifest), "\t\""), "one quoted action per line")

	// Same inputs, same outputs: the check must pass against what was just written.
	assert.NoError(t, run(sw, ap, dir, true))
}

func TestRunCheckReportsMissingOutput(t *testing.T) {
	sw, ap := realSpecs(t)

	err := run(sw, ap, t.TempDir(), true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "do not match")
	assert.Contains(t, err.Error(), "missing")
	// The message says how to fix it, since that is the whole point of the gate.
	assert.Contains(t, err.Error(), "gen")
}

func TestRunCheckReportsAnOutOfDateFile(t *testing.T) {
	sw, ap := realSpecs(t)
	dir := t.TempDir()
	require.NoError(t, run(sw, ap, dir, false))

	// Simulate a hand-edit of generated output, which is exactly what the gate
	// exists to catch.
	// The name must be one the generator still produces, or this exercises the
	// stale-file branch instead of the hand-edit branch.
	target := filepath.Join(dir, "gen_msg_vpn_queue.go")
	require.NoError(t, os.WriteFile(target, []byte("package modules\n"), 0o644))

	err := run(sw, ap, dir, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of date")
	assert.Contains(t, err.Error(), "gen_msg_vpn_queue.go")
}

func TestRunRemovesADescriptorTheSpecNoLongerProduces(t *testing.T) {
	sw, ap := realSpecs(t)
	dir := t.TempDir()

	stale := filepath.Join(dir, "gen_removed_object.go")
	require.NoError(t, os.WriteFile(stale, []byte("package modules\n"), 0o644))
	keep := filepath.Join(dir, "driver.go")
	require.NoError(t, os.WriteFile(keep, []byte("package modules\n"), 0o644))

	require.NoError(t, run(sw, ap, dir, false))

	// A descriptor for an object the specification has dropped would otherwise
	// keep registering an action that no longer exists.
	assert.NoFileExists(t, stale)
	// Hand-written files in the same package are not the generator's to remove.
	assert.FileExists(t, keep)
}

func TestRunCheckReportsAStaleFile(t *testing.T) {
	sw, ap := realSpecs(t)
	dir := t.TempDir()
	require.NoError(t, run(sw, ap, dir, false))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gen_gone.go"), []byte("package modules\n"), 0o644))

	err := run(sw, ap, dir, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale")
	assert.Contains(t, err.Error(), "gen_gone.go")
}

func TestRunRejectsMismatchedSpecificationVersions(t *testing.T) {
	// Merging two broker versions would silently produce an API that is neither.
	dir := t.TempDir()
	write := func(name, version string) string {
		p := filepath.Join(dir, name)
		body := `{"info":{"version":"` + version + `"},` +
			`"paths":{"/":{"patch":{"operationId":"updateBroker","description":"",` +
			`"parameters":[{"in":"body","name":"body","schema":{"$ref":"#/definitions/Broker"}}]}}},` +
			`"definitions":{"Broker":{"properties":{}}}}`
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
		return p
	}
	sw := write("sw.json", "10.26.0.8755")
	ap := write("ap.json", "10.25.0.1000")

	err := run(sw, ap, t.TempDir(), false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "versions differ")
}

func TestRunReportsAMissingSpecification(t *testing.T) {
	err := run(filepath.Join(t.TempDir(), "nope.json"), filepath.Join(t.TempDir(), "nope.json"), t.TempDir(), false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope.json")
}
