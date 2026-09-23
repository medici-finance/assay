package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cmdDown tears the session (and this cell's deskd) down. What `up` opened in a non-tmux cockpit
// is closed where that cockpit offers a verb for it, and NAMED for you to close by hand where it
// does not — never left unsaid.
func cmdDown(cell string, args []string) {
	c := loadCell(cell)

	if c.Kind == "container" {
		if len(args) > 0 {
			die("container down does not accept host cockpit or deskd flags")
		}
		c.containerRun("down")
		return
	}
	if c.Kind == "scrubbed" {
		if len(args) > 0 {
			die("scrubbed down does not accept host cockpit or deskd flags")
		}
		sockpath := filepath.Join(c.Dir, "run", "tmux.sock")
		sessname := c.Name + "-cell"
		if isSocket(sockpath) && exec.Command("tmux", "-S", sockpath, "has-session", "-t", sessname).Run() == nil {
			_ = exec.Command("tmux", "-S", sockpath, "kill-session", "-t", sessname).Run()
			fmt.Printf("[cell] %s killed (private socket)\n", sessname)
		} else {
			fmt.Printf("[cell] no session %s\n", sessname)
		}
		// The lock is cleared HERE — never on the harness process's own exit — and the worktrees
		// under <cell>/worktrees/ are kept, exactly like every other kind's `down`.
		_ = os.RemoveAll(filepath.Join(c.Dir, "run", "lock.d"))
		return
	}

	keep := false
	cockpitFlag := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--keep-deskd":
			keep = true
		case "--cockpit":
			cockpitFlag = needFlagValue(args, &i, "--cockpit needs a value (auto|tmux|herdr|orca)")
		default:
			if strings.HasPrefix(a, "--") {
				die("down: unknown flag %s", a)
			}
			die("down: unexpected argument '%s'", a)
		}
	}
	want, src := c.cockpitWant(cockpitFlag)
	res := c.resolveCockpit(want, src)
	if res.Err == "" {
		fmt.Printf("[cockpit] %s (%s)\n", res.Cockpit, res.Why)
	} else {
		fmt.Fprintf(os.Stderr, "[cockpit] unresolved: %s — tearing down the tmux session and this cell's deskd only\n", res.Err)
		res.Cockpit = "tmux"
	}
	// The tmux session is torn down whichever cockpit resolves NOW: a cell brought up in tmux and
	// taken down after another cockpit was installed must still stop, and a kill-session on a
	// name that does not exist is a no-op, not an error.
	if exec.Command("tmux", "has-session", "-t", c.Session).Run() == nil {
		_ = exec.Command("tmux", "kill-session", "-t", c.Session).Run()
		fmt.Printf("[cell] %s killed\n", c.Session)
	} else {
		fmt.Printf("[cell] no session %s\n", c.Session)
	}
	switch res.Cockpit {
	case "herdr":
		c.downHerdr()
	case "orca":
		c.downOrca()
	}
	if !keep && c.Deskd == "1" {
		if exec.Command("pkill", "-f", "deskd --config "+c.CellsCfg).Run() == nil {
			fmt.Println("[cell] deskd stopped")
		} else {
			fmt.Println("[cell] no deskd for this cell running")
		}
	}
}

// upRoles is the role list `up` opens: ROLES, with the-desk prepended when a hand-edited
// cell.env dropped it, or removed when --no-the-desk asked for the four loop roles alone.
func (c *Cell) upRoles(noTheDesk bool) []string {
	roles := c.Env.Get("ROLES")
	if !strings.Contains(" "+roles+" ", " the-desk ") {
		roles = "the-desk " + roles
	}
	if noTheDesk {
		roles = strings.ReplaceAll(roles, "the-desk", "")
	}
	return strings.Fields(roles)
}

