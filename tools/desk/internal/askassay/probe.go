package askassay

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// READ-ONLY, STRUCTURALLY
// -----------------------
// This file holds the ONLY subprocess call site in the package (see
// TestExactlyOneSubprocessCallSite). Everything reaching it passes
// [GuardReadOnly] first, which is a CLOSED allow-list of binaries, verbs and
// modes: an argv that is not positively recognised is REFUSED, not permitted.
// Default-deny is the whole point — a permit-list that falls through to allow
// is one new verb away from a write.
//
// The refusal is not a flag. There is no configuration, environment variable
// or exported field that widens the allow-list, and [Runner]'s subprocess seam
// is an UNEXPORTED field so no caller outside this package can replace it.
// Making the pane able to write requires editing this file and deleting a
// test — which is the standard being aimed at, because "the gap between a
// flag and a grant is one commit."
//
// THE BOUND ON THIS CLAIM, STATED
// -------------------------------
// The guard is a verb/mode allow-list over an argv. It is NOT a capability
// proof of the binaries it permits: if a permitted read verb of `gh`,
// `statusgen` or `deskboard` acquires a write side effect, this guard will not
// notice. What it does guarantee is that this package never ASKS for one, that
// the set it asks for is enumerated in source, and that adding to that set is
// a reviewable diff in a file whose header says so.

// ErrRefused is returned for an argv the read-only guard does not positively
// recognise. It maps to the suite's REFUSED class: fix the input and re-run
// the same verb; never reach for a different tool to make the same call.
var ErrRefused = errors.New("refused: not a declared read-only probe")

// readOnlyBinaries is the closed set of executables this pane may run.
var readOnlyBinaries = map[string]bool{
	"gh":        true,
	"statusgen": true,
	"deskboard": true,
}

// ghReadVerbs maps a `gh` subcommand to the sub-subcommands permitted under
// it. An empty set means the subcommand takes no further verb.
var ghReadVerbs = map[string]map[string]bool{
	"api":   {},
	"issue": {"list": true, "view": true},
	"pr":    {"list": true, "view": true},
	"run":   {"list": true, "view": true},
}

// statusgenReadModes are the modes that read without writing. statusgen's
// DEFAULT mode writes the status file, so a bare invocation with no mode is
// refused: an omitted flag is not a read.
var statusgenReadModes = map[string]bool{
	"--lint": true, "--check": true, "--json": true, "--dora": true,
	"--trend": true, "--alarms": true, "--code": true,
	"--gate-scores": true, "--launch": true, "--signoff-digest": true,
	"--verify-issues": true, "--decision-issues": true, "--consumers": true,
	"--corroborate": true,
}

// statusgenWriteModes are refused by name as well as by absence from the read
// set, so that a mode which both reads and writes cannot slip in on the
// strength of a read flag appearing elsewhere in the same argv.
//
// --bottleneck is in this set, and it is the one that had to be MEASURED
// rather than reasoned about. It reads like a diagnostic and its own doc
// comment calls it a self-contained diagnostic sub-command, but its runner
// writes a dated report unconditionally — there is no flag that suppresses it
// and no json mode that skips it. Measured on a clean worktree: invoking it
// exits 0 and leaves an untracked docs/reports/factory-floor/<date>.md behind.
// Worse, that path carries no row in the publication manifest, so what the
// "read" leaves behind is unclassified for publication. A probe whose side
// effect is an unclassified file is not a read at any strength.
var statusgenWriteModes = map[string]bool{
	"--record": true, "--scan-issues": true, "--close-verify": true,
	"--register-links": true, "--roadmap": true, "--export-evidence": true,
	"--init": true, "--bottleneck": true,
}

// deskboardReadVerbs are the board verbs that only derive.
var deskboardReadVerbs = map[string]bool{
	"prs": true, "actions": true, "queue": true, "health": true,
	"awaiting": true, "nextup": true, "scope": true, "policydrift": true,
	"stalled": true, "reviews": true, "diff": true, "files": true,
	"dispatch": true, "todo": true, "next": true, "next-up": true,
}

