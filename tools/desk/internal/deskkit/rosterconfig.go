package deskkit

// Roster / authority configuration.
//
// WHAT MOVED AND WHY. The trust roster, the write-authorisation repo set, the
// human-name→login map and the risk-path additions used to be compiled-in
// constants naming ONE organisation. Publishing the tools means a different
// organisation must be able to name a different human — without losing the
// property the compiled-in form was providing.
//
// THE PROPERTY (P2). The source of truth lies OUTSIDE every ref the tools
// evaluate. Not a file in the work tree, not a CLI flag, not a PR-supplied
// input — nothing a pull request can author. The principle:
//
//	an escape settable by the population it constrains is not a guard. A
//	CODEOWNERS file, a .releasers file, or a list inline in this source tree
//	all live in the diff — the very PR being refused could add its own author
//	to the allowlist in the same commit. A repository Actions variable is read
//	from repository settings, is not part of any ref, and is writable only with
//	repo-admin rights. A PR cannot reach it.
//
// In CI that is a repository/organization Actions variable, surfaced to the
// process as an environment variable by the workflow. On a developer laptop the
// same property is met by a file under the config home, equally outside every
// ref.
//
// THE SPLIT BY ACTION CLASS. A locally-run tool that ACTS —
// posts, flips, dispatches, or writes a work item — must NOT consult the
// environment for the roster: a local env var is one injected sentence away from
// an agent steered by untrusted text. Such tools read the config-home file ONLY.
// A genuinely READ-ONLY tool may read env-first with a config-home fallback.
// ClassWrite is the ZERO VALUE, so the restrictive class is what a caller gets
// by forgetting to choose — the default is the safe one, not the convenient one.
//
// IS A LOCALLY-WRITTEN CONFIG FILE TRUSTED INPUT? Partly, under constraints, with
// the residual stated rather than silently taken:
//   - the loader enforces the sshd rule (below): a group/world-writable file or
//     directory, or one owned by another user, is REFUSED;
//   - no env is consulted on the write path, so the one-sentence route is closed;
//   - the effective value is echoed every run (P3), so a change is visible.
//   - RESIDUAL, recorded: a steered agent running AS the user can still write the
//     file, as it can already write any user-owned file. The compiled-in constant
//     was the only form that made that impossible, and publication requires the
//     move.
//
// FAIL-CLOSED (P1). Unset is CLOSED, not open. An absent or empty roster trusts
// nobody and blesses nobody; it never degrades to "no roster ⇒ no filtering".
// Any validation problem collapses the WHOLE configuration to unconfigured and
// records a loud reason — a partially-parsed roster is exactly the
// configured-but-empty shape this design exists to prevent.
//
// KEEP IN SYNC with statusgen/rosterconfig.go. statusgen and desk-tools are
// separate Go modules that deliberately share no code (the documented-duplicate
// pattern); TestScanIssuesTrustGateEnforced binds the two READERS behaviourally
// so the duplication cannot drift.

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

