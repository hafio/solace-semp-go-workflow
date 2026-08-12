// Package cli wires the cobra commands (run, validate, list-modules, init) and
// maps their outcomes to process exit codes: 0 success, 1 workflow failures,
// 2 config/template errors, 130 interrupt.
package cli

import (
	stderrors "errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sort"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"solace-semp-workflow/internal/config"
	wferrors "solace-semp-workflow/internal/errors"
	"solace-semp-workflow/internal/engine"
	"solace-semp-workflow/internal/modules"
	"solace-semp-workflow/internal/output"
	"solace-semp-workflow/templates"
)

// Execute builds the command tree, installs an interrupt handler, and runs the
// CLI, returning the process exit code.
func Execute(version string) int {
	// Ctrl-C -> print and exit 130.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		output.PrintError("Interrupted by user")
		os.Exit(130)
	}()

	code := 0
	root := newRootCmd(version, &code)
	if err := root.Execute(); err != nil {
		// Flag/usage error — cobra has already printed it. Click uses exit 2 for
		// usage errors.
		return 2
	}
	return code
}

func newRootCmd(version string, code *int) *cobra.Command {
	root := &cobra.Command{
		Use:           "semp-workflow",
		Short:         "SEMP Workflow Automation - Ansible-like playbooks for Solace SEMP.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate("semp-workflow, version {{.Version}}\n")
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(newRunCmd(code))
	root.AddCommand(newValidateCmd(code))
	root.AddCommand(newListModulesCmd())
	root.AddCommand(newInitCmd(code))
	return root
}

func newRunCmd(code *int) *cobra.Command {
	var (
		configPath   string
		templatesDir string
		dryRun       bool
		check        bool
		failFast     bool
		verbose      bool
	)
	cmd := &cobra.Command{
		Use:           "run",
		Short:         "Execute workflows defined in a config file.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			_ = verbose // accepted for CLI compatibility; Go modules do not emit debug logs

			appConfig, err := config.LoadConfig(configPath)
			if err != nil {
				*code = classifyExit(err)
				output.PrintError(err.Error())
				return nil
			}

			if templatesDir != "" {
				// Explicit --templates-dir always wins; disable bundled fallback.
				appConfig.TemplatesDir = templatesDir
				appConfig.UseBundledTemplates = false
			}

			eng, err := engine.NewEngine(appConfig, dryRun || check, failFast)
			if err != nil {
				*code = classifyExit(err)
				output.PrintError(err.Error())
				return nil
			}

			results := eng.Run()
			for i := range results {
				if results[i].HasFailures() {
					*code = 1
					break
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config YAML file.")
	cmd.Flags().StringVarP(&templatesDir, "templates-dir", "t", "", "Override the workflow templates directory.")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes.")
	cmd.Flags().BoolVar(&check, "check", false, "Alias for --dry-run.")
	cmd.Flags().BoolVarP(&failFast, "fail-fast", "f", false, "Stop execution on first failure.")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose/debug logging.")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func newValidateCmd(code *int) *cobra.Command {
	var (
		configPath   string
		templatesDir string
	)
	cmd := &cobra.Command{
		Use:           "validate",
		Short:         "Validate config and templates without executing.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			appConfig, err := config.LoadConfig(configPath)
			if err != nil {
				*code = classifyExit(err)
				output.PrintError(err.Error())
				return nil
			}

			if templatesDir != "" {
				appConfig.TemplatesDir = templatesDir
				appConfig.UseBundledTemplates = false
			}

			var tmpls map[string]*config.WorkflowTemplate
			if appConfig.UseBundledTemplates {
				tmpls, err = config.LoadTemplatesFS(templates.FS())
			} else {
				tmpls, err = config.LoadTemplatesDir(appConfig.TemplatesDir)
			}
			if err != nil {
				*code = classifyExit(err)
				output.PrintError(err.Error())
				return nil
			}

			for i, wf := range appConfig.Workflows {
				if _, ok := tmpls[wf.Template]; !ok {
					available := sortedKeys(tmpls)
					output.PrintError(fmt.Sprintf("Workflow %d: template '%s' not found. Available: %s",
						i+1, wf.Template, joinComma(available)))
					*code = 2
					return nil
				}
			}

			output.PrintValidationOK(configPath, len(tmpls), len(appConfig.Workflows))
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config YAML file.")
	cmd.Flags().StringVarP(&templatesDir, "templates-dir", "t", "", "Override the workflow templates directory.")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func newListModulesCmd() *cobra.Command {
	var outputFile string
	cmd := &cobra.Command{
		Use:           "list-modules",
		Short:         "List all available action modules.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			output.PrintModuleList(modules.List())
			if outputFile != "" {
				md := output.RenderModuleDocsMD(modules.Info())
				if err := os.WriteFile(outputFile, []byte(md), 0o644); err != nil {
					output.PrintError(fmt.Sprintf("Failed to write module reference: %s", err))
					return nil
				}
				fmt.Fprintf(output.Out, "Module reference written to: %s\n", outputFile)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write module reference docs to a Markdown file (e.g. all-modules.md).")
	return cmd
}

func newInitCmd(code *int) *cobra.Command {
	var (
		outputDir string
		force     bool
	)
	cmd := &cobra.Command{
		Use:           "init",
		Short:         "Copy bundled workflow templates to a local directory.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			fsys := templates.FS()
			names, err := fs.Glob(fsys, "*.yaml")
			if err != nil || len(names) == 0 {
				output.PrintError("No bundled templates found.")
				*code = 2
				return nil
			}
			sort.Strings(names)

			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				output.PrintError(fmt.Sprintf("Failed to create output directory: %s", err))
				*code = 2
				return nil
			}

			cyan := color.New(color.FgCyan)
			green := color.New(color.FgGreen)
			copied, skipped := 0, 0
			for _, name := range names {
				target := filepath.Join(outputDir, name)
				if _, statErr := os.Stat(target); statErr == nil && !force {
					fmt.Fprintf(output.Out, "  %s  %s  (already exists, use --force to overwrite)\n", cyan.Sprint("skip"), target)
					skipped++
					continue
				}
				content, readErr := fs.ReadFile(fsys, name)
				if readErr != nil {
					output.PrintError(fmt.Sprintf("Failed to read bundled template '%s': %s", name, readErr))
					*code = 2
					return nil
				}
				if writeErr := os.WriteFile(target, content, 0o644); writeErr != nil {
					output.PrintError(fmt.Sprintf("Failed to write '%s': %s", target, writeErr))
					*code = 2
					return nil
				}
				fmt.Fprintf(output.Out, "  %s %s\n", green.Sprint("write"), target)
				copied++
			}

			abs, _ := filepath.Abs(outputDir)
			fmt.Fprintf(output.Out, "\n  %d file(s) written, %d skipped -> %s\n", copied, skipped, abs)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "templates", "Directory to copy bundled templates into.")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing files.")
	return cmd
}

// classifyExit maps a workflow error to the process exit code: config and
// template errors are usage-level (2); any other workflow error is a run
// failure (1).
func classifyExit(err error) int {
	var ce *wferrors.ConfigError
	var te *wferrors.TemplateError
	if stderrors.As(err, &ce) || stderrors.As(err, &te) {
		return 2
	}
	return 1
}

func sortedKeys(m map[string]*config.WorkflowTemplate) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func joinComma(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
