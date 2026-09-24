package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// shimTemplate is the generated wrapper body. Everything in it is LITERAL shell, evaluated when
// the generated shim itself runs under the caller's REAL, un-swapped HOME — only the `env HOME=`
// on the exec lines swaps it for the wrapped verb.
//
// assay#1145: `gh`'s ambient credential (keychain on macOS, hosts.yml-adjacent elsewhere) is
// keyed to the REAL HOME, so a `gh` subprocess a shimmed verb shells out to silently loses auth
// once HOME is swapped. It is resolved HERE, before the swap.
//
// assay#1631: it is NOT handed to the verb as GH_TOKEN. Desk verbs read an inherited GH_TOKEN as
// the operator's EXPLICIT choice of credential (deskdispatch skipped its role-App mint on it), so
// exporting the human login there made every self-minting verb act as the human. The token
// travels as CELLCTL_GH_AMBIENT instead — a name no desk verb reads — and only the cell's `gh`
// wrapper (ghWrapTemplate, prepended to the verb's PATH) turns it back into GH_TOKEN, for a `gh`
// child with none of its own. An explicit GH_TOKEN/GH_ENTERPRISE_TOKEN already in the caller's
// env still passes through untouched and is never looked up against.
const shimTemplate = `#!/usr/bin/env bash
# cellctl shim (__CELL__): run this desk verb with the CELL config-home; the session keeps the
# real HOME. the ambient gh credential is keyed to the real HOME, so it is resolved HERE (before
# HOME is swapped below) and handed over as CELLCTL_GH_AMBIENT — never as GH_TOKEN, which desk
# verbs read as an explicit operator override (assay#1631). Only the cell's gh wrapper, first on
# the verb's PATH, turns it back into GH_TOKEN, for a ` + "`gh`" + ` child with none of its own (assay#1145).
gh_token="${CELLCTL_GH_AMBIENT:-}"
if [[ -z "$gh_token" && -z "${GH_TOKEN:-}" && -z "${GH_ENTERPRISE_TOKEN:-}" ]] && command -v gh >/dev/null 2>&1; then
  gh_token="$(gh auth token 2>/dev/null || true)"
fi
if [[ -n "$gh_token" ]]; then
  exec env HOME="__CELL_HOME__" PATH="__GH_WRAP__:$PATH" CELLCTL_GH_AMBIENT="$gh_token" "__BIN__" "$@"
else
  exec env HOME="__CELL_HOME__" "__BIN__" "$@"
fi
`

// ghWrapTemplate is the cell's `gh` wrapper (assay#1631), written to <cell>/shim-gh/gh and put
// first on a shimmed verb's PATH only. It removes its own directory from PATH (every occurrence:
// a shimmed verb that runs another shimmed verb prepends it twice) so the `gh` it execs is the
// real one, never itself, and sets GH_TOKEN from CELLCTL_GH_AMBIENT only when the caller has no
// GH_TOKEN/GH_ENTERPRISE_TOKEN — a verb that hands its `gh` child its own role token keeps it.
const ghWrapTemplate = `#!/usr/bin/env bash
# cellctl gh wrapper (__CELL__): the ONE place the cell's ambient gh credential becomes GH_TOKEN,
# and only for gh itself (assay#1631). A desk verb's own code never sees it as GH_TOKEN.
wrap="__GH_WRAP__"
p=":$PATH:"
while [[ "$p" == *":$wrap:"* ]]; do p="${p//":$wrap:"/:}"; done
p="${p#:}"; p="${p%:}"
export PATH="$p"
if [[ -z "${GH_TOKEN:-}" && -z "${GH_ENTERPRISE_TOKEN:-}" && -n "${CELLCTL_GH_AMBIENT:-}" ]]; then
  exec env GH_TOKEN="$CELLCTL_GH_AMBIENT" gh "$@"
fi
exec gh "$@"
`