// ghReadHTTPMethods is the CLOSED set of methods `gh api` may carry. It is an
// ALLOW-list, not a deny-list of write verbs: the previous shape enumerated
// POST/PUT/PATCH/DELETE and permitted anything else, which is one novel method
// away from a write and contradicts this file's default-deny header.
var ghReadHTTPMethods = map[string]bool{"GET": true, "HEAD": true}

// ghFieldFlags are the flags that add request PARAMETERS to a `gh api` call.
// They matter because of gh's own documented behaviour, quoted from `gh api
// --help`:
//
//	"The default HTTP request method is GET normally and POST if any
//	 parameters were added."
//	"Note that adding request parameters will automatically switch the
//	 request method to POST. To send the parameters as a GET query string
//	 instead, use --method GET."
//
// Measured on gh at the time of writing: an api call against an arbitrary path
// carrying `-f k=v` and NO method flag issues `POST <path>`; the same call with
// no field flag issues `GET <path>`. So a REST write can be spelled with no
// method token in the argv and no mutation keyword anywhere — the exact shape
// the earlier guard fell through on, because it only looked for an explicit
// method flag and for the mutation keyword.
var ghFieldFlags = map[string]bool{
	"-f": true, "--raw-field": true, "-F": true, "--field": true,
}

// graphQLMutation matches a GraphQL mutation operation in a query argument.
var graphQLMutation = regexp.MustCompile(`(?i)\bmutation\b`)

// GuardReadOnly reports whether argv is a declared read-only probe. Anything
// it does not positively recognise is refused.
func GuardReadOnly(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("%w: empty argv", ErrRefused)
	}
	bin := argv[0]
	if !readOnlyBinaries[bin] {
		return fmt.Errorf("%w: %q is not one of the declared read-only binaries (%s)", ErrRefused, bin, sortedKeys(readOnlyBinaries))
	}
	switch bin {
	case "gh":
		return guardGh(argv)
	case "statusgen":
		return guardStatusgen(argv)
	case "deskboard":
		return guardDeskboard(argv)
	}
	return fmt.Errorf("%w: %q has no guard", ErrRefused, bin)
}

func guardGh(argv []string) error {
	if len(argv) < 2 {
		return fmt.Errorf("%w: gh with no subcommand", ErrRefused)
	}
	sub := argv[1]
	subVerbs, ok := ghReadVerbs[sub]
	if !ok {
		return fmt.Errorf("%w: gh %s is not a declared read subcommand (%s)", ErrRefused, sub, sortedNestedKeys(ghReadVerbs))
	}
	if len(subVerbs) > 0 {
		if len(argv) < 3 {
			return fmt.Errorf("%w: gh %s with no verb", ErrRefused, sub)
		}
		if !subVerbs[argv[2]] {
			return fmt.Errorf("%w: gh %s %s is not a declared read verb", ErrRefused, sub, argv[2])
		}
	}
	for _, a := range argv {
		if graphQLMutation.MatchString(a) {
			return fmt.Errorf("%w: gh argument carries a GraphQL mutation", ErrRefused)
		}
	}
	if sub == "api" {
		return guardGhAPI(argv)
	}
	// A method or request-body flag on a subcommand that has no such flag is
	// not a declared read shape. Refusing it keeps the earlier guard's coverage
	// (which scanned for a method flag on every gh subcommand) rather than
	// narrowing it while widening the api path.
	for _, a := range argv[1:] {
		for _, bad := range []string{"-X", "--method", "--input"} {
			if a == bad || strings.HasPrefix(a, bad+"=") || (bad == "-X" && strings.HasPrefix(a, "-X") && len(a) > 2) {
				return fmt.Errorf("%w: gh %s carries %s, which is not part of any declared read shape for that subcommand", ErrRefused, sub, bad)
			}
		}
	}
	return nil
}

