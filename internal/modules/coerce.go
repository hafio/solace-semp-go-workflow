package modules

import (
	"fmt"
	"strings"

	"solace-semp-workflow/internal/semp"
)

// Workflow args arrive from YAML or from a rendered Jinja2 expression, and a
// rendered expression is always a string even when the attribute is numeric or
// boolean. Coercion is therefore driven by the declared field kind rather than
// by the value's Go type, and the error names the attribute so an operator can
// see which one was wrong.

// coerceField coerces payload[f.Name] to f's declared type in place. The caller
// has already established that the key is present.
func coerceField(f fieldDesc, payload map[string]any) error {
	v := payload[f.Name]

	switch f.Kind {
	case kindInt:
		n, err := semp.CoerceInt(v)
		if err != nil {
			return fmt.Errorf("'%s' must be an integer: %w", f.canonical(), err)
		}
		payload[f.Name] = n

	case kindBool:
		payload[f.Name] = semp.CoerceBool(v)

	case kindObject:
		nested, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("'%s' must be an object with fields: %s", f.canonical(), fieldNames(f.Fields))
		}
		for _, sub := range f.Fields {
			if _, present := nested[sub.Name]; !present {
				continue
			}
			if err := coerceField(sub, nested); err != nil {
				return fmt.Errorf("'%s': %w", f.canonical(), err)
			}
		}
		payload[f.Name] = nested

	case kindString:
		// Left as supplied: CleanPayload has already dropped blanks, and the
		// broker accepts the JSON-encoded value as-is.
	}
	return nil
}

// fieldNames lists a nested group's attribute names for an error message, in the
// canonical spelling an operator would write them.
func fieldNames(fields []fieldDesc) string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.canonical()
	}
	return strings.Join(names, ", ")
}
