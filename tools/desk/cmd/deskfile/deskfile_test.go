package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeGHSource is compiled once (TestMain) into a temp dir placed FIRST on PATH, so the
// tool's `gh` invocations hit this canned stand-in instead of the network. It dispatches
// on the gh subcommand:
//   - `search issues` prints a JSON array from FAKEGH_SEARCH_HITS (default "[]"), prints
//     NOTHING when FAKEGH_SEARCH_EMPTY is set (exit 0, empty stdout — the "unanswered
//     search" shape), or exits 1 when FAKEGH_SEARCH_FAIL is set (the fail-closed path);
//   - `issue view` prints {state,url} from FAKEGH_ISSUE_STATE / FAKEGH_ISSUE_URL;
//   - `issue create` prints a fake issue URL, or exits 1 when FAKEGH_CREATE_FAIL is set
//     (the SENT-but-unconfirmable create, which must charge session budget);
//   - `issue comment` prints a fake comment URL.
//
// FAKEGH_STDERR_PAYLOAD, when set, is written to stderr on every FAILING path, so a test
// can drive attacker-shaped bytes through gh's own diagnostics into deskfile's error text.
//
// The in-process execCommand recorder (installed by withEnv) is the authoritative one used
// by assertions; this binary only makes gh behave canned.
const fakeGHSource = `package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]
	env := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	fail := func(msg string) {
		if p := os.Getenv("FAKEGH_STDERR_PAYLOAD"); p != "" {
			fmt.Fprintln(os.Stderr, p)
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
	switch {
	case len(args) >= 1 && args[0] == "search":
		if os.Getenv("FAKEGH_SEARCH_FAIL") != "" {
			fail("search: simulated API outage")
		}
		if os.Getenv("FAKEGH_SEARCH_EMPTY") != "" {
			os.Exit(0) // exit 0, no stdout at all
		}
		fmt.Println(env("FAKEGH_SEARCH_HITS", "[]"))
	case len(args) >= 2 && args[0] == "issue" && args[1] == "view":
		// Encoded with encoding/json, NOT %q: Go's %q emits \x1b for ESC, which is not a
		// valid JSON escape, so a crafted state/url would fail to parse and the test would
		// never deliver its payload to the code under test. json.Marshal emits .
		if err := json.NewEncoder(os.Stdout).Encode(map[string]string{
			"state": env("FAKEGH_ISSUE_STATE", "OPEN"),
			"url":   env("FAKEGH_ISSUE_URL", "https://github.com/medici-finance/assay/issues/42"),
		}); err != nil {
			os.Exit(1)
		}
	case len(args) >= 2 && args[0] == "issue" && args[1] == "create":
		if os.Getenv("FAKEGH_CREATE_FAIL") != "" {
			fail("issue create: simulated timeout after the request was sent")
		}
		fmt.Println("https://github.com/medici-finance/assay/issues/100")
	case len(args) >= 2 && args[0] == "issue" && args[1] == "comment":
		fmt.Println("https://github.com/medici-finance/assay/issues/42#issuecomment-7")
	default:
		fmt.Fprintf(os.Stderr, "fake gh: unknown args %v\n", args)
		os.Exit(1)
	}
}
`

var (
	fakeGHDir string
	origPATH  string
)

func TestMain(m *testing.M) {
	rosterCleanup, rerr := installFixtureRoster()
	if rerr != nil {
		panic("cannot install the test-fixture roster: " + rerr.Error())
	}
	defer rosterCleanup()
	origPATH = os.Getenv("PATH")
	dir, err := os.MkdirTemp("", "deskfile-fakegh")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fakegh\n\ngo 1.25\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(fakeGHSource), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(dir, "gh"), ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	if out, berr := build.CombinedOutput(); berr != nil {
		fmt.Fprintf(os.Stderr, "build fake gh: %v\n%s\n", berr, out)
		os.Exit(1)
	}
	fakeGHDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// --- fixtures ---------------------------------------------------------------------

const allowedRepo = "medici-finance/assay"

// withEnv points deskkit's runtime dir at a fresh HOME, prepends the fake gh to PATH,
// installs the in-process command recorder, and returns the recorded argv slice. Each
// test gets a clean audit log (fresh HOME) so budget/idempotency assertions are isolated.
func withEnv(t *testing.T) *[][]string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test")
	t.Setenv("PATH", fakeGHDir+string(os.PathListSeparator)+origPATH)
	// Clear every fake-gh switch so an ambient value in the developer's environment
	// cannot silently change what a test exercises.
	for _, k := range []string{
		"FAKEGH_SEARCH_HITS", "FAKEGH_SEARCH_FAIL", "FAKEGH_SEARCH_EMPTY",
		"FAKEGH_ISSUE_STATE", "FAKEGH_ISSUE_URL", "FAKEGH_CREATE_FAIL",
		"FAKEGH_STDERR_PAYLOAD",
	} {
		t.Setenv(k, "")
	}

	calls := &[][]string{}
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		return oldExec(name, args...)
	}
	t.Cleanup(func() { execCommand = oldExec })
	return calls
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func bodyFileWith(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "body.md")
	writeFile(t, p, body)
	return p
}

// searchHitsJSON builds a FAKEGH_SEARCH_HITS payload from titles (number assigned
// sequentially from 11). A title wrapped in [class] also gets the `class` label.
func searchHitsJSON(t *testing.T, titles ...string) string {
	t.Helper()
	type label struct {
		Name string `json:"name"`
	}
	type hit struct {
		Number int     `json:"number"`
		Title  string  `json:"title"`
		URL    string  `json:"url"`
		Labels []label `json:"labels"`
	}
	var hits []hit
	for i, tc := range titles {
		h := hit{
			Number: 11 + i,
			Title:  tc,
			URL:    fmt.Sprintf("https://github.com/%s/issues/%d", allowedRepo, 11+i),
		}
		if strings.HasPrefix(tc, "[class]") {
			h.Title = strings.TrimSpace(strings.TrimPrefix(tc, "[class]"))
			h.Labels = []label{{Name: "class"}}
		}
		hits = append(hits, h)
	}
	b, err := json.Marshal(hits)
	if err != nil {
		t.Fatalf("marshal hits: %v", err)
	}
	return string(b)
}

// seedNewAudit appends n deskfile `new` ok entries charged to (repo, session) dated now,
// so checkSessionBudget can be driven to its cap. HOME must already be set by withEnv.
func seedNewAudit(t *testing.T, n int, repo, session string) {
	t.Helper()
	seedNewAuditResult(t, n, repo, session, deskkit.ResultOK, "created")
}

