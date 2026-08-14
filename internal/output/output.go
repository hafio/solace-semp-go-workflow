// Package output renders Ansible-style colored console output and the module
// reference Markdown. Color is handled by fatih/color, which honors NO_COLOR and
// non-TTY output so captured logs stay clean.
package output

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"

	"solace-semp-workflow/internal/models"
	"solace-semp-workflow/internal/modules"
	"solace-semp-workflow/internal/semp"
)

// Out and ErrOut are the destinations for normal and error output. They are
// package-level so tests can capture output; production uses stdout/stderr.
var (
	Out    io.Writer = os.Stdout
	ErrOut io.Writer = os.Stderr
)

// SEPARATOR and TASK_SEP are the horizontal rules used in the output.
var (
	SEPARATOR = strings.Repeat("=", 70)
	TASK_SEP  = strings.Repeat("-", 70)
)

var (
	cBold   = color.New(color.Bold)
	cYellow = color.New(color.FgYellow, color.Bold)
	cCyan   = color.New(color.FgCyan, color.Bold)
	cRed    = color.New(color.FgRed, color.Bold)
	cGreen  = color.New(color.FgGreen, color.Bold)
)

// PrintBanner prints the top-of-run banner.
func PrintBanner() {
	fmt.Fprintf(Out, "\n%s\n", cBold.Sprint(SEPARATOR))
	fmt.Fprintln(Out, "  SEMP Workflow Automation")
	fmt.Fprintf(Out, "%s\n\n", cBold.Sprint(SEPARATOR))
}

