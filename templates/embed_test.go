package templates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFSContainsBundledTemplates covers the FS accessor and verifies the
// embedded file system exposes the bundled workflow templates with content.
func TestFSContainsBundledTemplates(t *testing.T) {
	data, err := FS().ReadFile("app-inbound.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, data)
}
