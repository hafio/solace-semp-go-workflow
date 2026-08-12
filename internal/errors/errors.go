// Package errors defines the workflow error hierarchy. All error types are
// workflow errors; SEMPError additionally carries the SEMP responseCode and
// error code.
package errors

// WorkflowError is the base error for all workflow failures. The concrete types
// below all satisfy the workflowError marker so callers can test "is this any
// workflow error?" with errors.As against the IsWorkflowError helper.
type WorkflowError struct {
	Message string
}

func (e *WorkflowError) Error() string { return e.Message }
func (e *WorkflowError) workflowErr()  {}

// ConfigError signals invalid or missing configuration.
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string { return e.Message }
func (e *ConfigError) workflowErr()  {}

// TemplateError signals an error loading or resolving a workflow template.
type TemplateError struct {
	Message string
}

func (e *TemplateError) Error() string { return e.Message }
func (e *TemplateError) workflowErr()  {}

// ValidationError signals an input validation failure.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }
func (e *ValidationError) workflowErr()  {}

// SEMPError is an error returned by the Solace SEMP API. StatusCode holds the
// resolved responseCode (0 for transport failures); SempCode holds the broker's
// numeric error code (0 when absent).
type SEMPError struct {
	Message    string
	StatusCode int
	SempCode   int
}

func (e *SEMPError) Error() string { return e.Message }
func (e *SEMPError) workflowErr()  {}

// workflowError is the private marker implemented by every error type in this
// package.
type workflowError interface {
	error
	workflowErr()
}

// IsWorkflowError reports whether err is (or wraps) any workflow error type.
func IsWorkflowError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(workflowError)
	return ok
}

// NewWorkflowError builds a *WorkflowError.
func NewWorkflowError(msg string) *WorkflowError { return &WorkflowError{Message: msg} }

// NewConfigError builds a *ConfigError.
func NewConfigError(msg string) *ConfigError { return &ConfigError{Message: msg} }

// NewTemplateError builds a *TemplateError.
func NewTemplateError(msg string) *TemplateError { return &TemplateError{Message: msg} }

// NewValidationError builds a *ValidationError.
func NewValidationError(msg string) *ValidationError { return &ValidationError{Message: msg} }

// NewSEMPError builds a *SEMPError with the given status and SEMP codes.
func NewSEMPError(msg string, statusCode, sempCode int) *SEMPError {
	return &SEMPError{Message: msg, StatusCode: statusCode, SempCode: sempCode}
}