// seedNewAuditResult is seedNewAudit with the result and detail chosen by the caller, so a
// test can seed the audit shapes the budget must and must not charge — in particular
// ResultUnverifiable with and without createSentMarker (the sent-vs-unsent discriminator).
func seedNewAuditResult(t *testing.T, n int, repo, session, result, detail string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir audit dir: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open audit: %v", err)
	}
	defer f.Close()
	for i := 0; i < n; i++ {
		num := 100 + i
		e := deskkit.Entry{
			TS:         time.Now().UTC().Format(time.RFC3339),
			Tool:       "deskfile",
			Verb:       "new",
			Repo:       repo,
			PR:         &num,
			Result:     result,
			Detail:     detail,
			SessionTag: session,
		}
		b, _ := json.Marshal(e)
		if _, werr := f.Write(append(b, '\n')); werr != nil {
			t.Fatalf("seed audit: %v", werr)
		}
	}
}

// readAudit decodes the test's audit.jsonl into a slice for content assertions.
func readAudit(t *testing.T) []deskkit.Entry {
	t.Helper()
	entries, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatalf("load audit: %v", err)
	}
	return entries
}

// --- argv assertions --------------------------------------------------------------

func ghCalls(calls [][]string) [][]string {
	var out [][]string
	for _, c := range calls {
		if len(c) > 0 && filepath.Base(c[0]) == "gh" {
			out = append(out, c)
		}
	}
	return out
}

func anyCall(calls [][]string, want ...string) bool {
	for _, c := range calls {
		all := true
		for _, w := range want {
			found := false
			for _, a := range c {
				if a == w {
					found = true
					break
				}
			}
			if !found {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// forbiddenGHVerbs are the mutating gh tokens deskfile must NEVER emit: it has no
// merge/close/reopen/edit/ready/transition capability. The ONLY mutating verbs allowed are
// `issue create` (new) and `issue comment` (attach).
var forbiddenGHVerbs = []string{
	"merge", "close", "reopen", "edit", "ready", "review", "approve",
	"--approve", "--request-changes", "transition", "delete", "archive",
}

func assertNoForbiddenGH(t *testing.T, calls [][]string) {
	t.Helper()
	for _, c := range ghCalls(calls) {
		for _, a := range c[1:] {
			for _, bad := range forbiddenGHVerbs {
				if a == bad {
					t.Fatalf("deskfile emitted a forbidden gh token %q: %v", bad, c)
				}
			}
		}
	}
}

func assertNoIssueCreate(t *testing.T, calls [][]string) {
	t.Helper()
	if anyCall(ghCalls(calls), "issue", "create") {
		t.Fatalf("an `gh issue create` was made on a path that must not write: %v", ghCalls(calls))
	}
}

func assertNoIssueComment(t *testing.T, calls [][]string) {
	t.Helper()
	if anyCall(ghCalls(calls), "issue", "comment") {
		t.Fatalf("an `gh issue comment` was made on a path that must not write: %v", ghCalls(calls))
	}
}

func assertNoMutatingGH(t *testing.T, calls [][]string) {
	t.Helper()
	assertNoIssueCreate(t, calls)
	assertNoIssueComment(t, calls)
}

// --- Verify 1: dedupe refusal -----------------------------------------------------

// TestDedupeRefusal proves the gate: a stubbed near-duplicate title → exit 5, the output
// names the candidate and the attach command, and NO create call was made (fail-closed
// against the expensive direction — minting a possible duplicate).
func TestDedupeRefusal(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "oracle price feed goes stale"))
	body := bodyFileWith(t, "the oracle price feed is going stale under load")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "oracle price feed goes stale", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("near-duplicate new rc = %d, want 5; out=%s", rc, out)
	}
	if !strings.Contains(out, "#11") {
		t.Fatalf("refusal must name the candidate (#11); out=%s", out)
	}
	if !strings.Contains(out, "deskfile attach") || !strings.Contains(out, "--to 11") {
		t.Fatalf("refusal must print the attach command for the top candidate; out=%s", out)
	}
	if !strings.Contains(out, "score") {
		t.Fatalf("refusal must print the match score; out=%s", out)
	}
}

func TestDedupeRefusalNoCreateCall(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "oracle price feed goes stale"))
	body := bodyFileWith(t, "stale oracle")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "oracle price feed goes stale", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want 5", rc)
	}
	assertNoIssueCreate(t, *calls)
	assertNoForbiddenGH(t, *calls)
}

// TestDedupeNoMatchPassesToCreate: an unrelated candidate set does not trip the gate, the
// issue is created, and the only mutating call is `issue create`.
func TestDedupeNoMatchPassesToCreate(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "totally unrelated other topic"))
	body := bodyFileWith(t, "a fresh unique observation")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "brand new unrelated filing", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("unique new rc = %d, want 0", rc)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create`; gh calls: %v", ghCalls(*calls))
	}
	assertNoForbiddenGH(t, *calls)
}

// TestClassLabelBoost: a class issue with only MODERATE token overlap (below the pure-
// Jaccard threshold) is lifted over by the class boost, redirecting the filer to attach —
// the exact motion the gate exists to force when the target is a class issue.
func TestClassLabelBoost(t *testing.T) {
	withEnv(t)
	// "stale oracle price" (query) vs "oracle price feed stale" (candidate): Jaccard of
	// {oracle, price, stale} vs {oracle, price, feed, stale} = 3/4 = 0.5 — exactly at the
	// threshold even without the boost, so add a divergent token to make the boost load-
	// bearing: query "stale oracle price window", candidate "[class] oracle price feed".
	// Jaccard {stale, oracle, price, window} vs {oracle, price, feed} = 2/5 = 0.4; +0.15
	// class boost = 0.55 >= 0.5 → match.
	t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "[class] oracle price feed"))
	body := bodyFileWith(t, "observation")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "stale oracle price window", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("class-boost rc = %d, want 5; out=%s", rc, out)
	}
	if !strings.Contains(out, "class issue") {
		t.Fatalf("refusal must mark the candidate as a class issue; out=%s", out)
	}
}

// --- Verify 2: per-session budget -------------------------------------------------

// TestBudgetFourthNewRefuses: 3 prior `new` in one sessionTag+repo 24h window → the 4th
// `new` exits 4 and makes NO create call.
func TestBudgetFourthNewRefuses(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]") // no dupes
	seedNewAudit(t, defaultNewBudgetPerSession, allowedRepo, "test")
	body := bodyFileWith(t, "one new too many")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "budget exhausted filing", "--body-file", body})
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("4th new rc = %d, want 4", rc)
	}
	assertNoIssueCreate(t, *calls)
}

// TestBudgetThirdNewOK: at the cap minus one, the next `new` is still admitted (cap is 3,
// so the 3rd succeeds; the 4th is refused — proven above).
func TestBudgetThirdNewOK(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	seedNewAudit(t, defaultNewBudgetPerSession-1, allowedRepo, "test")
	body := bodyFileWith(t, "within budget")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "third filing this session", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("3rd new rc = %d, want 0", rc)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create`; gh calls: %v", ghCalls(*calls))
	}
}

