package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Class guard for #3939: a pull-request workflow that triggers on `labeled` /
// `unlabeled` and whose concurrency group cancels the run in progress on ANY
// label event. A label that the check does not read cannot change its verdict,
// but the cancel still lands, and the cancelled check-run stays on the
// unchanged head as a red X after a later run on the same head passes. The
// ready-flip's two-label swap did this to every flipped PR on changelog-check.
//
// The guard evaluates each concurrency site's `cancel-in-progress` for a
// `labeled` and an `unlabeled` event carrying a label no workflow reads, and
// fails when it comes out true. It models only the GitHub expression subset it
// needs (string and boolean literals, ==, !=, !, &&, ||, parentheses and the
// three context values below). Any other shape is an error, never a pass.
//
// Why not a job-level `if:` that skips unrelated label events instead: a
// skipped job reports success, so on a required check a skip on the same head
// replaces a red verdict. That shape is out of this guard's scope; the workflow
// comment says not to use it.

// labelCancelUnrelated is the label name used for the simulated events. No
// workflow in this repository reads a label of this name.
const labelCancelUnrelated = "unrelated-label-3939"

// regression: #3939
func TestLabelEventsNeverCancel(t *testing.T) {
	dir := filepath.Join(fixtureRepoRoot, ".github", "workflows")
	skipIfFixtureAbsent(t, dir, ".github/ is not part of this repository's published file set")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	read, labelled := 0, 0
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".yml" && ext != ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		read++
		faults, isLabelled, err := labelCancelFaults(raw)
		if err != nil {
			t.Errorf("%s: could-not-check: %v", e.Name(), err)
			continue
		}
		if isLabelled {
			labelled++
		}
		for _, f := range faults {
			t.Errorf("%s: %s", e.Name(), f)
		}
	}
	if read == 0 {
		t.Fatalf("read 0 workflow files under %s; this check proved nothing", dir)
	}
	t.Logf("%d workflow files read, %d trigger on label events", read, labelled)
}

// TestLabelCancelGuardFires is the positive control: the pre-#3939 shape and
// its variants must be reported, the fixed shape and unlabelled workflows must
// not, and a shape the evaluator does not model must be an error.
func TestLabelCancelGuardFires(t *testing.T) {
	const labelOn = "on:\n  pull_request:\n    types: [opened, synchronize, labeled, unlabeled]\n"
	cases := []struct {
		name, src string
		faults    int
		labelled  bool
		wantErr   bool
	}{
		{"pre-fix shape", labelOn + "concurrency:\n  group: g-${{ github.event.pull_request.number }}\n  cancel-in-progress: true\n", 2, true, false},
		{"fixed shape", labelOn + "concurrency:\n  group: g\n  cancel-in-progress: ${{ (github.event.action != 'labeled' && github.event.action != 'unlabeled') || github.event.label.name == 'changelog:skip' }}\n", 0, true, false},
		{"job-level cancel", labelOn + "jobs:\n  a:\n    concurrency:\n      group: g\n      cancel-in-progress: true\n", 2, true, false},
		{"one action missed", labelOn + "concurrency:\n  group: g\n  cancel-in-progress: ${{ github.event.action != 'labeled' }}\n", 1, true, false},
		{"case-insensitive ==", labelOn + "concurrency:\n  group: g\n  cancel-in-progress: ${{ !(github.event.action == 'LABELED' || github.event.action == 'Unlabeled') }}\n", 0, true, false},
		{"string literal true", labelOn + "concurrency:\n  group: g\n  cancel-in-progress: 'true'\n", 2, true, false},
		{"pull_request_target", "on:\n  pull_request_target:\n    types: labeled\nconcurrency:\n  group: g\n  cancel-in-progress: true\n", 1, true, false},
		{"group-only form", labelOn + "concurrency: g\n", 0, true, false},
		{"no cancel key", labelOn + "concurrency:\n  group: g\n", 0, true, false},
		{"cancel false", labelOn + "concurrency:\n  group: g\n  cancel-in-progress: false\n", 0, true, false},
		{"unlabelled workflow", "on:\n  pull_request:\n    types: [opened, synchronize]\nconcurrency:\n  group: g\n  cancel-in-progress: true\n", 0, false, false},
		{"default pull_request types", "on: [pull_request, push]\nconcurrency:\n  group: g\n  cancel-in-progress: true\n", 0, false, false},
		{"unmodelled context", labelOn + "concurrency:\n  group: g\n  cancel-in-progress: ${{ github.ref == 'main' }}\n", 0, true, true},
		{"unmodelled trigger shape", "on:\n  pull_request: 7\n", 0, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			faults, labelled, err := labelCancelFaults([]byte(c.src))
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, want error %v", err, c.wantErr)
			}
			if err != nil {
				return
			}
			if labelled != c.labelled || len(faults) != c.faults {
				t.Fatalf("labelled=%v faults=%d %q; want labelled=%v faults=%d", labelled, len(faults), faults, c.labelled, c.faults)
			}
		})
	}
}

