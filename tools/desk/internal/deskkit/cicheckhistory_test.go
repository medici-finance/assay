package deskkit

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- fixtures ------------------------------------------------------------------------------

const ciTestSHA = "0123456789abcdef0123456789abcdef01234567"

var ciTestRepo = ForgeRepo{Owner: "acme", Name: "widgets"}

// ciFake is an inner Forge serving fixed results for the three recording reads. Every other
// method is the embedded nil Forge — the decorator must not call one.
type ciFake struct {
	Forge
	checks *ChecksAtHead
	open   *OpenChanges
	queue  *ReviewQueue
	err    error
	calls  int
}

func (f *ciFake) ChecksAtHead(ForgeRepo, string) (*ChecksAtHead, error) {
	f.calls++
	return f.checks, f.err
}

func (f *ciFake) ListOpenChanges(ForgeRepo) (*OpenChanges, error) {
	f.calls++
	return f.open, f.err
}

func (f *ciFake) ReviewQueueSnapshot(ForgeRepo) (*ReviewQueue, error) {
	f.calls++
	return f.queue, f.err
}

// ciSetup isolates the state dir and the recorder's process state and hooks, and captures
// the recorder's stderr line. It returns the state dir and a pointer to the warnings.
func ciSetup(t *testing.T) (string, *[]string) {
	t.Helper()
	dir := setup(t)
	ciResetProcessState()
	oldNow, oldWait, oldSink, oldEnabled := ciNow, ciLockWait, ciWarnSink, ciRecorderEnabled
	var mu sync.Mutex
	var warns []string
	ciWarnSink = func(msg string) { mu.Lock(); warns = append(warns, msg); mu.Unlock() }
	t.Cleanup(func() {
		ciNow, ciLockWait, ciWarnSink, ciRecorderEnabled = oldNow, oldWait, oldSink, oldEnabled
		ciResetProcessState()
	})
	return dir, &warns
}

// ciDiskLines returns every raw line of every ci-checks file in dir — segments and the live
// file, NOT de-duplicated — so a test can count what was actually written.
func ciDiskLines(t *testing.T, dir string) []CICheckRecord {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read state dir: %v", err)
	}
	var names []string
	for _, e := range ents {
		if e.Name() == ciChecksFile || ciSegmentPattern.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []CICheckRecord
	for _, n := range names {
		f, err := os.Open(filepath.Join(dir, n))
		if err != nil {
			t.Fatalf("open %s: %v", n, err)
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var r CICheckRecord
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				t.Fatalf("%s: unparseable line %q: %v", n, sc.Text(), err)
			}
			out = append(out, r)
		}
		f.Close()
	}
	return out
}

func ciRawLive(t *testing.T, dir string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ciChecksFile))
	if err != nil {
		t.Fatalf("read live file: %v", err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

func ciRun(id, name, status, conclusion, started, completed string) CheckRun {
	return CheckRun{ID: id, Name: name, Status: status, Conclusion: conclusion,
		StartedAt: started, CompletedAt: completed}
}

// ciUnwritable points the state dir under a regular file, so MkdirAll fails.
func ciUnwritable(t *testing.T) {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := dirOverride
	dirOverride = filepath.Join(blocker, "assay")
	t.Cleanup(func() { dirOverride = old })
}

// --- row 2 ---------------------------------------------------------------------------------

// TestCICheckHistory_RecordsBothKinds — a commit-status gate is recorded beside the check
// runs, and only terminal results land.
func TestCICheckHistory_RecordsBothKinds(t *testing.T) {
	dir, _ := ciSetup(t)
	inner := &ciFake{checks: &ChecksAtHead{
		CheckRuns: []CheckRun{
			ciRun("101", "build", "completed", "success", "2026-10-08T10:00:00Z", "2026-10-08T10:01:30Z"),
			ciRun("102", "lint", "queued", "", "", ""),
		},
		Statuses: []StatusContext{
			{State: "success", Context: "external-scan", CreatedAt: "2026-10-08T10:02:00Z"},
			{State: "pending", Context: "deploy-preview", CreatedAt: "2026-10-08T10:03:00Z"},
		},
	}}
	f := OutboundChecked(inner, "worker-desk")
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatalf("ChecksAtHead: %v", err)
	}

	recs := ciDiskLines(t, dir)
	if len(recs) != 2 {
		t.Fatalf("want exactly 2 records (the completed run and the success status), got %d: %+v", len(recs), recs)
	}
	var run, st *CICheckRecord
	for i := range recs {
		switch recs[i].Kind {
		case CICheckKindRun:
			run = &recs[i]
		case CICheckKindStatus:
			st = &recs[i]
		}
	}
	if run == nil || st == nil {
		t.Fatalf("want one check-run and one status record, got %+v", recs)
	}
	if run.Name != "build" || run.Attempt != "101" || run.Conclusion != "success" ||
		run.DurationS == nil || *run.DurationS != 90 {
		t.Errorf("check-run record wrong: %+v (duration %v)", *run, run.DurationS)
	}
	if st.Name != "external-scan" || st.Attempt != "t:2026-10-08T10:02:00Z" || st.Conclusion != "success" ||
		st.DurationS != nil || st.StartedAt != "" || st.CompletedAt != "" {
		t.Errorf("status record wrong: %+v", *st)
	}
	for _, r := range recs {
		if r.Schema != CICheckSchema || r.Repo != "acme/widgets" || r.HeadSHA != ciTestSHA || r.PR != 0 ||
			r.Tool == "" || r.ObservedAt == "" {
			t.Errorf("common fields wrong: %+v", r)
		}
		if _, err := time.Parse(time.RFC3339, r.ObservedAt); err != nil {
			t.Errorf("observed_at %q is not RFC3339: %v", r.ObservedAt, err)
		}
	}
	// A ChecksAtHead read carries no change number: the key is omitted, not written as 0. A
	// status carries no duration: omitted, not 0.
	for _, line := range ciRawLive(t, dir) {
		if strings.Contains(line, `"pr"`) {
			t.Errorf("a ChecksAtHead record wrote a pr key: %s", line)
		}
		if strings.Contains(line, `"kind":"status"`) && strings.Contains(line, "duration_s") {
			t.Errorf("a status record wrote duration_s: %s", line)
		}
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(filepath.Join(dir, ciChecksFile)); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("live file mode = %v (err %v), want 0600", fi.Mode().Perm(), err)
		}
		if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
			t.Errorf("state dir mode = %v (err %v), want 0700", fi.Mode().Perm(), err)
		}
	}
}

