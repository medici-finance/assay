package main

// reconcileissue2443_test.go — the three wording/shape follow-ups to the
// read-only `reconcile --backfill` report (#2443, follow-up to #2441):
//
//  1. a --backfill run without --apply but WITH --report writes the drift
//     report file, so its output must not call itself read-only;
//  2. the reviewer step's admission comparison is bound to the entry's own
//     README, so a byte-identical row in another stream's README is not matched;
//  3. `wouldApply` is absent whenever the run could not look, so `[]` only ever
//     means "looked and found nothing".
//
// Mutation spec: reconcile-mutations.json (cd statusgen && muhar -spec
// reconcile-mutations.json).

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

// readTreeFiles returns every regular file under root (relative path -> bytes),
// .git excluded, so a test can assert on writes ANYWHERE under the tree.
func readTreeFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, p)
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func tokenFileFor(t *testing.T) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(f, []byte("test-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_TOKEN", "")
	return f
}

// regression: #2443
func TestReportRunNotCalledReadOnly(t *testing.T) {
	stubParityPulls(t)
	tok := tokenFileFor(t)

	// Control: --backfill alone is read-only and says so; nothing at all is
	// written anywhere under the tree.
	plain := writeParityBoard(t)
	commitFixture(t, plain)
	beforePlain := readTreeFiles(t, plain)
	code, out := runReconcileCaptured(t, "--backfill", "--repo", "o/r", "--root", plain, "--token-file", tok)
	if code != reconcileOK {
		t.Fatalf("--backfill exited %d", code)
	}
	if !strings.Contains(out, "reconcile --backfill (read-only): --apply would write 3 row(s)") {
		t.Fatalf("a --backfill run that wrote nothing must say read-only; got:\n%s", out)
	}
	if got := readTreeFiles(t, plain); !reflect.DeepEqual(got, beforePlain) {
		t.Fatalf("--backfill alone wrote under the tree:\ngot:  %q\nwant: %q", got, beforePlain)
	}

	// --backfill --report (no --apply): the drift report IS written, no stream
	// README is, and the output must not label the run read-only.
	rep := writeParityBoard(t)
	commitFixture(t, rep)
	beforeRep := snapshotReadmes(t, rep)
	code, out = runReconcileCaptured(t, "--backfill", "--report", "--repo", "o/r", "--root", rep, "--token-file", tok)
	if code != reconcileOK {
		t.Fatalf("--backfill --report exited %d", code)
	}
	drift, _ := filepath.Glob(filepath.Join(rep, "docs", "streams", "board-drift-*.md"))
	if len(drift) != 1 {
		t.Fatalf("fixture drift: --report must write exactly one drift report, found %v", drift)
	}
	if strings.Contains(out, "(read-only)") {
		t.Fatalf("a run that wrote the drift report must not call itself read-only; got:\n%s", out)
	}
	if !strings.Contains(out, "reconcile --backfill (no stream README written; --report wrote the drift report): --apply would write 3 row(s)") {
		t.Fatalf("the output must name what the run wrote; got:\n%s", out)
	}
	if got := snapshotReadmes(t, rep); !reflect.DeepEqual(got, beforeRep) {
		t.Fatalf("--backfill --report wrote a stream README:\ngot:  %q\nwant: %q", got, beforeRep)
	}
}

// regression: #2443
func TestWouldApplyAbsentNoLook(t *testing.T) {
	tok := tokenFileFor(t)
	board := func(t *testing.T) string {
		root := writeParityBoard(t)
		commitFixture(t, root)
		return root
	}
	noBoard := func(t *testing.T) string {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		commitFixture(t, root)
		return root
	}
	cases := []struct {
		name       string
		root       func(*testing.T) string
		args       []string
		stub       func(*testing.T)
		wantLooked bool
	}{
		{"offline", board, []string{"--offline"}, nil, false},
		{"no repo", board, nil, nil, false},
		{"http 401", board, []string{"--repo", "o/r"}, func(t *testing.T) { stubPullsServer(t, http.StatusUnauthorized, nil) }, false},
		{"http 403", board, []string{"--repo", "o/r"}, func(t *testing.T) { stubPullsServer(t, http.StatusForbidden, nil) }, false},
		{"root with no board", noBoard, []string{"--repo", "o/r"}, stubParityPulls, true},
		{"raw pull list fails", board, []string{"--repo", "o/r"}, func(t *testing.T) {
			// The PR list reads, the second read of the pull list does not.
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) > 1 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				fmt.Fprint(w, parityPulls)
			}))
			t.Cleanup(srv.Close)
			prev := reconcileGHClient
			reconcileGHClient = func(token string) *ghClient {
				return &ghClient{doer: srv.Client(), base: srv.URL, token: token}
			}
			t.Cleanup(func() { reconcileGHClient = prev })
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.stub != nil {
				tc.stub(t)
			}
			args := append([]string{"--backfill", "--root", tc.root(t), "--token-file", tok, "--json"}, tc.args...)
			res, raw := runReconcileJSON(t, args...)
			if res.LookedAt != tc.wantLooked {
				t.Fatalf("lookedAt = %v, want %v (reason %q)", res.LookedAt, tc.wantLooked, res.Reason)
			}
			if v, ok := raw["wouldApply"]; ok {
				t.Fatalf("a run that could not look must omit wouldApply, got %s", v)
			}
			if v, ok := raw["held"]; ok {
				t.Fatalf("a run that could not look must omit held, got %s", v)
			}
		})
	}

	// The other side: a run that looked and found nothing still says `[]`.
	stubParityPulls(t)
	root := writeParityBoard(t)
	commitFixture(t, root)
	runReconcileJSON(t, "--backfill", "--repo", "o/r", "--root", root, "--token-file", tok, "--json", "--apply") // tree is now applied
	_, raw := runReconcileJSON(t, "--backfill", "--repo", "o/r", "--root", root, "--token-file", tok, "--json")
	if string(raw["wouldApply"]) != "[]" {
		t.Fatalf("a run that looked and found nothing must report [], got %s", raw["wouldApply"])
	}
}

