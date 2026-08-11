package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Entry grammar
// ---------------------------------------------------------------------------

func TestParseConsumerEntry(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		site    string
		routing string
		targets []string
	}{
		{"bare fixed-here", "statusgen/consumers.go: fixed-here", "statusgen/consumers.go", routingFixedHere, nil},
		{
			// The detail is the most useful part of a real entry; a grammar
			// that rejected it would be conformed to by deleting it.
			"fixed-here with detail", "docs/brief-rules.md: fixed-here (rule 9 gains the field)",
			"docs/brief-rules.md", routingFixedHere, nil,
		},
		{"follow-up with target", "web/: follow-up methodology-metrics/41", "web/", routingFollowUp, []string{"methodology-metrics/41"}},
		{
			// One consumer legitimately routes to two briefs; dropping the
			// second would let an unbacked promise through.
			"follow-up with two targets",
			"docs/reports/daily/opmetrics.json: follow-up methodology-metrics/41 (reads it), follow-up methodology-metrics/42 (reads it)",
			"docs/reports/daily/opmetrics.json", routingFollowUp,
			[]string{"methodology-metrics/41", "methodology-metrics/42"},
		},
		{"follow-up with no target", "some component: follow-up (later)", "some component", routingFollowUp, nil},
		{"out-of-scope", "tools/x: out-of-scope (frozen tree)", "tools/x", routingOutOfScope, nil},
		{
			// A naive LastIndex(": ") split parses this as site
			// "tools/x: out-of-scope (blocked" and reports it unroutable.
			"out-of-scope reason containing a colon",
			"tools/x: out-of-scope (blocked: #123 must land first)",
			"tools/x", routingOutOfScope, nil,
		},
		{"site containing a colon", "docs/a.md: 'Notes: part two': fixed-here", "docs/a.md: 'Notes: part two'", routingFixedHere, nil},
		{"unroutable", "[example-app] #556 filing identity: cross-ref (nowhere durable)", "", routingUnknown, nil},
		{"token boundary", "docs/a.md: fixed-hereabouts", "", routingUnknown, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := parseConsumerEntry(c.raw)
			if e.Routing != c.routing {
				t.Fatalf("routing = %q, want %q", e.Routing, c.routing)
			}
			if c.routing != routingUnknown && e.Site != c.site {
				t.Errorf("site = %q, want %q", e.Site, c.site)
			}
			if len(e.Targets) != len(c.targets) {
				t.Fatalf("targets = %v, want %v", e.Targets, c.targets)
			}
			for i := range c.targets {
				if e.Targets[i] != c.targets[i] {
					t.Errorf("target[%d] = %q, want %q", i, e.Targets[i], c.targets[i])
				}
			}
		})
	}
}

func TestExpandBraces(t *testing.T) {
	got := expandBraces(".claude/skills/{the-desk,verify-desk}/SKILL.md")
	want := []string{".claude/skills/the-desk/SKILL.md", ".claude/skills/verify-desk/SKILL.md"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if one := expandBraces("statusgen/consumers.go"); len(one) != 1 || one[0] != "statusgen/consumers.go" {
		t.Errorf("plain path: got %v", one)
	}
	if unb := expandBraces("a/{b"); len(unb) != 1 || unb[0] != "a/{b" {
		t.Errorf("unbalanced brace should be literal: got %v", unb)
	}
}

func TestClassifySite(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "docs", "brief-rules.md"), "x")

	cases := []struct {
		name string
		site string
		kind siteKind
		// resolved: expect at least one match (only meaningful for siteRepoPath)
		resolved bool
	}{
		{"resolved path", "docs/brief-rules.md", siteRepoPath, true},
		{"absent path", "docs/nope.md", siteRepoPath, false},
		{"path with trailing descriptor", "docs/brief-rules.md 'rule 9' block", siteRepoPath, true},
		{"repo-tagged", "[example-app] .claude/skills/the-desk/SKILL.md", siteForeign, false},
		{"home-dir path", "~/.claude/skills/author-brief/SKILL.md", siteForeign, false},
		{"prose component", "every brief-v1 brief (frontmatter field)", siteProse, false},
		{"escape attempt", "../outside/thing.md", siteProse, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifySite(root, c.site)
			if got.Kind != c.kind {
				t.Fatalf("kind = %v, want %v (reason %q)", got.Kind, c.kind, got.Reason)
			}
			if c.kind == siteRepoPath && (len(got.Matches) > 0) != c.resolved {
				t.Errorf("matches = %v, want resolved=%v", got.Matches, c.resolved)
			}
			if c.kind != siteRepoPath && got.Reason == "" {
				t.Error("an unreachable site must say why — a silent skip reads as a pass")
			}
		})
	}
}

