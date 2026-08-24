package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// verdictrun.go — the DETERMINISTIC runner + batcher (brief 04). It pops the Awaiting queue,
// runs each brief's check/check:ci Verify rows locally (exit code = verdict), accumulates the
// row results into the verdict-v1 payload shape, and flushes one SIGNED payload per ~5-minute
// batch window (or when the queue drains). There is NO model in this hot path.
//
// Filing the resulting verifier-App-authored `verify-verdict` issue is the autonomous-drive
// cutover, which is gate: human — BLOCKED-ON-HUMAN. So this reference build STOPS at a signed
// would-be body:
//
//   - `--dry-run` composes + signs + prints the body without filing — the CI-testable
//     surface (brief 04 Task step 4). Rate gate at the real filing site is
//     deskkit.AllowVerdictIssueWrite (the verdict-issue bucket), wired at cutover.
//
// Fail-closed envelope (operating-envelope preflight pattern): a missing verifier PEM is
// reported loudly and NOTHING is filed — an unsigned verdict is never emitted.

// rowExec runs one Verify command from `root` and returns its exit code and combined output.
// It is the runner's single side-effecting primitive, injectable so the batch/payload logic
// is unit-testable without spawning shells.
type rowExec func(root, command string) (exit int, output string)

// shellExec is the default rowExec: run the command via `sh -c` from the repo root, exactly
// as a human running the Verify row would. The exit code is the verdict.
func shellExec(root, command string) (int, string) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return exitCodeOf(err), string(out)
}

// exitCodeOf extracts a process exit code from an exec error: 0 when nil, the real code for
// an *exec.ExitError, and 1 for any other failure (command not found, etc.) — a non-zero
// verdict either way, never a silent PASS.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return 1
}

// verdictRunConfig is the parsed invocation.
type verdictRunConfig struct {
	root    string
	repo    string // owner/name; derived from git origin when empty
	head    string // commit SHA the rows ran against; derived from git HEAD when empty
	runner  string // runner identity stamped into provenance (an App bot login)
	session string // runner session id
	pem     string // verifier private-key PEM override
	window  time.Duration
	dryRun  bool
	exec    rowExec // nil => shellExec
	now     func() time.Time
	out     io.Writer // nil => os.Stdout
}

func (c verdictRunConfig) execFn() rowExec {
	if c.exec != nil {
		return c.exec
	}
	return shellExec
}

func (c verdictRunConfig) nowFn() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c verdictRunConfig) emit() io.Writer {
	if c.out != nil {
		return c.out
	}
	return os.Stdout
}

// cmdVerdict parses the `verdict` subcommand flags and runs the deterministic runner.
func cmdVerdict(args []string) int {
	fs := flag.NewFlagSet("verdict", flag.ContinueOnError)
	root := fs.String("root", ".", "repo root to scan for the Awaiting queue")
	repo := fs.String("repo", "", "owner/name (default: derived from git origin)")
	sha := fs.String("sha", "", "commit SHA the rows ran against (default: git HEAD)")
	runner := fs.String("runner", "", "runner identity stamped into provenance")
	session := fs.String("session", "", "runner session id (default: CLAUDE_SESSION_ID)")
	pem := fs.String("pem", "", "verifier private-key PEM (default: VERIFIER_PEM, else <config-home>/verifier-app.pem)")
	window := fs.Duration("window", defaultBatchWindow, "batch flush window")
	dryRun := fs.Bool("dry-run", false, "compose + sign + print the would-be body without filing")
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitRefused
	}
	cfg := verdictRunConfig{
		root:    *root,
		repo:    *repo,
		head:    *sha,
		runner:  *runner,
		session: *session,
		pem:     *pem,
		window:  *window,
		dryRun:  *dryRun,
	}
	return deskkit.ExitCodeOf(runVerdict(cfg))
}