// PrintWorkflowHeader prints the PLAY header for a workflow.
func PrintWorkflowHeader(workflowName, templateRef string, inputs map[string]any, index int) {
	fmt.Fprintf(Out, "\n%s\n", cBold.Sprintf("PLAY %d [%s] (%s)", index, workflowName, templateRef))
	if len(inputs) > 0 {
		keys := make([]string, 0, len(inputs))
		for k := range inputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			if IsSensitiveName(k) {
				parts = append(parts, fmt.Sprintf("%s=%s", k, modules.Mask))
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%s", k, semp.Stringify(inputs[k])))
		}
		fmt.Fprintf(Out, "  Inputs: %s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintln(Out, TASK_SEP)
}

// PrintTaskResult prints a single task result with a color-coded status.
func PrintTaskResult(result models.ActionResult) {
	name := result.TaskName
	if name == "" {
		name = result.Module
	}

	var col *color.Color
	var label string
	switch result.Status {
	case models.StatusOK:
		col, label = cYellow, "changed"
	case models.StatusDryRun:
		col, label = cCyan, "dryrun"
	case models.StatusSkipped:
		col, label = cCyan, "skipped"
	default:
		col, label = cRed, "FAILED"
	}

	taskDisplay := fmt.Sprintf("TASK [%s]", name)
	padding := 55 - len(taskDisplay)
	if padding < 1 {
		padding = 1
	}
	dots := strings.Repeat(".", padding)

	fmt.Fprintf(Out, "  %s %s %s\n", taskDisplay, dots, col.Sprint(label))
	if result.Message != "" {
		if result.Status == models.StatusFailed {
			fmt.Fprintf(Out, "    => %s\n", color.New(color.FgRed).Sprint(result.Message))
		} else {
			fmt.Fprintf(Out, "    => %s\n", result.Message)
		}
	}
}

// PrintDryRunBanner announces dry-run mode.
func PrintDryRunBanner() {
	fmt.Fprintf(Out, "\n%s\n", cCyan.Sprint("** DRY RUN MODE ** No changes will be made"))
}

// PrintRecap prints the final recap summary and reports whether any task failed.
func PrintRecap(results []models.WorkflowResult) bool {
	fmt.Fprintf(Out, "\n%s\n", cBold.Sprint(SEPARATOR))
	fmt.Fprintln(Out, cBold.Sprint("RECAP"))
	fmt.Fprintln(Out, SEPARATOR)

	totalOK, totalDryRun, totalSkipped, totalFailed := 0, 0, 0, 0

	for i := range results {
		wf := &results[i]
		okc := wf.OKCount()
		dryc := wf.DryRunCount()
		skpc := wf.SkippedCount()
		fldc := wf.FailedCount()

		totalOK += okc
		totalDryRun += dryc
		totalSkipped += skpc
		totalFailed += fldc

		failedStr := fmt.Sprintf("failed=%d", fldc)
		if fldc > 0 {
			failedStr = cRed.Sprintf("failed=%d", fldc)
		}

		fmt.Fprintf(Out, "  Workflow %d (%s): %s  %s  %s  %s\n",
			i+1, wf.WorkflowName,
			cYellow.Sprintf("changed=%d", okc),
			cCyan.Sprintf("dryrun=%d", dryc),
			cCyan.Sprintf("skipped=%d", skpc),
			failedStr,
		)
	}

	fmt.Fprintln(Out, TASK_SEP)
	totalFailedStr := fmt.Sprintf("failed=%d", totalFailed)
	if totalFailed > 0 {
		totalFailedStr = cRed.Sprintf("failed=%d", totalFailed)
	}
	fmt.Fprintf(Out, "  %s %s  %s  %s  %s\n",
		cBold.Sprint("Total:"),
		cYellow.Sprintf("changed=%d", totalOK),
		cCyan.Sprintf("dryrun=%d", totalDryRun),
		cCyan.Sprintf("skipped=%d", totalSkipped),
		totalFailedStr,
	)
	fmt.Fprintln(Out, SEPARATOR)

	if totalFailed > 0 {
		fmt.Fprintf(Out, "\n%s\n", cRed.Sprint("Some tasks failed!"))
		return true
	}
	fmt.Fprintf(Out, "\n%s\n", cGreen.Sprint("All tasks completed successfully."))
	return false
}

// PrintModuleList prints all available modules grouped by object prefix.
func PrintModuleList(names []string) {
	fmt.Fprintf(Out, "\n%s\n\n", cBold.Sprint("Available Modules:"))

	groups := map[string][]string{}
	for _, name := range names {
		obj, verb := splitAction(name)
		groups[obj] = append(groups[obj], verb)
	}

	for _, obj := range sortedKeys(groups) {
		fmt.Fprintf(Out, "  %s\n", cBold.Sprint(obj))
		verbs := groups[obj]
		sort.Strings(verbs)
		for _, verb := range verbs {
			fmt.Fprintf(Out, "    - %s.%s\n", obj, verb)
		}
		fmt.Fprintln(Out)
	}
}

// PrintValidationOK prints the validation success summary.
func PrintValidationOK(configPath string, templateCount, workflowCount int) {
	fmt.Fprintf(Out, "\n%s\n", cGreen.Sprint("Validation passed!"))
	fmt.Fprintf(Out, "  Config: %s\n", configPath)
	fmt.Fprintf(Out, "  Templates loaded: %d\n", templateCount)
	fmt.Fprintf(Out, "  Workflows defined: %d\n", workflowCount)
}

// PrintError prints an error message to stderr.
func PrintError(message string) {
	fmt.Fprintf(ErrOut, "\n%s\n", cRed.Sprintf("ERROR: %s", message))
}

// secretNameFragments are the substrings that mark a workflow input as carrying
// a credential. Workflow inputs are declared per template and carry no type
// metadata, so the name is the only signal available; matching is deliberately
// broad because over-masking display output is harmless while under-masking
// writes a live password to the console.
var secretNameFragments = []string{
	"password", "passwd", "passphrase", "secret", "token",
	"credential", "api_key", "apikey", "private_key", "privatekey",
}

// IsSensitiveName reports whether an input name looks like a credential and so
// must be masked wherever it would otherwise be echoed.
func IsSensitiveName(name string) bool {
	lower := strings.ToLower(name)
	for _, frag := range secretNameFragments {
		if strings.Contains(lower, frag) {
			return true
		}
	}
	return false
}

// RenderModuleDocsMD renders all module metadata as a Markdown document.
func RenderModuleDocsMD(moduleInfo map[string]modules.ModuleInfo) string {
	var lines []string
	add := func(s string) { lines = append(lines, s) }

	add("# SEMP Workflow Automation - Module Reference")
	add("")
	add("All actions are **idempotent**: each checks current state before acting.")
	add("Result states: `changed` (action ran), `skipped` (already in desired state), `dryrun` (would change), `failed` (error).")
	add("")

	groups := groupByObject(moduleInfo)

	// Table of contents.
	add("## Contents")
	add("")
	for _, obj := range sortedKeys(groups) {
		anchor := strings.ReplaceAll(obj, "_", "-")
		add(fmt.Sprintf("- [%s](#%s)", obj, anchor))
	}
	add("")
	add("---")
	add("")

	for _, obj := range sortedKeys(groups) {
		add(fmt.Sprintf("## %s", obj))
		add("")

		for _, actionName := range groups[obj] {
			renderActionSection(add, "###", actionName, moduleInfo[actionName])
		}

		add("---")
		add("")
	}

	return strings.Join(lines, "\n")
}

// groupByObject buckets action names by their object prefix, each bucket sorted
// so a rendered document is stable between runs.
func groupByObject(moduleInfo map[string]modules.ModuleInfo) map[string][]string {
	groups := map[string][]string{}
	for name := range moduleInfo {
		obj, _ := splitAction(name)
		groups[obj] = append(groups[obj], name)
	}
	for _, names := range groups {
		sort.Strings(names)
	}
	return groups
}

// renderActionSection renders one action's heading, description and parameter
// table at the given heading level. Shared by the single-document, per-family and
// index renderers so the three cannot drift apart.
func renderActionSection(add func(string), level, actionName string, info modules.ModuleInfo) {
	add(fmt.Sprintf("%s `%s`", level, actionName))
	add("")
	if info.Description != "" {
		add(info.Description)
		add("")
	}

	if len(info.Params) == 0 {
		add("_No parameters._")
		add("")
		return
	}

	add("| Parameter | Type | Required | Default | Description |")
	add("|-----------|------|----------|---------|-------------|")
	for _, p := range info.Params {
		ptype := p.Type
		if ptype == "" {
			ptype = "string"
		}
		required := "No"
		if p.Required {
			required = "Yes"
		}
		def := "-"
		if p.Default != nil {
			def = fmt.Sprintf("`%s`", semp.Stringify(p.Default))
			if p.Sensitive {
				def = fmt.Sprintf("`%s`", modules.Mask)
			}
		}
		desc := p.Description
		if p.Sensitive {
			desc = strings.TrimSpace(desc + " Sensitive: never echoed in output or docs.")
		}
		if len(p.Enum) > 0 {
			allowed := make([]string, len(p.Enum))
			for i, v := range p.Enum {
				allowed[i] = fmt.Sprintf("`%s`", v)
			}
			joined := strings.Join(allowed, ", ")
			if desc != "" {
				desc = fmt.Sprintf("%s (%s)", desc, joined)
			} else {
				desc = joined
			}
		}
		add(fmt.Sprintf("| `%s` | %s | %s | %s | %s |", p.Name, ptype, required, def, mdCell(desc)))
	}
	add("")
}

// mdCell escapes a value for a Markdown table cell. Attribute descriptions come
// from the SEMP specification, so a pipe in one would silently corrupt the table
// for every row that follows. None carry one today; this is what stops a future
// broker release from breaking the reference without anyone noticing.
func mdCell(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// ModuleFamilies returns the object families present in moduleInfo, sorted.
func ModuleFamilies(moduleInfo map[string]modules.ModuleInfo) []string {
	return sortedKeys(groupByObject(moduleInfo))
}

// FamilyFileName is the per-family reference filename for an object prefix.
func FamilyFileName(family string) string { return family + ".md" }

// RenderModuleDocsIndexMD renders the index of the per-family reference. The
// full reference is split because a single document covering every action would
// run past ten thousand lines, which no one reads and every diff buries.
func RenderModuleDocsIndexMD(moduleInfo map[string]modules.ModuleInfo) string {
	var lines []string
	add := func(s string) { lines = append(lines, s) }

	groups := groupByObject(moduleInfo)
	families := sortedKeys(groups)

	add("# SEMP Workflow Automation - Module Reference")
	add("")
	add(fmt.Sprintf("%d actions across %d object families.", len(moduleInfo), len(families)))
	add("")
	add("All actions are **idempotent**: each checks current state before acting.")
	add("Result states: `changed` (action ran), `skipped` (already in desired state), `dryrun` (would change), `failed` (error).")
	add("")
	add("An `update` compares the broker's current state and reports `skipped` when the")
	add("object already matches, so re-running a workflow reports no change. Attributes")
	add("the broker never returns, such as passwords, cannot be compared and are re-sent.")
	add("")
	add("Generated by `semp-workflow list-modules --output-dir`. Do not edit by hand.")
	add("")
	add("| Family | Actions | Reference |")
	add("|--------|---------|-----------|")
	for _, family := range families {
		verbs := make([]string, 0, len(groups[family]))
		for _, action := range groups[family] {
			_, verb := splitAction(action)
			verbs = append(verbs, verb)
		}
		add(fmt.Sprintf("| `%s` | %s | [%s](%s) |",
			family, strings.Join(verbs, ", "), family, FamilyFileName(family)))
	}
	add("")

	return strings.Join(lines, "\n")
}

// RenderModuleDocsFamilyMD renders the reference page for one object family.
func RenderModuleDocsFamilyMD(family string, moduleInfo map[string]modules.ModuleInfo) string {
	var lines []string
	add := func(s string) { lines = append(lines, s) }

	add(fmt.Sprintf("# `%s`", family))
	add("")
	add("Generated by `semp-workflow list-modules --output-dir`. Do not edit by hand.")
	add("")
	add("[Back to the index](index.md)")
	add("")

	for _, actionName := range groupByObject(moduleInfo)[family] {
		renderActionSection(add, "##", actionName, moduleInfo[actionName])
	}

	return strings.Join(lines, "\n")
}

// splitAction splits an "object.verb" action name into its two parts.
func splitAction(name string) (obj, verb string) {
	parts := strings.SplitN(name, ".", 2)
	obj = parts[0]
	if len(parts) > 1 {
		verb = parts[1]
	}
	return obj, verb
}

// sortedKeys returns the sorted keys of a group map.
func sortedKeys(groups map[string][]string) []string {
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
