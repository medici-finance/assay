package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// restlane_test.go — the REST fallback behind a git lane that answers with the forge's bare,
// reasonless "failure". No local git server emits that word, so the git lane is stubbed through
// the pushRefUpdateFn / deleteRefFn seams; the REST lane runs for real against an httptest
// server modelling the four GitHub git-data endpoints it calls. Every case asserts the three
// properties the lane exists to keep: create-if-absent (422 "already exists" = HELD), release
// only of the value that was read, and fail-closed when both lanes refuse.

const testToken = "test-installation-token"

// fakeRefsAPI models GitHub's git-data REST surface for one repo: blobs, tags, refs.
type fakeRefsAPI struct {
	mu   sync.Mutex
	refs map[string]string // "refs/dispatch/<id>" -> object sha
	tags map[string]string // tag sha -> message
	log  []string          // "METHOD path" of every request, in order

	refuseRefs   int // non-zero: POST/DELETE git/refs answer with this status
	refuseRefMsg string
	sawBadAuth   bool
}

func newFakeRefsAPI() *fakeRefsAPI {
	return &fakeRefsAPI{refs: map[string]string{}, tags: map[string]string{}}
}

func (f *fakeRefsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, r.Method+" "+r.URL.EscapedPath())
	if r.Header.Get("Authorization") != "Bearer "+testToken {
		f.sawBadAuth = true
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	const repo = "/repos/o/r"
	path := strings.TrimPrefix(r.URL.Path, repo)
	reply := func(code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if v != nil {
			_ = json.NewEncoder(w).Encode(v)
		}
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	switch {
	case r.Method == http.MethodPost && path == "/git/blobs":
		reply(http.StatusCreated, map[string]string{"sha": gitcore.EmptyBlobHash})
	case r.Method == http.MethodPost && path == "/git/tags":
		sha := strings.Repeat("a", 39) + string(rune('0'+len(f.tags)%10))
		f.tags[sha] = body["message"].(string)
		reply(http.StatusCreated, map[string]string{"sha": sha})
	case r.Method == http.MethodPost && path == "/git/refs":
		if f.refuseRefs != 0 {
			reply(f.refuseRefs, map[string]string{"message": f.refuseRefMsg})
			return
		}
		ref := body["ref"].(string)
		if _, ok := f.refs[ref]; ok {
			reply(http.StatusUnprocessableEntity, map[string]string{"message": "Reference already exists"})
			return
		}
		f.refs[ref] = body["sha"].(string)
		reply(http.StatusCreated, map[string]any{"ref": ref})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/git/ref/"):
		ref := "refs/" + strings.TrimPrefix(path, "/git/ref/")
		sha, ok := f.refs[ref]
		if !ok {
			reply(http.StatusNotFound, map[string]string{"message": "Not Found"})
			return
		}
		reply(http.StatusOK, map[string]any{"ref": ref, "object": map[string]string{"sha": sha, "type": "tag"}})
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/git/refs/"):
		if f.refuseRefs != 0 {
			reply(f.refuseRefs, map[string]string{"message": f.refuseRefMsg})
			return
		}
		ref := "refs/" + strings.TrimPrefix(path, "/git/refs/")
		if _, ok := f.refs[ref]; !ok {
			reply(http.StatusUnprocessableEntity, map[string]string{"message": "Reference does not exist"})
			return
		}
		delete(f.refs, ref)
		w.WriteHeader(http.StatusNoContent)
	default:
		reply(http.StatusNotFound, map[string]string{"message": "unmodelled " + r.Method + " " + path})
	}
}

func (f *fakeRefsAPI) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.log...)
}

// restStore returns a gogitStore whose REST lane points at a fresh fake, plus the fake.
func restStore(t *testing.T) (*gogitStore, *fakeRefsAPI) {
	t.Helper()
	api := newFakeRefsAPI()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	old := restAPIBase
	restAPIBase = srv.URL
	t.Cleanup(func() { restAPIBase = old })
	g := &gogitStore{url: "https://github.com/o/r.git", host: "github.com",
		rest: newRESTLane(deskkit.ForgeGitHub, "github.com", "o", "r", testToken)}
	if g.rest == nil {
		t.Fatal("newRESTLane returned no lane for a GitHub repo on github.com")
	}
	return g, api
}

