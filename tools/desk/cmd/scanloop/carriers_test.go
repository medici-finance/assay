package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// carriers_test.go — a fresh scan carrier carries only its own delta (issue 6292).
//
// The defect: an open scan carrier past the coalesce window is sealed, and the next pass cut a
// fresh branch from the default branch and wrote EVERY placeholder the default branch lacked —
// including the ones already on the sealed, unmerged carriers. Each carrier was a superset of the
// last, and they conflicted add/add once one merged.
//
// These tests drive whole passes against a model of the target repo and its forge. The model
// keeps the state the defect turned on: which placeholders each OPEN carrier branch carries, and
// what `statusgen --scan-issues` writes into a worktree cut from the default branch. The forge is
// served over httptest through the real GitLab and GitHub backends — no live forge call.

// placeholderOf is the scan-directory path the model's scanner writes for an issue.
func placeholderOf(issue int) string {
	return fmt.Sprintf("%s/issue-%d.md", deskkit.ScanDir, issue)
}

type worldCarrier struct {
	branch  string
	files   map[string]bool
	open    bool
	created time.Time
}

type worldTree struct {
	branch    string
	base      map[string]bool // what the branch it was cut from already carries
	dirty     map[string]bool // written by the scanner, not yet committed
	staged    map[string]bool
	committed map[string]bool
}

// carrierWorld models the scan target: issues lacking a placeholder on the default branch, the
// scan carriers on the forge, and the worktrees the lane cuts.
type carrierWorld struct {
	t         *testing.T
	mu        sync.Mutex
	login     string
	issues    []int
	carriers  map[int]*worldCarrier
	next      int
	now       time.Time
	trees     map[string]*worldTree
	listFail  bool
	filesFail bool
	scans     int
	// noCoalesce wires a pass as `scanloop run` does with coalescing disabled: no open scan PR is
	// read, so every pass is fresh — the shape that built the supersets fastest.
	noCoalesce bool
}

func newCarrierWorld(t *testing.T) *carrierWorld {
	return &carrierWorld{t: t, login: "assay-issue-loop-app", carriers: map[int]*worldCarrier{},
		next: 500, trees: map[string]*worldTree{}}
}

func pathsAfterDashes(args []string) []string {
	for i, a := range args {
		if a == "--" {
			return args[i+1:]
		}
	}
	return nil
}

func (w *carrierWorld) exec(dir, name string, args ...string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if name == "git" && len(args) > 0 && args[0] == "--literal-pathspecs" {
		args = args[1:]
	}
	switch name {
	case "git":
		switch {
		case args[0] == "worktree" && args[1] == "add":
			// worktree add <path> -b|-B <branch> <cut-from>
			tr := &worldTree{branch: args[4], base: map[string]bool{}, dirty: map[string]bool{},
				staged: map[string]bool{}, committed: map[string]bool{}}
			if from := strings.TrimPrefix(args[5], "origin/"); from != args[5] && from != "main" {
				for _, c := range w.carriers {
					if c.branch == from {
						for p := range c.files {
							tr.base[p] = true
						}
					}
				}
			}
			w.trees[args[2]] = tr
		case args[0] == "worktree" && args[1] == "remove":
			delete(w.trees, args[2])
		case args[0] == "status":
			var paths []string
			for p := range w.trees[dir].dirty {
				paths = append(paths, p)
			}
			sort.Strings(paths)
			var b strings.Builder
			for _, p := range paths {
				b.WriteString("?? " + p + "\x00")
			}
			return b.String(), nil
		case args[0] == "clean" || args[0] == "checkout":
			for _, p := range pathsAfterDashes(args) {
				delete(w.trees[dir].dirty, p)
			}
		case args[0] == "add":
			tr := w.trees[dir]
			for p := range tr.dirty {
				tr.staged[p] = true
			}
		case args[0] == "commit":
			tr := w.trees[dir]
			if len(tr.staged) == 0 {
				return "", fmt.Errorf("nothing to commit, working tree clean")
			}
			for p := range tr.staged {
				tr.committed[p] = true
				delete(tr.dirty, p)
			}
			tr.staged = map[string]bool{}
		}
	case "statusgen":
		w.scans++
		tr := w.trees[dir]
		for _, n := range w.issues {
			if p := placeholderOf(n); !tr.base[p] && !tr.committed[p] {
				tr.dirty[p] = true
			}
		}
	case "deskscanbody":
		if args[0] == "emit" {
			return "intake scan: placeholders", nil
		}
	case "deskpr":
		tr := w.trees[dir]
		switch args[0] {
		case "create":
			w.next++
			c := &worldCarrier{branch: tr.branch, files: map[string]bool{}, open: true, created: w.now}
			for p := range tr.committed {
				c.files[p] = true
			}
			w.carriers[w.next] = c
		case "update":
			for _, c := range w.carriers {
				if c.branch == tr.branch {
					for p := range tr.committed {
						c.files[p] = true
					}
				}
			}
		}
	}
	return "", nil
}

