package main

// sets.go — `deskroster repos` and `deskroster apps`: print the LIVE sets the
// desk tools use, so nothing downstream has to carry its own copy.
//
// WHY. The desk skills each enumerated the repos they cover, and the lists had
// drifted from `deskkit`'s in BOTH directions (#258): they named repos the tools
// REFUSE to act on (phantom coverage — a session believing it monitors a repo the
// write boundary excludes) while omitting one the tools DO cover (a genuine
// monitoring blind spot). Neither direction is visible from inside a skill file,
// because a prose list has nothing to disagree with.
//
// The fix is not a better-synchronised list. It is having no second list: a skill
// says "run `deskroster repos`", and the answer comes from the same values the
// gates read. A list you PRINT cannot drift from the list you ENFORCE.
//
// THREE-STATE OUTPUT. An unconfigured roster does not print an empty set and exit
// 0 — an empty sweep is not an empty world. It prints what it consulted and exits
// 6 (unverifiable), the same way every other read surface refuses (#489).

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/topology"
)

// cmdRepos prints the repo sets. The scopes are DISTINCT sets, not views of one,
// and printing them under one command is what stops a reader conflating them:
//
//	write     ASSAY_ALLOWED_REPOS — the WRITE-AUTHORISATION boundary. A tool
//	          acting on a repo outside it refuses (exit 5).
//	scan      ASSAY_SCAN_REPOS — the intake READ scope. It covers repos the desk
//	          is the front door for but never writes to, and deliberately holds
//	          out at least one write-boundary repo. Reusing the write set here
//	          would silently conflate the two.
//	topology  topology.yaml — the repos whose visibility/CI/root/risk-trigger
//	          topology this tree STATES, each annotated with THIS CELL's
//	          relationship to it (owned | upstream). It is not an authorisation
//	          set and confers no access.
//
// THE ANNOTATION LANDS ON THE THIRD SET ONLY, and that placement is the point
// (ground-truth/08). `relationship: owned` says this cell is the repo's owner;
// it does NOT say a tool may write to it. Write authorisation is
// ASSAY_ALLOWED_REPOS — the first set — which is already per-cell-shaped because
// each cell's deployment sets its own roster. Printing the two sets side by side
// under one command is what stops the next reader collapsing them: a repo can be
// `owned` and outside the write boundary (the ordinary case for a repo this cell
// owns but this session is not configured for), and a repo can be inside the
// write boundary and `upstream` (a cell writing to a repo another cell owns,
// which is a roster question, not a topology one).
//
// The topology block also prints the file's CELL NAME. There is one topology
// file per cell, so the set is one cell's statement rather than an enterprise
// registry, and a reader who does not know whose statement they are reading
// cannot interpret `owned` at all.
func cmdRepos(args []string) error {
	fs := flag.NewFlagSet("repos", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	scope := fs.String("scope", "all", "which set to print: write | scan | topology | all")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("refused: " + err.Error())
	}
	if fs.NArg() > 0 {
		return deskkit.Refused(fmt.Sprintf("refused: `repos` takes no positional arguments (got %q)", fs.Arg(0)))
	}
	switch *scope {
	case "write", "scan", "topology", "all":
	default:
		return deskkit.Refused(fmt.Sprintf("refused: unknown --scope %q (want write|scan|topology|all)", *scope))
	}

	cfg := deskkit.EffectiveConfig()
	printed := 0

	if *scope == "write" || *scope == "all" {
		fmt.Printf("# write-authorisation set (%s) — a tool acting outside it refuses (exit 5)\n", deskkit.EnvAllowedRepos)
		for _, r := range deskkit.AllowedRepoScope() {
			fmt.Println(r)
			printed++
		}
		if len(deskkit.AllowedRepoScope()) == 0 {
			fmt.Println("# (empty)")
		}
	}

	if *scope == "scan" || *scope == "all" {
		if *scope == "all" {
			fmt.Println()
		}
		fmt.Printf("# intake READ scope (%s) — distinct from the write boundary on purpose\n", deskkit.EnvScanRepos)
		scan := deskkit.ScanRepos()
		for _, r := range scan {
			fmt.Println(r)
			printed++
		}
		if len(scan) == 0 {
			fmt.Println("# (empty)")
		}
	}

	if *scope == "topology" || *scope == "all" {
		if *scope == "all" {
			fmt.Println()
		}
		top := topology.Compiled()
		// The cell name is printed as UNSTATED rather than as an empty string when
		// the source names no cell. An empty field reads as "the assay
		// cell" to a reader skimming, which is the fail-open direction; a file that
		// names no cell also cannot state `owned` on anything (the loader refuses
		// it), so UNSTATED is the accurate report and not a hedge.
		cell := top.Cell
		if cell == "" {
			cell = "UNSTATED"
		}
		fmt.Printf("# stated topology of cell %q (%s) — visibility/CI/root/risk-triggers/relationship;\n"+
			"# NOT an authorisation set: `owned` is a topology claim, the write boundary is %s above\n",
			cell, topology.SourceFile, deskkit.EnvAllowedRepos)
		for _, slug := range top.RepoSlugs() {
			r, _ := top.Repo(slug)
			ci := "ci"
			if !r.CIRequired {
				ci = "no-ci"
			}
			line := fmt.Sprintf("%s\t%s\t%s\t%s", slug, r.Visibility, ci, r.Relationship)
			if r.Root != "" {
				line += "\troot=" + r.Root
			}
			if len(r.RiskPathTriggers) > 0 {
				line += "\trisk-triggers=" + strings.Join(r.RiskPathTriggers, ",")
			}
			fmt.Println(line)
			printed++
		}
	}

	if printed == 0 {
		// COULD-NOT-CHECK, not "there are none". A reader that treats an empty
		// print as an empty world has converted "I know nothing" into "there is
		// nothing" — the #489 failure, on the surface skills are told to consume.
		return deskkit.Unverifiable(fmt.Sprintf(
			"COULD-NOT-CHECK: every requested repo set is EMPTY, so this printed nothing and knows nothing.\n"+
				"  An empty print is not an empty world — a caller that copies this output would be "+
				"recording a blind spot as coverage.\n"+
				"Source consulted: %s (class %s).\n"+
				"CI: set the repository or organization Actions variables %s / %s.\n"+
				"Locally: add them to %s, mode 0600 — write-class tools do NOT read the environment.",
			cfg.Source, cfg.Class, deskkit.EnvAllowedRepos, deskkit.EnvScanRepos, deskkit.ConfigHomePath()), nil)
	}
	return nil
}

