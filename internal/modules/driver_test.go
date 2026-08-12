package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/internal/models"
)

// The three drivers implement every action in the registry, so they are tested
// once per structural shape rather than once per object. The shapes the ten
// carried-over families do not exercise -- the Broker singleton, broker-global
// scope, a three-part composite key, two levels of nesting, a nested attribute
// group, and convergence against current state -- are covered here.

// desc prepares a descriptor the way registerObject would, without touching the
// registry, so a test can drive one action in isolation.
func desc(d objectDesc) objectDesc {
	applyMsgDefaults(&d)
	return d
}

// noVPNClient is a client with no configured default VPN, for the case where a
// VPN-scoped action must name one explicitly.
type noVPNClient struct{ *fakeClient }

func (noVPNClient) DefaultMsgVPN() string { return "" }

// --- fixtures -------------------------------------------------------------

// queueLike is a simple VPN-scoped object with one key.
func queueLike() objectDesc {
	return desc(objectDesc{
		Prefix:       "thing",
		FriendlyName: "thing",
		Scope:        scopeVPN,
		Collection:   "things",
		OwnKey:       []keyDesc{{Name: "thingName", Desc: "Name of the thing"}},
		HasUpdate:    true,
		Fields: []fieldDesc{
			{Name: "label", Kind: kindString, Desc: "A label"},
			{Name: "size", Kind: kindInt, Desc: "A size"},
			{Name: "enabled", Kind: kindBool, Desc: "Enabled"},
		},
	})
}

// globalObj is broker-global: its path has no msgVpns hop at all.
func globalObj() objectDesc {
	return desc(objectDesc{
		Prefix:       "virtual_hostname",
		FriendlyName: "virtual hostname",
		Scope:        scopeGlobal,
		Collection:   "virtualHostnames",
		OwnKey:       []keyDesc{{Name: "virtualHostname", Desc: "The virtual hostname"}},
		HasUpdate:    true,
		Fields:       []fieldDesc{{Name: "enabled", Kind: kindBool, Desc: "Enabled"}},
	})
}

// brokerObj is the singleton: no key, no collection, empty path.
func brokerObj() objectDesc {
	return desc(objectDesc{
		Prefix:       "broker",
		FriendlyName: "broker",
		Scope:        scopeGlobal,
		IsSingleton:  true,
		HasUpdate:    true,
		Fields: []fieldDesc{
			{Name: "tlsServerCertPassword", Kind: kindString, WriteOnly: true, Desc: "Cert password"},
			{Name: "guaranteedMsgingEnabled", Kind: kindBool, Desc: "Enable guaranteed messaging"},
		},
	})
}

// deepObj has two ancestor hops, the outer one with a two-part composite key,
// and a three-part composite key of its own.
func deepObj() objectDesc {
	return desc(objectDesc{
		Prefix:             "bridge_remote_msg_vpn",
		FriendlyName:       "remote message VPN",
		ParentFriendlyName: "bridge",
		Scope:              scopeVPN,
		Parents: []parentHop{
			{Collection: "bridges", Keys: []keyDesc{{Name: "bridgeName"}, {Name: "bridgeVirtualRouter"}}},
		},
		Collection: "remoteMsgVpns",
		OwnKey: []keyDesc{
			{Name: "remoteMsgVpnName"},
			{Name: "remoteMsgVpnLocation"},
			{Name: "remoteMsgVpnInterface"},
		},
		HasUpdate: true,
		Fields:    []fieldDesc{{Name: "enabled", Kind: kindBool}},
	})
}

// --- path shapes ----------------------------------------------------------

func TestAddBuildsVPNScopedPaths(t *testing.T) {
	fc := absent()
	res := collectionAdd{queueLike()}.Execute(fc, map[string]any{"thingName": "orders"}, false)

	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, []string{"msgVpns/test-vpn/things/orders"}, fc.existsCalls)
	// A create POSTs to the collection, not to the item.
	assert.Equal(t, "msgVpns/test-vpn/things", fc.createPath)
}

