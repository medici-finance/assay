package main

import (
	"fmt"
	"strings"
)

// changedonly.go — `--lint --changed-only <paths>` (forge-neutral/18 task 5).
//
// The existing `--changed <file>` plumbing (main.go's `changed []string`, threaded through
// run()) already path-scopes eight checks for CI's differential-lint use. --changed-only is a
// SEPARATE, LOUDER surface built on the same plumbing, for a different purpose: a local
// pre-push convenience that a developer reaches for deliberately, never something the CI gate
// can end up running by accident.
//
// Two properties make that safe:
//
//  1. It REFUSES outright — no output, no scoped run, non-zero exit — the moment it detects it
//     is running inside the CI gate (scanInCI, the same GITHUB_ACTIONS signal every other
//     CI-mode switch in this binary reads). There is no flag, environment variable or argument
//     order that overrides this refusal: a scoped lint that CAN be the gate is a gate that
//     stops checking the moment someone finds it convenient.
//  2. Outside CI it prints a LOUD banner naming exactly which paths were given and stating, in
//     words, exactly what that narrows: the DAR-sync, stream-cap, stream-source,
//     register-integrity and verify-script-diff checks demote a pre-existing defect outside the
//     named set from PROBLEM to NOTICE. Every other check — and these five checks' own
//     defect-detection — still runs at full, unscoped breadth; --changed-only reuses the same
//     `changed []string` plumbing `--changed` already has (see main.go), it does not add a new,
//     narrower lint pass. The banner says exactly this so a scoped run can never be mistaken for
//     a full-tree scope by anyone reading the log rather than the flags.

// changedOnlyCIRefusalMessage is the message printed (and the reason for the non-zero exit)
// when --changed-only is invoked inside the CI gate. Named as a constant so the test asserting
// row 11's negative path and the runtime print share one string.
const changedOnlyCIRefusalMessage = "statusgen: --changed-only refuses to run inside the CI gate " +
	"(GITHUB_ACTIONS=true) — the CI gate requires a full, unscoped --lint. --changed-only is a " +
	"LOCAL pre-push convenience only; there is no flag, environment variable or argument order " +
	"that overrides this refusal. Run the full `statusgen --root . --lint` instead."

// changedOnlyRefusal reports the refusal message when inCI is true, "" otherwise. It takes the
// CI signal as a plain bool rather than reading the environment itself so the negative path
// (row 11) is exercised as a pure function, with no process environment to fake.
func changedOnlyRefusal(inCI bool) string {
	if !inCI {
		return ""
	}
	return changedOnlyCIRefusalMessage
}

// parseChangedOnly splits the --changed-only flag value on commas, trimming whitespace and
// dropping empty entries. It is deliberately NOT a file-of-paths like --changed: the flag names
// paths directly, so a one-off local scoped run needs no throwaway file.
func parseChangedOnly(raw string) []string {
	var paths []string
	for _, p := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(p); t != "" {
			paths = append(paths, t)
		}
	}
	return paths
}

// changedOnlyBanner renders the scope banner --changed-only prints (to stderr) before running.
// It names every path given and says, in as many words, exactly what narrows: the DAR-sync,
// stream-cap, stream-source, register-integrity and verify-script-diff checks demote a
// pre-existing defect outside this path set from PROBLEM to NOTICE — every other check, and
// these checks' own defect-detection, still runs at full, unscoped breadth. This is the second
// layer behind the CI refusal: even a scoped run that somehow reached a gate is visibly named as
// a narrow demotion in the log, never mistaken for a full-tree scope.
func changedOnlyBanner(paths []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "statusgen: --changed-only SCOPED LINT — examining %d path(s) only:\n", len(paths))
	for _, p := range paths {
		fmt.Fprintf(&b, "  %s\n", p)
	}
	b.WriteString("This demotes a pre-existing defect outside this path set, in the DAR-sync, " +
		"stream-cap, stream-source, register-integrity and verify-script-diff checks, from " +
		"PROBLEM to NOTICE. It does NOT skip those checks and it does NOT narrow any other " +
		"check's full-tree reach — every check still runs across the whole tree. This is a local " +
		"pre-push convenience, never a substitute for a full `statusgen --root . --lint` — the CI " +
		"gate always runs the unscoped check.")
	return b.String()
}

// resolveChangedOnly is the pure decision core behind the --changed-only flag: given the raw
// flag value, whether --changed (the file form) was also given, whether --lint was requested,
// and whether the process is running in CI, it decides what main() should do — without touching
// flag.CommandLine, os.Exit or the environment, so every flag-combination row (row 11's whole
// point) is a table-driven unit test rather than a subprocess spawn.
//
// Exactly one of (refusal, usageErr, "") is non-empty on a non-nil error path; paths and banner
// are set only on the success path.
type changedOnlyResult struct {
	// Paths is the parsed path set, non-empty only on success.
	Paths []string
	// Banner is the scope banner to print (stderr) on success.
	Banner string
	// Refusal is set — and nothing should run — when --changed-only was given while the process
	// is running in the CI gate. This is the row-11 negative path: no flag combination clears it.
	Refusal string
	// UsageErr is set for an ordinary misuse (both --changed and --changed-only given, or
	// --changed-only given without --lint, or a flag value that parses to zero paths). Distinct
	// from Refusal because it carries no CI-gate meaning — it is a plain exit-2 usage error.
	UsageErr string
}

func resolveChangedOnly(raw string, changedFileGiven, lintMode, inCI bool) changedOnlyResult {
	if raw == "" {
		return changedOnlyResult{}
	}
	// The CI-gate refusal is checked FIRST and unconditionally: nothing about the other flags —
	// including a malformed --changed-only value — may turn a CI invocation into anything other
	// than this exact refusal.
	if msg := changedOnlyRefusal(inCI); msg != "" {
		return changedOnlyResult{Refusal: msg}
	}
	if changedFileGiven {
		return changedOnlyResult{UsageErr: "--changed-only and --changed are mutually exclusive (both feed the same path-scoping plumbing; use one)"}
	}
	if !lintMode {
		return changedOnlyResult{UsageErr: "--changed-only is only valid with --lint (it demotes specific checks' out-of-scope defects, nothing else)"}
	}
	paths := parseChangedOnly(raw)
	if len(paths) == 0 {
		return changedOnlyResult{UsageErr: "--changed-only was given but no paths parsed from it (comma-separated repo-relative paths expected)"}
	}
	return changedOnlyResult{Paths: paths, Banner: changedOnlyBanner(paths)}
}
