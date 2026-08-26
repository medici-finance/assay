package main

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

//go:embed checkdefs
var checkdefsFS embed.FS

// execCommand is the single seam through which every exec.Command call flows.
// Production binds it to exec.Command; tests swap it to record args and return
// controlled output.
var execCommand = exec.Command

// --- GitHub API types ---

type advisoryResponse struct {
	GHSAID      string    `json:"ghsa_id"`
	State       string    `json:"state"`
	PrivateFork *forkInfo `json:"private_fork"`
}

type forkInfo struct {
	FullName string `json:"full_name"`
}

type repoResponse struct {
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
}

type branchResponse struct {
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// --- Check list types ---

type checkList struct {
	Version int         `json:"version"`
	Note    string      `json:"note,omitempty"`
	Checks  []checkSpec `json:"checks"`
}

type checkSpec struct {
	Name string   `json:"name"`
	Tool string   `json:"tool"`
	Args []string `json:"args"`

	// InvertExit marks a "forbidden pattern" check (grep-shaped): the check PASSES
	// only when the tool exits EXACTLY 1 -- grep's "no match". Exit 0 means the
	// forbidden pattern was found, which is a check FAILURE. Exit >= 2 is a TOOL
	// ERROR (bad regex, unreadable file, locale fault) and is unverifiable, never a
	// pass: a guard that malfunctioned is indistinguishable from a clean tree only
	// if you let it be.
	InvertExit bool `json:"invertExit,omitempty"`

	// RequireFiles lists paths (relative to the fetched tree) that must exist AND,
	// taken together, hold at least MinFiles regular files. Both halves matter: an
	// absent path catches a rename, and the file count catches the sharper case --
	// a path that EXISTS and holds nothing, over which grep and kubeconform both
	// report success having examined zero bytes.
	RequireFiles []string `json:"requireFiles,omitempty"`

	// MinFiles is the minimum number of regular files that must be present under
	// RequireFiles for the check to be considered to have had anything to examine.
	// Zero means 1 (see normalise): a check that declares paths but tolerates an
	// empty tree is a vacuous pass by construction.
	MinFiles int `json:"minFiles,omitempty"`

	// RequireOutputMatch is a regexp the tool's combined output must match for a
	// zero exit to count as a pass. It is the positive-work assertion for tools
	// that report what they did: `kubeconform` exits 0 over a directory holding no
	// manifests ("0 resource found in 0 file"), so requiring `Valid: [1-9]` is what
	// separates "validated something" from "found nothing to validate".
	// MANDATORY for every non-InvertExit check (see parseCheckList).
	RequireOutputMatch string `json:"requireOutputMatch,omitempty"`

	// Note is documentation carried with the definition (JSON has no comments).
	// The runner ignores it.
	Note string `json:"note,omitempty"`

	compiledOutputMatch *regexp.Regexp
}

// normalise fills in defaults that must not be left to the definition author.
func (c *checkSpec) normalise() {
	if c.MinFiles == 0 && len(c.RequireFiles) > 0 {
		c.MinFiles = 1
	}
}

// --- Env scrubbing (mirrored from cmd/deskgit/exec.go) ---

// envAllowlist is the ONLY set of environment variables passed to child processes.
// It mirrors deskgit's envAllowlist exactly. GIT_ASKPASS is deliberately NOT in this
// list -- deskadvisory adds its own controlled GIT_ASKPASS after scrubbing, so an
// inherited ambient askpass cannot reach a child process.
var envAllowlist = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true,
	"TERM": true, "TMPDIR": true, "TMP": true, "TEMP": true,
	"LANG": true, "LANGUAGE": true, "TZ": true,
	"SSH_AUTH_SOCK": true,
}

// scrubbedEnv returns the child environment: the allowlisted vars from the parent, plus
// GIT_TERMINAL_PROMPT=0 so a scrubbed-away askpass can never turn into an interactive
// hang. Every GIT_* var (and everything else not allowlisted) is dropped.
// Mirrored from cmd/deskgit/exec.go.
func scrubbedEnv(parent []string) []string {
	out := make([]string, 0, len(envAllowlist)+1)
	for _, kv := range parent {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		k := kv[:eq]
		if envAllowlist[k] || strings.HasPrefix(k, "LC_") {
			out = append(out, kv)
		}
	}
	out = append(out, "GIT_TERMINAL_PROMPT=0")
	return out
}

