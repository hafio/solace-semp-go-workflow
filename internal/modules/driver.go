package modules

import (
	"fmt"
	"strings"

	"solace-semp-workflow/internal/models"
	"solace-semp-workflow/internal/semp"
)

// This file implements every action in the registry. There are exactly three
// action types -- collectionAdd, collectionUpdate and collectionDelete -- each
// parameterised by one objectDesc describing a single SEMP config object. The
// generated files under gen_*.go contain nothing but those descriptors, so a
// fix to key encoding, payload filtering, idempotency or dry-run behaviour is
// made here once rather than in several hundred near-identical modules.

// scopeKind is where an object's collection lives in the SEMP config tree.
type scopeKind int

const (
	// scopeVPN objects are nested under msgVpns/{msgVpnName}.
	scopeVPN scopeKind = iota
	// scopeGlobal objects hang off the config root: dmrClusters, oauthProfiles,
	// certAuthorities, proxies, virtualHostnames, and msgVpns itself.
	scopeGlobal
)

// fieldKind is the Go type a body attribute is coerced to before it is sent.
type fieldKind int

const (
	kindString fieldKind = iota
	kindInt
	kindBool
	// kindObject is a nested attribute group. SEMP uses exactly one shape here,
	// the event threshold ({clearPercent, clearValue, setPercent, setValue}),
	// and documents every member as absent from a GET -- so a kindObject is
	// always treated as write-only when diffing.
	kindObject
)

// addMode selects how an add action establishes that it has work to do.
type addMode int

const (
	// addPrecheck GETs the item first: present means SKIPPED. This is the
	// default and the only mode generated objects use.
	addPrecheck addMode = iota
	// addOptimistic POSTs without a pre-check on the live path and relies on the
	// broker's AlreadyExists code to report SKIPPED. Dry-run still pre-checks,
	// since it has nothing else to report from. Used only by q_sub, whose
	// carried-over test asserts that no GET is issued.
	addOptimistic
)

// payloadHook runs after the payload has been cleaned, coerced and filtered,
// and may derive additional fields or fail the action. It is the escape hatch
// for per-object behaviour a descriptor cannot express.
type payloadHook func(args, payload map[string]any) error

// keyDesc is one identifying key value. Several keyDescs on the same hop form a
// composite key, which SEMP renders as one comma-joined path segment.
type keyDesc struct {
	Name string
	Desc string
	// Default is used when the action does not supply this key. It exists for
	// composite keys whose leading part is a discriminator an operator rarely
	// states -- an ACL exception's topic syntax defaults to "smf" -- so a
	// workflow can name the topic alone. Empty means the key is mandatory.
	Default string
}

// parentHop is one ancestor collection an object is nested under, outermost
// first. Its key values arrive as action args and are never sent in the body:
// the specification marks them Identifying and Read-Only because the URL
// already carries them.
type parentHop struct {
	Collection string
	Keys       []keyDesc
}

// fieldDesc is one non-key body attribute.
type fieldDesc struct {
	Name    string
	Kind    fieldKind
	Enum    []string
	Default any
	Desc    string
	// Required means the attribute must be supplied on add even though it is not
	// part of the key.
	Required bool
	// Const attributes are settable only at creation and are dropped from every
	// update payload.
	Const bool
	// WriteOnly attributes are never returned by a GET, so an update cannot tell
	// whether they already hold the desired value and always re-sends them.
	WriteOnly bool
	// Opaque attributes are secrets: masked in output and docs, and never diffed.
	Opaque bool
	// Deprecated mirrors the specification's x-deprecated flag.
	Deprecated bool
	// Fields is the nested attribute set of a kindObject, nil otherwise.
	Fields []fieldDesc
	// Requires and Conflicts carry the specification's attribute co-dependency
	// rules. When any member of a group changes, the whole group is sent so the
	// request is self-consistent however the broker validates it.
	Requires  []string
	Conflicts []string
}

