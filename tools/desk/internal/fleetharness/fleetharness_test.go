package fleetharness

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// The three-state classification the harness scores every scenario against.
//
// A pass/fail two-state cannot express what this harness observes, and the stream's
// shared convention says so: checked-clean / checked-failed / could-not-check. Here the
// distinction that matters is between a verb that REFUSED on a precondition and a verb
// that got all the way through its LOCAL preconditions and stopped only at the first
// step that needs the network. The second is the success state for an offline acceptance
// test — it is exactly "everything this machine can decide, this verb decided yes".
// ---------------------------------------------------------------------------

const (
	// stageOK — the verb completed (exit 0).
	stageOK = "ok"
	// stageReachedRemote — every LOCAL precondition passed and the verb stopped at the
	// public-repo gate, the first step that needs GitHub. In this harness the network is
	// deliberately unreachable, so this is the furthest an offline run can get, and it is
	// the state a verb usable from the mandated workflow must reach.
	stageReachedRemote = "reached-remote"
	// stageRefused — exit 5, a compiled-in constraint said no.
	stageRefused = "refused"
	// stageUnverifiable — exit 6 for any reason other than the remote being unreachable.
	stageUnverifiable = "unverifiable"
	// stageDisabled / stageRateLimited — exits 3 and 4.
	stageDisabled    = "disabled"
	stageRateLimited = "rate-limited"
	// stageCouldNotCheck — the harness itself could not run the scenario. Never green.
	stageCouldNotCheck = "could-not-check"
)

// remoteUnreachableMarker is the public-repo gate's own wording when it cannot read
// visibility. Keying on the gate's message is what lets the harness tell "stopped because
// the network is off" from "stopped because a precondition refused" without a live remote.
const remoteUnreachableMarker = "cannot determine repo visibility"

// expectation is one row of testdata/fleet-workflow-expectations.json.
type expectation struct {
	Stage string `json:"stage"`
	Issue string `json:"issue,omitempty"`
	Note  string `json:"note"`
}

// TestFleetWorkflowAcceptance is the acceptance criterion the fleet audit asked for, in
// its own words: "a detached worktree at origin/<branch>, in a repo different from the
// caller's own, replying successfully. If that case does not pass, the tool remains
// unusable for the roles it was built for."
//
// It is a DIFFERENTIAL, not a pass/fail assertion, and that is deliberate. The gated verbs
// do not all pass today — some of these gaps are open, and their fixes ride their own
// issues, not this harness. A harness that simply asserted "all green" could only be landed by
// deleting the scenarios that fail, which is how the class stayed invisible for fourteen
// passes. Instead every scenario's observed stage is compared against a CHECKED-IN
// expectation that names the issue when it is not a pass, so:
//
//   - a REGRESSION (a scenario that passed and now refuses) reddens;
//   - a FIX (a scenario that refused and now passes) also reddens, telling the author to
//     retire the expectation and close the issue — an improvement nobody records is an
//     improvement the next author has to rediscover;
//   - the expectations file is a standing, machine-readable statement of which gated verbs
//     the fleet's own mandated workflow cannot currently use.
//
// Release-blocking for the desk-tools ship bar.
func TestFleetWorkflowAcceptance(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is not on PATH: %v — this is could-not-check, not a pass", err)
	}
	env := newFleetEnv(t)
	expected := loadExpectations(t)

	observed := map[string]string{}
	for _, sc := range scenarios(env) {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			stage, detail := sc.run(t, env)
			observed[sc.name] = stage
			exp, ok := expected[sc.name]
			if !ok {
				t.Fatalf("scenario %q has no row in %s — every gated-verb scenario must "+
					"carry a recorded expectation, or a silent behaviour change has nowhere "+
					"to show up.\n  observed: %s\n  detail:   %s",
					sc.name, expectationsFile, stage, detail)
			}
			if stage == exp.Stage {
				t.Logf("%s = %s (as recorded%s)\n  %s", sc.name, stage, issueSuffix(exp), detail)
				return
			}
			t.Errorf("MANDATED-WORKFLOW DRIFT on %q\n"+
				"  expected: %s%s\n  observed: %s\n  detail:   %s\n  note:     %s\n\n"+
				"  If the verb IMPROVED, update %s and close the issue it names.\n"+
				"  If it REGRESSED, a gated verb just became unusable from the workflow the\n"+
				"  skills mandate — which means its guard stops running for every worker that\n"+
				"  takes the authorised exit-3/6 fallback.",
				sc.name, exp.Stage, issueSuffix(exp), stage, detail, exp.Note, expectationsFile)
		})
	}

	// A scenario deleted from the code but left in the file is the same defect as one
	// added without a row: the file stops describing the harness.
	for name := range expected {
		if _, ran := observed[name]; !ran {
			t.Errorf("%s carries a row for %q, but no scenario by that name ran — a "+
				"recorded expectation with nothing behind it is a green light for a case "+
				"nobody is checking", expectationsFile, name)
		}
	}
}