// fetchHardening mirrors deskgit's fetchHardening: pinned --refmap=, --upload-pack=git-upload-pack,
// and --no-recurse-submodules to prevent submodule config escaping the repo gate.
var fetchHardening = []string{"--refmap=", "--upload-pack=git-upload-pack", "--no-recurse-submodules"}

// runGit executes `git <args...>` in dir with the given environment.
// Mirrored from cmd/deskgit/exec.go.
func runGit(dir string, env []string, args ...string) (string, error) {
	cmd := execCommand("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	stdout := strings.TrimSpace(out.String())
	if err != nil {
		return stdout, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "),
			err, strings.TrimSpace(errb.String()))
	}
	return stdout, nil
}

// --- GitHub API helpers ---

// githubAPIBase is the base URL for GitHub API calls. It is a var (not const) so
// tests can point it at an httptest server. The host literal is sourced from
// deskkit.GitHubAPIBase (the forge module) so it is never constructed in a cmd package
// (the forge-abstraction seam).
var githubAPIBase = deskkit.GitHubAPIBase + "/"

// ghToken resolves the operator's ambient GitHub credential: GH_TOKEN,
// GITHUB_TOKEN, then `gh auth token`. It must NOT persist the token on disk.
func ghToken() (string, error) {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t, nil
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t, nil
	}
	out, err := execCommand("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("no GH_TOKEN, GITHUB_TOKEN, or gh auth token available: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ghAPI performs an authenticated GET to the GitHub API base + <path> and returns the body.
func ghAPI(path string) ([]byte, error) {
	token, err := ghToken()
	if err != nil {
		return nil, err
	}
	req, rerr := http.NewRequest("GET", githubAPIBase+path, nil)
	if rerr != nil {
		return nil, rerr
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, derr := http.DefaultClient.Do(req)
	if derr != nil {
		return nil, derr
	}
	defer resp.Body.Close()
	body, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		return nil, rerr
	}
	if resp.StatusCode >= 400 {
		return nil, &ghAPIError{StatusCode: resp.StatusCode, Body: string(body), Path: path}
	}
	return body, nil
}

type ghAPIError struct {
	StatusCode int
	Body       string
	Path       string
}

func (e *ghAPIError) Error() string {
	return fmt.Sprintf("GitHub API %s returned %d: %s", e.Path, e.StatusCode, e.Body)
}

// --- Check list loading ---

// loadCheckList loads the check definition for a repo. It tries the base repo's
// default branch first (via GitHub API), then falls back to a per-repo file
// committed in this repo's checkdefs/ directory.
func loadCheckList(baseRepo string) (*checkList, error) {
	body, err := ghAPI("repos/" + baseRepo + "/contents/.deskadvisory.json")
	if err == nil {
		var content struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		if jerr := json.Unmarshal(body, &content); jerr == nil && content.Encoding == "base64" {
			cleaned := strings.Map(func(r rune) rune {
				if r == '\n' || r == '\r' || r == ' ' {
					return -1
				}
				return r
			}, content.Content)
			decoded, derr := base64.StdEncoding.DecodeString(cleaned)
			if derr == nil {
				return parseCheckList(decoded)
			}
		}
	}

	embeddedPath := "checkdefs/" + baseRepo + ".json"
	data, ferr := checkdefsFS.ReadFile(embeddedPath)
	if ferr != nil {
		return nil, fmt.Errorf("no check list found: .deskadvisory.json not in base repo %s, and %s not found in this repo", baseRepo, embeddedPath)
	}
	return parseCheckList(data)
}

func parseCheckList(data []byte) (*checkList, error) {
	var cl checkList
	if err := json.Unmarshal(data, &cl); err != nil {
		return nil, fmt.Errorf("invalid check list: %w", err)
	}
	if cl.Version != 1 {
		return nil, fmt.Errorf("unsupported check list version %d", cl.Version)
	}
	if len(cl.Checks) == 0 {
		return nil, fmt.Errorf("check list contains no checks")
	}
	for i := range cl.Checks {
		c := &cl.Checks[i]
		c.normalise()
		if c.Name == "" {
			return nil, fmt.Errorf("check %d has no name", i)
		}
		if c.Tool == "" {
			return nil, fmt.Errorf("check %q has no tool", c.Name)
		}

		// Every check must be able to prove it had something to examine. A limited
		// view is never evidence about the world: a check with no declared paths
		// cannot distinguish "the tree is clean" from "the tree was not there".
		if len(c.RequireFiles) == 0 {
			return nil, fmt.Errorf("check %q declares no requireFiles: a check that "+
				"cannot say what it examined can pass over an empty tree", c.Name)
		}
		if c.MinFiles < 1 {
			return nil, fmt.Errorf("check %q has minFiles %d; must be >= 1", c.Name, c.MinFiles)
		}

		if c.InvertExit {
			// An inverted check proves its work through requireFiles/minFiles (the
			// files were there and the pattern was absent). It must NOT also carry
			// requireOutputMatch: a passing inverted check produces no output.
			if c.RequireOutputMatch != "" {
				return nil, fmt.Errorf("check %q sets both invertExit and requireOutputMatch; "+
					"an inverted check passes with EMPTY output, so the match could never hold", c.Name)
			}
			continue
		}

		// A non-inverted check passes on exit 0, which many tools return after
		// examining nothing. requireOutputMatch is the positive assertion that
		// turns "the tool did not complain" into "the tool reported work".
		if c.RequireOutputMatch == "" {
			return nil, fmt.Errorf("check %q declares no requireOutputMatch: a zero exit "+
				"alone cannot distinguish 'validated the tree' from 'found nothing to validate'", c.Name)
		}
		re, rerr := regexp.Compile(c.RequireOutputMatch)
		if rerr != nil {
			return nil, fmt.Errorf("check %q has an invalid requireOutputMatch %q: %w",
				c.Name, c.RequireOutputMatch, rerr)
		}
		c.compiledOutputMatch = re
	}
	return &cl, nil
}

// countTreeFiles returns the number of regular files under path (path itself counts
// as 1 when it is a regular file). A path that does not exist is an error --
// distinguishing "renamed away" from "present but empty" matters to the caller's
// message, not to the verdict: both are unverifiable.
func countTreeFiles(path string) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		if info.Mode().IsRegular() {
			return 1, nil
		}
		return 0, nil
	}
	n := 0
	werr := filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// .git is fetch machinery, not tree content. Counting it would let a
		// checkdef rooted at "." meet its minFiles floor on git objects alone,
		// while the working tree it means to examine is empty.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			n++
		}
		return nil
	})
	if werr != nil {
		return 0, werr
	}
	return n, nil
}

