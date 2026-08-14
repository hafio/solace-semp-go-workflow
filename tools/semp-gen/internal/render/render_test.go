package render

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/tools/semp-gen/internal/ir"
)

var update = flag.Bool("update", false, "rewrite the golden file in testdata/")

// buildGoldenSpec assembles the shapes render.go branches on: a simple
// VPN-scoped object, a broker-global object, the Broker singleton, a nested
// child with composite keys on both hops, a nested KindObject attribute
// group, an object with message overrides/a defaulted key/DisplayKey/
// Deprecated, and an attribute exercising Enum, Requires, Conflicts,
// WriteOnly and Opaque together. Each shape gets its own object so a golden
// mismatch points at one concern instead of an entangled diff.
func buildGoldenSpec() *ir.Spec {
	return &ir.Spec{
		Version: "TEST-1.0.0",
		Objects: []ir.Object{
			{
				// The Broker singleton: no key, no collection, IsSingleton only.
				Definition:   "Broker",
				Prefix:       "broker",
				FriendlyName: "broker",
				Scope:        ir.ScopeGlobal,
				IsSingleton:  true,
				HasUpdate:    true,
				Attributes: []ir.Attribute{
					// Canon is emitted only where it differs from Name, so a scalar
					// with a camelCase name carries it and a single-word one below
					// does not.
					{Name: "maintenanceModeEnabled", Canon: "maintenance_mode_enabled", Kind: ir.KindBool, Desc: "Enable or disable maintenance mode"},
				},
			},
			{
				// Broker-global: no msgVpns hop, add/delete only (HasUpdate false).
				Definition:   "Endpoint",
				Prefix:       "endpoint",
				FriendlyName: "endpoint",
				Scope:        ir.ScopeGlobal,
				Collection:   "endpoints",
				OwnKey:       []ir.Key{{Name: "endpointName", Desc: "Name of the endpoint"}},
				Attributes: []ir.Attribute{
					{Name: "active", Kind: ir.KindBool, Desc: "Whether the endpoint is active"},
				},
			},
			{
				// Simple VPN-scoped object, one key, one scalar attribute of each kind.
				Definition:   "Widget",
				Prefix:       "widget",
				FriendlyName: "widget",
				Scope:        ir.ScopeVPN,
				Collection:   "widgets",
				OwnKey:       []ir.Key{{Name: "widgetName", Canon: "widget_name", Desc: "Name of the widget"}},
				HasUpdate:    true,
				Attributes: []ir.Attribute{
					{Name: "label", Kind: ir.KindString, Desc: "A label"},
					{Name: "count", Kind: ir.KindInt, Desc: "A count"},
					{Name: "enabled", Kind: ir.KindBool, Desc: "Whether the widget is enabled"},
				},
			},
			{
				// Nested child: a two-part composite parent hop and a two-part
				// composite own key.
				Definition:         "WidgetGauge",
				Prefix:              "widget_gauge",
				FriendlyName:        "gauge",
				Scope:               ir.ScopeVPN,
				Parents:             []ir.ParentHop{{Collection: "widgets", Keys: []ir.Key{{Name: "widgetName"}, {Name: "widgetRegion"}}}},
				ParentFriendlyName:  "widget",
				Collection:          "gauges",
				OwnKey:              []ir.Key{{Name: "gaugeName"}, {Name: "gaugeUnit"}},
				HasUpdate:           true,
				Attributes: []ir.Attribute{
					{Name: "enabled", Kind: ir.KindBool, Desc: "Whether the gauge is enabled"},
				},
			},
			{
				// A nested KindObject attribute group, mirroring SEMP's
				// event-threshold shape (an object attribute with its own Fields).
				Definition:   "GaugeGroup",
				Prefix:       "gauge_group",
				FriendlyName: "gauge group",
				Scope:        ir.ScopeVPN,
				Collection:   "gaugeGroups",
				OwnKey:       []ir.Key{{Name: "gaugeGroupName"}},
				HasUpdate:    true,
				Attributes: []ir.Attribute{
					{Name: "eventThreshold", Canon: "event_threshold", Kind: ir.KindObject, Desc: "Threshold settings", Fields: []ir.Attribute{
						{Name: "clearPercent", Canon: "clear_percent", Kind: ir.KindInt, Desc: "Clear percent"},
						{Name: "setPercent", Canon: "set_percent", Kind: ir.KindInt, Desc: "Set percent"},
					}},
				},
			},
			{
				// Message overrides, a key Default, DisplayKey and Deprecated, and
				// zero attributes (so the Fields block is skipped entirely).
				Definition:   "MsgVpnLegacyFlag",
				Prefix:       "legacy_flag",
				FriendlyName: "legacy flag",
				Scope:        ir.ScopeVPN,
				Collection:   "legacyFlags",
				OwnKey: []ir.Key{
					{Name: "flagSyntax", Default: "legacy"},
					{Name: "flagValue"},
				},
				DisplayKey: 2,
				Deprecated: "superseded by legacy_flag_v2.",
				Messages: map[string]string{
					"MissingArg": "Custom missing arg message for {obj}",
					"Created":    "Custom created message for {obj} '{name}'",
				},
			},
			{
				// One attribute carrying Enum, Requires, Conflicts, WriteOnly and
				// Opaque together, to exercise every optional field on one line.
				Definition:   "FlagAttr",
				Prefix:       "flag_attr",
				FriendlyName: "flag",
				Scope:        ir.ScopeVPN,
				Collection:   "flagAttrs",
				OwnKey:       []ir.Key{{Name: "flagAttrName"}},
				Attributes: []ir.Attribute{
					{
						Name: "mode", Kind: ir.KindString,
						Enum:      []string{"auto", "manual"},
						WriteOnly: true, Opaque: true,
						Requires:  []string{"modeReason"},
						Conflicts: []string{"legacyMode"},
						Desc:      "Operating mode",
					},
				},
			},
		},
	}
}