func TestAddBuildsGlobalScopedPathWithoutVPNHop(t *testing.T) {
	fc := absent()
	res := collectionAdd{globalObj()}.Execute(fc, map[string]any{"virtualHostname": "vh1"}, false)

	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, []string{"virtualHostnames/vh1"}, fc.existsCalls)
	assert.Equal(t, "virtualHostnames", fc.createPath)
}

func TestExplicitMsgVpnNameOverridesTheDefault(t *testing.T) {
	fc := absent()
	collectionAdd{queueLike()}.Execute(fc, map[string]any{"thingName": "orders", "msgVpnName": "other-vpn"}, false)

	assert.Equal(t, []string{"msgVpns/other-vpn/things/orders"}, fc.existsCalls)
	// The VPN is a path key, so it must not appear in the request body.
	assert.NotContains(t, fc.createBody, "msgVpnName")
}

func TestVPNScopedActionFailsWhenNoVPNIsAvailable(t *testing.T) {
	fc := noVPNClient{absent()}
	res := collectionAdd{queueLike()}.Execute(fc, map[string]any{"thingName": "orders"}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "msgVpnName")
	assert.Equal(t, 0, fc.createdCount)
}

func TestCompositeKeysAreCommaJoinedAndEncodedPerPart(t *testing.T) {
	fc := absent()
	collectionAdd{deepObj()}.Execute(fc, map[string]any{
		"bridgeName":            "b1",
		"bridgeVirtualRouter":   "primary",
		"remoteMsgVpnName":      "vpn2",
		"remoteMsgVpnLocation":  "10.0.0.1:55555",
		"remoteMsgVpnInterface": "eth0",
	}, false)

	assert.Equal(t,
		[]string{"msgVpns/test-vpn/bridges/b1,primary/remoteMsgVpns/vpn2,10.0.0.1%3A55555,eth0"},
		fc.existsCalls)
	assert.Equal(t, "msgVpns/test-vpn/bridges/b1,primary/remoteMsgVpns", fc.createPath)

	// Ancestor keys are carried by the URL and rejected in the body, so they are
	// stripped; the object's own key is Required on a create and so is sent.
	for _, k := range []string{"msgVpnName", "bridgeName", "bridgeVirtualRouter"} {
		assert.NotContains(t, fc.createBody, k)
	}
	assert.Equal(t, "vpn2", fc.createBody["remoteMsgVpnName"])
	assert.Equal(t, "10.0.0.1:55555", fc.createBody["remoteMsgVpnLocation"])
	assert.Equal(t, "eth0", fc.createBody["remoteMsgVpnInterface"])
}

func TestOwnKeyDefaultIsAppliedToBothPathAndBody(t *testing.T) {
	// An ACL exception's topic syntax is a discriminator an operator rarely
	// states, so it defaults. The default has to reach the path (it is part of
	// the composite key) and the body (the specification marks it Required on a
	// create), or a workflow that names only the topic breaks.
	d := desc(objectDesc{
		Prefix: "exc", FriendlyName: "exception", Scope: scopeVPN,
		Parents:            []parentHop{{Collection: "aclProfiles", Keys: []keyDesc{{Name: "aclProfileName"}}}},
		ParentFriendlyName: "ACL profile",
		Collection:         "topicExceptions",
		OwnKey: []keyDesc{
			{Name: "syntax", Default: "smf"},
			{Name: "topic"},
		},
		DisplayKey: 1,
	})
	fc := absent()
	res := collectionAdd{d}.Execute(fc, map[string]any{"aclProfileName": "P", "topic": "SITEA/>"}, false)

	require.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, []string{"msgVpns/test-vpn/aclProfiles/P/topicExceptions/smf,SITEA%2F%3E"}, fc.existsCalls)
	assert.Equal(t, "smf", fc.createBody["syntax"])
	assert.Equal(t, "SITEA/>", fc.createBody["topic"])
	// DisplayKey selects the part an operator recognises.
	assert.Contains(t, res.Message, "'SITEA/>'")
}

