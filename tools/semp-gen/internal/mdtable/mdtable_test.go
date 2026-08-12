package mdtable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solace-semp-workflow/tools/semp-gen/internal/specmodel"
)

// A flag table is the only place Identifying/Const/Required/etc. are recorded,
// so a row must produce exactly the flags marked and nothing else -- a stray
// true would tell the generator an attribute is Const (or Required, or
// Read-Only) when the specification never said so.
func TestParseFlagTable_MarksOnlyDeclaredFlags(t *testing.T) {
	const doc = `Attribute|Identifying|Const|Required
:---|:---:|:---:|:---:
queueName|x||
maxMsgSpoolUsage||x|x
`
	parsed, err := Parse(doc)
	require.NoError(t, err)

	require.Contains(t, parsed.Attrs, "queueName")
	assert.True(t, parsed.Attrs["queueName"]["Identifying"])
	assert.NotContains(t, parsed.Attrs["queueName"], "Const")
	assert.NotContains(t, parsed.Attrs["queueName"], "Required")

	require.Contains(t, parsed.Attrs, "maxMsgSpoolUsage")
	assert.True(t, parsed.Attrs["maxMsgSpoolUsage"]["Const"])
	assert.True(t, parsed.Attrs["maxMsgSpoolUsage"]["Required"])
	assert.NotContains(t, parsed.Attrs["maxMsgSpoolUsage"], "Identifying")
}

// SEMP documents the same object across several operations, and the real
// caller merges Parsed.Attrs from each into one map. That merge only works if
// a single Parse call itself accumulates onto the same attribute when a
// description embeds more than one flag table (a create table plus one for a
// nested definition, say) rather than the second table silently overwriting
// the first.
func TestParseFlagTable_AccumulatesAcrossTablesInOneDescription(t *testing.T) {
	const doc = `Some intro prose about the object.

Attribute|Identifying|Required
:---|:---:|:---:
attrOne|x|
attrTwo||x

More prose between the two tables.

Attribute|Deprecated|Opaque
:---|:---:|:---:
attrOne||x
attrThree|x|
`
	parsed, err := Parse(doc)
	require.NoError(t, err)

	// attrOne is marked in both tables; a single Parse call must carry the
	// first table's flag forward rather than starting a fresh entry.
	assert.True(t, parsed.Attrs["attrOne"]["Identifying"])
	assert.True(t, parsed.Attrs["attrOne"]["Opaque"])
	assert.False(t, parsed.Attrs["attrOne"]["Deprecated"])

	assert.True(t, parsed.Attrs["attrTwo"]["Required"])
	assert.True(t, parsed.Attrs["attrThree"]["Deprecated"])
}

// An unrecognised flag column is a broker release the generator has not been
// taught about yet. Silently ignoring the column would mis-generate every
// object that uses it, so this must fail loud and say which column.
func TestParseFlagTable_UnknownColumnErrorsAndNamesTheColumn(t *testing.T) {
	const doc = `Attribute|Identifying|Frobnicate
:---|:---:|:---:
attrOne|x|x
`
	parsed, err := Parse(doc)

	require.Error(t, err)
	assert.Nil(t, parsed)
	assert.Contains(t, err.Error(), "Frobnicate")
}

// DELETE operations, and plenty of trivial objects, document no attribute
// metadata at all. That must produce an empty result, not an error.
func TestParse_NoTablesIsEmptyNotError(t *testing.T) {
	const doc = "Delete a queue. Any attribute missing from the request will be left unchanged.\n\nThis operation has no tables."

	parsed, err := Parse(doc)

	require.NoError(t, err)
	assert.Empty(t, parsed.Attrs)
	assert.Empty(t, parsed.Combinations)
}

