// Package schema owns cells.yaml: cells, loops, model ladders, notifications,
// access declarations and fail-closed validation. Identity rollout and human
// authority remain explicit opt-ins with separate runtime checks.
package schema

import (
	"bytes"
	"fmt"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// DefaultAnswerAs reads the explicitly selected startup principal. Empty means
// human operations are disabled; there is no built-in human identity.
func DefaultAnswerAs() string { return humanid.Login() }

// githubLoginRE validates a GitHub login: alphanumeric with internal hyphens,
// not starting or ending with a hyphen, at most 39 chars. answer_as must be a
// real login so the answerer-binding compares against something GitHub can
// authenticate as, never a free-form label.
var githubLoginRE = regexp.MustCompile(`^[A-Za-z0-9](?:-?[A-Za-z0-9])*$`)

// ── per_user_identity: the service contract Task 9 ROLLOUT GATE ─────────
//
// The per-user identity path (OAuth sign-in, the capability ladder, the
// comment tier) is OFF on every cell until a cell EXPLICITLY opts in. This
// block is the config half of that gate; internal/identity + cmd/deskd hold
// the runtime half, and cmd/cellsvalidate is where an operator meets it.
//
// The rollout contract separates five independent decisions:
//
//	fork 1  the ENABLE is this NEW explicit field. ABSENCE is OFF. A dev
//	        `environments:` declaration is a PRECONDITION for setting it and
//	        is never, by itself, an enable — the `assay` cell has declared
//	        dev environments since the service contract for verify-runner RBAC,
//	        which has nothing to do with viewer identity, and reading that
//	        declaration as an enable would switch a sensitive path on for a
//	        private cell the moment this landed.
//	fork 3  exactly `assay` and `house` are proving cells and MAY opt in.
//	fork 4  every OTHER private cell's opt-in is refused UNCONDITIONALLY —
//	        there is no in-config "dev-cell evidence has landed" marker for
//	        an offline validator to read, so the fail-closed reading is the
//	        only honest one.
//	fork 5  "private" = any cell without `allow_anonymous_read: true`. That
//	        is the only static signal an offline validator has; it agrees
//	        with deskd's existing doctrine that an unverifiable repo is
//	        treated as private.

// PerUserIdentity is the per-cell opt-in to the per-user identity path.
//
// The enum is CLOSED AT ONE MEMBER, and that member ENABLES. There is
// deliberately no `disabled`/`off`/`false` value: OFF is expressed by having
// no key at all, so a misspelling can only ever fail validation, never
// silently turn the path on. A second, negative value would have to be
// spelled correctly to keep something off — which is the wrong way round for
// a gate on the auth/identity chain.
type PerUserIdentity string

// PerUserIdentityEnabled is the only value the field admits. Absence is OFF.
const PerUserIdentityEnabled PerUserIdentity = "enabled"

// ProvingCells is the fixed code-owned allowlist of PRIVATE cells that may
// opt in to per-user identity. The current policy admits exactly assay and
// house. All other private cells fail closed, including spelling near misses.
// It is deliberately not a cells.yaml field: a config cannot authorize its
// own opt-in by widening the list it is checked against. Further changes to
// this list require code review and the human authority gate.
var ProvingCells = map[string]bool{"assay": true, "house": true}

// IsProvingCell reports whether a cell name is on the proving allowlist. It
// fails closed on anything else, the empty name included.
func IsProvingCell(name string) bool { return ProvingCells[name] }

// LoopRole is a per-cell CronJob loop role. §9.2 lists review, verify,
// dispatch and metrics as the loop roles this stream builds (briefs 05-07);
// "inbound" is reserved for a later stream and deliberately not accepted
// here yet — widen this set only when a brief actually consumes it.
type LoopRole string

const (
	RoleReview   LoopRole = "review"
	RoleVerify   LoopRole = "verify"
	RoleDispatch LoopRole = "dispatch"
	RoleMetrics  LoopRole = "metrics"
)

var validRoles = map[LoopRole]bool{
	RoleReview:   true,
	RoleVerify:   true,
	RoleDispatch: true,
	RoleMetrics:  true,
}

// NotificationTier is one leg of the §13.3 ambient-agents triad
// (notify / question / review) that decision-model escalations map onto.
type NotificationTier string

const (
	TierReview   NotificationTier = "review"   // gold gates — always push
	TierQuestion NotificationTier = "question" // collect silently — badge only
	TierNotify   NotificationTier = "notify"   // FYI stream — no badge
)

// PushBehavior is what a notification tier resolves to on the menubar/Slack
// push path.
type PushBehavior string

const (
	PushAlways PushBehavior = "push"  // always push (menubar + Slack)
	PushBadge  PushBehavior = "badge" // badge only, no push
	PushNone   PushBehavior = "none"  // no badge, FYI only
)

var validPushBehaviors = map[PushBehavior]bool{
	PushAlways: true,
	PushBadge:  true,
	PushNone:   true,
}

// Loop is one CronJob-per-role-per-cell entry (§9.2).
type Loop struct {
	// Role selects which loop this is; see LoopRole.
	Role LoopRole `yaml:"role"`
	// Schedule is a standard 5-field cron expression.
	Schedule string `yaml:"schedule"`
	// Models is the worker-initiated escalation ladder (§13.8 ruling):
	// cheapest-first, e.g. [deepseek-chat, glm-5.2]. May be empty for a
	// zero-AI, deterministic loop (metrics — the metrics design's "zero-AI first
	// mover"); every other role requires at least one model.
	Models []string `yaml:"models"`
	// App is the GitHub App identity name (bot slug, no [bot] suffix)
	// this loop authenticates as — the loop-identity design names the Secret per role
	// from this field. It is the loop's READ/actor identity and, absent
	// WriteApp, its write identity too.
	App string `yaml:"app"`
	// WriteApp is the OPTIONAL per-loop WRITE-surface identity (bot slug,
	// no [bot] suffix), the service contract read/write App split made
	// per-loop and config-driven (the service contract). When set, the loop
	// READS/acts as App but authors its platform WRITES as WriteApp; when
	// absent (the common case) the loop writes as App — so every existing
	// cell is byte-for-byte unchanged. Read the effective write identity
	// only through WriteAppOrDefault(); never read this field directly, so
	// the default-to-App rule lives in exactly one place.
	//
	// It is a *string, not a string, on purpose: the pointer distinguishes
	// an ABSENT key (nil — legal, defaults to App) from a PRESENT-but-blank
	// `write_app: ""` (non-nil, empty — a config error, mirroring the
	// App-non-empty rule). A plain string cannot tell those apart, so the
	// present-but-blank check would be unenforceable.
	//
	// This field GRANTS no privilege: it names an App slug whose authority
	// is fixed at GitHub App registration/installation. A mis-named or
	// uninstalled WriteApp cannot widen a loop's authority — the write
	// fails at token-mint, it never silently escalates.
	WriteApp *string `yaml:"write_app"`

	// Target is the OPTIONAL "where does this desk run" declaration the
	// cockpit terminal registry resolves a desk id to (the service contract).
	// Absent (the common case) means the desk has no attachable terminal
	// target: the handle read returns state could-not-check, never a 404
	// (spec §5.2). It is a *TargetSpec so an absent key (nil) is legal and
	// distinct from a present-but-malformed one.
	//
	// It GRANTS nothing and NAMES nothing a client sends: the registry — not
	// any request — reads this field to choose a driver (spec §5.1). It is
	// the ONE place a desk id is bound to a container/pod/local target; a
	// client's terminal read carries only the desk id.
	Target *TargetSpec `yaml:"target"`
}

// TargetKind is the driver category a desk's terminal target resolves to. The
// set is CLOSED: the container, pod and local drivers land as briefs 13/14/15,
// each implementing the internal/terminal.Target interface; a kind outside
// this set is refused at config load (cellsvalidate), so a cells.yaml can
// never name a driver that does not exist.
type TargetKind string

const (
	// TargetContainer attaches to a running container (docker exec).
	TargetContainer TargetKind = "container"
	// TargetPod attaches to a pod (kubectl exec), optionally a tmux pane.
	TargetPod TargetKind = "pod"
	// TargetLocal attaches to a local PTY running the given argv.
	TargetLocal TargetKind = "local"
)

var validTargetKinds = map[TargetKind]bool{
	TargetContainer: true,
	TargetPod:       true,
	TargetLocal:     true,
}

// TargetSpec is the parsed cells.yaml `target:` — the declaration the terminal
// registry resolves a desk id to (the service contract). Its shape is one of:
//
//	target: {kind: container, container: <name>}
//	target: {kind: pod, namespace: <ns>, pod: <name-or-selector>, pane: <tmux-pane>}
//	target: {kind: local, command: [<argv>]}
//
// KnownFields(true) at decode makes an unrecognised key here a parse error, so
// the field set below is the whole surface; validate() enforces that the ONE
// kind-specific name field for the declared kind is present.
type TargetSpec struct {
	Kind TargetKind `yaml:"kind"`
	// Container is the container name for kind: container.
	Container string `yaml:"container"`
	// Namespace and Pod address a kind: pod target; Pane optionally names a
	// tmux pane within it.
	Namespace string `yaml:"namespace"`
	Pod       string `yaml:"pod"`
	Pane      string `yaml:"pane"`
	// Command is the argv for kind: local.
	Command []string `yaml:"command"`
}

// validate enforces a known kind and the ONE kind-specific name field that
// makes the target resolvable. A missing name is a config error, not a
// silently-unreachable target: a driver that cannot even be addressed should
// fail at load, where the operator can fix it, not at attach time.
func (t *TargetSpec) validate() error {
	if t.Kind == "" {
		return fmt.Errorf("target: kind is required (one of container|pod|local)")
	}
	if !validTargetKinds[t.Kind] {
		return fmt.Errorf("target: kind %q is not one of container|pod|local", t.Kind)
	}
	switch t.Kind {
	case TargetContainer:
		if t.Container == "" {
			return fmt.Errorf("target: kind container requires a non-empty container:")
		}
	case TargetPod:
		if t.Namespace == "" || t.Pod == "" {
			return fmt.Errorf("target: kind pod requires a non-empty namespace: and pod:")
		}
	case TargetLocal:
		if len(t.Command) == 0 {
			return fmt.Errorf("target: kind local requires a non-empty command: [argv]")
		}
	}
	return nil
}

// WriteAppOrDefault returns the loop's effective WRITE-surface identity: its
// WriteApp when a per-loop write App was set, otherwise its App. This is the
// ONE place the default-to-App rule lives; every write-path consumer resolves
// the write identity through here rather than reading WriteApp directly, so an
// absent field always resolves to the loop's own App and never to empty or a
// hardcoded identity. Validation guarantees a non-nil WriteApp is non-empty,
// so a present WriteApp is always a usable slug.
func (l Loop) WriteAppOrDefault() string {
	if l.WriteApp != nil {
		return *l.WriteApp
	}
	return l.App
}

// ── environments: the dev environments a cell owns (the service contract) ────
//
// Verification of a ledger-backed product is not "run shell in a box": a
// Verify row that claims something about the running system has to reach the
// running system. `environments:` is the DECLARATION half of that — a cell
// says which dev environments it owns, and the cell's verify-runner identity
// is granted access to exactly those (by the deployment's runner manifests and
// per-namespace Roles, which live outside this module).
//
// The whole point of this block is that it is DEV-ONLY BY CONSTRUCTION. This
// validator is the single control standing between "someone edits cells.yaml"
// and "a verify pod is declared against prod". Behind it stand RBAC (bindings
// exist only where one was authored, and none is authored in an
// example-prod-* namespace), the deployment's environment-scope conformance
// check, the verify pool's own pod fence, and the cluster namespace fences —
// but this is the layer that refuses first, and it refuses on a schema/regex
// signal rather than on a cluster one.
//
// So the rules below are deliberately written as "the wrong thing cannot be
// SPELLED", not as "the wrong thing is rejected after being understood":
// there is no `prod` tier value to type, and no `write`/`admin`/
// `cluster-admin` access level to type. A verb the schema cannot say cannot
// be granted by config.

// EnvironmentKind is an environment's substrate. `k8s` is the only value this
// version admits — a second kind (a VM fleet, a hosted ledger) would need its
// own identity + custody design, not a new enum member.
type EnvironmentKind string

// EnvKindK8s is a Kubernetes-namespace-scoped environment.
const EnvKindK8s EnvironmentKind = "k8s"

var validEnvironmentKinds = map[EnvironmentKind]bool{EnvKindK8s: true}

// EnvironmentTier is the deployment tier an environment belongs to. The enum
// is CLOSED AT `dev` on purpose and must stay that way: adding `prod`,
// `staging` or `int` here is the one edit that would let a cell declare a
// non-dev environment, and no amount of downstream RBAC review compensates
// for a config that can express it. If a non-dev tier is ever genuinely
// needed, it needs its own brief, its own identity and its own human gate —
// not a member added to this map.
type EnvironmentTier string

// EnvTierDev is the only tier an environment may declare.
const EnvTierDev EnvironmentTier = "dev"

var validEnvironmentTiers = map[EnvironmentTier]bool{EnvTierDev: true}

// AccessLevel is one capability the cell's verify identity is granted in the
// environment's namespaces. Closed enum, and the closure is the control:
//
//	read           get/list/watch on pods, pods/log, services, endpoints,
//	               events, configmaps — never secrets.
//	logs           pods/log only, for a cell that wants log reach without the
//	               rest of `read`.
//	exec-admin-pod create on pods/exec, so a row may exec the documented probe
//	               scripts in the jump-box (#210 keeps cluster rows on those
//	               scripts; the runner never holds the ledger admin token).
//
// There is deliberately no `write`, no `admin`, no `cluster-admin` and no
// `secrets` level. Those are not omissions to be filled in later — a level
// the schema cannot name cannot be granted by a config edit, which is what
// makes this enum a control rather than documentation.
type AccessLevel string

const (
	// AccessRead is namespace-scoped read on non-secret resources.
	AccessRead AccessLevel = "read"
	// AccessExecAdminPod is create on pods/exec, granted only on its own
	// reviewed change.
	AccessExecAdminPod AccessLevel = "exec-admin-pod"
	// AccessLogs is pods/log read on its own.
	AccessLogs AccessLevel = "logs"
)

var validAccessLevels = map[AccessLevel]bool{
	AccessRead:         true,
	AccessExecAdminPod: true,
	AccessLogs:         true,
}

// devNamespaceRE is the POSITIVE half of the dev-only namespace rule: a
// namespace must carry a `-dev` segment (`example-dev-app`,
// `example-dev-canton`). A name with no `-dev` segment at all — `example-app`,
// `example-loops`, `flux-system` — is not a dev namespace by NAME and is
// refused; naming is the only signal available here, because dev and prod in
// this fleet are sibling namespaces in ONE cluster under ONE context, not a
// context or cluster split. That is precisely why the schema binds namespaces
// and never contexts: a context name proves nothing about blast radius.
var devNamespaceRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*-dev(-[a-z0-9]+)*$`)

// forbiddenNamespaceRE is the NEGATIVE half, applied on top of the positive
// one so the two must BOTH be satisfied. It exists because the positive rule
// alone is satisfiable by a name that also carries a non-dev segment
// (`example-dev-prod-mirror` matches devNamespaceRE). Segment-anchored so it
// catches `prod` as a whole segment and never as a substring of an innocent
// word — `example-dev-production-notes` is caught, a hypothetical
// `example-dev-prodigy` is not a false positive on the `-`/end anchors.
var forbiddenNamespaceRE = regexp.MustCompile(`(^|-)(prod|production|int|validator)(-|$)`)

// cantonNamespaceRE identifies a Canton-participant namespace, for the ONE
// rule that treats such a namespace differently: `exec-admin-pod` may not be
// declared for it.
//
// The namespace isolation policy refuses exec into every Canton namespace.
// App and participant namespaces have different authority boundaries, even
// when they share a cluster or runner. Enforce this at config load so a one-word
// access change cannot silently cross that boundary.
//
// Note the useful side effect: because access is declared per ENVIRONMENT and
// this rule rejects exec on any canton namespace in that environment, a cell
// that wants exec in its app namespace CANNOT express it as one entry
// spanning both namespaces. It must split into two entries with different
// access. The correct shape is the only shape the schema admits.
var cantonNamespaceRE = regexp.MustCompile(`(^|-)canton(-|$)`)

// AdminPod names the documented jump-box workload an `exec-admin-pod` row
// execs into (#210: cluster rows run only via its committed probe scripts).
type AdminPod struct {
	// Namespace must be one of the environment's own namespaces.
	Namespace string `yaml:"namespace"`
	// Workload is the kubectl target, e.g. `deploy/example-admin`.
	Workload string `yaml:"workload"`
}

// Environment is one dev environment a cell owns, and the input from which
// its verify identity's RBAC is authored and audited.
type Environment struct {
	// Name identifies the environment within the cell, e.g. `tracker-dev`.
	Name string `yaml:"name"`
	// Kind is the substrate; `k8s` only in this version.
	Kind EnvironmentKind `yaml:"kind"`
	// Tier is the closed `{dev}` enum.
	Tier EnvironmentTier `yaml:"tier"`
	// Cluster is INFORMATIONAL ONLY — a label for humans reading the file.
	// Nothing is authorized by it and no check reads it: RBAC binds
	// namespaces, and dev/prod here are namespaces inside one cluster.
	Cluster string `yaml:"cluster"`
	// Namespaces is the authorization unit: the dev namespaces the cell's
	// verify identity may be bound in, and nowhere else.
	Namespaces []string `yaml:"namespaces"`
	// Access is the declared capability set; see AccessLevel.
	Access []AccessLevel `yaml:"access"`
	// AdminPod is optional and only meaningful alongside `exec-admin-pod`.
	AdminPod *AdminPod `yaml:"admin_pod"`
	// RunnerPool is the `runs-on` label of the cell's verify runner pool.
	// Unique across ALL cells — two cells sharing one pool would share one
	// identity, and the pool's RBAC would then be the union of both cells'
	// declarations with nothing in the config saying so.
	RunnerPool string `yaml:"runner_pool"`
}

// Cell is one entry in cells.yaml — a repo set plus a stream set (design
// doc §3.2: "cell config is data, not code ... a second cell is a config
// entry rather than a fork").
type Cell struct {
	Name              string                            `yaml:"name"`
	Repos             []string                          `yaml:"repos"`
	StreamsRoot       string                            `yaml:"streams_root"`
	Loops             []Loop                            `yaml:"loops"`
	SilenceThreshold  string                            `yaml:"silence_threshold"`
	NotificationTiers map[NotificationTier]PushBehavior `yaml:"notification_tiers"`
	// AnswerAs is the GitHub login authorized to ANSWER on this cell:
	// the authenticated viewer whose login equals it is the answerer tier. It
	// defaults to the explicit startup principal when human operations are enabled.
	// The answer path authors as that principal via request custody; this field
	// names WHO may invoke it. It cannot enable an unconfigured process.
	AnswerAs string `yaml:"answer_as"`
	// AllowAnonymousRead is a per-cell OPT-IN to anonymous (unauthenticated)
	// read. It defaults to false — deny-by-default. It is valid ONLY
	// when deskd verifies EVERY repo in the cell is public against the GitHub API
	// at startup and each poll cycle; a private repo in an opted-in cell is a
	// fail-closed runtime error (enforced in deskd, not here — the schema cannot
	// see repo visibility). Setting it does NOT itself expose data; the runtime
	// verification does or refuses.
	AllowAnonymousRead bool `yaml:"allow_anonymous_read"`
	// PerUserIdentity is the per-cell opt-in to the per-user identity path
	// (the service contract Task 9). ABSENT — the zero value — is OFF, which is
	// where every cell starts and stays until a human edits this file.
	// See [PerUserIdentity] for why the enum has exactly one member.
	//
	// Setting it is valid only when BOTH hold: the cell declares at least one
	// dev [Environment] (the precondition), and the cell is either public (a
	// verified `allow_anonymous_read` opt-in) or named in [ProvingCells].
	// Read the effective state through [Cell.PerUserIdentityEnabled], never
	// off this field directly, so the three conditions live in one place.
	PerUserIdentity PerUserIdentity `yaml:"per_user_identity"`
	// Environments is OPTIONAL (the service contract). A cell with none simply
	// has no verify pool: its cluster-touching Verify rows stay
	// could-not-check, exactly as they are today. Absence is a valid,
	// meaningful state — never default one in to be helpful.
	Environments []Environment `yaml:"environments"`
	// Chat is the OPTIONAL Ask Assay model-backend configuration
	// (the service contract, design §13.8). ABSENT — nil — is OFF, and OFF is
	// where every cell starts: the pane renders could-not-check rather than a
	// fabricated answer, and no desk data leaves the cluster.
	//
	// It is a *ChatConfig, not a ChatConfig, for the same reason
	// [Loop.WriteApp] is a *string: the pointer distinguishes an ABSENT key
	// (nil — legal, and OFF) from a PRESENT-but-empty `chat: {}` (non-nil, no
	// models — a config error). A plain struct cannot tell those apart, so
	// "a chat block with no model" would validate as OFF, which is precisely
	// the ambiguity a sensitive-data path must not carry.
	Chat *ChatConfig `yaml:"chat"`
	// Overrides is the OPTIONAL per-cell operator override block
	// (the service contract, ruling §10.5) — the record-PR target that mirrors the
	// deskd-local override table to git. ABSENT — nil — is "no overrides", the
	// default. It is validated by the SAME [domain.ValidateOverrides] the HTTP
	// POST applies, called from [validateOverrides] below with this cell's model
	// ladder — one validator, two callers, so the file can never carry a value
	// the API would refuse. The LOCAL table stays authoritative until this
	// block's record PR merges; on the next config reload deskd adopts a block
	// newer than the table (internal/local convergence, layer b).
	Overrides *domain.Overrides `yaml:"overrides"`
	// Master is the OPTIONAL loop role that is this cell's MASTER desk
	// (the service contract, spec §5.2) — a CONFIG KEY, never a heuristic. Absent
	// (the common case) resolves to `dispatch` for backward compatibility with
	// cells that run `dispatch` as their
	// the-desk identity). Read the effective value only through
	// [Cell.MasterRoleOrDefault]; a `master:` naming a role this cell does not
	// configure is a validation error, never a silent fallback.
	Master string `yaml:"master"`
	// Peers is the OPTIONAL list of other cells' masters this cell lists as
	// peers (the service contract, spec §5.2) — the desk registry read renders
	// them as `group: peer` rows carrying the peer's URL so a client can go
	// there directly (deskd never proxies; spec §7). This brief only LISTS
	// peers (state always could-not-check — no cross-deskd read here); peer reachability
	// makes them reachable.
	Peers []Peer `yaml:"peers"`
}

// Peer is one entry of a cell's OPTIONAL `peers:` list (the service contract):
// another cell's deskd this one is configured to list as a peer master.
type Peer struct {
	// Cell is the peer cell's name, e.g. "tracker".
	Cell string `yaml:"cell"`
	// URL is the peer deskd's base URL. It must be https, or http against a
	// loopback host (127.0.0.1/localhost) — the same rule a local dev deskd
	// would need and nothing wider, so a peer entry can never point a client
	// at an arbitrary plaintext network address.
	URL string `yaml:"url"`
}

// AnswerAsLogin returns the cell's authorized-answerer login, applying the
// DefaultAnswerAs() fallback. It is the single place the default is resolved, so
// deskd and the schema agree.
func (c *Cell) AnswerAsLogin() string {
	if !humanid.Enabled() {
		return ""
	}
	if c.AnswerAs == "" {
		return DefaultAnswerAs()
	}
	return c.AnswerAs
}

// IsPrivate reports whether the cell is PRIVATE for the purposes of the
// rollout gate: any cell that does not carry `allow_anonymous_read: true`.
//
// This is a deliberately coarse, OFFLINE reading. cellsvalidate has no GitHub
// access and cannot see repo visibility, so the only static signal available
// is the operator's own anonymous-read opt-in — and that opt-in is itself
// verified against the API at deskd startup (every repo confirmed public, or
// deskd refuses to start). A cell that has not claimed to be public is
// treated as private, which is the fail-closed direction.
func (c *Cell) IsPrivate() bool { return !c.AllowAnonymousRead }

// HasDevEnvironment reports whether the cell declares at least one dev
// environment — the PRECONDITION for the per-user-identity opt-in.
//
// The tier is checked explicitly even though [EnvironmentTier] is closed at
// `dev` today. The closure is the control that keeps a prod environment from
// being declarable at all; this check is what keeps THIS rule correct if that
// enum is ever widened by a future brief, rather than silently accepting a
// non-dev environment as the precondition.
func (c *Cell) HasDevEnvironment() bool {
	for _, e := range c.Environments {
		if e.Tier == EnvTierDev {
			return true
		}
	}
	return false
}

// PerUserIdentityEnabled reports whether the per-user identity path is live
// for this cell. It is the SINGLE resolution of the gate: cellsvalidate
// refuses a document that could not satisfy it, and deskd calls it again at
// runtime to decide what to serve.
//
// The duplication is deliberate defense in depth, not redundancy: the
// validator refuses the FILE on a schema signal, at config-edit time; this
// predicate refuses the BEHAVIOUR on a runtime signal, per request, in
// another process. A deskd handed a config that never went through
// [File.Validate] — an older binary, a hand-edited file, a future loader —
// still fails closed, because it re-derives all three conditions itself
// rather than trusting that someone else checked them.
func (c *Cell) PerUserIdentityEnabled() bool {
	if c.PerUserIdentity != PerUserIdentityEnabled {
		return false // absent, or any value validation would have refused
	}
	if !c.HasDevEnvironment() {
		return false // precondition unmet — or unresolvable, which is the same answer
	}
	if c.IsPrivate() && !IsProvingCell(c.Name) {
		return false
	}
	return true
}

// File is the top-level cells.yaml document.
type File struct {
	Cells []Cell `yaml:"cells"`
}

// Load reads and strictly validates a cells.yaml file at path: unknown
// fields anywhere in the document are errors, and every
// structural/semantic rule below must hold.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Parse(data)
}

// Parse strictly decodes and validates cells.yaml content.
func Parse(data []byte) (*File, error) {
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // unknown fields are errors
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

// Validate checks every semantic rule cells.yaml must satisfy. Structural
// unknown-field rejection already happened during decode (KnownFields).
func (f *File) Validate() error {
	if len(f.Cells) == 0 {
		return fmt.Errorf("cells: at least one cell is required")
	}
	seen := map[string]bool{}
	// runner_pool uniqueness is a FILE-level rule, not a cell-level one:
	// two CELLS sharing a verify pool share one identity, so the pool's RBAC
	// silently becomes the union of both cells' declarations with nothing in
	// the config recording that. Checked here because a per-cell validator
	// structurally cannot see it.
	//
	// ACROSS CELLS, not across environments. One cell may legitimately run
	// several environments on ONE identity. App and participant environments
	// need separate access declarations under the namespace exec rule, while
	// sharing the same cell's runner pool. Rejecting that would require either
	// a second runner identity or an access override inside one environment. Both are
	// worse: the first multiplies identities to encode a permission, and the
	// second hides the difference inside an entry that still reads as uniform.
	//
	// The invariant that actually matters is preserved and is what the map
	// records: one pool belongs to one CELL.
	seenPools := map[string]string{} // pool -> the cell that claimed it
	for i, c := range f.Cells {
		if err := c.validate(); err != nil {
			return fmt.Errorf("cells[%d] (%s): %w", i, c.Name, err)
		}
		if seen[c.Name] {
			return fmt.Errorf("cells[%d]: duplicate cell name %q", i, c.Name)
		}
		seen[c.Name] = true
		for j, e := range c.Environments {
			if owner, dup := seenPools[e.RunnerPool]; dup && owner != c.Name {
				return fmt.Errorf(
					"cells[%d] (%s): environments[%d] (%s): runner_pool %q is already declared by cell %q — a verify pool is one identity and may not be shared between cells",
					i, c.Name, j, e.Name, e.RunnerPool, owner)
			}
			seenPools[e.RunnerPool] = c.Name
		}
	}
	return nil
}

func (c *Cell) validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	// A desk id is "<cell>/<role>" and every consumer reads the cell as the
	// text before the first "/", so a "/" in a cell name would let one cell's
	// scope cover another's desks (review pr192-S2).
	if strings.Contains(c.Name, "/") {
		return fmt.Errorf("name %q must not contain \"/\": desk ids are <cell>/<role>, so the cell is read up to the first slash", c.Name)
	}
	if len(c.Repos) == 0 {
		return fmt.Errorf("repos: at least one repo is required")
	}
	for _, r := range c.Repos {
		if r == "" {
			return fmt.Errorf("repos: entries must not be empty")
		}
	}
	if c.StreamsRoot == "" {
		return fmt.Errorf("streams_root is required")
	}
	if len(c.Loops) == 0 {
		return fmt.Errorf("loops: at least one loop is required")
	}
	seenRoles := map[LoopRole]bool{}
	for i, l := range c.Loops {
		if err := l.validate(); err != nil {
			return fmt.Errorf("loops[%d]: %w", i, err)
		}
		if seenRoles[l.Role] {
			return fmt.Errorf("loops[%d]: duplicate role %q for cell %q", i, l.Role, c.Name)
		}
		seenRoles[l.Role] = true
	}
	if c.SilenceThreshold == "" {
		return fmt.Errorf("silence_threshold is required")
	}
	if _, err := time.ParseDuration(c.SilenceThreshold); err != nil {
		return fmt.Errorf("silence_threshold: %w", err)
	}
	if err := validateNotificationTiers(c.NotificationTiers); err != nil {
		return err
	}
	// answer_as is optional (defaults to DefaultAnswerAs()); when present it must
	// be a real GitHub login. len<=39 is GitHub's own cap.
	if c.AnswerAs != "" {
		if len(c.AnswerAs) > 39 || !githubLoginRE.MatchString(c.AnswerAs) {
			return fmt.Errorf("answer_as %q is not a valid GitHub login (alphanumeric with internal hyphens, <=39 chars)", c.AnswerAs)
		}
	}
	// environments is optional; when present every entry must hold, and
	// names must be unique within the cell.
	seenEnvs := map[string]bool{}
	for i, e := range c.Environments {
		if err := e.validate(); err != nil {
			return fmt.Errorf("environments[%d] (%s): %w", i, e.Name, err)
		}
		if seenEnvs[e.Name] {
			return fmt.Errorf("environments[%d]: duplicate environment name %q", i, e.Name)
		}
		seenEnvs[e.Name] = true
	}
	// chat is optional (the service contract); absence is OFF. When present, the
	// §13.8 model policy binds: closed ladder, cheapest-first, and a key that
	// is NAMED rather than carried.
	if c.Chat != nil {
		if err := c.Chat.validate(); err != nil {
			return err
		}
	}
	// overrides is optional (the service contract); absence is "no overrides".
	// When present it is validated by the SAME validator the HTTP POST applies.
	if err := c.validateOverrides(); err != nil {
		return err
	}
	// master/peers (the service contract): master must name a configured loop
	// role when present; each peer must carry a name and an https/loopback url.
	if err := c.validateMasterAndPeers(); err != nil {
		return err
	}
	// The rollout gate (the service contract Task 9) is checked LAST, because it
	// reads the environments validated just above: a malformed environment
	// should report as malformed, not as a failed precondition.
	if err := c.validatePerUserIdentity(); err != nil {
		return err
	}
	return nil
}

// ChatEnabled reports whether the Ask Assay model backend is configured on
// this cell. Read the effective state through here rather than off the Chat
// field, so "absence is OFF" lives in one place.
func (c *Cell) ChatEnabled() bool { return c.Chat != nil }

// ModelLadderIDs is the union of the model ids this cell's loops name, in
// first-seen order — the closed set a `model_floor` override may raise to. It
// is the cell context [domain.ValidateOverrides] needs, resolved here so the
// HTTP POST handler and this validator pass the SAME set to the SAME validator.
func (c *Cell) ModelLadderIDs() []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range c.Loops {
		for _, m := range l.Models {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	return out
}

// validateOverrides refuses a cells.yaml `overrides:` block the HTTP POST would
// refuse — it calls the shared [domain.ValidateOverrides] with this cell's own
// model ladder, so the file and the API can never diverge on what a value may
// be (spec: one validator, two callers). Absence is valid: no overrides is the
// default state.
func (c *Cell) validateOverrides() error {
	if c.Overrides == nil {
		return nil
	}
	if err := domain.ValidateOverrides(*c.Overrides, c.ModelLadderIDs()); err != nil {
		return fmt.Errorf("overrides: %w", err)
	}
	return nil
}

// DefaultMasterRole is the master loop role when a cell omits master.
// It is the coordinator's dispatch role.
const DefaultMasterRole = RoleDispatch

// MasterRoleOrDefault returns the cell's effective master loop role, applying
// [DefaultMasterRole] when `master:` is absent. Read the effective value only
// through here — never off c.Master directly — so the one default lives in
// one place, exactly as [Cell.AnswerAsLogin] does for answer_as.
func (c *Cell) MasterRoleOrDefault() LoopRole {
	if c.Master == "" {
		return DefaultMasterRole
	}
	return LoopRole(c.Master)
}

// peerURLRE is the loopback exception to the https-only rule: http is legal
// only against 127.0.0.1 or localhost, optionally with a port and path — the
// shape a local dev deskd needs and nothing wider (the service contract).
var peerURLRE = regexp.MustCompile(`^http://(127\.0\.0\.1|localhost)(:[0-9]+)?(/.*)?$`)

// validateMasterAndPeers enforces the two the service contract rules: `master:`,
// when present, must name a role this cell actually configures — a heuristic
// like "the first loop" is exactly what this field replaces, so a name that
// resolves to nothing is a config error, never a silent fallback to
// [DefaultMasterRole]. Each peer must carry a non-empty cell name and a
// url that is https, or http against a loopback host.
func (c *Cell) validateMasterAndPeers() error {
	if c.Master != "" {
		found := false
		for _, l := range c.Loops {
			if l.Role == LoopRole(c.Master) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("master %q names no loop role this cell configures (the service contract: a master: naming an unconfigured role is refused, never a silent fallback)", c.Master)
		}
	}
	seenPeers := map[string]bool{}
	for i, p := range c.Peers {
		if p.Cell == "" {
			return fmt.Errorf("peers[%d]: cell is required", i)
		}
		if p.Cell == c.Name {
			return fmt.Errorf("peers[%d]: cell %q names itself — a cell cannot peer with itself", i, p.Cell)
		}
		if seenPeers[p.Cell] {
			return fmt.Errorf("peers[%d]: duplicate peer cell %q", i, p.Cell)
		}
		seenPeers[p.Cell] = true
		if !strings.HasPrefix(p.URL, "https://") && !peerURLRE.MatchString(p.URL) {
			return fmt.Errorf("peers[%d] (%s): url %q must be https, or http against a loopback host (127.0.0.1/localhost) — never a plaintext network address", i, p.Cell, p.URL)
		}
	}
	return nil
}

// validatePerUserIdentity enforces the three rollout-gate rules. Each refusal
// names the cell and states the reason, so an operator reading cellsvalidate's
// stderr learns the rule rather than only that they were refused (Task 9).
func (c *Cell) validatePerUserIdentity() error {
	if c.PerUserIdentity == "" {
		return nil // ABSENT is OFF, and OFF is always valid — the default state
	}
	if c.PerUserIdentity != PerUserIdentityEnabled {
		return fmt.Errorf(
			"per_user_identity %q is not a value this field admits: the only value is %q, and OFF is expressed by OMITTING the key entirely (there is no disabled/false/off value, so a typo can never turn the per-user identity path on)",
			c.PerUserIdentity, PerUserIdentityEnabled)
	}
	if !c.HasDevEnvironment() {
		return fmt.Errorf(
			"cell %q sets per_user_identity: %s but declares no %s environment — a dev environments: declaration is the PRECONDITION for the opt-in (the service contract Task 9). Declaring one does NOT by itself enable the path, and enabling the path without one is refused",
			c.Name, PerUserIdentityEnabled, EnvTierDev)
	}
	if c.IsPrivate() && !IsProvingCell(c.Name) {
		return fmt.Errorf(
			"cell %q sets per_user_identity: %s but is PRIVATE (no allow_anonymous_read: true) and is not on the proving-cell allowlist — a private cell may not enable the per-user identity path before the dev-cell rows are proved on a proving cell (the service contract Verify 12b, the authorization contract). The proving allowlist is a compile-time constant in config/schema/cells.go, deliberately not a field of this file: widening it is a reviewed code change past the brief's human gate",
			c.Name, PerUserIdentityEnabled)
	}
	return nil
}

// validate enforces every dev-only rule on one environment entry
// (the service contract task 1). Read the ORDER as deliberate: `tier` and the
// namespace rules are checked before anything cosmetic, so the error a
// misdeclared prod environment produces names the prod thing, not a missing
// `cluster` field it also happened to lack.
func (e *Environment) validate() error {
	if e.Name == "" {
		return fmt.Errorf("name is required")
	}
	if e.Kind == "" {
		return fmt.Errorf("kind is required (only %q is supported in this version)", EnvKindK8s)
	}
	if !validEnvironmentKinds[e.Kind] {
		return fmt.Errorf("kind %q is not supported (only %q in this version)", e.Kind, EnvKindK8s)
	}
	if e.Tier == "" {
		return fmt.Errorf("tier is required and must be %q — environments are dev-only by construction", EnvTierDev)
	}
	if !validEnvironmentTiers[e.Tier] {
		return fmt.Errorf(
			"tier %q is not a declarable tier: the enum is closed at %q, so there is no prod/staging/int value to accept — environments are dev-only by construction",
			e.Tier, EnvTierDev)
	}
	if len(e.Namespaces) == 0 {
		return fmt.Errorf(
			"namespaces: at least one namespace is required — RBAC binds namespaces, never a cluster or a context, so an entry naming only a cluster authorizes nothing and must not be declarable")
	}
	seenNS := map[string]bool{}
	for _, ns := range e.Namespaces {
		if ns == "" {
			return fmt.Errorf("namespaces: entries must not be empty")
		}
		if seenNS[ns] {
			return fmt.Errorf("namespaces: duplicate namespace %q", ns)
		}
		seenNS[ns] = true
		if !devNamespaceRE.MatchString(ns) {
			return fmt.Errorf(
				"namespaces: %q is not a dev namespace: it must match %s (dev-only rule — every declared namespace carries a -dev segment)",
				ns, devNamespaceRE)
		}
		if forbiddenNamespaceRE.MatchString(ns) {
			return fmt.Errorf(
				"namespaces: %q names a forbidden segment: it must not match %s (dev-only rule — prod/production/int/validator namespaces may never be declared)",
				ns, forbiddenNamespaceRE)
		}
	}
	if len(e.Access) == 0 {
		return fmt.Errorf("access: at least one access level is required (one of %s)", accessLevelList())
	}
	seenAccess := map[AccessLevel]bool{}
	for _, a := range e.Access {
		if !validAccessLevels[a] {
			return fmt.Errorf(
				"access: %q is not a grantable access level: the enum is closed at %s, so write/admin/cluster-admin/secrets cannot be expressed here at all",
				a, accessLevelList())
		}
		if seenAccess[a] {
			return fmt.Errorf("access: duplicate access level %q", a)
		}
		seenAccess[a] = true
	}
	// `exec-admin-pod` is refused for a Canton namespace, unconditionally.
	// See cantonNamespaceRE for why this is a rule and not a comment. Checked
	// after the access enum so an unrecognised level is reported as such
	// rather than as a canton violation.
	if seenAccess[AccessExecAdminPod] {
		for _, ns := range e.Namespaces {
			if cantonNamespaceRE.MatchString(ns) {
				return fmt.Errorf(
					"access: %q may not be declared for the Canton namespace %q — namespace isolation forbids this access level. Declare the app namespace and the canton namespace as SEPARATE environments with different access",
					AccessExecAdminPod, ns)
			}
		}
	}
	if e.AdminPod != nil {
		if e.AdminPod.Namespace == "" {
			return fmt.Errorf("admin_pod.namespace is required when admin_pod is set")
		}
		if e.AdminPod.Workload == "" {
			return fmt.Errorf("admin_pod.workload is required when admin_pod is set")
		}
		if !seenNS[e.AdminPod.Namespace] {
			return fmt.Errorf(
				"admin_pod.namespace %q is not one of this environment's namespaces %v — the jump-box must live inside the declared blast radius, never beside it",
				e.AdminPod.Namespace, e.Namespaces)
		}
	}
	if e.RunnerPool == "" {
		return fmt.Errorf("runner_pool is required — it is the runs-on label the verify identity is reached through")
	}
	return nil
}

// accessLevelList renders the closed access enum in a stable order for error
// messages (map iteration order is randomised, and an error string that
// changes between runs is one nobody can grep a log for).
func accessLevelList() string {
	return fmt.Sprintf("{%s, %s, %s}", AccessRead, AccessExecAdminPod, AccessLogs)
}

func (l *Loop) validate() error {
	if l.Role == "" {
		return fmt.Errorf("role is required")
	}
	if !validRoles[l.Role] {
		return fmt.Errorf("role %q is not a recognised loop role", l.Role)
	}
	if l.Schedule == "" {
		return fmt.Errorf("schedule is required")
	}
	if err := validateCronFields(l.Schedule); err != nil {
		return fmt.Errorf("schedule: %w", err)
	}
	// Zero-AI loops (metrics — the metrics design) may ship an empty ladder;
	// every other role needs at least one model to escalate from.
	if l.Role != RoleMetrics && len(l.Models) == 0 {
		return fmt.Errorf("models: at least one model is required for role %q", l.Role)
	}
	if l.App == "" {
		return fmt.Errorf("app is required")
	}
	// write_app is OPTIONAL (absence is legal and the common case), but a
	// PRESENT key must be non-empty — `write_app: ""` is a config error,
	// mirroring the app-non-empty rule above. The App-required check above
	// is independent and unchanged: both validations stand.
	if l.WriteApp != nil && *l.WriteApp == "" {
		return fmt.Errorf("write_app must be non-empty when present (omit the key to write as app)")
	}
	// target: is OPTIONAL (absence is legal and the common case); a PRESENT
	// one must name a known kind and its kind-specific address (the service contract).
	if l.Target != nil {
		if err := l.Target.validate(); err != nil {
			return err
		}
	}
	return nil
}

// validateNotificationTiers enforces the §13.3 triad exactly: all three
// keys (review, question, notify) present, no extras, values from the
// known push-behavior set.
func validateNotificationTiers(m map[NotificationTier]PushBehavior) error {
	if m == nil {
		return fmt.Errorf("notification_tiers is required")
	}
	want := []NotificationTier{TierReview, TierQuestion, TierNotify}
	if len(m) != len(want) {
		return fmt.Errorf("notification_tiers: must have exactly the triad %v, got %d entries", want, len(m))
	}
	for _, tier := range want {
		behavior, ok := m[tier]
		if !ok {
			return fmt.Errorf("notification_tiers: missing tier %q", tier)
		}
		if !validPushBehaviors[behavior] {
			return fmt.Errorf("notification_tiers[%q]: unrecognised push behavior %q", tier, behavior)
		}
	}
	return nil
}

// validateCronFields does a light structural check (5 whitespace-separated
// fields) — not a full cron-expression parse, which is out of scope for
// this brief.
func validateCronFields(schedule string) error {
	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return fmt.Errorf("expected 5 cron fields (min hour dom month dow), got %d in %q", len(fields), schedule)
	}
	return nil
}
