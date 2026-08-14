package ir

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/tools/semp-gen/internal/mdtable"
	"solace-semp-workflow/tools/semp-gen/internal/specmodel"
)

// bodyParam is the create operation's body parameter, which is how the
// extractor learns which definition an object's create payload uses.
func bodyParam(definition string) specmodel.Parameter {
	return specmodel.Parameter{In: "body", Name: "body", Schema: &specmodel.Schema{Ref: "#/definitions/" + definition}}
}

// --- extractOne: scope -----------------------------------------------------

func TestExtractOneStripsTheVPNHopAndMarksVPNScope(t *testing.T) {
	doc := &specmodel.Doc{
		Paths: map[string]specmodel.PathItem{
			"/msgVpns/{msgVpnName}/queues": {
				"post": specmodel.Operation{OperationID: "createMsgVpnQueue", Parameters: []specmodel.Parameter{bodyParam("Queue")}},
			},
			"/msgVpns/{msgVpnName}/queues/{queueName}": {
				"get": specmodel.Operation{},
			},
		},
		Definitions: map[string]specmodel.Definition{
			"Queue": {Properties: map[string]specmodel.Property{
				"queueName":     {Type: "string"},
				"egressEnabled": {Type: "boolean"},
			}},
		},
	}

	objs, err := extractOne(doc)
	require.NoError(t, err)
	require.Len(t, objs, 1)
	o := objs[0]

	assert.Equal(t, ScopeVPN, o.Scope)
	// The VPN hop is a scope, supplied by the runtime, not an ancestor: it must
	// never surface as a ParentHop the way a real nested collection does.
	assert.Empty(t, o.Parents)
	require.Len(t, o.OwnKey, 1)
	assert.Equal(t, "queueName", o.OwnKey[0].Name)
}

func TestExtractOneBrokerGlobalObjectHasNoVPNHop(t *testing.T) {
	doc := &specmodel.Doc{
		Paths: map[string]specmodel.PathItem{
			"/clientCertAuthorities": {
				"post": specmodel.Operation{OperationID: "createClientCertAuthority", Parameters: []specmodel.Parameter{bodyParam("ClientCertAuthority")}},
			},
			"/clientCertAuthorities/{clientCertAuthorityName}": {
				"get": specmodel.Operation{},
			},
		},
		Definitions: map[string]specmodel.Definition{
			"ClientCertAuthority": {Properties: map[string]specmodel.Property{
				"clientCertAuthorityName": {Type: "string"},
				"revocationCheckEnabled":  {Type: "boolean"},
			}},
		},
	}

	objs, err := extractOne(doc)
	require.NoError(t, err)
	require.Len(t, objs, 1)
	o := objs[0]

	assert.Equal(t, ScopeGlobal, o.Scope)
	assert.Empty(t, o.Parents)
	require.Len(t, o.OwnKey, 1)
	assert.Equal(t, "clientCertAuthorityName", o.OwnKey[0].Name)
}

// --- extractOne: nesting and keys -------------------------------------------

func TestExtractOneNestedChildGetsAParentHopAndItsOwnKeyFromTheItemPath(t *testing.T) {
	doc := &specmodel.Doc{
		Paths: map[string]specmodel.PathItem{
			"/msgVpns/{msgVpnName}/things/{thingName}/subThings": {
				"post": specmodel.Operation{OperationID: "createMsgVpnThingSubThing", Parameters: []specmodel.Parameter{bodyParam("SubThing")}},
			},
			"/msgVpns/{msgVpnName}/things/{thingName}/subThings/{subThingName}": {
				"get": specmodel.Operation{},
			},
		},
		Definitions: map[string]specmodel.Definition{
			"SubThing": {Properties: map[string]specmodel.Property{
				"subThingName": {Type: "string"},
				"label":        {Type: "string"},
			}},
		},
	}

	objs, err := extractOne(doc)
	require.NoError(t, err)
	require.Len(t, objs, 1)
	o := objs[0]

	require.Len(t, o.Parents, 1)
	assert.Equal(t, "things", o.Parents[0].Collection)
	require.Len(t, o.Parents[0].Keys, 1)
	assert.Equal(t, "thingName", o.Parents[0].Keys[0].Name)

	require.Len(t, o.OwnKey, 1)
	assert.Equal(t, "subThingName", o.OwnKey[0].Name)
}