// newest is the most recently created carrier's number.
func (w *carrierWorld) newest() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	best := 0
	for n := range w.carriers {
		if n > best {
			best = n
		}
	}
	return best
}

func (w *carrierWorld) filesOf(n int) []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	if c := w.carriers[n]; c != nil {
		for p := range c.files {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// openSorted is every open carrier, newest first, as the forge lists them.
func (w *carrierWorld) openSorted() []int {
	var ns []int
	for n, c := range w.carriers {
		if c.open {
			ns = append(ns, n)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ns)))
	return ns
}

var (
	glDiffsRe = regexp.MustCompile(`/merge_requests/([0-9]+)/diffs$`)
	ghFilesRe = regexp.MustCompile(`^/repos/medici-finance/assay/pulls/([0-9]+)/files$`)
)

// gitlab serves the world's merge requests through GitLab's REST shape.
func (w *carrierWorld) gitlab(rw http.ResponseWriter, r *http.Request) {
	w.mu.Lock()
	defer w.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/merge_requests"):
		if w.listFail {
			rw.WriteHeader(http.StatusBadGateway)
			return
		}
		// GitLab has no "[bot]" decoration: the author is whatever login the roster binds the role
		// to, served as the username — the same identity the discovery compares against.
		login, ok := deskkit.RoleAppLogin("issue-loop")
		if !ok {
			w.t.Errorf("the fixture roster binds no issue-loop App")
		}
		var mrs []map[string]any
		for _, n := range w.openSorted() {
			c := w.carriers[n]
			mrs = append(mrs, map[string]any{
				"id": n, "iid": n, "title": "Draft: chore(intake): scan", "state": "opened", "draft": true,
				"source_branch": c.branch, "target_branch": "main",
				"created_at": c.created.UTC().Format(time.RFC3339),
				"author":     map[string]any{"id": 300000003, "username": login},
			})
		}
		_ = json.NewEncoder(rw).Encode(mrs)
	case r.Method == http.MethodGet && glDiffsRe.MatchString(r.URL.Path):
		if w.filesFail {
			rw.WriteHeader(http.StatusBadGateway)
			return
		}
		n, _ := strconv.Atoi(glDiffsRe.FindStringSubmatch(r.URL.Path)[1])
		var diffs []map[string]any
		for p := range w.carriers[n].files {
			diffs = append(diffs, map[string]any{"old_path": p, "new_path": p, "new_file": true, "diff": "+x\n"})
		}
		_ = json.NewEncoder(rw).Encode(diffs)
	default:
		w.t.Errorf("unexpected GitLab request %s %s", r.Method, r.URL.Path)
		rw.WriteHeader(http.StatusNotFound)
	}
}

// github serves the same world through GitHub's GraphQL list and REST files shapes.
func (w *carrierWorld) github(rw http.ResponseWriter, r *http.Request) {
	w.mu.Lock()
	defer w.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/repos/medici-finance/assay/issues/"):
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"number": 161, "title": "an inbound issue", "state": "open",
			"user": map[string]any{"login": "ada", "id": 2001, "type": "User"},
		})
	case r.Method == http.MethodPost && r.URL.Path == "/graphql":
		if w.listFail {
			rw.WriteHeader(http.StatusBadGateway)
			return
		}
		var nodes []map[string]any
		for _, n := range w.openSorted() {
			c := w.carriers[n]
			nodes = append(nodes, scanChange(n, c.branch, w.login, "Bot", true, c.created))
		}
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"data": map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": nodes}}},
		})
	case r.Method == http.MethodGet && ghFilesRe.MatchString(r.URL.Path):
		if w.filesFail {
			rw.WriteHeader(http.StatusBadGateway)
			return
		}
		n, _ := strconv.Atoi(ghFilesRe.FindStringSubmatch(r.URL.Path)[1])
		var files []map[string]any
		for p := range w.carriers[n].files {
			files = append(files, map[string]any{"filename": p, "status": "added", "patch": "+x"})
		}
		_ = json.NewEncoder(rw).Encode(files)
	default:
		w.t.Errorf("unexpected GitHub request %s %s", r.Method, r.URL.Path)
		rw.WriteHeader(http.StatusNotFound)
	}
}

