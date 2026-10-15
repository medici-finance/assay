package contexthandoff

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

func fixtureTree(t *testing.T, path string) any {
	t.Helper()
	return parseTree(t, readFile(t, path))
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// set returns a copy of tree with the value at the dotted path replaced.
// Numeric segments index arrays.
func set(tree any, path string, val any) any { return edit(tree, splitPath(path), val, false) }

// del returns a copy of tree with the object key at the dotted path removed.
func del(tree any, path string) any { return edit(tree, splitPath(path), nil, true) }

func splitPath(path string) []any {
	var out []any
	for _, s := range strings.Split(path, ".") {
		n := 0
		isNum := s != ""
		for _, c := range s {
			if c < '0' || c > '9' {
				isNum = false
				break
			}
			n = n*10 + int(c-'0')
		}
		if isNum {
			out = append(out, n)
		} else {
			out = append(out, s)
		}
	}
	return out
}

func wantRule(t *testing.T, label, rule string, err error) {
	t.Helper()
	if got := RuleOf(err); got != rule {
		t.Errorf("%s: rule = %q (%v), want %q", label, got, err, rule)
	}
}

// regression: statusgen/19 Verify row 1
func TestContextHandoffAmendWithoutPatchFails(t *testing.T) {
	h := pairedHandoff(t)
	base := fixtureTree(t, pairedManifestPath)
	m, err := DecodeManifest(marshal(t, base))
	if err != nil {
		t.Fatalf("valid manifest refused: %v", err)
	}
	if err := CheckManifest(h, m); err != nil {
		t.Fatalf("valid manifest does not bind its handoff: %v", err)
	}
	if m.Entries[0].Action != "amend" || m.Entries[0].Binding.Patch == nil {
		t.Fatalf("fixture entry 0 is not an amend obligation with a bound patch: %+v", m.Entries[0])
	}

	// Decoded records: every shape of "no bound patch and no follow-up".
	unbound := map[string]any{
		"empty binding":               set(base, "entries.0.binding", map[string]any{}),
		"null binding":                set(base, "entries.0.binding", nil),
		"absent binding":              del(base, "entries.0.binding"),
		"null patch":                  set(base, "entries.0.binding", map[string]any{"patch": nil}),
		"empty patch":                 set(base, "entries.0.binding", map[string]any{"patch": map[string]any{}}),
		"empty patch values":          set(base, "entries.0.binding", map[string]any{"patch": map[string]any{"base_revision": "", "digest": ""}}),
		"patch without base":          del(base, "entries.0.binding.patch.base_revision"),
		"patch without digest":        del(base, "entries.0.binding.patch.digest"),
		"patch with a non-digest":     set(base, "entries.0.binding.patch.digest", "see the PR"),
		"empty follow-up":             set(base, "entries.0.binding", map[string]any{"follow_up": ""}),
		"null follow-up":              set(base, "entries.0.binding", map[string]any{"follow_up": nil}),
		"string binding":              set(base, "entries.0.binding", "patch pending"),
		"patch and follow-up at once": set(base, "entries.0.binding.follow_up", "widgets:11"),
	}
	names := make([]string, 0, len(unbound))
	for n := range unbound {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		_, err := DecodeManifest(marshal(t, unbound[n]))
		wantRule(t, "decode "+n, RuleAmendUnbound, err)
	}

	// Records built in code (the resolver's path) meet the same rule.
	for _, b := range []Binding{{}, {Patch: &Patch{}}, {Patch: &Patch{BaseRevision: "3f9c2a1"}}, {FollowUp: new(string)}} {
		built := *m
		built.Entries = append([]Entry(nil), m.Entries...)
		built.Entries[0].Binding = b
		wantRule(t, "built binding", RuleAmendUnbound, CheckManifest(h, &built))
	}

	// An explicit follow-up brief is a binding.
	follow := set(base, "entries.0.binding", map[string]any{"follow_up": "widgets:11"})
	fm, err := DecodeManifest(marshal(t, follow))
	if err != nil {
		t.Fatalf("follow-up binding refused: %v", err)
	}
	if got := *fm.Entries[0].Binding.FollowUp; got != "example:demo:widgets:11" {
		t.Errorf("follow-up = %q, want the expanded example:demo:widgets:11", got)
	}
}

// regression: statusgen/19 Verify row 2
func TestContextHandoffCoverageGapRequired(t *testing.T) {
	base := fixtureTree(t, pairedHandoffPath)
	if _, err := DecodeHandoff(marshal(t, base)); err != nil {
		t.Fatalf("valid handoff refused: %v", err)
	}
	// A receipt over a universe with nothing included, so the only thing
	// wrong is the missing coverage statement itself.
	bare := set(set(base, "discovery.sources", []any{}), "discovery.coverage",
		map[string]any{"state": "complete", "enumerated": []any{}, "unavailable": []any{}})
	if _, err := DecodeHandoff(marshal(t, bare)); err != nil {
		t.Fatalf("a receipt with stated empty coverage is refused: %v", err)
	}
	cases := map[string]any{
		"no coverage":                         del(bare, "discovery.coverage"),
		"null coverage":                       set(bare, "discovery.coverage", nil),
		"no coverage, full universe":          del(base, "discovery.coverage"),
		"coverage without unavailable":        del(base, "discovery.coverage.unavailable"),
		"coverage without enumerated":         del(base, "discovery.coverage.enumerated"),
		"unavailable source silently omitted": set(base, "discovery.coverage.unavailable", []any{}),
		"unavailable source omitted, claimed complete": set(set(base, "discovery.coverage.unavailable", []any{}),
			"discovery.coverage.state", "complete"),
		"complete with an unavailable source": set(base, "discovery.coverage.state", "complete"),
		"enumerated source with no revision": set(set(base, "discovery.coverage.unavailable", []any{}),
			"discovery.coverage.enumerated", []any{"demo", "demo-docs"}),
		"source both enumerated and unavailable":       set(base, "discovery.coverage.enumerated", []any{"demo", "demo-docs"}),
		"coverage names a source outside the universe": set(base, "discovery.coverage.enumerated", []any{"demo", "elsewhere"}),
	}
	names := make([]string, 0, len(cases))
	for n := range cases {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		_, err := DecodeHandoff(marshal(t, cases[n]))
		wantRule(t, n, RuleCoverage, err)
	}
	h := pairedHandoff(t)
	if h.Discovery.Coverage.State != "partial" || len(h.Discovery.Coverage.Unavailable) != 1 {
		t.Errorf("decoded coverage = %+v, want the stated partial coverage with its one gap", h.Discovery.Coverage)
	}
}

// regression: statusgen/19 Verify row 3
func TestContextHandoffDraftIsNotApproved(t *testing.T) {
	approved := pairedHandoff(t)
	if err := approved.RequireApproved(); err != nil {
		t.Fatalf("approved fixture fails RequireApproved: %v", err)
	}
	draft, err := DecodeHandoff(readFile(t, draftHandoffPath))
	if err != nil {
		t.Fatalf("draft fixture refused: %v", err)
	}
	wantRule(t, "draft RequireApproved", RuleNotApproved, draft.RequireApproved())
	proposed := *draft
	proposed.Target.State = TargetProposed
	wantRule(t, "proposed RequireApproved", RuleNotApproved, proposed.RequireApproved())
	// A record built in code that claims approval without a recorded revision
	// does not satisfy the check either.
	claimed := *approved
	claimed.Target.Approval = nil
	wantRule(t, "approved state, no approval", RuleNotApproved, claimed.RequireApproved())
	claimed.Target.Approval = &Approval{Reference: "https://example.invalid/reviews/1"}
	wantRule(t, "approval with empty revision", RuleNotApproved, claimed.RequireApproved())

	base := fixtureTree(t, pairedHandoffPath)
	dtree := fixtureTree(t, draftHandoffPath)
	refusals := []struct {
		name, rule string
		tree       any
	}{
		{"approval without a revision", RuleApproval, del(base, "target.approval.revision")},
		{"approval with an empty revision", RuleApproval, set(base, "target.approval.revision", "")},
		{"approved without an approval", RuleApproval, set(base, "target.approval", nil)},
		{"approval of another revision", RuleApproval, set(base, "target.approval.revision", "9e8d7c6")},
		{"draft carrying an approval", RuleApproval, set(dtree, "target.approval",
			map[string]any{"revision": "0b1c2d3e4f", "reference": "https://example.invalid/reviews/2"})},
		{"draft dispatching", RuleNotApproved, set(set(set(dtree, "impacts.0.disposition", "defer"),
			"readiness.unresolved", []any{}), "readiness.next_act", "dispatch")},
		{"proposed publishing", RuleNotApproved, set(set(set(set(dtree, "target.state", "proposed"),
			"impacts.0.disposition", "defer"), "readiness.unresolved", []any{}), "readiness.next_act", "publish")},
		{"draft claiming proposal_approved", RuleReadiness, set(dtree, "readiness.facts.proposal_approved", true)},
	}
	for _, c := range refusals {
		_, err := DecodeHandoff(marshal(t, c.tree))
		wantRule(t, c.name, c.rule, err)
	}
}

// reorder writes v as JSON with every object's keys in REVERSE byte order and
// indented, so it differs from the input bytes in both order and whitespace.
func reorder(v any) []byte {
	var b bytes.Buffer
	var w func(v any, indent string)
	w = func(v any, indent string) {
		switch t := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Sort(sort.Reverse(sort.StringSlice(keys)))
			b.WriteString("{\n")
			for i, k := range keys {
				if i > 0 {
					b.WriteString(",\n")
				}
				kb, _ := json.Marshal(k)
				b.WriteString(indent + "\t" + string(kb) + " :  ")
				w(t[k], indent+"\t")
			}
			b.WriteString("\n" + indent + "}")
		case []any:
			b.WriteString("[ ")
			for i, e := range t {
				if i > 0 {
					b.WriteString(" , ")
				}
				w(e, indent)
			}
			b.WriteString(" ]")
		default:
			eb, _ := json.Marshal(t)
			b.Write(eb)
		}
	}
	w(v, "")
	return b.Bytes()
}