// TestCIRollupRecords — the rollup shapes: GraphQL's uppercase values are lowercased, the
// change number is carried, an untagged node is classified by its name field, a run without
// an id keys on its start stamp, a keyless entry and the GitLab sentinel are skipped.
func TestCIRollupRecords(t *testing.T) {
	dir, _ := ciSetup(t)
	oc := OpenChange{Number: 42, HeadSHA: ciTestSHA, Rollup: []RollupNode{
		{Typename: "CheckRun", ID: "7001", Name: "test", Status: "COMPLETED", Conclusion: "FAILURE",
			StartedAt: "2026-10-08T09:00:00Z", CompletedAt: "2026-10-08T09:00:10Z"},
		{Typename: "CheckRun", Name: "no-id", Status: "COMPLETED", Conclusion: "SUCCESS",
			StartedAt: "2026-10-08T09:01:00Z", CompletedAt: "2026-10-08T09:01:05Z"},
		{Typename: "CheckRun", Name: "keyless", Status: "COMPLETED", Conclusion: "SUCCESS"},
		{Typename: "CheckRun", ID: "7002", Name: "running", Status: "IN_PROGRESS"},
		{Typename: "StatusContext", Context: "ci/gitlab", State: "FAILURE", CreatedAt: "2026-10-08T09:02:00Z"},
		{Context: "untagged-status", State: "error", CreatedAt: "2026-10-08T09:03:00Z"},
		{Typename: "StatusContext", Context: "expected", State: "EXPECTED", CreatedAt: "2026-10-08T09:04:00Z"},
		{Typename: GitLabRollupUnmapped},
	}}
	f := OutboundChecked(&ciFake{open: &OpenChanges{Changes: []OpenChange{oc}}}, "worker-desk")
	if _, err := f.ListOpenChanges(ciTestRepo); err != nil {
		t.Fatal(err)
	}
	got := map[string]CICheckRecord{}
	for _, r := range ciDiskLines(t, dir) {
		got[r.Name] = r
		if r.PR != 42 {
			t.Errorf("%s: pr = %d, want 42", r.Name, r.PR)
		}
	}
	want := map[string][3]string{ // name -> kind, attempt, conclusion
		"test":            {CICheckKindRun, "7001", "failure"},
		"no-id":           {CICheckKindRun, "t:2026-10-08T09:01:00Z", "success"},
		"ci/gitlab":       {CICheckKindStatus, "t:2026-10-08T09:02:00Z", "failure"},
		"untagged-status": {CICheckKindStatus, "t:2026-10-08T09:03:00Z", "error"},
	}
	if len(got) != len(want) {
		t.Fatalf("records = %v, want exactly %v", got, want)
	}
	for name, w := range want {
		r, ok := got[name]
		if !ok || r.Kind != w[0] || r.Attempt != w[1] || r.Conclusion != w[2] {
			t.Errorf("%s: got %+v, want kind/attempt/conclusion %v", name, r, w)
		}
	}
}

// --- row 3 ---------------------------------------------------------------------------------

// ciGitHubServer serves one head's REST check-runs + statuses and one open change whose
// GraphQL rollup carries the same run. It counts every request.
type ciGitHubServer struct {
	srv   *httptest.Server
	mu    sync.Mutex
	count int
	runID int64
}

