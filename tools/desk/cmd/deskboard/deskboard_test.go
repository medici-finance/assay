package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// fake gh shim — placed first in PATH so the REAL exec path is exercised. It
// records every invocation (one per line) to $DESKBOARD_GH_LOG and answers reads
// from env-supplied fixtures. This is the machinery behind the read-only proof:
// the test enumerates the recorded invocations, it does not just check "no error".
// ---------------------------------------------------------------------------

const ghShim = `#!/bin/sh
printf '%s ' "$@" >> "$DESKBOARD_GH_LOG"
printf '\n' >> "$DESKBOARD_GH_LOG"

if [ -n "$DESKBOARD_GH_FAIL_REPO" ]; then
  for a in "$@"; do
    [ "$a" = "$DESKBOARD_GH_FAIL_REPO" ] && { echo "gh: simulated failure for $DESKBOARD_GH_FAIL_REPO" >&2; exit 1; }
  done
fi

s="$*"

# The open-PR read is now a gh api graphql whose repo travels as split owner=/name=
# args, so an owner/name FAIL_REPO never appears as one whole argv element and the loop
# above cannot see it. Match the split form too, so failing a whole repo still
# reaches the enumeration read the desk starts every sweep with.
if [ -n "$DESKBOARD_GH_FAIL_REPO" ]; then
  case "$DESKBOARD_GH_FAIL_REPO" in
    */*)
      fo=${DESKBOARD_GH_FAIL_REPO%%/*}
      fn=${DESKBOARD_GH_FAIL_REPO#*/}
      case "$s" in
        *"owner=$fo "*"name=$fn"*) echo "gh: simulated failure for $DESKBOARD_GH_FAIL_REPO" >&2; exit 1 ;;
      esac ;;
  esac
fi

# DESKBOARD_GH_FAIL_MATCH fails any call whose full argv CONTAINS the pattern.
# DESKBOARD_GH_FAIL_REPO cannot reach the per-PR reads: it compares whole argv
# ELEMENTS, and the slug is embedded inside "repos/<slug>/pulls/<n>", so an API
# failure on that endpoint was unreachable from a test and its fail-closed branch
# was consequently unpinned (#247 / review R5).
case "$s" in
  "") ;;
  *) if [ -n "$DESKBOARD_GH_FAIL_MATCH" ]; then
       case "$s" in
         *"$DESKBOARD_GH_FAIL_MATCH"*) echo "gh: simulated failure (401/rate limit) for $DESKBOARD_GH_FAIL_MATCH" >&2; exit 1 ;;
       esac
     fi ;;
esac

# DESKBOARD_GH_FAIL_PATH: fail ONLY the reads whose argv contains this substring, so a
# test can break one PR's signal (a /reviews or /commits/ read) while the rest of the
# board answers normally. DESKBOARD_GH_FAIL_REPO above kills a whole repo; this is the
# per-PR granularity the "unassessable" row is defined at.
if [ -n "$DESKBOARD_GH_FAIL_PATH" ]; then
  case "$s" in
    *"$DESKBOARD_GH_FAIL_PATH"*) echo "gh: simulated failure for $DESKBOARD_GH_FAIL_PATH" >&2; exit 1 ;;
  esac
fi

# DESKBOARD_GH_HANG_PATH: WEDGE the reads whose argv contains this substring — block far
# longer than the test's per-unit gh budget (ghTimeout) so ghRun's context deadline must
# fire and kill this subprocess. Models #594's blocking auth/token-refresh hang, which a
# fast exit-1 (DESKBOARD_GH_FAIL_PATH) never did. exec replaces the shell with sleep so
# there is no grandchild left holding the stdout pipe when the deadline kills the process.
if [ -n "$DESKBOARD_GH_HANG_PATH" ]; then
  case "$s" in
    *"$DESKBOARD_GH_HANG_PATH"*) exec sleep 60 ;;
  esac
fi

case "$s" in
  *"pullRequests(states:OPEN"*)
    # The open-PR enumeration (fetchOpenPRs) — a gh api graphql whose --jq already
    # reshapes the response into the SAME flat array the old gh pr list --json produced,
    # so the shim serves that flat fixture directly (jq is not re-run here). The repo
    # travels as split owner=/name= args, so DESKBOARD_GH_PR_REPO selection
    # reconstructs owner/name rather than matching an "owner/repo" element.
    if [ -n "$DESKBOARD_GH_PR_REPO" ]; then
      po=${DESKBOARD_GH_PR_REPO%%/*}
      pn=${DESKBOARD_GH_PR_REPO#*/}
      case "$s" in
        *"owner=$po "*"name=$pn"*) printf '%s' "$DESKBOARD_GH_PRLIST_JSON" ;;
        *) printf '[]' ;;
      esac
    elif [ -n "$DESKBOARD_GH_PRLIST_JSON" ]; then
      printf '%s' "$DESKBOARD_GH_PRLIST_JSON"
    else
      printf '[]'
    fi
    ;;
  *"api graphql"*)
    if [ -n "$DESKBOARD_GH_GRAPHQL_JSON" ]; then printf '%s' "$DESKBOARD_GH_GRAPHQL_JSON"; else printf '{"data":{"repository":{"pullRequest":{"lastEditedAt":null,"comments":{"pageInfo":{"hasNextPage":false},"nodes":[]},"reviews":{"pageInfo":{"hasNextPage":false},"nodes":[]},"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[]}},"issue":{"lastEditedAt":null,"comments":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}'; fi
    ;;
  *"pr list"*)
    if [ -n "$DESKBOARD_GH_PR_REPO" ]; then
      case "$s" in
        *"$DESKBOARD_GH_PR_REPO"*) printf '%s' "$DESKBOARD_GH_PRLIST_JSON" ;;
        *) printf '[]' ;;
      esac
    elif [ -n "$DESKBOARD_GH_PRLIST_JSON" ]; then
      printf '%s' "$DESKBOARD_GH_PRLIST_JSON"
    else
      printf '[]'
    fi
    ;;
  *"/reviews"*)
    if [ -n "$DESKBOARD_GH_REVIEWS_JSON" ]; then printf '%s' "$DESKBOARD_GH_REVIEWS_JSON"; else printf '[]'; fi
    ;;
  *"search prs"*)
    if [ -n "$DESKBOARD_GH_SEARCH_JSON" ]; then printf '%s' "$DESKBOARD_GH_SEARCH_JSON"; else printf '[]'; fi
    ;;
  *"/files"*)
    # GET /pulls/{n}/files — the PAGINATED changed-file read. DESKBOARD_GH_PRFILES_JSON
    # is a REST array; page 2+ is always empty (fixtures are single-page).
    case "$s" in
      *"page=1"*) if [ -n "$DESKBOARD_GH_PRFILES_JSON" ]; then printf '%s' "$DESKBOARD_GH_PRFILES_JSON"; else printf '[]'; fi ;;
      *) printf '[]' ;;
    esac
    ;;
  *"/pulls/"*)
    # GET repos/{o}/{r}/pulls/{n} — ONE endpoint with TWO readers in this binary:
    # fetchChangedFiles' changed_files reconciliation (board.go) and fetchPRState's
    # merged-vs-closed tombstone read (prstate.go). Real GitHub answers BOTH from one
    # payload, so the shim serves one arm; two arms for one endpoint would let the
    # first match silently starve the other reader.
    #
    # DESKBOARD_GH_PRSTATE_JSON takes precedence and may carry changed_files too, so a
    # single fixture can drive both readers (pinned by
    # TestPRPayload_OneEndpointServesBothReaders).
    #
    # The DEFAULT is deliberately an unusable payload (#247): a test that does not state
    # how a PR left the open set gets "unknown", never a convenient MERGED — and it also
    # leaves changed_files absent, which disables the count cross-check rather than
    # faking one.
    if [ -n "$DESKBOARD_GH_PRSTATE_JSON" ]; then printf '%s' "$DESKBOARD_GH_PRSTATE_JSON"
    elif [ -n "$DESKBOARD_GH_PRMETA_JSON" ]; then printf '%s' "$DESKBOARD_GH_PRMETA_JSON"
    else printf '{}'; fi
    ;;
  *"pr view"*)
    if [ -n "$DESKBOARD_GH_FILES_JSON" ]; then printf '%s' "$DESKBOARD_GH_FILES_JSON"; else printf '{"files":[]}'; fi
    ;;
  *"pr diff"*)
    printf 'diff --git a/x b/x\n+hello\n'
    ;;
  *"/commits/"*"/status"*)
    # GET /commits/{sha}/status — the zero-CI probe's external-CI check (#1652).
    if [ -n "$DESKBOARD_GH_COMBINED_STATUS_JSON" ]; then printf '%s' "$DESKBOARD_GH_COMBINED_STATUS_JSON"; else printf '{"total_count":0,"statuses":[]}'; fi
    ;;
  *"/contents/.github/workflows?ref="*)
    # Directory listing for the zero-CI probe (#1652). Default: 404 (repo has no
    # workflows directory — a known answer, not a failure).
    if [ -n "$DESKBOARD_GH_WORKFLOWS_DIR_JSON" ]; then printf '%s' "$DESKBOARD_GH_WORKFLOWS_DIR_JSON"; else echo "gh: HTTP 404: Not Found" >&2; exit 1; fi
    ;;
  *"/contents/.github/workflows/"*)
    # One workflow file's contents. Fixture env var per file:
    # DESKBOARD_GH_WORKFLOW_<NAME> with the file name uppercased and every
    # non-alphanumeric mapped to _ (tools.yml -> DESKBOARD_GH_WORKFLOW_TOOLS_YML).
    name="${s##*/contents/.github/workflows/}"
    name="${name%%\?*}"
    key=$(printf '%s' "$name" | tr '[:lower:]' '[:upper:]' | tr -c 'A-Z0-9' '_')
    eval "body=\$DESKBOARD_GH_WORKFLOW_$key"
    if [ -n "$body" ]; then printf '%s' "$body"; else echo "gh: HTTP 404: Not Found" >&2; exit 1; fi
    ;;
  *"/contents/"*)
    printf '{"content":"aGVsbG8K","encoding":"base64"}'
    ;;
  *"/comments"*)
    # GET /issues/<n>/comments — PR conversation comments (stall clock).
    # DESKBOARD_GH_PRCOMMENTS_JSON is a REST array; page 2+ is always empty (fixtures are single-page).
    case "$s" in
      *"page=1"*) if [ -n "$DESKBOARD_GH_PRCOMMENTS_JSON" ]; then printf '%s' "$DESKBOARD_GH_PRCOMMENTS_JSON"; else printf '[]'; fi ;;
      *) printf '[]' ;;
    esac
    ;;
  *"/issues"*)
    if [ -n "$DESKBOARD_GH_ISSUES_JSON" ]; then printf '%s' "$DESKBOARD_GH_ISSUES_JSON"; else printf '[]'; fi
    ;;
  *"/compare/"*)
    if [ -n "$DESKBOARD_GH_COMPARE_JSON" ]; then printf '%s' "$DESKBOARD_GH_COMPARE_JSON"; else printf '{"files":[]}'; fi
    ;;
  *"/check-runs"*)
    if [ -n "$DESKBOARD_GH_CR_RED_REPO" ]; then
      case "$s" in
        *"$DESKBOARD_GH_CR_RED_REPO"*) printf '%s' "$DESKBOARD_GH_CR_RED_JSON"; exit 0 ;;
      esac
    fi
    if [ -n "$DESKBOARD_GH_CHECKRUNS_JSON" ]; then printf '%s' "$DESKBOARD_GH_CHECKRUNS_JSON"; else printf '{"total_count":1,"check_runs":[{"name":"ci","status":"completed","conclusion":"success","head_sha":"deadbeef0000"}]}'; fi
    ;;
  *"/commits?"*)
    if [ -n "$DESKBOARD_GH_COMMITS_JSON" ]; then printf '%s' "$DESKBOARD_GH_COMMITS_JSON"; else printf '[{"sha":"deadbeef0000"}]'; fi
    ;;
  *"/commits/"*)
    # GET /repos/<r>/commits/<sha> — single-commit metadata (head-push clock).
    # The committer date drives the stall window; a fixture sets DESKBOARD_GH_COMMIT_JSON.
    # Default = "now" so a run with no explicit fixture reads as a fresh push (not stalled).
    if [ -n "$DESKBOARD_GH_COMMIT_JSON" ]; then printf '%s' "$DESKBOARD_GH_COMMIT_JSON"; else printf '{"commit":{"committer":{"date":"'$(date -u '+%Y-%m-%dT%H:%M:%SZ')'"}}}'; fi
    ;;
  *"api repos/"*)
    # repo metadata for policydrift. Every repo answers "private" unless it
    # is listed in DESKBOARD_GH_PUBLIC_REPOS; DESKBOARD_GH_REPOMETA_OVERRIDE replaces
    # the whole body (for the malformed/contradictory-API cases).
    if [ -n "$DESKBOARD_GH_REPOMETA_OVERRIDE" ]; then printf '%s' "$DESKBOARD_GH_REPOMETA_OVERRIDE"; exit 0; fi
    r=""
    for a in "$@"; do case "$a" in repos/*) r="${a#repos/}" ;; esac; done
    vis=private; priv=true
    for p in $DESKBOARD_GH_PUBLIC_REPOS; do
      [ "$p" = "$r" ] && { vis=public; priv=false; }
    done
    printf '{"visibility":"%s","private":%s}' "$vis" "$priv"
    ;;
  *)
    printf '{}'
    ;;
esac
`

