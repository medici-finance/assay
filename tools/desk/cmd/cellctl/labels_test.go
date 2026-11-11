package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/forgelabels"
)

// labelsRoster binds six roles: the house cell's label set is derived from exactly these.
const labelsRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk-app:300000001,intake-loop=example-intake-loop-app:300000002,issue-loop=example-issue-loop-app:300000003,reviewer=example-reviewer-app:300000004,verifier=example-verifier-app:300000005,worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=o/r:ci:private
`

var labelsRosterRoles = []string{"desk", "intake-loop", "issue-loop", "reviewer", "verifier", "worker"}

// fakeLabelForge is an in-memory label API for ONE repo/project, speaking just enough of
// GitHub's or GitLab's REST shape for internal/forgelabels: a paginated list, and a create
// that answers "already present" the way the real forge does. group holds GitLab labels
// inherited from an ancestor group (listed with include_ancestor_groups, refused on create as
// "already exists at group level"). No live forge is ever contacted.
type fakeLabelForge struct {
	forge   string
	mu      sync.Mutex
	project map[string]bool
	group   map[string]bool
	deny    bool // every create answers 403
	failGet bool // every list answers 500
	creates int
	noAuth  int // requests that arrived without the credential header
}

const fakeLabelCred = "op-cred-example"

func newFakeLabelForge(t *testing.T, forge string) (*fakeLabelForge, *httptest.Server) {
	t.Helper()
	f := &fakeLabelForge{forge: forge, project: map[string]bool{}, group: map[string]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	prev := labelsHTTP
	labelsHTTP = srv.Client()
	t.Cleanup(func() { labelsHTTP = prev })
	return f, srv
}

func (f *fakeLabelForge) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	authed := r.Header.Get("Authorization") == "Bearer "+fakeLabelCred
	if f.forge == "gitlab" {
		authed = r.Header.Get("PRIVATE-TOKEN") == fakeLabelCred
	}
	if !authed {
		f.noAuth++
		w.WriteHeader(401)
		return
	}
	if !strings.HasSuffix(r.URL.Path, "/labels") {
		w.WriteHeader(404)
		return
	}
	switch r.Method {
	case "GET":
		if f.failGet {
			w.WriteHeader(500)
			return
		}
		var names []string
		for n := range f.project {
			names = append(names, n)
		}
		if f.forge == "gitlab" && r.URL.Query().Get("include_ancestor_groups") == "true" {
			for n := range f.group {
				names = append(names, n)
			}
		}
		sort.Strings(names)
		per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if per <= 0 || page <= 0 {
			w.WriteHeader(400)
			return
		}
		lo, hi := (page-1)*per, page*per
		if lo > len(names) {
			lo = len(names)
		}
		if hi > len(names) {
			hi = len(names)
		}
		out := []map[string]string{}
		for _, n := range names[lo:hi] {
			out = append(out, map[string]string{"name": n})
		}
		_ = json.NewEncoder(w).Encode(out)
	case "POST":
		if f.deny {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch {
		case f.forge == "gitlab" && f.group[body.Name]:
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"message":"Label already exists at group level for example-group. Please choose another one."}`))
		case f.project[body.Name] && f.forge == "gitlab":
			w.WriteHeader(409)
			_, _ = w.Write([]byte(`{"message":"Label already exists"}`))
		case f.project[body.Name]:
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"message":"Validation Failed","errors":[{"resource":"Label","code":"already_exists","field":"name"}]}`))
		default:
			f.project[body.Name] = true
			f.creates++
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{}`))
		}
	default:
		w.WriteHeader(405)
	}
}

// present is every label the project sees, its own plus inherited.
func (f *fakeLabelForge) present() map[string]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	all := map[string]bool{}
	for n := range f.project {
		all[n] = true
	}
	for n := range f.group {
		all[n] = true
	}
	return all
}

