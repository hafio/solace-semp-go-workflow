// Package engine orchestrates template resolution, variable rendering, and
// idempotent module execution.
package engine

import (
	"fmt"
	"sort"
	"strings"

	"solace-semp-workflow/internal/config"
	wferrors "solace-semp-workflow/internal/errors"
	"solace-semp-workflow/internal/models"
	"solace-semp-workflow/internal/modules"
	"solace-semp-workflow/internal/output"
	"solace-semp-workflow/internal/semp"
	"solace-semp-workflow/internal/templating"
	"solace-semp-workflow/templates"
)

// maxRenderPasses bounds the input-rendering loop. A single pass is not enough
// when a provided value references a global_var that itself contains
// {{ inputs.X }} (a 3-level chain); reaching the limit with unresolved markers
// signals a circular reference.
const maxRenderPasses = 2

// Engine loads templates, resolves variables, and executes workflows.
type Engine struct {
	config         *config.AppConfig
	dryRun         bool
	failFast       bool
	templateEngine *templating.Engine
	templates      map[string]*config.WorkflowTemplate
	client         modules.Client
}

// NewEngine builds an engine: it loads the workflow templates from the bundled
// (embedded) source or the configured directory, and constructs the SEMP client.
func NewEngine(cfg *config.AppConfig, dryRun, failFast bool) (*Engine, error) {
	var tmpls map[string]*config.WorkflowTemplate
	var err error
	if cfg.UseBundledTemplates {
		tmpls, err = config.LoadTemplatesFS(templates.FS())
	} else {
		tmpls, err = config.LoadTemplatesDir(cfg.TemplatesDir)
	}
	if err != nil {
		return nil, err
	}

	client := semp.NewClient(
		cfg.Semp.Host,
		cfg.Semp.Username,
		cfg.Semp.Password,
		cfg.Semp.MsgVPN,
		cfg.Semp.VerifySSL,
		cfg.Semp.Timeout,
	)

	return &Engine{
		config:         cfg,
		dryRun:         dryRun,
		failFast:       failFast,
		templateEngine: templating.NewEngine(),
		templates:      tmpls,
		client:         client,
	}, nil
}

// Run executes all workflows defined in the config and returns their results.
func (e *Engine) Run() []models.WorkflowResult {
	output.PrintBanner()
	if e.dryRun {
		output.PrintDryRunBanner()
	}

	var results []models.WorkflowResult

	for i, entry := range e.config.Workflows {
		wfResult, err := e.runWorkflow(entry, i+1)
		if err != nil {
			// Template/validation/SEMP errors — record as a single failed task.
			failedResult := models.WorkflowResult{
				WorkflowName: entry.Template,
				TemplateRef:  entry.Template,
				TaskResults: []models.ActionResult{{
					Status:   models.StatusFailed,
					Message:  err.Error(),
					Module:   "engine",
					TaskName: "Template Resolution",
				}},
			}
			results = append(results, failedResult)
			output.PrintTaskResult(failedResult.TaskResults[0])
			if e.failFast {
				break
			}
			continue
		}

		results = append(results, *wfResult)
		if e.failFast && wfResult.HasFailures() {
			break
		}
	}

	output.PrintRecap(results)
	return results
}

