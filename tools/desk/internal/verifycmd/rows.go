package verifycmd

import "strings"

// SplitRow splits one markdown pipe-table row into its cells, honouring the GFM
// escaped pipe: `\|` is cell content, never a column break, so a Command cell
// that writes a shell pipeline (`a \| b`) or an `\|\|` stays ONE cell. The escape
// is kept in the cell text exactly as written (`\|`); an executor un-escapes the
// command it lifts with UnescapePipes just before running it. One outer pipe is
// dropped from each end, so a zero-length leading cell keeps its column. This is
// the rule statusgen's Verify-row reader applies (statusgen/verifyrows.go
// splitRowEscaped), so the two readers agree on what a row's command is (#1953).
func SplitRow(line string) []string {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == '|':
			cur.WriteString(`\|`)
			i++
		case s[i] == '|':
			cells = append(cells, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(s[i])
		}
	}
	// A row closed by an outer pipe leaves one zero-length cell after it; a row
	// with no closing pipe keeps its last cell.
	if rest := cur.String(); rest != "" || len(cells) == 0 {
		cells = append(cells, rest)
	}
	return cells
}

// UnescapePipes turns a cell's GFM-escaped pipes back into the shell's `|`. It
// is applied to a lifted command just before it runs, as statusgen's
// UnescapePipes is, and to nothing else.
func UnescapePipes(s string) string {
	return strings.ReplaceAll(s, `\|`, "|")
}

// EscapePipes is UnescapePipes' inverse, for writing a command back into a
// table cell (a re-baselined row, an Evidence row) without breaking the table.
func EscapePipes(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}
