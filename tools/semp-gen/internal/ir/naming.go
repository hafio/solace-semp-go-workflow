package ir

import (
	"fmt"
	"sort"
	"strings"
)

// Registry prefixes and canonical attribute names are both the snake_case form of
// their SEMP identifier, which is what the Solace Terraform provider publishes.
// Every one of the 86 prefixes is mechanical -- there is no alias table.

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

// ToSnake converts a SEMP attribute or path-parameter name to the snake_case
// spelling the Solace Terraform provider publishes: split on CamelCase word
// boundaries, lower-case, join with underscores. Digits stay attached to the word
// they follow, so tlsBlockVersion11Enabled becomes tls_block_version11_enabled --
// which is byte-for-byte what the provider documents.
func ToSnake(name string) string {
	words := splitCamel(name)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_")
}

// canonOf is ToSnake, but empty when the name is already snake_case -- an
// all-lower-case name such as "owner" or "permission" needs no second spelling.
// Emitting the field only where it differs keeps the generated descriptors free
// of ~27 redundant lines and makes a real rename visible in a diff.
func canonOf(name string) string {
	if s := ToSnake(name); s != name {
		return s
	}
	return ""
}

// FromSnake is ToSnake's inverse. Nothing converts in this direction at runtime;
// it exists so CheckSnakeRoundTrip can prove ToSnake loses no information.
func FromSnake(name string) string {
	var b strings.Builder
	for i, p := range strings.Split(name, "_") {
		if p == "" {
			continue
		}
		if i == 0 {
			b.WriteString(p)
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

// CheckSnakeRoundTrip reports names whose snake_case spelling cannot be converted
// back, which means ToSnake lost information.
//
// Every attribute and key in both specifications round-trips today, because SEMP
// writes Tls and Http rather than TLS and HTTP: no property name anywhere in
// either file contains two adjacent capitals. That is an observed property of the
// specification, not a promise from Solace, and the whole canonical-naming scheme
// rests on it. So it is asserted at generation time -- a spec bump that
// introduces "TLSEnabled" (or an underscore) fails the build here rather than
// silently publishing a canonical name that maps back to something else.
func CheckSnakeRoundTrip(objects []Object) error {
	var bad []string
	seen := map[string]bool{}

	report := func(owner, name string) {
		if name == "" || seen[owner+"."+name] {
			return
		}
		seen[owner+"."+name] = true
		if back := FromSnake(ToSnake(name)); back != name {
			bad = append(bad, fmt.Sprintf("%s.%s -> %s -> %s", owner, name, ToSnake(name), back))
		}
	}
	var walk func(owner string, attrs []Attribute)
	walk = func(owner string, attrs []Attribute) {
		for _, a := range attrs {
			report(owner, a.Name)
			walk(owner, a.Fields)
		}
	}
	for _, o := range objects {
		for _, k := range o.OwnKey {
			report(o.Definition, k.Name)
		}
		for _, p := range o.Parents {
			for _, k := range p.Keys {
				report(o.Definition, k.Name)
			}
		}
		walk(o.Definition, o.Attributes)
	}
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf("names do not survive the snake_case round trip, so the canonical spelling would be ambiguous (a name with two adjacent capitals or an underscore does this):\n  %s",
		strings.Join(bad, "\n  "))
}

// Prefix is the registry prefix for an object: the snake_case form of its SEMP
// definition name, which is exactly what the Solace Terraform provider calls the
// equivalent resource once its own "solacebroker_" provider prefix is dropped.
// MsgVpnQueue becomes msg_vpn_queue, matching the provider's msg_vpn_queue.
//
// There is no alias table. All 86 prefixes are mechanical, so a name is
// predictable from the SEMP object without consulting anything, and the two
// reasons the old table existed are both gone:
//
//   - The createProxy / createMsgVpnProxy collision disappears, because the VPN
//     scope is no longer stripped: Proxy is proxy, MsgVpnProxy is msg_vpn_proxy.
//   - There is no length budget to keep names short. Aligning with Terraform
//     means accepting Terraform's lengths, up to 73 characters.
func Prefix(definition string) string {
	return ToSnake(definition)
}

// CheckPrefixes reports objects that collide on a prefix. Two definitions cannot
// produce the same snake_case name unless SEMP itself introduces an ambiguity, so
// this is defence in depth rather than a routine occurrence -- but a collision
// would silently shadow one object's actions with another's, so it stops the run.
func CheckPrefixes(objects []Object) error {
	seen := map[string][]string{}
	for _, o := range objects {
		seen[o.Prefix] = append(seen[o.Prefix], o.Definition)
	}
	var problems []string
	for prefix, defs := range seen {
		if len(defs) > 1 {
			problems = append(problems, fmt.Sprintf("prefix %q claimed by %s", prefix, strings.Join(defs, " and ")))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
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