func TestExtractOneSplitsACompositeOwnKeyAndACompositeParentKeyOnTheComma(t *testing.T) {
	doc := &specmodel.Doc{
		Paths: map[string]specmodel.PathItem{
			"/msgVpns/{msgVpnName}/bridges/{bridgeName},{bridgeVirtualRouter}/remoteMsgVpns": {
				"post": specmodel.Operation{OperationID: "createMsgVpnBridgeRemoteMsgVpn", Parameters: []specmodel.Parameter{bodyParam("RemoteMsgVpn")}},
			},
			"/msgVpns/{msgVpnName}/bridges/{bridgeName},{bridgeVirtualRouter}/remoteMsgVpns/{remoteMsgVpnName},{remoteMsgVpnLocation},{remoteMsgVpnInterface}": {
				"get": specmodel.Operation{},
			},
		},
		Definitions: map[string]specmodel.Definition{
			"RemoteMsgVpn": {Properties: map[string]specmodel.Property{
				"remoteMsgVpnName":      {Type: "string"},
				"remoteMsgVpnLocation":  {Type: "string"},
				"remoteMsgVpnInterface": {Type: "string"},
				"enabled":               {Type: "boolean"},
			}},
		},
	}

	objs, err := extractOne(doc)
	require.NoError(t, err)
	require.Len(t, objs, 1)
	o := objs[0]

	require.Len(t, o.Parents, 1)
	require.Len(t, o.Parents[0].Keys, 2)
	assert.Equal(t, "bridgeName", o.Parents[0].Keys[0].Name)
	assert.Equal(t, "bridgeVirtualRouter", o.Parents[0].Keys[1].Name)

	require.Len(t, o.OwnKey, 3)
	assert.Equal(t, "remoteMsgVpnName", o.OwnKey[0].Name)
	assert.Equal(t, "remoteMsgVpnLocation", o.OwnKey[1].Name)
	assert.Equal(t, "remoteMsgVpnInterface", o.OwnKey[2].Name)
}

// --- extractOne: the Broker singleton ---------------------------------------

