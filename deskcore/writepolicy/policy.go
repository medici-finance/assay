package writepolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// APIVersion identifies the observation schema, not a published module release.
const APIVersion = "writepolicy/v1"

type Profile string

const (
	LegacyHook Profile = "legacy-hook/v1"
	// Managed is an admission contract. Importing it does not qualify or activate
	// an adapter, authenticate its observations, or replace a sandbox.
	Managed Profile = "managed/v1"
)

type Platform string

const (
	POSIX   Platform = "posix"
	Windows Platform = "windows"
)

type Decision string

const (
	Allow   Decision = "allow"
	Deny    Decision = "deny"
	Unknown Decision = "unknown"
)

type ReasonCode string

const (
	NoObjection        ReasonCode = "no-objection"
	Exempt             ReasonCode = "shared-home-exemption"
	SentinelDenied     ReasonCode = "human-only-sentinel"
	SharedDenied       ReasonCode = "shared-write"
	CalloutDenied      ReasonCode = "callout-refusal"
	MissingObservation ReasonCode = "observation-required"
	InvalidObservation ReasonCode = "invalid-observation"
	ManagedHold        ReasonCode = "managed-admission-hold"
	Unsupported        ReasonCode = "unsupported-profile-or-platform"
)

type Request struct{ Tool, Target string }
type LexicalContext struct {
	Platform  Platform
	Cwd, Home string
	Env       map[string]string
}

// Classification freezes a request and its explicit lexical snapshot. No resolver
// callback is accepted. Path requests can depend on earlier symlink observations;
// Evaluate returns the next requests after each batch instead of flattening that
// dependency before its base has been observed.
type Classification struct {
	id      string
	request Request
	lex     LexicalContext
	tokens  []string
	masked  string
}

func Classify(r Request, lex LexicalContext) Classification {
	env := make(map[string]string, len(lex.Env))
	for k, v := range lex.Env {
		env[k] = v
	}
	lex.Env = env
	b, _ := json.Marshal(struct {
		Request Request
		Lexical LexicalContext
	}{r, lex})
	h := sha256.Sum256(b)
	return Classification{id: hex.EncodeToString(h[:]), request: r, lex: lex,
		tokens: pathTokens(r.Target), masked: maskLiterals(r.Target)}
}
func (c Classification) ID() string           { return c.id }
func (c Classification) PathTokens() []string { return append([]string(nil), c.tokens...) }
func (c Classification) Masked() string       { return c.masked }

// ReadsAmbientForgeToken reports whether the classified Bash command runs the
// gh CLI's token read (`gh auth token` in any executed form). It is lexical
// only; whether that is refused is the adapter's gate (forgetoken.go).
func (c Classification) ReadsAmbientForgeToken() bool {
	return forgeTokenRead(c.masked, c.request.Target, 0)
}

type PathRequest struct{ Input string }
type PathState string

const (
	PathResolved PathState = "resolved"
	PathFailed   PathState = "failed"
)

// Input is the exact lexical path requested by Evaluate. Ancestor is the deepest
// existing ancestor, ResolvedAncestor its observed canonical name, and Suffix the
// new descendant. The evaluator validates that the pieces reconstruct Resolved.
type PathObservation struct {
	Input                                        string
	State                                        PathState
	Resolved, Ancestor, ResolvedAncestor, Suffix string
}

type Policy struct {
	Profile                                      Profile
	SharedRoot, ProjectDir, CwdTop, SentinelPath string
}
type ClaimSource string

const (
	NoClaim          ClaimSource = "none"
	EnvironmentClaim ClaimSource = "environment"
	SentinelClaim    ClaimSource = "sentinel"
	AgentClaim       ClaimSource = "agent"
)

type ClaimObservation struct {
	Source              ClaimSource
	Token               string
	Readable, Directory bool
	// BodyPending means only metadata has been observed. Expired claims and
	// directories never request a body read, preserving the legacy effect bound.
	BodyPending bool
	Body        string
	// Numeric times are supplied by the observation boundary. Legacy intentionally
	// accepts a future mtime; Managed refuses it. TTL must be positive.
	ObservedAt, ModifiedAt, TTL int64
}

