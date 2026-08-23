package main

// The engine abstraction.
//
// Detection is no longer ours. gitleaks and trufflehog are maintained scanner
// ecosystems whose rule sets no in-house engine will match, and the in-house
// engine has two false clean-certificates on its record. What is NOT theirs is
// the certificate shell: the three-state model, the positive controls, the
// canary, and the refusal to ever report clean on an error. That stays here, and
// the engines run UNDER it.
//
// The contract every engine implements:
//
//   - it either RAN or it did not. "Did not run" is could-not-check, never clean.
//   - its exit code is never trusted on its own. trufflehog exits 0 with findings
//     on stdout unless asked otherwise; gitleaks' exit code is configurable. Both
//     are parsed from their structured report, and a report we cannot parse is
//     could-not-check — a scanner whose output we misread is a scanner we did not
//     run.
//   - it must prove itself live twice per run, on evidence produced by that same
//     invocation path: every rule must fire on the sample fixture, and the canary
//     must fire on the canary fixture using the engine's OWN stock rules.
//
// LICENSING, which is a hard constraint and not a preference:
//   - gitleaks CLI is MIT. gitleaks-ACTION is license-key-gated for organisations
//     and is never adopted here. Only the CLI binary is invoked.
//   - trufflehog is AGPL-3.0. It is exec'd as an external tool and nothing else:
//     no code is embedded, linked, vendored, copied from, or redistributed. The
//     only thing this repo holds is a version string and a sha256.
// Both are therefore invoked as pinned external binaries, never as libraries.

import (
	"os/exec"
	"strings"
)

// finding is one engine's report of one match, normalised.
//
// match carries the matched text — the maximal identifier window for
// except-bearing rules — because that is what the exception predicate needs to
// see. An engine that cannot report the matched text cannot support exceptions,
// and would have to declare that rather than certify.
type finding struct {
	engine string
	ruleID string
	path   string // projection-relative; mapped back to the tree by the caller
	line   int
	match  string
}

// engineOutcome is everything one engine contributes to one run.
//
// ranOK is the load-bearing field: false means every entry is could-not-check
// for this engine, whatever else is set. The zero value is therefore the SAFE
// value — a struct nobody filled in certifies nothing.
type engineOutcome struct {
	name       string
	version    string
	binPath    string
	ranOK      bool
	failReason string

	findings []finding // over the swept tree

	// rulesFired is the set of rule ids that fired on the sample fixture: the
	// per-rule liveness proof. A rule absent from this set is a rule this engine
	// could not be shown to be running, so every entry it serves is
	// could-not-check — the #643 dead-pipeline defence at rule granularity.
	rulesFired map[string]bool

	// canaryRules is the set of the engine's OWN stock rule ids that fired on the
	// canary fixture. Empty means the engine's stock rule set is not running at
	// all, which is a dead alerter regardless of how many house rules fired.
	canaryRules []string
}

// engine is one external scanner, wrapped.
type engine interface {
	name() string
	// locate resolves and version-probes the binary. A failure here is
	// could-not-check, never clean.
	locate() (path, version string, err error)
	// scan runs the engine over dir with the compiled rules and returns raw
	// findings. It must return an error if it cannot be sure it ran to completion.
	scan(bin, dir string, rules []rule) ([]finding, error)
	// scanStock runs the engine over dir with ONLY its own built-in rule set and
	// returns the ids of the rules that fired. This is the canary path: it proves
	// the ecosystem rule set — the entire reason for adopting these tools — is
	// live, which no house-rule result can show.
	scanStock(bin, dir string) ([]string, error)
}

// probeVersion runs `bin args...` and returns the first non-empty output line.
// A binary that will not answer a version probe is not a binary we are going to
// trust a clean certificate to.
func probeVersion(bin string, args ...string) (string, error) {
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return "", err
	}
	for _, ln := range strings.Split(string(out), "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			return ln, nil
		}
	}
	return "", nil
}