// serveForge points the open-change and changed-file seams at the world through a real forge
// backend of the named kind.
func (w *carrierWorld) serveForge(t *testing.T, kind string) {
	t.Helper()
	var f deskkit.Forge
	switch kind {
	case "gitlab":
		srv := httptest.NewServer(http.HandlerFunc(w.gitlab))
		t.Cleanup(srv.Close)
		f = &deskkit.GitLabForge{Token: "x-test-token", BaseURL: srv.URL, Client: srv.Client()}
	default:
		srv := httptest.NewServer(http.HandlerFunc(w.github))
		t.Cleanup(srv.Close)
		f = &deskkit.GitHubForge{Token: "x-test-token", BaseURL: srv.URL, Client: srv.Client()}
	}
	fr := deskkit.ForgeRepo{Owner: "medici-finance", Name: "assay"}
	oldList, oldFiles := listOpenChangesFn, listChangedFilesFn
	listOpenChangesFn = func(string) (*deskkit.OpenChanges, error) { return f.ListOpenChanges(fr) }
	listChangedFilesFn = func(_ string, n int) ([]deskkit.ChangedFile, error) { return f.ListChangedFiles(fr, n) }
	t.Cleanup(func() { listOpenChangesFn, listChangedFilesFn = oldList, oldFiles })
	withRoleOnly(t, "issue-loop")
}

// pass runs one drain pass at `at` for a newly inbound issue, wired exactly as `scanloop run` wires
// its forge reads.
func (w *carrierWorld) pass(t *testing.T, issue int, at time.Time) (string, error) {
	t.Helper()
	w.mu.Lock()
	w.issues = append(w.issues, issue)
	w.now = at
	w.mu.Unlock()
	loop := passLoop(t, fmt.Sprintf("INBOUND: medici-finance/assay#%d %s\n", issue,
		at.Add(-time.Minute).UTC().Format(time.RFC3339)))
	loop.DryRun = false
	loop.Exec = w.exec
	loop.Write = func(string, string) error { return nil }
	loop.Now = func() time.Time { return at }
	loop.OpenPR, loop.Carriers = forgeScanReads(loop.ScanTarget, at, CoalescePolicy{}.window())
	loop.CarrierFiles = carrierFilesOn(loop.ScanTarget)
	if w.noCoalesce {
		loop.OpenPR = nil
	}
	var sb strings.Builder
	loop.Emit = &sb
	err := drainPass(passConfig(t), loop, &sb)
	renderPassReport(&sb, loop)
	return sb.String(), err
}

func wantFiles(t *testing.T, w *carrierWorld, n int, issues ...int) {
	t.Helper()
	var want []string
	for _, i := range issues {
		want = append(want, placeholderOf(i))
	}
	sort.Strings(want)
	if got := w.filesOf(n); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("carrier #%d carries %v, want only %v — a fresh carrier must be its own delta", n, got, want)
	}
}

// multiPass is the three-sealed-passes-then-close-unmerged scenario on one forge kind.
func multiPass(t *testing.T, kind string) {
	w := newCarrierWorld(t)
	w.serveForge(t, kind)
	t0 := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	step := 30 * time.Minute // past the coalesce window: every pass seals the previous carrier

	var made []int
	for i, issue := range []int{301, 302, 303} {
		out, err := w.pass(t, issue, t0.Add(time.Duration(i)*step))
		if err != nil || strings.Contains(out, "dispatch-error") {
			t.Fatalf("pass %d errored: %v\n%s", i+1, err, out)
		}
		made = append(made, w.newest())
		wantFiles(t, w, made[i], issue)
	}
	if made[0] == made[1] || made[1] == made[2] {
		t.Fatalf("carriers %v: each sealed pass must open its own carrier", made)
	}

	// The oldest sealed carrier is closed unmerged. The next pass no longer sees it as open, so its
	// placeholder is written again rather than lost.
	w.mu.Lock()
	w.carriers[made[0]].open = false
	w.mu.Unlock()
	out, err := w.pass(t, 304, t0.Add(3*step))
	if err != nil || strings.Contains(out, "dispatch-error") {
		t.Fatalf("pass 4 errored: %v\n%s", err, out)
	}
	wantFiles(t, w, w.newest(), 301, 304)
}

