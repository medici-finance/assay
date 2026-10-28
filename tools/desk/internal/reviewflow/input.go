package reviewflow

import "strings"

type ChangeFact struct {
	Title, Body, MergeStateStatus string
	IsDraft                       bool
	Labels                        []string
	Checks                        []CheckFact
}
type MergeVerdict int

const (
	MergeVerdictOK      MergeVerdict = iota // a recognised state that permits a merge
	MergeVerdictBlocked                     // DIRTY / BLOCKED — definitely not mergeable
	MergeVerdictUnknown                     // could-not-check: UNKNOWN, absent, or unrecognised
	// MergeVerdictBehind (#54) — BEHIND: the base has moved since the head was last synced.
	// The review's "correctness" was established against a `main` that is now stale.
	MergeVerdictBehind
)

// ReadMergeState maps the raw `mergeStateStatus` string to its verdict. The recognised
// mergeable set is GitHub's own enum minus the two blocking values and BEHIND: CLEAN,
// HAS_HOOKS, UNSTABLE (only non-required checks are failing), and DRAFT (the board flips
// drafts, so draft-ness is not a merge-state objection here). BEHIND (base moved on) is its
// own verdict (#54) — a merge is still mechanically possible, but the review that approved
// this head did not verify it against the base it would actually merge into.
func ReadMergeState(s string) MergeVerdict {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "CLEAN", "HAS_HOOKS", "UNSTABLE", "DRAFT":
		return MergeVerdictOK
	case "BEHIND":
		return MergeVerdictBehind
	case "DIRTY", "BLOCKED":
		return MergeVerdictBlocked
	default:
		return MergeVerdictUnknown
	}
}

func BuildClassifyInput(p ChangeFact, rs ReviewState, ciRequired bool, zeroCI string) ClassifyInput {
	pass, pending, fail, unknownChecks := CountChecks(p.Checks)

	// CI is green when no checks are failing or pending, AND either
	// at least one check passed, or CI is not required for this repo
	// (vacuously green — there are no checks to fail).
	// #268: an uninterpretable rollup entry can never count toward green.
	// #400 R2: a false here (nothing failed, nothing pending, and nothing passed
	// either, on a repo that runs CI) is what stops the board asserting green over
	// an empty rollup — classify's CI-UNVERIFIED arm reads ciGreen directly.
	// #1652 tightens the vacuous (`!ciRequired`) half for a ZERO rollup the probe
	// actually READ something negative about: checks-never-ran and unverified are
	// never green on any repo, whether or not that repo requires CI, because absence
	// of evidence is not evidence. A zero the caller never probed (zeroCI=="", the
	// same value an ordinary non-zero rollup carries) is NOT treated as negative
	// evidence — the pre-#1652 vacuous-green rule for a CI-less repo still applies,
	// same as a probed, CONFIRMED no-checks zero.
	ciGreen := unknownChecks == 0 && pass > 0 && pending == 0 && fail == 0
	if unknownChecks == 0 && pass == 0 && pending == 0 && fail == 0 {
		switch zeroCI {
		case ZeroCINeverRan, ZeroCIUnverified:
			ciGreen = false
		default: // ZeroCINoChecks, or "" (never probed)
			ciGreen = !ciRequired
		}
	}

	// #241: the human-gate declaration is read from the PR itself.
	humanGate, hgReason := HumanGateDeclared(p.Title, p.Body, p.Labels)

	mv := ReadMergeState(p.MergeStateStatus)

	return ClassifyInput{
		Ever: rs.Ever, AtHead: rs.AtHead, Blocking: rs.Blocking,
		ApprovedAtHead: rs.Approved, Draft: p.IsDraft,
		Pass: pass, Pending: pending, Fail: fail,
		SecurityPass:           rs.SecurityPass,
		CiGreen:                ciGreen,
		CiUnknown:              unknownChecks,
		HumanGate:              humanGate,
		HumanGateReason:        hgReason,
		MergeConflict:          mv == MergeVerdictBlocked,
		MergeStateUnknown:      mv == MergeVerdictUnknown,
		MergeBehind:            mv == MergeVerdictBehind,
		MergeStateRaw:          p.MergeStateStatus,
		ZeroCI:                 zeroCI,
		SuspectNoOp:            rs.SuspectNoOp,
		ExternalPrereqDeclared: rs.ExternalPrereqDeclared,
	}
}
