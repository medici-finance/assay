package contexthandoff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// The journey corpus: two neutral example repositories and a guideline
// source. The resolver's input carries no expected answer; every expected
// identity below lives only in these tests.
const journeyRoot = fixtureRoot + "/journeys/base"

const (
	idA01 = "example:repo-a:alpha:01"
	idA02 = "example:repo-a:alpha:02"
	idA03 = "example:repo-a:alpha:03"
	idA04 = "example:repo-a:alpha:04"
	idA05 = "example:repo-a:alpha:05"
	idB01 = "example:repo-b:beta:01"
	idB02 = "example:repo-b:beta:02"
	idB03 = "example:repo-b:beta:03"
	idG04 = "example:repo-b:gamma:04"

	fileA01 = "sources/repo-a/docs/streams/alpha/brief-01-nightly-export.md"
	fileA02 = "sources/repo-a/docs/streams/alpha/brief-02-reconciliation-report.md"
	fileA03 = "sources/repo-a/docs/streams/alpha/brief-03-export-layout-v2.md"
	fileA04 = "sources/repo-a/docs/streams/alpha/brief-04-export-reader.md"
	fileA05 = "sources/repo-a/docs/streams/alpha/brief-05-audit-log-rotation.md"
	fileG04 = "sources/repo-b/docs/streams/gamma/brief-04-dashboard.md"
)

// loadJourney copies the base corpus into memory so each test can vary it.
func loadJourney(t *testing.T) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	root := os.DirFS(journeyRoot)
	err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(root, p)
		if err != nil {
			return err
		}
		m[p] = &fstest.MapFile{Data: b}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func snap(t *testing.T, m fstest.MapFS, lim Limits) *Snapshot {
	t.Helper()
	s, err := LoadSnapshot(m, lim)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	return s
}

func discover(t *testing.T, s *Snapshot) *Handoff {
	t.Helper()
	h, err := Discover(s)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return h
}

// editSnapshot rewrites snapshot.json through a generic tree.
func editSnapshot(t *testing.T, m fstest.MapFS, f func(map[string]any)) {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(m["snapshot.json"].Data, &v); err != nil {
		t.Fatal(err)
	}
	f(v)
	m["snapshot.json"] = &fstest.MapFile{Data: marshal(t, v)}
}

func replaceIn(t *testing.T, m fstest.MapFS, path, old, new string) {
	t.Helper()
	s := string(m[path].Data)
	if !strings.Contains(s, old) {
		t.Fatalf("%s does not contain %q", path, old)
	}
	m[path] = &fstest.MapFile{Data: []byte(strings.Replace(s, old, new, 1))}
}

func impactOf(h *Handoff, id string) *Impact {
	for i := range h.Impacts {
		if h.Impacts[i].Subject == id {
			return &h.Impacts[i]
		}
	}
	return nil
}

func results(h *Handoff, rule string) []string { return queryResults(h, rule) }

// reshape decodes a copy of h after f edits it, so every handoff a test builds
// is one the statusgen/19 contract accepts.
func reshape(t *testing.T, h *Handoff, f func(*Handoff)) *Handoff {
	t.Helper()
	c, err := DecodeHandoff(marshal(t, h))
	if err != nil {
		t.Fatal(err)
	}
	f(c)
	out, err := DecodeHandoff(marshal(t, c))
	if err != nil {
		t.Fatalf("test handoff is not contract-valid: %v", err)
	}
	return out
}

// decide records an author disposition that needs no obligation.
func decide(t *testing.T, h *Handoff, subject, disposition, reason string) *Handoff {
	t.Helper()
	return reshape(t, h, func(c *Handoff) {
		im := impactOf(c, subject)
		if im == nil {
			t.Fatalf("no impact on %s", subject)
		}
		im.Disposition, im.Reason = disposition, reason
		c.Readiness.Unresolved = without(c.Readiness.Unresolved, subject)
	})
}

// amend records an amend obligation on subject, bound to a candidate patch
// (or to nothing, when the binding comes from a manifest).
func amend(t *testing.T, h *Handoff, subject, after string, cand *Patch) *Handoff {
	t.Helper()
	return reshape(t, h, func(c *Handoff) {
		im := impactOf(c, subject)
		if im == nil {
			t.Fatalf("no impact on %s", subject)
		}
		im.Disposition, im.Reason = "amend", "author: the target changes this brief's task"
		c.Readiness.Unresolved = without(c.Readiness.Unresolved, subject)
		c.Amendments = append(c.Amendments, Amendment{
			ID: "ob-1", Subject: subject, Action: "amend",
			Changes:    []Change{{Aspect: "task", Before: "the v1 layout", After: after}},
			ReplacedBy: []string{}, Candidate: cand,
		})
	})
}