// The Conflicts column is optional -- some objects document only a Requires
// relationship -- so both shapes must parse to the same Combination fields,
// with Conflicts simply absent rather than causing a misalignment of the
// other columns.
func TestParseCombinationTable_WithAndWithoutConflictsColumn(t *testing.T) {
	cases := []struct {
		name string
		doc  string
	}{
		{
			name: "with Conflicts column",
			doc: `Class|Attribute|Requires|Conflicts
:---|:---|:---|:---
Broker|tlsServerCertPassword|tlsServerCertContent|
`,
		},
		{
			name: "without Conflicts column",
			doc: `Class|Attribute|Requires
:---|:---|:---
Broker|tlsServerCertPassword|tlsServerCertContent
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := Parse(tc.doc)
			require.NoError(t, err)

			want := []Combination{{
				Class:     "Broker",
				Attribute: "tlsServerCertPassword",
				Requires:  []string{"tlsServerCertContent"},
			}}
			assert.Equal(t, want, parsed.Combinations)
		})
	}
}

// Requires and Conflicts are free-form comma lists in the source Markdown, not
// pre-normalised, so the parser -- not the broker -- is responsible for
// trimming stray whitespace and dropping the empty entries a double comma or
// a missing cell would otherwise leave behind.
func TestParseCombinationTable_SplitsAndTrimsRequiresAndConflicts(t *testing.T) {
	const doc = `Class|Attribute|Requires|Conflicts
:---|:---|:---|:---
Foo|attrX|a, b ,,c|d,  e
Foo|attrY||
`
	parsed, err := Parse(doc)
	require.NoError(t, err)
	require.Len(t, parsed.Combinations, 2)

	assert.Equal(t, []string{"a", "b", "c"}, parsed.Combinations[0].Requires)
	assert.Equal(t, []string{"d", "e"}, parsed.Combinations[0].Conflicts)

	// An empty cell is "no requirement", not a list containing one blank
	// entry.
	assert.Nil(t, parsed.Combinations[1].Requires)
	assert.Nil(t, parsed.Combinations[1].Conflicts)
}

// The specification is not consistent about the case of its "x" marks, and a
// cell holding some other letter (seen where an author typed a stray
// character) must stay unmarked rather than being treated as a mark by
// accident.
func TestParseFlagTable_XMarkIsCaseInsensitiveAndSelective(t *testing.T) {
	const doc = `Attribute|Identifying|Required
:---|:---:|:---:
attrOne|X|y
attrTwo|x|
`
	parsed, err := Parse(doc)
	require.NoError(t, err)

	assert.True(t, parsed.Attrs["attrOne"]["Identifying"], "uppercase X must mark the flag")
	assert.NotContains(t, parsed.Attrs["attrOne"], "Required", "a 'y' cell is not a mark")
	assert.True(t, parsed.Attrs["attrTwo"]["Identifying"], "lowercase x must mark the flag")
}

// A line that merely contains a pipe -- ordinary prose mentioning two things
// separated by "|" -- is not a table without a separator row beneath it. This
// is the difference between reading Markdown and reading "any line with a
// pipe in it", and getting it wrong would fabricate attribute rows out of
// running text.
func TestParse_TableLikeTextWithoutSeparatorIsIgnored(t *testing.T) {
	const doc = "This mentions A|B and C|D inline but is not a table.\n" +
		"It continues as prose on the next line, with a hyphen - here too.\n"

	parsed, err := Parse(doc)

	require.NoError(t, err)
	assert.Empty(t, parsed.Attrs)
	assert.Empty(t, parsed.Combinations)
}

// The generator trusts Parse to cover the entire real specification, not just
// the shapes above. This runs it over every operation description in the
// committed spec and checks the totals stay at least as large as the figures
// verified by hand against 10.26.0.8755, so a parser regression that starts
// dropping rows is caught even though no single crafted case would show it.
func TestParse_RealSpecificationHasPlausibleRowCounts(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "specs", "software", "10.26.0.8755", "semp-v2-config.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("real spec not present: " + path)
	}

	doc, err := specmodel.Load(path)
	require.NoError(t, err)

	var flagRows, comboRows int
	for pathKey, item := range doc.Paths {
		for verb, op := range item {
			if op.Description == "" {
				continue
			}
			parsed, err := Parse(op.Description)
			require.NoErrorf(t, err, "%s %s", verb, pathKey)
			flagRows += len(parsed.Attrs)
			comboRows += len(parsed.Combinations)
		}
	}

	// Verified by hand against 10.26.0.8755: 1649 flag rows, 727 combination
	// rows. A lower bound, not an exact match, so a future spec bump that adds
	// attributes does not fail this test spuriously.
	assert.GreaterOrEqual(t, flagRows, 1649)
	assert.GreaterOrEqual(t, comboRows, 727)
}