// resolveEntry resolves a workflow entry's template and inputs: schema
// validation and defaults, multi-pass rendering until values stabilise, then a
// check that nothing is still unresolved. It returns the template, the resolved
// inputs and the rendering context.
//
// Shared by Run and ValidateWorkflows so an offline check sees exactly what a run
// would send. Duplicating the multi-pass loop instead would give two copies of
// subtle logic that drift.
func (e *Engine) resolveEntry(entry config.WorkflowEntry) (*config.WorkflowTemplate, map[string]any, map[string]any, error) {
	template, err := e.resolveTemplate(entry.Template)
	if err != nil {
		return nil, nil, nil, err
	}

	// First pass: validate and apply defaults with global_vars context only.
	baseContext := map[string]any{"global_vars": e.config.GlobalVars}
	validatedInputs, err := templating.ValidateInputs(entry.Inputs, template.Inputs, e.templateEngine, baseContext)
	if err != nil {
		return nil, nil, nil, err
	}

	// Full context with resolved inputs.
	context := map[string]any{
		"global_vars": e.config.GlobalVars,
		"inputs":      validatedInputs,
	}

	// Multi-pass rendering: resolve Jinja2 expressions in input values. Loop
	// until values stabilise or the pass limit is reached.
	for pass := 0; pass < maxRenderPasses; pass++ {
		changed := false
		for key := range validatedInputs {
			val, ok := validatedInputs[key].(string)
			if !ok || (!strings.Contains(val, "{{") && !strings.Contains(val, "{%")) {
				continue
			}
			newValRaw, rerr := e.templateEngine.Render(val, context)
			if rerr != nil {
				return nil, nil, nil, wferrors.NewWorkflowError(fmt.Sprintf("Failed to resolve input '%s': %s", key, rerr))
			}
			newVal, _ := newValRaw.(string)
			if newVal != val {
				validatedInputs[key] = newVal
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	// Detect unresolved expressions after all passes -- a circular reference or a
	// typo referencing a non-existent input.
	for key, v := range validatedInputs {
		if s, ok := v.(string); ok && (strings.Contains(s, "{{") || strings.Contains(s, "{%")) {
			return nil, nil, nil, wferrors.NewWorkflowError(fmt.Sprintf(
				"Input '%s' still contains an unresolved Jinja2 expression after rendering -- "+
					"possible circular reference or undefined variable: '%s'", key, s))
		}
	}

	return template, validatedInputs, context, nil
}

// ValidateWorkflows checks every configured workflow without contacting the
// broker: its template resolves, its inputs resolve, every action names a
// registered module, and each action's rendered args satisfy that module's
// schema. Every problem is collected rather than stopping at the first, so one
// run lists everything to fix.
func (e *Engine) ValidateWorkflows() []error {
	var problems []error
	for i, entry := range e.config.Workflows {
		label := fmt.Sprintf("workflow %d (%s)", i+1, entry.Template)

		template, _, context, err := e.resolveEntry(entry)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", label, err))
			continue
		}

		for _, action := range template.Actions {
			where := fmt.Sprintf("%s, task '%s' [%s]", label, action.Name, action.Module)

			module, err := modules.Get(action.Module)
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", where, err))
				continue
			}
			resolvedRaw, err := e.templateEngine.Render(action.Args, context)
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", where, err))
				continue
			}
			resolvedArgs, _ := resolvedRaw.(map[string]any)
			if err := module.Validate(resolvedArgs); err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", where, err))
			}
		}
	}
	return problems
}

// runWorkflow executes a single workflow entry.
func (e *Engine) runWorkflow(entry config.WorkflowEntry, index int) (*models.WorkflowResult, error) {
	template, validatedInputs, context, err := e.resolveEntry(entry)
	if err != nil {
		return nil, err
	}

	output.PrintWorkflowHeader(template.Name, entry.Template, validatedInputs, index)

	wfResult := &models.WorkflowResult{
		WorkflowName: template.Name,
		TemplateRef:  entry.Template,
	}

	for _, action := range template.Actions {
		taskResult := e.runAction(action.Name, action.Module, action.Args, context)
		wfResult.TaskResults = append(wfResult.TaskResults, taskResult)
		output.PrintTaskResult(taskResult)

		if e.failFast && taskResult.Status == models.StatusFailed {
			break
		}
	}

	return wfResult, nil
}

// runAction resolves an action's args and executes its module idempotently. A
// panic in a module is recovered into a FAILED result so one bad action cannot
// abort the whole run.
func (e *Engine) runAction(taskName, moduleName string, rawArgs, context map[string]any) (result models.ActionResult) {
	defer func() {
		if r := recover(); r != nil {
			result = models.ActionResult{
				Status:   models.StatusFailed,
				Message:  fmt.Sprintf("Unexpected error: %v", r),
				Module:   moduleName,
				TaskName: taskName,
			}
		}
	}()

	resolvedRaw, err := e.templateEngine.Render(rawArgs, context)
	if err != nil {
		return models.ActionResult{
			Status:   models.StatusFailed,
			Message:  fmt.Sprintf("Template error: %s", err),
			Module:   moduleName,
			TaskName: taskName,
		}
	}
	resolvedArgs, _ := resolvedRaw.(map[string]any)
	if resolvedArgs == nil {
		resolvedArgs = map[string]any{}
	}

	module, err := modules.Get(moduleName)
	if err != nil {
		return models.ActionResult{
			Status:   models.StatusFailed,
			Message:  err.Error(),
			Module:   moduleName,
			TaskName: taskName,
		}
	}

	result = module.Execute(e.client, resolvedArgs, e.dryRun)
	result.Module = moduleName
	result.TaskName = taskName
	return result
}

// resolveTemplate resolves a "filename.TemplateName" reference to a template.
func (e *Engine) resolveTemplate(ref string) (*config.WorkflowTemplate, error) {
	t, ok := e.templates[ref]
	if !ok {
		avail := make([]string, 0, len(e.templates))
		for k := range e.templates {
			avail = append(avail, k)
		}
		sort.Strings(avail)
		return nil, wferrors.NewTemplateError(fmt.Sprintf("Template '%s' not found. Available: %s", ref, strings.Join(avail, ", ")))
	}
	return t, nil
}
