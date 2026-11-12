// Package forgelabels is the ONE canonical forge label set the desk tools stamp, and the
// operator-side ensure / check over it.
//
// THE SET. `review-request`; one `raised-by:<role>` provenance stamp and one `to:<role>`
// addressee stamp per desk role; and the `authorization-needed` / `approval-needed` PR-state
// pair. Before this package the set lived as two hand-copied tables (deskfleet's and
// deskgitlab's), held equal by tests that read each other's source, and both lacked the
// `to:<role>` rows `deskfile --to` stamps — so even an operator who ran `deskfleet labels` by
// hand never got the addressee labels, and every `--to` filing degraded to unaddressed.
//
// THE ROLE ROWS ARE DERIVED, not listed. Canonical takes the role list and builds both
// prefixed rows from it with deskkit's own prefixes, so a `to:` row exists for exactly the
// roles a `raised-by:` row does. The list callers pass is the roster's bound roles
// (deskkit.Config.BoundRoles) — the same projection deskkit.AddressedToLabel and
// deskkit.RaisedByLabel resolve `--to` / `--raised-by` against — so the labels an install
// creates and the roles `deskfile --to` accepts cannot drift.
//
// DocumentedRoles is the one exception, and it is a fallback, never a second source: the six
// roles the shipped docs name, used only when a caller has no roster to derive from (a hand
// run of `deskfleet labels` with no --roles, `deskgitlab create` provisioning a fleet before
// its roster exists). Every caller that falls back says so. TestDocumentedRolesMatchFleetRoles
// (cmd/deskfleet) diffs it against the provisioner's own role table.
//
// LABEL CREATION IS AN OPERATOR ACT. Nothing here mints a credential or reads a desk role's
// App token: the ensure runs under a credential its caller was handed by the operator, and a
// credential that cannot write labels makes the step UNFINISHED with the exact command to
// finish it, never a silent success.
package forgelabels

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Label is one row of the set. Color is six hex digits with no leading '#'; a backend that
// needs the '#' (GitLab) adds it, so the table stays forge-agnostic.
type Label struct {
	Name        string
	Color       string
	Description string
}

// The fixed rows' colors. A role row's color is per prefix, not per role.
const (
	reviewRequestColor = "d4c5f9"
	raisedByColor      = "BFDADC"
	addressedToColor   = "C2E0C6"
	authNeededColor    = "FBCA04"
	approvalColor      = "5319E7"
)

// DocumentedRoles returns the six desk roles the shipped docs name, in the order the label
// set has always listed them. It is the FALLBACK role list for a caller with no roster to
// derive from; see the package comment. A fresh slice on every call, so no caller can edit
// the fallback for another.
func DocumentedRoles() []string {
	return []string{"desk", "worker", "reviewer", "verifier", "issue-loop", "intake-loop"}
}

// roleNoun names a documented role for a description. A role the roster binds that the
// docs do not name gets a generic description, never a refusal: the
// roster is the authority on which roles exist.
var roleNoun = map[string]string{
	"desk":        "the process desk (the-desk)",
	"worker":      "a worker (worker-desk)",
	"reviewer":    "the reviewer desk (pr-review-desk)",
	"verifier":    "the verify desk (verify-desk)",
	"issue-loop":  "the intake/issue loop (intake-desk)",
	"intake-loop": "the intake loop",
}

// raisedByDescription keeps the descriptions the raised-by rows have always carried.
func raisedByDescription(role string) string {
	switch role {
	case "intake-loop":
		return "filed by the intake loop (roster-bound; no skill stamps it yet)"
	}
	if n, ok := roleNoun[role]; ok {
		return "filed by " + n
	}
	return "filed by the " + role + " role (roster-bound)"
}

func addressedToDescription(role string) string {
	if n, ok := roleNoun[role]; ok {
		return "addressed to " + n + ": its inbox sweep picks this up"
	}
	return "addressed to the " + role + " role (roster-bound): its inbox sweep picks this up"
}

// Canonical is the full label set for roles: review-request, then one raised-by row per
// role, then one to: row per role (both in the order given), then the PR-state pair. Roles
// are normalized as deskkit's resolver normalizes them (trimmed, lowercased); a blank or
// repeated role is dropped, so the set never carries a `raised-by:` with an empty role or
// the same name twice.
func Canonical(roles []string) []Label {
	rs := NormalizeRoles(roles)
	out := make([]Label, 0, 3+2*len(rs))
	out = append(out, Label{"review-request", reviewRequestColor,
		"dispatch token: a review session picks this up, runs the skill, posts the verdict"})
	for _, r := range rs {
		out = append(out, Label{deskkit.RaisedByPrefix + r, raisedByColor, raisedByDescription(r)})
	}
	for _, r := range rs {
		out = append(out, Label{deskkit.AddressedToPrefix + r, addressedToColor, addressedToDescription(r)})
	}
	out = append(out,
		Label{"authorization-needed", authNeededColor,
			"review lane has not approved at head — waiting on a reviewer verdict / open findings"},
		Label{"approval-needed", approvalColor,
			"review lane fully approved; flipped ready — waiting on the human's merge approval"})
	return out
}

// NormalizeRoles trims and lowercases each role and drops blanks and repeats, keeping the
// first occurrence's position.
func NormalizeRoles(roles []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range roles {
		r = strings.ToLower(strings.TrimSpace(r))
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

// ParseRoles reads a comma-separated role list (the --roles flag's value). It refuses a role
// that could not be a label suffix — anything outside [a-z0-9-] — rather than creating a
// label no desk could ever be addressed by.
func ParseRoles(raw string) ([]string, error) {
	var roles []string
	for _, f := range strings.Split(raw, ",") {
		r := strings.ToLower(strings.TrimSpace(f))
		if r == "" {
			continue
		}
		for _, ch := range r {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return nil, &RoleError{Role: r}
			}
		}
		roles = append(roles, r)
	}
	if len(roles) == 0 {
		return nil, &RoleError{}
	}
	return NormalizeRoles(roles), nil
}

// RoleError is a --roles value ParseRoles refused.
type RoleError struct{ Role string }

func (e *RoleError) Error() string {
	if e.Role == "" {
		return "--roles names no role (want a comma-separated list of the roster's role= names, e.g. desk,worker,reviewer)"
	}
	return "--roles: " + e.Role + " is not a role name (letters, digits and '-' only — the roster's role= names)"
}

// Names returns the label names of ls, in order.
func Names(ls []Label) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Name
	}
	return out
}
