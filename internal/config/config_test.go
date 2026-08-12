package config

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wferrors "solace-semp-workflow/internal/errors"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

const validConfig = `
semp:
  host: https://localhost:8080
  username: admin
  password: secret
  msg_vpn: default
workflows:
  - template: app-inbound.TestFlow
    inputs:
      queueName: Q1
`

func TestLoadConfigValid(t *testing.T) {
	path := writeConfig(t, validConfig)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "https://localhost:8080", cfg.Semp.Host)
	assert.Equal(t, "default", cfg.Semp.MsgVPN)
	assert.False(t, cfg.Semp.VerifySSL) // default
	assert.Equal(t, 30, cfg.Semp.Timeout)
	assert.NotNil(t, cfg.GlobalVars) // default empty map
	require.Len(t, cfg.Workflows, 1)
	assert.Equal(t, "app-inbound.TestFlow", cfg.Workflows[0].Template)
	assert.Equal(t, "Q1", cfg.Workflows[0].Inputs["queueName"])
	assert.True(t, cfg.UseBundledTemplates) // no templates dir on disk
}

func TestLoadConfigTemplatesDirOnDisk(t *testing.T) {
	path := writeConfig(t, validConfig)
	require.NoError(t, os.Mkdir(filepath.Join(filepath.Dir(path), "templates"), 0o755))
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.False(t, cfg.UseBundledTemplates)
	assert.Equal(t, filepath.Join(filepath.Dir(path), "templates"), cfg.TemplatesDir)
}

func TestLoadConfigOverrides(t *testing.T) {
	path := writeConfig(t, `
semp:
  host: h
  username: u
  password: p
  msg_vpn: v
  verify_ssl: true
  timeout: 60
`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.True(t, cfg.Semp.VerifySSL)
	assert.Equal(t, 60, cfg.Semp.Timeout)
	assert.Empty(t, cfg.Workflows)
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nope.yaml"))
	require.Error(t, err)
	var ce *wferrors.ConfigError
	require.True(t, stderrors.As(err, &ce))
	assert.Contains(t, err.Error(), "Config file not found")
}

func TestLoadConfigNotAMapping(t *testing.T) {
	path := writeConfig(t, "- a\n- b\n")
	_, err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a YAML mapping")
}

func TestLoadConfigMissingSemp(t *testing.T) {
	path := writeConfig(t, "workflows: []\n")
	_, err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing 'semp' section")
}

func TestLoadConfigMissingSempKey(t *testing.T) {
	path := writeConfig(t, `
semp:
  host: h
  username: u
  password: p
`)
	_, err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing 'semp.msg_vpn'")
}

func TestLoadConfigWorkflowsNotList(t *testing.T) {
	path := writeConfig(t, `
semp: {host: h, username: u, password: p, msg_vpn: v}
workflows: 5
`)
	_, err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'workflows' must be a list")
}

func TestLoadConfigWorkflowMissingTemplate(t *testing.T) {
	path := writeConfig(t, `
semp: {host: h, username: u, password: p, msg_vpn: v}
workflows:
  - inputs: {}
`)
	_, err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Workflow entry 1 must have a 'template' field")
}

// --- template loading -----------------------------------------------------

const templateYAML = `
workflow-templates:
  - name: TestFlow
    inputs:
      required:
        - queueName
      optional:
        clientProfile: default
        port: 443
    actions:
      - name: Create queue
        module: queue.add
        args:
          queueName: "{{ inputs.queueName }}"
`

func TestLoadTemplatesFS(t *testing.T) {
	fsys := fstest.MapFS{
		"app-inbound.yaml": &fstest.MapFile{Data: []byte(templateYAML)},
	}
	reg, err := LoadTemplatesFS(fsys)
	require.NoError(t, err)

	tmpl, ok := reg["app-inbound.TestFlow"]
	require.True(t, ok)
	assert.Equal(t, "TestFlow", tmpl.Name)

	require.Len(t, tmpl.Inputs, 3)
	assert.Equal(t, "queueName", tmpl.Inputs[0].Name)
	assert.True(t, tmpl.Inputs[0].Required)
	// Optional inputs follow, sorted by name.
	assert.Equal(t, "clientProfile", tmpl.Inputs[1].Name)
	assert.True(t, tmpl.Inputs[1].HasDefault)
	assert.Equal(t, "port", tmpl.Inputs[2].Name)

	require.Len(t, tmpl.Actions, 1)
	assert.Equal(t, "queue.add", tmpl.Actions[0].Module)
	assert.Equal(t, "Create queue", tmpl.Actions[0].Name)
}

func TestLoadTemplatesMissingModule(t *testing.T) {
	bad := `
workflow-templates:
  - name: Broken
    actions:
      - name: no module here
        args: {}
`
	fsys := fstest.MapFS{"broken.yaml": &fstest.MapFile{Data: []byte(bad)}}
	_, err := LoadTemplatesFS(fsys)
	require.Error(t, err)
	var te *wferrors.TemplateError
	require.True(t, stderrors.As(err, &te))
	assert.Contains(t, err.Error(), "missing required 'module' field")
}

func TestLoadTemplatesNonListSkipped(t *testing.T) {
	fsys := fstest.MapFS{"weird.yaml": &fstest.MapFile{Data: []byte("workflow-templates: notalist\n")}}
	reg, err := LoadTemplatesFS(fsys)
	require.NoError(t, err)
	assert.Empty(t, reg)
}

func TestLoadTemplatesDirNotFound(t *testing.T) {
	_, err := LoadTemplatesDir(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
	var te *wferrors.TemplateError
	require.True(t, stderrors.As(err, &te))
	assert.Contains(t, err.Error(), "Templates directory not found")
}
