package forgeban

// allowlist.go — the two in-tree registers the ban is read against. Both are ordinary Go
// source: growing either is a reviewable diff in a file whose header says what growing it
// means, which is the only property that makes an allowlist a control rather than a hole.
//
// THE TWO REGISTERS ARE NOT THE SAME THING.
//
//	AllowedInvocations is a PERMIT. Each entry says "this call site invokes a forge CLI and
//	is knowingly still doing so". The target is EMPTY. Every entry carries a TODO naming
//	what has to happen first, because none of them is a case of "we prefer the CLI here".
//
//	UnresolvedArgv is a REGISTER, not a permit. Each entry is an exec site whose argv[0] is
//	not a compile-time constant, so the checker cannot see what it launches. That is a
//	could-not-check, and it is recorded as one: none of these launches a forge CLI on any
//	path the checker can reach, but the checker cannot PROVE it, and a checker that quietly
//	rounded "I could not look" up to "clean" would be exactly the false-clean this ban
//	exists to prevent. Its value is that a new indirection cannot appear silently — a
//	`gh` hidden behind a variable lands here and has to be argued for in a diff.
//
// WHY AllowedInvocations IS NOT EMPTY AT LANDING, stated once rather than repeated per row.
// Nearly every entry below is one of two shapes:
//
//	(identity) The tool reaches the forge under the caller's AMBIENT CLI credential, by
//	documented design — deskclose, deskdigest and deskfile each carry a comment saying the
//	tool gates WHETHER and WHAT, never WHO, and that it mints no token on any path. Both
//	Forge backends REFUSE to construct a client without an explicitly minted token
//	(forge_github.go restClient, forge_gitlab.go client) — deliberately, so that no desk
//	write can silently run as whatever identity happens to be active. Routing these tools
//	through the seam therefore changes WHO performs the write, which is a token-custody
//	decision, not a transport change. It is not those tools' briefs to make.
//
//	deskflip and deskreply were listed under this heading and are NOT here any more: both
//	already MINTED an App installation token and already refused an ambient fallback, so
//	their identity question was answered before the migration and the eight rows they held
//	(deskflip's six reads and its two writes, deskreply's one helper) came off the register
//	with the transport swap. The ceiling below came down by the same eight.
//
//	deskroster's two READ rows (ghViewPR, ghListOpenPRs) are also gone. They were held under
//	the (no-op) heading — one blocked on a title field GetPullRequest lacked, the other on a
//	missing list op — but both blockers are now stale: GetPullRequest gained Title (landed
//	with deskpr edit's consumer) and ListOpenChanges landed (with deskboard's fetchOpenPRs),
//	so the roster's display reads route onto EXISTING enumerated ops with no speculative
//	addition (spec §6). They are display annotations under the session's own minted token, not
//	a write, so no token-custody question gated them. The ceiling came down by the same two.
//
//	(no-op) The operation has no enumerated Forge method, and spec §6's freeze rule forbids
//	adding one without converting its consuming callsite in the same change. PR listing,
//	branch→PR resolution and issue listing are each a real op set with a real GitLab mapping
//	question behind it, and each needs its own brief rather than a speculative method added
//	here. LABELS are no longer in that list — ApplyLabels landed with its two consuming call
//	sites — so the rows below that cited "the label brief" now cite whatever is actually left
//	blocking them. REPO-HARDENING READS are no longer in that list either — the forge-gitlab guard-read-custody brief
//	gave them their own enumerated op (RepoHardeningRead, op 40) and moved
//	`cmd/repohardenguard` onto it under a dedicated read-only `auditor` identity, so this
//	register carries no repohardenguard row any more; the ceiling came down by one more.
//
//	deskdispatch's stepStamp row is gone too (#1154). Its label WRITES were already mapped to
//	ApplyLabels; what held the row was the re-stamp's read of the label HISTORY, which
//	ListLabelEvents now serves on both backends, and identity, which ResolveForge answers by
//	reading the lane's own dispatcher credential (the desk App for a worker dispatch, the
//	reviewer App for a review dispatch). The step now reads the present labels and their
//	history and writes the stamp through the resolved Forge, the same seam its queue-label
//	step already used, so a GitLab-served project is stamped by one code path. The ceiling
//	came down by one more.
//
// A migration that does not answer its row's blocker is not a migration; it is the ban being
// satisfied by moving the identity question somewhere less visible.

