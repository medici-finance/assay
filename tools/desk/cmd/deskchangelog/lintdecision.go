package main

// The repo-wide `## Human decision` default lint, ported from
// tools/lint-human-decision-defaults.sh (now an adapter onto
// `deskchangelog lint-decision-defaults`).
//
// Every `gate: human` brief under ROOT/docs/streams whose `## Human decision` section is
// non-empty must carry a parseable default line — `Default if no answer: <X> after
// YYYY-MM-DD` or `Default if no answer: none — blocks until answered`. The one exemption is
// the placeholder a `decision-trigger: spec` brief carries before pickup: a section that is
// only comments, or a single `decision-trigger: spec` marker line.

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const lintName = "lint-human-decision-defaults"

var (
	defaultAfterRe = regexp.MustCompile(`Default if no answer:.*after[ \t\n\v\f\r]+[0-9]{4}-[0-9]{2}-[0-9]{2}`)
	specMarkerRe   = regexp.MustCompile(`^[ \t\n\v\f\r]*decision-trigger:[ \t\n\v\f\r]*spec([^0-9A-Za-z_-]|$)`)
)

const defaultBlocks = "Default if no answer: none — blocks until answered"

// posixSpace is the C locale's [[:space:]].
func posixSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

func blankLine(s string) bool {
	for i := 0; i < len(s); i++ {
		if !posixSpace(s[i]) {
			return false
		}
	}
	return true
}

// fileLines is a file's newline-separated records, as awk reads them.
func fileLines(b []byte) []string {
	s := string(b)
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// frontmatter is the lines between a first line of exactly `---` and the next `^---\s*$`.
func frontmatter(lines []string) []string {
	if len(lines) == 0 || lines[0] != "---" {
		return nil
	}
	var out []string
	for _, ln := range lines[1:] {
		if rest, ok := strings.CutPrefix(ln, "---"); ok && blankLine(rest) {
			break
		}
		out = append(out, ln)
	}
	return out
}

// fmField is awk -F': *' '/^key:/{v=$2; strip one leading and one trailing quote; trim
// trailing space}' over the first matching line.
func fmField(fm []string, key string) string {
	for _, ln := range fm {
		rest, ok := strings.CutPrefix(ln, key+":")
		if !ok {
			continue
		}
		v := strings.TrimLeft(rest, " ")
		v, _, _ = strings.Cut(v, ":")
		v = strings.TrimPrefix(v, `"`)
		v = strings.TrimSuffix(v, `"`)
		return strings.TrimRightFunc(v, func(r rune) bool { return r < 0x80 && posixSpace(byte(r)) })
	}
	return ""
}

// decisionBlock is the `## Human decision` section body with leading empty lines and
// trailing blank lines dropped.
func decisionBlock(lines []string) []string {
	var block []string
	in := false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "## Human decision") {
			in = true
			continue
		}
		if in && strings.HasPrefix(ln, "## ") {
			break
		}
		if in {
			block = append(block, ln)
		}
	}
	// awk 'NF{f=1} f': a line counts once it has a field (a character other than space/tab).
	for len(block) > 0 && strings.Trim(block[0], " \t") == "" {
		block = block[1:]
	}
	for len(block) > 0 && blankLine(block[len(block)-1]) {
		block = block[:len(block)-1]
	}
	return block
}

// isPlaceholder is the spec-trigger exemption: after removing every <!-- ... --> (an
// unterminated comment is not a placeholder), nothing is left, or exactly one line that is
// a `decision-trigger: spec` marker and not a default line.
func isPlaceholder(block []string) bool {
	s := strings.Join(block, "\n") + "\n"
	var out strings.Builder
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			break
		}
		out.WriteString(s[:i])
		j := strings.Index(s[i+4:], "-->")
		if j < 0 {
			return false
		}
		s = s[i+4+j+3:]
	}
	out.WriteString(s)
	var residue []string
	for _, ln := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if !blankLine(ln) {
			residue = append(residue, ln)
		}
	}
	if len(residue) == 0 {
		return true
	}
	if len(residue) != 1 || strings.Contains(residue[0], "Default if no answer:") {
		return false
	}
	return specMarkerRe.MatchString(asciiLower(residue[0]))
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

func hasParseableDefault(block []string) bool {
	for _, ln := range block {
		if defaultAfterRe.MatchString(ln) || strings.Contains(ln, defaultBlocks) {
			return true
		}
	}
	return false
}

// violates reports whether one brief file breaks the rule.
func violates(b []byte) bool {
	lines := fileLines(b)
	fm := frontmatter(lines)
	if fmField(fm, "gate") != "human" {
		return false
	}
	block := decisionBlock(lines)
	if len(block) == 0 {
		return false
	}
	trigger := fmField(fm, "decision-trigger")
	if trigger == "" {
		trigger = "start"
	}
	if trigger == "spec" && isPlaceholder(block) {
		return false
	}
	return !hasParseableDefault(block)
}

func runLintDecision(root string, stdout, stderr io.Writer) int {
	streams := root + "/docs/streams"
	if st, err := os.Stat(streams); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "%s: refused: %s/docs/streams not found\n", lintName, root)
		return 2
	}
	var files []string
	_ = filepath.WalkDir(streams, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		if ok, _ := filepath.Match("brief-*.md", d.Name()); !ok {
			return nil
		}
		if rel, rerr := filepath.Rel(streams, p); rerr == nil {
			files = append(files, streams+"/"+filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	violations := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil || !violates(b) {
			continue
		}
		fmt.Fprintf(stdout, "%s: has a `## Human decision` section with no parseable default\n", strings.TrimPrefix(f, root+"/"))
		violations++
	}
	if violations > 0 {
		fmt.Fprintf(stderr, "%s: %d brief(s) with an unparseable `## Human decision` default\n", lintName, violations)
		return 1
	}
	fmt.Fprintf(stdout, "%s: clean\n", lintName)
	return 0
}

func lintDecisionCommand(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "lint-decision-defaults [root]",
		Short: "report gate: human briefs whose '## Human decision' has no parseable default",
		Long: `Walks [root]/docs/streams (default: the working directory) for brief-*.md files and
reports each 'gate: human' brief whose non-empty '## Human decision' section carries
no parseable default line: 'Default if no answer: <X> after YYYY-MM-DD' or
'Default if no answer: none — blocks until answered'. A 'decision-trigger: spec'
brief's pre-pickup placeholder (comments only, or one 'decision-trigger: spec'
marker line) is exempt.

Exit codes: 0 clean · 1 one or more violations · 2 usage, or no docs/streams.`,
		Args: argsBetween(0, 1),
		RunE: func(_ *cobra.Command, a []string) error {
			root := "."
			if len(a) == 1 && a[0] != "" {
				root = a[0]
			}
			return codeErr(runLintDecision(root, stdout, stderr))
		},
	}
}
