package config

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadConfigReadError exercises the os.ReadFile failure branch: a directory
// path passes os.Stat but cannot be read as a file.
func TestLoadConfigReadError(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadConfig(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Config file not found")
}

// TestLoadConfigParseErrors covers the YAML-unmarshal failure branch and the
// non-map workflow entry branch.
func TestLoadConfigParseErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "malformed yaml",
			body: "foo: bar: baz\n",
			want: "must be a YAML mapping",
		},
		{
			name: "workflow entry not a map",
			body: "semp: {host: h, username: u, password: p, msg_vpn: v}\nworkflows:\n  - scalarentry\n",
			want: "Workflow entry 1 must have a 'template' field",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(writeConfig(t, tt.body))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// TestLoadConfigEdgeDefaults covers the fall-through defaulting branches:
// workflow without inputs, explicit templates_dir, a non-string scalar coerced
// via cfgStr, and an unparseable timeout falling back to the default.
func TestLoadConfigEdgeDefaults(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		check func(t *testing.T, cfg *AppConfig, path string)
	}{
		{
			name: "workflow without inputs defaults to empty map",
			body: "semp: {host: h, username: u, password: p, msg_vpn: v}\nworkflows:\n  - template: f.T\n",
			check: func(t *testing.T, cfg *AppConfig, _ string) {
				require.Len(t, cfg.Workflows, 1)
				assert.Equal(t, "f.T", cfg.Workflows[0].Template)
				assert.NotNil(t, cfg.Workflows[0].Inputs)
				assert.Empty(t, cfg.Workflows[0].Inputs)
			},
		},
		{
			name: "explicit templates_dir",
			body: "semp: {host: h, username: u, password: p, msg_vpn: v}\ntemplates_dir: custom-templates\n",
			check: func(t *testing.T, cfg *AppConfig, path string) {
				assert.Equal(t, filepath.Join(filepath.Dir(path), "custom-templates"), cfg.TemplatesDir)
			},
		},
		{
			name: "non-string scalar coerced to string",
			body: "semp: {host: h, username: u, password: p, msg_vpn: 123}\n",
			check: func(t *testing.T, cfg *AppConfig, _ string) {
				assert.Equal(t, "123", cfg.Semp.MsgVPN)
			},
		},
		{
			name: "unparseable timeout falls back to default",
			body: "semp: {host: h, username: u, password: p, msg_vpn: v, timeout: notanumber}\n",
			check: func(t *testing.T, cfg *AppConfig, _ string) {
				assert.Equal(t, 30, cfg.Semp.Timeout)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.body)
			cfg, err := LoadConfig(path)
			require.NoError(t, err)
			tt.check(t, cfg, path)
		})
	}
}

// TestLoadTemplatesDirSuccess exercises the LoadTemplatesDir happy path (reading
// real *.yaml files off disk via os.DirFS).
func TestLoadTemplatesDirSuccess(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app-inbound.yaml"), []byte(templateYAML), 0o644))
	reg, err := LoadTemplatesDir(dir)
	require.NoError(t, err)
	_, ok := reg["app-inbound.TestFlow"]
	assert.True(t, ok)
}

// TestLoadTemplatesReadError covers the fs.ReadFile failure branch: a directory
// whose name matches *.yaml is listed by Glob but cannot be read as a file.
func TestLoadTemplatesReadError(t *testing.T) {
	fsys := fstest.MapFS{"weird.yaml/inner.txt": &fstest.MapFile{Data: []byte("x")}}
	_, err := LoadTemplatesFS(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to read template")
}

// TestLoadTemplatesParseError covers the template-file YAML-unmarshal failure
// branch.
func TestLoadTemplatesParseError(t *testing.T) {
	fsys := fstest.MapFS{"bad.yaml": &fstest.MapFile{Data: []byte("foo: bar: baz\n")}}
	_, err := LoadTemplatesFS(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to parse template file")
}

// TestLoadTemplatesSkippedEntries covers the several "continue" branches that
// silently skip malformed structures without erroring.
func TestLoadTemplatesSkippedEntries(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
		check func(t *testing.T, reg map[string]*WorkflowTemplate)
	}{
		{
			name:  "top-level not a mapping",
			yaml:  "- a\n- b\n",
			check: func(t *testing.T, reg map[string]*WorkflowTemplate) { assert.Empty(t, reg) },
		},
		{
			name:  "template entry not a map",
			yaml:  "workflow-templates:\n  - scalarentry\n",
			check: func(t *testing.T, reg map[string]*WorkflowTemplate) { assert.Empty(t, reg) },
		},
		{
			name:  "template without a name",
			yaml:  "workflow-templates:\n  - inputs: {}\n",
			check: func(t *testing.T, reg map[string]*WorkflowTemplate) { assert.Empty(t, reg) },
		},
		{
			name: "action entry not a map",
			yaml: "workflow-templates:\n  - name: T\n    actions:\n      - scalaraction\n",
			check: func(t *testing.T, reg map[string]*WorkflowTemplate) {
				wt, ok := reg["f.T"]
				require.True(t, ok)
				assert.Empty(t, wt.Actions)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{"f.yaml": &fstest.MapFile{Data: []byte(tt.yaml)}}
			reg, err := LoadTemplatesFS(fsys)
			require.NoError(t, err)
			tt.check(t, reg)
		})
	}
}

// TestLoadTemplatesActionNoArgs covers the branch where an action has a module
// but no args, defaulting args to an empty (non-nil) map.
func TestLoadTemplatesActionNoArgs(t *testing.T) {
	tmpl := `
workflow-templates:
  - name: NoArgs
    actions:
      - name: a1
        module: queue.add
`
	fsys := fstest.MapFS{"f.yaml": &fstest.MapFile{Data: []byte(tmpl)}}
	reg, err := LoadTemplatesFS(fsys)
	require.NoError(t, err)

	wt, ok := reg["f.NoArgs"]
	require.True(t, ok)
	require.Len(t, wt.Actions, 1)
	assert.Equal(t, "queue.add", wt.Actions[0].Module)
	assert.Equal(t, "a1", wt.Actions[0].Name)
	assert.NotNil(t, wt.Actions[0].Args)
	assert.Empty(t, wt.Actions[0].Args)
}
