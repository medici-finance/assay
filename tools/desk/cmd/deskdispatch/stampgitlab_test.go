package main

// stampgitlab_test.go — #1154: a stamped dispatch on a GitLab-served project.
//
// THE DEFECT. `deskdispatch --kit review --pr <N> --model <slug>` on a project whose forge
// resolves to GitLab failed at the model-stamp step: the step read the change's labels and
// its label timeline through the GitHub CLI and wrote the stamp with `gh pr edit`, none of
// which can address a merge request. The step failed closed (correctly — stamping blind is
// worse than not stamping), the dispatch returned non-zero, and the forge-neutral
// `authorization-needed` queue label (step 6) was never reached. Without `--model` the
// dispatch completed but the review it produced was refused by the model-capability floor,
// so a GitLab review dispatch was caught either way.
//
// THE HARNESS. This is the closest thing to the real thing short of a live instance: a
// roster that binds `example-org/example-project` to GitLab, the role PAT custody files the
// resolver reads, `GITLAB_API_BASE` pointed at an httptest server that speaks the handful of
// endpoints the step and the queue label touch, and the WHOLE verb run through `run` with the
// same argv the review desk uses. The resolver constructs a real GitLabForge; only the far
// side of the wire is fake. The execCommand recorder that used to be the assertion surface is
// now the guard: no `gh` may be launched.
//
// The tests are named TestGitLab* so the forge-gitlab mutation map's `-run` selector picks
// them up alongside the backend's own.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	glProject = "example-org/example-project"
	glMRIID   = "7"
	// glPATPlaceholder is a low-entropy placeholder — NOT a real credential. It deliberately
	// carries no vendor token prefix: the pattern leg of the leak gate (gitleaks' `gitlab-pat`
	// rule) matches on the prefix shape alone, so a prefixed placeholder reddens CI whatever
	// the text after it says. Sibling tests spell fake PATs the same bare way.
	glPATPlaceholder = "example-placeholder-pat-0000"
)

// glStampServer is the fake GitLab instance. Paths arrive URL-encoded
// (`/api/v4/projects/example-org%2Fexample-project/merge_requests/7`), which is why every
// match reads r.URL.EscapedPath.
type glStampServer struct {
	srv      *httptest.Server
	requests []glStampReq

	labels []string     // labels currently on MR 7
	events []glLabelEvt // its resource label events, in order
	failMR bool         // the MR read answers 500
	failEv bool         // the label-event read answers 500
}

type glStampReq struct {
	Method string
	Path   string
	Body   string
	Auth   string
}

// glLabelEvt is one resource_label_event as GitLab renders it.
type glLabelEvt struct {
	Action string // add | remove
	Label  string
	User   string
}

func (s *glStampServer) mrJSON() map[string]any {
	return map[string]any{
		"iid": 7, "id": 1007, "state": "opened", "draft": true, "title": "example change",
		"description": "", "labels": s.labels, "sha": "abc123", "changes_count": "1",
		"source_branch": "feat/x", "target_branch": "main",
		"web_url":               s.srv.URL + "/" + glProject + "/-/merge_requests/7",
		"detailed_merge_status": "mergeable",
		"author":                map[string]any{"id": 1, "username": "example-bot"},
	}
}