// --- Advisory resolution ---

// advisoryStatesWithFork is the set of advisory states that CAN carry a temporary
// private fork -- the rule is to refuse when the advisory is in a state that should
// not carry a fork.
//
// The TPF exists for exactly the pre-disclosure window. While an advisory is in draft it
// can report a live private fork (`{"state":"draft","private_fork":"example-org/example-k8s-…"}`);
// re-queried after publication the SAME advisory reports
// `{"state":"published","private_fork":null}` -- the fork is deleted when the
// advisory publishes. So `published` and `closed` are precisely the states in which
// this tool has nothing to fetch, and `triage`/`draft` are the only states it can
// ever operate on. An earlier revision of this gate had the condition the other way
// round and refused every advisory the tool exists to check.
var advisoryStatesWithFork = map[string]bool{
	"triage": true,
	"draft":  true,
}

func sortedStatesWithFork() []string {
	out := make([]string, 0, len(advisoryStatesWithFork))
	for s := range advisoryStatesWithFork {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// resolveFork resolves the TPF slug from the base repo's advisory endpoint.
func resolveFork(baseRepo, ghsaID string) (string, error) {
	path := "repos/" + baseRepo + "/security-advisories/" + ghsaID
	body, err := ghAPI(path)
	if err != nil {
		if is404(err) {
			return "", fmt.Errorf("advisory could not be resolved or access was denied")
		}
		return "", fmt.Errorf("cannot fetch advisory: %w", err)
	}

	var adv advisoryResponse
	if jerr := json.Unmarshal(body, &adv); jerr != nil {
		return "", fmt.Errorf("cannot parse advisory response: %w", jerr)
	}

	if adv.State != "" && !advisoryStatesWithFork[adv.State] {
		return "", fmt.Errorf("advisory %s is in state %q; a temporary private fork exists only "+
			"while an advisory is %s -- there is nothing to check at this state",
			ghsaID, adv.State, strings.Join(sortedStatesWithFork(), " or "))
	}

	if adv.PrivateFork == nil || adv.PrivateFork.FullName == "" {
		return "", fmt.Errorf("advisory %s has no private fork", ghsaID)
	}

	return adv.PrivateFork.FullName, nil
}

func is404(err error) bool {
	return err != nil && strings.Contains(err.Error(), "returned 404")
}

// resolveHeadSHA returns the head SHA of the TPF's default branch, confirming the
// repo reports private:true.
func resolveHeadSHA(tpfSlug string) (string, error) {
	body, err := ghAPI("repos/" + tpfSlug)
	if err != nil {
		return "", fmt.Errorf("cannot fetch fork repo info: %w", err)
	}
	var repo repoResponse
	if jerr := json.Unmarshal(body, &repo); jerr != nil {
		return "", fmt.Errorf("cannot parse fork repo info: %w", jerr)
	}
	if !repo.Private {
		return "", fmt.Errorf("fork %s is not private", tpfSlug)
	}

	branchBody, berr := ghAPI("repos/" + tpfSlug + "/branches/" + repo.DefaultBranch)
	if berr != nil {
		return "", fmt.Errorf("cannot resolve fork head SHA: %w", berr)
	}
	var br branchResponse
	if jerr := json.Unmarshal(branchBody, &br); jerr != nil {
		return "", fmt.Errorf("cannot parse fork branch info: %w", jerr)
	}
	if br.Commit.SHA == "" {
		return "", fmt.Errorf("fork %s has no commit SHA on branch %s", tpfSlug, repo.DefaultBranch)
	}

	return br.Commit.SHA, nil
}

// --- Tree fetch ---

// fetchAdvisoryTree fetches the TPF at the given SHA into a temporary directory.
// Uses deskgit-grade hardening pins and an ephemeral GIT_ASKPASS credential.
// The askpass script is written to a sibling temp dir (not inside the fetched tree)
// and cleaned up before this function returns.
func fetchAdvisoryTree(tpfSlug, sha string) (string, error) {
	token, terr := ghToken()
	if terr != nil {
		return "", fmt.Errorf("cannot resolve credential for fetch: %w", terr)
	}

	dir, derr := os.MkdirTemp("", "deskadvisory-*")
	if derr != nil {
		return "", fmt.Errorf("cannot create temp directory: %w", derr)
	}

	// Write askpass to a sibling temp dir to keep the fetched tree clean.
	askpassDir, aderr := os.MkdirTemp("", "deskadvisory-askpass-*")
	if aderr != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("cannot create askpass temp dir: %w", aderr)
	}
	defer os.RemoveAll(askpassDir)

	askpass, aerr := writeAskpass(askpassDir)
	if aerr != nil {
		os.RemoveAll(dir)
		return "", aerr
	}

	env := scrubbedEnv(os.Environ())
	env = append(env, "GIT_ASKPASS="+askpass)
	env = append(env, "DESKADVISORY_TOKEN="+token)

	if _, ierr := runGit(dir, env, "init", "-b", "main"); ierr != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("cannot init temp repo: %w", ierr)
	}

	fetchURL := "https://github.com/" + tpfSlug + ".git"
	args := append([]string{"-c", "credential.helper=", "fetch"}, fetchHardening...)
	args = append(args, fetchURL, "+refs/heads/*:refs/remotes/tpf/*")

	if _, ferr := runGit(dir, env, args...); ferr != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("cannot fetch from fork: %w", ferr)
	}

	if _, cerr := runGit(dir, env, "checkout", sha); cerr != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("cannot checkout SHA %s: %w", sha, cerr)
	}

	return dir, nil
}