// Allowance is one register row. Key is a Finding.Key() — `<file>::<enclosing decl>::<bin>`,
// with `<unresolved>` for an unresolved argv[0]. It is deliberately the WHOLE key as one
// string rather than three fields: a struct whose field held the bare literal "gh" would be
// flagged by this package's own layer 2, and a checker that has to exempt its own allowlist
// file is one edit away from exempting anything else in it.
type Allowance struct {
	Key    string
	Reason string
}

// allowedInvocationCeiling locks in the ban's current strength. It is a RATCHET: the test
// fails when the permit list is longer (a new forge-CLI call site landed) AND when it is
// shorter (a call site was migrated but the gain was not locked in). Lowering it is the
// second half of every migration; raising it is a decision a reviewer sees as a diff.
const allowedInvocationCeiling = 5

// AllowedInvocations permits a resolved forge-CLI invocation at a named call site. TARGET: 0.
var AllowedInvocations = []Allowance{
	{
		Key: "cmd/deskadvisory/advisory.go::ghToken::gh",
		Reason: "TODO(forge-surface): `gh auth token` READS the ambient CLI credential — it is not a forge " +
			"operation at all but the identity layer, which inventory delta D2 keeps deliberately outside the " +
			"interface. Retiring it means giving deskadvisory a minted token of its own; there is no Forge " +
			"method it could move to.",
	},
	{
		Key: "cmd/deskdigest/exec.go::runGH::gh",
		Reason: "TODO(forge-surface): identity, the same documented ambient-credential contract as deskclose. " +
			"Its read verbs also include `issue list`, which has no enumerated op.",
	},
	{
		Key: "cmd/deskdisposition/exec.go::gh::gh",
		Reason: "TODO(forge-surface): mixed, and now WRITE-ONLY — `sweep`'s `pr list` came off this row when it " +
			"migrated onto the enumerated ListOpenChanges (#1123), which is what made the verb answer on a " +
			"GitLab project at all. What is left is `set`: `pr comment` maps to PostComment and the label " +
			"verbs now map to ApplyLabels, but `label list` still has no enumerated op and the verb mints no " +
			"token, so routing its writes through the seam is a token-custody decision.",
	},
	{
		Key: "cmd/deskmerge/exec.go::runGH::gh",
		Reason: "TODO(forge-surface): read-only (`pr view --json`, one `gh api` read of the merge-authority " +
			"surface). The pr view half maps to GetPullRequest; the authority read has no enumerated op.",
	},
	{
		Key: "cmd/deskpushguard/main.go::fetchPR::gh",
		Reason: "TODO(forge-surface): `pr view <branch> --json state,number` resolves a PR from a BRANCH NAME. " +
			"No enumerated op does that — every read on the interface is keyed by number. Needs a typed " +
			"branch→change lookup, with its GitLab source-branch mapping, in its own brief.",
	},
}