// installFakeGH writes the shim first in PATH, isolates HOME (audit/kill-switch), and
// points the invocation log at a temp file. Returns the log path.
func installFakeGH(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	home := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDir, "gh")
	if err := os.WriteFile(shim, []byte(ghShim), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "gh.log")
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	plantFixtureRoster(t, home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DESKBOARD_GH_LOG", logPath)
	t.Setenv("DESK_TOOLS_DISABLED", "")          // ensure the kill switch is disarmed
	t.Setenv("DESKBOARD_GATE_SCORES_JSON", "[]") // default: no gate scores (existing tests)
	installFakeForge(t)
	return logPath
}

// installFakeForge overrides the forgeFor seam so the board's THREE typed reads
// (ListOpenChanges + the PR/issue trust events) resolve to a fake backend that reads the SAME
// env fixtures the gh shim used before those reads migrated off the CLI (the read-verbs-on-the-seam migration):
// DESKBOARD_GH_PRLIST_JSON for the enumeration, DESKBOARD_GH_GRAPHQL_JSON for the trust
// queries, and DESKBOARD_GH_FAIL_REPO to fail the enumeration for one whole repo. The board's
// PERIPHERAL reads (reviews, changed files, commits, search, status) stay on the gh shim, so
// the two mechanisms run side by side and no per-test fixture changes.
// fakeTrust records how many typed trust-events reads (PRTrustEvents + IssueTrustEvents) the
// board issued, so a bounded-fetch test can assert exactly one fired for the one untrusted
// author. The board's sweep is concurrent, so the counter is mutex-guarded; the tests that
// read it are sequential (t.Setenv forbids t.Parallel), so it is reset per install.
var (
	fakeTrustMu    sync.Mutex
	fakeTrustReads int
)

func resetFakeTrust() {
	fakeTrustMu.Lock()
	fakeTrustReads = 0
	fakeTrustMu.Unlock()
}

func countFakeTrustReads() int {
	fakeTrustMu.Lock()
	defer fakeTrustMu.Unlock()
	return fakeTrustReads
}

func installFakeForge(t *testing.T) {
	t.Helper()
	resetFakeTrust()
	resetForgeHooks()
	fakeForgeT = t
	prev := forgeFor
	prevOwner := forgeForOwner
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, ok := strings.Cut(repo, "/")
		if !ok {
			return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("bad repo "+repo, nil)
		}
		return &fakeForge{repo: repo}, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	// The owner-wide search resolves through the same fake; the coordinate is nominal (the
	// fake's SearchOpenChanges reads an env fixture, not the repo).
	forgeForOwner = func(owner string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return &fakeForge{repo: owner + "/_"}, deskkit.ForgeRepo{Owner: owner}, nil
	}
	t.Cleanup(func() { forgeFor = prev; forgeForOwner = prevOwner; resetForgeHooks() })
}

// fakeForge serves the three migrated reads from env fixtures; it embeds the interface, so any
// OTHER (write) op the board never calls would panic if reached.
type fakeForge struct {
	deskkit.Forge
	repo string
}

func (f *fakeForge) ListOpenChanges(deskkit.ForgeRepo) (*deskkit.OpenChanges, error) {
	if fr := os.Getenv("DESKBOARD_GH_FAIL_REPO"); fr != "" && fr == f.repo {
		// The enumeration read is the split-owner gh-api-graphql the shim's FAIL_REPO branch
		// used to break; the typed op fails the same whole repo.
		return nil, deskkit.Unverifiable("cannot read open PRs for "+f.repo+": simulated failure", nil)
	}
	js := os.Getenv("DESKBOARD_GH_PRLIST_JSON")
	if prRepo := os.Getenv("DESKBOARD_GH_PR_REPO"); prRepo != "" && prRepo != f.repo {
		js = "" // per-repo selection: only the named repo serves the fixture
	}
	if strings.TrimSpace(js) == "" {
		return &deskkit.OpenChanges{Cap: prListLimit}, nil
	}
	return openChangesFromPRListJSON(js, f.repo)
}

// openChangesFromPRListJSON maps the SAME flat prBase-shaped array the old openPRs reshape
// produced into an *OpenChanges — so an existing fixture (a []prBase JSON literal) drives the
// typed ListOpenChanges read unchanged. It is the exact inverse of fetchOpenPRs's own
// OpenChange→prBase mapping, which is what makes the round trip byte-faithful.
func openChangesFromPRListJSON(js, repo string) (*deskkit.OpenChanges, error) {
	var prs []prBase
	if err := json.Unmarshal([]byte(js), &prs); err != nil {
		return nil, deskkit.Unverifiable("cannot parse PR list fixture for "+repo, err)
	}
	oc := &deskkit.OpenChanges{Cap: prListLimit}
	for _, p := range prs {
		change := deskkit.OpenChange{
			Number: p.Number, Title: p.Title, Body: p.Body, State: p.State, Draft: p.IsDraft,
			CreatedAt: p.CreatedAt, LastEditedAt: p.LastEditedAt, MergeStateStatus: p.MergeStateStatus,
			HeadSHA: p.HeadRefOid, HeadRef: p.HeadRefName, BaseRef: p.BaseRefName,
			Author: deskkit.Account{Login: p.Author.Login}, Labels: p.labelNames(),
		}
		for _, c := range p.StatusCheckRollup {
			change.Rollup = append(change.Rollup, deskkit.RollupNode{
				Typename: c.TypeName, Name: c.Name, Status: c.Status, Conclusion: c.Conclusion,
				StartedAt: c.StartedAt, CompletedAt: c.CompletedAt,
				Context: c.Context, State: c.State, CreatedAt: c.CreatedAt,
			})
		}
		oc.Changes = append(oc.Changes, change)
	}
	oc.TruncatedAtCap = len(oc.Changes) >= prListLimit
	return oc, nil
}

// stubForgeList overrides forgeFor so the bulk open-change read is served by list(repo) — for a
// direct fetchOpenPRs/headOfPR unit test that needs a per-repo Go error (out-of-installation,
// 401) the env fixture cannot express. The trust reads still read env fixtures, and the
// peripheral gh reads are unaffected.
func stubForgeList(t *testing.T, list func(repo string) (*deskkit.OpenChanges, error)) {
	t.Helper()
	prev := forgeFor
	t.Cleanup(func() { forgeFor = prev })
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, ok := strings.Cut(repo, "/")
		if !ok {
			return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("bad repo "+repo, nil)
		}
		return &listFnForge{fakeForge: fakeForge{repo: repo}, list: list}, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
}

type listFnForge struct {
	fakeForge
	list func(repo string) (*deskkit.OpenChanges, error)
}

func (f *listFnForge) ListOpenChanges(deskkit.ForgeRepo) (*deskkit.OpenChanges, error) {
	return f.list(f.repo)
}

func (f *fakeForge) PRTrustEvents(_ deskkit.ForgeRepo, _ int) (*deskkit.TrustPayload, error) {
	return f.trustFromFixture(true)
}

func (f *fakeForge) IssueTrustEvents(_ deskkit.ForgeRepo, _ int) (*deskkit.TrustPayload, error) {
	return f.trustFromFixture(false)
}

