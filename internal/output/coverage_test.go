package output

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"solace-semp-workflow/internal/modules"
)

func TestPrintBanner(t *testing.T) {
	out, _ := capture(t)
	PrintBanner()
	s := out.String()
	assert.Contains(t, s, "SEMP Workflow Automation")
	assert.Contains(t, s, SEPARATOR)
}

func TestPrintWorkflowHeaderWithInputs(t *testing.T) {
	out, _ := capture(t)
	// Keys deliberately out of order to prove they are rendered sorted.
	inputs := map[string]any{"zebra": "z", "alpha": 1}
	PrintWorkflowHeader("wf-name", "tmpl.Ref", inputs, 3)
	s := out.String()
	assert.Contains(t, s, "PLAY 3 [wf-name] (tmpl.Ref)")
	assert.Contains(t, s, "Inputs: alpha=1, zebra=z")
	assert.Contains(t, s, TASK_SEP)
}

func TestPrintWorkflowHeaderNoInputs(t *testing.T) {
	out, _ := capture(t)
	PrintWorkflowHeader("wf-name", "tmpl.Ref", nil, 1)
	s := out.String()
	assert.Contains(t, s, "PLAY 1 [wf-name] (tmpl.Ref)")
	assert.NotContains(t, s, "Inputs:")
}

func TestPrintDryRunBanner(t *testing.T) {
	out, _ := capture(t)
	PrintDryRunBanner()
	s := out.String()
	assert.Contains(t, s, "DRY RUN MODE")
	assert.Contains(t, s, "No changes will be made")
}

func TestPrintValidationOK(t *testing.T) {
	out, _ := capture(t)
	PrintValidationOK("/path/to/config.yaml", 3, 5)
	s := out.String()
	assert.Contains(t, s, "Validation passed!")
	assert.Contains(t, s, "Config: /path/to/config.yaml")
	assert.Contains(t, s, "Templates loaded: 3")
	assert.Contains(t, s, "Workflows defined: 5")
}

func TestRenderModuleDocsMDEnumAndNoParams(t *testing.T) {
	info := map[string]modules.ModuleInfo{
		"thing.withenum": {
			Description: "Configure a thing.",
			Params: []modules.ParamSpec{
				{
					Name:        "mode",
					Description: "Failover mode",
					Enum:        []string{"active", "standby"},
				},
			},
		},
		"thing.noparams": {
			Description: "A thing with no parameters.",
		},
	}

	md := RenderModuleDocsMD(info)

	// Enum values rendered backticked and joined with ", ", appended to the
	// existing description.
	assert.Contains(t, md, "Failover mode (`active`, `standby`)")
	// Param row for the enum action, with defaulted type/required/default cells.
	assert.Contains(t, md, "| `mode` | string | No | - |")
	// No-params action renders the placeholder line.
	assert.Contains(t, md, "_No parameters._")
}