func issueSuffix(e expectation) string {
	if e.Issue == "" {
		return ""
	}
	return " [" + e.Issue + "]"
}

const expectationsFile = "testdata/fleet-workflow-expectations.json"

func loadExpectations(t *testing.T) map[string]expectation {
	t.Helper()
	raw, err := os.ReadFile(expectationsFile)
	if err != nil {
		t.Fatalf("cannot read %s: %v — could-not-check, never green", expectationsFile, err)
	}
	var m map[string]expectation
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("cannot parse %s: %v", expectationsFile, err)
	}
	if len(m) == 0 {
		t.Fatalf("%s is empty", expectationsFile)
	}
	return m
}

// ---------------------------------------------------------------------------
// The fixture: two origins, a worker clone, a detached worktree, PATH shims.
// ---------------------------------------------------------------------------

type fleetEnv struct {
	bin        string // dir holding the built verbs + the gh/desktoken shims
	home       string // private HOME (roster, audit log)
	ownRepo    string // owner/name of the worker's own origin
	otherRepo  string // owner/name of a DIFFERENT repo in the allowed set
	detached   string // worktree created with --detach from refs/remotes/origin/<branch>
	onBranch   string // worktree with the feature branch checked out (the non-mandated shape)
	branch     string
	bodyClean  string // a body file that scans clean (and carries the link trailer)
	bodyNoTrailer string // a clean body WITHOUT the link trailer (example-stream/02 rows)
	bodySecret string // a body file carrying a synthetic credential (plus the trailer)
}

