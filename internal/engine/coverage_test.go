package engine

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/internal/config"
	"solace-semp-workflow/internal/models"
	"solace-semp-workflow/internal/templating"
)

// TestNewEngineBundledTemplates covers the NewEngine constructor happy path: the
// embedded template bundle is loaded and the SEMP client is constructed.
func TestNewEngineBundledTemplates(t *testing.T) {
	cfg := &config.AppConfig{
		Semp: config.SempConfig{
			Host:     "https://broker.example.com:943",
			Username: "admin",
			Password: "secret",
			MsgVPN:   "default",
		},
		GlobalVars:          map[string]any{},
		UseBundledTemplates: true,
	}

	eng, err := NewEngine(cfg, true, false)
	require.NoError(t, err)
	require.NotNil(t, eng)
	assert.NotEmpty(t, eng.templates) // bundled app-*.yaml templates loaded
	assert.NotNil(t, eng.client)
	assert.Same(t, cfg, eng.config)
	assert.True(t, eng.dryRun)
	assert.False(t, eng.failFast)
}

// TestNewEngineMissingTemplatesDir covers the NewEngine failure branch when the
// configured (non-bundled) templates directory does not exist.
func TestNewEngineMissingTemplatesDir(t *testing.T) {
	cfg := &config.AppConfig{
		Semp:                config.SempConfig{Host: "h", Username: "u", Password: "p", MsgVPN: "v"},
		GlobalVars:          map[string]any{},
		UseBundledTemplates: false,
		TemplatesDir:        filepath.Join(t.TempDir(), "does-not-exist"),
	}

	eng, err := NewEngine(cfg, false, false)
	require.Error(t, err)
	assert.Nil(t, eng)
	assert.Contains(t, err.Error(), "Templates directory not found")
}

// TestRunWorkflowMissingRequiredInput covers the ValidateInputs error branch:
// a required template input that the workflow entry does not provide.
func TestRunWorkflowMissingRequiredInput(t *testing.T) {
	silence(t)
	tmpls := map[string]*config.WorkflowTemplate{
		"f.Flow": {
			Name:   "Flow",
			Inputs: templating.InputSchema{{Name: "q", Required: true}},
		},
	}
	eng := newTestEngine(&engineFake{}, tmpls, false, false)

	_, err := eng.runWorkflow(config.WorkflowEntry{Template: "f.Flow", Inputs: map[string]any{}}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Required input 'q' not provided")
}

// TestRunWorkflowInputRenderError covers the render-error branch inside the
// multi-pass input resolver: a provided input value references an undefined
// variable, so StrictUndefined turns the pass into a WorkflowError.
func TestRunWorkflowInputRenderError(t *testing.T) {
	silence(t)
	tmpls := map[string]*config.WorkflowTemplate{
		"f.Flow": {
			Name:   "Flow",
			Inputs: templating.InputSchema{{Name: "q", Required: true}},
		},
	}
	eng := newTestEngine(&engineFake{}, tmpls, false, false)

	_, err := eng.runWorkflow(
		config.WorkflowEntry{Template: "f.Flow", Inputs: map[string]any{"q": "{{ inputs.nope }}"}}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to resolve input 'q'")
}

// TestRunFailFastStopsOnTemplateError covers the fail-fast break in Run() after a
// template-resolution error: the second workflow entry never produces a result.
func TestRunFailFastStopsOnTemplateError(t *testing.T) {
	silence(t)
	eng := newTestEngine(&engineFake{}, map[string]*config.WorkflowTemplate{}, false, true)
	eng.config.Workflows = []config.WorkflowEntry{
		{Template: "missing.A"},
		{Template: "missing.B"},
	}

	results := eng.Run()
	require.Len(t, results, 1) // fail-fast broke after the first entry
	require.Len(t, results[0].TaskResults, 1)
	assert.Equal(t, models.StatusFailed, results[0].TaskResults[0].Status)
}

// TestRunFailFastStopsAfterFailedWorkflow covers the fail-fast break in Run()
// after a workflow completes with a failed action: the second entry never runs.
func TestRunFailFastStopsAfterFailedWorkflow(t *testing.T) {
	silence(t)
	tmpls := map[string]*config.WorkflowTemplate{
		"f.Flow": {
			Name: "Flow",
			Actions: []config.ActionSpec{
				{Name: "Update missing", Module: "queue.update", Args: map[string]any{"queueName": "Q", "egressEnabled": "false"}},
			},
		},
	}
	// absent client → queue.update fails (resource does not exist).
	eng := newTestEngine(&engineFake{exists: false}, tmpls, false, true)
	eng.config.Workflows = []config.WorkflowEntry{
		{Template: "f.Flow"},
		{Template: "f.Flow"},
	}

	results := eng.Run()
	require.Len(t, results, 1) // fail-fast broke after the first workflow failed
	assert.True(t, results[0].HasFailures())
}

// TestResolveTemplateNotFoundListsAvailable covers the available-names loop in
// resolveTemplate when the registry is non-empty and the ref is not found.
func TestResolveTemplateNotFoundListsAvailable(t *testing.T) {
	eng := newTestEngine(&engineFake{}, map[string]*config.WorkflowTemplate{
		"f.Flow":  {Name: "Flow"},
		"g.Other": {Name: "Other"},
	}, false, false)

	_, err := eng.resolveTemplate("nope.X")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Available:")
	assert.Contains(t, err.Error(), "f.Flow")
	assert.Contains(t, err.Error(), "g.Other")
}
