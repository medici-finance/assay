package main

// desklabel_test.go — the verb's suite.
//
// Two instruments. A RECORDING fake forge (fakeForge) drives every refusal, no-op and
// dry-run case, so "zero forge calls before the ownership check" is asserted against the
// calls the fake saw rather than inferred from output. The two POSITIVE end-to-end cases
// drive a REAL deskkit.GitHubForge / deskkit.GitLabForge pointed at an httptest instance,
// so the kind resolution → target dispatch → wire shape is the backend's own, not a
// double's.

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/topology"
)

const allowedRepo = "example-org/tracker"

// fakeForge records every call. Embedding a nil deskkit.Forge means any method the verb was
// not expected to reach PANICS — which is the point: the verb touches GetIssue /
// GetIssueTyped / ApplyLabels and nothing else.
type fakeForge struct {
	deskkit.Forge
	issue    *deskkit.Issue
	issueErr error
	applyErr error
	reads    []string
	writes   []deskkit.LabelChange
}

func (f *fakeForge) GetIssue(fr deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	f.reads = append(f.reads, "GetIssue")
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	iss := *f.issue
	iss.Number = n
	return &iss, nil
}

func (f *fakeForge) GetIssueTyped(fr deskkit.ForgeRepo, n int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	f.reads = append(f.reads, "GetIssueTyped:"+string(kind))
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	iss := *f.issue
	iss.Number = n
	return &iss, nil
}

func (f *fakeForge) ApplyLabels(fr deskkit.ForgeRepo, n int, change deskkit.LabelChange) (*deskkit.LabelOutcome, error) {
	f.writes = append(f.writes, change)
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	out := &deskkit.LabelOutcome{}
	for _, l := range change.Add {
		out.Added = append(out.Added, l.Name)
	}
	out.Removed = append(out.Removed, change.Remove...)
	return out, nil
}

func (f *fakeForge) calls() int { return len(f.reads) + len(f.writes) }

// plantWorld isolates HOME (roster fixture, audit log), pins the clock, fixes the session
// role, and points the forge resolver at fg. No desktoken/mint path runs.
func plantWorld(t *testing.T, role string, fg deskkit.Forge) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "desklabel-test")
	t.Setenv("DESK_LOOP", "")

	oldRole, oldForge, oldNow := roleFn, forgeForFn, nowFunc
	oldMinted, oldToken := mintedRole, ghToken
	roleFn = func() (string, error) { return role, nil }
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return fg, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	nowFunc = func() time.Time { return time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC) }
	mintedRole, ghToken = "", ""
	t.Cleanup(func() {
		roleFn, forgeForFn, nowFunc = oldRole, oldForge, oldNow
		mintedRole, ghToken = oldMinted, oldToken
	})
}

func runVerb(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	var err error
	switch args[0] {
	case "add":
		err = cmdAdd(args[1:], &buf)
	case "rm":
		err = cmdRm(args[1:], &buf)
	default:
		t.Fatalf("unknown verb %q", args[0])
	}
	out := buf.String()
	if err != nil {
		out += "\nERR: " + err.Error()
	}
	return deskkit.ExitCodeOf(err), out
}

func issueWith(labels ...string) *deskkit.Issue {
	return &deskkit.Issue{State: "open", Labels: labels}
}

// --- Negative paths: the ownership check runs FIRST and refuses before any forge call ----

