// Package models holds the core result data types: the result status of an
// idempotent action and the per-workflow aggregation used by the recap.
package models

// ResultStatus is the outcome of a single idempotent action.
type ResultStatus string

const (
	// StatusOK means the action ran and changed something.
	StatusOK ResultStatus = "ok"
	// StatusSkipped means the resource was already in the desired state.
	StatusSkipped ResultStatus = "skipped"
	// StatusFailed means the action errored.
	StatusFailed ResultStatus = "failed"
	// StatusDryRun means the action would have changed something (dry-run mode).
	StatusDryRun ResultStatus = "dryrun"
)

// ActionResult is the result of a single task execution. Module and TaskName are
// back-filled by the engine after the module returns.
type ActionResult struct {
	Status   ResultStatus
	Message  string
	Module   string
	TaskName string
}

// NewActionResult builds an ActionResult with the given status and message.
func NewActionResult(status ResultStatus, message string) ActionResult {
	return ActionResult{Status: status, Message: message}
}

// WorkflowResult aggregates the task results of a single workflow execution.
type WorkflowResult struct {
	WorkflowName string
	TemplateRef  string
	TaskResults  []ActionResult
}

// OKCount returns the number of tasks that changed state.
func (w *WorkflowResult) OKCount() int { return w.count(StatusOK) }

// SkippedCount returns the number of skipped tasks.
func (w *WorkflowResult) SkippedCount() int { return w.count(StatusSkipped) }

// FailedCount returns the number of failed tasks.
func (w *WorkflowResult) FailedCount() int { return w.count(StatusFailed) }

// DryRunCount returns the number of dry-run tasks.
func (w *WorkflowResult) DryRunCount() int { return w.count(StatusDryRun) }

// HasFailures reports whether any task failed.
func (w *WorkflowResult) HasFailures() bool { return w.FailedCount() > 0 }

func (w *WorkflowResult) count(status ResultStatus) int {
	n := 0
	for _, t := range w.TaskResults {
		if t.Status == status {
			n++
		}
	}
	return n
}
