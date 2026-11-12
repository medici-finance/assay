package main

// tables.go — the two tables this verb carries ACROSS from the bash reference implementation
// (tools/create-fleet-gitlab.sh), not re-derived from it.
//
// fleetRoles is ROLE_TABLE, row for row: role, access-level name, access-level number, scopes.
// A scope widened in translation is a privilege escalation nobody asked for, so the table is
// data, never computed, and a test pins it field for field against an independent copy.
//
// The label set is NOT carried here: it is internal/forgelabels' one canonical table, whose
// role rows (`raised-by:<role>`, `to:<role>`) are derived from a role list — the roster's
// bound roles where a caller has one, forgelabels.DocumentedRoles() otherwise.

import "github.com/medici-finance/assay/tools/desk/internal/forgelabels"

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

// fleetLabel is the shared table's row type.
type fleetLabel = forgelabels.Label

// fleetLabels is the documented-role label set: what `provision --project` creates (a fleet
// provisioned before its roster exists) and what `labels` creates without --roles.
var fleetLabels = forgelabels.Canonical(forgelabels.DocumentedRoles())

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

// The board-push credential the scaffolded statusgen-regen job reads (pushtoken.go). A
// DEDICATED project access token, never a role-fleet PAT: write_repository only (no `api`),
// at Maintainer — the credential class docs/adopting-assay-gitlab.md §2c names.
const (
	pushTokenVariable    = "STATUSGEN_PUSH_TOKEN"
	pushTokenName        = "assay-statusgen-push"
	pushTokenScope       = "write_repository"
	pushTokenAccessLevel = 40
)

// tokenFileName is the custody file a role's PAT is written to: the exact name
// `desktoken --forge gitlab <role>` resolves, so no link/copy step is needed afterwards.
func tokenFileName(role string) string { return "gitlab-" + role + ".token" }

// serviceAccountUsername / patName mirror the script's naming.
func serviceAccountUsername(prefix, role string) string { return prefix + "-" + role + "-bot" }
func patName(role string) string                        { return "assay-" + role + "-fleet" }
