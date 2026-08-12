package ir

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"solace-semp-workflow/tools/semp-gen/internal/mdtable"
	"solace-semp-workflow/tools/semp-gen/internal/specmodel"
)

// brokerPath is the Broker singleton: the config root itself, with no create and
// no key.
const brokerPath = "/"

// Extract builds the merged representation from the software and appliance
// specifications. The appliance spec is not a subset of the software one -- it
// carries attributes and a wider enum the software spec lacks -- so both are
// extracted and unioned, with anything platform-specific tagged.
func Extract(software, appliance *specmodel.Doc) (*Spec, error) {
	sw, err := extractOne(software)
	if err != nil {
		return nil, fmt.Errorf("software spec: %w", err)
	}
	ap, err := extractOne(appliance)
	if err != nil {
		return nil, fmt.Errorf("appliance spec: %w", err)
	}

	objects := union(sw, ap)

	ids := map[string]bool{}
	for _, o := range objects {
		if o.OperationID != "" {
			ids[o.OperationID] = true
		}
	}
	if err := ValidateAliases(ids); err != nil {
		return nil, err
	}

	for i := range objects {
		applyLegacy(&objects[i])
	}
	if err := CheckPrefixes(objects); err != nil {
		return nil, err
	}
	if err := checkLegacyMatched(objects); err != nil {
		return nil, err
	}

	sort.Slice(objects, func(i, j int) bool { return objects[i].Prefix < objects[j].Prefix })
	return &Spec{Version: software.Info.Version, Objects: objects}, nil
}

