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
func foldTrackingFixture(t *testing.T, mode string) (reconcileFoldData, string, string) {
	t.Helper()
	source := t.TempDir()
	foldGit(t, source, foldMailWorker, "init", "-q")
	if err := os.WriteFile(filepath.Join(source, "implementation.txt"), []byte("source repository\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if mode == "missing tracking but source present" {
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
	if mode == "missing tracking but source present" {
		foldGit(t, root, foldMailWorker, "fetch", "-q", "--no-tags", source, sourceSHA)
	}
	att := fmt.Sprintf("Verification-Attestation: example-org/tracker#7 run=%s source-repo=example-org/agents source=%s tracking-source=%s brief=%s model=fixture-model tier=strong", strings.Repeat("a", 64), sourceSHA, trackingSHA, trackingFixtureRel)
	switch mode {
	case "legacy":
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
	case "foreign homed source missing":
		att = ""
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
	for _, mode := range []string{"current", "duplicate", "stale", "legacy"} {
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
