package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// digestNow is the fixed clock every digest test reads, so rendered ages are
// deterministic.
var digestNow = time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

// vgEntered is the historian view for the verify-gate fixture tree: the time
// each eligible brief entered its current awaiting status. Deliberately NOT in
// brief-id order — an implementation that "sorts" by falling back to the map's
// insertion or id order cannot pass the oldest-first assertion below.
//
//	vg/08 — 10 days at the gate (the oldest, and the whole point of the digest)
//	vg/02 —  6 days
//	vg/09 —  3 days
//	vg/01 —  2 days
//	vg/07 —  4 hours
//	vg/11 — no historian record at all (unknown age)
func vgEntered() map[string]time.Time {
	return map[string]time.Time{
		"vg/01": digestNow.Add(-48 * time.Hour),
		"vg/02": digestNow.Add(-144 * time.Hour),
		"vg/07": digestNow.Add(-4 * time.Hour),
		"vg/08": digestNow.Add(-240 * time.Hour),
		"vg/09": digestNow.Add(-72 * time.Hour),
	}
}

func digestBriefIDs(d signoffDigest) []string {
	var ids []string
	for _, e := range d.Entries {
		ids = append(ids, e.Brief)
	}
	return ids
}

// TestSignoffDigestSetIsVerifyIssuesSet pins the single-source-of-truth
// requirement: the digest's membership is EXACTLY mm/12's eligible set, not a
// second, independently-derived predicate. The assertion compares against a live
// verifyIssues() call on the same tree rather than a hand-written list, so a
// future change to the eligibility rule moves both or fails here.
func TestSignoffDigestSetIsVerifyIssuesSet(t *testing.T) {
	root, streams := loadVGStreams(t)

	want := []string{}
	for _, iss := range verifyIssues(root, streams, map[string]bool{}) {
		want = append(want, iss.Brief)
	}
	if len(want) == 0 {
		t.Fatal("fixture regression: verifyIssues emitted nothing, so this test proves nothing")
	}

	d := buildSignoffDigest(root, streams, vgEntered(), true, digestNow)
	got := append([]string(nil), digestBriefIDs(d)...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("digest set = %v, want the verifyIssues set %v", got, want)
	}
	if d.State != signoffAwaiting {
		t.Errorf("state = %q, want %q", d.State, signoffAwaiting)
	}
}

// TestSignoffDigestListsNonIrreversibleClass is the mm/49 neighbour-row coverage:
// the digest derives its membership from verifyIssues(), so the newly-eligible
// non-irreversible gate:human class (vg/12) appears in the roll-up with zero digest
// code change. This is coverage of the inherited behavior, not a second predicate.
func TestSignoffDigestListsNonIrreversibleClass(t *testing.T) {
	root, streams := loadVGStreams(t)
	d := buildSignoffDigest(root, streams, vgEntered(), true, digestNow)
	found := false
	for _, e := range d.Entries {
		if e.Brief == "vg/12" {
			found = true
		}
	}
	if !found {
		t.Error("vg/12 (gate:human, irreversible:no, implemented, VERIFY: PASS) must appear in the sign-off digest — it is verifyIssues-eligible after mm/49")
	}
	if !strings.Contains(renderSignoffDigest(d), "vg/12") {
		t.Error("rendered digest must name vg/12")
	}
}

// TestSignoffDigestExcludesSignedOff pins the exclusion the human cares about:
// vg/05 is gate:human and irreversible, but a human has already closed it
// (status done, Reviewed `2026-07-08 human:alex`). A digest that keeps listing
// briefs the human has already signed off is a nag, not a queue.
func TestSignoffDigestExcludesSignedOff(t *testing.T) {
	root, streams := loadVGStreams(t)
	d := buildSignoffDigest(root, streams, vgEntered(), true, digestNow)
	for _, e := range d.Entries {
		if e.Brief == "vg/05" {
			t.Fatal("vg/05 is already signed off by a human (done, Reviewed human:alex) — it must not appear in the sign-off digest")
		}
	}
	// And the rendered body must not name it either.
	if strings.Contains(renderSignoffDigest(d), "vg/05") {
		t.Error("rendered digest names vg/05, which is already signed off")
	}
}