// TestDesklabelRefusesUnownedLabel is Verify row 6. A worker-role session asking for a
// reviewer-owned label is refused (exit 5) with ZERO forge calls, and the message names
// both roles. The table-absent case takes the same shape, naming the missing entry.
func TestDesklabelRefusesUnownedLabel(t *testing.T) {
	cases := []struct {
		name, role, label string
		wantWords         []string
	}{
		{"worker on the reviewer's approval-needed", roleWorker, "approval-needed",
			[]string{"approval-needed", "reviewer", "worker"}},
		{"worker on the reviewer's authorization-needed", roleWorker, "authorization-needed",
			[]string{"authorization-needed", "reviewer", "worker"}},
		{"reviewer on the worker's superseded?", roleReviewer, "superseded?",
			[]string{"superseded?", "worker", "reviewer"}},
		{"reviewer on the worker's disposition family", roleReviewer, "disposition:needs-rebase",
			[]string{"disposition:needs-rebase", "worker", "reviewer"}},
		{"desk role on a worker-owned label", "desk", "superseded?",
			[]string{"superseded?", "worker", "desk"}},
		{"a label with no vocabulary entry", roleWorker, "raised-by:worker",
			[]string{"raised-by:worker", "no desklabel vocabulary entry", "worker"}},
		{"a size-family label some other verb provisions", roleReviewer, "size:xl",
			[]string{"size:xl", "no desklabel vocabulary entry", "reviewer"}},
	}
	for _, tc := range cases {
		for _, verb := range []string{"add", "rm"} {
			t.Run(tc.name+"/"+verb, func(t *testing.T) {
				fg := &fakeForge{issue: issueWith()}
				plantWorld(t, tc.role, fg)
				code, out := runVerb(t, verb, allowedRepo, "12", tc.label)
				if code != deskkit.ExitRefused {
					t.Fatalf("want exit 5 (refused), got %d: %s", code, out)
				}
				for _, w := range tc.wantWords {
					if !strings.Contains(out, w) {
						t.Errorf("refusal must name %q; got %s", w, out)
					}
				}
				if fg.calls() != 0 {
					t.Errorf("the ownership check must refuse BEFORE any forge call; saw reads=%v writes=%d",
						fg.reads, len(fg.writes))
				}
			})
		}
	}
}

// TestDesklabelRefusesHumanDecidedForEveryRole is Verify row 7: EVERY role the loop → role
// map carries (plus the two the table names) is refused on `human-decided`, add and rm,
// with zero forge calls — the label records a human act and no role may self-apply it.
func TestDesklabelRefusesHumanDecidedForEveryRole(t *testing.T) {
	roles := map[string]bool{roleWorker: true, roleReviewer: true}
	for _, r := range deskkit.LoopTokenRoles() {
		roles[r] = true
	}
	var names []string
	for r := range roles {
		names = append(names, r)
	}
	sort.Strings(names)
	if len(names) < 3 {
		t.Fatalf("the loop → role map yielded only %v — the enumeration is not finding the roster's roles", names)
	}
	for _, role := range names {
		for _, verb := range []string{"add", "rm"} {
			for _, spelling := range []string{"human-decided", "Human-Decided"} {
				t.Run(role+"/"+verb+"/"+spelling, func(t *testing.T) {
					fg := &fakeForge{issue: issueWith("human-decided")}
					plantWorld(t, role, fg)
					code, out := runVerb(t, verb, allowedRepo, "12", spelling)
					if code != deskkit.ExitRefused {
						t.Fatalf("role %s must be refused on human-decided; got exit %d: %s", role, code, out)
					}
					if !strings.Contains(out, "no role") {
						t.Errorf("the refusal must say the label belongs to NO role; got %s", out)
					}
					if fg.calls() != 0 {
						t.Errorf("zero forge calls expected; saw reads=%v writes=%d", fg.reads, len(fg.writes))
					}
				})
			}
		}
	}
}

// TestDesklabelRefusesWhenRoleUnresolved: no session role → refused (exit 5) before any
// forge call. There is no worker default.
func TestDesklabelRefusesWhenRoleUnresolved(t *testing.T) {
	fg := &fakeForge{issue: issueWith()}
	plantWorld(t, roleWorker, fg)
	roleFn = func() (string, error) {
		return "", deskkit.Refused("refused: desklabel could not resolve which App role this session acts under")
	}
	code, out := runVerb(t, "add", allowedRepo, "12", "question")
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d: %s", code, out)
	}
	if fg.calls() != 0 {
		t.Errorf("an unresolved role must make no forge call; saw reads=%v writes=%d", fg.reads, len(fg.writes))
	}
}

