package deskkit

// Trust tiers + the contributor ledger.
//
// THE PROBLEM. The inbound author bar is binary: an identity is in the operator's
// roster (trust.go) or it is a stranger, and one item is blessed or it is
// quarantined. There is nowhere to record "this identity has a record" or "this
// identity's last submission was closed unreviewed", so a contributor whose third
// change is landing is assessed exactly like an account nobody has seen before.
//
// THE MODEL (design record DR-trust-tiers-ledger). Four tiers, ordered:
// `unknown` < `blessed-once` < `contributor` < `maintainer`. `unknown` is every
// identity the ledger does not name — today's bar, and the fail-closed default.
// `blessed-once` records that a maintainer admitted ONE specific item and nothing
// more. `contributor` is a standing grant a maintainer recorded after seeing the
// identity's work land. `maintainer` is the EXISTING roster (trust.go),
// unchanged — it is resolved from the roster, never from the ledger, and the
// ledger cannot lower it.
//
// WHERE THE LEDGER LIVES. Operator-side configuration, named by a roster key
// (EnvContributorLedger, rosterconfig.go) on the same outside-every-ref discipline
// the roster itself uses (P2). It is NOT a file in this repository: the rows are
// trust judgements about named external people, and this tree is public and its
// registers are append-only — a demotion recorded here would be a permanent
// public mark on an individual. No tool writes it except the structured blessing
// act that follows this brief. See docs/contributor-trust.md for the published
// model.
//
// FAIL-CLOSED, THREE-STATE. An absent ledger (unset, or the configured file does
// not exist) is a LEGITIMATE empty and resolves `unknown` — that is today's bar.
// An unreadable or malformed ledger is DIFFERENT: could-not-check, announced on
// stderr, and it ALSO resolves `unknown` — a broken ledger can only narrow what
// automation does, never widen it. The two are distinguishable in the returned
// Provenance and are never collapsed.
//
// THIS BRIEF IS INERT ON LANDING. No caller is wired to ResolveTier or the
// capability table here — that follow-up work wires it — so with no ledger
// configured every identity resolves exactly as it does today, and no existing
// caller's answer changes.
import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Tier is the ordered contributor-trust level a resolved identity holds against
// one repository. The zero value, TierUnknown, is the fail-closed default: every
// identity the ledger does not name, and every identity when the roster and the
// ledger are both unconfigured.
type Tier int

const (
	TierUnknown Tier = iota
	TierBlessedOnce
	TierContributor
	TierMaintainer
)

// String renders the tier's published name (docs/contributor-trust.md).
func (t Tier) String() string {
	switch t {
	case TierBlessedOnce:
		return "blessed-once"
	case TierContributor:
		return "contributor"
	case TierMaintainer:
		return "maintainer"
	default:
		return "unknown"
	}
}

// AtLeast reports whether t sits at or above min in the total order
// unknown < blessed-once < contributor < maintainer.
func (t Tier) AtLeast(min Tier) bool { return t >= min }

// parseTier resolves a ledger row's recorded tier name. ok is false for anything
// other than the four published names — a row that fails to parse grants nothing,
// the same fail-closed direction as every other malformed-row case here.
func parseTier(s string) (Tier, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "unknown":
		return TierUnknown, true
	case "blessed-once":
		return TierBlessedOnce, true
	case "contributor":
		return TierContributor, true
	case "maintainer":
		return TierMaintainer, true
	default:
		return TierUnknown, false
	}
}

// Provenance names WHERE a resolved tier came from, so a caller — and the tests
// here — can tell a legitimately empty ledger from a broken one rather than
// collapsing both into an unexplained "unknown".
type Provenance string

