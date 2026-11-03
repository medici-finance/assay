package portfolio

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func base(t *testing.T) Snapshot {
	t.Helper()
	data, err := os.ReadFile("../testdata/portfolio/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func measure(t *testing.T, s Snapshot) Ledger {
	t.Helper()
	l, err := Measure(s)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func i64(v int64) *int64 { return &v }

func agent(l Ledger, bucket, m string) Quantity {
	if bucket == "" {
		return l.Unattributed.Agent[m]
	}
	return l.Buckets[bucket].Agent[m]
}

func human(l Ledger, bucket, m string) Quantity {
	if bucket == "" {
		return l.Unattributed.Human[m]
	}
	return l.Buckets[bucket].Human[m]
}

func want(t *testing.T, what string, got, expect Quantity) {
	t.Helper()
	if got != expect {
		t.Fatalf("%s: got %+v, want %+v", what, got, expect)
	}
}

// regression: F-portfolio-measurement
func TestPortfolioLedgerIdentityAndUnits(t *testing.T) {
	s := base(t)
	retry := s.Segments[0]
	retry.Invocation.ID = "inv-0003"
	s.Segments = append(s.Segments, s.Segments[0], retry, s.Segments[0])
	s.Decisions = append(s.Decisions, s.Decisions[0])
	l := measure(t, s)
	// Two duplicate reports of inv-0001/s1 count once; the retry, otherwise identical, is
	// its own invocation and counts once too.
	want(t, "reliability active", agent(l, "reliability", ActiveSeconds), Quantity{240, 2, 0})
	want(t, "reliability cost", agent(l, "reliability", CostMicros), Quantity{2400, 2, 0})
	want(t, "reliability decisions", human(l, "reliability", Decisions), Quantity{1, 1, 0})
	if len(l.Duplicates) != 3 {
		t.Fatalf("duplicate reports not listed: %+v", l.Duplicates)
	}
	// Counts are not time: decision minutes never join the decision count, and neither
	// human measure appears among agent durations.
	want(t, "features decisions", human(l, "features", Decisions), Quantity{1, 1, 0})
	want(t, "features minutes", human(l, "features", DecisionMinutes), Quantity{30, 1, 0})
	want(t, "total decisions", l.Total.Human[Decisions], Quantity{3, 3, 0})
	for m := range l.Total.Agent {
		if m == Decisions || m == DecisionMinutes {
			t.Fatalf("human measure %s tallied as agent work", m)
		}
	}
	want(t, "unattributed decision stays visible", human(l, "", Decisions), Quantity{1, 1, 0})
	// Weighted time uses the actual model's explicit weight.
	want(t, "features weighted", agent(l, "features", WeightedActive), Quantity{5880 * 2500, 1, 0})

	// A second revision of one segment is a conflict, not a second count.
	revised := s.Segments[0]
	revised.Revision = "r2"
	revised.Active = &Interval{"2026-10-02T10:00:00Z", "2026-10-02T10:05:00Z"}
	c := base(t)
	c.Segments = append(c.Segments, revised)
	l = measure(t, c)
	want(t, "conflict stays unknown", agent(l, "reliability", ActiveSeconds), Quantity{0, 0, 1})
	if len(l.Conflicts) != 1 || l.Conflicts[0].Key != "desk-worker/inv-0001" {
		t.Fatalf("conflict not reported: %+v", l.Conflicts)
	}
	c.Reconciliations = []Reconciliation{{Invocation: revised.Invocation, Accept: []SegmentRef{{"s1", "r2"}}, Reason: "r2 supersedes r1", Source: "runner-ledger"}}
	l = measure(t, c)
	want(t, "reconciled once", agent(l, "reliability", ActiveSeconds), Quantity{300, 1, 0})
	if len(l.Excluded) != 1 || !strings.Contains(l.Excluded[0].Key, "@r1") {
		t.Fatalf("reconciliation did not list the rejected report: %+v", l.Excluded)
	}

	// Two segments of one invocation with overlapping intervals need reconciliation.
	o := base(t)
	second := o.Segments[0]
	second.ID = "s2"
	second.Active = &Interval{"2026-10-02T10:01:00Z", "2026-10-02T10:03:00Z"}
	o.Segments = append(o.Segments, second)
	l = measure(t, o)
	if len(l.Conflicts) != 1 || agent(l, "reliability", ActiveSeconds).Known != 0 {
		t.Fatalf("overlap double-counted: %+v %+v", l.Conflicts, agent(l, "reliability", ActiveSeconds))
	}
	second.Active = &Interval{"2026-10-02T10:02:00Z", "2026-10-02T10:03:00Z"}
	o.Segments[len(o.Segments)-1] = second
	want(t, "adjacent segments both count", agent(measure(t, o), "reliability", ActiveSeconds), Quantity{180, 2, 0})

	// A decision identity reported twice with different content is a visible conflict.
	d := base(t)
	moved := d.Decisions[0]
	moved.Window = Interval{"2026-10-06T00:00:00Z", "2026-10-06T01:00:00Z"}
	d.Decisions = append(d.Decisions, moved)
	l = measure(t, d)
	want(t, "conflicting decision", human(l, "reliability", Decisions), Quantity{0, 0, 1})
}

// regression: F-portfolio-measurement
func TestPortfolioUnknownAndZero(t *testing.T) {
	s := base(t)
	s.Segments[0].Cost = nil
	s.Segments[1].ActualModel = "model-unlisted"
	l := measure(t, s)
	// Unknown cost taints cost only; an unlisted model's weight taints weighted time only.
	want(t, "reliability cost", agent(l, "reliability", CostMicros), Quantity{0, 0, 1})
	want(t, "features cost", agent(l, "features", CostMicros), Quantity{4000000, 1, 0})
	want(t, "total cost", l.Total.Agent[CostMicros], Quantity{4000000, 1, 1})
	want(t, "features weighted", agent(l, "features", WeightedActive), Quantity{0, 0, 1})
	want(t, "reliability weighted", agent(l, "reliability", WeightedActive), Quantity{120000, 1, 0})
	want(t, "active untouched", l.Total.Agent[ActiveSeconds], Quantity{6000, 2, 0})
	if sh := l.ShareOf("features", true, CostMicros); sh.Defined || !strings.Contains(sh.Reason, "unknown") {
		t.Fatalf("incomplete cost produced a share: %+v", sh)
	}
	if sh := l.ShareOf("reliability", true, ActiveSeconds); !sh.Defined || sh.Numerator != 120 || sh.Denominator != 6000 {
		t.Fatalf("complete active share lost: %+v", sh)
	}
	// An unmeasured interval is unknown active time, not zero, and a straddling one too.
	s = base(t)
	s.Segments[0].Active = nil
	s.Segments[1].Active = &Interval{"2026-09-30T23:00:00Z", "2026-10-01T01:00:00Z"}
	l = measure(t, s)
	want(t, "unmeasured", l.Total.Agent[ActiveSeconds], Quantity{0, 0, 2})
	// All zero: no records leaves every share undefined, never 0%.
	z := base(t)
	z.Segments, z.Decisions = []Segment{}, []Decision{}
	l = measure(t, z)
	for _, b := range z.Policy.Buckets {
		for _, sh := range []Share{l.ShareOf(b.ID, true, ActiveSeconds), l.ShareOf(b.ID, false, Decisions)} {
			if sh.Defined || !strings.Contains(sh.Reason, "zero total") {
				t.Fatalf("zero total produced a share: %+v", sh)
			}
		}
	}
	for _, d := range Compare(z.Policy, l).Divergences {
		if d.State != Undefined {
			t.Fatalf("zero total compared as %s", d.State)
		}
	}
	// A measured zero is a real reading: zero cost everywhere is still a zero total.
	z = base(t)
	z.Segments[0].Cost, z.Segments[1].Cost = i64(0), i64(0)
	if sh := measure(t, z).ShareOf("features", true, CostMicros); sh.Defined || !strings.Contains(sh.Reason, "zero total") {
		t.Fatalf("all-zero cost produced a share: %+v", sh)
	}
}

func runnerFixtures(t *testing.T) ([]byte, []byte) {
	t.Helper()
	req, err := os.ReadFile("../../loopadmin/testdata/runner/standing-desk-request.json")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := os.ReadFile("../testdata/portfolio/runner-observation.json")
	if err != nil {
		t.Fatal(err)
	}
	return req, obs
}

// runnerKeys asserts the observation fixture uses only members the runner contract defines,
// so the translation is fed a report the runner could actually emit.
func runnerKeys(t *testing.T, obs []byte) {
	t.Helper()
	data, err := os.ReadFile("../../schemas/loop-admin-runner-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Defs map[string]struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(obs, &doc); err != nil {
		t.Fatal(err)
	}
	check := func(def string, value map[string]json.RawMessage) {
		d := schema.Defs[def]
		for k := range value {
			if _, ok := d.Properties[k]; !ok {
				t.Fatalf("runner %s has no member %q", def, k)
			}
		}
		for _, k := range d.Required {
			if _, ok := value[k]; !ok {
				t.Fatalf("runner %s fixture lacks required %q", def, k)
			}
		}
	}
	check("observation", doc)
	for _, def := range []string{"usage", "result"} {
		var v map[string]json.RawMessage
		if err := json.Unmarshal(doc[def], &v); err != nil {
			t.Fatal(err)
		}
		check(def, v)
	}
}

// regression: F-portfolio-measurement
func TestPortfolioRunnerAdapterFlow(t *testing.T) {
	req, obs := runnerFixtures(t)
	runnerKeys(t, obs)
	s := base(t)
	reliability := &s.Policy.Buckets[0].Members[0]
	attributed := Attribution{
		Segment: SegmentRef{"s1", "r1"}, ProfileRevision: "pr2", Source: "runner-ledger", Subject: reliability,
		Active: &Interval{"2026-10-02T10:00:00Z", "2026-10-02T10:04:00Z"}, Elapsed: i64(900),
	}
	source := func(a Attribution) AttributionSource {
		return func(inv Invocation) (Attribution, bool) { return a, inv == Invocation{"desk-worker", "inv-0001"} }
	}
	g, err := FromRunner(req, obs, source(attributed))
	if err != nil {
		t.Fatal(err)
	}
	if g.Profile != (Profile{"profile-a", "pr2", "model-a"}) || g.ActualModel != "model-a" || g.Cost == nil || *g.Cost != 1500 {
		t.Fatalf("runner identity, profile or model lost: %+v", g)
	}
	s.Segments[0] = g
	l := measure(t, s)
	want(t, "attributed active", agent(l, "reliability", ActiveSeconds), Quantity{240, 1, 0})
	want(t, "attributed weighted", agent(l, "reliability", WeightedActive), Quantity{240000, 1, 0})
	want(t, "proxy separate", agent(l, "reliability", ElapsedProxy), Quantity{900, 1, 0})

	// No measured interval: the elapsed proxy stays a proxy and active time is unknown.
	unmeasured := attributed
	unmeasured.Active = nil
	g, err = FromRunner(req, obs, source(unmeasured))
	if err != nil {
		t.Fatal(err)
	}
	s.Segments[0] = g
	l = measure(t, s)
	want(t, "missing interval", agent(l, "reliability", ActiveSeconds), Quantity{0, 0, 1})
	want(t, "missing interval weighted", agent(l, "reliability", WeightedActive), Quantity{0, 0, 1})
	want(t, "proxy kept", agent(l, "reliability", ElapsedProxy), Quantity{900, 1, 0})
	if sh := l.ShareOf("reliability", true, ActiveSeconds); sh.Defined {
		t.Fatalf("missing interval produced an active share: %+v", sh)
	}

	// The pinned model never stands in for an unreported actual model.
	var o map[string]any
	_ = json.Unmarshal(obs, &o)
	delete(o, "actual_model")
	noModel, _ := json.Marshal(o)
	g, err = FromRunner(req, noModel, source(attributed))
	if err != nil || g.ActualModel != "" || g.Profile.Model != "model-a" {
		t.Fatalf("pinned model substituted: %+v %v", g, err)
	}
	// The translation refuses without an attribution source, an unattributed invocation,
	// a result echoing another invocation and an unsupported runner version.
	if _, err := FromRunner(req, obs, nil); err == nil {
		t.Fatal("translated without an attribution source")
	}
	if _, err := FromRunner(req, obs, func(Invocation) (Attribution, bool) { return Attribution{}, false }); err == nil {
		t.Fatal("translated an unattributed invocation")
	}
	o["result"].(map[string]any)["id"] = "inv-9999"
	other, _ := json.Marshal(o)
	if _, err := FromRunner(req, other, source(attributed)); err == nil {
		t.Fatal("accepted a result echoing another invocation")
	}
	future := []byte(strings.Replace(string(req), "loop-admin-runner/v1", "loop-admin-runner/v2", 1))
	if _, err := FromRunner(future, obs, source(attributed)); err == nil {
		t.Fatal("accepted an unsupported runner version")
	}
}

// regression: F-portfolio-measurement
func TestPortfolioTargetNotConsumption(t *testing.T) {
	s := base(t)
	before, _ := json.Marshal(s.Policy)
	l := measure(t, s)
	c := Compare(s.Policy, l)
	after, _ := json.Marshal(s.Policy)
	if string(before) != string(after) {
		t.Fatal("comparison rewrote the policy")
	}
	got := map[string]Divergence{}
	for _, d := range c.Divergences {
		got[d.Bucket+"/"+d.Party] = d
	}
	r := got["reliability/agent"]
	if r.State != Divergent || r.TargetBasis != 500 || r.Observed.Numerator != 120 || r.Observed.Denominator != 6000 {
		t.Fatalf("5%% target against 2%% consumption: %+v", r)
	}
	if h := got["reliability/human"]; h.State != Divergent || h.TargetBasis != 2000 || h.Observed.Numerator != 1 || h.Observed.Denominator != 3 {
		t.Fatalf("human target compared against the wrong measure: %+v", h)
	}
	if len(c.Limits) != 1 || c.Limits[0].State != "within" || c.Limits[0].Value.Known != 4000000 {
		t.Fatalf("limit status: %+v", c.Limits)
	}
	if !reflect.DeepEqual(c, Compare(s.Policy, l)) {
		t.Fatal("comparison is not a pure function of its inputs")
	}
	// The divergence is reported, not acted on: measuring again after the comparison sees
	// the same policy and the same ledger.
	if again := measure(t, s); !reflect.DeepEqual(again, l) {
		t.Fatal("comparison changed the measured ledger")
	}
	// Within the explicit tolerance the same consumption is aligned.
	s.Policy.Buckets[0].AgentTarget.BasisPoints = 230
	if d := Compare(s.Policy, l).Divergences[0]; d.State != Aligned || d.TargetBasis != 230 {
		t.Fatalf("tolerance band: %+v", d)
	}
	// An exceeded limit is reported from the known subtotal even when coverage is partial.
	s = base(t)
	s.Policy.Limits[0].Max = 100
	s.Segments = append(s.Segments, Segment{Invocation: Invocation{"desk-worker", "inv-0009"}, ID: "s1", Revision: "r1",
		Profile: Profile{"profile-b", "pr1", "model-b"}, Source: "runner-ledger", Subject: s.Segments[1].Subject})
	if lim := Compare(s.Policy, measure(t, s)).Limits[0]; lim.State != "exceeded" {
		t.Fatalf("exceeded limit hidden by unknown coverage: %+v", lim)
	}
	s.Policy.Limits[0].Max = 9000000
	if lim := Compare(s.Policy, measure(t, s)).Limits[0]; lim.State != "unknown" {
		t.Fatalf("incomplete measure reported within its limit: %+v", lim)
	}
}

// regression: F-portfolio-measurement
func TestPortfolioDecode(t *testing.T) {
	data, err := os.ReadFile("../testdata/portfolio/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	refused := map[string]string{
		"unknown member":      strings.Replace(doc, `"tolerance-basis-points": 50,`, `"tolerance-basis-points": 50, "default-weight": 1000,`, 1),
		"case-folded member":  strings.Replace(doc, `"scope":`, `"Scope":`, 1),
		"duplicate member":    strings.Replace(doc, `"revision": "p3",`, `"revision": "p3", "revision": "p4",`, 1),
		"missing decisions":   strings.Replace(doc, `"decisions": [`, `"other": [`, 1),
		"other version":       strings.Replace(doc, `portfolio-measurement/v1`, `portfolio-measurement/v2`, 1),
		"trailing data":       doc + `{}`,
		"ambiguous bucket":    strings.Replace(doc, `REQ-feature-search" }],`, `REQ-feature-search" }, { "repository": "example-org/product", "requirement": "REQ-reliability-core" }],`, 1),
		"uncovered source":    strings.Replace(doc, `"source": "decision-log"`, `"source": "chat-export"`, 1),
		"proxy as target":     strings.Replace(doc, `"agent-target": { "measure": "active-seconds", "basis-points": 500 }`, `"agent-target": { "measure": "elapsed-seconds-proxy", "basis-points": 500 }`, 1),
		"human count as time": strings.Replace(doc, `"human-target": { "measure": "decisions", "basis-points": 2000 }`, `"human-target": { "measure": "active-seconds", "basis-points": 2000 }`, 1),
		"quantity on count":   strings.Replace(doc, `"unit": "decision",`, `"unit": "decision", "quantity": 2,`, 1),
		"minutes no quantity": strings.Replace(doc, `"quantity": 30,`, ``, 1),
		"negative cost":       strings.Replace(doc, `"cost-micros": 1200`, `"cost-micros": -1`, 1),
		"local time":          strings.Replace(doc, `2026-10-02T10:02:00Z`, `2026-10-02T10:02:00+01:00`, 1),
		"reversed interval":   strings.Replace(doc, `2026-10-02T10:02:00Z`, `2026-10-02T09:59:00Z`, 1),
		"targets over 100%":   strings.Replace(doc, `"basis-points": 9500`, `"basis-points": 9600`, 1),
		"missing tolerance":   strings.Replace(doc, `"tolerance-basis-points": 50,`, ``, 1),
	}
	for name, input := range refused {
		if input == doc {
			t.Fatalf("%s: control did not change the document", name)
		}
		if _, err := Decode([]byte(input)); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

// pureImports is the closed allow-list for production files of this package.
var pureImports = map[string]bool{
	"encoding/json": true, "fmt": true, "math/big": true, "regexp": true, "sort": true,
	"time": true, "unicode/utf8": true, "github.com/medici-finance/assay/deskcore/domain": true,
}

// impure names the time functions that read the clock; parsing a supplied time is pure.
var impure = map[string]bool{"Now": true, "Since": true, "Until": true, "Sleep": true, "After": true, "Tick": true, "NewTimer": true, "NewTicker": true, "AfterFunc": true}

func boundary(source []byte) error {
	f, err := parser.ParseFile(token.NewFileSet(), "source.go", source, 0)
	if err != nil {
		return err
	}
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if !pureImports[path] {
			return &boundaryError{"import " + path}
		}
	}
	var found error
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "time" && impure[sel.Sel.Name] {
				found = &boundaryError{"time." + sel.Sel.Name}
			}
		}
		return found == nil
	})
	return found
}

type boundaryError struct{ what string }

func (e *boundaryError) Error() string { return "non-pure portfolio use: " + e.what }

// regression: F-portfolio-measurement
func TestPortfolioBoundary(t *testing.T) {
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
		if err := boundary(data); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		count++
	}
	if count == 0 {
		t.Fatal("no production files checked")
	}
	for _, src := range []string{
		"package p\nimport \"os\"",
		"package p\nimport \"net/http\"",
		"package p\nimport \"os/exec\"",
		"package p\nimport \"io/fs\"",
		"package p\nimport \"example.invalid/dispatch\"",
		"package p\nimport \"time\"\nvar _ = time.Now",
	} {
		if boundary([]byte(src)) == nil {
			t.Fatalf("boundary accepted %q", src)
		}
	}
}
