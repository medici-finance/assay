package main

// harness_test.go — the RECORDING HTTP TRANSPORT this package's tests drive the verb
// through, and the successor to the `gh`-argv recorder they used before the forge migration.
//
// WHY IT LOOKS LIKE THIS. deskflip used to reach the forge by launching `gh`, so its tests
// wrapped execCommand and asserted on the constructed argv ("pr ready 7", "--add-label …").
// Every forge read and write now goes through the resolved deskkit.Forge — an HTTP client
// bound to an explicitly minted App token — so the argv recorder has nothing left to record.
// The successor records the same facts one layer down: METHOD, PATH, QUERY, BODY and the
// Authorization header of every request the verb actually emits. Each retired argv assertion
// has a named successor here (see the map in deskflip_test.go's header), and the successors
// are strictly more specific: an argv assertion could only see the words a CLI was given,
// while these see the request that reached the forge.
//
// The whole verb runs against it: the resolver constructs a real GitHubForge, the custody
// hook hands it the stubbed token, and forgeAPIBase points it at this server. Nothing about
// the code path under test is stubbed out — only the far side of the wire is.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// recordedReq is one request the verb emitted, in the form the assertions read.
type recordedReq struct {
	Method string
	Path   string
	Query  string
	Body   string
	Auth   string
}

func (r recordedReq) String() string {
	if r.Query == "" {
		return r.Method + " " + r.Path
	}
	return r.Method + " " + r.Path + "?" + r.Query
}

// prFixture is the change the fake forge serves. It is the REST payload's fields, named as
// the gate names them, so a test still reads as "a PR that is open, draft, mergeable".
type prFixture struct {
	Number       int
	State        string
	IsDraft      bool
	Mergeable    string
	HeadRefOid   string
	ChangedFiles int
	Labels       []string
	NodeID       string
}

// stub is the fake forge instance plus every knob a case needs. A case sets only what it is
// about; everything else has the green default.
type stub struct {
	srv      *httptest.Server
	requests []recordedReq

	pr     prFixture
	rollup []rollupEntry

	reviews []reviewInfo
	// reviewsAfterFirstRead, when non-nil, is what EVERY read of the reviews endpoint after
	// the first returns. It models the race the pre-mutation re-read exists to close: a
	// verdict posted at the SAME head while the checks were running, which no amount of head
	// re-reading can see because the head never moves for it.
	reviewsAfterFirstRead []reviewInfo
	reviewReads           int

	// head2 is what the SECOND read of the change reports as its head; "" means unchanged.
	head2   string
	prReads int

	failPR bool
	// failPath makes every request whose method+path+query contains this fragment answer 500
	// — the successor to the old recorder's `failGH` argv fragment.
	failPath string

	// files is the COMPLETE changed-file list the fake forge will SERVE, independent of what
	// pr.ChangedFiles ASSERTS. Keeping the two independent is the point: the pair (assert
	// 163, serve 100) is a truncating forge, and the pair (assert 163, serve 163 across two
	// pages) is the case the verb has to walk. nil means "the one-file diff greenPR
	// describes"; an explicitly empty slice serves nothing.
	files []string
	// filesPerPage is the fake forge's page size. Zero means the size the verb asks for, so a
	// 163-entry list lands as 100 + 63 without any test having to say so.
	filesPerPage int
	fileReads    int

	// labelEvents is the change's label-application timeline — the dispatcher attestation the
	// model-capability floor reads. nil serves an empty timeline, which the floor reads as
	// UNATTESTED (a NOTICE, not a refusal). Each event carries the applier login, so a
	// fixture can distinguish a dispatcher stamp from a self-applied one.
	labelEvents []deskkit.LabelEvent
	timelineErr bool

	// checkTotalOverride, when non-zero, is the total_count the check-runs rollup ASSERTS
	// regardless of how many it serves — a forge that claims more than it hands over.
	checkTotalOverride int
}

const stubToken = "stub-installation-token"