// admitsHunk is the reviewer step's bound comparison, written as the skill
// states it: a hunk row (removed line, added line) in README file `readme` for
// brief `id` is reproduced only by an entry for THAT file and THAT brief whose
// rowBefore/rowAfter equal the hunk's lines.
func admitsHunk(entries []appliedRow, readme, id, removed, added string) bool {
	for _, e := range entries {
		if e.Path == readme && e.ID == id && e.RowBefore == removed && e.RowAfter == added {
			return true
		}
	}
	return false
}

// regression: #2443
func TestWouldApplyBoundToReadme(t *testing.T) {
	row := "| 01 | [first](brief-01-first.md) | 0 | M | todo | — | — |\n"
	root := writeBoard(t, map[string]string{
		"apstream": applyFixtureHeader + row,
		"bpstream": strings.ReplaceAll(applyFixtureHeader, "apstream", "bpstream") + row,
	})
	before := readTreeFiles(t, root)
	// A witness for the first stream only; bpstream/01 is unwitnessed.
	planned, held, err := planReconcileWrites(root, []BriefCell{
		{ID: "apstream/01", Cell: "implemented", Source: "pr", Witness: "PR #10"},
		{ID: "bpstream/01", Cell: "todo", Source: "pr", Reason: "no PR"},
	})
	if err != nil || len(held) != 0 {
		t.Fatalf("plan: err=%v held=%+v", err, held)
	}
	if got := readTreeFiles(t, root); !reflect.DeepEqual(got, before) {
		t.Fatal("the plan wrote to the tree")
	}
	if len(planned) != 1 {
		t.Fatalf("fixture drift: want exactly the witnessed row, got %+v", planned)
	}
	e := planned[0]
	removed, added := strings.TrimSuffix(row, "\n"), strings.Replace(strings.TrimSuffix(row, "\n"), "| todo |", "| implemented |", 1)
	if e.RowBefore != removed || e.RowAfter != added {
		t.Fatalf("fixture drift: want the identical todo->implemented line, got %q -> %q", e.RowBefore, e.RowAfter)
	}
	apPath := filepath.Join(root, "docs", "streams", "apstream", "README.md")
	bpPath := filepath.Join(root, "docs", "streams", "bpstream", "README.md")
	if !admitsHunk(planned, apPath, "apstream/01", removed, added) {
		t.Fatalf("the witnessed row's own hunk must be matched; entry %+v", e)
	}
	// The same bytes in the other stream's README, and under that stream's
	// brief id, are NOT reproduced by the entry for the first.
	if admitsHunk(planned, bpPath, "bpstream/01", removed, added) {
		t.Fatalf("a byte-identical row in another README was matched by %+v", e)
	}
	if e.Path == bpPath {
		t.Fatalf("the entry names the unwitnessed README: %+v", e)
	}
}

// printedTable renders res through the verb's own table printer.
func printedTable(t *testing.T, res reconcileResult) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "table")
	if err != nil {
		t.Fatal(err)
	}
	printReconcileTable(f, res)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestHeldLabelFollowsRunMode pins finding C2548-1: the held line's label is
// chosen from the run's mode, never from whether wouldApply is present, so a
// run without --apply that carries held rows and no wouldApply never prints
// the --apply label.
// regression: #2443
func TestHeldLabelFollowsRunMode(t *testing.T) {
	held := []heldRow{{ID: "apstream/01", From: "todo", To: "implemented", Witness: "PR #10", Problems: []string{"p"}}}
	cases := []struct {
		name      string
		res       reconcileResult
		want, not string
	}{
		{"no apply, no wouldApply", reconcileResult{LookedAt: true, Held: held},
			"reconcile --backfill (read-only): --apply would hold 1 row(s)", "reconcile --apply"},
		{"no apply, report written", reconcileResult{LookedAt: true, Held: held, reportWrote: true},
			"reconcile --backfill (no stream README written; --report wrote the drift report): --apply would hold 1 row(s)", "reconcile --apply"},
		{"apply", reconcileResult{LookedAt: true, Held: held, applyRun: true},
			"reconcile --apply: held 1 row(s)", "would hold"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := printedTable(t, tc.res)
			if !strings.Contains(out, tc.want) {
				t.Fatalf("want %q in:\n%s", tc.want, out)
			}
			if strings.Contains(out, tc.not) {
				t.Fatalf("the label misstates the run's mode (%q present):\n%s", tc.not, out)
			}
		})
	}
}