// TestBudgetAttachUnbudgeted: attach is never subject to THIS brief's per-session
// new-issue budget. A session that has spent its full `new` budget, and has already posted
// more attaches than the per-PR outward-write cap allows on ANOTHER issue, can still
// attach — directing filers to attach is the motion the gate exists to force, so it must
// never be the path the new-issue budget refuses.
//
// Precisely what this does NOT claim: attach is still subject to deskkit's ordinary
// outward-write meters (per-issue and per-repo). The seeded attaches are on a different
// issue (42) than the one under test (11) so the per-issue meter is not the thing being
// probed here — the per-session new-issue budget is.
func TestBudgetAttachUnbudgeted(t *testing.T) {
	calls := withEnv(t)
	// Many prior attaches on ANOTHER issue — and a full `new` budget — must NOT block this
	// attach.
	seedAttachAudit(t, deskkit.RateLimitPerPRPerHour+5, allowedRepo, "test")
	seedNewAudit(t, defaultNewBudgetPerSession, allowedRepo, "test")
	body := bodyFileWith(t, "instance of the class issue")

	rc, _ := runCapture([]string{"attach", "-R", allowedRepo, "--to", "11", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("attach rc = %d, want 0 (attach is unbudgeted)", rc)
	}
	if !anyCall(ghCalls(*calls), "issue", "comment") {
		t.Fatalf("expected an `gh issue comment`; gh calls: %v", ghCalls(*calls))
	}
}

// TestBudgetUnknownSessionBucket: when CLAUDE_SESSION_ID is unset, all such sessions share
// the single "unknown" bucket (deskkit.SessionTag() fallback). The conservative direction
// — they pool, they do not each get a fresh budget.
func TestBudgetUnknownSessionBucket(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("CLAUDE_SESSION_ID", "") // → SessionTag() returns "unknown"
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	seedNewAudit(t, defaultNewBudgetPerSession, allowedRepo, "unknown")
	body := bodyFileWith(t, "another unset-session filing")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "unset session filing", "--body-file", body})
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("unset-session 4th new rc = %d, want 4 (shared unknown bucket)", rc)
	}
	assertNoIssueCreate(t, *calls)
}

// TestBudgetSessionScoped: a full budget under session A does NOT block a `new` under a
// different session B — the budget is per-session, and the audit trace records each
// session's filings (the control on rotating the ID).
func TestBudgetSessionScoped(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	seedNewAudit(t, defaultNewBudgetPerSession, allowedRepo, "session-A")
	t.Setenv("CLAUDE_SESSION_ID", "session-B") // different bucket
	body := bodyFileWith(t, "different session")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "session B filing", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("different-session new rc = %d, want 0 (per-session budget)", rc)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create`; gh calls: %v", ghCalls(*calls))
	}
	// The audit line for this create records session-B, the tag it charged.
	entries := readAudit(t)
	var last *deskkit.Entry
	for i := range entries {
		if entries[i].Verb == "new" && entries[i].Result == deskkit.ResultOK {
			last = &entries[i]
		}
	}
	if last == nil {
		t.Fatal("no ok `new` audit line found")
	}
	if last.SessionTag != "session-B" {
		t.Fatalf("audit sessionTag = %q, want \"session-B\" (every new records the tag it charged)", last.SessionTag)
	}
}

// seedAttachAudit appends n deskfile `attach` ok entries (used to prove attach is not
// blocked by prior attach volume).
func seedAttachAudit(t *testing.T, n int, repo, session string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir audit dir: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open audit: %v", err)
	}
	defer f.Close()
	for i := 0; i < n; i++ {
		num := 42
		e := deskkit.Entry{
			TS:         time.Now().UTC().Format(time.RFC3339),
			Tool:       "deskfile",
			Verb:       "attach",
			Repo:       repo,
			PR:         &num,
			Result:     deskkit.ResultOK,
			SessionTag: session,
		}
		b, _ := json.Marshal(e)
		if _, werr := f.Write(append(b, '\n')); werr != nil {
			t.Fatalf("seed attach audit: %v", werr)
		}
	}
}

// --- Verify 3: fail-closed on search API failure + --force-new --------------------

// TestFailClosedSearchError: a search API error → `new` exits 6 with ZERO write calls.
// Minting a possibly-duplicate issue is the expensive direction.
func TestFailClosedSearchError(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_FAIL", "1")
	body := bodyFileWith(t, "filing during an outage")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "filing during api outage", "--body-file", body})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("search-fail new rc = %d, want 6", rc)
	}
	assertNoMutatingGH(t, *calls)
}

// TestFailClosedCheckSearchError: `check` also fails closed on a search error — it cannot
// promise "no duplicate" on an unanswered search.
func TestFailClosedCheckSearchError(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_FAIL", "1")

	rc, _ := runCapture([]string{"check", "-R", allowedRepo, "--title", "anything"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("search-fail check rc = %d, want 6", rc)
	}
	assertNoMutatingGH(t, *calls)
}

// TestFailClosedForceNewOverridesSearchError: --force-new --reason bypasses the dedupe
// search and proceeds despite the API outage, and the audit line records "force-new" + the
// reason.
//
// The name carries the `TestFailClosed` prefix deliberately: a Verify command that runs
// `-run 'TestFailClosed'` asserts BOTH halves — the fail-closed
// refusal AND that `--force-new --reason x` proceeds and is audited. A Verify command that
// does not select the test proving its own Expect proves nothing.
func TestFailClosedForceNewOverridesSearchError(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_FAIL", "1")
	body := bodyFileWith(t, "urgent filing during outage")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "urgent unique filing", "--body-file", body,
		"--force-new", "--reason", "search API down; the owner asked for this filed now"})
	if rc != deskkit.ExitOK {
		t.Fatalf("force-new rc = %d, want 0", rc)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create` under --force-new; gh calls: %v", ghCalls(*calls))
	}
	// No search call was made (--force-new skips it entirely).
	if anyCall(ghCalls(*calls), "search") {
		t.Fatalf("--force-new must skip the dedupe search; gh calls: %v", ghCalls(*calls))
	}
	// The audit line carries "force-new" and the reason.
	entries := readAudit(t)
	var last *deskkit.Entry
	for i := range entries {
		if entries[i].Verb == "new" && entries[i].Result == deskkit.ResultOK {
			last = &entries[i]
		}
	}
	if last == nil {
		t.Fatal("no ok `new` audit line found")
	}
	if !strings.Contains(last.Detail, "force-new") {
		t.Fatalf("audit detail must contain \"force-new\"; got %q", last.Detail)
	}
	if !strings.Contains(last.Detail, "search API down") {
		t.Fatalf("audit detail must contain the reason; got %q", last.Detail)
	}
}

