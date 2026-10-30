package main

// roots.go — the ONE root resolution, and the per-root worker pool.
//
// WHY THIS EXISTS. Two verbs read the configured stream roots — `dispatch` (via
// `statusgen --next-up`) and `awaiting` (via `statusgen --gate-scores`) — and each carried
// its own identical five-step preamble: ConfiguredRoots, resolveStatusgen, a ResolveRoot
// loop, resolveStatusgenPin, statusgenVersionOf. Then each walked its roots SERIALLY,
// spawning one statusgen subprocess at a time. `throughput` calls both verbs in full to read
// two integers out of them, so a single throughput run resolved and pinned every root twice
// and spawned 2N statusgen subprocesses in one serial chain. Measured on one operating desk
// host (four roots): `throughput` 82.82s wall, of which the `actions` call is ~70.8s and the
// two report re-runs are the remaining ~12s.
//
// This file supplies both halves of the fix: resolveRootsOnce (perform the preamble exactly
// once, fail-closed, in the order the two verbs already performed it) and runPerRoot (read
// the roots concurrently under a bounded pool).
//
// WHY THE PER-ROOT BOUND IS ITS OWN NUMBER. sweepConcurrency bounds concurrent FORGE reads,
// and its doc comment says exactly why 6 and why the safe direction to be wrong in is low: a
// burst of REST reads trips GitHub's secondary rate limits, which on this code fails the
// whole run closed. The per-root work is a different kind entirely — a LOCAL statusgen
// subprocess reading a distinct directory, bounded by CPU and disk and touching no forge at
// all — so tying the two together would mean a change made for a forge reason silently
// re-tuning a subprocess pool, or the reverse.
//
// ONLY STATUSGEN'S READ MODES RUN CONCURRENTLY, AND ONLY AGAINST DISTINCT ROOTS. `--next-up`
// and `--gate-scores` are query modes that write nothing, and each invocation is handed a
// different `--root`. No write mode of statusgen is run under this pool.