func TestUpdateDropsTheObjectsOwnKeyFromTheBody(t *testing.T) {
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return true, map[string]any{"label": "old"}, nil
	}}
	res := collectionUpdate{queueLike()}.Execute(fc, map[string]any{"thingName": "orders", "label": "new"}, false)

	require.Equal(t, models.StatusOK, res.Status)
	// The key identifies the target in the URL and is Const, so a PATCH must not
	// restate it.
	assert.NotContains(t, fc.updateBody, "thingName")
	assert.Equal(t, map[string]any{"label": "new"}, fc.updateBody)
}

func TestMissingKeysAreAllReportedTogether(t *testing.T) {
	d := desc(objectDesc{
		Prefix: "thing", FriendlyName: "thing", Scope: scopeVPN, Collection: "things",
		Parents: []parentHop{{Collection: "outers", Keys: []keyDesc{{Name: "outerName"}}}},
		OwnKey:  []keyDesc{{Name: "thingName"}},
	})
	res := collectionAdd{d}.Execute(absent(), map[string]any{}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	// Both names, and the plural agrees with the count, so an operator fixes the
	// call in one pass instead of one key per run.
	assert.Equal(t, "Missing required args: outerName, thingName", res.Message)
}

func TestSingleMissingKeyReadsSingular(t *testing.T) {
	res := collectionAdd{queueLike()}.Execute(absent(), map[string]any{}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Equal(t, "Missing required arg: thingName", res.Message)
}

func TestNestedObjectNamesItsParentInMessages(t *testing.T) {
	res := collectionAdd{deepObj()}.Execute(existing(), map[string]any{
		"bridgeName":            "b1",
		"bridgeVirtualRouter":   "primary",
		"remoteMsgVpnName":      "vpn2",
		"remoteMsgVpnLocation":  "loc",
		"remoteMsgVpnInterface": "eth0",
	}, false)

	assert.Equal(t, models.StatusSkipped, res.Status)
	assert.Contains(t, res.Message, "Remote message VPN 'vpn2'")
	assert.Contains(t, res.Message, "on bridge 'b1'")
}

func TestMissingParentKeyIsReportedByName(t *testing.T) {
	res := collectionAdd{deepObj()}.Execute(absent(), map[string]any{"remoteMsgVpnName": "vpn2"}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "bridgeName")
}

// --- the Broker singleton -------------------------------------------------

func TestSingletonUpdateUsesTheEmptyPath(t *testing.T) {
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return true, map[string]any{"guaranteedMsgingEnabled": false}, nil
	}}
	res := collectionUpdate{brokerObj()}.Execute(fc, map[string]any{"guaranteedMsgingEnabled": "true"}, false)

	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, []string{""}, fc.existsCalls)
	assert.Equal(t, "", fc.updatePath)
	assert.Equal(t, map[string]any{"guaranteedMsgingEnabled": true}, fc.updateBody)
	assert.Equal(t, "Broker updated", res.Message)
}

func TestSingletonUpdateSkipsWhenAlreadyConverged(t *testing.T) {
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return true, map[string]any{"guaranteedMsgingEnabled": true}, nil
	}}
	res := collectionUpdate{brokerObj()}.Execute(fc, map[string]any{"guaranteedMsgingEnabled": "true"}, false)

	assert.Equal(t, models.StatusSkipped, res.Status)
	assert.Equal(t, 0, fc.updatedCount)
}

// --- convergence ----------------------------------------------------------

func TestUpdateSendsOnlyChangedAttributes(t *testing.T) {
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		// size already matches; label does not. Note the broker returns numbers
		// as float64, which must not read as a change against a coerced int.
		return true, map[string]any{"label": "old", "size": float64(10), "enabled": true}, nil
	}}
	res := collectionUpdate{queueLike()}.Execute(fc, map[string]any{
		"thingName": "orders",
		"label":     "new",
		"size":      "10",
		"enabled":   "true",
	}, false)

	require.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, map[string]any{"label": "new"}, fc.updateBody)
	assert.Equal(t, "msgVpns/test-vpn/things/orders", fc.updatePath)
}