func newCIGitHubServer(t *testing.T, runID int64) *ciGitHubServer {
	t.Helper()
	s := &ciGitHubServer{runID: runID}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.count++
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		switch {
		case strings.HasSuffix(r.URL.Path, "/commits/"+ciTestSHA+"/status"):
			_ = enc.Encode(map[string]any{"state": "success", "total_count": 1, "statuses": []map[string]any{
				{"state": "success", "context": "external-scan", "created_at": "2026-10-08T10:02:00Z"},
			}})
		case strings.HasSuffix(r.URL.Path, "/commits/"+ciTestSHA+"/check-runs"):
			_ = enc.Encode(map[string]any{"total_count": 1, "check_runs": []map[string]any{
				{"id": s.runID, "name": "build", "status": "completed", "conclusion": "success",
					"started_at": "2026-10-08T10:00:00Z", "completed_at": "2026-10-08T10:01:30Z"},
			}})
		case r.URL.Path == "/graphql" && r.Method == http.MethodPost:
			node := map[string]any{
				"number": 9, "title": "t", "body": "", "state": "OPEN", "isDraft": true,
				"headRefOid": ciTestSHA, "headRefName": "feat/x", "baseRefName": "main",
				"reviews": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []any{}},
				"labels":  map[string]any{"nodes": []any{}},
				"commits": map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{
					"statusCheckRollup": map[string]any{"contexts": map[string]any{"nodes": []any{
						map[string]any{"__typename": "CheckRun", "databaseId": s.runID, "name": "build",
							"status": "COMPLETED", "conclusion": "SUCCESS",
							"startedAt": "2026-10-08T10:00:00Z", "completedAt": "2026-10-08T10:01:30Z"},
						map[string]any{"__typename": "StatusContext", "context": "external-scan",
							"state": "SUCCESS", "createdAt": "2026-10-08T10:02:00Z"},
					}}},
				}}}},
			}
			_ = enc.Encode(map[string]any{"data": map[string]any{"repository": map[string]any{
				"pullRequests": map[string]any{"nodes": []any{node}}}}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *ciGitHubServer) forge() *GitHubForge {
	return &GitHubForge{Token: "stub", BaseURL: s.srv.URL, Client: s.srv.Client()}
}

func (s *ciGitHubServer) requests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

func TestCICheckHistory_Dedupe(t *testing.T) {
	t.Run("same run read twice is one line", func(t *testing.T) {
		dir, _ := ciSetup(t)
		inner := &ciFake{checks: &ChecksAtHead{CheckRuns: []CheckRun{
			ciRun("101", "build", "completed", "success", "2026-10-08T10:00:00Z", "2026-10-08T10:01:30Z")}}}
		f := OutboundChecked(inner, "worker-desk")
		for i := 0; i < 2; i++ {
			if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
				t.Fatal(err)
			}
		}
		// A fresh process (empty in-memory set) must find the key in the live file.
		ciResetProcessState()
		if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
			t.Fatal(err)
		}
		if n := len(ciDiskLines(t, dir)); n != 1 {
			t.Errorf("lines on disk = %d, want 1", n)
		}
	})

	t.Run("REST and GraphQL reads of one run share a key", func(t *testing.T) {
		dir, _ := ciSetup(t)
		srv := newCIGitHubServer(t, 4242)
		f := OutboundChecked(srv.forge(), "worker-desk")
		if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
			t.Fatal(err)
		}
		if _, err := f.ListOpenChanges(ciTestRepo); err != nil {
			t.Fatal(err)
		}
		if _, err := f.ReviewQueueSnapshot(ciTestRepo); err != nil {
			t.Fatal(err)
		}
		recs := ciDiskLines(t, dir)
		if len(recs) != 2 {
			t.Fatalf("lines on disk = %d, want 2 (one run + one status, each read three ways): %+v", len(recs), recs)
		}
		for _, r := range recs {
			if r.Kind == CICheckKindRun && r.Attempt != "4242" {
				t.Errorf("run attempt = %q, want the forge run id 4242", r.Attempt)
			}
		}
	})

	t.Run("a re-run is a new attempt", func(t *testing.T) {
		dir, _ := ciSetup(t)
		inner := &ciFake{checks: &ChecksAtHead{CheckRuns: []CheckRun{
			ciRun("101", "build", "completed", "failure", "2026-10-08T10:00:00Z", "2026-10-08T10:01:00Z")}}}
		f := OutboundChecked(inner, "worker-desk")
		if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
			t.Fatal(err)
		}
		inner.checks = &ChecksAtHead{CheckRuns: []CheckRun{
			ciRun("103", "build", "completed", "success", "2026-10-08T10:05:00Z", "2026-10-08T10:06:00Z")}}
		if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
			t.Fatal(err)
		}
		recs := ciDiskLines(t, dir)
		if len(recs) != 2 || recs[0].Attempt != "101" || recs[1].Attempt != "103" {
			t.Errorf("want two attempts 101 then 103, got %+v", recs)
		}
	})

	t.Run("a duplicate across a day rotation loads once", func(t *testing.T) {
		dir, _ := ciSetup(t)
		inner := &ciFake{checks: &ChecksAtHead{CheckRuns: []CheckRun{
			ciRun("101", "build", "completed", "success", "2026-10-08T10:00:00Z", "2026-10-08T10:01:30Z")}}}
		f := OutboundChecked(inner, "worker-desk")
		if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
			t.Fatal(err)
		}
		// The live file's last append was "yesterday": the next write rotates it away, so the
		// writer's live-file dedupe cannot see the key and writes it again.
		yesterday := time.Now().Add(-36 * time.Hour)
		if err := os.Chtimes(filepath.Join(dir, ciChecksFile), yesterday, yesterday); err != nil {
			t.Fatal(err)
		}
		ciResetProcessState()
		if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
			t.Fatal(err)
		}
		seg := filepath.Join(dir, ciChecksFile+"."+yesterday.UTC().Format("2006-01-02"))
		if _, err := os.Stat(seg); err != nil {
			t.Fatalf("the live file was not rotated to %s: %v", filepath.Base(seg), err)
		}
		if n := len(ciDiskLines(t, dir)); n != 2 {
			t.Fatalf("lines on disk = %d, want 2 (one per day)", n)
		}
		got, err := LoadCIChecks()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Errorf("LoadCIChecks returned %d records, want 1: %+v", len(got), got)
		}
	})
}