func (s *stub) install(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_SESSION", "deskflip-test")
	t.Setenv("CLAUDE_SESSION_ID", "deskflip-test")
	t.Setenv("DESK_LOOP", flipRole)

	// The App-token condition is exercised past its refusal here: every case below is about a
	// LATER condition, and a verb that could not authenticate would never reach one. The
	// refusal itself has its own tests (identity_test.go), which is what keeps this stub from
	// making the condition vacuous.
	oldMint := mintTokenFn
	mintTokenFn = func(role, repo string) (string, string, error) {
		return stubToken, filepath.Join(home, ".config", "assay", role+"-token-stub"), nil
	}
	t.Cleanup(func() { mintTokenFn = oldMint })

	if s.files == nil {
		s.files = greenFiles()
	}
	if s.pr.NodeID == "" {
		s.pr.NodeID = "PR_stub_node_id"
	}

	s.srv = httptest.NewServer(http.HandlerFunc(s.handle(t)))
	t.Cleanup(s.srv.Close)

	oldBase := forgeAPIBase
	forgeAPIBase = s.srv.URL
	t.Cleanup(func() { forgeAPIBase = oldBase })

	// git is the only child process left, and no case here needs one: --repo is always given,
	// so resolveRepo never shells out. The seam is still pinned to a recorder so a
	// reintroduced subprocess would be visible rather than silently launched.
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		t.Errorf("the verb launched a subprocess (%s %s) — every forge call goes through the "+
			"resolved backend now", name, strings.Join(args, " "))
		// A name no PATH resolves: the call fails without a shell, and the t.Errorf above is
		// what actually reports the regression.
		return exec.Command("deskflip-must-not-launch-subprocesses")
	}
	t.Cleanup(func() { execCommand = oldExec })

	return root
}