// TestFailClosedForceNewWithoutReasonRefuses: the escape hatch requires a stated reason —
// an unreasoned bypass of the dedupe gate is not an escape hatch, it is the gate removed.
// Prefixed `TestFailClosed` so Verify row 3's `-run` selects it with the rest of the
// fail-closed set.
func TestFailClosedForceNewWithoutReasonRefuses(t *testing.T) {
	calls := withEnv(t)
	body := bodyFileWith(t, "x")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "t", "--body-file", body, "--force-new"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("force-new without reason rc = %d, want 5", rc)
	}
	assertNoMutatingGH(t, *calls)
}

// --- refusal tests ------------------------------------------------------------

func TestRepoNotAllowedRefuses(t *testing.T) {
	calls := withEnv(t)
	body := bodyFileWith(t, "x")

	rc, _ := runCapture([]string{"new", "-R", "example/otherrepo",
		"--title", "t", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("disallowed repo new rc = %d, want 5", rc)
	}
	if len(ghCalls(*calls)) != 0 {
		t.Fatalf("a gh call was made for a disallowed repo: %v", ghCalls(*calls))
	}
}

func TestRepoNotAllowedRefusesAttach(t *testing.T) {
	calls := withEnv(t)
	body := bodyFileWith(t, "x")

	rc, _ := runCapture([]string{"attach", "-R", "example/otherrepo", "--to", "7", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("disallowed repo attach rc = %d, want 5", rc)
	}
	if len(ghCalls(*calls)) != 0 {
		t.Fatalf("a gh call was made for a disallowed repo: %v", ghCalls(*calls))
	}
}

func TestClosedTargetAttachRefuses(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_ISSUE_STATE", "CLOSED")
	body := bodyFileWith(t, "instance of a closed class issue")

	rc, out := runCapture([]string{"attach", "-R", allowedRepo, "--to", "11", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("attach to closed rc = %d, want 5", rc)
	}
	if !strings.Contains(out, "reopen") && !strings.Contains(out, "deskfile new") {
		t.Fatalf("closed-target refusal must give reopen-or-new guidance; out=%s", out)
	}
	assertNoIssueComment(t, *calls)
}

func TestKillSwitchDisabled(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	body := bodyFileWith(t, "should never file")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo, "--title", "t", "--body-file", body})
	if rc != deskkit.ExitDisabled {
		t.Fatalf("kill switch rc = %d, want 3", rc)
	}
	assertNoMutatingGH(t, *calls)
}

func TestOversizeBodyRefuses(t *testing.T) {
	calls := withEnv(t)
	body := bodyFileWith(t, strings.Repeat("a", maxBodyBytes+1))

	rc, _ := runCapture([]string{"new", "-R", allowedRepo, "--title", "t", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("oversize body rc = %d, want 5", rc)
	}
	assertNoMutatingGH(t, *calls)
}

func TestSecretInBodyRefuses(t *testing.T) {
	calls := withEnv(t)
	body := bodyFileWith(t, "token ghp_"+strings.Repeat("a", 36))

	rc, _ := runCapture([]string{"new", "-R", allowedRepo, "--title", "t", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("secret in body rc = %d, want 5", rc)
	}
	assertNoMutatingGH(t, *calls)
}

func TestMissingFlagsRefuse(t *testing.T) {
	withEnv(t)
	if rc, _ := runCapture([]string{"new", "-R", allowedRepo, "--title", "t"}); rc != deskkit.ExitRefused {
		t.Fatalf("missing --body-file rc = %d, want 5", rc)
	}
	if rc, _ := runCapture([]string{"new", "--title", "t", "--body-file", "/x"}); rc != deskkit.ExitRefused {
		t.Fatalf("missing -R rc = %d, want 5", rc)
	}
	if rc, _ := runCapture([]string{"attach", "-R", allowedRepo, "--body-file", "/x"}); rc != deskkit.ExitRefused {
		t.Fatalf("missing --to rc = %d, want 5", rc)
	}
}

// --- attach success + check dry-run ----------------------------------------------

func TestAttachSuccessOnlyMutatingCallIsComment(t *testing.T) {
	calls := withEnv(t)
	body := bodyFileWith(t, "observation as the worker")

	rc, _ := runCapture([]string{"attach", "-R", allowedRepo, "--to", "11", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("attach rc = %d, want 0", rc)
	}
	if !anyCall(ghCalls(*calls), "issue", "comment") {
		t.Fatalf("expected an `gh issue comment`; gh calls: %v", ghCalls(*calls))
	}
	assertNoForbiddenGH(t, *calls)
	if anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("attach must never call `issue create`; gh calls: %v", ghCalls(*calls))
	}
}

func TestCheckDryRunNoWrites(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "unrelated topic"))

	rc, _ := runCapture([]string{"check", "-R", allowedRepo, "--title", "a unique new title"})
	if rc != deskkit.ExitOK {
		t.Fatalf("check (no dup) rc = %d, want 0", rc)
	}
	assertNoMutatingGH(t, *calls)
}

func TestCheckDryRunFindsDuplicate(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "oracle price feed goes stale"))

	rc, out := runCapture([]string{"check", "-R", allowedRepo, "--title", "oracle price feed goes stale"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("check (dup) rc = %d, want 5", rc)
	}
	if !strings.Contains(out, "#11") || !strings.Contains(out, "deskfile attach") {
		t.Fatalf("check must name the candidate + attach command; out=%s", out)
	}
	assertNoMutatingGH(t, *calls)
}

// --- no new repo list (review concern) --------------------------------------------

// TestNoHardcodedRepoSet proves deskfile introduces NO new repo list: no string literal
// in its non-test source names an allowed-repo TARGET (owner/repo). Repo scope comes from
// deskkit.IsAllowedRepo (the single compiled-in set). This is the mechanical proof
// the review asks for.
//
// Import declarations are excluded: the module path
// "github.com/medici-finance/assay/tools/desk/..." legitimately contains the org prefix,
// and a real embedded repo list would live in a const/var map or a function body — never
// in an import. So only non-import declarations are walked.
func TestNoHardcodedRepoSet(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob sources: %v", err)
	}
	// The exact allowed-repo name strings a hand-rolled list would embed. Org prefixes
	// alone are too loose (they appear in the import path); the full owner/repo names do not.
	//
	// DERIVED from deskkit.AllowedRepos(), never hand-copied: a hand-maintained duplicate
	// drifts silently the moment a repo is added to config.go (it already had, missing the
	// three `*-slides` entries), and a check that has stopped covering the new entries reads
	// as passing while proving less than it claims.
	forbiddenRepoLiterals := deskkit.AllowedRepos()
	if len(forbiddenRepoLiterals) == 0 {
		t.Fatal("deskkit.AllowedRepos() is empty — the no-repo-list check would prove nothing")
	}
	scanned := 0
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue // tests legitimately name allowed repos as fixtures
		}
		scanned++
		af, perr := parser.ParseFile(fset, f, nil, 0) // no ParseComments: comments are not walked
		if perr != nil {
			t.Fatalf("parse %s: %v", f, perr)
		}
		for _, decl := range af.Decls {
			if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
				continue // the module import path legitimately carries the org prefix
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				for _, bad := range forbiddenRepoLiterals {
					if strings.Contains(lit.Value, bad) {
						t.Fatalf("%s embeds repo literal %q — deskfile must take scope from deskkit.IsAllowedRepo, not a new list", f, lit.Value)
					}
				}
				return true
			})
		}
	}
	if scanned == 0 {
		t.Fatal("scanned 0 source files — the no-repo-list check proved nothing")
	}
}