// --- row 4 ---------------------------------------------------------------------------------

func TestCICheckHistory_NoExtraForgeReads(t *testing.T) {
	reads := map[string]func(Forge) error{
		"ChecksAtHead": func(f Forge) error { _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); return err },
		"ListOpenChanges": func(f Forge) error {
			_, err := f.ListOpenChanges(ciTestRepo)
			return err
		},
		"ReviewQueueSnapshot": func(f Forge) error {
			_, err := f.ReviewQueueSnapshot(ciTestRepo)
			return err
		},
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			// Baseline: the bare backend, no decorator, no recorder.
			ciSetup(t)
			bare := newCIGitHubServer(t, 4242)
			if err := read(bare.forge()); err != nil {
				t.Fatal(err)
			}

			// Recording enabled, writable state dir: records land.
			dir, _ := ciSetup(t)
			on := newCIGitHubServer(t, 4242)
			if err := read(OutboundChecked(on.forge(), "worker-desk")); err != nil {
				t.Fatal(err)
			}
			if len(ciDiskLines(t, dir)) == 0 {
				t.Errorf("no records landed with the recorder enabled")
			}

			// State dir unwritable: the recorder fails, the read does not.
			_, warns := ciSetup(t)
			ciUnwritable(t)
			off := newCIGitHubServer(t, 4242)
			if err := read(OutboundChecked(off.forge(), "worker-desk")); err != nil {
				t.Fatal(err)
			}
			if len(*warns) != 1 {
				t.Errorf("want one recorder warning with the state dir unwritable, got %q", *warns)
			}

			if bare.requests() != on.requests() || on.requests() != off.requests() {
				t.Errorf("forge requests: bare %d, recording %d, recorder failing %d — recording must add none",
					bare.requests(), on.requests(), off.requests())
			}
		})
	}
}

// --- row 5 ---------------------------------------------------------------------------------

// ciFailureModes each break the recorder a different way and return a check that it did not
// write. The read must be unaffected by every one.
func ciFailureModes() map[string]func(t *testing.T, dir string) {
	return map[string]func(t *testing.T, dir string){
		"unwritable state dir": func(t *testing.T, _ string) { ciUnwritable(t) },
		"held lock": func(t *testing.T, dir string) {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			lf, err := os.OpenFile(filepath.Join(dir, ciChecksLock), os.O_CREATE|os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			if err := TryLockExclusive(lf); err != nil {
				t.Fatalf("take the lock: %v", err)
			}
			t.Cleanup(func() { _ = UnlockFile(lf); _ = lf.Close() })
			ciLockWait = 100 * time.Millisecond
		},
		"recorder panic": func(t *testing.T, _ string) {
			ciNow = func() time.Time { panic("recorder fault injected by the test") }
		},
	}
}

func TestCICheckHistory_ReadUnchangedOnRecorderFailure(t *testing.T) {
	completed := []CheckRun{ciRun("101", "build", "completed", "success", "2026-10-08T10:00:00Z", "2026-10-08T10:01:30Z")}
	rollup := []RollupNode{{Typename: "CheckRun", ID: "101", Name: "build", Status: "COMPLETED",
		Conclusion: "SUCCESS", StartedAt: "2026-10-08T10:00:00Z", CompletedAt: "2026-10-08T10:01:30Z"}}
	change := OpenChange{Number: 9, HeadSHA: ciTestSHA, Rollup: rollup}

	for mode, breakIt := range ciFailureModes() {
		t.Run(mode, func(t *testing.T) {
			dir, warns := ciSetup(t)
			breakIt(t, dir)
			inner := &ciFake{
				checks: &ChecksAtHead{CheckRuns: completed},
				open:   &OpenChanges{Changes: []OpenChange{change}},
				queue:  &ReviewQueue{Changes: []QueuedChange{{OpenChange: change}}},
			}
			f := OutboundChecked(inner, "worker-desk")

			start := time.Now()
			c, err := f.ChecksAtHead(ciTestRepo, ciTestSHA)
			if c != inner.checks || err != nil {
				t.Errorf("ChecksAtHead = (%p, %v), want the inner (%p, nil)", c, err, inner.checks)
			}
			o, err := f.ListOpenChanges(ciTestRepo)
			if o != inner.open || err != nil {
				t.Errorf("ListOpenChanges = (%p, %v), want the inner (%p, nil)", o, err, inner.open)
			}
			q, err := f.ReviewQueueSnapshot(ciTestRepo)
			if q != inner.queue || err != nil {
				t.Errorf("ReviewQueueSnapshot = (%p, %v), want the inner (%p, nil)", q, err, inner.queue)
			}
			// Three reads, each bounded by one lock wait at most.
			if el := time.Since(start); el > 3*ciLockWait+time.Second {
				t.Errorf("three reads took %s — the recorder must give up after a bounded wait", el)
			}
			if inner.calls != 3 {
				t.Errorf("inner calls = %d, want 3", inner.calls)
			}
			if len(*warns) != 1 {
				t.Errorf("want exactly one recorder warning per process, got %q", *warns)
			}
			if mode != "unwritable state dir" {
				if n := len(ciDiskLines(t, dir)); n != 0 {
					t.Errorf("the broken recorder still wrote %d line(s)", n)
				}
			}
		})
	}

	t.Run("inner read error", func(t *testing.T) {
		dir, warns := ciSetup(t)
		sentinel := errors.New("forge said no")
		inner := &ciFake{
			checks: &ChecksAtHead{CheckRuns: completed},
			open:   &OpenChanges{Changes: []OpenChange{change}},
			queue:  &ReviewQueue{Changes: []QueuedChange{{OpenChange: change}}},
			err:    sentinel,
		}
		f := OutboundChecked(inner, "worker-desk")
		c, err := f.ChecksAtHead(ciTestRepo, ciTestSHA)
		if err != sentinel || c != inner.checks {
			t.Errorf("ChecksAtHead = (%p, %v), want the inner (%p, %v)", c, err, inner.checks, sentinel)
		}
		o, err := f.ListOpenChanges(ciTestRepo)
		if err != sentinel || o != inner.open {
			t.Errorf("ListOpenChanges = (%p, %v), want the inner (%p, %v)", o, err, inner.open, sentinel)
		}
		q, err := f.ReviewQueueSnapshot(ciTestRepo)
		if err != sentinel || q != inner.queue {
			t.Errorf("ReviewQueueSnapshot = (%p, %v), want the inner (%p, %v)", q, err, inner.queue, sentinel)
		}
		if _, statErr := os.Stat(filepath.Join(dir, ciChecksFile)); !os.IsNotExist(statErr) {
			t.Errorf("a failed read left a record file (stat err %v)", statErr)
		}
		if len(*warns) != 0 {
			t.Errorf("a failed read reached the recorder: %q", *warns)
		}
	})
}

// --- row 7 ---------------------------------------------------------------------------------

// ciSchemaDoc is the canonical schema, which ships beside this package, read here so the
// record type and the document cannot drift apart.
const ciSchemaDoc = "ci-check-v1.md"

// ciDocFields returns the backticked first-column names of the doc's "## Fields" table.
func ciDocFields(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(ciSchemaDoc)
	if err != nil {
		t.Fatalf("read the schema doc: %v", err)
	}
	var out []string
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(line, "## Fields"):
			in = true
			continue
		case in && (strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ")):
			in = false
		}
		if !in || !strings.HasPrefix(line, "| `") {
			continue
		}
		cell := strings.TrimSpace(strings.SplitN(strings.TrimPrefix(line, "|"), "|", 2)[0])
		out = append(out, strings.Trim(cell, "`"))
	}
	if len(out) == 0 {
		t.Fatalf("no field rows found under \"## Fields\" in %s", ciSchemaDoc)
	}
	sort.Strings(out)
	return out
}

