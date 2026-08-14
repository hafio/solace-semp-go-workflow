package modules

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/internal/models"
	"solace-semp-workflow/internal/semp"
)

// exec is a convenience wrapper: look up a module by name and run it.
func exec(t *testing.T, name string, c Client, args map[string]any, dryRun bool) models.ActionResult {
	t.Helper()
	m, err := Get(name)
	require.NoError(t, err)
	return m.Execute(c, args, dryRun)
}

func TestRegistryCompleteness(t *testing.T) {
	want := []string{
		"queue.add", "queue.delete", "queue.update",
		"q_sub.add", "q_sub.delete",
		"rdp.add", "rdp.delete", "rdp.update",
		"rdp_rc.add", "rdp_rc.delete",
		"rdp_qb.add", "rdp_qb.delete",
		"client_profile.add", "client_profile.delete", "client_profile.update",
		"client_username.add", "client_username.delete", "client_username.update",
		"acl_profile.add", "acl_profile.delete",
		"acl_publish_exception.add", "acl_publish_exception.delete",
		"acl_subscribe_exception.add", "acl_subscribe_exception.delete",
	}
	// The registry must match the generated manifest exactly, so an action that
	// disappears from a spec bump fails here instead of vanishing quietly.
	assert.ElementsMatch(t, ExpectedModules, List())

	// The 24 names above are the backward-compatibility contract: existing
	// templates and customer configs use them, so they are asserted separately
	// from the generated total and must never depend on it.
	for _, name := range want {
		_, err := Get(name)
		assert.NoErrorf(t, err, "module %s should be registered", name)
	}
}

func TestGetUnknownModule(t *testing.T) {
	_, err := Get("nope.verb")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown module 'nope.verb'")
	assert.Contains(t, err.Error(), "Available:")
}

func TestInfoHasParams(t *testing.T) {
	info := Info()
	assert.Len(t, info, len(ExpectedModules))
	// Info is the documented surface, so it is keyed by canonical names only --
	// the legacy short names resolve through Get but are deliberately not listed.
	assert.NotContains(t, info, "queue.add")
	qa := info["msg_vpn_queue.add"]
	assert.NotEmpty(t, qa.Description)
	require.NotEmpty(t, qa.Params)
	// Params documents the canonical snake_case spelling; queueName still works
	// as an argument (see TestLegacyAndCanonicalArgsProduceTheSamePayload).
	assert.Equal(t, "queue_name", qa.Params[0].Name) // required name is first (ordered)
	assert.True(t, qa.Params[0].Required)
}

// --- queue ---------------------------------------------------------------

func TestQueueAddCreatesWithDerivedFields(t *testing.T) {
	fc := absent()
	res := exec(t, "queue.add", fc, map[string]any{
		"queueName":          "TEST-Q",
		"maxTtl":             60,
		"maxRedeliveryCount": -1,
		"ingressEnabled":     "true",
	}, false)

	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, "msgVpns/test-vpn/queues", fc.createPath)
	assert.Equal(t, true, fc.createBody["respectTtlEnabled"]) // maxTtl > 0
	assert.Equal(t, 0, fc.createBody["maxRedeliveryCount"])   // -1 sentinel -> 0
	assert.Equal(t, false, fc.createBody["redeliveryEnabled"])
	assert.Equal(t, true, fc.createBody["ingressEnabled"])
	assert.Equal(t, 60, fc.createBody["maxTtl"])
}

func TestQueueAddRedeliveryEnabledDerivation(t *testing.T) {
	fc := absent()
	exec(t, "queue.add", fc, map[string]any{"queueName": "Q", "maxRedeliveryCount": 5}, false)
	assert.Equal(t, true, fc.createBody["redeliveryEnabled"])
	assert.Equal(t, 5, fc.createBody["maxRedeliveryCount"])

	fc = absent()
	exec(t, "queue.add", fc, map[string]any{"queueName": "Q", "maxRedeliveryCount": 0}, false)
	assert.Equal(t, false, fc.createBody["redeliveryEnabled"])
}

