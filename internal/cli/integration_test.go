//go:build integration

// CLI-level integration tests exercise the run/validate/list-modules commands
// against a live Solace SEMP v2 broker. The fixture template lives in
// testdata/integration and is passed via --templates-dir.
//
// Gated behind the `integration` build tag; skips unless the broker env vars are
// set: SEMP_HOST, SEMP_USERNAME, SEMP_PASSWORD, SEMP_MSG_VPN (SEMP_VERIFY_SSL
// optional, default false).
//
// Run with: go test -tags integration ./internal/cli/...
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/internal/semp"
)

const integrationPrefix = "TEST-SEMP-WF-CLI"

func integrationEnv(t *testing.T) (host, user, pass, vpn string, verify bool) {
	t.Helper()
	host = os.Getenv("SEMP_HOST")
	user = os.Getenv("SEMP_USERNAME")
	pass = os.Getenv("SEMP_PASSWORD")
	vpn = os.Getenv("SEMP_MSG_VPN")
	if host == "" || user == "" || pass == "" || vpn == "" {
		t.Skip("integration test skipped: set SEMP_HOST/SEMP_USERNAME/SEMP_PASSWORD/SEMP_MSG_VPN")
	}
	verify = strings.EqualFold(os.Getenv("SEMP_VERIFY_SSL"), "true")
	return
}

// integrationFixturesDir returns the absolute path to testdata/integration so
// it can be passed to --templates-dir regardless of the config file location.
func integrationFixturesDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "integration"))
	require.NoError(t, err)
	return dir
}

// writeIntegrationConfig writes a config.yaml referencing the given template and
// returns its path. Templates are supplied separately via --templates-dir.
func writeIntegrationConfig(t *testing.T, dir, template string) string {
	t.Helper()
	host, user, pass, vpn, verify := integrationEnv(t)
	body := fmt.Sprintf(`semp:
  host: "%s"
  username: "%s"
  password: "%s"
  msg_vpn: "%s"
  verify_ssl: %t
  timeout: 30
workflows:
  - template: "%s"
    inputs:
      prefix: "%s"
`, host, user, pass, vpn, verify, template, integrationPrefix)
	return writeFile(t, dir, "config.yaml", body)
}

func integrationSempClient(t *testing.T) *semp.Client {
	t.Helper()
	host, user, pass, vpn, verify := integrationEnv(t)
	return semp.NewClient(host, user, pass, vpn, verify, 30)
}

func TestIntegrationCLIListModules(t *testing.T) {
	integrationEnv(t) // gate on the broker env for suite consistency
	code, out, _ := runCLI(t, "list-modules")
	assert.Equal(t, 0, code)
	for _, name := range []string{
		"acl_profile.add", "acl_profile.delete",
		"client_profile.add", "client_profile.delete",
		"client_username.add", "client_username.delete",
		"queue.add", "queue.delete", "queue.update",
		"q_sub.add", "q_sub.delete",
		"rdp.add", "rdp.delete",
		"rdp_rc.add", "rdp_rc.delete",
		"rdp_qb.add", "rdp_qb.delete",
	} {
		assert.Containsf(t, out, name, "missing module in list-modules output: %s", name)
	}
}

func TestIntegrationCLIValidate(t *testing.T) {
	fixtures := integrationFixturesDir(t)

	// Valid config + real template → exit 0.
	cfg := writeIntegrationConfig(t, t.TempDir(), "test-artifacts.create")
	code, out, _ := runCLI(t, "validate", "-c", cfg, "-t", fixtures)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "Validation passed!")

	// Unknown template ref → non-zero exit.
	badCfg := writeIntegrationConfig(t, t.TempDir(), "bad.nonexistent")
	code, _, errOut := runCLI(t, "validate", "-c", badCfg, "-t", fixtures)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, errOut, "not found")
}

func TestIntegrationCLIRun(t *testing.T) {
	fixtures := integrationFixturesDir(t)
	c := integrationSempClient(t)

	queuePath := "queues/" + semp.Enc(integrationPrefix+"-QUEUE")
	rdpPath := "restDeliveryPoints/" + semp.Enc(integrationPrefix+"-RDP")
	rcPath := rdpPath + "/restConsumers/" + semp.Enc(integrationPrefix+"-RC")
	qbPath := rdpPath + "/queueBindings/" + semp.Enc(integrationPrefix+"-QUEUE")
	cuPath := "clientUsernames/" + semp.Enc(integrationPrefix+"-USER")
	cpPath := "clientProfiles/" + semp.Enc(integrationPrefix+"-CP")
	aclPath := "aclProfiles/" + semp.Enc(integrationPrefix+"-ACL")

	clean := func() {
		for _, p := range []string{qbPath, rcPath, rdpPath, queuePath, cuPath, cpPath, aclPath} {
			_ = c.Delete(p)
		}
	}
	clean()
	t.Cleanup(clean)

	cfg := writeIntegrationConfig(t, t.TempDir(), "test-artifacts.create")

	// Dry-run must exit 0 and create nothing.
	code, _, _ := runCLI(t, "run", "-c", cfg, "-t", fixtures, "--dry-run")
	assert.Equal(t, 0, code)
	found, _, err := c.Exists(queuePath)
	require.NoError(t, err)
	assert.False(t, found, "dry-run must not create the queue")

	// Real run must exit 0 and provision the resources.
	code, _, _ = runCLI(t, "run", "-c", cfg, "-t", fixtures)
	assert.Equal(t, 0, code)

	foundQ, _, err := c.Exists(queuePath)
	require.NoError(t, err)
	assert.True(t, foundQ, "queue should exist after run")

	foundRDP, _, err := c.Exists(rdpPath)
	require.NoError(t, err)
	assert.True(t, foundRDP, "RDP should exist after run")
}