func ciTypeFields() []string {
	var out []string
	rt := reflect.TypeOf(CICheckRecord{})
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			name = rt.Field(i).Name
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func TestCICheckRecord_NoFreeTextFields(t *testing.T) {
	doc, typ := ciDocFields(t), ciTypeFields()
	if !reflect.DeepEqual(doc, typ) {
		t.Fatalf("CICheckRecord JSON fields %v != the ci-check-v1.md field table %v — a field is added in "+
			"one place and not the other", typ, doc)
	}
	// Belt and braces: the never-collected list, by name. The exact-set check above is what
	// actually binds; this names the class a future edit must not add.
	for _, f := range typ {
		for _, banned := range []string{"output", "title", "summary", "annotation", "log", "url", "actor",
			"app", "user", "login", "author", "transcript", "text", "message", "body"} {
			if strings.Contains(strings.ToLower(f), banned) {
				t.Errorf("field %q carries free text or an identity (%q) — never collected", f, banned)
			}
		}
	}
	// Every field is a string or an int (a *int for the optional duration): no nested object
	// or slice that could smuggle free text past the name check.
	rt := reflect.TypeOf(CICheckRecord{})
	for i := 0; i < rt.NumField(); i++ {
		k := rt.Field(i).Type.Kind()
		if k == reflect.Pointer {
			k = rt.Field(i).Type.Elem().Kind()
		}
		if k != reflect.String && k != reflect.Int {
			t.Errorf("field %s has kind %s — only scalar string/int fields are allowed", rt.Field(i).Name, k)
		}
	}
}

// --- helpers under test ---------------------------------------------------------------------

func TestCIDurationS(t *testing.T) {
	for _, c := range []struct {
		s, e string
		want string
	}{
		{"2026-10-08T10:00:00Z", "2026-10-08T10:01:30Z", "90"},
		{"2026-10-08T10:00:00Z", "2026-10-08T10:00:00Z", "0"},
		{"", "2026-10-08T10:00:00Z", "nil"},
		{"2026-10-08T10:00:00Z", "", "nil"},
		{"not-a-time", "2026-10-08T10:00:00Z", "nil"},
		{"2026-10-08T10:01:00Z", "2026-10-08T10:00:00Z", "nil"},
	} {
		got := "nil"
		if d := ciDurationS(c.s, c.e); d != nil {
			got = fmt.Sprint(*d)
		}
		if got != c.want {
			t.Errorf("ciDurationS(%q, %q) = %s, want %s", c.s, c.e, got, c.want)
		}
	}
}

func TestCISegmentsDisjointFromAudit(t *testing.T) {
	for _, n := range []string{"ci-checks.jsonl.2026-10-08", "ci-checks.jsonl.2026-10-08.2"} {
		if !ciSegmentPattern.MatchString(n) || segmentPattern.MatchString(n) {
			t.Errorf("%s: ci pattern %v, audit pattern %v — want ci only", n,
				ciSegmentPattern.MatchString(n), segmentPattern.MatchString(n))
		}
	}
	for _, n := range []string{"audit.jsonl.2026-10-08", "ci-checks.jsonl", "ci-checks.lock"} {
		if ciSegmentPattern.MatchString(n) {
			t.Errorf("%s matches the CI segment pattern", n)
		}
	}
}

func TestSetCICheckRecording(t *testing.T) {
	dir, _ := ciSetup(t)
	restore := SetCICheckRecording(false)
	f := OutboundChecked(&ciFake{checks: &ChecksAtHead{CheckRuns: []CheckRun{
		ciRun("101", "build", "completed", "success", "", "")}}}, "worker-desk")
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	if n := len(ciDiskLines(t, dir)); n != 0 {
		t.Errorf("recording disabled, yet %d line(s) written", n)
	}
	restore()
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	if n := len(ciDiskLines(t, dir)); n != 1 {
		t.Errorf("recording restored, %d line(s) written, want 1", n)
	}
}

// --- review round 1 (findings B1, A1-A5, S1-S5 on the change that added this file) ----------

// ciWriteLive writes raw content as the live file, creating the state dir.
func ciWriteLive(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ciChecksFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// ciLine renders one valid record line for a run with the given id (no trailing newline).
func ciLine(t *testing.T, id string) string {
	t.Helper()
	rec, ok := ciRunRecord(ciBase(ciTestRepo, ciTestSHA, 0, time.Now()), id, "seed", "completed",
		"success", "2026-10-08T08:00:00Z", "2026-10-08T08:00:05Z")
	if !ok {
		t.Fatal("seed record not built")
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestCIConclusionFlipKept — one forge object whose terminal state changes under the same
// attempt (a GitLab pipeline retried inside the same pipeline; a run rewritten under one id)
// keeps each distinct conclusion, in-process and across processes, on write and on read.
//
// regression: #2423
func TestCIConclusionFlipKept(t *testing.T) {
	dir, _ := ciSetup(t)
	at := func(conc string) *ChecksAtHead {
		return &ChecksAtHead{
			CheckRuns: []CheckRun{ciRun("101", "build", "completed", conc, "2026-10-08T10:00:00Z", "2026-10-08T10:01:00Z")},
			Statuses:  []StatusContext{{Context: "pipeline", State: conc, CreatedAt: "2026-10-08T10:00:00Z"}},
		}
	}
	inner := &ciFake{checks: at("failure")}
	f := OutboundChecked(inner, "worker-desk")
	read := func() {
		t.Helper()
		if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
			t.Fatal(err)
		}
	}
	read()
	inner.checks = at("success")
	read() // same process
	ciResetProcessState()
	read() // fresh process, same success: deduped against the live file
	inner.checks = at("failure")
	read() // back to the first conclusion: deduped, first occurrence wins

	recs := ciDiskLines(t, dir)
	if len(recs) != 4 {
		t.Fatalf("lines on disk = %d, want 4 (failure then success for the run and the status): %+v", len(recs), recs)
	}
	got, err := LoadCIChecks()
	if err != nil {
		t.Fatal(err)
	}
	seq := map[string][]string{}
	for _, r := range got {
		seq[r.Kind] = append(seq[r.Kind], r.Conclusion)
	}
	for _, kind := range []string{CICheckKindRun, CICheckKindStatus} {
		if !reflect.DeepEqual(seq[kind], []string{"failure", "success"}) {
			t.Errorf("%s conclusions loaded = %v, want [failure success]", kind, seq[kind])
		}
	}
}

// TestCILockTimeoutLatches — after one lock timeout the process stops recording, so a stuck
// holder costs one bounded wait per process, not one per read.
//
// regression: #2423
func TestCILockTimeoutLatches(t *testing.T) {
	dir, warns := ciSetup(t)
	ciLockWait = 100 * time.Millisecond
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	lf, err := os.OpenFile(filepath.Join(dir, ciChecksLock), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := TryLockExclusive(lf); err != nil {
		t.Fatalf("take the lock: %v", err)
	}
	inner := &ciFake{checks: &ChecksAtHead{CheckRuns: []CheckRun{
		ciRun("101", "build", "completed", "success", "2026-10-08T10:00:00Z", "2026-10-08T10:01:00Z")}}}
	f := OutboundChecked(inner, "worker-desk")
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	_ = UnlockFile(lf)
	_ = lf.Close()

	// The lock is free now, but this process already gave up: nothing is written.
	inner.checks = &ChecksAtHead{CheckRuns: []CheckRun{
		ciRun("102", "build", "completed", "success", "2026-10-08T10:02:00Z", "2026-10-08T10:03:00Z")}}
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ciChecksFile)); !os.IsNotExist(err) {
		t.Errorf("recording continued after a lock timeout in the same process (stat err %v): %+v", err, ciDiskLines(t, dir))
	}
	if len(*warns) != 1 {
		t.Errorf("want exactly one warning, got %q", *warns)
	}
	// A fresh process records again.
	ciResetProcessState()
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	if n := len(ciDiskLines(t, dir)); n != 1 {
		t.Errorf("a fresh process wrote %d line(s), want 1", n)
	}
}

// TestCITornTailRepaired — a live file left without a final newline (a partial write) does
// not swallow the next batch's first record.
//
// regression: #2423
func TestCITornTailRepaired(t *testing.T) {
	dir, warns := ciSetup(t)
	ciWriteLive(t, dir, ciLine(t, "900")+"\n"+`{"schema":"ci-check-v1","name":"tor`)
	f := OutboundChecked(&ciFake{checks: &ChecksAtHead{CheckRuns: []CheckRun{
		ciRun("101", "build", "completed", "success", "2026-10-08T10:00:00Z", "2026-10-08T10:01:00Z")}}}, "worker-desk")
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCIChecks()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("LoadCIChecks = %d records, want 2 (the seed and the new run): %+v", len(got), got)
	}
	if len(*warns) != 0 {
		t.Errorf("unexpected warning: %q", *warns)
	}
}

// TestCIOverlongLineSkipped — a line over the scan cap is skipped on read (the writer keeps
// recording, LoadCIChecks keeps loading), and the writer never produces one.
//
// regression: #2423
func TestCIOverlongLineSkipped(t *testing.T) {
	dir, warns := ciSetup(t)
	ciWriteLive(t, dir, strings.Repeat("x", (1<<20)+16)+"\n"+ciLine(t, "900")+"\n")
	long := strings.Repeat("n", 70_000)
	f := OutboundChecked(&ciFake{checks: &ChecksAtHead{CheckRuns: []CheckRun{
		ciRun("101", "build", "completed", "success", "2026-10-08T10:00:00Z", "2026-10-08T10:01:00Z"),
		ciRun("102", long, "completed", "success", "2026-10-08T10:00:00Z", "2026-10-08T10:01:00Z"),
	}}}, "worker-desk")
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	if len(*warns) != 0 {
		t.Errorf("an over-long line on disk stopped the writer: %q", *warns)
	}
	got, err := LoadCIChecks()
	if err != nil {
		t.Fatalf("LoadCIChecks failed on an over-long line: %v", err)
	}
	var names []string
	for _, r := range got {
		if len(r.Name) > 64 {
			names = append(names, fmt.Sprintf("<%d chars>", len(r.Name)))
			continue
		}
		names = append(names, r.Name)
	}
	if !reflect.DeepEqual(names, []string{"seed", "build"}) {
		t.Errorf("loaded names = %v, want [seed build] (the over-long record is never written)", names)
	}
}

// TestCIWarningOmitsPath — the once-per-process warning names a file, never a directory path.
//
// regression: #2423
func TestCIWarningOmitsPath(t *testing.T) {
	_, warns := ciSetup(t)
	ciUnwritable(t)
	f := OutboundChecked(&ciFake{checks: &ChecksAtHead{CheckRuns: []CheckRun{
		ciRun("101", "build", "completed", "success", "", "")}}}, "worker-desk")
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	if len(*warns) != 1 {
		t.Fatalf("want one warning, got %q", *warns)
	}
	root := filepath.Dir(filepath.Dir(dirOverride)) // the test's temp dir
	if strings.Contains((*warns)[0], root) || strings.Contains((*warns)[0], string(filepath.Separator)) {
		t.Errorf("warning carries a path: %q", (*warns)[0])
	}
}

// TestCILiveFileHardened — the live file is only ever a regular file of mode 0600: a symlink
// at its path is refused (nothing is written through it), and a wider mode is tightened.
//
// regression: #2423
func TestCILiveFileHardened(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes and symlinks")
	}
	run := &ChecksAtHead{CheckRuns: []CheckRun{ciRun("101", "build", "completed", "success", "", "")}}

	t.Run("symlink refused", func(t *testing.T) {
		dir, warns := ciSetup(t)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "elsewhere")
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, ciChecksFile)); err != nil {
			t.Fatal(err)
		}
		if _, err := OutboundChecked(&ciFake{checks: run}, "worker-desk").ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(target); len(b) != 0 {
			t.Errorf("the recorder wrote through a symlink: %q", b)
		}
		if len(*warns) != 1 {
			t.Errorf("want one warning, got %q", *warns)
		}
	})

	t.Run("wide mode tightened", func(t *testing.T) {
		dir, _ := ciSetup(t)
		ciWriteLive(t, dir, "")
		if err := os.Chmod(filepath.Join(dir, ciChecksFile), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := OutboundChecked(&ciFake{checks: run}, "worker-desk").ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(filepath.Join(dir, ciChecksFile))
		if err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("live file mode = %v (err %v), want 0600", fi.Mode().Perm(), err)
		}
		if lf, err := os.Stat(filepath.Join(dir, ciChecksLock)); err != nil || lf.Mode().Perm() != 0o600 {
			t.Errorf("lock file mode = %v (err %v), want 0600", lf.Mode().Perm(), err)
		}
	})
}