func TestUpdateSkipsWhenEverySuppliedAttributeAlreadyMatches(t *testing.T) {
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return true, map[string]any{"label": "same", "size": float64(10)}, nil
	}}
	res := collectionUpdate{queueLike()}.Execute(fc, map[string]any{
		"thingName": "orders", "label": "same", "size": 10,
	}, false)

	assert.Equal(t, models.StatusSkipped, res.Status)
	assert.Contains(t, res.Message, "already in the desired state")
	assert.Equal(t, 0, fc.updatedCount)
}

func TestUpdateSkipsWhenNoAttributesSupplied(t *testing.T) {
	res := collectionUpdate{queueLike()}.Execute(existing(), map[string]any{"thingName": "orders"}, false)

	assert.Equal(t, models.StatusSkipped, res.Status)
	assert.Equal(t, "No fields to update", res.Message)
}

func TestUpdateFailsWhenTheObjectDoesNotExist(t *testing.T) {
	fc := absent()
	res := collectionUpdate{queueLike()}.Execute(fc, map[string]any{"thingName": "orders", "label": "x"}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "does not exist")
	assert.Equal(t, 0, fc.updatedCount)
}

func TestUpdateAlwaysResendsWriteOnlyAttributes(t *testing.T) {
	// A write-only attribute is absent from every GET, so it can never be shown
	// to already hold the desired value and must be re-sent each time.
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return true, map[string]any{"guaranteedMsgingEnabled": true}, nil
	}}
	res := collectionUpdate{brokerObj()}.Execute(fc, map[string]any{
		"tlsServerCertPassword":   "hunter2",
		"guaranteedMsgingEnabled": "true",
	}, false)

	require.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, map[string]any{"tlsServerCertPassword": "hunter2"}, fc.updateBody)
}

func TestUpdateDropsConstAttributes(t *testing.T) {
	d := desc(objectDesc{
		Prefix: "thing", FriendlyName: "thing", Scope: scopeVPN, Collection: "things",
		OwnKey:    []keyDesc{{Name: "thingName"}},
		HasUpdate: true,
		Fields: []fieldDesc{
			{Name: "flavour", Kind: kindString, Const: true},
			{Name: "label", Kind: kindString},
		},
	})
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return true, map[string]any{"flavour": "old", "label": "old"}, nil
	}}
	res := collectionUpdate{d}.Execute(fc, map[string]any{
		"thingName": "orders", "flavour": "new", "label": "new",
	}, false)

	require.Equal(t, models.StatusOK, res.Status)
	// flavour is settable only at creation, so it never reaches a PATCH even
	// when the caller supplies a different value.
	assert.Equal(t, map[string]any{"label": "new"}, fc.updateBody)
}

func TestUpdateWidensCoDependentAttributeGroups(t *testing.T) {
	d := desc(objectDesc{
		Prefix: "thing", FriendlyName: "thing", Scope: scopeVPN, Collection: "things",
		OwnKey:    []keyDesc{{Name: "thingName"}},
		HasUpdate: true,
		Fields: []fieldDesc{
			{Name: "clearPercent", Kind: kindInt, Requires: []string{"setPercent"}},
			{Name: "setPercent", Kind: kindInt, Requires: []string{"clearPercent"}},
		},
	})
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		// Only clearPercent differs, but the pair must travel together so the
		// request validates whether or not the broker merges it onto current state.
		return true, map[string]any{"clearPercent": float64(50), "setPercent": float64(80)}, nil
	}}
	res := collectionUpdate{d}.Execute(fc, map[string]any{
		"thingName": "orders", "clearPercent": 60, "setPercent": 80,
	}, false)

	require.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, map[string]any{"clearPercent": 60, "setPercent": 80}, fc.updateBody)
}

