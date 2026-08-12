package templating

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wferrors "solace-semp-workflow/internal/errors"
)

func ctx(inputs, globals map[string]any) map[string]any {
	return map[string]any{"inputs": inputs, "global_vars": globals}
}

func TestRenderPassthrough(t *testing.T) {
	e := NewEngine()
	// No template markers → returned unchanged (fast path).
	out, err := e.Render("plain text", ctx(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, "plain text", out)

	// Non-string scalars pass through with their type intact.
	out, err = e.Render(5, ctx(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, 5, out)

	out, err = e.Render(true, ctx(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, true, out)
}

func TestRenderString(t *testing.T) {
	e := NewEngine()
	out, err := e.Render("{{ inputs.name }}", ctx(map[string]any{"name": "queue-1"}, nil))
	require.NoError(t, err)
	assert.Equal(t, "queue-1", out)
}

func TestRenderDefaultFilter(t *testing.T) {
	e := NewEngine()
	// The default filter must yield the fallback for a missing key even under
	// StrictUndefined — this is what the shipped templates rely on.
	out, err := e.Render("{{ inputs.missing | default('fallback') }}", ctx(map[string]any{}, nil))
	require.NoError(t, err)
	assert.Equal(t, "fallback", out)
}

func TestRenderUndefinedIsTemplateError(t *testing.T) {
	e := NewEngine()
	_, err := e.Render("{{ inputs.nope }}", ctx(map[string]any{}, nil))
	require.Error(t, err)
	var te *wferrors.TemplateError
	assert.True(t, stderrors.As(err, &te))
}

func TestRenderRecursesMapsAndSlices(t *testing.T) {
	e := NewEngine()
	in := map[string]any{
		"name":  "{{ inputs.n }}",
		"count": 3,
		"list":  []any{"{{ inputs.n }}", "literal"},
	}
	out, err := e.Render(in, ctx(map[string]any{"n": "x"}, nil))
	require.NoError(t, err)
	m := out.(map[string]any)
	assert.Equal(t, "x", m["name"])
	assert.Equal(t, 3, m["count"])
	assert.Equal(t, []any{"x", "literal"}, m["list"])
}

func TestValidateInputsProvidedWins(t *testing.T) {
	e := NewEngine()
	schema := InputSchema{
		{Name: "topic", Required: false, HasDefault: true, Default: "def-topic"},
	}
	out, err := ValidateInputs(map[string]any{"topic": "provided"}, schema, e, ctx(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, "provided", out["topic"])
}

func TestValidateInputsUsesDefault(t *testing.T) {
	e := NewEngine()
	schema := InputSchema{
		{Name: "topic", HasDefault: true, Default: "def-topic"},
	}
	out, err := ValidateInputs(map[string]any{}, schema, e, ctx(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, "def-topic", out["topic"])
}

func TestValidateInputsRequiredMissing(t *testing.T) {
	e := NewEngine()
	schema := InputSchema{{Name: "queueName", Required: true}}
	_, err := ValidateInputs(map[string]any{}, schema, e, ctx(nil, nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Required input 'queueName' not provided")
}

func TestValidateInputsUnexpected(t *testing.T) {
	e := NewEngine()
	schema := InputSchema{{Name: "known", Required: true}}
	_, err := ValidateInputs(map[string]any{"known": "v", "surprise": "x", "another": "y"}, schema, e, ctx(nil, nil))
	require.Error(t, err)
	// Sorted for determinism.
	assert.Contains(t, err.Error(), "Unexpected inputs: another, surprise")
}

func TestValidateInputsCoercionToString(t *testing.T) {
	e := NewEngine()
	schema := InputSchema{{Name: "port", Required: true}} // no Type → defaults to string
	out, err := ValidateInputs(map[string]any{"port": 8080}, schema, e, ctx(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, "8080", out["port"]) // stringified via Stringify
}

func TestValidateInputsIntegerType(t *testing.T) {
	e := NewEngine()
	schema := InputSchema{{Name: "n", Required: true, Type: "integer"}}
	out, err := ValidateInputs(map[string]any{"n": "42"}, schema, e, ctx(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, 42, out["n"])

	_, err = ValidateInputs(map[string]any{"n": "notanint"}, schema, e, ctx(nil, nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be integer")
}

func TestValidateInputsBooleanType(t *testing.T) {
	e := NewEngine()
	schema := InputSchema{{Name: "flag", Required: true, Type: "boolean"}}
	out, err := ValidateInputs(map[string]any{"flag": "yes"}, schema, e, ctx(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, true, out["flag"])
}

func TestValidateInputsUnresolvableDefaultKeptRaw(t *testing.T) {
	e := NewEngine()
	// A default that references inputs (not yet available) must survive as its
	// raw string so the engine's later render pass can resolve it.
	schema := InputSchema{
		{Name: "derived", HasDefault: true, Default: "{{ inputs.base }}/suffix"},
	}
	out, err := ValidateInputs(map[string]any{}, schema, e, ctx(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, "{{ inputs.base }}/suffix", out["derived"])
}
