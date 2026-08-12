// Package specmodel is a thin, faithful mirror of the parts of the SEMP v2
// Swagger 2.0 document the generator reads. It deliberately models only what is
// needed rather than pulling in a general OpenAPI library: the interesting
// metadata is not in the schema at all, it is in Markdown tables inside each
// operation's description (see package mdtable).
package specmodel

import (
	"encoding/json"
	"fmt"
	"os"
)

// Doc is a parsed SEMP v2 config specification.
type Doc struct {
	Info        Info                  `json:"info"`
	BasePath    string                `json:"basePath"`
	Paths       map[string]PathItem   `json:"paths"`
	Definitions map[string]Definition `json:"definitions"`
}

// Info carries the broker version the specification was generated from.
type Info struct {
	Version string `json:"version"`
}

// PathItem maps a lower-case HTTP verb to its operation.
type PathItem map[string]Operation

// Operation is one verb on one path.
type Operation struct {
	OperationID string      `json:"operationId"`
	Description string      `json:"description"`
	Summary     string      `json:"summary"`
	Parameters  []Parameter `json:"parameters"`
}

// Parameter is a path, query or body parameter.
type Parameter struct {
	In          string  `json:"in"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Required    bool    `json:"required"`
	Type        string  `json:"type"`
	Schema      *Schema `json:"schema"`
	Ref         string  `json:"$ref"`
}

// Schema is a body parameter's schema, which is always a definition reference.
type Schema struct {
	Ref string `json:"$ref"`
}

// Definition is one object schema. SEMP definitions are flat apart from the
// event-threshold groups, which are single-level $ref sub-objects.
type Definition struct {
	Description string              `json:"description"`
	Properties  map[string]Property `json:"properties"`
}

// Property is one attribute of a definition.
type Property struct {
	Type        string   `json:"type"`
	Format      string   `json:"format"`
	Description string   `json:"description"`
	Enum        []string `json:"enum"`
	Ref         string   `json:"$ref"`
	Deprecated  bool     `json:"x-deprecated"`
}

// BodyDefinition returns the definition name an operation's body parameter
// refers to, or "" when the operation takes no body.
func (o Operation) BodyDefinition() string {
	for _, p := range o.Parameters {
		if p.In == "body" && p.Schema != nil && p.Schema.Ref != "" {
			return refName(p.Schema.Ref)
		}
	}
	return ""
}

// PathParams returns the operation's path parameters, keyed by name, so a key's
// documented description can be reused verbatim in the generated module.
func (o Operation) PathParams() map[string]string {
	out := map[string]string{}
	for _, p := range o.Parameters {
		if p.In == "path" {
			out[p.Name] = p.Description
		}
	}
	return out
}

// RefName is the definition name a "#/definitions/X" reference points at.
func RefName(ref string) string { return refName(ref) }

func refName(ref string) string {
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '/' {
			return ref[i+1:]
		}
	}
	return ref
}

// Load reads and parses a specification from disk.
func Load(path string) (*Doc, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // generator input, an explicit committed path
	if err != nil {
		return nil, fmt.Errorf("read spec %s: %w", path, err)
	}
	var doc Doc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse spec %s: %w", path, err)
	}
	if len(doc.Paths) == 0 || len(doc.Definitions) == 0 {
		return nil, fmt.Errorf("spec %s has no paths or definitions", path)
	}
	return &doc, nil
}
