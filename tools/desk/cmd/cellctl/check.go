package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// checker accumulates `cellctl check`'s rows and its exit code. Three row kinds, and the
// distinction between them is the whole point of the verb:
//
//	ok    — the precondition holds
//	MISS  — it does not, and `check` exits 1
//	n/a   — it belongs to the OTHER kind or forge, and is STATED, never silently skipped, so a
//	        half-provisioned or mis-kinded cell reads as such rather than clean
//	warn  — VISIBLE but non-fatal: it names a condition that will degrade the desks without
//	        being a precondition cellctl can assert (staying current with an upstream is an
//	        operator step), so it must not decide the exit code
type checker struct{ ok bool }

func (k *checker) chk(cond bool, format string, args ...any) {
	if cond {
		fmt.Printf("  ok    %s\n", fmt.Sprintf(format, args...))
		return
	}
	fmt.Printf("  MISS  %s\n", fmt.Sprintf(format, args...))
	k.ok = false
}

func (k *checker) na(format string, args ...any) {
	fmt.Printf("  n/a   %s\n", fmt.Sprintf(format, args...))
}
func (k *checker) warn(format string, args ...any) {
	fmt.Printf("  warn  %s\n", fmt.Sprintf(format, args...))
}

func cmdCheck(cell, cfgArg string) {
	c := loadCell(cell)
	k := &checker{ok: true}

	policy, policySource, policyErr := c.cellModelPolicy()
	if policyErr != nil {
		die("%s", policyErr)
	}
	if policy != nil && (c.Kind == "container" || c.Kind == "scrubbed") {
		die("model policy requires a house or k8s cell")
	}

	if c.Kind == "container" {
		if cfgArg != "" {
			die("container check does not accept a host config directory")
		}
		for _, role := range c.Roles {
			rm := c.resolveRoleModel(role, c.Harness)
			if !rm.OK {
				die("%s", rm.Src)
			}
			if role == "the-desk" && c.Harness == "claude" {
				refuseOpusForTheDesk(rm.Model)
			}
			fmt.Printf("[model] role=%s harness=%s model=%s\n", role, c.Harness, rm.Model)
		}
		c.containerRun("check")
		return
	}

	fmt.Printf("[check] cell=%s dir=%s kind=%s forge=%s\n", c.Name, c.Dir, c.Kind, c.Forge)
	// A scrubbed cell's whole point is that it never touches the operator's real config home —
	// this row is n/a there (checkScrubbed proves the cell's OWN config home instead), unlike
	// k8s/house where the cell's custody IS (a copy of, or a symlink to) that directory.
	real := realConfigHome(c.Env)
	if c.Kind == "scrubbed" {
		k.na("operator config home — not applicable on a scrubbed cell (it never reads %s; see the config-home rows below)", real)
	} else {
		k.chk(isDir(real), "operator config home (symlink targets): %s", real)
	}
	k.chk(isRegular(filepath.Join(c.Config, "roster.env")), "roster: %s/roster.env", c.Config)
	k.chk(!hasBrokenSymlink(filepath.Join(c.Home, ".config")), "App key symlinks resolve")
	k.chk(c.ForgeAPIBase != "", "forge endpoint (%s): %s", c.Forge, c.ForgeAPIBase)

	// the-desk's resolved model, same precedence cmd_desk uses. `check` is deliberately never
	// affected by --model / DESK_MODEL_OVERRIDE: it has no --model flag and does not read the
	// override, so this row reports what a PLAIN boot would resolve to.
	theDeskModel := c.Env.GetOr("DESK_MODEL_the_desk", c.Env.Get("DESK_MODEL_DEFAULT"))
	if policy != nil {
		k.na("legacy coordinator pin — model policy takes precedence")
	} else if isOpusPin(theDeskModel) {
		k.chk(false, "the-desk model: the-desk runs on the top tier; an Opus pin is refused for the coordinator — resolved DESK_MODEL_the_desk=%s (from DESK_MODEL_the_desk or DESK_MODEL_DEFAULT); set DESK_MODEL_the_desk=fable (or another non-Opus id) in cell.env", theDeskModel)
	} else {
		k.chk(true, "the-desk model: %s", theDeskModel)
	}

	// Per-role harness + resolved-model rows (#986): what `cellctl desk <cell> <role>` (no
	// --model) would resolve to on THIS cell's harness, for every role the cell runs. A role
	// with no per-harness pin and no tier match is a MISS naming exactly what was checked,
	// surfaced HERE rather than discovered as a startup failure.
	for _, role := range c.Roles {
		if policy != nil {
			route, err := policy.Resolve(role, "", "", "")
			if err != nil {
				k.chk(false, "%s", err)
				continue
			}
			k.chk(true, "role %s: provider=%s harness=%s model=%s effort=%s source=%s sha256=%s", role, route.Provider, route.Harness, route.Model, route.Effort, policySource, policy.SHA256)
			continue
		}
		rm := c.resolveRoleModel(role, c.Harness)
		switch {
		case !rm.OK:
			k.chk(false, "model pin: role=%s harness=%s — %s", role, c.Harness, rm.Src)
		case role == "the-desk" && c.Harness == "claude" && isOpusPin(rm.Model):
			k.chk(false, "model pin: role=%s harness=%s — the-desk runs on the top tier; an Opus pin is refused for the coordinator — resolved %s=%s; set DESK_MODEL_the_desk=fable (or another non-Opus id) in cell.env", role, c.Harness, rm.Src, rm.Model)
		default:
			k.chk(true, "model pin: role=%s harness=%s model=%s (from %s)", role, c.Harness, rm.Model, rm.Src)
		}
	}

	// The cell's DEFAULT provider. Unset is a legitimate n/a (Anthropic, the long-standing
	// default) rather than a MISS — a provider is opt-in per cell. Values shown are the
	// endpoint, the token env var's NAME, the model and whether the named variable is set —
	// never a token value.
	if policy != nil {
		seen := map[string]bool{}
		for _, role := range c.Roles {
			route, err := policy.Resolve(role, "", "", "")
			if err != nil || seen[route.Provider] {
				continue
			}
			seen[route.Provider] = true
			if route.Provider == "anthropic" || route.Provider == "codex" {
				k.na("provider %s uses native harness authentication", route.Provider)
				continue
			}
			base, _ := c.providerValue(route.Provider, "BASE_URL")
			tokenVar, _ := c.providerValue(route.Provider, "TOKEN_ENV")
			k.chk(base != "", "provider %s endpoint: %s", route.Provider, base)
			k.chk(tokenVar != "" && c.Env.Get(tokenVar) != "", "provider %s: token environment variable %s is set", route.Provider, tokenVar)
		}
	} else if p := c.Env.Get("CELL_PROVIDER"); p != "" {
		baseVar, tokVar := providerVar(p, "BASE_URL"), providerVar(p, "TOKEN_ENV")
		baseVal, baseSrc := c.providerValue(p, "BASE_URL")
		tokVal, tokSrc := c.providerValue(p, "TOKEN_ENV")
		modelVal, modelSrc := c.providerValue(p, "MODEL")
		k.chk(baseVal != "", "provider %s: %s=%s (%s)", p, baseVar, orDefault(baseVal, "<unset>"), baseSrc)
		suffix := ""
		if tokVal != "" {
			suffix = "=" + tokVal
		}
		k.chk(tokVal != "", "provider %s: %s (names the token env var, never the token)%s (%s)", p, tokVar, suffix, tokSrc)
		if tokVal != "" {
			k.chk(c.Env.Get(tokVal) != "", "provider %s: $%s is set in this shell", p, tokVal)
		}
		if modelVal != "" {
			k.chk(true, "provider %s: model=%s (%s; exported as ANTHROPIC_MODEL + ANTHROPIC_DEFAULT_{OPUS,SONNET,HAIKU}_MODEL at launch)", p, modelVal, modelSrc)
		} else {
			k.na("provider %s: model — no %s and no preset; the role pin is exported as is", p, providerVar(p, "MODEL"))
		}
	} else {
		k.na("provider — CELL_PROVIDER unset (Anthropic, the default; set CELL_PROVIDER + CELL_PROVIDER_<NAME>_BASE_URL/_TOKEN_ENV in cell.env to switch it)")
	}

	harnessCheck := *c
	if policy != nil {
		harnessCheck.Harness = "claude"
		for _, role := range c.Roles {
			if route, err := policy.Resolve(role, "", "", ""); err == nil && route.Harness == "codex" {
				harnessCheck.Harness = "codex"
			}
		}
	}
	harnessCheck.checkCodexHarness(k)
	switch c.Kind {
	case "house":
		c.checkHouse(k, cfgArg)
	case "scrubbed":
		if cfgArg != "" {
			die("scrubbed check does not accept a host config directory")
		}
		c.checkScrubbed(k)
	default:
		c.checkK8s(k)
	}

	// Stream-root drift. The roots are what the desk verbs READ, and nothing in cellctl
	// advances them, so a root behind its upstream is the failure that presents as a
	// permanently stale board rather than as an error anywhere. Never fatal.
	if roots := c.Env.Get("CELL_ROOTS"); roots != "" {
		for _, e := range rootEntries(roots) {
			name, p := e[0], e[1]
			st, ok := rootsState(p)
			if !ok {
				k.na("root %s — not comparable (missing, detached HEAD, or no upstream); drift unknown", name)
				continue
			}
			if st.Behind != "0" {
				k.warn("root %s: %s is %s commit(s) BEHIND %s — the desks read this tree (fix: git -C %s merge --ff-only %s, or CELL_FF_ROOTS=1 in cell.env)", name, st.Branch, st.Behind, st.Upstream, p, st.Upstream)
			} else {
				k.chk(true, "root %s current with %s (branch %s)", name, st.Upstream, st.Branch)
			}
			if st.Ahead != "0" {
				k.warn("root %s: %s is %s commit(s) ahead of %s (unpushed work)", name, st.Branch, st.Ahead, st.Upstream)
			}
			if st.Dirty != "0" {
				k.warn("root %s: %s uncommitted change(s) in %s", name, st.Dirty, p)
			}
		}
	}

	k.chk(onPath("tmux"), "tmux (the always-works cockpit, and every other cockpit's fallback)")
	// The COCKPIT row resolves exactly as `up` will and says why, so "which surface will my role
	// windows appear in" is answerable before booting. An explicit CELL_COCKPIT that is not
	// available is a MISS (it is what `up` would refuse on); `auto` can never MISS.
	want, src := c.cockpitWant("")
	if res := c.resolveCockpit(want, src); res.Err == "" {
		fmt.Printf("  ok    cockpit: %s (%s)\n", res.Cockpit, res.Why)
		// The same resolved value is what every role window exports as ASSAY_COCKPIT — the
		// worker-desk worktree-create arm — so the row says which arm that is, not only which
		// surface.
		if c.Kind != "scrubbed" {
			arm := res.Cockpit + " worktree create"
			if res.Cockpit == "tmux" {
				arm = "plain git worktree add, no cockpit CLI needed"
			}
			fmt.Printf("  ok    %s=%s exported into every role window (worker-desk worktree arm: %s)\n", envAssayCockpit, res.Cockpit, arm)
		}
	} else {
		fmt.Printf("  MISS  cockpit: %s\n", res.Err)
		k.ok = false
	}
	// Orca reachability is stated whenever orca is installed, whichever cockpit won: its CLI is
	// a thin client of the desktop app, and an unreachable app is why `auto` passed it over.
	if onPath("orca") {
		if c.orcaReachable() {
			fmt.Println("  ok    orca desktop app reachable (its CLI is a thin client)")
		} else {
			fmt.Println("  n/a   orca on PATH but its desktop app is not reachable — auto passes it over; start the app (or 'orca serve') to use it")
		}
	} else {
		k.na("orca — not installed (auto uses herdr if present, else tmux)")
	}

	if c.Deskd == "1" {
		k.chk(isExecFile(filepath.Join(c.Dir, "bin", "deskd")), "deskd binary: %s/bin/deskd", c.Dir)
		k.chk(isExecFile(filepath.Join(c.Dir, "bin", "deskcli")), "deskcli binary: %s/bin/deskcli", c.Dir)
		k.chk(c.deskdUp(), "deskd up on %s", c.DeskdAddr)
	} else {
		k.na("deskd — not required on this %s cell with DESKD=0 (set DESKD=1 in cell.env to require one)", c.Kind)
	}

	if k.ok {
		fmt.Println("[check] all preconditions met")
		return
	}
	fmt.Println("[check] fix the MISS rows first")
	exitWith(1)
}