// runVerdict is the testable core: resolve the envelope (PEM), read the queue, run the
// rows, batch, sign, and print. It returns a *deskkit.DeskError on a fail-closed envelope /
// signing failure and nil on success.
func runVerdict(cfg verdictRunConfig) error {
	// Operating-envelope preflight: resolve the verifier PEM up front. A missing PEM is an
	// envelope error reported loudly — file nothing, sign nothing.
	pemPath, err := resolveVerifierPEMPath(cfg.pem)
	if err != nil {
		return err
	}

	repo := cfg.repo
	if repo == "" {
		repo = deriveRepo(cfg.root)
	}
	head := cfg.head
	if head == "" {
		head = deriveHead(cfg.root)
	}
	session := cfg.session
	if session == "" {
		session = deskkit.SessionTag()
	}
	runner := cfg.runner
	if runner == "" {
		runner = "verify-desk-engine"
	}
	meta := sessionMeta{ID: session, Runner: runner}

	items, err := scanAwaiting(cfg.root, head)
	if err != nil {
		return deskkit.Unverifiable("cannot read the Awaiting queue", err)
	}

	out := cfg.emit()
	execFn := cfg.execFn()
	window := cfg.window
	if window <= 0 {
		window = defaultBatchWindow
	}

	var b batch
	var flushed int
	flush := func() error {
		if len(b.rows) == 0 {
			return nil
		}
		n, err := emitBatch(out, repo, head, cfg.nowFn(), meta, b.rows, pemPath, cfg.dryRun, window)
		if err != nil {
			return err
		}
		flushed += n
		b.reset()
		return nil
	}

	for i, it := range items {
		rows := runBriefRows(cfg.root, it, execFn)
		for _, r := range rows {
			b.add(r, cfg.nowFn())
		}
		drained := i == len(items)-1
		if b.dueToFlush(cfg.nowFn(), window, drained) {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	// Drain-flush anything the loop left (e.g. an empty queue produced nothing).
	if err := flush(); err != nil {
		return err
	}

	if flushed == 0 {
		fmt.Fprintln(out, "verdict runner: no runner-executed (check/check:ci) rows in the Awaiting queue — nothing to sign")
	}
	return nil
}

// emitBatch composes, signs, and prints one batch's payload. It returns the number of rows
// in the payload. Filing is BLOCKED-ON-HUMAN, so both dry-run and default paths print the
// signed body rather than file it; the rate gate at the real filing site is
// deskkit.AllowVerdictIssueWrite, wired at cutover.
func emitBatch(out io.Writer, repo, head string, ts time.Time, meta sessionMeta, rows []rowResult, pemPath string, dryRun bool, window time.Duration) (int, error) {
	payload := composePayload(repo, head, ts, meta, rows)
	body, err := signPayload(payload, pemPath)
	if err != nil {
		return 0, deskkit.Unverifiable("cannot sign verdict payload (verify signing key envelope)", err)
	}
	briefs := map[string]bool{}
	for _, r := range rows {
		briefs[r.BriefPath] = true
	}
	fmt.Fprint(out, body)
	tail := "dry-run: not filed"
	if !dryRun {
		tail = "filing a verify-verdict issue is the autonomous cutover (gate: human, BLOCKED-ON-HUMAN) — not filed"
	}
	fmt.Fprintf(out, "\nsigned verdict for %d row(s) across %d brief(s) (window %s) — %s\n",
		len(rows), len(briefs), window, tail)
	return len(rows), nil
}

// runBriefRows reads the brief for an Awaiting item, parses its Verify table, and runs every
// runner-executed (check/check:ci) row from the repo root. gate:model / gate:human rows are
// skipped — they stay on their judgment lanes. A brief with no resolvable path yields nothing.
func runBriefRows(root string, it loopengine.Item, execFn rowExec) []rowResult {
	if strings.TrimSpace(it.BriefPath) == "" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(root, it.BriefPath))
	if err != nil {
		return nil
	}
	var results []rowResult
	for _, row := range parseVerifyRows(string(raw)) {
		if !row.runnerExecuted() {
			continue
		}
		exit, output := execFn(root, row.Command)
		results = append(results, rowResult{
			BriefPath: it.BriefPath,
			Row:       row.Num,
			Class:     row.Class,
			Command:   row.Command,
			Exit:      exit,
			Output:    output,
		})
	}
	return results
}

// resolveVerifierPEMPath resolves the verifier private-key PEM, honouring (in order) an
// explicit override, the VERIFIER_PEM env, and finally verifier-app.pem on the App-credential
// search path — the SAME resolution deskverdict/deskevidence use. It FAILS CLOSED, naming
// every directory searched, so a missing key is a loud envelope error and never a silent pass.
func resolveVerifierPEMPath(override string) (string, error) {
	if override != "" {
		return expandHomePath(override), nil
	}
	if v := strings.TrimSpace(os.Getenv("VERIFIER_PEM")); v != "" {
		return expandHomePath(v), nil
	}
	path, searched, found := deskkit.FindConfigFile("verifier-app.pem")
	if !found {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"verify signing envelope RED: cannot find the verifier private key — set VERIFIER_PEM=<file>, "+
				"or place verifier-app.pem in one of: %s. Nothing signed, nothing filed.",
			strings.Join(searched, ", ")), nil)
	}
	return expandHomePath(path), nil
}

// expandHomePath expands a leading "~/" to the user's home directory. No-op otherwise.
func expandHomePath(p string) string {
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

// deriveRepo returns owner/name from the git origin URL of root, or "" when it cannot be
// determined (the payload then carries an empty repo, which a consumer treats as a refuse).
func deriveRepo(root string) string {
	out, err := exec.Command("git", "-C", root, "config", "--get", "remote.origin.url").CombinedOutput()
	if err != nil {
		return ""
	}
	return repoFromRemote(strings.TrimSpace(string(out)))
}

// repoFromRemote extracts owner/name from an https or ssh git remote URL.
func repoFromRemote(url string) string {
	url = strings.TrimSuffix(url, ".git")
	if i := strings.Index(url, "github.com"); i >= 0 {
		rest := url[i+len("github.com"):]
		rest = strings.TrimLeft(rest, ":/")
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			return parts[len(parts)-2] + "/" + parts[len(parts)-1]
		}
	}
	return ""
}

// deriveHead returns the current git HEAD SHA of root, or "" when unavailable.
func deriveHead(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
