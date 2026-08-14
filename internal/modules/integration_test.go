//go:build integration

// Module-level integration tests drive the real action modules against a live
// Solace SEMP v2 broker, covering the queue, RDP, and access-control lifecycles.
//
// They are gated behind the `integration` build tag and skip unless the broker
// env vars are set:
//
//	SEMP_HOST, SEMP_USERNAME, SEMP_PASSWORD, SEMP_MSG_VPN  (SEMP_VERIFY_SSL optional, default false)
//
// Run with: go test -tags integration ./internal/modules/...
//
// All resources are prefixed TEST-SEMP-WF- and removed at the end of each test.
package modules

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/internal/models"
	"solace-semp-workflow/internal/semp"
)

const integrationPrefix = "TEST-SEMP-WF-"

// integrationClient builds a live SEMP client from the environment, skipping the
// test when the required broker vars are unset. *semp.Client satisfies the
// modules.Client interface, so the exec() helper drives real modules with it.
func integrationClient(t *testing.T) *semp.Client {
	t.Helper()
	host := os.Getenv("SEMP_HOST")
	user := os.Getenv("SEMP_USERNAME")
	pass := os.Getenv("SEMP_PASSWORD")
	vpn := os.Getenv("SEMP_MSG_VPN")
	if host == "" || user == "" || pass == "" || vpn == "" {
		t.Skip("integration test skipped: set SEMP_HOST/SEMP_USERNAME/SEMP_PASSWORD/SEMP_MSG_VPN")
	}
	verify := strings.EqualFold(os.Getenv("SEMP_VERIFY_SSL"), "true")
	return semp.NewClient(host, user, pass, vpn, verify, 30)
}

// vpnPath builds a root-relative path to a VPN-scoped collection item, the same
// way the modules do. The client is rooted at the config API rather than at the
// message VPN, so a teardown path has to carry the VPN hop itself. semp.Path
// percent-encodes each key part, so name is passed raw.
func vpnPath(c *semp.Client, collection, name string) string {
	return semp.Path(semp.VPNSeg(c.DefaultMsgVPN()), semp.Seg{Literal: collection, Parts: []string{name}})
}

// cleanupPaths registers a best-effort teardown that deletes each path (in the
// given order) when the test ends. Errors are ignored.
func cleanupPaths(t *testing.T, c *semp.Client, paths ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, p := range paths {
			_ = c.Delete(p)
		}
	})
}

