package main

// reconcilewouldapply_test.go — the read-only form of `reconcile --backfill`
// (#2440): without --apply the verb reports, as `wouldApply`, the rows --apply
// would write, computed by the same function that writes (reconcileWrites).
//
// The parity tests below are the issue's acceptance check: on the same fixture,
// the read-only rows EQUAL the rows --apply writes, the read-only run leaves
// the tree byte-for-byte unchanged, and each row's rowBefore/rowAfter pair
// reproduces the README --apply leaves behind. Re-runnable mutation spec,
// consumed by the desk mutation harness (tools/desk/cmd/muhar):
//
//	cd statusgen && muhar -spec reconcile-mutations.json

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// parityFixtureReadmes is a two-stream board covering every branch of the
// write rule: a todo and an in-progress row that flip, a done row and a
// no-witness todo row that must not, and a second stream whose only witness is
// the declared backfill branch match.
var parityFixtureReadmes = map[string]string{
	"apstream": applyFixtureHeader +
		"| 01 | [first](brief-01-first.md) | 0 | M | todo | — | — |\n" +
		"| 02 | [second](brief-02-second.md) | 0 | M |  in-progress  | — | human:someone |\n" +
		"| 03 | [third](brief-03-third.md) | 0 | M | done | 2026-01-02 v | 2026-01-03 r |\n" +
		"| 04 | [fourth](brief-04-fourth.md) | 0 | M | todo | — | — |\n",
	"bpstream": strings.ReplaceAll(applyFixtureHeader, "apstream", "bpstream") +
		"| 01 | [only](brief-01-only.md) | 0 | M | todo | — | — |\n",
}