const (
	// ProvenanceRoster: the identity is a member of the existing operator roster
	// (trust.go / rosterconfig.go) — TierMaintainer. The ledger is never
	// consulted for it and cannot lower it.
	ProvenanceRoster Provenance = "roster"
	// ProvenanceLedgerRow: a ledger row matched repo + identity on the strict,
	// pinned-id path.
	ProvenanceLedgerRow Provenance = "ledger-row"
	// ProvenanceDefaultAbsent: a LEGITIMATE empty — no ledger is configured, the
	// configured file does not exist, or it loaded cleanly with no row naming
	// this identity for this repo. This is today's bar and is never an anomaly.
	ProvenanceDefaultAbsent Provenance = "default-absent"
	// ProvenanceDefaultUnreadable: the configured ledger could not be read or did
	// not parse. This IS an anomaly — announced on stderr — but still resolves
	// TierUnknown: a broken ledger can only narrow automation, never widen it.
	ProvenanceDefaultUnreadable Provenance = "default-unreadable"
)

// LedgerRow is one recorded trust judgement: a human act, with a reason, not a
// computed fact. The structured blessing act that follows this brief is the only writer.
type LedgerRow struct {
	Repo     string `json:"repo"`
	Login    string `json:"login"`
	ID       int64  `json:"id"`
	Tier     string `json:"tier"`
	Date     string `json:"date"`
	Recorder string `json:"recorder"`
	Reason   string `json:"reason"`
}

// LedgerLoader loads the raw ledger rows, or an error. It is the injection seam:
// production reads the operator-configured file (loadLedgerFile);
// tests install a canned loader so ResolveTier's resolution logic is exercised
// with no filesystem dependency (pure, offline).
type LedgerLoader func() ([]LedgerRow, error)

// errLedgerNotConfigured is the sentinel a LedgerLoader returns to say "there is
// no ledger to read" — no path configured, or the configured file does not exist.
// ResolveTier distinguishes this (ProvenanceDefaultAbsent, a legitimate empty)
// from every other error (ProvenanceDefaultUnreadable, an anomaly) with
// errors.Is, never by matching error text.
var errLedgerNotConfigured = errors.New("ledger not configured")

// contributorLedgerLoader is the production loader. Tests swap it via
// withLedgerLoader rather than writing real files through rosterconfig.
var contributorLedgerLoader LedgerLoader = loadLedgerFile

// withLedgerLoader installs loader as the ledger source for the duration of a
// test and returns a restore func. Test-only seam: nothing outside this package
// reads or writes contributorLedgerLoader.
func withLedgerLoader(loader LedgerLoader) (restore func()) {
	prev := contributorLedgerLoader
	contributorLedgerLoader = loader
	return func() { contributorLedgerLoader = prev }
}

// loadLedgerFile reads the operator-configured ledger
// (ContributorLedgerPath, rosterconfig.go). Absent configuration or a missing
// file is errLedgerNotConfigured — a legitimate empty. Any other read or parse
// failure is a distinct, non-nil, descriptive error.
//
// Format: one JSON object per non-blank, non-'#'-prefixed line (JSON Lines), each
// shaped like LedgerRow. The structured blessing act is the only writer; this
// function only reads, and never partially — one malformed line
// fails the WHOLE read, the same fail-closed direction rosterconfig.go's
// parseConfig takes on a bad roster entry.
func loadLedgerFile() ([]LedgerRow, error) {
	path := ContributorLedgerPath()
	if path == "" {
		return nil, errLedgerNotConfigured
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errLedgerNotConfigured
		}
		return nil, fmt.Errorf("ledger %s: %w", path, err)
	}
	defer f.Close()

	var rows []LedgerRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var row LedgerRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("ledger %s: line %d: malformed row: %w", path, lineNo, err)
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("ledger %s: %w", path, err)
	}
	return rows, nil
}

