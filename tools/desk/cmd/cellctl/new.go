package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// cmdNew scaffolds a cell. Every template below is a BYTE CONTRACT with the shell oracle: the
// parity harness diffs the whole tree each implementation writes — paths, mode bits and file
// contents — so a stray space in a comment line here is a divergence, not a nit.
func cmdNew(args []string) {
	// --help before anything else, so `cellctl new --help` prints the kind- and forge-aware
	// usage rather than dying on a missing cell name or flag.
	for _, a := range args {
		if a == "-h" || a == "--help" {
			usage(0)
		}
	}
	e := newEnvFromProcess()

	// DESK_APP_ID is the default because `cellctl deskd` reads the CELL home's apps.env, and
	// cell homes name their Apps by generic role.
	kind, forge, idvar, port := "k8s", "github", "DESK_APP_ID", "8787"
	cell, repo, yaml, orgs, pem := "", "", "", "", ""
	group, gitlabAPIBase, tokenStore := "", "", ""
	roots, launcher, repoSlug := "", "", ""
	roles, rolesSet := rolesDefault, false

	for i := 0; i < len(args); i++ {
		next := func(msg string) string { return needFlagValue(args, &i, msg) }
		switch a := args[i]; a {
		case "--kind":
			kind = nextOrEmpty(args, &i)
		case "--launcher":
			launcher = next("--launcher needs an absolute executable")
		case "--forge":
			forge = nextOrEmpty(args, &i)
		case "--repo":
			repo = nextOrEmpty(args, &i)
		case "--cells-yaml":
			yaml = nextOrEmpty(args, &i)
		case "--orgs":
			orgs = nextOrEmpty(args, &i)
		case "--repo-slug":
			repoSlug = next("--repo-slug needs a value (<owner>/<repo>)")
		case "--deskd-app-pem":
			pem = nextOrEmpty(args, &i)
		case "--deskd-app-id-var":
			idvar = nextOrEmpty(args, &i)
		case "--port":
			port = nextOrEmpty(args, &i)
		case "--group":
			group = nextOrEmpty(args, &i)
		case "--gitlab-api-base":
			gitlabAPIBase = nextOrEmpty(args, &i)
		case "--gitlab-token-store":
			tokenStore = nextOrEmpty(args, &i)
		case "--roots":
			roots = nextOrEmpty(args, &i)
		case "--roles":
			roles = nextOrEmpty(args, &i)
			rolesSet = true
		default:
			if strings.HasPrefix(a, "--") {
				die("new: unknown flag %s", a)
			}
			// The cell name is a POSITIONAL that may appear anywhere among the flags. It is
			// captured as the ONE bare token so that a missing CUSTODY input is diagnosed before
			// a missing cell name — the forge-custody validation must fire whether or not a cell
			// name was given.
			if cell != "" {
				die("new: unexpected extra argument '%s' (the cell name is '%s')", a, cell)
			}
			cell = a
		}
	}

	root := cellsRoot(e)
	switch kind {
	case "container":
		if !rolesSet {
			roles = "the-desk"
		}
		if yaml+orgs+pem+group+tokenStore != "" {
			die("container new does not accept host credential or deskd configuration")
		}
		newContainer(e, root, cell, repo, launcher, roles, roots)
		return
	case "scrubbed":
		if yaml+orgs+pem+group+tokenStore+launcher != "" {
			die("scrubbed new does not accept host credential, deskd or container-launcher configuration")
		}
		newScrubbed(e, root, cell, repo, repoSlug, roots, roles)
		return
	}
	if launcher != "" {
		die("--launcher is only valid for --kind container")
	}
	switch kind {
	case "k8s":
	case "house":
		newHouse(e, root, cell, repo, roots, roles, port)
		return
	default:
		die("new: --kind must be k8s, house, container or scrubbed, got '%s'", kind)
	}
	if forge != "github" && forge != "gitlab" {
		die("new: --forge must be github or gitlab, got '%s'", forge)
	}
	// Common requirements on BOTH forges.
	if repo == "" || yaml == "" {
		die("new: --repo and --cells-yaml are required (both forges)")
	}
	// Forge-specific custody inputs, asserted BEFORE the cell-name / yaml-is-a-file tests, so a
	// missing custody input names ITSELF — and so the GitHub App requirement never fires on a
	// GitLab cell, and the GitLab custody requirement never fires on a GitHub one.
	if forge == "github" {
		if orgs == "" || pem == "" {
			die("new: on the github path --orgs and --deskd-app-pem are required (the App mints per-org installation tokens)")
		}
	} else if group == "" {
		die("new: on the gitlab path --group is required (the GitLab group this cell reads); the role token store (gitlab-<role>.token) is a custody hand step, NOT an App PEM")
	}
	if cell == "" {
		die("new: a cell name is required (cellctl new <cell> …)")
	}
	if !isRegular(yaml) {
		die("new: --cells-yaml is not a file: %s", yaml)
	}
	if roots != "" && !rootsValid(roots) {
		die("new: --roots is malformed")
	}
	d := filepath.Join(root, cell)
	if exists(d) {
		die("%s already exists", d)
	}
	mustMkdirAll(filepath.Join(d, "home", ".config", "assay"), filepath.Join(d, "bin"),
		filepath.Join(d, "index"), filepath.Join(d, "worktrees"))
	copyFile(yaml, filepath.Join(d, "cells-"+cell+".yaml"))
	linkIfPresent(filepath.Join(e.Get("HOME"), ".gitconfig"), filepath.Join(d, "home", ".gitconfig"))
	chmod700(filepath.Join(d, "home"), filepath.Join(d, "home", ".config"), filepath.Join(d, "home", ".config", "assay"))
	githubHost := e.GetOr("GITHUB_HOST", "github.com")
	rootsLine := "# CELL_ROOTS=<owner>/<repo>=<abs path>,...   (exported as DESK_ROOTS at boot; unset = placeholder topology)"
	if roots != "" {
		rootsLine = "CELL_ROOTS=" + roots
	}
	today := time.Now().UTC().Format("2006-01-02")

	if forge == "github" {
		// The gh CLI config is a GitHub custody artifact; linked only on a github cell.
		linkIfPresent(filepath.Join(e.Get("HOME"), ghConfigRelPath), filepath.Join(d, "home", ghConfigRelPath))
		// The endpoint is DERIVED from the host, never spelled as a literal.
		forgeAPIBase := "https://api." + githubHost
		writeFile(filepath.Join(d, "cell.env"), fmt.Sprintf(`# cellctl cell.env — %s (k8s, github, scaffolded %s)
CELL=%s
CELL_KIND=k8s
CELL_FORGE=github
CELL_REPO=%s
%s
CELLS_CONFIG=%s/cells-%s.yaml
FORGE_API_BASE=%s
DESKD_ADDR=127.0.0.1:%s
DESKD_INDEX=%s/index/index.db
DESKD_APP_PEM=%s
DESKD_APP_ID_VAR=%s
ORGS=%s
ROLES="%s"
`+cockpitBlock+pinnedBlock, cell, today, cell, repo, rootsLine, d, cell, forgeAPIBase, port, d, pem, idvar, orgs, roles))
		writeFile(filepath.Join(d, "README.md"), fmt.Sprintf(githubReadme, cell, cell, idvar, realConfigHome(e), idvar, cell, cell, cell))
	} else {
		forgeAPIBase := gitlabAPIBase
		if forgeAPIBase == "" {
			forgeAPIBase = "https://gitlab.com/api/v4"
		}
		store := tokenStore
		if store == "" {
			store = filepath.Join(d, "home", ".config", "assay")
		}
		writeFile(filepath.Join(d, "cell.env"), fmt.Sprintf(`# cellctl cell.env — %s (k8s, gitlab, scaffolded %s)
CELL=%s
CELL_KIND=k8s
CELL_FORGE=gitlab
CELL_REPO=%s
%s
CELLS_CONFIG=%s/cells-%s.yaml
FORGE_API_BASE=%s
GITLAB_API_BASE=%s
GITLAB_GROUP=%s
GITLAB_TOKEN_STORE=%s
DESKD_GITLAB_TOKEN_FILE=%s/gitlab-deskd.token
DESKD_ADDR=127.0.0.1:%s
DESKD_INDEX=%s/index/index.db
ROLES="%s"
`+cockpitBlock+pinnedBlock, cell, today, cell, repo, rootsLine, d, cell, forgeAPIBase, forgeAPIBase, group, store, store, port, d, roles))
		writeFile(filepath.Join(d, "README.md"), fmt.Sprintf(gitlabReadme, cell, cell, store, cell, forgeAPIBase, group, cell, cell))
	}
	fmt.Printf("[new] scaffolded %s (%s cell) — see %s/README.md for the hand steps\n", d, forge, d)
}

