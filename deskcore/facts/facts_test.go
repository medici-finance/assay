package facts

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskcore/domain"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// sample is a valid bundle: one complete source with one fact, one partial source with one
// fact, and one failed source.
const sample = `{
  "schema": "desk-facts-v1",
  "snapshot_id": "0123456789abcdef",
  "collected_at": "2026-10-01T12:00:00Z",
  "policy_digest": "",
  "graph_revision": "abc123",
  "pattern_digest": "",
  "sources": [
    {"identity": "repo-a/issues", "revision": "etag-1", "collected_at": "2026-10-01T11:59:00Z", "completeness": "complete", "pagination_done": true},
    {"identity": "repo-b/issues", "revision": "", "collected_at": "2026-10-01T11:59:30Z", "completeness": "partial", "pagination_done": false},
    {"identity": "repo-c/issues", "revision": "", "collected_at": "2026-10-01T11:59:40Z", "completeness": "failed", "pagination_done": false}
  ],
  "collection_errors": [
    {"source": "repo-b/issues", "cause": "page 3 returned 502"},
    {"source": "repo-c/issues", "cause": "permission denied"},
    {"source": "repo-d/issues", "cause": "repository could not be enumerated"}
  ],
  "facts": [
    {"id": "f1", "source": "repo-a/issues", "kind": "issue.open", "subject": "7", "observed_at": "2026-10-01T11:59:00Z", "value": {"title": "x"}},
    {"id": "f2", "source": "repo-b/issues", "kind": "issue.open", "subject": "9", "observed_at": "2026-10-01T11:59:30Z"}
  ]
}`

func load(t *testing.T, s string) *Bundle {
	t.Helper()
	b, err := Load(strings.NewReader(s))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

func edit(t *testing.T, mutate func(m map[string]any)) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(sample), &m); err != nil {
		t.Fatal(err)
	}
	mutate(m)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func source(m map[string]any, i int) map[string]any {
	return m["sources"].([]any)[i].(map[string]any)
}

func TestLoadValidBundle(t *testing.T) {
	b := load(t, sample)
	if len(b.Sources) != 3 || len(b.Facts) != 2 || len(b.CollectionErrors) != 3 {
		t.Fatalf("unexpected shape: %+v", b)
	}
}

func TestUnknownSchemaRefused(t *testing.T) {
	for name, s := range map[string]string{
		"v2":      strings.Replace(sample, `"desk-facts-v1"`, `"desk-facts-v2"`, 1),
		"missing": strings.Replace(sample, `"schema": "desk-facts-v1",`, ``, 1),
		"empty":   strings.Replace(sample, `"desk-facts-v1"`, `""`, 1),
	} {
		_, err := Load(strings.NewReader(s))
		if !errors.Is(err, ErrUnknownSchema) {
			t.Errorf("%s: got %v, want ErrUnknownSchema", name, err)
		}
	}
}

func TestLoadRefusesMalformedBundles(t *testing.T) {
	cases := map[string]string{
		"no pagination_done": edit(t, func(m map[string]any) { delete(source(m, 0), "pagination_done") }),
		"no collection_errors": edit(t, func(m map[string]any) {
			delete(m, "collection_errors")
		}),
		"null collection_errors": edit(t, func(m map[string]any) { m["collection_errors"] = nil }),
		"no policy_digest":       edit(t, func(m map[string]any) { delete(m, "policy_digest") }),
		"unknown field":          edit(t, func(m map[string]any) { m["evaluator_version"] = "1.0.0" }),
		"bad snapshot_id":        edit(t, func(m map[string]any) { m["snapshot_id"] = "NOT-HEX" }),
		"bad policy_digest":      edit(t, func(m map[string]any) { m["policy_digest"] = "sha256:short" }),
		"complete w/o pages":     edit(t, func(m map[string]any) { source(m, 0)["pagination_done"] = false }),
		"partial unexplained": edit(t, func(m map[string]any) {
			m["collection_errors"] = m["collection_errors"].([]any)[1:]
		}),
		"complete with error": edit(t, func(m map[string]any) {
			m["collection_errors"] = append(m["collection_errors"].([]any), map[string]any{"source": "repo-a/issues", "cause": "x"})
		}),
		"bad completeness": edit(t, func(m map[string]any) { source(m, 0)["completeness"] = "mostly" }),
		"dup source":       edit(t, func(m map[string]any) { source(m, 1)["identity"] = "repo-a/issues" }),
		"source after bundle": edit(t, func(m map[string]any) {
			source(m, 0)["collected_at"] = "2026-10-01T12:00:01Z"
		}),
		"fact undeclared source": edit(t, func(m map[string]any) {
			m["facts"].([]any)[0].(map[string]any)["source"] = "repo-z/issues"
		}),
		"fact from failed source": edit(t, func(m map[string]any) {
			m["facts"].([]any)[0].(map[string]any)["source"] = "repo-c/issues"
		}),
		"dup fact id": edit(t, func(m map[string]any) {
			m["facts"].([]any)[1].(map[string]any)["id"] = "f1"
		}),
		"fact no subject": edit(t, func(m map[string]any) {
			delete(m["facts"].([]any)[0].(map[string]any), "subject")
		}),
		"trailing data": sample + `{}`,
	}
	for name, s := range cases {
		if _, err := Load(strings.NewReader(s)); err == nil {
			t.Errorf("%s: Load accepted a malformed bundle", name)
		}
	}
}

