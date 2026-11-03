package portfolio

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// regression: F-portfolio-overflow
func TestPortfolioOverflowUnknown(t *testing.T) {
	const half = int64(1) << 62
	s := base(t)
	s.Segments[1].Cost = i64(half)
	retry := s.Segments[1]
	retry.Invocation.ID = "inv-0003"
	s.Segments = append(s.Segments, retry)
	l := measure(t, s)
	// The second 2^62 cannot join a subtotal that already holds 2^62: it is unknown in its
	// measure, in the bucket and the total alike, never a wrapped negative figure.
	want(t, "features cost", agent(l, "features", CostMicros), Quantity{half, 1, 1})
	want(t, "total cost", l.Total.Agent[CostMicros], Quantity{half + 1200, 2, 1})
	if sh := l.ShareOf("features", true, CostMicros); sh.Defined || sh.Numerator < 0 || sh.Denominator < 0 {
		t.Fatalf("overflowed cost produced a share: %+v", sh)
	}
	if lim := Compare(s.Policy, l).Limits[0]; lim.State != "exceeded" {
		t.Fatalf("a known subtotal above the limit is not exceeded: %+v", lim)
	}
	s.Policy.Limits[0].Max = math.MaxInt64
	if lim := Compare(s.Policy, measure(t, s)).Limits[0]; lim.State != "unknown" {
		t.Fatalf("an overflowed measure reported %s against its limit: %+v", lim.State, lim)
	}

	// Weighted time is a product: one that leaves the int64 range is unknown, not wrapped.
	w := base(t)
	w.Policy.Weights.Models[1].Milli = half
	l = measure(t, w)
	want(t, "features weighted", agent(l, "features", WeightedActive), Quantity{0, 0, 1})
	want(t, "features active", agent(l, "features", ActiveSeconds), Quantity{5880, 1, 0})

	// Human minutes go through the same checked sum.
	h := base(t)
	big := h.Decisions[2]
	big.ID, big.Quantity = "dec-5", i64(half)
	h.Decisions[2].Quantity = i64(half)
	h.Decisions = append(h.Decisions, big)
	want(t, "features minutes", human(measure(t, h), "features", DecisionMinutes), Quantity{half, 1, 1})

	// Active time is exact for every interval the time format admits; a duration does not
	// saturate at about 292 years.
	a := base(t)
	a.Policy.Window = Interval{"0001-01-01T00:00:00Z", "9999-01-01T00:00:00Z"}
	a.Segments[0].Active = &Interval{"1000-01-01T00:00:00Z", "3000-01-01T00:00:00Z"}
	start, _ := time.Parse(time.RFC3339, "1000-01-01T00:00:00Z")
	end, _ := time.Parse(time.RFC3339, "3000-01-01T00:00:00Z")
	exact := end.Unix() - start.Unix()
	l = measure(t, a)
	want(t, "long interval", agent(l, "reliability", ActiveSeconds), Quantity{exact, 1, 0})
	want(t, "long interval weighted", agent(l, "reliability", WeightedActive), Quantity{exact * 1000, 1, 0})
}