// checkLegacyMatched reports overlay entries that matched no object, which would
// mean a backward-compatibility guarantee had silently stopped applying.
func checkLegacyMatched(objects []Object) error {
	got := map[string]bool{}
	for _, o := range objects {
		got[o.Prefix] = true
	}
	var missing []string
	for _, p := range LegacyPrefixes() {
		if !got[p] {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("legacy overlay entries matched no object, so their backward-compatibility "+
		"guarantee no longer applies: %s", strings.Join(missing, ", "))
}

// extractOne walks one specification into objects.
func extractOne(doc *specmodel.Doc) ([]Object, error) {
	items := itemPaths(doc)

	var out []Object
	for path, ops := range doc.Paths {
		if path == brokerPath {
			o, err := extractBroker(doc, ops)
			if err != nil {
				return nil, err
			}
			out = append(out, *o)
			continue
		}
		post, ok := ops["post"]
		if !ok {
			continue
		}
		o, err := extractCollection(doc, path, post, items[path])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, *o)
	}
	return out, nil
}

// itemPaths maps a collection path to its item path, the one ending in a key
// segment. An object's update and delete availability comes from the item path's
// verbs, not the collection's.
func itemPaths(doc *specmodel.Doc) map[string]string {
	out := map[string]string{}
	for p := range doc.Paths {
		segs := pathSegments(p)
		if len(segs) == 0 || !isKeySegment(segs[len(segs)-1]) {
			continue
		}
		out["/"+strings.Join(segs[:len(segs)-1], "/")] = p
	}
	return out
}

func pathSegments(p string) []string {
	var out []string
	for _, s := range strings.Split(strings.Trim(p, "/"), "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func isKeySegment(s string) bool { return strings.HasPrefix(s, "{") }

// keyNames splits a path segment into its key names, so "{a},{b}" yields a and b.
// SEMP renders a composite key as one comma-joined segment.
func keyNames(segment string) []string {
	var out []string
	for _, part := range strings.Split(segment, ",") {
		out = append(out, strings.Trim(part, "{}"))
	}
	return out
}

// extractBroker builds the singleton, which supports update only.
func extractBroker(doc *specmodel.Doc, ops specmodel.PathItem) (*Object, error) {
	patch, ok := ops["patch"]
	if !ok {
		return nil, fmt.Errorf("%s: no patch operation, so the Broker object is not updatable", brokerPath)
	}
	def := patch.BodyDefinition()
	if def == "" {
		return nil, fmt.Errorf("%s: patch has no body definition", brokerPath)
	}
	attrs, _, err := attributes(doc, def, ops, nil)
	if err != nil {
		return nil, err
	}
	return &Object{
		Definition:   def,
		Prefix:       "broker",
		FriendlyName: "broker",
		Article:      "a",
		Scope:        ScopeGlobal,
		IsSingleton:  true,
		HasUpdate:    true,
		Attributes:   attrs,
	}, nil
}

// extractCollection builds one creatable object from its collection path.
func extractCollection(doc *specmodel.Doc, path string, post specmodel.Operation, itemPath string) (*Object, error) {
	if itemPath == "" {
		return nil, fmt.Errorf("no item path, so the object cannot be addressed individually")
	}
	def := post.BodyDefinition()
	if def == "" {
		return nil, fmt.Errorf("post has no body definition")
	}

	segs := pathSegments(path)
	collection := segs[len(segs)-1]
	if isKeySegment(collection) {
		return nil, fmt.Errorf("collection path ends in a key segment")
	}

	scope := ScopeGlobal
	ancestors := segs[:len(segs)-1]
	if len(ancestors) >= 2 && ancestors[0] == "msgVpns" && keyNames(ancestors[1])[0] == "msgVpnName" {
		// The VPN hop is supplied by the runtime from the connection config, so it
		// is a scope rather than a parent.
		scope = ScopeVPN
		ancestors = ancestors[2:]
	}

	pathDescs := post.PathParams()
	var parents []ParentHop
	for i := 0; i+1 < len(ancestors); i += 2 {
		lit, keySeg := ancestors[i], ancestors[i+1]
		if isKeySegment(lit) || !isKeySegment(keySeg) {
			return nil, fmt.Errorf("unexpected ancestor shape at %q/%q", lit, keySeg)
		}
		hop := ParentHop{Collection: lit}
		for _, k := range keyNames(keySeg) {
			hop.Keys = append(hop.Keys, Key{Name: k, Desc: firstSentence(pathDescs[k])})
		}
		parents = append(parents, hop)
	}

	itemSegs := pathSegments(itemPath)
	var ownKey []Key
	itemDescs := post.PathParams()
	if get, ok := doc.Paths[itemPath]["get"]; ok {
		// The item operation documents its own key parameters; the collection's
		// POST does not name the object's own key at all.
		for k, v := range get.PathParams() {
			itemDescs[k] = v
		}
	}
	for _, k := range keyNames(itemSegs[len(itemSegs)-1]) {
		ownKey = append(ownKey, Key{Name: k, Desc: firstSentence(itemDescs[k])})
	}

	itemOps := doc.Paths[itemPath]
	_, hasUpdate := itemOps["patch"]

	// Flags are merged across every operation that documents this object: the
	// create table gives Required and Const, the update table gives Write-Only,
	// Requires-Disable and Auto-Disable.
	allOps := specmodel.PathItem{"post": post}
	for verb, op := range itemOps {
		allOps["item-"+verb] = op
	}
	attrs, allDeprecated, err := attributes(doc, def, allOps, ownKeyNames(parents, ownKey))
	if err != nil {
		return nil, fmt.Errorf("attributes of %s: %w", def, err)
	}

	parentDef := ""
	if len(parents) > 0 {
		parentDef = parentDefinition(doc, path)
	}
	friendly := friendlyName(def, parentDef)

	o := &Object{
		Definition:         def,
		OperationID:        post.OperationID,
		Prefix:             Prefix(post.OperationID),
		FriendlyName:       friendly,
		Article:            indefiniteArticle(friendly),
		Scope:              scope,
		Parents:            parents,
		ParentFriendlyName: friendlyName(parentDef, ""),
		Collection:         collection,
		OwnKey:             ownKey,
		Attributes:         attrs,
		HasUpdate:          hasUpdate,
	}
	if allDeprecated {
		o.Deprecated = "every attribute of this object is marked deprecated in the SEMP specification."
	}
	return o, nil
}

// parentDefinition is the definition name of the immediate ancestor collection,
// used to trim a child's friendly name and to name the parent in messages.
func parentDefinition(doc *specmodel.Doc, childCollectionPath string) string {
	segs := pathSegments(childCollectionPath)
	// Drop the child's own collection segment and the ancestor's key segment to
	// land on the ancestor's collection path.
	if len(segs) < 3 {
		return ""
	}
	parentColl := "/" + strings.Join(segs[:len(segs)-2], "/")
	if post, ok := doc.Paths[parentColl]["post"]; ok {
		return post.BodyDefinition()
	}
	return ""
}

// ownKeyNames is every path key, which must not be offered as a body attribute
// by the extractor: the runtime derives them from the path instead.
func ownKeyNames(parents []ParentHop, own []Key) map[string]bool {
	out := map[string]bool{"msgVpnName": true}
	for _, p := range parents {
		for _, k := range p.Keys {
			out[k.Name] = true
		}
	}
	for _, k := range own {
		out[k.Name] = true
	}
	return out
}

// attributes builds an object's body attributes from its definition, enriched
// with the flags merged from every operation that documents it. Path keys are
// excluded: they are represented by Parents and OwnKey.
func attributes(doc *specmodel.Doc, def string, ops specmodel.PathItem, pathKeys map[string]bool) ([]Attribute, bool, error) {
	d, ok := doc.Definitions[def]
	if !ok {
		return nil, false, fmt.Errorf("definition %s not found", def)
	}

	flags := map[string]mdtable.Flags{}
	combos := map[string][]mdtable.Combination{}
	for _, op := range ops {
		parsed, err := mdtable.Parse(op.Description)
		if err != nil {
			return nil, false, fmt.Errorf("%s: %w", op.OperationID, err)
		}
		for name, f := range parsed.Attrs {
			if flags[name] == nil {
				flags[name] = mdtable.Flags{}
			}
			for k, v := range f {
				if v {
					flags[name][k] = true
				}
			}
		}
		for _, c := range parsed.Combinations {
			combos[c.Class] = append(combos[c.Class], c)
		}
	}

	names := make([]string, 0, len(d.Properties))
	for n := range d.Properties {
		names = append(names, n)
	}
	sort.Strings(names)

	var out []Attribute
	for _, name := range names {
		if pathKeys[name] {
			continue
		}
		prop := d.Properties[name]
		f := flags[name]

		// An ancestor's key threaded through the URL is Identifying and Read-Only
		// on the child. It is already excluded by pathKeys, but a spec that names
		// one the path does not is a signal worth honouring rather than sending.
		if f["Read-Only"] {
			continue
		}

		attr, err := buildAttribute(doc, name, prop, f, combos)
		if err != nil {
			return nil, false, err
		}
		out = append(out, *attr)
	}

	// Deprecation is judged over every property of the definition, not over the
	// attributes that survive filtering. The superseded ACL exception collections
	// consist entirely of identifying keys, so their Fields are empty and a check
	// on Fields alone would never mark them -- which is precisely where an
	// operator needs the warning, since they sit beside the supported objects.
	allDep := len(names) > 0
	for _, name := range names {
		if !(d.Properties[name].Deprecated || flags[name]["Deprecated"]) {
			allDep = false
			break
		}
	}
	return out, allDep, nil
}

func buildAttribute(doc *specmodel.Doc, name string, prop specmodel.Property,
	f mdtable.Flags, combos map[string][]mdtable.Combination) (*Attribute, error) {
	attr := &Attribute{
		Name:            name,
		Enum:            prop.Enum,
		Desc:            describe(prop.Description, f),
		Deprecated:      prop.Deprecated || f["Deprecated"],
		Required:        f["Required"],
		Const:           f["Const"],
		ReadOnly:        f["Read-Only"],
		WriteOnly:       f["Write-Only"],
		Opaque:          f["Opaque"],
		RequiresDisable: f["Requires-Disable"],
		AutoDisable:     f["Auto-Disable"],
	}

	switch {
	case prop.Ref != "":
		attr.Kind = KindObject
		nested := specmodel.RefName(prop.Ref)
		nd, ok := doc.Definitions[nested]
		if !ok {
			return nil, fmt.Errorf("attribute %s references unknown definition %s", name, nested)
		}
		subNames := make([]string, 0, len(nd.Properties))
		for n := range nd.Properties {
			subNames = append(subNames, n)
		}
		sort.Strings(subNames)
		for _, sn := range subNames {
			sub, err := buildAttribute(doc, sn, nd.Properties[sn], nil, combos)
			if err != nil {
				return nil, err
			}
			// Co-dependency rules are recorded against the nested definition.
			applyCombos(sub, combos[nested])
			attr.Fields = append(attr.Fields, *sub)
		}
	case prop.Type == "string":
		attr.Kind = KindString
		attr.Default = parseDefault(prop.Description, KindString)
	case prop.Type == "integer":
		attr.Kind = KindInt
		attr.Default = parseDefault(prop.Description, KindInt)
	case prop.Type == "boolean":
		attr.Kind = KindBool
		attr.Default = parseDefault(prop.Description, KindBool)
	default:
		// A structured or array attribute would need a payload model the runtime
		// does not have. Verified absent in 10.26.0.8755, so a new one is a
		// deliberate change rather than something to fall back from silently.
		return nil, fmt.Errorf("attribute %s has unsupported type %q (format %q): the runtime payload model "+
			"handles string, integer, boolean and single-level object groups only", name, prop.Type, prop.Format)
	}
	return attr, nil
}

func applyCombos(a *Attribute, cs []mdtable.Combination) {
	for _, c := range cs {
		if c.Attribute != a.Name {
			continue
		}
		a.Requires = append(a.Requires, c.Requires...)
		a.Conflicts = append(a.Conflicts, c.Conflicts...)
	}
}

// defaultRE matches the prose sentence SEMP uses to state a default. Verified
// against every documented default in 10.26.0.8755: all 892 are a bare integer,
// true, false, or a double-quoted string.
var defaultRE = regexp.MustCompile("The default value is `([^`]*)`")

// parseDefault reads a documented default out of an attribute description. The
// value is documentation only: the tool never sends an unspecified attribute, so
// the broker applies its own default either way.
func parseDefault(description string, kind Kind) any {
	m := defaultRE.FindStringSubmatch(description)
	if m == nil {
		return nil
	}
	lit := m[1]
	switch kind {
	case KindBool:
		switch lit {
		case "true":
			return true
		case "false":
			return false
		}
	case KindInt:
		if n, err := strconv.Atoi(lit); err == nil {
			return n
		}
	case KindString:
		if unquoted, err := strconv.Unquote(lit); err == nil {
			if unquoted == "" {
				// An empty default carries no information in the docs table.
				return nil
			}
			return unquoted
		}
		return lit
	}
	return nil
}

// boilerplate is the access-scope and config-sync prose every SEMP attribute
// description carries. It is dropped: it says the same thing 2,054 times and
// would dominate the generated files and the module reference.
var boilerplate = regexp.MustCompile(`(?s)\n\n.*`)

// describe reduces an attribute description to its first paragraph and appends
// the operational notes an operator needs but the prose buries.
func describe(description string, f mdtable.Flags) string {
	s := strings.TrimSpace(boilerplate.ReplaceAllString(strings.TrimSpace(description), ""))
	s = strings.Join(strings.Fields(s), " ")

	var notes []string
	if f["Deprecated"] {
		notes = append(notes, "Deprecated.")
	}
	if f["Const"] {
		notes = append(notes, "Settable only when the object is created.")
	}
	if f["Write-Only"] {
		notes = append(notes, "Never returned by the broker, so it is re-sent on every update.")
	}
	if f["Requires-Disable"] {
		notes = append(notes, "The broker rejects a change to this while the service is enabled; disable it first.")
	}
	if f["Auto-Disable"] {
		notes = append(notes, "Changing this may briefly interrupt the affected service.")
	}
	if len(notes) == 0 {
		return s
	}
	return strings.TrimSpace(s + " " + strings.Join(notes, " "))
}

// firstSentence trims a path parameter description to one sentence, which is all
// a key needs in the module reference.
func firstSentence(s string) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	if i := strings.Index(s, ". "); i > 0 {
		return s[:i+1]
	}
	return s
}
