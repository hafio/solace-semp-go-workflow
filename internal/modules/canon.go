package modules

import (
	"fmt"
	"sort"
	"strings"
)

// Argument keys are accepted in two spellings. The canonical one is snake_case,
// matching the Solace Terraform provider, and is what the docs publish. The SEMP
// spelling is also accepted, unchanged and undocumented, because templates and
// customer configs written against the previous implementation use it.
//
// Everything downstream of normalizeArgs works in the SEMP spelling alone, so the
// two conventions converge before any payload, path or diff is built and neither
// gets a second code path.

// canonToWire maps every canonical argument name this object accepts to its SEMP
// equivalent: body attributes, the object's own key, every ancestor key, and the
// msgVpnName override. Names spelled identically in both conventions ("owner",
// "permission") are absent, since renaming them would be a no-op.
func (d objectDesc) canonToWire() map[string]string {
	pairs := fieldCanonToWire(d.Fields)
	for _, p := range d.Parents {
		for _, k := range p.Keys {
			if c := k.canonical(); c != k.Name {
				pairs[c] = k.Name
			}
		}
	}
	for _, k := range d.OwnKey {
		if c := k.canonical(); c != k.Name {
			pairs[c] = k.Name
		}
	}
	if d.Scope == scopeVPN {
		// Not a declared field: resolve() reads the VPN override straight from the
		// args map, so its canonical spelling has to be mapped here too.
		pairs["msg_vpn_name"] = "msgVpnName"
	}
	return pairs
}

// fieldCanonToWire maps one level of attributes. A nested group's own fields are
// mapped separately, when that group's value is walked.
func fieldCanonToWire(fields []fieldDesc) map[string]string {
	pairs := make(map[string]string, len(fields))
	for _, f := range fields {
		if c := f.canonical(); c != f.Name {
			pairs[c] = f.Name
		}
	}
	return pairs
}

// normalizeArgs returns args with every canonical key rewritten to its SEMP
// spelling, recursing one level into a nested attribute group to match how
// checkValue addresses them.
//
// Keys it does not recognise pass through untouched, so unknownArgs still reports
// them exactly as the caller typed them rather than as something rewritten.
func (d objectDesc) normalizeArgs(args map[string]any) (map[string]any, error) {
	var dup []string
	out := renameKeys(args, d.canonToWire(), &dup)

	for _, f := range d.Fields {
		if f.Kind != kindObject {
			continue
		}
		nested, ok := out[f.Name].(map[string]any)
		if !ok {
			// A non-object value is a type error, which checkValue reports with the
			// field's accepted shape. Nothing to rename here.
			continue
		}
		out[f.Name] = renameKeys(nested, fieldCanonToWire(f.Fields), &dup)
	}

	if len(dup) > 0 {
		sort.Strings(dup)
		return nil, fmt.Errorf(
			"the same attribute was supplied under both names (%s); use the first of each pair",
			strings.Join(dup, "; "))
	}
	return out, nil
}

// renameKeys copies m with each canonical key replaced by its SEMP equivalent.
//
// Supplying both spellings of one attribute is recorded rather than resolved: Go
// randomises map iteration order, so letting one overwrite the other would make
// which value reached the broker depend on chance. Two spellings of one attribute
// is a mistake worth naming, not a precedence rule worth inventing.
func renameKeys(m map[string]any, canonToWire map[string]string, dup *[]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	for canon, wire := range canonToWire {
		v, ok := m[canon]
		if !ok {
			continue
		}
		if _, clash := m[wire]; clash {
			*dup = append(*dup, canon+" and "+wire)
			continue
		}
		delete(out, canon)
		out[wire] = v
	}
	return out
}