// regression: F-portfolio-overflow
func TestPortfolioOverflowTotal(t *testing.T) {
	const half = int64(1) << 62
	// Two tallies that each fit, 2^62 apiece, whose sum in the total does not: the value that
	// would wrap the total is unknown in its own tally and the total, so the total stays the
	// sum of its parts. Each case is the bucket-or-unattributed tally against the total.
	undefined := func(what string, l Ledger, isAgent bool, m string, buckets ...string) {
		t.Helper()
		for _, b := range buckets {
			if sh := l.ShareOf(b, isAgent, m); sh.Defined || sh.Numerator < 0 || sh.Denominator < 0 {
				t.Fatalf("%s: an overflowed total produced a %s share: %+v", what, b, sh)
			}
		}
	}

	// Two buckets: reliability (counted first) and features.
	s := base(t)
	s.Segments[0].Cost = i64(half)
	s.Segments[1].Cost = i64(half)
	l := measure(t, s)
	want(t, "cross-bucket reliability cost", agent(l, "reliability", CostMicros), Quantity{half, 1, 0})
	want(t, "cross-bucket features cost", agent(l, "features", CostMicros), Quantity{0, 0, 1})
	want(t, "cross-bucket total cost", l.Total.Agent[CostMicros], Quantity{half, 1, 1})
	undefined("cross-bucket", l, true, CostMicros, "reliability", "features")
	if lim := Compare(s.Policy, l).Limits[0]; lim.State != "unknown" {
		t.Fatalf("cross-bucket: an overflowed total reported %s against the features limit: %+v", lim.State, lim)
	}

	// Unattributed work (counted first) and a bucket.
	u := base(t)
	u.Segments[0].Subject = nil
	u.Segments[0].Cost = i64(half)
	u.Segments[1].Cost = i64(half)
	l = measure(t, u)
	want(t, "unattributed cost", agent(l, "", CostMicros), Quantity{half, 1, 0})
	want(t, "beside-unattributed features cost", agent(l, "features", CostMicros), Quantity{0, 0, 1})
	want(t, "beside-unattributed total cost", l.Total.Agent[CostMicros], Quantity{half, 1, 1})
	undefined("beside-unattributed", l, true, CostMicros, "features")

	// Human minutes in two buckets: features (dec-3, counted first) and reliability.
	h := base(t)
	h.Decisions[2].Quantity = i64(half)
	rel := h.Decisions[2]
	rel.ID, rel.Primary = "dec-5", h.Decisions[0].Primary
	h.Decisions = append(h.Decisions, rel)
	l = measure(t, h)
	want(t, "cross-bucket features minutes", human(l, "features", DecisionMinutes), Quantity{half, 1, 0})
	want(t, "cross-bucket reliability minutes", human(l, "reliability", DecisionMinutes), Quantity{0, 0, 1})
	want(t, "cross-bucket total minutes", l.Total.Human[DecisionMinutes], Quantity{half, 1, 1})
	undefined("cross-bucket minutes", l, false, DecisionMinutes, "features", "reliability")
}

// regression: F-portfolio-window
func TestPortfolioWindowEdges(t *testing.T) {
	// A measured interval straddling the window: the window cannot apportion the segment, so
	// every agent measure of it is unknown, its cost and elapsed proxy included.
	s := base(t)
	s.Segments[1].Active = &Interval{"2026-09-30T23:00:00Z", "2026-10-01T01:00:00Z"}
	l := measure(t, s)
	for _, m := range []string{ActiveSeconds, WeightedActive, CostMicros, ElapsedProxy} {
		want(t, "straddling "+m, agent(l, "features", m), Quantity{0, 0, 1})
	}
	// No measured interval: the segment is not placed against the window. Its cost and
	// proxy count as reported and its active time is unknown.
	s = base(t)
	s.Segments[1].Active = nil
	l = measure(t, s)
	want(t, "unplaced cost", agent(l, "features", CostMicros), Quantity{4000000, 1, 0})
	want(t, "unplaced proxy", agent(l, "features", ElapsedProxy), Quantity{6000, 1, 0})
	want(t, "unplaced active", agent(l, "features", ActiveSeconds), Quantity{0, 0, 1})

	// A conflict whose reports all lie outside the window is excluded and listed; it does not
	// add unknown records that would undefine in-window shares.
	s = base(t)
	out := s.Segments[1]
	out.Invocation.ID = "inv-0005"
	out.Active = &Interval{"2026-11-01T00:00:00Z", "2026-11-01T01:00:00Z"}
	rev := out
	rev.Revision = "r2"
	rev.Active = &Interval{"2026-11-02T00:00:00Z", "2026-11-02T01:00:00Z"}
	s.Segments = append(s.Segments, out, rev)
	l = measure(t, s)
	if len(l.Conflicts) != 0 || !listed(l.Excluded, "invocation", "desk-worker/inv-0005") {
		t.Fatalf("out-of-window conflict: conflicts %+v excluded %+v", l.Conflicts, l.Excluded)
	}
	if sh := l.ShareOf("features", true, ActiveSeconds); !sh.Defined {
		t.Fatalf("out-of-window conflict undefined an in-window share: %+v", sh)
	}
	// One report inside the window keeps the conflict, unknown in its bucket.
	rev.Active = &Interval{"2026-10-02T11:00:00Z", "2026-10-02T12:00:00Z"}
	s.Segments[len(s.Segments)-1] = rev
	l = measure(t, s)
	if len(l.Conflicts) != 1 {
		t.Fatalf("in-window conflict not reported: %+v", l.Conflicts)
	}
	want(t, "in-window conflict", agent(l, "features", ActiveSeconds), Quantity{5880, 1, 1})

	// The same holds for a decision identity whose conflicting reports are all outside.
	d := base(t)
	a := d.Decisions[0]
	a.ID, a.Window = "dec-9", Interval{"2026-11-01T00:00:00Z", "2026-11-01T01:00:00Z"}
	b := a
	b.Window = Interval{"2026-11-02T00:00:00Z", "2026-11-02T01:00:00Z"}
	d.Decisions = append(d.Decisions, a, b)
	l = measure(t, d)
	if len(l.Conflicts) != 0 || !listed(l.Excluded, "decision", "dec-9") {
		t.Fatalf("out-of-window decision conflict: conflicts %+v excluded %+v", l.Conflicts, l.Excluded)
	}
	want(t, "reliability decisions", human(l, "reliability", Decisions), Quantity{1, 1, 0})
	b.Window = Interval{"2026-10-06T00:00:00Z", "2026-10-06T01:00:00Z"}
	d.Decisions[len(d.Decisions)-1] = b
	want(t, "in-window decision conflict", human(measure(t, d), "reliability", Decisions), Quantity{1, 1, 1})

	// A reconciliation that leaves a conflict lists the invocation once, as a conflict; its
	// reports are not also listed as excluded.
	c := base(t)
	r2 := c.Segments[0]
	r2.Revision = "r2"
	c.Segments = append(c.Segments, r2)
	c.Reconciliations = []Reconciliation{{Invocation: r2.Invocation, Accept: []SegmentRef{{"s1", "r2"}, {"s9", "r1"}}, Reason: "s9 never reported", Source: "runner-ledger"}}
	l = measure(t, c)
	if len(l.Conflicts) != 1 || listed(l.Excluded, "segment", "desk-worker/inv-0001/s1@r1") {
		t.Fatalf("failed reconciliation listed twice: conflicts %+v excluded %+v", l.Conflicts, l.Excluded)
	}
}

