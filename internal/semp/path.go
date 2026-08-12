package semp

import "strings"

// Seg is one hop of a SEMP v2 config path: a literal collection name plus the
// identifying key values that select an item within it. Parts is empty for a
// bare collection hop, which is the POST target. A hop with two or more Parts
// renders as a single comma-joined URL segment, the form SEMP uses for
// composite keys such as bridges/{bridgeName},{bridgeVirtualRouter}.
type Seg struct {
	Literal string
	Parts   []string
}

// VPNSeg is the leading hop of every VPN-scoped path. Broker-global objects
// (dmrClusters, oauthProfiles, certAuthorities, proxies, virtualHostnames,
// msgVpns itself) omit it and address the config root directly.
func VPNSeg(msgVPN string) Seg {
	return Seg{Literal: "msgVpns", Parts: []string{msgVPN}}
}

// Path joins segments into a path relative to the SEMP v2 config root. Key
// parts are percent-encoded individually before a hop's parts are comma-joined,
// so a key value containing a comma, slash or space cannot alter the path
// structure. Literals come from the specification and are used verbatim.
func Path(segs ...Seg) string {
	out := make([]string, 0, len(segs)*2)
	for _, s := range segs {
		out = append(out, s.Literal)
		if len(s.Parts) == 0 {
			continue
		}
		enc := make([]string, len(s.Parts))
		for i, p := range s.Parts {
			enc[i] = Enc(p)
		}
		out = append(out, strings.Join(enc, ","))
	}
	return strings.Join(out, "/")
}

// ResolveVPN picks the message VPN an action targets: the value named on the
// action when set, otherwise the connection default.
func ResolveVPN(explicit, defaulted string) string {
	if explicit != "" {
		return explicit
	}
	return defaulted
}