// The configured control surfaces. These names are FIXED: external checks set
// them by name, so renaming one silently turns those checks into assertions
// about nothing.
const (
	// EnvBlessLogin carries the SINGLE blessing authority as `login:id`. The id
	// is MANDATORY here (unlike its siblings): the blessing authority is the
	// highest-value target for login recycling, so a config author dropping the
	// id must not silently re-open it. A value containing a separator, or one
	// naming a bot/App, is refused outright.
	EnvBlessLogin = "ASSAY_BLESS_LOGIN"
	// EnvTrustedLogins carries trusted HUMAN authors, comma-separated `login[:id]`.
	EnvTrustedLogins = "ASSAY_TRUSTED_LOGINS"
	// EnvTrustedBotSlugs carries trusted App slugs, comma-separated
	// `[role=]slug[:id]`. The optional `role=` prefix additionally binds a desk
	// role (reviewer / verifier / worker / desk …) to that App, which is what
	// replaces the compiled-in `<slug>[bot]` literals in the tools.
	EnvTrustedBotSlugs = "ASSAY_TRUSTED_BOT_SLUGS"
	// EnvAllowedRepos carries the write-authorisation repo set,
	// comma-separated `owner/name[:ci|:no-ci][:public|:private]`. An entry may
	// instead be a bare PATTERN `owner/*`, which admits every
	// current and future repo under that owner to IsAllowedRepo with no further
	// entry required; it carries no ci/visibility tokens. See parseConfig's
	// "allowed repos" section and config.go's allowedRepos doc comment.
	EnvAllowedRepos = "ASSAY_ALLOWED_REPOS"
	// EnvHumanLoginMap carries the human-name→login map, comma-separated
	// `name:login`.
	EnvHumanLoginMap = "ASSAY_HUMAN_LOGIN_MAP"
	// EnvRiskPathTriggersExtra carries ADDITIONAL risk-path triggers,
	// comma-separated. It is ADDITIVE-ONLY: it unions onto the compiled sets and
	// there is no value syntax that drops a compiled entry. The name is
	// deliberately not ASSAY_RISK_PATH_TRIGGERS — a variable reading as "the
	// triggers" invites a replacement implementation, and a narrowing knob on a
	// security gate is a waiver waiting to be set.
	EnvRiskPathTriggersExtra = "ASSAY_RISK_PATH_TRIGGERS_EXTRA"
	// EnvRiskCallout is the adopter-executable RISK-CLASSIFICATION callout
	// (the frozen JSON in/out contract): an
	// absolute path to an executable the desk execs as `<path> classify`, feeding
	// `{"repo":"...","changedFiles":["...",...]}` on stdin and reading
	// `{"riskClassed":true|false,"reason":"..."}` from stdout. Exit 0 = answered.
	//
	// It is the SAME extension-point shape ASSAY_WRITEGUARD_CALLOUT uses (05): the
	// adopter ships an executable, not a pattern list, so the house's real
	// risk-classification triggers stay compiled and reviewed on the adopter's own
	// side of the boundary rather than becoming a config syntax this shared tool
	// has to interpret.
	//
	// TWO rules, both load-bearing:
	//
	//	ONLY-WIDENS   risk classification is the UNION of the pattern classifier and
	//	              the callout: a callout answering "not classed" can never clear
	//	              a pattern hit, and a pattern miss can never suppress a
	//	              callout's "classed". There is no removal syntax anywhere in
	//	              this seam.
	//	FAIL-CLOSED   every way the question can go unanswered — non-zero exit,
	//	              timeout, unparseable/unreadable output, a missing or
	//	              non-executable binary, an empty configured path at invocation
	//	              time — answers classed=true. A risk gate that "could not ask"
	//	              is never treated as clean.
	//
	// Unset is NOT an error and NOT a refusal: pattern-only classification, exactly
	// today's behavior (the same unset-is-complete semantics as
	// ASSAY_RISK_PATH_TRIGGERS_EXTRA).
	//
	// The path must be ABSOLUTE, for the same reason ASSAY_WRITEGUARD_CALLOUT's
	// must be: a relative path resolves against whatever directory the desk tool
	// happened to be spawned in, which is caller-influenced input choosing this
	// gate's own policy source.
	EnvRiskCallout = "ASSAY_RISK_CALLOUT"
	// EnvRepoAliases carries the boards' DISPLAY grouping overrides,
	// comma-separated `repo=short:product`. It is the ADOPTER half of the repo
	// taxonomy: the generic default (short = last path segment, product = repo
	// owner) is compiled into the resolver (repoalias.go), and the house short
	// names and product buckets that used to be hardwired into the boards'
	// switches live HERE instead. `repo` matches either a full `owner/name` slug
	// or a bare basename (last path segment). An empty short or product keeps the
	// generic default for THAT facet (e.g. `repo=:ledger` overrides only the
	// product). It is display-only — no security decision reads it — but a
	// malformed entry REFUSES the whole configuration like any other roster error,
	// so broken config never silently mis-groups.
	EnvRepoAliases = "ASSAY_REPO_ALIASES"
	// EnvRepoForges is the SOURCE of "which forge serves this repo?":
	// comma-separated `owner/name=github` or `owner/name=gitlab` entries, consulted by
	// ForgeFor (forgeresolve.go) BEFORE it falls back to mapping the origin remote's host.
	// Unlike ASSAY_REPO_ALIASES (display-only), this key chooses which minted credential a
	// write is performed as, so it is held to a STRICTER grammar than the aliases key:
	//
	//	FULL SLUG ONLY   a bare basename is refused. A display grouping override may
	//	                 reasonably collapse two orgs' same-named repos onto one label;
	//	                 collapsing them onto one CUSTODY IDENTITY is exactly the silent
	//	                 widening a forge binding must never do.
	//	FAIL-CLOSED      a malformed entry, an unrecognised forge value, or a repo bound
	//	                 twice refuses the WHOLE roster — the same treatment every other
	//	                 security-relevant roster key gets, never a silent skip.
	//
	// Unset is neither an error nor a refusal: ForgeFor's remote-host fallback (and,
	// failing that, its Unverifiable refusal) is a complete answer on its own.
	EnvRepoForges = "ASSAY_REPO_FORGES"
	// EnvReleaseRepo names the repo deskrelease cuts release tags in, as a single
	// `owner/name` slug. Unset means the shipped default — the project's own public
	// home — so an adopter who has not configured anything gets byte-identical
	// behaviour to the compiled-in slug this replaces.
	//
	// It does NOT widen anything: deskrelease still passes the resolved slug through
	// IsAllowedRepo, so a configured value outside the write-authorisation set is
	// refused exactly as an out-of-set repo always was. This variable chooses WHICH
	// in-scope repo is the release home; it cannot admit one.
	EnvReleaseRepo = "ASSAY_RELEASE_REPO"
	// EnvWriteguardCallout is the ADOPTER-SUPPLIED dangerous-command callout for
	// writeguard: an absolute path to an executable the guard invokes to ask whether
	// a command is one the adopter refuses. It is the SAME extension-point shape the
	// risk classifier uses — the adopter ships an executable, not a pattern list —
	// because a pattern list in configuration is a policy the shared tool would then
	// have to interpret, and interpreting somebody else's policy is how a guard
	// acquires cases it was never designed for.
	//
	// TWO rules, and both are load-bearing:
	//
	//	ONLY-WIDENS   a callout verdict can only ADD a block. It is consulted AFTER
	//	              the compiled indicators have already answered, so there is no
	//	              value it can return that clears one of them. Unset means the
	//	              compiled generic indicators alone — exactly the shipped
	//	              behaviour, byte for byte.
	//	FAIL-CLOSED   for a write GUARD, failing closed means BLOCK. A callout that
	//	              exits non-zero, times out, cannot be executed, or prints
	//	              anything the guard does not recognise BLOCKS the command. An
	//	              adopter who configures a callout has said "ask this before you
	//	              allow"; a guard that allowed on "I could not ask" would be
	//	              answering a question nobody put to it.
	//
	// The path must be ABSOLUTE. A relative path would resolve against whatever
	// directory the hook happened to be spawned in, which is caller-influenced — the
	// one input a guard's own policy source must not depend on.
	EnvWriteguardCallout = "ASSAY_WRITEGUARD_CALLOUT"
	// EnvRosterSchema optionally pins the FORMAT version of the roster. Absent
	// means version 1, the only format that has ever existed. It exists so a
	// future format change is distinguishable from a stale file: without it, an
	// old file silently parses under new rules and the difference shows up as
	// missing trust rather than as an error.
	EnvRosterSchema = "ASSAY_ROSTER_SCHEMA"

	// EnvAllowCluster is the clusterguard operator opt-in. It is NOT a roster
	// value and is never read from here: cmd/clusterguard reads it with a direct
	// os.Getenv, because it is a per-SHELL export an operator makes deliberately
	// and a desk session must never inherit — the WRITEGUARD_SHARED_OK shape.
	//
	// It is declared and RECOGNISED here for one reason: it lives in the ASSAY_
	// namespace, and an unrecognised ASSAY_ key in roster.env refuses the WHOLE
	// configuration (parseConfig). An operator who records the opt-in in the
	// shared roster.env — a natural mistake, since that is where every other
	// ASSAY_ knob lives — would otherwise take every desk tool's trust roster
	// down at once. Recognised and ignored is the fail-safe reading. Same
	// treatment as EnvSweepWithheldStreams and EnvWithheldIdentifiers below.
	EnvAllowCluster = "ASSAY_ALLOW_CLUSTER"

	// EnvHomeRepo and EnvScanRepos are STATUSGEN-only roster values (the home repo
	// and the --scan-issues scope). deskkit does not consume them — but the two
	// readers share one roster.env, and an unknown key in the ASSAY_ namespace
	// REFUSES the whole configuration (parseConfig). So deskkit must RECOGNISE them,
	// or a roster.env that configures statusgen would collapse every desk tool's
	// configuration. They are recognised here and otherwise ignored. KEEP IN SYNC
	// with statusgen/rosterconfig.go's scanEnvHomeRepo / scanEnvScanRepos.
	EnvHomeRepo  = "ASSAY_HOME_REPO"
	EnvScanRepos = "ASSAY_SCAN_REPOS"

	// EnvAuthorizedAuthors is the STATUSGEN-only rostered authorized-author set
	// (the scan-transcribe lane's R-7 clause-1 boarding predicate). deskkit does
	// not consume it — but the two readers share one roster.env, and an unknown
	// key in the ASSAY_ namespace REFUSES the whole configuration, so deskkit must
	// RECOGNISE it or a roster.env that arms the scan lane would collapse every
	// desk tool's configuration fleet-wide. Recognised here and otherwise ignored.
	// KEEP IN SYNC with statusgen/rosterconfig.go's scanEnvAuthorizedAuthors.
	EnvAuthorizedAuthors = "ASSAY_AUTHORIZED_AUTHORS"

	// EnvFormerHumanLoginMap is a STATUSGEN-only roster value: the FORMER-humans map
	// (name:login for departed humans who were confirmed at some past point), which
	// statusgen's verifier floor consults as the "confirmed historically" half of its
	// human-token exemption. deskkit does not consume it — but the two readers share
	// one roster.env, and an unknown key in the ASSAY_ namespace REFUSES the whole
	// configuration (parseConfig), so deskkit must RECOGNISE it or a roster.env that
	// records departed humans for statusgen would collapse every desk tool's
	// configuration fleet-wide. Recognised here and otherwise ignored. KEEP IN SYNC
	// with statusgen/rosterconfig.go's scanEnvFormerHumanLoginMap.
	EnvFormerHumanLoginMap = "ASSAY_FORMER_HUMAN_LOGIN_MAP"

	// EnvChannelDriftTarget is a STATUSGEN-only roster value (the repo-relative
	// path to statusgen's --lint accepted-channel-drift register — a de-housed
	// withheld-stream path the house supplies at runtime). deskkit does not
	// consume it — but the two readers share one roster.env, and an unknown key
	// in the ASSAY_ namespace REFUSES the whole configuration, so deskkit must
	// RECOGNISE it or a roster.env that arms statusgen's in-house channel sweep
	// would collapse every desk tool's configuration fleet-wide. Recognised here
	// and otherwise ignored. KEEP IN SYNC with statusgen/rosterconfig.go's
	// scanEnvChannelDriftTarget.
	EnvChannelDriftTarget = "ASSAY_CHANNEL_DRIFT_TARGET"

	// EnvDeterministicGatePatterns is a STATUSGEN-only roster value: additional
	// house-specific deterministic-gate name substrings the autonomy report merges
	// on top of its generic built-in set (statusgen/autonomy.go's
	// EnvDeterministicGatePatterns — a house's own build- or contract-specific
	// gate name).
	// deskkit does not consume it — but the two readers share one roster.env, and
	// an unknown key in the ASSAY_ namespace REFUSES the whole configuration
	// (parseConfig). So deskkit must RECOGNISE it, or a roster.env that configures
	// statusgen's gate classification would collapse every desk tool's
	// configuration. It is recognised here and otherwise ignored. KEEP IN SYNC with
	// statusgen/rosterconfig.go's scanEnvDeterministicGatePatterns.
	EnvDeterministicGatePatterns = "ASSAY_DETERMINISTIC_GATE_PATTERNS"
)