// labelCancelFaults returns one fault per concurrency site and label action
// whose cancel-in-progress is true for an unrelated label, and whether the
// workflow triggers on label events at all.
func labelCancelFaults(src []byte) ([]string, bool, error) {
	var wf map[string]any
	if err := yaml.Unmarshal(src, &wf); err != nil {
		return nil, false, err
	}
	events, err := labelTriggerEvents(wf["on"])
	if err != nil || len(events) == 0 {
		return nil, false, err
	}
	type site struct {
		where string
		conc  any
	}
	sites := []site{{"concurrency", wf["concurrency"]}}
	jobs, _ := wf["jobs"].(map[string]any)
	ids := make([]string, 0, len(jobs))
	for id := range jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if job, ok := jobs[id].(map[string]any); ok {
			sites = append(sites, site{"jobs." + id + ".concurrency", job["concurrency"]})
		}
	}
	var faults []string
	for _, s := range sites {
		if s.conc == nil {
			continue
		}
		if _, groupOnly := s.conc.(string); groupOnly {
			continue // group-only form: cancel-in-progress defaults to false
		}
		m, ok := s.conc.(map[string]any)
		if !ok {
			return nil, true, fmt.Errorf("%s: unmodelled shape %T", s.where, s.conc)
		}
		cip, ok := m["cancel-in-progress"]
		if !ok {
			continue
		}
		for _, ev := range events {
			ctx := map[string]string{
				"github.event_name":       ev[0],
				"github.event.action":     ev[1],
				"github.event.label.name": labelCancelUnrelated,
			}
			on, err := labelCancelValue(cip, ctx)
			if err != nil {
				return nil, true, fmt.Errorf("%s: %v", s.where, err)
			}
			if on {
				faults = append(faults, fmt.Sprintf("%s: cancel-in-progress is true on %s/%s for a label the check does not read, "+
					"so label churn cancels a run on an unchanged head (#3939)", s.where, ev[0], ev[1]))
			}
		}
	}
	return faults, true, nil
}

// labelTriggerEvents returns the (event, action) pairs for every label action
// a pull_request or pull_request_target trigger fires on.
func labelTriggerEvents(on any) ([][2]string, error) {
	m, ok := on.(map[string]any)
	if !ok {
		// A bare event name or a list of names carries no types:, and the
		// default pull-request types include no label action.
		switch on.(type) {
		case string, []any, nil:
			return nil, nil
		}
		return nil, fmt.Errorf("on: unmodelled shape %T", on)
	}
	var out [][2]string
	for _, ev := range []string{"pull_request", "pull_request_target"} {
		v, present := m[ev]
		if !present || v == nil {
			continue
		}
		cfg, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("on.%s: unmodelled shape %T", ev, v)
		}
		var types []string
		switch tv := cfg["types"].(type) {
		case nil:
		case string:
			types = []string{tv}
		case []any:
			for _, x := range tv {
				s, ok := x.(string)
				if !ok {
					return nil, fmt.Errorf("on.%s.types: unmodelled entry %T", ev, x)
				}
				types = append(types, s)
			}
		default:
			return nil, fmt.Errorf("on.%s.types: unmodelled shape %T", ev, tv)
		}
		for _, ty := range types {
			if ty == "labeled" || ty == "unlabeled" {
				out = append(out, [2]string{ev, ty})
			}
		}
	}
	return out, nil
}

// labelCancelValue resolves a cancel-in-progress value for one simulated event.
func labelCancelValue(v any, ctx map[string]string) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		s := strings.TrimSpace(x)
		if inner, ok := strings.CutPrefix(s, "${{"); ok {
			inner, ok = strings.CutSuffix(inner, "}}")
			if !ok {
				return false, fmt.Errorf("unterminated expression %q", s)
			}
			p := &ghExprParser{src: inner, ctx: ctx}
			val, err := p.parse()
			if err != nil {
				return false, fmt.Errorf("expression %q: %v", s, err)
			}
			return val.truthy(), nil
		}
		switch strings.ToLower(s) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		return false, fmt.Errorf("unmodelled literal %q", s)
	}
	return false, fmt.Errorf("unmodelled value %T", v)
}