// TestIntegrationQueueSubscriptionLifecycle walks the full queue + subscription
// lifecycle through the module layer: add → add-again-skipped → update →
// subscription add/skip/delete/skip → delete → delete-again-skipped →
// update-nonexistent-fails.
func TestIntegrationQueueSubscriptionLifecycle(t *testing.T) {
	c := integrationClient(t)
	qName := integrationPrefix + "QUEUE-LIFECYCLE"
	topic := integrationPrefix + "TEST/TOPIC"
	queuePath := vpnPath(c, "queues", qName)

	cleanupPaths(t, c, queuePath)
	_ = c.Delete(queuePath) // clean slate

	// queue.add → OK, then add again → SKIPPED.
	res := exec(t, "queue.add", c, map[string]any{"queueName": qName}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "queue.add", c, map[string]any{"queueName": qName}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)

	// queue.update → OK.
	res = exec(t, "queue.update", c, map[string]any{"queueName": qName, "maxMsgSpoolUsage": 512}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	// q_sub.add → OK, then add again → SKIPPED.
	res = exec(t, "q_sub.add", c, map[string]any{"queueName": qName, "subscriptionTopic": topic}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "q_sub.add", c, map[string]any{"queueName": qName, "subscriptionTopic": topic}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)

	// q_sub.delete → OK, then delete again → SKIPPED.
	res = exec(t, "q_sub.delete", c, map[string]any{"queueName": qName, "subscriptionTopic": topic}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "q_sub.delete", c, map[string]any{"queueName": qName, "subscriptionTopic": topic}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)

	// queue.delete → OK, then delete again → SKIPPED.
	res = exec(t, "queue.delete", c, map[string]any{"queueName": qName}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "queue.delete", c, map[string]any{"queueName": qName}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)

	// queue.update on a now-absent queue → FAILED (never creates on update).
	res = exec(t, "queue.update", c, map[string]any{"queueName": qName, "owner": "x"}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
}

// TestIntegrationRdpLifecycle covers the RDP + REST consumer + queue binding
// lifecycle and the dependency-ordered teardown.
func TestIntegrationRdpLifecycle(t *testing.T) {
	c := integrationClient(t)
	rdpName := integrationPrefix + "RDP-LIFECYCLE"
	rcName := integrationPrefix + "RC"
	qName := integrationPrefix + "QUEUE-RDP"

	rdpPath := vpnPath(c, "restDeliveryPoints", rdpName)
	queuePath := vpnPath(c, "queues", qName)
	// Deleting the RDP cascades its consumers and bindings; the queue is separate.
	cleanupPaths(t, c, rdpPath, queuePath)
	_ = c.Delete(rdpPath)
	_ = c.Delete(queuePath)

	// rdp.add → OK, then add again → SKIPPED.
	res := exec(t, "rdp.add", c, map[string]any{"restDeliveryPointName": rdpName}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "rdp.add", c, map[string]any{"restDeliveryPointName": rdpName}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)

	// rdp_rc.add → OK.
	res = exec(t, "rdp_rc.add", c, map[string]any{
		"restDeliveryPointName": rdpName,
		"restConsumerName":      rcName,
		"remoteHost":            "backend.example.com",
		"remotePort":            443,
		"tlsEnabled":            true,
	}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	// Queue to bind, then rdp_qb.add → OK.
	res = exec(t, "queue.add", c, map[string]any{"queueName": qName}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "rdp_qb.add", c, map[string]any{
		"restDeliveryPointName": rdpName,
		"queueBindingName":      qName,
		"postRequestTarget":     "/api/receive",
	}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	// Teardown in dependency order: binding → consumer → RDP → queue.
	res = exec(t, "rdp_qb.delete", c, map[string]any{"restDeliveryPointName": rdpName, "queueBindingName": qName}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "rdp_rc.delete", c, map[string]any{"restDeliveryPointName": rdpName, "restConsumerName": rcName}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "rdp.delete", c, map[string]any{"restDeliveryPointName": rdpName}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "queue.delete", c, map[string]any{"queueName": qName}, false)
	assert.Equal(t, models.StatusOK, res.Status)
}

// TestIntegrationAccessControlLifecycle covers acl_profile, client_profile, and
// client_username through the same add/skip/dry-run/delete lifecycle, including
// the assertions that dry-run neither creates nor deletes.
func TestIntegrationAccessControlLifecycle(t *testing.T) {
	c := integrationClient(t)

	cases := []struct {
		label      string
		addModule  string
		delModule  string
		argKey     string
		name       string
		collection string
	}{
		{"acl_profile", "acl_profile.add", "acl_profile.delete", "aclProfileName", integrationPrefix + "ACL", "aclProfiles"},
		{"client_profile", "client_profile.add", "client_profile.delete", "clientProfileName", integrationPrefix + "CLIENT-PROFILE", "clientProfiles"},
		{"client_username", "client_username.add", "client_username.delete", "clientUsername", integrationPrefix + "CLIENT-USER", "clientUsernames"},
	}

	for _, tc := range cases {
		tc := tc
		path := vpnPath(c, tc.collection, tc.name)
		cleanupPaths(t, c, path)
		_ = c.Delete(path) // clean slate

		t.Run(tc.label, func(t *testing.T) {
			args := map[string]any{tc.argKey: tc.name}

			// Dry-run add must not create the resource.
			res := exec(t, tc.addModule, c, args, true)
			assert.Equal(t, models.StatusDryRun, res.Status)
			found, _, err := c.Exists(path)
			require.NoError(t, err)
			assert.Falsef(t, found, "dry-run add must not create %s", tc.label)

			// add → OK, then add again → SKIPPED.
			res = exec(t, tc.addModule, c, args, false)
			assert.Equal(t, models.StatusOK, res.Status)

			res = exec(t, tc.addModule, c, args, false)
			assert.Equal(t, models.StatusSkipped, res.Status)

			// Dry-run delete must not remove the resource.
			res = exec(t, tc.delModule, c, args, true)
			assert.Equal(t, models.StatusDryRun, res.Status)
			found, _, err = c.Exists(path)
			require.NoError(t, err)
			assert.Truef(t, found, "dry-run delete must not remove %s", tc.label)

			// delete → OK, then delete again → SKIPPED.
			res = exec(t, tc.delModule, c, args, false)
			assert.Equal(t, models.StatusOK, res.Status)

			res = exec(t, tc.delModule, c, args, false)
			assert.Equal(t, models.StatusSkipped, res.Status)
		})
	}
}
