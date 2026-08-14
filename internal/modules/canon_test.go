package modules

import (
	"testing"

	"solace-semp-workflow/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// canonObj mirrors the shape of a real generated descriptor: multi-word names
// carry both spellings, a single-word one ("owner") carries only the SEMP name,
// and a nested group carries them at both levels.
func canonObj() objectDesc {
	return desc(objectDesc{
		Prefix:       "thing",
		FriendlyName: "thing",
		Scope:        scopeVPN,
		Collection:   "things",
		OwnKey:       []keyDesc{{Name: "thingName", Canon: "thing_name", Desc: "Name of the thing"}},
		HasUpdate:    true,
		Fields: []fieldDesc{
			{Name: "egressEnabled", Canon: "egress_enabled", Kind: kindBool, Desc: "Egress"},
			{Name: "maxMsgSpoolUsage", Canon: "max_msg_spool_usage", Kind: kindInt, Desc: "Spool"},
			{Name: "owner", Kind: kindString, Desc: "Owner"},
			{Name: "eventBindCountThreshold", Canon: "event_bind_count_threshold", Kind: kindObject, Desc: "Threshold",
				Fields: []fieldDesc{
					{Name: "clearPercent", Canon: "clear_percent", Kind: kindInt},
					{Name: "setPercent", Canon: "set_percent", Kind: kindInt},
				}},
		},
	})
}

// This is the load-bearing test of the whole naming change. SEMP requires the
// camelCase spelling on the wire; the canonical snake_case name is a presentation
// concern that must never reach the broker. If this passes with snake_case input,
// nothing an operator types can change what SEMP receives.
func TestCanonicalArgsStillSendWireNames(t *testing.T) {
	fc := absent()
	res := collectionAdd{canonObj()}.Execute(fc, map[string]any{
		"thing_name":          "orders",
		"egress_enabled":      true,
		"max_msg_spool_usage": 5000,
		"owner":               "someone",
	}, false)

	require.Equal(t, models.StatusOK, res.Status, res.Message)
	assert.Equal(t, "msgVpns/test-vpn/things", fc.createPath)

	assert.Equal(t, "orders", fc.createBody["thingName"])
	assert.Equal(t, true, fc.createBody["egressEnabled"])
	assert.Equal(t, 5000, fc.createBody["maxMsgSpoolUsage"])
	assert.Equal(t, "someone", fc.createBody["owner"])

	for _, canon := range []string{"thing_name", "egress_enabled", "max_msg_spool_usage"} {
		assert.NotContains(t, fc.createBody, canon, "the canonical spelling must never reach the broker")
	}
}

// Both conventions converge before any payload is built, so neither gets a second
// code path and a customer config written against the previous implementation
// produces byte-identical requests.
func TestLegacyAndCanonicalArgsProduceTheSamePayload(t *testing.T) {
	canonical := absent()
	rc := collectionAdd{canonObj()}.Execute(canonical, map[string]any{
		"thing_name":          "orders",
		"egress_enabled":      true,
		"max_msg_spool_usage": 5000,
	}, false)

	legacy := absent()
	rl := collectionAdd{canonObj()}.Execute(legacy, map[string]any{
		"thingName":        "orders",
		"egressEnabled":    true,
		"maxMsgSpoolUsage": 5000,
	}, false)

	require.Equal(t, models.StatusOK, rc.Status, rc.Message)
	require.Equal(t, models.StatusOK, rl.Status, rl.Message)

	assert.Equal(t, legacy.createPath, canonical.createPath)
	assert.Equal(t, legacy.createBody, canonical.createBody)
	assert.Equal(t, rl.Message, rc.Message)
}

// Mixing the two spellings across different attributes is allowed: a template
// part-way through migration must not be rejected.
func TestMixedSpellingsInOneArgsBlockAreAccepted(t *testing.T) {
	fc := absent()
	res := collectionAdd{canonObj()}.Execute(fc, map[string]any{
		"thingName":      "orders", // legacy
		"egress_enabled": true,     // canonical
	}, false)

	require.Equal(t, models.StatusOK, res.Status, res.Message)
	assert.Equal(t, "orders", fc.createBody["thingName"])
	assert.Equal(t, true, fc.createBody["egressEnabled"])
}

// Go randomises map iteration, so letting one spelling overwrite the other would
// make the value that reached the broker depend on chance. Naming the mistake is
// the only deterministic option.
func TestSupplyingBothSpellingsOfOneAttributeFails(t *testing.T) {
	fc := absent()
	res := collectionAdd{canonObj()}.Execute(fc, map[string]any{
		"thing_name":     "orders",
		"egress_enabled": true,
		"egressEnabled":  false,
	}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "egress_enabled")
	assert.Contains(t, res.Message, "egressEnabled")
	assert.Empty(t, fc.createPath, "nothing may be sent when the request is ambiguous")
}

// A nested attribute group is addressed one level down, so its own field names
// need translating inside the nested map rather than at the top level.
func TestCanonicalArgsAreTranslatedInsideANestedGroup(t *testing.T) {
	fc := absent()
	res := collectionAdd{canonObj()}.Execute(fc, map[string]any{
		"thing_name": "orders",
		"event_bind_count_threshold": map[string]any{
			"clear_percent": 60,
			"set_percent":   80,
		},
	}, false)

	require.Equal(t, models.StatusOK, res.Status, res.Message)

	nested, ok := fc.createBody["eventBindCountThreshold"].(map[string]any)
	require.True(t, ok, "the group itself must be keyed by its SEMP name")
	assert.Equal(t, 60, nested["clearPercent"])
	assert.Equal(t, 80, nested["setPercent"])
	assert.NotContains(t, nested, "clear_percent")
}

// An unrecognised key is passed through untouched so the error quotes exactly
// what the caller typed, rather than something the normaliser rewrote.
func TestAnUnknownArgIsReportedAsTyped(t *testing.T) {
	fc := absent()
	res := collectionAdd{canonObj()}.Execute(fc, map[string]any{
		"thing_name":     "orders",
		"egress_enabeld": true, // typo
	}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "egress_enabeld")
}

// The VPN override is not a declared field -- resolve() reads it straight from
// the args map -- so its canonical spelling has to be mapped explicitly.
func TestCanonicalMsgVpnNameOverridesTheConfiguredVPN(t *testing.T) {
	fc := absent()
	res := collectionAdd{canonObj()}.Execute(fc, map[string]any{
		"thing_name":   "orders",
		"msg_vpn_name": "other-vpn",
	}, false)

	require.Equal(t, models.StatusOK, res.Status, res.Message)
	assert.Equal(t, "msgVpns/other-vpn/things", fc.createPath)
	assert.NotContains(t, fc.createBody, "msg_vpn_name")
	assert.NotContains(t, fc.createBody, "msgVpnName")
}

// Dual-accept is applied at every verb, not just add: update diffs the canonical
// value against a broker response keyed by the SEMP name, and delete builds a path
// from a canonical key.
func TestCanonicalArgsWorkOnUpdateAndDelete(t *testing.T) {
	t.Run("update", func(t *testing.T) {
		fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
			return true, map[string]any{"egressEnabled": false}, nil
		}}
		res := collectionUpdate{canonObj()}.Execute(fc, map[string]any{
			"thing_name":     "orders",
			"egress_enabled": true,
		}, false)

		require.Equal(t, models.StatusOK, res.Status, res.Message)
		assert.Equal(t, "msgVpns/test-vpn/things/orders", fc.updatePath)
		assert.Equal(t, true, fc.updateBody["egressEnabled"])
		assert.NotContains(t, fc.updateBody, "egress_enabled")
	})

	t.Run("update converges", func(t *testing.T) {
		// The diff compares against the SEMP spelling the broker returned, so a
		// canonical arg already in the desired state must report SKIPPED rather
		// than patching every run.
		fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
			return true, map[string]any{"egressEnabled": true}, nil
		}}
		res := collectionUpdate{canonObj()}.Execute(fc, map[string]any{
			"thing_name":     "orders",
			"egress_enabled": true,
		}, false)

		assert.Equal(t, models.StatusSkipped, res.Status)
		assert.Equal(t, 0, fc.updatedCount)
	})

	t.Run("delete", func(t *testing.T) {
		fc := existing()
		res := collectionDelete{canonObj()}.Execute(fc, map[string]any{"thing_name": "orders"}, false)

		require.Equal(t, models.StatusOK, res.Status, res.Message)
		assert.Equal(t, "msgVpns/test-vpn/things/orders", fc.deletePath)
	})
}