// cockpitBlock and pinnedBlock are the trailing comment blocks EVERY k8s cell.env carries, in
// both forges, verbatim. They are constants rather than two copies for the same reason the shell
// keeps one heredoc shape: the two forges' files differ only above this line.
const cockpitBlock = "# The cockpit `cellctl up` opens the role windows in: auto (herdr if on PATH, else orca if on\n" +
	"# PATH and its desktop app answers, else tmux) | tmux | herdr | orca. Only the SURFACE changes.\n" +
	"CELL_COCKPIT=auto\n"

const pinnedBlock = "# Pinned models (values `claude --model` accepts); the CLI default is never used.\n" +
	"DESK_MODEL_DEFAULT=sonnet\n" +
	"DESK_MODEL_the_desk=fable\n" +
	"# The harness a role window boots on: claude (default) or codex. `cellctl desk`/`up --harness`\n" +
	"# overrides it for one run without touching this line; see docs/cellctl.md's Harnesses section.\n" +
	"CELL_HARNESS=claude\n" +
	"# Codex gets its OWN model-pin namespace (CODEX_MODEL_<role> / CODEX_MODEL_default) — the two\n" +
	"# DESK_MODEL_* lines above are Claude-only and are never read on --harness codex. Absent here,\n" +
	"# a role falls back to the tier map (docs/cellctl.md's Pinned models section); uncomment to pin:\n" +
	"# CODEX_MODEL_default=gpt-5.6-terra\n" +
	"# CODEX_MODEL_the_desk=gpt-5.6-terra\n"