func (f *fakeForge) trustFromFixture(pr bool) (*deskkit.TrustPayload, error) {
	fakeTrustMu.Lock()
	fakeTrustReads++
	fakeTrustMu.Unlock()
	js := os.Getenv("DESKBOARD_GH_GRAPHQL_JSON")
	if strings.TrimSpace(js) == "" {
		if pr {
			js = `{"data":{"repository":{"pullRequest":{"lastEditedAt":null,"comments":{"pageInfo":{"hasNextPage":false},"nodes":[]},"reviews":{"pageInfo":{"hasNextPage":false},"nodes":[]},"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`
		} else {
			js = `{"data":{"repository":{"issue":{"lastEditedAt":null,"comments":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`
		}
	}
	var (
		be       time.Time
		ev       []deskkit.ContentEvent
		complete bool
		err      error
	)
	if pr {
		be, ev, complete, err = deskkit.ParsePRTrustPayload([]byte(js))
	} else {
		be, ev, complete, err = deskkit.ParseIssueTrustPayload([]byte(js))
	}
	if err != nil {
		return nil, deskkit.Unverifiable("cannot parse trust fixture", err)
	}
	return &deskkit.TrustPayload{BodyEdited: be, Events: ev, Complete: complete}, nil
}

// forgeHooks lets a UNIT test drive the board's peripheral reads programmatically (per-sha
// fixtures, error injection) where an env fixture cannot — the replacement for the old
// per-call `ghRun` stub. A nil hook falls through to the env-fixture reader below, which is
// what keeps the end-to-end run() tests working unchanged. Tests are sequential (t.Setenv
// forbids t.Parallel), so a package-level hook set is safe; installFakeForge resets it.
type forgeHookSet struct {
	recentCommits func(repo string, limit int) ([]deskkit.RepoCommit, error)
	getCommit     func(repo, sha string) (*deskkit.RepoCommit, error)
	checks        func(repo, sha string) (*deskkit.ChecksAtHead, error)
	reviews       func(repo string, num int) ([]deskkit.Review, error)
	comments      func(repo string, num int) ([]deskkit.Comment, error)
	labelEvents   func(repo string, num int) ([]deskkit.LabelEvent, error)
	compare       func(repo, base, head string) (*deskkit.RefComparison, error)
	getPR         func(repo string, num int) (*deskkit.PullRequest, error)
}

var forgeHooks forgeHookSet

func resetForgeHooks() { forgeHooks = forgeHookSet{} }

// envJSON reads env var name into out; empty/unset leaves out unchanged and returns false.
func envJSON(t *testing.T, name string, out any) bool {
	t.Helper()
	js := os.Getenv(name)
	if strings.TrimSpace(js) == "" {
		return false
	}
	if err := json.Unmarshal([]byte(js), out); err != nil {
		t.Fatalf("bad %s fixture: %v", name, err)
		return false
	}
	return true
}

// failFor honors the fail-injection envs the gh shim used (DESKBOARD_GH_FAIL_PATH /
// DESKBOARD_GH_FAIL_MATCH): if either is a substring of any hint, the op fails could-not-check,
// mirroring the shim's per-read exit-1. It lets the per-PR fail-closed tests drive the typed
// reads exactly as they drove the gh argv.
func (f *fakeForge) failFor(hints ...string) error {
	for _, env := range []string{"DESKBOARD_GH_FAIL_PATH", "DESKBOARD_GH_FAIL_MATCH"} {
		pat := os.Getenv(env)
		if pat == "" {
			continue
		}
		for _, h := range hints {
			if strings.Contains(h, pat) {
				return deskkit.Unverifiable("simulated failure for "+pat, nil)
			}
		}
	}
	return nil
}

func (f *fakeForge) tb() *testing.T { return fakeForgeT }

// fakeForgeT is the current test, set by installFakeForge, so a fixture parse failure fails
// the test rather than panicking.
var fakeForgeT *testing.T

func (f *fakeForge) GetPullRequest(_ deskkit.ForgeRepo, num int) (*deskkit.PullRequest, error) {
	if forgeHooks.getPR != nil {
		return forgeHooks.getPR(f.repo, num)
	}
	if err := f.failFor(fmt.Sprintf("/repos/%s/pulls/%d", f.repo, num), fmt.Sprintf("/pulls/%d", num)); err != nil {
		return nil, err
	}
	var w struct {
		State        string `json:"state"`
		MergedAt     string `json:"merged_at"`
		Merged       bool   `json:"merged"`
		ChangedFiles int    `json:"changed_files"`
	}
	js := os.Getenv("DESKBOARD_GH_PRSTATE_JSON")
	if js == "" {
		js = os.Getenv("DESKBOARD_GH_PRMETA_JSON")
	}
	if strings.TrimSpace(js) != "" {
		// An unparseable payload is a READ failure the real backend returns as could-not-check
		// (not a test-fatal), so fetchPRState can map it to `unknown` rather than a guessed merge.
		if err := json.Unmarshal([]byte(js), &w); err != nil {
			return nil, deskkit.Unverifiable("cannot parse PR payload", err)
		}
	}
	return &deskkit.PullRequest{State: w.State, MergedAt: w.MergedAt, Merged: w.Merged, ChangedFiles: w.ChangedFiles}, nil
}

func (f *fakeForge) ReviewsAtHead(_ deskkit.ForgeRepo, num int) ([]deskkit.Review, error) {
	if forgeHooks.reviews != nil {
		return forgeHooks.reviews(f.repo, num)
	}
	if err := f.failFor(fmt.Sprintf("/repos/%s/pulls/%d/reviews", f.repo, num), "/reviews"); err != nil {
		return nil, err
	}
	var raw []struct {
		User        struct{ Login string } `json:"user"`
		Body        string                 `json:"body"`
		State       string                 `json:"state"`
		CommitID    string                 `json:"commit_id"`
		SubmittedAt string                 `json:"submitted_at"`
	}
	envJSON(f.tb(), "DESKBOARD_GH_REVIEWS_JSON", &raw)
	out := make([]deskkit.Review, 0, len(raw))
	for _, r := range raw {
		out = append(out, deskkit.Review{
			Author: deskkit.Account{Login: r.User.Login}, Body: r.Body, State: r.State,
			CommitID: r.CommitID, SubmittedAt: r.SubmittedAt,
		})
	}
	return out, nil
}

func (f *fakeForge) ListChangedFiles(_ deskkit.ForgeRepo, _ int) ([]deskkit.ChangedFile, error) {
	var raw []struct {
		Filename         string `json:"filename"`
		PreviousFilename string `json:"previous_filename"`
	}
	envJSON(f.tb(), "DESKBOARD_GH_PRFILES_JSON", &raw)
	out := make([]deskkit.ChangedFile, 0, len(raw))
	for _, c := range raw {
		out = append(out, deskkit.ChangedFile{Filename: c.Filename, PreviousFilename: c.PreviousFilename})
	}
	return out, nil
}

func (f *fakeForge) CompareRefs(_ deskkit.ForgeRepo, base, head string) (*deskkit.RefComparison, error) {
	if forgeHooks.compare != nil {
		return forgeHooks.compare(f.repo, base, head)
	}
	if err := f.failFor(fmt.Sprintf("/repos/%s/compare/%s...%s", f.repo, base, head), "/compare/"); err != nil {
		return nil, err
	}
	var w struct {
		Status   string `json:"status"`
		BehindBy int    `json:"behind_by"`
		Files    []struct {
			Filename string `json:"filename"`
		} `json:"files"`
	}
	envJSON(f.tb(), "DESKBOARD_GH_COMPARE_JSON", &w)
	out := &deskkit.RefComparison{Status: w.Status, BehindBy: w.BehindBy}
	for _, c := range w.Files {
		out.Files = append(out.Files, deskkit.ChangedFile{Filename: c.Filename})
	}
	return out, nil
}

func (f *fakeForge) ListComments(_ deskkit.ForgeRepo, num int) ([]deskkit.Comment, error) {
	if forgeHooks.comments != nil {
		return forgeHooks.comments(f.repo, num)
	}
	if err := f.failFor(fmt.Sprintf("repos/%s/issues/%d/comments", f.repo, num), fmt.Sprintf("issues/%d/comments", num), "/comments"); err != nil {
		return nil, err
	}
	var raw []struct {
		User      struct{ Login string } `json:"user"`
		CreatedAt string                 `json:"created_at"`
		Body      string                 `json:"body"`
	}
	envJSON(f.tb(), "DESKBOARD_GH_PRCOMMENTS_JSON", &raw)
	out := make([]deskkit.Comment, 0, len(raw))
	for _, c := range raw {
		out = append(out, deskkit.Comment{Author: deskkit.Account{Login: c.User.Login}, CreatedAt: c.CreatedAt, Body: c.Body})
	}
	return out, nil
}

func (f *fakeForge) ListLabelEvents(_ deskkit.ForgeRepo, num int) ([]deskkit.LabelEvent, error) {
	if forgeHooks.labelEvents != nil {
		return forgeHooks.labelEvents(f.repo, num)
	}
	var raw []struct {
		Event     string `json:"event"`
		CreatedAt string `json:"created_at"`
		Label     struct {
			Name string `json:"name"`
		} `json:"label"`
	}
	envJSON(f.tb(), "DESKBOARD_GH_LABELEVENTS_JSON", &raw)
	out := make([]deskkit.LabelEvent, 0, len(raw))
	for _, e := range raw {
		if e.Event != "labeled" {
			continue
		}
		out = append(out, deskkit.LabelEvent{Name: e.Label.Name, CreatedAt: e.CreatedAt})
	}
	return out, nil
}