func TestUpdateAlwaysSendsNestedAttributeGroups(t *testing.T) {
	// SEMP documents every member of an event threshold as absent from a GET,
	// so the group cannot be diffed and is sent whenever it is supplied.
	d := desc(objectDesc{
		Prefix: "thing", FriendlyName: "thing", Scope: scopeVPN, Collection: "things",
		OwnKey:    []keyDesc{{Name: "thingName"}},
		HasUpdate: true,
		Fields: []fieldDesc{
			{Name: "eventBindCountThreshold", Kind: kindObject, Fields: []fieldDesc{
				{Name: "setPercent", Kind: kindInt},
			}},
		},
	})
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return true, map[string]any{}, nil
	}}
	res := collectionUpdate{d}.Execute(fc, map[string]any{
		"thingName":               "orders",
		"eventBindCountThreshold": map[string]any{"setPercent": "80"},
	}, false)

	require.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, map[string]any{"eventBindCountThreshold": map[string]any{"setPercent": 80}}, fc.updateBody)
}

// --- required attributes and validation -----------------------------------

func TestAddFailsOnAMissingRequiredNonKeyAttribute(t *testing.T) {
	d := desc(objectDesc{
		Prefix: "claim", FriendlyName: "claim", Scope: scopeVPN, Collection: "claims",
		OwnKey: []keyDesc{{Name: "claimName"}},
		Fields: []fieldDesc{{Name: "claimValue", Kind: kindString, Required: true}},
	})
	fc := absent()
	res := collectionAdd{d}.Execute(fc, map[string]any{"claimName": "sub"}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "claimValue")
	assert.Equal(t, 0, fc.createdCount)
}

func TestAddAcceptsRequiredFalseAndZero(t *testing.T) {
	// A supplied false or 0 is a real value, not an omission, so presence must
	// be judged by kind rather than by truthiness.
	d := desc(objectDesc{
		Prefix: "thing", FriendlyName: "thing", Scope: scopeVPN, Collection: "things",
		OwnKey: []keyDesc{{Name: "thingName"}},
		Fields: []fieldDesc{
			{Name: "deliverAlwaysEnabled", Kind: kindBool, Required: true},
			{Name: "weight", Kind: kindInt, Required: true},
		},
	})
	res := collectionAdd{d}.Execute(absent(), map[string]any{
		"thingName": "t", "deliverAlwaysEnabled": false, "weight": 0,
	}, false)

	assert.Equal(t, models.StatusOK, res.Status)
}

func TestNameLengthViolationFailsBeforeAnyRequest(t *testing.T) {
	d := desc(objectDesc{
		Prefix: "queue", FriendlyName: "queue", Scope: scopeVPN, Collection: "queues",
		// queueName has a broker limit of 200 in semp.NameMaxLengths.
		OwnKey: []keyDesc{{Name: "queueName"}},
	})
	fc := absent()
	long := ""
	for i := 0; i < 201; i++ {
		long += "a"
	}
	res := collectionAdd{d}.Execute(fc, map[string]any{"queueName": long}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "broker limit is 200")
	assert.Empty(t, fc.existsCalls)
}

func TestIntCoercionErrorNamesTheAttribute(t *testing.T) {
	fc := absent()
	res := collectionAdd{queueLike()}.Execute(fc, map[string]any{"thingName": "t", "size": "huge"}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "'size'")
	assert.Equal(t, 0, fc.createdCount)
}

// --- add modes and error mapping ------------------------------------------

func TestOptimisticAddSkipsThePrecheckOnTheLivePath(t *testing.T) {
	d := queueLike()
	d.AddMode = addOptimistic
	fc := absent()

	res := collectionAdd{d}.Execute(fc, map[string]any{"thingName": "orders"}, false)

	assert.Equal(t, models.StatusOK, res.Status)
	assert.Empty(t, fc.existsCalls, "optimistic add must not GET before posting")
	assert.Equal(t, 1, fc.createdCount)
}