// --- review findings 4/5/6/2/7/9 (PR #484, assay-reviewer-app) --------------------
//
// Every test in this block was written against a REPRODUCED failure: each one goes red
// against the code as it stood before the fix and green only with the corresponding fix.

// lastAudit returns the most recent audit entry, or fails.
func lastAudit(t *testing.T) deskkit.Entry {
	t.Helper()
	entries := readAudit(t)
	if len(entries) == 0 {
		t.Fatal("no audit entries written — every invocation must emit exactly one line")
	}
	return entries[len(entries)-1]
}

// auditEntriesFor returns every audit entry for a verb.
func auditEntriesFor(t *testing.T, verb string) []deskkit.Entry {
	t.Helper()
	var out []deskkit.Entry
	for _, e := range readAudit(t) {
		if e.Verb == verb {
			out = append(out, e)
		}
	}
	return out
}

// --- Finding 4: a pre-write failure must not consume the budget the escape hatch needs

// TestFailClosedOutageKeepsHatchUsable is the reviewer's exact reproduction:
// three `new` attempts during a dedupe-search outage, then the DOCUMENTED escape hatch.
//
// Each outage attempt exits 6 having sent nothing — zero `gh issue create` calls — so it
// must not charge the per-session budget. Before the fix the three exit-6 lines charged as
// ResultUnverifiable and the fourth attempt, `--force-new --reason`, was refused exit 4
// "3 `new` ... in the last 24h" with zero issues filed: the escape hatch was unreachable
// during the exact outage it exists for.
//
// The name carries the `TestFailClosed` prefix on purpose — Verify row 3 selects on it and
// this is a fail-closed-behaviour test (the outage refusals themselves are asserted here).
func TestFailClosedOutageKeepsHatchUsable(t *testing.T) {
	calls := withEnv(t)
	body := bodyFileWith(t, "urgent filing during an outage")

	t.Setenv("FAKEGH_SEARCH_FAIL", "1")
	for i := 1; i <= defaultNewBudgetPerSession; i++ {
		rc, _ := runCapture([]string{"new", "-R", allowedRepo,
			"--title", fmt.Sprintf("urgent thing number %d", i), "--body-file", body})
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("outage attempt %d rc = %d, want 6", i, rc)
		}
	}
	// Nothing was sent: the whole premise of not charging these.
	assertNoMutatingGH(t, *calls)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "urgent unique filing", "--body-file", body,
		"--force-new", "--reason", "search API is down, urgent filing"})
	if rc != deskkit.ExitOK {
		t.Fatalf("escape hatch after %d outage refusals rc = %d, want 0 — the outage consumed "+
			"the budget the hatch needs; out=%s", defaultNewBudgetPerSession, rc, out)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("escape hatch made no `gh issue create`; gh calls: %v", ghCalls(*calls))
	}
}

// TestBudgetUnsentCreateNoCharge pins the discriminator directly:
// seeded pre-write Unverifiable lines (no createSentMarker) leave the budget untouched.
func TestBudgetUnsentCreateNoCharge(t *testing.T) {
	calls := withEnv(t)
	seedNewAuditResult(t, defaultNewBudgetPerSession, allowedRepo, "test",
		deskkit.ResultUnverifiable, "dedupe search failed — refuse rather than mint a possible duplicate")
	body := bodyFileWith(t, "a filing after three outages")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a genuinely new subject", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 — pre-write unverifiable lines must not charge budget; out=%s", rc, out)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create`; gh calls: %v", ghCalls(*calls))
	}
}

// TestBudgetSentCreateCharges is the fail-closed half, and the reason
// the fix is a discriminator rather than "stop charging Unverifiable": a create that WAS
// sent and could not be confirmed may have minted an issue, so it must still charge. Drop
// the marker check from chargedNewEntry and this test goes red.
func TestBudgetSentCreateCharges(t *testing.T) {
	calls := withEnv(t)
	seedNewAuditResult(t, defaultNewBudgetPerSession, allowedRepo, "test",
		deskkit.ResultUnverifiable, createSentMarker+"gh issue create failed: timeout")
	body := bodyFileWith(t, "a fourth filing")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a genuinely new subject", "--body-file", body})
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("rc = %d, want 4 — a SENT-but-unconfirmable create must charge budget; out=%s", rc, out)
	}
	assertNoMutatingGH(t, *calls)
}

// TestNewStampsSentMarkerOnFailedCreate proves the marker is actually written
// by the real code path (not only by the seeder): a `gh issue create` that fails AFTER
// being invoked produces exit 6 with a marked audit line.
func TestNewStampsSentMarkerOnFailedCreate(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_CREATE_FAIL", "1")
	body := bodyFileWith(t, "a filing whose create times out")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a filing whose create times out", "--body-file", body})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("failed-create rc = %d, want 6", rc)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected the create to have been ATTEMPTED; gh calls: %v", ghCalls(*calls))
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultUnverifiable {
		t.Fatalf("audit result = %q, want %q", e.Result, deskkit.ResultUnverifiable)
	}
	if !strings.HasPrefix(e.Detail, createSentMarker) {
		t.Fatalf("audit detail = %q, want the %q prefix — the budget cannot tell a sent create "+
			"from a pre-write failure without it", e.Detail, createSentMarker)
	}
	if !chargedNewEntry(e) {
		t.Fatal("a sent-but-unconfirmable create must charge session budget")
	}
}

// TestBudgetBodyFileFailureNoCharge: three mistyped --body-file paths sent
// nothing either, so they must not lock the session out for 24h.
func TestBudgetBodyFileFailureNoCharge(t *testing.T) {
	calls := withEnv(t)
	missing := filepath.Join(t.TempDir(), "does-not-exist.md")
	for i := 1; i <= defaultNewBudgetPerSession; i++ {
		rc, _ := runCapture([]string{"new", "-R", allowedRepo,
			"--title", fmt.Sprintf("typo attempt %d", i), "--body-file", missing})
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("missing body-file attempt %d rc = %d, want 6", i, rc)
		}
	}
	for _, e := range auditEntriesFor(t, "new") {
		if chargedNewEntry(e) {
			t.Fatalf("an unreadable --body-file charged budget: %+v", e)
		}
	}
	body := bodyFileWith(t, "the corrected filing")
	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "the corrected filing", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc after three typos = %d, want 0; out=%s", rc, out)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create`; gh calls: %v", ghCalls(*calls))
	}
}

