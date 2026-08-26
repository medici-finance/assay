package main

// verifyrun — an EXECUTION WITNESS for Verify rows (ground-truth/01, #284).
//
// THE PROBLEM. A brief's `## Verify` table enumerates the checks the work must
// survive. Its `## Evidence` section records that they were run. Until now the
// second artifact was hand-written prose: nothing bound a line of Evidence to a
// process that ever executed. `verified` was therefore self-reported, and rows
// that were red at merge shipped anyway (#701, #694, #633) with an Evidence
// section that read clean.
//
// THE MECHANISM. `statusgen verifyrun --brief <path>` runs each Verify row's
// Command in a fresh subshell at the repo root and writes back what actually
// happened — the command, the exit code, a hash of the output, the date, the
// executing identity, and the tree it ran against:
//
//	| # | Command | Result | Output | Date | Runner |
//	| 1 | `go test ./...` | pass exit=0 | sha256:6f1a0b3c9d22 | 2026-08-13 | human:alex @ b988d175ab12 |
//
// `--check` reads that table back and reports, per Verify row, whether a witness
// exists, still describes that row, and passed.
//
// THREE STATES, AND WHY could-not-run IS NOT A KIND OF FAILURE.
//
//	pass           the row ran and satisfied every constraint readable from Expect
//	fail           the row ran and contradicted one
//	could-not-run  no verdict was produced at all
//
// could-not-run is the state this whole file exists to keep separable. A binary
// pass/fail instrument has to put "the command was not found" somewhere, and
// wherever it puts it, it lies: called pass, an absent check reports green;
// called fail, it is indistinguishable from a real regression and gets muted as
// noise. Keeping it third means silence about a row is never green — which is
// the same derivation unrun.go already runs at the board level (a Verify row is
// UNRUN unless something proves it ran) applied at the row level, one layer
// down. The two interlock on purpose: `could-not-run` contains the substring
// `not-run`, which is exactly what unrunMarkerRe reads, so a could-not-run
// witness row does NOT clear the UNRUN derivation for that Verify row. A row
// nobody could run cannot be laundered into coverage by writing a witness for
// it.
//
// could-not-run covers two shapes, deliberately: the command never ran (not
// found, not executable, timed out, unsubstituted placeholder), AND it ran but
// yielded no verdict (Expect declares a numeric floor and the output carries no
// number). Both are "the instrument did not look"; neither may render green.
//
// It is a LOWER BOUND on the class, never the complete set. A shell reports only
// 126/127 distinctly; a row whose fixture is missing and whose command reports
// that as exit 1 is recorded `fail`, not `could-not-run`. verifyrun narrows the
// unproven set — it does not close it.
//
// RUNNER ATTRIBUTION IS DERIVED, NEVER SUPPLIED.
//
// There is no `--runner` flag, and passing one is a usage error with its own
// message rather than a generic "flag not defined" — because the flag is the
// attack, not a typo. The identity comes from the process: GITHUB_ACTOR under
// GitHub Actions, otherwise the repo's git identity, rendered `human:<name>` for
// a person and verbatim for a bot/App slug. When neither is available verifyrun
// REFUSES to write anything (exit 2) instead of writing an unattributed row: a
// witness whose runner is blank is not a witness, and inventing a placeholder
// there would make "who ran this" the one field of the record that anybody could
// have written. A witness you can caption is a witness you can forge.
//
// What that does NOT establish: the environment is not a cryptographic anchor —
// whoever controls the process controls GITHUB_ACTOR and git config. The witness
// is evidence for a reviewer reading a diff, not an unforgeable attestation. Its
// real strength is that it lands in the PR next to the tree SHA it names, where
// a second person can re-run the same command and compare hashes.
//
// ── A HAZARD FOR WHOEVER AUTOMATES THIS (ground-truth/02, read this first) ───
//
// verifyrun EXECUTES SHELL COMMANDS LIFTED OUT OF A MARKDOWN FILE. On a branch
// anyone can push to, the Verify table is attacker-controlled input, and running
// it is arbitrary code execution as whoever invoked it. That is fine — required,
// even — for the thing it is: a command a person or a desk runs deliberately,
// against a tree they have looked at.
//
// It is NOT fine as an automatic pull_request-triggered job. `pull_request_target`
// or a self-hosted runner would hand a fork's PR author a shell on the runner
// with the workflow's secrets. Whoever wires this into CI must run it only on
// the `pull_request` trigger with a read-only GITHUB_TOKEN and no secrets in
// scope, or gate it behind an explicit human approval for outside contributors.
// This is why verifyrun is a deliberate sub-command and is never invoked from
// `--lint`: the lint runs everywhere, automatically, and must stay incapable of
// executing anything.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// States
// ---------------------------------------------------------------------------

const (
	statePass        = "pass"
	stateFail        = "fail"
	stateCouldNotRun = "could-not-run"
	// stateSkipped is a FOURTH disposition, and it is neither green nor a
	// failure: an EXPLICITLY env-bound `check` row (verdict-lane/02) that this
	// CI-context run deliberately did not execute, because its verdict rests on
	// authorship+signature (R-6 c.1–3), not on re-execution. A skipped row is NOT
	// written to the Evidence witness table (there is no run to record) and does
	// not enter the exit-code fold — it is reported to the console alone, so a
	// "not selected here" can never be mistaken for a pass.
	stateSkipped = "skip"
)

// verifyEnvBoundNote is the fixed wording for a `check` row skipped in CI. It is
// a compile-time constant, not caller text, so recording it forges nothing. It
// deliberately avoids the UNRUN marker words (unrun/not-run/deferred/skipped as
// a status) and reads as the same deliberate scoping the `-t`-filter convention
// uses: the row IS run — by the runner, elsewhere — just not selected here.
const verifyEnvBoundNote = "env-bound: runner-executed, not selected here"

// verifyrunExit* are the THREE distinct exit codes. Consumers (ground-truth/02
// wires this as a required PR check) branch on them, so 1 and 2 must never be
// collapsed: "one row failed" and "one row was never looked at" prescribe
// different actions — fix the code vs fix the check.
const (
	verifyrunExitPass       = 0 // every row has a witness that matches and passed
	verifyrunExitFail       = 1 // at least one witness records a failure
	verifyrunExitCouldNot   = 2 // at least one row is unwitnessed / stale / could-not-run
	verifyrunExitUsageError = 2 // usage and refusal share the could-not-check code
)

