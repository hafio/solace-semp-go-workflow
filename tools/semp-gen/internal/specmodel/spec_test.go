package specmodel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalSpecJSON is a syntactically complete SEMP v2 config specification: one
// path with a create and a delete operation, and the definition the create's
// body refers to. Small enough to read in one glance, but real enough to
// exercise the fields extract.go actually reads.
const minimalSpecJSON = `{
	"info": {"version": "10.26.0.8755"},
	"basePath": "/SEMP/v2/config",
	"paths": {
		"/msgVpns/{msgVpnName}/queues": {
			"post": {
				"operationId": "createMsgVpnQueue",
				"description": "Create a Queue object.",
				"summary": "Create a Queue object.",
				"parameters": [
					{"in": "path", "name": "msgVpnName", "description": "The name of the Message VPN.", "required": true, "type": "string"},
					{"in": "body", "name": "body", "required": true, "schema": {"$ref": "#/definitions/MsgVpnQueue"}}
				]
			},
			"delete": {
				"operationId": "deleteMsgVpnQueue",
				"description": "Delete a Queue object.",
				"summary": "Delete a Queue object.",
				"parameters": [
					{"in": "path", "name": "msgVpnName", "description": "The name of the Message VPN.", "required": true, "type": "string"},
					{"in": "path", "name": "queueName", "description": "The name of the Queue.", "required": true, "type": "string"}
				]
			}
		}
	},
	"definitions": {
		"MsgVpnQueue": {
			"description": "A Queue object.",
			"properties": {
				"queueName": {"type": "string", "description": "The name of the Queue."}
			}
		}
	}
}`

// writeSpec writes content to a fresh temp file and returns its path, so each
// Load case gets a path Load can name in its error without cases colliding.
func writeSpec(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadValidMinimalSpec(t *testing.T) {
	doc, err := Load(writeSpec(t, minimalSpecJSON))
	require.NoError(t, err)

	assert.Equal(t, "10.26.0.8755", doc.Info.Version)
	require.Contains(t, doc.Paths, "/msgVpns/{msgVpnName}/queues")
	op := doc.Paths["/msgVpns/{msgVpnName}/queues"]["post"]
	assert.Equal(t, "createMsgVpnQueue", op.OperationID)
	require.Contains(t, doc.Definitions, "MsgVpnQueue")
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name string
		path func(t *testing.T) string
	}{
		// A wrong -software/-appliance flag must fail with the path in the
		// message, not a bare "file not found" that leaves which flag unclear.
		{"missing file", func(t *testing.T) string {
			return filepath.Join(t.TempDir(), "missing.json")
		}},
		{"malformed json", func(t *testing.T) string {
			return writeSpec(t, "{not valid json")
		}},
		// Syntactically fine but empty: a spec pointed at the wrong file (e.g. an
		// unrelated JSON document) parses without error, so content is checked too.
		{"no paths or definitions", func(t *testing.T) string {
			return writeSpec(t, `{"info": {"version": "1.0"}, "paths": {}, "definitions": {}}`)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.path(t)
			_, err := Load(path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), path)
		})
	}
}

func TestBodyDefinition(t *testing.T) {
	cases := []struct {
		name string
		op   Operation
		want string
	}{
		{"body parameter with a ref names the definition", Operation{Parameters: []Parameter{
			{In: "path", Name: "msgVpnName"},
			{In: "body", Name: "body", Schema: &Schema{Ref: "#/definitions/MsgVpnQueue"}},
		}}, "MsgVpnQueue"},
		// A delete takes keys only, so it carries no body parameter at all.
		{"delete operation has no body parameter", Operation{Parameters: []Parameter{
			{In: "path", Name: "msgVpnName"},
			{In: "path", Name: "queueName"},
		}}, ""},
		// Guards against a body parameter whose schema is present but unreferenced.
		{"body parameter with no schema ref", Operation{Parameters: []Parameter{
			{In: "body", Name: "body", Schema: &Schema{}},
		}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.op.BodyDefinition())
		})
	}
}

func TestPathParams(t *testing.T) {
	op := Operation{Parameters: []Parameter{
		{In: "path", Name: "msgVpnName", Description: "The name of the Message VPN."},
		{In: "path", Name: "queueName", Description: "The name of the Queue."},
		{In: "body", Name: "body"},
		{In: "query", Name: "select", Description: "Attributes to include."},
	}}

	got := op.PathParams()
	assert.Equal(t, map[string]string{
		"msgVpnName": "The name of the Message VPN.",
		"queueName":  "The name of the Queue.",
	}, got)

	// extract.go builds a keyDesc straight from this map without copying it
	// first, so a second call must not see the first call's mutation.
	got["msgVpnName"] = "mutated"
	again := op.PathParams()
	assert.Equal(t, "The name of the Message VPN.", again["msgVpnName"])
}

func TestRefName(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		want string
	}{
		{"full definitions pointer", "#/definitions/MsgVpnQueue", "MsgVpnQueue"},
		{"bare name with no slash", "MsgVpnQueue", "MsgVpnQueue"},
		{"empty ref", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RefName(tc.ref))
		})
	}
}