// --- Finding 5: correct gate operation must not open a breaker on the gate's own remedy

// TestCheckRefusalsDoNotBlockAttach is the reviewer's second reproduction. `check` finding
// a duplicate IS the verb succeeding, but it exits 5, and deskkit's breaker counted
// consecutive `refused` per TOOL: five correct dedupe hits opened a 15-minute breaker
// against `attach` — the motion the refusal message had just told the caller to make.
//
// HONEST SCOPE. #454 landed on main after that review and scoped the primary
// breaker to the (repo, PR) bucket, which alone fixes the reviewer's five-hit repro:
// `check` audits with no issue number and `attach` targets one, so they no longer share a
// bucket. What #454 does NOT cover is checkBreakerBackstop, the TOOL-WIDE stop that trips
// at BreakerBackstopTrip consecutive non-progress lines across all targets — which is
// exactly the shape a `check` loop produces. This test therefore drives the BACKSTOP, so
// it stays load-bearing against merged main rather than passing on #454's coat-tails.
//
// `check` writes nothing on any path, so its audit lines are ResultDryRun, which both
// breaker walks ignore by design (#214).
func TestCheckRefusalsDoNotBlockAttach(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "deskfile breaker collateral damage"))

	for i := 1; i <= deskkit.BreakerBackstopTrip; i++ {
		rc, _ := runCapture([]string{"check", "-R", allowedRepo,
			"--title", "deskfile breaker collateral damage"})
		if rc != deskkit.ExitRefused {
			t.Fatalf("check #%d rc = %d, want 5 (the gate correctly finding the duplicate)", i, rc)
		}
	}
	assertNoMutatingGH(t, *calls)

	body := bodyFileWith(t, "the observation the refusal told me to attach")
	rc, out := runCapture([]string{"attach", "-R", allowedRepo, "--to", "11", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("attach after %d correct dedupe hits rc = %d, want 0 — the gate opened a breaker "+
			"on its own remedy; out=%s", deskkit.BreakerBackstopTrip, rc, out)
	}
	if !anyCall(ghCalls(*calls), "issue", "comment") {
		t.Fatalf("expected an `gh issue comment`; gh calls: %v", ghCalls(*calls))
	}
}

// TestCheckOutageChargesNoWriteBudget is the OTHER meter, and the half #454 does
// not touch. A `check` whose search failed logged ResultUnverifiable, which deskkit's
// chargesBudget counts — so a READ verb spent deskfile's outward-WRITE budget, and
// RateLimitPerPRPerHour unanswered searches locked `new` out of the repo for an hour with
// no write ever attempted. ResultDryRun charges nothing.
func TestCheckOutageChargesNoWriteBudget(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_FAIL", "1")
	for i := 1; i <= deskkit.RateLimitPerPRPerHour; i++ {
		rc, _ := runCapture([]string{"check", "-R", allowedRepo, "--title", "some subject"})
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("check #%d rc = %d, want 6", i, rc)
		}
	}
	assertNoMutatingGH(t, *calls)

	t.Setenv("FAKEGH_SEARCH_FAIL", "")
	body := bodyFileWith(t, "a filing after the search recovered")
	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a filing after the search recovered", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("new after %d failed READS rc = %d, want 0 — a read verb consumed the "+
			"outward-write budget; out=%s", deskkit.RateLimitPerPRPerHour, rc, out)
	}
	if !anyCall(ghCalls(*calls), "issue", "create") {
		t.Fatalf("expected an `gh issue create`; gh calls: %v", ghCalls(*calls))
	}
}

// TestCheckAuditsAsDryRunOnEveryOutcome pins the result class on all three of check's
// outcomes. ResultDryRun is invisible to BOTH deskkit meters, which is the point: a read
// verb must neither trip the breaker (finding 5) nor charge outward-WRITE budget (which a
// `check` whose search failed did, since chargesBudget counts ResultUnverifiable).
func TestCheckAuditsAsDryRunOnEveryOutcome(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		withEnv(t)
		t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "an unrelated topic"))
		if rc, _ := runCapture([]string{"check", "-R", allowedRepo, "--title", "a distinct subject"}); rc != deskkit.ExitOK {
			t.Fatalf("rc = %d, want 0", rc)
		}
		assertDryRun(t)
	})
	t.Run("duplicate", func(t *testing.T) {
		withEnv(t)
		t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "oracle price feed goes stale"))
		if rc, _ := runCapture([]string{"check", "-R", allowedRepo, "--title", "oracle price feed goes stale"}); rc != deskkit.ExitRefused {
			t.Fatalf("rc = %d, want 5", rc)
		}
		e := assertDryRun(t)
		if !strings.Contains(e.Detail, "likely duplicate") {
			t.Fatalf("dryrun line lost the refusal reason: detail=%q", e.Detail)
		}
	})
	t.Run("search-outage", func(t *testing.T) {
		withEnv(t)
		t.Setenv("FAKEGH_SEARCH_FAIL", "1")
		if rc, _ := runCapture([]string{"check", "-R", allowedRepo, "--title", "any subject"}); rc != deskkit.ExitUnverifiable {
			t.Fatalf("rc = %d, want 6", rc)
		}
		assertDryRun(t)
	})
	t.Run("bad-flags", func(t *testing.T) {
		withEnv(t)
		if rc, _ := runCapture([]string{"check", "-R", allowedRepo}); rc != deskkit.ExitRefused {
			t.Fatalf("rc = %d, want 5", rc)
		}
		assertDryRun(t)
	})
}

func assertDryRun(t *testing.T) deskkit.Entry {
	t.Helper()
	e := lastAudit(t)
	if e.Verb != "check" {
		t.Fatalf("last audit verb = %q, want \"check\"", e.Verb)
	}
	if e.Result != deskkit.ResultDryRun {
		t.Fatalf("check audit result = %q, want %q — a verb that writes nothing must feed "+
			"neither deskkit meter", e.Result, deskkit.ResultDryRun)
	}
	return e
}