// witnessTimeoutDefault bounds a single row. A row that hangs is not a row that
// passed; it is a row nobody has an answer for, so the timeout lands on
// could-not-run rather than fail.
const witnessTimeoutDefault = 10 * time.Minute

// treeSHALen is how much of the HEAD SHA a witness records. Longer than git's
// 7-char default on purpose: this is the field a re-runner uses to fetch the
// exact tree the output hash belongs to, so collision-resistance beats brevity.
const treeSHALen = 12

// witnessHeader is the Evidence table header verifyrun writes. The column names
// are the ones unrun.go's parseEvidenceRows already keys on (`#`, `Date`,
// `Runner`), so a witness table is read by the existing UNRUN derivation with no
// second parser.
const witnessHeader = "| # | Command | Result | Output | Date | Runner |\n" +
	"|---|---------|--------|--------|------|--------|"

// ---------------------------------------------------------------------------
// The witness row
// ---------------------------------------------------------------------------

// witness is one executed Verify row.
type witness struct {
	ID      string // the Verify row's "#" cell
	Command string // the command AS AUTHORED (pipes still `\|`-escaped)
	State   string // pass | fail | could-not-run
	Exit    int    // the observed exit code; -1 when nothing was executed
	OutHash string // first 12 hex of sha256(combined stdout+stderr)
	Date    string // YYYY-MM-DD
	Runner  string // derived executing identity
	Tree    string // short HEAD SHA, suffixed +dirty when the tree is modified
	// Note is console-only commentary (why could-not-run, which Expect
	// constraints were undecidable). It is deliberately NOT written into the
	// row: the row is a record, and free text in a record is where a caption
	// gets forged.
	Note string
}

// row renders the witness as its Evidence table row.
//
// The Command is written back exactly as it appears in the Verify cell —
// `\|`-escaped and code-spanned — so `--check` can compare witness text to row
// text by string equality, and so a pipeline in a command does not shred the
// table it is recorded in.
func (w witness) row() string {
	result := fmt.Sprintf("%s exit=%s", w.State, exitCell(w.Exit))
	// A could-not-run row's REASON belongs in Evidence, not only the console: a
	// reader (or a tool) looking at the recorded table must be able to tell WHY a
	// row produced no verdict. For a cluster row that reason IS the stable,
	// greppable cluster-pending marker (verdict-lane/07) — the offline lane's
	// could-not-check hand-off, which the --cluster-pending-queue derivation reads
	// back off Evidence. Scoped to could-not-run so no pass/fail row's format
	// changes; witnessStateRe still lifts the leading state and witnessResultRe
	// still finds `exit=`, both ahead of the appended note.
	if w.State == stateCouldNotRun && w.Note != "" {
		result += " — " + w.Note
	}
	return fmt.Sprintf("| %s | `%s` | %s | sha256:%s | %s | %s @ %s |",
		w.ID, w.Command, result, w.OutHash, w.Date, w.Runner, w.Tree)
}

// exitCell renders the exit code, or `-` when nothing ran. `exit=-1` would read
// as an exit code that was observed; `exit=-` reads as the absence of one.
func exitCell(code int) string {
	if code < 0 {
		return "-"
	}
	return strconv.Itoa(code)
}

// A witness row is read POSITIONALLY, cell by cell — never by scanning the row
// as one string.
//
// This is not fastidiousness; scanning the whole row is a live defect, and this
// file's own Verify table caught it. Row 5 of ground-truth/01 is
// `grep -c 'could-not-run' statusgen/verifyrun*.go`: a passing row whose COMMAND
// contains the literal text `could-not-run`. A whole-row regex found that token
// before it found `pass` and reported a green row as could-not-run. Command and
// Output cells are verbatim CONTENT — a command, a hash — and content must never
// be able to impersonate a verdict. unrun.go documents the mirror-image hazard
// (a command cell tripping the UNRUN marker) and solves it the same way.
const (
	witnessCellCommand = 1
	witnessCellResult  = 2
	witnessCellOutput  = 3
	witnessCellCount   = 6
)

// witnessResultRe matches the Result cell's `exit=<code>` marker; `-` is the
// code for a row that never executed.
var witnessResultRe = regexp.MustCompile(`exit=(-|\d+)`)

// witnessHashRe matches the Output cell's fingerprint.
var witnessHashRe = regexp.MustCompile(`sha256:[0-9a-f]{6,64}`)

// witnessStateRe lifts the three-state verdict. could-not-run leads the
// alternation so the compound token is never shortened to a bare `run`.
var witnessStateRe = regexp.MustCompile(`\b(could-not-run|pass|fail)\b`)

// witnessCells splits an Evidence row, or returns nil when it is not shaped
// like a witness. Both markers are required, each in ITS OWN cell: `exit=`
// alone appears in prose Evidence already on main, and a bare `sha256:` could
// be a hash someone pasted.
func witnessCells(text string) []string {
	cells := splitRowEscaped(strings.TrimSpace(text))
	if len(cells) < witnessCellCount {
		return nil
	}
	if !witnessResultRe.MatchString(cells[witnessCellResult]) {
		return nil
	}
	if !witnessHashRe.MatchString(cells[witnessCellOutput]) {
		return nil
	}
	return cells
}

// isWitnessRow reports whether an Evidence row was written by verifyrun.
func isWitnessRow(text string) bool { return witnessCells(text) != nil }

// witnessStateOf reads the verdict out of a witness row's Result cell.
func witnessStateOf(text string) string {
	cells := witnessCells(text)
	if cells == nil {
		return ""
	}
	m := witnessStateRe.FindStringSubmatch(cells[witnessCellResult])
	if m == nil {
		return ""
	}
	return m[1]
}

// witnessCommandOf lifts the Command cell of a witness row. Returns "" when the
// row is malformed — which `--check` then treats as a text mismatch, i.e.
// could-not-check, never pass.
func witnessCommandOf(text string) string {
	cells := witnessCells(text)
	if cells == nil {
		return ""
	}
	return codeSpan(cells[witnessCellCommand])
}