// herdrTabIDForLabel is the tab_id `herdr tab list` reports for this label, or "". `herdr tab
// close` takes a positional <tab_id> — it advertises no `--label` at all — so closing "the tab
// labelled <cell>-<role>" is a list-then-match, not a one-shot flag.
func herdrTabIDForLabel(label string) string {
	out, err := exec.Command("herdr", "tab", "list").Output()
	if err != nil {
		return ""
	}
	var d struct {
		Result struct {
			Tabs []struct {
				Label string `json:"label"`
				TabID string `json:"tab_id"`
			} `json:"tabs"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &d) != nil {
		return ""
	}
	for _, t := range d.Result.Tabs {
		if t.Label == label {
			return t.TabID
		}
	}
	return ""
}

func (c *Cell) downHerdr() {
	roles := c.upRoles(false)
	if !helpHas("close", "herdr", "tab") {
		fmt.Println("[cell] this herdr build advertises no 'tab close' — close these tabs by hand:")
		for _, r := range append([]string{"cell"}, roles...) {
			fmt.Printf("  %s-%s\n", c.Name, r)
		}
		return
	}
	// The oracle reads `${FIRST_NAME:-cell}` here, and `down` never calls first_window — so
	// FIRST_NAME is unset on this path and the label is always `<cell>-cell`. Spelled as the
	// literal it always is, rather than as a variable that is never set.
	labels := []string{c.Name + "-cell"}
	for _, r := range roles {
		labels = append(labels, c.Name+"-"+r)
	}
	total, closed := 0, 0
	for _, label := range labels {
		total++
		if tid := herdrTabIDForLabel(label); tid != "" {
			if exec.Command("herdr", "tab", "close", tid).Run() == nil {
				closed++
			}
		}
	}
	fmt.Printf("[cell] herdr: closed %d/%d tabs ('herdr tab list' → tab_id by label → 'herdr tab close <tab_id>')\n", closed, total)
}

func (c *Cell) downOrca() {
	roles := c.upRoles(false)
	dir := orDefault(c.Repo, c.Dir)
	// `orca terminal close --worktree <selector> --all` stops every terminal orca owns for that
	// worktree in one call; `up_orca` registers CELL_REPO via `orca repo add` before ever
	// creating a terminal there, so this is a real close whenever the build advertises it.
	h := helpText("orca", "terminal", "close")
	wtflag, hasWT := firstFlag(h, "--worktree")
	allflag, hasAll := firstFlag(h, "--all")
	if hasWT && hasAll {
		if exec.Command("orca", "terminal", "close", wtflag, "path:"+dir, allflag).Run() == nil {
			fmt.Printf("[cell] orca: closed every terminal for %s ('orca terminal close %s path:%s %s')\n", dir, wtflag, dir, allflag)
		} else {
			fmt.Fprintf(os.Stderr, "[cell] 'orca terminal close %s path:%s %s' failed — close the %s-<role> terminals by hand:\n", wtflag, dir, allflag, c.Name)
			for _, r := range append([]string{"cell"}, roles...) {
				fmt.Fprintf(os.Stderr, "  %s-%s\n", c.Name, r)
			}
		}
	} else {
		fmt.Printf("[cell] this orca build advertises no worktree-scoped terminal close — close the %s-<role> terminals by hand ('orca terminal list --worktree path:%s'):\n", c.Name, dir)
		for _, r := range append([]string{"cell"}, roles...) {
			fmt.Printf("  %s-%s\n", c.Name, r)
		}
	}
	// Scheduled automations OUTLIVE a `down` on purpose: `up --automate` is the run-while-away
	// shape, and taking the windows down is not the same act as cancelling the schedule. They
	// are named here so cancelling is one command away.
	if helpHas("delete", "orca", "automations") {
		fmt.Printf("[cell] scheduled automations are NOT removed by down — cancel with: orca automations delete --name %s-<role>\n", c.Name)
	} else {
		fmt.Printf("[cell] scheduled automations are NOT removed by down — cancel the %s-<role> automations in orca\n", c.Name)
	}
}

func isSocket(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode()&os.ModeSocket != 0
}
