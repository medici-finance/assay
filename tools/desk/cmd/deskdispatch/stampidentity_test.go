package main

// stampidentity_test.go — the WRITER half of the dispatcher-attestation contract: the
// model stamp must be applied under the DISPATCHER App's identity, because that is the
// only applier the capability floor's reader will accept.
//
// THE DEFECT THESE PIN. The stamp step shelled out to `gh` with no token in the child's
// environment, so the labels landed under whatever credential the calling shell happened
// to hold — a role App for some other role, or the operator's own login. The floor's
// applier-aware reader then read exactly the shape it exists to refuse (a dispatched-*
// label from a non-dispatcher) and refused every correctly-dispatched, genuinely
// strong-tier PR, while an UNSTAMPED PR sailed through on the absent-attestation NOTICE.
// Present-but-untrusted was worse than absent, and the stamp the dispatcher itself wrote
// was the untrusted one.
//
// WHY THE HARNESS IS AN HTTP SERVER (#1154). The step used to reach the forge by launching
// `gh`, so these tests wrapped execCommand and asserted on the constructed argv
// ("pr edit 77 --add-label …"). Every read and write now goes through the resolved
// deskkit.Forge — an HTTP client bound to the lane's own dispatcher credential — so the argv
// recorder has nothing left to record. The successor records the same facts one layer down:
// the METHOD and PATH of every request the step actually emits, plus the role the custody
// minter was asked for. Nothing about the code path under test is stubbed out — the resolver
// constructs a real GitHubForge, the custody hook hands it the stub token, and the API base
// points it at this server. Only the far side of the wire is fake.

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ghStampServer is the fake GitHub the stamp step is driven against. It serves the two
// reads the re-stamp needs — the change (with its CURRENT labels) and its label timeline —
// and accepts the ensure/apply/remove writes, recording each request.
type ghStampServer struct {
	srv      *httptest.Server
	requests []ghStampReq

	labels   []string     // the labels currently on PR 77
	timeline []ghStampEvt // its label timeline, in order
	failPR   bool         // the change read answers 500
	failTL   bool         // the timeline read answers 500
	minted   []mintCall   // every custody request the resolver made
	mintErr  error        // when set, the custody minter refuses
}

type ghStampReq struct {
	Method string
	Path   string
	Body   string
	Auth   string
}

// ghStampEvt is one `labeled`/`unlabeled` timeline event as the wire renders it.
type ghStampEvt struct {
	Event string
	Label string
	Actor string
}

type mintCall struct{ role, repo string }

const ghStampToken = "example-installation-token"

// installGHStamp stands the fake forge up and routes the resolver's GitHub custody to it:
// the minter returns the stub token AND this server's URL as the API base, which is the one
// documented hook for both (deskkit.SetGitHubCustodyMinter).
func installGHStamp(t *testing.T) *ghStampServer {
	t.Helper()
	s := &ghStampServer{}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.srv.Close)
	deskkit.SetGitHubCustodyMinter(func(role string, repo deskkit.ForgeRepo) (string, string, error) {
		s.minted = append(s.minted, mintCall{role: role, repo: repo.Slug()})
		if s.mintErr != nil {
			return "", "", s.mintErr
		}
		return ghStampToken, s.srv.URL, nil
	})
	t.Cleanup(func() { deskkit.SetGitHubCustodyMinter(nil) })
	return s
}