// checkK8s: its own deskd, its own roster and keys, a cells.yaml slice.
func (c *Cell) checkK8s(k *checker) {
	k.chk(isDir(filepath.Join(c.Repo, ".git")), "cell.env CELL_REPO is a git checkout: %s", c.Repo)
	k.chk(isRegular(c.CellsCfg), "cells config: %s", c.CellsCfg)
	if roots := c.Env.Get("CELL_ROOTS"); roots != "" {
		k.chk(rootsValid(roots), "CELL_ROOTS well-formed (exported as DESK_ROOTS at boot)")
	} else {
		k.na("CELL_ROOTS unset — role windows boot WITHOUT DESK_ROOTS (the verbs fall back to their compiled placeholder topology)")
	}
	switch c.Forge {
	case "github":
		k.chk(isRegular(filepath.Join(c.Config, "apps.env")), "apps.env: %s/apps.env", c.Config)
		k.chk(exists(filepath.Join(c.Home, ghConfigRelPath)), "gh config linked: %s/.config/gh", c.Home)
		// The App key and the org list exist to mint deskd's per-org tokens, so they are
		// preconditions only when this cell requires a deskd.
		if c.Deskd == "1" {
			k.chk(isReadable(c.Env.GetOr("DESKD_APP_PEM", "/nonexistent")), "deskd read App key: %s", c.Env.GetOr("DESKD_APP_PEM", "unset"))
			k.chk(c.Env.Get("ORGS") != "", "orgs to mint tokens for: %s", c.Env.GetOr("ORGS", "unset"))
		} else {
			k.na("deskd read App key / orgs to mint — not applicable with DESKD=0 (no deskd on this cell)")
		}
		k.na("GitLab role token store — not applicable on a github cell")
	case "gitlab":
		k.chk(c.Env.Get("GITLAB_GROUP") != "", "GitLab group: %s", c.Env.GetOr("GITLAB_GROUP", "unset"))
		k.chk(isDir(c.Env.GetOr("GITLAB_TOKEN_STORE", "/nonexistent")), "GitLab role token store: %s", c.Env.GetOr("GITLAB_TOKEN_STORE", "unset"))
		// NOT gated on DESKD, unlike the github arm's App key: despite the deskd- prefix,
		// DESKD_GITLAB_TOKEN_FILE is ALSO the credential the boot fetch uses, and that runs with
		// DESKD=0. A gitlab cell needs this token to fetch CELL_REPO at all.
		k.chk(isReadable(c.Env.GetOr("DESKD_GITLAB_TOKEN_FILE", "/nonexistent")),
			"GitLab cell token (0600, hand-provisioned; deskd read + boot fetch credential): %s", c.Env.GetOr("DESKD_GITLAB_TOKEN_FILE", "unset"))
		// CELL_REPO's own fetch transport, not just the API token: probes the SAME credential
		// helper the boot fetch uses, so a broken/missing token is a MISS here rather than a
		// boot-time hang at an interactive Username prompt.
		k.chk(c.gitlabFetchReachable(), "GitLab fetch transport reachable (ls-remote, prompts disabled): %s", c.Repo)
		// GitHub-only preconditions on a GitLab cell are reported explicitly, never dropped: a
		// stray App PEM is a MISS (a github artifact on a gitlab cell is a misconfiguration to
		// surface), and the gh-config link is n/a.
		if pem := c.Env.Get("DESKD_APP_PEM"); pem != "" {
			k.chk(false, "stray GitHub App PEM on a gitlab cell (DESKD_APP_PEM should be unset): %s", pem)
		} else {
			k.na("GitHub App PEM — not applicable on a gitlab cell")
		}
		k.na("gh config link — not applicable on a gitlab cell (GitLab custody is the role token store)")
	default:
		k.chk(false, "known forge (github|gitlab): %s", c.Forge)
	}
	k.chk(isExecFile(filepath.Join(deskToolsBin(c.Env), "deskboot")), "desk-tools: %s/deskboot", deskToolsBin(c.Env))
}