func TestExtractBrokerRequiresAPatchOperation(t *testing.T) {
	_, err := extractBroker(&specmodel.Doc{}, specmodel.PathItem{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not updatable")
}

func TestExtractOneBuildsTheBrokerSingletonFromTheRootPath(t *testing.T) {
	doc := &specmodel.Doc{
		Paths: map[string]specmodel.PathItem{
			"/": {
				"patch": specmodel.Operation{Parameters: []specmodel.Parameter{bodyParam("Broker")}},
			},
		},
		Definitions: map[string]specmodel.Definition{
			"Broker": {Properties: map[string]specmodel.Property{"guaranteedMsgingEnabled": {Type: "boolean"}}},
		},
	}

	objs, err := extractOne(doc)
	require.NoError(t, err)
	require.Len(t, objs, 1)
	o := objs[0]

	// The singleton has no create, so it is never a candidate for a Parent hop
	// or a key -- it is addressed by the empty path.
	assert.True(t, o.IsSingleton)
	assert.True(t, o.HasUpdate)
	assert.Equal(t, ScopeGlobal, o.Scope)
}

// --- extractOne: HasUpdate ---------------------------------------------------

func TestExtractCollectionHasUpdateComesFromTheItemPathOnly(t *testing.T) {
	cases := []struct {
		name       string
		itemOps    specmodel.PathItem
		collOps    specmodel.PathItem
		wantUpdate bool
	}{
		{
			name:       "a patch on the item path grants update",
			itemOps:    specmodel.PathItem{"get": specmodel.Operation{}, "patch": specmodel.Operation{}},
			wantUpdate: true,
		},
		{
			// Swagger has no notion of PATCHing a collection, but the extractor
			// must read HasUpdate from the item path's verbs and nowhere else, so
			// a stray verb on the collection path can never grant it by accident.
			name:       "a patch on the collection path itself does not count",
			itemOps:    specmodel.PathItem{"get": specmodel.Operation{}},
			collOps:    specmodel.PathItem{"patch": specmodel.Operation{}},
			wantUpdate: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collPathItem := specmodel.PathItem{
				"post": specmodel.Operation{OperationID: "createMsgVpnThing", Parameters: []specmodel.Parameter{bodyParam("Thing")}},
			}
			for k, v := range tc.collOps {
				collPathItem[k] = v
			}
			doc := &specmodel.Doc{
				Paths: map[string]specmodel.PathItem{
					"/msgVpns/{msgVpnName}/things":             collPathItem,
					"/msgVpns/{msgVpnName}/things/{thingName}": tc.itemOps,
				},
				Definitions: map[string]specmodel.Definition{
					"Thing": {Properties: map[string]specmodel.Property{"thingName": {Type: "string"}}},
				},
			}

			objs, err := extractOne(doc)
			require.NoError(t, err)
			require.Len(t, objs, 1)
			assert.Equal(t, tc.wantUpdate, objs[0].HasUpdate)
		})
	}
}

// --- attributes(): filtering -------------------------------------------------

func TestAttributesExcludesPathKeysAndReadOnlyProperties(t *testing.T) {
	doc := &specmodel.Doc{
		Definitions: map[string]specmodel.Definition{
			"Thing": {Properties: map[string]specmodel.Property{
				"id":           {Type: "string"},
				"internalFlag": {Type: "boolean"},
				"label":        {Type: "string"},
			}},
		},
	}
	ops := specmodel.PathItem{
		"post": specmodel.Operation{Description: "" +
			"Attribute | Read-Only\n" +
			":--- | :---:\n" +
			"internalFlag | x\n",
		},
	}

	out, _, err := attributes(doc, "Thing", ops, map[string]bool{"id": true})
	require.NoError(t, err)
	require.Len(t, out, 1)
	// "id" is excluded because it is a path key, represented by Parents/OwnKey
	// instead of a settable body attribute. "internalFlag" is excluded because
	// the broker never accepts it on a request, so offering it as a parameter
	// would only invite a rejected call.
	assert.Equal(t, "label", out[0].Name)
}

// --- parseDefault -------------------------------------------------------------

func TestParseDefaultReadsTheProseSentence(t *testing.T) {
	cases := []struct {
		name        string
		description string
		kind        Kind
		want        any
	}{
		{"bool true", "The default value is `true`.", KindBool, true},
		{"bool false", "The default value is `false`.", KindBool, false},
		{"integer", "The default value is `42`.", KindInt, 42},
		{"quoted string", "The default value is `\"idle\"`.", KindString, "idle"},
		// An empty quoted default carries no information the docs table can act
		// on, so it must not surface as a false-y non-nil default value.
		{"empty quoted string yields nil", "The default value is `\"\"`.", KindString, nil},
		{"no documented default yields nil", "This attribute has no stated default.", KindString, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseDefault(tc.description, tc.kind))
		})
	}
}

// --- describe -----------------------------------------------------------------

func TestDescribeTrimsBoilerplateAfterTheFirstBlankLine(t *testing.T) {
	const body = "The first paragraph is the real documentation."
	full := body + "\n\nThis is the access-scope and config-sync boilerplate repeated on every attribute in the specification."

	assert.Equal(t, body, describe(full, nil))
}

func TestDescribeAppendsTheOperationalNoteForEachFlag(t *testing.T) {
	const body = "What the attribute controls."
	cases := []struct {
		name string
		flag string
		note string
	}{
		{"const", "Const", "Settable only when the object is created."},
		{"write-only", "Write-Only", "Never returned by the broker, so it is re-sent on every update."},
		{"requires-disable", "Requires-Disable", "The broker rejects a change to this while the service is enabled; disable it first."},
		{"auto-disable", "Auto-Disable", "Changing this may briefly interrupt the affected service."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := describe(body, mdtable.Flags{tc.flag: true})
			assert.Equal(t, body+" "+tc.note, got)
		})
	}
}

// --- buildAttribute -----------------------------------------------------------

