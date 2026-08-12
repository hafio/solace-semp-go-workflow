package templating

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wferrors "solace-semp-workflow/internal/errors"
)

// A render error inside a map value must abort the whole Render and surface as a
// TemplateError (covers the error-return in the map recursion branch).
func TestRenderMapRecursionError(t *testing.T) {
	e := NewEngine()
	out, err := e.Render(map[string]any{"bad": "{{ inputs.nope }}"}, ctx(map[string]any{}, nil))
	require.Error(t, err)
	assert.Nil(t, out)
	var te *wferrors.TemplateError
	assert.True(t, stderrors.As(err, &te))
}

// Same for a render error inside a slice element (covers the slice recursion
// error-return branch).
func TestRenderSliceRecursionError(t *testing.T) {
	e := NewEngine()
	out, err := e.Render([]any{"literal", "{{ inputs.nope }}"}, ctx(map[string]any{}, nil))
	require.Error(t, err)
	assert.Nil(t, out)
	var te *wferrors.TemplateError
	assert.True(t, stderrors.As(err, &te))
}

// A template that fails to parse (unbalanced parenthesis) must become a
// TemplateError from the NewTemplate error branch, tagged "Template rendering
// error" rather than "Undefined variable".
func TestRenderMalformedTemplateError(t *testing.T) {
	e := NewEngine()
	_, err := e.Render("{{ (1 }}", ctx(nil, nil))
	require.Error(t, err)
	var te *wferrors.TemplateError
	require.True(t, stderrors.As(err, &te))
	assert.Contains(t, err.Error(), "Template rendering error")
}

// An empty subscript ({{ inputs[] }}) parses to a GetItem with no argument, which
// under StrictUndefined fails at execution with an "undefined" message — driving
// renderString down the isUndefinedErr branch ("Undefined variable in ...").
func TestRenderUndefinedIndexBranch(t *testing.T) {
	e := NewEngine()
	_, err := e.Render("{{ inputs[] }}", ctx(map[string]any{}, nil))
	require.Error(t, err)
	var te *wferrors.TemplateError
	require.True(t, stderrors.As(err, &te))
	assert.Contains(t, err.Error(), "Undefined variable")
}

// An optional input with no default that is not provided is silently skipped and
// omitted from the result (covers the else/continue branch in ValidateInputs).
func TestValidateInputsOptionalSkipped(t *testing.T) {
	e := NewEngine()
	schema := InputSchema{{Name: "opt", Required: false, HasDefault: false}}
	out, err := ValidateInputs(map[string]any{}, schema, e, ctx(nil, nil))
	require.NoError(t, err)
	_, ok := out["opt"]
	assert.False(t, ok)
	assert.Empty(t, out)
}

// An unrecognized declared type falls through coerceType unchanged, preserving
// the original value and type (covers coerceType's default branch).
func TestValidateInputsUnknownTypePassthrough(t *testing.T) {
	e := NewEngine()
	schema := InputSchema{{Name: "amount", Required: true, Type: "float"}}
	out, err := ValidateInputs(map[string]any{"amount": 3.14}, schema, e, ctx(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, 3.14, out["amount"])
}

// Integer coercion across every toInt input kind: bools map to 1/0, native ints
// pass through, int64 narrows, floats truncate toward zero, and an unsupported
// type (slice) hits toInt's default branch and yields the "must be integer" error.
func TestValidateInputsIntegerCoercionVariants(t *testing.T) {
	e := NewEngine()
	schema := InputSchema{{Name: "n", Required: true, Type: "integer"}}
	tests := []struct {
		name    string
		value   any
		want    int
		wantErr bool
	}{
		{"bool true", true, 1, false},
		{"bool false", false, 0, false},
		{"native int", 7, 7, false},
		{"int64 narrows", int64(9), 9, false},
		{"float truncates", 3.9, 3, false},
		{"uncoercible slice", []any{1, 2}, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ValidateInputs(map[string]any{"n": tc.value}, schema, e, ctx(nil, nil))
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "must be integer")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, out["n"])
		})
	}
}