func (f *fakeForge) GetCommit(_ deskkit.ForgeRepo, sha string) (*deskkit.RepoCommit, error) {
	if forgeHooks.getCommit != nil {
		return forgeHooks.getCommit(f.repo, sha)
	}
	if err := f.failFor(fmt.Sprintf("/repos/%s/commits/%s", f.repo, sha), "/commits/"); err != nil {
		return nil, err
	}
	js := os.Getenv("DESKBOARD_GH_COMMIT_JSON")
	if strings.TrimSpace(js) == "" {
		return &deskkit.RepoCommit{SHA: sha, CommittedDate: time.Now().UTC().Format(time.RFC3339)}, nil
	}
	var w struct {
		Author    *struct{ Login string } `json:"author"`
		Committer *struct{ Login string } `json:"committer"`
		Commit    struct {
			Committer struct {
				Date string `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	if err := json.Unmarshal([]byte(js), &w); err != nil {
		return nil, deskkit.Unverifiable("bad DESKBOARD_GH_COMMIT_JSON", err)
	}
	rc := &deskkit.RepoCommit{SHA: sha, CommittedDate: w.Commit.Committer.Date}
	if w.Author != nil {
		rc.AuthorLogin = w.Author.Login
	}
	if w.Committer != nil {
		rc.CommitterLogin = w.Committer.Login
	}
	return rc, nil
}

func (f *fakeForge) ListRecentCommits(_ deskkit.ForgeRepo, limit int) ([]deskkit.RepoCommit, error) {
	if forgeHooks.recentCommits != nil {
		return forgeHooks.recentCommits(f.repo, limit)
	}
	js := os.Getenv("DESKBOARD_GH_COMMITS_JSON")
	if strings.TrimSpace(js) == "" {
		js = `[{"sha":"deadbeef0000"}]`
	}
	if js == "not json at all" { // TestActions_HealthUnknownDoesNotFailTheBoard
		return nil, deskkit.Unverifiable("unparseable commits payload", nil)
	}
	var raw []struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal([]byte(js), &raw); err != nil {
		return nil, deskkit.Unverifiable("bad DESKBOARD_GH_COMMITS_JSON", err)
	}
	out := make([]deskkit.RepoCommit, 0, len(raw))
	for _, c := range raw {
		out = append(out, deskkit.RepoCommit{SHA: c.SHA})
	}
	return out, nil
}

func (f *fakeForge) ChecksAtHead(_ deskkit.ForgeRepo, sha string) (*deskkit.ChecksAtHead, error) {
	if forgeHooks.checks != nil {
		return forgeHooks.checks(f.repo, sha)
	}
	if err := f.failFor(fmt.Sprintf("/repos/%s/commits/%s/check-runs", f.repo, sha), "/check-runs"); err != nil {
		return nil, err
	}
	out := &deskkit.ChecksAtHead{}
	// Check-runs rollup: the RED-repo override wins, else DESKBOARD_GH_CHECKRUNS_JSON, else a
	// default single green run (mirrors the gh shim's default).
	crJS := ""
	if red := os.Getenv("DESKBOARD_GH_CR_RED_REPO"); red != "" && red == f.repo {
		crJS = os.Getenv("DESKBOARD_GH_CR_RED_JSON")
	}
	if crJS == "" {
		crJS = os.Getenv("DESKBOARD_GH_CHECKRUNS_JSON")
	}
	if strings.TrimSpace(crJS) == "" {
		crJS = `{"total_count":1,"check_runs":[{"name":"ci","status":"completed","conclusion":"success"}]}`
	}
	var cr struct {
		TotalCount int `json:"total_count"`
		CheckRuns  []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := json.Unmarshal([]byte(crJS), &cr); err != nil {
		return nil, deskkit.Unverifiable("bad check-runs fixture", err)
	}
	out.CheckRunsTotalCount = cr.TotalCount
	for _, c := range cr.CheckRuns {
		out.CheckRuns = append(out.CheckRuns, deskkit.CheckRun{Name: c.Name, Status: c.Status, Conclusion: c.Conclusion})
	}
	// Combined-status rollup (zero-CI probe reads StatusTotalCount).
	if csJS := os.Getenv("DESKBOARD_GH_COMBINED_STATUS_JSON"); strings.TrimSpace(csJS) != "" {
		var cs struct {
			TotalCount int `json:"total_count"`
			Statuses   []struct {
				State   string `json:"state"`
				Context string `json:"context"`
			} `json:"statuses"`
		}
		if err := json.Unmarshal([]byte(csJS), &cs); err != nil {
			return nil, deskkit.Unverifiable("bad combined-status fixture", err)
		}
		out.StatusTotalCount = cs.TotalCount
		for _, s := range cs.Statuses {
			out.Statuses = append(out.Statuses, deskkit.StatusContext{State: s.State, Context: s.Context})
		}
	}
	return out, nil
}

func (f *fakeForge) ListWorkflowFiles(_ deskkit.ForgeRepo, _ string) ([]string, error) {
	js := os.Getenv("DESKBOARD_GH_WORKFLOWS_DIR_JSON")
	if strings.TrimSpace(js) == "" {
		return nil, &deskkit.ForgeAPIError{Status: 404, Method: "GET", Path: "/contents/.github/workflows"}
	}
	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(js), &entries); err != nil {
		return nil, deskkit.Unverifiable("bad workflows-dir fixture", err)
	}
	var names []string
	for _, e := range entries {
		if e.Type != "file" {
			continue
		}
		if strings.HasSuffix(e.Name, ".yml") || strings.HasSuffix(e.Name, ".yaml") {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (f *fakeForge) ReadFile(_ deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	if strings.HasPrefix(in.File, ".github/workflows/") {
		name := strings.TrimPrefix(in.File, ".github/workflows/")
		key := strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z':
				return r - 32
			case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
				return r
			default:
				return '_'
			}
		}, name)
		body := os.Getenv("DESKBOARD_GH_WORKFLOW_" + key)
		if body == "" {
			return nil, &deskkit.ForgeAPIError{Status: 404, Method: "GET", Path: "/contents/" + in.File}
		}
		// The fixture is the Contents-API shape ({"content":<base64>,"encoding":"base64"}) the
		// gh shim served; decode it to the raw file bytes the typed ReadFile returns.
		return &deskkit.FileContent{Content: decodeContentsFixture(body), Exists: true, SHA: "wf"}, nil
	}
	// Default: the gh shim served base64 "aGVsbG8K" = "hello\n".
	return &deskkit.FileContent{Content: []byte("hello\n"), Exists: true, SHA: "c"}, nil
}

func (f *fakeForge) ChangeDiff(_ deskkit.ForgeRepo, _ int) (string, error) {
	return "diff --git a/x b/x\n+hello\n", nil
}

// decodeContentsFixture turns a Contents-API fixture ({"content":<base64>,"encoding":"base64"})
// into the raw file bytes; a plain (non-JSON) fixture is returned as-is.
func decodeContentsFixture(body string) []byte {
	var v struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return []byte(body)
	}
	if v.Encoding == "base64" {
		if dec, derr := base64.StdEncoding.DecodeString(strings.ReplaceAll(v.Content, "\n", "")); derr == nil {
			return dec
		}
	}
	return []byte(v.Content)
}

func (f *fakeForge) SearchOpenChanges(owner string) (*deskkit.ChangeSearchResults, error) {
	// The gh shim failed `search prs --owner <owner>` when DESKBOARD_GH_FAIL_REPO matched the
	// owner arg; a search that could not run is could-not-check, never "no gaps".
	if fr := os.Getenv("DESKBOARD_GH_FAIL_REPO"); fr != "" && strings.Contains(owner, fr) {
		return nil, deskkit.Unverifiable("cannot search open PRs for owner "+owner, nil)
	}
	var raw []struct {
		Number     int    `json:"number"`
		Title      string `json:"title"`
		CreatedAt  string `json:"createdAt"`
		Repository struct {
			NameWithOwner string `json:"nameWithOwner"`
		} `json:"repository"`
	}
	envJSON(f.tb(), "DESKBOARD_GH_SEARCH_JSON", &raw)
	out := &deskkit.ChangeSearchResults{Cap: 200}
	for _, r := range raw {
		out.Results = append(out.Results, deskkit.ChangeSearchResult{
			Repo: r.Repository.NameWithOwner, Number: r.Number, Title: r.Title, CreatedAt: r.CreatedAt,
		})
	}
	// Mirror the real GitHub backend's completeness signal: a read that came back exactly at
	// (or above) the page cap may be truncated. The scope consumer reads this authoritative
	// flag rather than re-deriving it from the row count.
	out.TruncatedAtCap = len(out.Results) >= out.Cap
	return out, nil
}

func (f *fakeForge) ListOpenIssues(_ deskkit.ForgeRepo) ([]deskkit.IssueSummary, error) {
	var raw []struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		User    struct {
			Login string `json:"login"`
			ID    int64  `json:"id"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		CreatedAt   string    `json:"created_at"`
		PullRequest *struct{} `json:"pull_request"`
	}
	envJSON(f.tb(), "DESKBOARD_GH_ISSUES_JSON", &raw)
	out := make([]deskkit.IssueSummary, 0, len(raw))
	for _, is := range raw {
		if is.PullRequest != nil {
			continue
		}
		labels := make([]string, 0, len(is.Labels))
		for _, l := range is.Labels {
			labels = append(labels, l.Name)
		}
		out = append(out, deskkit.IssueSummary{
			Number: is.Number, Title: is.Title, URL: is.HTMLURL, CreatedAt: is.CreatedAt,
			Author: deskkit.Account{Login: is.User.Login, ID: is.User.ID}, Labels: labels,
		})
	}
	return out, nil
}

func (f *fakeForge) RepoVisibility(_ deskkit.ForgeRepo) (string, error) {
	// The policy-drift fail-closed test names a repo the read cannot resolve via
	// DESKBOARD_GH_FAIL_REPO ("repos/<owner>/<name>"); an unreadable repo is could-not-check.
	if fr := os.Getenv("DESKBOARD_GH_FAIL_REPO"); fr != "" && strings.Contains(fr, f.repo) {
		return "", deskkit.Unverifiable("cannot read repo metadata for "+f.repo, nil)
	}
	if o := os.Getenv("DESKBOARD_GH_REPOMETA_OVERRIDE"); strings.TrimSpace(o) != "" {
		var w struct {
			Visibility string `json:"visibility"`
		}
		if err := json.Unmarshal([]byte(o), &w); err != nil {
			return "", deskkit.Unverifiable("bad repometa override", err)
		}
		if w.Visibility == "" {
			return "", deskkit.Unverifiable("repo "+f.repo+" has no .visibility field", nil)
		}
		return w.Visibility, nil
	}
	for _, p := range strings.Fields(os.Getenv("DESKBOARD_GH_PUBLIC_REPOS")) {
		if p == f.repo {
			return "public", nil
		}
	}
	return "private", nil
}

// mutatingVerbs are the gh subcommand verbs that WRITE. A read-only tool must never
// emit any of them in the verb position (fields[1]).
var mutatingVerbs = map[string]bool{
	"comment": true, "review": true, "ready": true, "create": true, "edit": true,
	"merge": true, "close": true, "delete": true, "reopen": true, "lock": true,
	"unlock": true, "update": true, "transfer": true, "sync": true,
}

// firstOffense returns a non-empty reason if a recorded gh invocation is mutating:
// a write verb in the subcommand position, or a POST/PATCH/PUT/DELETE method / field
// flag on `gh api`. Returns "" for a clean read-only invocation.
//
// `gh api graphql` is the one sanctioned use of -f/-F: GraphQL rides POST for reads
// too, so the read-only proof instead asserts NO field of the invocation carries a
// mutation — the trust-gate query is `query(...)`, and any "mutation" token fails.
func firstOffense(fields []string) string {
	for _, f := range fields {
		if f == "graphql" {
			for _, g := range fields {
				if strings.Contains(strings.ToLower(g), "mutation") {
					return "graphql invocation carries a mutation: " + g
				}
			}
			return ""
		}
	}
	if len(fields) >= 2 && mutatingVerbs[fields[1]] {
		return "mutating subcommand verb: " + strings.Join(fields[:2], " ")
	}
	for i, f := range fields {
		switch {
		case f == "-X" || f == "--method":
			if i+1 < len(fields) {
				m := strings.ToUpper(fields[i+1])
				if m == "POST" || m == "PATCH" || m == "PUT" || m == "DELETE" {
					return "mutating method flag: " + f + " " + fields[i+1]
				}
			}
		case strings.HasPrefix(f, "-X") && len(f) > 2:
			m := strings.ToUpper(f[2:])
			if m == "POST" || m == "PATCH" || m == "PUT" || m == "DELETE" {
				return "mutating method flag: " + f
			}
		case f == "-f" || f == "-F" || f == "--field" || f == "--raw-field":
			return "field/body flag on gh api: " + f
		}
	}
	return ""
}

func readInvocations(t *testing.T, logPath string) [][]string {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading gh log: %v", err)
	}
	var out [][]string
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		out = append(out, strings.Fields(ln))
	}
	return out
}