// TestDesklabelProductionRoleReadIsTheSession pins sessionRole to DESK_LOOP with no
// default: unset → refused; a loop with no App role → refused; a bound loop → its role.
func TestDesklabelProductionRoleReadIsTheSession(t *testing.T) {
	plantWorld(t, roleWorker, &fakeForge{issue: issueWith()})
	t.Setenv("DESK_LOOP", "")
	if _, err := sessionRole(); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Errorf("DESK_LOOP unset must refuse (5), got %v", err)
	}
	t.Setenv("DESK_LOOP", "worker-desk")
	got, err := sessionRole()
	if err != nil || got != roleWorker {
		t.Errorf("worker-desk must resolve to the worker role, got %q %v", got, err)
	}
	t.Setenv("DESK_LOOP", "pr-review-desk")
	got, err = sessionRole()
	if err != nil || got != roleReviewer {
		t.Errorf("pr-review-desk must resolve to the reviewer role, got %q %v", got, err)
	}
}

func TestDesklabelRefusesRepoOutsideSet(t *testing.T) {
	fg := &fakeForge{issue: issueWith()}
	plantWorld(t, roleWorker, fg)
	code, out := runVerb(t, "add", "someone-else/private-thing", "12", "question")
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d: %s", code, out)
	}
	if fg.calls() != 0 {
		t.Errorf("zero forge calls expected; saw %v", fg.reads)
	}
}

// --- Positive paths ------------------------------------------------------------------------

// TestDesklabelSharedVocabularyAnyRole is Verify row 8: the escalation trio succeeds under
// BOTH a worker-role and a reviewer-role session — no role check fires on the shared set.
func TestDesklabelSharedVocabularyAnyRole(t *testing.T) {
	shared := ownedBy(ownerShared)
	if len(shared) < 3 {
		t.Fatalf("the shared set has %d rows (%v) — fewer than the escalation vocabulary the brief names; the loop below would prove little", len(shared), shared)
	}
	for _, role := range []string{roleWorker, roleReviewer} {
		for _, label := range shared {
			t.Run(role+"/add/"+label, func(t *testing.T) {
				fg := &fakeForge{issue: issueWith()}
				plantWorld(t, role, fg)
				code, out := runVerb(t, "add", allowedRepo, "12", label)
				if code != deskkit.ExitOK {
					t.Fatalf("want exit 0, got %d: %s", code, out)
				}
				if len(fg.writes) != 1 || len(fg.writes[0].Add) != 1 || fg.writes[0].Add[0].Name != label {
					t.Fatalf("want ONE ApplyLabels adding %q, got %+v", label, fg.writes)
				}
				if fg.writes[0].Target != deskkit.TargetIssue {
					t.Errorf("a plain issue must be labelled with Target=issue, got %q", fg.writes[0].Target)
				}
				if !strings.Contains(out, "added: "+label) {
					t.Errorf("want an added: line, got %s", out)
				}
			})
			t.Run(role+"/rm/"+label, func(t *testing.T) {
				fg := &fakeForge{issue: issueWith(label)}
				plantWorld(t, role, fg)
				code, out := runVerb(t, "rm", allowedRepo, "12", label)
				if code != deskkit.ExitOK {
					t.Fatalf("want exit 0, got %d: %s", code, out)
				}
				if len(fg.writes) != 1 || len(fg.writes[0].Remove) != 1 || fg.writes[0].Remove[0] != label {
					t.Fatalf("want ONE ApplyLabels removing %q, got %+v", label, fg.writes)
				}
			})
		}
	}
}