// writeParityBoard lays parityFixtureReadmes (and one legacy brief file per
// row, so reconcileBriefIdents enumerates them) under a fresh tree.
func writeParityBoard(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for stream, body := range parityFixtureReadmes {
		dir := filepath.Join(root, "docs", "streams", stream)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(body, "\n") {
			if i := strings.Index(line, "](brief-"); i >= 0 {
				name := line[i+2 : strings.Index(line[i:], ")")+i]
				if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	return root
}

// snapshotReadmes returns every stream README's bytes under root.
func snapshotReadmes(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for stream := range parityFixtureReadmes {
		b, err := os.ReadFile(filepath.Join(root, "docs", "streams", stream, "README.md"))
		if err != nil {
			t.Fatal(err)
		}
		out[stream] = string(b)
	}
	return out
}

// replayRows applies each row's rowBefore -> rowAfter to before, the way a
// reviewer reads a diff hunk, and returns the result per stream.
func replayRows(t *testing.T, root string, before map[string]string, rows []appliedRow) map[string]string {
	t.Helper()
	out := map[string]string{}
	for k, v := range before {
		out[k] = v
	}
	for _, r := range rows {
		stream := filepath.Base(filepath.Dir(r.Path))
		if r.RowBefore == "" || r.RowAfter == "" || r.RowBefore == r.RowAfter {
			t.Fatalf("row %s: rowBefore/rowAfter must both be set and differ: %+v", r.ID, r)
		}
		if strings.Count(out[stream], r.RowBefore+"\n") != 1 {
			t.Fatalf("row %s: rowBefore %q is not exactly one line of %s", r.ID, r.RowBefore, stream)
		}
		out[stream] = strings.Replace(out[stream], r.RowBefore+"\n", r.RowAfter+"\n", 1)
	}
	return out
}

// TestWouldApplyParityUnit pins the parity at the shared function: on one
// tree, planReconcileWrites reports exactly the rows applyReconcileWrites then
// writes, writes nothing itself, and its rows replay to the applied README.
// The cell list carries two ids that reduce to the SAME row (a hierarchical id
// and its short form): --apply writes that row once, so the read-only form must
// report it once too — the overlay case a parallel re-implementation misses.
func TestWouldApplyParityUnit(t *testing.T) {
	root := writeParityBoard(t)
	cells := []BriefCell{
		{ID: "apstream/01", Cell: "implemented", Source: "pr", Witness: "PR #10"},
		{ID: "cell:repo:apstream:01", Cell: "implemented", Source: "pr", Witness: "PR #10"},
		{ID: "apstream/02", Cell: "implemented", Source: "backfill", Witness: "PR #11 backfill"},
		{ID: "apstream/03", Cell: "implemented", Source: "pr", Witness: "PR #12"},
		{ID: "apstream/04", Cell: "todo", Source: "pr", Reason: "no PR"},
		{ID: "bpstream/01", Cell: "implemented", Source: "backfill", Witness: "PR #13 backfill"},
		{ID: "nostream/01", Cell: "implemented", Source: "pr", Witness: "PR #14"},
	}
	before := snapshotReadmes(t, root)

	planned, err := planReconcileWrites(root, cells)
	if err != nil {
		t.Fatalf("planReconcileWrites: %v", err)
	}
	if got := snapshotReadmes(t, root); !reflect.DeepEqual(got, before) {
		t.Fatalf("the read-only form wrote to the tree:\ngot:  %q\nwant: %q", got, before)
	}

	applied, err := applyReconcileWrites(root, cells)
	if err != nil {
		t.Fatalf("applyReconcileWrites: %v", err)
	}
	if len(applied) != 3 {
		t.Fatalf("fixture drift: want --apply to write 3 rows, got %d: %+v", len(applied), applied)
	}
	if !reflect.DeepEqual(planned, applied) {
		t.Fatalf("wouldApply rows differ from the rows --apply wrote:\nplanned: %+v\napplied: %+v", planned, applied)
	}
	if got, want := replayRows(t, root, before, planned), snapshotReadmes(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("replaying rowBefore->rowAfter does not reproduce the applied README:\ngot:  %q\nwant: %q", got, want)
	}
}

// runReconcileJSON runs the verb with args and decodes its --json stdout.
func runReconcileJSON(t *testing.T, args ...string) (reconcileResult, map[string]json.RawMessage) {
	t.Helper()
	dir := t.TempDir()
	stdout, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	code := runReconcile(args, stdout, stderr)
	stdout.Close()
	stderr.Close()
	out, _ := os.ReadFile(stdout.Name())
	errText, _ := os.ReadFile(stderr.Name())
	if code != reconcileOK {
		t.Fatalf("reconcile %v exited %d; stderr:\n%s", args, code, errText)
	}
	var res reconcileResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("stdout is not reconcile JSON: %v\n%s", err, out)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	return res, raw
}

// TestWouldApplyParityCLI runs the whole verb twice on one git-backed tree
// against a local recorded-response server (no network): first the read-only
// `--backfill --json` form, then `--backfill --apply --json`. The read-only run
// must leave the tree untouched and report as `wouldApply` exactly the rows the
// --apply run reports as `applied`; each form carries only its own list.
func TestWouldApplyParityCLI(t *testing.T) {
	root := writeParityBoard(t)
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	pulls := `[
	  {"number": 10, "state": "closed", "body": "Brief: apstream/01", "merged_at": "2026-01-01T00:00:00Z", "merge_commit_sha": "aaaaaaa1", "head": {"sha": "h10", "ref": "feat/x"}},
	  {"number": 11, "state": "closed", "body": "no trailer", "merged_at": "2026-01-01T00:00:00Z", "merge_commit_sha": "bbbbbbb2", "head": {"sha": "h11", "ref": "feat/apstream-02"}},
	  {"number": 12, "state": "closed", "body": "Brief: apstream/03", "merged_at": "2026-01-01T00:00:00Z", "merge_commit_sha": "ccccccc3", "head": {"sha": "h12", "ref": "feat/y"}},
	  {"number": 13, "state": "closed", "body": "", "merged_at": "2026-01-01T00:00:00Z", "merge_commit_sha": "ddddddd4", "head": {"sha": "h13", "ref": "work/bpstream/01"}}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/pulls" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, pulls)
	}))
	defer srv.Close()
	prev := reconcileGHClient
	reconcileGHClient = func(token string) *ghClient {
		return &ghClient{doer: srv.Client(), base: srv.URL, token: token}
	}
	defer func() { reconcileGHClient = prev }()

	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("read-only-test-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_TOKEN", "")
	base := []string{"--backfill", "--repo", "o/r", "--root", root, "--token-file", tokenFile, "--json"}
	before := snapshotReadmes(t, root)

	dry, dryRaw := runReconcileJSON(t, base...)
	if !dry.LookedAt {
		t.Fatalf("fixture drift: the read-only run did not look: %s", dry.Reason)
	}
	if got := snapshotReadmes(t, root); !reflect.DeepEqual(got, before) {
		t.Fatalf("the read-only run wrote to the tree:\ngot:  %q\nwant: %q", got, before)
	}
	if _, ok := dryRaw["applied"]; ok {
		t.Fatalf("the read-only run must not report `applied`: %s", dryRaw["applied"])
	}
	if dry.WouldApply == nil {
		t.Fatalf("the read-only run must report `wouldApply`; keys: %v", dryRaw)
	}

	wet, wetRaw := runReconcileJSON(t, append(base, "--apply")...)
	if _, ok := wetRaw["wouldApply"]; ok {
		t.Fatalf("the --apply run must not report `wouldApply`: %s", wetRaw["wouldApply"])
	}
	if len(wet.Applied) != 3 {
		t.Fatalf("fixture drift: want --apply to write 3 rows, got %d: %+v", len(wet.Applied), wet.Applied)
	}
	if !reflect.DeepEqual(*dry.WouldApply, wet.Applied) {
		t.Fatalf("wouldApply differs from applied:\nwouldApply: %+v\napplied:    %+v", *dry.WouldApply, wet.Applied)
	}
	if got, want := replayRows(t, root, before, *dry.WouldApply), snapshotReadmes(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("replaying wouldApply does not reproduce the applied README:\ngot:  %q\nwant: %q", got, want)
	}

	// A second read-only run on the now-applied tree looked and found nothing:
	// an explicit empty list, never an absent field.
	again, againRaw := runReconcileJSON(t, base...)
	if again.WouldApply == nil || len(*again.WouldApply) != 0 || string(againRaw["wouldApply"]) != "[]" {
		t.Fatalf("want wouldApply [] on an already-applied tree, got %s", againRaw["wouldApply"])
	}
}
