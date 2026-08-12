// Package templates embeds the bundled workflow templates into the binary.
// The embedded files are the fallback template source when no templates
// directory is available on disk.
package templates

import "embed"

//go:embed *.yaml
var bundled embed.FS

// FS returns the embedded bundled templates as a read-only file system whose
// root contains the *.yaml template files.
func FS() embed.FS {
	return bundled
}