// writeAskpass creates a controlled GIT_ASKPASS script that reads the token from
// DESKADVISORY_TOKEN.
func writeAskpass(dir string) (string, error) {
	path := filepath.Join(dir, "askpass.sh")
	script := "#!/bin/sh\ncase \"$1\" in\n  *Username*) echo \"x-access-token\" ;;\n  *) echo \"$DESKADVISORY_TOKEN\" ;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return "", fmt.Errorf("cannot write askpass script: %w", err)
	}
	return path, nil
}

// --- Check runner ---

// coverage records what a run of the check list actually examined, so the verdict
// line can state it rather than leaving the reader to assume "PASS" means "looked at
// everything".
type coverage struct {
	Checks int
	Files  int
	// PerCheck is "<name>: <n> file(s)" in check order.
	PerCheck []string
}

func (c coverage) String() string {
	return fmt.Sprintf("%d check(s), %d file(s) examined [%s]",
		c.Checks, c.Files, strings.Join(c.PerCheck, "; "))
}

// exitCodeOf returns the process exit status of a command error, or -1 when the
// error is not an exit status at all (tool could not be started, killed by a signal).
func exitCodeOf(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// runChecks executes each check in the check list against the fetched tree.
//
// Its whole job is to turn tool exit codes into a verdict, so every way a tool can
// finish is classified explicitly and the default is refusal:
//
//   - required paths absent, or present but holding fewer than minFiles files ->
//     unverifiable. This is the vacuous-pass guard: `grep` over an empty directory
//     exits 1 ("no match") and `kubeconform` over one exits 0 ("0 resource found"),
//     so without a file count both report success having read nothing.
//   - invertExit: exit 1 -> pass; exit 0 -> the forbidden pattern was FOUND, fail;
//     exit >= 2 (or a signal) -> the guard malfunctioned, unverifiable.
//   - plain check: exit 0 AND output matching requireOutputMatch -> pass. Exit 0 with
//     non-matching output means the tool examined nothing -> unverifiable. Non-zero
//     -> fail.
func runChecks(cl *checkList, treeDir string) (coverage, error) {
	env := scrubbedEnv(os.Environ())
	cov := coverage{}

	for _, c := range cl.Checks {
		// Every listed path must exist AND hold something. A stale path (renamed
		// dir, typo) and a path that survived but was emptied both become
		// ExitUnverifiable before the tool is invoked.
		files := 0
		for _, p := range c.RequireFiles {
			n, err := countTreeFiles(filepath.Join(treeDir, p))
			if err != nil {
				return cov, fmt.Errorf("check %q: required path %q not usable in fetched tree: %w",
					c.Name, p, err)
			}
			files += n
		}
		if files < c.MinFiles {
			return cov, fmt.Errorf("check %q: required paths %v hold %d file(s), need at least %d -- "+
				"the check would have reported success having examined nothing",
				c.Name, c.RequireFiles, files, c.MinFiles)
		}

		toolPath, lookErr := exec.LookPath(c.Tool)
		if lookErr != nil {
			return cov, fmt.Errorf("check %q: tool %q not found on PATH (run only tools already on PATH; "+
				"never execute scripts from the fetched tree)", c.Name, c.Tool)
		}

		cmd := execCommand(toolPath, c.Args...)
		cmd.Dir = treeDir
		cmd.Env = env
		var outb, errb bytes.Buffer
		cmd.Stdout = &outb
		cmd.Stderr = &errb

		runErr := cmd.Run()
		fmt.Fprintf(os.Stderr, "check %q (%s) over %d file(s):\n",
			c.Name, strings.Join(append([]string{c.Tool}, c.Args...), " "), files)
		if outb.Len() > 0 {
			fmt.Fprint(os.Stderr, outb.String())
		}
		if errb.Len() > 0 {
			fmt.Fprint(os.Stderr, errb.String())
		}

		code := 0
		if runErr != nil {
			code = exitCodeOf(runErr)
		}

		if c.InvertExit {
			switch code {
			case 1:
				// grep's "no match": the forbidden pattern is absent. Pass.
			case 0:
				return cov, fmt.Errorf("check %q failed: forbidden pattern FOUND in the tree "+
					"(tool exited 0 under invertExit)", c.Name)
			default:
				// Exit >= 2 from grep is a tool error -- a bad regex, an unreadable
				// file, a locale fault. Treating it as "no match" would let a guard
				// that never ran read as a clean tree, which is the single most
				// dangerous outcome for a tool whose job is the last check before an
				// unstoppable merge.
				return cov, fmt.Errorf("check %q could not be run: tool exited %d, which is neither "+
					"'pattern found' (0) nor 'pattern absent' (1) -- the guard malfunctioned "+
					"and its result says nothing about the tree: %w", c.Name, code, runErr)
			}
		} else {
			if runErr != nil {
				return cov, fmt.Errorf("check %q failed: %w", c.Name, runErr)
			}
			combined := outb.String() + errb.String()
			if !c.compiledOutputMatch.MatchString(combined) {
				return cov, fmt.Errorf("check %q could not be run: tool exited 0 but its output does "+
					"not match requireOutputMatch %q, so nothing establishes that it examined the "+
					"tree rather than finding nothing to examine", c.Name, c.RequireOutputMatch)
			}
		}

		cov.Checks++
		cov.Files += files
		cov.PerCheck = append(cov.PerCheck, fmt.Sprintf("%s: %d file(s)", c.Name, files))
	}

	if cov.Checks == 0 {
		return cov, fmt.Errorf("check list ran zero checks")
	}
	return cov, nil
}

// --- Main pipeline ---

// checkAdvisory runs the full check pipeline for a base repo and GHSA advisory ID.
func checkAdvisory(baseRepo, ghsaID string) error {
	// Step 1: Admit the base.
	if !deskkit.IsAllowedRepo(baseRepo) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s is not in the desk-tools repo set", baseRepo))
	}

	// Step 2: Resolve the fork.
	tpfSlug, ferr := resolveFork(baseRepo, ghsaID)
	if ferr != nil {
		return deskkit.Refused(ferr.Error())
	}

	// Step 3: Resolve the fork's head SHA, confirm private.
	sha, serr := resolveHeadSHA(tpfSlug)
	if serr != nil {
		return deskkit.Unverifiable("cannot resolve fork head", serr)
	}

	// Load the check list BEFORE fetching. It is ordered ahead of the fetch on
	// purpose: a missing, malformed or structurally invalid checkdef is knowable
	// without touching the tree, and there is no reason to clone an EMBARGOED
	// pre-disclosure tree to /tmp only to discover there was never anything to run
	// against it. The source is unchanged — the base repo's default branch, else this
	// binary's embedded copy; never the fetched tree.
	cl, lerr := loadCheckList(baseRepo)
	if lerr != nil {
		return deskkit.Unverifiable("cannot load check list", lerr)
	}

	// Step 4: Fetch the tree with hardening pins.
	treeDir, terr := fetchAdvisoryTree(tpfSlug, sha)
	if terr != nil {
		return deskkit.Unverifiable("cannot fetch advisory tree", terr)
	}
	defer func() {
		if rerr := os.RemoveAll(treeDir); rerr != nil {
			fmt.Fprintf(os.Stderr, "deskadvisory: warning: cannot clean up temp dir %s: %v\n", treeDir, rerr)
		}
	}()

	// Step 5: Run the check list.
	cov, cerr := runChecks(cl, treeDir)
	if cerr != nil {
		fmt.Fprintf(os.Stderr, "deskadvisory: FAIL %s @ %s\n", ghsaID, sha)
		return deskkit.Unverifiable("checks failed", cerr)
	}

	// Step 6: Report pass, WITH what was examined. A bare "PASS" is read as "this
	// advisory fix is verified"; the coverage figure is what lets the human see the
	// difference between that and "three checks reported no complaint".
	fmt.Printf("deskadvisory: PASS %s @ %s -- %s\n", ghsaID, sha, cov.String())
	return nil
}