// rosterSchemaVersion is the format version this build speaks.
const rosterSchemaVersion = "1"

// configHomeFile is the local source of truth: a dotenv-style file carrying the
// same variable names the CI transport uses. It lives under the config home,
// which is outside every ref the tools evaluate (P2's local expression).
const configHomeFile = "~/.config/assay/roster.env"

// ToolClass is how the loader learns what its caller is allowed to read from.
type ToolClass int

const (
	// ClassWrite is the ZERO VALUE and the default for every locally-run tool:
	// the roster is read from the config-home file ONLY, never from the
	// environment. All four current locally-run tools (deskpost, issueboard,
	// verifyloop, statusgen --scan-issues) are this class — scan-issues WRITES
	// placeholder files that reach Next-up, and verifyloop renders and dispatches
	// prompts; neither is a read view. Any locally-run tool defaults here.
	ClassWrite ToolClass = iota
	// ClassCI is for a process running inside a CI job, where the environment IS
	// the repository/organization Actions variable transport and no config home
	// exists. The config-home file is not consulted.
	ClassCI
	// ClassReadOnly is env-first with a config-home fallback. It is available to a
	// future genuinely READ-ONLY tool. No current tool uses it, on purpose.
	ClassReadOnly
)

func (c ToolClass) String() string {
	switch c {
	case ClassCI:
		return "ci"
	case ClassReadOnly:
		return "read-only"
	default:
		return "write"
	}
}

// RepoAlias is one repo's DISPLAY grouping override, parsed from an
// ASSAY_REPO_ALIASES entry. An empty field keeps the generic default for THAT
// facet (short = last path segment, product = repo owner). See repoalias.go for
// the resolver that unions this over the generic default.
type RepoAlias struct {
	Short   string
	Product string
}

// Identity is a login (in its rendered form, lowercased) with its PERMANENT
// numeric GitHub id. Logins can be recycled — an account is deleted and the
// login re-registered by an attacker — so wherever a surface carries the numeric
// id it must be compared alongside the login. ID 0 means "not pinned".
type Identity struct {
	Login string
	ID    int64
}

// Config is one fully-parsed, validated configuration. A Config with a non-empty
// Problems slice carries NOTHING else: validation failure collapses the whole
// configuration to unconfigured rather than admitting a half-parsed roster.
type Config struct {
	Class  ToolClass
	Source string

	// Bless is the single blessing authority. Zero value = unconfigured.
	Bless Identity
	// Humans maps a lowercased human login to its pinned id (0 = unpinned).
	Humans map[string]int64
	// Bots maps a lowercased GitHub App slug to its BOT USER id (0 = unpinned). It
	// carries GITHUB entries only — the github-slug-keyed shape trust.go's GitHub
	// paths read. A GitLab identity lives in BotIdents (and its username in Logins),
	// never here, so a github-slug lookup never resolves a GitLab account by accident.
	Bots map[string]int64
	// BotIdents maps a lowercased slug-or-login to its full forge-qualified identity
	// (forge, inferred flag, id). It is the source of truth for every forge-aware
	// consumer — the commit-identity check, the role commit identity, the
	// forge-agreement gate — while Bots/Logins remain the flat views the pre-existing
	// GitHub trust paths read. See forgeidentity.go.
	BotIdents map[string]BotIdentity
	// RoleBots maps a desk role ("reviewer") to its App slug-or-login.
	RoleBots map[string]string
	// Logins is the set of ACCEPTED rendered login forms, lowercased: each human
	// login verbatim, and each App slug in BOTH GitHub renderings. The BARE slug
	// is deliberately absent — App slugs and usernames live in separate GitHub
	// namespaces, so a plain user account named after a slug could otherwise
	// spoof a desk identity.
	Logins map[string]bool

	Repos       map[string]repoPolicy
	HumanLogins map[string]string
	RiskExtra   []string

	// RiskCallout is the configured risk-classification callout path
	// (EnvRiskCallout), empty when unset. Empty means pattern classification only
	// — the shipped behavior, byte for byte.
	RiskCallout string

	// RepoAliases are the boards' DISPLAY grouping overrides parsed from
	// ASSAY_REPO_ALIASES, keyed by full `owner/name` slug OR bare basename. The
	// resolver (repoalias.go) consults this on top of the generic default; an
	// empty facet keeps the default. Display-only — no trust decision reads it.
	RepoAliases map[string]RepoAlias

	// RepoForges is the configured forge binding parsed from ASSAY_REPO_FORGES, keyed by
	// the LOWERCASED full `owner/name` slug (a bare basename is refused at parse time, so
	// no basename key ever lands here). The value is "github" or "gitlab". ForgeFor
	// (forgeresolve.go) consults this FIRST, before its remote-host fallback. Empty when
	// unset — that is a complete configuration, not a degraded one.
	RepoForges map[string]string

	// ReleaseRepo is the configured release home (EnvReleaseRepo), empty when
	// unset — the consumer applies its own shipped default, so "unset" and
	// "configured to the default" are the same behaviour rather than two states
	// this struct has to distinguish.
	ReleaseRepo string

	// WriteguardCallout is the absolute path of the adopter's writeguard callout
	// (EnvWriteguardCallout), empty when unset. Empty means the compiled generic
	// indicators alone — see the const's ONLY-WIDENS note.
	WriteguardCallout string

	// RepoPatterns is the sorted, de-duplicated set of owner/* PATTERN entries parsed
	// out of ASSAY_ALLOWED_REPOS (extended to configuration: an entry
	// shaped "owner/*" instead of "owner/name"). Each element is the bare owner half
	// ("medici-finance", never "medici-finance/*"). It widens ONLY IsAllowedRepo
	// (config.go) — Repos (above), CIRequired, RepoVisibility and AllowedRepos all stay
	// explicit-only, exactly as they did under the compiled-in org-default this
	// generalises. See config.go's allowedRepos doc comment for the full property.
	RepoPatterns []string

	// ScanRepos is the intake SCAN scope (ASSAY_SCAN_REPOS): the owned-repo set the
	// issue lane READS. It is a distinct set from Repos (the WRITE boundary,
	// ASSAY_ALLOWED_REPOS) on purpose — see EnvScanRepos. issueboard's ownedRepos
	// reads it; statusgen's scanRepos reads the identical key, which is what now
	// keeps the desk's read surface and the scanner's write surface from drifting
	// (ownedrepos_coupling_test.go) — one roster value, not two hand-synced lists.
	ScanRepos []string

	// UnknownKeys are keys present in the source that this version does not
	// recognise, sorted. They are ECHOED, never applied. A key in the ASSAY_
	// namespace is a REFUSAL instead (see parseConfig) and never lands here: a
	// typo'd roster key yields a configuration that reports itself correct with
	// that whole control surface empty, which is the silent half of the
	// validation gap the security review recorded as item 3.
	UnknownKeys []string

	// Problems are the loud, human-readable reasons a configuration was refused.
	Problems []string
}

// Configured reports whether a usable roster was loaded. Unset, empty, or
// refused all answer false, and every trust decision is false in that state.
func (c Config) Configured() bool {
	return len(c.Problems) == 0 && c.Bless.Login != "" && len(c.Logins) > 0
}

var (
	cfgMu    sync.Mutex
	cfgClass ToolClass
	cfgOnce  bool
	cfgValue Config
)

// InCI reports whether this process is running inside a GitHub Actions job.
//
// GITHUB_ACTIONS=true is set by the runner itself and is the documented signal.
// It is forgeable by anything that can already set this process's environment,
// which is why it is NOT sufficient on its own — see ClassForTool.
func InCI() bool { return strings.EqualFold(strings.TrimSpace(os.Getenv("GITHUB_ACTIONS")), "true") }