// TestDesklabelNoopWhenStateAlreadyHolds: adding a present label / removing an absent one
// is a no-op — exit 0, "noop", and NO write (the budget is not charged for a state that
// already holds). The read is case-insensitive, as forge labels are.
func TestDesklabelNoopWhenStateAlreadyHolds(t *testing.T) {
	fg := &fakeForge{issue: issueWith("Needs-Decision", "keep-me")}
	plantWorld(t, roleWorker, fg)
	code, out := runVerb(t, "add", allowedRepo, "12", "needs-decision")
	if code != deskkit.ExitOK || !strings.Contains(out, "noop:") {
		t.Fatalf("present label must be a no-op: exit %d, %s", code, out)
	}
	code, out = runVerb(t, "rm", allowedRepo, "12", "question")
	if code != deskkit.ExitOK || !strings.Contains(out, "noop:") {
		t.Fatalf("absent removal must be a no-op: exit %d, %s", code, out)
	}
	if len(fg.writes) != 0 {
		t.Errorf("a no-op must make no write; got %+v", fg.writes)
	}
	if len(fg.reads) != 2 {
		t.Errorf("each no-op is ONE read; got %v", fg.reads)
	}
}

// TestDesklabelDryRunWritesNothing: --dry-run passes the ownership check, reads the target's
// kind, prints what would happen, and makes NO write.
func TestDesklabelDryRunWritesNothing(t *testing.T) {
	fg := &fakeForge{issue: &deskkit.Issue{State: "open", IsPullRequest: true, Labels: []string{"superseded?"}}}
	plantWorld(t, roleWorker, fg)
	code, out := runVerb(t, "rm", allowedRepo, "12", "superseded?", "--dry-run")
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d: %s", code, out)
	}
	if !strings.Contains(out, "dry-run: would remove label \"superseded?\"") || !strings.Contains(out, "(change)") {
		t.Errorf("dry-run must print the would-be write with the resolved kind; got %s", out)
	}
	if len(fg.writes) != 0 {
		t.Errorf("dry-run must not write; got %+v", fg.writes)
	}
	if len(fg.reads) != 1 {
		t.Errorf("dry-run reads the target once; got %v", fg.reads)
	}
	// And the ownership check still bites under --dry-run: a refusal is not a rehearsal.
	fg2 := &fakeForge{issue: issueWith()}
	plantWorld(t, roleWorker, fg2)
	code, _ = runVerb(t, "add", allowedRepo, "12", "approval-needed", "--dry-run")
	if code != deskkit.ExitRefused || fg2.calls() != 0 {
		t.Errorf("--dry-run must not bypass the ownership check: exit %d, calls %d", code, fg2.calls())
	}
}

// TestDesklabelWritesCanonicalCase: matched case-insensitively, written canonically.
func TestDesklabelWritesCanonicalCase(t *testing.T) {
	fg := &fakeForge{issue: &deskkit.Issue{State: "open", IsPullRequest: true}}
	plantWorld(t, roleReviewer, fg)
	code, out := runVerb(t, "add", allowedRepo, "12", "Approval-Needed")
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d: %s", code, out)
	}
	if len(fg.writes) != 1 || fg.writes[0].Add[0].Name != "approval-needed" {
		t.Fatalf("want the CANONICAL spelling written, got %+v", fg.writes)
	}
	if fg.writes[0].Target != deskkit.TargetChange {
		t.Errorf("a pull request must be labelled with Target=change, got %q", fg.writes[0].Target)
	}
}

// TestDesklabelKindFlagRoutesTypedRead: --kind takes the seam's typed read (op 38) and the
// stated kind is the write's target; without it the untyped read (op 2) resolves the kind.
func TestDesklabelKindFlagRoutesTypedRead(t *testing.T) {
	fg := &fakeForge{issue: &deskkit.Issue{State: "open", IsPullRequest: true}}
	plantWorld(t, roleReviewer, fg)
	if code, out := runVerb(t, "add", allowedRepo, "12", "authorization-needed", "--kind", "mr"); code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d: %s", code, out)
	}
	if len(fg.reads) != 1 || fg.reads[0] != "GetIssueTyped:change" {
		t.Errorf("--kind mr must route through GetIssueTyped(change); got %v", fg.reads)
	}
	if len(fg.writes) != 1 || fg.writes[0].Target != deskkit.TargetChange {
		t.Errorf("the stated kind must be the write's target; got %+v", fg.writes)
	}
	if code, out := runVerb(t, "add", allowedRepo, "12", "authorization-needed", "--kind", "ticket"); code != deskkit.ExitRefused {
		t.Errorf("an unknown --kind must be refused, got %d: %s", code, out)
	} else if !strings.Contains(out, "issue, mr") {
		t.Errorf("the refusal must name the accepted kinds; got %s", out)
	}
}

