package main

// Roster / authority configuration — statusgen's half.
//
// KEEP IN SYNC with the desk-tools deskkit roster loader. statusgen and
// desk-tools are separate Go modules that deliberately share no code (the
// documented-duplicate pattern), so this is a cross-tree duplicate of deskkit's
// loader. A behavioural coupling test binds the two READERS over a shared
// vector file, so the duplication cannot drift. A change to either copy must be
// made in both.
//
// The full rationale — why a repository/organization variable rather than a file
// in the repo, why the write class must not consult the environment, why an
// unset roster is CLOSED, and what the accepted residual is — lives in the
// deskkit copy's package comment and is not restated here. The short form:
//
//   - P1 unset is CLOSED: an absent or empty roster trusts nobody and blesses
//     nobody, loudly.
//   - P2 the source of truth lies OUTSIDE every ref the tools evaluate.
//   - P3 the effective value is echoed every run, in BOTH directions.
//   - P4 numeric-id pinning survives: the format is `login:id`, and the id is
//     MANDATORY for the blessing authority.
//
// statusgen's class is chosen per MODE (scanClassForMode). `--scan-issues` is the
// acting mode: it creates durable desk work items (placeholder files) that reach
// Next-up, where a worker acts on them, so it is WRITE CLASS and consults the
// config-home file ONLY — an ASSAY_* variable exported in the local environment has
// no effect on it, which is the 2026-08-04 split-by-action-class ruling, pinned by
// TestWriteToolsRefuseEnvRoster.
//
// Every OTHER mode (lint / check / record / report) reads the CI transport when the
// process is actually inside a GitHub Actions job. `statusgen --lint` runs in the
// consumer's CI on every PR and there is no config home on a runner, so without
// that route the human-login map was unreadable and the verifier floor failed on
// briefs the PR never touched (correctness review, finding 1).
//
// RESIDUAL — the place the environment does widen, stated rather than taken
// silently. An earlier draft of this comment claimed "an empty map is strictly
// stricter, so the environment cannot widen a decision through them". The first
// clause is true and the conclusion does not follow: an empty map is stricter, but
// a NON-EMPTY one supplied through the environment is strictly wider than no map,
// because ASSAY_HUMAN_LOGIN_MAP is consulted only to ACCEPT and its gate-clearing
// consumers do not consult Configured(). A steered agent running locally as the user
// can export GITHUB_ACTIONS=true alongside ASSAY_HUMAN_LOGIN_MAP and clear:
//
//	authorizedByVerifiedHuman (registers.go)  — an `authorized-by: human:<name>` key
//	                         whose name is unknown today becomes known.
//	corroborateStamps        (corroborate.go) — a `human:<name>` stamp flips from
//	                         MISSING-CORROBORATION to CORROBORATED, flipping
//	                         `--corroborate`'s exit status (1 only when anyMissing).
//
// TWO gates, both map-widened. `--corroborate` is one of them despite an earlier note
// dismissing it as "not a widening one — a mapping only names a login whose APPROVED
// review must then be found on the PR". The residual review measured that false: the
// actor setting the map also chooses which login the name resolves to, so any account
// already carrying an approval-shaped comment on the PR satisfies the barrier —
// including a shared agent account, which is the identity `human:<name>` exists to
// distinguish from a human. `--corroborate` is not `--scan-issues`, so
// scanClassForMode(false) puts it on the same ClassCI transport as the register gate.
//
// The verifierFloorFailure gate (attribution.go) is a THIRD map-widened gate. It
// clears a `human:<name>` token only when the name was EVER a confirmed human — in the
// CURRENT map here (via HumanLogin) OR in the FORMER-humans map (ASSAY_FORMER_HUMAN_LOGIN_MAP,
// via FormerHumanLogin) — and rejects a name confirmed by neither. So a mapped name
// clears the floor where an unmapped, never-confirmed one fails: the current map
// ACCEPT-widens the floor, exactly as it does the two gates above. (An earlier form,
// #104, cleared the floor on login SHAPE alone and so was NOT map-widened; that
// dropped the forgery rejection — a plausible but never-confirmed name cleared — and
// Ian ruled it too permissive. The floor now consults the map again, tightening back
// toward main, carving out only the leaver case via the former map.) The former map is
// consulted ONLY by the floor: the two identity gates above resolve through the
// current map alone, because a departed human cannot approve today.
//
// That is the whole of it, and the boundary is pinned across all three gates by
// TestCIHumanLoginMapResidualBoundary: the map admits no repo, blesses nobody, makes
// nobody a trusted author, and cannot reach any write/flip/dispatch surface, because
// every tool holding those is write-class and refuses the environment unconditionally.
//
// Because those three gates exist to establish that a HUMAN acted, the LOGIN half of
// each entry is validated the same way ASSAY_BLESS_LOGIN is: a value that renders as a
// bot/App/shared-agent account (scanLooksLikeBot) collapses the whole configuration,
// so `human:<name>` can never be pointed at the shared-agent identity the token exists
// to exclude. This is a strict mirror of the bless rule, not a full identity check —
// a real, non-bot-shaped account that is not a verified human still parses clean,
// exactly as the bless authority does; that residual is inherent to naming an account
// by login and is the reason the id-pinning and corroboration gates exist downstream.
//
// Closing the residual is NOT a matter of narrowing the check: requiring Configured()
// here would re-break the CI case above wherever the adopter sets only this one
// variable, which is the state a single-variable adopter is in today. It is an
// ops question (repo- vs org-level variables), not a
// code question this file can settle.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	scanEnvBlessLogin            = "ASSAY_BLESS_LOGIN"
	scanEnvTrustedLogins         = "ASSAY_TRUSTED_LOGINS"
	scanEnvTrustedBotSlugs       = "ASSAY_TRUSTED_BOT_SLUGS"
	scanEnvAllowedRepos          = "ASSAY_ALLOWED_REPOS"
	scanEnvHumanLoginMap         = "ASSAY_HUMAN_LOGIN_MAP"
	// scanEnvFormerHumanLoginMap carries the FORMER-humans map: name:login entries
	// for humans who WERE confirmed at some past point but have since left the
	// roster. Same format and bot-shape validation as ASSAY_HUMAN_LOGIN_MAP. It is
	// consulted ONLY by the verifier floor (FormerHumanLogin / confirmedHumanEver,
	// attribution.go): the floor clears a `human:<name>` token for a human confirmed
	// NOW (ASSAY_HUMAN_LOGIN_MAP) OR HISTORICALLY (this map), and rejects a name
	// confirmed by neither — the leaver carve-out that lets a departed human's
	// historical Verified stamps keep clearing the model-capability floor without
	// admitting a never-confirmed name. It is DELIBERATELY not consulted by the
	// identity gates (--corroborate, the register human-authorisation key): "only
	// confirmed humans can approve" resolves through the CURRENT map alone, because a
	// departed human cannot approve today. UNSET is the stricter direction (no former
	// human recognised). A consumer that drops a human from ASSAY_HUMAN_LOGIN_MAP
	// moves that entry here to preserve the boards that human historically verified.
	scanEnvFormerHumanLoginMap   = "ASSAY_FORMER_HUMAN_LOGIN_MAP"
	scanEnvRiskPathTriggersExtra = "ASSAY_RISK_PATH_TRIGGERS_EXTRA"
	// scanEnvRepoAliases is the boards' DISPLAY grouping override
	// (ASSAY_REPO_ALIASES). It is a DESK-only roster value: statusgen does not
	// group repos and never consumes it — but the desk tools and statusgen share
	// one roster.env, and an unknown key in the ASSAY_ namespace REFUSES the whole
	// configuration (parseConfig). So it must be RECOGNISED here, or a roster.env
	// that configures the boards' grouping would collapse every statusgen trust
	// decision. Recognised and otherwise ignored, exactly like the desk-only keys
	// deskkit recognises for statusgen. KEEP IN SYNC with
	// deskkit/rosterconfig.go's EnvRepoAliases.
	scanEnvRepoAliases = "ASSAY_REPO_ALIASES"
	// scanEnvReleaseRepo (ASSAY_RELEASE_REPO) and scanEnvWriteguardCallout
	// (ASSAY_WRITEGUARD_CALLOUT) are the other two DESK-only roster values
	// (deskrelease's release home, writeguard's dangerous-command callout).
	// statusgen consumes neither, but must RECOGNISE both for the same reason as
	// scanEnvRepoAliases: a shared roster.env carrying them must not collapse
	// statusgen's configuration. KEEP IN SYNC with deskkit/rosterconfig.go's
	// EnvReleaseRepo / EnvWriteguardCallout.
	scanEnvReleaseRepo       = "ASSAY_RELEASE_REPO"
	scanEnvWriteguardCallout = "ASSAY_WRITEGUARD_CALLOUT"
	scanEnvRosterSchema      = "ASSAY_ROSTER_SCHEMA"

	// scanEnvHomeRepo names the owner/repo whose issues get the bare
	// `issue-<NN>.md` placeholder filename and whose slug the rendered
	// --verify-issues bodies link into (verifyRepoSlug). A single owner/name
	// slug. Statusgen-specific: the repo roster used to be compiled in
	// (verifyRepoSlug / scanRepos), naming ONE organisation; publishing the
	// tools means the home repo is adopter configuration, read at runtime like
	// every other roster value. Unset is CLOSED for its consumers: --verify-issues
	// renders no absolute blob link and --scan-issues gives no repo the bare
	// filename (both harmless, and --scan-issues scans nothing without
	// ASSAY_SCAN_REPOS anyway).
	scanEnvHomeRepo = "ASSAY_HOME_REPO"
	// scanEnvScanRepos names the owner/repo set --scan-issues reads OPEN issues
	// from, comma/space-separated `owner/name`. It is DELIBERATELY a dedicated
	// variable, not ASSAY_ALLOWED_REPOS: the scan SCOPE and the write-authorisation
	// boundary are different sets on purpose — the intake scanner covers repos the
	// desk is the front door for (e.g. site repos) that are not write targets, and
	// deliberately HOLDS OUT a write-boundary repo whose PRs cannot yet be
	// risk-classed (see the scanRepos comment). Reusing ASSAY_ALLOWED_REPOS would
	// silently change the scan scope. Unset is CLOSED: an empty scan set scans
	// nothing rather than falling back to the write boundary.
	scanEnvScanRepos = "ASSAY_SCAN_REPOS"
	// scanEnvAuthorizedAuthors names the rostered AUTHORIZED-AUTHOR set the
	// scan-transcribe lane (transcribescan.go, issue-flow rulings R-7 clause 1)
	// admits placeholder CREATEs from. It is a comma/space-separated set of
	// login:id identities, the id MANDATORY (recycled-login defense — this set
	// gates whether an issue reaches the dispatch board unattended). It is a
	// DEDICATED value, deliberately NOT ASSAY_TRUSTED_LOGINS: the general trusted
	// set carries shared-agent and other identities that must NOT by themselves
	// board external work through the unattended lane, so R-7 boards over a
	// NARROWER, id-pinned human set. Desk App identities are trusted for boarding
	// too, but they come from ASSAY_TRUSTED_BOT_SLUGS (read the way deskroster
	// does), never from this value.
	//
	// SEEDED, never crashing: the effective authorized set is this value UNIONED
	// with the blessing authority (authorizedAuthorSet). An UNSET value therefore
	// degrades to the seeded default — the bless identity alone — rather
	// than to an empty set or a refusal. Widening is a roster EDIT, never an edit
	// to R-7 or any prose list. A bot-shaped entry is refused: App identities
	// belong in ASSAY_TRUSTED_BOT_SLUGS.
	scanEnvAuthorizedAuthors = "ASSAY_AUTHORIZED_AUTHORS"

	// scanEnvChannelDriftTarget (ASSAY_CHANNEL_DRIFT_TARGET) names the
	// repo-relative path to the accepted-channel-drift register the --lint
	// channel-conformance sweep consults (see channelconformance.go's
	// EnvChannelDriftPath). It is a de-housed WITHHELD-stream path: compiling it
	// in would disclose the withheld stream by name in a public copy of
	// statusgen, so the house supplies it at runtime. statusgen CONSUMES it, but
	// through channelconformance's direct os.Getenv read (the reporting-tool env
	// transport), NOT through this scanConfig struct. It must still be RECOGNISED
	// here so a shared roster.env FILE carrying the key for the in-house sweep
	// does not collapse statusgen's whole trust configuration on the
	// unknown-ASSAY_-key refusal — exactly as the desk-only keys above are
	// recognised. KEEP IN SYNC with deskkit/rosterconfig.go's EnvChannelDriftTarget.
	scanEnvChannelDriftTarget = "ASSAY_CHANNEL_DRIFT_TARGET"

	// scanEnvSweepWithheldStreams (ASSAY_SWEEP_WITHHELD_STREAMS) is a DESK-only
	// roster value: the repo-relative stream paths the desk's S2 disclosure
	// sweep routes away from (deskkit/sweepconfig.go). statusgen never consumes
	// it — but the two readers share one roster.env, and #1333's de-housing
	// REQUIRES the house to set this key for the sweep to route, so it WILL be
	// present in the shared roster.env. It must therefore be RECOGNISED here or a
	// roster.env that arms the desk sweep would collapse statusgen's whole trust
	// configuration on the unknown-ASSAY_-key refusal — exactly as the other
	// desk-only keys above are recognised. KEEP IN SYNC with
	// deskkit/rosterconfig.go's EnvSweepWithheldStreams (sweepconfig.go).
	scanEnvSweepWithheldStreams = "ASSAY_SWEEP_WITHHELD_STREAMS"

	// scanEnvDeterministicGatePatterns (ASSAY_DETERMINISTIC_GATE_PATTERNS) carries
	// additional house-specific deterministic-gate name substrings that the
	// autonomy report MERGES on top of its generic built-in set (autonomy.go's
	// EnvDeterministicGatePatterns / deterministicGatePatterns — a house's own
	// build- or contract-specific gate name). statusgen CONSUMES it, but through autonomy.go's
	// direct os.Getenv read (the reporting-tool env transport), NOT through this
	// scanConfig struct — exactly as ASSAY_CHANNEL_DRIFT_TARGET is consumed. It
	// must still be RECOGNISED here so a shared roster.env FILE carrying the key
	// for the in-house gate classification does not collapse statusgen's whole
	// trust configuration on the unknown-ASSAY_-key refusal — exactly as the
	// desk-only keys above are recognised. KEEP IN SYNC with
	// deskkit/rosterconfig.go's EnvDeterministicGatePatterns.
	scanEnvDeterministicGatePatterns = "ASSAY_DETERMINISTIC_GATE_PATTERNS"

	// The four keys below are DESK-only roster values that statusgen does not
	// consume, recognised here for the one reason every recognised-not-applied key
	// above is: the desk tools and statusgen read the SAME roster.env, and an
	// unrecognised key in the ASSAY_ namespace REFUSES the WHOLE configuration
	// (parseConfig). "Refuses the whole configuration" means every statusgen gate
	// that reads the roster reports it unconfigured and fails closed — so a roster
	// key that only the desk tools need is, until it is recognised here, a
	// fleet-wide outage of statusgen's trust-gated lanes. Recognising is NOT
	// applying: nothing below is read into scanConfig, and adding one here grants
	// statusgen no behaviour at all.
	//
	// They are held identical to deskkit's set by the shared key list in
	// statusgen/testdata/roster_coupling.json (scanKnownRosterKeys +
	// TestRosterKeySchemaCoupling, and deskkit's twin), so this comment is
	// documentation, not the enforcement.

	// scanEnvRepoForges (ASSAY_REPO_FORGES) binds `owner/name` to the forge that
	// serves it, and is consulted by the desk verbs' ForgeFor
	// (deskkit/forgeresolve.go) BEFORE its origin-remote-host fallback. It chooses
	// which minted credential a WRITE is performed as, so the desk tools cannot
	// operate on a repo whose remote host does not name its forge without it —
	// which makes it a key that WILL be present in any shared roster.env the desk
	// verbs are configured from. statusgen performs no forge-bound write and never
	// consumes it. KEEP IN SYNC with deskkit/rosterconfig.go's EnvRepoForges.
	scanEnvRepoForges = "ASSAY_REPO_FORGES"

	// scanEnvRiskCallout (ASSAY_RISK_CALLOUT) is the adopter-supplied
	// RISK-CLASSIFICATION callout: an absolute path to an executable the desk
	// tools exec as `<path> classify` and union with their pattern classifier
	// (deskkit/riskcallout.go). It is only-widens and fail-closed on the DESK
	// side; statusgen classifies no diffs and never consumes it. KEEP IN SYNC
	// with deskkit/rosterconfig.go's EnvRiskCallout.
	scanEnvRiskCallout = "ASSAY_RISK_CALLOUT"

	// scanEnvWithheldIdentifiers (ASSAY_WITHHELD_IDENTIFIERS) carries the
	// identifiers the desk's public-repo self-containment scan
	// (deskkit/selfcontain.go) refuses to let leave the house. The desk reads it
	// with a direct os.Getenv rather than through its roster struct — but a house
	// that configures it records it in the SAME shared roster.env, so an
	// unrecognised-key refusal here would mean turning that scan's register
	// category on collapses statusgen's whole configuration. statusgen never
	// consumes it. KEEP IN SYNC with deskkit/selfcontain.go's
	// EnvWithheldIdentifiers.
	scanEnvWithheldIdentifiers = "ASSAY_WITHHELD_IDENTIFIERS"

	// scanEnvAllowCluster (ASSAY_ALLOW_CLUSTER) is the desk clusterguard's
	// OPERATOR opt-in, read by that command with a direct os.Getenv because it is
	// a per-SHELL export an operator makes deliberately and a session must never
	// inherit. Recognised here — and only recognised — because an operator who
	// records it in the shared roster.env (the natural mistake: every other ASSAY_
	// knob lives there) would otherwise take statusgen's whole trust
	// configuration down. Recognised is emphatically not applied: it grants no
	// opt-in on either side from a file. KEEP IN SYNC with
	// deskkit/rosterconfig.go's EnvAllowCluster.
	scanEnvAllowCluster = "ASSAY_ALLOW_CLUSTER"
)

