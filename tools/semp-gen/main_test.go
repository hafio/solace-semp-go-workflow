package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/tools/semp-gen/internal/ir"
	"solace-semp-workflow/tools/semp-gen/internal/render"
)

func TestActionCount(t *testing.T) {
	// One of each shape: a singleton (update only), a plain object (add+delete),
	// and one with HasUpdate (add+update+delete) -- 1 + 2 + 3 = 6.
	spec := &ir.Spec{Objects: []ir.Object{
		{IsSingleton: true},
		{},
		{HasUpdate: true},
	}}
	assert.Equal(t, 6, actionCount(spec))
}

func TestSummary(t *testing.T) {
	spec := &ir.Spec{Objects: []ir.Object{
		{Scope: ir.ScopeGlobal, IsSingleton: true, Attributes: []ir.Attribute{{Name: "a"}}},
		{Scope: ir.ScopeVPN, Attributes: []ir.Attribute{{Name: "b"}, {Name: "c"}}},
		{Scope: ir.ScopeVPN, HasUpdate: true, Deprecated: "superseded", Attributes: []ir.Attribute{{Name: "d"}}},
	}}

	// attrs: 1+2+1=4; VPN-scoped: 2 of the 3 objects; broker-global: the
	// remaining 1; fully deprecated: the one with a non-empty Deprecated note.
	assert.Equal(t, "4 attributes; 2 VPN-scoped, 1 broker-global; 1 objects fully deprecated", summary(spec))
}

func TestTruncate(t *testing.T) {
	five := []string{"a", "b", "c", "d", "e"}
	six := []string{"a", "b", "c", "d", "e", "f"}

	cases := []struct {
		name string
		in   []string
		want []string
	}{
		// Exactly at the limit must pass through untouched -- a drift report of
		// five files should list all five, not four plus an ellipsis.
		{"at the limit is left untouched", five, five},
		{"over the limit is cut to five plus an ellipsis marker", six, []string{"a", "b", "c", "d", "e", "..."}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, truncate(tc.in))
		})
	}
}

func TestStaleFiles(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(t *testing.T) string
		files     []render.File
		wantStale []string
		wantErr   bool
	}{
		{
			// gen_keep.go is still wanted so it must not appear; notgen.go is not a
			// generated descriptor at all and must be left alone regardless of
			// whether it is wanted; the two stale names come back sorted.
			name: "flags unwanted gen_ files, sorted, and ignores non-gen files",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				for _, name := range []string{"gen_zulu.go", "gen_alpha.go", "gen_keep.go", "notgen.go"} {
					require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package modules\n"), 0o644))
				}
				return dir
			},
			files:     []render.File{{Name: "gen_keep.go"}},
			wantStale: []string{"gen_alpha.go", "gen_zulu.go"},
		},
		{
			name: "nothing stale when every gen_ file on disk is still wanted",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "gen_keep.go"), []byte("package modules\n"), 0o644))
				return dir
			},
			files: []render.File{{Name: "gen_keep.go"}},
		},
		{
			// An outDir that cannot be read (a bad -out flag) must fail the run
			// rather than silently report nothing stale.
			name: "an unreadable output directory is an error",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "does-not-exist")
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := staleFiles(tc.setup(t), tc.files)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantStale, got)
		})
	}
}