import (
	"fmt"
	"github.com/medici-finance/assay/statusgen/verifyqueue"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rootConcurrency bounds how many configured roots are read at once.
//
// 4 is chosen for the work this actually is: a local statusgen subprocess per root, on a
// host that typically configures a handful of roots (four on the desk host these numbers
// were measured on), so 4 is one wave for a real configuration while staying well under any
// plausible core count. It is deliberately NOT sweepConcurrency — see the file comment.
const rootConcurrency = 4

// rootPoolLimit is the width runPerRoot actually runs at. It is rootConcurrency in production
// and nothing in this binary writes it; it is a variable only so a test can run `dispatch` and
// `awaiting` at a width of 1 — the serial reference the pooled rows must equal, in order
// (TestPerRootPoolMatchesSerial).
var rootPoolLimit = rootConcurrency

// rootSet is everything both root-reading verbs resolve before they read anything: the
// resolved roots in configured order, the pinned statusgen binary, and the pin/version facts
// their headers report. It is built once by resolveRootsOnce and never written after, so
// passing it into concurrent workers shares no mutable state.
type rootSet struct {
	roots     []deskkit.RootConfig // resolved to ABSOLUTE paths, in configured order
	bin       string               // the pinned statusgen binary
	pinnedTag string
	pinRepo   string
	running   string // the version the resolved binary reports
}

// skew reports whether the running statusgen disagrees with the pin, the same comparison
// both verbs' headers already made inline.
func (rs rootSet) skew() bool { return rs.running != rs.pinnedTag }

// resolveRootsOnce performs the root/pin/version preamble exactly once.
//
// FAIL-CLOSED, IN THE ORDER IT ALREADY RAN. Every root is resolved UP FRONT, before any of
// them is read — the discipline both callers already stated in their own comments, for two
// reasons that are unchanged here: a bad root aborts before a single row is collected, and
// the report can carry the resolved ABSOLUTE path so the coverage lines name the directory
// the rows actually came from rather than the configured spelling.
func resolveRootsOnce() (rootSet, error) {
	roots, err := deskkit.ConfiguredRoots()
	if err != nil {
		return rootSet{}, err
	}
	bin, err := resolveStatusgen()
	if err != nil {
		return rootSet{}, err
	}
	resolved := make([]deskkit.RootConfig, 0, len(roots))
	for _, r := range roots {
		abs, rerr := deskkit.ResolveRoot(r)
		if rerr != nil {
			return rootSet{}, rerr // fail-closed: never a partial board
		}
		resolved = append(resolved, deskkit.RootConfig{Repo: r.Repo, Path: abs})
	}
	pinnedTag, pinRepo, err := resolveStatusgenPin(resolved)
	if err != nil {
		return rootSet{}, err
	}
	return rootSet{
		roots:     resolved,
		bin:       bin,
		pinnedTag: pinnedTag,
		pinRepo:   pinRepo,
		running:   statusgenVersionOf(bin),
	}, nil
}

// runPerRoot reads every configured root concurrently under a pool of rootConcurrency
// workers and returns one result per root IN CONFIGURED ORDER, so a caller merging the
// slice sees a deterministic order regardless of which subprocess finished first.
//
// Fail-closed, with the same rule sweepConcurrent enforces for the forge sweep: if any
// root's work errors, the whole call fails and the error returned is the LOWEST-INDEX root's
// — so which failure surfaces does not depend on process scheduling, and a root that could
// not be read can never leave the board reporting a queue that silently excludes it.
func runPerRoot[T any](rs rootSet, work func(deskkit.RootConfig) (T, error)) ([]T, error) {
	return sweepConcurrent(rs.roots, rootPoolLimit, work)
}

// ---------------------------------------------------------------------------
// Queue-contract coverage (brief 45).
//
// `statusgen --gate-scores` rows carry the verification queue's classification
// (statusgen/awaiting_bucket.go queueFor). A root is only COUNTED toward a queue
// depth when it delivered that classification; every other state is a named
// coverage gap, never a zero. The states, per root:
//
//	healthy      every row carries a supported queueVersion and a valid class
//	degraded     supported rows, but some are explicitly unreadable (unknown)
//	unsupported  some row has no queueVersion / an unknown one (older statusgen)
//	unreadable   the root itself could not be read (statusgen failed, bad JSON,
//	             or the root's declared repo disagrees with its configuration)
// ---------------------------------------------------------------------------

const (
	// queueContractVersion is the statusgen queue contract this binary reads.
	queueContractVersion = verifyqueue.Version

	rootHealthy     = "healthy"
	rootDegraded    = "degraded"
	rootUnsupported = "unsupported"
	rootUnreadable  = "unreadable"

	// queueClassUnclassified is what a consumer calls a row whose statusgen did
	// not deliver a supported classification. It is never work and never zero.
	queueClassUnclassified = "unclassified"

	queueClassJudgment   = "judgment-dispatch"
	queueClassRunner     = "runnable-offline"
	queueClassContinue   = "continuation"
	queueClassHuman      = "human-action"
	queueClassUnreadable = "unreadable"
	queueClassPaused     = "paused"
	queueClassParked     = "parked"
)

// rootCoverage is one root's queue-contract reading.
type rootCoverage struct {
	Repo   string `json:"repo"`
	Path   string `json:"path"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	Rows   int    `json:"rows"`
	// Unknown counts rows whose class is unreadable or unclassified: awaiting
	// work this reading cannot place.
	Unknown int `json:"unknown"`
}

// gap reports whether this root's reading is anything less than complete.
func (c rootCoverage) gap() bool { return c.State != rootHealthy }

// rowSupported reports whether a gate-scores row carries a queue classification
// this binary understands.
func rowSupported(r gateScoreRow) bool {
	return verifyqueue.Supported(r.QueueVersion, r.QueueClass)
}

// classOf is the row's class as a consumer must read it: the statusgen class
// when supported, else queueClassUnclassified.
func classOf(r gateScoreRow) string {
	if !rowSupported(r) {
		return queueClassUnclassified
	}
	if !r.QueueValid {
		return queueClassUnreadable
	}
	return r.QueueClass
}

// coverageOf grades one root's rows (or its read error) against the contract.
func coverageOf(rc deskkit.RootConfig, rows []gateScoreRow, readErr error) rootCoverage {
	c := rootCoverage{Repo: rc.Repo, Path: rc.Path, State: rootHealthy, Rows: len(rows)}
	if readErr != nil {
		c.State, c.Reason, c.Rows = rootUnreadable, readErr.Error(), 0
		return c
	}
	unsupported, invalid := 0, 0
	for _, r := range rows {
		switch cl := classOf(r); cl {
		case queueClassUnclassified:
			unsupported++
			c.Unknown++
		case queueClassUnreadable:
			invalid++
			c.Unknown++
		}
	}
	switch {
	case unsupported > 0:
		c.State = rootUnsupported
		c.Reason = fmt.Sprintf("%d row(s) carry no supported queueVersion (statusgen predates the queue contract v%d)",
			unsupported, queueContractVersion)
	case invalid > 0:
		c.State = rootDegraded
		c.Reason = fmt.Sprintf("%d row(s) are could-not-check", invalid)
	}
	return c
}