// msgSet is the user-facing wording for one object's actions, as templates over
// the placeholders {obj}, {Obj}, {name}, {parent}, {pname} and {err}. An unset
// field falls back to the generic default, which is what every generated object
// uses; the families carried over from the previous implementation override the
// fields whose historical wording differs ("added" rather than "created",
// "removed from" rather than "deleted from", failures that omit the object
// name) so their output is byte-identical to before.
//
// Templates are substituted, not formatted, so a template that omits a
// placeholder or names one that does not apply is harmless rather than a
// runtime panic.
type msgSet struct {
	MissingArg    string
	AlreadyExists string
	NotExists     string
	Converged     string
	WouldCreate   string
	WouldUpdate   string
	WouldDelete   string
	Created       string
	Updated       string
	Deleted       string
	NoFields      string
	CheckErr      string
	CreateErr     string
	UpdateErr     string
	DeleteErr     string
}

// topLevelMsgs is the default wording for an object with no parent collection.
var topLevelMsgs = msgSet{
	MissingArg:    "Missing required arg{plural}: {arg}",
	AlreadyExists: "{Obj} '{name}' already exists",
	NotExists:     "{Obj} '{name}' does not exist",
	Converged:     "{Obj} '{name}' is already in the desired state",
	WouldCreate:   "Would create {obj} '{name}'",
	WouldUpdate:   "Would update {obj} '{name}'",
	WouldDelete:   "Would delete {obj} '{name}'",
	Created:       "{Obj} '{name}' created",
	Updated:       "{Obj} '{name}' updated",
	Deleted:       "{Obj} '{name}' deleted",
	NoFields:      "No fields to update",
	CheckErr:      "Error checking {obj}: {err}",
	CreateErr:     "Failed to create {obj} '{name}': {err}",
	UpdateErr:     "Failed to update {obj} '{name}': {err}",
	DeleteErr:     "Failed to delete {obj} '{name}': {err}",
}

// nestedMsgs is the default wording for an object nested under a parent, which
// names the parent so an operator can tell two same-named children apart.
var nestedMsgs = msgSet{
	MissingArg:    "Missing required arg{plural}: {arg}",
	AlreadyExists: "{Obj} '{name}' already exists on {parent} '{pname}'",
	NotExists:     "{Obj} '{name}' does not exist on {parent} '{pname}'",
	Converged:     "{Obj} '{name}' on {parent} '{pname}' is already in the desired state",
	WouldCreate:   "Would create {obj} '{name}' on {parent} '{pname}'",
	WouldUpdate:   "Would update {obj} '{name}' on {parent} '{pname}'",
	WouldDelete:   "Would delete {obj} '{name}' from {parent} '{pname}'",
	Created:       "{Obj} '{name}' created on {parent} '{pname}'",
	Updated:       "{Obj} '{name}' updated on {parent} '{pname}'",
	Deleted:       "{Obj} '{name}' deleted from {parent} '{pname}'",
	NoFields:      "No fields to update",
	CheckErr:      "Error checking {obj}: {err}",
	CreateErr:     "Failed to create {obj}: {err}",
	UpdateErr:     "Failed to update {obj}: {err}",
	DeleteErr:     "Failed to delete {obj}: {err}",
}

// singletonMsgs is the wording for the Broker object, which has no key at all.
var singletonMsgs = msgSet{
	MissingArg:  "Missing required arg{plural}: {arg}",
	Converged:   "{Obj} is already in the desired state",
	WouldUpdate: "Would update {obj}",
	Updated:     "{Obj} updated",
	NoFields:    "No fields to update",
	CheckErr:    "Error checking {obj}: {err}",
	UpdateErr:   "Failed to update {obj}: {err}",
}