// scanKnownRosterKeys is the ASSAY_-namespace roster SCHEMA this binary speaks:
// every key parseConfig recognises. It is a function rather than a literal inside
// parseConfig so a test can read the set without re-deriving it, and so the set
// can be bound to the desk tools' twin (deskkit.KnownRosterKeys) over the shared
// vector file statusgen/testdata/roster_coupling.json.
//
// WHY THE BINDING EXISTS. Both binaries read the SAME ~/.config/assay/roster.env,
// and both REFUSE the whole configuration on an ASSAY_ key they do not recognise
// (an unapplied key leaves a control surface empty while the configuration reports
// itself correct — see parseConfig). So a key one binary knows and the other does
// not is not a cosmetic difference: it makes a roster that is valid and REQUIRED
// for one tool a total refusal for the other, with no roster edit able to satisfy
// both. That is exactly what happened when the desk tools grew ASSAY_REPO_FORGES
// and this set did not follow.
//
// RECOGNISED IS NOT APPLIED. Most of these are consumed by only one of the two
// readers; each constant's own comment names who consumes it and why the other
// must not fail closed on it. Adding a key here does NOT make statusgen consume
// it.
//
// Product-namespaced (non-ASSAY_) config keys are NOT listed here: they are
// supplied by build-tagged product hooks (scanProductConfigKeys /
// scanApplyProductConfig / scanProductConfigLines), are absent from the default
// (open-core) build, and sit outside the namespace whose unknown keys refuse. See
// openstub.go for the no-op defaults.
func scanKnownRosterKeys() []string {
	return []string{
		scanEnvBlessLogin, scanEnvTrustedLogins, scanEnvTrustedBotSlugs,
		scanEnvAllowedRepos, scanEnvHumanLoginMap, scanEnvFormerHumanLoginMap,
		scanEnvRiskPathTriggersExtra,
		scanEnvRepoAliases, scanEnvReleaseRepo, scanEnvWriteguardCallout,
		scanEnvRosterSchema, scanEnvHomeRepo, scanEnvScanRepos,
		scanEnvAuthorizedAuthors, scanEnvChannelDriftTarget,
		scanEnvSweepWithheldStreams, scanEnvDeterministicGatePatterns,
		// DESK-only, recognised-not-applied — see their declarations above for who
		// consumes each and why statusgen must not fail closed on it.
		scanEnvRepoForges, scanEnvRiskCallout,
		scanEnvWithheldIdentifiers, scanEnvAllowCluster,
	}
}