type Binding struct{ Allocation, Generation, Repository, Owner, Session string }
type ContextObservation struct {
	Expected, Observed     Binding
	Validated              bool
	ProducerAPI            string
	ObservedAt, ValidUntil int64
}
type CalloutState string

const (
	CalloutUnconfigured CalloutState = "unconfigured"
	CalloutPending      CalloutState = "pending"
	CalloutAllow        CalloutState = "allow"
	CalloutRefuse       CalloutState = "refuse"
	CalloutFailure      CalloutState = "failure"
)

type CalloutObservation struct{ State CalloutState }
type Evaluation struct {
	Classified Classification
	// Observations must be bound to the classification they were collected for.
	ClassificationID string
	Policy           Policy
	Paths            []PathObservation
	Claim            ClaimObservation
	Context          ContextObservation
	Callout          CalloutObservation
}
type Detail struct {
	Tool, Target, Home string
	SharedHomed        bool
}
type Need struct {
	Paths     []PathRequest
	Callout   bool
	ClaimBody bool
}
type Outcome struct {
	Decision Decision
	Reason   ReasonCode
	Revision string
	Detail   Detail
	Need     Need
}

type verdict struct {
	Block  bool
	Code   ReasonCode
	Detail Detail
}
type config struct {
	classified                                        Classification
	SharedRoot, ProjectDir, Cwd, CwdTop, SentinelPath string
	SharedOK, managed                                 bool
	lex                                               LexicalContext
	paths                                             pathSyntax
	observed                                          map[string]PathObservation
	pending                                           map[string]PathRequest
	needCallout, needClaimBody, unresolved            *bool
	callout                                           CalloutObservation
}

func outcome(d Decision, r ReasonCode) Outcome {
	return Outcome{Decision: d, Reason: r, Revision: APIVersion}
}