func TestQueryStates(t *testing.T) {
	b := load(t, sample)
	fresh := t0.Add(time.Minute)
	hour := time.Hour
	cases := []struct {
		name string
		q    Query
		now  time.Time
		want State
	}{
		{"present", Query{"repo-a/issues", "issue.open", "7", hour}, fresh, Present},
		{"known negative", Query{"repo-a/issues", "issue.open", "8", hour}, fresh, KnownNegative},
		{"stale hit", Query{"repo-a/issues", "issue.open", "7", hour}, t0.Add(2 * hour), Stale},
		{"stale miss", Query{"repo-a/issues", "issue.open", "8", hour}, t0.Add(2 * hour), Stale},
		{"partial hit", Query{"repo-b/issues", "issue.open", "9", hour}, fresh, Present},
		{"partial miss", Query{"repo-b/issues", "issue.open", "10", hour}, fresh, CouldNotCheck},
		{"failed", Query{"repo-c/issues", "issue.open", "1", hour}, fresh, CouldNotCheck},
		{"absent source", Query{"repo-d/issues", "issue.open", "1", hour}, fresh, CouldNotCheck},
		{"no bound", Query{"repo-a/issues", "issue.open", "8", 0}, fresh, CouldNotCheck},
	}
	for _, c := range cases {
		got := b.Query(c.q, c.now)
		if got.State != c.want {
			t.Errorf("%s: state %s (%s), want %s", c.name, got.State, got.Reason, c.want)
		}
		if got.State != Present && got.State != Stale && len(got.Facts) != 0 {
			t.Errorf("%s: a %s answer carried facts", c.name, got.State)
		}
	}
	if got := b.Query(Query{"repo-a/issues", "issue.open", "7", hour}, t0.Add(2*hour)); len(got.Facts) != 1 {
		t.Errorf("a stale answer dropped the matching fact")
	}
}

// TestStaleIsNotKnownNegative pins the distinction directly: the same complete source that
// answers known-negative within its bound answers stale beyond it.
func TestStaleIsNotKnownNegative(t *testing.T) {
	b := load(t, sample)
	q := Query{"repo-a/issues", "issue.open", "8", 30 * time.Minute}
	if s := b.Query(q, t0).State; s != KnownNegative {
		t.Fatalf("within bound: %s, want known-negative", s)
	}
	if s := b.Query(q, t0.Add(31*time.Minute)).State; s != Stale {
		t.Fatalf("beyond bound: %s, want stale", s)
	}
}