func (s *glStampServer) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	path := r.URL.EscapedPath()
	s.requests = append(s.requests, glStampReq{Method: r.Method, Path: path, Body: string(body),
		Auth: r.Header.Get("PRIVATE-TOKEN") + r.Header.Get("Authorization")})
	enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	mrPath := "/api/v4/projects/example-org%2Fexample-project/merge_requests/" + glMRIID
	switch {
	case r.Method == http.MethodGet && path == mrPath+"/resource_label_events":
		if s.failEv {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		evs := make([]map[string]any, 0, len(s.events))
		for i, e := range s.events {
			evs = append(evs, map[string]any{
				"id": i + 1, "action": e.Action,
				"user":  map[string]any{"id": 10 + i, "username": e.User},
				"label": map[string]any{"id": 100 + i, "name": e.Label},
			})
		}
		enc(evs)
	case r.Method == http.MethodGet && path == mrPath:
		if s.failMR {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		enc(s.mrJSON())
	case r.Method == http.MethodPut && path == mrPath:
		enc(s.mrJSON())
	case r.Method == http.MethodPost && path == "/api/v4/projects/example-org%2Fexample-project/labels":
		w.WriteHeader(http.StatusCreated)
		enc(map[string]any{"id": 5, "name": "created"})
	default:
		w.WriteHeader(http.StatusNotFound)
		enc(map[string]any{"message": "404 Not Found: " + r.Method + " " + path})
	}
}

// puts returns the bodies of every MR update, in order — the label writes.
func (s *glStampServer) puts() []string {
	var out []string
	for _, r := range s.requests {
		if r.Method == http.MethodPut {
			out = append(out, r.Body)
		}
	}
	return out
}

// installGLStamp stands the fake instance up and makes the resolver land on it: a roster
// binding the project to GitLab, both dispatcher roles' PAT custody files (0600, where
// gitlabCustody looks), and GITLAB_API_BASE. It runs AFTER stub.install, which planted the
// GitHub-shaped fixture roster under the same HOME.
func installGLStamp(t *testing.T, home string) *glStampServer {
	t.Helper()
	s := &glStampServer{}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.srv.Close)

	dir := filepath.Join(home, ".config", "assay")
	roster := "ASSAY_BLESS_LOGIN=ada:2001\n" +
		"ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002\n" +
		"ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk-bot:300000001,intake-loop=example-intake-bot:300000002," +
		"issue-loop=example-issue-bot:300000003,reviewer=example-reviewer-bot:300000004," +
		"verifier=example-verifier-bot:300000005,worker=example-worker-bot:300000006\n" +
		"ASSAY_ALLOWED_REPOS=" + glProject + ":ci:private\n" +
		"ASSAY_REPO_FORGES=" + glProject + "=gitlab\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatalf("planting the GitLab roster: %v", err)
	}
	for _, role := range deskkit.DispatcherRoles() {
		if err := os.WriteFile(filepath.Join(dir, "gitlab-"+role+".token"), []byte(glPATPlaceholder+"\n"), 0o600); err != nil {
			t.Fatalf("planting the %s PAT custody file: %v", role, err)
		}
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	t.Setenv("GITLAB_API_BASE", s.srv.URL)
	return s
}

// glStampRun drives a stamped dispatch onto the GitLab project and returns the exit code and
// the recorded subprocess argv. The origin remote is GitLab-shaped for good measure; the
// roster entry is what actually resolves the forge.
func glStampRun(t *testing.T, s *stub, root string, extra ...string) int {
	t.Helper()
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@gitlab.com:" + glProject + ".git"},
		{match: "deskwt add", stdout: "/private/tmp/worker-home"},
	}
	args := []string{"item-1", "--root", root, "--repo", glProject, "--pr", glMRIID,
		"--model", "example-model-1", "--tier", "strong",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}
	return run(append(args, extra...))
}

func noForgeCLI(t *testing.T, s *stub) {
	t.Helper()
	for _, c := range s.calls {
		if len(c) > 0 && (c[0] == "gh" || c[0] == "glab") {
			t.Fatalf("the dispatch launched the forge CLI on a GitLab-served project (%v) — that CLI cannot "+
				"address the merge request, and this is exactly the step that failed closed before the "+
				"queue label (#1154)", c)
		}
	}
}

// THE ISSUE'S OWN SCENARIO. A review dispatch with --model on a GitLab project: the stamp is
// written through the resolved GitLab backend (one PUT carrying both halves as add_labels),
// no forge CLI is launched, the dispatch completes, and — the whole point — step 6's
// forge-neutral queue label is REACHED and lands on the same MR after the stamp.
func TestGitLabReviewDispatchStampsThroughTheResolvedForge(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)
	gl := installGLStamp(t, home)
	t.Setenv("DESK_LOOP", "pr-review-desk")

	if rc := glStampRun(t, s, root, "--kit", "review"); rc != deskkit.ExitOK {
		t.Fatalf("review dispatch rc = %d, want 0 — a stamped review dispatch must complete on a GitLab project", rc)
	}
	noForgeCLI(t, s)

	model := deskkit.DispatchedModelPrefix + "example-model-1"
	tier := deskkit.DispatchedTierPrefix + "strong"
	puts := gl.puts()
	stampAt, queueAt := -1, -1
	for i, b := range puts {
		if strings.Contains(b, "add_labels") && strings.Contains(b, model) && strings.Contains(b, tier) {
			stampAt = i
		}
		if strings.Contains(b, "add_labels") && strings.Contains(b, queueLabelAuthorizationNeeded) {
			queueAt = i
		}
	}
	if stampAt < 0 {
		t.Fatalf("no MR update applied BOTH stamp halves through the GitLab backend; PUT bodies: %q", puts)
	}
	if queueAt < 0 {
		t.Fatalf("the review-lane queue label never reached the MR — step 6 was not run after the stamp; "+
			"PUT bodies: %q", puts)
	}
	if queueAt < stampAt {
		t.Errorf("the queue label (PUT %d) landed before the stamp (PUT %d) — the stamp is step 5, the queue "+
			"label step 6", queueAt, stampAt)
	}
	for _, r := range gl.requests {
		if !strings.Contains(r.Auth, glPATPlaceholder) {
			t.Errorf("%s %s reached the GitLab instance without the role's PAT (auth=%q)", r.Method, r.Path, r.Auth)
		}
	}
}