func TestOptimisticAddStillPrechecksUnderDryRun(t *testing.T) {
	// Dry-run has nothing else to base its answer on, so it reads state even in
	// the mode that skips the read when acting for real.
	d := queueLike()
	d.AddMode = addOptimistic
	fc := existing()

	res := collectionAdd{d}.Execute(fc, map[string]any{"thingName": "orders"}, true)

	assert.Equal(t, models.StatusSkipped, res.Status)
	assert.Len(t, fc.existsCalls, 1)
	assert.Equal(t, 0, fc.createdCount)
}

func TestAlreadyExistsOnCreateMapsToSkipped(t *testing.T) {
	// A create that loses a race, or an optimistic add whose object is already
	// present, has reached the desired state and is not a failure.
	fc := absent()
	fc.onCreate = func(string, map[string]any) (map[string]any, error) { return nil, sempErr(10) }

	res := collectionAdd{queueLike()}.Execute(fc, map[string]any{"thingName": "orders"}, false)

	assert.Equal(t, models.StatusSkipped, res.Status)
	assert.Contains(t, res.Message, "already exists")
}

func TestCheckErrorIsReportedWithTheObjectName(t *testing.T) {
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return false, nil, sempErr(500)
	}}
	res := collectionAdd{queueLike()}.Execute(fc, map[string]any{"thingName": "orders"}, false)

	assert.Equal(t, models.StatusFailed, res.Status)
	assert.Contains(t, res.Message, "Error checking thing")
}

// --- delete ---------------------------------------------------------------

func TestDeleteSkipsWhenAbsentAndDeletesWhenPresent(t *testing.T) {
	absentFC := absent()
	res := collectionDelete{queueLike()}.Execute(absentFC, map[string]any{"thingName": "orders"}, false)
	assert.Equal(t, models.StatusSkipped, res.Status)
	assert.Equal(t, 0, absentFC.deletedCount)

	presentFC := existing()
	res = collectionDelete{queueLike()}.Execute(presentFC, map[string]any{"thingName": "orders"}, false)
	assert.Equal(t, models.StatusOK, res.Status)
	assert.Equal(t, "msgVpns/test-vpn/things/orders", presentFC.deletePath)
}

func TestDryRunNeverMutates(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(Client) models.ActionResult
		fc   *fakeClient
	}{
		{"add", func(c Client) models.ActionResult {
			return collectionAdd{queueLike()}.Execute(c, map[string]any{"thingName": "t"}, true)
		}, absent()},
		{"delete", func(c Client) models.ActionResult {
			return collectionDelete{queueLike()}.Execute(c, map[string]any{"thingName": "t"}, true)
		}, existing()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.run(tc.fc)
			assert.Equal(t, models.StatusDryRun, res.Status)
			assert.Equal(t, 0, tc.fc.createdCount)
			assert.Equal(t, 0, tc.fc.updatedCount)
			assert.Equal(t, 0, tc.fc.deletedCount)
		})
	}
}

func TestUpdateDryRunReportsWithoutPatching(t *testing.T) {
	fc := &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return true, map[string]any{"label": "old"}, nil
	}}
	res := collectionUpdate{queueLike()}.Execute(fc, map[string]any{"thingName": "t", "label": "new"}, true)

	assert.Equal(t, models.StatusDryRun, res.Status)
	assert.Equal(t, 0, fc.updatedCount)
}

// --- documented metadata --------------------------------------------------

func TestParamsByMode(t *testing.T) {
	d := deepObj()

	add := buildParams(d, paramAdd)
	// Ancestor keys lead, then the object's own key, then attributes, then the
	// optional VPN override last so the required identifier reads first.
	assert.Equal(t, "bridgeName", add[0].Name)
	assert.True(t, add[0].Required)
	assert.Equal(t, "msgVpnName", add[len(add)-1].Name)
	assert.False(t, add[len(add)-1].Required)

	del := buildParams(d, paramDelete)
	// A delete has no body, so it documents keys only.
	for _, p := range del {
		assert.NotEqual(t, "enabled", p.Name)
	}
}

