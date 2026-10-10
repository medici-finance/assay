package main

// reconcilefold_test.go — pins that the PRODUCTION callers of DeriveLifecycle
// populate every fold input from real reads (#1787), each by the rule the
// state's own writer obeys: `verified` from the latest strict PASS run the
// roster's verifier committed, at the brief version it ran against, with
// coverage released; gate:model `done` from decideModelFlip; gate:human `done`
// from the anchored README Reviewed-cell stamp; `blocked` from the linked
// issues' labels. Every read that could not be made renders `unknown` and
// is disclosed. One fixture per guard: each test fails when its guard is
// removed.

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// foldFixtureBrief is one brief file in the fold fixture tree.
type foldFixtureBrief struct {
	num, id, gate string
	issues        []int
	// ev is the Evidence section body; "{SHA}" becomes the implementation
	// commit's short sha. "" leaves the brief unwitnessed.
	ev string
	// evMail commits the Evidence; "" is the roster's verifier.
	evMail string
	// verifyTail is appended to the Verify section in every commit.
	verifyTail string
	// version, when non-zero, makes the brief brief-v2: the implementation
	// commit carries version 1 and the Evidence commit writes this one.
	version int
}

const (
	foldRow    = "| 1 | `true` | pass exit=0 | sha256:aaaaaaaaaaaa | 2026-10-01 | assay-verifier-app[bot] @ {SHA} |\n"
	foldPassEv = witnessHeader + "\n" + foldRow + "\n**VERIFY: PASS** — row 1 green.\n"
	foldRedEv  = witnessHeader + "\n| 1 | `true` | fail exit=1 | sha256:aaaaaaaaaaaa | 2026-10-01 | assay-verifier-app[bot] @ {SHA} |\n" +
		"\n**VERIFY: FAIL** — row 1 red.\n"
	foldBlockedEv = witnessHeader + "\n" + foldRow + "\n**VERIFY: BLOCKED** — rows pass; held on an open issue.\n"
	foldHeldEv    = witnessHeader + "\n" + foldRow + "\nRow 2 HELD — the smoke runner is offline.\n\n**VERIFY: PASS** — row 1 green.\n"
)

const (
	foldMailVerifier = "300000005+assay-verifier-app[bot]@users.noreply.github.com"
	foldMailWorker   = "300000006+assay-worker-app[bot]@users.noreply.github.com"
)

func foldBriefFile(b foldFixtureBrief, version int, ev string) string {
	issues := "[]"
	if len(b.issues) > 0 {
		parts := make([]string, len(b.issues))
		for i, n := range b.issues {
			parts[i] = strconv.Itoa(n)
		}
		issues = "[" + strings.Join(parts, ", ") + "]"
	}
	schema := "schema: brief-v1\n"
	if version > 0 {
		schema = "schema: brief-v2\nversion: " + strconv.Itoa(version) + "\n"
	}
	return "---\n" + schema + "brief: " + b.id + "\n" + "title: Fold fixture " + b.num + "\n" +
		"wave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: " + b.gate + "\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: " + issues + "\nauthored: 2026-09-10 by fold fixture\nsources: [\"fixture\"]\n---\n\n" +
		"# Brief " + b.num + "\n\n## Verify\n\n" +
		"| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | exit 0 |\n" + b.verifyTail + "\n" +
		"## Evidence\n\n" + ev + "\n## Review\n\nGate: " + b.gate + ".\n"
}