// checkHouse: the operator's own desks. What must hold is what a hand boot gets wrong — the
// checkout is a real git checkout, the roster the desk verbs will read PARSES (not merely
// exists), every stream root in the map is a checkout carrying docs/streams/, the desk verbs are
// installed, and the assay plugin is enabled for the checkout the windows will open in.
func (c *Cell) checkHouse(k *checker, cfgArg string) {
	cfg := resolveCfg(c.Env, cfgArg)
	k.chk(isGitCheckout(c.Repo), "cell.env CELL_REPO is a git checkout: %s", c.Repo)
	link, _ := os.Readlink(c.Config)
	k.chk(link == realConfigHome(c.Env), "config home linked to the operator's: %s -> %s", c.Config, realConfigHome(c.Env))
	k.chk(exists(filepath.Join(c.Home, ".gitconfig")), "gitconfig linked: %s/.gitconfig", c.Home)
	k.chk(exists(filepath.Join(c.Home, ghConfigRelPath)), "gh config linked: %s/.config/gh", c.Home)
	k.chk(c.rosterParses(), "roster parses under the cell home: deskroster repos --scope scan")
	k.chk(rootsValid(c.Env.Get("CELL_ROOTS")), "CELL_ROOTS well-formed: %s", c.Env.Get("CELL_ROOTS"))
	for _, e := range rootEntries(c.Env.Get("CELL_ROOTS")) {
		k.chk(isDir(e[1]), "root %s exists: %s", e[0], e[1])
		k.chk(isDir(filepath.Join(e[1], "docs", "streams")), "root %s carries docs/streams/", e[0])
	}
	for _, v := range strings.Fields(houseVerbs) {
		k.chk(isExecFile(filepath.Join(deskToolsBin(c.Env), v)), "desk verb: %s/%s", deskToolsBin(c.Env), v)
	}
	k.chk(onPath("claude"), "claude on PATH")
	k.chk(c.pluginEnabled(cfg), "plugin assay@assay enabled for %s (config %s)", c.Repo, cfg)
	k.na("cells config / apps.env / deskd App key — not applicable on a house cell (the operator's own config home is used as is)")
}