func listed(entries []Entry, kind, key string) bool {
	for _, e := range entries {
		if e.Kind == kind && e.Key == key {
			return true
		}
	}
	return false
}

// regression: F-portfolio-overlap
func TestPortfolioOverlapSweep(t *testing.T) {
	const n = 20000
	s := base(t)
	tmpl := s.Segments[1]
	from, _ := time.Parse(time.RFC3339, "2026-10-03T00:00:00Z")
	accept := make([]SegmentRef, 0, n)
	for i := 0; i < n; i++ {
		g := tmpl
		g.ID = fmt.Sprintf("p%05d", i)
		at := from.Add(time.Duration(i) * 10 * time.Second)
		g.Active = &Interval{at.Format(time.RFC3339), at.Add(10 * time.Second).Format(time.RFC3339)}
		s.Segments = append(s.Segments, g)
		accept = append(accept, SegmentRef{g.ID, g.Revision})
	}
	accept = append(accept, SegmentRef{"s1", "r1"})
	s.Reconciliations = []Reconciliation{{Invocation: tmpl.Invocation, Accept: accept, Reason: "every report accepted", Source: "runner-ledger"}}
	began := time.Now()
	l := measure(t, s)
	if spent := time.Since(began); spent > 10*time.Second {
		t.Fatalf("one invocation's %d segments took %v: the overlap check is not linear", n, spent)
	}
	if len(l.Conflicts) != 0 {
		t.Fatalf("adjacent segments read as overlapping: %+v", l.Conflicts[0])
	}
	want(t, "many adjacent segments", agent(l, "features", ActiveSeconds), Quantity{5880 + n*10, n + 1, 0})
	// One planted overlap anywhere in the run is found.
	late := tmpl
	late.ID = "z-late"
	late.Active = &Interval{"2026-10-03T00:00:05Z", "2026-10-03T00:00:06Z"}
	s.Segments = append(s.Segments, late)
	s.Reconciliations[0].Accept = append(s.Reconciliations[0].Accept, SegmentRef{"z-late", "r1"})
	l = measure(t, s)
	if len(l.Conflicts) != 1 || !strings.Contains(l.Conflicts[0].Reason, "z-late") {
		t.Fatalf("planted overlap not found: %+v", l.Conflicts)
	}
}