// stubGitPush makes the git lane's ref update answer with the given verdict.
func stubGitPush(t *testing.T, v gitcore.RefUpdateVerdict, err error) *int {
	t.Helper()
	calls := 0
	old := pushRefUpdateFn
	pushRefUpdateFn = func(context.Context, gitcore.RefUpdate) (gitcore.RefUpdateVerdict, error) {
		calls++
		return v, err
	}
	t.Cleanup(func() { pushRefUpdateFn = old })
	return &calls
}

// stubGitDelete makes the git lane's delete answer with the given result/error.
func stubGitDelete(t *testing.T, res gitcore.DeleteResult, err error) {
	t.Helper()
	old := deleteRefFn
	deleteRefFn = func(context.Context, string, transport.AuthMethod, plumbing.ReferenceName) (gitcore.DeleteResult, error) {
		return res, err
	}
	t.Cleanup(func() { deleteRefFn = old })
}

func captureErr(t *testing.T) *bytes.Buffer {
	t.Helper()
	var b bytes.Buffer
	old := errOut
	errOut = &b
	t.Cleanup(func() { errOut = old })
	return &b
}

var genericFailure = gitcore.RefUpdateVerdict{Result: gitcore.RefUpdateRejected, Status: "failure"}

const probeID = "at--issue-4242"

// FREE: the git lane says "failure", the REST lane creates the ref — Applied, with the claim tag
// carrying the exact wire-contract message, and a NOTICE saying which lane wrote it.
func TestRESTFallbackCreatesClaimAfterGenericGitFailure(t *testing.T) {
	g, api := restStore(t)
	stubGitPush(t, genericFailure, nil)
	se := captureErr(t)
	msg := claimMessage(probeID, "sess-A", "claimed", "-", "")

	if got := g.CreateIfAbsent(probeID, msg); got != deskkit.ClaimWriteApplied {
		t.Fatalf("CreateIfAbsent = %v, want Applied through the REST lane; cause=%q requests=%v", got, g.TransportCause(), api.requests())
	}
	sha, ok := api.refs[refPrefix+"/"+probeID]
	if !ok {
		t.Fatalf("REST lane reported Applied but created no ref; requests=%v", api.requests())
	}
	if api.tags[sha] != msg {
		t.Fatalf("claim tag message = %q, want the wire-contract message %q", api.tags[sha], msg)
	}
	if c := g.TransportCause(); c != "" {
		t.Fatalf("a create the REST lane applied left cause %q, want none", c)
	}
	if !strings.Contains(se.String(), "REST refs API") {
		t.Fatalf("no NOTICE naming the lane that wrote the claim; stderr=%q", se.String())
	}
	if api.sawBadAuth {
		t.Fatal("the REST lane did not send the store's own token")
	}
}

// HELD: the REST create answers 422 "Reference already exists" — the create-if-absent answer.
// It must come back as Rejected (the verb then reads the holder), NEVER as Applied.
func TestRESTFallbackCreate422IsHeldNeverApplied(t *testing.T) {
	g, api := restStore(t)
	stubGitPush(t, genericFailure, nil)
	api.refs[refPrefix+"/"+probeID] = strings.Repeat("b", 40) // a live holder the git lane never reported

	if got := g.CreateIfAbsent(probeID, claimMessage(probeID, "sess-B", "claimed", "-", "")); got != deskkit.ClaimWriteRejected {
		t.Fatalf("CreateIfAbsent over an existing ref = %v, want Rejected (HELD)", got)
	}
	if api.refs[refPrefix+"/"+probeID] != strings.Repeat("b", 40) {
		t.Fatal("the REST lane overwrote a held claim")
	}
	if c := g.TransportCause(); !strings.Contains(c, "failure") || !strings.Contains(c, "already exists") {
		t.Fatalf("cause = %q, want both the git refusal and the REST 'already exists'", c)
	}
}