// orDefault returns v, or def when v is unset.
func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// applyMsgDefaults fills every unset message template from the default set for
// this object's shape, so a descriptor only states the wording it needs to
// differ on. Called by registerObject before the drivers are constructed.
func applyMsgDefaults(d *objectDesc) {
	base := topLevelMsgs
	switch {
	case d.IsSingleton:
		base = singletonMsgs
	case len(d.Parents) > 0:
		base = nestedMsgs
	}
	m := &d.Msg
	m.MissingArg = orDefault(m.MissingArg, base.MissingArg)
	m.AlreadyExists = orDefault(m.AlreadyExists, base.AlreadyExists)
	m.NotExists = orDefault(m.NotExists, base.NotExists)
	m.Converged = orDefault(m.Converged, base.Converged)
	m.WouldCreate = orDefault(m.WouldCreate, base.WouldCreate)
	m.WouldUpdate = orDefault(m.WouldUpdate, base.WouldUpdate)
	m.WouldDelete = orDefault(m.WouldDelete, base.WouldDelete)
	m.Created = orDefault(m.Created, base.Created)
	m.Updated = orDefault(m.Updated, base.Updated)
	m.Deleted = orDefault(m.Deleted, base.Deleted)
	m.NoFields = orDefault(m.NoFields, base.NoFields)
	m.CheckErr = orDefault(m.CheckErr, base.CheckErr)
	m.CreateErr = orDefault(m.CreateErr, base.CreateErr)
	m.UpdateErr = orDefault(m.UpdateErr, base.UpdateErr)
	m.DeleteErr = orDefault(m.DeleteErr, base.DeleteErr)
}

// objectDesc is the complete description of one SEMP config object: everything
// the three drivers need at runtime, plus the metadata that feeds Params() and
// the generated documentation.
type objectDesc struct {
	// Prefix is the registry key prefix, e.g. "queue" in "queue.add".
	Prefix string
	// FriendlyName is how the object is named in result messages, lower case
	// except for genuine acronyms ("queue", "RDP", "ACL profile").
	FriendlyName string
	// Article is the indefinite article for FriendlyName in generated
	// descriptions; "a" when unset. It is stated rather than derived because no
	// rule gets both "an RDP" and "a REST consumer" right -- initialism versus
	// acronym is about pronunciation, which the spelling does not carry.
	Article string
	Scope        scopeKind
	// IsSingleton is true only for the Broker object, which supports update only
	// and whose path is empty.
	IsSingleton bool
	// Parents are the ancestor collections, outermost first.
	Parents []parentHop
	// ParentFriendlyName names the immediate parent in messages, e.g. "RDP".
	ParentFriendlyName string
	// Collection is this object's own collection segment, e.g. "queueBindings".
	Collection string
	// OwnKey is this object's identifying key, one entry per path parameter.
	OwnKey []keyDesc
	// DisplayKey selects which OwnKey entry appears in messages, for objects
	// whose key is composite (an ACL exception names the topic, not the syntax).
	DisplayKey int
	Fields     []fieldDesc
	HasUpdate  bool
	// Deprecated marks an object the specification has superseded in full.
	Deprecated string
	AddMode    addMode
	Msg        msgSet
	AddHook    payloadHook
	UpdateHook payloadHook
}

// resolved holds the values a driver needs after key resolution: the item and
// collection paths, plus the names used in messages.
type resolved struct {
	itemPath   string
	collPath   string
	name       string
	parentName string
	// ownValues holds this object's resolved key values, including any that came
	// from a keyDesc default, so a create can put them in the body even when the
	// action did not name them.
	ownValues map[string]any
}

// capitalizeFirst upper-cases the first rune only, so an already-correct
// acronym ("RDP", "ACL profile") survives and an ordinary word is title-cased.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// render substitutes the message placeholders. Templates never reach fmt, so a
// placeholder that does not apply to this object is simply left unreplaced
// rather than producing a formatting panic.
func (d objectDesc) render(tmpl string, r resolved, arg string, err error) string {
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	rep := strings.NewReplacer(
		"{obj}", d.FriendlyName,
		"{Obj}", capitalizeFirst(d.FriendlyName),
		"{name}", r.name,
		"{parent}", d.ParentFriendlyName,
		"{pname}", r.parentName,
		"{arg}", arg,
		"{err}", errText,
	)
	return rep.Replace(tmpl)
}

