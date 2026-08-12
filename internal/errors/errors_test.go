package errors

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsWorkflowError(t *testing.T) {
	assert.False(t, IsWorkflowError(nil))
	assert.False(t, IsWorkflowError(stderrors.New("plain")))

	assert.True(t, IsWorkflowError(NewWorkflowError("w")))
	assert.True(t, IsWorkflowError(NewConfigError("c")))
	assert.True(t, IsWorkflowError(NewTemplateError("t")))
	assert.True(t, IsWorkflowError(NewValidationError("v")))
	assert.True(t, IsWorkflowError(NewSEMPError("s", 400, 6)))
}

func TestErrorMessages(t *testing.T) {
	assert.Equal(t, "boom", NewConfigError("boom").Error())
	assert.Equal(t, "bad template", NewTemplateError("bad template").Error())

	se := NewSEMPError("api down", 503, 0)
	assert.Equal(t, "api down", se.Error())
	assert.Equal(t, 503, se.StatusCode)
	assert.Equal(t, 0, se.SempCode)
}

func TestErrorsAsConcreteTypes(t *testing.T) {
	var ce *ConfigError
	require.True(t, stderrors.As(NewConfigError("x"), &ce))

	var te *TemplateError
	require.True(t, stderrors.As(NewTemplateError("x"), &te))

	// A wrapped SEMPError is still discoverable via errors.As.
	wrapped := fmt.Errorf("context: %w", NewSEMPError("boom", 400, 10))
	var se *SEMPError
	require.True(t, stderrors.As(wrapped, &se))
	assert.Equal(t, 10, se.SempCode)
}