const githubReadme = "# %s cell (github) — scaffolded by cellctl\n" + `
Four steps remain. Each is a custody act — it moves key material or states who this cell trusts —
so ` + "`cellctl new`" + ` does not do them for you. Then run ` + "`cellctl check %s`" + `.

1. ` + "`home/.config/assay/roster.env`" + ` — THIS cell's roster: ` + "`ASSAY_TRUSTED_LOGINS`" + `,
   ` + "`ASSAY_BLESS_LOGIN`" + `, ` + "`ASSAY_TRUSTED_BOT_SLUGS`" + ` with forge-qualified ` + "`role=github:<slug>:<id>`" + `
   bindings to THIS cell's Apps (the forge-qualified grammar), ` + "`ASSAY_REPO_FORGES`" + ` binding
   the cell's repos to ` + "`github`" + `, and ` + "`ASSAY_ALLOWED_REPOS`" + ` / ` + "`ASSAY_SCAN_REPOS`" + ` naming the
   cell's repos ONLY. Start from another cell's file as a template and replace every App — a
   copied binding points this cell's writes at another cell's identity.
2. ` + "`home/.config/assay/apps.env`" + ` — the cell's App ids under the generic role names
   (` + "`%s`" + `, ` + "`REVIEWER_APP_ID`" + `, …), plus one ` + "`<role>-app.pem`" + ` SYMLINK per role into your real
   config home (` + "`%s`" + `). Symlink, never copy: one custody location for the private
   keys. If this cell names its deskd App id something other than ` + "`%s`" + `, set
   ` + "`DESKD_APP_ID_VAR`" + ` in ` + "`cell.env`" + ` to the name used here.
3. ` + "`bin/deskd`" + `, ` + "`bin/deskcli`" + ` — from the desk console (build its ` + "`cmd/deskd`" + ` and
   ` + "`cmd/deskcli`" + `), or from the release tarball if your channel ships them.
4. ` + "`cells-%s.yaml`" + ` — the cell's slice of ` + "`cells.yaml`" + ` and nothing else; it must validate on
   its own, and no other cell's repos may appear in it.

Then: ` + "`CELL_ATTENDED=1 cellctl deskd %s`" + ` once, and ` + "`cellctl up %s`" + ` for the cockpit.
`

