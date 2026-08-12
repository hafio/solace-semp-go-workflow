// Package mdtable parses the Markdown tables SEMP embeds in its operation
// descriptions. Those tables are the only machine-readable source of the
// per-attribute metadata the generator needs -- Identifying, Const, Required,
// Read-Only, Write-Only, Requires-Disable, Auto-Disable, Deprecated and Opaque
// are documented nowhere else in the specification.
//
// Column sets vary by verb and by object, so headers are read dynamically and an
// unrecognised column is a hard error: a broker release that adds a flag must be
// a reviewed change to knownFlags, not a silent loss of fidelity.
package mdtable

import (
	"fmt"
	"regexp"
	"strings"
)

// knownFlags is every flag column the generator understands. Verified against
// the 10.26.0.8755 specification, which uses exactly these nine.
var knownFlags = map[string]bool{
	"Identifying":      true,
	"Const":            true,
	"Required":         true,
	"Read-Only":        true,
	"Write-Only":       true,
	"Requires-Disable": true,
	"Auto-Disable":     true,
	"Deprecated":       true,
	"Opaque":           true,
}

// separator matches a Markdown header underline such as ":---|:---:|:---:".
var separator = regexp.MustCompile(`^[:\-| ]+$`)

// Flags is the set of flag columns marked for one attribute.
type Flags map[string]bool

// Combination is one row of a "Class|Attribute|Requires|Conflicts" table, which
// records attributes that may only be supplied together with, or instead of,
// their siblings. Class is the definition the attribute belongs to, which for a
// threshold group is the nested definition rather than the top-level object.
type Combination struct {
	Class     string
	Attribute string
	Requires  []string
	Conflicts []string
}

// Parsed is everything of interest in one operation description.
type Parsed struct {
	// Attrs maps an attribute name to the flags marked for it. Only attributes
	// with at least one flag appear -- the full attribute list comes from the
	// definition's properties.
	Attrs map[string]Flags
	// Combinations are the co-dependency rows, in document order.
	Combinations []Combination
}

// Parse extracts the flag and combination tables from an operation description.
// A description with no tables yields an empty Parsed and no error, which is the
// normal case for DELETE and for trivial objects.
func Parse(description string) (*Parsed, error) {
	out := &Parsed{Attrs: map[string]Flags{}}
	lines := strings.Split(description, "\n")

	for i := 0; i < len(lines); i++ {
		header, rows, next, ok := readTable(lines, i)
		if !ok {
			continue
		}
		i = next - 1

		switch header[0] {
		case "Attribute":
			if err := out.addFlagTable(header, rows); err != nil {
				return nil, err
			}
		case "Class":
			out.addCombinationTable(header, rows)
		}
	}
	return out, nil
}

// readTable reads a Markdown table starting at lines[i], returning its header
// cells, its data rows and the index just past the table.
func readTable(lines []string, i int) (header []string, rows [][]string, next int, ok bool) {
	if !strings.Contains(lines[i], "|") || i+1 >= len(lines) {
		return nil, nil, 0, false
	}
	under := strings.TrimSpace(lines[i+1])
	if !strings.Contains(under, "-") || !separator.MatchString(under) {
		return nil, nil, 0, false
	}
	header = cells(lines[i])
	j := i + 2
	for j < len(lines) && strings.Contains(lines[j], "|") && strings.TrimSpace(lines[j]) != "" {
		rows = append(rows, cells(lines[j]))
		j++
	}
	return header, rows, j, true
}

func cells(line string) []string {
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// addFlagTable merges one "Attribute|<flags...>" table. Flags accumulate across
// the several operations that document the same object, so a create table and an
// update table together give the full picture.
func (p *Parsed) addFlagTable(header []string, rows [][]string) error {
	for _, col := range header[1:] {
		if col == "" {
			continue
		}
		if !knownFlags[col] {
			return fmt.Errorf("unknown attribute flag column %q: a new broker release has added a flag, "+
				"so mdtable.knownFlags and the intermediate representation must be extended deliberately", col)
		}
	}
	for _, row := range rows {
		name := row[0]
		if name == "" {
			continue
		}
		if p.Attrs[name] == nil {
			p.Attrs[name] = Flags{}
		}
		for c := 1; c < len(header) && c < len(row); c++ {
			if header[c] != "" && strings.EqualFold(row[c], "x") {
				p.Attrs[name][header[c]] = true
			}
		}
	}
	return nil
}

// addCombinationTable merges one "Class|Attribute|Requires[|Conflicts]" table.
// The Conflicts column is optional; some objects document requirements only.
func (p *Parsed) addCombinationTable(header []string, rows [][]string) {
	idx := map[string]int{}
	for i, c := range header {
		idx[c] = i
	}
	for _, row := range rows {
		get := func(col string) string {
			if i, ok := idx[col]; ok && i < len(row) {
				return row[i]
			}
			return ""
		}
		attr := get("Attribute")
		if attr == "" {
			continue
		}
		p.Combinations = append(p.Combinations, Combination{
			Class:     get("Class"),
			Attribute: attr,
			Requires:  splitList(get("Requires")),
			Conflicts: splitList(get("Conflicts")),
		})
	}
}

// splitList splits a comma-separated cell, dropping empties.
func splitList(cell string) []string {
	if strings.TrimSpace(cell) == "" {
		return nil
	}
	parts := strings.Split(cell, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