// REFUSED: both lanes refuse — fail closed (Unverifiable), naming both answers.
func TestRESTFallbackCreateBothLanesRefusedFailsClosed(t *testing.T) {
	g, api := restStore(t)
	stubGitPush(t, gitcore.RefUpdateVerdict{Result: gitcore.RefUpdateRejected, Status: "failure", Remote: "internal error"}, nil)
	api.refuseRefs, api.refuseRefMsg = http.StatusForbidden, "Resource not accessible by integration"

	if got := g.CreateIfAbsent(probeID, claimMessage(probeID, "sess-A", "claimed", "-", "")); got != deskkit.ClaimWriteUnverifiable {
		t.Fatalf("CreateIfAbsent with both lanes refusing = %v, want Unverifiable", got)
	}
	c := g.TransportCause()
	for _, want := range []string{"failure (remote: internal error)", "HTTP 403", "Resource not accessible by integration"} {
		if !strings.Contains(c, want) {
			t.Errorf("cause %q missing %q", c, want)
		}
	}
	if len(api.refs) != 0 {
		t.Fatalf("a refused create left refs behind: %v", api.refs)
	}
}

// A NAMED refusal (the CAS losing, a hook) is final: the REST lane is never consulted.
func TestRESTFallbackNotTakenForNamedRefusal(t *testing.T) {
	for _, status := range []string{
		"cannot lock ref 'refs/dispatch/at--issue-4242': reference already exists",
		"pre-receive hook declined",
		"stale info",
	} {
		g, api := restStore(t)
		stubGitPush(t, gitcore.RefUpdateVerdict{Result: gitcore.RefUpdateRejected, Status: status}, nil)
		if got := g.CreateIfAbsent(probeID, claimMessage(probeID, "sess-A", "claimed", "-", "")); got != deskkit.ClaimWriteRejected {
			t.Fatalf("status %q: CreateIfAbsent = %v, want Rejected", status, got)
		}
		if reqs := api.requests(); len(reqs) != 0 {
			t.Fatalf("status %q: the REST lane was consulted for a named refusal: %v", status, reqs)
		}
	}
}

// An advance/steal (old≠zero) never falls back: REST cannot keep its compare-and-swap.
func TestRESTFallbackNotTakenForUpdate(t *testing.T) {
	g, api := restStore(t)
	stubGitPush(t, genericFailure, nil)
	if got := g.UpdateFrom(probeID, strings.Repeat("c", 40), claimMessage(probeID, "sess-A", "dispatched", "b", "")); got != deskkit.ClaimWriteRejected {
		t.Fatalf("UpdateFrom = %v, want the git lane's Rejected to stand", got)
	}
	if reqs := api.requests(); len(reqs) != 0 {
		t.Fatalf("an advance consulted the REST lane: %v", reqs)
	}
}

// No lane off github.com: the git refusal stands, exactly as before the lane existed.
func TestNoRESTLaneOffGitHubDotCom(t *testing.T) {
	for _, tc := range []struct {
		kind deskkit.ForgeKind
		host string
	}{
		{deskkit.ForgeGitLab, "gitlab.com"},
		{deskkit.ForgeGitHub, "github.example.com"},
		{deskkit.ForgeGitHub, ""},
	} {
		if l := newRESTLane(tc.kind, tc.host, "o", "r", testToken); l != nil {
			t.Errorf("newRESTLane(%s, %q) built a lane; want none", tc.kind, tc.host)
		}
	}
}

// --- release ------------------------------------------------------------------

func rejectedDelete(old string) error {
	return &gitcore.RefRejectedError{Ref: plumbing.ReferenceName(refPrefix + "/" + probeID), Old: plumbing.NewHash(old), Status: "failure"}
}

// The git delete says "failure"; the ref still holds the value that delete was CAS'd against —
// the REST lane deletes it. Applied, existed.
func TestRESTFallbackReleasesTheValueTheGitDeleteRead(t *testing.T) {
	g, api := restStore(t)
	held := strings.Repeat("d", 40)
	api.refs[refPrefix+"/"+probeID] = held
	stubGitDelete(t, 0, rejectedDelete(held))
	captureErr(t)

	out, existed := g.Remove(probeID)
	if out != deskkit.ClaimWriteApplied || !existed {
		t.Fatalf("Remove = (%v, %v), want (Applied, true); cause=%q", out, existed, g.TransportCause())
	}
	if _, still := api.refs[refPrefix+"/"+probeID]; still {
		t.Fatal("REST lane reported released but the ref is still there")
	}
}