// dumpFiles renders every file to one comparable text block, delimited by its
// filename, so a single golden file covers every shape without one file per
// case.
func dumpFiles(files []File) string {
	parts := make([]string, len(files))
	for i, f := range files {
		parts[i] = fmt.Sprintf("=== %s ===\n%s", f.Name, Bytes(&f))
	}
	return strings.Join(parts, "\n\n") + "\n"
}

// TestObjectGolden is the primary defence for this package: a change to field
// order, indentation, or which fields are omitted when zero shows up as a
// diff here instead of silently reordering 86 real descriptor files. Run
// with -update to rewrite the golden after a deliberate render.go change.
func TestObjectGolden(t *testing.T) {
	spec := buildGoldenSpec()
	files, err := All(spec)
	require.NoError(t, err)

	got := dumpFiles(files)
	goldenPath := filepath.Join("testdata", "objects.golden")

	if *update {
		require.NoError(t, os.WriteFile(goldenPath, []byte(got), 0o644))
	}

	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

// TestObjectsAreValidGofmtStableGo parses the renderer's own output with
// go/parser, which catches a malformed literal (a missing comma, an
// unbalanced brace) that a golden-file diff would show but not explain, and
// confirms reformatting it is a no-op -- the renderer must never emit
// something gofmt itself would then rewrite.
func TestObjectsAreValidGofmtStableGo(t *testing.T) {
	spec := buildGoldenSpec()
	files, err := All(spec)
	require.NoError(t, err)

	for _, f := range files {
		t.Run(f.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			_, err := parser.ParseFile(fset, f.Name, f.Content, parser.AllErrors)
			require.NoErrorf(t, err, "rendered output for %s does not parse as Go", f.Name)

			again, err := format.Source(f.Content)
			require.NoError(t, err)
			assert.Equal(t, string(f.Content), string(again), "%s changed under a second gofmt pass", f.Name)
		})
	}
}

// TestManifestNamesAreSortedAndCountMatchesComment builds three objects in
// an order chosen NOT to already be sorted, so a renderer that forgot to
// sort would pass a golden-only check by accident but fail this one.
func TestManifestNamesAreSortedAndCountMatchesComment(t *testing.T) {
	spec := &ir.Spec{
		Version: "v1",
		Objects: []ir.Object{
			{Prefix: "zebra", FriendlyName: "zebra", Scope: ir.ScopeVPN, Collection: "zebras", OwnKey: []ir.Key{{Name: "zebraName"}}},
			{Prefix: "apple", FriendlyName: "apple", Scope: ir.ScopeVPN, Collection: "apples", OwnKey: []ir.Key{{Name: "appleName"}}, HasUpdate: true},
			{Prefix: "mango", FriendlyName: "mango", Scope: ir.ScopeGlobal, IsSingleton: true},
		},
	}
	f, err := Manifest(spec)
	require.NoError(t, err)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, f.Name, f.Content, parser.ParseComments)
	require.NoError(t, err)

	var got []string
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, s := range gd.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "ExpectedModules" {
				continue
			}
			lit := vs.Values[0].(*ast.CompositeLit)
			for _, elt := range lit.Elts {
				s, err := strconv.Unquote(elt.(*ast.BasicLit).Value)
				require.NoError(t, err)
				got = append(got, s)
			}
		}
	}

	want := []string{
		"apple.add", "apple.delete", "apple.update",
		"mango.update",
		"zebra.add", "zebra.delete",
	}
	assert.Equal(t, want, got, "ExpectedModules must be alphabetically sorted, not spec.Objects order")

	m := regexp.MustCompile(`register \((\d+)\)`).FindStringSubmatch(string(f.Content))
	require.NotNil(t, m, "doc comment must state a count")
	assert.Equal(t, fmt.Sprintf("%d", len(want)), m[1], "the stated count must match the number of entries actually emitted")
}

