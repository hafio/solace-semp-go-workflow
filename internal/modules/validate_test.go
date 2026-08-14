package modules

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/internal/models"
)

// validatable is a descriptor covering every rule the validator enforces.
func validatable() objectDesc {
	return desc(objectDesc{
		Prefix:       "thing",
		FriendlyName: "thing",
		Scope:        scopeVPN,
		Parents:      []parentHop{{Collection: "outers", Keys: []keyDesc{{Name: "outerName"}}}},
		Collection:   "things",
		OwnKey:       []keyDesc{{Name: "thingName"}},
		HasUpdate:    true,
		Fields: []fieldDesc{
			{Name: "label", Kind: kindString},
			{Name: "size", Kind: kindInt},
			{Name: "mode", Kind: kindString, Enum: []string{"active", "standby"}},
			{Name: "flavour", Kind: kindString, Const: true},
			{Name: "claimValue", Kind: kindString, Required: true},
			{Name: "threshold", Kind: kindObject, Fields: []fieldDesc{
				{Name: "clearPercent", Kind: kindInt, Requires: []string{"setPercent"}, Conflicts: []string{"clearValue"}},
				{Name: "clearValue", Kind: kindInt},
				{Name: "setPercent", Kind: kindInt},
			}},
		},
	})
}

// validArgs is a complete, accepted argument set for the add verb.
func validArgs() map[string]any {
	return map[string]any{
		"outerName":  "O",
		"thingName":  "T",
		"claimValue": "v",
		"label":      "x",
		"size":       10,
		"mode":       "active",
	}
}

func TestValidateAcceptsAWellFormedCall(t *testing.T) {
	d := validatable()
	require.NoError(t, d.validate(validArgs(), paramAdd))

	// Keys are arguments too, including the optional VPN override.
	args := validArgs()
	args["msgVpnName"] = "other-vpn"
	assert.NoError(t, d.validate(args, paramAdd))
}

func TestValidateRejectsUnknownArguments(t *testing.T) {
	// Until every attribute was declared from the specification this could not be
	// checked: an unrecognised name might have been a real attribute the tool did
	// not know about. Now it is a typo, and saying so locally beats a round trip.
	args := validArgs()
	args["labelz"] = "x"
	args["notAThing"] = 1

	err := validatable().validate(args, paramAdd)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown arguments: labelz, notAThing")
	// The message points at where the accepted names are written down.
	assert.Contains(t, err.Error(), "docs/modules/thing.md")
}

func TestValidateRejectsBodyArgumentsOnDelete(t *testing.T) {
	err := validatable().validate(map[string]any{
		"outerName": "O", "thingName": "T", "label": "x",
	}, paramDelete)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown argument: label")
	assert.Contains(t, err.Error(), "only the keys that identify the object")
}

func TestValidateEnforcesEnums(t *testing.T) {
	args := validArgs()
	args["mode"] = "sideways"

	err := validatable().validate(args, paramAdd)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "'mode' must be one of 'active', 'standby', got 'sideways'")
}

func TestValidateRejectsCreateOnlyAttributesOnUpdate(t *testing.T) {
	d := validatable()

	// Accepted at creation.
	args := validArgs()
	args["flavour"] = "plain"
	require.NoError(t, d.validate(args, paramAdd))

	// Rejected on update, where the broker would refuse it anyway.
	err := d.validate(map[string]any{"outerName": "O", "thingName": "T", "flavour": "plain"}, paramUpdate)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'flavour' can only be set when the object is created")
}

func TestValidateReportsMissingRequiredOnAddOnly(t *testing.T) {
	d := validatable()
	args := validArgs()
	delete(args, "claimValue")

	err := d.validate(args, paramAdd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required argument 'claimValue'")

	// An update changes only what it names, so a required attribute it omits is
	// not missing -- it is simply unchanged.
	assert.NoError(t, d.validate(map[string]any{"outerName": "O", "thingName": "T", "label": "x"}, paramUpdate))
}

func TestValidateRejectsUncoercibleIntegers(t *testing.T) {
	args := validArgs()
	args["size"] = "huge"

	err := validatable().validate(args, paramAdd)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "'size' must be an integer, got 'huge'")
}