// The ambiguity check guards every verb, so no path can send a value chosen by
// map iteration order.
func TestSupplyingBothSpellingsFailsOnEveryVerb(t *testing.T) {
	args := map[string]any{
		"thing_name":     "orders",
		"egress_enabled": true,
		"egressEnabled":  false,
	}

	t.Run("update", func(t *testing.T) {
		fc := existing()
		res := collectionUpdate{canonObj()}.Execute(fc, args, false)
		assert.Equal(t, models.StatusFailed, res.Status)
		assert.Equal(t, 0, fc.updatedCount)
	})

	t.Run("delete", func(t *testing.T) {
		fc := existing()
		res := collectionDelete{canonObj()}.Execute(fc, args, false)
		assert.Equal(t, models.StatusFailed, res.Status)
		assert.Equal(t, 0, fc.deletedCount)
	})
}

// Offline validation shares the normaliser, so `semp-workflow validate` accepts
// and rejects exactly what a run would.
func TestValidateAcceptsBothSpellingsAndRejectsThemTogether(t *testing.T) {
	d := canonObj()

	assert.NoError(t, collectionAdd{d}.Validate(map[string]any{"thing_name": "o", "egress_enabled": true}))
	assert.NoError(t, collectionAdd{d}.Validate(map[string]any{"thingName": "o", "egressEnabled": true}))
	assert.NoError(t, collectionUpdate{d}.Validate(map[string]any{"thing_name": "o", "egress_enabled": true}))
	assert.NoError(t, collectionDelete{d}.Validate(map[string]any{"thing_name": "o"}))

	both := map[string]any{"thing_name": "o", "egress_enabled": true, "egressEnabled": false}
	assert.Error(t, collectionAdd{d}.Validate(both))
	assert.Error(t, collectionUpdate{d}.Validate(both))
	assert.Error(t, collectionDelete{d}.Validate(both))
}