func TestQueueAddSkippedWhenExists(t *testing.T) {
	res := exec(t, "queue.add", existing(), map[string]any{"queueName": "Q"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)
}

func TestQueueAddDryRun(t *testing.T) {
	fc := absent()
	res := exec(t, "queue.add", fc, map[string]any{"queueName": "Q"}, true)
	assert.Equal(t, models.StatusDryRun, res.Status)
	assert.Equal(t, 0, fc.createdCount)
}

func TestQueueAddMissingName(t *testing.T) {
	res := exec(t, "queue.add", absent(), map[string]any{}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "Missing required arg: queue_name")
}

func TestQueueAddNameTooLong(t *testing.T) {
	res := exec(t, "queue.add", absent(), map[string]any{"queueName": strings.Repeat("x", 201)}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "200")
}

func TestQueueAddExistsError(t *testing.T) {
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return false, nil, stderrors.New("boom")
	}}
	res := exec(t, "queue.add", fc, map[string]any{"queueName": "Q"}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "Error checking queue")
}

func TestQueueAddCreateError(t *testing.T) {
	fc := &fakeClient{
		onExists: func(string) (bool, map[string]any, error) { return false, nil, nil },
		onCreate: func(string, map[string]any) (map[string]any, error) { return nil, stderrors.New("nope") },
	}
	res := exec(t, "queue.add", fc, map[string]any{"queueName": "Q"}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "Failed to create queue")
}

func TestQueueAddBadIntFails(t *testing.T) {
	fc := absent()
	res := exec(t, "queue.add", fc, map[string]any{"queueName": "Q", "maxTtl": "abc"}, false)
	assert.Equal(t, models.StatusFailed, res.Status)

	// Deliberate change from the previous implementation, which surfaced the raw
	// strconv error ("invalid literal for int(): 'abc'") without saying which
	// attribute it came from. An object can declare a dozen integers, so the
	// attribute name is the part the operator actually needs.
	assert.Contains(t, res.Message, "'max_ttl'")
	assert.Contains(t, res.Message, "must be an integer")
	// The value is now rejected before any request is issued.
	assert.Empty(t, fc.existsCalls)
}