// TestDesklabelReadFailureIsCouldNotCheck: a target that cannot be read is never labelled.
func TestDesklabelReadFailureIsCouldNotCheck(t *testing.T) {
	fg := &fakeForge{issueErr: errors.New("boom: 502")}
	plantWorld(t, roleWorker, fg)
	code, out := runVerb(t, "add", allowedRepo, "12", "question")
	if code != deskkit.ExitUnverifiable || !strings.Contains(out, "could-not-check") {
		t.Fatalf("want exit 6 could-not-check, got %d: %s", code, out)
	}
	if len(fg.writes) != 0 {
		t.Errorf("no write on an unread target; got %+v", fg.writes)
	}
}

func TestDesklabelParseArgs(t *testing.T) {
	req, err := parseArgs(opAdd, []string{"--dry-run", allowedRepo, "#42", "question", "--kind", "issue"})
	if err != nil || !req.dryRun || req.number != 42 || req.kind != deskkit.TargetIssue || req.label != "question" {
		t.Errorf("interleaved flags must parse: %+v %v", req, err)
	}
	for _, bad := range [][]string{
		{allowedRepo, "12"},
		{allowedRepo, "twelve", "question"},
		{"not-a-slug", "12", "question"},
		{allowedRepo, "0", "question"},
		{allowedRepo, "12", "  "},
	} {
		if _, err := parseArgs(opAdd, bad); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Errorf("%v must be refused, got %v", bad, err)
		}
	}
}

// --- GitHub, end to end through the real backend (Verify row 5) --------------------------

type wireCall struct{ method, path, body string }

func record(calls *[]wireCall, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	*calls = append(*calls, wireCall{r.Method, r.URL.Path, string(b)})
}