// scanConfigHomeFile is the local source of truth — outside every ref.
const scanConfigHomeFile = "~/.config/assay/roster.env"

// scanRosterSchemaVersion is the format version this build speaks.
// KEEP IN SYNC with deskkit.rosterSchemaVersion.
const scanRosterSchemaVersion = "1"

// scanToolClass mirrors deskkit.ToolClass. scanClassWrite is the ZERO VALUE, so a
// caller that forgets to choose gets the restrictive class.
type scanToolClass int

const (
	scanClassWrite scanToolClass = iota
	scanClassCI
	scanClassReadOnly
)

func (c scanToolClass) String() string {
	switch c {
	case scanClassCI:
		return "ci"
	case scanClassReadOnly:
		return "read-only"
	default:
		return "write"
	}
}

// scanIdentity is a login with its PERMANENT numeric GitHub id (0 = not pinned).
type scanIdentity struct {
	Login string
	ID    int64
}

// scanConfig is one fully-parsed, validated configuration. A scanConfig with a
// non-empty Problems slice carries NOTHING else: validation failure collapses the
// whole configuration to unconfigured rather than admitting a half-parsed roster.
type scanConfig struct {
	Class  scanToolClass
	Source string

	Bless    scanIdentity
	Humans   map[string]int64
	Bots     map[string]int64
	RoleBots map[string]string
	Logins   map[string]bool

	Repos       map[string]string
	HumanLogins map[string]string
	// FormerHumanLogins is the FORMER-humans map (ASSAY_FORMER_HUMAN_LOGIN_MAP):
	// name -> login for departed humans who were confirmed at some past point. It is
	// consulted ONLY by the verifier floor (FormerHumanLogin), never by the identity
	// gates — see the scanEnvFormerHumanLoginMap doc.
	FormerHumanLogins map[string]string
	RiskExtra         []string

	// HomeRepo and ScanRepos are statusgen-only roster values (the compiled-in
	// owned-repo roster this externalises). deskkit RECOGNISES their keys — the
	// two readers share one roster.env, so an unknown ASSAY_ key would collapse
	// the OTHER reader's whole configuration — but does not consume them.
	HomeRepo  string
	ScanRepos []string

	// AuthorizedAuthors is the rostered AUTHORIZED-AUTHOR set (ASSAY_AUTHORIZED_AUTHORS),
	// the R-7 clause-1 boarding predicate's human half (login -> mandatory numeric
	// id). Statusgen-only; deskkit RECOGNISES the key (shared roster.env) but does
	// not consume it. Read through authorizedAuthorSet(), which SEEDS it with the
	// bless identity so an unset value degrades to {the bless identity}, never to empty.
	AuthorizedAuthors map[string]int64

	// Product holds product-namespaced (non-ASSAY_) config values, populated by
	// the build-tagged scanApplyProductConfig hook. Empty in the default
	// (open-core) build. Not trust-bearing — a value here never collapses the
	// roster (these are not ASSAY_ keys). The product build reads it through its
	// own typed accessors behind a build tag.
	Product map[string]string

	// UnknownKeys are echoed, never applied. KEEP IN SYNC with deskkit.Config.
	UnknownKeys []string

	Problems []string
}

