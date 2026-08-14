package modules

import (
	"fmt"
	"sort"
	"strings"

	"solace-semp-workflow/internal/semp"
)

// Argument validation happens locally, before anything reaches the broker. Until
// the descriptors were generated this was not possible: only a subset of each
// object's attributes was declared, so an unrecognised argument might have been
// a perfectly good attribute the tool simply did not know about. Now that every
// object declares its full specification surface, an argument that is not a key
// and not a declared attribute is a mistake, and saying so locally beats a round
// trip and a broker error naming a field the author cannot find in the docs.

// validate checks resolved args against this object's schema for one verb,
// collecting every problem rather than stopping at the first so a single run
// lists everything to fix.
func (d objectDesc) validate(args map[string]any, mode paramMode) error {
	byName := make(map[string]fieldDesc, len(d.Fields))
	for _, f := range d.Fields {
		byName[f.Name] = f
	}

	var problems []string
	problems = append(problems, d.unknownArgs(args, byName, mode)...)

	for _, name := range sortedArgNames(args) {
		f, declared := byName[name]
		if !declared {
			continue // already reported as unknown
		}
		if mode == paramUpdate && f.Const {
			problems = append(problems, fmt.Sprintf(
				"'%s' can only be set when the object is created, so it cannot appear in an update", name))
			continue
		}
		problems = append(problems, checkValue(f, args[name], "")...)
	}

	if mode == paramAdd {
		if missing := d.requiredMissing(args); missing != "" {
			problems = append(problems, fmt.Sprintf("missing required argument '%s'", missing))
		}
	}
	problems = append(problems, checkCombinations(d.Fields, args, "")...)

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "; "))
}

// unknownArgs reports args that are neither a path key nor a declared attribute.
func (d objectDesc) unknownArgs(args map[string]any, byName map[string]fieldDesc, mode paramMode) []string {
	allowed := d.pathOnlyNames()
	for _, k := range d.OwnKey {
		allowed[k.Name] = true
	}
	// A delete carries no body, so an attribute name is as wrong as a typo.
	if mode != paramDelete {
		for name := range byName {
			allowed[name] = true
		}
	}

	var unknown []string
	for name := range args {
		if !allowed[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)

	noun := "argument"
	if len(unknown) > 1 {
		noun = "arguments"
	}
	hint := fmt.Sprintf("see docs/modules/%s.md for the accepted attributes", d.Prefix)
	if mode == paramDelete {
		hint = "a delete accepts only the keys that identify the object"
	}
	return []string{fmt.Sprintf("unknown %s: %s (%s)", noun, strings.Join(unknown, ", "), hint)}
}

// checkValue validates one supplied value against its attribute, recursing one
// level into a nested group. prefix names the enclosing group, if any.
func checkValue(f fieldDesc, v any, prefix string) []string {
	name := prefix + f.canonical()
	var problems []string

	switch f.Kind {
	case kindInt:
		if _, err := semp.CoerceInt(v); err != nil {
			problems = append(problems, fmt.Sprintf("'%s' must be an integer, got '%s'", name, semp.Stringify(v)))
		}
	case kindObject:
		nested, ok := v.(map[string]any)
		if !ok {
			return append(problems, fmt.Sprintf("'%s' must be an object with fields: %s", name, fieldNames(f.Fields)))
		}
		sub := make(map[string]fieldDesc, len(f.Fields))
		for _, s := range f.Fields {
			sub[s.Name] = s
		}
		var unknown []string
		for k := range nested {
			if _, ok := sub[k]; !ok {
				unknown = append(unknown, k)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			problems = append(problems, fmt.Sprintf("'%s' has unknown field(s): %s (accepts: %s)",
				name, strings.Join(unknown, ", "), fieldNames(f.Fields)))
		}
		for _, k := range sortedArgNames(nested) {
			if s, ok := sub[k]; ok {
				problems = append(problems, checkValue(s, nested[k], name+".")...)
			}
		}
	case kindString:
		// The same attribute can be a path key on one object and a body attribute
		// on another -- aclProfileName identifies an ACL profile but is an
		// attribute of a client username. Length is checked in both positions, so
		// an over-long value fails locally either way. CheckNameLength returns
		// nothing for an attribute with no curated limit, so this only applies
		// where a limit is actually known.
		if msg := semp.CheckNameLength(f.Name, name, semp.Stringify(v)); msg != "" {
			problems = append(problems, msg)
		}
	case kindBool:
		// CoerceBool accepts anything.
	}

	if len(f.Enum) > 0 && f.Kind != kindObject {
		got := semp.Stringify(v)
		if !contains(f.Enum, got) {
			problems = append(problems, fmt.Sprintf("'%s' must be one of %s, got '%s'",
				name, strings.Join(quoteAll(f.Enum), ", "), got))
		}
	}
	return problems
}

// checkCombinations enforces the specification's attribute co-dependency rules:
// an attribute that Requires another cannot be supplied alone, and one that
// Conflicts with another cannot be supplied alongside it. The broker enforces
// these too, but its rejection costs a round trip and names the constraint less
// clearly than the attribute that broke it.
func checkCombinations(fields []fieldDesc, args map[string]any, prefix string) []string {
	supplied := func(name string) bool {
		_, ok := args[name]
		return ok
	}

	// Requires and Conflicts name SEMP attributes, but the message has to quote
	// the canonical spelling. The lookup is built from THIS level's fields, not a
	// shared one: a nested group's peer names live only in its own Fields, so
	// reusing the top-level map would silently miss them.
	canonOf := make(map[string]string, len(fields))
	for _, f := range fields {
		canonOf[f.Name] = f.canonical()
	}
	display := func(name string) string {
		if c, ok := canonOf[name]; ok {
			return c
		}
		return name
	}

	var problems []string
	for _, f := range fields {
		if f.Kind == kindObject {
			if nested, ok := args[f.Name].(map[string]any); ok {
				problems = append(problems, checkCombinations(f.Fields, nested, prefix+f.canonical()+".")...)
			}
			continue
		}
		if !supplied(f.Name) {
			continue
		}
		for _, req := range f.Requires {
			if !supplied(req) {
				problems = append(problems, fmt.Sprintf("'%s%s' requires '%s%s' to be set as well",
					prefix, f.canonical(), prefix, display(req)))
			}
		}
		for _, con := range f.Conflicts {
			if supplied(con) {
				problems = append(problems, fmt.Sprintf("'%s%s' cannot be combined with '%s%s'",
					prefix, f.canonical(), prefix, display(con)))
			}
		}
	}
	return problems
}

func sortedArgNames(args map[string]any) []string {
	out := make([]string, 0, len(args))
	for k := range args {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func quoteAll(in []string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = "'" + v + "'"
	}
	return out
}