func ghIssueJSON(number int, isPR bool, labels ...string) string {
	m := map[string]any{
		"number": number, "state": "open", "title": "t", "html_url": "https://example.invalid/x",
		"user": map[string]any{"login": "someone", "id": 7},
	}
	ls := []map[string]any{}
	for _, l := range labels {
		ls = append(ls, map[string]any{"name": l})
	}
	m["labels"] = ls
	if isPR {
		m["pull_request"] = map[string]any{"url": "https://example.invalid/pr"}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// TestDesklabelAppliesOwnedLabelGitHub: session role → vocabulary check → GetIssue kind
// resolution → ApplyLabels on GitHub's ONE labels endpoint. A worker applies its own
// `superseded?` to a pull request; a reviewer clears its own `authorization-needed`.
func TestDesklabelAppliesOwnedLabelGitHub(t *testing.T) {
	var calls []wireCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(&calls, r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/issues/12"):
			io.WriteString(w, ghIssueJSON(12, true, "authorization-needed"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/labels") && !strings.Contains(r.URL.Path, "/issues/"):
			w.WriteHeader(http.StatusUnprocessableEntity) // label already exists on the repo
			io.WriteString(w, `{"message":"Validation Failed"}`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issues/12/labels"):
			io.WriteString(w, `[{"name":"superseded?"}]`)
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/issues/12/labels/authorization-needed"):
			io.WriteString(w, `[]`)
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()
	gh := &deskkit.GitHubForge{Token: "test-injected-token-0000", BaseURL: srv.URL, Client: srv.Client()}

	plantWorld(t, roleWorker, gh)
	code, out := runVerb(t, "add", allowedRepo, "12", "superseded?")
	if code != deskkit.ExitOK {
		t.Fatalf("worker add superseded?: want exit 0, got %d: %s", code, out)
	}
	if !sawCall(calls, http.MethodPost, "/issues/12/labels", `"superseded?"`) {
		t.Errorf("want POST /issues/12/labels carrying superseded?; got %+v", calls)
	}

	calls = nil
	plantWorld(t, roleReviewer, gh)
	code, out = runVerb(t, "rm", allowedRepo, "12", "authorization-needed")
	if code != deskkit.ExitOK {
		t.Fatalf("reviewer rm authorization-needed: want exit 0, got %d: %s", code, out)
	}
	if !sawCall(calls, http.MethodDelete, "/issues/12/labels/authorization-needed", "") {
		t.Errorf("want DELETE /issues/12/labels/authorization-needed; got %+v", calls)
	}
	for _, c := range calls {
		if c.method == http.MethodPost {
			t.Errorf("rm must add nothing; saw %+v", c)
		}
	}
}

func sawCall(calls []wireCall, method, pathSuffix, bodyPart string) bool {
	for _, c := range calls {
		if c.method == method && strings.HasSuffix(c.path, pathSuffix) && strings.Contains(c.body, bodyPart) {
			return true
		}
	}
	return false
}

// --- GitLab, end to end through the real backend (Verify row 5) --------------------------

func glObjectJSON(iid int, labels ...string) string {
	m := map[string]any{
		"id": 1000 + iid, "iid": iid, "state": "opened", "title": "t", "web_url": "https://example.invalid/x",
		"author": map[string]any{"id": 9, "username": "someone"}, "labels": labels,
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// TestDesklabelAppliesOwnedLabelGitLab: GitLab numbers issues and merge requests in SEPARATE
// sequences, so the kind resolution decides WHICH endpoint the label write reaches.
//
//   - #7 exists only as an ISSUE   → PUT …/issues/7   (add_labels), never …/merge_requests/7
//   - !9 exists only as an MR      → PUT …/merge_requests/9 (remove_labels), never …/issues/9
//   - 5 exists as BOTH             → refused could-not-check (exit 6), NO write; `--kind mr`
//     routes the typed read (op 38) at the merge request alone and the write follows it.
func TestDesklabelAppliesOwnedLabelGitLab(t *testing.T) {
	var calls []wireCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(&calls, r)
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(p, "/labels"):
			w.WriteHeader(http.StatusConflict) // ensure: already exists
			io.WriteString(w, `{"message":"Label already exists"}`)
		// #7: issue only
		case r.Method == http.MethodGet && strings.HasSuffix(p, "/issues/7"):
			io.WriteString(w, glObjectJSON(7, "keep-me"))
		case r.Method == http.MethodPut && strings.HasSuffix(p, "/issues/7"):
			io.WriteString(w, glObjectJSON(7, "keep-me", "disposition:needs-rebase"))
		// !9: merge request only
		case r.Method == http.MethodGet && strings.HasSuffix(p, "/merge_requests/9"):
			io.WriteString(w, glObjectJSON(9, "approval-needed"))
		case r.Method == http.MethodPut && strings.HasSuffix(p, "/merge_requests/9"):
			io.WriteString(w, glObjectJSON(9))
		// 5: both
		case r.Method == http.MethodGet && strings.HasSuffix(p, "/issues/5"):
			io.WriteString(w, glObjectJSON(5))
		case r.Method == http.MethodGet && strings.HasSuffix(p, "/merge_requests/5"):
			io.WriteString(w, glObjectJSON(5))
		case r.Method == http.MethodPut && strings.HasSuffix(p, "/merge_requests/5"):
			io.WriteString(w, glObjectJSON(5, "question"))
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"message":"404 Not found"}`)
		default:
			http.Error(w, "unexpected "+r.Method+" "+p, http.StatusNotFound)
		}
	}))
	defer srv.Close()
	gl := &deskkit.GitLabForge{Token: "test-injected-token-0000", BaseURL: srv.URL, Client: srv.Client()}

	t.Run("issue only → the issues endpoint", func(t *testing.T) {
		calls = nil
		plantWorld(t, roleWorker, gl)
		code, out := runVerb(t, "add", allowedRepo, "7", "disposition:needs-rebase")
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d: %s", code, out)
		}
		if !sawCall(calls, http.MethodPut, "/issues/7", "disposition:needs-rebase") {
			t.Errorf("want PUT …/issues/7 carrying the label; got %+v", calls)
		}
		if sawCall(calls, http.MethodPut, "/merge_requests/7", "") {
			t.Errorf("an ISSUE must never be written through the merge-request endpoint; got %+v", calls)
		}
		if !strings.Contains(out, "(issue)") {
			t.Errorf("the report names the resolved kind; got %s", out)
		}
	})

	t.Run("merge request only → the merge_requests endpoint", func(t *testing.T) {
		calls = nil
		plantWorld(t, roleReviewer, gl)
		code, out := runVerb(t, "rm", allowedRepo, "9", "approval-needed")
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d: %s", code, out)
		}
		if !sawCall(calls, http.MethodPut, "/merge_requests/9", "approval-needed") {
			t.Errorf("want PUT …/merge_requests/9 removing the label; got %+v", calls)
		}
		if sawCall(calls, http.MethodPut, "/issues/9", "") {
			t.Errorf("a MERGE REQUEST must never be written through the issues endpoint; got %+v", calls)
		}
	})

	t.Run("both resolve → refused without --kind, routed with it", func(t *testing.T) {
		calls = nil
		plantWorld(t, roleWorker, gl)
		code, out := runVerb(t, "add", allowedRepo, "5", "question")
		if code != deskkit.ExitUnverifiable || !strings.Contains(out, "BOTH") {
			t.Fatalf("a both-resolve must be could-not-check (6) naming the ambiguity; got %d: %s", code, out)
		}
		for _, c := range calls {
			if c.method == http.MethodPut {
				t.Errorf("no write on an ambiguous number; saw %+v", c)
			}
		}
		calls = nil
		code, out = runVerb(t, "add", allowedRepo, "5", "question", "--kind", "mr")
		if code != deskkit.ExitOK {
			t.Fatalf("--kind mr must resolve the ambiguity; got %d: %s", code, out)
		}
		if !sawCall(calls, http.MethodPut, "/merge_requests/5", "question") {
			t.Errorf("want PUT …/merge_requests/5; got %+v", calls)
		}
		if sawCall(calls, http.MethodGet, "/issues/5", "") {
			t.Errorf("--kind mr must not probe the issue sequence at all (op 38 reads ONE kind); got %+v", calls)
		}
	})
}

// --- Verify row 9 as code: no role-override flag exists ------------------------------------

func TestDesklabelNoRoleOverrideFlag(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(".", n))
		if err != nil {
			t.Fatal(err)
		}
		seen++
		for _, needle := range []string{`flag.String("as"`, `"--as"`, `fs.String("as"`, `fs.String("role"`, `Getenv("DESKLABEL_ROLE")`} {
			if strings.Contains(string(b), needle) {
				t.Errorf("%s carries %s — the acting role is read from the session only, never from a flag or a private env var", n, needle)
			}
		}
	}
	if seen < 4 {
		t.Fatalf("only %d source files scanned — the enumeration is not finding the package", seen)
	}
}

// TestDesklabelVocabularyIsClosed pins the table's shape: the desklabel-owned entries the
// brief names, each with an owner the roster's role map recognises (or a sentinel), plus
// the shared rows the topology loader's decision-owed set declares (and `help wanted`),
// and human-decided owned by nobody. The shared expectation is READ from the loader, not
// restated, so this test cannot itself become the hand table the drift registry forbids.
func TestDesklabelVocabularyIsClosed(t *testing.T) {
	want := map[string]string{
		helpWantedLabel: ownerShared,
		"superseded?":   roleWorker, "disposition:superseded": roleWorker,
		"disposition:resolved-elsewhere": roleWorker, "disposition:needs-rebase": roleWorker,
		"authorization-needed": roleReviewer, "approval-needed": roleReviewer,
		"human-decided": ownerNone,
	}
	decisionOwed := topology.Compiled().DecisionOwedLabelNames()
	if len(decisionOwed) == 0 {
		t.Fatal("COULD-NOT-CHECK: the topology loader serves an EMPTY decision-owed set — the shared rows cannot be derived from nothing")
	}
	for _, name := range decisionOwed {
		want[name] = ownerShared
	}
	if len(vocabulary) != len(want) {
		t.Fatalf("the table has %d entries, want %d (%d loader-declared shared + help wanted + 7 desklabel-owned) — an entry was added or dropped",
			len(vocabulary), len(want), len(decisionOwed))
	}
	for _, e := range vocabulary {
		if want[e.Canonical] != e.Owner {
			t.Errorf("%q: owner %q, want %q", e.Canonical, e.Owner, want[e.Canonical])
		}
		if e.Why == "" {
			t.Errorf("%q: every entry cites the code that makes its owner the owner", e.Canonical)
		}
	}
	var buf bytes.Buffer
	printVocabulary(&buf)
	if !strings.Contains(buf.String(), "human-decided\tno role") {
		t.Errorf("the vocabulary print must show human-decided as no role's; got %s", buf.String())
	}
}

// TestDesklabelSharedRowsAreTheLoadersEscalationSet is the fix for the drift-registry
// finding: the shared rows are DERIVED from the topology loader's decision-owed set, not
// restated. Three assertions: (1) the live shared set equals the loader's decision-owed
// names plus `help wanted` — no more, no fewer; (2) `help wanted` is the ONLY shared row
// the loader does not declare (it is in no topology category, which is why it is
// desklabel's own row); (3) POSITIVE CONTROL — handing buildVocabulary a topology with a
// different decision-owed set moves the shared rows with it, so the equality in (1) is a
// derivation and not a coincidence of two literals.
func TestDesklabelSharedRowsAreTheLoadersEscalationSet(t *testing.T) {
	top := topology.Compiled()
	want := append(top.DecisionOwedLabelNames(), helpWantedLabel)
	sort.Strings(want)
	if got := ownedBy(ownerShared); !reflect.DeepEqual(got, want) {
		t.Fatalf("shared rows %v, want the loader's decision-owed set plus %q: %v", got, helpWantedLabel, want)
	}
	loaderSet := top.DecisionOwedLabelSet()
	for _, name := range ownedBy(ownerShared) {
		if !loaderSet[strings.ToLower(name)] && name != helpWantedLabel {
			t.Errorf("shared row %q is neither loader-declared nor the help-wanted row — a hand-added escalation label", name)
		}
	}
	// Positive control: a different declared set → different shared rows.
	bent := topology.Topology{DecisionOwedLabels: []topology.Label{{Name: "example-escalation-a"}, {Name: "example-escalation-b"}}}
	var shared []string
	for _, e := range buildVocabulary(bent) {
		if e.Owner == ownerShared {
			shared = append(shared, e.Canonical)
		}
	}
	sort.Strings(shared)
	if wantBent := []string{"example-escalation-a", "example-escalation-b", helpWantedLabel}; !reflect.DeepEqual(shared, wantBent) {
		t.Fatalf("POSITIVE CONTROL FAILED: buildVocabulary did not follow the topology it was given; shared rows %v, want %v", shared, wantBent)
	}
	// The desklabel-owned rows never move with topology.
	for _, e := range buildVocabulary(bent) {
		if e.Owner != ownerShared && loaderSet[strings.ToLower(e.Canonical)] {
			t.Errorf("desklabel-owned row %q is also a loader-declared label — it would be a second copy", e.Canonical)
		}
	}
}
