// Package templating resolves Jinja2 expressions in workflow action args using
// gonja (a Jinja2-compatible engine). StrictUndefined is enabled so an undefined
// variable is an error, not a silent blank -- matching Jinja2's StrictUndefined.
package templating

import (
	stderrors "errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/config"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/nikolalohinski/gonja/v2/loaders"

	wferrors "solace-semp-workflow/internal/errors"
	"solace-semp-workflow/internal/semp"
)

// Engine renders Jinja2 expressions in workflow action args. Context is a map
// with keys like "inputs" and "global_vars". Filters such as
// {{ inputs.x | default('fallback') }} are supported.
type Engine struct {
	cfg *config.Config
}

// NewEngine builds a template engine with StrictUndefined enabled and trailing
// newlines trimmed.
func NewEngine() *Engine {
	cfg := config.New()
	cfg.StrictUndefined = true
	cfg.KeepTrailingNewline = false
	return &Engine{cfg: cfg}
}

// Render recursively renders Jinja2 expressions in a data structure: strings are
// rendered as templates, maps and slices are walked, and any other type is
// passed through unchanged (so a literal YAML bool/int keeps its type).
func (e *Engine) Render(value any, context map[string]any) (any, error) {
	switch v := value.(type) {
	case string:
		return e.renderString(v, context)
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			r, err := e.Render(item, context)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			r, err := e.Render(item, context)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	default:
		return value, nil
	}
}

// renderString renders a single string through gonja. Strings with no template
// markers take a fast path. Undefined variables and other rendering errors both
// become TemplateError, distinguished only by the message prefix.
func (e *Engine) renderString(text string, context map[string]any) (string, error) {
	if !strings.Contains(text, "{{") && !strings.Contains(text, "{%") {
		return text, nil // Fast path: no template syntax
	}

	// gonja's memory loader requires every key to start with '/'; the same path
	// is passed to NewTemplate, which Read()s it back out of the loader.
	loader, err := loaders.NewMemoryLoader(map[string]string{"/t": text})
	if err != nil {
		return "", wferrors.NewTemplateError(fmt.Sprintf("Template rendering error in '%s': %s", text, err))
	}
	tpl, err := exec.NewTemplate("/t", e.cfg, loader, gonja.DefaultEnvironment)
	if err != nil {
		return "", wferrors.NewTemplateError(fmt.Sprintf("Template rendering error in '%s': %s", text, err))
	}

	out, err := tpl.ExecuteToString(exec.NewContext(context))
	if err != nil {
		if isUndefinedErr(err) {
			return "", wferrors.NewTemplateError(fmt.Sprintf("Undefined variable in '%s': %s", text, err))
		}
		return "", wferrors.NewTemplateError(fmt.Sprintf("Template rendering error in '%s': %s", text, err))
	}
	return out, nil
}

// isUndefinedErr reports whether a gonja render error is an undefined-variable
// error, so the message can flag it distinctly. The check is cosmetic -- both
// branches produce a TemplateError, which is what the two-pass resolver and
// default-fallback logic depend on.
func isUndefinedErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "undefined") || strings.Contains(msg, "unable to resolve")
}

// InputSpec describes one workflow-template input. It is stored in an ordered
// slice so a missing-required error fires in declaration order (required inputs
// are listed first, then optional).
type InputSpec struct {
	Name       string
	Required   bool
	HasDefault bool
	Default    any
	// Type is the declared coercion type; "" defaults to "string".
	Type string
}

// InputSchema is an ordered set of input specs for a workflow template.
type InputSchema []InputSpec

// ValidateInputs validates and fills defaults for workflow inputs against a
// schema. Provided values win; otherwise a renderable default is used (a default
// that references not-yet-available inputs raises TemplateError and its raw
// value is kept for the engine's later pass); a missing required input is an
// error; other optional inputs are skipped. Unexpected inputs are rejected.
func ValidateInputs(provided map[string]any, schema InputSchema, engine *Engine, context map[string]any) (map[string]any, error) {
	validated := make(map[string]any, len(schema))

	for _, spec := range schema {
		var val any
		if pv, ok := provided[spec.Name]; ok {
			val = pv
		} else if spec.HasDefault {
			rendered, err := engine.Render(spec.Default, context)
			if err != nil {
				var te *wferrors.TemplateError
				if stderrors.As(err, &te) {
					// Cannot resolve yet (e.g. references inputs.X missing from
					// the first-pass context). Keep raw for the second pass.
					val = spec.Default
				} else {
					return nil, err
				}
			} else {
				val = rendered
			}
		} else if spec.Required {
			return nil, wferrors.NewTemplateError(fmt.Sprintf("Required input '%s' not provided", spec.Name))
		} else {
			continue
		}

		coerced, err := coerceType(spec.Name, val, spec.Type)
		if err != nil {
			return nil, err
		}
		validated[spec.Name] = coerced
	}

	// Reject unexpected inputs (provided but not in the schema).
	known := make(map[string]struct{}, len(schema))
	for _, spec := range schema {
		known[spec.Name] = struct{}{}
	}
	var unexpected []string
	for k := range provided {
		if _, ok := known[k]; !ok {
			unexpected = append(unexpected, k)
		}
	}
	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		return nil, wferrors.NewTemplateError("Unexpected inputs: " + strings.Join(unexpected, ", "))
	}

	return validated, nil
}

// coerceType coerces a value to the expected type. The type defaults to
// "string", so every input is stringified via Stringify unless a template
// explicitly declares another type.
func coerceType(name string, value any, expectedType string) (any, error) {
	if expectedType == "" {
		expectedType = "string"
	}
	switch expectedType {
	case "string":
		return semp.Stringify(value), nil
	case "integer":
		n, ok := toInt(value)
		if !ok {
			return nil, wferrors.NewTemplateError(fmt.Sprintf("Input '%s' must be integer, got: %s", name, semp.Stringify(value)))
		}
		return n, nil
	case "boolean":
		return semp.CoerceBool(value), nil
	default:
		return value, nil
	}
}

// toInt coerces a value to int: bools become 1/0, floats truncate toward zero,
// and strings parse as base-10 integers (no decimal point). It reports false
// when the value cannot be coerced.
func toInt(value any) (int, bool) {
	switch v := value.(type) {
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}