// missing builds the FAILED result naming every absent required argument, so an
// operator adds them all in one go rather than one per run. {plural} lets the
// default wording agree with the count; an object whose historical wording was
// always plural overrides the template outright.
func (d objectDesc) missing(names []string) models.ActionResult {
	plural := ""
	if len(names) > 1 {
		plural = "s"
	}
	tmpl := strings.ReplaceAll(d.Msg.MissingArg, "{plural}", plural)
	return failed(d.render(tmpl, resolved{}, strings.Join(names, ", "), nil))
}

// hopKeys reads one hop's key values from args in path-parameter order, falling
// back to a key's declared default. The second return lists every key that is
// absent and has no default, and is empty on success.
func hopKeys(keys []keyDesc, args map[string]any) (parts, missing []string) {
	parts = make([]string, 0, len(keys))
	for _, k := range keys {
		v := argStr(args, k.Name, k.Default)
		if v == "" {
			missing = append(missing, k.Name)
			continue
		}
		parts = append(parts, v)
	}
	if len(missing) > 0 {
		return nil, missing
	}
	return parts, nil
}

// resolve turns action args into the item and collection paths for this object,
// validating that every key is present -- or defaulted -- and within the
// broker's name limits. The second return lists the missing arguments; the third
// is a name-length violation message.
func (d objectDesc) resolve(c Client, args map[string]any) (resolved, []string, string) {
	r := resolved{ownValues: map[string]any{}}
	segs := make([]semp.Seg, 0, len(d.Parents)+2)
	var missing []string

	if d.Scope == scopeVPN {
		vpn := semp.ResolveVPN(argStr(args, "msgVpnName", ""), c.DefaultMsgVPN())
		if vpn == "" {
			// Without a VPN no path can be built at all, so report immediately
			// rather than accumulating keys that would be meaningless anyway.
			return r, []string{"msgVpnName"}, ""
		}
		segs = append(segs, semp.VPNSeg(vpn))
	}

	for _, p := range d.Parents {
		parts, miss := hopKeys(p.Keys, args)
		if len(miss) > 0 {
			missing = append(missing, miss...)
			continue
		}
		for i, v := range parts {
			if msg := semp.CheckNameLength(p.Keys[i].Name, v); msg != "" {
				return r, nil, msg
			}
		}
		segs = append(segs, semp.Seg{Literal: p.Collection, Parts: parts})
		// The innermost parent is the one named in messages.
		r.parentName = parts[0]
	}

	if d.IsSingleton {
		// The Broker object is the config root itself: both paths are empty.
		return r, missing, ""
	}

	ownParts, miss := hopKeys(d.OwnKey, args)
	missing = append(missing, miss...)
	if len(missing) > 0 {
		return r, missing, ""
	}
	for i, v := range ownParts {
		if msg := semp.CheckNameLength(d.OwnKey[i].Name, v); msg != "" {
			return r, nil, msg
		}
	}
	for i, v := range ownParts {
		r.ownValues[d.OwnKey[i].Name] = v
	}
	if d.DisplayKey < len(ownParts) {
		r.name = ownParts[d.DisplayKey]
	} else {
		r.name = ownParts[0]
	}

	r.collPath = semp.Path(append(segs, semp.Seg{Literal: d.Collection})...)
	r.itemPath = semp.Path(append(segs, semp.Seg{Literal: d.Collection, Parts: ownParts})...)
	return r, nil, ""
}

// pathOnlyNames is the set of argument names carried by the URL and never by the
// body: the VPN and every ancestor key. SEMP marks these Identifying and
// Read-Only on the child, so sending them is rejected.
//
// An object's OWN key is not in this set: the specification marks it Identifying
// and Required for a create, so it must appear in the POST body. It is dropped
// from an update instead, where it is Const.
func (d objectDesc) pathOnlyNames() map[string]bool {
	out := map[string]bool{"msgVpnName": true}
	for _, p := range d.Parents {
		for _, k := range p.Keys {
			out[k.Name] = true
		}
	}
	return out
}