// TestReadsReachNoForgeCLI is the successor to the old read-only PATH-shim proof. That proof
// ran every subcommand and asserted no recorded `gh` invocation was a mutating call; the
// the forge-seam migration retired the last `gh` read in cmd/deskboard, so the property is
// now STRONGER and structural — deskboard reaches no forge CLI at all (the ban's
// TestNoForgeCLIShellout pins that at the source level; Verify row 4 greps it to zero), and a
// typed Forge READ op cannot mutate. This test keeps the end-to-end half: every subcommand
// runs green through the forge seam, and the fake `gh` on PATH is NEVER invoked (its log stays
// empty) — a recorded invocation would mean a read slipped back onto the CLI.
func TestReadsReachNoForgeCLI(t *testing.T) {
	logPath := installFakeGH(t)
	// One PR per repo so prs/actions make real per-PR reads.
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":1,"title":"t","state":"OPEN","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"abc123","baseRefName":"main","mergeStateStatus":"BLOCKED","statusCheckRollup":[]}]`)
	// The PR above has a zero rollup, so the zero-CI probe fires on prs and
	// actions. These fixtures drive it down its FULL read path (check-runs →
	// combined status → workflows listing → workflow contents), so the read-only
	// enumeration below covers every endpoint the probe can emit.
	t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
	t.Setenv("DESKBOARD_GH_WORKFLOWS_DIR_JSON",
		`[{"name":"ci.yml","type":"file","path":".github/workflows/ci.yml"}]`)
	t.Setenv("DESKBOARD_GH_WORKFLOW_CI_YML", workflowFixture("on:\n  pull_request:\n"))

	repo := "example-org/tracker"
	runs := [][]string{
		{"prs"},
		{"actions"},
		{"queue"},
		{"scope"}, // #359: the reconciliation read must be GET-only too
		{"reviews", repo, "1"},
		{"diff", repo, "1"},
		{"files", repo, "1"},
		{"files", repo, "1", "secrets/foo.yaml"},
		{"policydrift"},
		// stalled: a stalled candidate exercises the per-PR reads the
		// read-only proof must cover — reviews, single-commit, conversation comments, and
		// the behind-main compare. Commit dated 2020 → past the 48h window; no comments.
		{"stalled"},
	}
	// stalled fixtures: a CHANGES_REQUESTED-at-head review, an old head commit (stalled),
	// no author comments, and a behind-main compare so every stalled read fires once per
	// repo. These only change stalled's path; other verbs tolerate them (still exit 0).
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
		`[{"user":{"login":"assay-reviewer-app[bot]"},"state":"CHANGES_REQUESTED","commit_id":"abc123","submitted_at":"2026-08-01T00:00:00Z"}]`)
	t.Setenv("DESKBOARD_GH_COMMIT_JSON", `{"commit":{"committer":{"date":"2020-01-01T00:00:00Z"}}}`)
	t.Setenv("DESKBOARD_GH_PRCOMMENTS_JSON", `[]`)
	t.Setenv("DESKBOARD_GH_COMPARE_JSON", `{"status":"behind","behind_by":5,"files":[]}`)
	// policydrift compares compiled-in visibility against the API; tell the shim the
	// truth so the read-only proof exercises it on its exit-0 path.
	t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))
	for _, args := range runs {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(%v) = exit %d, stderr=%s", args, code, errb.String())
		}
	}

	// deskboard reaches every forge read through the typed Forge seam now, so the fake `gh`
	// must never have been invoked: an empty log is the whole point. A recorded invocation —
	// mutating or not — would mean a read regressed back onto the CLI, which the ban would also
	// catch at the source level.
	if b, err := os.ReadFile(logPath); err == nil {
		if inv := readInvocations(t, logPath); len(inv) != 0 {
			t.Fatalf("deskboard invoked the forge CLI %d time(s) — every read must reach the typed "+
				"Forge seam, not gh:\n%s", len(inv), string(b))
		}
	}
	t.Logf("no-forge-CLI proof: every subcommand ran green through the Forge seam; the gh shim was never invoked")
}

// TestClassify ports v1's ACTION semantics (same input → same ACTION) and adds the
// #216 security gate and the MERGE-NOW class.
func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		in   classifyInput
		want string
	}{
		{"never reviewed", classifyInput{ever: false}, actNeedsReview},
		{"head advanced, benign merge", classifyInput{ever: true, atHead: false, ownFilesChanged: false}, actMergeCurr},
		{"head advanced, own files changed", classifyInput{ever: true, atHead: false, ownFilesChanged: true}, actReReview},
		{"blocking at head", classifyInput{ever: true, atHead: true, blocking: true, draft: true}, actBlocked},
		// approved at head + CI green + mergeConflict (DIRTY) → NOT MERGE-NOW
		{"merge-now dirty merge state", classifyInput{ever: true, atHead: true, approvedAtHead: true, ciGreen: true, draft: false, pass: 1, mergeConflict: true}, actReady},
		// approved at head + CI green + non-draft → MERGE-NOW
		{"merge-now ready", classifyInput{ever: true, atHead: true, approvedAtHead: true, ciGreen: true, draft: false, pass: 1}, actMergeNow},
		// approved at head + CI green + draft → MERGE-NOW
		{"merge-now flip", classifyInput{ever: true, atHead: true, approvedAtHead: true, ciGreen: true, draft: true, pass: 1}, actMergeNow},
		// approved at head + CI green + draft + risk + security pass → MERGE-NOW
		{"merge-now risk with secpass", classifyInput{ever: true, atHead: true, approvedAtHead: true, ciGreen: true, draft: true, riskClassed: true, securityPass: true, pass: 1}, actMergeNow},
		// approved at head + CI green + draft + risk + NO security → SEC-REVIEW-REQUIRED
		{"merge-now risk no secpass", classifyInput{ever: true, atHead: true, approvedAtHead: true, ciGreen: true, draft: true, riskClassed: true, securityPass: false, pass: 1}, actSecReview},
		// approved at head but CI NOT green → still READY (non-draft) or CI-RED (draft)
		{"ready with ci red non-draft", classifyInput{ever: true, atHead: true, approvedAtHead: true, ciGreen: false, draft: false, fail: 1}, actReady},
		// APPROVED then new push → RE-REVIEW (not MERGE-NOW)
		{"approved then new push", classifyInput{ever: true, atHead: false, ownFilesChanged: true}, actReReview},
		// existing cases (semantics unchanged; the FIXTURES are corrected — see below)
		{"non-draft, no approval at head", classifyInput{ever: true, atHead: true, draft: false, approvedAtHead: false}, actReady},
		{"approved but ci red", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, fail: 1}, actCIRed},
		{"approved but ci pending", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, pending: 1}, actWaitCI},
		//
		// #400 R2 — the five rows below used to say "approved GREEN" in their names and
		// leave ciGreen at its zero value, false. Every one of them was asserting a
		// verdict about a state it did not construct, and two of them ("non-risk",
		// "with secpass") expected FLIP — which is precisely how the FLIP arm could
		// claim "CI green" for a row whose CI was never established and stay green in
		// CI. The fixture could not represent the state it defended, so the guard was
		// never exercised. They now set ciGreen + a passing check, which is what their
		// names always claimed.
		{"approved green non-risk draft", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, ciGreen: true, pass: 1}, actMergeNow},
		{"approved green but merge conflict (#569)", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, ciGreen: true, pass: 1, mergeConflict: true}, actConflict},
		{"approved green risk no secpass", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, ciGreen: true, pass: 1, riskClassed: true, securityPass: false}, actSecReview},
		{"approved green risk with secpass", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, ciGreen: true, pass: 1, riskClassed: true, securityPass: true}, actMergeNow},
		//
		// #400 R2 — approved at head with NOTHING red, NOTHING pending, and nothing
		// green either: no check reported a verdict at all. Draft and non-draft alike,
		// this is a could-not-check, never a flip and never "already ready".
		{"approved, empty rollup, draft", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, ciGreen: false}, actCIUnverified},
		{"approved, empty rollup, non-draft", classifyInput{ever: true, atHead: true, draft: false, approvedAtHead: true, ciGreen: false}, actCIUnverified},
		//
		// #400 R3 — mergeability unknown. The flip does not depend on it (draft → FLIP,
		// saying so); the MERGE is withheld (non-draft → MERGE-STATE-UNKNOWN).
		{"approved green, merge state unknown, draft", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, ciGreen: true, pass: 1, mergeStateUnknown: true, mergeStateRaw: "UNKNOWN"}, actFlip},
		{"approved green, merge state unknown, non-draft", classifyInput{ever: true, atHead: true, draft: false, approvedAtHead: true, ciGreen: true, pass: 1, mergeStateUnknown: true, mergeStateRaw: ""}, actMergeStateUnknown},
		// #54: mergeStateStatus BEHIND — main has moved since this head was last synced.
		// The App's APPROVED verdict verified the diff against review-time main, not
		// current main. Draft flip is still safe (draft-ness alone gates it); MERGE-NOW
		// is withheld either way, mirroring the mergeStateUnknown pair above.
		{"approved green, merge state behind, draft", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, ciGreen: true, pass: 1, mergeBehind: true, mergeStateRaw: "BEHIND"}, actFlip},
		{"approved green, merge state behind, non-draft", classifyInput{ever: true, atHead: true, draft: false, approvedAtHead: true, ciGreen: true, pass: 1, mergeBehind: true, mergeStateRaw: "BEHIND"}, actMergeBehind},
		{"defensive check default", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: false}, actCheck},
		// #37: a no-op approval (suppressed by reduceReviews into an effective
		// CHANGES_REQUESTED, so blocking=true and approvedAtHead=false here) must classify
		// SUSPECT-APPROVAL, never FLIP or MERGE-NOW, even though CI is green.
		{"suspect no-op approval, ci green", classifyInput{ever: true, atHead: true, blocking: true, suspectNoOp: true, draft: true, ciGreen: true, pass: 1}, actSuspectApproval},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, _ := classify(c.in); got != c.want {
				t.Errorf("classify(%+v) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

// TestActions_PartialFailure_Exit6 proves the fail-closed rule: one repo failing
// fails the whole run with exit 6, the failing repo named — never a partial board.
func TestActions_PartialFailure_Exit6(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_FAIL_REPO", "example-org/agents")

	var out, errb bytes.Buffer
	code := run([]string{"prs"}, &out, &errb)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("run(prs) with a failing repo = exit %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(errb.String(), "example-org/agents") {
		t.Errorf("exit-6 message must name the failing repo; got: %s", errb.String())
	}
	if strings.Contains(out.String(), "\"prs\"") {
		t.Errorf("a failed run must not emit a (partial) board on stdout; got: %s", out.String())
	}
}

// TestActions_Tombstone_209 seeds a prior deskboard run that saw a PR open, then runs
// actions when that PR is gone (merged/closed) — a tombstone row must appear.
func TestActions_Tombstone_209(t *testing.T) {
	installFakeGH(t) // no PRLIST → every repo returns [] (nothing currently open)
	// #247: the tombstone must now be TOLD how the PR left the open set. This one
	// merged; TestTombstone_MergedVsClosed covers the other two states.
	t.Setenv("DESKBOARD_GH_PRSTATE_JSON", `{"state":"closed","merged":true,"merged_at":"2026-07-31T14:01:35Z"}`)

	// Prior audit line (within the trailing hour) recording agents#6 open.
	if err := deskkit.Log(deskkit.Entry{
		Tool:   "deskboard",
		Verb:   "actions",
		Result: deskkit.ResultOK,
		Detail: "open=example-org/agents#6",
		TS:     time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seeding audit: %v", err)
	}

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	found := false
	for _, ts := range rep.Tombstones {
		if ts.Repo == "example-org/agents" && ts.Number == 6 {
			found = true
			if !strings.Contains(ts.Note, "MERGED") {
				t.Errorf("tombstone note should say MERGED/drop; got %q", ts.Note)
			}
		}
	}
	if !found {
		t.Errorf("expected a #209 tombstone for agents#6; got %+v", rep.Tombstones)
	}
}

// TestSecurityReviewRequired_216: a risk-classed PR (changed file under secrets/) that the
// App APPROVED at head is SECURITY-REVIEW-REQUIRED without the Security-Review: pass
// line, and FLIP with it.
func TestSecurityReviewRequired_216(t *testing.T) {
	repo := "example-org/tracker"
	head := "deadbeefcafe"

	base := func(t *testing.T, reviewBody string) actionsReport {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", repo)
		// mergeStateStatus CLEAN — this test's subject is the risk/security gate,
		// not merge state (#569 gives DIRTY/BLOCKED their own CONFLICT outcome,
		// which would otherwise shadow FLIP/SECURITY-REVIEW-REQUIRED here).
		t.Setenv("DESKBOARD_GH_PRLIST_JSON",
			`[{"number":42,"title":"risky","isDraft":true,"author":{"login":"app/assay-worker-app"},"headRefOid":"`+head+`","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
			`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+`","body":`+jsonStr(reviewBody)+`,"submitted_at":"2026-07-10T00:00:00Z"}]`)
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"secrets/foo.yaml"}]`)

		var out, errb bytes.Buffer
		if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
		}
		var rep actionsReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
		}
		return rep
	}

	actionOf := func(rep actionsReport) (string, bool) {
		for _, r := range rep.Rows {
			if r.Number == 42 {
				return r.Action, r.RiskClassed
			}
		}
		return "", false
	}

	t.Run("no Security-Review line -> SECURITY-REVIEW-REQUIRED", func(t *testing.T) {
		rep := base(t, "looks good to me")
		act, risk := actionOf(rep)
		if !risk {
			t.Errorf("PR touching secrets/ must be risk-classed")
		}
		if act != actSecReview {
			t.Errorf("action = %s, want %s", act, actSecReview)
		}
	})

	t.Run("with Security-Review: pass -> FLIP", func(t *testing.T) {
		// Override merge state to CLEAN so the MERGE-NOW guard (which now requires
		// a mergeable PR) passes — a BLOCKED merge state correctly prevents MERGE-NOW.
		// The base fixture uses mergeStateStatus: BLOCKED; reclassify with CLEAN.
		head := "deadbeefcafe"
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", repo)
		t.Setenv("DESKBOARD_GH_PRLIST_JSON",
			`[{"number":42,"title":"risky","isDraft":true,"author":{"login":"app/assay-worker-app"},"headRefOid":"`+head+`","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
			`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+`","body":`+jsonStr("looks good\nSecurity-Review: pass")+`,"submitted_at":"2026-07-10T00:00:00Z"}]`)
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"secrets/foo.yaml"}]`)

		var out, errb bytes.Buffer
		if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
		}
		var rep actionsReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
		}
		act, _ := actionOf(rep)
		// approved at head + CI green + CLEAN → MERGE-NOW (was FLIP before the MERGE-NOW class).
		if act != actMergeNow {
			t.Errorf("action = %s, want %s", act, actMergeNow)
		}
	})
}

// TestNoOpApprovalSuspect_37 reproduces the LIVE EVIDENCE from #37 end-to-end
// through `deskboard actions`: decks#17 / decks#11 (2026-07-14) both flipped to APPROVED at
// an UNCHANGED head, no intervening commit, while a blocking CHANGES_REQUESTED stood. Before
// this fix the board read "latest bot review state at head" and would have reported both as
// FLIP-eligible (draft) / MERGE-NOW (non-draft) — this is the exact gap the live-evidence
// comment flags: "the board reads latest bot review state at head and would have reported
// both as FLIP-eligible... nothing in the system noticed." The row must classify
// SUSPECT-APPROVAL, never FLIP or MERGE-NOW, regardless of CI/draft state.
func TestNoOpApprovalSuspect_37(t *testing.T) {
	repo := "example-org/tracker"
	head := "deadbeefcafe"

	reviewsJSON := `[` +
		`{"user":{"login":"` + reviewerBotDisplay() + `"},"state":"CHANGES_REQUESTED","commit_id":"` + head + `",` +
		`"body":"blocker, address before merge","submitted_at":"2026-07-14T18:03:22Z"},` +
		`{"user":{"login":"` + reviewerBotDisplay() + `"},"state":"APPROVED","commit_id":"` + head + `",` +
		`"body":"looks good now","submitted_at":"2026-07-14T18:24:55Z"}` + // no push between these two
		`]`

	runFor := func(t *testing.T, draft bool) actionsReport {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", repo)
		draftStr := "false"
		if draft {
			draftStr = "true"
		}
		t.Setenv("DESKBOARD_GH_PRLIST_JSON",
			`[{"number":17,"title":"grant deck","isDraft":`+draftStr+`,"author":{"login":"app/assay-worker-app"},`+
				`"headRefOid":"`+head+`","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON", reviewsJSON)

		var out, errb bytes.Buffer
		if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
		}
		var rep actionsReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
		}
		return rep
	}

	actionOf := func(rep actionsReport) string {
		for _, r := range rep.Rows {
			if r.Number == 17 {
				return r.Action
			}
		}
		return ""
	}

	t.Run("draft — never FLIP", func(t *testing.T) {
		rep := runFor(t, true)
		act := actionOf(rep)
		if act != actSuspectApproval {
			t.Errorf("action = %s, want %s (draft no-op approval must never read FLIP)", act, actSuspectApproval)
		}
		if act == actFlip {
			t.Fatal("regression: the live-evidence no-op approval classified FLIP")
		}
	})

	t.Run("non-draft — never MERGE-NOW", func(t *testing.T) {
		rep := runFor(t, false)
		act := actionOf(rep)
		if act != actSuspectApproval {
			t.Errorf("action = %s, want %s (non-draft no-op approval must never read MERGE-NOW)", act, actSuspectApproval)
		}
		if act == actMergeNow {
			t.Fatal("regression: the live-evidence no-op approval classified MERGE-NOW")
		}
	})
}

