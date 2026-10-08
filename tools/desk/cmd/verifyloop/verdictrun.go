package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
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
// reported loudly and NOTHING is filed — an unsigned verdict BODY is never emitted; an unsigned
// PAYLOAD is written only to an explicit `--unsigned-out` file.
//
//   - `--unsigned-out <file>` (brief desk-tools/29) is the split path for a fenced runner: it
//     runs the same rows, composes ONE payload in the same canonical form, writes it to a NEW
//     file UNSIGNED and prints only its sha256. It never resolves, opens or reads a key — the
//     branch is taken before resolveVerifierPEMPath (runVerdictUnsigned, below).
//     The key-holding host signs it afterwards with `deskverdict sign --expect-sha256 …`.

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

	// unsignedOut, when set, selects the keyless compose-only mode (--unsigned-out): the
	// canonical payload is written to this NEW file unsigned and no key is ever resolved.
	unsignedOut string
	// writeHook writes the unsigned payload bytes to the file the composer just created.
	// nil => f.Write. Tests set it to fail the write after the create; production never does.
	writeHook func(f *os.File, b []byte) error
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
	cfg, err := parseVerdictFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	// Fail-closed: a red envelope (missing/unreadable PEM, an unreadable Awaiting queue, a
	// signing failure) is reported LOUDLY on stderr — never a silent non-zero exit. The
	// verdict lane's whole contract is "file nothing, report the envelope error loudly"
	// (brief 04 Context), so the CLI must surface it exactly as the `plan` path does.
	if err := runVerdict(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	return deskkit.ExitOK
}

// parseVerdictFlags is the flag-parsing seam for `verdict`: it returns the parsed config, or a
// refusal (exit 5) for a flag error or a combination that has no meaning. --unsigned-out never
// combines with --pem (a key path in a keyless run is a caller error, never silently ignored),
// --dry-run, or an EXPLICIT --window (the batching window does not apply to a run that composes
// one payload; a silent ignore would hide a caller who expected batching).
func parseVerdictFlags(args []string) (verdictRunConfig, error) {
	fs := flag.NewFlagSet("verdict", flag.ContinueOnError)
	root := fs.String("root", ".", "repo root to scan for the Awaiting queue")
	repo := fs.String("repo", "", "owner/name (default: derived from git origin)")
	sha := fs.String("sha", "", "commit SHA the rows ran against (default: git HEAD)")
	runner := fs.String("runner", "", "runner identity stamped into provenance")
	session := fs.String("session", "", "runner session id (default: CLAUDE_SESSION_ID)")
	pem := fs.String("pem", "", "verifier private-key PEM (default: VERIFIER_PEM, else <config-home>/verifier-app.pem)")
	window := fs.Duration("window", defaultBatchWindow, "batch flush window")
	dryRun := fs.Bool("dry-run", false, "compose + sign + print the would-be body without filing")
	unsignedOut := fs.String("unsigned-out", "", "write the canonical verdict-v1 payload to this new file UNSIGNED and print its sha256; resolves and reads no key; sign it on the key-holding host with deskverdict sign --expect-sha256")
	if err := fs.Parse(args); err != nil {
		return verdictRunConfig{}, deskkit.RefusedWithCause("verifyloop verdict: bad flags", err)
	}
	if *unsignedOut != "" {
		passed := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { passed[f.Name] = true })
		for _, bad := range []string{"pem", "dry-run", "window"} {
			if passed[bad] {
				return verdictRunConfig{}, deskkit.Refused(fmt.Sprintf(
					"verifyloop verdict: --unsigned-out does not combine with --%s — the keyless mode reads no key, "+
						"files nothing and composes ONE payload per run; drop --%s", bad, bad))
			}
		}
	}
	return verdictRunConfig{
		root:        *root,
		repo:        *repo,
		head:        *sha,
		runner:      *runner,
		session:     *session,
		pem:         *pem,
		window:      *window,
		dryRun:      *dryRun,
		unsignedOut: *unsignedOut,
	}, nil
}