// regression: F-portfolio-runner-strict
func TestPortfolioRunnerStrict(t *testing.T) {
	req, obs := runnerFixtures(t)
	source := func(Invocation) (Attribution, bool) {
		return Attribution{Segment: SegmentRef{"s1", "r1"}, ProfileRevision: "pr2", Source: "runner-ledger"}, true
	}
	if _, err := FromRunner(req, obs, source); err != nil {
		t.Fatal(err)
	}
	const planted = "planted-content-7"
	r, o := string(req), string(obs)
	requests := map[string]string{
		"unknown member":         strings.Replace(r, `"workspace":`, `"`+planted+`": 1, "workspace":`, 1),
		"case-variant duplicate": strings.Replace(r, `"caller": "desk-worker",`, `"caller": "desk-worker", "Caller": "`+planted+`",`, 1),
		"duplicate key":          strings.Replace(r, `"workspace": "workspaces/w1",`, `"workspace": "workspaces/w1", "workspace": "`+planted+`",`, 1),
		"missing workspace":      strings.Replace(r, `"workspace": "workspaces/w1",`, ``, 1),
		"nested unknown":         strings.Replace(r, `"validated_by": "claim-store"`, `"validated_by": "claim-store", "`+planted+`": 1`, 1),
		"other version":          strings.Replace(r, `loop-admin-runner/v1`, `loop-admin-runner/`+planted, 1),
		"trailing data":          r + `{"` + planted + `": 1}`,
	}
	observations := map[string]string{
		"unknown member":         strings.Replace(o, `"state": "finished",`, `"state": "finished", "`+planted+`": 1,`, 1),
		"case-variant duplicate": strings.Replace(o, `"actual_model": "model-a",`, `"actual_model": "model-a", "Actual_Model": "`+planted+`",`, 1),
		"usage unknown":          strings.Replace(o, `"cost_micros": 1500`, `"cost_micros": 1500, "`+planted+`": 1`, 1),
		"duplicate generation":   strings.Replace(o, `"generation": 3,`, `"generation": 3, "generation": 3,`, 1),
		"missing usage":          strings.Replace(o, `"usage": { "input_tokens": 1200, "output_tokens": 300, "cost_micros": 1500 },`, ``, 1),
	}
	check := func(what, name string, a, b []byte, changed string) {
		t.Helper()
		if changed == r || changed == o {
			t.Fatalf("%s %s: control did not change the document", what, name)
		}
		_, err := FromRunner(a, b, source)
		if err == nil {
			t.Fatalf("%s %s: accepted", what, name)
		}
		if strings.Contains(err.Error(), planted) {
			t.Fatalf("%s %s: refusal carries report content: %v", what, name, err)
		}
	}
	for name, doc := range requests {
		check("request", name, []byte(doc), obs, doc)
	}
	for name, doc := range observations {
		check("observation", name, req, []byte(doc), doc)
	}
	// An unattributed invocation is refused without naming it: its caller and id are what the
	// report said, not what failed.
	var named struct {
		Caller string `json:"caller"`
		ID     string `json:"id"`
	}
	if err := json.Unmarshal(req, &named); err != nil || named.Caller == "" || named.ID == "" {
		t.Fatalf("request fixture names no invocation: %+v %v", named, err)
	}
	_, err := FromRunner(req, obs, func(Invocation) (Attribution, bool) { return Attribution{}, false })
	if err == nil || strings.Contains(err.Error(), named.Caller) || strings.Contains(err.Error(), named.ID) {
		t.Fatalf("unattributed invocation: refusal %v carries report content", err)
	}
	runnerShape(t)
}