// checkScrubbed: what a hand-composed environment gets wrong. Unlike checkHouse (which proves a
// SYMLINK resolves), a scrubbed cell's whole point is that nothing is shared with the operator's
// real config — so this proves the config home is a REAL directory at 0700, every PEM under it
// is a regular 0600 file, the roster scopes the cell to exactly one repo, the harness is logged
// in under the CELL's own home, the roster parses, every CELL_ROOTS entry resolves, the desk
// verbs are installed, and the lock/run directory is writable.
func (c *Cell) checkScrubbed(k *checker) {
	k.chk(isGitCheckout(c.Repo), "cell.env CELL_REPO is a git checkout: %s", c.Repo)
	k.chk(isDir(c.Config) && !isSymlink(c.Config), "config home is a REAL directory (never a symlink): %s", c.Config)
	k.chk(modeIs(c.Config, 0o700), "config home mode 0700: %s", c.Config)

	// Every PEM the cell's apps.env names, plus every *-app.pem present directly under the cell
	// config home — de-duplicated. None present yet is a legitimate n/a (the hand step from
	// `new`'s README has not run), never a MISS.
	var pems []string
	if matches, _ := filepath.Glob(filepath.Join(c.Config, "*-app.pem")); matches != nil {
		for _, m := range matches {
			if exists(m) || isSymlink(m) {
				pems = append(pems, m)
			}
		}
	}
	if f, err := os.Open(filepath.Join(c.Config, "apps.env")); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			if i := strings.Index(line, "_PEM="); i >= 0 {
				if v := line[i+len("_PEM="):]; v != "" {
					pems = append(pems, v)
				}
			}
		}
		f.Close()
	}
	if len(pems) == 0 {
		k.na("role App PEMs — none present yet under %s (copy them in as regular 0600 files, per the cell's README)", c.Config)
	} else {
		seen := map[string]bool{}
		for _, f := range pems {
			if seen[f] {
				continue
			}
			seen[f] = true
			switch {
			case isSymlink(f):
				k.chk(false, "PEM %s is a regular file (not a symlink)", f)
			case !isRegular(f):
				k.chk(false, "PEM %s exists", f)
			default:
				k.chk(modeIs(f, 0o600), "PEM %s is regular, mode 0600", f)
			}
		}
	}

	allowed := c.rosterAllowedRepos()
	k.chk(allowed == c.Env.Get("CELL_REPO_SLUG"),
		"roster ASSAY_ALLOWED_REPOS is exactly %s (a scrubbed cell is scoped to one repo)", c.Env.Get("CELL_REPO_SLUG"))

	if c.Harness == "codex" {
		cmd := exec.Command("codex", "login", "status")
		cmd.Env = append(os.Environ(), "CODEX_HOME="+filepath.Join(c.Home, ".codex"))
		ok := cmd.Run() == nil
		if !ok {
			cmd = exec.Command("codex", "login", "status", "--json")
			cmd.Env = append(os.Environ(), "CODEX_HOME="+filepath.Join(c.Home, ".codex"))
			ok = cmd.Run() == nil
		}
		k.chk(ok, "harness login under the cell home: codex login status (CODEX_HOME=%s/.codex)", c.Home)
	} else {
		ok := isDir(filepath.Join(c.Home, ".claude"))
		if ok {
			cmd := exec.Command("claude", "--version")
			cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+filepath.Join(c.Home, ".claude"))
			ok = cmd.Run() == nil
		}
		k.chk(ok, "harness login under the cell home: %s/.claude present, claude --version (CLAUDE_CONFIG_DIR=%s/.claude)", c.Home, c.Home)
	}
	k.chk(c.rosterParses(), "roster parses under the cell home: deskroster repos --scope scan")
	if roots := c.Env.Get("CELL_ROOTS"); roots != "" {
		k.chk(rootsValid(roots), "CELL_ROOTS well-formed: %s", roots)
		for _, e := range rootEntries(roots) {
			k.chk(isDir(e[1]), "root %s exists: %s", e[0], e[1])
			k.chk(isDir(filepath.Join(e[1], "docs", "streams")), "root %s carries docs/streams/", e[0])
		}
	} else {
		k.na("CELL_ROOTS — unset (role windows boot WITHOUT DESK_ROOTS)")
	}
	for _, v := range strings.Fields(houseVerbs) {
		k.chk(isExecFile(filepath.Join(deskToolsBin(c.Env), v)), "desk verb: %s/%s", deskToolsBin(c.Env), v)
	}
	runDir := filepath.Join(c.Dir, "run")
	k.chk(os.MkdirAll(runDir, 0o700) == nil && isWritableDir(runDir), "lock/run dir writable: %s", runDir)
}