func (c scanConfig) Configured() bool {
	return len(c.Problems) == 0 && c.Bless.Login != "" && len(c.Logins) > 0
}

var (
	scanCfgMu    sync.Mutex
	scanCfgClass scanToolClass
	scanCfgOnce  bool
	scanCfgValue scanConfig
)

// scanReloadConfig discards the cached configuration so the next read re-parses.
func scanReloadConfig() {
	scanCfgMu.Lock()
	defer scanCfgMu.Unlock()
	scanCfgOnce = false
}

// scanInCI reports whether this process is running inside a GitHub Actions job.
// KEEP IN SYNC with deskkit.InCI.
func scanInCI() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("GITHUB_ACTIONS")), "true")
}

// scanClassForMode resolves the roster class from the MODE statusgen was invoked
// in. KEEP IN SYNC with deskkit.ClassForTool.
//
// This is the statusgen half of correctness finding 1: there was no
// setter at all on this side, so scanCfgClass was a package var nothing ever wrote
// and every invocation loaded ClassWrite — file-only. `statusgen --lint` runs in
// the consumer's CI on every PR (.github/workflows/statusgen.yml), where there is
// no config home and never will be, so the human-login map was unreadable and the
// verifier floor failed on briefs the PR never touched.
//
// The split is by what the MODE does, not by where it runs:
//
//	--scan-issues  WRITES placeholder briefs into the work queue from GitHub issue
//	               text. It is the acting mode, it is run locally, and it stays
//	               file-only ALWAYS — including under GITHUB_ACTIONS. This is the
//	               tool whose steering the split-by-action-class ruling is about.
//	everything else  lint / check / record / report. These classify and print; they
//	               grant nothing DIRECTLY, admit nothing to a queue, and write no
//	               authorisation. An EMPTY human-login map makes them stricter, not
//	               looser — but a NON-EMPTY one arriving through the environment IS a
//	               widening (it is consulted only to ACCEPT), clearing the three human
//	               gates enumerated in the RESIDUAL note above. Reporting is not
//	               decision-neutral; do not restate that as "the environment cannot
//	               widen a decision".
func scanClassForMode(scanIssuesMode bool) scanToolClass {
	if scanIssuesMode || !scanInCI() {
		return scanClassWrite
	}
	return scanClassCI
}

