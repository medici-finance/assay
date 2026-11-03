// Package identity is deskd's PER-USER authentication and capability ladder.
// It replaces the single compile-time "operator" persona with a real
// per-viewer GitHub identity and a three-tier capability derived from that
// user's GitHub repo permission.
//
// # The security model in one paragraph
//
// A viewer authenticates via a GitHub OAuth authorization-code flow ([Auth]).
// The login is resolved SERVER-SIDE by calling GET /user with the token the
// flow returned — it is never a request field, header, or config entry. The
// token lives only in the in-memory [SessionStore], keyed by an HttpOnly
// cookie; a deskd restart logs everyone out. A viewer's [Capability] on any
// given item is derived from that item's repo permission, queried with DESKD'S
// OWN read identity (the per-org read-token router) — never with the
// viewer's token, so a viewer's token scopes cannot influence their tier. The
// derivation fails CLOSED to [Anonymous] on any error. The [Middleware] wraps
// every data and write route and enforces the required tier server-side; the
// web UI renders affordances by tier but is explicitly NOT a control.
package identity

// Capability is the tier a viewer holds on a specific item's repo. The ladder
// is ordered: Answerer implies Commenter implies Anonymous. The zero value is
// [Anonymous], so a capability that was never derived fails closed.
type Capability int

const (
	// Anonymous is an unauthenticated viewer, or an authenticated viewer with
	// no read permission on the item's repo, or ANY viewer whose tier could not
	// be derived (fail-closed). On a private-repo cell an anonymous viewer sees
	// nothing but the login prompt.
	Anonymous Capability = iota
	// Commenter is an authenticated viewer whose GitHub permission on the item's
	// repo is at least read (pull). A commenter may read the item and add a
	// comment AS THEMSELVES; a commenter may NEVER answer or close.
	Commenter
	// Answerer is an authenticated viewer whose login equals the cell's
	// answer_as login (cells.yaml, default "operator"). An answerer is a commenter
	// first, plus the existing answer/ack write path. "operator" is thereby a
	// configured authorized-answerer value, not THE identity.
	Answerer
)

// String renders the tier as the lowercase token the /session response and the
// web chrome use. A value outside the ladder renders as "anonymous" — the
// fail-closed reading.
func (c Capability) String() string {
	switch c {
	case Answerer:
		return "answerer"
	case Commenter:
		return "commenter"
	default:
		return "anonymous"
	}
}

// AtLeastCommenter reports whether the tier may read data and comment.
func (c Capability) AtLeastCommenter() bool { return c >= Commenter }

// IsAnswerer reports whether the tier may reach the answer/ack write path.
func (c Capability) IsAnswerer() bool { return c >= Answerer }

// baseTierForPermission maps a GitHub collaborator-permission value to the
// pre-answerer tier. GitHub's permission field is one of none/read/triage/
// write/maintain/admin (the collaborators/{u}/permission endpoint reports the
// coarse permission plus a role_name; we key off the coarse permission). Any
// value at read or above is a commenter; none — and any unrecognised value,
// fail-closed — is anonymous. The answerer step (login == answer_as) is applied
// ON TOP of this by the Deriver; it is never encoded here, because a repo
// permission alone can never promote to answerer.
func baseTierForPermission(permission string) Capability {
	switch permission {
	case "read", "triage", "write", "maintain", "admin":
		return Commenter
	default:
		// "none", "", or anything unrecognised: fail closed to anonymous.
		return Anonymous
	}
}