func TestValidateEnforcesAttributeCoDependencies(t *testing.T) {
	d := validatable()

	// clearPercent requires setPercent.
	args := validArgs()
	args["threshold"] = map[string]any{"clearPercent": 60}
	err := d.validate(args, paramAdd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'threshold.clearPercent' requires 'threshold.setPercent' to be set as well")

	// clearPercent conflicts with clearValue.
	args = validArgs()
	args["threshold"] = map[string]any{"clearPercent": 60, "setPercent": 80, "clearValue": 5}
	err = d.validate(args, paramAdd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'threshold.clearPercent' cannot be combined with 'threshold.clearValue'")

	// Satisfied.
	args = validArgs()
	args["threshold"] = map[string]any{"clearPercent": 60, "setPercent": 80}
	assert.NoError(t, d.validate(args, paramAdd))
}

func TestValidateChecksInsideNestedGroups(t *testing.T) {
	d := validatable()

	args := validArgs()
	args["threshold"] = map[string]any{"setPercent": "eighty"}
	err := d.validate(args, paramAdd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'threshold.setPercent' must be an integer")

	args = validArgs()
	args["threshold"] = map[string]any{"setPercent": 80, "typo": 1}
	err = d.validate(args, paramAdd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'threshold' has unknown field(s): typo")

	args = validArgs()
	args["threshold"] = "not-an-object"
	err = d.validate(args, paramAdd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'threshold' must be an object with fields")
}

func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	// One run should list everything to fix rather than making the author
	// discover problems one request at a time.
	args := map[string]any{
		"outerName": "O",
		"thingName": "T",
		"mode":      "sideways",
		"size":      "huge",
		"bogus":     1,
	}
	err := validatable().validate(args, paramAdd)

	require.Error(t, err)
	for _, want := range []string{"unknown argument: bogus", "'mode' must be one of", "'size' must be an integer", "missing required argument 'claimValue'"} {
		assert.Contains(t, err.Error(), want)
	}
}

// A rejected call must not reach the broker at all: that is the whole point of
// validating locally, and a request issued anyway would mutate state the author
// did not intend.
func TestInvalidArgsFailWithoutIssuingAnyRequest(t *testing.T) {
	cases := []struct {
		name string
		run  func(Client) models.ActionResult
	}{
		{"add", func(c Client) models.ActionResult {
			args := validArgs()
			args["bogus"] = 1
			return collectionAdd{validatable()}.Execute(c, args, false)
		}},
		{"update", func(c Client) models.ActionResult {
			return collectionUpdate{validatable()}.Execute(c, map[string]any{
				"outerName": "O", "thingName": "T", "bogus": 1,
			}, false)
		}},
		{"delete", func(c Client) models.ActionResult {
			return collectionDelete{validatable()}.Execute(c, map[string]any{
				"outerName": "O", "thingName": "T", "label": "x",
			}, false)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fc := existing()
			res := tc.run(fc)

			assert.Equal(t, models.StatusFailed, res.Status)
			assert.Empty(t, fc.existsCalls, "validation must run before any GET")
			assert.Equal(t, 0, fc.createdCount)
			assert.Equal(t, 0, fc.updatedCount)
			assert.Equal(t, 0, fc.deletedCount)
		})
	}
}

func TestValidateChecksLengthOfBodyAttributesToo(t *testing.T) {
	// An attribute can be a path key on one object and a body attribute on
	// another: aclProfileName identifies an ACL profile but is an attribute of a
	// client username. Only the key position used to be length-checked, so the
	// same over-long value failed locally in one place and at the broker in the
	// other.
	d := desc(objectDesc{
		Prefix: "client_username", FriendlyName: "client username", Scope: scopeVPN,
		Collection: "clientUsernames",
		OwnKey:     []keyDesc{{Name: "clientUsername"}},
		HasUpdate:  true,
		Fields: []fieldDesc{
			{Name: "aclProfileName", Kind: kindString},
			{Name: "label", Kind: kindString},
		},
	})

	tooLong := strings.Repeat("a", 33) // aclProfileName is limited to 32
	err := d.validate(map[string]any{"clientUsername": "u", "aclProfileName": tooLong}, paramAdd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'aclProfileName' value is 33 bytes")
	assert.Contains(t, err.Error(), "limit is 32")

	// Exactly the limit is fine, and an attribute with no curated limit is not
	// constrained at all.
	assert.NoError(t, d.validate(map[string]any{
		"clientUsername": "u",
		"aclProfileName": strings.Repeat("a", 32),
		"label":          strings.Repeat("a", 5000),
	}, paramAdd))
}

func TestValidateChecksClientUsernameBodyAttributes(t *testing.T) {
	// The two *ClientUsername attributes name a Solace client username, so they
	// take its 189-byte limit. The other *Username attributes on these objects are
	// credentials for a remote system and are deliberately unconstrained.
	d := desc(objectDesc{
		Prefix: "bridge", FriendlyName: "bridge", Scope: scopeVPN, Collection: "bridges",
		OwnKey: []keyDesc{{Name: "bridgeName"}},
		Fields: []fieldDesc{
			{Name: "remoteAuthenticationBasicClientUsername", Kind: kindString},
			{Name: "authenticationBasicUsername", Kind: kindString},
		},
	})

	err := d.validate(map[string]any{
		"bridgeName":                              "b",
		"remoteAuthenticationBasicClientUsername": strings.Repeat("u", 190),
	}, paramAdd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit is 189")

	assert.NoError(t, d.validate(map[string]any{
		"bridgeName":                  "b",
		"authenticationBasicUsername": strings.Repeat("u", 500),
	}, paramAdd))
}

// The exported Validate is what `semp-workflow validate` calls, so it must agree
// with what Execute enforces.
func TestExportedValidateMatchesTheVerb(t *testing.T) {
	d := validatable()

	args := validArgs()
	args["flavour"] = "plain"
	assert.NoError(t, collectionAdd{d}.Validate(args))
	assert.Error(t, collectionUpdate{d}.Validate(args), "flavour is create-only")
	assert.Error(t, collectionDelete{d}.Validate(args), "a delete takes no body")
}