// buildPayload assembles the request body: blank and nil args dropped, every
// declared field coerced to its spec type, URL-only arguments removed, and -- on
// update -- the object's own key and its const fields removed too. On a create
// the resolved key values are written in, so a key that came from a default is
// still stated in the body as the specification requires. The hook runs last so
// derived fields see final values.
func (d objectDesc) buildPayload(r resolved, args map[string]any, forUpdate bool) (map[string]any, error) {
	payload := semp.CleanPayload(args)

	drop := d.pathOnlyNames()
	if forUpdate {
		for _, k := range d.OwnKey {
			drop[k.Name] = true
		}
	}
	for name := range payload {
		if drop[name] {
			delete(payload, name)
		}
	}
	if !forUpdate {
		for name, v := range r.ownValues {
			payload[name] = v
		}
	}

	for _, f := range d.Fields {
		if _, present := payload[f.Name]; !present {
			continue
		}
		if forUpdate && f.Const {
			delete(payload, f.Name)
			continue
		}
		if err := coerceField(f, payload); err != nil {
			return nil, err
		}
	}

	hook := d.AddHook
	if forUpdate {
		hook = d.UpdateHook
	}
	if hook != nil {
		if err := hook(args, payload); err != nil {
			return nil, err
		}
	}
	return payload, nil
}

// requiredMissing returns the name of the first required non-key field absent
// from args, or "". Presence is judged by kind: a string must be non-blank,
// while a caller-supplied false or 0 is a real value, not an omission.
func (d objectDesc) requiredMissing(args map[string]any) string {
	for _, f := range d.Fields {
		if !f.Required {
			continue
		}
		v, present := args[f.Name]
		if !present || v == nil {
			return f.Name
		}
		if f.Kind == kindString && strings.TrimSpace(semp.Stringify(v)) == "" {
			return f.Name
		}
	}
	return ""
}

// collectionAdd creates an object if it is not already present.
type collectionAdd struct{ d objectDesc }

// article is the indefinite article to use before FriendlyName.
func (d objectDesc) article() string {
	if d.Article != "" {
		return d.Article
	}
	return "a"
}

func (a collectionAdd) Description() string {
	d := a.d
	s := fmt.Sprintf("Create %s %s. Skipped if it already exists.", d.article(), d.FriendlyName)
	if d.Deprecated != "" {
		s += " Deprecated: " + d.Deprecated
	}
	return s
}

func (a collectionAdd) Params() []ParamSpec { return buildParams(a.d, paramAdd) }

func (a collectionAdd) Execute(c Client, args map[string]any, dryRun bool) models.ActionResult {
	d := a.d
	r, missing, lenErr := d.resolve(c, args)
	if len(missing) > 0 {
		return d.missing(missing)
	}
	if lenErr != "" {
		return failed(lenErr)
	}
	if f := d.requiredMissing(args); f != "" {
		return d.missing([]string{f})
	}

	// The optimistic mode still pre-checks under dry-run: without a GET there is
	// nothing to base "would create" versus "already exists" on.
	if d.AddMode == addPrecheck || dryRun {
		exists, _, err := c.Exists(r.itemPath)
		if err != nil {
			return failed(d.render(d.Msg.CheckErr, r, "", err))
		}
		if exists {
			return skipped(d.render(d.Msg.AlreadyExists, r, "", nil))
		}
	}
	if dryRun {
		return dryrun(d.render(d.Msg.WouldCreate, r, "", nil))
	}

	payload, err := d.buildPayload(r, args, false)
	if err != nil {
		return failed(err.Error())
	}
	if _, err := c.Create(r.collPath, payload); err != nil {
		// A concurrent create, or an optimistic add whose object already exists,
		// is the desired state rather than a failure.
		if sempCodeOf(err) == semp.AlreadyExists {
			return skipped(d.render(d.Msg.AlreadyExists, r, "", nil))
		}
		return failed(d.render(d.Msg.CreateErr, r, "", err))
	}
	return ok(d.render(d.Msg.Created, r, "", nil))
}