// regression: statusgen/19 Verify row 4
func TestContextHandoffCanonicalDigestStable(t *testing.T) {
	raw := readFile(t, pairedHandoffPath)
	h, err := DecodeHandoff(raw)
	if err != nil {
		t.Fatal(err)
	}
	reordered := reorder(parseTree(t, raw))
	if bytes.Equal(reordered, raw) {
		t.Fatal("reordered input is byte-identical to the fixture; the test proves nothing")
	}
	h2, err := DecodeHandoff(reordered)
	if err != nil {
		t.Fatalf("reordered record refused: %v", err)
	}
	if h.Digest() != h2.Digest() {
		t.Errorf("digest changed with field order and whitespace: %s vs %s", h.Digest(), h2.Digest())
	}
	// An escaped spelling of the same string is the same value: R written as
	// its \u escape.
	const backslash = "\x5c"
	escaped := bytes.Replace(raw, []byte(`"Replace the widget`), []byte(`"`+backslash+`u0052eplace the widget`), 1)
	if bytes.Equal(escaped, raw) {
		t.Fatal("escape substitution did not apply")
	}
	h3, err := DecodeHandoff(escaped)
	if err != nil {
		t.Fatalf("escaped record refused: %v", err)
	}
	if h.Digest() != h3.Digest() {
		t.Errorf("digest changed with string escaping: %s vs %s", h.Digest(), h3.Digest())
	}
	// Canonical bytes are a fixed point and carry sorted keys, no whitespace.
	c := h.Canonical()
	h4, err := DecodeHandoff(c)
	if err != nil {
		t.Fatalf("canonical form refused: %v", err)
	}
	if !bytes.Equal(h4.Canonical(), c) {
		t.Error("canonicalizing the canonical form changed it")
	}
	if !bytes.HasPrefix(c, []byte(`{"active_work":[{"acknowledged":true,`)) {
		t.Errorf("canonical form does not start with sorted keys and no whitespace: %.60s", c)
	}
	if !strings.HasPrefix(h.Digest(), "sha256:") || len(h.Digest()) != len("sha256:")+64 {
		t.Errorf("digest %q is not sha256:<64 hex>", h.Digest())
	}
	// A changed value is a different record.
	changed := set(parseTree(t, raw), "target.intent", "Replace the widget cache key with a content digest.")
	h5, err := DecodeHandoff(marshal(t, changed))
	if err != nil {
		t.Fatal(err)
	}
	if h5.Digest() == h.Digest() {
		t.Error("a changed intent kept the same digest")
	}
	flipped := set(parseTree(t, raw), "active_work.0.treatment", "reconcile")
	h6, err := DecodeHandoff(marshal(t, flipped))
	if err != nil {
		t.Fatal(err)
	}
	if h6.Digest() == h.Digest() {
		t.Error("a changed treatment kept the same digest")
	}
	// The manifest digest follows the same rules.
	mraw := readFile(t, pairedManifestPath)
	m1, err := DecodeManifest(mraw)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := DecodeManifest(reorder(parseTree(t, mraw)))
	if err != nil {
		t.Fatal(err)
	}
	if m1.Digest() != m2.Digest() {
		t.Errorf("manifest digest changed with field order: %s vs %s", m1.Digest(), m2.Digest())
	}
}

