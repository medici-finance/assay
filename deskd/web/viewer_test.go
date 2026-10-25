package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/index"
)

type fakeViewer struct{ info identity.ViewerInfo }

func (f fakeViewer) ViewerFor(*http.Request, string, string) identity.ViewerInfo { return f.info }
func (f fakeViewer) SurfaceViewer(*http.Request) identity.ViewerInfo             { return f.info }

// SurfaceAuthorizer for the chrome tests allows every repo — these tests assert
// the sign-in/affordance chrome, not the row-level capability filter (which the
// capability-gate tests exercise directly). An allow-all keeps seeded rows
// visible so the chrome assertions are unaffected.
func (f fakeViewer) SurfaceAuthorizer(*http.Request) func(owner, name string) bool {
	return func(string, string) bool { return true }
}

func renderWithViewer(t *testing.T, db *index.DB, path string, v ViewerSource) (int, string) {
	t.Helper()
	srv, err := New(Deps{DB: db, Now: func() time.Time { return time.Now().UTC() }, Viewer: v})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr.Code, rr.Body.String()
}

func seedOne(t *testing.T) (*index.DB, string) {
	escs := []domain.Escalation{{
		ID: "IT1", Cell: "assay", Automation: attributed(true), Kind: domain.KindNeedsDecision,
		Provenance: domain.AgentRaised, Urgency: domain.UrgencyRoutine,
		Title: "decide", Situation: "a real decision", Rev: "r1", FiledAt: time.Now().Add(-time.Hour),
	}}
	raw := map[domain.EscalationID]adapter.RawIssue{"IT1": {Number: 11, URL: issueURL(11)}}
	db := seed(t, escs, raw)
	rows, err := db.Inbox("", true)
	if err != nil || len(rows) == 0 {
		t.Fatalf("seed produced no rows: %v", err)
	}
	return db, string(rows[0].ID)
}

// TestItemAffordancesByTier: the answer block renders ONLY for
// an answerer; the comment box renders for a commenter; an unauthenticated
// viewer sees the sign-in affordance and neither write surface. The server still
// gates every route regardless — this only checks the CHROME matches the tier.
func TestItemAffordancesByTier(t *testing.T) {
	db, id := seedOne(t)
	path := "/item/" + id

	// Answerer: answer block present.
	_, ansBody := renderWithViewer(t, db, path, fakeViewer{identity.ViewerInfo{
		Authenticated: true, Login: "operator", Capability: "answerer", CanComment: true, CanAnswer: true,
	}})
	if !strings.Contains(ansBody, "answer-block") {
		t.Fatal("answerer should see the answer block")
	}
	if !strings.Contains(ansBody, "answer as operator") {
		t.Fatal("answer heading should name the authenticated answerer")
	}

	// Commenter: comment box present, answer block ABSENT.
	_, comBody := renderWithViewer(t, db, path, fakeViewer{identity.ViewerInfo{
		Authenticated: true, Login: "bob", Capability: "commenter", CanComment: true, CanAnswer: false,
	}})
	if strings.Contains(comBody, "answer-block") {
		t.Fatal("a commenter must NOT see the answer block (UI affordance gating)")
	}
	if !strings.Contains(comBody, "comment-block") || !strings.Contains(comBody, "comment as bob") {
		t.Fatal("a commenter should see the comment box naming their login")
	}

	// Anonymous: neither write surface, sign-in shown.
	_, anonBody := renderWithViewer(t, db, path, fakeViewer{identity.ViewerInfo{}})
	if strings.Contains(anonBody, "answer-block") || strings.Contains(anonBody, "comment-block") {
		t.Fatal("an anonymous viewer must see no write surface")
	}
	if !strings.Contains(anonBody, "/auth/login") {
		t.Fatal("an anonymous viewer should see a sign-in affordance")
	}
}

// denyRepoViewer authenticates a login for the chrome but DENIES a named repo
// on the surface authorizer — the shape of an authenticated viewer who holds no
// read capability on a cell's private repo.
type denyRepoViewer struct {
	info      identity.ViewerInfo
	denyOwner string
	denyName  string
}

func (v denyRepoViewer) ViewerFor(*http.Request, string, string) identity.ViewerInfo {
	return v.info
}
func (v denyRepoViewer) SurfaceViewer(*http.Request) identity.ViewerInfo { return v.info }
func (v denyRepoViewer) SurfaceAuthorizer(*http.Request) func(owner, name string) bool {
	return func(owner, name string) bool {
		return !(owner == v.denyOwner && name == v.denyName)
	}
}

// TestInboxHidesRowsWithoutCapability (the capability model security fix, Security-Review
// 5091037270): an authenticated viewer with NO read capability on a cell's
// private repos must NOT receive that cell's inbox metadata. Pre-fix the surface
// listing gated on login-presence alone (SurfaceLoginGated() && sess==nil) and
// served every row across every cell, so this row's title/deep-link leaked to
// an authenticated-but-unauthorized viewer. Post-fix the listing is filtered
// per-repo through the capability authorizer, so the row is dropped.
func TestInboxHidesRowsWithoutCapability(t *testing.T) {
	db, id := seedOne(t) // row IT1 on example-org/console, a PRIVATE repo

	// Authenticated as bob, who is Anonymous (no read) on the row's repo.
	viewer := denyRepoViewer{
		info:      identity.ViewerInfo{Authenticated: true, Login: "bob"},
		denyOwner: "example-org",
		denyName:  "console",
	}
	code, body := renderWithViewer(t, db, "/inbox", viewer)
	if code == http.StatusUnauthorized {
		t.Fatalf("authenticated viewer got 401 on the surface; want a filtered feed, not a block")
	}
	// The private row's withheld metadata must be absent from the feed.
	if strings.Contains(body, "/item/"+id) {
		t.Fatalf("LEAK: an authenticated viewer with no read on the row's private repo received its item deep-link (/item/%s)", id)
	}
	if strings.Contains(body, "a real decision") {
		t.Fatal("LEAK: the private row's situation text was served to an unauthorized authenticated viewer")
	}

	// Control: a viewer authorized on that repo still sees the row.
	okViewer := denyRepoViewer{
		info:      identity.ViewerInfo{Authenticated: true, Login: "operator"},
		denyOwner: "nobody",
		denyName:  "nowhere",
	}
	_, okBody := renderWithViewer(t, db, "/inbox", okViewer)
	if !strings.Contains(okBody, "/item/"+id) {
		t.Fatal("an authorized viewer must still see the row — the filter over-blocked")
	}
}

// TestInboxChromeSignIn: the inbox chrome shows sign-in when anonymous and the
// login when authenticated.
func TestInboxChromeSignIn(t *testing.T) {
	db, _ := seedOne(t)
	_, anon := renderWithViewer(t, db, "/inbox", fakeViewer{identity.ViewerInfo{}})
	if !strings.Contains(anon, "/auth/login") {
		t.Fatal("anonymous inbox should offer sign-in")
	}
	_, authed := renderWithViewer(t, db, "/inbox", fakeViewer{identity.ViewerInfo{Authenticated: true, Login: "bob"}})
	if !strings.Contains(authed, "bob") || !strings.Contains(authed, "sign out") {
		t.Fatal("authenticated inbox should show the login and a sign-out")
	}
}
