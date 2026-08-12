package semp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringify(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, "None"},
		{"true", true, "True"},
		{"false", false, "False"},
		{"string", "abc", "abc"},
		{"int", 5, "5"},
		{"int64", int64(7), "7"},
		{"float whole", 5.0, "5.0"},
		{"float frac", 5.5, "5.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Stringify(tc.in))
		})
	}
}

func TestEnc(t *testing.T) {
	// Every byte but the unreserved set is percent-encoded with uppercase hex;
	// '/' is NOT left alone.
	assert.Equal(t, "a%20b", Enc("a b"))
	assert.Equal(t, "SITEA%2FAPP%2F%3E", Enc("SITEA/APP/>"))
	assert.Equal(t, "abc-_.~", Enc("abc-_.~"))
	assert.Equal(t, "a%40b", Enc("a@b"))
}

func TestCoerceBool(t *testing.T) {
	truthy := []any{true, "true", "TRUE", "Yes", "1", 1, int64(2), 3.5}
	for _, v := range truthy {
		assert.Truef(t, CoerceBool(v), "expected %v truthy", v)
	}
	falsy := []any{false, "false", "no", "", "0", nil, 0, int64(0), 0.0}
	for _, v := range falsy {
		assert.Falsef(t, CoerceBool(v), "expected %v falsy", v)
	}
}

func TestCoerceInt(t *testing.T) {
	n, err := CoerceInt(5)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = CoerceInt(int64(9))
	require.NoError(t, err)
	assert.Equal(t, 9, n)

	n, err = CoerceInt("10")
	require.NoError(t, err)
	assert.Equal(t, 10, n)

	n, err = CoerceInt("  42 ")
	require.NoError(t, err)
	assert.Equal(t, 42, n)

	// A float stringifies to "5.0", which is not a valid base-10 int, so this errors.
	_, err = CoerceInt(5.0)
	assert.Error(t, err)

	_, err = CoerceInt("abc")
	assert.Error(t, err)
}

func TestCleanPayload(t *testing.T) {
	in := map[string]any{
		"nilVal":   nil,
		"blank":    "   ",
		"empty":    "",
		"keep":     "x",
		"zero":     0,
		"falseVal": false,
	}
	out := CleanPayload(in)
	assert.NotContains(t, out, "nilVal")
	assert.NotContains(t, out, "blank")
	assert.NotContains(t, out, "empty")
	assert.Equal(t, "x", out["keep"])
	assert.Equal(t, 0, out["zero"])
	assert.Equal(t, false, out["falseVal"])
}

func TestCheckNameLength(t *testing.T) {
	assert.Equal(t, "", CheckNameLength("queueName", "short"))
	assert.Equal(t, "", CheckNameLength("unknownField", strings.Repeat("x", 500)))

	msg := CheckNameLength("queueName", strings.Repeat("x", 201))
	assert.Contains(t, msg, "200")
	assert.Contains(t, msg, "queueName")

	// Boundary: exactly the limit is allowed.
	assert.Equal(t, "", CheckNameLength("restConsumerName", strings.Repeat("x", 32)))
	assert.NotEqual(t, "", CheckNameLength("restConsumerName", strings.Repeat("x", 33)))
}