func TestQueueDelete(t *testing.T) {
	fc := existing()
	res := exec(t, "queue.delete", fc, map[string]any{"queueName": "Q"}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, "msgVpns/test-vpn/queues/Q", fc.deletePath)

	res = exec(t, "queue.delete", absent(), map[string]any{"queueName": "Q"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)

	res = exec(t, "queue.delete", existing(), map[string]any{"queueName": "Q"}, true)
	assert.Equal(t, models.StatusDryRun, res.Status)
}

func TestQueueUpdate(t *testing.T) {
	// Absent -> FAILED (never creates on update).
	res := exec(t, "queue.update", absent(), map[string]any{"queueName": "Q", "egressEnabled": "false"}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "does not exist")

	// Exists, has fields -> update, name key dropped.
	fc := existing()
	res = exec(t, "queue.update", fc, map[string]any{"queueName": "Q", "egressEnabled": "false"}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.NotContains(t, fc.updateBody, "queueName")
	assert.Equal(t, false, fc.updateBody["egressEnabled"])

	// Exists, only the name -> nothing to update.
	res = exec(t, "queue.update", existing(), map[string]any{"queueName": "Q"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)
	assert.Contains(t, res.Message, "No fields to update")
}

// --- q_sub (POST-first idempotency) --------------------------------------

func TestSubscriptionAddPostsDirectly(t *testing.T) {
	fc := &fakeClient{} // default Create succeeds; Exists not consulted on live path
	res := exec(t, "q_sub.add", fc, map[string]any{"queueName": "Q", "subscriptionTopic": "SITEA/APP/>"}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, "msgVpns/test-vpn/queues/Q/subscriptions", fc.createPath)
	assert.Empty(t, fc.existsCalls) // no pre-check on the live path
	assert.Equal(t, "SITEA/APP/>", fc.createBody["subscriptionTopic"])
}

func TestSubscriptionAddAlreadyExists(t *testing.T) {
	fc := &fakeClient{onCreate: func(string, map[string]any) (map[string]any, error) {
		return nil, sempErr(semp.AlreadyExists)
	}}
	res := exec(t, "q_sub.add", fc, map[string]any{"queueName": "Q", "subscriptionTopic": "t"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)
}

func TestSubscriptionAddCreateError(t *testing.T) {
	fc := &fakeClient{onCreate: func(string, map[string]any) (map[string]any, error) {
		return nil, sempErr(3)
	}}
	res := exec(t, "q_sub.add", fc, map[string]any{"queueName": "Q", "subscriptionTopic": "t"}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "Failed to add subscription")
}

func TestSubscriptionAddDryRun(t *testing.T) {
	res := exec(t, "q_sub.add", existing(), map[string]any{"queueName": "Q", "subscriptionTopic": "t"}, true)
	assert.Equal(t, models.StatusSkipped, res.Status) // already exists

	res = exec(t, "q_sub.add", absent(), map[string]any{"queueName": "Q", "subscriptionTopic": "t"}, true)
	assert.Equal(t, models.StatusDryRun, res.Status)
}

func TestSubscriptionAddMissingArgs(t *testing.T) {
	res := exec(t, "q_sub.add", absent(), map[string]any{"queueName": "Q"}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "Missing required args")
}

func TestSubscriptionDelete(t *testing.T) {
	fc := existing()
	res := exec(t, "q_sub.delete", fc, map[string]any{"queueName": "Q", "subscriptionTopic": "t"}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "q_sub.delete", absent(), map[string]any{"queueName": "Q", "subscriptionTopic": "t"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)
}

// --- rdp ------------------------------------------------------------------

func TestRdpAddAndUpdate(t *testing.T) {
	fc := absent()
	res := exec(t, "rdp.add", fc, map[string]any{"restDeliveryPointName": "RDP", "enabled": "true"}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, "msgVpns/test-vpn/restDeliveryPoints", fc.createPath)
	assert.Equal(t, true, fc.createBody["enabled"])

	res = exec(t, "rdp.update", absent(), map[string]any{"restDeliveryPointName": "RDP", "enabled": "false"}, false)
	assert.Equal(t, models.StatusFailed, res.Status) // absent -> fails on update

	fc = existing()
	res = exec(t, "rdp.update", fc, map[string]any{"restDeliveryPointName": "RDP", "enabled": "false"}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.NotContains(t, fc.updateBody, "restDeliveryPointName")
}

func TestRdpDelete(t *testing.T) {
	res := exec(t, "rdp.delete", existing(), map[string]any{"restDeliveryPointName": "RDP"}, false)
	assert.Equal(t, models.StatusOK, res.Status)
}

// --- rdp_rc (drops path-only field, coerces ints/bools) ------------------

func TestRestConsumerAdd(t *testing.T) {
	fc := absent()
	res := exec(t, "rdp_rc.add", fc, map[string]any{
		"restDeliveryPointName": "RDP",
		"restConsumerName":      "RC",
		"remotePort":            "8080",
		"tlsEnabled":            "true",
	}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, "msgVpns/test-vpn/restDeliveryPoints/RDP/restConsumers", fc.createPath)
	assert.NotContains(t, fc.createBody, "restDeliveryPointName") // path-only
	assert.Equal(t, 8080, fc.createBody["remotePort"])
	assert.Equal(t, true, fc.createBody["tlsEnabled"])
}

func TestRestConsumerAddBadPort(t *testing.T) {
	res := exec(t, "rdp_rc.add", absent(), map[string]any{
		"restDeliveryPointName": "RDP", "restConsumerName": "RC", "remotePort": "x",
	}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
}

func TestRestConsumerNameTooLong(t *testing.T) {
	res := exec(t, "rdp_rc.add", absent(), map[string]any{
		"restDeliveryPointName": "RDP", "restConsumerName": strings.Repeat("x", 33),
	}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "32")
}

// --- rdp_qb (pre-check AND already-exists fallback) ----------------------

func TestQueueBindingAddPreCheck(t *testing.T) {
	res := exec(t, "rdp_qb.add", existing(), map[string]any{"restDeliveryPointName": "RDP", "queueBindingName": "Q"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)
}

func TestQueueBindingAddCreates(t *testing.T) {
	fc := absent()
	res := exec(t, "rdp_qb.add", fc, map[string]any{
		"restDeliveryPointName":                "RDP",
		"queueBindingName":                     "Q",
		"gatewayReplaceTargetAuthorityEnabled": "true",
	}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, "msgVpns/test-vpn/restDeliveryPoints/RDP/queueBindings", fc.createPath)
	assert.NotContains(t, fc.createBody, "restDeliveryPointName")
	assert.Equal(t, true, fc.createBody["gatewayReplaceTargetAuthorityEnabled"])
}

func TestQueueBindingAddAlreadyExistsFallback(t *testing.T) {
	fc := &fakeClient{
		onExists: func(string) (bool, map[string]any, error) { return false, nil, nil },
		onCreate: func(string, map[string]any) (map[string]any, error) { return nil, sempErr(semp.AlreadyExists) },
	}
	res := exec(t, "rdp_qb.add", fc, map[string]any{"restDeliveryPointName": "RDP", "queueBindingName": "Q"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)
}

// --- acl profile + exceptions --------------------------------------------

func TestAclProfileAddDelete(t *testing.T) {
	res := exec(t, "acl_profile.add", absent(), map[string]any{"aclProfileName": "P"}, false)
	assert.Equal(t, models.StatusOK, res.Status)

	res = exec(t, "acl_profile.add", existing(), map[string]any{"aclProfileName": "P"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)

	res = exec(t, "acl_profile.delete", absent(), map[string]any{"aclProfileName": "P"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)
}

func TestPublishExceptionAddCompositePath(t *testing.T) {
	fc := absent()
	res := exec(t, "acl_publish_exception.add", fc, map[string]any{
		"aclProfileName":        "P",
		"publishTopicException": "SITEA/>",
	}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, "msgVpns/test-vpn/aclProfiles/P/publishTopicExceptions", fc.createPath)
	// The existence check uses the "syntax,topic" composite segment.
	require.NotEmpty(t, fc.existsCalls)
	assert.Contains(t, fc.existsCalls[0], "smf,")
	assert.Equal(t, "smf", fc.createBody["publishTopicExceptionSyntax"])
}

func TestPublishExceptionMissingArgs(t *testing.T) {
	res := exec(t, "acl_publish_exception.add", absent(), map[string]any{"publishTopicException": "t"}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "acl_profile_name")

	res = exec(t, "acl_publish_exception.add", absent(), map[string]any{"aclProfileName": "P"}, false)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "publish_topic_exception")
}

func TestSubscribeExceptionAdd(t *testing.T) {
	fc := absent()
	res := exec(t, "acl_subscribe_exception.add", fc, map[string]any{
		"aclProfileName":          "P",
		"subscribeTopicException": "SITEA/>",
	}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, "msgVpns/test-vpn/aclProfiles/P/subscribeTopicExceptions", fc.createPath)
}

// --- client_profile + client_username ------------------------------------

func TestClientProfileAddCoercion(t *testing.T) {
	fc := absent()
	res := exec(t, "client_profile.add", fc, map[string]any{
		"clientProfileName":  "CP",
		"compressionEnabled": "true",
		"maxEgressFlowCount": "100",
	}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, true, fc.createBody["compressionEnabled"])
	assert.Equal(t, 100, fc.createBody["maxEgressFlowCount"])
}

func TestClientProfileUpdateNoFields(t *testing.T) {
	res := exec(t, "client_profile.update", existing(), map[string]any{"clientProfileName": "CP"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)
	assert.Contains(t, res.Message, "No fields to update")
}

func TestClientUsernameLifecycle(t *testing.T) {
	fc := absent()
	res := exec(t, "client_username.add", fc, map[string]any{"clientUsername": "svc", "enabled": "true"}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, true, fc.createBody["enabled"])

	res = exec(t, "client_username.update", absent(), map[string]any{"clientUsername": "svc", "enabled": "false"}, false)
	assert.Equal(t, models.StatusFailed, res.Status)

	res = exec(t, "client_username.delete", existing(), map[string]any{"clientUsername": "svc"}, false)
	assert.Equal(t, models.StatusOK, res.Status)
}