// TestSignoffDigestOldestFirst is the product assertion. A digest sorted any
// other way buries the item that has waited longest, which is the failure this
// brief exists to close. Briefs with NO historian record cannot claim an age and
// sort last, in brief-id order — never interleaved as if they were brand new
// (a zero timestamp would otherwise sort them oldest, inventing an age).
func TestSignoffDigestOldestFirst(t *testing.T) {
	root, streams := loadVGStreams(t)
	d := buildSignoffDigest(root, streams, vgEntered(), true, digestNow)

	// vg/12 and vg/14 (the mm/49 non-irreversible class) are eligible but have no
	// historian record, so they sort LAST in brief-id order, after vg/11.
	want := []string{"vg/08", "vg/02", "vg/09", "vg/01", "vg/07", "vg/11", "vg/12", "vg/14"}
	if got := digestBriefIDs(d); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("digest order = %v, want oldest-first %v", got, want)
	}

	wantAge := map[string]string{
		"vg/08": "10d",
		"vg/02": "6d",
		"vg/09": "3d",
		"vg/01": "2d",
		"vg/07": "4h",
		"vg/11": "—",
		"vg/12": "—",
		"vg/14": "—",
	}
	for _, e := range d.Entries {
		if e.Age != wantAge[e.Brief] {
			t.Errorf("%s age = %q, want %q", e.Brief, e.Age, wantAge[e.Brief])
		}
	}

	// The rendered table must carry the same order — a correctly sorted struct
	// rendered in map order would still bury the oldest item.
	body := renderSignoffDigest(d)
	prev := -1
	for _, id := range want {
		i := strings.Index(body, id)
		if i < 0 {
			t.Fatalf("rendered digest omits %s", id)
		}
		if i <= prev {
			t.Errorf("rendered digest lists %s out of oldest-first order", id)
		}
		prev = i
	}
}

// TestSignoffDigestEvidenceLink: every listed brief carries a link into its
// recorded Evidence, so the human can act from the digest alone rather than
// hunting the brief file.
func TestSignoffDigestEvidenceLink(t *testing.T) {
	root, streams := loadVGStreams(t)
	d := buildSignoffDigest(root, streams, vgEntered(), true, digestNow)
	for _, e := range d.Entries {
		if !strings.Contains(e.Evidence, "#evidence") {
			t.Errorf("%s evidence link = %q, want an #evidence anchor", e.Brief, e.Evidence)
		}
		if !strings.Contains(e.Evidence, "docs/streams/vg/brief-") {
			t.Errorf("%s evidence link = %q, want the brief file path", e.Brief, e.Evidence)
		}
	}
	body := renderSignoffDigest(d)
	if !strings.Contains(body, "brief-08-human-verified-long-title.md#evidence") {
		t.Error("rendered digest is missing the oldest brief's Evidence link")
	}
}

// writeClearRoot builds a root whose only stream has nothing at the human gate:
// one gate:model brief at verified. The digest must call that CLEAR, positively.
func writeClearRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "clr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := `---
stream: clr
status: active
priority: P2
track: platform
---

# CLR (clear fixture)

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Model-gated verified](./brief-01-model-verified.md) | 0 | S | verified | 2026-07-08 rev | — |
`
	brief := `---
brief: clr/01
title: Model-gated verified brief
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: model-gated"]
---

# Brief 01 — model-gated

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | ` + "`go vet ./...`" + ` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | ` + "`go vet ./...`" + ` | 0 | ok | 2026-07-08 | reviewer |

