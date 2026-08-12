package ir

import (
	"sort"
	"strings"
)

// The appliance specification is not a subset of the software one. For
// 10.26.0.8755 the software spec adds ten objects (Kafka senders and receivers
// with their bindings, Kerberos realms) while the appliance spec adds ten
// attributes and one wider enum. Generating from either alone would silently drop
// configuration a customer can legitimately set, so both are unioned and anything
// platform-specific is tagged and noted in its description.

// union merges the software and appliance object sets by definition name.
func union(software, appliance []Object) []Object {
	byDef := map[string]Object{}
	order := []string{}
	for _, o := range software {
		o.Platform = PlatformSoftwareOnly
		byDef[o.Definition] = o
		order = append(order, o.Definition)
	}
	for _, ap := range appliance {
		sw, both := byDef[ap.Definition]
		if !both {
			ap.Platform = PlatformApplianceOnly
			byDef[ap.Definition] = ap
			order = append(order, ap.Definition)
			continue
		}
		sw.Platform = PlatformBoth
		sw.Attributes = unionAttributes(sw.Attributes, ap.Attributes)
		byDef[ap.Definition] = sw
	}

	out := make([]Object, 0, len(byDef))
	for _, def := range order {
		out = append(out, byDef[def])
	}
	return out
}

// unionAttributes merges two platforms' attribute lists for the same object.
func unionAttributes(software, appliance []Attribute) []Attribute {
	idx := map[string]int{}
	out := make([]Attribute, 0, len(software)+4)
	for _, a := range software {
		a.Platform = PlatformSoftwareOnly
		idx[a.Name] = len(out)
		out = append(out, a)
	}
	for _, ap := range appliance {
		i, both := idx[ap.Name]
		if !both {
			ap.Platform = PlatformApplianceOnly
			ap.Desc = withNote(ap.Desc, PlatformApplianceOnly)
			out = append(out, ap)
			continue
		}
		merged := out[i]
		merged.Platform = PlatformBoth
		merged.Enum = unionEnum(merged.Enum, ap.Enum)
		if len(merged.Enum) > len(out[i].Enum) {
			// One platform accepts values the other does not, so the wider set is
			// offered and the restriction stated rather than one spec's view winning.
			merged.Desc = strings.TrimSpace(merged.Desc +
				" Some values are accepted on one platform only; see the SEMP documentation for your broker.")
		}
		merged.Fields = unionAttributes(out[i].Fields, ap.Fields)
		out[i] = merged
	}

	// Tag the attributes only the software spec had, now that both are seen.
	for i := range out {
		if out[i].Platform == PlatformSoftwareOnly {
			out[i].Desc = withNote(out[i].Desc, PlatformSoftwareOnly)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func withNote(desc string, p Platform) string {
	note := p.Note()
	if note == "" {
		return desc
	}
	return strings.TrimSpace(strings.TrimSpace(desc) + " " + note)
}

// unionEnum returns the union of two enum value sets, preserving the order of the
// longer one so the common case keeps the specification's ordering.
func unionEnum(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	base, extra := a, b
	if len(b) > len(a) {
		base, extra = b, a
	}
	seen := map[string]bool{}
	for _, v := range base {
		seen[v] = true
	}
	out := append([]string{}, base...)
	for _, v := range extra {
		if !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	return out
}