func foldGit(t *testing.T, root, mail string, args ...string) {
	t.Helper()
	name, _, _ := strings.Cut(mail, "@")
	runGitEnv(t, root, []string{"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + mail,
		"GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + mail},
		append([]string{"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
}

// writeFoldFixture builds a one-stream board (fx/01..NN) in a fresh git repo
// under t.TempDir(): the implementation commit (by the worker) carries every
// brief with an empty Evidence section; each witnessed brief's Evidence then
// lands in its own commit by evMail, naming the implementation commit's sha.
// rows maps brief num -> its README Status/Verified/Reviewed cells.
func writeFoldFixture(t *testing.T, briefs []foldFixtureBrief, rows map[string][3]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "fx")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var table strings.Builder
	table.WriteString("---\nstream: fx\nstatus: active\npriority: P1\ntrack: platform\n---\n\n" +
		"# Fold fixture stream\n\n## Briefs\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n")
	for _, b := range briefs {
		r := rows[b.num]
		table.WriteString("| " + b.num + " | [Fixture " + b.num + "](./brief-" + b.num + "-fixture.md) | 0 | S | " +
			r[0] + " | " + r[1] + " | " + r[2] + " |\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(table.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	path := func(b foldFixtureBrief) string { return filepath.Join(dir, "brief-"+b.num+"-fixture.md") }
	for _, b := range briefs {
		v := 0
		if b.version > 0 {
			v = 1 // a brief-v2 brief is authored at v1; the Evidence commit bumps it
		}
		if err := os.WriteFile(path(b), []byte(foldBriefFile(b, v, "")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	foldGit(t, root, foldMailWorker, "init", "-q")
	foldGit(t, root, foldMailWorker, "add", "-A")
	foldGit(t, root, foldMailWorker, "commit", "-q", "-m", "implement")
	out, err := exec.Command("git", "-C", root, "rev-parse", "--short=12", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(string(out))
	for _, b := range briefs {
		if b.ev == "" {
			continue
		}
		mail := b.evMail
		if mail == "" {
			mail = foldMailVerifier
		}
		ev := strings.ReplaceAll(b.ev, "{SHA}", sha)
		if err := os.WriteFile(path(b), []byte(foldBriefFile(b, b.version, ev)), 0o644); err != nil {
			t.Fatal(err)
		}
		foldGit(t, root, mail, "add", "-A")
		foldGit(t, root, mail, "commit", "-q", "-m", "evidence "+b.id)
	}
	return root
}

// foldServer answers the reads the wired fold makes: the paged pulls list
// (nil pulls = HTTP 500) and the paged open-issues list, page N served by
// issuePage(N) (a nil func = HTTP 500). It also installs a model-flip source
// that resolves no PR (could-not-check) so no test reaches the live forge;
// a test that judges gate:model done replaces it with foldFlipSeam.
func foldServer(t *testing.T, pulls []map[string]any, issuePage func(page int) []map[string]any) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/issues"):
			if issuePage == nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"no issues fixture"}`))
				return
			}
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			got := issuePage(page)
			if got == nil {
				got = []map[string]any{}
			}
			_ = json.NewEncoder(w).Encode(got)
		case strings.Contains(r.URL.Path, "/pulls"):
			if pulls == nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"no pulls fixture"}`))
				return
			}
			if r.URL.Query().Get("page") != "" && r.URL.Query().Get("page") != "1" {
				_ = json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			_ = json.NewEncoder(w).Encode(pulls)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"unstubbed path"}`))
		}
	}))
	t.Cleanup(srv.Close)
	prev := reconcileGHClient
	reconcileGHClient = func(token string) *ghClient {
		return &ghClient{doer: srv.Client(), base: srv.URL, token: token}
	}
	t.Cleanup(func() { reconcileGHClient = prev })
	foldFlipSeam(t, &fakeFlipSource{})
}

// foldFlipSeam makes decideModelFlip read src instead of the live forge.
func foldFlipSeam(t *testing.T, src modelFlipSource) {
	t.Helper()
	prev := reconcileFlipSource
	reconcileFlipSource = func(forgeKind) modelFlipSource { return src }
	t.Cleanup(func() { reconcileFlipSource = prev })
}

// foldIssues serves one page of issues and nothing after it.
func foldIssues(issues ...map[string]any) func(int) []map[string]any {
	return func(page int) []map[string]any {
		if page == 1 {
			return issues
		}
		return nil
	}
}

func foldMergedPull(n int, briefRef, headSHA string) map[string]any {
	return map[string]any{
		"number": n, "state": "closed", "body": "Does it.\n\nBrief: " + briefRef + "\n",
		"merged_at": "2026-10-01T00:00:00Z", "merge_commit_sha": strings.Repeat("a", 40),
		"head": map[string]any{"sha": headSHA, "ref": "feat/fx"},
	}
}

const (
	foldPR11Head   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	foldPR11Commit = "ccccccccccccccccccccccccccccccccccccccc1"
	foldReviewer   = "assay-reviewer-app[bot]"
)

// foldApproved is a flip source under which PR #11 delivered fx/01 and the
// bound reviewer approved it at its merged head.
func foldApproved() *fakeFlipSource {
	return &fakeFlipSource{
		commits: map[string][]string{"brief-01-fixture.md": {foldPR11Commit}},
		prs:     map[string]int{foldPR11Commit: 11},
		shapes: map[int]prShape{11: {Files: []string{"internal/fx/fx.go"}, BriefTrailers: 1,
			Briefs: []string{"fx/01"}}},
		states: map[int]prReviewState{11: {Merged: true, HeadSHA: foldPR11Head, Reviews: []ghReview{
			{Author: ghAuthor{Login: foldReviewer}, State: "APPROVED", CommitOID: foldPR11Head, Id: "PRR_11"},
		}}},
	}
}

// foldStdTree is the standard tree:
//
//	01 gate:model, PASS by the verifier — merged PR #11
//	02 gate:human, PASS by the verifier — merged PR #12, Reviewed human:alex
//	03 gate:model, unwitnessed, issues [7] — no PR
//	04 gate:model, FAIL — merged PR #14 (stays at the PR base)
//	05 gate:model, unwitnessed — nothing anywhere
func foldStdTree(t *testing.T) string {
	t.Helper()
	return writeFoldFixture(t,
		[]foldFixtureBrief{
			{num: "01", id: "fx/01", gate: "model", ev: foldPassEv},
			{num: "02", id: "fx/02", gate: "human", ev: foldPassEv},
			{num: "03", id: "fx/03", gate: "model", issues: []int{7}},
			{num: "04", id: "fx/04", gate: "model", ev: foldRedEv},
			{num: "05", id: "fx/05", gate: "model"},
		},
		map[string][3]string{
			"01": {"implemented", "2026-10-01 verifier", "—"},
			"02": {"done", "2026-10-01 verifier", "2026-10-05 human:alex"},
			"03": {"todo", "—", "—"},
			"04": {"implemented", "—", "—"},
			"05": {"todo", "—", "—"},
		})
}

func foldStdPulls() []map[string]any {
	return []map[string]any{
		foldMergedPull(11, "fx/01", foldPR11Head),
		foldMergedPull(12, "fx/02", strings.Repeat("d", 40)),
		foldMergedPull(14, "fx/04", strings.Repeat("e", 40)),
	}
}

// foldRun runs reconcile online over root and returns the exit, the decoded
// result and stderr.
func foldRun(t *testing.T, root string, extra ...string) (int, reconcileResult, string) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "test-token")
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	args := append([]string{"--root", root, "--repo", "o/r", "--json"}, extra...)
	code := runReconcile(args, out, errf)
	e, _ := os.ReadFile(errf.Name())
	b, _ := os.ReadFile(out.Name())
	var res reconcileResult
	if len(b) > 0 {
		if err := json.Unmarshal(b, &res); err != nil {
			t.Fatalf("decoding --json: %v\n%s", err, b)
		}
	}
	return code, res, string(e)
}

func foldCells(t *testing.T, root string) map[string]BriefCell {
	t.Helper()
	code, res, e := foldRun(t, root)
	if code != reconcileOK {
		t.Fatalf("reconcile exit %d, want 0; stderr:\n%s", code, e)
	}
	cells := map[string]BriefCell{}
	for _, c := range res.Briefs {
		cells[c.ID] = c
	}
	return cells
}

func wantCell(t *testing.T, cells map[string]BriefCell, id, want, why string) BriefCell {
	t.Helper()
	c := cells[id]
	if c.Cell != want {
		t.Errorf("%s: cell = %q (source %s, witness %q, reason %q), want %s — %s",
			id, c.Cell, c.Source, c.Witness, c.Reason, want, why)
	}
	return c
}

// TestFoldWiresAllInputs is the class-closing test for #1787: a full online
// reconcile derives done (both gates), blocked, and the red-witness fallback —
// none reachable while a fold map is left nil by the production constructor.
func TestFoldWiresAllInputs(t *testing.T) {
	root := foldStdTree(t)
	foldServer(t, foldStdPulls(), foldIssues(
		map[string]any{"number": 7, "labels": []map[string]any{{"name": "question"}}}))
	foldFlipSeam(t, foldApproved())
	cells := foldCells(t, root)

	c := wantCell(t, cells, "fx/01", "done", "verifier PASS + bound reviewer approved at the merged head")
	if !strings.Contains(c.Witness, foldReviewer) || !strings.Contains(c.Witness, "PR #11") {
		t.Errorf("fx/01: done must name the approval it read, got %q", c.Witness)
	}
	c = wantCell(t, cells, "fx/02", "done", "verifier PASS + human:alex Reviewed stamp")
	if !strings.Contains(c.Witness, "human:alex") {
		t.Errorf("fx/02: done must name the stamp it read, got %q", c.Witness)
	}
	wantCell(t, cells, "fx/03", "blocked", "linked issue 7 carries question")
	wantCell(t, cells, "fx/04", "implemented", "a red witness promotes nothing")
	wantCell(t, cells, "fx/05", "todo", "nothing anywhere")
}

// TestFoldVerifiedPositive: a verifier-committed PASS at a real sha, with the
// App approval refused, derives verified and names the run it read.
func TestFoldVerifiedPositive(t *testing.T) {
	root := foldStdTree(t)
	foldServer(t, foldStdPulls(), foldIssues())
	src := foldApproved()
	src.states[11] = prReviewState{Merged: true, HeadSHA: foldPR11Head}
	foldFlipSeam(t, src)
	c := wantCell(t, foldCells(t, root), "fx/01", "verified", "no approval: the brief stays verified")
	if !strings.Contains(c.Witness, "assay-verifier-app[bot] @ ") {
		t.Errorf("fx/01: verified must name the run it read, got %q", c.Witness)
	}
}

// TestFoldModelUncheckedUnknown: a could-not-check approval read is unknown
// with the reason, never verified and never done.
func TestFoldModelUncheckedUnknown(t *testing.T) {
	root := foldStdTree(t)
	foldServer(t, foldStdPulls(), foldIssues())
	src := foldApproved()
	src.errs = map[int]error{11: errors.New("HTTP 502")}
	foldFlipSeam(t, src)
	code, res, _ := foldRun(t, root)
	if code != reconcileOK {
		t.Fatalf("exit %d", code)
	}
	for _, c := range res.Briefs {
		if c.ID == "fx/01" && (c.Cell != "unknown" || !strings.Contains(c.Reason, "App approval")) {
			t.Errorf("fx/01: cell = %q (%q), want unknown naming the approval read", c.Cell, c.Reason)
		}
	}
	if len(res.Unread) == 0 {
		t.Errorf("the failed approval read must be disclosed in unread")
	}
}

// TestFoldStaleApprovalNotDone: an approval at an earlier commit, not the
// merged head, is not done (the at-head requirement).
func TestFoldStaleApprovalNotDone(t *testing.T) {
	root := foldStdTree(t)
	foldServer(t, foldStdPulls(), foldIssues())
	src := foldApproved()
	src.states[11] = prReviewState{Merged: true, HeadSHA: foldPR11Head, Reviews: []ghReview{
		{Author: ghAuthor{Login: foldReviewer}, State: "APPROVED", CommitOID: strings.Repeat("9", 40), Id: "PRR_old"},
	}}
	foldFlipSeam(t, src)
	wantCell(t, foldCells(t, root), "fx/01", "verified", "an approval at an earlier commit is not at head")
}

// TestFoldOtherApproverNotDone: an approval by an identity other than the
// bound reviewer is not done.
func TestFoldOtherApproverNotDone(t *testing.T) {
	root := foldStdTree(t)
	foldServer(t, foldStdPulls(), foldIssues())
	src := foldApproved()
	src.states[11] = prReviewState{Merged: true, HeadSHA: foldPR11Head, Reviews: []ghReview{
		{Author: ghAuthor{Login: "someone-else"}, State: "APPROVED", CommitOID: foldPR11Head, Id: "PRR_x"},
	}}
	foldFlipSeam(t, src)
	wantCell(t, foldCells(t, root), "fx/01", "verified", "only the bound reviewer's approval counts")
}

// TestFoldModelGateIgnoresStamp: a gate:model brief's human stamp is not its
// done witness.
func TestFoldModelGateIgnoresStamp(t *testing.T) {
	root := writeFoldFixture(t,
		[]foldFixtureBrief{{num: "01", id: "fx/01", gate: "model", ev: foldPassEv}},
		map[string][3]string{"01": {"implemented", "—", "2026-10-05 human:alex"}})
	foldServer(t, foldStdPulls()[:1], foldIssues())
	src := foldApproved()
	src.states[11] = prReviewState{Merged: true, HeadSHA: foldPR11Head}
	foldFlipSeam(t, src)
	wantCell(t, foldCells(t, root), "fx/01", "verified", "gate:model closes on the App approval only")
}

// TestHumanSignoffAnchored pins the anchored stamp read: a substring, a
// relay, a bare token and prose never count, and the name stops at the login
// (trailing punctuation is not part of it).
func TestHumanSignoffAnchored(t *testing.T) {
	for cell, want := range map[string]string{
		"2026-10-05 human:alex":                         "human:alex",
		"human:alex":                                    "human:alex",
		"2026-10-05 human:alex, per #12":                "human:alex",
		"2026-10-05 superhuman:x":                       "",
		"2026-10-05 non-human:bot":                      "",
		"2026-10-05 human:":                             "",
		"no human: review yet":                          "",
		"2026-10-05 app[bot] (on-behalf-of human:alex)": "",
		"—": "",
	} {
		if got := humanSignoff(cell); got != want {
			t.Errorf("humanSignoff(%q) = %q, want %q", cell, got, want)
		}
	}
}

// TestFoldHumanSubstringNotDone: the same through the wired fold — a
// gate:human brief whose Reviewed cell carries only a substring stamp stays
// verified.
func TestFoldHumanSubstringNotDone(t *testing.T) {
	root := writeFoldFixture(t,
		[]foldFixtureBrief{{num: "02", id: "fx/02", gate: "human", ev: foldPassEv}},
		map[string][3]string{"02": {"implemented", "—", "2026-10-05 superhuman:x"}})
	foldServer(t, foldStdPulls()[1:2], foldIssues())
	wantCell(t, foldCells(t, root), "fx/02", "verified", "superhuman:x is not a human stamp")
}

// foldOneModel is a single gate:model brief with a merged PR and the given
// Evidence, approved at head — so any witness that survives would reach done.
func foldOneModel(t *testing.T, b foldFixtureBrief) map[string]BriefCell {
	t.Helper()
	b.num, b.id, b.gate = "01", "fx/01", "model"
	root := writeFoldFixture(t, []foldFixtureBrief{b},
		map[string][3]string{"01": {"implemented", "—", "—"}})
	foldServer(t, foldStdPulls()[:1], foldIssues())
	foldFlipSeam(t, foldApproved())
	return foldCells(t, root)
}

// TestFoldBlockedVerdictNoWitness: Evidence whose latest verdict is BLOCKED
// (rows pass) is no verified witness.
func TestFoldBlockedVerdictNoWitness(t *testing.T) {
	wantCell(t, foldOneModel(t, foldFixtureBrief{ev: foldBlockedEv}), "fx/01", "implemented",
		"the latest verdict is not a strict PASS")
}

// TestFoldHeldRowNoWitness: a PASS beside an unrouted HELD row is no
// verified witness (closeVerify's held refusal).
func TestFoldHeldRowNoWitness(t *testing.T) {
	wantCell(t, foldOneModel(t, foldFixtureBrief{ev: foldHeldEv}), "fx/01", "implemented",
		"a non-deferred HELD row contradicts the PASS")
}

// TestFoldImplementerPassNoWitness: a PASS committed by the implementer (the
// worker App), not the verifier, is no verified witness.
func TestFoldImplementerPassNoWitness(t *testing.T) {
	wantCell(t, foldOneModel(t, foldFixtureBrief{ev: foldPassEv, evMail: foldMailWorker}), "fx/01", "implemented",
		"the PASS lines must be the verifier's")
}

// TestFoldRefusedAuditNoWitness: an unterminated comment opener in the
// Verify section refuses the closure audit (closureWitnesses), so a PASS
// that would otherwise stand is no witness.
func TestFoldRefusedAuditNoWitness(t *testing.T) {
	wantCell(t, foldOneModel(t, foldFixtureBrief{ev: foldPassEv, verifyTail: "\n<!-- an opener never closed\n"}),
		"fx/01", "implemented", "a refused Evidence audit yields no witness")
}

// TestFoldNoVerifyRowsNoWitness: a PASS marker with a witness table but no
// Verify row to bind it to is no witness.
func TestFoldNoVerifyRowsNoWitness(t *testing.T) {
	root := writeFoldFixture(t,
		[]foldFixtureBrief{{num: "01", id: "fx/01", gate: "model", ev: foldPassEv}},
		map[string][3]string{"01": {"implemented", "—", "—"}})
	p := filepath.Join(root, "docs", "streams", "fx", "brief-01-fixture.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(raw), "| 1 | `true` | exit 0 |\n", "", 1)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	foldGit(t, root, foldMailVerifier, "commit", "-q", "-am", "drop verify row")
	foldServer(t, foldStdPulls()[:1], foldIssues())
	foldFlipSeam(t, foldApproved())
	wantCell(t, foldCells(t, root), "fx/01", "implemented", "no Verify row, no witness")
}

// TestFoldStaleVersionUnknown: the run was made against v1 and the brief is
// now v2 — the version is read at the run's sha, never copied from the
// current brief, so the stale-version demotion fires.
func TestFoldStaleVersionUnknown(t *testing.T) {
	c := wantCell(t, foldOneModel(t, foldFixtureBrief{ev: foldPassEv, version: 2}), "fx/01", "unknown",
		"witness for v1, brief is v2")
	if !strings.Contains(c.Reason, "v1") {
		t.Errorf("fx/01: reason must name the stale version, got %q", c.Reason)
	}
}

// TestFoldPRReadFailedUnknown: the PR list read failed; a witnessed brief is
// unknown, never verified.
func TestFoldPRReadFailedUnknown(t *testing.T) {
	root := foldStdTree(t)
	foldServer(t, nil, foldIssues())
	cells := foldCells(t, root)
	for _, id := range []string{"fx/01", "fx/02"} {
		wantCell(t, cells, id, "unknown", "the PR read failed: nothing to overlay")
	}
}

// TestFoldUnmergedWitnessUnknown: a passing witness with only a closed,
// unmerged PR on record is unknown, never verified.
func TestFoldUnmergedWitnessUnknown(t *testing.T) {
	root := writeFoldFixture(t,
		[]foldFixtureBrief{{num: "01", id: "fx/01", gate: "model", ev: foldPassEv}},
		map[string][3]string{"01": {"todo", "—", "—"}})
	closed := foldMergedPull(11, "fx/01", foldPR11Head)
	delete(closed, "merged_at")
	foldServer(t, []map[string]any{closed}, foldIssues())
	foldFlipSeam(t, foldApproved())
	wantCell(t, foldCells(t, root), "fx/01", "unknown", "no merged PR carries the trailer")
}

// TestFoldIssuePRsFiltered: a PR entry in the issues list never blocks.
func TestFoldIssuePRsFiltered(t *testing.T) {
	root := foldStdTree(t)
	foldServer(t, foldStdPulls(), foldIssues(map[string]any{
		"number": 7, "pull_request": map[string]any{"url": "x"},
		"labels": []map[string]any{{"name": "question"}},
	}))
	wantCell(t, foldCells(t, root), "fx/03", "todo", "issue #7 is a PR, not an issue")
}

// foldFullPage is one full page (100) of open issues labelled only `chore`.
func foldFullPage(page int) []map[string]any {
	out := make([]map[string]any, 100)
	for i := range out {
		out[i] = map[string]any{"number": 1000 + page*100 + i, "labels": []map[string]any{{"name": "chore"}}}
	}
	return out
}

// TestFoldIssuesPaged: the blocking issue on page 2 is read.
func TestFoldIssuesPaged(t *testing.T) {
	root := foldStdTree(t)
	foldServer(t, foldStdPulls(), func(page int) []map[string]any {
		if page == 1 {
			return foldFullPage(1)
		}
		if page == 2 {
			return []map[string]any{{"number": 7, "labels": []map[string]any{{"name": "question"}}}}
		}
		return nil
	})
	wantCell(t, foldCells(t, root), "fx/03", "blocked", "issue #7 is on page 2")
}

// TestFoldIssuesCapUnknown: a read that reaches the page cap with full pages
// is truncated: the brief with linked issues is unknown, never todo.
func TestFoldIssuesCapUnknown(t *testing.T) {
	root := foldStdTree(t)
	foldServer(t, foldStdPulls(), foldFullPage)
	code, res, _ := foldRun(t, root)
	if code != reconcileOK {
		t.Fatalf("exit %d", code)
	}
	for _, c := range res.Briefs {
		if c.ID == "fx/03" && (c.Cell != "unknown" || !strings.Contains(c.Reason, "cap")) {
			t.Errorf("fx/03: cell = %q (%q), want unknown naming the page cap", c.Cell, c.Reason)
		}
		if c.ID == "fx/05" && c.Cell != "todo" {
			t.Errorf("fx/05: cell = %q, want todo — it links no issue", c.Cell)
		}
	}
}

// TestFoldIssuesFailUnknown: a failed labels read renders the linked brief
// unknown, discloses it in the JSON, and refuses --apply with exit 3.
func TestFoldIssuesFailUnknown(t *testing.T) {
	root := foldStdTree(t)
	foldServer(t, foldStdPulls(), nil)
	code, res, _ := foldRun(t, root)
	if code != reconcileOK {
		t.Fatalf("exit %d", code)
	}
	for _, c := range res.Briefs {
		if c.ID == "fx/03" && c.Cell != "unknown" {
			t.Errorf("fx/03: cell = %q, want unknown — a failed issues read is not a definite todo", c.Cell)
		}
	}
	if len(res.Unread) == 0 || !strings.Contains(strings.Join(res.Unread, "\n"), "open-issues") {
		t.Errorf("the failed issues read must be disclosed in unread, got %v", res.Unread)
	}
	readme := filepath.Join(root, "docs", "streams", "fx", "README.md")
	before, _ := os.ReadFile(readme)
	if code, _, e := foldRun(t, root, "--apply"); code != reconcileCouldNotCheck {
		t.Errorf("--apply exit %d, want %d; stderr:\n%s", code, reconcileCouldNotCheck, e)
	}
	if after, _ := os.ReadFile(readme); string(after) != string(before) {
		t.Errorf("--apply wrote with a fold read unread")
	}
}

// TestFoldApplyWritesWitnessed is C7: a brief whose README row is
// in-progress, with a merged PR and a passing witness (derived verified),
// is still written implemented by --apply, naming the merged PR.
func TestFoldApplyWritesWitnessed(t *testing.T) {
	root := writeFoldFixture(t,
		[]foldFixtureBrief{{num: "01", id: "fx/01", gate: "model", ev: foldPassEv}},
		map[string][3]string{"01": {"in-progress", "—", "—"}})
	foldServer(t, foldStdPulls()[:1], foldIssues())
	src := foldApproved()
	src.states[11] = prReviewState{Merged: true, HeadSHA: foldPR11Head}
	foldFlipSeam(t, src)
	code, res, e := foldRun(t, root, "--apply")
	if code != reconcileOK {
		t.Fatalf("--apply exit %d; stderr:\n%s", code, e)
	}
	if len(res.Applied) != 1 || res.Applied[0].ID != "fx/01" || !strings.Contains(res.Applied[0].Witness, "PR #11") {
		t.Fatalf("want fx/01 written naming PR #11, got applied=%+v held=%+v", res.Applied, res.Held)
	}
}

// TestFoldOfflineStaysUnknown pins that the wiring changes nothing about the
// offline arm: no tree or forge witness populates --offline.
func TestFoldOfflineStaysUnknown(t *testing.T) {
	root := foldStdTree(t)
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	if code := runReconcile([]string{"--root", root, "--offline", "--json"}, out, errf); code != reconcileOK {
		t.Fatalf("exit %d, want 0", code)
	}
	b, _ := os.ReadFile(out.Name())
	var res reconcileResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Briefs {
		if c.Cell != "unknown" {
			t.Fatalf("%s: offline cell = %q, want unknown for every brief", c.ID, c.Cell)
		}
	}
}

// TestRegenDriftUsesWiredFold pins the SECOND production caller (#1787 names
// both): the regen drift comparator folds the same wired inputs.
func TestRegenDriftUsesWiredFold(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := foldStdTree(t)
	foldServer(t, foldStdPulls(), foldIssues(
		map[string]any{"number": 7, "labels": []map[string]any{{"name": "question"}}}))
	foldFlipSeam(t, foldApproved())

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range streams {
		s.Board = "generated"
	}
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(regenDriftNotices(root, streams, "o/r", false, "", errf), "\n")
	if !strings.Contains(joined, "fx/01") || !strings.Contains(joined, `"done"`) {
		t.Errorf("fx/01 implemented-vs-done drift must be NOTICEd, got:\n%s", joined)
	}
	if strings.Contains(joined, "fx/02") {
		t.Errorf("fx/02 agrees (done) and must produce no drift NOTICE, got:\n%s", joined)
	}
	if !strings.Contains(joined, "fx/03") || !strings.Contains(joined, `"blocked"`) {
		t.Errorf("fx/03 todo-vs-blocked drift must be NOTICEd, got:\n%s", joined)
	}
}

// TestDriftNoticeHierarchicalID pins the comparator's join key directly:
// before the fix, a hierarchical (brief-v2) cell id never matched the
// stream/NN-shaped asserted map, so the comparator was blind on a v2 tree.
func TestDriftNoticeHierarchicalID(t *testing.T) {
	s := &Stream{
		Name:   "fx",
		Briefs: []Brief{{Num: "01", Status: "todo"}},
	}
	derived := []BriefCell{
		{ID: "cell:repo:fx:01", Cell: "implemented", Witness: "PR #7 (merged abc1234)"},
	}
	notices := assertedVsDerivedNotices(s, derived)
	if len(notices) != 1 {
		t.Fatalf("want exactly 1 drift NOTICE for the hierarchical id, got %d: %v", len(notices), notices)
	}
}
