// Package modules implements the idempotent SEMP action modules and their
// registry. Each module file registers its actions via init(), which the
// registry collects at startup.
package modules

import (
	stderrors "errors"

	wferrors "solace-semp-workflow/internal/errors"
	"solace-semp-workflow/internal/models"
	"solace-semp-workflow/internal/semp"
)

// Client is the subset of the SEMP client that action modules depend on. The
// concrete *semp.Client satisfies it structurally in production; tests inject a
// fake that implements the same interface.
//
// Paths are relative to /SEMP/v2/config, so VPN-scoped objects prepend their own
// msgVpns/{vpn} hop, broker-global objects address the root, and the empty path
// addresses the Broker singleton.
type Client interface {
	Exists(path string) (bool, map[string]any, error)
	Create(path string, payload map[string]any) (map[string]any, error)
	Update(path string, payload map[string]any) (map[string]any, error)
	Delete(path string) error
	// DefaultMsgVPN is the VPN from the connection config, used by VPN-scoped
	// actions that do not name one explicitly.
	DefaultMsgVPN() string
}

// ParamSpec describes a single module parameter. It is stored as an ordered
// slice (Go maps do not preserve insertion order) so generated module
// documentation is stable.
type ParamSpec struct {
	Name        string
	Type        string
	Required    bool
	Default     any
	Description string
	Enum        []string
	// Sensitive marks a parameter whose value must never be echoed: passwords,
	// shared secrets and any attribute the SEMP specification flags Write-Only
	// or Opaque. Console output and generated docs render Mask in its place.
	Sensitive bool
}

// Mask is substituted for the value of any Sensitive parameter in console
// output and generated documentation.
const Mask = "***"

// Module is a single idempotent SEMP action.
type Module interface {
	// Execute runs the action idempotently and returns its result.
	Execute(c Client, args map[string]any, dryRun bool) models.ActionResult
	// Description returns the one-line description used in docs and listings.
	Description() string
	// Params returns the ordered parameter schema used for documentation.
	Params() []ParamSpec
}

// argStr returns args[key] rendered as a string via Stringify, or def when the
// key is absent or nil.
func argStr(args map[string]any, key, def string) string {
	if v, ok := args[key]; ok && v != nil {
		return semp.Stringify(v)
	}
	return def
}

// sempCodeOf returns the SEMP error code carried by err, or 0 if err is not a
// *SEMPError.
func sempCodeOf(err error) int {
	var se *wferrors.SEMPError
	if stderrors.As(err, &se) {
		return se.SempCode
	}
	return 0
}

// failed is a small constructor for a FAILED ActionResult.
func failed(msg string) models.ActionResult {
	return models.NewActionResult(models.StatusFailed, msg)
}

// ok is a small constructor for an OK ActionResult.
func ok(msg string) models.ActionResult {
	return models.NewActionResult(models.StatusOK, msg)
}

// skipped is a small constructor for a SKIPPED ActionResult.
func skipped(msg string) models.ActionResult {
	return models.NewActionResult(models.StatusSkipped, msg)
}

// dryrun is a small constructor for a DRYRUN ActionResult.
func dryrun(msg string) models.ActionResult {
	return models.NewActionResult(models.StatusDryRun, msg)
}