// runVerdict is the testable core: resolve the envelope (PEM), read the queue, run the
// rows, batch, sign, and print. It returns a *deskkit.DeskError on a fail-closed envelope /
// signing failure and nil on success.
func runVerdict(cfg verdictRunConfig) error {
	// The keyless compose-only mode branches HERE, before the key is resolved: nothing below
	// this line runs for it, so it never reaches resolveVerifierPEMPath or signPayload.
	if cfg.unsignedOut != "" {
		return runVerdictUnsigned(cfg)
	}

	// Operating-envelope preflight: resolve the verifier PEM up front. A missing PEM is an
	// envelope error reported loudly — file nothing, sign nothing.
	pemPath, err := resolveVerifierPEMPath(cfg.pem)
	if err != nil {
		return err
	}

	repo, head, meta := runIdentity(cfg)

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

// runIdentity derives the repo, head and provenance the payload is stamped with — the explicit
// flag values, else git origin / git HEAD / the session tag / the engine's default runner name.
// Both the signed path and the keyless path call it, so the two stamp identical values.
func runIdentity(cfg verdictRunConfig) (repo, head string, meta sessionMeta) {
	repo = cfg.repo
	if repo == "" {
		repo = deriveRepo(cfg.root)
	}
	head = cfg.head
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
	return repo, head, sessionMeta{ID: session, Runner: runner}
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

// runVerdictUnsigned is the keyless compose-only branch of runVerdict (--unsigned-out, brief
// desk-tools/29). It runs the same rows as the signed path, composes ONE payload over all of
// them in the same canonical form signPayload signs, writes it to a NEW file and prints only its
// sha256. It never calls resolveVerifierPEMPath, signPayload or deskkit.FindConfigFile, and it
// reads neither VERIFIER_PEM nor ASSAY_CONFIG_HOME: the rows it runs are arbitrary shell, and
// the point of this mode is that no key is ever in their reach.
//
// On every non-zero exit it leaves no file of its own behind and prints no digest, so a file
// present after a failed run was not written by this run. The digest it prints is the binding
// between the file and this run; the key-holding host checks it with `deskverdict sign
// --expect-sha256`, and takes repo, head and the time bounds from its own dispatch record.
func runVerdictUnsigned(cfg verdictRunConfig) error {
	path := cfg.unsignedOut
	// Refuse an existing entry (a file, a link, a FIFO — anything) before the queue is read or
	// any row runs, and leave it untouched.
	if _, err := os.Lstat(path); err == nil {
		return deskkit.Refused(fmt.Sprintf(
			"verdict runner: --unsigned-out %s already exists — refusing to replace it; nothing composed, nothing written", path))
	} else if !errors.Is(err, fs.ErrNotExist) {
		return deskkit.Unverifiable(fmt.Sprintf(
			"verdict runner: cannot check --unsigned-out %s before composing; nothing composed, nothing written", path), err)
	}

	repo, head, meta := runIdentity(cfg)

	items, err := scanAwaiting(cfg.root, head)
	if err != nil {
		return deskkit.Unverifiable("cannot read the Awaiting queue", err)
	}
	execFn := cfg.execFn()
	var rows []rowResult
	for _, it := range items {
		rows = append(rows, runBriefRows(cfg.root, it, execFn)...)
	}
	out := cfg.emit()
	if len(rows) == 0 {
		fmt.Fprintln(out, "verdict runner: no runner-executed (check/check:ci) rows in the Awaiting queue — nothing to compose")
		return nil
	}

	payload := composePayload(repo, head, cfg.nowFn(), meta, rows)
	canonical, err := canonicalPayloadBytes(payload)
	if err != nil {
		return deskkit.Unverifiable("cannot canonicalise the verdict payload; nothing written", err)
	}
	data := append(canonical, '\n')
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return deskkit.RefusedWithCause(fmt.Sprintf(
				"verdict runner: --unsigned-out %s appeared while the rows ran — refusing to replace it; the existing entry is untouched and nothing was written", path), err)
		}
		return deskkit.Unverifiable(fmt.Sprintf("verdict runner: cannot create --unsigned-out %s; nothing written", path), err)
	}
	created, statErr := f.Stat()
	write := cfg.writeHook
	if write == nil {
		write = func(f *os.File, b []byte) error { _, err := f.Write(b); return err }
	}
	werr := write(f, data)
	cerr := f.Close()
	if werr != nil || cerr != nil {
		cause := werr
		if cause == nil {
			cause = cerr
		}
		return removeUnsignedPartial(path, created, statErr, cause)
	}

	briefs := map[string]bool{}
	for _, r := range rows {
		briefs[r.BriefPath] = true
	}
	fmt.Fprintf(out, "unsigned verdict payload for %d row(s) across %d brief(s) written to %s sha256=%s — NOT signed; "+
		"sign it on the key-holding host with deskverdict sign --expect-sha256 <this digest>, "+
		"taking repo, head and time bounds from the host's own dispatch record\n",
		len(rows), len(briefs), path, digest)
	return nil
}

// removeUnsignedPartial cleans up after a write or close failure on a file the composer itself
// created: it removes the path only while it still names that same file (os.SameFile), so it
// never deletes an entry something else put there. It always returns an exit-6 error naming the
// path, and the caller prints no digest.
func removeUnsignedPartial(path string, created os.FileInfo, statErr, cause error) error {
	msg := fmt.Sprintf("verdict runner: writing --unsigned-out %s failed; nothing signed, no digest printed", path)
	if statErr != nil {
		return deskkit.Unverifiable(msg+fmt.Sprintf(" — could not identify the file it created, so it removed nothing; remove %s by hand", path), cause)
	}
	cur, err := os.Lstat(path)
	if err != nil || !os.SameFile(created, cur) {
		return deskkit.Unverifiable(msg+fmt.Sprintf(" — %s no longer names the file it created, so it removed nothing", path), cause)
	}
	if err := os.Remove(path); err != nil {
		return deskkit.Unverifiable(msg+fmt.Sprintf(" — and removing the partial file %s failed (%v); remove it by hand", path, err), cause)
	}
	return deskkit.Unverifiable(msg+" — the partial file it created was removed", cause)
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