func without(list []string, s string) []string {
	out := []string{}
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

// unifiedDiff writes a one-hunk unified diff from old to new with up to three
// lines of context, in the form git produces.
func unifiedDiff(path string, old, new []byte) []byte {
	a, b := splitLines(old), splitLines(new)
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	lo := max(0, p-3)
	hiA := min(len(a), len(a)-s+3)
	tail := hiA - (len(a) - s)
	var w bytes.Buffer
	fmt.Fprintf(&w, "--- a/%s\n+++ b/%s\n", path, path)
	fmt.Fprintf(&w, "@@ -%d,%d +%d,%d @@\n", lo+1, hiA-lo, lo+1, (p-lo)+(len(b)-s-p)+tail)
	for _, l := range a[lo:p] {
		w.WriteString(" " + l + "\n")
	}
	for _, l := range a[p : len(a)-s] {
		w.WriteString("-" + l + "\n")
	}
	for _, l := range b[p : len(b)-s] {
		w.WriteString("+" + l + "\n")
	}
	for _, l := range a[len(a)-s : hiA] {
		w.WriteString(" " + l + "\n")
	}
	return w.Bytes()
}

// candPatch builds a patch from a text edit of one brief file and binds it to
// the file's current blob.
func candPatch(t *testing.T, m fstest.MapFS, file, old, new string) (*Patch, []byte) {
	t.Helper()
	raw := m[file].Data
	if !strings.Contains(string(raw), old) {
		t.Fatalf("%s does not contain %q", file, old)
	}
	edited := []byte(strings.Replace(string(raw), old, new, 1))
	inner := strings.TrimPrefix(file, "sources/")
	inner = inner[strings.IndexByte(inner, '/')+1:]
	p := unifiedDiff(inner, raw, edited)
	fp, err := parsePatch(p)
	if err != nil {
		t.Fatalf("test patch does not parse: %v\n%s", err, p)
	}
	res, err := fp.apply(raw)
	if err != nil || !bytes.Equal(res.content, edited) {
		t.Fatalf("test patch does not apply cleanly (%v)\n%s", err, p)
	}
	return &Patch{BaseRevision: BlobRevision(raw)[:12], Digest: PatchDigest(p)}, p
}

func check(t *testing.T, h *Handoff, m *Manifest, patches map[string][]byte, s *Snapshot) *Report {
	t.Helper()
	rep, err := Check(h, m, patches, s)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return rep
}

func verdictOf(t *testing.T, rep *Report, obligation string) Verdict {
	t.Helper()
	for _, v := range rep.Verdicts {
		if v.Obligation == obligation {
			return v
		}
	}
	t.Fatalf("no verdict for %s in %+v", obligation, rep.Verdicts)
	return Verdict{}
}

func findingOf(fs []Finding, scope string) *Finding {
	for i := range fs {
		if fs[i].Scope == scope {
			return &fs[i]
		}
	}
	return nil
}

// regression: statusgen/20 Verify row 1 — discovery from typed edges alone
// drops the consumers:-field consumer and the prose-only consumer that has no
// typed edge and no README row.
func TestContextResolverWithheldConsumers(t *testing.T) {
	s := snap(t, loadJourney(t), Limits{})
	h := discover(t, s)

	want := map[string]string{ // subject -> relationship basis
		idA01: "observed", // the declaring brief depends on it
		idA02: "declared", // its consumers: field names the contract
		idA04: "observed", // depends on the declaring brief
		idB01: "declared", // declared semantic owner of the contract term
		idB02: "observed", // cross-repository depends on the declaring brief
		idG04: "inferred", // prose only: no frontmatter, no README row
	}
	got := map[string]string{}
	for _, im := range h.Impacts {
		got[im.Subject] = im.Relationship.Basis
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("impacts = %v\nwant      %v", got, want)
	}
	for _, id := range []string{idA03, idA05, idB03} {
		if impactOf(h, id) != nil {
			t.Errorf("%s is not an owner or consumer, but discovery reports it", id)
		}
	}
	if !contains(results(h, QueryDirectory), idG04) {
		t.Errorf("directory enumeration missed %s (it has no README row)", idG04)
	}
	if !contains(results(h, QueryProse), "repo-a:docs/notes/ledger.md") {
		t.Errorf("prose candidates = %v, want the plan note that names the term", results(h, QueryProse))
	}

	// Disposition: the delivered brief is settled and routed; every other
	// impact waits for an author and is listed as unresolved.
	if im := impactOf(h, idA01); im == nil || im.Disposition != "retain" || !strings.Contains(im.Reason, idA03+" (linked follow-on)") {
		t.Errorf("delivered brief impact = %+v, want retain routed to %s", im, idA03)
	}
	for _, im := range h.Impacts {
		if im.Subject != idA01 && (im.Disposition != "unresolved" || !contains(h.Readiness.Unresolved, im.Subject)) {
			t.Errorf("%s: disposition %q, want unresolved and listed", im.Subject, im.Disposition)
		}
	}
	if g := impactOf(h, idG04); g == nil || g.Lifecycle != "unknown" {
		t.Errorf("prose-only consumer = %+v, want lifecycle unknown (no README row)", g)
	}

	// History and active work are recorded for the impacted briefs.
	kinds := map[string]int{}
	for _, x := range h.History {
		if x.Subject == idA01 {
			kinds[x.Kind]++
		}
	}
	if kinds["evidence"] != 1 || kinds["source-account"] != 2 {
		t.Errorf("history of the delivered brief = %v, want its Evidence and two dated source accounts", kinds)
	}
	if len(h.ActiveWork) != 1 || h.ActiveWork[0].Subject != idA04 || h.ActiveWork[0].Treatment != "reconcile" {
		t.Errorf("active work = %+v, want the claim on %s, unacknowledged", h.ActiveWork, idA04)
	}
	if h.Discovery.Coverage.State != "complete" || !contains(h.Readiness.Prerequisites, ArchitectureOnly) {
		t.Errorf("coverage %q, prerequisites %v", h.Discovery.Coverage.State, h.Readiness.Prerequisites)
	}

	// Deterministic: a second load and run give the same receipt digest.
	if again := discover(t, snap(t, loadJourney(t), Limits{})); again.Digest() != h.Digest() {
		t.Errorf("receipt digest is not reproducible: %s vs %s", again.Digest(), h.Digest())
	}
}

// regression: statusgen/20 Verify row 2 — invalidating every conclusion on any
// edit discards author decisions an unrelated edit cannot affect.
func TestContextRecheckUnrelatedEdit(t *testing.T) {
	m := loadJourney(t)
	prior := decide(t, discover(t, snap(t, m, Limits{})), idB01, "defer", "author: the owner keeps the v1 term until the transition ends")

	same, err := Recheck(prior, snap(t, m, Limits{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(same.Findings) != 0 || same.Receipt.Digest() != prior.Digest() {
		t.Errorf("same inputs: findings %+v, digest %s, want none and %s", same.Findings, same.Receipt.Digest(), prior.Digest())
	}

	replaceIn(t, m, fileA05, "Rotate the audit log weekly.", "Rotate the audit log weekly and compress old segments.")
	rc, err := Recheck(prior, snap(t, m, Limits{}))
	if err != nil {
		t.Fatal(err)
	}
	if rc.Reconcile() || rc.Hold() {
		t.Errorf("an unrelated edit forced reconciliation: %+v", rc.Findings)
	}
	f := findingOf(rc.Findings, "file:repo-a:docs/streams/alpha/brief-05-audit-log-rotation.md")
	if f == nil || f.Class != FindingPreserved || len(rc.Findings) != 1 {
		t.Errorf("findings = %+v, want exactly the unrelated edit, preserved", rc.Findings)
	}
	if im := impactOf(rc.Receipt, idB01); im == nil || im.Disposition != "defer" {
		t.Errorf("author decision on %s was not preserved: %+v", idB01, im)
	}
	if im := impactOf(rc.Receipt, idA01); im == nil || im.Disposition != "retain" {
		t.Errorf("routing of %s was not preserved: %+v", idA01, im)
	}
}

// regression: statusgen/20 Verify row 3 — a recheck of previously read file
// hashes alone cannot see a consumer added outside the prior read set.
func TestContextRecheckNewConsumerOutsideReadSet(t *testing.T) {
	m := loadJourney(t)
	prior := discover(t, snap(t, m, Limits{}))

	// A new stream directory in an existing source; no previously read file changes.
	m["sources/repo-b/docs/streams/delta/brief-07-ledger-archive.md"] = &fstest.MapFile{
		Data: []byte("# Brief 07 — Ledger archive\n\n## Task\n\nKeep every ledger-export file for seven years.\n"),
	}
	s := snap(t, m, Limits{})
	fresh := atMap(results(discover(t, s), QueryReadSet))
	for loc, rev := range atMap(results(prior, QueryReadSet)) {
		if fresh[loc] != rev {
			t.Fatalf("fixture error: previously read %s changed", loc)
		}
	}

	rc, err := Recheck(prior, s)
	if err != nil {
		t.Fatal(err)
	}
	const newID = "example:repo-b:delta:07"
	f := findingOf(rc.Findings, "subject:"+newID)
	if f == nil || f.Class != FindingReconcile || !strings.Contains(f.Reason, "outside the prior read set") {
		t.Errorf("findings = %+v, want a reconcile for the new consumer outside the prior read set", rc.Findings)
	}
	if !rc.Reconcile() || rc.Receipt.Readiness.NextAct != "explore" || !contains(rc.Receipt.Readiness.Unresolved, newID) {
		t.Errorf("readiness = %+v, want explore with %s unresolved", rc.Receipt.Readiness, newID)
	}

	// A deleted consumer is reconciled too.
	gone := loadJourney(t)
	delete(gone, fileG04)
	rc, err = Recheck(prior, snap(t, gone, Limits{}))
	if err != nil {
		t.Fatal(err)
	}
	if f := findingOf(rc.Findings, "subject:"+idG04); f == nil || f.Class != FindingReconcile {
		t.Errorf("findings = %+v, want a reconcile for the deleted consumer", rc.Findings)
	}
}

// regression: statusgen/20 Verify row 4 — an unreadable source reported as
// empty reads as complete coverage with no consumers in it.
func TestContextMissingInputScoped(t *testing.T) {
	base := loadJourney(t)
	prior := discover(t, snap(t, base, Limits{}))

	// A name that exists only inside repo-b, and an over-limit file that makes
	// repo-b unreadable for this run.
	const private = "payroll-withheld-name"
	m := loadJourney(t)
	m["sources/repo-b/docs/streams/beta/brief-09-"+private+".md"] = &fstest.MapFile{Data: bytes.Repeat([]byte("ledger-export "+private+"\n"), 300)}
	lim := Limits{MaxFiles: 100, MaxFileBytes: 4096, MaxTotalBytes: 1 << 20, MaxResults: 100}
	s := snap(t, m, lim)
	h := discover(t, s)

	cov := h.Discovery.Coverage
	if cov.State != "partial" || len(cov.Unavailable) != 1 || cov.Unavailable[0].Source != "repo-b" || contains(cov.Enumerated, "repo-b") {
		t.Fatalf("coverage = %+v, want repo-b unavailable and nothing else", cov)
	}
	raw := string(marshal(t, h))
	for _, leak := range []string{private, "gamma", "brief-02-export-consumer", "brief-03-cache-warmup", "billing"} {
		if strings.Contains(raw, leak) {
			t.Errorf("receipt names %q from inside the unavailable source", leak)
		}
	}
	if regexp.MustCompile(`[0-9]`).MatchString(cov.Unavailable[0].Reason) {
		t.Errorf("unavailable reason %q carries a count", cov.Unavailable[0].Reason)
	}
	// The declared edge is a snapshot declaration, not repo-b content: it
	// stays, as could-not-check.
	if im := impactOf(h, idB01); im == nil || im.Lifecycle != "unknown" || !strings.Contains(im.Reason, "could-not-check") {
		t.Errorf("declared owner in the unavailable source = %+v, want could-not-check", im)
	}
	if !contains(h.Readiness.Prerequisites, "hold: source repo-b unavailable: could-not-check, scoped to this source") || h.Readiness.Facts.SourceAvailable {
		t.Errorf("readiness = %+v, want a hold scoped to repo-b", h.Readiness)
	}

	// Unrelated authorized work continues: repo-a conclusions stand, and an
	// amendment in repo-a still checks while one in repo-b holds.
	if im := impactOf(h, idA01); im == nil || im.Disposition != "retain" {
		t.Errorf("repo-a routing lost: %+v", im)
	}
	cand, p := candPatch(t, m, fileA02, "Build the reconciliation report.\n", "Build the reconciliation report.\nAdd a layout-version column.\n")
	rep := check(t, amend(t, h, idA02, "Report per layout version.", cand), nil, map[string][]byte{cand.Digest: p}, s)
	if v := verdictOf(t, rep, "ob-1"); v.Outcome != OutcomePass || rep.Outcome != OutcomeHold {
		t.Errorf("repo-a amendment = %+v, report %s; want pass under a scoped hold", v, rep.Outcome)
	}
	rep = check(t, amend(t, h, idB01, "Own the v2 layout.", cand), nil, map[string][]byte{cand.Digest: p}, s)
	if v := verdictOf(t, rep, "ob-1"); v.Outcome != OutcomeHold {
		t.Errorf("repo-b amendment = %+v, want hold (could-not-check)", v)
	}

	// Recheck of a receipt taken while repo-b was readable: a scoped hold; the
	// prior repo-b conclusions are kept unconfirmed, not dropped.
	rc, err := Recheck(prior, s)
	if err != nil {
		t.Fatal(err)
	}
	if f := findingOf(rc.Findings, "source:repo-b"); f == nil || f.Class != FindingHold || rc.Reconcile() {
		t.Errorf("findings = %+v, want only a hold scoped to repo-b", rc.Findings)
	}
	for _, id := range []string{idB02, idG04} {
		if im := impactOf(rc.Receipt, id); im == nil || im.Disposition != "unresolved" || !strings.HasPrefix(im.Reason, "hold:") {
			t.Errorf("prior conclusion on %s = %+v, want kept as a hold", id, im)
		}
	}
}

// regression: a query truncated at the result limit makes coverage partial,
// yet a recheck counted only unavailable sources as holds, so a truncated
// receipt read as current and check passed it. Change detection on an
// impacted brief also went through the read-set map, so a brief whose entry
// the truncation cut was compared as unchanged (or, cut on one side only, as
// changed).
func TestContextRecheckTruncatedCoverageHolds(t *testing.T) {
	const fileB02 = "sources/repo-b/docs/streams/beta/brief-02-export-consumer.md"
	filler := func(m fstest.MapFS) {
		for i := 1; i <= 2100; i++ {
			m[fmt.Sprintf("sources/repo-a/aaa/f%04d.md", i)] = &fstest.MapFile{Data: []byte(fmt.Sprintf("# filler %04d\n", i))}
		}
	}
	variant := func(fill, edit bool) *Snapshot {
		m := loadJourney(t)
		if fill {
			filler(m)
		}
		if edit {
			old := m[fileB02].Data
			m[fileB02] = &fstest.MapFile{Data: append(append([]byte(nil), old...), "\nAn added sentence that changes what the consumer must do.\n"...)}
		}
		return snap(t, m, Limits{})
	}
	classOf := func(fs []Finding, scope, class string) *Finding {
		for i := range fs {
			if fs[i].Scope == scope && fs[i].Class == class {
				return &fs[i]
			}
		}
		return nil
	}
	// notable drops the preserved file findings the filler adds, so a failure
	// message names only what decides the outcome.
	notable := func(fs []Finding) []Finding {
		var out []Finding
		for _, f := range fs {
			if f.Class != FindingPreserved {
				out = append(out, f)
			}
		}
		return out
	}
	readSetTruncated := func(h *Handoff) bool {
		for _, q := range h.Discovery.Queries {
			if q.Rule == QueryReadSet {
				return q.Truncated
			}
		}
		return false
	}

	// The prior receipt is itself truncated: the impacted beta:02 brief is cut
	// from its read set, so its revision cannot be compared. Edited or not,
	// recheck and check hold; neither reads as current or passes.
	heldCand, heldP := candPatch(t, loadJourney(t), fileA02, "Build the reconciliation report.\n", "Build the reconciliation report.\nAdd a layout-version column.\n")
	prior := discover(t, variant(true, false))
	if prior.Discovery.Coverage.State != "partial" || !readSetTruncated(prior) {
		t.Fatalf("fixture: prior coverage %q, read set truncated %v; want partial and truncated", prior.Discovery.Coverage.State, readSetTruncated(prior))
	}
	for _, edit := range []bool{false, true} {
		s := variant(true, edit)
		rc, err := Recheck(prior, s)
		if err != nil {
			t.Fatal(err)
		}
		if !rc.Hold() || classOf(rc.Findings, "coverage:prior:"+QueryReadSet, FindingHold) == nil || classOf(rc.Findings, "coverage:snapshot:"+QueryReadSet, FindingHold) == nil {
			t.Errorf("truncated prior and snapshot (edit %v): findings %+v, want holds scoped to each truncated read set", edit, notable(rc.Findings))
		}
		if classOf(rc.Findings, "subject:"+idB02, FindingHold) == nil {
			t.Errorf("truncated prior (edit %v): findings %+v, want a hold on %s, whose prior revision the truncation cut", edit, notable(rc.Findings), idB02)
		}
		// A hold is not confirmation: the receipt is not current, yet it is
		// not stale either, so the outcome stays a hold and an obligation
		// that binds and applies is not refused as a stale receipt.
		if rep := check(t, prior, nil, nil, s); rep.Outcome != OutcomeHold || rep.Current {
			t.Errorf("check over partial coverage (edit %v) = %s, current %v; want hold, current false", edit, rep.Outcome, rep.Current)
		}
		rep := check(t, amend(t, prior, idA02, "Report per layout version.", heldCand), nil, map[string][]byte{heldCand.Digest: heldP}, s)
		if v := verdictOf(t, rep, "ob-1"); rep.Outcome != OutcomeHold || rep.Current || v.Rule == CheckStaleReceipt {
			t.Errorf("check of an amendment over partial coverage (edit %v) = %s, current %v, verdict %+v; want hold, current false, no stale-receipt refusal", edit, rep.Outcome, rep.Current, v)
		}
	}

	// A complete prior against a truncated snapshot: the impacted briefs cut
	// from the new read set are compared at their own revision, so the edited
	// brief alone reconciles and the unchanged ones do not, and the partial
	// snapshot still holds.
	full := discover(t, variant(false, false))
	for _, edit := range []bool{false, true} {
		rc, err := Recheck(full, variant(true, edit))
		if err != nil {
			t.Fatal(err)
		}
		if !rc.Hold() || classOf(rc.Findings, "coverage:snapshot:"+QueryReadSet, FindingHold) == nil {
			t.Errorf("truncated snapshot (edit %v): findings %+v, want a hold on the truncated read set", edit, notable(rc.Findings))
		}
		for _, im := range full.Impacts {
			f := classOf(rc.Findings, "subject:"+im.Subject, FindingReconcile)
			if want := edit && im.Subject == idB02; (f != nil) != want {
				t.Errorf("truncated snapshot (edit %v): reconcile on %s = %+v, want %v", edit, im.Subject, f, want)
			} else if want && !strings.Contains(f.Reason, "the brief changed since the prior receipt") {
				t.Errorf("reconcile on %s = %q, want the brief-changed reason", im.Subject, f.Reason)
			}
		}
	}

	// Control: the same edit with complete coverage on both sides reconciles
	// the edited brief and holds nothing.
	rc, err := Recheck(full, variant(false, true))
	if err != nil {
		t.Fatal(err)
	}
	if f := classOf(rc.Findings, "subject:"+idB02, FindingReconcile); f == nil || rc.Hold() || !strings.Contains(f.Reason, "the brief changed since the prior receipt") {
		t.Errorf("control: findings %+v, want only a reconcile on %s", notable(rc.Findings), idB02)
	}
	// A reconcile makes the receipt stale: not current, and each obligation
	// is refused as a stale receipt, with or without partial coverage too.
	for _, fill := range []bool{false, true} {
		rep := check(t, amend(t, full, idA02, "Report per layout version.", heldCand), nil, map[string][]byte{heldCand.Digest: heldP}, variant(fill, true))
		if v := verdictOf(t, rep, "ob-1"); rep.Outcome != OutcomeRefuse || rep.Current || v.Outcome != OutcomeRefuse || v.Rule != CheckStaleReceipt {
			t.Errorf("check after a reconcile (truncated snapshot %v) = %s, current %v, verdict %+v; want refuse, current false, refuse %s", fill, rep.Outcome, rep.Current, v, CheckStaleReceipt)
		}
	}
}

// regression: statusgen/20 Verify row 5 — checking only that a patch applies
// lets a clean patch rewrite populated Evidence or a dated source account.
func TestContextAmendmentPreservesDatedSources(t *testing.T) {
	m := loadJourney(t)
	s := snap(t, m, Limits{})
	h := discover(t, s)
	if im := impactOf(h, idA02); im == nil || im.Lifecycle != "todo" || im.Evidence == nil {
		t.Fatalf("fixture: %s must be unfinished with populated Evidence: %+v", idA02, im)
	}

	stale := loadJourney(t)
	replaceIn(t, stale, fileA02, "Build the reconciliation report.", "Build the monthly reconciliation report.")
	staleCand, staleP := candPatch(t, stale, fileA02, "## Task\n", "## Task\n\nAdd a layout-version column.\n")

	cases := []struct {
		name, old, new, rule string
	}{
		{"additive task line", "Build the reconciliation report.\n", "Build the reconciliation report.\nAdd a layout-version column.\n", ""},
		{"appended evidence row", "| 1 | partial run recorded | 2026-10-03 |\n", "| 1 | partial run recorded | 2026-10-03 |\n| 2 | full run recorded | 2026-10-20 |\n", ""},
		{"rewritten evidence row", "| 1 | partial run recorded |", "| 1 | full run recorded |", CheckHistoryRewrite},
		{"rewritten dated account", "asked for a daily cut.", "asked for a weekly cut.", CheckHistoryRewrite},
		{"insert inside dated account", "## Source account — 2026-10-02\n\n", "## Source account — 2026-10-02\n\nSuperseded.\n\n", CheckHistoryRewrite},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cand, p := candPatch(t, m, fileA02, c.old, c.new) // applies cleanly, or fails the test here
			v := verdictOf(t, check(t, amend(t, h, idA02, "Report per layout version.", cand), nil, map[string][]byte{cand.Digest: p}, s), "ob-1")
			if c.rule == "" && v.Outcome != OutcomePass {
				t.Errorf("verdict = %+v, want pass", v)
			}
			if c.rule != "" && (v.Outcome != OutcomeRefuse || v.Rule != c.rule) {
				t.Errorf("verdict = %+v, want refuse %s", v, c.rule)
			}
		})
	}

	// A patch bound to an older base is refused even before it is applied.
	v := verdictOf(t, check(t, amend(t, h, idA02, "Report per layout version.", staleCand), nil, map[string][]byte{staleCand.Digest: staleP}, s), "ob-1")
	if v.Outcome != OutcomeRefuse || v.Rule != CheckPatchApply {
		t.Errorf("stale-base verdict = %+v, want refuse %s", v, CheckPatchApply)
	}
	// A patch whose context does not match is refused.
	cand, p := candPatch(t, m, fileA02, "Build the reconciliation report.\n", "Build the reconciliation report.\nAdd a layout-version column.\n")
	bad := bytes.Replace(p, []byte(" ## Task\n"), []byte(" ## Tasks\n"), 1)
	cand.Digest = PatchDigest(bad)
	v = verdictOf(t, check(t, amend(t, h, idA02, "Report per layout version.", cand), nil, map[string][]byte{cand.Digest: bad}, s), "ob-1")
	if v.Outcome != OutcomeRefuse || v.Rule != CheckPatchApply {
		t.Errorf("mismatched-context verdict = %+v, want refuse %s", v, CheckPatchApply)
	}
}

// regression: statusgen/20 Verify row 6 — ignoring claims lets an amendment
// change a brief under an active worker without its owner's acknowledgement.
func TestContextActiveWorkerRefusesSilentRebind(t *testing.T) {
	m := loadJourney(t)
	s := snap(t, m, Limits{})
	h := discover(t, s)
	if s.Claims[0].BriefRevision == BlobRevision(m[fileA04].Data)[:7] {
		t.Fatal("fixture: the claim must be at an older brief revision")
	}
	cand, p := candPatch(t, m, fileA04, "Read the nightly file into the warehouse.\n", "Read the nightly file into the warehouse.\nAccept both layouts during the transition.\n")
	patches := map[string][]byte{cand.Digest: p}
	amended := amend(t, h, idA04, "Read both layouts.", cand)

	v := verdictOf(t, check(t, amended, nil, patches, s), "ob-1")
	if v.Outcome != OutcomeRefuse || v.Rule != CheckSilentRebind {
		t.Errorf("unacknowledged claim: verdict = %+v, want refuse %s", v, CheckSilentRebind)
	}
	for _, tr := range []struct {
		treatment string
		ack       bool
	}{{"adjust", true}, {"cancel", false}} {
		recorded := reshape(t, amended, func(c *Handoff) {
			c.ActiveWork[0].Treatment, c.ActiveWork[0].Acknowledged = tr.treatment, tr.ack
		})
		if v := verdictOf(t, check(t, recorded, nil, patches, s), "ob-1"); v.Outcome != OutcomePass {
			t.Errorf("%s recorded: verdict = %+v, want pass", tr.treatment, v)
		}
	}
	// Dropping the claim from the receipt does not get past it: the claim is
	// no longer bound, so the receipt is stale.
	dropped := reshape(t, amended, func(c *Handoff) { c.ActiveWork = []ActiveWork{} })
	if v := verdictOf(t, check(t, dropped, nil, patches, s), "ob-1"); v.Outcome != OutcomeRefuse || v.Rule != CheckStaleReceipt {
		t.Errorf("claim dropped from the receipt: verdict = %+v, want refuse %s", v, CheckStaleReceipt)
	}
}

// regression: statusgen/20 Verify row 7 — accepting any textually clean merge
// lets two approved deltas that contradict each other both land.
func TestContextCompetingProposals(t *testing.T) {
	m := loadJourney(t)
	r0 := discover(t, snap(t, m, Limits{}))
	ourCand, ourP := candPatch(t, m, fileA02, "Build the reconciliation report.\n", "Build the reconciliation report.\nShow v1 and v2 side by side.\n")
	rivalCand, rivalP := candPatch(t, m, fileA02, "| 1 | `make report` | exit 0 |\n", "| 1 | `make report` | exit 0 |\n| 2 | `make report-v2` | exit 0 |\n")
	// The two patches touch disjoint lines: textually they merge cleanly.
	if bytes.Contains(ourP, []byte("make report")) || bytes.Contains(rivalP, []byte("side by side")) {
		t.Fatal("fixture: the rival patches must not overlap")
	}
	approved := amend(t, r0, idA02, "Report only the v2 layout.", rivalCand)
	draft := reshape(t, amend(t, r0, idA02, "Report only the v1 layout.", rivalCand), func(c *Handoff) {
		c.Target.State, c.Target.Approval = TargetProposed, nil
		c.Readiness.Facts.ProposalApproved, c.Readiness.NextAct = false, "explore"
	})

	run := func(t *testing.T, rivals map[string]*Handoff, after string) (Verdict, *Report) {
		t.Helper()
		mm := loadJourney(t)
		var paths []any
		for _, name := range sortedKeys(rivals) {
			path := "proposals/" + name + ".json"
			mm[path] = &fstest.MapFile{Data: marshal(t, rivals[name])}
			paths = append(paths, path)
		}
		editSnapshot(t, mm, func(v map[string]any) { v["proposals"] = paths })
		s := snap(t, mm, Limits{})
		ours := amend(t, discover(t, s), idA02, after, ourCand)
		rep := check(t, ours, nil, map[string][]byte{ourCand.Digest: ourP, rivalCand.Digest: rivalP}, s)
		return verdictOf(t, rep, "ob-1"), rep
	}

	if v, _ := run(t, map[string]*Handoff{"approved": approved, "draft": draft}, "Report v1 and v2 side by side."); v.Outcome != OutcomeRefuse || v.Rule != CheckIncompatibleDelta {
		t.Errorf("incompatible approved rival: verdict = %+v, want refuse %s", v, CheckIncompatibleDelta)
	}
	v, rep := run(t, map[string]*Handoff{"draft": draft}, "Report v1 and v2 side by side.")
	if v.Outcome != OutcomePass {
		t.Errorf("unapproved rival only: verdict = %+v, want pass", v)
	}
	if f := findingOf(rep.Findings, "proposal:proposals/draft.json"); f == nil || f.Class != FindingIgnored {
		t.Errorf("findings = %+v, want the unapproved rival ignored", rep.Findings)
	}
	if v, _ := run(t, map[string]*Handoff{"approved": approved}, "Report only the v2 layout."); v.Outcome != OutcomePass {
		t.Errorf("compatible approved rival: verdict = %+v, want pass", v)
	}
}

// regression: statusgen/20 Verify row 8 — patching a delivered brief's Task
// edits closed history in place instead of routing the new obligation.
func TestContextDeliveredBriefRoutesFollowOn(t *testing.T) {
	m := loadJourney(t)
	s := snap(t, m, Limits{})
	h := discover(t, s)

	// A clean patch to the delivered brief's Task, outside every protected
	// region, is refused: the obligation routes to the linked follow-on.
	cand, p := candPatch(t, m, fileA01, "Write the ledger-export v1 file every night.\n", "Write the ledger-export v1 file every night.\nAlso write the v2 layout.\n")
	v := verdictOf(t, check(t, amend(t, h, idA01, "Write both layouts.", cand), nil, map[string][]byte{cand.Digest: p}, s), "ob-1")
	if v.Outcome != OutcomeRefuse || v.Rule != RuleAmendUnbound || !strings.Contains(v.Reason, idA03+" (linked follow-on)") {
		t.Errorf("patch on a delivered brief: verdict = %+v, want refuse %s naming %s", v, RuleAmendUnbound, idA03)
	}

	// Bound to the follow-on instead, it passes and leaves the brief unchanged.
	follow := func(to string) Verdict {
		amended := amend(t, h, idA01, "Write both layouts.", nil)
		mf := &Manifest{Schema: ManifestSchema, DeclaredBy: amended.DeclaredBy, Handoff: amended.Digest(), Entries: []Entry{{
			Obligation: "ob-1", Brief: idA01, Action: "amend",
			DeliveryPaths: []string{"docs/streams/alpha/brief-03-export-layout-v2.md"},
			Depends:       []string{}, TestCommands: []string{}, ActiveWork: "none observed", Effort: "S",
			Binding: Binding{FollowUp: &to},
		}}}
		mf, err := DecodeManifest(marshal(t, mf))
		if err != nil {
			t.Fatalf("test manifest: %v", err)
		}
		return verdictOf(t, check(t, amended, mf, nil, s), "ob-1")
	}
	if v := follow(idA03); v.Outcome != OutcomePass || !strings.Contains(v.Reason, "left unchanged") {
		t.Errorf("follow-on binding: verdict = %+v, want pass", v)
	}
	if v := follow(idB03); v.Outcome != OutcomeRefuse || v.Rule != RuleAmendUnbound {
		t.Errorf("binding to a done brief: verdict = %+v, want refuse %s", v, RuleAmendUnbound)
	}
	// An unfinished brief that is neither linked to the delivered brief nor
	// an owner cannot take its obligation; the unfinished owner can.
	if v := follow(idA05); v.Outcome != OutcomeRefuse || v.Rule != RuleAmendUnbound || !strings.Contains(v.Reason, "not linked") {
		t.Errorf("binding to an unlinked unfinished brief: verdict = %+v, want refuse %s", v, RuleAmendUnbound)
	}
	if v := follow(idB01); v.Outcome != OutcomePass || !strings.Contains(v.Reason, "unfinished owner") {
		t.Errorf("binding to the unfinished owner: verdict = %+v, want pass", v)
	}

	// With no linked follow-on, the obligation routes to the unfinished owner;
	// with neither, a new follow-on brief is needed.
	noLink := loadJourney(t)
	replaceIn(t, noLink, fileA03, `depends: ["alpha/01"]`, `depends: []`)
	replaceIn(t, noLink, fileA04, `depends: ["alpha/01", "alpha/03"]`, `depends: ["alpha/03"]`)
	if im := impactOf(discover(t, snap(t, noLink, Limits{})), idA01); im == nil || im.Disposition != "retain" || !strings.Contains(im.Reason, idB01+" (unfinished owner)") {
		t.Errorf("no linked follow-on: impact = %+v, want routed to the unfinished owner %s", im, idB01)
	}
	editSnapshot(t, noLink, func(v map[string]any) { v["edges"] = []any{} })
	if im := impactOf(discover(t, snap(t, noLink, Limits{})), idA01); im == nil || im.Disposition != "unresolved" || !strings.Contains(im.Reason, "new follow-on brief") {
		t.Errorf("no route: impact = %+v, want unresolved, needing a new follow-on", im)
	}
}

// Two brief files with one identity: the first in path order holds it, and
// the collision is named on the receipt instead of resolving silently.
func TestContextDuplicateBriefIDVisible(t *testing.T) {
	m := loadJourney(t)
	if h := discover(t, snap(t, m, Limits{})); len(h.Readiness.Prerequisites) != 1 {
		t.Fatalf("fixture: prerequisites %v, want only the architecture line", h.Readiness.Prerequisites)
	}
	m["sources/repo-b/docs/streams/beta/brief-02-zz-copy.md"] = &fstest.MapFile{Data: []byte("# Brief 02 — Copy\n")}
	h := discover(t, snap(t, m, Limits{}))
	want := "duplicate: brief id " + idB02 + " is held by repo-b:docs/streams/beta/brief-02-export-consumer.md; repo-b:docs/streams/beta/brief-02-zz-copy.md is ignored"
	if !contains(h.Readiness.Prerequisites, want) {
		t.Errorf("prerequisites %v, want %q", h.Readiness.Prerequisites, want)
	}
}

// The patch reader is strict: anything but one file's exact hunks is refused.
func TestContextPatchStrict(t *testing.T) {
	old := []byte("a\nb\nc\n")
	good := []byte("--- a/f.md\n+++ b/f.md\n@@ -1,3 +1,4 @@\n a\n+x\n b\n c\n")
	fp, err := parsePatch(good)
	if err != nil {
		t.Fatal(err)
	}
	res, err := fp.apply(old)
	if err != nil || string(res.content) != "a\nx\nb\nc\n" || fmt.Sprint(res.inserted) != "[1]" {
		t.Fatalf("apply = %+v, %v", res, err)
	}
	for name, p := range map[string]string{
		"two files":     string(good) + "--- a/g.md\n+++ b/g.md\n@@ -1 +1 @@\n-a\n+b\n",
		"short hunk":    "--- a/f.md\n+++ b/f.md\n@@ -1,3 +1,4 @@\n a\n+x\n b\n",
		"no prefix":     "--- f.md\n+++ f.md\n@@ -1 +1 @@\n-a\n+b\n",
		"stray header":  "From: someone\n" + string(good),
		"no hunks":      "--- a/f.md\n+++ b/f.md\n",
		"no final line": strings.TrimSuffix(string(good), "\n"),
	} {
		if _, err := parsePatch([]byte(p)); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	for name, p := range map[string]string{
		"context mismatch": "--- a/f.md\n+++ b/f.md\n@@ -1,2 +1,2 @@\n a\n-c\n+d\n",
		"create":           "--- /dev/null\n+++ b/f.md\n@@ -0,0 +1 @@\n+a\n",
		"rename":           "--- a/f.md\n+++ b/g.md\n@@ -1 +1 @@\n-a\n+b\n",
		"offset":           "--- a/f.md\n+++ b/f.md\n@@ -2,1 +2,1 @@\n-a\n+z\n",
	} {
		fp, err := parsePatch([]byte(p))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := fp.apply(old); err == nil {
			t.Errorf("%s: applied", name)
		}
	}
	keys := sortedKeys(map[string]int{"b": 1, "a": 2})
	if !sort.StringsAreSorted(keys) {
		t.Fatal(keys)
	}
}
