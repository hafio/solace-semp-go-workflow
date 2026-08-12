package semp

import "strconv"

// AttrEqual reports whether a broker attribute already holds the desired value.
//
// current comes from a decoded SEMP GET response, so its numbers are float64
// regardless of the attribute's declared width, while desired has been coerced
// to a Go int/bool/string. Numbers are therefore compared by value rather than
// by type; anything the type switch does not recognise falls back to the same
// canonical string form the rest of the client uses.
//
// A caller must not use this for attributes the broker never returns
// (write-only, opaque, and the nested threshold groups): current is absent for
// those, so they would always compare unequal.
func AttrEqual(current, desired any) bool {
	switch d := desired.(type) {
	case nil:
		return current == nil
	case bool:
		c, ok := current.(bool)
		return ok && c == d
	case int:
		return numericEqual(current, float64(d))
	case int64:
		return numericEqual(current, float64(d))
	case float64:
		return numericEqual(current, d)
	case string:
		c, ok := current.(string)
		return ok && c == d
	case []string:
		return sliceEqual(current, toAnySlice(d))
	case []any:
		return sliceEqual(current, d)
	default:
		return Stringify(current) == Stringify(desired)
	}
}

// numericEqual compares a decoded JSON number against a desired numeric value.
// A string current value is parsed, so a broker that returns a numeric
// attribute as a quoted string still compares equal.
func numericEqual(current any, want float64) bool {
	switch c := current.(type) {
	case float64:
		return c == want
	case int:
		return float64(c) == want
	case int64:
		return float64(c) == want
	case string:
		got, err := strconv.ParseFloat(c, 64)
		return err == nil && got == want
	default:
		return false
	}
}

// sliceEqual compares two lists element-wise in order, recursing through
// AttrEqual so elements get the same numeric-type tolerance as scalars: a list
// of numbers decoded as float64 matches one supplied as int. Comparing by
// canonical string form instead would fail that case, because Stringify gives a
// float64 a trailing ".0" that an int does not have.
func sliceEqual(current any, want []any) bool {
	got, ok := current.([]any)
	if !ok {
		return false
	}
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if !AttrEqual(got[i], want[i]) {
			return false
		}
	}
	return true
}

func toAnySlice(in []string) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
