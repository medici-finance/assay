package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const trackingFixtureRel = "docs/streams/fx/brief-01-fixture.md"

// Separate object stores reproduce a tracking brief whose run SHA exists only
// in the deliverable repository. The tracking snapshot is v2, independently
// committed before the new verifier Evidence; v1 is retained for stale tests.
//
// A mode ending in sourcePresent also fetches a deliverable commit carrying a
// v2 brief at the same path into the tracking store, so a read that falls back
// to the source revision succeeds instead of failing for a missing object.
func foldTrackingFixture(t *testing.T, mode string) (reconcileFoldData, string, string) {
	t.Helper()
	foreign := mode == "missing tracking but source present" || strings.HasSuffix(mode, sourcePresent)
	mode = strings.TrimSuffix(mode, sourcePresent)
	source := t.TempDir()
	foldGit(t, source, foldMailWorker, "init", "-q")
	if err := os.WriteFile(filepath.Join(source, "implementation.txt"), []byte("source repository\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if foreign {
		foreignPath := filepath.Join(source, filepath.FromSlash(trackingFixtureRel))
		if err := os.MkdirAll(filepath.Dir(foreignPath), 0700); err != nil {
			t.Fatal(err)
		}
		foreignBrief := foldFixtureBrief{num: "01", id: "fx/01", gate: "model"}
		if err := os.WriteFile(foreignPath, []byte(foldBriefFile(foreignBrief, 2, "")), 0600); err != nil {
			t.Fatal(err)
		}
		foldGit(t, source, foldMailWorker, "add", trackingFixtureRel)
	}
	foldGit(t, source, foldMailWorker, "add", "implementation.txt")
	foldGit(t, source, foldMailWorker, "commit", "-qm", "implementation")
	sourceSHA := foldTrackingGitLine(t, source, "rev-parse", "HEAD")
	b := foldFixtureBrief{num: "01", id: "fx/01", gate: "model", ev: foldPassEv, version: 2}
	root := writeFoldFixture(t, []foldFixtureBrief{b}, map[string][3]string{"01": {"implemented", "—", "—"}})
	foldGit(t, root, foldMailWorker, "remote", "add", "origin", "https://github.com/example-org/tracker.git")
	trackingSHA := foldTrackingGitLine(t, root, "rev-parse", "HEAD")
	if mode == "stale" {
		trackingSHA = foldTrackingGitLine(t, root, "rev-parse", "HEAD^")
	}
	if mode == "missing history" || mode == "missing tracking but source present" {
		trackingSHA = strings.Repeat("f", 40)
	}
	if foreign {
		foldGit(t, root, foldMailWorker, "fetch", "-q", "--no-tags", source, sourceSHA)
	}
	att := fmt.Sprintf("Verification-Attestation: example-org/tracker#7 run=%s source-repo=example-org/agents source=%s tracking-source=%s brief=%s model=fixture-model tier=strong", strings.Repeat("a", 64), sourceSHA, trackingSHA, trackingFixtureRel)
	switch mode {
	case "legacy", "legacy note":
		sourceSHA = trackingSHA
		att = fmt.Sprintf("Verification-Attestation: example-org/tracker#7 run=%s source=%s brief=%s model=fixture-model tier=strong", strings.Repeat("a", 64), sourceSHA, trackingFixtureRel)
	case "mixed":
		att += "\n" + fmt.Sprintf("Verification-Attestation: example-org/tracker#7 run=%s source=%s brief=%s model=fixture-model tier=strong", strings.Repeat("a", 64), sourceSHA, trackingFixtureRel)
	case "inline prefix":
		att = "`ignored`" + att
	case "missing origin":
		foldGit(t, root, foldMailWorker, "remote", "remove", "origin")
	case "short run":
		att = strings.Replace(att, "run="+strings.Repeat("a", 64), "run=bad", 1)
	case "wrong repo":
		att = strings.Replace(att, "example-org/tracker#7", "example-org/other#7", 1)
	case "wrong path":
		att = strings.Replace(att, "brief="+trackingFixtureRel, "brief=docs/streams/fx/brief-02-other.md", 1)
	case "wrong source":
		att = strings.Replace(att, "source="+sourceSHA, "source="+strings.Repeat("d", 40), 1)
	case "short tracking":
		att = strings.Replace(att, "tracking-source="+trackingSHA, "tracking-source="+trackingSHA[:12], 1)
	case "malformed":
		att = strings.Replace(att, "tracking-source="+trackingSHA, "tracking-source=HEAD", 1)
	case "duplicate":
		att += "\n" + att
	case "conflicting":
		att += "\n" + strings.Replace(att, "tracking-source="+trackingSHA, "tracking-source="+foldTrackingGitLine(t, root, "rev-parse", "HEAD^"), 1)
	case "fenced":
		att = "```text\n" + att + "\n```"
	case "blockquoted":
		att = "> " + att
	case "indented":
		att = "  " + att
	case "unparseable":
		att = strings.Replace(att, " tier=strong", "", 1)
	case "tracking-source only":
		att = strings.Replace(att, " source-repo=example-org/agents", "", 1)
	case "no tracking-source":
		att = strings.Replace(att, " tracking-source="+trackingSHA, "", 1)
	case "non-github origin":
		foldGit(t, root, foldMailWorker, "remote", "set-url", "origin", "https://gitlab.example/example-org/tracker.git")
	case "quoted v1 beside v2":
		att += "\n`quoted`" + fmt.Sprintf("Verification-Attestation: example-org/tracker#7 run=%s source=%s brief=%s model=fixture-model tier=strong", strings.Repeat("a", 64), sourceSHA, trackingFixtureRel)
	case "foreign homed source missing":
		att = ""
	}
	if mode == "legacy note" {
		att = "Verification-Attestation: unavailable on this legacy host"
	}
	ev := strings.ReplaceAll(foldPassEv, "{SHA}", sourceSHA[:12])
	ev = strings.Replace(ev, "**VERIFY: PASS**", att+"\n\n**VERIFY: PASS**", 1)
	if mode == "prior run only" {
		ev = att + "\n" + strings.ReplaceAll(foldRedEv, "{SHA}", sourceSHA[:12]) + "\n" + strings.ReplaceAll(foldPassEv, "{SHA}", sourceSHA[:12])
	}
	path := filepath.Join(root, filepath.FromSlash(trackingFixtureRel))
	if err := os.WriteFile(path, []byte(foldBriefFile(b, 2, ev)), 0600); err != nil {
		t.Fatal(err)
	}
	foldGit(t, root, foldMailVerifier, "add", trackingFixtureRel)
	foldGit(t, root, foldMailVerifier, "commit", "-qm", "independent cross repository evidence")
	data, _, err := loadReconcileFoldData(root)
	if err != nil || len(data) != 1 {
		t.Fatalf("data=%d err=%v", len(data), err)
	}
	return data[0], sourceSHA, trackingSHA
}

// sourcePresent marks a fixture mode whose deliverable object is local.
const sourcePresent = " (source present)"

// foldTrackingGitLine returns one fixture Git line; it never contacts a forge.
func foldTrackingGitLine(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := coverageGit(root, args...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// regression: #5475
func TestFoldTrackingRevision(t *testing.T) {
	for _, mode := range []string{"current", "duplicate", "stale", "legacy", "legacy note", "quoted v1 beside v2"} {
		t.Run(mode, func(t *testing.T) {
			d, sourceSHA, _ := foldTrackingFixture(t, mode)
			w, ok, why := foldVerified(d, map[string]Coverage{d.key: {Released: true}})
			wantVersion := 2
			if mode == "stale" {
				wantVersion = 1
			}
			if !ok || why != "" || w.Version != wantVersion || !strings.Contains(w.Run, sourceSHA[:12]) {
				t.Fatalf("w=%+v ok=%v why=%s; want version=%d and deliverable identity", w, ok, why, wantVersion)
			}
			cells := DeriveLifecycle(LifecycleInput{Briefs: []BriefIdent{d.ident}, PRs: []PRRecord{{BriefRef: d.ident.ID, Number: 1, State: prMerged}}, Witnesses: map[string]WitnessInfo{d.ident.ID: w}, LookedAt: true})
			want := "verified"
			if mode == "stale" {
				want = "unknown"
			}
			if len(cells) != 1 || cells[0].Cell != want {
				t.Fatalf("cells=%+v want %s", cells, want)
			}
		})
	}
}

// regression: #5475
func TestFoldTrackingRevisionRefusals(t *testing.T) {
	for _, mode := range []string{"missing history", "missing tracking but source present", "mixed", "inline prefix", "missing origin", "short run", "wrong repo", "wrong path", "wrong source", "short tracking", "malformed", "conflicting", "fenced", "prior run only", "foreign homed source missing"} {
		t.Run(mode, func(t *testing.T) {
			d, _, _ := foldTrackingFixture(t, mode)
			w, ok, why := foldVerified(d, map[string]Coverage{d.key: {Released: true}})
			if ok || w.Passed || !strings.Contains(why, "could-not-check the brief version") {
				t.Fatalf("w=%+v ok=%v why=%q", w, ok, why)
			}
		})
	}
}

// regression: #5475 — each case fetches the deliverable commit, with a v2
// brief at the same path, into the tracking store. A selector that skips the
// cross-repository line and reads at the source revision would then pass, so
// these cases fail on the selector itself, not on a missing object. The
// reason names which check refused, so a different refusal does not mask a
// skipped one.
func TestFoldTrackingRevisionRefusalsSourcePresent(t *testing.T) {
	for mode, reason := range map[string]string{
		"fenced":               "not bound by literal selected-run metadata",
		"prior run only":       "not bound by literal selected-run metadata",
		"blockquoted":          "not bound by literal selected-run metadata",
		"indented":             "not bound by literal selected-run metadata",
		"unparseable":          "malformed cross-repository run attestation",
		"tracking-source only": "malformed cross-repository run attestation",
		"no tracking-source":   "it names no tracking-source=",
		"non-github origin":    "needs a github.com origin",
	} {
		t.Run(mode, func(t *testing.T) {
			d, _, _ := foldTrackingFixture(t, mode+sourcePresent)
			w, ok, why := foldVerified(d, map[string]Coverage{d.key: {Released: true}})
			if ok || w.Passed || !strings.Contains(why, "could-not-check the brief version") || !strings.Contains(why, reason) {
				t.Fatalf("w=%+v ok=%v why=%q; want could-not-check naming %q", w, ok, why, reason)
			}
		})
	}
}

// regression: #5497 — the whole reconcile verb over a cross-repository PASS
// whose source= exists only in the deliverable repository: every read is made
// (so --apply does not exit could-not-check) and the witness carries the brief
// version read at tracking-source=. The same-repo (legacy) row runs through
// the same path unchanged and still derives verified. Evidence coverage is a
// separate read and is not asserted for the cross-repository row here.
func TestReconcileCrossRepoPass(t *testing.T) {
	for _, mode := range []string{"current", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			d, _, _ := foldTrackingFixture(t, mode)
			root := strings.TrimSuffix(d.path, filepath.FromSlash(trackingFixtureRel))
			foldServer(t, foldStdPulls()[:1], foldIssues())
			src := foldApproved()
			src.states[11] = prReviewState{Merged: true, HeadSHA: foldPR11Head}
			foldFlipSeam(t, src)
			code, res, e := foldRun(t, root)
			if code != reconcileOK || len(res.Unread) != 0 {
				t.Fatalf("exit %d unread=%q; want 0 and every read made; stderr:\n%s", code, res.Unread, e)
			}
			var cell BriefCell
			for _, c := range res.Briefs {
				if c.ID == "fx/01" {
					cell = c
				}
			}
			if cell.Version != 2 || !strings.Contains(cell.Witness, "pass (v2)") {
				t.Fatalf("fx/01 = %+v; want a v2 passing witness", cell)
			}
			if mode == "legacy" && cell.Cell != "verified" {
				t.Fatalf("fx/01 = %+v; a same-repository row must still derive verified", cell)
			}
			if code, _, e := foldRun(t, root, "--apply"); code != reconcileOK {
				t.Fatalf("--apply exit %d, want 0; stderr:\n%s", code, e)
			}
		})
	}
}

// regression: #5497 — a run whose source= is a commit in another repository
// and whose selected run binds no attestation naming tracking-source= stays
// could-not-check, and the reason says why instead of a bare exit status.
func TestFoldCrossRepoNoBinding(t *testing.T) {
	d, sourceSHA, _ := foldTrackingFixture(t, "foreign homed source missing")
	w, ok, why := foldVerified(d, map[string]Coverage{d.key: {Released: true}})
	for _, want := range []string{"could-not-check the brief version for run source " + sourceSHA[:12], "fatal:", "this run binds none"} {
		if ok || w.Passed || !strings.Contains(why, want) {
			t.Fatalf("w=%+v ok=%v why=%q; want could-not-check naming %q", w, ok, why, want)
		}
	}
}
