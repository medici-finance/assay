package perfdata

// context.go — the aliased correlation context. Every identity here is a random alias
// minted by the local source/epoch alias mapper (see alias.go); a raw native ID, a
// qualified work locator or any free text fails the alias pattern and is refused.

// DeskRole is the originating desk role (attribution only, never authorization).
type DeskRole string

const (
	DeskCoordinator DeskRole = "the-desk"
	DeskWorker      DeskRole = "worker-desk"
	DeskReview      DeskRole = "pr-review-desk"
	DeskVerify      DeskRole = "verify-desk"
	DeskIntake      DeskRole = "intake-desk"
	DeskUnavailable DeskRole = "unavailable"
)

// Role is the assigned execution function, supplied by the launcher.
type Role string

const (
	RoleCoordinate  Role = "coordinate"
	RoleImplement   Role = "implement"
	RoleReview      Role = "review"
	RoleVerify      Role = "verify"
	RoleTriage      Role = "triage"
	RoleInvestigate Role = "investigate"
	RoleOther       Role = "other"
	RoleUnavailable Role = "unavailable"
)

// WorkKind is the kind of a typed work reference.
type WorkKind string

const (
	WorkBrief WorkKind = "brief"
	WorkIssue WorkKind = "issue"
)

// MaxWorkRefs bounds the work references on one event.
const MaxWorkRefs = 8

// WorkRef is one aliased work reference.
type WorkRef struct {
	Kind    WorkKind `json:"kind"`
	Ref     string   `json:"ref"`
	Primary bool     `json:"primary,omitempty"`
}

// Context is the correlation context of an event. In the tool profile only Invocation
// may be present, and it is an unlinked record identity.
type Context struct {
	Invocation       string    `json:"invocation,omitempty"`
	ParentInvocation string    `json:"parentInvocation,omitempty"`
	Session          string    `json:"session,omitempty"`
	Agent            string    `json:"agent,omitempty"`
	ParentAgent      string    `json:"parentAgent,omitempty"`
	Attempt          string    `json:"attempt,omitempty"`
	SharedRefresh    string    `json:"sharedRefresh,omitempty"`
	DeskRole         DeskRole  `json:"deskRole,omitempty"`
	Role             Role      `json:"role,omitempty"`
	Work             []WorkRef `json:"work,omitempty"`
}

var deskRoles = map[DeskRole]bool{DeskCoordinator: true, DeskWorker: true, DeskReview: true, DeskVerify: true, DeskIntake: true, DeskUnavailable: true}
var roles = map[Role]bool{RoleCoordinate: true, RoleImplement: true, RoleReview: true, RoleVerify: true, RoleTriage: true, RoleInvestigate: true, RoleOther: true, RoleUnavailable: true}

// ParseDeskRole maps producer input onto the closed vocabulary; anything else is the
// explicit unavailable value, never copied text.
func ParseDeskRole(s string) DeskRole {
	if r := DeskRole(s); deskRoles[r] {
		return r
	}
	return DeskUnavailable
}

// ParseRole maps producer input onto the closed vocabulary; anything else is the
// explicit unavailable value, never copied text.
func ParseRole(s string) Role {
	if r := Role(s); roles[r] {
		return r
	}
	return RoleUnavailable
}

// linked reports whether c carries any workflow-only (linkable) field.
func (c *Context) linked() bool {
	return c.ParentInvocation != "" || c.Session != "" || c.Agent != "" || c.ParentAgent != "" ||
		c.Attempt != "" || c.SharedRefresh != "" || c.DeskRole != "" || c.Role != "" || len(c.Work) > 0
}

func (c *Context) validate(p Profile, k Kind) error {
	needsInvocation := k != KindHealth && k != KindCorrection
	if c == nil {
		if needsInvocation {
			return refuse(RefuseSchema, "ctx.invocation", "invocation identity is required")
		}
		return nil
	}
	if p == ProfileTool && c.linked() {
		return refuse(RefusePrivacy, "ctx", "tool profile cannot carry workflow identity")
	}
	if needsInvocation && c.Invocation == "" {
		return refuse(RefuseSchema, "ctx.invocation", "invocation identity is required")
	}
	for _, f := range []struct{ name, v string }{
		{"ctx.invocation", c.Invocation}, {"ctx.parentInvocation", c.ParentInvocation},
		{"ctx.session", c.Session}, {"ctx.agent", c.Agent}, {"ctx.parentAgent", c.ParentAgent},
		{"ctx.attempt", c.Attempt}, {"ctx.sharedRefresh", c.SharedRefresh},
	} {
		if f.v != "" && !aliasRE.MatchString(f.v) {
			return refuse(RefusePrivacy, f.name, "identity is not a scoped random alias")
		}
	}
	if c.ParentInvocation != "" && c.ParentInvocation == c.Invocation {
		return refuse(RefuseSchema, "ctx.parentInvocation", "an invocation cannot be its own parent")
	}
	if c.ParentAgent != "" && c.ParentAgent == c.Agent {
		return refuse(RefuseSchema, "ctx.parentAgent", "an agent cannot be its own parent")
	}
	if p == ProfileWorkflow {
		if !deskRoles[c.DeskRole] {
			return refuse(RefuseSchema, "ctx.deskRole", "deskRole must be a known role or unavailable")
		}
		if !roles[c.Role] {
			return refuse(RefuseSchema, "ctx.role", "role must be a known role or unavailable")
		}
	}
	if len(c.Work) > MaxWorkRefs {
		return refuse(RefuseBounds, "ctx.work", "too many work references")
	}
	primaries := 0
	seen := map[string]bool{}
	for _, w := range c.Work {
		if w.Kind != WorkBrief && w.Kind != WorkIssue {
			return refuse(RefuseSchema, "ctx.work.kind", "unknown work kind")
		}
		if !aliasRE.MatchString(w.Ref) {
			return refuse(RefusePrivacy, "ctx.work.ref", "work reference is not a scoped random alias")
		}
		if seen[w.Ref] {
			return refuse(RefuseSchema, "ctx.work.ref", "duplicate work reference")
		}
		seen[w.Ref] = true
		if w.Primary {
			primaries++
		}
	}
	if primaries > 1 {
		return refuse(RefuseSchema, "ctx.work.primary", "at most one primary subject")
	}
	if k == KindAttempt && c.Attempt == "" {
		return refuse(RefuseSchema, "ctx.attempt", "attempt events need an attempt identity")
	}
	return nil
}