// TestCheckNeverCallsAMutatingVerb is the PRECONDITION for emitting ResultDryRun at all
// ("a path that provably performed no outward write", deskkit/audit.go). If `check` ever
// grows a write, this goes red before the laundering does any harm.
func TestCheckNeverCallsAMutatingVerb(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		args []string
	}{
		{"clean", map[string]string{"FAKEGH_SEARCH_HITS": "[]"}, []string{"check", "-R", allowedRepo, "--title", "some distinct subject"}},
		{"duplicate", nil, []string{"check", "-R", allowedRepo, "--title", "oracle price feed goes stale"}},
		{"outage", map[string]string{"FAKEGH_SEARCH_FAIL": "1"}, []string{"check", "-R", allowedRepo, "--title", "some subject"}},
		{"bad-repo", nil, []string{"check", "-R", "evil/elsewhere", "--title", "some subject"}},
		{"bad-flags", nil, []string{"check", "-R", allowedRepo}},
		{"untokenizable", nil, []string{"check", "-R", allowedRepo, "--title", "a to the"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := withEnv(t)
			if tc.name == "duplicate" {
				t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "oracle price feed goes stale"))
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			runCapture(tc.args)
			assertNoMutatingGH(t, *calls)
			assertNoForbiddenGH(t, *calls)
		})
	}
}

// --- Finding 6: untrusted remote text must never reach a terminal live

// controlPayload is a crafted remote string carrying the exact primitives
// deskkit.StripControl's doc names: ESC[K (erase-in-line), CR (rewrite the current line),
// ESC[32m (colour). Two repos in the fixed set are PUBLIC, so issue titles are authored by
// arbitrary external users.
const controlPayload = "\x1b[K\r\x1b[32mdeskfile filing gate"

// TestNoRawControlBytesInAnyOutput drives the payload through EVERY remote surface
// deskfile reads — search-hit titles and urls, `issue view` state and url, and gh's own
// stderr — across every verb, and asserts that no invocation ever emits a terminal-active
// byte. Before the fix, `new`'s duplicate refusal printed the candidate title with %s and
// the ESC/CR survived to stderr verbatim.
func TestNoRawControlBytesInAnyOutput(t *testing.T) {
	craftedHits := func(t *testing.T) string {
		t.Helper()
		b, err := json.Marshal([]map[string]any{{
			"number": 11,
			"title":  "oracle price feed goes stale" + controlPayload,
			"url":    "https://github.com/" + allowedRepo + "/issues/11" + controlPayload,
			"labels": []map[string]string{{"name": "class" + controlPayload}},
		}})
		if err != nil {
			t.Fatalf("marshal crafted hits: %v", err)
		}
		return string(b)
	}

	cases := []struct {
		name string
		env  map[string]string
		args func(body string) []string
	}{
		{"new-duplicate-refusal", nil, func(b string) []string {
			return []string{"new", "-R", allowedRepo, "--title", "oracle price feed goes stale", "--body-file", b}
		}},
		{"check-duplicate", nil, func(string) []string {
			return []string{"check", "-R", allowedRepo, "--title", "oracle price feed goes stale"}
		}},
		{"check-clean-summary", nil, func(string) []string {
			return []string{"check", "-R", allowedRepo, "--title", "a completely different subject"}
		}},
		{"attach-closed-target", map[string]string{
			"FAKEGH_ISSUE_STATE": "CLOSED" + controlPayload,
			"FAKEGH_ISSUE_URL":   "https://x/11" + controlPayload,
		}, func(b string) []string {
			return []string{"attach", "-R", allowedRepo, "--to", "11", "--body-file", b}
		}},
		{"gh-stderr-on-search-failure", map[string]string{
			"FAKEGH_SEARCH_FAIL":    "1",
			"FAKEGH_STDERR_PAYLOAD": controlPayload,
		}, func(b string) []string {
			return []string{"new", "-R", allowedRepo, "--title", "oracle price feed goes stale", "--body-file", b}
		}},
		{"gh-stderr-on-create-failure", map[string]string{
			"FAKEGH_CREATE_FAIL":    "1",
			"FAKEGH_STDERR_PAYLOAD": controlPayload,
		}, func(b string) []string {
			return []string{"new", "-R", allowedRepo, "--title", "a distinct new subject", "--body-file", b}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", craftedHits(t))
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			body := bodyFileWith(t, "an observation")
			_, out := runCapture(tc.args(body))
			assertNoTerminalActiveBytes(t, "stdout+stderr", out)
			// The audit log is replayed into terminals and agent context too.
			for _, e := range readAudit(t) {
				assertNoTerminalActiveBytes(t, "audit title", e.Title)
				assertNoTerminalActiveBytes(t, "audit detail", e.Detail)
			}
		})
	}
}

// TestFormatCandidatesNoNewlineRowInjection proves a title carrying an embedded newline
// cannot fabricate an extra candidate row. StripControl deliberately KEEPS \n and \t
// (deskboard/issueboard need them for layout), so assertNoTerminalActiveBytes above cannot
// see this path — it skips both runes on purpose. A remote title such as
// "real title\n  - #999 (score 1.00, class issue): consolidated here", printed with %s,
// would render as a second, fully-formed row for a candidate the search never returned
// (delta re-review, PR #484, "Blocking — newline row injection survives in
// formatCandidates"). formatCandidates now prints with %q, so the whole title — embedded
// newline included — stays on the one line its candidate owns.
func TestFormatCandidatesNoNewlineRowInjection(t *testing.T) {
	injected := "real title\n  - #999 (score 1.00, class issue): consolidated here"
	cands := []candidate{{Number: 7, Title: injected, Score: 0.59}}

	out := formatCandidates(cands)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("formatCandidates produced %d lines from 1 candidate, want 1 — a title's "+
			"embedded newline fabricated an extra row; out=%q", len(lines), out)
	}
	if strings.Contains(out, "\n  - #999") {
		t.Fatalf("a real newline still precedes the fabricated row; out=%q", out)
	}
}

// assertNoTerminalActiveBytes fails if s carries any C0/C1 control rune other than tab and
// newline — the two the renderers use for layout and neither of which can move the cursor
// backwards (deskkit/strip.go).
func assertNoTerminalActiveBytes(t *testing.T, what, s string) {
	t.Helper()
	for i, r := range s {
		if r == '\t' || r == '\n' {
			continue
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			t.Fatalf("%s carries terminal-active rune %q at byte %d — remote text reached the "+
				"terminal unsanitized; full value: %q", what, r, i, s)
		}
	}
}

// --- Finding 2: an unanswered search must not read as "no duplicates exist"

// TestFailClosedEmptySearchOutput: exit 0 with EMPTY stdout is a search that produced
// nothing at all, not an empty result set (a real `gh --json` prints `[]`). Reading it as
// "no duplicates" is the one direction this tool says it will not take.
func TestFailClosedEmptySearchOutput(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_EMPTY", "1")
	body := bodyFileWith(t, "a filing against a silent search")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a filing against a silent search", "--body-file", body})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("empty-search new rc = %d, want 6", rc)
	}
	assertNoMutatingGH(t, *calls)

	rc, _ = runCapture([]string{"check", "-R", allowedRepo, "--title", "a filing against a silent search"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("empty-search check rc = %d, want 6", rc)
	}
	assertNoMutatingGH(t, *calls)
}