// collectionUpdate converges an existing object on the supplied attributes. It
// reads current state first and sends only genuinely differing fields, so a
// second run of the same workflow reports no change.
type collectionUpdate struct{ d objectDesc }

func (u collectionUpdate) Description() string {
	d := u.d
	if d.IsSingleton {
		return fmt.Sprintf("Update the %s. Skipped if it is already in the desired state.", d.FriendlyName)
	}
	s := fmt.Sprintf("Update an existing %s. Fails if it does not exist, skipped if it is already in the desired state.", d.FriendlyName)
	if d.Deprecated != "" {
		s += " Deprecated: " + d.Deprecated
	}
	return s
}

func (u collectionUpdate) Params() []ParamSpec { return buildParams(u.d, paramUpdate) }

func (u collectionUpdate) Execute(c Client, args map[string]any, dryRun bool) models.ActionResult {
	d := u.d
	r, missing, lenErr := d.resolve(c, args)
	if len(missing) > 0 {
		return d.missing(missing)
	}
	if lenErr != "" {
		return failed(lenErr)
	}

	exists, current, err := c.Exists(r.itemPath)
	if err != nil {
		return failed(d.render(d.Msg.CheckErr, r, "", err))
	}
	// The Broker object always exists; every other object must be created first.
	if !exists && !d.IsSingleton {
		return failed(d.render(d.Msg.NotExists, r, "", nil))
	}

	desired, err := d.buildPayload(r, args, true)
	if err != nil {
		return failed(err.Error())
	}
	if len(desired) == 0 {
		return skipped(d.render(d.Msg.NoFields, r, "", nil))
	}

	changes := d.diff(desired, current)
	if len(changes) == 0 {
		return skipped(d.render(d.Msg.Converged, r, "", nil))
	}
	if dryRun {
		return dryrun(d.render(d.Msg.WouldUpdate, r, "", nil))
	}
	if _, err := c.Update(r.itemPath, changes); err != nil {
		return failed(d.render(d.Msg.UpdateErr, r, "", err))
	}
	return ok(d.render(d.Msg.Updated, r, "", nil))
}

// diff returns the subset of desired that differs from the broker's current
// state. Attributes the broker never returns -- write-only, opaque and the
// nested threshold groups -- cannot be compared and are always included.
func (d objectDesc) diff(desired, current map[string]any) map[string]any {
	byName := make(map[string]fieldDesc, len(d.Fields))
	for _, f := range d.Fields {
		byName[f.Name] = f
	}

	changes := make(map[string]any, len(desired))
	for name, want := range desired {
		f, known := byName[name]
		// Derived fields added by a hook are not in the descriptor; send them.
		if !known || f.WriteOnly || f.Opaque || f.Kind == kindObject {
			changes[name] = want
			continue
		}
		if !semp.AttrEqual(current[name], want) {
			changes[name] = want
		}
	}
	if len(changes) == 0 {
		return changes
	}
	d.widenGroups(changes, desired, byName)
	return changes
}

// widenGroups pulls in every co-dependent attribute of any changed attribute,
// so a request that changes one member of a Requires/Conflicts group carries
// the whole group and validates the same way regardless of whether the broker
// checks the request in isolation or merged onto current state.
func (d objectDesc) widenGroups(changes, desired map[string]any, byName map[string]fieldDesc) {
	// The changed names are snapshotted because the loop inserts into changes,
	// and a map grown during its own range has undefined visitation.
	changed := make([]string, 0, len(changes))
	for name := range changes {
		changed = append(changed, name)
	}
	for _, name := range changed {
		f, ok := byName[name]
		if !ok {
			continue
		}
		peers := make([]string, 0, len(f.Requires)+len(f.Conflicts))
		peers = append(peers, f.Requires...)
		peers = append(peers, f.Conflicts...)
		for _, peer := range peers {
			if v, supplied := desired[peer]; supplied {
				changes[peer] = v
			}
		}
	}
}