// TestSharedValueTriggerIsNarrow pins the deliberate narrowness of the prompt:
// single common words fire everywhere on a prose corpus, and an advisory that
// fires everywhere gets trained away instead of acted on.
func TestSharedValueTriggerIsNarrow(t *testing.T) {
	if p := sharedValueTrigger("the agent reads a secret and a party identity"); p != "" {
		t.Errorf("common words must not trigger; got %q", p)
	}
	if p := sharedValueTrigger("Shared value: the new frontmatter field is read by two skills"); p == "" {
		t.Error("an explicit shared-value declaration must trigger")
	}
}

// ---------------------------------------------------------------------------
// Offline lint half
// ---------------------------------------------------------------------------

// consumersFixture writes a two-brief stream whose brief-01 carries the given
// consumers list, and returns the root.
func consumersFixture(t *testing.T, consumers []string, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "alpha")
	readme := `---
stream: alpha
status: active
priority: P1
track: platform
---

# Alpha

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Consumer claims](./brief-01-claims.md) | 0 | M | in-progress | — | — |
| 02 | [Follow-up target](./brief-02-target.md) | 0 | S | todo | — | — |
`
	mustWrite(t, filepath.Join(dir, "README.md"), readme)

	var list strings.Builder
	for _, c := range consumers {
		list.WriteString("  - " + quoteYAML(c) + "\n")
	}
	brief := `---
brief: alpha/01
title: A brief carrying routed consumer claims
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-01 by fixture
sources: ["fixture"]
consumers:
` + list.String() + `---

# Brief 01

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | ` + "`statusgen --consumers --brief alpha/01`" + ` | 0 |
`
	mustWrite(t, filepath.Join(dir, "brief-01-claims.md"), brief)
	mustWrite(t, filepath.Join(dir, "brief-02-target.md"), `---
brief: alpha/02
title: The follow-up target brief
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-01 by fixture
sources: ["fixture"]
---

# Brief 02

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `+"`true`"+` | 0 |
`)
	for rel, content := range extra {
		mustWrite(t, filepath.Join(root, rel), content)
	}
	return root
}

