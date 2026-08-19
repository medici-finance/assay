package main

import (
	"fmt"
	"os"
	"regexp"
)

// Acquisition channels — the ONE declared source (distribution/05).
//
// "How do you obtain this tool" is stated exactly once, here. Every other
// statement of that fact in this tree is DERIVED from this file at run time —
// the regenerate hints below, the `--lint` drift NOTICE in
// channelconformance.go — or is diffed against it by a test. Nothing restates
// it by hand, because a hand-maintained second copy of a channel list is the
// same divergence class the distribution stream exists to close (#274).
//
// To change what is sanctioned, change THIS table. Do not add a second list.

// acquisitionChannel is one documented way to obtain a released assay tool.
type acquisitionChannel struct {
	// ID is the stable letter the adopter runbook (docs/adopting-assay.md)
	// refers to. It is part of the published contract; do not renumber.
	ID string
	// Name is a one-line description of the mechanism.
	Name string
	// Sanctioned records whether an adopter may be TAUGHT this channel today.
	Sanctioned bool
	// Why records the reason, so a NOTICE can explain itself rather than
	// asserting. A retired channel keeps its row: deleting it would lose the
	// reason it was retired, and the next adopter would re-derive it.
	Why string
}

// sanctionedChannelSet is the declared source. The A–E letters are the
// PUBLISHED contract in docs/adopting-assay.md § "Channels that are NOT the
// default (and why)" — they are stable identifiers, not a ranking, and they
// must not be renumbered or reassigned. TestChannelSetMatchesAdopterRunbook
// CI-diffs this table against that section, so the two cannot drift apart:
// this file is the declared source, the runbook is the prose view of it.
var sanctionedChannelSet = []acquisitionChannel{
	{
		ID:         "A",
		Name:       "vendor the tool's source into the consumer repo",
		Sanctioned: false,
		Why: "a vendored copy is an unpinned fork and forks rot silently — the live instance is a " +
			"statusgen fork frozen at its landing commit, with no pin and no newer flags, still gating another repo's PRs. " +
			"A `go run` against a vendored path is this channel wearing a command",
	},
	{
		ID:         "B",
		Name:       "git submodule, built from the submodule tree",
		Sanctioned: false,
		Why: "pins SOURCE like D, adds submodule-update failure modes to every clone and CI checkout, " +
			"and still hash-checks nothing",
	},
	{
		ID:         "C",
		Name:       "published Go module — `go run`/`go install`/`go get` the module path at a version",
		Sanctioned: false,
		Why:        "rebuilds from source per run, so it verifies no artifact by hash and nothing is reproducible",
	},
	{
		ID:         "D",
		Name:       "CI fetch-and-run at a pinned git ref",
		Sanctioned: true,
		Why: "FALLBACK ONLY, for a runner that cannot download release assets — it pins SOURCE and rebuilds " +
			"per run, so the thing that runs is never hash-checked",
	},
	{
		ID:         "E",
		Name:       "the sha256-pinned release binary named in .assay-versions",
		Sanctioned: true,
		Why:        "the default — one pin file, one hash-verified artifact, one place to bump",
	},
}

// pinSpecRef names the settled home of the pin-file specification. It is a
// prose reference, deliberately not a backticked path: this string is emitted
// into generated files in CONSUMER repos, where docs/distribution.md does not
// exist, and a backticked path there would trip the link check.
const pinSpecRef = "docs/distribution.md, section: The .assay-versions pin file"

// regenerateHeaderHint is the invocation written into the header of every
// GENERATED file.
//
// It is deliberately NOT situation-aware, unlike regenerateHint below. The
// header is PERSISTED and byte-compared by `--check`, so a build-dependent
// string would make an installed release binary and a `go run` CI disagree
// about a file neither of them changed — a drift report with no drift. The
// header therefore names both sanctioned situations and lets the reader pick,
// and points at the pin spec for the rest.
const regenerateHeaderHint = "run statusgen against this repo root — installed release binary " +
	"`statusgen --root .`, or `go run . --root ..` from statusgen/ inside the assay repo. Channels + pin spec: " +
	pinSpecRef

// regenerateHint returns the situation-aware regenerate command for STDERR
// remediation messages. Unlike the header hint this is never persisted, so it
// is free to describe the binary that is actually running:
//
//   - a stamped release binary knows it was installed  → `statusgen --root <root>`
//   - an unstamped build sitting next to ./statusgen    → `go run ./statusgen --root <root>`
//   - an unstamped build running from inside statusgen/ → `go run . --root <root>`
//   - anything else: it cannot tell, so it says the channel-neutral thing and
//     points at the pin spec rather than guessing a path that may not exist.
//
// That last branch is the point. The dead `go run ./tools/statusgen` string
// this replaced was a hardcoded guess about the caller's layout; swapping it
// for a different hardcoded guess would reproduce the defect (#249).
func regenerateHint(root string) string {
	if root == "" {
		root = "."
	}
	if statusgenVersion != "dev" {
		return "statusgen --root " + root
	}
	if st, err := os.Stat("statusgen/main.go"); err == nil && !st.IsDir() {
		return "go run ./statusgen --root " + root
	}
	if st, err := os.Stat("main.go"); err == nil && !st.IsDir() {
		return "go run . --root " + root
	}
	return "statusgen --root " + root + " (see " + pinSpecRef + " for how to obtain it)"
}