// regression: #6292
func TestCarriers_GitLabOwnDelta(t *testing.T) { multiPass(t, "gitlab") }

// regression: #6292
func TestCarriers_GitHubOwnDelta(t *testing.T) { multiPass(t, "github") }

// failClosed seeds one open sealed carrier, breaks one forge read, and asserts the pass writes
// nothing — never the full delta.
func failClosed(t *testing.T, kind string, breakRead func(*carrierWorld)) {
	w := newCarrierWorld(t)
	w.serveForge(t, kind)
	t0 := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	if out, err := w.pass(t, 401, t0); err != nil || strings.Contains(out, "dispatch-error") {
		t.Fatalf("seed pass errored: %v\n%s", err, out)
	}
	seeded := w.newest()
	breakRead(w)
	out, err := w.pass(t, 402, t0.Add(30*time.Minute))
	if w.newest() != seeded {
		t.Fatalf("a pass blind to the open carriers opened carrier #%d carrying %v:\n%s",
			w.newest(), w.filesOf(w.newest()), out)
	}
	if w.scans != 1 {
		t.Fatalf("scans = %d, want 1 — a blind pass must not even write the delta", w.scans)
	}
	if err == nil && !strings.Contains(out, "dispatch-error") {
		t.Fatalf("a pass blind to the open carriers reported success:\n%s", out)
	}
	if !strings.Contains(out, "could-not-check") && !strings.Contains(out, "cannot read") {
		t.Fatalf("the refusal does not say the carriers were could-not-check:\n%s", out)
	}
}

// With coalescing disabled the open scan PR is never read, so the carrier read is the ONLY forge
// read the pass makes — and its failure must still refuse.
// regression: #6292
func TestCarriers_ListUnreadRefuses(t *testing.T) {
	failClosed(t, "gitlab", func(w *carrierWorld) { w.listFail, w.noCoalesce = true, true })
}

// A list cut at the forge's cap may hide an open carrier, so it is could-not-check, not "none".
// regression: #6292
func TestCarriers_TruncatedRefuses(t *testing.T) {
	r := &scanRead{login: "assay-issue-loop-app[bot]", oc: &deskkit.OpenChanges{Cap: 100, TruncatedAtCap: true}}
	if cs, err := scanCarriersIn(r); err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("a truncated open-change list gave carriers %v, err %v — want could-not-check", cs, err)
	}
}

// Only this App's carriers count. A scan-shaped branch by another account must not be able to
// keep placeholders out of intake.
// regression: #6292
func TestCarriers_ForeignIgnored(t *testing.T) {
	r := &scanRead{login: "assay-issue-loop-app[bot]", oc: &deskkit.OpenChanges{Cap: 100, Changes: []deskkit.OpenChange{
		{Number: 9, HeadRef: scanBranchPrefix + "x", Author: deskkit.Account{Login: "someone-else"}},
		{Number: 8, HeadRef: "fix/other", Author: deskkit.Account{Login: "assay-issue-loop-app[bot]"}},
		{Number: 7, HeadRef: scanBranchPrefix + "y", Author: deskkit.Account{Login: "assay-issue-loop-app[bot]"}},
	}}}
	cs, err := scanCarriersIn(r)
	if err != nil || len(cs) != 1 || cs[0].Number != 7 {
		t.Fatalf("carriers = %v (err %v), want only #7 — this App's scan-shaped branch", cs, err)
	}
}

// regression: #6292
func TestCarriers_FilesUnreadRefuses(t *testing.T) {
	failClosed(t, "gitlab", func(w *carrierWorld) { w.filesFail = true })
}

// regression: #6292
func TestCarriers_GitHubFilesUnread(t *testing.T) {
	failClosed(t, "github", func(w *carrierWorld) { w.filesFail = true })
}

