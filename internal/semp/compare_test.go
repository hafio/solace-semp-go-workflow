package semp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A GET response is decoded from JSON, so every number arrives as float64 no
// matter how the attribute is declared, while a desired value has been coerced
// to a Go int or bool. These cases pin that asymmetry down: an update must not
// re-send an attribute merely because 8 and 8.0 have different Go types.
func TestAttrEqual(t *testing.T) {
	cases := []struct {
		name    string
		current any
		desired any
		want    bool
	}{
		{"nil desired matches absent current", nil, nil, true},
		{"nil desired does not match a value", "x", nil, false},
		{"absent current does not match a string", nil, "x", false},

		{"bool equal", true, true, true},
		{"bool unequal", false, true, false},
		{"bool desired against a string current", "true", true, false},

		{"int against decoded float", float64(5), 5, true},
		{"int against decoded float, unequal", float64(6), 5, false},
		{"int against fractional float", 5.5, 5, false},
		{"int against int", 5, 5, true},
		{"int against int64", int64(5), 5, true},
		{"int against a numeric string current", "5", 5, true},
		{"int against a non-numeric string current", "five", 5, false},
		{"zero against decoded zero", float64(0), 0, true},
		{"int64 desired against decoded float", float64(9000000000), int64(9000000000), true},
		{"float desired against decoded float", float64(1.5), 1.5, true},
		{"int against absent current", nil, 5, false},

		{"string equal", "orders", "orders", true},
		{"string unequal", "orders", "events", false},
		{"string desired against numeric current", float64(5), "5", false},

		{"string slice equal", []any{"a", "b"}, []string{"a", "b"}, true},
		{"string slice order matters", []any{"b", "a"}, []string{"a", "b"}, false},
		{"string slice length differs", []any{"a"}, []string{"a", "b"}, false},
		{"string slice against non-slice current", "a", []string{"a"}, false},
		{"empty slices match", []any{}, []string{}, true},
		{"any slice compares by canonical form", []any{float64(1), float64(2)}, []any{1, 2}, true},

		// A type the switch does not name falls back to canonical string form.
		{"unlisted slice type compares by canonical form", []any{1, 2}, []int{1, 2}, true},
		// Pinned rather than desirable: Stringify gives a float64 a trailing
		// ".0" but sends a float32 through %v, so the two never compare equal.
		// No SEMP attribute is float32, so this is a documented edge, not a bug
		// to work around.
		{"float32 desired never matches a decoded float64", float64(5), float32(5), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, AttrEqual(tc.current, tc.desired))
		})
	}
}