// TestCIRecordGuards — the keyless and SHA-less guards, one by one.
//
// regression: #2423
func TestCIRecordGuards(t *testing.T) {
	base := ciBase(ciTestRepo, ciTestSHA, 0, time.Now())
	if _, ok := ciStatusRecord(base, "ctx", "success", ""); ok {
		t.Error("a status with no created_at was recorded")
	}
	if _, ok := ciStatusRecord(base, "", "success", "2026-10-08T10:00:00Z"); ok {
		t.Error("a status with no context was recorded")
	}
	if _, ok := ciRunRecord(base, "101", "", "completed", "success", "", ""); ok {
		t.Error("a run with no name was recorded")
	}
	if _, ok := ciRunRecord(base, "", "build", "completed", "success", "", ""); ok {
		t.Error("a run with neither an id nor a start stamp was recorded")
	}
	now := time.Now()
	checks := &ChecksAtHead{CheckRuns: []CheckRun{ciRun("101", "build", "completed", "success", "", "")}}
	if got := ciRecordsFromChecks(ciTestRepo, "", checks, now); len(got) != 0 {
		t.Errorf("a ChecksAtHead read with no SHA was recorded: %+v", got)
	}
	oc := OpenChange{Number: 1, Rollup: []RollupNode{{Typename: "CheckRun", ID: "101", Name: "build", Status: "COMPLETED", Conclusion: "SUCCESS"}}}
	if got := ciRecordsFromRollup(ciTestRepo, oc, now); len(got) != 0 {
		t.Errorf("a rollup with no head SHA was recorded: %+v", got)
	}
}

