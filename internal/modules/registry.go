package modules

import (
	"fmt"
	"sort"
	"strings"
)

// registry maps "object.verb" action names to their Module implementation.
var registry = map[string]Module{}

// register adds a module action to the registry. Called from each module's
// init() at startup.
//
// A duplicate name is a systemic defect -- a generator bug, or a generated
// prefix colliding with a hand-written one -- and silently shadowing one module
// with another would be undetectable at runtime, so it panics during init()
// rather than starting a binary whose registry does not match its source.
func register(name string, m Module) {
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("modules: duplicate registration for %q", name))
	}
	registry[name] = m
}

// descriptors records every registered object descriptor in registration order,
// so tests and the generated module manifest can enumerate the action surface
// without importing the generator.
var descriptors []objectDesc

// registerObject wires up the actions one SEMP config object supports: add and
// delete for a collection object, update as well where the item path accepts
// PATCH, and update alone for the Broker singleton.
//
// Any hand-written override for this prefix is applied first, so a generated
// descriptor never has to know that its object has bespoke behaviour, and the
// order in which the generated files' init() functions run does not matter.
func registerObject(d objectDesc) {
	if fn, ok := overrides[d.Prefix]; ok {
		fn(&d)
	}
	applyMsgDefaults(&d)
	descriptors = append(descriptors, d)

	// A carried-over family answers to its old short prefix as well. The same
	// Module value is registered under both, so the two names cannot drift.
	legacy, hasLegacy := legacyActionPrefixes[d.Prefix]
	reg := func(verb string, m Module) {
		register(d.Prefix+"."+verb, m)
		if hasLegacy {
			legacyRegistry[legacy+"."+verb] = m
		}
	}

	if d.IsSingleton {
		reg("update", collectionUpdate{d})
		return
	}
	reg("add", collectionAdd{d})
	if d.HasUpdate {
		reg("update", collectionUpdate{d})
	}
	reg("delete", collectionDelete{d})
}

// Get looks up a registered module by "object.verb" name, falling back to the
// legacy short names so a template written against the previous implementation
// still resolves. The error message lists the canonical modules only, which is
// what a new template should use.
func Get(name string) (Module, error) {
	if m, ok := registry[name]; ok {
		return m, nil
	}
	if m, ok := legacyRegistry[name]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("Unknown module '%s'. Available: %s", name, strings.Join(List(), ", ")) //nolint:staticcheck // message text is an intentional exact wording
}

// List returns the sorted names of all registered modules.
func List() []string {
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ModuleInfo is the description + parameter schema for a single module.
type ModuleInfo struct {
	Description string
	Params      []ParamSpec
}

// Info returns the description and parameter schema for every registered
// module. Callers that need deterministic ordering should iterate List().
func Info() map[string]ModuleInfo {
	out := make(map[string]ModuleInfo, len(registry))
	for name, m := range registry {
		out[name] = ModuleInfo{Description: m.Description(), Params: m.Params()}
	}
	return out
}