// scanSetToolClass declares the invocation's class. It must be called before any
// roster read; the zero value is scanClassWrite, the restrictive default.
func scanSetToolClass(c scanToolClass) {
	scanCfgMu.Lock()
	defer scanCfgMu.Unlock()
	scanCfgClass = c
	scanCfgOnce = false
}

// scanEffectiveConfig returns the process's configuration, loading on first use.
func scanEffectiveConfig() scanConfig {
	scanCfgMu.Lock()
	defer scanCfgMu.Unlock()
	if !scanCfgOnce {
		scanCfgValue = scanLoadConfig(scanCfgClass)
		// Wire the deployment's risk vocabulary into the placeholder gate from the
		// config it already sets (ASSAY_RISK_PATH_TRIGGERS_EXTRA). The shipped
		// open-core default names no product domain; this re-adds the deployment's
		// own risk words so a risk-labelled/-titled placeholder still derives
		// gate: human. Idempotent (rebuilds from the neutral base each load), so an
		// unset config leaves the neutral default in place.
		kw := riskKeywordsFromPathTriggers(scanCfgValue.RiskExtra)
		registerRiskGateVocabulary(kw, kw)
		scanCfgOnce = true
	}
	return scanCfgValue
}

// scanLoadConfig reads and validates the configuration for class. It never
// returns an error: an unreadable, absent or invalid configuration is an
// UNCONFIGURED one with Problems recorded, because every caller has to fail
// closed on that state anyway and an error return invites a caller that logs and
// continues.
func scanLoadConfig(class scanToolClass) scanConfig {
	vals, source, problems := scanReadRawConfig(class)
	cfg := scanParseConfig(class, source, vals)
	if len(problems) > 0 {
		return scanConfig{Class: class, Source: source, Problems: append(problems, cfg.Problems...)}
	}
	return cfg
}

func scanReadRawConfig(class scanToolClass) (map[string]string, string, []string) {
	keys := []string{
		scanEnvBlessLogin, scanEnvTrustedLogins, scanEnvTrustedBotSlugs,
		scanEnvAllowedRepos, scanEnvHumanLoginMap, scanEnvFormerHumanLoginMap,
		scanEnvRiskPathTriggersExtra,
		scanEnvRepoAliases, scanEnvRosterSchema, scanEnvHomeRepo, scanEnvScanRepos,
		scanEnvAuthorizedAuthors,
	}
	keys = append(keys, scanProductConfigKeys()...)
	fromEnv := func() map[string]string {
		m := map[string]string{}
		for _, k := range keys {
			if v := strings.TrimSpace(os.Getenv(k)); v != "" {
				m[k] = v
			}
		}
		return m
	}

	switch class {
	case scanClassCI:
		return fromEnv(), "environment (repository/organization Actions variables)", nil

	case scanClassReadOnly:
		file, path, ferr := scanReadConfigFile()
		m := fromEnv()
		src := "environment"
		if ferr == nil {
			for k, v := range file {
				if _, ok := m[k]; !ok {
					m[k] = v
				}
			}
			src = "environment, then " + path
		} else if !os.IsNotExist(ferr) {
			src = "environment (config file unusable: " + ferr.Error() + ")"
		}
		return m, src, nil

	default: // scanClassWrite
		// NO ENVIRONMENT READ — the whole point of the split.
		file, path, ferr := scanReadConfigFile()
		if ferr != nil {
			if os.IsNotExist(ferr) {
				return nil, "config file " + path + " (absent)", []string{fmt.Sprintf(
					"no roster configured: %s does not exist. Write-class tools read the roster "+
						"ONLY from this file — the environment is deliberately not consulted, so "+
						"exporting %s has no effect. Create it with 0600 permissions.",
					path, scanEnvTrustedLogins)}
			}
			return nil, "config file " + path + " (unusable)", []string{fmt.Sprintf(
				"refusing to load the roster from %s: %v", path, ferr)}
		}
		return file, "config file " + path, nil
	}
}

// scanConfigHomePath is the resolved path of the local config file.
func scanConfigHomePath() string { return scanExpandHome(scanConfigHomeFile) }

func scanExpandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

func scanReadConfigFile() (map[string]string, string, error) {
	path := scanConfigHomePath()
	if err := scanCheckOwnerPerms(filepath.Dir(path), true); err != nil {
		return nil, path, err
	}
	if err := scanCheckOwnerPerms(path, false); err != nil {
		return nil, path, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, err
	}
	return scanParseDotenv(string(data)), path, nil
}

// scanCheckOwnerPerms is the sshd rule: a configuration that decides who is
// trusted must not be writable by anyone but its owner, and must be owned by the
// user running the tool.
func scanCheckOwnerPerms(path string, isDir bool) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if isDir && !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	if mode := fi.Mode().Perm(); mode&0o022 != 0 {
		kind := "file"
		fix := "0600"
		if isDir {
			kind = "directory"
			fix = "0700"
		}
		return fmt.Errorf("roster config %s %s is group- or world-writable (mode %04o): "+
			"anything that can write it can name the accounts this tool trusts. "+
			"Fix with `chmod %s %s`", kind, path, mode, fix, path)
	}
	// Owner check is platform-specific: unix compares the owning uid; windows has
	// no uid and skips it LOUDLY (see rosterowner_{unix,windows}.go). The
	// group/world-writable mode check above runs on both platforms.
	return checkFileOwner(path, fi)
}

