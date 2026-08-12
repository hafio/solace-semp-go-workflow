package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"solace-semp-workflow/internal/models"
	"solace-semp-workflow/internal/modules"
)

// capture redirects Out/ErrOut to buffers for the duration of a test and
// disables ANSI color so assertions match plain text.
func capture(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	origNoColor := color.NoColor
	color.NoColor = true
	origOut, origErr := Out, ErrOut
	var out, errOut bytes.Buffer
	Out, ErrOut = &out, &errOut
	t.Cleanup(func() {
		Out, ErrOut = origOut, origErr
		color.NoColor = origNoColor
	})
	return &out, &errOut
}

func TestPrintTaskResultLabels(t *testing.T) {
	cases := []struct {
		status models.ResultStatus
		label  string
	}{
		{models.StatusOK, "changed"},
		{models.StatusDryRun, "dryrun"},
		{models.StatusSkipped, "skipped"},
		{models.StatusFailed, "FAILED"},
	}
	for _, tc := range cases {
		out, _ := capture(t)
		PrintTaskResult(models.ActionResult{Status: tc.status, TaskName: "Do thing"})
		assert.Contains(t, out.String(), "TASK [Do thing]")
		assert.Contains(t, out.String(), tc.label)
	}
}

func TestPrintTaskResultMessage(t *testing.T) {
	out, _ := capture(t)
	PrintTaskResult(models.ActionResult{Status: models.StatusFailed, TaskName: "T", Message: "boom"})
	s := out.String()
	assert.Contains(t, s, "=> boom")
}

func TestPrintTaskResultFallsBackToModuleName(t *testing.T) {
	out, _ := capture(t)
	PrintTaskResult(models.ActionResult{Status: models.StatusOK, Module: "queue.add"})
	assert.Contains(t, out.String(), "TASK [queue.add]")
}

func TestPrintRecapCountsAndReturn(t *testing.T) {
	out, _ := capture(t)
	results := []models.WorkflowResult{
		{
			WorkflowName: "WF1",
			TaskResults: []models.ActionResult{
				{Status: models.StatusOK},
				{Status: models.StatusSkipped},
				{Status: models.StatusFailed},
			},
		},
	}
	failed := PrintRecap(results)
	assert.True(t, failed)
	s := out.String()
	assert.Contains(t, s, "changed=1")
	assert.Contains(t, s, "skipped=1")
	assert.Contains(t, s, "failed=1")
	assert.Contains(t, s, "Some tasks failed!")
}

func TestPrintRecapAllSuccess(t *testing.T) {
	out, _ := capture(t)
	results := []models.WorkflowResult{
		{WorkflowName: "WF1", TaskResults: []models.ActionResult{{Status: models.StatusOK}}},
	}
	failed := PrintRecap(results)
	assert.False(t, failed)
	assert.Contains(t, out.String(), "All tasks completed successfully.")
}

func TestPrintModuleList(t *testing.T) {
	out, _ := capture(t)
	PrintModuleList([]string{"queue.add", "queue.delete", "rdp.add"})
	s := out.String()
	assert.Contains(t, s, "queue")
	assert.Contains(t, s, "- queue.add")
	assert.Contains(t, s, "- queue.delete")
	assert.Contains(t, s, "- rdp.add")
}

func TestPrintError(t *testing.T) {
	_, errOut := capture(t)
	PrintError("something failed")
	assert.Contains(t, errOut.String(), "ERROR: something failed")
}

// docsFixture is a minimal module set covering the two shapes the renderer
// branches on: an object prefix containing an underscore (to exercise the TOC
// anchor rule) and an action carrying parameters. It is deliberately local
// rather than modules.Info(), so this test exercises the renderer and does not
// silently re-test whichever modules happen to be registered.
func docsFixture() map[string]modules.ModuleInfo {
	return map[string]modules.ModuleInfo{
		"queue.add": {
			Description: "Create a queue.",
			Params: []modules.ParamSpec{
				{Name: "queueName", Type: "string", Required: true, Description: "Name of the queue"},
			},
		},
		"acl_publish_exception.add": {
			Description: "Create a publish topic exception.",
			Params: []modules.ParamSpec{
				{Name: "aclProfileName", Type: "string", Required: true, Description: "Owning ACL profile"},
			},
		},
	}
}

func TestRenderModuleDocsMDStructure(t *testing.T) {
	md := RenderModuleDocsMD(docsFixture())

	// Title + legend.
	assert.True(t, strings.HasPrefix(md, "# SEMP Workflow Automation - Module Reference"))
	assert.Contains(t, md, "All actions are **idempotent**")

	// TOC anchor: object prefix underscores become dashes.
	assert.Contains(t, md, "[acl_publish_exception](#acl-publish-exception)")

	// Section headers and per-action headings.
	assert.Contains(t, md, "## queue")
	assert.Contains(t, md, "### `queue.add`")

	// Param table header rendered for a module with params.
	assert.Contains(t, md, "| Parameter | Type | Required | Default | Description |")
}

func TestRenderModuleDocsMDMasksSensitiveDefault(t *testing.T) {
	md := RenderModuleDocsMD(map[string]modules.ModuleInfo{
		"client_username.add": {
			Description: "Create a client username.",
			Params: []modules.ParamSpec{
				{Name: "password", Type: "string", Default: "hunter2", Description: "The password.", Sensitive: true},
				{Name: "enabled", Type: "boolean", Default: true, Description: "Enable the username."},
			},
		},
	})

	// The real default never reaches the document; the row is marked sensitive.
	assert.NotContains(t, md, "hunter2")
	assert.Contains(t, md, "| `password` | string | No | `"+modules.Mask+"` |")
	assert.Contains(t, md, "Sensitive: never echoed in output or docs.")

	// A non-sensitive parameter still shows its real default.
	assert.Contains(t, md, "| `enabled` | boolean | No | `True` |")
}

func TestPrintWorkflowHeaderMasksSecrets(t *testing.T) {
	out, _ := capture(t)
	PrintWorkflowHeader("wf", "tmpl.Ref", map[string]any{
		"rc_auth_password": "hunter2",
		"bearer_token":     "abc123",
		"queue_name":       "orders",
	}, 1)
	s := out.String()

	assert.NotContains(t, s, "hunter2")
	assert.NotContains(t, s, "abc123")
	assert.Contains(t, s, "bearer_token="+modules.Mask)
	assert.Contains(t, s, "rc_auth_password="+modules.Mask)
	// Non-secret inputs are still shown, so the header stays useful.
	assert.Contains(t, s, "queue_name=orders")
}

func TestIsSensitiveName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"rc_auth_password", true},
		{"PASSWORD", true},
		{"passwd", true},
		{"ssh_passphrase", true},
		{"clientSecret", true},
		{"bearer_token", true},
		{"aws_credential", true},
		{"api_key", true},
		{"apiKey", true},
		{"private_key", true},
		{"privateKey", true},
		{"queue_name", false},
		{"vpn", false},
		{"keystore_path", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsSensitiveName(tc.name))
		})
	}
}