// TestReduceReviews_NoOpApproval_37 unit-tests reduceReviews directly: a CHANGES_REQUESTED
// followed by an APPROVED at the SAME commit (no push) must reduce to blocking=true,
// approved=false, suspectNoOp=true — never approved=true. A genuine push (different
// commit_id) between the two must reduce normally to approved=true, suspectNoOp=false.
func TestReduceReviews_NoOpApproval_37(t *testing.T) {
	bot := reviewerBotDisplay()
	mk := func(state, commit string) review {
		var r review
		r.User.Login = bot
		r.State = state
		r.CommitID = commit
		r.SubmittedAt = "2026-07-14T18:24:55Z"
		return r
	}

	t.Run("same head — suspect, not approved", func(t *testing.T) {
		head := "deadbeefcafe"
		reviews := []review{
			mk("CHANGES_REQUESTED", head),
			mk("APPROVED", head), // no push between these two
		}
		st := reduceReviews(reviews, head)
		if !st.atHead {
			t.Fatal("expected atHead=true")
		}
		if st.approved {
			t.Error("a no-op approval must never reduce to approved=true")
		}
		if !st.blocking {
			t.Error("a no-op approval must still reduce to blocking=true")
		}
		if !st.suspectNoOp {
			t.Error("expected suspectNoOp=true")
		}
	})

	t.Run("retried same-head forgery still suspect", func(t *testing.T) {
		head := "deadbeefcafe"
		reviews := []review{
			mk("CHANGES_REQUESTED", head),
			mk("APPROVED", head),
			mk("APPROVED", head), // retry
		}
		st := reduceReviews(reviews, head)
		if st.approved {
			t.Error("a retried no-op approval must never reduce to approved=true")
		}
		if !st.suspectNoOp {
			t.Error("expected suspectNoOp=true on the retry too")
		}
	})

	t.Run("different head — genuine re-review, approved and not suspect", func(t *testing.T) {
		oldHead, newHead := "0f1e2d3c", "deadbeefcafe"
		reviews := []review{
			mk("CHANGES_REQUESTED", oldHead),
			mk("APPROVED", newHead), // a real push happened
		}
		st := reduceReviews(reviews, newHead)
		if !st.approved {
			t.Error("a genuine re-review after a push must reduce to approved=true")
		}
		if st.suspectNoOp {
			t.Error("a genuine re-review after a push must not be flagged suspectNoOp")
		}
	})
}

