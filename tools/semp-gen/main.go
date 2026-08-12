// Command semp-gen generates the SEMP action module descriptors from the
// committed specifications in specs/.
//
// It is a development tool, not part of the shipped binary: cmd/semp-workflow
// never reads a specification. Output is committed Go source so a spec bump is a
// reviewable diff, and the `gen` dev-script task fails CI when the checked-in
// files no longer match the specs.
//
// Usage:
//
//	go run ./tools/semp-gen -software <spec.json> -appliance <spec.json> -out internal/modules
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"solace-semp-workflow/tools/semp-gen/internal/ir"
	"solace-semp-workflow/tools/semp-gen/internal/render"
	"solace-semp-workflow/tools/semp-gen/internal/specmodel"
)

func main() {
	software := flag.String("software", "specs/software/10.26.0.8755/semp-v2-config.json",
		"Path to the software-broker SEMP v2 config specification.")
	appliance := flag.String("appliance", "specs/appliance/10.26.0.8755/semp-v2-config.json",
		"Path to the appliance SEMP v2 config specification.")
	out := flag.String("out", "internal/modules", "Directory to write the generated descriptors to.")
	check := flag.Bool("check", false,
		"Report what would change without writing, for a drift gate.")
	flag.Parse()

	if err := run(*software, *appliance, *out, *check); err != nil {
		fmt.Fprintf(os.Stderr, "semp-gen: %v\n", err)
		os.Exit(1)
	}
}

func run(softwarePath, appliancePath, outDir string, check bool) error {
	sw, err := specmodel.Load(softwarePath)
	if err != nil {
		return err
	}
	ap, err := specmodel.Load(appliancePath)
	if err != nil {
		return err
	}
	if sw.Info.Version != ap.Info.Version {
		return fmt.Errorf("specification versions differ (software %s, appliance %s): generating from "+
			"mismatched broker versions would merge two different APIs", sw.Info.Version, ap.Info.Version)
	}

	spec, err := ir.Extract(sw, ap)
	if err != nil {
		return err
	}
	files, err := render.All(spec)
	if err != nil {
		return err
	}

	// Every descriptor is regenerated from scratch, so a stale file from an object
	// the specification has removed must be deleted rather than left behind
	// registering an action that no longer exists.
	stale, err := staleFiles(outDir, files)
	if err != nil {
		return err
	}

	if check {
		return reportDrift(outDir, files, stale)
	}

	for _, f := range files {
		path := filepath.Join(outDir, f.Name)
		if err := os.WriteFile(path, f.Content, 0o644); err != nil { //nolint:gosec // generated source, world-readable by intent
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	for _, name := range stale {
		if err := os.Remove(filepath.Join(outDir, name)); err != nil {
			return fmt.Errorf("remove stale %s: %w", name, err)
		}
	}

	fmt.Printf("semp-gen: SEMP %s -> %d objects, %d actions, %d files written",
		spec.Version, len(spec.Objects), actionCount(spec), len(files))
	if len(stale) > 0 {
		fmt.Printf(", %d stale removed", len(stale))
	}
	fmt.Println()
	fmt.Println("  " + summary(spec))
	return nil
}

// staleFiles lists gen_*.go files in outDir that the current specification no
// longer produces.
func staleFiles(outDir string, files []render.File) ([]string, error) {
	want := map[string]bool{}
	for _, f := range files {
		want[f.Name] = true
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", outDir, err)
	}
	var stale []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "gen_") || !strings.HasSuffix(name, ".go") {
			continue
		}
		if !want[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	return stale, nil
}

// reportDrift compares the checked-in files with what the specs produce now.
func reportDrift(outDir string, files []render.File, stale []string) error {
	var changed, missing []string
	for _, f := range files {
		existing, err := os.ReadFile(filepath.Join(outDir, f.Name)) //nolint:gosec // path from generated names
		if os.IsNotExist(err) {
			missing = append(missing, f.Name)
			continue
		}
		if err != nil {
			return err
		}
		if string(existing) != string(f.Content) {
			changed = append(changed, f.Name)
		}
	}
	if len(changed) == 0 && len(missing) == 0 && len(stale) == 0 {
		fmt.Println("semp-gen: generated descriptors are up to date")
		return nil
	}
	var msg []string
	if len(missing) > 0 {
		msg = append(msg, fmt.Sprintf("%d missing (%s)", len(missing), strings.Join(truncate(missing), ", ")))
	}
	if len(changed) > 0 {
		msg = append(msg, fmt.Sprintf("%d out of date (%s)", len(changed), strings.Join(truncate(changed), ", ")))
	}
	if len(stale) > 0 {
		msg = append(msg, fmt.Sprintf("%d stale (%s)", len(stale), strings.Join(truncate(stale), ", ")))
	}
	return fmt.Errorf("generated descriptors do not match specs/: %s\n  run the `gen` dev task and commit the result",
		strings.Join(msg, "; "))
}

func truncate(in []string) []string {
	if len(in) <= 5 {
		return in
	}
	return append(in[:5:5], "...")
}

func actionCount(spec *ir.Spec) int {
	n := 0
	for _, o := range spec.Objects {
		if o.IsSingleton {
			n++
			continue
		}
		n += 2
		if o.HasUpdate {
			n++
		}
	}
	return n
}

func summary(spec *ir.Spec) string {
	attrs, vpn, global, deprecated := 0, 0, 0, 0
	for _, o := range spec.Objects {
		attrs += len(o.Attributes)
		if o.Scope == ir.ScopeVPN {
			vpn++
		} else {
			global++
		}
		if o.Deprecated != "" {
			deprecated++
		}
	}
	return fmt.Sprintf("%d attributes; %d VPN-scoped, %d broker-global; %d objects fully deprecated",
		attrs, vpn, global, deprecated)
}
