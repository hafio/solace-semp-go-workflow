package ir

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestDefaultPrefix(t *testing.T) {
	cases := map[string]string{
		"createMsgVpnQueue":                         "queue",
		"createMsgVpnQueueSubscription":             "queue_subscription",
		"createMsgVpnRestDeliveryPointRestConsumer": "rest_delivery_point_rest_consumer",
		"createDmrCluster":                          "dmr_cluster",
		"createVirtualHostname":                     "virtual_hostname",
		"createOauthProfile":                        "oauth_profile",
		// The Msg+Vpn guard must not empty the message VPN's own name.
		"createMsgVpn": "msg_vpn",
	}
	for op, want := range cases {
		t.Run(op, func(t *testing.T) {
			assert.Equal(t, want, DefaultPrefix(op))
		})
	}
}

// The alias table is hand-maintained, so a copy-paste that points two
// operationIds at the same prefix must fail here rather than at generation time
// with a confusing collision error.
func TestPrefixAliasesAreUnique(t *testing.T) {
	seen := map[string]string{}
	for op, prefix := range prefixAliases {
		if prev, dup := seen[prefix]; dup {
			t.Errorf("prefix %q is claimed by both %s and %s", prefix, prev, op)
		}
		seen[prefix] = op
	}
}

// An alias only earns its place for a stated reason, and length is the one the
// generator can check: an alias longer than the mechanical name it replaces is a
// mistake.
func TestLengthAliasesAreShorterThanTheMechanicalName(t *testing.T) {
	for op, alias := range prefixAliases {
		mech := DefaultPrefix(op)
		if len(mech) <= maxPrefixLen {
			continue // a backward-compatibility or collision alias, not a length one
		}
		assert.Lessf(t, len(alias), len(mech),
			"alias %q for %s is not shorter than the mechanical %q", alias, op, mech)
		assert.LessOrEqualf(t, len(alias), maxPrefixLen,
			"alias %q for %s is still over the budget", alias, op)
	}
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