// cmdApps prints the desk App ROLES and their binding state.
//
// It prints roles and bindings, never credentials: no token, no private-key path,
// no installation secret. The App SLUG of a bound role is already visible on
// every PR the App touches, so printing it discloses nothing new; an UNBOUND role
// prints as unbound rather than as an empty login, because an empty identity
// MATCHES a deleted GitHub author and has satisfied a flip gate before
// (deskkit.RoleAppLogin's ok-return exists for exactly that).
func cmdApps(args []string) error {
	if len(args) > 0 {
		return deskkit.Refused(fmt.Sprintf("refused: `apps` takes no arguments (got %q)", args[0]))
	}
	roles := topology.Compiled().AppRoles()
	if len(roles) == 0 {
		return deskkit.Unverifiable("COULD-NOT-CHECK: the topology states no App roles", nil)
	}
	sort.Strings(roles)
	fmt.Printf("# desk App roles (%s) — binding comes from %s; ids are NEVER stated here\n",
		topology.SourceFile, deskkit.EnvTrustedBotSlugs)
	unbound := 0
	for _, role := range roles {
		login, ok := deskkit.RoleAppLogin(role)
		if !ok {
			fmt.Printf("%s\tUNBOUND\n", role)
			unbound++
			continue
		}
		fmt.Printf("%s\t%s\n", role, login)
	}
	if unbound == len(roles) {
		// Every role unbound means the roster is not loaded, which is a
		// could-not-check about the WORLD, not a report that the world is empty.
		return deskkit.Unverifiable(fmt.Sprintf(
			"COULD-NOT-CHECK: no App role is bound — %s carries no `role=` prefixes (or the roster is "+
				"unconfigured), so this output states which roles EXIST and nothing about which App answers "+
				"for them. Source consulted: %s.",
			deskkit.EnvTrustedBotSlugs, deskkit.EffectiveConfig().Source), nil)
	}
	return nil
}