// collectionDelete removes an object if it is present.
type collectionDelete struct{ d objectDesc }

func (x collectionDelete) Description() string {
	d := x.d
	s := fmt.Sprintf("Delete %s %s. Skipped if it does not exist.", d.article(), d.FriendlyName)
	if d.Deprecated != "" {
		s += " Deprecated: " + d.Deprecated
	}
	return s
}

func (x collectionDelete) Params() []ParamSpec { return buildParams(x.d, paramDelete) }

func (x collectionDelete) Execute(c Client, args map[string]any, dryRun bool) models.ActionResult {
	d := x.d
	r, missing, lenErr := d.resolve(c, args)
	if len(missing) > 0 {
		return d.missing(missing)
	}
	if lenErr != "" {
		return failed(lenErr)
	}

	exists, _, err := c.Exists(r.itemPath)
	if err != nil {
		return failed(d.render(d.Msg.CheckErr, r, "", err))
	}
	if !exists {
		return skipped(d.render(d.Msg.NotExists, r, "", nil))
	}
	if dryRun {
		return dryrun(d.render(d.Msg.WouldDelete, r, "", nil))
	}
	if err := c.Delete(r.itemPath); err != nil {
		return failed(d.render(d.Msg.DeleteErr, r, "", err))
	}
	return ok(d.render(d.Msg.Deleted, r, "", nil))
}

// paramMode selects which attributes an action documents and accepts.
type paramMode int

const (
	// paramAdd documents keys and every settable attribute, with defaults.
	paramAdd paramMode = iota
	// paramUpdate drops const attributes, which are settable only at creation,
	// and states no defaults: omitting an attribute leaves it unchanged, so
	// advertising the create-time default would be misleading.
	paramUpdate
	// paramDelete documents keys only -- a delete has no body.
	paramDelete
)

// buildParams renders the documented parameter schema: ancestor keys first, then
// this object's own key, then body attributes in declaration order, and finally
// the optional msgVpnName override for VPN-scoped objects. Keys lead so the
// required identifier of an action is the first thing an operator reads, which
// is also the ordering the carried-over tests assert.
func buildParams(d objectDesc, mode paramMode) []ParamSpec {
	specs := make([]ParamSpec, 0, len(d.Fields)+4)

	for _, p := range d.Parents {
		for _, k := range p.Keys {
			specs = append(specs, ParamSpec{Name: k.Name, Type: "string", Required: true, Description: k.Desc})
		}
	}
	for _, k := range d.OwnKey {
		specs = append(specs, ParamSpec{Name: k.Name, Type: "string", Required: true, Description: k.Desc})
	}

	if mode != paramDelete {
		for _, f := range d.Fields {
			if mode == paramUpdate && f.Const {
				continue
			}
			spec := ParamSpec{
				Name:        f.Name,
				Type:        f.Kind.String(),
				Required:    f.Required && mode == paramAdd,
				Description: f.Desc,
				Enum:        f.Enum,
				Sensitive:   f.WriteOnly || f.Opaque,
			}
			if mode == paramAdd {
				spec.Default = f.Default
			}
			specs = append(specs, spec)
		}
	}

	if d.Scope == scopeVPN {
		specs = append(specs, ParamSpec{
			Name:        "msgVpnName",
			Type:        "string",
			Description: "Message VPN to act on. Defaults to the VPN in the connection config.",
		})
	}
	return specs
}

// String is the documented type name for a field kind.
func (k fieldKind) String() string {
	switch k {
	case kindInt:
		return "integer"
	case kindBool:
		return "boolean"
	case kindObject:
		return "object"
	default:
		return "string"
	}
}