// unresolvedRegister records every exec site whose argv[0] the checker cannot resolve. It is
// a LEDGER of blind spots, not a permit — see the file header.
var UnresolvedArgv = []Allowance{
	// cmd/cellctl — the cell launcher (the Go port of tools/cellctl/cellctl). It reaches NO forge
	// at all: its one credential path goes through deskkit.RoleTokenForRepo, and its own brief
	// asserts at the source level that the package carries no signing primitive, no certificate
	// package, no bearer-assertion format and no TLS-toolkit shell-out. What it DOES launch is
	// the local surface a launcher has to: the operator's harness, a terminal multiplexer, the
	// two cockpit CLIs it probes by name, and the cell's own operator-owned binaries. None of
	// those is a forge CLI, and none of the argv[0]s is a compile-time constant because the NAME
	// is the thing being selected at run time.
	{
		Key: "cmd/cellctl/cockpit.go::onPath::<unresolved>",
		Reason: "exec.LookPath of a cockpit/harness name held in a variable (tmux, herdr, orca, claude, " +
			"codex); presence probe, launches nothing.",
	},
	{
		Key: "cmd/cellctl/cockpit.go::runBounded::<unresolved>",
		Reason: "the bounded probe runner. It launches whatever its caller names — in this package only " +
			"`orca repo list`, the desktop-app reachability probe — optionally behind timeout/gtimeout. " +
			"Two sites, because the bound is enforced natively when neither of those is installed.",
	},
	{
		Key: "cmd/cellctl/cockpit.go::helpText::<unresolved>",
		Reason: "runs `<cockpit CLI> --help` to probe which verbs and flags the INSTALLED build advertises " +
			"(herdr and orca ship often and their spellings move); reads help text, acts on nothing else.",
	},
	{
		Key: "cmd/cellctl/container.go::Cell.containerRun::<unresolved>",
		Reason: "runs the operator-registered container launcher named by CELL_CONTAINER_LAUNCHER, which " +
			"cell.env must give as an absolute executable path. Executable argv, never eval; the launcher " +
			"is trusted host code that owns its own runtime custody.",
	},
	{
		Key: "cmd/cellctl/deskd.go::cmdDeskd::<unresolved>",
		Reason: "runs the cell's OWN deskd binary at <cell>/bin/deskd (a read daemon, not a forge CLI). " +
			"Attended-only, and the credentials it is handed come from deskkit.RoleTokenForRepo.",
	},
	{
		Key: "cmd/cellctl/env.go::scrubbedHarnessPath::<unresolved>",
		Reason: "exec.LookPath of the harness name (claude|codex) to pin its directory into a scrubbed " +
			"cell's composed PATH; presence probe, launches nothing.",
	},
	{
		Key: "cmd/cellctl/launch.go::runForeground::<unresolved>",
		Reason: "the hand-over site: replaces this process, as far as the caller can tell, with the harness " +
			"or with tmux attach. argv is composed by the verb that calls it and is never a forge CLI.",
	},
	{
		Key: "cmd/cellctl/smoke.go::cmdSmoke::<unresolved>",
		Reason: "runs the harness once, read-only and tool-free, to prove a scrubbed cell answers READY. " +
			"argv[0] is the selected harness (claude|codex).",
	},
	{
		Key: "cmd/clusterguard/shim.go::passThrough::<unresolved>",
		Reason: "the cluster-CLI shim's ONE pass-through site. argv[0] is the path clusterguard itself " +
			"resolved from PATH for the CLI name it was invoked as, and the shimmed set is a compiled-in " +
			"five (kubectl, flux, helm, talosctl, k9s) — no forge CLI is in it, and no caller can widen it. " +
			"It is reachable only past the operator opt-in, and never through a shell.",
	},
	{
		Key:    "cmd/deskadvisory/advisory.go::runChecks::<unresolved>",
		Reason: "exec.LookPath of a tool name held in a variable; presence probe, launches nothing.",
	},
	{
		Key:    "cmd/deskboard/board.go::execGateScores::<unresolved>",
		Reason: "runs the resolved statusgen binary path; the binary is resolved by deskboard's own resolver.",
	},
	{
		Key:    "cmd/deskboard/dispatch.go::nextUpForRoot::<unresolved>",
		Reason: "runs the resolved statusgen binary path.",
	},
	{
		Key:    "cmd/deskboard/nextup.go::resolveStatusgen::<unresolved>",
		Reason: "exec.LookPath for statusgen; presence probe, launches nothing.",
	},
	{
		Key:    "cmd/deskboard/nextup.go::statusgenVersionOf::<unresolved>",
		Reason: "runs the resolved statusgen binary with --version.",
	},
	{
		Key:    "cmd/deskboard/nextup.go::gateScoresForRoot::<unresolved>",
		Reason: "runs the resolved statusgen binary with --gate-scores.",
	},
	{
		Key:    "cmd/deskpreflight/main.go::realOutput::<unresolved>",
		Reason: "the preflight probe seam; the argv comes from deskkit's preflight probe table, not from a caller.",
	},
	{
		Key:    "cmd/verifyloop/durable.go::newGitDurable::<unresolved>",
		Reason: "git-only durable-state helper; its callers pass git argvs.",
	},
	{
		Key:    "internal/acp/client.go::Spawn::<unresolved>",
		Reason: "spawns the configured agent-protocol server command; not a forge path.",
	},
	{
		Key: "askassay/probe.go::execRead::<unresolved>",
		Reason: "the ask-pane's ONE subprocess site, reachable only past GuardReadOnly — a closed default-deny " +
			"allow-list of binaries, verbs and HTTP methods. It CAN launch `gh` (two registry questions read " +
			"issue counts through it), which is worth stating plainly: the stream brief describes this as a " +
			"vestigial binary-present probe that could simply be dropped, and the tree does not agree — dropping " +
			"`gh` from readOnlyBinaries would remove two answers, not a dead check. Retiring it needs the answers " +
			"re-sourced through the interface first, which is a brief of its own.",
	},
	{
		Key:    "internal/deskkit/callout.go::Callout.Run::<unresolved>",
		Reason: "runs an operator-configured callout binary by path; not a forge path.",
	},
	{
		Key:    "internal/deskkit/preflight.go::coldMintProbe::<unresolved>",
		Reason: "runs the resolved desktoken binary; the identity layer, deliberately outside the interface (D2).",
	},
	{
		Key: "cmd/deskrelease/github.go::resolveDeskTokenPath::<unresolved>",
		Reason: "exec.LookPath of the desktoken binary name (\"desktoken\"/\"desktoken.exe\") — the identity-mint " +
			"layer, deliberately outside the interface (D2), the same binary coldMintProbe runs. argv[0] is a " +
			"variable ONLY because of the .exe OS conditional and because this LookPath is the dev-workflow " +
			"LAST resort: resolveDeskTokenPath prefers the desktoken co-located with the running deskrelease " +
			"binary via os.Executable(), and falls back to PATH lookup then the bare name only when no " +
			"co-located sibling exists (go run/go test). Never a forge CLI. Mirrors migrate.go::resolveStatusgenBinary.",
	},
	{
		Key:    "internal/deskkit/riskcallout.go::runRiskCallout::<unresolved>",
		Reason: "runs the configured risk-classifier binary by path; not a forge path.",
	},
	{
		Key: "internal/deskkit/untrustscan.go::runSemgrep::<unresolved>",
		Reason: "runs the Semgrep engine by resolved path (opts.SemgrepPath or `semgrep` on PATH) over a temp " +
			"copy of the sample with metrics and the version check disabled; not a forge CLI. argv[0] is a variable " +
			"so the checker cannot prove the binary, hence a ledger row, not a permit — Semgrep is a static-analysis " +
			"engine, never a forge surface.",
	},
	{
		Key: "cmd/desksupervise/live.go::showClaim::<unresolved>",
		Reason: "the SINGLE exec site (readLiveClaims' enumeration and reconcile.go's live claim reader " +
			"both route through it) that runs the target repo's own tools/dispatch-claim.sh `show` verb " +
			"(the SAME consumer-repo claim script cmd/deskdispatch/dispatch.go's stepClaim shells to, " +
			"resolved at runtime under --root/--claim-root, never a compile-time literal) to read one " +
			"dispatch claim's state/owner/branch. Not a forge CLI: it is a script this tree does not " +
			"ship, external to every consumer repo it runs against.",
	},
	{
		Key: "internal/deskkit/migrate.go::runStatusgenRegen::<unresolved>",
		Reason: "runs the resolved statusgen binary path for a statusgen-regen migration op; the binary " +
			"is resolved by resolveStatusgenBinary (STATUSGEN_BIN or `statusgen` on PATH — the installed, " +
			"sha256-verified pinned release), never a forge CLI. Mirrors deskboard's execGateScores row.",
	},
	{
		Key:    "internal/deskkit/migrate.go::resolveStatusgenBinary::<unresolved>",
		Reason: "exec.LookPath of the STATUSGEN_BIN env value for a presence probe; launches nothing, and the bare `statusgen` fallback in the same func is a compile-time literal. Not a forge path. Mirrors deskboard's nextup.go::resolveStatusgen row.",
	},
}