// guardGhAPI adjudicates the flag surface of `gh api`, where the method is
// IMPLICIT unless stated. Three rules, all default-deny:
//
//  1. An explicit method must be in [ghReadHTTPMethods]. Unknown method,
//     refused — not "not one of the four write verbs I know".
//  2. A request-body file is refused outright. It forces the method to POST and
//     its contents are opaque to this guard, so permitting it would be
//     permitting an unreadable payload.
//  3. On a REST path, a field flag with NO explicit read method is a write,
//     because gh switches the method to POST for it. On the graphql path field
//     flags are how a QUERY is carried, so they are permitted there — but only
//     with a literal value: `@`-prefixed values read from a file or stdin,
//     which this guard cannot inspect, are refused for the same reason as (2).
func guardGhAPI(argv []string) error {
	var (
		isGraphQL  = len(argv) > 2 && argv[2] == "graphql"
		sawMethod  bool
		fieldFlags []string
	)
	for i := 1; i < len(argv); i++ {
		a := argv[i]

		if name, val, ok := splitFlag(a, argv, i, map[string]bool{"-X": true, "--method": true}); ok {
			if strings.TrimSpace(val) == "" {
				return fmt.Errorf("%w: gh api %s with no method", ErrRefused, name)
			}
			if !ghReadHTTPMethods[strings.ToUpper(val)] {
				return fmt.Errorf("%w: gh api with method %q — the permitted methods are %s, and every other method is a write until proven otherwise", ErrRefused, val, sortedKeys(ghReadHTTPMethods))
			}
			sawMethod = true
			continue
		}

		if _, _, ok := splitFlag(a, argv, i, map[string]bool{"--input": true}); ok {
			return fmt.Errorf("%w: gh api with a request-body file — it switches the method to POST and this guard cannot read what is in it", ErrRefused)
		}

		if name, val, ok := splitFlag(a, argv, i, ghFieldFlags); ok {
			fieldFlags = append(fieldFlags, name)
			// A field flag's value is itself a key=value pair, so the
			// indirection marker sits AFTER the key's '=', not at the start of
			// the token. Checking the token start instead of the pair's value
			// is a check that never fires — it was written that way first, and
			// the roster below is what caught it.
			pairVal := val
			if j := strings.IndexByte(pairVal, '='); j >= 0 {
				pairVal = pairVal[j+1:]
			}
			if strings.HasPrefix(strings.TrimSpace(pairVal), "@") {
				return fmt.Errorf("%w: gh api field %s reads its value from a file or stdin (%q), which this guard cannot inspect for a mutation", ErrRefused, name, val)
			}
		}
	}
	if len(fieldFlags) > 0 && !isGraphQL && !sawMethod {
		return fmt.Errorf("%w: gh api carries request parameters (%s) on a REST path with no explicit read method — gh switches the request to POST for exactly this argv, so this is a write with no method token in it. State %s explicitly to send them as a query string",
			ErrRefused, strings.Join(fieldFlags, " "), "--method GET")
	}
	return nil
}

// splitFlag recognises a flag in any of the three spellings gh accepts:
// separated (`-X GET`), joined-with-equals (`--method=GET`) and joined
// shorthand (`-XGET`). It returns the canonical flag name and its value.
// Matching is by exact flag name, never by substring — a membership test that
// accepts a prefix is how an unrelated flag gets read as a permitted one.
func splitFlag(a string, argv []string, i int, names map[string]bool) (string, string, bool) {
	if names[a] {
		if i+1 < len(argv) {
			return a, argv[i+1], true
		}
		return a, "", true
	}
	if j := strings.IndexByte(a, '='); j > 0 && names[a[:j]] {
		return a[:j], a[j+1:], true
	}
	for n := range names {
		if len(n) == 2 && strings.HasPrefix(n, "-") && !strings.HasPrefix(n, "--") &&
			len(a) > 2 && strings.HasPrefix(a, n) {
			return n, a[2:], true
		}
	}
	return "", "", false
}

func guardStatusgen(argv []string) error {
	for _, a := range argv[1:] {
		name := a
		if i := strings.IndexByte(a, '='); i >= 0 {
			name = a[:i]
		}
		if statusgenWriteModes[name] {
			return fmt.Errorf("%w: statusgen %s writes", ErrRefused, name)
		}
	}
	for _, a := range argv[1:] {
		name := a
		if i := strings.IndexByte(a, '='); i >= 0 {
			name = a[:i]
		}
		if statusgenReadModes[name] {
			return nil
		}
	}
	return fmt.Errorf("%w: statusgen with no declared read mode — its DEFAULT mode writes the status file, so an omitted mode flag is a write, not a read (declared read modes: %s)", ErrRefused, sortedKeys(statusgenReadModes))
}

