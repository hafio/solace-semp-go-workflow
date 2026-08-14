package ir

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitCamel(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"MsgVpnQueue", []string{"Msg", "Vpn", "Queue"}},
		{"MsgVpnAclProfilePublishTopicException",
			[]string{"Msg", "Vpn", "Acl", "Profile", "Publish", "Topic", "Exception"}},
		{"DmrCluster", []string{"Dmr", "Cluster"}},
		// The last capital of a run starts a new word only when a lowercase
		// follows, which is the rule that needed lookahead a regexp cannot do.
		{"ACLProfile", []string{"ACL", "Profile"}},
		{"RDP", []string{"RDP"}},
		{"TLSVersion", []string{"TLS", "Version"}},
		// Digits attach to the word they follow.
		{"tlsBlockVersion10Enabled", []string{"tls", "Block", "Version10", "Enabled"}},
		{"lowercase", []string{"lowercase"}},
		{"", nil},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, splitCamel(tc.in))
		})
	}
}

// The expected values are the spelling the Solace Terraform provider publishes,
// so this pins the two conventions together rather than merely pinning our own
// output. Each was read off the provider's own resource documentation.
func TestToSnakeMatchesTheTerraformSpelling(t *testing.T) {
	cases := map[string]string{
		"egressEnabled":    "egress_enabled",
		"maxMsgSpoolUsage": "max_msg_spool_usage",
		"queueName":        "queue_name",
		"msgVpnName":       "msg_vpn_name",
		"deadMsgQueue":     "dead_msg_queue",
		"rejectMsgToSenderOnDiscardBehavior": "reject_msg_to_sender_on_discard_behavior",
		"maxDeliveredUnackedMsgsPerFlow":     "max_delivered_unacked_msgs_per_flow",
		// Composite key parts.
		"publishTopicException":       "publish_topic_exception",
		"publishTopicExceptionSyntax": "publish_topic_exception_syntax",
		// A nested event-threshold field.
		"clearPercent": "clear_percent",
		// A digit stays attached to the word it follows: the provider publishes
		// tls_block_version11_enabled, not tls_block_version_11_enabled.
		"tlsBlockVersion11Enabled": "tls_block_version11_enabled",
		// Already single-word: unchanged.
		"owner":      "owner",
		"permission": "permission",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, ToSnake(in))
		})
	}
}

// canonOf leaves an already-snake_case name empty so the generated descriptors
// carry the field only where it says something.
func TestCanonOfIsEmptyWhenTheNameIsUnchanged(t *testing.T) {
	assert.Equal(t, "", canonOf("owner"))
	assert.Equal(t, "", canonOf("permission"))
	assert.Equal(t, "egress_enabled", canonOf("egressEnabled"))
}

func TestCheckSnakeRoundTripAcceptsSpecShapedNames(t *testing.T) {
	assert.NoError(t, CheckSnakeRoundTrip([]Object{{
		Definition: "MsgVpnQueue",
		OwnKey:     []Key{{Name: "queueName"}},
		Parents:    []ParentHop{{Collection: "aclProfiles", Keys: []Key{{Name: "aclProfileName"}}}},
		Attributes: []Attribute{
			{Name: "egressEnabled"},
			{Name: "owner"},
			{Name: "eventBindCountThreshold", Fields: []Attribute{{Name: "clearPercent"}}},
		},
	}}))
}