// boardProvenanceLine names WHICH binary produced a red board, so a reader can
// tell a stale-oracle red from ground truth (#186). A board's PROBLEM set can be
// produced by more than one statusgen: the sha256-pinned release binary named in
// .assay-versions, or an unstamped local build (a `go run ./statusgen`, or a
// stale installed copy) that predates a check the pin already fixed. The stale
// build reds a board the pinned binary passes — the exact shape of #186, where a
// pre-pin local build emitted DAR-version PROBLEMs that v0.5.0 exits 0 on.
//
// The stamp names the tag but is NOT self-certifying. statusgen cannot read the
// consumer's pin from inside this process (the pin lives in the consumer repo's
// .assay-versions, which the producer tree here does not even carry), so a
// STAMPED binary can still be BEHIND the pin — a v0.5.0 build reporting a red on a
// desk whose pin has moved to v0.8.1 is itself a stale oracle, the very case
// #186 warns about. So BOTH branches point the reader back at the pin: the "dev"
// branch because it is provably unstamped, the stamped branch because the tag it
// names must be confirmed to match the consumer's pin before its red is trusted.
// Neither branch asserts authority the process cannot establish.
//
// This line is stderr-only — never the persisted STATUS.md and never the stdout
// verdict, both of which are byte-compared, so a build-dependent value there
// would report false drift between an installed binary and a `go run` CI (cf.
// emit.go's header note). It is emitted on the board-verdict red paths (blocking
// board-source reds, off-board reds, and --check drift) and DELIBERATELY not on a
// green board or on a fatal source-load error, neither of which is a board whose
// PROBLEM content a differently-versioned binary would reproduce.
// n is the number of PROBLEM lines just emitted above it.
func boardProvenanceLine(n int) string {
	if statusgenVersion == "dev" {
		return fmt.Sprintf(
			"statusgen: the %d PROBLEM(s) above were produced by an UNSTAMPED local build "+
				"(version \"dev\") — a stale local statusgen can red a board the pinned binary "+
				"passes (#186). Re-check against the sha256-pinned release named in .assay-versions "+
				"(see %s) before treating this red as ground truth.",
			n, pinSpecRef)
	}
	return fmt.Sprintf(
		"statusgen: the %d PROBLEM(s) above were produced by statusgen %s. Confirm this tag "+
			"matches the sha256-pinned release named in .assay-versions (see %s) before treating "+
			"this red as ground truth — a STAMPED binary that is BEHIND the pin is itself a stale "+
			"oracle (#186).",
		n, statusgenVersion, pinSpecRef)
}

// staleGeneratedFileMsg is the SINGLE constructor for "this generated file is
// out of date, here is how to regenerate it". main.go's --check verdict and
// both register-view checks in registers.go call it, so the remediation string
// has one definition and cannot drift back apart — which is how three of the
// four #249 emission sites came to say the same dead thing.
func staleGeneratedFileMsg(name, root string) string {
	return fmt.Sprintf("%s is out of date — run: %s", name, regenerateHint(root))
}

// --- drift patterns -------------------------------------------------------

// channelPattern matches prose or an emitted string that TEACHES a
// non-sanctioned channel. Each pattern names the channel it mints, so the
// NOTICE can cite sanctionedChannelSet rather than repeat its reasoning.
type channelPattern struct {
	ID      string
	Channel string // the acquisitionChannel.ID this instruction leads to
	Re      *regexp.Regexp
}

// channelDriftPatterns is the matcher set. It is intentionally small and
// literal: a heuristic that tries to infer intent from prose produces false
// positives, and an advisory check that cries wolf is turned off.
var channelDriftPatterns = []channelPattern{
	{
		// A `go run` against a vendored path IS channel A — it is the vendored
		// copy being executed. It is called out as its own pattern because the
		// path it names (./tools/statusgen) does not exist in a consumer
		// post-selfcontain, so it is also simply unrunnable advice (#249).
		ID:      "vendored-go-run",
		Channel: "A",
		Re:      regexp.MustCompile(`go run \./tools/statusgen`),
	},
	{
		ID:      "vendor-copy-command",
		Channel: "A",
		Re:      regexp.MustCompile(`cp -R[^\n]*statusgen`),
	},
	{
		ID:      "vendor-copy-prose",
		Channel: "A",
		Re:      regexp.MustCompile("(?i)copy `?statusgen/`? into"),
	},
	{
		ID:      "go-install",
		Channel: "C",
		// Anchored on a MODULE path (a dotted host segment before the first
		// slash), so `go run ./statusgen` — the sanctioned in-repo dev
		// invocation, and what this brief's own replacement hint emits — is
		// not matched. A pattern that flagged the sanctioned command would
		// make the check unshippable.
		Re: regexp.MustCompile(`go (?:install|get|run) [A-Za-z0-9_-]+\.[A-Za-z0-9_.-]+/[^\s]*statusgen`),
	},
}

// retiredMarkerRe exempts a line that is DESCRIBING a retired channel rather
// than teaching it. The adopter runbook has to be able to say "channel A is
// RETIRED, and here is the command it used to be" without the drift check
// reading that as advice — a check that forbids naming the retired thing makes
// the record of why it was retired unwritable.
//
// Structural, not registry-driven, on purpose: it works identically in this
// repo and in the published tree, where the accepted-drift registry (a
// do-not-copy stream doc) does not exist.
var retiredMarkerRe = regexp.MustCompile(`(?i)\bretired\b|\bnot sanctioned\b|\bnon-sanctioned\b|\bdo not vendor\b|\bdon't vendor\b`)

// channelByID resolves a pattern's Channel back to its declared row. ok=false
// means the pattern names a channel that is not in the declared set — a code
// bug, surfaced rather than swallowed (see TestChannelPatternsResolve).
func channelByID(id string) (acquisitionChannel, bool) {
	for _, c := range sanctionedChannelSet {
		if c.ID == id {
			return c, true
		}
	}
	return acquisitionChannel{}, false
}