// regression: statusgen/19 Verify row 5
func TestContextHandoffQualifiedIDsResolve(t *testing.T) {
	declaring := &QID{Cell: "example", Repo: "demo", Stream: "widgets", NN: "07"}
	accepted := map[string]string{
		"example:demo:gadgets:04": "example:demo:gadgets:04",
		"demo:gadgets:04":         "example:demo:gadgets:04",
		"gadgets:04":              "example:demo:gadgets:04",
		"other:gadgets:04":        "example:other:gadgets:04",
		"cell2:other:gadgets:12a": "cell2:other:gadgets:12a",
	}
	for ref, want := range accepted {
		q, err := ResolveQualifiedID(ref, declaring)
		if err != nil {
			t.Errorf("%q refused: %v", ref, err)
			continue
		}
		if q.String() != want {
			t.Errorf("%q resolved to %q, want %q", ref, q.String(), want)
		}
	}
	refused := []string{
		"#4", "demo#4", "gadgets/04", "docs/streams/gadgets/brief-04-cache-reader.md",
		"brief-04-cache-reader.md", "gadgets", "04", "", " gadgets:04", "Gadgets:04",
		"a:b:c:gadgets:04", "gadgets:4", "gadgets:four", "demo:gadgets/04", ":gadgets:04",
	}
	for _, ref := range refused {
		if q, err := ResolveQualifiedID(ref, declaring); err == nil {
			t.Errorf("%q accepted as %s; it is not a qualified brief ID", ref, q)
		}
	}
	if _, err := ResolveQualifiedID("gadgets:04", nil); err == nil {
		t.Error("an elided reference with no declaring record was accepted")
	}

	// Decoded records: every elision of every reference canonicalizes to the
	// same four-segment record and digest as the all-full form.
	base := fixtureTree(t, pairedHandoffPath)
	full := base
	for path, id := range map[string]string{
		"discovery.dependency_slice.0": "example:demo:widgets:03",
		"discovery.dependency_slice.1": "example:demo:gadgets:04",
		"impacts.0.subject":            "example:demo:widgets:03",
		"impacts.1.subject":            "example:demo:gadgets:04",
		"amendments.0.subject":         "example:demo:widgets:03",
		"amendments.1.subject":         "example:demo:gadgets:04",
		"amendments.1.replaced_by.0":   "example:demo:gadgets:08",
		"amendments.1.replaced_by.1":   "example:demo:gadgets:09",
		"history.0.subject":            "example:demo:widgets:03",
		"active_work.0.subject":        "example:demo:gadgets:04",
	} {
		full = set(full, path, id)
	}
	hFull, err := DecodeHandoff(marshal(t, full))
	if err != nil {
		t.Fatalf("all-full record refused: %v", err)
	}
	hElided := pairedHandoff(t)
	if hFull.Digest() != hElided.Digest() {
		t.Errorf("elided and full references give different digests: %s vs %s", hElided.Digest(), hFull.Digest())
	}
	shortest := set(set(full, "impacts.1.subject", "gadgets:04"), "amendments.1.subject", "demo:gadgets:04")
	if h, err := DecodeHandoff(marshal(t, shortest)); err != nil || h.Digest() != hFull.Digest() {
		t.Errorf("mixed elisions: err=%v, digest differs from the full form", err)
	}
	canon := string(hElided.Canonical())
	for _, s := range []string{`"widgets:03"`, `"gadgets:04"`, `"demo:gadgets:04"`, `"gadgets:08"`} {
		if strings.Contains(canon, s) {
			t.Errorf("canonical record keeps the elided reference %s", s)
		}
	}
	if hElided.Impacts[0].Subject != "example:demo:widgets:03" {
		t.Errorf("decoded subject = %q, want the expanded four-segment form", hElided.Impacts[0].Subject)
	}
	for _, c := range []struct{ path, ref string }{
		{"impacts.0.subject", "#3"},
		{"impacts.0.subject", "docs/streams/widgets/brief-03-cache-writer.md"},
		{"impacts.0.subject", "widgets/03"},
		{"impacts.0.subject", "widgets"},
		{"readiness.unresolved", "brief-03"},
		{"declared_by", "widgets:07"},
		{"declared_by", "demo:widgets:07"},
	} {
		tree := set(base, c.path, c.ref)
		if c.path == "readiness.unresolved" {
			tree = set(base, c.path, []any{c.ref})
		}
		_, err := DecodeHandoff(marshal(t, tree))
		wantRule(t, c.path+"="+c.ref, RuleQualifiedID, err)
	}
	// A different repo segment is a different brief, not an elision of this one.
	other := set(base, "history.0.subject", "other:widgets:03")
	h, err := DecodeHandoff(marshal(t, other))
	if err != nil {
		t.Fatal(err)
	}
	if h.History[0].Subject != "example:other:widgets:03" || h.Digest() == hElided.Digest() {
		t.Errorf("repo-elided reference resolved to %q", h.History[0].Subject)
	}
}

func TestManifestBindsObligations(t *testing.T) {
	h := pairedHandoff(t)
	base := fixtureTree(t, pairedManifestPath)
	cases := map[string]any{
		"other handoff digest":         set(base, "handoff", "sha256:"+strings.Repeat("c", 64)),
		"obligation without entry":     set(base, "entries", []any{base.(map[string]any)["entries"].([]any)[0]}),
		"action disagrees":             set(base, "entries.1.action", "replace"),
		"brief disagrees":              set(base, "entries.1.brief", "gadgets:05"),
		"other declaring brief":        set(base, "declared_by", "example:demo:widgets:12"),
		"entry for unknown obligation": set(base, "entries.1.obligation", "ob-9"),
	}
	for n, tree := range cases {
		m, err := DecodeManifest(marshal(t, tree))
		if err != nil {
			t.Errorf("%s: decode refused: %v", n, err)
			continue
		}
		wantRule(t, n, RuleManifestMismatch, CheckManifest(h, m))
	}
}