// runnerShape holds the translation's runner records to the runner schema's members and
// required lists, so the strict decode accepts exactly what the runner contract defines.
func runnerShape(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile("../../schemas/loop-admin-runner-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	type node struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	var schema struct {
		node
		Defs map[string]node `json:"$defs"`
	}
	if err := json.Unmarshal(data, &schema.node); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	prop := func(name string) node {
		var n node
		if err := json.Unmarshal(schema.Properties[name], &n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, c := range []struct {
		name   string
		record any
		node   node
	}{
		{"request", runnerRequest{}, schema.node},
		{"profile", runnerProfile{}, prop("profile")},
		{"authority", runnerAuthority{}, prop("authority")},
		{"observation", runnerObservation{}, schema.Defs["observation"]},
		{"usage", runnerUsage{}, schema.Defs["usage"]},
		{"result", runnerResult{}, schema.Defs["result"]},
	} {
		var fields []string
		rt := reflect.TypeOf(c.record)
		for i := 0; i < rt.NumField(); i++ {
			name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
			fields = append(fields, name)
		}
		var members []string
		for k := range c.node.Properties {
			members = append(members, k)
		}
		sort.Strings(fields)
		sort.Strings(members)
		if !reflect.DeepEqual(fields, members) {
			t.Fatalf("runner %s: record members %v, schema members %v", c.name, fields, members)
		}
		var required []string
		if rk, ok := c.record.(interface{ RequiredKeys() []string }); ok {
			required = append(required, rk.RequiredKeys()...)
		}
		want := append([]string(nil), c.node.Required...)
		sort.Strings(required)
		sort.Strings(want)
		if len(required) != 0 || len(want) != 0 {
			if !reflect.DeepEqual(required, want) {
				t.Fatalf("runner %s: required %v, schema required %v", c.name, required, want)
			}
		}
	}
}

// checkedHelpers are the only functions of this package allowed integer arithmetic: each
// reports when its result would leave the int64 range.
var checkedHelpers = map[string]bool{"addInt64": true, "subInt64": true, "mulInt64": true}

// arithmetic refuses an integer arithmetic operator outside the checked helpers. A "+" is a
// string concatenation, and allowed, only when a string literal joins it.
func arithmetic(source []byte) error {
	f, err := parser.ParseFile(token.NewFileSet(), "source.go", source, 0)
	if err != nil {
		return err
	}
	var found error
	ast.Inspect(f, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		switch x := n.(type) {
		case *ast.FuncDecl:
			return !checkedHelpers[x.Name.Name]
		case *ast.BinaryExpr:
			switch x.Op {
			case token.SUB, token.MUL, token.QUO, token.REM, token.SHL:
				found = fmt.Errorf("unchecked %s", x.Op)
			case token.ADD:
				if !joinsString(x) {
					found = fmt.Errorf("unchecked +")
				}
			}
		case *ast.UnaryExpr:
			if x.Op == token.SUB {
				found = fmt.Errorf("unchecked negation")
			}
		case *ast.AssignStmt:
			switch x.Tok {
			case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN, token.REM_ASSIGN, token.SHL_ASSIGN:
				found = fmt.Errorf("unchecked %s", x.Tok)
			}
		}
		return true
	})
	return found
}

func joinsString(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.BasicLit:
		return x.Kind == token.STRING
	case *ast.ParenExpr:
		return joinsString(x.X)
	case *ast.BinaryExpr:
		return x.Op == token.ADD && (joinsString(x.X) || joinsString(x.Y))
	}
	return false
}

// regression: F-portfolio-overflow
func TestPortfolioCheckedMath(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := arithmetic(data); err != nil {
			t.Fatalf("%s: %v (route integer arithmetic through addInt64, subInt64 or mulInt64)", path, err)
		}
		count++
	}
	if count == 0 {
		t.Fatal("no production files checked")
	}
	// Positive controls: each planted instance must be flagged, each concatenation passed.
	for _, src := range []string{
		"package p\nfunc f(a, b int64) int64 { return a * b }",
		"package p\nfunc f(a, b int64) int64 { return a - b }",
		"package p\nfunc f(a, b int64) int64 { return a + b }",
		"package p\nfunc f(a, b int64) { a += b }",
		"package p\nfunc f(a int64) int64 { return -a }",
		"package p\nvar x = 1 << 62",
		"package p\nfunc f(a, b int64) string { return \"n=\" + g(a*b) }",
	} {
		if arithmetic([]byte(src)) == nil {
			t.Fatalf("checked-math guard accepted %q", src)
		}
	}
	for _, src := range []string{
		"package p\nfunc f(a, b string) string { return a + \"/\" + b }",
		"package p\nfunc addInt64(a, b int64) int64 { return a + b }",
	} {
		if err := arithmetic([]byte(src)); err != nil {
			t.Fatalf("checked-math guard refused %q: %v", src, err)
		}
	}
}