// fixtureRoster mirrors cmd/deskpr's own test roster: the tools read their allowed-repo
// set and trust roster from the config home, so a harness that does not plant one measures
// nothing but "unconfigured, refuse" on every scenario.
const fixtureRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=worker=assay-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private,example-org/agents:ci:private
ASSAY_HUMAN_LOGIN_MAP=alex:ada
`

func newFleetEnv(t *testing.T) *fleetEnv {
	t.Helper()
	root := t.TempDir()
	e := &fleetEnv{
		bin:       filepath.Join(root, "bin"),
		home:      filepath.Join(root, "home"),
		ownRepo:   "example-org/tracker",
		otherRepo: "example-org/agents",
		branch:    "brief/fleet-acceptance",
	}
	mkdir(t, e.bin)
	mkdir(t, filepath.Join(e.home, ".config", "assay"))
	write(t, filepath.Join(e.home, ".config", "assay", "roster.env"), fixtureRoster)

	buildVerb(t, e.bin, "deskpr")
	buildVerb(t, e.bin, "deskreply")
	buildShim(t, e.bin, "gh", ghShimSource)
	buildShim(t, e.bin, "desktoken", desktokenShimSource)

	// Two origins, laid out so that `remote.origin.url`'s last two path components parse
	// to the owner/name slugs above — the tools derive the repo from the remote URL, so a
	// fixture whose remote does not parse to an allowed repo tests the allowlist and
	// nothing else.
	origins := filepath.Join(root, "origins", "example-org")
	mkdir(t, origins)
	ownOrigin := seedOrigin(t, filepath.Join(origins, "tracker"), e.branch)
	seedOrigin(t, filepath.Join(origins, "agents"), e.branch)

	// The worker clone, and THE MANDATED SHAPE on top of it: a worktree added with
	// --detach from the remote-tracking ref, never a named local branch. The skills
	// require this because a branch can only be checked out in one worktree at a time, so
	// a stale sibling worktree holding the branch makes a named checkout impossible
	// without destroying another session's state.
	clone := filepath.Join(root, "clone")
	run(t, root, "git", "clone", ownOrigin, clone)
	run(t, clone, "git", "remote", "set-head", "origin", "--auto")
	e.detached = filepath.Join(root, "wt-detached")
	run(t, clone, "git", "worktree", "add", e.detached, "refs/remotes/origin/"+e.branch, "--detach")

	// The shape the verbs were WRITTEN for, kept as the harness's own control: if this
	// one does not reach the remote either, the harness is broken and its verdict on the
	// mandated shape means nothing.
	e.onBranch = filepath.Join(root, "wt-branch")
	run(t, clone, "git", "worktree", "add", e.onBranch, "-b", e.branch+"-local", "refs/remotes/origin/"+e.branch)

	e.bodyClean = filepath.Join(root, "body-clean.md")
	write(t, e.bodyClean, "## Summary\n\nA clean body citing tools/desk/internal/deskkit/bodycheck.go:45.\n\nBrief: fixture/01\n")
	// example-stream/02: a clean body WITHOUT the trailer, for the new link-gate rows.
	e.bodyNoTrailer = filepath.Join(root, "body-no-trailer.md")
	write(t, e.bodyNoTrailer, "## Summary\n\nA clean body with no link trailer.\n")
	// A synthetic credential, split so this SOURCE file does not carry a contiguous one
	// (the scan reads added diff lines; see deskkit's scanSecret40). Carries the trailer
	// too, so the logged-override scenario passes the link gate after passing the scan.
	e.bodySecret = filepath.Join(root, "body-secret.md")
	write(t, e.bodySecret, "## Summary\n\ntoken ghp"+"_0123456789ABCDEF"+"abcdef0123456789ABCD\n\nBrief: fixture/01\n")
	return e
}

// seedOrigin creates a bare repo at path with a main branch and a feature branch one
// commit ahead, and returns the path. Bare, because a worker clones an origin; the feature
// branch exists on the remote already, because the mandated worktree is created from
// refs/remotes/origin/<branch>.
func seedOrigin(t *testing.T, path, branch string) string {
	t.Helper()
	mkdir(t, path)
	run(t, path, "git", "init", "--bare", "--initial-branch=main", ".")
	seed := t.TempDir()
	run(t, seed, "git", "init", "--initial-branch=main", ".")
	run(t, seed, "git", "config", "user.email", "harness@example.invalid")
	run(t, seed, "git", "config", "user.name", "fleet harness")
	write(t, filepath.Join(seed, "README.md"), "seed\n")
	// example-stream/02: a real brief so `Brief: fixture/01` trailers in the harness
	// bodies resolve under --root in every fixture worktree.
	mkdir(t, filepath.Join(seed, "docs", "streams", "fixture"))
	write(t, filepath.Join(seed, "docs", "streams", "fixture", "brief-01-test.md"),
		"---\nschema: brief-v1\nbrief: fixture/01\ntitle: fixture brief\n---\n\nFixture brief.\n")
	run(t, seed, "git", "add", "-A")
	run(t, seed, "git", "commit", "-m", "seed")
	run(t, seed, "git", "checkout", "-b", branch)
	write(t, filepath.Join(seed, "CHANGE.md"), "the branch's own commit\n")
	run(t, seed, "git", "add", "-A")
	run(t, seed, "git", "commit", "-m", "brief work")
	run(t, seed, "git", "remote", "add", "origin", path)
	run(t, seed, "git", "push", "origin", "main", branch)
	run(t, path, "git", "symbolic-ref", "HEAD", "refs/heads/main")
	return path
}

// ---------------------------------------------------------------------------
// Scenarios.
// ---------------------------------------------------------------------------

type scenario struct {
	name string
	run  func(t *testing.T, e *fleetEnv) (stage, detail string)
}

func scenarios(e *fleetEnv) []scenario {
	return []scenario{
		{
			// The acceptance sentence, first half.
			name: "deskreply/detached-worktree-own-repo",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				return e.invoke(t, e.detached, "deskreply", e.ownRepo, "1", "--body-file", e.bodyClean)
			},
		},
		{
			// The acceptance sentence, second half: a repo different from the
			// caller's own. Cross-repo dispatch is normal for this fleet.
			name: "deskreply/detached-worktree-cross-repo",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				return e.invoke(t, e.detached, "deskreply", e.otherRepo, "1", "--body-file", e.bodyClean)
			},
		},
		{
			// The harness's own control for deskreply: the shape the verb was written for.
			name: "deskreply/branch-checked-out-own-repo",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				return e.invoke(t, e.onBranch, "deskreply", e.ownRepo, "1", "--body-file", e.bodyClean)
			},
		},
		{
			name: "deskpr-create/detached-worktree",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				return e.invoke(t, e.detached, "deskpr", "create", "--title", "fleet acceptance", "--body-file", e.bodyClean)
			},
		},
		{
			// example-stream/02: the link gate is the FIRST local precondition — a
			// trailer-less body refuses at exit 5 before getwd/preflight and before any
			// network call, in every worktree shape.
			name: "deskpr-create/no-trailer",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				return e.invoke(t, e.onBranch, "deskpr", "create", "--title", "fleet acceptance", "--body-file", e.bodyNoTrailer)
			},
		},
		{
			// example-stream/02: update reads the PR's body from GitHub; a body without
			// the trailer refuses at exit 5 with the line to add (the migration-window
			// path for pre-trailer PRs).
			name: "deskpr-update/no-trailer-body",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				t.Setenv("FAKEGH_LIST_HAS_PR", "1")
				t.Setenv("FAKEGH_HEAD_REF", e.branch+"-local")
				t.Setenv("FAKEGH_PR_BODY", "no trailer here\n")
				return e.invoke(t, e.onBranch, "deskpr", "update")
			},
		},
		{
			// The harness's control for deskpr: local preconditions all satisfied.
			name: "deskpr-create/branch-checked-out",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				return e.invoke(t, e.onBranch, "deskpr", "create", "--title", "fleet acceptance", "--body-file", e.bodyClean)
			},
		},
		{
			name: "deskpr-update/detached-worktree",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				return e.invoke(t, e.detached, "deskpr", "update")
			},
		},
		{
			// An approved, ready-flipped PR gone stale has no
			// sanctioned push path.
			name: "deskpr-update/ready-flipped-pr",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				t.Setenv("FAKEGH_LIST_HAS_PR", "1")
				t.Setenv("FAKEGH_LIST_DRAFT", "0")
				t.Setenv("FAKEGH_HEAD_REF", e.branch+"-local")
				return e.invoke(t, e.onBranch, "deskpr", "update")
			},
		},
		{
			// The scan refusal itself, through a real binary: exit 5, and the message
			// must advertise the audited override.
			name: "deskpr-create/secret-body-no-override",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				stage, detail := e.invoke(t, e.onBranch, "deskpr", "create",
					"--title", "fleet acceptance", "--body-file", e.bodySecret)
				if !strings.Contains(detail, "force-scan-override") {
					t.Errorf("the scan refusal does not advertise the audited override — "+
						"a worker left the sanctioned transport precisely "+
						"because the refusal said none existed:\n%s", detail)
				}
				return stage, detail
			},
		},
		{
			// The override end to end: the write proceeds AND an audit row lands. Both
			// halves are asserted, because an override that does not log is the unlogged
			// bypass this brief exists to eliminate.
			name: "deskpr-create/secret-body-logged-override",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				stage, detail := e.invoke(t, e.onBranch, "deskpr", "create",
					"--title", "fleet acceptance", "--body-file", e.bodySecret,
					"--force-scan-override", "acceptance-harness exercise of the audited bypass")
				if rows := e.overrideRows(t); rows == 0 {
					t.Errorf("the override proceeded with NO scan_override audit row — that is "+
						"an unlogged bypass, which is the outcome this flag exists to replace.\n%s", detail)
				}
				return stage, detail
			},
		},
		{
			// The cold-shell case: desktoken not resolvable on PATH.
			// It shares exit 6 with the detached-HEAD refusal, which is why the two causes
			// must be kept distinguishable — the harness records
			// them as separate scenarios so a conflated exit code is at least visible.
			name: "deskpr-create/desktoken-absent-from-path",
			run: func(t *testing.T, e *fleetEnv) (string, string) {
				return e.invokeWithoutShim(t, e.onBranch, "desktoken", "deskpr", "create",
					"--title", "fleet acceptance", "--body-file", e.bodyClean)
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Invocation + classification.
// ---------------------------------------------------------------------------

func (e *fleetEnv) invoke(t *testing.T, dir, verb string, args ...string) (string, string) {
	return e.exec(t, dir, e.bin, verb, args...)
}

// invokeWithoutShim removes ONE shim from the PATH dir for the duration of the call, which
// is how a cold worker shell that never prepended /opt/desk-tools/bin actually looks.
func (e *fleetEnv) invokeWithoutShim(t *testing.T, dir, shim, verb string, args ...string) (string, string) {
	t.Helper()
	partial := t.TempDir()
	entries, err := os.ReadDir(e.bin)
	if err != nil {
		t.Fatalf("cannot read the harness bin dir: %v", err)
	}
	for _, en := range entries {
		if en.Name() == shim {
			continue
		}
		linkOrCopy(t, filepath.Join(e.bin, en.Name()), filepath.Join(partial, en.Name()))
	}
	return e.exec(t, dir, partial, verb, args...)
}

func (e *fleetEnv) exec(t *testing.T, dir, pathDir, verb string, args ...string) (string, string) {
	t.Helper()
	cmd := exec.Command(filepath.Join(e.bin, verb), args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"PATH="+pathDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HOME="+e.home,
		"XDG_CONFIG_HOME="+filepath.Join(e.home, ".config"),
		"CLAUDE_SESSION_ID=fleet-harness",
		"DESK_TOOLS_DISABLED=",
		// The workflow these scenarios measure is a DISPATCHED WORKER's, and such a
		// session is booted — it presents a loop identity, which is what makes a
		// STOP.<loop> flag able to halt it. Every outward verb now refuses without one, so
		// a harness that presented none would stop at that gate and every row below would
		// measure the gate instead of the shape it is about. The refusal itself is pinned
		// per verb in each verb's own package.
		"DESK_LOOP=worker-desk",
		"GIT_CONFIG_GLOBAL="+filepath.Join(e.home, "gitconfig"),
		// The network is deliberately unreachable, so the public-repo gate — the first
		// step that needs GitHub — fails fast and marks the boundary between "the local
		// preconditions all passed" and everything before it. A harness that reached the
		// real API would be neither hermetic nor safe to run in CI.
		"HTTPS_PROXY=http://127.0.0.1:1",
		"HTTP_PROXY=http://127.0.0.1:1",
		"NO_PROXY=",
		"FAKEGH_LIST_HAS_PR="+os.Getenv("FAKEGH_LIST_HAS_PR"),
		"FAKEGH_LIST_DRAFT="+os.Getenv("FAKEGH_LIST_DRAFT"),
		"FAKEGH_HEAD_REF="+os.Getenv("FAKEGH_HEAD_REF"),
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if ok := asExitError(err, &ee); ok {
			code = ee.ExitCode()
		} else {
			return stageCouldNotCheck, fmt.Sprintf("could not run %s: %v", verb, err)
		}
	}
	text := strings.TrimSpace(string(out))
	return classify(code, text), fmt.Sprintf("exit=%d\n%s", code, indent(salient(text)))
}

// classify maps an exit code plus the tool's own output onto the harness's states. The
// message is consulted for exactly one distinction — remote-unreachable versus any other
// unverifiable — because that is the boundary the harness measures and the exit code
// alone conflates it: the two rc=6 causes must be distinguished so the telemetry is
// readable.
func classify(code int, out string) string {
	switch code {
	case 0:
		return stageOK
	case 3:
		return stageDisabled
	case 4:
		return stageRateLimited
	case 5:
		return stageRefused
	case 6:
		if strings.Contains(out, remoteUnreachableMarker) {
			return stageReachedRemote
		}
		return stageUnverifiable
	default:
		return stageCouldNotCheck
	}
}

// overrideRows counts scan_override rows in the harness HOME's audit log.
func (e *fleetEnv) overrideRows(t *testing.T) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(e.home, ".config", "assay", "audit.jsonl"))
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, `"verb":"scan_override"`) {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Build helpers and PATH shims.
// ---------------------------------------------------------------------------

func buildVerb(t *testing.T, bin, name string) {
	t.Helper()
	// The module root is two levels up from internal/fleetharness.
	run(t, filepath.Join("..", ".."), "go", "build", "-o", filepath.Join(bin, name), "./cmd/"+name)
}

func buildShim(t *testing.T, bin, name, src string) {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.go"), src)
	write(t, filepath.Join(dir, "go.mod"), "module shim\n\ngo 1.22\n")
	run(t, dir, "go", "build", "-o", filepath.Join(bin, name), ".")
}

// ghShimSource stands in for the GitHub CLI. It answers only the reads the gated verbs
// perform BEFORE the public-repo gate — `pr list` for deskpr — and refuses anything else
// loudly, so a verb that reached further than the harness models cannot look like a pass.
const ghShimSource = `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	joined := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(joined, "pr list"):
		if os.Getenv("FAKEGH_LIST_HAS_PR") != "1" {
			fmt.Println("[]")
			return
		}
		draft := "true"
		if os.Getenv("FAKEGH_LIST_DRAFT") == "0" {
			draft = "false"
		}
		head := os.Getenv("FAKEGH_HEAD_REF")
		fmt.Printf("[{\"number\":7,\"url\":\"https://example.invalid/pull/7\",\"isDraft\":%s,\"headRefName\":%q}]\n", draft, head)
	case strings.HasPrefix(joined, "pr view") && strings.Contains(joined, "body"):
		// example-stream/02: deskpr update reads the PR body for the trailer check.
		// FAKEGH_PR_BODY overrides; the default carries a resolving trailer.
		b := os.Getenv("FAKEGH_PR_BODY")
		if b == "" {
			b = "Brief: fixture/01\n"
		}
		fmt.Printf("{\"body\":%q}\n", b)
	default:
		fmt.Fprintf(os.Stderr, "fleet-harness gh shim: unmodelled call %q — the verb got further than the harness models\n", joined)
		os.Exit(1)
	}
}
`

// desktokenShimSource stands in for the cold App-token mint: it writes a token file and
// prints its path, which is exactly the contract mintWorkerToken depends on.
const desktokenShimSource = `package main