func TestGlobalObjectDocumentsNoMsgVpnName(t *testing.T) {
	for _, p := range buildParams(globalObj(), paramAdd) {
		assert.NotEqual(t, "msgVpnName", p.Name)
	}
}

func TestWriteOnlyParamsAreMarkedSensitive(t *testing.T) {
	var found bool
	for _, p := range buildParams(brokerObj(), paramUpdate) {
		if p.Name == "tlsServerCertPassword" {
			found = true
			assert.True(t, p.Sensitive, "a write-only attribute must be masked in output and docs")
		}
	}
	assert.True(t, found)
}

func TestUpdateParamsOmitDefaultsAndConstAttributes(t *testing.T) {
	d := desc(objectDesc{
		Prefix: "thing", FriendlyName: "thing", Scope: scopeVPN, Collection: "things",
		OwnKey: []keyDesc{{Name: "thingName"}},
		Fields: []fieldDesc{
			{Name: "flavour", Kind: kindString, Const: true, Default: "plain"},
			{Name: "label", Kind: kindString, Default: "none"},
		},
	})

	for _, p := range buildParams(d, paramUpdate) {
		assert.NotEqual(t, "flavour", p.Name, "const attributes are settable only at creation")
		if p.Name == "label" {
			// Omitting an attribute on update leaves it unchanged, so advertising
			// the create-time default would be misleading.
			assert.Nil(t, p.Default)
		}
	}
	for _, p := range buildParams(d, paramAdd) {
		if p.Name == "label" {
			assert.Equal(t, "none", p.Default)
		}
	}
}

func TestDescriptionUsesTheDeclaredArticle(t *testing.T) {
	// No rule derives this correctly: "RDP" is spelled out letter by letter and
	// takes "an", while "REST consumer" is pronounced as a word and takes "a",
	// yet both are all-caps. So the article is stated per object.
	d := queueLike()
	assert.Contains(t, collectionAdd{d}.Description(), "Create a thing.")
	assert.Contains(t, collectionDelete{d}.Description(), "Delete a thing.")

	d.FriendlyName, d.Article = "RDP", "an"
	assert.Contains(t, collectionAdd{d}.Description(), "Create an RDP.")
	assert.Contains(t, collectionDelete{d}.Description(), "Delete an RDP.")
}

func TestEveryRegisteredDescriptorDescribesItself(t *testing.T) {
	// The article itself cannot be checked by rule -- that is why it is data --
	// but a descriptor with no name, or a description that never mentions the
	// object, is a defect the docs would carry straight through.
	for _, d := range descriptors {
		assert.NotEmptyf(t, d.FriendlyName, "%s: FriendlyName", d.Prefix)
		assert.NotEmptyf(t, d.Collection != "" || d.IsSingleton, "%s: Collection", d.Prefix)

		if d.IsSingleton {
			assert.Contains(t, collectionUpdate{d}.Description(), d.FriendlyName)
			continue
		}
		for verb, got := range map[string]string{
			"add":    collectionAdd{d}.Description(),
			"delete": collectionDelete{d}.Description(),
		} {
			assert.Containsf(t, got, d.FriendlyName, "%s.%s description omits the object name", d.Prefix, verb)
			assert.NotContainsf(t, got, "  ", "%s.%s: doubled space", d.Prefix, verb)
		}
	}
}

func TestDeprecatedObjectSaysSoInItsDescription(t *testing.T) {
	d := queueLike()
	d.Deprecated = "superseded by thing_v2."

	assert.Contains(t, collectionAdd{d}.Description(), "Deprecated: superseded by thing_v2.")
	assert.Contains(t, collectionDelete{d}.Description(), "Deprecated: superseded by thing_v2.")
}

