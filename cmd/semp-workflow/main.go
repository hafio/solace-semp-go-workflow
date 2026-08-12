// Command semp-workflow is the CLI entry point for SEMP Workflow Automation —
// Ansible-like idempotent playbooks for the Solace SEMP v2 config REST API.
package main

import (
	"os"

	"solace-semp-workflow/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=<version>".
// It defaults to a development marker for `go run` / unversioned builds.
var version = "0.0.0-dev"

func main() {
	os.Exit(cli.Execute(version))
}
