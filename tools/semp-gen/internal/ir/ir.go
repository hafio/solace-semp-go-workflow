// Package ir is the intermediate representation the generator builds from the
// SEMP specifications and renders into module descriptors. It exists so that
// extraction (messy: Markdown tables, prose defaults, two platforms) is fully
// separated from rendering (mechanical: emit Go).
package ir

// Scope is where an object's collection lives in the SEMP config tree.
type Scope int

const (
	// ScopeVPN objects are nested under msgVpns/{msgVpnName}. The VPN hop is
	// stripped from Parents because the runtime supplies it from the connection
	// config unless an action names one.
	ScopeVPN Scope = iota
	// ScopeGlobal objects hang off the config root.
	ScopeGlobal
)

// Platform records which broker platforms expose an object or attribute. The
// appliance specification is not a subset of the software one, so both are read
// and merged as a union.
type Platform int

const (
	PlatformBoth Platform = iota
	PlatformSoftwareOnly
	PlatformApplianceOnly
)

// Note returns the parenthetical to append to a description, or "".
func (p Platform) Note() string {
	switch p {
	case PlatformSoftwareOnly:
		return "Software broker only."
	case PlatformApplianceOnly:
		return "Appliance only."
	default:
		return ""
	}
}

// Kind is the Go type an attribute is coerced to. It maps 1:1 onto the runtime's
// fieldKind constants.
type Kind int

const (
	KindString Kind = iota
	KindInt
	KindBool
	// KindObject is a nested attribute group: SEMP's event-threshold shape.
	KindObject
)

// RuntimeConst is the identifier the runtime uses for this kind.
func (k Kind) RuntimeConst() string {
	switch k {
	case KindInt:
		return "kindInt"
	case KindBool:
		return "kindBool"
	case KindObject:
		return "kindObject"
	default:
		return "kindString"
	}
}

// Key is one identifying path parameter.
type Key struct {
	Name string
	Desc string
	// Default is supplied by the legacy overlay for keys that historically
	// defaulted, so a workflow can omit them. The specification has no notion of
	// a default for an identifying attribute.
	Default string
}

// ParentHop is one ancestor collection, outermost first.
type ParentHop struct {
	Collection string
	Keys       []Key
}

// Attribute is one body attribute of an object.
type Attribute struct {
	Name     string
	Kind     Kind
	Enum     []string
	Default  any
	Desc     string
	Platform Platform

	Required   bool
	Const      bool
	ReadOnly   bool
	WriteOnly  bool
	Opaque     bool
	Deprecated bool
	// RequiresDisable and AutoDisable are documented in the attribute
	// description rather than enforced: the broker rejects a change that needs a
	// disable, and its own error names the constraint.
	RequiresDisable bool
	AutoDisable     bool

	Requires  []string
	Conflicts []string

	// Fields is the nested attribute set of a KindObject.
	Fields []Attribute
}

// Object is one creatable SEMP config object, or the Broker singleton.
type Object struct {
	// Definition is the swagger definition name, e.g. "MsgVpnQueue".
	Definition string
	// OperationID is the create operationId, the input to naming. Empty for the
	// Broker singleton, which cannot be created.
	OperationID string

	Prefix       string
	FriendlyName string
	Article      string

	Scope              Scope
	IsSingleton        bool
	Parents            []ParentHop
	ParentFriendlyName string
	Collection         string
	OwnKey             []Key
	DisplayKey         int

	Attributes []Attribute

	HasUpdate  bool
	Deprecated string
	Platform   Platform

	// Messages are the wording deviations the legacy overlay pins, keyed by the
	// runtime msgSet field name. Empty for a generated object, which uses the
	// runtime's shape defaults.
	Messages map[string]string
	// SkipHookDerived lists attributes a payload hook derives, which must not be
	// offered as settable parameters.
	SkipHookDerived []string
}

// Spec is the merged representation the renderer consumes.
type Spec struct {
	Version string
	Objects []Object
}