// ---------------------------------------------------------------------------
// Expect — what "pass" means for a row
// ---------------------------------------------------------------------------

// expectSpec is the machine-decidable part of an Expect cell.
//
// Expect is prose by design (brief-rule 7 asks for "expected exit/output", not a
// schema), so this parser is deliberately a READER of the conventions the corpus
// already uses rather than a new format nobody has written yet. Anything it
// cannot decide it does not pretend to decide: the exit code is always checked,
// declared constraints are checked, and the output hash is in the row so a human
// can settle the rest.
type expectSpec struct {
	exitCode int  // expected exit status (default 0)
	exitFrom bool // true when Expect named it explicitly

	// wantLine is set by the `output is `X`` convention: some line of the
	// output, trimmed, must equal X.
	wantLine string

	// minCount is set by the `≥ `N`` / `>= N` convention: the last integer on
	// the last non-empty output line must be at least N.
	minCount int
	minSet   bool
}

// decidable reports whether Expect said anything beyond the default exit code.
// When it did not, `pass` rests on the exit status alone — recorded honestly in
// the run log rather than dressed up as a full verdict.
func (e expectSpec) decidable() bool { return e.exitFrom || e.wantLine != "" || e.minSet }

var (
	// `output is `rc=0`` — the dominant convention in the corpus.
	expectOutputIsRe = regexp.MustCompile("(?i)output\\s+is\\s+`([^`]*)`")
	// `exit 0`, `exit=1`, `exit code 2`. Requires the digits to follow
	// immediately so prose like "its own exit, not pass" does not match.
	expectExitRe = regexp.MustCompile(`(?i)\bexit(?:\s+code)?\s*=?\s*(\d+)`)
	// `≥ `1``, `>= 4`, `at least 2`.
	expectMinRe = regexp.MustCompile("(?i)(?:≥|>=|at least)\\s*`?(\\d+)`?")
	// trailing integer on a line — `12`, `path/to/file.go:12`.
	trailingIntRe = regexp.MustCompile(`(\d+)\D*$`)
	// a backtick-delimited code span. A backticked token in an Expect cell is a
	// QUOTED LITERAL — an output token, a filename, a claim string — never the
	// row's own required process exit, which the corpus always writes UNQUOTED
	// (`exit 0`, `exit NON-zero`, `exit code 2`). It is stripped before the exit
	// is read so that a backticked `exit=N` — the token a `(...) ; echo exit=$?`
	// control row PRINTS to stdout as its evidence — is not misread as the
	// compound's own exit status. The compound always exits 0 by construction, so
	// reading that printed token as a required non-zero exit reddened every such
	// row (#955).
	expectCodeSpanRe = regexp.MustCompile("`[^`]*`")
)

// parseExpect reads an Expect cell into the constraints verifyrun can check.
func parseExpect(expect string) expectSpec {
	spec := expectSpec{exitCode: 0}
	// The output convention is read FIRST and its match is then removed, so a
	// digit inside the expected output (`output is `rc=2``) can never be
	// mistaken for an expected exit code.
	rest := expect
	if m := expectOutputIsRe.FindStringSubmatch(expect); m != nil {
		spec.wantLine = strings.TrimSpace(m[1])
		rest = expectOutputIsRe.ReplaceAllString(expect, " ")
	}
	// The required PROCESS exit is read only from the UNQUOTED text: backticked
	// tokens are quoted literals (see expectCodeSpanRe), and a `(...) ; echo
	// exit=$?` control row writes the exit it is asserting as a backticked stdout
	// token (`exit=1`) precisely because the compound itself always exits 0. The
	// min-count convention legitimately quotes its floor (`≥ `1``), so it still
	// reads the backticks-intact text — only the exit parse sees them stripped.
	exitRest := expectCodeSpanRe.ReplaceAllString(rest, " ")
	if m := expectExitRe.FindStringSubmatch(exitRest); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			spec.exitCode, spec.exitFrom = n, true
		}
	}
	if m := expectMinRe.FindStringSubmatch(rest); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			spec.minCount, spec.minSet = n, true
		}
	}
	return spec
}

// verdict decides pass/fail/could-not-run for an executed row.
//
// The order matters. Anything that means "no verdict was produced" is settled
// before any comparison runs, so a could-not-run row can never fall through into
// a pass on a coincidence (exit 127 is, after all, not exit 0 — but a row that
// EXPECTS 127 must still not pass on a command that does not exist).
func (e expectSpec) verdict(res runResult) (state, note string) {
	if res.couldNotRun {
		return stateCouldNotRun, res.reason
	}
	if res.exit != e.exitCode {
		return stateFail, fmt.Sprintf("exit %d, expected %d", res.exit, e.exitCode)
	}
	if e.wantLine != "" && !outputHasLine(res.output, e.wantLine) {
		return stateFail, fmt.Sprintf("no output line equals %q", e.wantLine)
	}
	if e.minSet {
		n, ok := lastInt(res.output)
		if !ok {
			// Expect declared a numeric floor; the output carries no number to
			// compare it against. The command ran, but the check did not
			// produce a verdict — which is could-not-run, not pass. Calling
			// this pass on the exit status alone is exactly the failure mode
			// the three-state rule exists to prevent.
			return stateCouldNotRun, fmt.Sprintf("Expect requires a count ≥ %d but the output carries no number to read", e.minCount)
		}
		if n < e.minCount {
			return stateFail, fmt.Sprintf("count %d is below the expected minimum %d", n, e.minCount)
		}
	}
	if !e.decidable() {
		return statePass, "expect: exit-status only (nothing else in the Expect cell is machine-decidable — the output hash is the record a reviewer weighs)"
	}
	return statePass, ""
}