func guardDeskboard(argv []string) error {
	for _, a := range argv[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if !deskboardReadVerbs[a] {
			return fmt.Errorf("%w: deskboard %s is not a declared read verb (%s)", ErrRefused, a, sortedKeys(deskboardReadVerbs))
		}
		return nil
	}
	return fmt.Errorf("%w: deskboard with no verb", ErrRefused)
}

func sortedKeys(m map[string]bool) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func sortedNestedKeys(m map[string]map[string]bool) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

// Runner executes declared read-only probes. Its subprocess seam is
// unexported: no caller outside this package can substitute it, so there is no
// supported way to route a write through this type.
type Runner struct {
	// Dir is the working directory for the probe.
	Dir string
	// Timeout bounds a probe. Zero uses defaultProbeTimeout.
	Timeout time.Duration

	// run is the subprocess seam, replaced only by this package's own tests.
	run func(ctx context.Context, dir string, argv []string) ([]byte, error)
}

const defaultProbeTimeout = 60 * time.Second

// Run executes argv after the guard permits it. A refused argv never reaches a
// subprocess.
func (r Runner) Run(ctx context.Context, argv []string) ([]byte, error) {
	if err := GuardReadOnly(argv); err != nil {
		return nil, err
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = defaultProbeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if r.run != nil {
		return r.run(ctx, r.Dir, argv)
	}
	return execRead(ctx, r.Dir, argv)
}

// execRead is the single subprocess call site in this package. It is
// unexported, it is reached only through [Runner.Run], and [Runner.Run] is
// reached only past [GuardReadOnly].
func execRead(ctx context.Context, dir string, argv []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	return cmd.Output()
}

// blindPatterns are the strings a throttled, denied or degraded probe emits.
// Any of them turns a result into could-not-check regardless of exit code,
// because a rate-limited probe that still exits 0 with an empty body is the
// exact shape that renders as a confident zero.
var blindPatterns = []string{
	"api rate limit exceeded",
	"secondary rate limit",
	"you have exceeded a secondary rate limit",
	"was submitted too quickly",
	"abuse detection",
	"http 403",
	"http 401",
	"http 502",
	"http 503",
	"bad credentials",
	"could not resolve to a repository",
	"gh: not found",
	"context deadline exceeded",
	"signal: killed",
}

// Classify turns a probe result into a three-state verdict and a reason. This
// is where "an empty result is BLIND, not idle" is enforced: an empty payload
// is could-not-check unless the question has declared, in writing, why an
// empty payload from ITS source is a genuine zero.
//
// Order matters. The blind patterns are checked BEFORE the exit code, because
// the failure that produced the wrong answers being guarded against here was a
// throttled call that still looked successful.
func Classify(q Question, out []byte, runErr error) (State, string) {
	hay := strings.ToLower(string(out))
	if runErr != nil {
		hay += " " + strings.ToLower(runErr.Error())
	}
	for _, p := range blindPatterns {
		if strings.Contains(hay, p) {
			return CouldNotCheck, fmt.Sprintf("the probe came back BLIND, not empty: its output or error matched %q. A blind probe renders could-not-check; rendering 0 here would assert a falsehood about %s", p, q.Source.Cmd)
		}
	}
	if runErr != nil {
		if errors.Is(runErr, ErrRefused) {
			return CouldNotCheck, "the probe was REFUSED by the read-only guard: " + runErr.Error()
		}
		return CouldNotCheck, "the probe did not complete: " + runErr.Error()
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		if q.EmptyMeansZero {
			return Checked, "empty payload accepted as a real zero: " + q.EmptyRationale
		}
		return CouldNotCheck, fmt.Sprintf("the probe returned an empty payload and %s does not declare an empty result to be a real zero. An empty result is blind, not idle", q.ID)
	}
	return Checked, ""
}