// The claim MOVED between the git read and the REST lane (re-taken by another desk): nothing is
// deleted and the release is could-not-check.
func TestRESTFallbackNeverDeletesAClaimThatMoved(t *testing.T) {
	g, api := restStore(t)
	api.refs[refPrefix+"/"+probeID] = strings.Repeat("e", 40) // re-taken
	stubGitDelete(t, 0, rejectedDelete(strings.Repeat("d", 40)))

	out, _ := g.Remove(probeID)
	if out != deskkit.ClaimWriteUnverifiable {
		t.Fatalf("Remove of a moved claim = %v, want Unverifiable", out)
	}
	if api.refs[refPrefix+"/"+probeID] != strings.Repeat("e", 40) {
		t.Fatal("the REST lane deleted a claim this release never read")
	}
	for _, r := range api.requests() {
		if strings.HasPrefix(r, "DELETE") {
			t.Fatalf("a DELETE was sent for a moved claim: %v", api.requests())
		}
	}
}

// Already gone by the time the REST lane looks: a released no-op, as on the git lane.
func TestRESTFallbackReleaseOfVanishedRefIsNoop(t *testing.T) {
	g, _ := restStore(t)
	stubGitDelete(t, 0, rejectedDelete(strings.Repeat("d", 40)))
	captureErr(t)
	out, existed := g.Remove(probeID)
	if out != deskkit.ClaimWriteApplied || existed {
		t.Fatalf("Remove of a vanished ref = (%v, %v), want (Applied, false)", out, existed)
	}
}

// Both lanes refuse the release: could-not-check, naming both.
func TestRESTFallbackReleaseBothLanesRefusedFailsClosed(t *testing.T) {
	g, api := restStore(t)
	held := strings.Repeat("d", 40)
	api.refs[refPrefix+"/"+probeID] = held
	api.refuseRefs, api.refuseRefMsg = http.StatusForbidden, "Resource not accessible by integration"
	stubGitDelete(t, 0, rejectedDelete(held))

	out, _ := g.Remove(probeID)
	if out != deskkit.ClaimWriteUnverifiable {
		t.Fatalf("Remove with both lanes refusing = %v, want Unverifiable", out)
	}
	c := g.TransportCause()
	if !strings.Contains(c, "rejected: failure") || !strings.Contains(c, "HTTP 403") {
		t.Fatalf("cause %q must name both the git refusal and the REST refusal", c)
	}
}

// A transport error (not a server refusal) on the git delete never falls back.
func TestRESTFallbackNotTakenForGitTransportError(t *testing.T) {
	g, api := restStore(t)
	stubGitDelete(t, 0, errors.New("gitcore: receive-pack advertise: authentication required"))
	if out, _ := g.Remove(probeID); out != deskkit.ClaimWriteUnverifiable {
		t.Fatalf("Remove on a transport error = %v, want Unverifiable", out)
	}
	if reqs := api.requests(); len(reqs) != 0 {
		t.Fatalf("a transport error consulted the REST lane: %v", reqs)
	}
}

// Verb level: acquire through the fallback exits 0 with the ordinary "acquired" line.
func TestAcquireVerbSucceedsThroughRESTFallback(t *testing.T) {
	g, _ := restStore(t)
	stubGitPush(t, genericFailure, nil)
	var so, se bytes.Buffer
	oldBuild, oldOut, oldErr := buildStore, out, errOut
	buildStore = func(_, _ string) (deskkit.ClaimStore, error) { return g, nil }
	out, errOut = &so, &se
	t.Cleanup(func() { buildStore, out, errOut = oldBuild, oldOut, oldErr })

	if rc := run([]string{"acquire", probeID, "--repo", "o/r", "--owner", "sess-A"}); rc != exitOK {
		t.Fatalf("acquire rc = %d, want 0; stdout=%q stderr=%q", rc, so.String(), se.String())
	}
	if !strings.Contains(so.String(), "acquired "+probeID) {
		t.Fatalf("stdout = %q, want the ordinary acquired line", so.String())
	}
}