// outputHasLine reports whether any line of output, trimmed, equals want.
//
// CONTAINMENT, not whole-output equality, is the right reading of "output is
// `rc=0`": a row that redirects a build log and then echoes its status prints
// both, and the convention names the status line. The looseness is real and
// documented — output that states rc=1 and then rc=0 satisfies this — which is
// why the row also carries the full output hash.
func outputHasLine(output []byte, want string) bool {
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// lastInt reads the trailing integer of the last non-empty output line. It
// handles both a bare count (`3`) and grep's multi-file form
// (`statusgen/verifyrun.go:12`).
func lastInt(output []byte) (int, bool) {
	lines := strings.Split(string(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		m := trailingIntRe.FindStringSubmatch(line)
		if m == nil {
			return 0, false
		}
		n, err := strconv.Atoi(m[1])
		return n, err == nil
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------

// runResult is the raw observation of one command.
type runResult struct {
	exit        int
	output      []byte
	couldNotRun bool
	reason      string
}

// runVerifyCommand executes one Verify command.
//
// "Clean subshell" means a FRESH shell per row, at the repo root, with no state
// carried from the previous row — not a scrubbed environment. Stripping the
// environment would break the majority of real rows (`go test` needs PATH, HOME
// and the Go caches) and would manufacture could-not-run results that say
// nothing about the work. What isolation buys here is that row N cannot leave a
// shell variable, a cwd, or an exported flag behind for row N+1 to pass on.
//
// stdout and stderr are captured COMBINED, in interleaved order, because that is
// what a person running the row in a terminal sees and what the Expect cells in
// the corpus are written against (`2>&1` appears in them constantly).
func runVerifyCommand(root, command string, timeout time.Duration) runResult {
	return runVerifyCommandWith(root, command, timeout, nil)
}

// runHermetically executes a `check:ci` row with the network DISABLED.
//
// check:ci hermeticity is ENFORCED here, never merely declared (rulings.md R-6
// c.6): the row is wrapped in a network namespace so a command that reaches for
// the network is killed by the sandbox rather than trusted to be offline. When
// no such facility is available on this host (not Linux, no `unshare`, or
// unprivileged user namespaces disabled) the row is could-not-run — NOT pass:
// a hermetic check that could not be run hermetically established nothing, and
// the honest three-state answer is "I could not look", reported with the reason.
func runHermetically(root, command string, timeout time.Duration) runResult {
	wrapper, ok, why := networkOffWrapper()
	if !ok {
		return runResult{exit: -1, couldNotRun: true,
			reason: "check:ci hermetic execution requires a network-off sandbox, unavailable on this host: " + why + ". check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net`"}
	}
	return runVerifyCommandWith(root, command, timeout, wrapper)
}

// runVerifyCommandWith executes one Verify command, optionally under a wrapper
// argv prefix (the network-off sandbox for check:ci rows). A nil wrapper runs
// the command directly, which is the env-bound `check`/legacy path.
func runVerifyCommandWith(root, command string, timeout time.Duration, wrapper []string) runResult {
	if strings.TrimSpace(command) == "" {
		return runResult{exit: -1, couldNotRun: true, reason: "the Verify row has no command"}
	}
	// A row still carrying `<placeholder>` text cannot run as literally
	// written; substituting a value here would mean the command recorded in the
	// brief is not the command that produced the witness. Reuses the same
	// detector the unfailable-row lint already applies to this corpus.
	if mv := unsubstitutedMetavars(command); len(mv) > 0 {
		return runResult{exit: -1, couldNotRun: true,
			reason: "unsubstituted placeholder(s) " + strings.Join(mv, ", ") + " — the row cannot run as written"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// The wrapper (e.g. `unshare --net --map-root-user`) prefixes the `sh -c`
	// invocation, so the network namespace is entered before any of the row's
	// own shell runs.
	argv := append(append([]string{}, wrapper...), "sh", "-c", unescapePipes(command))
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	// A row must never be able to consume the parent's stdin: a command that
	// blocks on input would hang to the timeout and report could-not-run for a
	// reason that has nothing to do with the check.
	cmd.Stdin = nil

	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return runResult{exit: -1, output: out, couldNotRun: true,
			reason: fmt.Sprintf("timed out after %s — no verdict was produced", timeout)}
	}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return runResult{exit: 0, output: out}
	case errors.As(err, &exitErr):
		code := exitErr.ExitCode()
		// 127 = command not found, 126 = found but not executable. These are
		// the shell saying it never ran the check, and they are the only two
		// "did not run" signals a POSIX shell reports distinctly.
		if code == 127 || code == 126 {
			return runResult{exit: code, output: out, couldNotRun: true,
				reason: "the shell could not execute the command (exit " + strconv.Itoa(code) + ")"}
		}
		return runResult{exit: code, output: out}
	default:
		// The shell itself could not be started.
		return runResult{exit: -1, output: out, couldNotRun: true, reason: "could not start a shell: " + err.Error()}
	}
}

// unescapePipes converts a markdown-table-escaped `\|` back into the pipe the
// author wrote. A Verify cell MUST escape its pipes or the table shreds; the
// shell must receive the unescaped form or every pipeline in the corpus runs as
// a literal backslash.
func unescapePipes(command string) string {
	return strings.ReplaceAll(command, `\|`, `|`)
}

// networkOffWrapper returns the argv prefix that runs a command with the network
// namespace isolated, and whether such a facility exists on this host.
//
// The facility is `unshare --net --map-root-user`: a new, empty network
// namespace (only loopback, no route off-box) entered via an unprivileged user
// namespace, which is the sandbox GitHub's Linux runner images provide and the
// verdict runner (verdict-lane/04) will use. `--map-root-user` is what makes the
// namespace creatable WITHOUT root; the process's real uid is unchanged outside
// the namespace, so the Go build/module caches (owned by the invoking user)
// remain readable.
//
// ok is false — with a human-legible reason — off Linux, when `unshare` is
// absent, or when a live probe of unprivileged net-namespace creation fails
// (user namespaces disabled). The caller renders that as could-not-run: a
// hermetic row that cannot be run hermetically must never be recorded pass.
func networkOffWrapper() (prefix []string, ok bool, why string) {
	if runtime.GOOS != "linux" {
		return nil, false, "the network sandbox uses `unshare --net`, a Linux facility, and this host is " + runtime.GOOS
	}
	path, err := exec.LookPath("unshare")
	if err != nil {
		return nil, false, "`unshare` is not on PATH"
	}
	// Probe: on a host with user namespaces disabled `unshare` exists but fails
	// at run time. Detecting that here keeps a sandbox-setup failure from being
	// mislabelled as the row's own failure.
	if err := exec.Command(path, "--net", "--map-root-user", "true").Run(); err != nil {
		return nil, false, "`unshare --net --map-root-user` failed on this host (" + err.Error() + ") — unprivileged user namespaces may be disabled"
	}
	return []string{path, "--net", "--map-root-user"}, true, ""
}

// hashOutput is the output fingerprint: the first 12 hex of sha256 over the
// combined output bytes.
//
// A HASH, not the output itself, for two reasons that point the same way. The
// witness lives in a markdown table inside a brief a human reads, and pasting
// a 400-line test log into it would drown the record it belongs to. And a hash
// is checkable by re-running: a second person gets the same 12 hex or they do
// not, with no argument about whitespace. 12 hex (48 bits) is a fingerprint for
// human comparison, not a security boundary — it is not, and must not be read
// as, tamper-proof.
func hashOutput(output []byte) string {
	sum := sha256.Sum256(output)
	return hex.EncodeToString(sum[:])[:12]
}

// ---------------------------------------------------------------------------
// Identity — derived from the executing process, never from an argument
// ---------------------------------------------------------------------------

// runnerNameRe strips a git user.name down to the token shape `human:<name>`
// takes (see corroborate.go's humanStampRe: ASCII word characters only). A name
// that reduces to nothing yields no runner at all, which fails the run closed
// rather than emitting `human:` with an empty name — the exact shape
// attribution.go documents as unparseable.
var runnerNameRe = regexp.MustCompile(`[^0-9A-Za-z_]+`)

// executingRunner derives the runner from the process, in priority order:
//
//  1. GitHub Actions: GITHUB_ACTOR, the identity the workflow ran as. For an
//     App this is the `<slug>[bot]` form, recorded verbatim — it is the App's
//     own name, and rewriting it would lose the bot/human distinction that
//     makes the field worth reading.
//  2. The repository's git identity. A bot identity (`[bot]` in the name or
//     the noreply email) is recorded verbatim; a person becomes `human:<name>`,
//     the token the rest of the toolkit already parses.
//
// ok is false when NEITHER is available. The caller must refuse to write a
// witness in that case — see the file header.
func executingRunner(root string) (runner string, ok bool) {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		if actor := strings.TrimSpace(os.Getenv("GITHUB_ACTOR")); actor != "" {
			return strings.Join(strings.Fields(actor), "-"), true
		}
	}
	name := gitConfigValue(root, "user.name")
	email := gitConfigValue(root, "user.email")
	if strings.Contains(name, "[bot]") || strings.Contains(email, "[bot]") {
		if name != "" {
			return strings.Join(strings.Fields(name), "-"), true
		}
		local, _, _ := strings.Cut(email, "@")
		if local != "" {
			return strings.Join(strings.Fields(local), "-"), true
		}
	}
	candidate := name
	if candidate == "" {
		candidate, _, _ = strings.Cut(email, "@")
		// A noreply address is `<id>+<login>@users.noreply.github.com`; the
		// login is the half worth recording.
		if _, after, found := strings.Cut(candidate, "+"); found {
			candidate = after
		}
	}
	if fields := strings.Fields(candidate); len(fields) > 0 {
		candidate = fields[0]
	}
	candidate = strings.ToLower(runnerNameRe.ReplaceAllString(candidate, ""))
	if candidate == "" {
		return "", false
	}
	return "human:" + candidate, true
}

func gitConfigValue(root, key string) string {
	out, err := exec.Command("git", "-C", root, "config", "--get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// treeSHA is the tree the commands ran against.
//
// A dirty working tree gets a `+dirty` suffix, and that suffix is load-bearing:
// without it a witness taken over uncommitted edits names a commit whose content
// is NOT what produced the output, and a re-runner checking out that SHA would
// get a different hash and conclude the witness was falsified. `+dirty` says
// "this SHA is a neighbourhood, not an address".
func treeSHA(root string) string {
	sha := gitCurrentSHA(root)
	if sha == "" {
		return "no-git"
	}
	if len(sha) > treeSHALen {
		sha = sha[:treeSHALen]
	}
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		// Cannot tell whether the tree is clean — say so rather than imply it
		// is. An unqualified SHA would be a claim this run cannot support.
		return sha + "+unknown"
	}
	if strings.TrimSpace(string(out)) != "" {
		return sha + "+dirty"
	}
	return sha
}

// ---------------------------------------------------------------------------
// The run
// ---------------------------------------------------------------------------

// briefVerifyRows lifts a brief file's Verify rows (id + authored command cell +
// Expect cell), reusing the shared table parser rather than adding a second one.
type verifyRow struct {
	ID      string
	Command string // as authored: `\|`-escaped, code span lifted
	Expect  string
	// Class is the resolved Verify row class (verdict-lane/02): check:ci, check,
	// gate:model, gate:human. A legacy table with no Class column resolves every
	// row to `check` (legacyRowClass). Classed records whether the table opted
	// into the column — the bit that separates an explicitly env-bound `check`
	// (skipped in CI) from a legacy-default `check` (CI still re-executes it).
	Class   string
	Classed bool
}

func briefVerifyRows(verifySection string) []verifyRow {
	var rows []verifyRow
	ordinal := 0
	verifyRowTable(verifySection, func(r verifyRowCells) {
		cmd := codeSpan(r.Command)
		if strings.TrimSpace(cmd) == "" && strings.TrimSpace(r.Expect) == "" {
			return
		}
		ordinal++
		id := strconv.Itoa(ordinal)
		if v := normalizeRowID(r.Num); v != "" {
			id = v
		}
		rows = append(rows, verifyRow{ID: id, Command: cmd, Expect: r.Expect, Class: r.class(), Classed: r.Classed})
	})
	return rows
}

// briefSections reads a brief file's Verify and Evidence bodies.
func briefSections(path string) (verify, evidence string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	body := content
	if first, _, _ := strings.Cut(content, "\n"); strings.TrimSpace(first) == "---" {
		if _, b, ferr := splitFrontmatter(content); ferr == nil {
			body = b
		}
	}
	return extractSectionByPrefix(body, "Verify"), extractEvidence(body), nil
}

// runWitnesses executes every Verify row and returns its witness.
//
// Execution is CLASS-AWARE (verdict-lane/02):
//   - check:ci — re-executed with the network DISABLED (runHermetically).
//   - check    — when ci is set AND the row was EXPLICITLY classed, SKIPPED with
//     the env-bound wording: it is runner-executed, and its verdict rests on
//     authorship+signature, not on a CI re-run. A legacy-default check row
//     (Classed=false) is the inherited corpus, which the scheduled main-rerun
//     must still execute — so it does NOT skip.
//   - everything else (legacy check off-CI, gate:*) — executed as before.
func runWitnesses(root string, rows []verifyRow, runner, tree, date string, timeout time.Duration, ci bool) []witness {
	out := make([]witness, 0, len(rows))
	for _, r := range rows {
		// A cluster row (verdict-lane/07) is env-bound to a live cluster, whose
		// runner is the privileged pod runner. This is the OFFLINE lane — it holds
		// no cluster access and can NEVER run one — so it records could-not-run
		// with the stable, greppable clusterPendingMarker naming the probe, in
		// EVERY environment (not conditioned on ci). could-not-run, not skip: a
		// skip is omitted from the witness table, but the whole point of the marker
		// is that it lands in Evidence for the queue derivation and a human to grep.
		if r.Class == classCheckCluster {
			out = append(out, witness{
				ID: r.ID, Command: r.Command, State: stateCouldNotRun, Exit: -1,
				Date: date, Runner: runner, Tree: tree,
				Note: clusterPendingMarker(clusterProbe(r.Command)),
			})
			continue
		}
		if ci && r.Classed && r.Class == classCheck {
			out = append(out, witness{
				ID: r.ID, Command: r.Command, State: stateSkipped, Exit: -1,
				Date: date, Runner: runner, Tree: tree, Note: verifyEnvBoundNote,
			})
			continue
		}
		var res runResult
		if r.Class == classCheckCI {
			res = runHermetically(root, r.Command, timeout)
		} else {
			res = runVerifyCommand(root, r.Command, timeout)
		}
		state, note := parseExpect(r.Expect).verdict(res)
		out = append(out, witness{
			ID:      r.ID,
			Command: r.Command,
			State:   state,
			Exit:    res.exit,
			OutHash: hashOutput(res.output),
			Date:    date,
			Runner:  runner,
			Tree:    tree,
			Note:    note,
		})
	}
	return out
}

// witnessTable renders a full Evidence table for a run.
//
// SKIPPED rows are omitted: a skip is the deliberate ABSENCE of a run (an
// env-bound check row not selected in CI), and writing it into the witness table
// would record a run that did not happen — the exact forgery the witness exists
// to prevent. The skip is reported to the console instead.
func witnessTable(ws []witness) string {
	var b strings.Builder
	b.WriteString(witnessHeader)
	for _, w := range ws {
		if w.State == stateSkipped {
			continue
		}
		b.WriteString("\n")
		b.WriteString(w.row())
	}
	return b.String()
}

// appendWitnessTable inserts a witness table at the END of a brief's `##
// Evidence` section, leaving everything already there untouched.
//
// APPEND, never overwrite. Evidence is a log: an implementer's run, then an
// independent verifier's re-run, then a re-verify at a later SHA are three
// separate facts, and parseEvidenceRows already unions several tables for
// exactly this reason. Rewriting the section in place would let a green re-run
// erase a red one — editing the recorded basis of a past sign-off, which is the
// falsification the surrounding checks exist to catch.
func appendWitnessTable(path, table string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Evidence" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return fmt.Errorf("%s has no `## Evidence` section to append a witness to", path)
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	// Back up over the section's trailing blank lines so the table lands
	// against the content, not adrift at the section boundary.
	insert := end
	for insert > start && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}
	block := []string{"", table}
	// Only add a trailing blank when the section did not already end in one —
	// otherwise every run stacks another blank line under the table.
	if insert >= len(lines) || strings.TrimSpace(lines[insert]) != "" {
		block = append(block, "")
	}
	merged := append([]string{}, lines[:insert]...)
	merged = append(merged, block...)
	merged = append(merged, lines[insert:]...)
	return os.WriteFile(path, []byte(strings.Join(merged, "\n")), 0o644)
}

// ---------------------------------------------------------------------------
// --check
// ---------------------------------------------------------------------------

// checkFinding is one Verify row's audit result.
type checkFinding struct {
	ID     string
	State  string // pass | fail | could-not-run  (the AUDIT's verdict, not the witness's)
	Detail string
}

// checkWitnesses audits an Evidence section against the current Verify table.
//
// Three questions per row, in the order that makes each meaningful: does a
// witness exist; does it still describe THIS row; did it pass. A stale witness
// (the Verify command was edited after the run) is could-not-check, not fail and
// certainly not pass — the recorded output belongs to a command that is no
// longer in the table, so nothing is known about the row as it stands today.
// Treating it as pass would make "edit the command after running it" the
// cheapest way to keep a green witness.
func checkWitnesses(verifySection, evidenceSection string) []checkFinding {
	rows := briefVerifyRows(verifySection)
	evidence := parseEvidenceRows(evidenceSection)
	var out []checkFinding
	for _, r := range rows {
		var latest string
		for _, er := range evidence[r.ID] {
			if isWitnessRow(er.Text) {
				latest = er.Text // last witness for the row wins
			}
		}
		switch {
		case latest == "":
			out = append(out, checkFinding{r.ID, stateCouldNotRun,
				"no execution witness in Evidence — run `statusgen verifyrun --brief <path>`"})
		case normalizeCommandText(witnessCommandOf(latest)) != normalizeCommandText(r.Command):
			out = append(out, checkFinding{r.ID, stateCouldNotRun,
				"the witness records a different command than the Verify row now carries — the row changed after it was run, so the recorded output proves nothing about it"})
		default:
			switch witnessStateOf(latest) {
			case statePass:
				out = append(out, checkFinding{r.ID, statePass, "witness matches the row and passed"})
			case stateFail:
				out = append(out, checkFinding{r.ID, stateFail, "the witness records a failure"})
			default:
				out = append(out, checkFinding{r.ID, stateCouldNotRun,
					"the witness records could-not-run — the row produced no verdict"})
			}
		}
	}
	return out
}

// normalizeCommandText collapses whitespace so a reflowed table cell is not
// mistaken for an edited command.
func normalizeCommandText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// checkExitCode folds per-row findings into the process exit code.
//
// could-not-check DOMINATES a plain failure. Both are non-green, so no result
// is hidden either way, but when the two collide the run reports the weaker
// state of knowledge: "I could not establish this" is a stronger claim about the
// instrument than "this failed", and the action it prescribes (repair the check)
// has to happen before the failure count means anything.
func checkExitCode(findings []checkFinding) int {
	code := verifyrunExitPass
	for _, f := range findings {
		switch f.State {
		case stateCouldNotRun:
			return verifyrunExitCouldNot
		case stateFail:
			code = verifyrunExitFail
		}
	}
	return code
}

// ---------------------------------------------------------------------------
// --lint NOTICE
// ---------------------------------------------------------------------------

// witnessNotices flags a brief the stream README calls `verified`/`done` whose
// Evidence carries no execution witness for one or more Verify rows.
//
// SEVERITY — NOTICE, not PROBLEM, and for the same reason gate-why and
// unfailableRowNotices are NOTICEs: every brief closed before this mechanism
// existed lacks witnesses by construction. A hard error would red main on merge,
// and the only way to green it would be to write witnesses into already-closed
// briefs — retroactively manufacturing evidence for runs nobody performed, which
// is precisely the falsification the witness exists to prevent. So: NOTICE now,
// visible on every run, backfill the ACTIVE streams, then flip to a hard problem
// once main is clean. The phased path is the established one in this codebase.
// ROLLED UP PER STREAM, not per brief. The unwitnessed set is the entire corpus
// closed before this mechanism existed — 141 briefs on main the day it landed —
// and 141 lines would have grown `--lint`'s notice output by half, drowning the
// findings a reader is meant to act on. An alarm nobody can read is an alarm
// nobody reads (ISA-18.2, the same reasoning behind the flood threshold). The
// roll-up loses nothing: every offending brief is still named, with its
// unwitnessed-row count, on its stream's line.
func witnessNotices(streams []*Stream) []string {
	var notices []string
	for _, s := range streams {
		var offenders []string
		firstPath := ""
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Status != "verified" && br.Status != "done" {
				continue
			}
			art, ok := loadBriefArtifacts(s, br.Num)
			if !ok {
				continue
			}
			rows := briefVerifyRows(art.Verify)
			if len(rows) == 0 {
				continue // no Verify table: a different check's business
			}
			evidence := parseEvidenceRows(art.Evidence)
			missing := 0
			for _, r := range rows {
				witnessed := false
				for _, er := range evidence[r.ID] {
					if isWitnessRow(er.Text) {
						witnessed = true
						break
					}
				}
				if !witnessed {
					missing++
				}
			}
			if missing == 0 {
				continue
			}
			if firstPath == "" {
				firstPath = relDisplayPath(s.Root, art.Path)
			}
			offenders = append(offenders, fmt.Sprintf("%s (%s, %d of %d rows)", br.Num, br.Status, missing, len(rows)))
		}
		if len(offenders) == 0 {
			continue
		}
		notices = append(notices, fmt.Sprintf(
			"%s: %d closed brief(s) carry Verify rows with no EXECUTION WITNESS in Evidence — %s. The Evidence says the rows were run; nothing records a run. `statusgen verifyrun --brief %s` writes one. NOTICE this phase: a brief closed before the witness existed cannot have one, and hand-writing witnesses into closed briefs to green the gate would manufacture exactly the evidence the witness exists to replace",
			s.Name, len(offenders), strings.Join(offenders, ", "), firstPath))
	}
	sort.Strings(notices)
	return notices
}

// relDisplayPath renders a brief path relative to the root when it can, so the
// NOTICE prints a command a reader can paste.
func relDisplayPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// ---------------------------------------------------------------------------
// The sub-command
// ---------------------------------------------------------------------------

const verifyrunUsage = `statusgen verifyrun — execution witness for a brief's Verify rows

Usage:
  statusgen verifyrun --brief <path> [--dry-run] [--timeout <dur>] [--root <dir>]
  statusgen verifyrun --check <path>

Runs each Verify row's Command in a fresh subshell at the repo root and appends
one witness row per Verify row to the brief's ` + "`## Evidence`" + ` section:

  | # | Command | Result | Output | Date | Runner |
  | 1 | ` + "`go test ./...`" + ` | pass exit=0 | sha256:6f1a0b3c9d22 | 2026-08-13 | human:alex @ b988d175ab12 |

Flags:
  --brief <path>    brief file whose Verify table to run (or pass the path positionally)
  --check           audit an existing witness table; runs nothing
  --dry-run         run the rows and print the witness table, but do not write it back
  --timeout <dur>   per-row wall-clock limit (default 10m); a row that times out is could-not-run
  --root <dir>      repo root the commands run in (default: the git toplevel of the cwd)
  --ci              CI context: EXPLICITLY-classed check (env-bound) rows are
                    SKIPPED (runner-executed, not selected here); legacy rows and
                    check:ci rows are unaffected. check:ci rows always run network-off.

Result is three-state. could-not-run (command not found, unsubstituted
placeholder, timeout, or a verdict that could not be derived) is NEVER pass: a
check that could not look does not report green.

Exit codes:
  0  every Verify row has a witness that matches the row and passed
  1  at least one row failed
  2  at least one row could not run, has no witness, or carries a witness whose
     command no longer matches the Verify row (also: usage errors)

Runner attribution is DERIVED from the executing identity — GITHUB_ACTOR under
GitHub Actions, otherwise the repo's git identity. There is deliberately no flag
for it: a witness you can caption is a witness you can forge. When no identity
can be derived, verifyrun refuses to write a witness at all (exit 2).
`

// forbiddenRunnerFlags are rejected with their own message rather than the flag
// package's generic one. The point is not to stop a typo — it is to make the
// refusal legible to whoever tried, so the design intent survives contact with
// the next person who wants to caption a run.
var forbiddenRunnerFlags = map[string]bool{
	"runner": true, "run-as": true, "as": true, "identity": true, "attribution": true, "who": true,
}

// runVerifyrun is the `statusgen verifyrun` entry point. args excludes the
// sub-command name itself.
func runVerifyrun(args []string, stdout, stderr *os.File) int {
	for _, a := range args {
		// Only actual flags: a positional brief path that happens to be spelled
		// `who` is a path, not an attempt to caption the run.
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		name, _, _ = strings.Cut(name, "=")
		if forbiddenRunnerFlags[strings.ToLower(name)] {
			fmt.Fprintf(stderr, "statusgen verifyrun: refusing %q — the runner is derived from the executing identity (GITHUB_ACTOR, or the repo's git identity), never from an argument. A witness whose runner the caller names is a witness the caller can forge, which would defeat the only thing this record is for.\n", a)
			return verifyrunExitUsageError
		}
	}

	fs := flag.NewFlagSet("verifyrun", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// Silenced deliberately. flag.ContinueOnError calls Usage on BOTH a parse
	// error and `--help`, and the two want different streams: help is output and
	// belongs on stdout, a usage error is diagnostics and belongs on stderr.
	// Letting the package print it would emit it twice on `--help` — once here
	// and once from the ErrHelp branch below.
	fs.Usage = func() {}
	briefPath := fs.String("brief", "", "brief file whose Verify table to run")
	checkMode := fs.Bool("check", false, "audit an existing witness table instead of running anything")
	dryRun := fs.Bool("dry-run", false, "print the witness table without writing it back")
	rootDir := fs.String("root", "", "repo root the commands run in")
	timeout := fs.Duration("timeout", witnessTimeoutDefault, "per-row wall-clock limit")
	ci := fs.Bool("ci", false, "CI context: skip explicitly-classed env-bound `check` rows")
	// --help is handled by ContinueOnError returning flag.ErrHelp; print the
	// usage and exit 0, because asking for help is not an error.
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, verifyrunUsage)
			return 0
		}
		fmt.Fprint(stderr, verifyrunUsage)
		return verifyrunExitUsageError
	}

	path := *briefPath
	if path == "" && fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	if path == "" {
		fmt.Fprint(stderr, verifyrunUsage)
		return verifyrunExitUsageError
	}

	verify, evidence, err := briefSections(path)
	if err != nil {
		fmt.Fprintln(stderr, "statusgen verifyrun:", err)
		return verifyrunExitCouldNot
	}
	if strings.TrimSpace(verify) == "" {
		fmt.Fprintf(stderr, "statusgen verifyrun: %s has no `## Verify` section — there is nothing to witness\n", path)
		return verifyrunExitCouldNot
	}

	if *checkMode {
		return runVerifyrunCheck(path, verify, evidence, stdout)
	}

	root := *rootDir
	if root == "" {
		root = repoRootFor(path)
	}
	runner, ok := executingRunner(root)
	if !ok {
		fmt.Fprintln(stderr, "statusgen verifyrun: could-not-attribute — no executing identity is available (no GITHUB_ACTOR under GitHub Actions, no git user.name/user.email in this repo). Refusing to write a witness with no runner: an unattributed witness is not a witness, and inventing a placeholder would make the one field naming who ran this the one field anybody could have written. Set the repo's git identity and re-run.")
		return verifyrunExitCouldNot
	}

	rows := briefVerifyRows(verify)
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "statusgen verifyrun: %s has a `## Verify` section with no runnable rows\n", path)
		return verifyrunExitCouldNot
	}

	ws := runWitnesses(root, rows, runner, treeSHA(root), nowFunc().Format("2006-01-02"), *timeout, *ci)
	table := witnessTable(ws)

	worst := verifyrunExitPass
	for _, w := range ws {
		if w.State == stateSkipped {
			// A skip is neither a run nor a verdict — report it, but it never
			// enters the exit-code fold and it is not in the witness table.
			fmt.Fprintf(stdout, "row %s: skip — %s\n", w.ID, w.Note)
			continue
		}
		line := fmt.Sprintf("row %s: %s (exit=%s, sha256:%s)", w.ID, w.State, exitCell(w.Exit), w.OutHash)
		if w.Note != "" {
			line += " — " + w.Note
		}
		fmt.Fprintln(stdout, line)
		switch w.State {
		case stateCouldNotRun:
			worst = verifyrunExitCouldNot
		case stateFail:
			if worst != verifyrunExitCouldNot {
				worst = verifyrunExitFail
			}
		}
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, table)

	if *dryRun {
		fmt.Fprintf(stdout, "\n--dry-run: the table above was NOT written to %s\n", path)
		return worst
	}
	if err := appendWitnessTable(path, table); err != nil {
		fmt.Fprintln(stderr, "statusgen verifyrun:", err)
		return verifyrunExitCouldNot
	}
	fmt.Fprintf(stdout, "\nwitness appended to the `## Evidence` section of %s\n", path)
	return worst
}

// runVerifyrunCheck is the `--check` half: audit, never execute.
func runVerifyrunCheck(path, verify, evidence string, stdout *os.File) int {
	findings := checkWitnesses(verify, evidence)
	if len(findings) == 0 {
		fmt.Fprintf(stdout, "%s: no Verify rows to check\n", path)
		return verifyrunExitCouldNot
	}
	counts := map[string]int{}
	for _, f := range findings {
		counts[f.State]++
		fmt.Fprintf(stdout, "row %s: %s — %s\n", f.ID, f.State, f.Detail)
	}
	fmt.Fprintf(stdout, "%s: %d pass, %d fail, %d could-not-run/missing (of %d Verify rows)\n",
		path, counts[statePass], counts[stateFail], counts[stateCouldNotRun], len(findings))
	return checkExitCode(findings)
}

// repoRootFor resolves the repo root the commands should run in: the git
// toplevel above the brief, falling back to the cwd's toplevel and then the cwd.
func repoRootFor(briefPath string) string {
	dir := filepath.Dir(briefPath)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output(); err == nil {
		if top := strings.TrimSpace(string(out)); top != "" {
			return top
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}