// A foreign-applied stamp on the MR is REPLACED, not added over: the removal is its own
// MR update (remove_labels only) and precedes the application (add_labels only), so GitLab
// records two label events and the standing applier becomes the dispatcher. One PUT that
// removed and re-added the same names would leave the label set — and the standing applier —
// exactly as it was.
func TestGitLabStampReplacesAForeignAppliedStamp(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)
	gl := installGLStamp(t, home)
	model := deskkit.DispatchedModelPrefix + "example-model-1"
	tier := deskkit.DispatchedTierPrefix + "strong"
	gl.labels = []string{model, tier}
	gl.events = []glLabelEvt{{"add", model, "some-other-user"}, {"add", tier, "some-other-user"}}

	if rc := glStampRun(t, s, root); rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	noForgeCLI(t, s)
	puts := gl.puts()
	if len(puts) < 2 {
		t.Fatalf("want a remove-then-add pair of MR updates, got %d: %q", len(puts), puts)
	}
	if !strings.Contains(puts[0], "remove_labels") || strings.Contains(puts[0], "add_labels") ||
		!strings.Contains(puts[0], model) || !strings.Contains(puts[0], tier) {
		t.Errorf("the first MR update must remove BOTH foreign halves and add nothing: %q", puts[0])
	}
	if !strings.Contains(puts[1], "add_labels") || strings.Contains(puts[1], "remove_labels") ||
		!strings.Contains(puts[1], model) || !strings.Contains(puts[1], tier) {
		t.Errorf("the second MR update must re-apply BOTH halves and remove nothing: %q", puts[1])
	}
}

// A stamp already standing under the dispatcher is a NO-OP on GitLab too: no MR update
// touches a dispatched-* label, so a re-dispatch neither churns the label events nor leaves
// the MR briefly unstamped.
func TestGitLabStampLeavesItsOwnStampAlone(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)
	gl := installGLStamp(t, home)
	d, ok := deskkit.RoleAppLogin(deskkit.DispatcherRole)
	if !ok {
		t.Fatalf("the planted roster binds no App to the dispatcher role %q", deskkit.DispatcherRole)
	}
	model := deskkit.DispatchedModelPrefix + "example-model-1"
	tier := deskkit.DispatchedTierPrefix + "strong"
	gl.labels = []string{model, tier}
	gl.events = []glLabelEvt{{"add", model, d}, {"add", tier, d}}

	if rc := glStampRun(t, s, root); rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	noForgeCLI(t, s)
	for _, b := range gl.puts() {
		if strings.Contains(b, "dispatched-") {
			t.Errorf("the dispatcher rewrote its OWN standing stamp — an identical stamp is a no-op: %q", b)
		}
	}
}

// An unreadable label history is could-not-check: WHO applied the present stamp cannot be
// established, so nothing is written — stamping blind over a foreign stamp would report
// success on an MR the floor keeps refusing.
func TestGitLabStampRefusesWhenTheHistoryReadFails(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)
	gl := installGLStamp(t, home)
	gl.labels = []string{deskkit.DispatchedModelPrefix + "example-model-1"}
	gl.failEv = true

	rc := glStampRun(t, s, root)
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (an unreadable label history is could-not-check)", rc, deskkit.ExitUnverifiable)
	}
	if puts := gl.puts(); len(puts) != 0 {
		t.Errorf("a label was written on an unverifiable history read: %q", puts)
	}
}

// The present-label read is the other half of the same rule.
func TestGitLabStampRefusesWhenTheChangeReadFails(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)
	gl := installGLStamp(t, home)
	gl.failMR = true

	rc := glStampRun(t, s, root)
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (an unreadable label set is could-not-check)", rc, deskkit.ExitUnverifiable)
	}
	if puts := gl.puts(); len(puts) != 0 {
		t.Errorf("a label was written on an unverifiable change read: %q", puts)
	}
}