const gitlabReadme = "# %s cell (gitlab) — scaffolded by cellctl\n" + `
Four steps remain. Each is a custody act — it moves credential material or states who this cell
trusts — so ` + "`cellctl new`" + ` does not do them for you, and it MINTS NOTHING: GitLab role tokens
rotate by hand. Then run ` + "`cellctl check %s`" + `.

1. ` + "`home/.config/assay/roster.env`" + ` — THIS cell's roster: ` + "`ASSAY_TRUSTED_LOGINS`" + `,
   ` + "`ASSAY_BLESS_LOGIN`" + `, ` + "`ASSAY_TRUSTED_BOT_SLUGS`" + ` with forge-qualified ` + "`role=gitlab:<slug>:<id>`" + `
   bindings to THIS cell's bot accounts (the forge-qualified grammar — a bare slug is refused,
   and a ` + "`gitlab:`" + ` entry against a github repo is refused), ` + "`ASSAY_REPO_FORGES`" + ` binding the
   cell's repos to ` + "`gitlab`" + `, and ` + "`ASSAY_ALLOWED_REPOS`" + ` / ` + "`ASSAY_SCAN_REPOS`" + ` naming the cell's
   repos ONLY. A cell stood up for GitLab MUST write forge-qualified entries or the verbs refuse.
2. The role token store — ` + "`%s`" + ` — holds one ` + "`gitlab-<role>.token`" + ` file per role (0600),
   provisioned BY HAND: ` + "`gitlab-deskd.token`" + ` for the read daemon, plus one per write role
   (` + "`gitlab-worker.token`" + `, ` + "`gitlab-reviewer.token`" + `, …) per the role-token custody binding.
   These are GitLab group/project access tokens you mint in GitLab and place here yourself;
   cellctl never mints or rotates them. Lock each to 0600.
3. ` + "`bin/deskd`" + `, ` + "`bin/deskcli`" + ` — from the desk console (build its ` + "`cmd/deskd`" + ` and
   ` + "`cmd/deskcli`" + `), or from the release tarball if your channel ships them.
4. ` + "`cells-%s.yaml`" + ` — the cell's slice of ` + "`cells.yaml`" + ` and nothing else; it must validate on
   its own, and no other cell's repos may appear in it.

The GitLab endpoint is ` + "`%s`" + ` (` + "`cell.env`" + `: ` + "`FORGE_API_BASE`" + ` / ` + "`GITLAB_API_BASE`" + `);
the group this cell reads is ` + "`%s`" + `. Then: ` + "`CELL_ATTENDED=1 cellctl deskd %s`" + ` once, and
` + "`cellctl up %s`" + ` for the cockpit.
`

