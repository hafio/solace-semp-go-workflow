package ir

import (
	"fmt"
	"strings"
)

// Registry prefixes are derived from the create operationId, then overridden by
// the alias table below. Only 24 of the 86 objects need an alias; the rest take
// the mechanical name so it is predictable from the SEMP object without anyone
// consulting a table.

// splitCamel splits a CamelCase identifier into its words. It is hand-written
// rather than a regexp because the rule needs lookahead -- the last capital of a
// run starts a new word only when a lowercase letter follows, so "ACLProfile"
// splits as ACL + Profile rather than AC + LProfile -- and Go's RE2 engine has no
// lookahead. Digits attach to the word they follow, so "Version10Enabled" splits
// as Version10 + Enabled.
func splitCamel(s string) []string {
	rs := []rune(s)
	var words []string
	var cur []rune

	isUpper := func(r rune) bool { return r >= 'A' && r <= 'Z' }
	isLower := func(r rune) bool { return r >= 'a' && r <= 'z' }

	for i, r := range rs {
		if isUpper(r) && len(cur) > 0 {
			prevUpper := isUpper(cur[len(cur)-1])
			nextLower := i+1 < len(rs) && isLower(rs[i+1])
			if !prevUpper || nextLower {
				words = append(words, string(cur))
				cur = nil
			}
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		words = append(words, string(cur))
	}
	return words
}

// DefaultPrefix derives a registry prefix from a create operationId: strip
// "create", split CamelCase, drop a leading Msg+Vpn because the VPN is implicit,
// lower-case and join with underscores. The Msg+Vpn guard leaves "createMsgVpn"
// itself alone, which would otherwise reduce to nothing.
func DefaultPrefix(operationID string) string {
	s := strings.TrimPrefix(operationID, "create")
	words := splitCamel(s)
	if len(words) > 2 && words[0] == "Msg" && words[1] == "Vpn" {
		words = words[2:]
	}
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_")
}

// prefixAliases overrides DefaultPrefix. Every entry is here for one of three
// stated reasons, and nothing else earns a place:
//
//  1. Backward compatibility. The ten prefixes carried over from the previous
//     implementation are load-bearing: existing templates and customer configs
//     name them. These are not negotiable.
//  2. A genuine collision. createProxy and createMsgVpnProxy both reduce to
//     "proxy". Aliasing only the broker-global one resolves it and leaves the
//     VPN-scoped object, the common case, with the natural name.
//  3. A mechanical name over 45 characters. Thirteen qualify; the tail is
//     shortened and the family root kept so siblings still sort together.
var prefixAliases = map[string]string{
	// 1. Backward compatibility.
	"createMsgVpnQueue":                             "queue",
	"createMsgVpnQueueSubscription":                 "q_sub",
	"createMsgVpnRestDeliveryPoint":                 "rdp",
	"createMsgVpnRestDeliveryPointRestConsumer":     "rdp_rc",
	"createMsgVpnRestDeliveryPointQueueBinding":     "rdp_qb",
	"createMsgVpnClientProfile":                     "client_profile",
	"createMsgVpnClientUsername":                    "client_username",
	"createMsgVpnAclProfile":                        "acl_profile",
	"createMsgVpnAclProfilePublishTopicException":   "acl_publish_exception",
	"createMsgVpnAclProfileSubscribeTopicException": "acl_subscribe_exception",

	// 2. Collision.
	"createProxy": "broker_proxy",

	// 3. Over the 45-character budget.
	"createMsgVpnDistributedCacheClusterGlobalCachingHomeClusterTopicPrefix": "distributed_cache_home_cluster_prefix",
	"createOauthProfileAccessLevelGroupMsgVpnAccessLevelException":           "oauth_profile_access_group_vpn_exception",
	"createMsgVpnAuthenticationOauthProfileResourceServerRequiredClaim":      "authentication_oauth_rs_claim",
	"createMsgVpnRestDeliveryPointQueueBindingProtectedRequestHeader":        "rdp_qb_protected_header",
	"createMsgVpnRestDeliveryPointRestConsumerTlsTrustedCommonName":          "rdp_rc_tls_cn",
	"createMsgVpnDistributedCacheClusterGlobalCachingHomeCluster":            "distributed_cache_home_cluster",
	"createOauthProfileDefaultMsgVpnAccessLevelException":                    "oauth_profile_default_vpn_exception",
	"createClientCertAuthorityOcspTlsTrustedCommonName":                      "client_cert_authority_ocsp_cn",
	"createMsgVpnAuthenticationOauthProfileClientRequiredClaim":              "authentication_oauth_client_claim",
	"createMsgVpnRestDeliveryPointRestConsumerOauthJwtClaim":                 "rdp_rc_jwt_claim",
	"createMsgVpnRestDeliveryPointQueueBindingRequestHeader":                 "rdp_qb_header",
	"createMsgVpnTelemetryProfileReceiverAclConnectException":                "telemetry_profile_receiver_acl_exception",
	"createDmrClusterCertMatchingRuleAttributeFilter":                        "dmr_cluster_cert_matching_rule_filter",
}

// maxPrefixLen is the budget above which a mechanical name earns an alias. Two
// aliased names land at 40 and 44 characters, which is accepted: shortening them
// further would mean inventing jargon.
const maxPrefixLen = 45

// Prefix resolves the registry prefix for a create operationId.
func Prefix(operationID string) string {
	if alias, ok := prefixAliases[operationID]; ok {
		return alias
	}
	return DefaultPrefix(operationID)
}

// ValidateAliases reports aliases that no longer match any operationId in the
// specification, which is how a Solace rename surfaces as a reviewed change
// rather than a silently ignored table entry.
func ValidateAliases(operationIDs map[string]bool) error {
	var stale []string
	for id := range prefixAliases {
		if !operationIDs[id] {
			stale = append(stale, id)
		}
	}
	if len(stale) == 0 {
		return nil
	}
	return fmt.Errorf("prefix aliases match no operationId in the specification (renamed or removed upstream): %s",
		strings.Join(stale, ", "))
}

// CheckPrefixes reports objects that collide on a prefix, and mechanical names
// that exceed the budget and so need an alias adding.
func CheckPrefixes(objects []Object) error {
	seen := map[string][]string{}
	var tooLong []string
	for _, o := range objects {
		seen[o.Prefix] = append(seen[o.Prefix], o.Definition)
		if _, aliased := prefixAliases[o.OperationID]; !aliased && len(o.Prefix) > maxPrefixLen {
			tooLong = append(tooLong, fmt.Sprintf("%s (%d chars, from %s)", o.Prefix, len(o.Prefix), o.OperationID))
		}
	}
	var problems []string
	for prefix, defs := range seen {
		if len(defs) > 1 {
			problems = append(problems, fmt.Sprintf("prefix %q claimed by %s", prefix, strings.Join(defs, " and ")))
		}
	}
	for _, t := range tooLong {
		problems = append(problems, "prefix over the "+fmt.Sprint(maxPrefixLen)+"-character budget, add an alias: "+t)
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("naming problems:\n  %s", strings.Join(problems, "\n  "))
}

// friendlyName turns a definition name into the wording used in result messages:
// CamelCase split into lower-case words, with the SEMP scope prefix dropped and
// known initialisms preserved.
func friendlyName(definition string, parent string) string {
	s := definition
	// Drop the ancestor's definition name, so MsgVpnRestDeliveryPointQueueBinding
	// under MsgVpnRestDeliveryPoint reads "queue binding" rather than repeating
	// the parent.
	if parent != "" && strings.HasPrefix(s, parent) {
		s = strings.TrimPrefix(s, parent)
	}
	// The VPN scope prefix is implicit for a VPN-scoped object, so it is dropped
	// -- but not from the message VPN's own name, which would leave nothing and
	// read as "msgvpn" in every result message.
	if s != "MsgVpn" {
		s = strings.TrimPrefix(s, "MsgVpn")
	}
	words := splitCamel(s)
	if len(words) == 0 {
		return strings.ToLower(definition)
	}
	out := make([]string, 0, len(words))
	for _, w := range words {
		if up := strings.ToUpper(w); initialisms[up] {
			out = append(out, up)
			continue
		}
		if noun, ok := properNouns[w]; ok {
			out = append(out, noun)
			continue
		}
		if expanded, ok := wordExpansions[w]; ok {
			out = append(out, expanded)
			continue
		}
		out = append(out, strings.ToLower(w))
	}
	return strings.Join(out, " ")
}

// initialisms stay upper-case in a friendly name, so an object reads "TLS
// trusted common name" rather than "tls trusted common name".
var initialisms = map[string]bool{
	"ACL": true, "AMQP": true, "API": true, "DMR": true, "HTTP": true,
	"JNDI": true, "JWT": true, "MQTT": true, "OCSP": true, "REST": true,
	"RDP": true, "SMF": true, "TLS": true, "URI": true, "URL": true,
	"VPN": true, "WWN": true,
}

// properNouns keep their conventional casing rather than being lower-cased.
var properNouns = map[string]string{
	"Kafka":    "Kafka",
	"Kerberos": "Kerberos",
	"Oauth":    "OAuth",
}

// wordExpansions spell out the abbreviations SEMP uses in definition names, so a
// result message reads "remote message VPN" rather than "remote msg VPN".
var wordExpansions = map[string]string{
	"Msg": "message",
}

// spelledOut are the initialisms pronounced letter by letter. That is what
// decides the indefinite article, and it cannot be derived from spelling: "an
// MQTT session" because M is said "em", but "a REST consumer" because REST is
// said as a word. Both are all-caps.
var spelledOut = map[string]bool{
	"ACL": true, "AMQP": true, "API": true, "DMR": true, "HTTP": true,
	"JNDI": true, "JWT": true, "MQTT": true, "OCSP": true, "RDP": true,
	"SMF": true, "TLS": true, "URI": true, "URL": true, "VPN": true, "WWN": true,
}

// vowelSoundLetters are the letters whose spoken name begins with a vowel sound,
// so "F" ("ef") takes "an" while "B" ("bee") takes "a".
const vowelSoundLetters = "AEFHILMNORSX"

// indefiniteArticle is the article for a friendly name. A spelled-out initialism
// is judged on its first letter's spoken name; anything else on its first letter.
// The legacy overlay can still pin a value, which is how the ten carried-over
// families keep their exact historical wording.
func indefiniteArticle(friendly string) string {
	if friendly == "" {
		return "a"
	}
	first := friendly
	if i := strings.Index(friendly, " "); i > 0 {
		first = friendly[:i]
	}
	if spelledOut[first] {
		if strings.ContainsRune(vowelSoundLetters, rune(first[0])) {
			return "an"
		}
		return "a"
	}
	switch strings.ToLower(first)[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return "an"
	}
	return "a"
}