func scanParseDotenv(s string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		if v := strings.TrimSpace(val); v != "" {
			m[key] = v
		}
	}
	return m
}

func scanSplitList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func scanSplitIdentity(entry string) (login string, id int64, ok bool) {
	l, rest, found := strings.Cut(entry, ":")
	l = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "@")))
	if l == "" {
		return "", 0, false
	}
	if !found || strings.TrimSpace(rest) == "" {
		return l, 0, true
	}
	n, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return l, n, true
}

// scanLooksLikeBot mirrors deskkit.looksLikeBot.
// The authorisation half of a two-factor mechanism has to be a human act, and a
// blessing IS that authorisation half.
func scanLooksLikeBot(login string) bool {
	l := strings.ToLower(strings.TrimSpace(login))
	return strings.HasSuffix(l, "[bot]") ||
		strings.HasPrefix(l, "app/") ||
		strings.HasSuffix(l, "-app") ||
		strings.HasSuffix(l, "-bot")
}

func scanParseConfig(class scanToolClass, source string, vals map[string]string) scanConfig {
	cfg := scanConfig{
		Class:             class,
		Source:            source,
		Humans:            map[string]int64{},
		Bots:              map[string]int64{},
		RoleBots:          map[string]string{},
		Logins:            map[string]bool{},
		Repos:             map[string]string{},
		HumanLogins:       map[string]string{},
		FormerHumanLogins: map[string]string{},
		AuthorizedAuthors: map[string]int64{},
	}
	var problems []string
	bad := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	// Key recognition BEFORE any value is applied. A typo inside the ASSAY_
	// namespace REFUSES (it would otherwise leave a control surface empty while the
	// configuration reported itself correct); a key outside it is echoed.
	known := map[string]bool{}
	for _, k := range scanKnownRosterKeys() {
		known[k] = true
	}
	for _, k := range scanProductConfigKeys() {
		known[k] = true
	}
	for k := range vals {
		if known[k] {
			continue
		}
		if strings.HasPrefix(k, "ASSAY_") {
			bad("%s: unknown key in the ASSAY_ namespace. It is not applied, so whatever it was "+
				"meant to configure is EMPTY — and an empty control surface that reports itself "+
				"configured is the failure this refusal exists to prevent", k)
			continue
		}
		cfg.UnknownKeys = append(cfg.UnknownKeys, k)
	}
	sort.Strings(cfg.UnknownKeys)

	if v := strings.TrimSpace(vals[scanEnvRosterSchema]); v != "" && v != scanRosterSchemaVersion {
		bad("%s=%q is not a schema version this build understands (it speaks %q). Refusing: a "+
			"file written for a different format cannot be safely half-applied",
			scanEnvRosterSchema, v, scanRosterSchemaVersion)
	}

	for _, entry := range scanSplitList(vals[scanEnvTrustedBotSlugs]) {
		role := ""
		if r, rest, found := strings.Cut(entry, "="); found {
			role = strings.ToLower(strings.TrimSpace(r))
			entry = rest
		}
		slug, id, ok := scanSplitIdentity(entry)
		if !ok {
			bad("%s: cannot parse entry %q — expected [role=]slug[:id] with a positive numeric id",
				scanEnvTrustedBotSlugs, entry)
			continue
		}
		cfg.Bots[slug] = id
		// BOTH GitHub renderings; the BARE slug never (username-squatting fail-close).
		cfg.Logins[slug+"[bot]"] = true
		cfg.Logins["app/"+slug] = true
		if role != "" {
			cfg.RoleBots[role] = slug
		}
	}

	for _, entry := range scanSplitList(vals[scanEnvTrustedLogins]) {
		login, id, ok := scanSplitIdentity(entry)
		if !ok {
			bad("%s: cannot parse entry %q — expected login[:id] with a positive numeric id",
				scanEnvTrustedLogins, entry)
			continue
		}
		if scanLooksLikeBot(login) {
			bad("%s lists %q, which renders as a bot/App account. App identities belong in %s",
				scanEnvTrustedLogins, login, scanEnvTrustedBotSlugs)
			continue
		}
		cfg.Humans[login] = id
		cfg.Logins[login] = true
	}

	if raw := strings.TrimSpace(vals[scanEnvBlessLogin]); raw != "" {
		if strings.ContainsAny(raw, ",; \t\n\r") {
			bad("%s=%q contains a separator. It is a SINGLE identity, never a list: blessing the "+
				"first entry (or every entry) of a list would silently grant the authority this "+
				"variable exists to restrict. Refusing, exactly as if it were unset",
				scanEnvBlessLogin, raw)
		} else {
			login, id, ok := scanSplitIdentity(raw)
			_, inBotSet := cfg.Bots[login]
			switch {
			case !ok:
				bad("%s=%q cannot be parsed — expected login:id with a positive numeric id",
					scanEnvBlessLogin, raw)
			case id == 0:
				bad("%s=%q carries no numeric id. The id is MANDATORY for the blessing authority "+
					"(unlike its siblings): a deleted login can be re-registered by an attacker, and "+
					"the blessing authority is the highest-value target for that. Refusing, exactly "+
					"as if it were unset", scanEnvBlessLogin, raw)
			case scanLooksLikeBot(login):
				bad("%s=%q names a bot/App/shared-agent account. The blessing is the authorisation "+
					"half of the trust gate and has to be a HUMAN act — an App that could bless would "+
					"admit external text into its own queue. Refusing", scanEnvBlessLogin, raw)
			case inBotSet:
				bad("%s=%q is also listed in %s. A trusted App is a trusted AUTHOR; it must never be "+
					"the blessing authority", scanEnvBlessLogin, raw, scanEnvTrustedBotSlugs)
			default:
				cfg.Bless = scanIdentity{Login: login, ID: id}
				cfg.Humans[login] = id
				cfg.Logins[login] = true
			}
		}
	}

	// The policy tokens are VALIDATED here, exactly as deskkit validates them.
	//
	// They were not, and that was the sharpest finding of the correctness review:
	// one roster file with one typo (`…assay:cii:private`) made
	// every desk tool refuse everything — "unknown policy token" collapses the whole
	// configuration on that side — while statusgen stored the token unparsed and
	// reported configured=true. Two readers, one file, opposite verdicts. The
	// PROPERTY the package comment claims ("validation failure collapses the WHOLE
	// configuration") was simply false in this copy, and the coupling vector could
	// not see it because it carried only the three trust variables.
	//
	// KEEP IN SYNC with deskkit's parseConfig.
	for _, entry := range scanSplitList(vals[scanEnvAllowedRepos]) {
		parts := strings.Split(entry, ":")
		slug := strings.TrimSpace(parts[0])
		if strings.Count(slug, "/") != 1 || strings.HasPrefix(slug, "/") || strings.HasSuffix(slug, "/") {
			bad("%s: %q is not an owner/name repo slug", scanEnvAllowedRepos, entry)
			continue
		}
		// Fail-closed defaults, identical to deskkit: CI required unless stated,
		// visibility UNKNOWN unless stated (unknown risk-classes every PR).
		ci, vis := "ci", "unknown"
		malformed := false
		for _, tok := range parts[1:] {
			switch strings.ToLower(strings.TrimSpace(tok)) {
			case "ci":
				ci = "ci"
			case "no-ci":
				ci = "no-ci"
			case "public":
				vis = "public"
			case "private":
				vis = "private"
			case "":
			default:
				bad("%s: %q carries unknown policy token %q — expected ci|no-ci and public|private",
					scanEnvAllowedRepos, slug, tok)
				malformed = true
			}
		}
		if malformed {
			continue
		}
		cfg.Repos[slug] = ci + ":" + vis
	}

	for _, entry := range scanSplitList(vals[scanEnvHumanLoginMap]) {
		name, login, found := strings.Cut(entry, ":")
		name = strings.ToLower(strings.TrimSpace(name))
		login = strings.ToLower(strings.TrimSpace(login))
		if !found || name == "" || login == "" {
			bad("%s: cannot parse entry %q — expected name:login", scanEnvHumanLoginMap, entry)
			continue
		}
		if scanLooksLikeBot(login) {
			bad("%s: entry %q maps to a bot/App/shared-agent login %q. The human-login map "+
				"exists to establish that a HUMAN acted — it clears the verifier floor, the "+
				"register human-authorisation check and --corroborate — so a bot/App value would "+
				"let `human:<name>` resolve to the shared-agent identity the token exists to "+
				"exclude. Refusing", scanEnvHumanLoginMap, entry, login)
			continue
		}
		cfg.HumanLogins[name] = login
	}

	// The FORMER-humans map, parsed and validated EXACTLY like the current map above
	// (name:login, bot-shape rejected). It is consulted only by the verifier floor
	// (FormerHumanLogin) as the "confirmed historically" half of the human-token
	// exemption — never by the identity gates. The bot-shape refusal matters for the
	// same reason it does on the current map: a `human:<name>` must never resolve to a
	// shared-agent identity, even for a departed human. An unset value is neither an
	// error nor a refusal.
	for _, entry := range scanSplitList(vals[scanEnvFormerHumanLoginMap]) {
		name, login, found := strings.Cut(entry, ":")
		name = strings.ToLower(strings.TrimSpace(name))
		login = strings.ToLower(strings.TrimSpace(login))
		if !found || name == "" || login == "" {
			bad("%s: cannot parse entry %q — expected name:login", scanEnvFormerHumanLoginMap, entry)
			continue
		}
		if scanLooksLikeBot(login) {
			bad("%s: entry %q maps to a bot/App/shared-agent login %q. The former-humans map "+
				"clears the verifier floor for a departed human, so a bot/App value would let "+
				"`human:<name>` resolve to the shared-agent identity the token exists to exclude. "+
				"Refusing", scanEnvFormerHumanLoginMap, entry, login)
			continue
		}
		cfg.FormerHumanLogins[name] = login
	}

	// ADDITIVE-ONLY, and an unset value is neither an error nor a refusal.
	cfg.RiskExtra = scanSplitList(vals[scanEnvRiskPathTriggersExtra])
	sort.Strings(cfg.RiskExtra)

	// The owned-repo roster (statusgen-only). A malformed slug REFUSES the whole
	// configuration, exactly as a malformed ASSAY_ALLOWED_REPOS entry does: a
	// half-parsed repo roster is the configured-but-wrong shape this design exists
	// to prevent. An UNSET value is neither an error nor a refusal — its consumers
	// fail safe on empty (verifyRepoSlug renders no absolute link, scanRepos scans
	// nothing).
	validSlug := func(s string) bool {
		return strings.Count(s, "/") == 1 && !strings.HasPrefix(s, "/") && !strings.HasSuffix(s, "/")
	}
	if home := strings.TrimSpace(vals[scanEnvHomeRepo]); home != "" {
		if !validSlug(home) {
			bad("%s: %q is not an owner/name repo slug", scanEnvHomeRepo, home)
		} else {
			cfg.HomeRepo = home
		}
	}
	for _, entry := range scanSplitList(vals[scanEnvScanRepos]) {
		if !validSlug(entry) {
			bad("%s: %q is not an owner/name repo slug", scanEnvScanRepos, entry)
			continue
		}
		cfg.ScanRepos = append(cfg.ScanRepos, entry)
	}

	// The rostered AUTHORIZED-AUTHOR set (R-7 cl.1 boarding predicate). login:id,
	// the id MANDATORY: this set governs which authors reach the dispatch board via
	// the unattended scan-transcribe lane, so an unpinned login (recycling-defensible
	// only by its id) is refused, exactly as the blessing authority is. A bot-shaped
	// entry is refused — App identities are trusted for boarding through
	// ASSAY_TRUSTED_BOT_SLUGS, never here (a bot in the human boarding set would let
	// an App both file and board its own work). An UNSET value is neither an error
	// nor a refusal: authorizedAuthorSet() seeds the effective set with the bless
	// identity, so the lane always boards at least the bless identity and never crashes on an
	// empty roster value.
	for _, entry := range scanSplitList(vals[scanEnvAuthorizedAuthors]) {
		login, id, ok := scanSplitIdentity(entry)
		if !ok {
			bad("%s: cannot parse entry %q — expected login:id with a positive numeric id",
				scanEnvAuthorizedAuthors, entry)
			continue
		}
		if scanLooksLikeBot(login) {
			bad("%s lists %q, which renders as a bot/App account. Desk App identities are trusted "+
				"for boarding through %s, never through the human authorized-author set",
				scanEnvAuthorizedAuthors, login, scanEnvTrustedBotSlugs)
			continue
		}
		if id == 0 {
			bad("%s=%q carries no numeric id. The id is MANDATORY for an authorized author: this "+
				"set decides who reaches the dispatch board unattended, and a deleted login can be "+
				"re-registered by an attacker. Refusing the entry", scanEnvAuthorizedAuthors, entry)
			continue
		}
		cfg.AuthorizedAuthors[login] = id
	}

	// Product-namespaced (non-ASSAY_) config, applied by the build-tagged hook.
	// Deliberately NOT run through bad(): product config, not trust config, so a
	// missing or malformed value must not collapse the roster. No-op in the
	// default (open-core) build.
	scanApplyProductConfig(&cfg, vals)

	if len(problems) > 0 {
		return scanConfig{Class: class, Source: source, Problems: problems}
	}
	return cfg
}