// Evaluate is the only public decision entrypoint. Missing observations produce
// Unknown with requests; callers collect those effects and reevaluate the SAME
// immutable classification. No provisional lexical fallback becomes an allow.
// Adapters must hold Unknown unless the named legacy protocol explicitly maps it
// to no-decision. Managed never receives that mapping.
// semantic: S-write-policy
func Evaluate(in Evaluation) Outcome {
	cl := in.Classified
	if cl.id == "" || in.ClassificationID != cl.id {
		return outcome(Unknown, InvalidObservation)
	}
	if (in.Policy.Profile != LegacyHook && in.Policy.Profile != Managed) ||
		(cl.lex.Platform != POSIX && cl.lex.Platform != Windows) {
		return outcome(Unknown, Unsupported)
	}
	managed := in.Policy.Profile == Managed
	if managed && !validManaged(in) {
		return outcome(Unknown, ManagedHold)
	}
	switch in.Claim.Source {
	case NoClaim, EnvironmentClaim, SentinelClaim, AgentClaim:
	default:
		return outcome(Unknown, InvalidObservation)
	}
	switch in.Callout.State {
	case CalloutUnconfigured, CalloutPending, CalloutAllow, CalloutRefuse, CalloutFailure:
	default:
		return outcome(Unknown, InvalidObservation)
	}
	paths := pathSyntax{platform: cl.lex.Platform}
	observed := make(map[string]PathObservation, len(in.Paths))
	for _, o := range in.Paths {
		if _, duplicate := observed[o.Input]; duplicate {
			return outcome(Unknown, InvalidObservation)
		}
		if o.Input == "" || paths.Clean(o.Input) != o.Input {
			return outcome(Unknown, InvalidObservation)
		}
		switch o.State {
		case PathResolved:
			if o.Resolved == "" || o.Ancestor == "" || o.ResolvedAncestor == "" ||
				paths.Join(o.Ancestor, o.Suffix) != o.Input || paths.Join(o.ResolvedAncestor, o.Suffix) != o.Resolved {
				return outcome(Unknown, InvalidObservation)
			}
		case PathFailed:
			if o.Resolved != "" || o.Ancestor != "" || o.ResolvedAncestor != "" || o.Suffix != "" {
				return outcome(Unknown, InvalidObservation)
			}
		default:
			return outcome(Unknown, InvalidObservation)
		}
		observed[o.Input] = o
	}
	pending := map[string]PathRequest{}
	needCallout, needClaimBody, unresolved := false, false, false
	c := config{SharedRoot: in.Policy.SharedRoot, ProjectDir: in.Policy.ProjectDir, Cwd: cl.lex.Cwd,
		CwdTop: in.Policy.CwdTop, SentinelPath: in.Policy.SentinelPath, managed: managed,
		classified: cl, lex: cl.lex, paths: paths, observed: observed, pending: pending, needCallout: &needCallout,
		needClaimBody: &needClaimBody, unresolved: &unresolved, callout: in.Callout}
	r := cl.request
	result := outcome(Allow, NoObjection)
	switch r.Tool {
	case "Write", "Edit", "MultiEdit", "NotebookEdit", "Bash":
	default:
		if managed {
			return outcome(Unknown, Unsupported)
		}
		return result
	}
	if r.Target == "" {
		if managed {
			return outcome(Unknown, ManagedHold)
		}
		return result
	}
	if managed && r.Tool == "Bash" {
		// An out-of-shared-scope command must not inherit the legacy early allow
		// when its destination is unresolved. This scan only records uncertainty;
		// the normal ordered checks below still decide sentinel/shared refusals.
		c.scanWriteIndicators(r.Target, func(string) bool { return false }, false)
	}
	// Sentinel-before-exemption is authority, not an adapter convention.
	v := c.CheckClaimSentinel(r.Tool, r.Target)
	if !v.Block {
		c.SharedOK = c.claimed(in.Claim)
		if c.Exempt() {
			result.Reason = Exempt
		} else if r.Tool == "Bash" {
			v = c.CheckBash(r.Target)
		} else {
			v = c.CheckFilePath(r.Tool, r.Target)
		}
	}
	if len(pending) > 0 {
		result = outcome(Unknown, MissingObservation)
		for _, req := range pending {
			result.Need.Paths = append(result.Need.Paths, req)
		}
		sort.Slice(result.Need.Paths, func(i, j int) bool { return result.Need.Paths[i].Input < result.Need.Paths[j].Input })
		return result
	}
	if managed && unresolved {
		return outcome(Unknown, ManagedHold)
	}
	if needClaimBody {
		result = outcome(Unknown, MissingObservation)
		result.Need.ClaimBody = true
		return result
	}
	if v.Block {
		result = outcome(Deny, v.Code)
		result.Detail = v.Detail
		return result
	}
	if needCallout {
		result = outcome(Unknown, MissingObservation)
		result.Need.Callout = true
	}
	return result
}

func (c config) claimed(o ClaimObservation) bool {
	if o.Source == EnvironmentClaim {
		return o.Token != "" && !c.managed
	}
	if o.Source != SentinelClaim || o.Directory || o.TTL <= 0 {
		return false
	}
	if o.ObservedAt > o.ModifiedAt && uint64(o.ObservedAt)-uint64(o.ModifiedAt) > uint64(o.TTL) {
		return false
	}
	if c.managed && o.ModifiedAt > o.ObservedAt {
		return false
	}
	if o.BodyPending {
		*c.needClaimBody = true
		return false
	}
	if !o.Readable {
		return false
	}
	scoped := false
	for _, line := range strings.Split(o.Body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		scoped = true
		if c.samePath(c.resolvePath(line), c.resolvePath(c.SharedRoot)) {
			return true
		}
	}
	return !scoped
}

func validManaged(in Evaluation) bool {
	c := in.Context
	b := c.Expected
	return c.Validated && c.ProducerAPI == APIVersion && b == c.Observed &&
		b.Allocation != "" && b.Generation != "" && b.Repository != "" && b.Owner != "" && b.Session != "" &&
		c.ObservedAt > 0 && c.ValidUntil >= c.ObservedAt && in.Classified.lex.Cwd != "" &&
		in.Policy.SharedRoot != "" && in.Policy.ProjectDir != "" && in.Policy.CwdTop != "" &&
		in.Claim.Source != AgentClaim && in.Claim.Source != EnvironmentClaim
}