// ResolveTier resolves the trust tier login (with id, its pinned numeric identity
// id — 0 means the caller has none) holds against repo.
//
// Resolution order (design record DR-trust-tiers-ledger):
//
//  1. Roster membership (trust.go / EffectiveConfig) resolves TierMaintainer. The
//     ledger is never consulted and cannot lower this; Provenance is
//     ProvenanceRoster.
//  2. The ledger row for repo + identity, matched on the STRICT pinned-id path:
//     the row's own recorded id must be non-zero and equal the supplied id,
//     mirroring trust.go's trustedContentAuthor / expectedID recycled-login
//     defence. A row with no pinned id, or a supplied id that does not match,
//     grants nothing — the strict path never falls back to login-only.
//     Provenance is ProvenanceLedgerRow.
//  3. TierUnknown otherwise — the fail-closed default. Provenance distinguishes
//     ProvenanceDefaultAbsent (no ledger configured, the file does not exist, or
//     it loaded cleanly with no matching row: a legitimate empty) from
//     ProvenanceDefaultUnreadable (the configured ledger could not be read or
//     parsed: an anomaly, announced on stderr, that STILL resolves unknown).
func ResolveTier(repo, login string, id int64) (Tier, Provenance) {
	l := strings.ToLower(strings.TrimSpace(login))
	r := strings.TrimSpace(repo)
	if l == "" || r == "" {
		return TierUnknown, ProvenanceDefaultAbsent
	}

	if c := EffectiveConfig(); c.Configured() && c.Logins[l] {
		return TierMaintainer, ProvenanceRoster
	}

	rows, err := contributorLedgerLoader()
	if err != nil {
		if errors.Is(err, errLedgerNotConfigured) {
			return TierUnknown, ProvenanceDefaultAbsent
		}
		fmt.Fprintf(os.Stderr, "trust-ledger: %v — resolving %s/%s to unknown\n", err, r, l)
		return TierUnknown, ProvenanceDefaultUnreadable
	}

	for _, row := range rows {
		if !strings.EqualFold(strings.TrimSpace(row.Repo), r) {
			continue
		}
		if strings.ToLower(strings.TrimSpace(row.Login)) != l {
			continue
		}
		// Strict path: a row with no pinned id (row.ID == 0) grants nothing, and
		// neither does a supplied id (id == 0) with nothing to compare it against
		// — the same "want != 0 && want == id" shape trust.go's
		// trustedContentAuthor enforces against login recycling.
		if row.ID == 0 || id == 0 || row.ID != id {
			continue
		}
		tier, ok := parseTier(row.Tier)
		if !ok {
			continue
		}
		return tier, ProvenanceLedgerRow
	}
	return TierUnknown, ProvenanceDefaultAbsent
}

// Capability is one named grant a tier may hold. The set is fixed and small on
// purpose: review-lane depth, fork continuous-integration approve-and-run
// posture, desk-automation eligibility, and changelog-proxy eligibility. No tier
// grants merge authority — merge stays a human act behind branch protection, and
// there is deliberately no Capability for it.
type Capability string

const (
	CapabilityReviewDepth    Capability = "review-depth"
	CapabilityForkCIAutoRun  Capability = "fork-ci-auto-run"
	CapabilityDeskAutomation Capability = "desk-automation"
	CapabilityChangelogProxy Capability = "changelog-proxy"
)

// capabilityTable is the whole grant, tier by tier, as DATA rather than scattered
// conditionals — so a reader (and TestTrustTierPublishedModelMatchesTable, which
// checks it against docs/contributor-trust.md) can see it in one place.
//
//   - TierUnknown grants nothing — today's bar.
//   - TierBlessedOnce grants review depth only: the one-time admission this tier
//     exists for is a claims-versus-diff check on that single item, never a
//     standing automation grant.
//   - TierContributor and TierMaintainer grant the full set. No caller is wired
//     to this table yet — follow-up work wires it — so TierMaintainer's entry
//     changes no existing behaviour: trust.go's roster checks are untouched and
//     are what every current caller still consults.
var capabilityTable = map[Tier][]Capability{
	TierUnknown:     {},
	TierBlessedOnce: {CapabilityReviewDepth},
	TierContributor: {CapabilityReviewDepth, CapabilityForkCIAutoRun, CapabilityDeskAutomation, CapabilityChangelogProxy},
	TierMaintainer:  {CapabilityReviewDepth, CapabilityForkCIAutoRun, CapabilityDeskAutomation, CapabilityChangelogProxy},
}

// Capabilities returns the sorted capability names t is granted — data a caller,
// or a test comparing against the published doc, can read without re-deriving
// conditionals.
func (t Tier) Capabilities() []string {
	caps := capabilityTable[t]
	out := make([]string, 0, len(caps))
	for _, c := range caps {
		out = append(out, string(c))
	}
	sort.Strings(out)
	return out
}