func quoteYAML(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lintConsumers(t *testing.T, root string) (problems, notices []string) {
	t.Helper()
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return consumersCheck(root, streams)
}

// TestFollowUpToNonexistentBriefIsProblem is the gate the OFFLINE half can
// fire: a routing claim the stream tables flatly disprove.
func TestFollowUpToNonexistentBriefIsProblem(t *testing.T) {
	root := consumersFixture(t, []string{"web/thing.go: follow-up alpha/99"}, nil)
	problems, _ := lintConsumers(t, root)
	if !hasProblem(problems, "follow-up alpha/99", "not a brief in any stream README") {
		t.Fatalf("a follow-up to a nonexistent brief must be a PROBLEM; got %v", problems)
	}
}

// TestFollowUpBackReferenceCorrectsIt is the other side of the same gate: the
// same entry stops being reported once the target brief actually references
// back. Without this pairing the check is unproven — a lint that has never been
// shown to go green on the corrected input is not a lint, it is a constant.
func TestFollowUpBackReferenceCorrectsIt(t *testing.T) {
	root := consumersFixture(t, []string{"web/thing.go: follow-up alpha/02"}, nil)
	problems, notices := lintConsumers(t, root)
	if len(problems) != 0 {
		t.Fatalf("an existing follow-up target must not be a PROBLEM; got %v", problems)
	}
	if !hasProblem(notices, "never references alpha/01") {
		t.Fatalf("a one-way follow-up must be NOTICEd; got %v", notices)
	}

	// Now make alpha/02 reference back, and the notice must clear.
	path := filepath.Join(root, "docs", "streams", "alpha", "brief-02-target.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, string(raw)+"\nCovers the consumer deferred by alpha/01.\n")
	_, notices = lintConsumers(t, root)
	if hasProblem(notices, "never references alpha/01") {
		t.Fatalf("back-reference must clear the notice; got %v", notices)
	}
}

func TestUnroutableAndThinReasonsAreNoticed(t *testing.T) {
	root := consumersFixture(t, []string{
		"[example-app] #556 filing identity: cross-ref (nowhere durable)",
		"tools/x: out-of-scope (n/a)",
		"some component: follow-up (later)",
	}, nil)
	problems, notices := lintConsumers(t, root)
	if len(problems) != 0 {
		t.Fatalf("underspecified entries are NOTICEs, not PROBLEMs; got %v", problems)
	}
	for _, want := range []string{"names no routing", "no substantive reason", "names no <stream>/<NN> target"} {
		if !hasProblem(notices, want) {
			t.Errorf("missing notice %q in %v", want, notices)
		}
	}
}

func TestProseConsumersIsNoticedNotFatal(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "alpha")
	mustWrite(t, filepath.Join(dir, "README.md"), `---
stream: alpha
status: active
priority: P1
track: platform
---

# Alpha

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Prose consumers](./brief-01-prose.md) | 0 | S | todo | — | — |
`)
	mustWrite(t, filepath.Join(dir, "brief-01-prose.md"), `---
brief: alpha/01
title: Prose consumers
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-01 by fixture
sources: ["fixture"]
consumers: this brief introduces a bounded exception, so every reader of the rule is a consumer
---

# Brief 01
`)
	// A scalar consumers: must not be a hard parse error — several briefs on
	// main use the prose form and a hard error would red-gate the whole board.
	if _, ok, err := parseBriefFile(filepath.Join(dir, "brief-01-prose.md")); err != nil || !ok {
		t.Fatalf("prose consumers must still parse: ok=%v err=%v", ok, err)
	}
	problems, notices := lintConsumers(t, root)
	if len(problems) != 0 {
		t.Fatalf("prose consumers is a NOTICE, not a PROBLEM; got %v", problems)
	}
	if !hasProblem(notices, "prose paragraph") {
		t.Fatalf("prose consumers must be noticed; got %v", notices)
	}
}

// ---------------------------------------------------------------------------
// Diff-aware corroboration — the gate
// ---------------------------------------------------------------------------

// withDiff substitutes the branch diff for the duration of a test.
func withDiff(t *testing.T, paths ...string) {
	t.Helper()
	orig := changedPathsSince
	changedPathsSince = func(root, base string) ([]string, error) { return paths, nil }
	t.Cleanup(func() { changedPathsSince = orig })
}

// TestConsumersGateFiresOnFalseFixedHere is the demonstration the whole design
// turns on: a `fixed-here` claim whose path is nowhere in the branch diff is
// DISPROVED and the command exits 1. A gate never shown to fire is not a gate.
func TestConsumersGateFiresOnFalseFixedHere(t *testing.T) {
	root := consumersFixture(t, []string{"web/untouched.go: fixed-here"}, map[string]string{
		"web/untouched.go": "package web",
	})
	// The branch touched the brief, but not the file it claims to have fixed.
	withDiff(t, "docs/streams/alpha/brief-01-claims.md")
	if code := runConsumers(root, "origin/main", ""); code != 1 {
		t.Fatalf("a false fixed-here claim must exit 1; got %d", code)
	}

	// Correct the claim by actually touching the consumer: exit 0.
	withDiff(t, "docs/streams/alpha/brief-01-claims.md", "web/untouched.go")
	if code := runConsumers(root, "origin/main", ""); code != 0 {
		t.Fatalf("a corroborated fixed-here claim must exit 0; got %d", code)
	}
}

// TestConsumersGateFiresOnFabricatedPath covers the cheapest false entry there
// is: a path that does not exist at all.
func TestConsumersGateFiresOnFabricatedPath(t *testing.T) {
	root := consumersFixture(t, []string{"web/does-not-exist.go: fixed-here (imaginary)"}, nil)
	withDiff(t, "docs/streams/alpha/brief-01-claims.md")
	if code := runConsumers(root, "origin/main", ""); code != 1 {
		t.Fatalf("a fabricated path must exit 1; got %d", code)
	}
}

// TestConsumersDeletedPathIsCorroborated: a path the branch DELETES is absent
// from the tree but present in the diff. Deleting the consumer IS fixing it, so
// the offline lint could never settle this one — the diff can.
func TestConsumersDeletedPathIsCorroborated(t *testing.T) {
	root := consumersFixture(t, []string{"web/removed.go: fixed-here (deleted here)"}, nil)
	withDiff(t, "docs/streams/alpha/brief-01-claims.md", "web/removed.go")
	if code := runConsumers(root, "origin/main", ""); code != 0 {
		t.Fatalf("a deleted consumer must corroborate; got %d", code)
	}
}

// TestConsumersUncheckedNeverPassesSilently: out-of-repo and out-of-scope
// entries cannot be settled here. They must exit 0 (nothing was disproved) AND
// be reported as UNCHECKED — an instrument that cannot see the evidence says so.
func TestConsumersUncheckedNeverPassesSilently(t *testing.T) {
	root := consumersFixture(t, []string{
		"~/.claude/skills/author-brief/SKILL.md: fixed-here",
		"tools/frozen/: out-of-scope (tree is frozen; consumers read the pinned release binary)",
	}, nil)
	withDiff(t, "docs/streams/alpha/brief-01-claims.md")

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	bf, ok, err := parseBriefFile(filepath.Join(root, "docs", "streams", "alpha", "brief-01-claims.md"))
	if err != nil || !ok {
		t.Fatalf("fixture must parse: %v", err)
	}
	verdicts := corroborateBrief(root, streams, map[string]bool{}, nil, bf)
	if len(verdicts) != 2 {
		t.Fatalf("want 2 verdicts, got %d: %+v", len(verdicts), verdicts)
	}
	for _, v := range verdicts {
		if v.State != stateUnchecked {
			t.Errorf("%q: state = %s, want %s", v.Entry, v.State, stateUnchecked)
		}
		if v.Reason == "" {
			t.Errorf("%q: UNCHECKED with no reason reads as a pass", v.Entry)
		}
	}
	if code := runConsumers(root, "origin/main", ""); code != 0 {
		t.Fatalf("unchecked entries disprove nothing, so exit 0; got %d", code)
	}
}

// TestConsumersCouldNotCheckIsNotZero: when the diff itself cannot be taken the
// command must NOT report success. Two states, not three, is the defect the
// three-state instrument rule exists to prevent.
func TestConsumersCouldNotCheckIsNotZero(t *testing.T) {
	root := consumersFixture(t, []string{"web/x.go: fixed-here"}, nil)
	orig := changedPathsSince
	changedPathsSince = func(root, base string) ([]string, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() { changedPathsSince = orig })
	if code := runConsumers(root, "origin/main", ""); code != 2 {
		t.Fatalf("could-not-check must exit 2, never 0; got %d", code)
	}
}

// TestConsumersBriefFilterWithoutDiffIsCouldNotCheck is the post-merge verifier
// path (review of #371, finding 2). `--brief` lifts the diff SCOPING, not the
// need for a diff: re-run on merged main, where HEAD == base and the diff is
// empty, the branch that made the claims true is gone. Reporting exit 1 there
// hands the verifier eight false DISPROVEDs on a brief that is fine; reporting 0
// would be worse still — a pass nothing established. COULD-NOT-CHECK is the only
// honest third answer, and the exit code has to differ from both.
func TestConsumersBriefFilterWithoutDiffIsCouldNotCheck(t *testing.T) {
	root := consumersFixture(t, []string{"web/untouched.go: fixed-here"}, map[string]string{
		"web/untouched.go": "package web",
	})
	withDiff(t) // empty diff: the post-merge re-run
	if code := runConsumers(root, "origin/main", "alpha/01"); code != 2 {
		t.Fatalf("--brief with no diff for the brief must be could-not-check (2), never a false 1 or a false 0; got %d", code)
	}
	if code := runConsumers(root, "origin/main", "alpha/99"); code != 2 {
		t.Fatalf("an unknown --brief is could-not-check (2), not clean (0); got %d", code)
	}
	// With the brief in the diff, --brief still judges it: the escape hatch is
	// scoped to "no evidence", not to "this brief".
	withDiff(t, "docs/streams/alpha/brief-01-claims.md")
	if code := runConsumers(root, "origin/main", "alpha/01"); code != 1 {
		t.Fatalf("--brief must still disprove a false claim when the diff carries the brief; got %d", code)
	}
}

// withBaseEntries substitutes the merge-base entry set for the duration of a
// test, standing in for `git show <merge-base>:<brief>`.
func withBaseEntries(t *testing.T, entries ...string) {
	t.Helper()
	orig := consumerEntriesAtBase
	set := map[string]bool{}
	for _, e := range entries {
		set[e] = true
	}
	consumerEntriesAtBase = func(root, base, rel string) map[string]bool { return set }
	t.Cleanup(func() { consumerEntriesAtBase = orig })
}

// TestInheritedClaimsAreNotJudgedByThisDiff is the false-positive class that
// would have got the gate switched off (review of #371, finding 1). A
// verify-desk Evidence commit, a status flip or a typo fix edits a merged
// brief's FILE while changing none of its CLAIMS — and the file being in the
// diff was enough to put every year-old `fixed-here` on trial against a diff
// that was never supposed to contain it. Measured on main at review time: 6 of
// 18 briefs carrying consumers: exited 1 the moment anyone touched them.
func TestInheritedClaimsAreNotJudgedByThisDiff(t *testing.T) {
	root := consumersFixture(t, []string{"web/untouched.go: fixed-here"}, map[string]string{
		"web/untouched.go": "package web",
	})
	// The Evidence commit: the brief file is in the diff, the consumer is not.
	withDiff(t, "docs/streams/alpha/brief-01-claims.md")
	withBaseEntries(t, "web/untouched.go: fixed-here")

	out := captureStdout(t, func() {
		if code := runConsumers(root, "origin/main", ""); code != 0 {
			t.Errorf("an inherited claim must not be disproved by a diff that never made it; got %d", code)
		}
	})
	if !strings.Contains(out, stateUnchecked) || !strings.Contains(out, "did not make this claim") {
		t.Errorf("the inherited entry must be reported UNCHECKED with the reason named, not silently skipped:\n%s", out)
	}
	if strings.Contains(out, stateDisproved) {
		t.Errorf("no entry should be DISPROVED here:\n%s", out)
	}

	// The other half: the same branch EDITS the entry, and it is judged again.
	// Provenance narrows what the gate looks at; it must not disarm it.
	withBaseEntries(t, "web/untouched.go: fixed-here (an older wording)")
	if code := runConsumers(root, "origin/main", ""); code != 1 {
		t.Fatalf("an entry this branch rewrote is this branch's claim and must be judged; got %d", code)
	}
}

// TestRoutingTokenIsNormalized: whether a claim is CHECKABLE must not be under
// the claimant's control (review of #371, finding 3). A doubled space or a
// capital letter is invisible in review, and both used to demote a false claim
// from DISPROVED to UNCHECKED at zero cost.
func TestRoutingTokenIsNormalized(t *testing.T) {
	for _, raw := range []string{
		"web/untouched.go: fixed-here",
		"web/untouched.go:  fixed-here",
		"web/untouched.go:\tfixed-here",
		"web/untouched.go: Fixed-here",
		"web/untouched.go:fixed-here",
		"web/untouched.go: FIXED-HERE (shouting is still a claim)",
	} {
		t.Run(raw, func(t *testing.T) {
			if e := parseConsumerEntry(raw); e.Routing != routingFixedHere {
				t.Fatalf("routing = %q, want %q", e.Routing, routingFixedHere)
			}
			root := consumersFixture(t, []string{raw}, map[string]string{"web/untouched.go": "package web"})
			withDiff(t, "docs/streams/alpha/brief-01-claims.md")
			if code := runConsumers(root, "origin/main", ""); code != 1 {
				t.Fatalf("a false claim must be DISPROVED however it was spaced or capitalized; got %d", code)
			}
		})
	}
}

// TestUnroutableEntryIsDisprovedByTheGate: "you wrote a routing nobody can read"
// needs no diff to settle, so the gate settles it. UNCHECKED here would make
// unreadability the cheapest way past the gate.
func TestUnroutableEntryIsDisprovedByTheGate(t *testing.T) {
	root := consumersFixture(t, []string{"web/thing.go: cross-ref (nowhere durable)"}, nil)
	withDiff(t, "docs/streams/alpha/brief-01-claims.md")
	if code := runConsumers(root, "origin/main", ""); code != 1 {
		t.Fatalf("an entry with no routing token must be DISPROVED by the gate; got %d", code)
	}
	// The offline lint half stays a NOTICE: it runs over briefs merged months
	// ago, and a corpus-wide PROBLEM would demand rewriting closed briefs.
	problems, notices := lintConsumers(t, root)
	if len(problems) != 0 {
		t.Fatalf("the offline half must not turn this into a corpus PROBLEM; got %v", problems)
	}
	if !hasProblem(notices, "names no routing") {
		t.Fatalf("the offline half must still notice it; got %v", notices)
	}
}

// TestWildcardSiteIsNotAFreeCorroboration (review of #371, finding 4). Passing
// on a single match made widening the site the cheapest way to green a claim:
// `*/*: fixed-here` matched the whole tree and corroborated against any diff at
// all. A site naming a set claims the set.
func TestWildcardSiteIsNotAFreeCorroboration(t *testing.T) {
	extra := map[string]string{
		"web/one.go":   "package web",
		"web/two.go":   "package web",
		"web/three.go": "package web",
	}
	root := consumersFixture(t, []string{"web/*.go: fixed-here (the whole package)"}, extra)

	// One of three touched: a partial fix stated as a whole-set claim.
	withDiff(t, "docs/streams/alpha/brief-01-claims.md", "web/one.go")
	if code := runConsumers(root, "origin/main", ""); code != 1 {
		t.Fatalf("a set claim with untouched members must be DISPROVED; got %d", code)
	}
	// All three: the claim is true and the gate says so.
	withDiff(t, "docs/streams/alpha/brief-01-claims.md", "web/one.go", "web/two.go", "web/three.go")
	if code := runConsumers(root, "origin/main", ""); code != 0 {
		t.Fatalf("a set claim whose whole set is touched must be CORROBORATED; got %d", code)
	}
}

// TestDirectorySiteMatchesItsContents (review of #371, finding 5). Diffs name
// files, never directories, so a trailing-slash site — the form brief-rules.md's
// own example uses — resolved to the directory entry, matched nothing and was
// DISPROVED on the very branch that rewrote it. The gate calling a TRUE claim
// false is worse than not checking it.
func TestDirectorySiteMatchesItsContents(t *testing.T) {
	root := consumersFixture(t, []string{"web/: fixed-here (whole package reworked)"}, map[string]string{
		"web/one.go": "package web",
	})
	withDiff(t, "docs/streams/alpha/brief-01-claims.md", "web/one.go")
	if code := runConsumers(root, "origin/main", ""); code != 0 {
		t.Fatalf("a directory site is touched when its contents are; got %d", code)
	}
	// And it still fires when nothing under it moved.
	withDiff(t, "docs/streams/alpha/brief-01-claims.md")
	if code := runConsumers(root, "origin/main", ""); code != 1 {
		t.Fatalf("an untouched directory site must still be DISPROVED; got %d", code)
	}
}

// TestDirectorySiteShowsItsEvidence (re-review of #371, finding C). A directory
// site is corroborated when ANY path under it is in the diff — finding 5's fix,
// and correct. But it used to print a bare CORROBORATED, so `web/: fixed-here`
// resting on one file and `web/one.go: fixed-here` naming that same file were
// byte-identical in the log. The wider claim is the cheaper one to write, and
// the output gave a reviewer nothing to weigh it with. The verdict now names
// the paths the corroboration rests on and how many there are.
//
// Both halves matter: the evidence line must appear for a directory site, and
// must NOT appear for a plain file site (an unconditional reason would be noise
// on every entry and would tell a reviewer nothing).
//
// Re-review of #371, finding 1. The original version of this test put exactly
// one of the fixture's two files in the diff, so "corroborated by 1 path(s)"
// was the only value that fixture could ever produce — at n=1 a directory site
// and a file site convey the same information, and the multi-path machinery in
// touchedBy/directoryEvidence (collect ALL hits, sort them, cap the shown list,
// state the exact count) was reachable by nothing here. Confirmed: inserting a
// `break` after the first match in touchedBy's collection loop, or dropping its
// `sort.Strings(out)` call, or shrinking maxDirectoryEvidencePaths to 1, all left
// the suite green. This version puts two files in the diff (kills the break and
// asserts the exact count and both names in order) and adds a case above the cap
// (kills the cap silently truncating without saying so).
func TestDirectorySiteShowsItsEvidence(t *testing.T) {
	root := consumersFixture(t, []string{"web/: fixed-here (whole package reworked)"}, map[string]string{
		"web/one.go": "package web",
		"web/two.go": "package web",
	})
	withDiff(t, "docs/streams/alpha/brief-01-claims.md", "web/one.go", "web/two.go")
	out := captureStdout(t, func() {
		if code := runConsumers(root, "origin/main", ""); code != 0 {
			t.Fatalf("a directory site touched by all its members is CORROBORATED; got %d", code)
		}
	})
	if !strings.Contains(out, "CORROBORATED") {
		t.Fatalf("expected CORROBORATED; got:\n%s", out)
	}
	// The count is the part that exposes widening, and it must be exact — a
	// single-file fixture cannot tell "collect every hit" from "stop at the
	// first one", so both files here are in the diff.
	if !strings.Contains(out, "corroborated by 2 path(s) under it") {
		t.Fatalf("a corroborated directory site must state the exact count it rests on; got:\n%s", out)
	}
	// Both names, in the tool's sorted order — dropping touchedBy's
	// sort.Strings(out) makes this order a coin flip on Go's randomized map
	// iteration, so an exact ordered match gives that mutation somewhere to
	// fail instead of passing by accident.
	if !strings.Contains(out, "(web/one.go, web/two.go)") {
		t.Fatalf("a corroborated directory site must name every path it rests on, in order; got:\n%s", out)
	}

	// Above the cap: a site whose hits exceed maxDirectoryEvidencePaths, so the
	// exact count and the "+N more" suffix are both exercised — neither is
	// reachable by the two-file fixture above.
	overflow := 2
	total := maxDirectoryEvidencePaths + overflow
	capFiles := map[string]string{}
	capNames := make([]string, total)
	for i := 0; i < total; i++ {
		name := fmt.Sprintf("cap/f%02d.go", i)
		capFiles[name] = "package cap"
		capNames[i] = name
	}
	rootCap := consumersFixture(t, []string{"cap/: fixed-here (whole package reworked)"}, capFiles)
	withDiff(t, append([]string{"docs/streams/alpha/brief-01-claims.md"}, capNames...)...)
	outCap := captureStdout(t, func() {
		if code := runConsumers(rootCap, "origin/main", ""); code != 0 {
			t.Fatalf("a directory site touched by all its members is CORROBORATED; got %d", code)
		}
	})
	// capNames is already in sorted order (zero-padded numeric suffixes), so the
	// first maxDirectoryEvidencePaths of it is exactly what the cap should show.
	wantShown := fmt.Sprintf("(%s, +%d more)", strings.Join(capNames[:maxDirectoryEvidencePaths], ", "), overflow)
	if !strings.Contains(outCap, wantShown) {
		t.Fatalf("expected the capped evidence list %q; got:\n%s", wantShown, outCap)
	}
	wantCount := fmt.Sprintf("corroborated by %d path(s) under it", total)
	if !strings.Contains(outCap, wantCount) {
		t.Fatalf("the count above the cap must still be exact, not capped; got:\n%s", outCap)
	}

	// Control: the precise file site naming that same file must stay reason-free, so
	// the new line marks widening specifically rather than decorating everything.
	root2 := consumersFixture(t, []string{"web/one.go: fixed-here"}, map[string]string{
		"web/one.go": "package web",
		"web/two.go": "package web",
	})
	withDiff(t, "docs/streams/alpha/brief-01-claims.md", "web/one.go")
	out2 := captureStdout(t, func() {
		if code := runConsumers(root2, "origin/main", ""); code != 0 {
			t.Fatalf("a precise file site in the diff is CORROBORATED; got %d", code)
		}
	})
	if strings.Contains(out2, "is a directory, corroborated by") {
		t.Fatalf("a plain file site must not carry directory evidence; got:\n%s", out2)
	}
}

// TestOutOfScopeContradictedByTheDiff (review of #371, finding 7). Most
// out-of-scope judgements are unsettleable; this sub-case settles itself — the
// branch declares a consumer excluded and then edits it.
func TestOutOfScopeContradictedByTheDiff(t *testing.T) {
	root := consumersFixture(t, []string{"web/one.go: out-of-scope (frozen until the pin bump lands)"}, map[string]string{
		"web/one.go": "package web",
	})
	withDiff(t, "docs/streams/alpha/brief-01-claims.md", "web/one.go")
	if code := runConsumers(root, "origin/main", ""); code != 1 {
		t.Fatalf("excluding a consumer this diff edits is a self-contradiction the diff can settle; got %d", code)
	}
	// Untouched: back to the reviewer's judgement, UNCHECKED and exit 0.
	withDiff(t, "docs/streams/alpha/brief-01-claims.md")
	if code := runConsumers(root, "origin/main", ""); code != 0 {
		t.Fatalf("an out-of-scope consumer the diff leaves alone stays UNCHECKED; got %d", code)
	}
}

// TestBriefClaimingNothingIsReportedSeparately (review of #371, finding 6). "No
// consumers: field" and "consumers checked clean" printed the same summary line
// and the same exit code — an UNRUN check indistinguishable from a clean one, in
// the subsystem built to tell those apart. The gate still cannot tell whether a
// list is COMPLETE; it can at least stop implying it looked.
func TestBriefClaimingNothingIsReportedSeparately(t *testing.T) {
	root := consumersFixture(t, []string{"web/one.go: fixed-here"}, map[string]string{
		"web/one.go": "package web",
	})
	withDiff(t, "docs/streams/alpha/brief-01-claims.md", "docs/streams/alpha/brief-02-target.md", "web/one.go")
	out := captureStdout(t, func() {
		if code := runConsumers(root, "origin/main", ""); code != 0 {
			t.Errorf("claiming nothing is not a failure; got %d", code)
		}
	})
	if !strings.Contains(out, "NO-CLAIM") || !strings.Contains(out, "alpha/02") {
		t.Errorf("a brief in the diff with no consumers: must be named, not omitted:\n%s", out)
	}
	if !strings.Contains(out, "1 brief(s) claiming nothing") {
		t.Errorf("the summary must separate 'nothing claimed' from 'nothing wrong':\n%s", out)
	}
	if !strings.Contains(out, "OMITTED consumer is invisible") {
		t.Errorf("the completeness limit must be stated in the output, not only in the docs:\n%s", out)
	}
}

// TestChangedPathsSinceReadsRealGit (review of #371, finding 8). Every gate test
// substitutes the diff, so the code that decides what the diff IS — three-dot
// semantics, the porcelain `XY ` prefix, the `old -> new` rename split — was
// exercised by nothing. That is the exact shape of the two prior defects this
// brief cites: a green suite over an untested seam.
func TestChangedPathsSinceReadsRealGit(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	mustWrite(t, filepath.Join(root, "base.txt"), "base\n")
	mustWrite(t, filepath.Join(root, "renamed-from.txt"), "move me\n")
	git("add", "-A")
	git("commit", "-qm", "base")

	// main moves on AFTER the branch point: three-dot must NOT report this file,
	// or the gate would corroborate a claim against somebody else's work.
	git("checkout", "-qb", "feature")
	git("checkout", "-q", "main")
	mustWrite(t, filepath.Join(root, "main-only.txt"), "not ours\n")
	git("add", "-A")
	git("commit", "-qm", "main moves on")
	git("checkout", "-q", "feature")

	mustWrite(t, filepath.Join(root, "committed.txt"), "ours\n")
	git("add", "-A")
	git("commit", "-qm", "branch work")
	git("mv", "renamed-from.txt", "renamed-to.txt")
	mustWrite(t, filepath.Join(root, "dirty.txt"), "uncommitted\n")
	mustWrite(t, filepath.Join(root, "untracked.txt"), "untracked\n")

	got, err := changedPathsSince(root, "main")
	if err != nil {
		t.Fatalf("changedPathsSince: %v", err)
	}
	set := map[string]bool{}
	for _, p := range got {
		set[p] = true
	}
	for _, want := range []string{"committed.txt", "renamed-from.txt", "renamed-to.txt", "dirty.txt", "untracked.txt"} {
		if !set[want] {
			t.Errorf("missing %q from %v", want, got)
		}
	}
	if set["main-only.txt"] {
		t.Errorf("three-dot must exclude commits made on the base after the branch point; got %v", got)
	}
}

// TestConsumersUntouchedBriefsAreNotJudged: a brief this branch did not touch
// refers to ITS OWN branch's diff. Judging it against this one would red-gate
// every later edit — the false-positive class that makes diff-keyed checks get
// switched off.
func TestConsumersUntouchedBriefsAreNotJudged(t *testing.T) {
	root := consumersFixture(t, []string{"web/untouched.go: fixed-here"}, map[string]string{
		"web/untouched.go": "package web",
	})
	withDiff(t, "README.md")
	if code := runConsumers(root, "origin/main", ""); code != 0 {
		t.Fatalf("an untouched brief must not be judged against this diff; got %d", code)
	}
}