// ClassForTool resolves the class a tool must load with. It is the ONE place the
// CI transport is selected, and every main calls it rather than choosing a class
// by hand.
//
// WHY THIS EXISTS (a correctness review). SetToolClass had no
// caller anywhere in the tree and statusgen had no setter at all, so every tool was
// ClassWrite and ClassWrite never reads the environment. Actions variables arrive AS
// environment variables — so the repository/organization variables the paired
// consumer PR documents reached no code at all. The configuration route was
// unreachable, and the concrete consequence was a red `statusgen --lint` on every
// consumer PR once the pin bumped.
//
// TWO conditions, and both are load-bearing:
//
//	ciEligible  a per-tool property of what the tool DOES with the roster. It is
//	            false for every tool that ACTS on it — posts, flips, dispatches,
//	            writes a work item — regardless of where it runs. Those tools are
//	            locally run and stay file-only, so the split-by-action-
//	            class ruling is untouched: no acting tool gains an env route here.
//	InCI()      the runner signal.
//
// RESIDUAL, stated rather than taken silently. A steered agent running locally as
// the user can export GITHUB_ACTIONS=true alongside a hostile roster and reach the
// env path — but ONLY for a ci-eligible tool, and the eligible set is deliberately
// limited to report/lint paths. No write authorisation, no flip, no blessing, and
// no queue admission is reachable this way, because every tool holding those is
// ciEligible=false and refuses the environment unconditionally. That is a smaller
// residual than the one the package comment already records for the config file
// itself.
//
// One correction to an earlier draft, which said the eligible paths "grant nothing".
// They grant nothing DIRECTLY, but they are not decision-neutral: the human-login
// map (ASSAY_HUMAN_LOGIN_MAP) is consulted only to ACCEPT, so a non-empty map
// arriving through the environment is strictly wider than no map. In THIS module
// the map has no gate-clearing consumer; in statusgen it clears THREE — the verifier
// floor, the register human-authorisation check, and `--corroborate`'s stamp check
// (whose exit status it flips). The exception is written down at the
// head of statusgen/rosterconfig.go and pinned by
// TestCIHumanLoginMapResidualBoundary there. Do not restore the phrase "the
// environment cannot widen a decision" to either copy: it is measurably false for a
// non-empty map, and a reader relies on this note.
//
// The parser rejects a bot/App/shared-agent LOGIN in this map (looksLikeBot),
// mirroring the ASSAY_BLESS_LOGIN rule and the statusgen copy so the two readers stay
// byte-identical on the same file. This module has no gate-clearing consumer of the
// map, but the check stays here for that agreement — do not drop it as dead.
func ClassForTool(ciEligible bool) ToolClass {
	if ciEligible && InCI() {
		return ClassCI
	}
	return ClassWrite
}

// SetToolClass declares the calling tool's action class. It must be called
// before any trust decision; not calling it leaves ClassWrite, the restrictive
// default. Changing the class discards any cached configuration.
func SetToolClass(c ToolClass) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	cfgClass = c
	cfgOnce = false
}

// ReloadConfig discards the cached configuration so the next read re-parses.
func ReloadConfig() {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	cfgOnce = false
}

// EffectiveConfig returns the process's configuration, loading it on first use.
func EffectiveConfig() Config {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	if !cfgOnce {
		cfgValue = LoadConfig(cfgClass)
		cfgOnce = true
	}
	return cfgValue
}

// LoadConfig reads and validates the configuration for class, without touching
// the process cache. It never returns an error: an unreadable, absent or invalid
// configuration is an UNCONFIGURED one with Problems recorded, because every
// caller of this package has to fail closed on that state anyway and an error
// return invites a caller that logs and continues.
func LoadConfig(class ToolClass) Config {
	vals, source, problems := readRawConfig(class)
	cfg := parseConfig(class, source, vals)
	if len(problems) > 0 {
		// A source-level problem (bad permissions, unreadable file) refuses
		// everything: see the fail-closed note in the package comment.
		return Config{Class: class, Source: source, Problems: append(problems, cfg.Problems...)}
	}
	return cfg
}

// readRawConfig resolves the raw key→value pairs for class. It returns the
// values, a human-readable source description for the echo, and any source-level
// problems (which refuse the whole configuration).
func readRawConfig(class ToolClass) (map[string]string, string, []string) {
	keys := []string{
		EnvBlessLogin, EnvTrustedLogins, EnvTrustedBotSlugs,
		EnvAllowedRepos, EnvHumanLoginMap, EnvRiskPathTriggersExtra,
		EnvRiskCallout, EnvRepoAliases, EnvRepoForges, EnvReleaseRepo, EnvWriteguardCallout, EnvRosterSchema,
	}
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
	case ClassCI:
		// The environment IS the Actions-variable transport here. The config home
		// is not consulted: a CI runner has none, and reading one would make the
		// runner's filesystem a second, unreviewed source of truth.
		return fromEnv(), "environment (repository/organization Actions variables)", nil

	case ClassReadOnly:
		file, path, ferr := readConfigFile()
		m := fromEnv()
		src := "environment"
		if ferr == nil {
			for k, v := range file {
				if _, ok := m[k]; !ok {
					m[k] = v
				}
			}
			src = "environment, then " + path
		}
		// A read-only tool does not refuse on a bad config file: env may have
		// supplied everything. The problem is still surfaced by the echo.
		var probs []string
		if ferr != nil && !os.IsNotExist(ferr) {
			probs = nil // recorded via source, never fatal for a read-only caller
			src = "environment (config file unusable: " + ferr.Error() + ")"
		}
		return m, src, probs

	default: // ClassWrite
		// NO ENVIRONMENT READ. This is the whole point of the split: a write-class
		// tool steered by injected text must not be able to reach the roster
		// through an env var. TestWriteToolsRefuseEnvRoster pins it.
		file, path, ferr := readConfigFile()
		if ferr != nil {
			if os.IsNotExist(ferr) {
				return nil, "config file " + path + " (absent)", []string{fmt.Sprintf(
					"no roster configured: %s does not exist. Write-class tools read the roster "+
						"ONLY from this file — the environment is deliberately not consulted, so "+
						"exporting %s has no effect. Create it with 0600 permissions.",
					path, EnvTrustedLogins)}
			}
			return nil, "config file " + path + " (unusable)", []string{fmt.Sprintf(
				"refusing to load the roster from %s: %v", path, ferr)}
		}
		return file, "config file " + path, nil
	}
}

// ConfigHomePath is the resolved path of the local config file.
func ConfigHomePath() string { return expandHome(configHomeFile) }

// readConfigFile parses the config-home file after enforcing the sshd rule on it
// and on its directory.
func readConfigFile() (map[string]string, string, error) {
	path := ConfigHomePath()
	if err := checkOwnerPerms(filepath.Dir(path), true); err != nil {
		return nil, path, err
	}
	if err := checkOwnerPerms(path, false); err != nil {
		return nil, path, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, err
	}
	return parseDotenv(string(data)), path, nil
}

// checkOwnerPerms is the sshd rule: a configuration that decides who is trusted
// must not be writable by anyone but its owner, and must be owned by the user
// running the tool. A co-located unprivileged process then cannot plant or edit
// the roster without already holding the user's own write access.
//
// A MISSING directory or file is reported as os.ErrNotExist so the caller can
// tell "not configured" from "configured wrongly" — the two need different
// messages, and conflating them is how "unconfigured" starts reading as "fine".
func checkOwnerPerms(path string, isDir bool) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if isDir && !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	if mode := fi.Mode().Perm(); mode&0o022 != 0 {
		kind := "file"
		if isDir {
			kind = "directory"
		}
		return fmt.Errorf("roster config %s %s is group- or world-writable (mode %04o): "+
			"anything that can write it can name the accounts this tool trusts. "+
			"Fix with `chmod %s %s`", kind, path, mode, map[bool]string{true: "0700", false: "0600"}[isDir], path)
	}
	// Owner check is platform-specific: unix compares the owning uid; windows has
	// no uid and skips it LOUDLY (see rosterowner_{unix,windows}.go). The
	// group/world-writable mode check above runs on both platforms.
	return checkFileOwner(path, fi)
}