// TestSchemaMatchesGoTypes keeps schemas/desk-facts-v1.json and the Go types in step: every
// JSON field of each type is a schema property, every schema property is a Go field, and the
// schema's required lists equal the keys Load checks for presence.
func TestSchemaMatchesGoTypes(t *testing.T) {
	raw, err := os.ReadFile("../schemas/desk-facts-v1.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var s struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
		Defs       map[string]struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	var errItem struct {
		Items struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"items"`
	}
	if err := json.Unmarshal(s.Properties["collection_errors"], &errItem); err != nil {
		t.Fatal(err)
	}
	check := func(name string, typ any, props map[string]json.RawMessage, required, presence []string) {
		t.Helper()
		got := keysOf(props)
		want := jsonFields(reflect.TypeOf(typ))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: schema properties %v, Go fields %v", name, got, want)
		}
		if presence != nil && !reflect.DeepEqual(sorted(required), sorted(presence)) {
			t.Errorf("%s: schema required %v, Load presence check %v", name, sorted(required), sorted(presence))
		}
	}
	check("bundle", Bundle{}, s.Properties, s.Required, Bundle{}.RequiredKeys())
	check("source", Source{}, s.Defs["source"].Properties, s.Defs["source"].Required, Source{}.RequiredKeys())
	check("fact", domain.Fact{}, s.Defs["fact"].Properties, s.Defs["fact"].Required, domain.Fact{}.RequiredKeys())
	check("collection error", CollectionError{}, errItem.Items.Properties, errItem.Items.Required, CollectionError{}.RequiredKeys())
}

func keysOf(m map[string]json.RawMessage) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return sorted(out)
}

func jsonFields(t reflect.Type) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		tag := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		if tag != "" && tag != "-" {
			out = append(out, tag)
		}
	}
	return sorted(out)
}

func sorted(s []string) []string {
	c := append([]string(nil), s...)
	sort.Strings(c)
	return c
}

// TestLoadRefusesKeyVariants: a key that differs from the contract only in case, or a key given
// twice, is refused at every level. encoding/json alone folds case and keeps the last duplicate,
// so without this a bundle the schema refuses could decode as complete.
func TestLoadRefusesKeyVariants(t *testing.T) {
	src0 := `{"identity": "repo-a/issues", "revision": "etag-1", "collected_at": "2026-10-01T11:59:00Z", "completeness": "complete", "pagination_done": true}`
	cases := map[string]string{
		"source case variant overrides": strings.Replace(sample, src0,
			`{"identity": "repo-e/issues", "revision": "", "collected_at": "2026-10-01T11:59:00Z", "completeness": "partial", "pagination_done": false, "Completeness": "complete", "PAGINATION_DONE": true}`+",\n    "+src0, 1),
		"source duplicate key": strings.Replace(sample, src0,
			`{"identity": "repo-e/issues", "revision": "", "collected_at": "2026-10-01T11:59:00Z", "completeness": "partial", "pagination_done": false, "completeness": "complete", "pagination_done": true}`+",\n    "+src0, 1),
		"bundle case variant": strings.Replace(sample, `"graph_revision": "abc123",`, `"graph_revision": "abc123", "Graph_Revision": "x",`, 1),
		"bundle duplicate":    strings.Replace(sample, `"graph_revision": "abc123",`, `"graph_revision": "abc123", "graph_revision": "def456",`, 1),
		"fact case variant":   strings.Replace(sample, `"subject": "9",`, `"subject": "9", "Subject": "10",`, 1),
		"error case variant":  strings.Replace(sample, `"cause": "permission denied"`, `"cause": "permission denied", "Cause": "x"`, 1),
		"error duplicate":     strings.Replace(sample, `"cause": "permission denied"`, `"cause": "permission denied", "cause": "x"`, 1),
		"value duplicate":     strings.Replace(sample, `"value": {"title": "x"}`, `"value": {"title": "x", "title": "y"}`, 1),
	}
	for name, s := range cases {
		if s == sample {
			t.Fatalf("%s: the fixture edit did not apply", name)
		}
		if _, err := Load(strings.NewReader(s)); err == nil {
			t.Errorf("%s: Load accepted a bundle with a non-contract key", name)
		}
	}
}

// TestFutureDatedSourceIsNotFresh: a source or bundle collected after the caller's "now" cannot
// answer present or known-negative. A negative age must never pass a freshness bound.
func TestFutureDatedSourceIsNotFresh(t *testing.T) {
	b := load(t, sample)
	q := Query{"repo-a/issues", "issue.open", "8", time.Hour}
	before := t0.Add(-time.Hour) // now is before the source was collected
	if got := b.Query(q, before); got.State != CouldNotCheck {
		t.Errorf("source collected after now: %s (%s), want could-not-check", got.State, got.Reason)
	}
	if got := b.Query(Query{"repo-a/issues", "issue.open", "7", time.Hour}, before); got.State != CouldNotCheck {
		t.Errorf("present fact from a source collected after now: %s, want could-not-check", got.State)
	}
	// The bundle itself dated after now, with its source just before now: still not answerable.
	future := edit(t, func(m map[string]any) { m["collected_at"] = "2076-10-01T12:00:00Z" })
	fb := load(t, future)
	if got := fb.Query(q, t0.Add(time.Minute)); got.State != CouldNotCheck {
		t.Errorf("bundle collected after now: %s, want could-not-check", got.State)
	}
}

// TestAgeAtRefusesAFutureTime isolates ageAt from the bundle-level check in Query. Load's
// Validate bounds every source by the bundle time, so a loaded bundle never reaches ageAt with a
// future source unless the bundle is future too; a Bundle built in Go with no bundle time does,
// and only ageAt stands between it and a fresh answer.
func TestAgeAtRefusesAFutureTime(t *testing.T) {
	for _, c := range []struct {
		name   string
		t      time.Time
		want   time.Duration
		wantOK bool
	}{
		{"past", t0.Add(-time.Minute), time.Minute, true},
		{"now", t0, 0, true},
		{"one nanosecond ahead", t0.Add(time.Nanosecond), 0, false},
		{"an hour ahead", t0.Add(time.Hour), 0, false},
	} {
		got, ok := ageAt(t0, c.t)
		if got != c.want || ok != c.wantOK {
			t.Errorf("ageAt(%s): got (%s, %v), want (%s, %v)", c.name, got, ok, c.want, c.wantOK)
		}
	}
	b := &Bundle{Sources: []Source{{Identity: "s", Completeness: Complete, PaginationDone: true, CollectedAt: t0.Add(time.Hour)}}}
	if b.CollectedAt.After(t0) {
		t.Fatal("the bundle time must be zero so that only ageAt can refuse")
	}
	if got := b.Query(Query{"s", "issue.open", "7", time.Minute}, t0); got.State != CouldNotCheck {
		t.Errorf("a Go-built bundle with a source collected after now: %s (%s), want could-not-check", got.State, got.Reason)
	}
}

// TestLoadRefusesOversizedInput: Load reads at most MaxBundleBytes and refuses anything larger,
// so an unbounded input cannot exhaust memory.
func TestLoadRefusesOversizedInput(t *testing.T) {
	padded := strings.Repeat(" ", 16<<20) + sample
	_, err := Load(strings.NewReader(padded))
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("an oversized bundle: got %v, want a size refusal", err)
	}
}