// --- Finding 7: the caller's raw title must not reach GitHub's search QUERY LANGUAGE

// TestSearchQueryCarriesNoQualifiers proves the dedupe query is built from normalised
// tokens, so no colon-qualifier, quote or negation in a title can rescope the candidate
// set. A rescoped search returns a set that does not contain the duplicate, which reads to
// the gate as "no duplicate exists" — the fail-open direction. Desk titles routinely carry
// colons ("bugs-gc: prune closed-issue files"), so this is an everyday input, not an
// exotic one.
func TestSearchQueryCarriesNoQualifiers(t *testing.T) {
	calls := withEnv(t)
	title := `bugs-gc: prune closed-issue files repo:evil/elsewhere is:closed -label:"x" NOT foo`
	runCapture([]string{"check", "-R", allowedRepo, "--title", title})

	var query string
	for _, c := range ghCalls(*calls) {
		if len(c) >= 3 && c[1] == "search" && c[2] == "issues" {
			if len(c) < 4 {
				t.Fatalf("search call has no query positional: %v", c)
			}
			query = c[3]
		}
	}
	if query == "" {
		t.Fatalf("no `gh search issues` call recorded: %v", ghCalls(*calls))
	}
	for _, bad := range []string{":", `"`, "-", "(", ")", "NOT"} {
		if strings.Contains(query, bad) {
			t.Fatalf("search query %q still carries %q — a raw title can rescope the candidate set", query, bad)
		}
	}
	if !strings.Contains(query, "bugs") || !strings.Contains(query, "prune") {
		t.Fatalf("search query %q dropped the substantive tokens", query)
	}
}

// TestUntokenizableTitleRefuses: a title that normalises to NO scorable tokens can never
// score above the threshold against any candidate, so passing it through would be a free,
// unconditional bypass of the whole matcher. Both verbs refuse it (exit 5) and `new` makes
// no create.
func TestUntokenizableTitleRefuses(t *testing.T) {
	calls := withEnv(t)
	body := bodyFileWith(t, "a filing with a contentless title")

	rc, out := runCapture([]string{"new", "-R", allowedRepo, "--title", "a to the", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("untokenizable new rc = %d, want 5; out=%s", rc, out)
	}
	if !strings.Contains(out, "no scorable tokens") {
		t.Fatalf("refusal must say why; out=%s", out)
	}
	assertNoMutatingGH(t, *calls)

	rc, _ = runCapture([]string{"check", "-R", allowedRepo, "--title", "?! ..."})
	if rc != deskkit.ExitRefused {
		t.Fatalf("untokenizable check rc = %d, want 5", rc)
	}
	assertNoMutatingGH(t, *calls)
}

// --- Finding 9: pin the policy CONSTANTS, not just the behaviour around them

// TestPolicyConstantsPinned asserts the literal values, which is the only thing that
// catches a change to the policy itself. The behavioural tests all derive their fixtures
// from these constants (`strings.Repeat("a", maxBodyBytes+1)`, `seedNewAudit(t,
// defaultNewBudgetPerSession, ...)`), so they scale with whatever the constant becomes and
// stay GREEN against 16 MiB or a budget of 9999. deskkit's ratelimit_test.go carries the
// same counter-pattern for RateLimitPerPRPerHour.
//
// Changing any value here is a POLICY change: update this test in the same commit, with
// the argument in the message.
func TestPolicyConstantsPinned(t *testing.T) {
	if maxBodyBytes != 16*1024 {
		t.Errorf("maxBodyBytes = %d, want %d (body cap is 16 KiB)", maxBodyBytes, 16*1024)
	}
	if defaultNewBudgetPerSession != 3 {
		t.Errorf("defaultNewBudgetPerSession = %d, want 3 (the default per-session filing cap)",
			defaultNewBudgetPerSession)
	}
	if budgetWindow != 24*time.Hour {
		t.Errorf("budgetWindow = %v, want 24h", budgetWindow)
	}
	if matchThreshold != 0.5 {
		t.Errorf("matchThreshold = %v, want 0.5", matchThreshold)
	}
	if classLabelBoost != 0.15 {
		t.Errorf("classLabelBoost = %v, want 0.15", classLabelBoost)
	}
	if searchLimit != "20" {
		t.Errorf("searchLimit = %q, want \"20\"", searchLimit)
	}
	// The boost must not be able to mint a match from nothing even if both move.
	if classLabelBoost >= matchThreshold {
		t.Errorf("classLabelBoost %v >= matchThreshold %v — the boost alone could trigger a match",
			classLabelBoost, matchThreshold)
	}
}

// --- pure-function matcher tests --------------------------------------------------

func TestTokenize(t *testing.T) {
	cases := map[string][]string{
		"Oracle Price Feed Goes Stale": {"oracle", "price", "feed", "goes", "stale"},
		"the a an of to":               {}, // all stopwords / short
		"":                             {},
		"Foo-Bar baz! qux":             {"foo", "bar", "baz", "qux"},
	}
	for in, want := range cases {
		got := tokenize(in)
		if !equalSlice(got, want) {
			t.Errorf("tokenize(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestMatchScore(t *testing.T) {
	q := tokenize("oracle price feed goes stale")
	// near-duplicate: 4 of 5 shared → high score
	if s := matchScore(q, "oracle price feed goes stale under load", false); s < matchThreshold {
		t.Errorf("near-dup score = %.2f, want >= %.2f", s, matchThreshold)
	}
	// unrelated → 0
	if s := matchScore(q, "totally unrelated other topic", false); s != 0 {
		t.Errorf("unrelated score = %.2f, want 0", s)
	}
	// class boost alone never triggers a match from zero overlap
	if s := matchScore(q, "totally unrelated other topic", true); s != 0 {
		t.Errorf("class boost from zero overlap = %.2f, want 0", s)
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- helpers ----------------------------------------------------------------------

// runCapture runs the CLI in-process and captures combined stdout/stderr so refusal
// messages can be asserted. It does NOT use os/exec — run() is the same function main
// calls, so this tests the real dispatch path including Guard.
func runCapture(args []string) (int, string) {
	// Capture stdout+stderr by swapping the fds' writers. run() prints to os.Stdout/Stderr
	// directly, so redirect via temp files (simplest robust approach for in-process).
	oldOut, oldErr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()

	rc := run(args)
	wOut.Close()
	wErr.Close()
	var buf strings.Builder
	readInto(&buf, rOut)
	readInto(&buf, rErr)
	return rc, buf.String()
}

func readInto(b *strings.Builder, r *os.File) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			b.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
}