// TestMergeNow_ApprovedAge exercises MERGE-NOW end-to-end: approved at head, CI green,
// approved-age computed from the review's submitted_at. The --merge-now-threshold is set
// high so the decay banner does NOT fire.
func TestMergeNow_ApprovedAge(t *testing.T) {
	repo := "example-org/tracker"
	head := "deadbeefcafe"
	reviewTS := time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339)

	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":42,"title":"merge now test","isDraft":false,"author":{"login":"shared-agent"},"headRefOid":"`+head+`","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
		`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+`","body":"looks good","submitted_at":"`+reviewTS+`"}]`)

	var out, errb bytes.Buffer
	code := run([]string{"actions", "--merge-now-threshold", "1h"}, &out, &errb)
	if code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if len(rep.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rep.Rows))
	}
	r := rep.Rows[0]
	if r.Action != actMergeNow {
		t.Errorf("action = %s, want %s", r.Action, actMergeNow)
	}
	if r.ApprovedAge == "" {
		t.Error("approvedAge should be non-empty for MERGE-NOW")
	}
	t.Logf("approvedAge = %s", r.ApprovedAge)
	if rep.Header.MergeNowCount != 1 {
		t.Errorf("MergeNowCount = %d, want 1", rep.Header.MergeNowCount)
	}
	if rep.Header.MergeNowDecay {
		t.Error("MergeNowDecay should be false when approved-age < threshold")
	}
}

// TestMergeNow_DecayBanner exercises the decay alarm: approved-age > threshold emits
// the decay banner in both JSON header and --table output.
func TestMergeNow_DecayBanner(t *testing.T) {
	repo := "example-org/tracker"
	head := "deadbeefcafe"
	reviewTS := time.Now().UTC().Add(-30 * time.Minute).Format(time.RFC3339)

	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":42,"title":"decay test","isDraft":false,"author":{"login":"shared-agent"},"headRefOid":"`+head+`","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
		`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+`","body":"looks good","submitted_at":"`+reviewTS+`"}]`)

	var out, errb bytes.Buffer
	code := run([]string{"actions", "--merge-now-threshold", "10m"}, &out, &errb)
	if code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if !rep.Header.MergeNowDecay {
		t.Error("MergeNowDecay should be true when approved-age > threshold")
	}
	if len(rep.Header.MergeNowDecayPRs) == 0 || rep.Header.MergeNowDecayPRs[0] != 42 {
		t.Errorf("MergeNowDecayPRs = %v, want [42]", rep.Header.MergeNowDecayPRs)
	}

	var out2, errb2 bytes.Buffer
	code = run([]string{"actions", "--table", "--merge-now-threshold", "10m"}, &out2, &errb2)
	if code != deskkit.ExitOK {
		t.Fatalf("run(actions --table) = exit %d, stderr=%s", code, errb2.String())
	}
	tableStr := out2.String()
	if !strings.Contains(tableStr, "DECAY:") {
		t.Error("--table output should contain DECAY banner")
	}
	if !strings.Contains(tableStr, actMergeNow) {
		t.Error("--table output should contain MERGE-NOW action")
	}
}

// TestMergeNow_ApprovedThenPush_ReReview verifies: APPROVED at an old head, then new
// push with own files changed → RE-REVIEW (not MERGE-NOW).
func TestMergeNow_ApprovedThenPush_ReReview(t *testing.T) {
	repo := "example-org/tracker"
	oldHead := "aaa111"
	newHead := "bbb222"

	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":42,"title":"re-review test","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"`+newHead+`","mergeStateStatus":"BLOCKED","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
		`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+oldHead+`","body":"looks good","submitted_at":"2026-07-16T00:00:00Z"}]`)
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"board.go"}]`)
	t.Setenv("DESKBOARD_GH_COMPARE_JSON", `{"files":[{"filename":"board.go"}]}`)

	var out, errb bytes.Buffer
	code := run([]string{"actions", "--merge-now-threshold", "20m"}, &out, &errb)
	if code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if len(rep.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rep.Rows))
	}
	if rep.Rows[0].Action != actReReview {
		t.Errorf("action = %s, want %s (APPROVED at old head + new push = RE-REVIEW, not MERGE-NOW)", rep.Rows[0].Action, actReReview)
	}
}

// TestMergeNow_RankedFirst verifies MERGE-NOW rows sort before all other actions.
func TestMergeNow_RankedFirst(t *testing.T) {
	repo := "example-org/tracker"
	head := "deadbeefcafe"

	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":42,"title":"merge now","isDraft":false,"author":{"login":"shared-agent"},"headRefOid":"`+head+`","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]},{"number":43,"title":"needs review","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"abc123","mergeStateStatus":"BLOCKED","statusCheckRollup":[]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
		`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+`","body":"looks good","submitted_at":"2026-07-16T00:00:00Z"}]`)

	var out, errb bytes.Buffer
	code := run([]string{"actions"}, &out, &errb)
	if code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if len(rep.Rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(rep.Rows))
	}
	if rep.Rows[0].Number != 42 {
		t.Errorf("first row should be #42 (MERGE-NOW), got #%d (%s)", rep.Rows[0].Number, rep.Rows[0].Action)
	}
	if rep.Rows[1].Number != 43 {
		t.Errorf("second row should be #43 (NEEDS-REVIEW), got #%d (%s)", rep.Rows[1].Number, rep.Rows[1].Action)
	}
}

// TestMergeNow_ThresholdFlagParse verifies the --merge-now-threshold flag is parsed
// correctly and defaults to 20m.
func TestMergeNow_ThresholdFlagParse(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", "example-org/tracker")
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", "[]")

	var out bytes.Buffer
	code := run([]string{"actions"}, &out, &bytes.Buffer{})
	if code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d", code)
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	if rep.Header.MergeNowThreshold == "" {
		t.Error("MergeNowThreshold should be set (default 20m0s)")
	}

	out.Reset()
	code = run([]string{"actions", "--merge-now-threshold", "5m"}, &out, &bytes.Buffer{})
	if code != deskkit.ExitOK {
		t.Fatalf("run(actions --merge-now-threshold 5m) = exit %d", code)
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	if !strings.Contains(rep.Header.MergeNowThreshold, "5m") {
		t.Errorf("MergeNowThreshold = %s, want 5m0s", rep.Header.MergeNowThreshold)
	}

	var errb bytes.Buffer
	code = run([]string{"actions", "--merge-now-threshold", "xyz"}, &bytes.Buffer{}, &errb)
	if code != deskkit.ExitRefused {
		t.Errorf("invalid duration xyz = exit %d, want %d", code, deskkit.ExitRefused)
	}

	code = run([]string{"actions", "--merge-now-threshold"}, &bytes.Buffer{}, &errb)
	if code != deskkit.ExitRefused {
		t.Errorf("missing argument = exit %d, want %d", code, deskkit.ExitRefused)
	}
}

// TestMergeConflictGatesFlip_569 proves the #569 fix end-to-end: a PR that is otherwise
// FLIP-eligible (bot APPROVED at head, CI green, still draft, not blocking) but whose
// mergeStateStatus is DIRTY or BLOCKED must classify CONFLICT, never FLIP — deskboard
// must not signal "ready to flip" on a PR GitHub itself says can't be merged.
func TestMergeConflictGatesFlip_569(t *testing.T) {
	repo := "example-org/tracker"
	head := "deadbeefcafe"

	run569 := func(t *testing.T, mergeState string) actionsReport {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", repo)
		t.Setenv("DESKBOARD_GH_PRLIST_JSON",
			`[{"number":99,"title":"unmergeable","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"`+head+`","mergeStateStatus":"`+mergeState+`","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
			`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+`","body":"looks good","submitted_at":"2026-07-10T00:00:00Z"}]`)
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"README.md"}]`)

		var out, errb bytes.Buffer
		if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
		}
		var rep actionsReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
		}
		return rep
	}

	for _, mergeState := range []string{"DIRTY", "BLOCKED"} {
		t.Run(mergeState, func(t *testing.T) {
			rep := run569(t, mergeState)
			var found bool
			for _, r := range rep.Rows {
				if r.Number != 99 {
					continue
				}
				found = true
				if r.Action != actConflict {
					t.Errorf("mergeStateStatus=%s: action = %s, want %s (must not FLIP an unmergeable PR)", mergeState, r.Action, actConflict)
				}
			}
			if !found {
				t.Fatalf("mergeStateStatus=%s: PR #99 missing from actions report", mergeState)
			}
		})
	}
}

// TestKillSwitch_Exit3 proves the kill switch halts the tool before any read.
func TestKillSwitch_Exit3(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")

	var out, errb bytes.Buffer
	code := run([]string{"prs"}, &out, &errb)
	if code != deskkit.ExitDisabled {
		t.Fatalf("run(prs) with kill switch armed = exit %d, want %d", code, deskkit.ExitDisabled)
	}
}

// ---- stream-a/11: gate-score ordering tests ----

// TestGateScores_ScoreDescOrdering proves that owned PRs are ordered by gate score
// descending (highest score first), and that an unowned PR sorts to the bottom with
// the default score.
func TestGateScores_ScoreDescOrdering(t *testing.T) {
	repo := "example-org/tracker"
	installFakeGH(t)

	// Gate-scores fixture: stream-a/11=3000, stream-a/02=2000.
	t.Setenv("DESKBOARD_GATE_SCORES_JSON",
		`[{"brief":"stream-a/11","score":3000,"blockedCount":2,"stream":"stream-a","status":"implemented"},{"brief":"stream-a/02","score":2000,"blockedCount":0,"stream":"stream-a","status":"verified"}]`)

	// Three PRs: one owned (score 3000), one owned (score 2000), one unowned.
	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":20,"title":"low score","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"b","headRefName":"brief/stream-a-02-bar","mergeStateStatus":"CLEAN","statusCheckRollup":[]},{"number":10,"title":"high score","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"a","headRefName":"feat/stream-a-11-foo","mergeStateStatus":"CLEAN","statusCheckRollup":[]},{"number":30,"title":"unowned","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"c","headRefName":"fix/typo","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON", `[]`)
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[]`)

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}

	if len(rep.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rep.Rows))
	}

	// Row 0: highest score (3000, stream-a/11, PR #10)
	r0 := rep.Rows[0]
	if r0.Score != 3000 || r0.OwningBrief != "stream-a/11" || r0.Number != 10 {
		t.Errorf("row[0]: want score=3000 brief=stream-a/11 num=10; got score=%d brief=%q num=%d", r0.Score, r0.OwningBrief, r0.Number)
	}

	// Row 1: second highest score (2000, stream-a/02, PR #20)
	r1 := rep.Rows[1]
	if r1.Score != 2000 || r1.OwningBrief != "stream-a/02" || r1.Number != 20 {
		t.Errorf("row[1]: want score=2000 brief=stream-a/02 num=20; got score=%d brief=%q num=%d", r1.Score, r1.OwningBrief, r1.Number)
	}

	// Row 2: unowned (default score 0, PR #30)
	r2 := rep.Rows[2]
	if r2.Score != 0 || r2.OwningBrief != "" || r2.Number != 30 {
		t.Errorf("row[2]: want score=0 brief=\"\" num=30; got score=%d brief=%q num=%d", r2.Score, r2.OwningBrief, r2.Number)
	}
}