// TestAllOneFilePerObjectPlusManifest guards the file count All() promises:
// every object gets exactly one file, plus one manifest, never more or fewer.
func TestAllOneFilePerObjectPlusManifest(t *testing.T) {
	spec := &ir.Spec{
		Version: "v1",
		Objects: []ir.Object{
			{Prefix: "one", FriendlyName: "one", Scope: ir.ScopeVPN, Collection: "ones", OwnKey: []ir.Key{{Name: "oneName"}}},
			{Prefix: "two", FriendlyName: "two", Scope: ir.ScopeVPN, Collection: "twos", OwnKey: []ir.Key{{Name: "twoName"}}},
		},
	}
	files, err := All(spec)
	require.NoError(t, err)

	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	assert.ElementsMatch(t, []string{"gen_one.go", "gen_two.go", "manifest_generated.go"}, names)
}

// TestAllErrorsOnDuplicateFilename is the guard render.go documents: two
// objects sharing a Prefix would render to the same gen_*.go file and one
// would silently clobber the other on disk. All() must refuse instead.
func TestAllErrorsOnDuplicateFilename(t *testing.T) {
	spec := &ir.Spec{
		Version: "v1",
		Objects: []ir.Object{
			{Prefix: "dup", FriendlyName: "first", Scope: ir.ScopeVPN, Collection: "dups", OwnKey: []ir.Key{{Name: "dupName"}}},
			{Prefix: "dup", FriendlyName: "second", Scope: ir.ScopeVPN, Collection: "dups", OwnKey: []ir.Key{{Name: "dupName"}}},
		},
	}
	_, err := All(spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gen_dup.go")
}

// TestDescriptionSurvivesQuotesBackslashesAndNewlines is exactly why the
// renderer quotes descriptions with strconv.Quote instead of wrapping them in
// literal double quotes: any of these three characters, un-escaped, would
// either break the generated source or truncate the string at the first
// embedded quote.
func TestDescriptionSurvivesQuotesBackslashesAndNewlines(t *testing.T) {
	const desc = `has "quotes", a backslash \ and a` + "\n" + "newline"

	o := ir.Object{
		Definition: "Gizmo", Prefix: "gizmo", FriendlyName: "gizmo", Scope: ir.ScopeVPN,
		Collection: "gizmos", OwnKey: []ir.Key{{Name: "gizmoName"}},
		Attributes: []ir.Attribute{{Name: "note", Kind: ir.KindString, Desc: desc}},
	}
	f, err := Object(o, "v1")
	require.NoError(t, err)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, f.Name, f.Content, parser.AllErrors)
	require.NoError(t, err, "an unescaped special character in Desc would make this fail to parse")

	got := findAttrDesc(t, file, "note")
	assert.Equal(t, desc, got, "the description must round-trip exactly through the generated source")
}

// findAttrDesc locates the fieldDesc literal whose Name matches, and returns
// its unquoted Desc value.
func findAttrDesc(t *testing.T, file *ast.File, name string) string {
	t.Helper()
	want := strconv.Quote(name)
	var found string
	var ok bool
	ast.Inspect(file, func(n ast.Node) bool {
		lit, isLit := n.(*ast.CompositeLit)
		if !isLit {
			return true
		}
		var isMatch bool
		var descVal string
		for _, elt := range lit.Elts {
			kv, isKV := elt.(*ast.KeyValueExpr)
			if !isKV {
				continue
			}
			key, isIdent := kv.Key.(*ast.Ident)
			if !isIdent {
				continue
			}
			switch key.Name {
			case "Name":
				if bl, isBL := kv.Value.(*ast.BasicLit); isBL && bl.Value == want {
					isMatch = true
				}
			case "Desc":
				if bl, isBL := kv.Value.(*ast.BasicLit); isBL {
					descVal = bl.Value
				}
			}
		}
		if isMatch && descVal != "" {
			unquoted, err := strconv.Unquote(descVal)
			require.NoError(t, err)
			found = unquoted
			ok = true
		}
		return true
	})
	require.Truef(t, ok, "no fieldDesc with Name %s found", want)
	return found
}

// TestArticleOmittedForIndefiniteArticleA covers the renderer's shortcut: "a"
// is the runtime's own default, so writing it out on every one of 86 objects
// would be pure noise.
func TestArticleOmittedForIndefiniteArticleA(t *testing.T) {
	o := ir.Object{
		Definition: "Thing", Prefix: "thing", FriendlyName: "thing", Article: "a",
		Scope: ir.ScopeVPN, Collection: "things", OwnKey: []ir.Key{{Name: "thingName"}},
	}
	f, err := Object(o, "v1")
	require.NoError(t, err)
	assert.NotContains(t, string(f.Content), "Article:")
}

// TestArticleEmittedForIndefiniteArticleAn is the case the shortcut above
// must not swallow: an object whose FriendlyName starts with a vowel sound
// needs "an" stated, since no rule derives it from the spelling alone (e.g.
// "RDP" is spelled out and takes "an" despite starting with a consonant
// letter).
func TestArticleEmittedForIndefiniteArticleAn(t *testing.T) {
	o := ir.Object{
		Definition: "RDP", Prefix: "rdp", FriendlyName: "RDP", Article: "an",
		Scope: ir.ScopeVPN, Collection: "rdps", OwnKey: []ir.Key{{Name: "rdpName"}},
	}
	f, err := Object(o, "v1")
	require.NoError(t, err)
	assert.Contains(t, string(f.Content), `Article:      "an",`)
}
