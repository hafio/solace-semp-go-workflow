package engine

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/internal/config"
	"solace-semp-workflow/internal/models"
	"solace-semp-workflow/internal/modules"
	"solace-semp-workflow/internal/output"
	"solace-semp-workflow/internal/templating"
)

// engineFake is a modules.Client that records the last create call.
type engineFake struct {
	exists     bool
	createPath string
	createBody map[string]any
}

func (f *engineFake) Exists(string) (bool, map[string]any, error) { return f.exists, nil, nil }
func (f *engineFake) Create(path string, p map[string]any) (map[string]any, error) {
	f.createPath, f.createBody = path, p
	return map[string]any{}, nil
}
func (f *engineFake) Update(string, map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}
func (f *engineFake) Delete(string) error   { return nil }
func (f *engineFake) DefaultMsgVPN() string { return "test-vpn" }

func silence(t *testing.T) {
	t.Helper()
	origOut, origErr := output.Out, output.ErrOut
	output.Out, output.ErrOut = io.Discard, io.Discard
	t.Cleanup(func() { output.Out, output.ErrOut = origOut, origErr })
}

func newTestEngine(client modules.Client, tmpls map[string]*config.WorkflowTemplate, dryRun, failFast bool) *Engine {
	return &Engine{
		config:         &config.AppConfig{GlobalVars: map[string]any{}},
		dryRun:         dryRun,
		failFast:       failFast,
		templateEngine: templating.NewEngine(),
		templates:      tmpls,
		client:         client,
	}
}

func queueTemplate(args map[string]any, inputs templating.InputSchema) map[string]*config.WorkflowTemplate {
	return map[string]*config.WorkflowTemplate{
		"f.Flow": {
			Name:    "Flow",
			Inputs:  inputs,
			Actions: []config.ActionSpec{{Name: "Create", Module: "queue.add", Args: args}},
		},
	}
}

func TestRunWorkflowRendersArgs(t *testing.T) {
	silence(t)
	fc := &engineFake{exists: false}
	tmpls := queueTemplate(
		map[string]any{"queueName": "{{ inputs.queueName }}"},
		templating.InputSchema{{Name: "queueName", Required: true}},
	)
	eng := newTestEngine(fc, tmpls, false, false)

	res, err := eng.runWorkflow(config.WorkflowEntry{Template: "f.Flow", Inputs: map[string]any{"queueName": "Q1"}}, 1)
	require.NoError(t, err)
	require.Len(t, res.TaskResults, 1)
	assert.Equal(t, models.StatusOK, res.TaskResults[0].Status)
	assert.Equal(t, "Q1", fc.createBody["queueName"])
	assert.Equal(t, "queue.add", res.TaskResults[0].Module)
	assert.Equal(t, "Create", res.TaskResults[0].TaskName)
}

func TestRunWorkflowTwoPassRender(t *testing.T) {
	silence(t)
	fc := &engineFake{exists: false}
	// derived default references another input; needs the second pass to resolve.
	tmpls := queueTemplate(
		map[string]any{"queueName": "{{ inputs.derived }}"},
		templating.InputSchema{
			{Name: "base", Required: true},
			{Name: "derived", HasDefault: true, Default: "{{ inputs.base }}/suffix"},
		},
	)
	eng := newTestEngine(fc, tmpls, false, false)

	res, err := eng.runWorkflow(config.WorkflowEntry{Template: "f.Flow", Inputs: map[string]any{"base": "pre"}}, 1)
	require.NoError(t, err)
	assert.Equal(t, models.StatusOK, res.TaskResults[0].Status)
	assert.Equal(t, "pre/suffix", fc.createBody["queueName"])
}

func TestRunWorkflowCircularReference(t *testing.T) {
	silence(t)
	tmpls := map[string]*config.WorkflowTemplate{
		"f.Flow": {
			Name: "Flow",
			Inputs: templating.InputSchema{
				{Name: "a", HasDefault: true, Default: "{{ inputs.b }}"},
				{Name: "b", HasDefault: true, Default: "{{ inputs.a }}"},
			},
		},
	}
	eng := newTestEngine(&engineFake{}, tmpls, false, false)
	_, err := eng.runWorkflow(config.WorkflowEntry{Template: "f.Flow", Inputs: map[string]any{}}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unresolved")
}

func TestResolveTemplateNotFound(t *testing.T) {
	eng := newTestEngine(&engineFake{}, map[string]*config.WorkflowTemplate{}, false, false)
	_, err := eng.resolveTemplate("nope.X")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Template 'nope.X' not found")
}

func TestRunActionUnknownModule(t *testing.T) {
	eng := newTestEngine(&engineFake{}, nil, false, false)
	res := eng.runAction("task", "bad.mod", map[string]any{}, map[string]any{})
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "Unknown module")
	assert.Equal(t, "bad.mod", res.Module)
}

func TestRunActionTemplateError(t *testing.T) {
	eng := newTestEngine(&engineFake{}, nil, false, false)
	ctx := map[string]any{"inputs": map[string]any{}, "global_vars": map[string]any{}}
	res := eng.runAction("task", "queue.add", map[string]any{"queueName": "{{ inputs.missing }}"}, ctx)
	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "Template error")
}

func TestRunWorkflowFailFastStopsActions(t *testing.T) {
	silence(t)
	tmpls := map[string]*config.WorkflowTemplate{
		"f.Flow": {
			Name: "Flow",
			Actions: []config.ActionSpec{
				{Name: "Update missing", Module: "queue.update", Args: map[string]any{"queueName": "Q", "egressEnabled": "false"}},
				{Name: "Create", Module: "queue.add", Args: map[string]any{"queueName": "Q"}},
			},
		},
	}
	// absent client → queue.update fails (resource does not exist).
	eng := newTestEngine(&engineFake{exists: false}, tmpls, false, true)
	res, err := eng.runWorkflow(config.WorkflowEntry{Template: "f.Flow"}, 1)
	require.NoError(t, err)
	require.Len(t, res.TaskResults, 1) // second action never ran
	assert.Equal(t, models.StatusFailed, res.TaskResults[0].Status)
}

func TestRunDryRun(t *testing.T) {
	silence(t)
	tmpls := queueTemplate(
		map[string]any{"queueName": "{{ inputs.queueName }}"},
		templating.InputSchema{{Name: "queueName", Required: true}},
	)
	eng := newTestEngine(&engineFake{exists: false}, tmpls, true, false)
	eng.config.Workflows = []config.WorkflowEntry{{Template: "f.Flow", Inputs: map[string]any{"queueName": "Q1"}}}

	results := eng.Run()
	require.Len(t, results, 1)
	assert.Equal(t, 1, results[0].DryRunCount())
	assert.False(t, results[0].HasFailures())
}

func TestRunTemplateNotFoundSynthesizesFailure(t *testing.T) {
	silence(t)
	eng := newTestEngine(&engineFake{}, map[string]*config.WorkflowTemplate{}, false, false)
	eng.config.Workflows = []config.WorkflowEntry{{Template: "missing.X"}}

	results := eng.Run()
	require.Len(t, results, 1)
	require.Len(t, results[0].TaskResults, 1)
	assert.Equal(t, models.StatusFailed, results[0].TaskResults[0].Status)
	assert.Equal(t, "engine", results[0].TaskResults[0].Module)
	assert.Equal(t, "Template Resolution", results[0].TaskResults[0].TaskName)
}