// TestGateScores_OldestFirstTieBreak proves that equal-score PRs are ordered
// oldest-first (lower PR number = older).
func TestGateScores_OldestFirstTieBreak(t *testing.T) {
	repo := "example-org/tracker"
	installFakeGH(t)

	// One brief in gate-scores so both PRs get the same score.
	t.Setenv("DESKBOARD_GATE_SCORES_JSON",
		`[{"brief":"stream-a/11","score":3000,"blockedCount":2,"stream":"stream-a","status":"implemented"}]`)

	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	// PR #25 was created BEFORE PR #50 (lower number = older).
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":50,"title":"newer","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"b","headRefName":"feat/stream-a-11-newer","mergeStateStatus":"CLEAN","statusCheckRollup":[]},{"number":25,"title":"older","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"a","headRefName":"feat/stream-a-11-older","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON", `[]`)
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[]`)

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}

	if len(rep.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rep.Rows))
	}

	// Both have the same score, but #25 (older) should come before #50 (newer).
	r0 := rep.Rows[0]
	r1 := rep.Rows[1]
	if r0.Number != 25 {
		t.Errorf("row[0].Number = %d, want 25 (oldest-first tie-break, lower number = older)", r0.Number)
	}
	if r1.Number != 50 {
		t.Errorf("row[1].Number = %d, want 50", r1.Number)
	}
	if r0.Score != r1.Score {
		t.Errorf("both rows should have same score; got %d and %d", r0.Score, r1.Score)
	}
}

// TestGateScores_UnownedDefault proves that an unowned PR takes the default score
// (0) and sorts among the rest by that score.
func TestGateScores_UnownedDefault(t *testing.T) {
	repo := "example-org/tracker"
	installFakeGH(t)

	// One brief with a positive score.
	t.Setenv("DESKBOARD_GATE_SCORES_JSON",
		`[{"brief":"stream-a/11","score":3000,"blockedCount":0,"stream":"stream-a","status":"implemented"}]`)

	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":5,"title":"unowned","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"a","headRefName":"fix/typo","mergeStateStatus":"CLEAN","statusCheckRollup":[]},{"number":10,"title":"owned","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"b","headRefName":"feat/stream-a-11-foo","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON", `[]`)
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[]`)

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}

	if len(rep.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rep.Rows))
	}

	// Owned PR should be first (score 3000 > 0).
	r0 := rep.Rows[0]
	if r0.OwningBrief != "stream-a/11" || r0.Score != 3000 {
		t.Errorf("row[0]: owned PR should be first; got brief=%q score=%d", r0.OwningBrief, r0.Score)
	}
	// Unowned PR should be second (score 0).
	r1 := rep.Rows[1]
	if r1.OwningBrief != "" || r1.Score != 0 {
		t.Errorf("row[1]: unowned PR should be second; got brief=%q score=%d", r1.OwningBrief, r1.Score)
	}
}

// TestGateScores_FailureExit6 proves that a gate-scores failure results in exit 6
// (fail closed — never a silent arrival-order fallback).
func TestGateScores_FailureExit6(t *testing.T) {
	installFakeGH(t)

	// Simulate a statusgen crash. Clear the default JSON fixture first.
	t.Setenv("DESKBOARD_GATE_SCORES_JSON", "")
	t.Setenv("DESKBOARD_GATE_SCORES_FAIL", "simulated statusgen crash")
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":1,"title":"t","isDraft":true,"headRefOid":"abc123","headRefName":"feat/x","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)

	var out, errb bytes.Buffer
	code := run([]string{"actions"}, &out, &errb)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("run(actions) with gate-scores failure = exit %d, want %d (exit 6)", code, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(errb.String(), "gate-scores") {
		t.Errorf("exit-6 message must name gate-scores failure; got: %s", errb.String())
	}
	if strings.Contains(out.String(), "\"rows\"") {
		t.Errorf("a failed run must not emit a partial board; got: %s", out.String())
	}
}

// jsonStr JSON-encodes a string (for embedding a review body in a fixture literal).
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// gqlPRPayload builds a PRTrustQuery response fixture: body lastEditedAt ("" → null),
// conversation-comment nodes, and review nodes.
func gqlPRPayload(bodyEdited string, comments, reviews []string) string {
	le := "null"
	if bodyEdited != "" {
		le = `"` + bodyEdited + `"`
	}
	return `{"data":{"repository":{"pullRequest":{"lastEditedAt":` + le +
		`,"comments":{"pageInfo":{"hasNextPage":false},"nodes":[` + strings.Join(comments, ",") + `]}` +
		`,"reviews":{"pageInfo":{"hasNextPage":false},"nodes":[` + strings.Join(reviews, ",") + `]}` +
		`,"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`
}

func gqlPRComment(login, typename string, id int64, createdAt string) string {
	return `{"createdAt":"` + createdAt + `","lastEditedAt":null,"author":{"login":"` + login +
		`","__typename":"` + typename + `","databaseId":` + fmt.Sprint(id) + `}}`
}

func gqlPRReview(login, typename string, id int64, submittedAt string) string {
	return `{"submittedAt":"` + submittedAt + `","lastEditedAt":null,"author":{"login":"` + login +
		`","__typename":"` + typename + `","databaseId":` + fmt.Sprint(id) + `}}`
}

// TestTrustGate_ActionsQuarantine: an open PR by an untrusted external author with no
// ada blessing gets NO ACTION row — it lands in the external quarantine section,
// with the trust-events read fired only for that PR (bounded fetch).
func TestTrustGate_ActionsQuarantine(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", "example-org/tracker")
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":7,"title":"external drive-by PR","isDraft":true,"author":{"login":"external-user"},"headRefOid":"abc123","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
	// Default graphql fixture: no comments, no reviews → unblessed.

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if len(rep.Rows) != 0 {
		t.Errorf("quarantined PR must have no ACTION row; got %+v", rep.Rows)
	}
	found := false
	for _, e := range rep.External {
		if e.Number == 7 && e.Author == "external-user" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PR #7 in the external quarantine section; got %+v", rep.External)
	}
	// Bounded: exactly one trust-events read. The open-PR enumeration is now also a
	// graphql read, and it too carries `lastEditedAt` (the body-edit signal the
	// non-commit-resolution re-review trigger reads), so the trust read is identified by
	// `reviewThreads` — a field the PRTrustQuery requests and the enumeration query never
	// does — rather than by `lastEditedAt` or the bare word "graphql".
	// Bounded fetch: the trust-events read (now the typed PRTrustEvents op) fires ONCE, for the
	// single untrusted-author PR — never for a trusted-author one.
	if got := countFakeTrustReads(); got != 1 {
		t.Errorf("expected exactly 1 trust-events read, got %d", got)
	}
}

// TestTrustGate_AdaReviewBlesses: a ada REVIEW (not just an issue comment)
// counts as the blessing — the PR classifies normally.
func TestTrustGate_AdaReviewBlesses(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", "example-org/tracker")
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":7,"title":"external but review-blessed","isDraft":true,"author":{"login":"external-user"},"headRefOid":"abc123","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
	t.Setenv("DESKBOARD_GH_GRAPHQL_JSON",
		gqlPRPayload("", nil, []string{gqlPRReview("ada", "User", 2001, "2026-07-21T10:00:00Z")}))

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if len(rep.External) != 0 {
		t.Errorf("ada-review-blessed PR must not be quarantined; got %+v", rep.External)
	}
	found := false
	for _, r := range rep.Rows {
		if r.Number == 7 {
			found = true
		}
	}
	if !found {
		t.Errorf("blessed PR must classify normally; got %+v", rep.Rows)
	}
}

// TestTrustGate_PRBlessThenEdit: ada blessed, then the PR BODY was edited — the
// blessing is void; the PR re-quarantines.
func TestTrustGate_PRBlessThenEdit(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", "example-org/tracker")
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":7,"title":"blessed then edited","isDraft":true,"author":{"login":"external-user"},"headRefOid":"abc123","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
	t.Setenv("DESKBOARD_GH_GRAPHQL_JSON",
		gqlPRPayload("2026-07-22T10:00:00Z", nil,
			[]string{gqlPRReview("ada", "User", 2001, "2026-07-21T10:00:00Z")}))

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if len(rep.External) != 1 {
		t.Errorf("body edited after the blessing must re-quarantine; got rows=%+v external=%+v", rep.Rows, rep.External)
	}
}