func TestBuildAttributeResolvesANestedRefAndAppliesCombosByNestedDefinition(t *testing.T) {
	doc := &specmodel.Doc{
		Definitions: map[string]specmodel.Definition{
			"EventThreshold": {Properties: map[string]specmodel.Property{
				"clearPercent": {Type: "integer"},
				"setPercent":   {Type: "integer"},
			}},
		},
	}
	combos := map[string][]mdtable.Combination{
		// Keyed on the nested definition name, not the outer attribute's own
		// name: the same threshold group is embedded under many different
		// attribute names, but its co-dependency rule is defined once.
		"EventThreshold": {
			{Attribute: "clearPercent", Requires: []string{"setPercent"}},
			{Attribute: "setPercent", Requires: []string{"clearPercent"}, Conflicts: []string{"disablePercent"}},
		},
	}
	prop := specmodel.Property{Ref: "#/definitions/EventThreshold"}

	attr, err := buildAttribute(doc, "eventBindCountThreshold", prop, nil, combos)
	require.NoError(t, err)
	assert.Equal(t, KindObject, attr.Kind)
	require.Len(t, attr.Fields, 2)

	byName := map[string]Attribute{}
	for _, f := range attr.Fields {
		byName[f.Name] = f
	}
	assert.Equal(t, []string{"setPercent"}, byName["clearPercent"].Requires)
	assert.Equal(t, []string{"clearPercent"}, byName["setPercent"].Requires)
	assert.Equal(t, []string{"disablePercent"}, byName["setPercent"].Conflicts)
}

func TestBuildAttributeRejectsAnUnsupportedType(t *testing.T) {
	doc := &specmodel.Doc{}
	prop := specmodel.Property{Type: "array"}

	_, err := buildAttribute(doc, "weirdList", prop, nil, nil)

	require.Error(t, err)
	// The runtime payload model has no representation for a bare array, so this
	// is a deliberate fail-loud path: it must name the attribute rather than
	// silently dropping it or falling back to a string.
	assert.Contains(t, err.Error(), "weirdList")
}

// --- merge.go: union of software and appliance -------------------------------

func TestUnionTagsObjectsByWhichPlatformDefinesThem(t *testing.T) {
	cases := []struct {
		name         string
		software     []Object
		appliance    []Object
		wantPlatform Platform
	}{
		{"only in software", []Object{{Definition: "A"}}, nil, PlatformSoftwareOnly},
		{"only in appliance", nil, []Object{{Definition: "A"}}, PlatformApplianceOnly},
		{"in both", []Object{{Definition: "A"}}, []Object{{Definition: "A"}}, PlatformBoth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := union(tc.software, tc.appliance)
			require.Len(t, out, 1)
			assert.Equal(t, tc.wantPlatform, out[0].Platform)
		})
	}
}

func TestUnionAttributesTagsThePlatformSpecificOneAndNotesItsDescription(t *testing.T) {
	cases := []struct {
		name         string
		software     []Attribute
		appliance    []Attribute
		onlyAttr     string
		wantPlatform Platform
		wantNote     string
	}{
		{
			name:         "software-only attribute",
			software:     []Attribute{{Name: "shared"}, {Name: "swOnly", Desc: "sw desc"}},
			appliance:    []Attribute{{Name: "shared"}},
			onlyAttr:     "swOnly",
			wantPlatform: PlatformSoftwareOnly,
			wantNote:     "Software broker only.",
		},
		{
			name:         "appliance-only attribute",
			software:     []Attribute{{Name: "shared"}},
			appliance:    []Attribute{{Name: "shared"}, {Name: "apOnly", Desc: "ap desc"}},
			onlyAttr:     "apOnly",
			wantPlatform: PlatformApplianceOnly,
			wantNote:     "Appliance only.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := unionAttributes(tc.software, tc.appliance)
			byName := map[string]Attribute{}
			for _, a := range out {
				byName[a.Name] = a
			}
			// The attribute both platforms document is not narrowed to either one.
			assert.Equal(t, PlatformBoth, byName["shared"].Platform)
			assert.Equal(t, tc.wantPlatform, byName[tc.onlyAttr].Platform)
			assert.Contains(t, byName[tc.onlyAttr].Desc, tc.wantNote)
		})
	}
}

