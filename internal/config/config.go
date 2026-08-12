// Package config loads and validates the application config YAML and the
// workflow template files. Template inputs are parsed into an ordered
// templating.InputSchema.
package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	wferrors "solace-semp-workflow/internal/errors"
	"solace-semp-workflow/internal/semp"
	"solace-semp-workflow/internal/templating"
)

// SempConfig holds the SEMP broker connection details.
type SempConfig struct {
	Host      string
	Username  string
	Password  string
	MsgVPN    string
	VerifySSL bool
	Timeout   int
}

// WorkflowEntry is a single workflow invocation from the config file.
type WorkflowEntry struct {
	Template string // "filename.TemplateName"
	Inputs   map[string]any
}

// AppConfig is the top-level application configuration.
type AppConfig struct {
	Semp        SempConfig
	GlobalVars  map[string]any
	Workflows   []WorkflowEntry
	TemplatesDir string
	// UseBundledTemplates selects the embedded template bundle over TemplatesDir
	// when the configured directory does not exist on disk.
	UseBundledTemplates bool
}

// ActionSpec is a single action step within a workflow template.
type ActionSpec struct {
	Name   string
	Module string
	Args   map[string]any
}

// WorkflowTemplate is a parsed workflow template.
type WorkflowTemplate struct {
	Name    string
	Inputs  templating.InputSchema
	Actions []ActionSpec
}

// LoadConfig loads and validates the main config YAML file.
func LoadConfig(configPath string) (*AppConfig, error) {
	if _, err := os.Stat(configPath); err != nil {
		return nil, wferrors.NewConfigError(fmt.Sprintf("Config file not found: %s", configPath))
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, wferrors.NewConfigError(fmt.Sprintf("Config file not found: %s", configPath))
	}

	var data any
	if err := yaml.Unmarshal(content, &data); err != nil {
		return nil, wferrors.NewConfigError(fmt.Sprintf("Config file must be a YAML mapping: %s", err))
	}
	m, ok := data.(map[string]any)
	if !ok {
		return nil, wferrors.NewConfigError("Config file must be a YAML mapping")
	}

	// SEMP section (a present-but-empty section is treated as missing).
	sempData, _ := m["semp"].(map[string]any)
	if len(sempData) == 0 {
		return nil, wferrors.NewConfigError("Missing 'semp' section in config")
	}
	for _, required := range []string{"host", "username", "password", "msg_vpn"} {
		if _, ok := sempData[required]; !ok {
			return nil, wferrors.NewConfigError(fmt.Sprintf("Missing 'semp.%s' in config", required))
		}
	}

	sempCfg := SempConfig{
		Host:      cfgStr(sempData["host"]),
		Username:  cfgStr(sempData["username"]),
		Password:  cfgStr(sempData["password"]),
		MsgVPN:    cfgStr(sempData["msg_vpn"]),
		VerifySSL: cfgBool(sempData["verify_ssl"]),
		Timeout:   cfgInt(sempData["timeout"], 30),
	}

	// Global vars (default empty map).
	globalVars, _ := m["global_vars"].(map[string]any)
	if globalVars == nil {
		globalVars = map[string]any{}
	}

	// Workflows list.
	workflows := []WorkflowEntry{}
	if wfRaw, present := m["workflows"]; present {
		wfList, ok := wfRaw.([]any)
		if !ok {
			return nil, wferrors.NewConfigError("'workflows' must be a list")
		}
		for i, item := range wfList {
			wfMap, ok := item.(map[string]any)
			if !ok {
				return nil, wferrors.NewConfigError(fmt.Sprintf("Workflow entry %d must have a 'template' field", i+1))
			}
			tmplRef, ok := wfMap["template"]
			if !ok {
				return nil, wferrors.NewConfigError(fmt.Sprintf("Workflow entry %d must have a 'template' field", i+1))
			}
			inputs, _ := wfMap["inputs"].(map[string]any)
			if inputs == nil {
				inputs = map[string]any{}
			}
			workflows = append(workflows, WorkflowEntry{Template: cfgStr(tmplRef), Inputs: inputs})
		}
	}

	// Templates source precedence (highest first):
	//   1. --templates-dir CLI flag (applied later in cli, overrides bundled)
	//   2. templates_dir in config.yaml, if the directory exists on disk
	//   3. Embedded bundled templates (always compiled into the binary)
	templatesDirStr := "templates"
	if v, ok := m["templates_dir"]; ok && v != nil {
		templatesDirStr = cfgStr(v)
	}
	templatesDir := filepath.Join(filepath.Dir(configPath), templatesDirStr)
	useBundled := false
	if info, err := os.Stat(templatesDir); err != nil || !info.IsDir() {
		useBundled = true
	}

	return &AppConfig{
		Semp:                sempCfg,
		GlobalVars:          globalVars,
		Workflows:           workflows,
		TemplatesDir:        templatesDir,
		UseBundledTemplates: useBundled,
	}, nil
}