// parseDotenv reads KEY=VALUE lines. A leading "export " is tolerated, "#"
// comment lines are skipped, and surrounding quotes are trimmed — the same shape
// appconfig.go's apps.env already uses.
func parseDotenv(s string) map[string]string {
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
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		if v := strings.TrimSpace(val); v != "" {
			m[key] = v
		}
	}
	return m
}

// ---- parsing -----------------------------------------------------------------

// splitList splits a list value on the separators every list-shaped variable
// accepts. ASSAY_BLESS_LOGIN deliberately does NOT go through here.
func splitList(raw string) []string {
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

// splitIdentity splits `login[:id]`. ok is false when an id is present but not a
// number — a typo'd id must never degrade to login-only trust.
func splitIdentity(entry string) (login string, id int64, ok bool) {
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

// looksLikeBot reports whether a login renders as a GitHub App / bot / shared
// agent account. The argument: "the authorisation half of a two-factor mechanism
// has to be a human act". A blessing IS that authorisation half.
func looksLikeBot(login string) bool {
	l := strings.ToLower(strings.TrimSpace(login))
	return strings.HasSuffix(l, "[bot]") ||
		strings.HasPrefix(l, "app/") ||
		strings.HasSuffix(l, "-app") ||
		strings.HasSuffix(l, "-bot")
}

func parseConfig(class ToolClass, source string, vals map[string]string) Config {
	cfg := Config{
		Class:       class,
		Source:      source,
		Humans:      map[string]int64{},
		Bots:        map[string]int64{},
		BotIdents:   map[string]BotIdentity{},
		RoleBots:    map[string]string{},
		Logins:      map[string]bool{},
		Repos:       map[string]repoPolicy{},
		HumanLogins: map[string]string{},
	}
	var problems []string
	bad := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	// --- key recognition, BEFORE any value is applied ---
	//
	// A key this version does not know is one of two very different things, and
	// collapsing them is what made the gap silent:
	//
	//   ASSAY_ALOWED_REPOS=…   a TYPO on a roster key. The surface it was meant to
	//                          configure stays EMPTY while the configuration reports
	//                          itself correct. Fail closed is the safe direction for
	//                          the value, but "reports itself correct" is not — so
	//                          this REFUSES the whole configuration, the same way a
	//                          malformed value already does.
	//   PROD_DEPLOY_GUARDED_PATHS=…  a legitimate co-tenant. The two CI guards read
	//                          their own variables directly, and an adopter may keep
	//                          them in this same file. Refusing here would make the
	//                          documented layout unusable, so these are ECHOED.
	//
	// Namespace membership is the discriminator, not a near-miss distance metric: a
	// typo inside the owned namespace is rejected, and a key outside it is out of
	// scope entirely.
	known := map[string]bool{
		EnvBlessLogin: true, EnvTrustedLogins: true, EnvTrustedBotSlugs: true,
		EnvAllowedRepos: true, EnvHumanLoginMap: true, EnvRiskPathTriggersExtra: true,
		EnvRiskCallout: true, EnvRepoAliases: true, EnvRepoForges: true, EnvReleaseRepo: true,
		EnvWriteguardCallout: true, EnvRosterSchema: true,
		// STATUSGEN-only keys: recognised so a shared roster.env that configures
		// statusgen does not collapse deskkit's configuration; not consumed here.
		EnvHomeRepo: true, EnvScanRepos: true, EnvAuthorizedAuthors: true,
		EnvFormerHumanLoginMap: true,
		EnvChannelDriftTarget:  true, EnvDeterministicGatePatterns: true,
		// EnvSweepWithheldStreams (ASSAY_SWEEP_WITHHELD_STREAMS, sweepconfig.go) is
		// consumed by the S2 sweep via a direct os.Getenv read, NOT through this
		// scanConfig — but #1333's de-housing REQUIRES the house to set it in the
		// shared roster.env for the sweep to route, so it must be RECOGNISED here or
		// activating that de-housing collapses the whole roster on the
		// unknown-ASSAY_-key refusal. KEEP IN SYNC with statusgen's
		// scanEnvSweepWithheldStreams.
		EnvSweepWithheldStreams: true,
		// EnvWithheldIdentifiers (ASSAY_WITHHELD_IDENTIFIERS, selfcontain.go) is read by
		// the public-repo self-containment scan through a direct os.Getenv, NOT through
		// this scanConfig — but a house that configures it does so in the SAME shared
		// roster.env, and an unrecognised key in the ASSAY_ namespace refuses the whole
		// configuration. It must therefore be RECOGNISED here or turning the scan's
		// register category on would collapse every desk tool's roster at once.
		EnvWithheldIdentifiers: true,
		// EnvAllowCluster (ASSAY_ALLOW_CLUSTER) is the clusterguard operator opt-in, read
		// by cmd/clusterguard via a direct os.Getenv and never consumed here. It is
		// RECOGNISED so that an operator who records it in the shared roster.env does not
		// collapse every desk tool's roster on the unknown-ASSAY_-key refusal. Recognised
		// is not applied: putting it in roster.env still does NOT grant the opt-in, which
		// is a per-shell export by design.
		EnvAllowCluster: true,
	}
	for k := range vals {
		if known[k] {
			continue
		}
		if strings.HasPrefix(k, "ASSAY_") {
			bad("%s: unknown key in the ASSAY_ namespace. It is not applied, so whatever it was "+
				"meant to configure is EMPTY — and an empty control surface that reports itself "+
				"configured is the failure this refusal exists to prevent. Recognised keys: "+
				"%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s (and the optional %s)",
				k, EnvBlessLogin, EnvTrustedLogins, EnvTrustedBotSlugs, EnvAllowedRepos,
				EnvHumanLoginMap, EnvRiskPathTriggersExtra, EnvRiskCallout, EnvRepoAliases, EnvRepoForges,
				EnvReleaseRepo, EnvWriteguardCallout, EnvRosterSchema)
			continue
		}
		cfg.UnknownKeys = append(cfg.UnknownKeys, k)
	}
	sort.Strings(cfg.UnknownKeys)

	// --- schema version (optional, but validated when present) ---
	// Without this a future format change is indistinguishable from a stale file
	// (security review item 3). Absent means "the only schema that has ever
	// existed"; any other value refuses rather than guessing which format the file
	// is in.
	if v := strings.TrimSpace(vals[EnvRosterSchema]); v != "" && v != rosterSchemaVersion {
		bad("%s=%q is not a schema version this build understands (it speaks %q). Refusing: a "+
			"file written for a different format cannot be safely half-applied",
			EnvRosterSchema, v, rosterSchemaVersion)
	}

	// --- bot slugs first: the bless-login check needs the configured bot set ---
	for _, entry := range splitList(vals[EnvTrustedBotSlugs]) {
		role := ""
		if r, rest, found := strings.Cut(entry, "="); found {
			role = strings.ToLower(strings.TrimSpace(r))
			entry = rest
		}
		ident, ok := splitBotEntry(entry)
		if !ok {
			bad("%s: cannot parse entry %q — expected [role=]<forge>:slug-or-login[:id] "+
				"(forge is github or gitlab; an entry with no forge is read as github). The id, "+
				"when present, must be a positive number", EnvTrustedBotSlugs, entry)
			continue
		}
		cfg.BotIdents[ident.Slug] = ident
		// Per-forge renderings (forgeidentity.go): GitHub keeps <slug>[bot] and
		// app/<slug>; GitLab registers the account's username. The bare GitHub App slug
		// is never accepted on any forge. The bot USER id is kept in the flat Bots view
		// for GITHUB entries only, the shape trust.go's GitHub paths read.
		for _, login := range ident.AcceptedLogins() {
			cfg.Logins[login] = true
		}
		if ident.Forge == ForgeGitHub {
			cfg.Bots[ident.Slug] = ident.ID
		}
		if role != "" {
			cfg.RoleBots[role] = ident.Slug
		}
	}

	// --- trusted humans ---
	for _, entry := range splitList(vals[EnvTrustedLogins]) {
		login, id, ok := splitIdentity(entry)
		if !ok {
			bad("%s: cannot parse entry %q — expected login[:id] with a positive numeric id",
				EnvTrustedLogins, entry)
			continue
		}
		if looksLikeBot(login) {
			bad("%s lists %q, which renders as a bot/App account. App identities belong in %s "+
				"(where their rendered forms are derived), not in the human roster",
				EnvTrustedLogins, login, EnvTrustedBotSlugs)
			continue
		}
		cfg.Humans[login] = id
		cfg.Logins[login] = true
	}

	// --- the single blessing authority ---
	// Three extra rules apply here and NOWHERE else, because this is the single
	// highest-value target in the roster and its siblings' looser rules must not
	// leak onto it: the id is mandatory, the value is a single identity, and it
	// may not name a bot/App/shared-agent account.
	if raw := strings.TrimSpace(vals[EnvBlessLogin]); raw != "" {
		switch {
		case strings.ContainsAny(raw, ",; \t\n\r"):
			bad("%s=%q contains a separator. It is a SINGLE identity, never a list: "+
				"blessing the first entry (or every entry) of a list would silently grant the "+
				"authority this variable exists to restrict. Refusing, exactly as if it were unset",
				EnvBlessLogin, raw)
		default:
			login, id, ok := splitIdentity(raw)
			_, inBotSet := cfg.Bots[login]
			switch {
			case !ok:
				bad("%s=%q cannot be parsed — expected login:id with a positive numeric id", EnvBlessLogin, raw)
			case id == 0:
				bad("%s=%q carries no numeric id. The id is MANDATORY for the blessing authority "+
					"(unlike its siblings): a deleted login can be re-registered by an attacker, and the "+
					"blessing authority is the highest-value target for that. Refusing, exactly as if it "+
					"were unset", EnvBlessLogin, raw)
			case looksLikeBot(login):
				bad("%s=%q names a bot/App/shared-agent account. The blessing is the authorisation "+
					"half of the trust gate and has to be a HUMAN act — an App that could bless would "+
					"admit external text into its own queue. Refusing", EnvBlessLogin, raw)
			case inBotSet:
				bad("%s=%q is also listed in %s. A trusted App is a trusted AUTHOR; it must never be "+
					"the blessing authority", EnvBlessLogin, raw, EnvTrustedBotSlugs)
			default:
				cfg.Bless = Identity{Login: login, ID: id}
				// The blessing authority is trusted in its own right.
				cfg.Humans[login] = id
				cfg.Logins[login] = true
			}
		}
	}

	// --- allowed repos, the WRITE boundary ---
	//
	// Two shapes share this list: an EXPLICIT "owner/name[:ci|:no-ci][:public|:private]"
	// entry, and a PATTERN "owner/*" entry. Both pass the identical
	// slash-shape check below (exactly one "/", no leading/trailing slash) — a pattern
	// is distinguished by its name half being the literal "*", checked AFTER that shape
	// check so a malformed slug gets the same "not an owner/name repo slug" refusal
	// either way.
	seenPattern := map[string]bool{}
	for _, entry := range splitList(vals[EnvAllowedRepos]) {
		parts := strings.Split(entry, ":")
		slug := strings.TrimSpace(parts[0])
		if strings.Count(slug, "/") != 1 || strings.HasPrefix(slug, "/") || strings.HasSuffix(slug, "/") {
			bad("%s: %q is not an owner/name repo slug", EnvAllowedRepos, entry)
			continue
		}
		owner, name, _ := strings.Cut(slug, "/")
		if name == "*" {
			// PATTERN entry: admits every repo under owner to IsAllowedRepo alone (see
			// config.go's allowedRepos doc comment). It carries no ci/visibility policy
			// — those apply to ONE repo's actual workflows/visibility, which a pattern
			// does not name — so any trailing token here is refused rather than
			// silently ignored.
			if strings.Contains(owner, "*") {
				bad("%s: %q — the owner half of a pattern entry may not itself contain '*'",
					EnvAllowedRepos, entry)
				continue
			}
			if len(parts) > 1 {
				bad("%s: %q is a pattern entry (owner/*) and may not carry a ci/visibility policy — "+
					"a pattern-matched repo gets fail-closed CI/visibility defaults (CI-required, "+
					"visibility unknown) from the explicit census instead", EnvAllowedRepos, entry)
				continue
			}
			if !seenPattern[owner] {
				seenPattern[owner] = true
				cfg.RepoPatterns = append(cfg.RepoPatterns, owner)
			}
			continue
		}
		// Fail-closed defaults: CI required unless the adopter states otherwise,
		// and visibility UNKNOWN unless stated (unknown risk-classes every PR).
		pol := repoPolicy{CIRequired: true, Visibility: VisibilityUnknown}
		malformed := false
		for _, tok := range parts[1:] {
			switch strings.ToLower(strings.TrimSpace(tok)) {
			case "ci":
				pol.CIRequired = true
			case "no-ci":
				pol.CIRequired = false
			case "public":
				pol.Visibility = VisibilityPublic
			case "private":
				pol.Visibility = VisibilityPrivate
			case "":
			default:
				bad("%s: %q carries unknown policy token %q — expected ci|no-ci and public|private",
					EnvAllowedRepos, slug, tok)
				malformed = true
			}
		}
		if malformed {
			continue
		}
		cfg.Repos[slug] = pol
	}
	sort.Strings(cfg.RepoPatterns)

	// --- intake scan scope (ASSAY_SCAN_REPOS) ---
	// A DEDICATED set, not the write boundary above (see the field / EnvScanRepos
	// comments). A malformed slug REFUSES the whole configuration, exactly as a
	// malformed ASSAY_ALLOWED_REPOS entry does; an unset value is neither an error
	// nor a refusal (the read surface is simply empty).
	for _, entry := range splitList(vals[EnvScanRepos]) {
		if strings.Count(entry, "/") != 1 || strings.HasPrefix(entry, "/") || strings.HasSuffix(entry, "/") {
			bad("%s: %q is not an owner/name repo slug", EnvScanRepos, entry)
			continue
		}
		cfg.ScanRepos = append(cfg.ScanRepos, entry)
	}

	// --- human-name → login map ---
	for _, entry := range splitList(vals[EnvHumanLoginMap]) {
		name, login, found := strings.Cut(entry, ":")
		name = strings.ToLower(strings.TrimSpace(name))
		login = strings.ToLower(strings.TrimSpace(login))
		if !found || name == "" || login == "" {
			bad("%s: cannot parse entry %q — expected name:login", EnvHumanLoginMap, entry)
			continue
		}
		if looksLikeBot(login) {
			bad("%s: entry %q maps to a bot/App/shared-agent login %q. The human-login map "+
				"exists to establish that a HUMAN acted — it clears the verifier floor, the "+
				"register human-authorisation check and --corroborate — so a bot/App value would "+
				"let `human:<name>` resolve to the shared-agent identity the token exists to "+
				"exclude. Refusing", EnvHumanLoginMap, entry, login)
			continue
		}
		cfg.HumanLogins[name] = login
	}

	// --- risk-path triggers, ADDITIVE ONLY ---
	// Unset is NOT an error and NOT a refusal: the compiled set is a complete
	// configuration on its own, and there is no value syntax that drops a
	// compiled entry.
	cfg.RiskExtra = splitList(vals[EnvRiskPathTriggersExtra])
	sort.Strings(cfg.RiskExtra)

	// --- risk-classification callout (ASSAY_RISK_CALLOUT), Q1-ruled contract ---
	// An ABSOLUTE path to ONE executable. Only SHAPE is validated here; everything
	// about the callout that can change between load and use — exists, is
	// executable, is not group/world-writable, answers within the timeout,
	// answers in the JSON contract shape — is checked at INVOCATION time by the
	// classifier itself (riskcallout.go), because every one of those means
	// classed=true rather than "unconfigured".
	if raw := strings.TrimSpace(vals[EnvRiskCallout]); raw != "" {
		switch {
		case strings.ContainsAny(raw, ",;\t\n\r"):
			bad("%s=%q contains a separator. It names ONE executable, never a list", EnvRiskCallout, raw)
		case !filepath.IsAbs(raw):
			bad("%s=%q is not an absolute path. A relative callout path resolves against the "+
				"directory the desk tool happened to be spawned in — caller-influenced input "+
				"choosing this gate's own policy source. Refusing, exactly as if it were unset",
				EnvRiskCallout, raw)
		default:
			cfg.RiskCallout = raw
		}
	}

	// --- repo aliases, the boards' DISPLAY grouping override ---
	// The generic default (short = last path segment, product = repo owner) lives
	// in the resolver (repoalias.go). This is the ADOPTER override — the house
	// short names and product buckets that used to be compiled into the boards'
	// switches. Unset is neither error nor refusal (the generic default is a
	// complete configuration on its own), but a MALFORMED entry refuses the whole
	// configuration exactly as a bad ASSAY_ALLOWED_REPOS entry does: display-only
	// or not, a roster that silently mis-groups is the configured-but-wrong shape
	// this design refuses.
	cfg.RepoAliases = map[string]RepoAlias{}
	for _, entry := range splitList(vals[EnvRepoAliases]) {
		key, spec, hasEq := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !hasEq || key == "" {
			bad("%s: cannot parse entry %q — expected repo=short:product (repo is a full "+
				"owner/name slug or a bare basename)", EnvRepoAliases, entry)
			continue
		}
		short, prod, hasColon := strings.Cut(spec, ":")
		if !hasColon {
			bad("%s: entry %q has no ':' separating short label from product — expected "+
				"repo=short:product; write repo=short: or repo=:product to keep one facet generic",
				EnvRepoAliases, entry)
			continue
		}
		short = strings.TrimSpace(short)
		prod = strings.TrimSpace(prod)
		if short == "" && prod == "" {
			bad("%s: entry %q sets neither a short label nor a product — it configures nothing. "+
				"Drop it, or fill in a facet (an empty facet keeps the generic default)",
				EnvRepoAliases, entry)
			continue
		}
		if _, dup := cfg.RepoAliases[key]; dup {
			bad("%s: repo %q is aliased more than once — one alias per repo", EnvRepoAliases, key)
			continue
		}
		cfg.RepoAliases[key] = RepoAlias{Short: short, Product: prod}
	}

	// --- repo forge binding (ASSAY_REPO_FORGES) — the SOURCE ForgeFor (forgeresolve.go)
	// reads before its remote-host fallback. Unlike the aliases above (display-only), a
	// malformed OR ambiguous entry here refuses the WHOLE configuration: this key decides
	// which minted credential a write is performed as, so "configured but wrong" must fail
	// exactly as loudly as every other identity-adjacent roster value.
	cfg.RepoForges = map[string]string{}
	for _, entry := range splitList(vals[EnvRepoForges]) {
		key, val, hasEq := strings.Cut(entry, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.ToLower(strings.TrimSpace(val))
		if !hasEq || key == "" || val == "" {
			bad("%s: cannot parse entry %q — expected owner/name=github or owner/name=gitlab",
				EnvRepoForges, entry)
			continue
		}
		if strings.Count(key, "/") != 1 || strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") {
			bad("%s: entry %q's repo %q is not a full owner/name slug — unlike %s, a bare basename is "+
				"NOT accepted here: this key chooses a WRITE identity, and collapsing two orgs' "+
				"same-named repos onto one forge is exactly the silent widening it must refuse",
				EnvRepoForges, entry, key, EnvRepoAliases)
			continue
		}
		if val != string(ForgeGitHub) && val != string(ForgeGitLab) {
			bad("%s: entry %q names forge %q, which is neither %q nor %q",
				EnvRepoForges, entry, val, ForgeGitHub, ForgeGitLab)
			continue
		}
		if _, dup := cfg.RepoForges[key]; dup {
			bad("%s: repo %q is bound to a forge more than once", EnvRepoForges, entry)
			continue
		}
		cfg.RepoForges[key] = val
	}

	// --- release home (ASSAY_RELEASE_REPO) ---
	// A SINGLE slug, never a list: a release tool that took the first entry of a
	// list would pick its target by parse order. Unset is neither an error nor a
	// refusal — the consumer applies its shipped default.
	if raw := strings.TrimSpace(vals[EnvReleaseRepo]); raw != "" {
		switch {
		case strings.ContainsAny(raw, ",; \t\n\r"):
			bad("%s=%q contains a separator. It names ONE repo, never a list — cutting a release "+
				"in whichever entry happened to parse first is not a decision anyone made. "+
				"Refusing, exactly as if it were unset", EnvReleaseRepo, raw)
		case strings.Count(raw, "/") != 1 || strings.HasPrefix(raw, "/") || strings.HasSuffix(raw, "/"):
			bad("%s: %q is not an owner/name repo slug", EnvReleaseRepo, raw)
		case strings.Contains(raw, "*"):
			bad("%s=%q is a pattern. A release is cut in ONE repo; a pattern names a set, and "+
				"there is no rule here for choosing a member of it", EnvReleaseRepo, raw)
		default:
			cfg.ReleaseRepo = raw
		}
	}

	// --- writeguard callout (ASSAY_WRITEGUARD_CALLOUT) ---
	// An ABSOLUTE path to ONE executable. The absoluteness rule is the load-bearing
	// half: a relative path resolves against whatever directory the guard's process
	// was spawned in, and a guard whose policy source moves with the caller's cwd is
	// selecting its own policy from caller-influenced input.
	//
	// Everything else about the callout — that the file exists, is executable, is
	// not group-writable, exits 0, answers in time, answers in the vocabulary — is
	// checked at INVOCATION (callout.go), because each of those can change between
	// load and use and each of them means BLOCK rather than "unconfigured".
	if raw := strings.TrimSpace(vals[EnvWriteguardCallout]); raw != "" {
		switch {
		case strings.ContainsAny(raw, ",;\t\n\r"):
			bad("%s=%q contains a separator. It names ONE executable, never a list", EnvWriteguardCallout, raw)
		case !filepath.IsAbs(raw):
			bad("%s=%q is not an absolute path. A relative callout path resolves against the "+
				"directory the guard happened to be spawned in — caller-influenced input choosing "+
				"the guard's own policy source. Refusing, exactly as if it were unset",
				EnvWriteguardCallout, raw)
		default:
			cfg.WriteguardCallout = raw
		}
	}

	if len(problems) > 0 {
		return Config{Class: class, Source: source, Problems: problems}
	}
	return cfg
}

// ---- the effective-value echo (P3) -------------------------------------------

// EffectiveConfigLines renders every configured control surface this brief
// converts, one line each, in a stable order. It is the P3 visibility surface:
// because the value lives in repository settings (or a config-home file) rather
// than in a diff, the RUN is where a change becomes visible — in BOTH directions.
// A widened roster and a NARROWED trigger list must be equally apparent, so every
// line renders its full sorted set rather than a count or a digest.
//
// Logins and paths only — never a token, never a PEM path, never a secret-shaped
// value (deskpost's bodycheck continues to assert that separately).
func (c Config) EffectiveConfigLines() []string {
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
	repos := make([]string, 0, len(c.Repos)+len(c.RepoPatterns))
	for r, p := range c.Repos {
		ci := "no-ci"
		if p.CIRequired {
			ci = "ci"
		}
		repos = append(repos, r+":"+ci+":"+p.Visibility.String())
	}
	// PATTERN entries get the same treatment as explicit ones: a widened
	// IsAllowedRepo scope must be as visible in the echo as an explicit repo is,
	// never silently absent because it carries no ci/visibility policy of its own.
	for _, owner := range c.RepoPatterns {
		repos = append(repos, owner+"/*")
	}
	sort.Strings(repos)
	humanMap := make([]string, 0, len(c.HumanLogins))
	for n, l := range c.HumanLogins {
		humanMap = append(humanMap, n+"="+l)
	}
	sort.Strings(humanMap)
	// REPO ALIASES render one `repo=short:product` token per configured repo,
	// sorted. Like every other line here the FULL set is shown, not a count: a
	// grouping override lives in repository settings, so the run is the only place
	// a change to it becomes visible.
	aliases := make([]string, 0, len(c.RepoAliases))
	for repo, a := range c.RepoAliases {
		aliases = append(aliases, repo+"="+a.Short+":"+a.Product)
	}
	sort.Strings(aliases)
	// REPO FORGES render one `repo=forge` token per configured binding, sorted — the
	// forge-selection identity decision, so (like the role bindings below) it must be the
	// most visible thing here, never inferred from a diff.
	forges := make([]string, 0, len(c.RepoForges))
	for repo, kind := range c.RepoForges {
		forges = append(forges, repo+"="+kind)
	}
	sort.Strings(forges)
	// The release home and the writeguard callout both render their UNSET state as a
	// named default rather than as a blank. A blank reads as "nobody filled this in";
	// what these two actually mean when unset is a complete, shipped behaviour, and
	// the difference matters most for the callout — an empty line beside a guard is
	// the one place a reader must not have to guess whether the check is running.
	releaseStr := c.ReleaseRepo
	if releaseStr == "" {
		releaseStr = "(unset — the consumer's shipped default)"
	}
	calloutStr := c.WriteguardCallout
	if calloutStr == "" {
		calloutStr = "(unset — compiled generic indicators only)"
	}
	// ROLE BINDINGS get their own line. The bot-slug line above renders slug:id and
	// says nothing about which role each slug is BOUND to, so a dropped `role=`
	// prefix — the typo that silently unbinds a desk role — was invisible in the P3
	// echo as well as at load. It is the one narrowing that costs a whole identity
	// check, so it must be the most visible thing here, not the least.
	roles := make([]string, 0, len(c.RoleBots))
	for r, slug := range c.RoleBots {
		roles = append(roles, r+"="+slug)
	}
	sort.Strings(roles)
	rolesStr := strings.Join(roles, ",")
	if rolesStr == "" {
		rolesStr = "(none bound — every role-gated identity check REFUSES)"
	}
	// The REPO SET gets the same treatment, and for the same reason (#489). An empty
	// set rendered as a bare `ASSAY_ALLOWED_REPOS=` reads as a field nobody filled in,
	// when what it actually means is that every repo is out of scope for every tool.
	//
	// It is what makes the two states DISTINGUISHABLE at all on the write path. A write
	// verb refuses at exit 5 with "repo X is not in the fixed desk repo set" whether the
	// roster is absent or merely scope-less — same code, same sentence, two different
	// conditions, one of which is "there is no set" rather than "X is not in it".
	// Re-derived by running deskpost against both states: the refusal lines are byte
	// identical and only this echo differs.
	//
	// Fixing that at each of the ~13 IsAllowedRepo call sites would mean rewording every
	// write tool's refusal on a PR that must not disturb them; saying it once, here, is
	// the cheap half — every one of those tools already echoes this on every run.
	reposStr := strings.Join(repos, ",")
	if reposStr == "" {
		reposStr = "(EMPTY — no repo is in scope: reads are COULD-NOT-CHECK, writes REFUSE)"
	}
	// The risk-classification callout renders its UNSET state as a named default
	// rather than a blank, matching ASSAY_WRITEGUARD_CALLOUT's echo: a blank reads
	// as "nobody filled this in", when unset actually means a complete, shipped
	// behavior (pattern classification alone). When it IS configured, the raw
	// value is echoed VERBATIM — including a path that does not exist or is not
	// executable — because a fail-closed callout that cannot be consulted must be
	// loud about WHICH path it could not consult (Verify row 3).
	riskCalloutStr := c.RiskCallout
	if riskCalloutStr == "" {
		riskCalloutStr = "(unset — pattern classification only)"
	}

	lines := []string{
		fmt.Sprintf("assay-config: class=%s source=%s configured=%t", c.Class, c.Source, c.Configured()),
		fmt.Sprintf("assay-config: %s=%s", EnvBlessLogin, blessStr),
		fmt.Sprintf("assay-config: %s=%s", EnvTrustedLogins, sortedIdents(c.Humans)),
		fmt.Sprintf("assay-config: %s=%s", EnvTrustedBotSlugs, c.sortedBotIdents()),
		fmt.Sprintf("assay-config: role-bindings=%s", rolesStr),
		fmt.Sprintf("assay-config: %s=%s", EnvAllowedRepos, reposStr),
		fmt.Sprintf("assay-config: %s=%s", EnvScanRepos, strings.Join(c.ScanRepos, ",")),
		fmt.Sprintf("assay-config: %s=%s", EnvHumanLoginMap, strings.Join(humanMap, ",")),
		fmt.Sprintf("assay-config: %s=%s", EnvRiskPathTriggersExtra, strings.Join(c.RiskExtra, ",")),
		fmt.Sprintf("assay-config: %s=%s", EnvRiskCallout, riskCalloutStr),
		fmt.Sprintf("assay-config: %s=%s", EnvRepoAliases, strings.Join(aliases, ",")),
		fmt.Sprintf("assay-config: %s=%s", EnvRepoForges, strings.Join(forges, ",")),
		fmt.Sprintf("assay-config: %s=%s", EnvReleaseRepo, releaseStr),
		fmt.Sprintf("assay-config: %s=%s", EnvWriteguardCallout, calloutStr),
	}
	if len(c.UnknownKeys) > 0 {
		// Not a refusal on its own (a non-ASSAY_ key may legitimately share the file
		// with the guards' own variables), but it must never be silent: an admin whose
		// file is not the configuration in force has to be able to SEE that.
		lines = append(lines, fmt.Sprintf("assay-config: unrecognised-keys=%s (IGNORED)",
			strings.Join(c.UnknownKeys, ",")))
	}
	for _, p := range c.Problems {
		lines = append(lines, "assay-config: REFUSED — "+p)
	}
	return lines
}

// RiskCalloutPath returns the configured risk-classification callout path, or ""
// when none is configured. "" is the SHIPPED state, not a failure: it means
// pattern classification alone. Every failure that happens once a callout IS
// configured means classed=true — see riskcallout.go.
func RiskCalloutPath() string { return EffectiveConfig().RiskCallout }

// EchoEffectiveConfig writes the P3 echo. Every tool that makes a trust decision
// calls this once per run, so a settings change appears in run output and in CI
// history whether it widened or narrowed the set.
func EchoEffectiveConfig(w io.Writer) {
	for _, l := range EffectiveConfig().EffectiveConfigLines() {
		fmt.Fprintln(w, l)
	}
}

// ---- the configured knobs, as consumers read them ----------------------------

// ConfiguredReleaseRepo returns the configured release home, or "" when unset. The
// caller supplies its own shipped default for "" — deskkit does not carry one,
// because the default belongs to the tool that cuts the release and a default here
// would be a second place for it to drift from.
func ConfiguredReleaseRepo() string { return EffectiveConfig().ReleaseRepo }

// WriteguardCalloutPath returns the adopter's configured writeguard callout, or ""
// when none is configured. "" is the SHIPPED state, not a failure: it means the
// compiled generic indicators alone. Every failure that happens once a callout IS
// configured means BLOCK — see Callout.Run and the const's FAIL-CLOSED note.
func WriteguardCalloutPath() string { return EffectiveConfig().WriteguardCallout }

// RosterUnconfiguredError is the loud refusal every trust-gated caller emits when
// no usable roster was loaded. It never returns nil for an unconfigured roster:
// "nothing configured ⇒ nothing checked" is the one degradation this whole design
// exists to prevent.
func RosterUnconfiguredError() error {
	c := EffectiveConfig()
	if c.Configured() {
		return nil
	}
	detail := strings.Join(c.Problems, "\n  - ")
	if detail == "" {
		detail = fmt.Sprintf("%s and %s are unset", EnvBlessLogin, EnvTrustedLogins)
	}
	return fmt.Errorf("the trust roster is NOT CONFIGURED, so nothing is trusted and nothing can be "+
		"blessed (fail closed):\n  - %s\nSource consulted: %s (class %s).\n"+
		"CI: set the repository or organization Actions variables %s / %s / %s.\n"+
		"Locally: write them to %s, mode 0600 — write-class tools do NOT read the environment.",
		detail, c.Source, c.Class,
		EnvBlessLogin, EnvTrustedLogins, EnvTrustedBotSlugs, ConfigHomePath())
}