// Two adjacent capitals make the canonical spelling ambiguous: "TLSEnabled"
// snake-cases to "tls_enabled", which converts back to "tlsEnabled" -- a
// different attribute. Publishing that canonical name would silently address the
// wrong thing, so generation has to stop.
func TestCheckSnakeRoundTripReportsAnAmbiguousName(t *testing.T) {
	err := CheckSnakeRoundTrip([]Object{{
		Definition: "Broker",
		Attributes: []Attribute{{Name: "TLSEnabled"}},
	}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Broker.TLSEnabled")
	assert.Contains(t, err.Error(), "tls_enabled")
}

// The walk recurses, so a nested event-threshold field cannot smuggle one past.
func TestCheckSnakeRoundTripWalksNestedFields(t *testing.T) {
	err := CheckSnakeRoundTrip([]Object{{
		Definition: "Broker",
		Attributes: []Attribute{
			{Name: "eventThreshold", Fields: []Attribute{{Name: "setJSONValue"}}},
		},
	}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "setJSONValue")
}

// A key is checked as well as a body attribute: an identifying name that does not
// round-trip breaks the same way.
func TestCheckSnakeRoundTripChecksKeys(t *testing.T) {
	err := CheckSnakeRoundTrip([]Object{{
		Definition: "X",
		OwnKey:     []Key{{Name: "myURLName"}},
	}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "myURLName")
}

// The expected values are the Solace Terraform provider's own resource names,
// minus its "solacebroker_" provider prefix. Each was read off the provider's
// published resource list, so this pins the two naming schemes together.
func TestPrefixMatchesTheTerraformResourceName(t *testing.T) {
	cases := map[string]string{
		"MsgVpnQueue":                         "msg_vpn_queue",
		"MsgVpnQueueSubscription":             "msg_vpn_queue_subscription",
		"MsgVpnRestDeliveryPointRestConsumer": "msg_vpn_rest_delivery_point_rest_consumer",
		"MsgVpnAclProfilePublishTopicException": "msg_vpn_acl_profile_publish_topic_exception",
		"DmrCluster":                            "dmr_cluster",
		"VirtualHostname":                       "virtual_hostname",
		"OauthProfile":                          "oauth_profile",
		"MsgVpn":                                "msg_vpn",
		"Broker":                                "broker",
		// The pair that used to collide. Stripping the VPN scope reduced both to
		// "proxy" and forced an alias; keeping it makes them distinct mechanically.
		"Proxy":       "proxy",
		"MsgVpnProxy": "msg_vpn_proxy",
	}
	for def, want := range cases {
		t.Run(def, func(t *testing.T) {
			assert.Equal(t, want, Prefix(def))
		})
	}
}

// The collision guard is defence in depth now that names are mechanical, but a
// collision would silently shadow one object's actions with another's, so it must
// still stop the run.
func TestCheckPrefixesReportsACollision(t *testing.T) {
	err := CheckPrefixes([]Object{
		{Definition: "Thing", Prefix: "thing"},
		{Definition: "OtherThing", Prefix: "thing"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `prefix "thing" claimed by`)
	assert.Contains(t, err.Error(), "OtherThing")
}

func TestCheckPrefixesAcceptsDistinctNames(t *testing.T) {
	assert.NoError(t, CheckPrefixes([]Object{
		{Definition: "Proxy", Prefix: "proxy"},
		{Definition: "MsgVpnProxy", Prefix: "msg_vpn_proxy"},
	}))
}

func TestFriendlyNamePreservesInitialisms(t *testing.T) {
	cases := []struct {
		definition string
		parent     string
		want       string
	}{
		{"MsgVpnQueue", "", "queue"},
		{"MsgVpnAclProfile", "", "ACL profile"},
		{"MsgVpnBridgeTlsTrustedCommonName", "", "bridge TLS trusted common name"},
		// A child drops its parent's definition prefix so it does not repeat it.
		{"MsgVpnRestDeliveryPointQueueBinding", "MsgVpnRestDeliveryPoint", "queue binding"},
		{"DmrCluster", "", "DMR cluster"},

		// The message VPN's own name must survive the VPN-prefix strip. Removing
		// it unconditionally leaves nothing, and the fallback lower-cases the whole
		// definition, so every message read "Msgvpn 'x' created".
		{"MsgVpn", "", "message VPN"},
		// Msg is spelled out rather than left abbreviated.
		{"MsgVpnBridgeRemoteMsgVpn", "MsgVpnBridge", "remote message VPN"},
		{"OauthProfileDefaultMsgVpnAccessLevelException", "OauthProfile",
			"default message VPN access level exception"},
		// Proper nouns keep their conventional casing.
		{"MsgVpnKafkaSender", "", "Kafka sender"},
		{"MsgVpnAuthenticationKerberosRealm", "", "authentication Kerberos realm"},
		{"MsgVpnAuthenticationOauthProfile", "", "authentication OAuth profile"},
	}
	for _, tc := range cases {
		t.Run(tc.definition, func(t *testing.T) {
			assert.Equal(t, tc.want, friendlyName(tc.definition, tc.parent))
		})
	}
}

func TestIndefiniteArticle(t *testing.T) {
	cases := map[string]string{
		"queue":         "a",
		"exception":     "an",
		"instance":      "an",
		"message VPN":   "a",
		"":              "a",
		// A spelled-out initialism is judged on its first letter's spoken name:
		// "em-cue-tee-tee" takes "an", "dee-em-are" takes "a".
		"MQTT session":              "an",
		"ACL profile":               "an",
		"RDP":                       "an",
		"OCSP TLS trusted CN":       "an",
		"SMF something":             "an",
		"DMR cluster":               "a",
		"JNDI queue":                "a",
		"TLS trusted common name":   "a",
		// REST is said as a word, not spelled out, so it takes "a" despite
		// starting with the same letter as RDP. No spelling rule separates these.
		"REST consumer": "a",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, indefiniteArticle(in))
		})
	}
}
