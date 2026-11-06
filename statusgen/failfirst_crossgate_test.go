package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// ffCrossRepo builds a deliverable repository: older and base before the
// attested source, a later commit after it, and a sibling tree outside its
// history. It is the checkout a cross-repository run's Base cell names.
func ffCrossRepo(t *testing.T) (root, older, base, source, later, stray string) {
	t.Helper()
	root = t.TempDir()
	gitRun(t, root, "init", "-q")
	commit := func(content, msg string) string {
		writeTemp(t, root, "delivered.txt", content)
		gitRun(t, root, "add", "-A")
		gitRun(t, root, "commit", "-q", "-m", msg)
		return gitOutT(t, root, "rev-parse", "HEAD")
	}
	older = commit("older\n", "older")
	base = commit("base\n", "base")
	source = commit("new\n", "the attested source")
	later = commit("later\n", "a commit after the source")
	tree := gitOutT(t, root, "rev-parse", "HEAD^{tree}")
	stray = gitOutT(t, root, "commit-tree", tree, "-m", "a sibling tree outside this history")
	return root, older, base, source, later, stray
}

// ffCrossTracking builds the tracking clone the gate runs in: one commit that
// origin/main also names, so its merge-base resolves and only the deliverable
// commits are absent from it, as in a real cross-repository verify.
func ffCrossTracking(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	writeTemp(t, root, "tracking.txt", "tracking\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "tracking")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	return root
}

// ffCrossEvidence is one PASS run on a version-2 receipt: a fail-first
// witness table recorded against the deliverable source, then the literal
// attestation line, then the strict verdict. att rewrites the line.
func ffCrossEvidence(base, source string, att func(string) string) string {
	skip := failFirstCell("", baseNotSelected, -1, source[:12])
	mk := func(id, cmd, b string) witness {
		return witness{ID: id, Command: cmd, State: statePass, Exit: 0, OutHash: "0123456789ab",
			Date: "2026-10-08", Runner: "assay-verifier-app[bot] @ " + source[:12], Tree: source[:12], Base: b}
	}
	table := witnessTable([]witness{
		mk("1", "grep -q new flag.txt", failFirstCell(base[:12], baseRed, 1, source[:12])),
		mk("2", "test -f flag.txt", skip),
	})
	line := fmt.Sprintf("Verification-Attestation: example-org/one#7 run=%s source-repo=example-org/two source=%s tracking-source=%s brief=docs/streams/ff/brief-01-ff.md model=fixture-model tier=strong",
		strings.Repeat("a", 64), source, strings.Repeat("c", 40))
	if att != nil {
		line = att(line)
	}
	return table + "\n\n" + line + "\n\n" + vfPass + "\n"
}

// regression: #5549
// TestFFCrossRepoBase: a cross-repository run's fail-first base is a commit of
// the DELIVERABLE repository, so the closure gate resolves it in the
// deliverable checkout the operator configured for source-repo=, against the
// attested source. The source itself and its ancestors close; a later commit,
// a sibling tree and an unknown object each stay a PROBLEM. Before #5549 the
// gate looked the base up in the tracking clone, which never holds it, and
// refused every valid cross-repository closure.
func TestFFCrossRepoBase(t *testing.T) {
	withBase(t, true)
	deliverable, older, base, source, later, stray := ffCrossRepo(t)
	t.Setenv("DESK_ROOTS", "example-org/two="+deliverable)
	tracking := ffCrossTracking(t)
	for _, c := range []struct {
		name, base  string
		wantProblem bool
	}{
		{"the base the run chose", base, false},
		{"an older ancestor of the source", older, false},
		{"the attested source itself", source, false},
		{"a commit after the source", later, true},
		{"sibling tree outside this history", stray, true},
		{"object the deliverable lacks", strings.Repeat("d", 40), true},
	} {
		problems, _ := failFirstGateChecks(tracking, ffStream(t, tracking, ffCrossEvidence(c.base, source, nil)))
		got := containsSub(problems, "attested deliverable source")
		if got != c.wantProblem {
			t.Errorf("%s: base PROBLEM=%v, want %v; problems=%v", c.name, got, c.wantProblem, problems)
		}
		if !c.wantProblem && len(problems) != 0 {
			t.Errorf("%s: want a clean closure, got %v", c.name, problems)
		}
	}
}

// regression: #5549
// TestFFCrossRepoFailsClosed: the cross-repository arm never passes on what
// it cannot check. No configured deliverable checkout, an attestation that is
// not literal run metadata, and one whose source= is not the run's source each
// keep the closure blocked, and none falls back to the tracking-clone rule.
func TestFFCrossRepoFailsClosed(t *testing.T) {
	withBase(t, true)
	deliverable, _, base, source, _, _ := ffCrossRepo(t)
	tracking := ffCrossTracking(t)
	for _, c := range []struct {
		name, roots, want string
		att               func(string) string
	}{
		{"no configured deliverable checkout", "", "no configured checkout", nil},
		{"another repository configured", "example-org/three=" + deliverable, "no configured checkout", nil},
		{"attestation in a code span", "example-org/two=" + deliverable, "cannot be bound",
			func(l string) string { return "`" + l + "`" }},
		{"source= not the run's source", "example-org/two=" + deliverable, "cannot be bound",
			func(l string) string {
				return strings.Replace(l, "source="+source, "source="+strings.Repeat("e", 40), 1)
			}},
	} {
		t.Setenv("DESK_ROOTS", c.roots)
		problems, _ := failFirstGateChecks(tracking, ffStream(t, tracking, ffCrossEvidence(base, source, c.att)))
		if !containsSub(problems, c.want) {
			t.Errorf("%s: want a PROBLEM naming %q; problems=%v", c.name, c.want, problems)
		}
		if containsSub(problems, "not the merge-base or an ancestor") {
			t.Errorf("%s: fell back to the tracking-clone rule; problems=%v", c.name, problems)
		}
	}
}

// regression: #5549
// TestFFCrossRepoSelected: the selected-closure caller (verifyclosure and
// `brief --check-verified`, the check a deskevidence landing runs) reaches
// the same cross-repository arm as lint: a base that is an ancestor of the
// attested source authorizes the outcome, a later commit does not.
func TestFFCrossRepoSelected(t *testing.T) {
	deliverable, _, base, source, later, _ := ffCrossRepo(t)
	t.Setenv("DESK_ROOTS", "example-org/two="+deliverable)
	tracking := ffCrossTracking(t)
	for _, c := range []struct {
		name, base string
		wantErr    bool
	}{{"ancestor of the source", base, false}, {"commit after the source", later, true}} {
		ffStream(t, tracking, ffCrossEvidence(c.base, source, nil))
		path := filepath.Join(tracking, "docs", "streams", "ff", "brief-01-ff.md")
		verify, evidence, err := briefSections(path)
		if err != nil {
			t.Fatal(err)
		}
		_, err = checkSelectedClosure(tracking, "ff/01", "verified", "2026-10-08 human:fixture",
			briefArtifacts{Verify: verify, Evidence: evidence, Path: path})
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v, want error=%v", c.name, err, c.wantErr)
		}
		if c.wantErr && (err == nil || !strings.Contains(err.Error(), "attested deliverable source")) {
			t.Errorf("%s: want the cross-repository ancestry refusal, got %v", c.name, err)
		}
	}
}
