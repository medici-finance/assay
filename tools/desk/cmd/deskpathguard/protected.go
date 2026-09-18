package main

// protected.go — the protected-path classification and the pure decision, Evaluate.
//
// The protected set (docs/protected-paths.md, "Protected set (initial)"):
//   - a brief's `## Verify` table (detected from the diff text — see verifysection.go)
//   - .github/workflows/**
//   - .claude/guardrails/**
//   - tools/skillslint/**
//   - any path under a `verify.d/**` scripted-rows directory
//   - golden fixtures under **/testdata/**
//
// The label rule (brief verify-integrity/01, task item 1) has exactly three gates, ALL of
// which must hold for `wrote-to-the-test` to apply:
//
//  1. the author identity is NOT the desk or verifier identity (Task: "the worker App (or
//     any non-desk, non-verifier identity)") — this is a full exemption regardless of what
//     the diff touches, so a desk/verifier-authored regen or re-baseline PR is never
//     labelled no matter which paths it carries;
//  2. the diff touches at least one protected path;
//  3. the diff touches at least one file that is NEITHER a brief file nor a fixture file —
//     this is what makes an authoring-only diff (brief file(s) only, or fixture-only, e.g.
//     a pure re-baseline) exempt without needing a separate identity check: condition 3
//     cannot hold on such a diff.
//
// A `regen:` label on the PR is a fourth, independent exemption (facts: "or PRs carrying a
// regen: label") — it short-circuits before the three gates are evaluated at all.
import "strings"

// wroteToTheTestLabel is the label deskpathguard applies to a PR that fails the check.
const wroteToTheTestLabel = "wrote-to-the-test"

// gateForcedLine is printed verbatim to stdout whenever the label applies — the line the
// status transition reads (brief verify-integrity/01, task item 1).
const gateForcedLine = "gate-forced: " + wroteToTheTestLabel

// Three states, never two (the verify-integrity stream README's "Before starting" rule):
// every instrument reports checked-clean, checked-failed or could-not-check, and the third
// is never rendered as a pass.
const (
	stateCheckedClean  = "checked-clean"
	stateCheckedFailed = "checked-failed"
	stateCouldNotCheck = "could-not-check"
)

// isFixturePath reports whether p is a golden fixture under **/testdata/**.
func isFixturePath(p string) bool {
	p = strings.TrimPrefix(strings.TrimSpace(p), "/")
	return p == "testdata" || strings.Contains(p, "/testdata/") || strings.HasPrefix(p, "testdata/")
}

// isBriefPath reports whether p is a stream brief file (docs/streams/<stream>/brief-NN-*.md).
func isBriefPath(p string) bool {
	p = strings.TrimPrefix(strings.TrimSpace(p), "/")
	if !strings.HasSuffix(p, ".md") {
		return false
	}
	segs := strings.Split(p, "/")
	if len(segs) < 3 {
		return false
	}
	if segs[0] != "docs" || segs[1] != "streams" {
		return false
	}
	base := segs[len(segs)-1]
	return strings.HasPrefix(base, "brief-")
}

func isWorkflowPath(p string) bool {
	return strings.HasPrefix(strings.TrimPrefix(strings.TrimSpace(p), "/"), ".github/workflows/")
}

func isGuardrailPath(p string) bool {
	return strings.HasPrefix(strings.TrimPrefix(strings.TrimSpace(p), "/"), ".claude/guardrails/")
}

func isSkillslintPath(p string) bool {
	return strings.HasPrefix(strings.TrimPrefix(strings.TrimSpace(p), "/"), "tools/skillslint/")
}

// isVerifyDPath reports whether p sits under a `verify.d/**` scripted-rows directory,
// anywhere in the tree (the brief names no fixed parent for it).
func isVerifyDPath(p string) bool {
	p = strings.TrimPrefix(strings.TrimSpace(p), "/")
	return p == "verify.d" || strings.Contains(p, "/verify.d/") || strings.HasPrefix(p, "verify.d/")
}

// protectedBase reports whether p is protected WITHOUT needing the brief-specific
// Verify-table content check — that one is carried per-file on FileEntry.VerifySectionTouched
// because it depends on WHICH lines of a brief file changed, not just the path.
func protectedBase(p string) bool {
	return isWorkflowPath(p) || isGuardrailPath(p) || isSkillslintPath(p) || isVerifyDPath(p) || isFixturePath(p)
}

// hasRegenLabel reports whether labels carries a `regen:`-prefixed label (case-insensitive).
func hasRegenLabel(labels []string) bool {
	for _, l := range labels {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(l)), "regen:") {
			return true
		}
	}
	return false
}

func loginMatches(login string, accepted []string) bool {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" {
		return false
	}
	for _, l := range accepted {
		if strings.ToLower(strings.TrimSpace(l)) == login {
			return true
		}
	}
	return false
}

// FileEntry is one changed path plus — for a brief file only — whether its diff hunks
// touched the `## Verify` table specifically (see verifysection.go's verifySectionTouched).
// The field is meaningless (and ignored) for a non-brief path.
type FileEntry struct {
	Path                 string
	VerifySectionTouched bool
}

// EvalInput is Evaluate's whole input, deliberately forge-free: every field is a value
// already read, so the decision is a pure function, testable with zero network and zero
// forge fixture.
type EvalInput struct {
	AuthorLogin    string
	DeskLogins     []string
	VerifierLogins []string
	ExistingLabels []string
	Files          []FileEntry
	DiffReadable   bool
}

// EvalResult is deskpathguard check's verdict.
type EvalResult struct {
	State      string // checked-clean | checked-failed | could-not-check
	Label      bool   // apply wrote-to-the-test?
	GateForced bool
	Reason     string
	Protected  []string // protected paths that triggered the label (checked-failed only)
	Other      []string // non-brief, non-fixture paths that triggered the label
}

// Evaluate is the check (brief verify-integrity/01, task item 1). See the file header for
// the exemptions and the three-gate label rule.
func Evaluate(in EvalInput) EvalResult {
	if !in.DiffReadable {
		return EvalResult{
			State:  stateCouldNotCheck,
			Reason: "the PR's diff could not be read — could-not-check is never rendered as clean",
		}
	}
	if loginMatches(in.AuthorLogin, in.DeskLogins) {
		return EvalResult{State: stateCheckedClean, Reason: "author is the desk identity — exempt"}
	}
	if loginMatches(in.AuthorLogin, in.VerifierLogins) {
		return EvalResult{State: stateCheckedClean, Reason: "author is the verifier identity — exempt"}
	}
	if hasRegenLabel(in.ExistingLabels) {
		return EvalResult{State: stateCheckedClean, Reason: "PR carries a regen: label — exempt"}
	}

	var protected, other []string
	for _, f := range in.Files {
		brief := isBriefPath(f.Path)
		fixture := isFixturePath(f.Path)
		if protectedBase(f.Path) || (brief && f.VerifySectionTouched) {
			protected = append(protected, f.Path)
		}
		if !brief && !fixture {
			other = append(other, f.Path)
		}
	}
	if len(protected) > 0 && len(other) > 0 {
		return EvalResult{
			State: stateCheckedFailed, Label: true, GateForced: true,
			Protected: protected, Other: other,
			Reason: "touches a protected path (" + strings.Join(protected, ", ") +
				") alongside a non-brief, non-fixture file (" + strings.Join(other, ", ") + ")",
		}
	}
	return EvalResult{
		State:  stateCheckedClean,
		Reason: "no protected path was touched alongside a non-brief, non-fixture file",
	}
}