// labelsWorld is houseForgeWorld with a roster that binds labelsRosterRoles, plus an
// owner-only file holding the operator credential the fake forge accepts.
func labelsWorld(t *testing.T) (repo, cells, cred string) {
	t.Helper()
	repo, cells = houseForgeWorld(t)
	must(t, os.WriteFile(filepath.Join(os.Getenv("ASSAY_CONFIG_HOME"), "roster.env"), []byte(labelsRoster), 0o600))
	cred = filepath.Join(t.TempDir(), "op-labels.cred")
	must(t, os.WriteFile(cred, []byte(fakeLabelCred+"\n"), 0o600))
	return repo, cells, cred
}

func wantSet() []string { return forgelabels.Names(forgelabels.Canonical(labelsRosterRoles)) }

// regression: #6127
func TestNewHouseGitlabEnsuresLabelSetIdempotently(t *testing.T) {
	repo, _, cred := labelsWorld(t)
	f, srv := newFakeLabelForge(t, "gitlab")
	f.group["review-request"] = true // inherited from the group: present, never re-created
	var code int
	out := captureStdout(t, func() {
		code = inproc("new", "g", "--kind", "house", "--forge", "gitlab", "--group", "example-group",
			"--gitlab-api-base", srv.URL, "--repo", repo, "--roots", "o/r="+repo, "--labels-token-file", cred)
	})
	if code != 0 {
		t.Fatalf("new exited %d:\n%s", code, out)
	}
	set := wantSet()
	want := "[new] labels: ensured " + strconv.Itoa(len(set)) + " on o/r (" + strconv.Itoa(len(set)-1) + " created, 1 already present)"
	if !strings.Contains(out, want) {
		t.Errorf("new did not report the ensure; want %q in:\n%s", want, out)
	}
	have := f.present()
	for _, n := range set {
		if !have[n] {
			t.Errorf("label %q not ensured", n)
		}
	}
	for _, role := range labelsRosterRoles {
		if !have["to:"+role] {
			t.Errorf("to:%s missing: every role the roster binds needs its to: label", role)
		}
	}
	if f.project["review-request"] {
		t.Error("an inherited group label was re-created at project level")
	}
	if f.noAuth != 0 {
		t.Errorf("%d request(s) arrived without the operator credential", f.noAuth)
	}

	// A second ensure is a no-op: every label reads as already present.
	before := f.creates
	again := captureStdout(t, func() {
		if !ensureCellLabels("gitlab", srv.URL, []string{"o/r"}, labelsRosterRoles, cred) {
			t.Error("the second ensure reported unfinished")
		}
	})
	if f.creates != before || !strings.Contains(again, "(0 created, "+strconv.Itoa(len(set))+" already present)") {
		t.Errorf("second ensure was not a no-op (creates %d -> %d):\n%s", before, f.creates, again)
	}
}

// regression: #6127
func TestEnsureCellLabelsGithubCreatesTheSetThenNoOps(t *testing.T) {
	_, _, cred := labelsWorld(t)
	f, srv := newFakeLabelForge(t, "github")
	set := wantSet()
	out := captureStdout(t, func() {
		if !ensureCellLabels("github", srv.URL, []string{"o/r"}, labelsRosterRoles, cred) {
			t.Error("the ensure reported unfinished")
		}
	})
	if !strings.Contains(out, "ensured "+strconv.Itoa(len(set))+" on o/r ("+strconv.Itoa(len(set))+" created, 0 already present)") {
		t.Errorf("first ensure:\n%s", out)
	}
	out = captureStdout(t, func() { _ = ensureCellLabels("github", srv.URL, []string{"o/r"}, labelsRosterRoles, cred) })
	if f.creates != len(set) || !strings.Contains(out, "(0 created, "+strconv.Itoa(len(set))+" already present)") {
		t.Errorf("second ensure was not a no-op (creates=%d):\n%s", f.creates, out)
	}
}

