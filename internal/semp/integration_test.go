//go:build integration

// Package semp integration tests exercise a real Solace SEMP v2 broker. They are
// gated behind the `integration` build tag and skip unless the broker env vars
// are set:
//
//	SEMP_HOST, SEMP_USERNAME, SEMP_PASSWORD, SEMP_MSG_VPN
//
// Run with: go test -tags integration ./internal/semp/...
//
// All resources are prefixed TEST-SEMP-WF- and removed at the end of the test.
package semp

import (
	stderrors "errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wferrors "solace-semp-workflow/internal/errors"
)

const testResourcePrefix = "TEST-SEMP-WF-"

func integrationClient(t *testing.T) *Client {
	t.Helper()
	host := os.Getenv("SEMP_HOST")
	user := os.Getenv("SEMP_USERNAME")
	pass := os.Getenv("SEMP_PASSWORD")
	vpn := os.Getenv("SEMP_MSG_VPN")
	if host == "" || user == "" || pass == "" || vpn == "" {
		t.Skip("integration test skipped: set SEMP_HOST/SEMP_USERNAME/SEMP_PASSWORD/SEMP_MSG_VPN")
	}
	return NewClient(host, user, pass, vpn, false, 30)
}

func TestIntegrationConnection(t *testing.T) {
	c := integrationClient(t)
	require.True(t, c.TestConnection(), "broker should be reachable with the provided credentials")
}

func TestIntegrationQueueLifecycle(t *testing.T) {
	c := integrationClient(t)
	queueName := testResourcePrefix + "queue"
	queuePath := "queues/" + Enc(queueName)

	// Ensure a clean slate and always clean up.
	_ = c.Delete(queuePath)
	t.Cleanup(func() { _ = c.Delete(queuePath) })

	// Absent before creation.
	exists, _, err := c.Exists(queuePath)
	require.NoError(t, err)
	assert.False(t, exists)

	// Create.
	_, err = c.Create("queues", map[string]any{
		"queueName":      queueName,
		"accessType":     "exclusive",
		"ingressEnabled": true,
		"egressEnabled":  true,
		"permission":     "consume",
	})
	require.NoError(t, err)

	// Present after creation.
	exists, data, err := c.Exists(queuePath)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, queueName, data["queueName"])

	// Creating again returns AlreadyExists.
	_, err = c.Create("queues", map[string]any{"queueName": queueName, "accessType": "exclusive"})
	require.Error(t, err)
	var se *wferrors.SEMPError
	require.True(t, stderrors.As(err, &se))
	assert.Equal(t, AlreadyExists, se.SempCode)

	// Update.
	_, err = c.Update(queuePath, map[string]any{"egressEnabled": false})
	require.NoError(t, err)

	// Delete.
	require.NoError(t, c.Delete(queuePath))

	// Absent again.
	exists, _, err = c.Exists(queuePath)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestIntegrationExistsNotFound(t *testing.T) {
	c := integrationClient(t)
	path := "queues/" + Enc(fmt.Sprintf("%sdoes-not-exist", testResourcePrefix))
	exists, _, err := c.Exists(path)
	require.NoError(t, err)
	assert.False(t, exists)
}