func (s *ghStampServer) handle(w http.ResponseWriter, r *http.Request) {
	var body strings.Builder
	if r.Body != nil {
		b := make([]byte, 1<<16)
		n, _ := r.Body.Read(b)
		body.Write(b[:n])
	}
	s.requests = append(s.requests, ghStampReq{Method: r.Method, Path: r.URL.Path, Body: body.String(),
		Auth: r.Header.Get("Authorization")})
	enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/pulls/77"):
		if s.failPR {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		labels := make([]map[string]any, 0, len(s.labels))
		for _, l := range s.labels {
			labels = append(labels, map[string]any{"name": l})
		}
		enc(map[string]any{
			"number": 77, "state": "open", "draft": true, "node_id": "PR_example_node",
			"labels": labels, "head": map[string]any{"sha": "abc123", "ref": "feat/x"},
			"base": map[string]any{"ref": "main"}, "user": map[string]any{"login": "example-bot[bot]", "id": 1},
		})
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/issues/77/timeline"):
		if s.failTL {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Query().Get("page") != "1" {
			enc([]map[string]any{})
			return
		}
		evs := make([]map[string]any, 0, len(s.timeline))
		for _, e := range s.timeline {
			evs = append(evs, map[string]any{
				"event": e.Event, "label": map[string]any{"name": e.Label},
				"actor": map[string]any{"login": e.Actor},
			})
		}
		enc(evs)
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/labels"):
		// Both the repo-level ensure (POST /labels) and the apply (POST /issues/77/labels).
		w.WriteHeader(http.StatusCreated)
		enc([]map[string]any{})
	case r.Method == http.MethodDelete && strings.Contains(path, "/issues/77/labels/"):
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// removed reports whether the step took label l OFF the PR.
func (s *ghStampServer) removed(l string) bool {
	for _, r := range s.requests {
		if r.Method == http.MethodDelete && strings.HasSuffix(r.Path, "/issues/77/labels/"+l) {
			return true
		}
	}
	return false
}

// applied reports whether the step's apply write named label l.
func (s *ghStampServer) applied(l string) bool {
	for _, r := range s.requests {
		if r.Method == http.MethodPost && strings.HasSuffix(r.Path, "/issues/77/labels") && strings.Contains(r.Body, l) {
			return true
		}
	}
	return false
}

// wrote reports whether ANY label write (apply or remove) reached the forge.
func (s *ghStampServer) wrote() bool {
	for _, r := range s.requests {
		if r.Method == http.MethodDelete || (r.Method == http.MethodPost && strings.HasSuffix(r.Path, "/issues/77/labels")) {
			return true
		}
	}
	return false
}

func stampArgs(t *testing.T, root string, extra ...string) []string {
	t.Helper()
	args := []string{"item-1", "--root", root, "--repo", allowedRepo, "--pr", "77",
		"--model", "example-model-1", "--tier", "strong",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}
	return append(args, extra...)
}

// The stamp is applied as the App that DISPATCHED this lane — the identity the floor's reader
// accepts for it — and NOT as whatever role this session's own loop happens to act under. The
// session here presents the review loop while dispatching the WORKER kit, which is precisely
// the case that produced the field failure: the ambient identity is not the dispatcher of the
// lane being launched, and a stamp under it is a stamp the floor cannot trust.
func TestStampAppliedAsDispatcherApp(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	t.Setenv("DESK_LOOP", "pr-review-desk")
	gh := installGHStamp(t)

	rc := run(stampArgs(t, root))
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	if len(gh.minted) == 0 {
		t.Fatal("no App credential was resolved for the stamp — the labels were applied under the ambient " +
			"credential, which is exactly the applier the floor refuses")
	}
	for _, c := range gh.minted {
		if c.role != deskkit.DispatcherRole {
			t.Errorf("the stamp authenticated as the %q App; the floor accepts only the dispatcher role %q",
				c.role, deskkit.DispatcherRole)
		}
		if c.repo != allowedRepo {
			t.Errorf("credential resolved for %q, want the target repo %q (an installation token is per account)",
				c.repo, allowedRepo)
		}
	}
	model := deskkit.DispatchedModelPrefix + "example-model-1"
	tier := deskkit.DispatchedTierPrefix + "strong"
	if !gh.applied(model) {
		t.Errorf("the model half was not applied to the PR: %+v", gh.requests)
	}
	if !gh.applied(tier) {
		t.Errorf("the tier half was not applied to the PR: %+v", gh.requests)
	}
	// Every request carried the resolved token — the identity the forge records for the label.
	// Asserting the resolution alone would pass while the token sat in a variable nothing read.
	for _, r := range gh.requests {
		if !strings.Contains(r.Auth, ghStampToken) {
			t.Errorf("%s %s reached the forge without the dispatcher token (Authorization=%q) — the label "+
				"would be applied under the ambient identity", r.Method, r.Path, r.Auth)
		}
	}
}

// The REVIEW lane is dispatched by the reviewer App, so its stamp must be resolved under the
// reviewer role — not the desk role. Before this, a review dispatch either carried no stamp at
// all or carried one whose applier was not the identity that launched the session, and either
// way a correctly-run review could never present a floor-trusted attestation. Dispatcher and
// applier are the same identity again exactly when this resolution asks for the lane's own role.
func TestReviewKitStampsAsTheReviewerApp(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	t.Setenv("DESK_LOOP", "pr-review-desk")
	gh := installGHStamp(t)
	// The review-lane queue label is a separate step with its own seam; stub it so this
	// identity test asserts the STAMP's credential only.
	stubQueueLabel(t, nil)

	rc := run(stampArgs(t, root, "--kit", "review"))
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	if len(gh.minted) == 0 {
		t.Fatal("no App credential was resolved for the stamp on a review dispatch")
	}
	for _, c := range gh.minted {
		if c.role != deskkit.ReviewDispatcherRole {
			t.Errorf("the review lane's stamp authenticated as the %q App; it must be the lane's own "+
				"dispatcher %q, or applier and dispatcher are different identities again",
				c.role, deskkit.ReviewDispatcherRole)
		}
	}
}

// stampRoleForKit is the writer's half of the one-list rule: every role it can return must be
// one the floor's reader accepts, and every other kit must stay on the desk App. A role
// returned here that DispatcherRoles does not carry mints a stamp nothing will ever trust.
func TestStampRoleForKitStaysInsideTheTrustedSet(t *testing.T) {
	trusted := map[string]bool{}
	for _, r := range deskkit.DispatcherRoles() {
		trusted[r] = true
	}
	for _, kit := range append(kitNames(), "REVIEW", " review ", "", "nonsense") {
		got := stampRoleForKit(kit)
		if !trusted[got] {
			t.Errorf("kit %q stamps as role %q, which is outside the floor's trusted set %v — the stamp "+
				"would be minted under an identity nothing accepts", kit, got, deskkit.DispatcherRoles())
		}
		wantReviewer := strings.EqualFold(strings.TrimSpace(kit), "review")
		if wantReviewer != (got == deskkit.ReviewDispatcherRole) {
			t.Errorf("kit %q stamps as %q — only the review kit is the reviewer App's to attest for",
				kit, got)
		}
	}
}

// A stamp that cannot be applied under the dispatcher identity is NOT applied at all. An
// untrusted stamp is worse than no stamp: absent reads UNKNOWN and proceeds with a NOTICE,
// while present-but-untrusted refuses every authority-bearing write on that PR. So a
// credential the resolver cannot read stops before the first label, and says so.
func TestStampNeverUsesAmbientIdentity(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	gh := installGHStamp(t)
	gh.mintErr = errors.New("no credential")

	rc := run(stampArgs(t, root))
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (a stamp whose identity cannot be established is could-not-check)",
			rc, deskkit.ExitUnverifiable)
	}
	if len(gh.requests) != 0 {
		t.Fatalf("the forge was reached without a dispatcher credential: %+v", gh.requests)
	}
}

// THE FORGE-CLI BAN, at the verb (#1154). The stamp step was the last place in this verb
// that launched `gh`; on a GitLab-served project that CLI cannot address the change at all,
// so a stamped dispatch failed closed here before the forge-neutral queue label was reached.
// Every forge read and write now goes through the resolved backend, and this pins that no
// forge CLI is launched on a stamped dispatch — the argv recorder that used to be the
// harness is now the guard.
func TestStampLaunchesNoForgeCLI(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	installGHStamp(t)

	if rc := run(stampArgs(t, root)); rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	for _, c := range s.calls {
		if len(c) > 0 && (c[0] == "gh" || c[0] == "glab") {
			t.Fatalf("a stamped dispatch launched the forge CLI (%s) — the stamp must go through the "+
				"resolved Forge, or a GitLab project fails closed before the queue label: %v", c[0], c)
		}
	}
}

// dispatcherAppLogin is the roster's desk-App login — the identity the floor accepts, read
// from the fixture roster rather than written as a literal.
func dispatcherAppLogin(t *testing.T) string {
	t.Helper()
	login, ok := deskkit.RoleAppLogin(deskkit.DispatcherRole)
	if !ok {
		t.Fatalf("the fixture roster binds no App to the dispatcher role %q", deskkit.DispatcherRole)
	}
	return login
}

func labeled(label, actor string) ghStampEvt {
	return ghStampEvt{Event: "labeled", Label: label, Actor: actor}
}
func unlabeled(label, actor string) ghStampEvt {
	return ghStampEvt{Event: "unlabeled", Label: label, Actor: actor}
}

// THE NO-OP THIS PINS. Labels are a SET: applying a label the PR already carries changes
// nothing. So a PR stamped by some other login — the pre-fix dispatch path, or a human —
// stayed stamped by that login no matter how often the dispatcher re-ran, and every
// authority-bearing write on it kept refusing. A forge's label history is append-only, so the
// only repair it offers is to REMOVE the label and re-apply it as the dispatcher.
func TestStampReplacesAForeignAppliedStamp(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	model := deskkit.DispatchedModelPrefix + "example-model-1"
	tier := deskkit.DispatchedTierPrefix + "strong"
	gh := installGHStamp(t)
	gh.labels = []string{model, tier}
	gh.timeline = []ghStampEvt{labeled(model, "some-other-login"), labeled(tier, "some-other-login")}

	if rc := run(stampArgs(t, root)); rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	for _, l := range []string{model, tier} {
		if !gh.removed(l) {
			t.Errorf("the foreign application of %s was never removed — re-applying it on top is a no-op: %+v", l, gh.requests)
		}
		if !gh.applied(l) {
			t.Errorf("%s was not re-applied as the dispatcher: %+v", l, gh.requests)
		}
	}
	// ORDER: the removal precedes the application, or the re-apply lands first and the
	// removal then strips the dispatcher's own stamp.
	firstRemove, firstApply := -1, -1
	for i, r := range gh.requests {
		if firstRemove < 0 && r.Method == http.MethodDelete {
			firstRemove = i
		}
		if firstApply < 0 && r.Method == http.MethodPost && strings.HasSuffix(r.Path, "/issues/77/labels") {
			firstApply = i
		}
	}
	if firstRemove < 0 || firstApply < 0 || firstRemove > firstApply {
		t.Errorf("the removal (request %d) must precede the application (request %d): %+v",
			firstRemove, firstApply, gh.requests)
	}
}

// THE PRESENT-BUT-UNREADABLE DEADLOCK. An earlier run of the DISPATCHER itself left a stale
// dispatched-model half, so the PR carries two model slugs — a conflicting stamp the floor
// reads as unreadable and refuses. Both halves the re-dispatch wants are already standing
// under the dispatcher, so clearing only FOREIGN labels removes nothing and the conflict
// survives: re-dispatch reports OK while every verdict on the PR keeps refusing. The re-stamp
// must therefore drop the stale slug the dispatcher itself left, not just foreign labels.
func TestStampClearsADispatcherAppliedConflictingStamp(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	d := dispatcherAppLogin(t)
	model := deskkit.DispatchedModelPrefix + "example-model-1"
	staleModel := deskkit.DispatchedModelPrefix + "example-model-2"
	tier := deskkit.DispatchedTierPrefix + "strong"
	gh := installGHStamp(t)
	gh.labels = []string{model, staleModel, tier}
	gh.timeline = []ghStampEvt{labeled(model, d), labeled(staleModel, d), labeled(tier, d)}

	if rc := run(stampArgs(t, root)); rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	if !gh.removed(staleModel) {
		t.Errorf("the stale dispatcher-applied slug %s was not removed — the conflicting stamp survives "+
			"and the floor keeps refusing: %+v", staleModel, gh.requests)
	}
	// The intended pair, already standing under the dispatcher, must NOT be churned.
	if gh.removed(model) {
		t.Errorf("the wanted model half was removed — re-stamp must not churn a correct standing label: %+v", gh.requests)
	}
	if gh.removed(tier) {
		t.Errorf("the wanted tier half was removed — re-stamp must not churn a correct standing label: %+v", gh.requests)
	}
}

// A stamp already standing under the DISPATCHER is left alone: no removal, no re-application,
// no label churn on every re-dispatch. Without this the step would remove and re-apply the
// labels on every run, filling the history with noise and briefly leaving the PR unstamped.
// An identical existing stamp is a NO-OP, and the step says so.
func TestStampLeavesItsOwnStampAlone(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	d := dispatcherAppLogin(t)
	model := deskkit.DispatchedModelPrefix + "example-model-1"
	tier := deskkit.DispatchedTierPrefix + "strong"
	gh := installGHStamp(t)
	gh.labels = []string{model, tier}
	gh.timeline = []ghStampEvt{labeled(model, d), labeled(tier, d)}

	if rc := run(stampArgs(t, root)); rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	if gh.wrote() {
		t.Errorf("the dispatcher rewrote its OWN standing stamp — an identical stamp is a no-op: %+v", gh.requests)
	}
}

// A SUPERSEDED foreign application is not a reason to remove anything: the dispatcher already
// repaired this PR, and removing the label again would undo its own repair. The forge's
// label-event read serves applications only, in order, so the LAST application of each label
// is the standing one — here the dispatcher's.
func TestStampDoesNotRemoveAnAlreadyRepairedStamp(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	d := dispatcherAppLogin(t)
	model := deskkit.DispatchedModelPrefix + "example-model-1"
	tier := deskkit.DispatchedTierPrefix + "strong"
	gh := installGHStamp(t)
	gh.labels = []string{model, tier}
	gh.timeline = []ghStampEvt{
		labeled(model, "some-other-login"), labeled(tier, "some-other-login"),
		unlabeled(model, d), unlabeled(tier, d),
		labeled(model, d), labeled(tier, d),
	}

	if rc := run(stampArgs(t, root)); rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	if gh.removed(model) || gh.removed(tier) {
		t.Errorf("a PR the dispatcher had already re-stamped was stripped again: %+v", gh.requests)
	}
}

// A read the step cannot complete is UNVERIFIABLE, and NO label is written. Stamping blind
// would report success while adding over a foreign label — the exact no-op this step
// exists to defeat.
func TestStampRefusesWhenTheLabelReadFails(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	gh := installGHStamp(t)
	gh.failPR = true

	rc := run(stampArgs(t, root))
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (an unreadable label set is could-not-check)", rc, deskkit.ExitUnverifiable)
	}
	if gh.wrote() {
		t.Errorf("a label was written on an unverifiable read: %+v", gh.requests)
	}
}

// The history read is the other half of the same rule: the present labels alone cannot say
// WHO applied them, so an unreadable history means a foreign application could not be told
// from the dispatcher's own — and the step must not guess.
func TestStampRefusesWhenTheHistoryReadFails(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	gh := installGHStamp(t)
	gh.labels = []string{deskkit.DispatchedModelPrefix + "example-model-1"}
	gh.failTL = true

	rc := run(stampArgs(t, root))
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (an unreadable label history is could-not-check)", rc, deskkit.ExitUnverifiable)
	}
	if gh.wrote() {
		t.Errorf("a label was written on an unverifiable history read: %+v", gh.requests)
	}
}