// ghExprVal is a string or a boolean GitHub expression value.
type ghExprVal struct {
	isStr bool
	s     string
	b     bool
}

func (v ghExprVal) truthy() bool {
	if v.isStr {
		return v.s != ""
	}
	return v.b
}

// ghExprParser is a recursive-descent evaluator for the subset above:
//
//	or  := and ('||' and)*      and := not ('&&' not)*
//	not := '!' not | cmp        cmp := atom (('=='|'!=') atom)?
//	atom := '(' or ')' | 'string' | true | false | context.path
type ghExprParser struct {
	src string
	pos int
	ctx map[string]string
}

func (p *ghExprParser) parse() (ghExprVal, error) {
	v, err := p.or()
	if err == nil {
		p.space()
		if p.pos < len(p.src) {
			err = fmt.Errorf("unexpected %q at %d", p.src[p.pos:], p.pos)
		}
	}
	return v, err
}

func (p *ghExprParser) space() {
	for p.pos < len(p.src) && strings.ContainsRune(" \t\n\r", rune(p.src[p.pos])) {
		p.pos++
	}
}

func (p *ghExprParser) eat(tok string) bool {
	p.space()
	if strings.HasPrefix(p.src[p.pos:], tok) {
		p.pos += len(tok)
		return true
	}
	return false
}

func (p *ghExprParser) or() (ghExprVal, error) {
	l, err := p.and()
	for err == nil && p.eat("||") {
		var r ghExprVal
		if r, err = p.and(); err == nil && !l.truthy() {
			l = r
		}
	}
	return l, err
}

func (p *ghExprParser) and() (ghExprVal, error) {
	l, err := p.not()
	for err == nil && p.eat("&&") {
		var r ghExprVal
		if r, err = p.not(); err == nil && l.truthy() {
			l = r
		}
	}
	return l, err
}

func (p *ghExprParser) not() (ghExprVal, error) {
	p.space()
	if strings.HasPrefix(p.src[p.pos:], "!") && !strings.HasPrefix(p.src[p.pos:], "!=") {
		p.pos++
		v, err := p.not()
		return ghExprVal{b: !v.truthy()}, err
	}
	return p.cmp()
}

func (p *ghExprParser) cmp() (ghExprVal, error) {
	l, err := p.atom()
	if err != nil {
		return l, err
	}
	neg := false
	switch {
	case p.eat("=="):
	case p.eat("!="):
		neg = true
	default:
		return l, nil
	}
	r, err := p.atom()
	if err != nil {
		return r, err
	}
	if l.isStr != r.isStr {
		return ghExprVal{}, fmt.Errorf("unmodelled mixed-type comparison")
	}
	eq := l.b == r.b
	if l.isStr {
		eq = strings.EqualFold(l.s, r.s) // GitHub compares strings case-insensitively
	}
	return ghExprVal{b: eq != neg}, nil
}

func (p *ghExprParser) atom() (ghExprVal, error) {
	p.space()
	rest := p.src[p.pos:]
	switch {
	case p.eat("("):
		v, err := p.or()
		if err == nil && !p.eat(")") {
			err = fmt.Errorf("missing ) at %d", p.pos)
		}
		return v, err
	case strings.HasPrefix(rest, "'"):
		var b strings.Builder
		for i := 1; i < len(rest); i++ {
			if rest[i] != '\'' {
				b.WriteByte(rest[i])
				continue
			}
			if i+1 < len(rest) && rest[i+1] == '\'' {
				b.WriteByte('\'')
				i++
				continue
			}
			p.pos += i + 1
			return ghExprVal{isStr: true, s: b.String()}, nil
		}
		return ghExprVal{}, fmt.Errorf("unterminated string at %d", p.pos)
	}
	end := 0
	for end < len(rest) && (rest[end] == '.' || rest[end] == '_' || rest[end] == '-' ||
		(rest[end] >= 'a' && rest[end] <= 'z') || (rest[end] >= 'A' && rest[end] <= 'Z') || (rest[end] >= '0' && rest[end] <= '9')) {
		end++
	}
	word := rest[:end]
	p.pos += end
	switch word {
	case "true", "false":
		return ghExprVal{b: word == "true"}, nil
	case "":
		return ghExprVal{}, fmt.Errorf("unexpected %q at %d", rest, p.pos)
	}
	v, ok := p.ctx[word]
	if !ok {
		return ghExprVal{}, fmt.Errorf("unmodelled context %q", word)
	}
	return ghExprVal{isStr: true, s: v}, nil
}