// parseInputsSchema parses a template inputs block into an ordered schema.
//
// Format:
//
//	inputs:
//	  required:
//	    - name1
//	    - name2
//	  optional:
//	    var1: value
//	    var2: 443
//
// Required inputs are emitted first (in list order), then optional inputs
// (sorted by name — ordering among optionals has no observable effect since a
// missing-required error can only fire on a required input).
func parseInputsSchema(inputsData map[string]any) templating.InputSchema {
	var schema templating.InputSchema
	if len(inputsData) == 0 {
		return schema
	}

	if reqRaw, ok := inputsData["required"]; ok && reqRaw != nil {
		if reqList, ok := reqRaw.([]any); ok {
			for _, n := range reqList {
				schema = append(schema, templating.InputSpec{Name: cfgStr(n), Required: true})
			}
		}
	}

	if optRaw, ok := inputsData["optional"]; ok && optRaw != nil {
		if optMap, ok := optRaw.(map[string]any); ok {
			names := make([]string, 0, len(optMap))
			for k := range optMap {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, k := range names {
				spec := templating.InputSpec{Name: k, Required: false}
				if v := optMap[k]; v != nil {
					spec.HasDefault = true
					spec.Default = v
				}
				schema = append(schema, spec)
			}
		}
	}

	return schema
}

// LoadTemplatesDir loads all workflow templates from a filesystem directory.
func LoadTemplatesDir(dir string) (map[string]*WorkflowTemplate, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, wferrors.NewTemplateError(fmt.Sprintf("Templates directory not found: %s", dir))
	}
	return loadTemplatesFS(os.DirFS(dir))
}

// LoadTemplatesFS loads all workflow templates from an fs.FS, used for the
// embedded template bundle.
func LoadTemplatesFS(fsys fs.FS) (map[string]*WorkflowTemplate, error) {
	return loadTemplatesFS(fsys)
}

// loadTemplatesFS reads every *.yaml at the root of fsys and returns a registry
// keyed by "filename.TemplateName".
func loadTemplatesFS(fsys fs.FS) (map[string]*WorkflowTemplate, error) {
	files, err := fs.Glob(fsys, "*.yaml")
	if err != nil {
		return nil, wferrors.NewTemplateError(fmt.Sprintf("Failed to read templates: %s", err))
	}
	sort.Strings(files)

	registry := map[string]*WorkflowTemplate{}

	for _, fname := range files {
		content, err := fs.ReadFile(fsys, fname)
		if err != nil {
			return nil, wferrors.NewTemplateError(fmt.Sprintf("Failed to read template '%s': %s", fname, err))
		}

		var data any
		if err := yaml.Unmarshal(content, &data); err != nil {
			return nil, wferrors.NewTemplateError(fmt.Sprintf("Failed to parse template file '%s': %s", fname, err))
		}
		m, ok := data.(map[string]any)
		if !ok {
			continue // Not a valid YAML mapping — skip this file.
		}
		templatesList, ok := m["workflow-templates"].([]any)
		if !ok {
			continue // 'workflow-templates' is not a list — skip this file.
		}

		fileKey := strings.TrimSuffix(fname, ".yaml")

		for _, td := range templatesList {
			tmplData, ok := td.(map[string]any)
			if !ok {
				continue
			}
			name := cfgStr(tmplData["name"])
			if name == "" {
				continue // Skip template without a name.
			}

			inputsData, _ := tmplData["inputs"].(map[string]any)
			schema := parseInputsSchema(inputsData)

			var actions []ActionSpec
			if actsRaw, ok := tmplData["actions"].([]any); ok {
				for _, ad := range actsRaw {
					actionData, ok := ad.(map[string]any)
					if !ok {
						continue
					}
					aname := "Unnamed Action"
					if v, ok := actionData["name"]; ok && v != nil {
						aname = cfgStr(v)
					}
					moduleRaw, ok := actionData["module"]
					if !ok {
						return nil, wferrors.NewTemplateError(fmt.Sprintf(
							"Action '%s' in template '%s.%s' is missing required 'module' field", aname, fileKey, name))
					}
					args, _ := actionData["args"].(map[string]any)
					if args == nil {
						args = map[string]any{}
					}
					actions = append(actions, ActionSpec{Name: aname, Module: cfgStr(moduleRaw), Args: args})
				}
			}

			registry[fileKey+"."+name] = &WorkflowTemplate{Name: name, Inputs: schema, Actions: actions}
		}
	}

	return registry, nil
}

// cfgStr converts a YAML scalar to its string form via Stringify, so an int or
// bool value in the config reads the same as the equivalent literal.
func cfgStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return semp.Stringify(v)
}

// cfgBool reads a YAML bool value, defaulting to false.
func cfgBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// cfgInt reads a YAML int value, falling back to def when absent or unparseable.
func cfgInt(v any, def int) int {
	if v == nil {
		return def
	}
	n, err := semp.CoerceInt(v)
	if err != nil {
		return def
	}
	return n
}