// ---- the effective-value echo (P3) -------------------------------------------

func (c scanConfig) EffectiveConfigLines() []string {
	ident := func(login string, id int64) string {
		if id == 0 {
			return login
		}
		return login + ":" + strconv.FormatInt(id, 10)
	}
	sortedIdents := func(m map[string]int64) string {
		out := make([]string, 0, len(m))
		for l, id := range m {
			out = append(out, ident(l, id))
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	blessStr := "(unset)"
	if c.Bless.Login != "" {
		blessStr = ident(c.Bless.Login, c.Bless.ID)
	}
	repos := make([]string, 0, len(c.Repos))
	for r, pol := range c.Repos {
		if pol == "" {
			repos = append(repos, r)
			continue
		}
		repos = append(repos, r+":"+pol)
	}
	sort.Strings(repos)
	humanMap := make([]string, 0, len(c.HumanLogins))
	for n, l := range c.HumanLogins {
		humanMap = append(humanMap, n+"="+l)
	}
	sort.Strings(humanMap)
	formerHumanMap := make([]string, 0, len(c.FormerHumanLogins))
	for n, l := range c.FormerHumanLogins {
		formerHumanMap = append(formerHumanMap, n+"="+l)
	}
	sort.Strings(formerHumanMap)

	// Role bindings get their own line: the bot-slug line renders slug:id and says
	// nothing about which role each slug is BOUND to, so a dropped `role=` prefix
	// was invisible here as well as at load. KEEP IN SYNC with deskkit.
	roles := make([]string, 0, len(c.RoleBots))
	for r, slug := range c.RoleBots {
		roles = append(roles, r+"="+slug)
	}
	sort.Strings(roles)
	rolesStr := strings.Join(roles, ",")
	if rolesStr == "" {
		rolesStr = "(none bound — every role-gated identity check REFUSES)"
	}

	lines := []string{
		fmt.Sprintf("assay-config: class=%s source=%s configured=%t", c.Class, c.Source, c.Configured()),
		fmt.Sprintf("assay-config: %s=%s", scanEnvBlessLogin, blessStr),
		fmt.Sprintf("assay-config: %s=%s", scanEnvTrustedLogins, sortedIdents(c.Humans)),
		fmt.Sprintf("assay-config: %s=%s", scanEnvTrustedBotSlugs, sortedIdents(c.Bots)),
		fmt.Sprintf("assay-config: role-bindings=%s", rolesStr),
		fmt.Sprintf("assay-config: %s=%s", scanEnvAllowedRepos, strings.Join(repos, ",")),
		fmt.Sprintf("assay-config: %s=%s", scanEnvHumanLoginMap, strings.Join(humanMap, ",")),
		fmt.Sprintf("assay-config: %s=%s", scanEnvFormerHumanLoginMap, strings.Join(formerHumanMap, ",")),
		fmt.Sprintf("assay-config: %s=%s", scanEnvRiskPathTriggersExtra, strings.Join(c.RiskExtra, ",")),
		fmt.Sprintf("assay-config: %s=%s", scanEnvHomeRepo, c.HomeRepo),
		fmt.Sprintf("assay-config: %s=%s", scanEnvScanRepos, strings.Join(c.ScanRepos, ",")),
		fmt.Sprintf("assay-config: %s=%s", scanEnvAuthorizedAuthors, sortedIdents(c.AuthorizedAuthors)),
	}
	// Product config (non-ASSAY_) echoes under its own prefix via the build-tagged
	// hook; empty in the default (open-core) build.
	lines = append(lines, scanProductConfigLines(c)...)
	if len(c.UnknownKeys) > 0 {
		lines = append(lines, fmt.Sprintf("assay-config: unrecognised-keys=%s (IGNORED)",
			strings.Join(c.UnknownKeys, ",")))
	}
	for _, p := range c.Problems {
		lines = append(lines, "assay-config: REFUSED — "+p)
	}
	return lines
}

func scanEchoEffectiveConfig(w io.Writer) {
	for _, l := range scanEffectiveConfig().EffectiveConfigLines() {
		fmt.Fprintln(w, l)
	}
}

// scanRosterUnconfiguredError is the loud refusal --scan-issues emits when no
// usable roster was loaded. It never returns nil for an unconfigured roster:
// "nothing configured ⇒ nothing checked" is the one degradation this design
// exists to prevent.
func scanRosterUnconfiguredError() error {
	c := scanEffectiveConfig()
	if c.Configured() {
		return nil
	}
	detail := strings.Join(c.Problems, "\n  - ")
	if detail == "" {
		detail = fmt.Sprintf("%s and %s are unset", scanEnvBlessLogin, scanEnvTrustedLogins)
	}
	return fmt.Errorf("the trust roster is NOT CONFIGURED, so nothing is trusted and nothing can be "+
		"blessed (fail closed):\n  - %s\nSource consulted: %s (class %s).\n"+
		"--scan-issues creates durable desk work items from GitHub issues, including issues on "+
		"PUBLIC repos that arbitrary external users can author, so it refuses to run without a "+
		"roster rather than scanning ungated.\n"+
		"Write %s / %s / %s to %s, mode 0600. This tool is WRITE CLASS: it does NOT read the "+
		"environment, so exporting those names has no effect.",
		detail, c.Source, c.Class,
		scanEnvBlessLogin, scanEnvTrustedLogins, scanEnvTrustedBotSlugs, scanConfigHomePath())
}