// The ten carried-over families answer to both names, and both resolve to the
// same Module value so the two can never drift apart.
func TestLegacyAndCanonicalActionNamesResolveToTheSameModule(t *testing.T) {
	for canon, legacy := range legacyActionPrefixes {
		for _, verb := range []string{"add", "update", "delete"} {
			canonName, legacyName := canon+"."+verb, legacy+"."+verb

			cm, canonErr := Get(canonName)
			lm, legacyErr := Get(legacyName)
			if canonErr != nil {
				// Not every family has all three verbs; if the canonical name does
				// not exist, the legacy one must not either.
				assert.Errorf(t, legacyErr, "%s resolves but %s does not", legacyName, canonName)
				continue
			}
			require.NoErrorf(t, legacyErr, "legacy name %s must still resolve", legacyName)
			// The two names share one Module value, but a descriptor can carry
			// payload hooks and reflect.DeepEqual reports any non-nil func as
			// unequal -- so compare the func-free projections instead.
			assert.Equalf(t, cm.Description(), lm.Description(),
				"%s and %s must describe the same module", legacyName, canonName)
			assert.Equalf(t, cm.Params(), lm.Params(),
				"%s and %s must accept the same parameters", legacyName, canonName)
		}
	}
}

// Legacy names resolve but are not part of the documented surface, so they never
// appear in the module reference or inflate the action count.
func TestLegacyActionNamesAreNotListed(t *testing.T) {
	listed := map[string]bool{}
	for _, n := range List() {
		listed[n] = true
	}
	for _, legacy := range legacyActionPrefixes {
		assert.Falsef(t, listed[legacy+".add"], "%s.add must not appear in List()", legacy)
	}
	assert.Len(t, List(), len(ExpectedModules))
}

// overrides is keyed by canonical prefix. An entry matching no descriptor would
// silently stop applying -- the queue TTL derivation, the subscription's
// optimistic add, the ACL exceptions' DisplayKey -- with nothing else failing.
func TestEveryOverrideKeyMatchesARealDescriptor(t *testing.T) {
	known := map[string]bool{}
	for _, d := range descriptors {
		known[d.Prefix] = true
	}
	for prefix := range overrides {
		assert.Truef(t, known[prefix], "override %q matches no descriptor", prefix)
	}
}

// Likewise for the legacy action table: an entry naming a prefix that no longer
// exists would silently stop providing its backward-compatibility guarantee.
func TestEveryLegacyActionPrefixMatchesARealDescriptor(t *testing.T) {
	known := map[string]bool{}
	for _, d := range descriptors {
		known[d.Prefix] = true
	}
	for canon := range legacyActionPrefixes {
		assert.Truef(t, known[canon], "legacy action entry %q matches no descriptor", canon)
	}
}

// Params is the documented surface, so it publishes canonical names throughout --
// keys and body attributes alike.
func TestParamsPublishCanonicalNames(t *testing.T) {
	names := map[string]bool{}
	for _, p := range buildParams(canonObj(), paramAdd) {
		names[p.Name] = true
	}

	for _, want := range []string{"thing_name", "egress_enabled", "max_msg_spool_usage", "owner", "msg_vpn_name"} {
		assert.Truef(t, names[want], "expected canonical param %q in %v", want, names)
	}
	for _, unwanted := range []string{"thingName", "egressEnabled", "maxMsgSpoolUsage", "msgVpnName"} {
		assert.Falsef(t, names[unwanted], "the SEMP spelling %q must not be published", unwanted)
	}
}
