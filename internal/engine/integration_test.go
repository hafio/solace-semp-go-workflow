//go:build integration

// Engine-level integration test drives Engine.Run() against a live Solace SEMP v2
// broker using the self-contained fixture template
// (testdata/integration/test-artifacts.yaml), which exercises every supported
// artifact type: acl_profile, client_profile, client_username, queue, q_sub, rdp,
// rdp_rc, rdp_qb.
//
// Gated behind the `integration` build tag; skips unless the broker env vars are
// set: SEMP_HOST, SEMP_USERNAME, SEMP_PASSWORD, SEMP_MSG_VPN (SEMP_VERIFY_SSL
// optional, default false).
//
// Run with: go test -tags integration ./internal/engine/...
package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/internal/config"
	"solace-semp-workflow/internal/models"
	"solace-semp-workflow/internal/semp"
)

// integrationPrefix names every resource this test provisions.
const integrationPrefix = "TEST-SEMP-WF-ENG"

// fixturesDir resolves to testdata/integration relative to this package dir.
var fixturesDir = filepath.Join("..", "..", "testdata", "integration")

func integrationSemp(t *testing.T) config.SempConfig {
	t.Helper()
	host := os.Getenv("SEMP_HOST")
	user := os.Getenv("SEMP_USERNAME")
	pass := os.Getenv("SEMP_PASSWORD")
	vpn := os.Getenv("SEMP_MSG_VPN")
	if host == "" || user == "" || pass == "" || vpn == "" {
		t.Skip("integration test skipped: set SEMP_HOST/SEMP_USERNAME/SEMP_PASSWORD/SEMP_MSG_VPN")
	}
	return config.SempConfig{
		Host:      host,
		Username:  user,
		Password:  pass,
		MsgVPN:    vpn,
		VerifySSL: strings.EqualFold(os.Getenv("SEMP_VERIFY_SSL"), "true"),
		Timeout:   30,
	}
}

func makeConfig(t *testing.T, workflows []config.WorkflowEntry) *config.AppConfig {
	t.Helper()
	return &config.AppConfig{
		Semp:                integrationSemp(t),
		GlobalVars:          map[string]any{},
		Workflows:           workflows,
		TemplatesDir:        fixturesDir,
		UseBundledTemplates: false,
	}
}

// TestIntegrationEngineLifecycle runs the create/delete fixture workflows and
// asserts the full dry-run → create → idempotent-rerun → delete → rerun cycle,
// verifying broker state at each stage.
func TestIntegrationEngineLifecycle(t *testing.T) {
	sc := integrationSemp(t) // skips here if env is unset
	c := semp.NewClient(sc.Host, sc.Username, sc.Password, sc.MsgVPN, sc.VerifySSL, sc.Timeout)

	inputs := map[string]any{"prefix": integrationPrefix}
	createWF := []config.WorkflowEntry{{Template: "test-artifacts.create", Inputs: inputs}}
	deleteWF := []config.WorkflowEntry{{Template: "test-artifacts.delete", Inputs: inputs}}

	// Derived resource paths — must match the fixture template defaults exactly.
	aclPath := "aclProfiles/" + semp.Enc(integrationPrefix+"-ACL")
	cpPath := "clientProfiles/" + semp.Enc(integrationPrefix+"-CP")
	cuPath := "clientUsernames/" + semp.Enc(integrationPrefix+"-USER")
	queuePath := "queues/" + semp.Enc(integrationPrefix+"-QUEUE")
	rdpPath := "restDeliveryPoints/" + semp.Enc(integrationPrefix+"-RDP")
	rcPath := rdpPath + "/restConsumers/" + semp.Enc(integrationPrefix+"-RC")
	qbPath := rdpPath + "/queueBindings/" + semp.Enc(integrationPrefix+"-QUEUE")

	// Teardown in reverse dependency order, and start from a clean slate.
	clean := func() {
		for _, p := range []string{qbPath, rcPath, rdpPath, queuePath, cuPath, cpPath, aclPath} {
			_ = c.Delete(p)
		}
	}
	clean()
	t.Cleanup(clean)

	run := func(t *testing.T, workflows []config.WorkflowEntry, dryRun bool) []models.WorkflowResult {
		t.Helper()
		silence(t) // suppress banner/recap output (defined in engine_test.go)
		eng, err := NewEngine(makeConfig(t, workflows), dryRun, false)
		require.NoError(t, err)
		return eng.Run()
	}

	assertAll := func(t *testing.T, tasks []models.ActionResult, want ...models.ResultStatus) {
		t.Helper()
		for _, r := range tasks {
			assert.Containsf(t, want, r.Status, "task %q (%s) had unexpected status %q", r.TaskName, r.Module, r.Status)
		}
	}

	t.Run("dry-run creates nothing", func(t *testing.T) {
		results := run(t, createWF, true)
		require.Len(t, results, 1)
		assert.False(t, results[0].HasFailures())
		assertAll(t, results[0].TaskResults, models.StatusDryRun)

		for _, p := range []string{queuePath, rdpPath, aclPath} {
			found, _, err := c.Exists(p)
			require.NoError(t, err)
			assert.Falsef(t, found, "dry-run unexpectedly created %s", p)
		}
	})

	t.Run("create all artifacts", func(t *testing.T) {
		results := run(t, createWF, false)
		require.Len(t, results, 1)
		assert.False(t, results[0].HasFailures())
		assertAll(t, results[0].TaskResults, models.StatusOK, models.StatusSkipped)

		for _, p := range []string{aclPath, cpPath, cuPath, queuePath, rdpPath} {
			found, _, err := c.Exists(p)
			require.NoError(t, err)
			assert.Truef(t, found, "expected broker object not found: %s", p)
		}
	})

	t.Run("rerun all skipped", func(t *testing.T) {
		results := run(t, createWF, false)
		require.Len(t, results, 1)
		assert.False(t, results[0].HasFailures())
		assertAll(t, results[0].TaskResults, models.StatusSkipped)
	})

	t.Run("delete all artifacts", func(t *testing.T) {
		results := run(t, deleteWF, false)
		require.Len(t, results, 1)
		assert.False(t, results[0].HasFailures())
		assertAll(t, results[0].TaskResults, models.StatusOK, models.StatusSkipped)

		for _, p := range []string{aclPath, cpPath, cuPath, queuePath, rdpPath} {
			found, _, err := c.Exists(p)
			require.NoError(t, err)
			assert.Falsef(t, found, "resource still present after delete: %s", p)
		}
	})

	t.Run("delete again all skipped", func(t *testing.T) {
		results := run(t, deleteWF, false)
		require.Len(t, results, 1)
		assert.False(t, results[0].HasFailures())
		assertAll(t, results[0].TaskResults, models.StatusSkipped)
	})
}
