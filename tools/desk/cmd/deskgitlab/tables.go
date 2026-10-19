package main

// tables.go — the data the provisioner carries across from the bash implementation it
// replaces (tools/create-fleet-gitlab.sh), row for row, never re-derived.
//
// fleetRoles is the shared ROLE_TABLE of tools/fleet-gitlab-roles.sh (role, access-level
// name, access-level number, scopes). The renewal script still sources that file, so the two
// must not drift: tables_parity_test.go reads the shell table and compares it field for field.
// A scope widened in translation is a privilege escalation nobody asked for.
//
// fleetLabels is the script's LABEL_TABLE: `review-request`, the six `raised-by:<role>`
// provenance stamps, and the `authorization-needed` / `approval-needed` pair. Colors are
// stored bare (six hex digits) and rendered with GitLab's leading `#` where they are sent.

type fleetRole struct {
	Role        string
	AccessName  string
	AccessLevel string // as the shell table spells it; compared and sent as that JSON number
	Scopes      string // comma-separated, as the shell table spells it
}

var fleetRoles = []fleetRole{
	{"reviewer", "developer", "30", "api"},
	{"worker", "developer", "30", "api,write_repository"},
	{"verifier", "developer", "30", "api,write_repository"},
	{"desk", "developer", "30", "api"},
	{"issue-loop", "reporter", "20", "api"},
	{"intake-loop", "reporter", "20", "api"},
	{"board-writer", "developer", "30", "api,write_repository"},
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

const (
	// defaultPATDays is FLEET_PAT_DAYS (spec.md §5's "7 days RECOMMENDED" expiry backstop).
	defaultPATDays = 7
	// mergeAccessLevel: Maintainers. A hand repair that used 30 (Developers) let every
	// Developer service account merge its own MR.
	mergeAccessLevel = "40"
	// protectedTagGlob / protectedTagCreateLevel: every tag, creatable only by Maintainers,
	// through the SCALAR Free-tier field (never the Premium allowed_to_create array).
	protectedTagGlob        = "*"
	protectedTagCreateLevel = "40"
	// customRoleBaseLevel: Reporter, which cannot push to any branch — the base of the
	// --tier ultimate custom reviewer role.
	customRoleBaseLevel = "20"
	// defaultIconBase is where the public role icons are fetched from when avatars are left
	// at their default.
	defaultIconBase = "https://assay.guide/assets"
	// defaultGitLabURL is the instance used when --gitlab-url is not given.
	defaultGitLabURL = "https://gitlab.com"
	// defaultStatusCheckName is the external status check the verdict lane posts to.
	defaultStatusCheckName = "assay-verdict"
)

// fleetUsername / fleetPATName are fleet_username / fleet_pat_name of the shared shell file.
func fleetUsername(prefix, role string) string { return prefix + "-" + role + "-bot" }
func fleetPATName(role string) string          { return "assay-" + role + "-fleet" }