// checkCodexHarness: run only when this cell's CELL_HARNESS is codex — a claude-only cell
// reports n/a, never silently skipped, mirroring every other kind/forge-specific block.
func (c *Cell) checkCodexHarness(k *checker) {
	if c.Harness != "codex" {
		k.na("codex harness preconditions — CELL_HARNESS=%s (set CELL_HARNESS=codex in cell.env, or --harness codex per run, to require them)", c.Harness)
		return
	}
	k.chk(onPath("codex"), "codex on PATH")
	k.chk(onPath("codex") && exec.Command("codex", "--version").Run() == nil, "codex --version")
	authed := onPath("codex") && (exec.Command("codex", "login", "status").Run() == nil ||
		exec.Command("codex", "login", "status", "--json").Run() == nil)
	k.chk(authed, "codex authenticated (codex login status, or equivalent)")
	k.chk(c.codexMultiAgentOn(), "[features] multi_agent = true (effective config, or a -c override applied)")
	k.chk(fileContains(filepath.Join(c.Repo, "AGENTS.md"), "Assay resident operating rules"),
		"resident-rules fragment present: %s/AGENTS.md", c.Repo)
	k.chk(c.codexSkillsDiscoverable(), "skills discoverable: marketplace plugin assay@assay, or .agents/skills/ placed")
}

