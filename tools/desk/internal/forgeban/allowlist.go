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
//	documented design — deskclose, deskdigest, deskfile and deskflip each carry a comment
//	saying the tool gates WHETHER and WHAT, never WHO, and that it mints no token on any
//	path. Both Forge backends REFUSE to construct a client without an explicitly minted
//	token (forge_github.go restClient, forge_gitlab.go client) — deliberately, so that no
//	desk write can silently run as whatever identity happens to be active. Routing these
//	tools through the seam therefore changes WHO performs the write, which is a token-custody
//	decision, not a transport change. It is not this brief's to make.
//
//	(no-op) The operation has no enumerated Forge method, and spec §6's freeze rule forbids
//	adding one without converting its consuming callsite in the same change. Labels, PR
//	listing, branch→PR resolution, repo-hardening reads and issue listing are each a real op
//	set with a real GitLab mapping question behind it, and each needs its own brief rather
//	than a speculative method added here.
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
const allowedInvocationCeiling = 24

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
		Key: "cmd/deskboard/board.go::ghRun::gh",
		Reason: "TODO(forge-surface): the board's whole read surface (pr list, pr diff, repo contents, the " +
			"compare endpoint, two GraphQL queries and the issues-by-label walk). Four of those have enumerated " +
			"ops; pr list, pr diff, contents, compare and the GraphQL reads do not, and deskboard mints no token.",
	},
	{
		Key: "cmd/deskclose/exec.go::runGH::gh",
		Reason: "TODO(forge-surface): identity. GetIssue/PostComment/CloseIssue all exist on the interface, so " +
			"the ops are there — but exec.go states the contract explicitly: deskclose gates WHETHER and WHAT, " +
			"never WHO, and mints no token on any path. Migrating changes the closing identity.",
	},
	{
		Key: "cmd/deskdigest/exec.go::runGH::gh",
		Reason: "TODO(forge-surface): identity, the same documented ambient-credential contract as deskclose. " +
			"Its read verbs also include `issue list`, which has no enumerated op.",
	},
	{
		Key: "cmd/deskdispatch/dispatch.go::stepStamp::gh",
		Reason: "TODO(forge-surface): `label create` + `pr edit --add-label`. Labels are not in the frozen op " +
			"set; a SetLabels/CreateLabel pair needs its own brief (GitLab labels are project-scoped with a " +
			"different create/idempotency shape) rather than being invented here.",
	},
	{
		Key: "cmd/deskdisposition/exec.go::gh::gh",
		Reason: "TODO(forge-surface): mixed. `pr comment` maps to PostComment; `pr edit --add-label`, " +
			"`label list`, `label create` and `pr list` have no enumerated op. Blocked on the same label brief.",
	},
	{
		Key: "cmd/deskfile/exec.go::gh::gh",
		Reason: "TODO(forge-surface): identity. FileIssue/PostComment/CloseIssue exist; exec.go states deskfile " +
			"gates WHETHER and WHERE an issue is filed, never WHO, and mints no App token on any path.",
	},
	{
		Key: "cmd/deskflip/flip.go::flip::gh",
		Reason: "TODO(forge-surface): `pr ready` — MarkReadyForReview exists and takes the node id, which the " +
			"tool would first have to read. Blocked with the rest of deskflip on identity: the tool mints no token.",
	},
	{
		Key: "cmd/deskflip/flip.go::ensureLabelSwap::gh",
		Reason: "TODO(forge-surface): `pr edit --add-label`/`--remove-label`. No enumerated label op; blocked on " +
			"the label brief and on deskflip's identity question.",
	},
	{
		Key:    "cmd/deskflip/flip.go::readPR::gh",
		Reason: "TODO(forge-surface): `pr view --json` maps to GetPullRequest. Identity-blocked only.",
	},
	{
		Key: "cmd/deskflip/flip.go::readReviews::gh",
		Reason: "TODO(forge-surface): `gh api --paginate …/pulls/N/reviews` maps to ReviewsAtHead — and is a " +
			"PASSTHROUGH-shaped argv besides. Identity-blocked only; this is the next best migration after the " +
			"claim-release sink.",
	},
	{
		Key: "cmd/deskflip/flip.go::readChangedFiles::gh",
		Reason: "TODO(forge-surface): `gh api --paginate …/pulls/N/files` maps to ListChangedFiles, and is " +
			"passthrough-shaped. Identity-blocked only.",
	},
	{
		Key: "cmd/deskflip/flip.go::readHead::gh",
		Reason: "TODO(forge-surface): `pr view --json headRefOid` maps to GetPullRequest.HeadSHA. " +
			"Identity-blocked only.",
	},
	{
		Key: "cmd/deskflip/flip.go::readLabelEvents::gh",
		Reason: "TODO(forge-surface): `gh api --paginate …/issues/N/timeline` maps to a would-be " +
			"ListLabelEvents op (the applier-aware model-capability-floor read); passthrough-shaped, and " +
			"identity-blocked only with the rest of deskflip. The HTTP-client sibling (deskpost listLabelEvents) " +
			"already goes through the enumerated path.",
	},
	{
		Key: "cmd/deskmerge/exec.go::runGH::gh",
		Reason: "TODO(forge-surface): read-only (`pr view --json`, one `gh api` read of the merge-authority " +
			"surface). The pr view half maps to GetPullRequest; the authority read has no enumerated op.",
	},
	{
		Key: "cmd/deskpr/exec.go::gh::gh",
		Reason: "TODO(forge-surface): CreateDraftChange and GetPullRequest both exist and deskpr DOES mint a " +
			"worker token — but it also ships a documented `--as-app=false` ambient-identity fallback that the " +
			"token-refusing backends cannot serve. Retiring that flag is a behaviour change for its callers.",
	},
	{
		Key: "cmd/deskpushguard/main.go::fetchPR::gh",
		Reason: "TODO(forge-surface): `pr view <branch> --json state,number` resolves a PR from a BRANCH NAME. " +
			"No enumerated op does that — every read on the interface is keyed by number. Needs a typed " +
			"branch→change lookup, with its GitLab source-branch mapping, in its own brief.",
	},
	{
		Key: "cmd/deskreply/exec.go::gh::gh",
		Reason: "TODO(forge-surface): PostComment exists and deskreply mints a token, so this is the closest " +
			"of the identity-class rows to migratable; it is held with the rest so the identity ruling lands once " +
			"rather than tool by tool.",
	},
	{
		Key: "cmd/deskroster/roster.go::ghViewPR::gh",
		Reason: "TODO(forge-surface): `pr view --json state,isDraft,title`. GetPullRequest carries state and " +
			"draft but NOT title, so the migration needs either a field added to PullRequest (freeze rule: with " +
			"its consumer) or the roster to stop displaying titles.",
	},
	{
		Key: "cmd/deskroster/roster.go::ghListOpenPRs::gh",
		Reason: "TODO(forge-surface): `pr list --state open`. No enumerated list op; a ListChanges method is a " +
			"real addition with a real GitLab paging shape behind it.",
	},
	{
		Key: "cmd/issueboard/board.go::ghRun::gh",
		Reason: "TODO(forge-surface): `issue list`, `issue view --json title`, and a GraphQL count. No " +
			"enumerated issue-listing op; the count query has no REST equivalent on either forge.",
	},
	{
		Key: "cmd/repohardenguard/check.go::ghRun::gh",
		Reason: "TODO(forge-surface): `gh api` reads of rulesets, branch protection and App permissions. These " +
			"are repo-HARDENING reads, not workflow forge ops — the same class inventory delta D3 keeps out of " +
			"the frozen set. They need their own enumerated surface and their own GitLab mapping (protected " +
			"branches + push rules), which is a brief, not a line.",
	},
	{
		Key:    "cmd/scanloop/lane.go::scanCarrierPRLane.Execute::gh",
		Reason: "TODO(forge-surface): `pr edit`. Blocked on the label/edit brief.",
	},
	{
		Key: "cmd/scanloop/trust.go::ghTrustProbe::gh",
		Reason: "TODO(forge-surface): GET-only trust reads (author identity, and the association read for an " +
			"untrusted author). The author half is covered by GetIssue/GetPullRequest's Account; the association " +
			"read is not enumerated.",
	},
}

// unresolvedRegister records every exec site whose argv[0] the checker cannot resolve. It is
// a LEDGER of blind spots, not a permit — see the file header.
var UnresolvedArgv = []Allowance{
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
		Key: "cmd/scanloop/lane.go::RealExec::<unresolved>",
		Reason: "the lane executor seam. Its callers pass literal argvs, so a forge CLI reaching it is caught " +
			"by layer 2 at the caller — which is how the one `gh pr edit` caller in this file is already listed.",
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
		Key: "internal/askassay/probe.go::execRead::<unresolved>",
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
		Key:    "internal/deskkit/riskcallout.go::runRiskCallout::<unresolved>",
		Reason: "runs the configured risk-classifier binary by path; not a forge path.",
	},
}