// A coalesce into a young carrier must not pull in a sealed one's placeholders either: the class
// is "a carrier repeats what another open carrier carries", not only "a fresh cut does".
// regression: #6292
func TestCarriers_CoalesceOwnDelta(t *testing.T) {
	w := newCarrierWorld(t)
	w.serveForge(t, "gitlab")
	t0 := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	if out, err := w.pass(t, 501, t0); err != nil || strings.Contains(out, "dispatch-error") {
		t.Fatalf("pass 1 errored: %v\n%s", err, out)
	}
	sealed := w.newest()
	if out, err := w.pass(t, 502, t0.Add(30*time.Minute)); err != nil || strings.Contains(out, "dispatch-error") {
		t.Fatalf("pass 2 errored: %v\n%s", err, out)
	}
	young := w.newest()
	// Inside the window of the young carrier: this pass coalesces into it.
	out, err := w.pass(t, 503, t0.Add(35*time.Minute))
	if err != nil || strings.Contains(out, "dispatch-error") {
		t.Fatalf("pass 3 errored: %v\n%s", err, out)
	}
	if w.newest() != young {
		t.Fatalf("pass 3 opened #%d; it should have coalesced into #%d:\n%s", w.newest(), young, out)
	}
	wantFiles(t, w, sealed, 501)
	wantFiles(t, w, young, 502, 503)
}

// When every placeholder the scan writes is already on an open carrier, the pass has nothing of
// its own: no empty commit, no new carrier, and the item's exit names the carrier that holds it.
// regression: #6292
func TestCarriers_AllCarriedNoNewPR(t *testing.T) {
	w := newCarrierWorld(t)
	w.serveForge(t, "gitlab")
	t0 := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	if out, err := w.pass(t, 601, t0); err != nil || strings.Contains(out, "dispatch-error") {
		t.Fatalf("pass 1 errored: %v\n%s", err, out)
	}
	sealed := w.newest()
	// The same issue comes in again after the window: its placeholder is on the sealed carrier.
	w.mu.Lock()
	w.issues = w.issues[:0]
	w.mu.Unlock()
	out, err := w.pass(t, 601, t0.Add(30*time.Minute))
	if err != nil || strings.Contains(out, "dispatch-error") {
		t.Fatalf("pass 2 errored: %v\n%s", err, out)
	}
	if w.newest() != sealed {
		t.Fatalf("pass 2 opened #%d carrying %v — everything was already on #%d", w.newest(), w.filesOf(w.newest()), sealed)
	}
	if want := fmt.Sprintf("medici-finance/assay#%d", sealed); !strings.Contains(out, want) {
		t.Fatalf("the pass does not name %s as where the placeholder is carried:\n%s", want, out)
	}
}

// `scanloop run` itself wires the carrier read: with coalescing disabled — every pass fresh, the
// case that built the supersets — an unreadable carrier refuses the pass instead of writing the
// full delta.
// regression: #6292
func TestRun_CarriersReadWired(t *testing.T) {
	w := newCarrierWorld(t)
	w.carriers[700] = &worldCarrier{branch: "chore/intake-scan-2026-08-24-1100-aaaaaa",
		files: map[string]bool{placeholderOf(161): true}, open: true, created: coalesceNow.Add(-time.Hour)}
	w.filesFail = true
	srv := httptest.NewServer(http.HandlerFunc(w.github))
	t.Cleanup(srv.Close)
	oldBase := forgeAPIBase
	forgeAPIBase = srv.URL
	t.Cleanup(func() { forgeAPIBase = oldBase })
	stubIdentity(t,
		func(string) (string, string, error) { return "issue-loop", "intake-desk", nil },
		func(string, string) (string, string, error) { return "x-test-token", "/cache/x-test-token", nil })
	withDryRunTempBase(t)

	var out strings.Builder
	err := cmdRun([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--worktree-base", t.TempDir(),
		"--state-dir", armedStateDir(t),
		"--monitor", inboundPoller(t),
		"--now", "2026-08-24T12:00:00Z",
		"--coalesce-window", "-1s",
		"--dry-run",
	}, &out)
	if got := deskkit.ExitCodeOf(err); err == nil || got != deskkit.ExitUnverifiable {
		t.Fatalf("a pass that could not read open carrier #700's files exited %d (%v), want %d:\n%s",
			got, err, deskkit.ExitUnverifiable, out.String())
	}
	if strings.Contains(out.String(), "deskpr create") {
		t.Fatalf("a pass blind to carrier #700 still cut a fresh carrier:\n%s", out.String())
	}
}