// codexMultiAgentOn is true when `[features] multi_agent = true` resolves in codex's effective
// config. Probed via a `codex config get`-style verb where the installed build advertises one —
// codex's config-read surface has moved across releases, so it is never hard-coded to a single
// spelling — falling back to a literal read of config.toml under CODEX_HOME. A `-c` override
// given on the invocation itself is not visible here, which is why the row's text says "or -c
// override applied".
func (c *Cell) codexMultiAgentOn() bool {
	home := c.Env.Get("CODEX_HOME")
	if home == "" {
		home = filepath.Join(c.Env.Get("HOME"), ".codex")
	}
	if helpHas("get", "codex", "config") {
		out, err := exec.Command("codex", "config", "get", "features.multi_agent").Output()
		if err == nil && strings.TrimSpace(string(out)) == "true" {
			return true
		}
	}
	f, err := os.Open(filepath.Join(home, "config.toml"))
	if err != nil {
		return false
	}
	defer f.Close()
	insec := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "[features]") {
			insec = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			insec = false
		}
		if insec && strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), " ")) == "multi_agent = true" {
			return true
		}
		if insec {
			t := strings.Join(strings.Fields(line), " ")
			if t == "multi_agent = true" {
				return true
			}
		}
	}
	return false
}

// codexSkillsDiscoverable: either discovery arm satisfies invoke-by-name — arm A (marketplace
// plugin assay@assay installed) or arm B (skills copied into the checkout's .agents/skills/).
// Checked against CELL_REPO, the checkout the role worktree is cut from.
func (c *Cell) codexSkillsDiscoverable() bool {
	if onPath("codex") && helpHas("list", "codex", "plugin") {
		cmd := exec.Command("codex", "plugin", "list")
		cmd.Dir = c.Repo
		if out, err := cmd.Output(); err == nil && strings.Contains(strings.ToLower(string(out)), "assay") {
			return true
		}
	}
	entries, err := os.ReadDir(filepath.Join(c.Repo, ".agents", "skills"))
	if err != nil {
		return false
	}
	for _, en := range entries {
		if en.IsDir() {
			return true
		}
	}
	return false
}

// ---- small filesystem predicates, one spelling each ----

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func isRegular(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func isSymlink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

func isReadable(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func isWritableDir(p string) bool {
	probe := filepath.Join(p, ".cellctl-write-probe")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	f.Close()
	_ = os.Remove(probe)
	return true
}

func modeIs(p string, want os.FileMode) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().Perm() == want
}

// hasBrokenSymlink walks a tree looking for a symlink whose target does not resolve — the
// oracle's `find … -type l ! -exec test -e {} \; -print`.
func hasBrokenSymlink(root string) bool {
	found := false
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		if _, serr := os.Stat(p); serr != nil {
			found = true
		}
		return nil
	})
	return found
}

func fileContains(p, needle string) bool {
	raw, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return strings.Contains(string(raw), needle)
}