func TestUnionEnumKeepsTheWiderSetsOrder(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want []string
	}{
		{"a empty returns b unchanged", nil, []string{"x", "y"}, []string{"x", "y"}},
		{"b empty returns a unchanged", []string{"x", "y"}, nil, []string{"x", "y"}},
		{
			name: "b is wider, so b's order leads and a's extra values are appended",
			a:    []string{"b", "a"},
			b:    []string{"a", "b", "c"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "a is at least as wide, so a's order leads",
			a:    []string{"z", "y", "x"},
			b:    []string{"x", "w"},
			want: []string{"z", "y", "x", "w"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, unionEnum(tc.a, tc.b))
		})
	}
}

// --- legacy.go ----------------------------------------------------------------

func TestApplyLegacyPinsOverlayFieldsAndDropsSkippedAttributes(t *testing.T) {
	// applyLegacy reads legacyOverlays by SEMP definition name. Rather than
	// repurpose one of the real ten families (none of which exercises every field
	// at once), a throwaway entry is added for the duration of this test: the
	// point here is the overlay mechanism, not the real table's content.
	const definition = "ExtractTestProbe"
	legacyOverlays[definition] = legacyOverlay{
		FriendlyName:       "probe",
		Article:            "an",
		ParentFriendlyName: "harness",
		Messages:           map[string]string{"Created": "Probe created"},
		KeyDefaults:        map[string]string{"probeKey": "default-key"},
		KeyDescs:           map[string]string{"probeKey": "pinned key desc"},
		DisplayKey:         1,
		SkipAttributes:     []string{"derived"},
	}
	t.Cleanup(func() { delete(legacyOverlays, definition) })

	o := Object{
		Definition:   definition,
		Prefix:       Prefix(definition),
		FriendlyName: "spec-derived name",
		Article:      "a",
		OwnKey:       []Key{{Name: "probeKey", Desc: "spec-derived desc"}},
		Attributes: []Attribute{
			{Name: "kept"},
			{Name: "derived"},
		},
	}
	applyLegacy(&o)

	assert.Equal(t, "probe", o.FriendlyName)
	assert.Equal(t, "an", o.Article)
	assert.Equal(t, "harness", o.ParentFriendlyName)
	assert.Equal(t, map[string]string{"Created": "Probe created"}, o.Messages)
	assert.Equal(t, 1, o.DisplayKey)
	require.Len(t, o.OwnKey, 1)
	assert.Equal(t, "default-key", o.OwnKey[0].Default)
	assert.Equal(t, "pinned key desc", o.OwnKey[0].Desc)
	// "derived" is a payload-hook output, so it must not remain a settable
	// parameter after the overlay is applied.
	assert.Equal(t, []Attribute{{Name: "kept"}}, o.Attributes)
	assert.Equal(t, []string{"derived"}, o.SkipHookDerived)
}

func TestApplyLegacyLeavesAnObjectWithNoOverlayUntouched(t *testing.T) {
	o := Object{Definition: "NoSuchOverlay", Prefix: "no_such_overlay", FriendlyName: "spec name", Article: "a"}
	applyLegacy(&o)

	assert.Equal(t, "spec name", o.FriendlyName)
	assert.Equal(t, "a", o.Article)
	assert.Nil(t, o.Messages)
}

func TestCheckLegacyMatchedErrorsWhenAnOverlayDefinitionMatchesNoObject(t *testing.T) {
	all := LegacyDefinitions()
	require.NotEmpty(t, all, "the overlay table is expected to carry the ten backward-compatibility families")

	full := make([]Object, 0, len(all))
	for _, p := range all {
		full = append(full, Object{Definition: p})
	}
	assert.NoError(t, checkLegacyMatched(full), "every overlay definition has a matching object")

	// Drop one, simulating a rename upstream that the extractor no longer
	// produces: the overlay's backward-compatibility guarantee would silently
	// stop applying unless this is caught.
	missing := all[0]
	dropped := full[1:]

	err := checkLegacyMatched(dropped)
	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)
}
