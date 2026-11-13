package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskcore/issueflow"
)

// The class-pass reference holds one worked batch. Each example is an
// `outcome:` line naming its group, an `expect: <state> <route>` line (a trailing
// " direct" marks a direct repair, "none" marks an item with no record) and the
// fenced json record after it. classPassProblems reads that text the way the
// shipped writer would: it decodes each record and assesses it with facts that
// match the record, then compares the result with the expect line.

const (
	classPassRef       = "../../../../plugins/assay/references/intake-class-pass.md"
	classPassPlant     = "testdata/class-pass-plant.md"
	classPassQuarMsg   = "quarantined item has a record"
	classPassNoneMsg   = "no example for outcome"
	classPassAssessMsg = "assessed as"
	classPassDupMsg    = "duplicate successor is not a brief or issue reference"
)

var (
	classPassOutcomes = []string{"matched", "class", "below", "quarantined"}
	classPassOutLine  = regexp.MustCompile(`^outcome: (\S+)$`)
	classPassExpect   = regexp.MustCompile(`^expect: (none|[a-z-]+ [a-z]+( direct)?)$`)
)

// classPassAllowed is what each group may hold: the expect lines its outcome
// can legitimately produce, and the disposition of the record behind them.
var classPassAllowed = map[string]struct{ expect, disposition string }{
	"matched":     {"observed duplicate", "duplicate"},
	"class":       {"observed duplicate", "duplicate"},
	"below":       {"observed repair direct", "repair"},
	"quarantined": {"none", ""},
}

func classPassProblems(doc string) []string {
	var problems []string
	add := func(line int, format string, args ...any) {
		problems = append(problems, fmt.Sprintf("line %d: ", line)+fmt.Sprintf(format, args...))
	}
	type pending struct {
		line    int
		spec    string
		outcome string
		done    bool
	}
	var cur *pending
	outcome := ""
	counts := map[string]int{}
	items := map[string]int{}
	flush := func() {
		if cur != nil && cur.spec != "none" && !cur.done {
			add(cur.line, "expect line has no record after it")
		}
		cur = nil
	}
	lines := strings.Split(doc, "\n")
	for i := 0; i < len(lines); i++ {
		line, n := lines[i], i+1
		switch {
		case strings.HasPrefix(line, "#"):
			flush()
			outcome = ""
		case classPassOutLine.MatchString(line):
			flush()
			outcome = classPassOutLine.FindStringSubmatch(line)[1]
			if _, ok := classPassAllowed[outcome]; !ok {
				add(n, "unknown outcome %q", outcome)
				outcome = ""
			}
		case strings.HasPrefix(line, "expect:"):
			flush()
			m := classPassExpect.FindStringSubmatch(line)
			if m == nil {
				add(n, "malformed expect line %q", line)
				continue
			}
			if outcome == "" {
				add(n, "expect line outside an outcome group")
				continue
			}
			cur = &pending{line: n, spec: m[1], outcome: outcome}
			counts[outcome]++
			want := classPassAllowed[outcome].expect
			if cur.spec != want {
				if outcome == "quarantined" {
					add(n, "%s expectation %q", classPassQuarMsg, cur.spec)
				} else {
					add(n, "outcome %s expects %q, not %q", outcome, want, cur.spec)
				}
			}
		case strings.HasPrefix(line, "```json"):
			start := n
			var body []string
			for i++; i < len(lines) && !strings.HasPrefix(lines[i], "```"); i++ {
				body = append(body, lines[i])
			}
			if cur == nil {
				add(start, "record without an expect line")
				continue
			}
			if cur.spec == "none" || cur.outcome == "quarantined" {
				add(start, "%s: it must have none", classPassQuarMsg)
				cur.done = true
				continue
			}
			if cur.done {
				add(start, "second record under one expect line")
				continue
			}
			cur.done = true
			h, err := issueflow.DecodeIntakeHandoff([]byte(strings.Join(body, "\n")))
			if err != nil {
				add(start, "record does not decode: %v", err)
				continue
			}
			if prev, dup := items[h.Item]; dup {
				add(start, "item %s already has a record at line %d", h.Item, prev)
			}
			items[h.Item] = start
			a := issueflow.AssessIntakeHandoff(h, issueflow.IntakeFacts{Item: h.Item, Opened: h.Opened, Complete: true})
			got := a.State + " " + a.Route
			if a.DirectRepair {
				got += " direct"
			}
			if got != cur.spec {
				add(start, "expected %q, %s %q (%s)", cur.spec, classPassAssessMsg, got, a.Reason)
			}
			if h.Disposition == issueflow.Duplicate && a.State != "observed" {
				add(start, "%s: %q", classPassDupMsg, h.Successor)
			}
			if want := classPassAllowed[cur.outcome].disposition; want != "" && string(h.Disposition) != want {
				add(start, "outcome %s holds a %s record", cur.outcome, h.Disposition)
			}
		}
	}
	flush()
	for _, o := range classPassOutcomes {
		if counts[o] == 0 {
			problems = append(problems, fmt.Sprintf("%s %s", classPassNoneMsg, o))
		}
	}
	return problems
}

func readClassPassDoc(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return string(raw)
}

func TestClassPassExamples(t *testing.T) {
	doc := readClassPassDoc(t, classPassRef)
	if problems := classPassProblems(doc); len(problems) > 0 {
		t.Fatalf("class-pass examples disagree with the shipped assessment:\n%s", strings.Join(problems, "\n"))
	}
	if n := strings.Count(doc, "\nexpect: "); n < 4 {
		t.Fatalf("reference holds %d expect lines, want at least 4", n)
	}
}

func TestClassPassFlagsPlant(t *testing.T) {
	// The plant gives a quarantined item a repair record. The reader must say so.
	t.Run("plant", func(t *testing.T) {
		problems := strings.Join(classPassProblems(readClassPassDoc(t, classPassPlant)), "\n")
		if !strings.Contains(problems, classPassQuarMsg) {
			t.Fatalf("reader did not report the planted record; it reported:\n%s", problems)
		}
	})
	// The same reader must also notice the other ways the examples can go wrong.
	doc := readClassPassDoc(t, classPassRef)
	mutations := []struct{ name, from, to, want string }{
		{"direct flag dropped", "expect: observed repair direct", "expect: observed repair", classPassAssessMsg},
		{"group relabelled", "outcome: below", "outcome: matched", classPassNoneMsg},
		{"quarantined expectation", "\nexpect: none\n", "\nexpect: observed repair direct\n", classPassQuarMsg},
		{"duplicate successor not a reference", `"successor": "example-stream/07"`, `"successor": "see the old brief"`, classPassDupMsg},
	}
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			mutated := strings.Replace(doc, m.from, m.to, 1)
			if mutated == doc {
				t.Fatalf("mutation matched nothing in the reference: %q", m.from)
			}
			if problems := strings.Join(classPassProblems(mutated), "\n"); !strings.Contains(problems, m.want) {
				t.Fatalf("reader did not report %q; it reported:\n%s", m.want, problems)
			}
		})
	}
}