// newContainer is REGISTRATION ONLY: no daemon, host checkout, credential directory or symlink.
func newContainer(e *Env, root, cell, repo, launcher, roles, roots string) {
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`).MatchString(cell) {
		die("new: invalid container cell name")
	}
	if repo == "" {
		die("container new needs --repo <repo-id>")
	}
	if !strings.HasPrefix(launcher, "/") || !isExecFile(launcher) {
		die("container new needs --launcher <absolute-executable>")
	}
	if strings.TrimSpace(roles) == "" {
		die("container new needs at least one role")
	}
	for _, role := range strings.Fields(roles) {
		if !valueIn(role, knownRoles) {
			die("unknown container role '%s'", role)
		}
	}
	d := filepath.Join(root, cell)
	if exists(d) {
		die("%s already exists (cellctl new never overwrites a cell)", d)
	}
	mustMkdirAll(root)
	if err := os.Mkdir(d, 0o700); err != nil {
		die("new: cannot create %s: %v", d, err)
	}
	_ = os.Chmod(d, 0o700)
	body := "# Container cell registration; the launcher owns runtime custody.\n" +
		fmt.Sprintf("CELL=%s\nCELL_KIND=container\nCELL_REPO=%s\n", bashQuote(cell), bashQuote(repo)) +
		fmt.Sprintf("CELL_CONTAINER_LAUNCHER=%s\nROLES=%s\nCELL_ROOTS=%s\n", bashQuote(launcher), bashQuote(roles), bashQuote(roots)) +
		"CELL_HARNESS=claude\nDESK_MODEL_DEFAULT=sonnet\nDESK_MODEL_the_desk=fable\nDESKD=0\n"
	writeFile(filepath.Join(d, "cell.env"), body)
	_ = os.Chmod(filepath.Join(d, "cell.env"), 0o600)
	fmt.Printf("[new] registered %s (container) — cellctl check %s, then cellctl up %s\n", d, cell, cell)
}

// newHouse scaffolds the operator's own desks on the operator's own laptop. There is no custody
// act to leave for a hand step — the cell home's .config/assay IS the operator's config home,
// reached by ONE directory symlink, and nothing is copied. What the scaffold fixes is what a
// hand boot gets wrong: the stream-root map, the per-role worktree, and the pinned model.
func newHouse(e *Env, root, cell, repo, roots, roles, port string) {
	if repo == "" || roots == "" {
		die("new: --kind house needs --repo <checkout> and --roots '<owner>/<repo>=<abs path>,...'")
	}
	if cell == "" {
		die("new: a cell name is required (cellctl new <cell> --kind house …)")
	}
	if !rootsValid(roots) {
		die("new: --roots is malformed")
	}
	if !isGitCheckout(repo) {
		die("new: --repo is not a git checkout: %s", repo)
	}
	for _, en := range rootEntries(roots) {
		if !isDir(en[1]) {
			die("new: --roots path does not exist: %s (%s)", en[1], en[0])
		}
	}
	realCfg := realConfigHome(e)
	if !isDir(realCfg) {
		die("new: operator config home not found: %s (ASSAY_CONFIG_HOME to point elsewhere)", realCfg)
	}
	d := filepath.Join(root, cell)
	if exists(d) {
		die("%s already exists (cellctl new never overwrites a cell — remove it yourself, or pick another name)", d)
	}
	mustMkdirAll(filepath.Join(d, "home", ".config"), filepath.Join(d, "worktrees"), filepath.Join(d, "shim"))
	if err := os.Symlink(realCfg, filepath.Join(d, "home", ".config", "assay")); err != nil {
		die("new: cannot link the config home: %v", err)
	}
	linkIfPresent(filepath.Join(e.Get("HOME"), ghConfigRelPath), filepath.Join(d, "home", ghConfigRelPath))
	linkIfPresent(filepath.Join(e.Get("HOME"), ".gitconfig"), filepath.Join(d, "home", ".gitconfig"))
	chmod700(filepath.Join(d, "home"), filepath.Join(d, "home", ".config"))
	githubHost := e.GetOr("GITHUB_HOST", "github.com")
	today := time.Now().UTC().Format("2006-01-02")
	writeFile(filepath.Join(d, "cell.env"), fmt.Sprintf(`# cellctl cell.env — %s (house, scaffolded %s)
CELL=%s
CELL_KIND=house
CELL_FORGE=github
CELL_REPO=%s
# The stream-root map, exported to every role window as DESK_ROOTS.
CELL_ROOTS=%s
FORGE_API_BASE=https://api.%s
ROLES="%s"
`+cockpitBlock+pinnedBlock+houseProviderBlock+`DESKD=0
DESKD_ADDR=127.0.0.1:%s
`, cell, today, cell, repo, roots, githubHost, roles, cell, cell, port))
	writeFile(filepath.Join(d, "README.md"), fmt.Sprintf(houseReadme, cell, repo, realCfg, cell, cell, cell))
	fmt.Printf("[new] scaffolded %s (house cell) — cellctl check %s, then cellctl desk %s <role>\n", d, cell, cell)
}

const houseProviderBlock = "# Provider (optional; unset = Anthropic). `--model` alone only changes the model NAME — a\n" +
	"# non-Anthropic model additionally needs its endpoint and credential switched, which a provider\n" +
	"# does: uncomment and name a provider (any short slug), then declare its endpoint and the NAME of\n" +
	"# an env var THIS SHELL will carry the token in (never the token value itself):\n" +
	"# CELL_PROVIDER=zai\n" +
	"# CELL_PROVIDER_ZAI_BASE_URL=https://api.z.ai/api/anthropic\n" +
	"# CELL_PROVIDER_ZAI_TOKEN_ENV=ZAI_API_KEY\n" +
	"# --provider <name> on `cellctl desk`/`up` overrides this for one run without editing cell.env.\n" +
	"# No deskd on a house cell. DESKD=1 requires one, as on a k8s cell (then fill bin/deskd,\n" +
	"# bin/deskcli and a cells-%s.yaml slice, and stand it with CELL_ATTENDED=1 cellctl deskd %s).\n"

const houseReadme = "# %s cell (house) — scaffolded by cellctl\n" + `
A house cell is your own desks, on this laptop, booted the way a cell boots: one LOCKED
worktree per role off a fresh ` + "`origin/main`" + ` of ` + "`%s`" + `, the stream-root map exported as
` + "`DESK_ROOTS`" + `, the model pinned per role, and the roster beacon (` + "`DESK_SESSION`" + `) stamped with
the boot time. Nothing was copied: ` + "`home/.config/assay`" + ` is a symlink to ` + "`%s`" + `,
so the desk verbs read the roster and App keys you already have.

- ` + "`cellctl check %s`" + ` — prove the preconditions (git checkout, roster parses, every root
  carries ` + "`docs/streams/`" + `, desk verbs installed, assay plugin enabled).
- ` + "`cellctl desk %s <role>`" + ` — one role window; ` + "`cellctl up %s`" + ` — every role, in the
  cockpit ` + "`CELL_COCKPIT`" + ` resolves to (auto: herdr, else a reachable orca, else tmux).
- Edit ` + "`cell.env`" + ` to change ` + "`CELL_ROOTS`" + `, ` + "`ROLES`" + ` or a pinned model.
`

// newScrubbed scaffolds the OPPOSITE custody shape from a house cell: nothing is linked from the
// operator's real config home, because the whole point is that nothing of it reaches the
// harness. What IS created is a REAL (never symlinked) config home, a roster scoped to exactly
// one repo, and the directories the composed launch needs. Two hand steps remain, printed in the
// README: copy the role PEM(s) in as regular 0600 files, and log the harness in under the cell's
// own home.
func newScrubbed(e *Env, root, cell, repo, slug, roots, roles string) {
	if repo == "" {
		die("new: --kind scrubbed needs --repo <checkout>")
	}
	if slug == "" {
		die("new: --kind scrubbed needs --repo-slug <owner/repo>")
	}
	if !regexp.MustCompile(`^[^/\s]+/[^/\s]+$`).MatchString(slug) {
		die("new: --repo-slug must be <owner>/<repo>, got '%s'", slug)
	}
	if cell == "" {
		die("new: a cell name is required (cellctl new <cell> --kind scrubbed …)")
	}
	if !isGitCheckout(repo) {
		die("new: --repo is not a git checkout: %s", repo)
	}
	if roots != "" && !rootsValid(roots) {
		die("new: --roots is malformed")
	}
	d := filepath.Join(root, cell)
	if exists(d) {
		die("%s already exists (cellctl new never overwrites a cell — remove it yourself, or pick another name)", d)
	}
	mustMkdirAll(filepath.Join(d, "home", ".config", "assay"), filepath.Join(d, "home", ghConfigRelPath),
		filepath.Join(d, "worktrees"), filepath.Join(d, "tmp"), filepath.Join(d, "run"))
	writeFile(filepath.Join(d, "home", ".gitconfig"), "")
	chmod700(filepath.Join(d, "home"), filepath.Join(d, "home", ".config"),
		filepath.Join(d, "home", ".config", "assay"), filepath.Join(d, "tmp"))
	writeFile(filepath.Join(d, "home", ".config", "assay", "roster.env"),
		"# scrubbed cell roster — this cell is scoped to exactly one repo; a different or additional\n"+
			"# ASSAY_ALLOWED_REPOS entry is a MISS at `cellctl check` by design.\n"+
			"ASSAY_ALLOWED_REPOS="+slug+"\n"+
			"# Fill in by hand: ASSAY_TRUSTED_LOGINS, ASSAY_BLESS_LOGIN, ASSAY_TRUSTED_BOT_SLUGS (this cell's\n"+
			"# own App bindings — never copied from another cell), ASSAY_REPO_FORGES, ASSAY_SCAN_REPOS.\n")
	_ = os.Chmod(filepath.Join(d, "home", ".config", "assay", "roster.env"), 0o600)
	rootsLine := "# CELL_ROOTS=<owner>/<repo>=<abs path>,...   (exported as DESK_ROOTS at boot; unset = placeholder topology)"
	if roots != "" {
		rootsLine = "CELL_ROOTS=" + roots
	}
	today := time.Now().UTC().Format("2006-01-02")
	writeFile(filepath.Join(d, "cell.env"), fmt.Sprintf(`# cellctl cell.env — %s (scrubbed, scaffolded %s)
CELL=%s
CELL_KIND=scrubbed
CELL_REPO=%s
CELL_REPO_SLUG=%s
%s
ROLES="%s"
`+"# The harness a role window boots on: claude (default) or codex. `cellctl desk`/`smoke`\n"+
		"# `--harness` overrides it for one run without touching this line.\n"+
		"CELL_HARNESS=claude\n"+
		"# Pinned models (values `claude --model` accepts); the CLI default is never used.\n"+
		"DESK_MODEL_DEFAULT=sonnet\n"+
		"DESK_MODEL_the_desk=fable\n"+
		"# Codex gets its OWN model-pin namespace (CODEX_MODEL_<role> / CODEX_MODEL_default) — absent\n"+
		"# here, a role falls back to the tier map (docs/cellctl.md's Pinned models section):\n"+
		"# CODEX_MODEL_default=gpt-5.6-terra\n"+
		"# CODEX_MODEL_the_desk=gpt-5.6-terra\n"+
		"# Scrubbed cells never run a host deskd.\n"+
		"DESKD=0\n", cell, today, cell, repo, slug, rootsLine, roles))
	writeFile(filepath.Join(d, "README.md"), fmt.Sprintf(scrubbedReadme, cell, cell, d, d, cell, cell, cell, cell, cell))
	fmt.Printf("[new] scaffolded %s (scrubbed cell) — see %s/README.md for the hand steps\n", d, d)
}

const scrubbedReadme = "# %s cell (scrubbed) — scaffolded by cellctl\n" + `
A host-local harness cell whose environment is COMPOSED, not inherited: the launch runs
` + "`env -i`" + ` plus an explicit allowlist — nothing from your shell leaks in (no real ` + "`HOME`" + `, no
SSH agent, no forge/model credentials, no cluster access; ` + "`KUBECONFIG=/dev/null`" + ` is part of
the composed set). Two hand steps remain before ` + "`cellctl check %s`" + `:

1. Copy the role App PEM(s) you need into ` + "`home/.config/assay/`" + ` as REGULAR, mode-0600 files
   (` + "`<role>-app.pem`" + `) — never a symlink into your real config home. If this cell mints tokens,
   add a new ` + "`home/.config/assay/apps.env`" + ` naming the App ids and ` + "`<ROLE>_PEM=`" + ` paths.
2. Log the harness in UNDER THE CELL HOME — nothing is copied from your real harness login:
   - codex:  ` + "`CODEX_HOME=%s/home/.codex codex login`" + `
   - claude: ` + "`CLAUDE_CONFIG_DIR=%s/home/.claude claude`" + ` (log in inside that session)

Then: ` + "`cellctl check %s`" + `, ` + "`cellctl smoke %s`" + ` (a one-shot, tool-free, read-only readiness
probe — the harness answers ` + "`READY`" + ` or the verb says what it said instead), ` + "`cellctl desk %s\n<role>`" + `, ` + "`cellctl status %s`" + `, ` + "`cellctl down %s`" + `. The cell is single-occupancy: one
private-socket tmux session (` + "`run/tmux.sock`" + `) and one session lock (` + "`run/lock.d`" + `) at a time —
a second ` + "`desk`" + ` while one is live is refused, naming the live pid and session.
`

// ---- small write helpers --------------------------------------------------------------------

func nextOrEmpty(args []string, i *int) string {
	if *i+1 >= len(args) {
		*i++
		return ""
	}
	*i++
	return args[*i]
}

func mustMkdirAll(paths ...string) {
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			die("new: cannot create %s: %v", p, err)
		}
	}
}

func chmod700(paths ...string) {
	for _, p := range paths {
		if err := os.Chmod(p, 0o700); err != nil {
			die("new: cannot chmod %s: %v", p, err)
		}
	}
}

func writeFile(p, body string) {
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		die("new: cannot write %s: %v", p, err)
	}
}

func copyFile(src, dst string) {
	raw, err := os.ReadFile(src)
	if err != nil {
		die("new: cannot read %s: %v", src, err)
	}
	mode := os.FileMode(0o644)
	if st, serr := os.Stat(src); serr == nil {
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(dst, raw, mode); err != nil {
		die("new: cannot write %s: %v", dst, err)
	}
}

// linkIfPresent mirrors the oracle's `[[ -e <src> ]] && ln -s <src> <dst>`: absent is not an
// error, it simply leaves the link unmade.
func linkIfPresent(src, dst string) {
	if !exists(src) {
		return
	}
	_ = os.Symlink(src, dst)
}