func (s *stub) handle(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body bytes.Buffer
		if r.Body != nil {
			_, _ = body.ReadFrom(r.Body)
		}
		rec := recordedReq{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery,
			Body: body.String(), Auth: r.Header.Get("Authorization"),
		}
		s.requests = append(s.requests, rec)
		path := r.URL.Path
		enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }

		if s.failPath != "" && strings.Contains(rec.String(), s.failPath) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		switch {
		case r.Method == http.MethodPost && path == "/graphql":
			enc(map[string]any{"data": map[string]any{
				"markPullRequestReadyForReview": map[string]any{
					"pullRequest": map[string]any{"isDraft": false}}}})

		case strings.HasSuffix(path, "/timeline"):
			if s.timelineErr {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if r.URL.Query().Get("page") != "1" {
				enc([]map[string]any{})
				return
			}
			enc(s.servedTimeline())

		case strings.HasSuffix(path, "/reviews"):
			if r.URL.Query().Get("page") != "1" {
				enc([]map[string]any{})
				return
			}
			s.reviewReads++
			src := s.reviews
			if s.reviewReads > 1 && s.reviewsAfterFirstRead != nil {
				src = s.reviewsAfterFirstRead
			}
			enc(servedReviews(src))

		case strings.HasSuffix(path, "/files"):
			s.fileReads++
			enc(s.servedFilePage(r.URL.Query().Get("page")))

		case strings.HasSuffix(path, "/status"):
			enc(s.servedStatuses())

		case strings.HasSuffix(path, "/check-runs"):
			enc(s.servedCheckRuns())

		case r.Method == http.MethodGet && strings.Contains(path, "/pulls/"):
			if s.failPR {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			s.prReads++
			enc(s.servedPR())

		case strings.HasSuffix(path, "/labels") || strings.Contains(path, "/labels/"):
			// Label create / list / add / remove all live under this shape. The list read is
			// the only one that needs a payload; the writes answer 200 with an empty set.
			if r.Method == http.MethodGet {
				if r.URL.Query().Get("page") != "1" {
					enc([]map[string]any{})
					return
				}
				out := make([]map[string]any, 0, len(s.pr.Labels))
				for _, l := range s.pr.Labels {
					out = append(out, map[string]any{"name": l})
				}
				enc(out)
				return
			}
			enc([]map[string]any{})

		default:
			t.Errorf("the verb reached an endpoint the fake forge does not serve: %s", rec)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// servedPR renders the change. The SECOND read reports head2 when a case set one — the
// TOCTOU race, which on the forge seam is a second GetPullRequest rather than a second CLI
// invocation.
func (s *stub) servedPR() map[string]any {
	head := s.pr.HeadRefOid
	if s.prReads > 1 && s.head2 != "" {
		head = s.head2
	}
	labels := make([]map[string]any, 0, len(s.pr.Labels))
	for _, l := range s.pr.Labels {
		labels = append(labels, map[string]any{"name": l})
	}
	out := map[string]any{
		"number": s.pr.Number, "state": strings.ToLower(s.pr.State), "draft": s.pr.IsDraft,
		"node_id": s.pr.NodeID, "changed_files": s.pr.ChangedFiles,
		"user":   map[string]any{"login": "worker[bot]", "id": 99},
		"head":   map[string]any{"sha": head, "ref": "feat/x"},
		"labels": labels,
	}
	switch s.pr.Mergeable {
	case deskkit.Mergeable:
		out["mergeable"] = true
	case deskkit.MergeableConflicting:
		out["mergeable"] = false
	default:
		out["mergeable"] = nil // GitHub's "not computed yet"
	}
	return out
}

func servedReviews(in []reviewInfo) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for i, r := range in {
		out = append(out, map[string]any{
			"id": i + 1, "state": r.State, "commit_id": r.CommitID, "body": r.Body,
			"submitted_at": r.SubmittedAt,
			"user":         map[string]any{"login": r.User.Login, "id": int64(i + 1)},
		})
	}
	return out
}

// servedFilePage renders ONE page of s.files the way the forge paginates the changed-files
// endpoint. Reproducing the paging rather than one flat array is deliberate: a fake that
// returned everything on page one would let a reader that never walks pass, which is the very
// defect these tests exist to pin.
func (s *stub) servedFilePage(pageStr string) []map[string]any {
	size := s.filesPerPage
	if size == 0 {
		size = 100
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	start := (page - 1) * size
	if start >= len(s.files) {
		return []map[string]any{}
	}
	end := start + size
	if end > len(s.files) {
		end = len(s.files)
	}
	out := make([]map[string]any, 0, end-start)
	for _, f := range s.files[start:end] {
		out = append(out, map[string]any{"filename": f, "status": "modified"})
	}
	return out
}

// servedTimeline renders s.labelEvents as the `labeled` timeline shape the floor reads. A nil
// slice serves an empty array — a change with no labels, which the floor reads as UNATTESTED.
func (s *stub) servedTimeline() []map[string]any {
	out := make([]map[string]any, 0, len(s.labelEvents))
	for _, e := range s.labelEvents {
		out = append(out, map[string]any{
			"event": "labeled",
			"label": map[string]any{"name": e.Name},
			"actor": map[string]any{"login": e.AppliedBy},
		})
	}
	return out
}

// servedStatuses / servedCheckRuns split s.rollup by SHAPE: an entry with a Context is a
// legacy status context, an entry with a Name is a check run. Both shapes must be served
// because a reader that understands only one silently sees an empty rollup for the other,
// which on a CI-less policy reads as green.
func (s *stub) servedStatuses() map[string]any {
	items := []map[string]any{}
	for _, e := range s.rollup {
		if e.Context == "" {
			continue
		}
		items = append(items, map[string]any{
			"context": e.Context, "state": e.State, "created_at": e.CreatedAt,
		})
	}
	return map[string]any{"state": "success", "total_count": len(items), "statuses": items}
}

func (s *stub) servedCheckRuns() map[string]any {
	items := []map[string]any{}
	for _, e := range s.rollup {
		if e.Name == "" {
			continue
		}
		items = append(items, map[string]any{
			"name": e.Name, "status": e.Status, "conclusion": e.Conclusion,
			"started_at": e.StartedAt, "completed_at": e.CompletedAt,
		})
	}
	total := len(items)
	if s.checkTotalOverride != 0 {
		total = s.checkTotalOverride
	}
	return map[string]any{"total_count": total, "check_runs": items}
}

// --- assertions over the recording -------------------------------------------------

// saw reports whether ANY request matches method and a path fragment.
func (s *stub) saw(method, fragment string) bool {
	for _, r := range s.requests {
		if r.Method == method && strings.Contains(r.Path, fragment) {
			return true
		}
	}
	return false
}

// sawBody reports whether any request of that method+path fragment carried a body fragment.
func (s *stub) sawBody(method, pathFragment, bodyFragment string) bool {
	for _, r := range s.requests {
		if r.Method == method && strings.Contains(r.Path, pathFragment) &&
			strings.Contains(r.Body, bodyFragment) {
			return true
		}
	}
	return false
}

// count returns how many requests match method + path fragment.
func (s *stub) count(method, fragment string) int {
	n := 0
	for _, r := range s.requests {
		if r.Method == method && strings.Contains(r.Path, fragment) {
			n++
		}
	}
	return n
}

// indexOf returns the position of the FIRST matching request, or -1. Ordering assertions
// (the head re-read must be the last thing before the mutation) read this.
func (s *stub) indexOf(method, fragment string) int {
	for i, r := range s.requests {
		if r.Method == method && strings.Contains(r.Path, fragment) {
			return i
		}
	}
	return -1
}

// lastIndexOf returns the position of the LAST matching request, or -1.
func (s *stub) lastIndexOf(method, fragment string) int {
	out := -1
	for i, r := range s.requests {
		if r.Method == method && strings.Contains(r.Path, fragment) {
			out = i
		}
	}
	return out
}

// flipped reports whether the ready mutation was issued — the successor to the old
// `ran("pr ready 7")` argv assertion, and stricter: it checks the request BODY carries the
// mutation, not merely that something was POSTed to /graphql.
func (s *stub) flipped() bool {
	return s.sawBody(http.MethodPost, "/graphql", "markPullRequestReadyForReview")
}

// mutated reports every state-changing request the verb made. Every refusal test asserts on
// this: a gate that refuses AFTER mutating has not gated anything. It is the successor to the
// argv recorder's verb list (`pr ready`, `pr edit`, `pr merge`, `pr close`, `pr review`) and
// is BROADER than its predecessor — it catches any non-GET request at all, including ones no
// argv fragment was ever written for.
func (s *stub) mutated() []string {
	var out []string
	for _, r := range s.requests {
		if r.Method == http.MethodGet {
			continue
		}
		out = append(out, r.String())
	}
	return out
}

// --- fixtures ----------------------------------------------------------------------

const headSHA = "aaaaaaaabbbbbbbbccccccccdddddddd11111111"

// greenPR is a change that satisfies every condition: open, draft, mergeable, and carrying
// the pre-flip queue label. Its ChangedFiles pairs with greenFiles — the envelope's asserted
// total and the list the forge serves are set in two places because the verb reads them from
// two endpoints, and their agreement is what a green run depends on.
func greenPR() prFixture {
	return prFixture{
		Number: 7, State: "OPEN", IsDraft: true, Mergeable: deskkit.Mergeable,
		HeadRefOid: headSHA, ChangedFiles: 1,
		Labels: []string{labelBeforeFlip},
	}
}

// greenRollup is the one successful check greenPR's head carries.
func greenRollup() []rollupEntry {
	return []rollupEntry{{Name: "test", Status: "COMPLETED", Conclusion: "SUCCESS"}}
}

// greenFiles is the changed-file list greenPR's ChangedFiles asserts.
func greenFiles() []string { return []string{"README.md"} }

// newStub builds the fully-green stub every case starts from.
func newStub() *stub {
	return &stub{pr: greenPR(), rollup: greenRollup()}
}

// paddingFiles builds n quiet paths — the docs-only bulk that a register PR is made of, and
// the reason such a PR crosses a page boundary at all.
func paddingFiles(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, "docs/streams/example/"+strconv.Itoa(i)+".md")
	}
	return out
}

func approvalAtHead(t *testing.T, head string) []reviewInfo {
	t.Helper()
	r := reviewInfo{State: "APPROVED", CommitID: head, Body: "looks correct", SubmittedAt: "2026-01-01T00:00:00Z"}
	r.User.Login = reviewerBot(t)
	return []reviewInfo{r}
}

// The reviewer App identity is READ FROM THE FIXTURE ROSTER, never spelled as a literal here:
// the verb reads it from the roster too, so a test carrying its own copy would keep passing
// after the binding changed.
func reviewerBot(t *testing.T) string {
	t.Helper()
	login, ok := deskkit.RoleAppLogin(reviewerRole)
	if !ok {
		t.Fatal("the fixture roster does not bind the reviewer role")
	}
	return login
}

// dispatcherLogin is the roster's desk-App login — the ONLY applier whose dispatched-* stamp
// the floor trusts. Read from the fixture roster, never a literal, so a fixture stamp is
// applied by the same identity the verb vouches for.
func dispatcherLogin(t *testing.T) string {
	t.Helper()
	login, ok := deskkit.RoleAppLogin("desk")
	if !ok {
		t.Fatal("the fixture roster does not bind the desk (dispatcher) role")
	}
	return login
}

// strongStamp / cheapStamp build a complete dispatcher-applied tier attestation for a fixture
// change. The applier is the roster dispatcher, so AttestedModelStampOf trusts it.
func strongStamp(t *testing.T) []deskkit.LabelEvent {
	d := dispatcherLogin(t)
	return []deskkit.LabelEvent{
		{Name: deskkit.DispatchedModelPrefix + "opus-4.8", AppliedBy: d},
		{Name: deskkit.DispatchedTierPrefix + "strong", AppliedBy: d},
	}
}

func cheapStamp(t *testing.T) []deskkit.LabelEvent {
	d := dispatcherLogin(t)
	return []deskkit.LabelEvent{
		{Name: deskkit.DispatchedModelPrefix + "haiku-3", AppliedBy: d},
		{Name: deskkit.DispatchedTierPrefix + "any", AppliedBy: d},
	}
}

// privateCIRepo and publicRepo are read from the roster so the tests exercise the same policy
// the verb reads, rather than an assumption about it.
const (
	privateCIRepo = "medici-finance/assay"
	publicRepo    = "example-org/example-k8s"
)

// riskyPath is a path in the compiled BASE trigger set, so it risk-classes a PR in any repo.
// It is read through the shared classifier in the tests below rather than asserted as a
// literal fact about the trigger list.
const riskyPath = ".github/workflows/ci.yml"