import (
	"fmt"
	"os"
)

func main() {
	f, err := os.CreateTemp("", "fleet-harness-token-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprint(f, "fleet-harness-worker-token")
	f.Close()
	fmt.Println(f.Name())
}
`

// ---------------------------------------------------------------------------
// Small helpers.
// ---------------------------------------------------------------------------

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func write(t *testing.T, p, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func linkOrCopy(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Symlink(from, to); err == nil {
		return
	}
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatalf("copy %s: %v", from, err)
	}
	if err := os.WriteFile(to, b, 0o700); err != nil {
		t.Fatalf("copy to %s: %v", to, err)
	}
}

func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=fleet harness", "GIT_AUTHOR_EMAIL=harness@example.invalid",
		"GIT_COMMITTER_NAME=fleet harness", "GIT_COMMITTER_EMAIL=harness@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s (in %s): %v\n%s", name, strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// salient drops the per-run banners every desk tool emits — the effective-roster echo and
// the unpinned-build warning — so the recorded detail of a scenario is the tool's actual
// verdict rather than fourteen lines of configuration. The banners are deliberate
// behaviour, not noise, but they are the same on every scenario and drown the one line
// that differs.
func salient(s string) string {
	var kept []string
	for _, ln := range strings.Split(s, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "assay-config:") || strings.HasPrefix(t, "desk-tools WARNING:") {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

func indent(s string) string {
	if s == "" {
		return "    (no output)"
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = "    " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}