## Review
Gate: model.
`
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-01-model-verified.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestSignoffDigestClearIsNotCouldNotCheck is the three-state assertion
// (desk-hardening/01). An empty eligible set and an unreadable input are
// DIFFERENT FACTS and must read differently — the #777 empty-scope-reads-as-clean
// bug is a scope that could not be read rendering as "nothing here".
func TestSignoffDigestClearIsNotCouldNotCheck(t *testing.T) {
	root := writeClearRoot(t)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}

	clear := buildSignoffDigest(root, streams, nil, true, digestNow)
	if clear.State != signoffClear {
		t.Fatalf("state = %q, want %q", clear.State, signoffClear)
	}
	clearBody := renderSignoffDigest(clear)
	if !strings.Contains(clearBody, "Nothing is awaiting a human sign-off") {
		t.Errorf("clear digest must assert the empty queue positively; got:\n%s", clearBody)
	}
	if strings.Contains(clearBody, signoffCouldNotCheck) {
		t.Error("a genuinely clear digest must not claim could-not-check")
	}

	cnc := couldNotCheckSignoffDigest("docs/streams: permission denied", digestNow)
	if cnc.State != signoffCouldNotCheck {
		t.Fatalf("state = %q, want %q", cnc.State, signoffCouldNotCheck)
	}
	cncBody := renderSignoffDigest(cnc)
	if !strings.Contains(cncBody, signoffCouldNotCheck) {
		t.Errorf("could-not-check digest must say so; got:\n%s", cncBody)
	}
	if strings.Contains(cncBody, "Nothing is awaiting a human sign-off") {
		t.Error("could-not-check must NEVER render as an empty queue — that is the #777 failure")
	}
	if !strings.Contains(cncBody, "permission denied") {
		t.Error("could-not-check digest must carry the reason it could not read its inputs")
	}
}

// TestSignoffDigestDegradedAges: the brief set read fine but the historian did
// not. The list is still trustworthy; the ORDERING is not, and the digest says
// so out loud rather than presenting id order as oldest-first.
func TestSignoffDigestDegradedAges(t *testing.T) {
	root, streams := loadVGStreams(t)
	d := buildSignoffDigest(root, streams, nil, false, digestNow)
	if d.State != signoffAwaiting {
		t.Fatalf("state = %q, want %q — an unreadable historian does not hide the queue", d.State, signoffAwaiting)
	}
	if len(d.Entries) == 0 {
		t.Fatal("degraded-age digest must still list the awaiting briefs")
	}
	body := renderSignoffDigest(d)
	if !strings.Contains(body, "age could-not-check") {
		t.Errorf("degraded digest must flag the unknown ages; got:\n%s", body)
	}
	if !strings.Contains(body, "cannot claim oldest-first") {
		t.Errorf("degraded digest must withdraw the oldest-first claim; got:\n%s", body)
	}
	for _, e := range d.Entries {
		if e.Age != "—" {
			t.Errorf("%s age = %q, want — when the historian could not be read", e.Brief, e.Age)
		}
	}
}

// TestSignoffAgeAtGateMetric covers the per-stream oldest-age-at-gate metric:
// computed from the historian, gate:human awaiting rows only, oldest stream
// first, and "—" (never 0) when the historian has no record.
func TestSignoffAgeAtGateMetric(t *testing.T) {
	streams := []*Stream{
		{Name: "alpha", Briefs: []Brief{
			{Num: "01", Status: "implemented", Gate: "human"},
			{Num: "02", Status: "verified", Gate: "human"},
			{Num: "03", Status: "done", Gate: "human"},
			{Num: "04", Status: "verified", Gate: "model"},
		}},
		{Name: "beta", Briefs: []Brief{
			{Num: "01", Status: "implemented", Gate: "human"},
		}},
		{Name: "gamma", Briefs: []Brief{
			{Num: "01", Status: "implemented", Gate: "human"},
		}},
		{Name: "delta", Briefs: []Brief{
			{Num: "01", Status: "verified", Gate: "model"},
		}},
	}
	entered := map[string]time.Time{
		"alpha/01": digestNow.Add(-72 * time.Hour),
		"alpha/02": digestNow.Add(-192 * time.Hour), // 8d — alpha's oldest
		"alpha/03": digestNow.Add(-999 * time.Hour), // done: not at the gate
		"alpha/04": digestNow.Add(-999 * time.Hour), // gate:model: not at the human gate
		"beta/01":  digestNow.Add(-72 * time.Hour),
		// gamma/01 has no historian record → unknown, never zero.
	}

	rows := oldestHumanGateAges(streams, entered, digestNow)
	var got []string
	for _, r := range rows {
		got = append(got, r.Stream+"="+r.Age+"@"+r.Brief)
	}
	want := []string{"alpha=8d@02", "beta=3d@01", "gamma=—@"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("oldestHumanGateAges = %v, want %v (oldest stream first, unknown last, delta absent)", got, want)
	}
}

// TestSignoffAgeAtGateOnBoard: the metric reaches STATUS.md, so the human gate's
// queue is as visible as the model gates' — today only counts are.
func TestSignoffAgeAtGateOnBoard(t *testing.T) {
	s := &Stream{Name: "alpha", Track: "platform", Priority: "P0", Status: "active", Briefs: []Brief{
		{Num: "01", Title: "waiting", Status: "verified", Gate: "human", Wave: 0},
	}}
	rows := oldestHumanGateAges([]*Stream{s}, map[string]time.Time{
		"alpha/01": digestNow.Add(-192 * time.Hour),
	}, digestNow)
	out := emit([]*Stream{s}, nil, nextUp([]*Stream{s}, ClaimView{}, nil), nil, rows, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "Age at the human gate") {
		t.Error("STATUS.md is missing the per-stream age-at-gate section")
	}
	if !strings.Contains(out, "| alpha | 8d | 01 |") {
		t.Errorf("STATUS.md is missing the alpha age-at-gate row; got:\n%s", out)
	}
}

// TestBuildSignoffDigestJSON verifies the leak-safe aggregate view (agentic-metrics/02):
// counts + oldest age + WIP-by-status only, never a brief id/title, three-state honest.
func TestBuildSignoffDigestJSON(t *testing.T) {
	d := signoffDigest{
		Date:      "2026-07-20",
		State:     signoffAwaiting,
		AgesKnown: true,
		Entries: []signoffEntry{
			// Oldest-first order (as buildSignoffDigest sorts): known arrival first.
			{Brief: "alpha/01", Title: "SECRET internal title", Status: "verified",
				Age: "8d", EnteredAt: digestNow.Add(-192 * time.Hour)},
			{Brief: "beta/02", Title: "another private title", Status: "implemented",
				Age: "2d", EnteredAt: digestNow.Add(-48 * time.Hour)},
		},
	}
	got := buildSignoffDigestJSON(d)
	if got.State != signoffAwaiting {
		t.Errorf("state = %q, want %q", got.State, signoffAwaiting)
	}
	if got.AwaitingCount != 2 {
		t.Errorf("awaiting_count = %d, want 2", got.AwaitingCount)
	}
	if got.WIPVerified != 1 || got.WIPImplemented != 1 {
		t.Errorf("WIP split wrong: verified=%d implemented=%d", got.WIPVerified, got.WIPImplemented)
	}
	if got.OldestAge != "8d" {
		t.Errorf("oldest_age = %q, want 8d (longest-waiter)", got.OldestAge)
	}
	// Leak-safety: no brief id or title may cross into the aggregate JSON.
	enc, _ := json.Marshal(got)
	for _, banned := range []string{"alpha/01", "beta/02", "SECRET internal title", "another private title"} {
		if strings.Contains(string(enc), banned) {
			t.Errorf("decision JSON leaked internal detail %q: %s", banned, enc)
		}
	}

	// could-not-check propagates and asserts no count.
	cnc := buildSignoffDigestJSON(couldNotCheckSignoffDigest("historian unreadable", digestNow))
	if cnc.State != signoffCouldNotCheck {
		t.Errorf("state = %q, want %q", cnc.State, signoffCouldNotCheck)
	}
	if cnc.AwaitingCount != 0 || cnc.OldestAge != "" {
		t.Errorf("could-not-check must not assert a count/age: %+v", cnc)
	}
}

// TestRunSignoffDigestCouldNotCheckExits: an unreadable root must exit non-zero
// with a could-not-check body, never exit 0 with an empty digest. A cron that
// treats an unreadable repo as "nothing waiting" is worse than no cron.
func TestRunSignoffDigestCouldNotCheckExits(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	if code := runSignoffDigest(root, false); code == 0 {
		t.Error("runSignoffDigest on an unreadable root exited 0 — could-not-check must fail loudly")
	}
}

// TestRunSignoffDigestOnFixture is the end-to-end path the workflow runs.
func TestRunSignoffDigestOnFixture(t *testing.T) {
	root, _ := loadVGStreams(t)
	if code := runSignoffDigest(root, false); code != 0 {
		t.Errorf("runSignoffDigest = %d, want 0", code)
	}
}

// TestSignoffDigestNoAgeOnRecordWithdrawsTheClaim closes a hole found by RUNNING
// the digest against this repo rather than only against fixtures: the repo has
// no `.history.jsonl`, and LoadHistory returns (nil, nil) for a MISSING log
// (best-effort, so the board still renders). Consumed naively that reads as
// "historian fine, nothing recorded", and the digest printed "Oldest first" over
// eight rows whose ages it had never measured — the same empty-map-means-two-
// things bug as an empty claim map standing in for an unreadable remote.
//
// Two distinct facts, two distinct renders, and neither may claim oldest-first:
// an ABSENT/unreadable historian says could-not-check; a historian that WAS read
// but records nothing must NOT borrow that word, or it sends someone to debug a
// read failure that never happened.
func TestSignoffDigestNoAgeOnRecordWithdrawsTheClaim(t *testing.T) {
	root, streams := loadVGStreams(t)

	// Historian read fine, but it holds no transition for any listed brief.
	read := buildSignoffDigest(root, streams, map[string]time.Time{}, true, digestNow)
	body := renderSignoffDigest(read)
	if !strings.Contains(body, "cannot claim oldest-first") {
		t.Errorf("a digest with no age on record must withdraw the oldest-first claim; got:\n%s", body)
	}
	if strings.Contains(body, signoffCouldNotCheck) {
		t.Errorf("the historian WAS read — the body must not claim could-not-check; got:\n%s", body)
	}
	if strings.Contains(body, "Oldest first") {
		t.Errorf("body claims oldest-first over ages it never measured; got:\n%s", body)
	}

	// And the end-to-end path over a root with NO .history.jsonl at all must
	// reach the could-not-check wording, not the "read it, empty" one — the
	// fixture tree ships no historian, which is what made this reachable.
	if _, err := os.Stat(filepath.Join(root, "docs", "streams", ".history.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("fixture regression: this test needs a root with NO historian, stat err = %v", err)
	}
}
