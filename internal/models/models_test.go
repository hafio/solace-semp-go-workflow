package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowResultCounts(t *testing.T) {
	wf := WorkflowResult{
		WorkflowName: "wf",
		TaskResults: []ActionResult{
			{Status: StatusOK},
			{Status: StatusOK},
			{Status: StatusSkipped},
			{Status: StatusDryRun},
			{Status: StatusFailed},
		},
	}
	assert.Equal(t, 2, wf.OKCount())
	assert.Equal(t, 1, wf.SkippedCount())
	assert.Equal(t, 1, wf.DryRunCount())
	assert.Equal(t, 1, wf.FailedCount())
	assert.True(t, wf.HasFailures())
}

func TestWorkflowResultNoFailures(t *testing.T) {
	wf := WorkflowResult{TaskResults: []ActionResult{{Status: StatusOK}, {Status: StatusSkipped}}}
	assert.False(t, wf.HasFailures())
	assert.Equal(t, 0, wf.FailedCount())
}

func TestNewActionResult(t *testing.T) {
	r := NewActionResult(StatusFailed, "boom")
	assert.Equal(t, StatusFailed, r.Status)
	assert.Equal(t, "boom", r.Message)
	assert.Empty(t, r.Module)
	assert.Empty(t, r.TaskName)
}
