package main

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// noticeloopclosure_test.go — statusgen/32: a board-drift NOTICE must reach the owner
// that can act on it, on evidence strong enough to act, and no further.
//
// TestNoticeLoopClosure is one matrix, one sub-test per routing commitment. Each
// sub-test pairs the routed case with a NEGATIVE CONTROL (the look-alike that must
// not route) and a MUTATION (the routed case with the fix undone must go red), so a
// green run cannot be a reader that never fires.

// Closes statusgen/32: board-drift notices (phantom, merged-unflipped, Verified
// attribution, generated-view links) were observed but routed to nobody.

func nlcHash(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}

// nlcStream builds an active stream on disk with the given brief files and in-memory
// todo rows.
func nlcStream(t *testing.T, root, name string, files map[string]string, rows ...Brief) *Stream {
	t.Helper()
	s := writeStream(t, root, name, "Active stream.\n", "", "")
	for num, body := range files {
		if err := os.WriteFile(filepath.Join(s.Dir, "brief-"+num+"-x.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s.Status, s.Priority, s.Briefs = "active", "P1", rows
	s.LastTouch = day(0)
	return s
}

func nlcPickIDs(nu NextUp) []string {
	var ids []string
	for _, p := range nu.Picks {
		ids = append(ids, p.Stream.Name+"/"+p.Brief.Num)
	}
	sort.Strings(ids)
	return ids
}

func TestNoticeLoopClosure(t *testing.T) {
	t.Run("foreign-owned or already-delivered rows cannot dispatch", func(t *testing.T) {
		build := func() (*Stream, []mergedPR) {
			root := t.TempDir()
			s := nlcStream(t, root, "gtm", map[string]string{
				"01": "## Context\nDelivered by a merged implementation PR.\n",
				// NEGATIVE CONTROL: a brief ABOUT de-housing. The heuristic matches
				// it, but a discussion of ownership is not an ownership declaration.
				"03": "## Context\nThis brief designs how work is de-housed to another repo; it is local work.\n",
				"04": "## Context\nNamed only in a merge subject; nothing says it was delivered.\n",
				"05": "## Context\nA delivery PR landed part of it.\n",
				"06": "## Context\nAn ordinary brief.\n",
			},
				Brief{Num: "01", Status: "todo"},
				Brief{Num: "02", Status: "todo", RawCell: "Old row [homed→acme/widgets]"}, // pointer row: no file
				Brief{Num: "03", Status: "todo"},
				Brief{Num: "04", Status: "todo"},
				Brief{Num: "05", Status: "todo", Delivery: []DeliveryClaim{{In: "sib#1", Covers: "partial", Note: "task 2 owed"}}},
				Brief{Num: "06", Status: "todo"},
			)
			merged := []mergedPR{
				{Number: 11, Subject: "feat: gtm/01 (#11)", Briefs: []string{"gtm/01"}, Delivers: []string{"gtm/01"}},
				{Number: 12, Subject: "Merge pull request #12 from o/brief/gtm-04-x", Briefs: []string{"gtm/04"}},
				{Number: 13, Subject: "feat: gtm/05 (#13)", Briefs: []string{"gtm/05"}, Delivers: []string{"gtm/05"}},
			}
			return s, merged
		}

		s, merged := build()
		applyPhantomHolds([]*Stream{s}, merged, nil)
		nu := nextUp([]*Stream{s}, ClaimView{}, nil)
		got := nlcPickIDs(nu)
		if want := "gtm/03 gtm/04 gtm/05 gtm/06"; strings.Join(got, " ") != want {
			t.Fatalf("dispatchable rows = %v, want %s (01 delivered and 02 re-homed pointer are held)", got, want)
		}
		if len(nu.PhantomElsewhere) != 2 || nu.PhantomElsewhere["gtm/01"] == "" || nu.PhantomElsewhere["gtm/02"] == "" {
			t.Errorf("the held rows must be NAMED, not dropped; PhantomElsewhere = %v", nu.PhantomElsewhere)
		}
		// The negative controls still SURFACE (the notice owner is unchanged), they are
		// just not excluded.
		notices := strings.Join(boardHonestyNotices([]*Stream{s}, merged, nil), "\n")
		if !strings.Contains(notices, "(dehoused): gtm/03") {
			t.Errorf("the de-housing mention must still surface as a NOTICE; got:\n%s", notices)
		}
		// The same classifier and the same holds reach the other emitters' shared
		// eligibility function; there is no second allowlist to drift.
		if eligibleBase([]*Stream{s}, s, s.Briefs[0], map[string]bool{}, nil) {
			t.Error("eligibleBase must refuse the held row")
		}

		// MUTATION: without the application step the delivered row dispatches again.
		s2, _ := build()
		if ids := nlcPickIDs(nextUp([]*Stream{s2}, ClaimView{}, nil)); !strings.Contains(strings.Join(ids, " "), "gtm/01") {
			t.Fatalf("mutation control: with no hold applied gtm/01 must be dispatchable; got %v", ids)
		}
	})

	t.Run("an authoring-only PR does not advance status", func(t *testing.T) {
		root := t.TempDir()
		s := nlcStream(t, root, "gtm", map[string]string{"07": "## Context\nAuthored by a docs PR.\n"},
			Brief{Num: "07", Status: "todo"})
		authoring := []mergedPR{{Number: 20, Subject: "docs: add brief gtm/07 (#20)", Briefs: []string{"gtm/07"}, Authors: []string{"gtm/07"}}}

		if got := mergedPRStatusNotices([]*Stream{s}, authoring, nil); len(got) != 0 {
			t.Errorf("an Authors-only PR must not read as delivery; got %v", got)
		}
		if got := boardHonestyNotices([]*Stream{s}, authoring, nil); len(got) != 0 {
			t.Errorf("an Authors-only PR must not raise already-merged-unflipped; got %v", got)
		}
		applyPhantomHolds([]*Stream{s}, authoring, nil)
		if s.Briefs[0].PhantomHeld != "" {
			t.Errorf("an authoring PR must not hold the row: %q", s.Briefs[0].PhantomHeld)
		}

		// The same PR with a Brief: trailer IS a delivery (the control that proves the
		// authoring branch is what silenced it).
		delivering := []mergedPR{{Number: 20, Subject: "docs: add brief gtm/07 (#20)", Briefs: []string{"gtm/07"}, Delivers: []string{"gtm/07"}}}
		if got := mergedPRStatusNotices([]*Stream{s}, delivering, nil); len(got) != 1 || strings.Contains(got[0], "UNCONFIRMED") {
			t.Errorf("a Brief:-trailer merge must fire a confirmed NOTICE; got %v", got)
		}

		// The reader: trailers come out of the commit body, fence-aware.
		commits := map[string]gitCommit{
			"aaa": {msg: "docs: add brief gtm/07 (#20)\n\nAuthors: gtm:07\n"},
			"bbb": {msg: "feat: gtm/08 (#21)\n\nBrief: gtm:08\n"},
			"ccc": {msg: "feat: gtm/09 (#22)\n\n```\nBrief: gtm:09\n```\n"},
			"ddd": {msg: "Merge pull request #23 from o/brief/gtm-10-x\n"},
		}
		prs := parseMergedLog([]string{"aaa", "bbb", "ccc", "ddd"}, commits, func(sha string) string {
			if sha == "ddd" {
				return "feat: part\n\nBrief: gtm:10\n"
			}
			return ""
		})
		byNum := map[int]mergedPR{}
		for _, p := range prs {
			byNum[p.Number] = p
		}
		if byNum[20].deliveryOf("gtm/07") != mergeAuthoring {
			t.Errorf("PR 20 must read as authoring: %+v", byNum[20])
		}
		if byNum[21].deliveryOf("gtm/08") != mergeDelivered {
			t.Errorf("PR 21 must read as delivered: %+v", byNum[21])
		}
		if byNum[22].deliveryOf("gtm/09") != mergeUnconfirmed {
			t.Errorf("a trailer inside a code fence is documentation, not delivery: %+v", byNum[22])
		}
		if byNum[23].deliveryOf("gtm/10") != mergeDelivered {
			t.Errorf("a merge commit takes its trailers from the branch commits: %+v", byNum[23])
		}
	})

	t.Run("absent forge state stays unknown", func(t *testing.T) {
		root := t.TempDir()
		s := nlcStream(t, root, "gtm", map[string]string{"01": "## Context\nx\n"}, Brief{Num: "01", Status: "todo"})
		delivered := []mergedPR{{Number: 11, Subject: "feat: gtm/01 (#11)", Briefs: []string{"gtm/01"}, Delivers: []string{"gtm/01"}}}

		// An unreadable history holds nothing, even when a stale slice came back with it.
		applyPhantomHolds([]*Stream{s}, delivered, errNoGitForMerge)
		if s.Briefs[0].PhantomHeld != "" {
			t.Errorf("a failed merge read must not hold a row: %q", s.Briefs[0].PhantomHeld)
		}
		notices := strings.Join(boardHonestyNotices([]*Stream{s}, delivered, errNoGitForMerge), "\n")
		if !strings.Contains(notices, "could-not-check") || strings.Contains(notices, "NON-DISPATCHABLE (already-merged-unflipped)") {
			t.Errorf("a failed merge read must say could-not-check and conclude nothing; got:\n%s", notices)
		}

		// A tree with no .git is the same state through the emitter-side entry point.
		holdPhantomRowsFromGit([]*Stream{s}, root)
		if s.Briefs[0].PhantomHeld != "" {
			t.Errorf("a tree with no git history must hold nothing: %q", s.Briefs[0].PhantomHeld)
		}

		// A merge that names the brief but whose delivery is unconfirmed is a lead, not a hold.
		unconfirmed := []mergedPR{{Number: 11, Subject: "feat: gtm/01 (#11)", Briefs: []string{"gtm/01"}}}
		applyPhantomHolds([]*Stream{s}, unconfirmed, nil)
		if s.Briefs[0].PhantomHeld != "" {
			t.Errorf("a subject-only merge must not hold a row: %q", s.Briefs[0].PhantomHeld)
		}
		if got := mergedPRStatusNotices([]*Stream{s}, unconfirmed, nil); len(got) != 1 || !strings.Contains(got[0], "UNCONFIRMED") {
			t.Errorf("an unconfirmed merge must say so; got %v", got)
		}
	})

	t.Run("the Verified actor follows the accepted run", func(t *testing.T) {
		const first, second = "assay-worker-app[bot]", vfRunner
		run := func(runner, sha, date string) string {
			return witnessTableFor(
				"| 1 | `true` | pass exit=0 | sha256:6f1a0b3c9d22 | "+date+" | "+runner+" @ "+sha+" |",
				"| 2 | `false` | pass exit=1 | sha256:aa11bb22cc33 | "+date+" | "+runner+" @ "+sha+" |") +
				"\n\n" + vfPass + "\n\n"
		}
		evidence := run(first, "aaaaaaaaaaaa", "2026-08-10") + run(second, vfSHA, "2026-08-13")

		stamp, _, _, _, err := flipStampFromEvidence(evidence, vfSHA, second)
		if err != nil || !strings.Contains(stamp, second) || strings.Contains(stamp, first) {
			t.Fatalf("the stamp must name the LATEST accepted run's actor: stamp=%q err=%v", stamp, err)
		}
		// The caller cannot substitute an earlier run's actor (or its own identity).
		if _, _, _, _, err := flipStampFromEvidence(evidence, vfSHA, first); err == nil {
			t.Error("the first run's actor must be refused once a later run is accepted")
		}
		if hint := acceptedRunHint(evidence); !strings.Contains(hint, second) {
			t.Errorf("the attribution notice must name the cell the accepted run derives; got %q", hint)
		}
		// Historical records without a strict PASS stay as they were: no hint, nothing derived.
		if hint := acceptedRunHint(witnessTableFor("| 1 | `true` | pass | sha256:6f1a0b3c9d22 | 2026-08-10 | someone |")); hint != "" {
			t.Errorf("a record with no accepted run must derive nothing; got %q", hint)
		}
		// The stale-cell fixture's notice is unchanged for a historical record.
		if !hasProblem(rerunNotices(t), "rerun/brief-01", "k3-verifier", "opus-verifier", "2 of 3") {
			t.Error("the historical attribution notice must still fire")
		}
	})

	t.Run("a unique register link repairs once", func(t *testing.T) {
		root, brief := nlcRegisterTree(t)
		before := nlcHash(t, filepath.Join(filepath.Dir(brief), "brief-02-frozen.md"))

		rep, err := backfillRegisterRefsReport(root)
		if err != nil || rep.Repaired != 1 {
			t.Fatalf("exactly the one unique prose link must be repaired: rep=%+v err=%v", rep, err)
		}
		raw, _ := os.ReadFile(brief)
		got := string(raw)
		if !strings.Contains(got, "[F-15](../findings/2026-07-09-one.md)") {
			t.Errorf("the unique link must point at its durable entry; got:\n%s", got)
		}
		// Frozen text stays byte-identical: the fence, the code span, the Verify command
		// and the recorded Evidence still carry the view link.
		if n := strings.Count(got, "[F-15](../FINDINGS.md)"); n != 4 {
			t.Errorf("fence, code span and Evidence links must be untouched (want 4 view links left, have %d):\n%s", n, got)
		}
		if nlcHash(t, filepath.Join(filepath.Dir(brief), "brief-02-frozen.md")) != before {
			t.Error("a file with only frozen links must be byte-identical after the pass")
		}

		// Once: the second pass finds nothing to do and writes nothing.
		again := nlcHash(t, brief)
		rep2, err := backfillRegisterRefsReport(root)
		if err != nil || rep2.Repaired != 0 || nlcHash(t, brief) != again {
			t.Errorf("the repair must be idempotent: rep=%+v err=%v", rep2, err)
		}
	})

	t.Run("an ambiguous or unmapped link does not change", func(t *testing.T) {
		root, _ := nlcRegisterTree(t)
		amb := filepath.Join(root, "docs", "streams", "gtm", "brief-03-ambiguous.md")
		before := nlcHash(t, amb)

		rep, err := backfillRegisterRefsReport(root)
		if err != nil {
			t.Fatal(err)
		}
		if nlcHash(t, amb) != before {
			t.Error("a file whose only view links are ambiguous or unmapped must be byte-identical")
		}
		joined := strings.Join(rep.Unresolved, "\n")
		if !strings.Contains(joined, "[F-20](../FINDINGS.md) — ambiguous: 2 entries") ||
			!strings.Contains(joined, "[F-99](../FINDINGS.md) — no durable entry") {
			t.Errorf("both unresolved links must be LISTED as author-owed; got:\n%s", joined)
		}
		// Unresolved links are findings of the lint, not silently dropped.
		_, notices := registerRefProblems(root, []string{amb})
		if len(notices) != 2 {
			t.Errorf("the lint must keep reporting both links; got %v", notices)
		}
	})

	// regression: the merge-trailer read must not depend on the branch NAME.
	t.Run("a house-shape merge reads its branch commits", func(t *testing.T) {
		root := t.TempDir()
		gitInit(t, root, "t", "t@example.invalid")
		commit := func(msg string) { runGit(t, root, "commit", "--allow-empty", "-m", msg) }
		commit("init")
		runGit(t, root, "branch", "-M", "main")

		// PR 5: the house branch shape names no brief; the trailer is on its commit.
		runGit(t, root, "checkout", "-b", "feat/assay--gtm--01")
		commit("feat: part one\n\nBrief: gtm:01")
		runGit(t, root, "checkout", "main")
		runGit(t, root, "merge", "--no-ff", "-m", "Merge pull request #5 from o/feat/assay--gtm--01", "feat/assay--gtm--01")

		// PR 7 merged main into its branch first: main's own commit (PR 6, brief 02)
		// must NOT be absorbed into PR 7's branch commits.
		runGit(t, root, "checkout", "-b", "feat/assay--gtm--03")
		commit("feat: part three\n\nBrief: gtm:03")
		runGit(t, root, "checkout", "main")
		commit("feat: gtm/02 (#6)\n\nBrief: gtm:02")
		runGit(t, root, "checkout", "feat/assay--gtm--03")
		runGit(t, root, "merge", "-m", "Merge branch main into feat", "main")
		runGit(t, root, "checkout", "main")
		runGit(t, root, "merge", "--no-ff", "-m", "Merge pull request #7 from o/feat/assay--gtm--03", "feat/assay--gtm--03")

		prs, err := mergedPRsFromGit(root, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		got := map[int][]string{}
		for _, p := range prs {
			got[p.Number] = p.Delivers
		}
		for num, want := range map[int]string{5: "gtm/01", 6: "gtm/02", 7: "gtm/03"} {
			if len(got[num]) != 1 || got[num][0] != want {
				t.Errorf("PR %d must deliver exactly [%s]; got %v (all: %v)", num, want, got[num], got)
			}
		}

		// The pure half: the same shape through parsed commits, no git.
		commits := map[string]gitCommit{
			"m5": {parents: []string{"p1", "b1"}, msg: "Merge pull request #5 from o/feat/assay--gtm--01\n"},
			"b1": {parents: []string{"p1"}, msg: "feat\n\nBrief: gtm:01\n"},
			"p1": {msg: "init\n"},
		}
		window := []string{"m5", "p1"}
		prs2 := parseMergedLog(window, commits, branchBodiesFrom(commits, window))
		if len(prs2) != 1 || len(prs2[0].Delivers) != 1 || prs2[0].Delivers[0] != "gtm/01" {
			t.Errorf("a trailer-less merge of a house-shape branch must read its branch commits; got %+v", prs2)
		}
		// NEGATIVE CONTROL: with no branch reader the same merge is not a delivery.
		if prs3 := parseMergedLog(window, commits, nil); len(prs3) != 0 {
			t.Errorf("without a branch reader the merge names no brief; got %+v", prs3)
		}
		// A trailer shown inside a ~~~ fence is documentation, not delivery.
		if d, _ := mergeTrailerKeys("~~~\nBrief: gtm:09\n~~~\n"); len(d) != 0 {
			t.Errorf("a ~~~-fenced trailer must not read as delivery; got %v", d)
		}
		// regression: a ``` fence is closed only by ```, never by a ~~~ line inside it.
		if d, _ := mergeTrailerKeys("```\n~~~\nBrief: gtm:09\n```\nBrief: gtm:10\n"); len(d) != 1 || d[0] != "gtm/10" {
			t.Errorf("a ~~~ line inside a ``` fence must not close it; got %v", d)
		}
	})

	// regression: commit text is author-controlled; no byte in it may forge a record
	// or reach git as an option (the 0x1e/0x1f split once did both).
	t.Run("a commit body cannot forge", func(t *testing.T) {
		root := t.TempDir()
		gitInit(t, root, "t", "t@example.invalid")
		sink := t.TempDir()
		out := filepath.Join(sink, "pwned")
		// The root commit is the oldest in the window: its body once became the
		// exclusion argument of the branch read.
		runGit(t, root, "commit", "--allow-empty", "-m", "init\n\n\x1e--output="+out+"\x1fx")
		runGit(t, root, "branch", "-M", "main")
		forged := "chore: x\n\nnote\x1e" + strings.Repeat("a", 40) +
			"\x1fMerge pull request #9 from o/brief/gtm-09-x\x1fBrief: gtm:09\n"
		runGit(t, root, "commit", "--allow-empty", "-m", forged)

		prs, err := mergedPRsFromGit(root, "HEAD")
		if err != nil {
			t.Fatalf("a message carrying separator bytes is still readable: %v", err)
		}
		for _, p := range prs {
			if p.Number == 9 || len(p.Delivers) != 0 {
				t.Errorf("a commit body must not forge a merged PR: %+v", p)
			}
		}
		if left, _ := os.ReadDir(sink); len(left) != 0 {
			t.Errorf("a commit body reached git as an option and wrote %s", left[0].Name())
		}
		// The topology reader refuses anything that is not a full object name.
		if _, _, err := parseRevListParents("--output=x\n"); err == nil {
			t.Error("a non-hex rev-list token must be a malformed read, not a key")
		}
		// cat-file records are length-bounded: a message carrying a fake record
		// header stays one message.
		sha := strings.Repeat("b", 40)
		obj := "tree " + strings.Repeat("c", 40) + "\n\nsubj\n" + sha + " commit 4\nfake\n"
		msgs, err := parseCatFileBatch([]byte(sha+" commit "+strconv.Itoa(len(obj))+"\n"+obj+"\n"), []string{sha})
		if err != nil || msgs[sha] != "subj\n"+sha+" commit 4\nfake\n" {
			t.Errorf("a fake header inside a message must stay message text: %q err=%v", msgs[sha], err)
		}
	})

	// regression: a stacked branch (B2 cut from B1, B1 merged first) must not
	// re-deliver B1's commits when B2 merges.
	t.Run("stacked branches claim once", func(t *testing.T) {
		root := t.TempDir()
		gitInit(t, root, "t", "t@example.invalid")
		runGit(t, root, "commit", "--allow-empty", "-m", "init")
		runGit(t, root, "branch", "-M", "main")
		runGit(t, root, "checkout", "-b", "feat/assay--gtm--01")
		runGit(t, root, "commit", "--allow-empty", "-m", "feat: one\n\nBrief: gtm:01")
		runGit(t, root, "checkout", "-b", "feat/assay--gtm--02")
		runGit(t, root, "commit", "--allow-empty", "-m", "feat: two\n\nBrief: gtm:02")
		runGit(t, root, "checkout", "main")
		runGit(t, root, "merge", "--no-ff", "-m", "Merge pull request #5 from o/feat/assay--gtm--01", "feat/assay--gtm--01")
		runGit(t, root, "merge", "--no-ff", "-m", "Merge pull request #6 from o/feat/assay--gtm--02", "feat/assay--gtm--02")

		prs, err := mergedPRsFromGit(root, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		got := map[int][]string{}
		for _, p := range prs {
			got[p.Number] = p.Delivers
		}
		for num, want := range map[int]string{5: "gtm/01", 6: "gtm/02"} {
			if len(got[num]) != 1 || got[num][0] != want {
				t.Errorf("PR %d must deliver exactly [%s]; got %v (all: %v)", num, want, got[num], got)
			}
		}
	})

	// regression: a branch read that fails is could-not-check, never "no trailer".
	t.Run("a failed branch read is unknown", func(t *testing.T) {
		root := t.TempDir()
		gitInit(t, root, "t", "t@example.invalid")
		runGit(t, root, "commit", "--allow-empty", "-m", "init")
		runGit(t, root, "branch", "-M", "main")
		runGit(t, root, "checkout", "-b", "feat/assay--gtm--01")
		runGit(t, root, "commit", "--allow-empty", "-m", "feat: one\n\nBrief: gtm:01")
		branch := strings.TrimSpace(gitOutT(t, root, "rev-parse", "HEAD"))
		runGit(t, root, "checkout", "main")
		runGit(t, root, "merge", "--no-ff", "-m", "Merge pull request #5 from o/feat/assay--gtm--01", "feat/assay--gtm--01")
		// The first-parent walk never opens the branch commit; the branch read must.
		if err := os.Remove(filepath.Join(root, ".git", "objects", branch[:2], branch[2:])); err != nil {
			t.Fatal(err)
		}

		prs, err := mergedPRsFromGit(root, "HEAD")
		if err == nil {
			t.Fatalf("an unreadable branch commit must fail the read; got %+v", prs)
		}
		s := nlcStream(t, t.TempDir(), "gtm", map[string]string{"01": "## Context\nx\n"}, Brief{Num: "01", Status: "todo"})
		got := strings.Join(mergedPRStatusNotices([]*Stream{s}, prs, err), "\n")
		if !strings.Contains(got, "could-not-check") || strings.Contains(got, "UNCONFIRMED") {
			t.Errorf("a failed branch read must say could-not-check and conclude nothing; got:\n%s", got)
		}
	})

	// regression: the Class-first Verify shapes are frozen like the legacy one.
	t.Run("a Class-first table is frozen", func(t *testing.T) {
		root, _ := nlcRegisterTree(t)
		gDir := filepath.Join(root, "docs", "streams", "gtm")
		classFirst := writeTemp(t, gDir, "brief-04-class.md", "# Four\n\n## Verify\n"+
			"| # | Class | Command | Expect |\n|---|---|---|---|\n"+
			"| 1 | check | `grep -c x y` | names [F-15](../FINDINGS.md) and F-15 |\n")
		withShell := writeTemp(t, gDir, "brief-05-shell.md", "# Five\n\n## Verify\n"+
			"| # | Class | Shell | Command | Expect |\n|---|---|---|---|---|\n"+
			"| 1 | check | bash | `grep -c x y` | names [F-15](../FINDINGS.md) and F-15 |\n")
		before := map[string][32]byte{classFirst: nlcHash(t, classFirst), withShell: nlcHash(t, withShell)}

		if _, err := backfillRegisterRefsReport(root); err != nil {
			t.Fatal(err)
		}
		for path, h := range before {
			if nlcHash(t, path) != h {
				raw, _ := os.ReadFile(path)
				t.Errorf("a Verify row must be byte-identical after the pass (%s):\n%s", filepath.Base(path), raw)
			}
		}
		// NEGATIVE CONTROL: a table without both cells is not a Verify table (the prose
		// repair itself is pinned by "a unique register link repairs once").
		if !isVerifyHeaderRow("| # | Command | Expect |") || isVerifyHeaderRow("| # | Command | Result |") {
			t.Error("a header is a Verify header by its Command and Expect cells, and only then")
		}
	})

	// regression: the dispatch queue must hold what STATUS.md holds.
	t.Run("the dispatch queue holds a phantom row", func(t *testing.T) {
		queue := func(root string) string {
			out := captureStdout(t, func() {
				if code := runNextUp(root); code != 0 {
					t.Fatalf("runNextUp exit %d", code)
				}
			})
			var view dispatchView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("--next-up did not emit JSON: %v\n%q", err, out)
			}
			var ids []string
			for _, r := range view.Rows {
				ids = append(ids, r.Brief)
			}
			sort.Strings(ids)
			return strings.Join(ids, " ")
		}
		addPointerRow := func(root string) {
			readme := filepath.Join(root, "docs", "streams", "example-stream", "README.md")
			raw, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			row := "| 03 | Old row [homed→acme/widgets] | 0 | S | todo | — | — |\n"
			if err := os.WriteFile(readme, append(raw, []byte(row)...), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		// A tree with no .git: the re-homed pointer row needs no history, so the
		// queue holds it exactly as run() does; the ordinary row stays dispatchable.
		root := siblingBriefTree(t, "")
		if got := queue(root); got != "example-stream/02" {
			t.Fatalf("control: before the pointer row the queue is [example-stream/02]; got %q", got)
		}
		addPointerRow(root)
		if got := queue(root); got != "example-stream/02" {
			t.Errorf("the re-homed pointer row (03) must be absent from --next-up on a no-.git tree; got %q", got)
		}

		// A tree with history: a merged `Brief:`-trailer PR on a house-shape branch
		// holds its row out of the queue.
		root2 := siblingBriefTree(t, "")
		gitInit(t, root2, "t", "t@example.invalid")
		runGit(t, root2, "add", "-A")
		runGit(t, root2, "commit", "-m", "init")
		runGit(t, root2, "branch", "-M", "main")
		if got := queue(root2); got != "example-stream/02" {
			t.Fatalf("control: an undelivered row is dispatchable; got %q", got)
		}
		runGit(t, root2, "checkout", "-b", "feat/assay--example-stream--02")
		runGit(t, root2, "commit", "--allow-empty", "-m", "feat: deliver\n\nBrief: example-stream:02")
		runGit(t, root2, "checkout", "main")
		runGit(t, root2, "merge", "--no-ff", "-m", "Merge pull request #9 from o/feat/assay--example-stream--02", "feat/assay--example-stream--02")
		if got := queue(root2); got != "" {
			t.Errorf("a row delivered by a merged Brief: PR must be absent from --next-up; got %q", got)
		}
	})

	// regression: --register-links never rewrites fenced text or recorded Evidence.
	t.Run("register links leave tilde fences and Evidence alone", func(t *testing.T) {
		root := t.TempDir()
		fDir := filepath.Join(root, "docs", "streams", "findings")
		gDir := filepath.Join(root, "docs", "streams", "gtm")
		for _, d := range []string{fDir, gDir} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		writeTemp(t, fDir, "2026-07-09-one.md", "---\nid: F-15\ndate: \"2026-07-09\"\ntitle: T\naffects: []\nresolved: false\n---\nBody.\n")
		frozen := "# Frozen\n\n## Notes\n~~~\nsee [F-15](../FINDINGS.md) and F-15\n~~~\n\n## Evidence\nRecorded: F-15 reproduced; see [F-15](../FINDINGS.md).\n"
		writeTemp(t, gDir, "brief-01-frozen.md", frozen)
		// POSITIVE CONTROL: the same bare id in plain prose IS linked, so a green run
		// below is the skip, not a bare arm that never fires.
		prose := writeTemp(t, gDir, "brief-02-prose.md", "# Prose\n\n## Notes\nCaused by F-15.\n")
		before := nlcHash(t, filepath.Join(gDir, "brief-01-frozen.md"))

		t.Chdir(root) // the bare arm resolves entry paths against a repo-relative root
		rep, err := backfillRegisterRefsReport(".")
		if err != nil {
			t.Fatal(err)
		}
		if nlcHash(t, filepath.Join(gDir, "brief-01-frozen.md")) != before {
			raw, _ := os.ReadFile(filepath.Join(gDir, "brief-01-frozen.md"))
			t.Errorf("a ~~~ fence and recorded Evidence must be byte-identical after the pass:\n%s", raw)
		}
		if raw, _ := os.ReadFile(prose); !strings.Contains(string(raw), "[F-15](") {
			t.Errorf("positive control: a bare id in prose must link; got %s (report %+v)", raw, rep)
		}
		if rep.Linked != 1 || rep.Repaired != 0 {
			t.Errorf("only the prose id is linked and nothing is repaired; got %+v", rep)
		}
	})
}

// nlcRegisterTree lays a register with one unique entry (F-15) and one ambiguous id
// (F-20), and three brief files: one with a repairable prose link plus frozen copies,
// one holding only frozen copies, one holding only ambiguous and unmapped links.
func nlcRegisterTree(t *testing.T) (root, brief string) {
	t.Helper()
	root = t.TempDir()
	fDir := filepath.Join(root, "docs", "streams", "findings")
	gDir := filepath.Join(root, "docs", "streams", "gtm")
	for _, d := range []string{fDir, gDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	entry := func(id string) string {
		return "---\nid: " + id + "\ndate: \"2026-07-09\"\ntitle: T\naffects: []\nresolved: false\n---\nBody.\n"
	}
	writeTemp(t, fDir, "2026-07-09-one.md", entry("F-15"))
	writeTemp(t, fDir, "2026-07-09-dup-a.md", entry("F-20"))
	writeTemp(t, fDir, "2026-07-09-dup-b.md", entry("F-20"))

	frozen := "```\nsee [F-15](../FINDINGS.md)\n```\n" +
		"Code: `[F-15](../FINDINGS.md)`.\n" +
		"\n## Verify\n| # | Command | Expect |\n|---|---------|--------|\n| 1 | `grep -F '[F-15](../FINDINGS.md)' x` | exit 0 |\n" +
		"\n## Evidence\nRecorded: [F-15](../FINDINGS.md) on 2026-07-10.\n"
	brief = writeTemp(t, gDir, "brief-01-repair.md", "# One\n\nSee [F-15](../FINDINGS.md) for the cause.\n\n"+frozen)
	writeTemp(t, gDir, "brief-02-frozen.md", "# Two\n\n"+frozen)
	writeTemp(t, gDir, "brief-03-ambiguous.md", "# Three\n\nSee [F-20](../FINDINGS.md) and [F-99](../FINDINGS.md).\n")
	return root, brief
}