// genShims writes one wrapper per installed desk verb into <cell>/shim, the cell's `gh` wrapper
// into <cell>/shim-gh, plus a symlink per binary in <cell>/bin so the cell's own deskd/deskcli
// come first on PATH too. Substitution is by plain replacement, never sed: a bin path or CELL
// name containing `&` or a regex metacharacter would otherwise corrupt it.
func (c *Cell) genShims() {
	shimDir := filepath.Join(c.Dir, "shim")
	ghWrapDir := filepath.Join(c.Dir, "shim-gh")
	for _, d := range []string{shimDir, ghWrapDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			die("desk: cannot create %s: %v", d, err)
		}
	}
	// The CLI wrappers this cell writes, keyed by the file name each is installed under — one
	// today, the forge CLI the ambient credential is resolved for.
	for name, tmpl := range map[string]string{"gh": ghWrapTemplate} {
		wrap := strings.NewReplacer("__CELL__", c.Name, "__GH_WRAP__", ghWrapDir).Replace(tmpl)
		if werr := os.WriteFile(filepath.Join(ghWrapDir, name), []byte(wrap), 0o755); werr != nil {
			die("desk: cannot write the %s wrapper: %v", name, werr)
		}
	}
	bin := deskToolsBin(c.Env)
	entries, err := os.ReadDir(bin)
	if err == nil {
		for _, en := range entries {
			p := filepath.Join(bin, en.Name())
			st, serr := os.Stat(p)
			if serr != nil || st.IsDir() || st.Mode()&0o111 == 0 {
				continue
			}
			body := strings.NewReplacer(
				"__CELL__", c.Name,
				"__CELL_HOME__", c.Home,
				"__GH_WRAP__", ghWrapDir,
				"__BIN__", p,
			).Replace(shimTemplate)
			if werr := os.WriteFile(filepath.Join(shimDir, en.Name()), []byte(body), 0o755); werr != nil {
				die("desk: cannot write shim %s: %v", en.Name(), werr)
			}
		}
	}
	cellBin := filepath.Join(c.Dir, "bin")
	if es, berr := os.ReadDir(cellBin); berr == nil {
		for _, en := range es {
			target := filepath.Join(cellBin, en.Name())
			link := filepath.Join(shimDir, en.Name())
			_ = os.Remove(link)
			_ = os.Symlink(target, link)
		}
	}
}

// ensureCodexResidentRules appends the pre-generated resident-rules fragment to the WORKTREE's
// AGENTS.md at boot, once. codex has no Claude-style SessionStart hook to carry the resident
// operating rules, so on that arm they travel in AGENTS.md instead. Idempotent on a marker
// string unique to the fragment, so a re-boot of the same worktree never duplicates it.
func (c *Cell) ensureCodexResidentRules(wt string) {
	frag := filepath.Join(c.Repo, "plugins", "assay", "codex", "AGENTS-assay.md")
	const marker = "Assay resident operating rules"
	fragBody, err := os.ReadFile(frag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: no resident-rules fragment at %s — %s/AGENTS.md not updated\n", frag, wt)
		return
	}
	target := filepath.Join(wt, "AGENTS.md")
	if cur, rerr := os.ReadFile(target); rerr == nil && strings.Contains(string(cur), marker) {
		return
	}
	needNL := false
	if st, serr := os.Stat(target); serr == nil && st.Size() > 0 {
		needNL = true
	}
	f, oerr := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if oerr != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: could not append the resident-rules fragment to %s: %v\n", target, oerr)
		return
	}
	defer f.Close()
	if needNL {
		_, _ = f.WriteString("\n")
	}
	_, _ = f.Write(fragBody)
	fmt.Printf("[codex] resident-rules fragment appended to %s\n", target)
}

// pluginEnabled reports whether the assay@assay plugin is enabled for the checkout, as
// `claude plugin list` reports it. Run from CELL_REPO because a project-scoped enable is keyed
// to the project path.
func (c *Cell) pluginEnabled(cfg string) bool {
	if _, err := exec.LookPath("claude"); err != nil {
		return false
	}
	cmd := exec.Command("claude", "plugin", "list", "--json")
	cmd.Dir = c.Repo
	cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+cfg)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return pluginListHasAssay(out)
}