// Every descriptor must actually reach the registry, and every registry key
// must trace back to a descriptor. A generated file whose init() ran but whose
// actions were not wired -- or an action registered without a descriptor behind
// it -- would otherwise be invisible until a workflow named it.
func TestEveryDescriptorIsWiredIntoTheRegistry(t *testing.T) {
	require.NotEmpty(t, descriptors, "no descriptors registered")

	registered := map[string]bool{}
	for _, name := range List() {
		registered[name] = true
	}

	expected := 0
	for _, d := range descriptors {
		if d.IsSingleton {
			assert.Truef(t, registered[d.Prefix+".update"], "%s.update not registered", d.Prefix)
			expected++
			continue
		}
		assert.Truef(t, registered[d.Prefix+".add"], "%s.add not registered", d.Prefix)
		assert.Truef(t, registered[d.Prefix+".delete"], "%s.delete not registered", d.Prefix)
		expected += 2
		if d.HasUpdate {
			assert.Truef(t, registered[d.Prefix+".update"], "%s.update not registered", d.Prefix)
			expected++
		} else {
			assert.Falsef(t, registered[d.Prefix+".update"], "%s.update registered but HasUpdate is false", d.Prefix)
		}
	}
	assert.Len(t, List(), expected, "registry holds actions with no descriptor behind them")
}

// Message defaults are filled at registration, so no registered action can render
// a template that is still empty -- which would surface to an operator as a blank
// result message.
func TestRegisteredDescriptorsHaveCompleteMessageSets(t *testing.T) {
	for _, d := range descriptors {
		assert.NotEmpty(t, d.Msg.MissingArg, "%s: MissingArg", d.Prefix)
		assert.NotEmpty(t, d.Msg.Converged, "%s: Converged", d.Prefix)
		assert.NotEmpty(t, d.Msg.CheckErr, "%s: CheckErr", d.Prefix)
		assert.NotEmpty(t, d.Msg.UpdateErr, "%s: UpdateErr", d.Prefix)
		if d.IsSingleton {
			continue
		}
		for name, tmpl := range map[string]string{
			"AlreadyExists": d.Msg.AlreadyExists,
			"NotExists":     d.Msg.NotExists,
			"WouldCreate":   d.Msg.WouldCreate,
			"WouldDelete":   d.Msg.WouldDelete,
			"Created":       d.Msg.Created,
			"Deleted":       d.Msg.Deleted,
			"CreateErr":     d.Msg.CreateErr,
			"DeleteErr":     d.Msg.DeleteErr,
		} {
			assert.NotEmptyf(t, tmpl, "%s: %s", d.Prefix, name)
		}
	}
}

// The duplicate guard is a safety net for generated code: at 225 actions a
// prefix collision must stop the binary starting rather than let one module
// silently shadow another. A net that has never been shown to fire is not a net.
func TestRegisterPanicsOnADuplicateName(t *testing.T) {
	const name = "duplicate_probe.add"
	register(name, collectionAdd{queueLike()})
	t.Cleanup(func() { delete(registry, name) })

	assert.PanicsWithValue(t,
		`modules: duplicate registration for "duplicate_probe.add"`,
		func() { register(name, collectionAdd{queueLike()}) },
	)
}

func TestSingletonDescriptionsMentionTheObject(t *testing.T) {
	d := brokerObj()
	got := collectionUpdate{d}.Description()

	assert.Contains(t, got, "broker")
	// A singleton is never created, so its description must not imply otherwise.
	assert.NotContains(t, got, "does not exist")
}

func TestDeprecatedObjectSaysSoOnUpdateToo(t *testing.T) {
	d := queueLike()
	d.Deprecated = "superseded by thing_v2."

	assert.Contains(t, collectionUpdate{d}.Description(), "Deprecated: superseded by thing_v2.")
}

func TestCapitalizeFirstLeavesAcronymsAlone(t *testing.T) {
	cases := map[string]string{
		"queue":         "Queue",
		"RDP":           "RDP",
		"ACL profile":   "ACL profile",
		"queue binding": "Queue binding",
		"":              "",
	}
	for in, want := range cases {
		assert.Equal(t, want, capitalizeFirst(in))
	}
}
