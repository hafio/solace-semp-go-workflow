package ir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/tools/semp-gen/internal/specmodel"
)

// specPath locates a committed specification, skipping the test if it is absent.
func specPath(t *testing.T, platform string) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", "..", "specs", platform, "10.26.0.8755", "semp-v2-config.json")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("specification not present at %s", p)
	}
	return p
}

// Extract cannot be exercised with a small fixture: it validates the alias table
// against the specification, so any spec missing the aliased operationIds is
// rejected by design. The real specifications are therefore the fixture, which
// also makes this the one test that proves the whole pipeline works on the actual
// input rather than on something shaped to suit it.
func TestExtractOnTheRealSpecifications(t *testing.T) {
	sw, err := specmodel.Load(specPath(t, "software"))
	require.NoError(t, err)
	ap, err := specmodel.Load(specPath(t, "appliance"))
	require.NoError(t, err)

	spec, err := Extract(sw, ap)
	require.NoError(t, err)

	assert.Equal(t, "10.26.0.8755", spec.Version)
	// The headline surface. A change here is either a broker release or a
	// regression, and both should be noticed deliberately.
	assert.Len(t, spec.Objects, 86)

	actions, singletons, vpn, global := 0, 0, 0, 0
	prefixes := map[string]bool{}
	for _, o := range spec.Objects {
		assert.NotEmptyf(t, o.Prefix, "%s has no prefix", o.Definition)
		assert.NotEmptyf(t, o.FriendlyName, "%s has no friendly name", o.Definition)
		assert.Falsef(t, prefixes[o.Prefix], "prefix %q used twice", o.Prefix)
		prefixes[o.Prefix] = true

		if o.IsSingleton {
			singletons++
			actions++
			continue
		}
		assert.NotEmptyf(t, o.OwnKey, "%s has no key", o.Prefix)
		assert.NotEmptyf(t, o.Collection, "%s has no collection", o.Prefix)
		actions += 2
		if o.HasUpdate {
			actions++
		}
		if o.Scope == ScopeVPN {
			vpn++
		} else {
			global++
		}
	}
	assert.Equal(t, 225, actions)
	assert.Equal(t, 1, singletons, "only the Broker object is a singleton")
	assert.Equal(t, 61, vpn)
	assert.Equal(t, 24, global)

	// Objects are sorted by prefix so a regenerated file set is stable.
	for i := 1; i < len(spec.Objects); i++ {
		assert.LessOrEqual(t, spec.Objects[i-1].Prefix, spec.Objects[i].Prefix)
	}
}

// Every legacy overlay entry must land on a real object, or a
// backward-compatibility guarantee has silently stopped applying.
func TestExtractAppliesEveryLegacyOverlay(t *testing.T) {
	sw, err := specmodel.Load(specPath(t, "software"))
	require.NoError(t, err)
	ap, err := specmodel.Load(specPath(t, "appliance"))
	require.NoError(t, err)

	spec, err := Extract(sw, ap)
	require.NoError(t, err)

	byDef := map[string]Object{}
	for _, o := range spec.Objects {
		byDef[o.Definition] = o
	}
	for _, def := range LegacyDefinitions() {
		o, ok := byDef[def]
		require.Truef(t, ok, "legacy definition %q matched no object", def)
		assert.Equalf(t, legacyOverlays[def].FriendlyName, o.FriendlyName,
			"%s did not take its pinned friendly name", def)
	}

	// Spot-check the two pins that existing templates depend on most directly.
	assert.Equal(t, "smf", byDef["MsgVpnAclProfilePublishTopicException"].OwnKey[0].Default)
	assert.Equal(t, 1, byDef["MsgVpnAclProfilePublishTopicException"].DisplayKey)
	assert.Equal(t, "an", byDef["MsgVpnRestDeliveryPoint"].Article)
	for _, a := range byDef["MsgVpnQueue"].Attributes {
		assert.NotEqual(t, "respectTtlEnabled", a.Name, "a hook-derived attribute must not be settable")
		assert.NotEqual(t, "redeliveryEnabled", a.Name, "a hook-derived attribute must not be settable")
	}
}

// The overlay is keyed by definition precisely so it survives a change to how
// prefixes are derived. This pins that: every key must be a SEMP definition name
// (CamelCase), never a registry prefix (snake_case).
func TestLegacyOverlaysAreKeyedByDefinitionNotPrefix(t *testing.T) {
	for def := range legacyOverlays {
		assert.NotContainsf(t, def, "_",
			"%q looks like a registry prefix; the overlay is keyed by SEMP definition name", def)
	}
}

func TestKindRuntimeConstMatchesTheRuntimeIdentifiers(t *testing.T) {
	// These strings are emitted into generated Go, so a wrong one is a build
	// failure in the generated package rather than here.
	assert.Equal(t, "kindString", KindString.RuntimeConst())
	assert.Equal(t, "kindInt", KindInt.RuntimeConst())
	assert.Equal(t, "kindBool", KindBool.RuntimeConst())
	assert.Equal(t, "kindObject", KindObject.RuntimeConst())
}

func TestPlatformNote(t *testing.T) {
	assert.Equal(t, "", PlatformBoth.Note())
	assert.Equal(t, "Software broker only.", PlatformSoftwareOnly.Note())
	assert.Equal(t, "Appliance only.", PlatformApplianceOnly.Note())
}
