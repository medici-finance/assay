package main

// tables.go — the data the provisioner carries across from the bash implementation it
// replaces (tools/create-fleet-gitlab.sh), row for row, never re-derived.
//
// fleetRoles is the shared ROLE_TABLE of tools/fleet-gitlab-roles.sh (role, access-level
// name, access-level number, scopes). The renewal script still sources that file, so the two
// must not drift: tables_parity_test.go reads the shell table and compares it field for field.
// A scope widened in translation is a privilege escalation nobody asked for.
//
// fleetLabels is internal/forgelabels' canonical set over its documented roles — the one
// table deskfleet is driven from too, so the two provisioners cannot drift. It extends the
// script's LABEL_TABLE with the `to:<role>` addressee rows `deskfile --to` stamps. Colors are
// stored bare (six hex digits) and rendered with GitLab's leading `#` where they are sent.

import "github.com/medici-finance/assay/tools/desk/internal/forgelabels"

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

type fleetLabel = forgelabels.Label

var fleetLabels = forgelabels.Canonical(forgelabels.DocumentedRoles())

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

// The board-push credential the scaffolded statusgen-regen job reads (pushtoken.go). A
// DEDICATED project access token, never a role-fleet PAT: write_repository only (no `api`),
// at Maintainer — the credential class docs/adopting-assay-gitlab.md §2c names.
const (
	pushTokenVariable    = "STATUSGEN_PUSH_TOKEN"
	pushTokenName        = "assay-statusgen-push"
	pushTokenScope       = "write_repository"
	pushTokenAccessLevel = "40"
)

// fleetUsername / fleetPATName are fleet_username / fleet_pat_name of the shared shell file.
func fleetUsername(prefix, role string) string { return prefix + "-" + role + "-bot" }
func fleetPATName(role string) string          { return "assay-" + role + "-fleet" }