// TestCILoadSegmentsAndForeign — LoadCIChecks reads rotated segments, oldest first, and skips
// a line carrying another schema tag.
//
// regression: #2423
func TestCILoadSegmentsAndForeign(t *testing.T) {
	dir, _ := ciSetup(t)
	ciWriteLive(t, dir, `{"schema":"other-v1","repo":"acme/widgets","head_sha":"x","kind":"check-run","name":"n","attempt":"1","conclusion":"success"}`+"\n"+ciLine(t, "903")+"\n")
	for name, id := range map[string]string{"ci-checks.jsonl.2026-10-02": "902", "ci-checks.jsonl.2026-10-01": "901"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(ciLine(t, id)+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := LoadCIChecks()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range got {
		ids = append(ids, r.Attempt)
	}
	if !reflect.DeepEqual(ids, []string{"901", "902", "903"}) {
		t.Errorf("loaded attempts = %v, want [901 902 903] (segments oldest first, then live; foreign line skipped)", ids)
	}
}

// TestCIRotationKeepsSegment — a rotation whose target name is taken writes a .N segment and
// never overwrites the existing one.
//
// regression: #2423
func TestCIRotationKeepsSegment(t *testing.T) {
	dir, _ := ciSetup(t)
	yesterday := time.Now().Add(-36 * time.Hour)
	seg := filepath.Join(dir, ciChecksFile+"."+yesterday.UTC().Format("2006-01-02"))
	ciWriteLive(t, dir, ciLine(t, "902")+"\n")
	if err := os.WriteFile(seg, []byte(ciLine(t, "901")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(dir, ciChecksFile), yesterday, yesterday); err != nil {
		t.Fatal(err)
	}
	f := OutboundChecked(&ciFake{checks: &ChecksAtHead{CheckRuns: []CheckRun{
		ciRun("101", "build", "completed", "success", "", "")}}}, "worker-desk")
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(seg); err != nil || !strings.Contains(string(b), `"attempt":"901"`) {
		t.Errorf("the existing segment was overwritten or lost: %q (err %v)", b, err)
	}
	if b, err := os.ReadFile(seg + ".1"); err != nil || !strings.Contains(string(b), `"attempt":"902"`) {
		t.Errorf("the rotated live file is not at %s.1: %q (err %v)", filepath.Base(seg), b, err)
	}
	if got, err := LoadCIChecks(); err != nil || len(got) != 3 {
		t.Errorf("LoadCIChecks = %d records (err %v), want 3", len(got), err)
	}
}

// TestCIUnreadLiveNotSeen — a batch dropped because the live file could not be read leaves
// its keys unmarked, so a later read in the same process records them.
//
// regression: #2423
func TestCIUnreadLiveNotSeen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory read semantics")
	}
	dir, warns := ciSetup(t)
	live := filepath.Join(dir, ciChecksFile)
	if err := os.MkdirAll(live, 0o700); err != nil { // a directory where the file belongs
		t.Fatal(err)
	}
	f := OutboundChecked(&ciFake{checks: &ChecksAtHead{CheckRuns: []CheckRun{
		ciRun("101", "build", "completed", "success", "", "")}}}, "worker-desk")
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	if len(*warns) != 1 {
		t.Fatalf("want one warning for the unreadable live file, got %q", *warns)
	}
	if err := os.Remove(live); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ChecksAtHead(ciTestRepo, ciTestSHA); err != nil {
		t.Fatal(err)
	}
	if n := len(ciDiskLines(t, dir)); n != 1 {
		t.Errorf("lines on disk = %d, want 1 — the failed batch's key was marked seen", n)
	}
}
