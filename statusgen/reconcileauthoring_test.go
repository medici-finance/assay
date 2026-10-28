package main

// reconcileauthoring_test.go — a PR that only AUTHORED a brief is not a witness that the brief
// is implemented (#1502). Every reader that turns a PR into an `implemented` witness takes the
// same exemption: the `Brief:`-trailer fold (reconcile, and the regen drift comparator that
// shares it) and the --backfill branch/body match. The tests drive the whole verb against a
// local server that serves the pull list AND each pull's changed-file list.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// authoringFiles is the changed-file list of a PR that adds brief dg/03 and nothing else.
var authoringFiles = []map[string]any{
	{"filename": "docs/streams/dg/README.md", "status": "modified"},
	{"filename": "docs/streams/dg/brief-03-model-scoped.md", "status": "added"},
	{"filename": "changelog/briefs-dg.md", "status": "added"},
}

// deliveryFiles is the changed-file list of a PR that implements something.
var deliveryFiles = []map[string]any{
	{"filename": "internal/dg/dg.go", "status": "modified"},
	{"filename": "docs/streams/dg/README.md", "status": "modified"},
}

// authoringServer serves pulls, and each pull's files from files[n]; a PR in failFiles answers
// HTTP 500 for its file list. It returns the list of PR numbers whose files were read.
func authoringServer(t *testing.T, pulls []map[string]any, files map[int][]map[string]any, failFiles map[int]bool) *[]int {
	t.Helper()
	var mu sync.Mutex
	read := &[]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/files"):
			parts := strings.Split(strings.Trim(p, "/"), "/")
			n, _ := strconv.Atoi(parts[len(parts)-2])
			mu.Lock()
			*read = append(*read, n)
			mu.Unlock()
			if failFiles[n] {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(files[n])
		case strings.Contains(p, "/issues"):
			_, _ = w.Write([]byte(`[]`))
		case strings.Contains(p, "/pulls"):
			if pg := r.URL.Query().Get("page"); pg != "" && pg != "1" {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_ = json.NewEncoder(w).Encode(pulls)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	prev := reconcileGHClient
	reconcileGHClient = func(token string) *ghClient {
		return &ghClient{doer: srv.Client(), base: srv.URL, token: token}
	}
	t.Cleanup(func() { reconcileGHClient = prev })
	foldFlipSeam(t, &fakeFlipSource{})
	return read
}

func appliedIDs(t *testing.T, stdout string) []string {
	t.Helper()
	var res reconcileResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("decoding --json: %v\n%s", err, stdout)
	}
	var ids []string
	for _, a := range res.Applied {
		ids = append(ids, a.ID+"<"+a.Witness+">")
	}
	return ids
}

const dg03Row = "| 03 | [Model gate, in-progress, no record](./brief-03-model-scoped.md) | 0 | S | in-progress |"

// regression: #1502
func TestAuthoringPRIsNoWitness(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	authoringServer(t, []map[string]any{
		mergedPull(31, "docs/dg-author", "Authors the stream.\n\nBrief: dg/03\n"),
	}, map[int][]map[string]any{31: authoringFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0:\n%s", code, stdout)
	}
	if got := appliedIDs(t, stdout); len(got) != 0 {
		t.Fatalf("a briefs-authoring PR flipped a row: %v", got)
	}
	if !strings.Contains(readFixtureReadme(t, root), dg03Row) {
		t.Fatalf("dg/03 must stay in-progress:\n%s", readFixtureReadme(t, root))
	}
}

// The planted SECOND instance of the class: a different brief, authored by a different PR,
// takes the same path; and a delivery PR for the same brief still witnesses it.
// regression: #1502
func TestAuthoringSecondInstance(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	second := []map[string]any{
		{"filename": "docs/streams/dg/brief-01-record-present.md", "status": "added"},
	}
	authoringServer(t, []map[string]any{
		mergedPull(32, "docs/dg-more", "Brief: dg/01\n"),
		mergedPull(31, "docs/dg-author", "Brief: dg/03\n"),
		mergedPull(30, "feat/dg", "Brief: dg/03\n"),
	}, map[int][]map[string]any{31: authoringFiles, 32: second, 30: deliveryFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	got := appliedIDs(t, stdout)
	if len(got) != 1 || !strings.HasPrefix(got[0], "dg/03<PR #30") {
		t.Fatalf("want only dg/03 witnessed by delivery PR #30, got %v", got)
	}
	if !strings.Contains(readFixtureReadme(t, root), "(./brief-01-record-present.md) | 0 | S | in-progress |") {
		t.Fatal("dg/01 (authoring-only PR) must stay in-progress")
	}
}

// regression: #1502
func TestAuthoringUnreadIsUnknown(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	before := readFixtureReadme(t, root)
	authoringServer(t, []map[string]any{
		mergedPull(31, "feat/dg", "Brief: dg/03\n"),
	}, nil, map[int]bool{31: true})

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r")
	if code != reconcileCouldNotCheck {
		t.Fatalf("exit %d, want %d (could-not-check):\n%s", code, reconcileCouldNotCheck, stdout)
	}
	if readFixtureReadme(t, root) != before {
		t.Fatal("an unreadable file list must write nothing")
	}
}

// Every derived row is classified, including completed rows used by the drift reader.
// regression: #1502
func TestAuthoringAllDerivedStates(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := writeBoard(t, map[string]string{"apstream": applyFixtureHeader +
		"| 01 | [first](brief-01-first.md) | 0 | M | done | 2026-01-02 v | 2026-01-03 r |\n" +
		"| 02 | [second](brief-02-second.md) | 0 | M | todo | — | — |\n"})
	read := authoringServer(t, []map[string]any{
		mergedPull(1, "feat/a", "Brief: apstream/01\n"),
		mergedPull(2, "feat/b", "Brief: apstream/02\n"),
	}, map[int][]map[string]any{1: deliveryFiles, 2: deliveryFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	if len(*read) != 2 {
		t.Fatalf("file lists read for PRs %v, want both completed and todo rows", *read)
	}
}

// The --backfill reader: a merged PR matched by branch or body that only authored the brief
// is not the witness; an older delivery for the same brief is.
// regression: #1502
func TestAuthoringBackfillSkips(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	commitFixture(t, root)
	authoringServer(t, []map[string]any{
		mergedPull(31, "docs/dg-03-author", ""),
		mergedPull(30, "feat/dg-03-impl", ""),
	}, map[int][]map[string]any{31: authoringFiles, 30: deliveryFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--backfill", "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	got := appliedIDs(t, stdout)
	if len(got) != 1 || !strings.HasPrefix(got[0], "dg/03<PR #30") {
		t.Fatalf("want dg/03 witnessed by delivery PR #30 (not authoring PR #31), got %v", got)
	}
}

// regression: #1502
func TestAuthoringBackfillOnlyAuthor(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	commitFixture(t, root)
	before := readFixtureReadme(t, root)
	authoringServer(t, []map[string]any{
		mergedPull(31, "docs/dg-03-author", "Authors dg/03."),
	}, map[int][]map[string]any{31: authoringFiles}, nil)

	code, stdout := runReconcileCaptured(t, "--backfill", "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	if got := appliedIDs(t, stdout); len(got) != 0 {
		t.Fatalf("an authoring-only backfill match flipped a row: %v", got)
	}
	if readFixtureReadme(t, root) != before {
		t.Fatal("nothing should be written")
	}
}

// regression: #1502
func TestAuthoringBackfillUnread(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	commitFixture(t, root)
	before := readFixtureReadme(t, root)
	authoringServer(t, []map[string]any{
		mergedPull(31, "feat/dg-03-impl", ""),
	}, nil, map[int]bool{31: true})

	code, _ := runReconcileCaptured(t, "--backfill", "--apply", "--root", root, "--repo", "o/r")
	if code != reconcileCouldNotCheck {
		t.Fatalf("exit %d, want %d", code, reconcileCouldNotCheck)
	}
	if readFixtureReadme(t, root) != before {
		t.Fatal("an unreadable candidate must write nothing")
	}
}

// The regen drift comparator shares the reader: an authoring PR is no `implemented` witness
// there either: no NOTICE says the witnesses derive dg/03 "implemented".
// regression: #1502
func TestAuthoringRegenDriftAgrees(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	authoringServer(t, []map[string]any{
		mergedPull(31, "docs/dg-author", "Brief: dg/03\n"),
	}, map[int][]map[string]any{31: authoringFiles}, nil)
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
	for _, n := range regenDriftNotices(root, streams, "o/r", false, "", errf) {
		if strings.Contains(n, "dg/03") && strings.Contains(n, `derive "implemented"`) {
			t.Fatalf("an authoring PR made the comparator derive dg/03 implemented:\n%s", n)
		}
	}
}
