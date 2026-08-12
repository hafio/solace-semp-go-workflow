package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWorkflowAndValidationErrorMessages covers WorkflowError.Error() and
// ValidationError.Error(), which the other tests exercise only via the
// IsWorkflowError marker check.
func TestWorkflowAndValidationErrorMessages(t *testing.T) {
	assert.Equal(t, "msg", NewWorkflowError("msg").Error())
	assert.Equal(t, "msg", NewValidationError("msg").Error())
}
