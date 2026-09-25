package main

// tables.go — the two tables this verb carries ACROSS from the bash reference implementation
// (tools/create-fleet-gitlab.sh), not re-derived from it.
//
// fleetRoles is ROLE_TABLE, row for row: role, access-level name, access-level number, scopes.
// A scope widened in translation is a privilege escalation nobody asked for, so the table is
// data, never computed, and a test pins it field for field against an independent copy.
//
// fleetLabels is the ONE label table both forge backends are driven from (labels.go). It is
// the script's LABEL_TABLE and, identically, the GitHub create-labels primitive's nine
// `gh label create` lines: `review-request`, the six `raised-by:<role>` provenance stamps, and
// the `authorization-needed` / `approval-needed` PR-state pair. Colors are stored bare (six hex
// digits); a backend that needs a leading `#` adds it, so the table stays forge-agnostic.

type fleetRole struct {
	Role        string
	AccessName  string
	AccessLevel int
	Scopes      []string
}

var fleetRoles = []fleetRole{
	{"reviewer", "developer", 30, []string{"api"}},
	{"worker", "developer", 30, []string{"api", "write_repository"}},
	{"verifier", "developer", 30, []string{"api", "write_repository"}},
	{"desk", "developer", 30, []string{"api"}},
	{"issue-loop", "reporter", 20, []string{"api"}},
	{"intake-loop", "reporter", 20, []string{"api"}},
	{"board-writer", "developer", 30, []string{"api", "write_repository"}},
}

type fleetLabel struct {
	Name        string
	Color       string // six hex digits, no leading '#'
	Description string
}

var fleetLabels = []fleetLabel{
	{"review-request", "d4c5f9", "dispatch token: a review session picks this up, runs the skill, posts the verdict"},
	{"raised-by:desk", "BFDADC", "filed by the process desk (the-desk)"},
	{"raised-by:worker", "BFDADC", "filed by a worker (worker-desk)"},
	{"raised-by:reviewer", "BFDADC", "filed by the reviewer desk (pr-review-desk)"},
	{"raised-by:verifier", "BFDADC", "filed by the verify desk (verify-desk)"},
	{"raised-by:issue-loop", "BFDADC", "filed by the intake/issue loop (intake-desk)"},
	{"raised-by:intake-loop", "BFDADC", "filed by the intake loop (roster-bound; no skill stamps it yet)"},
	{"authorization-needed", "FBCA04", "review lane has not approved at head — waiting on a reviewer verdict / open findings"},
	{"approval-needed", "5319E7", "review lane fully approved; flipped ready — waiting on the human's merge approval"},
}

// Project-settings constants, carried from the script with its reasons.
const (
	// mergeAccessLevel: Maintainers. A hand repair that used 30 (Developers) let every
	// Developer service account merge its own MR.
	mergeAccessLevel = 40
	// protectedTagGlob / protectedTagCreateLevel: every tag, creatable only by Maintainers,
	// via the SCALAR Free-tier field (never the Premium allowed_to_create array).
	protectedTagGlob        = "*"
	protectedTagCreateLevel = 40
	// unprotectAccessLevel: Owners, for the Premium allowed_to_unprotect array.
	unprotectAccessLevel = 50
)

// tokenFileName is the custody file a role's PAT is written to: the exact name
// `desktoken --forge gitlab <role>` resolves, so no link/copy step is needed afterwards.
func tokenFileName(role string) string { return "gitlab-" + role + ".token" }

// serviceAccountUsername / patName mirror the script's naming.
func serviceAccountUsername(prefix, role string) string { return prefix + "-" + role + "-bot" }
func patName(role string) string                        { return "assay-" + role + "-fleet" }