// A credential that cannot write labels leaves the step unfinished: NOT ENSURED, the exact
// finishing command, and exit 1. It never succeeds silently.
//
// regression: #6127
func TestNewHouseLabelsDeniedIsNotEnsuredAndExits1(t *testing.T) {
	repo, _, cred := labelsWorld(t)
	f, srv := newFakeLabelForge(t, "gitlab")
	f.deny = true
	var code int
	out := captureStdout(t, func() {
		code = inproc("new", "g", "--kind", "house", "--forge", "gitlab", "--group", "example-group",
			"--gitlab-api-base", srv.URL, "--repo", repo, "--roots", "o/r="+repo, "--labels-token-file", cred)
	})
	if code != 1 {
		t.Errorf("new exited %d on a denied labels step, want 1:\n%s", code, out)
	}
	wantCmd := forgelabels.Command("gitlab", "o/r", labelsRosterRoles, srv.URL, false)
	for _, want := range []string{"[new] labels: NOT ENSURED on o/r", "the credential cannot write labels", "finish with: " + wantCmd} {
		if !strings.Contains(out, want) {
			t.Errorf("denied output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "labels: ensured") {
		t.Errorf("a denied step reads as ensured:\n%s", out)
	}
	if strings.Contains(out, fakeLabelCred) {
		t.Error("the credential reached the output")
	}
}

// With no credential the step is deferred, not done: NOT ENSURED with the command, exit 0.
func TestNewHouseLabelsWithoutCredentialIsDeferred(t *testing.T) {
	repo, _, _ := labelsWorld(t)
	var code int
	out := captureStdout(t, func() {
		code = inproc("new", "h", "--kind", "house", "--repo", repo, "--roots", "o/r="+repo)
	})
	if code != 0 {
		t.Errorf("new exited %d with no labels credential, want 0 (deferred)", code)
	}
	if !strings.Contains(out, "[new] labels: NOT ENSURED on o/r — no --labels-token-file given") ||
		!strings.Contains(out, "finish with: deskfleet labels --forge github --repo o/r --roles desk,intake-loop,issue-loop,reviewer,verifier,worker") {
		t.Errorf("deferred step does not say so with its command:\n%s", out)
	}
}

func TestNewLabelsTokenFileRefusedOffHouse(t *testing.T) {
	repo, _, cred := labelsWorld(t)
	stderr := captureStderr(t, func() {
		if code := inproc("new", "s", "--kind", "scrubbed", "--repo", repo, "--labels-token-file", cred); code == 0 {
			t.Error("a non-house new accepted --labels-token-file")
		}
	})
	if !strings.Contains(stderr, "--labels-token-file is only valid for --kind house") {
		t.Errorf("refusal does not name the flag: %q", stderr)
	}
}

// labelsCheckOut scaffolds a github house cell, points its cell.env at srv and (when cred is
// non-empty) at the credential, and returns `cellctl check`'s stdout.
func labelsCheckOut(t *testing.T, srv *httptest.Server, withCred bool) string {
	t.Helper()
	repo, cells, cred := labelsWorld(t)
	if code := inproc("new", "h", "--kind", "house", "--repo", repo, "--roots", "o/r="+repo); code != 0 {
		t.Fatalf("new exited %d", code)
	}
	envfile := filepath.Join(cells, "h", "cell.env")
	raw, err := os.ReadFile(envfile)
	must(t, err)
	extra := "FORGE_API_BASE=" + srv.URL + "\n"
	if withCred {
		extra += labelsTokenKey + "=" + cred + "\n"
	}
	must(t, os.WriteFile(envfile, append(raw, extra...), 0o600))
	return captureStdout(t, func() { _ = inproc("check", "h") })
}

// regression: #6127
func TestHouseCheckNamesEachMissingLabel(t *testing.T) {
	f, srv := newFakeLabelForge(t, "github")
	set := wantSet()
	for _, n := range set {
		if n != "to:worker" && n != "to:reviewer" {
			f.project[n] = true
		}
	}
	out := labelsCheckOut(t, srv, true)
	want := "MISS  labels on o/r: 2 of " + strconv.Itoa(len(set)) + " missing: to:reviewer, to:worker — finish with: deskfleet labels --forge github --repo o/r"
	if !strings.Contains(out, want) {
		t.Errorf("check does not name each missing label; want %q in:\n%s", want, out)
	}
	if strings.Contains(out, "all preconditions met") {
		t.Errorf("a check with missing labels reads clean:\n%s", out)
	}
}

func TestHouseCheckLabelsAllPresent(t *testing.T) {
	f, srv := newFakeLabelForge(t, "github")
	for _, n := range wantSet() {
		f.project[n] = true
	}
	out := labelsCheckOut(t, srv, true)
	if !strings.Contains(out, "ok    labels on o/r: all "+strconv.Itoa(len(wantSet()))+" present (roles desk,intake-loop,issue-loop,reviewer,verifier,worker)") {
		t.Errorf("a complete set is not an ok row:\n%s", out)
	}
}

// Labels that cannot be read are could-not-check, never ok.
//
// regression: #6127
func TestHouseCheckUnreadableLabelsAreCouldNotCheck(t *testing.T) {
	t.Run("credential unset", func(t *testing.T) {
		_, srv := newFakeLabelForge(t, "github")
		out := labelsCheckOut(t, srv, false)
		if !strings.Contains(out, "?     could-not-check: labels on o/r: CELL_LABELS_TOKEN_FILE unset in cell.env") {
			t.Errorf("no could-not-check row:\n%s", out)
		}
		if strings.Contains(out, "ok    labels") {
			t.Errorf("an unread label set reads ok:\n%s", out)
		}
	})
	t.Run("read fails", func(t *testing.T) {
		f, srv := newFakeLabelForge(t, "github")
		f.failGet = true
		out := labelsCheckOut(t, srv, true)
		if !strings.Contains(out, "?     could-not-check: labels on o/r: could not be read (HTTP 500") {
			t.Errorf("a failed read is not could-not-check:\n%s", out)
		}
		if strings.Contains(out, "ok    labels") || strings.Contains(out, "MISS  labels") {
			t.Errorf("a failed read was judged:\n%s", out)
		}
	})
}

// Both network-reaching label paths name the forge and the API base the operator credential
// goes to before the first request, as `deskfleet labels` prints its target: the line comes
// ahead of every per-repo result.
func TestLabelPathsNameTheirTargetFirst(t *testing.T) {
	_, _, cred := labelsWorld(t)
	_, srv := newFakeLabelForge(t, "github")
	out := captureStdout(t, func() { _ = ensureCellLabels("github", srv.URL, []string{"o/r"}, labelsRosterRoles, cred) })
	target := "[new] labels: target github API " + srv.URL + ", repos o/r, roles "
	if i, j := strings.Index(out, target), strings.Index(out, "[new] labels: ensured"); i < 0 || j < 0 || i > j {
		t.Errorf("new does not name its target before the first result; want %q first in:\n%s", target, out)
	}
	_, srv2 := newFakeLabelForge(t, "github")
	out = labelsCheckOut(t, srv2, true)
	target = "[check] labels: reading from github API " + srv2.URL + ", repos o/r"
	if i, j := strings.Index(out, target), strings.Index(out, "labels on o/r"); i < 0 || j < 0 || i > j {
		t.Errorf("check does not name its target before the first labels row; want %q first in:\n%s", target, out)
	}
}

// The summary of a run whose only open rows are could-not-check never says the cell is clean.
func TestCheckerSummaryWithCouldNotCheck(t *testing.T) {
	k := &checker{ok: true}
	out := captureStdout(t, func() {
		k.chk(true, "a")
		k.cnc("b")
		k.summary()
	})
	if strings.Contains(out, "all preconditions met") || !strings.Contains(out, "1 could-not-check row(s)") {
		t.Errorf("summary with a could-not-check row:\n%s", out)
	}
}
