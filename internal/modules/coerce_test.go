package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A rendered Jinja2 expression is always a string, even for a numeric or boolean
// attribute, so coercion is driven by the declared field kind rather than by the
// value's Go type. These cases cover both the templated (string) and the literal
// YAML forms of each kind.
func TestCoerceFieldScalars(t *testing.T) {
	cases := []struct {
		name  string
		field fieldDesc
		in    any
		want  any
	}{
		{"int from a rendered string", fieldDesc{Name: "maxTtl", Kind: kindInt}, "300", 300},
		{"int from a YAML integer", fieldDesc{Name: "maxTtl", Kind: kindInt}, 300, 300},
		{"negative int from a string", fieldDesc{Name: "maxRedeliveryCount", Kind: kindInt}, "-1", -1},
		{"zero int", fieldDesc{Name: "maxTtl", Kind: kindInt}, "0", 0},

		{"bool from a rendered string", fieldDesc{Name: "egressEnabled", Kind: kindBool}, "true", true},
		{"bool from a YAML boolean", fieldDesc{Name: "egressEnabled", Kind: kindBool}, false, false},

		{"string is left as supplied", fieldDesc{Name: "owner", Kind: kindString}, "svc-user", "svc-user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{tc.field.Name: tc.in}
			require.NoError(t, coerceField(tc.field, payload))
			assert.Equal(t, tc.want, payload[tc.field.Name])
		})
	}
}

func TestCoerceFieldIntErrorNamesTheAttribute(t *testing.T) {
	payload := map[string]any{"maxTtl": "not-a-number"}
	err := coerceField(fieldDesc{Name: "maxTtl", Kind: kindInt}, payload)

	require.Error(t, err)
	// The attribute name is the point: an object can declare a dozen integer
	// attributes, and an error that omits the name leaves the operator guessing.
	assert.Contains(t, err.Error(), "'maxTtl'")
	assert.Contains(t, err.Error(), "must be an integer")
}

// threshold mirrors the one nested shape SEMP uses, an event threshold group.
func threshold(name string) fieldDesc {
	return fieldDesc{
		Name: name,
		Kind: kindObject,
		Fields: []fieldDesc{
			{Name: "clearPercent", Kind: kindInt, Requires: []string{"setPercent"}, Conflicts: []string{"clearValue", "setValue"}},
			{Name: "clearValue", Kind: kindInt},
			{Name: "setPercent", Kind: kindInt, Requires: []string{"clearPercent"}},
			{Name: "setValue", Kind: kindInt},
		},
	}
}

func TestCoerceFieldNestedObject(t *testing.T) {
	payload := map[string]any{
		"eventBindCountThreshold": map[string]any{
			"clearPercent": "60",
			"setPercent":   80,
		},
	}
	require.NoError(t, coerceField(threshold("eventBindCountThreshold"), payload))

	nested, ok := payload["eventBindCountThreshold"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 60, nested["clearPercent"])
	assert.Equal(t, 80, nested["setPercent"])
	// Absent members are not invented.
	assert.NotContains(t, nested, "clearValue")
	assert.NotContains(t, nested, "setValue")
}

func TestCoerceFieldNestedObjectWrongShape(t *testing.T) {
	payload := map[string]any{"eventBindCountThreshold": "60"}
	err := coerceField(threshold("eventBindCountThreshold"), payload)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "'eventBindCountThreshold'")
	assert.Contains(t, err.Error(), "must be an object")
	// The message lists what the group accepts, so the fix is obvious.
	assert.Contains(t, err.Error(), "clearPercent, clearValue, setPercent, setValue")
}

func TestCoerceFieldNestedObjectInnerError(t *testing.T) {
	payload := map[string]any{
		"eventBindCountThreshold": map[string]any{"clearPercent": "sixty"},
	}
	err := coerceField(threshold("eventBindCountThreshold"), payload)

	require.Error(t, err)
	// Both the group and the offending member are named.
	assert.Contains(t, err.Error(), "'eventBindCountThreshold'")
	assert.Contains(t, err.Error(), "'clearPercent'")
}
