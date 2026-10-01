package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/cellcontainer"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func (c *Cell) nativeContainerPlan(role, harness, model string) *cellcontainer.Plan {
	cfg, err := cellcontainer.Load(c.Env.Get("CELL_CONTAINER_CONFIG"))
	if err != nil {
		die("container config: %v", err)
	}
	key, err := cfg.RoleKey(c.Name, role)
	if err != nil {
		die("%v", err)
	}
	p, err := cfg.Plan(c.Name, key, harness, model)
	if err != nil {
		die("%v", err)
	}
	return p
}
func (c *Cell) nativeContainerPlans() []*cellcontainer.Plan {
	var plans []*cellcontainer.Plan
	for _, role := range c.Roles {
		rm := c.resolveRoleModel(role, c.Harness)
		if !rm.OK {
			die("%s", rm.Src)
		}
		plans = append(plans, c.nativeContainerPlan(role, c.Harness, rm.Model))
	}
	return plans
}
func (c *Cell) nativeContainer(args ...string) {
	verb := args[0]
	var plans []*cellcontainer.Plan
	if verb == "desk" {
		if len(args) != 6 || args[2] != "--harness" || args[4] != "--model" {
			die("invalid native container desk arguments")
		}
		plans = []*cellcontainer.Plan{c.nativeContainerPlan(args[1], args[3], args[5])}
	} else {
		plans = c.nativeContainerPlans()
	}
	for _, p := range plans {
		if c.Env.Get("DRY_RUN") == "1" {
			fmt.Printf("[dry-run] native container %s target=%s name=%s harness=%s model=%s\n", verb, p.Host, p.Name, p.Harness, p.Model)
			if verb == "desk" {
				for _, a := range append([]string{"docker", "--host", p.Host}, p.RunArgs()...) {
					fmt.Printf("%s ", bashQuote(a))
				}
				fmt.Println()
			}
			continue
		}
		fmt.Printf("[container] %s target=%s name=%s\n", verb, p.Host, p.Name)
		e := cellcontainer.Docker(p.Host)
		switch verb {
		case "desk":
			c.nativeContainerUp(p, args[1])
		case "check":
			if err := e.Check(p); err != nil {
				die("container check: %v", err)
			}
			fmt.Printf("[check] %s ready (no model or forge calls)\n", p.Name)
		case "status":
			s, err := e.Inspect(p)
			if err != nil {
				die("container status: %v", err)
			}
			if s == nil {
				fmt.Printf("%s: stopped\n", p.Name)
				continue
			}
			if err = p.Validate(s); err != nil {
				die("container status: %v", err)
			}
			fmt.Printf("%s: %s (%s)\n", p.Name, s.State.Status, s.ID)
		case "down":
			s, err := e.Inspect(p)
			if err != nil {
				die("container down: %v", err)
			}
			if s != nil {
				if err = p.Validate(s); err != nil {
					die("container down: %v", err)
				}
				if _, err = e.Output("stop", s.ID); err != nil {
					die("container down: %v", err)
				}
			}
			// Stop the verified container before removing its console. Volumes are never removed.
			socket, session := c.nativeConsole(p)
			if isSocket(socket) {
				out, err := exec.Command("tmux", "-S", socket, "kill-session", "-t", session).CombinedOutput()
				if err != nil {
					fmt.Fprintf(os.Stderr, "[container] console cleanup: %s\n", strings.TrimSpace(string(out)))
				}
			}
			fmt.Printf("%s: stopped; workspace volume retained\n", p.Name)
		default:
			die("unsupported native container operation %s", verb)
		}
	}
}
func (c *Cell) nativeConsole(p *cellcontainer.Plan) (string, string) {
	return filepath.Join(c.Dir, "run", "container.sock"), c.Name + "-" + p.RoleKey
}
func (c *Cell) nativeContainerUp(p *cellcontainer.Plan, role string) {
	if !onPath("tmux") {
		die("native container console requires tmux")
	}
	e := cellcontainer.Docker(p.Host)
	// Do all fallible state checks in the visible caller as well as in the runner.
	if err := e.Check(p); err != nil {
		die("container preflight: %v", err)
	}
	socket, session := c.nativeConsole(p)
	mustMkdirAll(filepath.Dir(socket))
	// Serialize console creation independently of the long-lived host/session lock.
	lock := containerLock(filepath.Join(c.Dir, "run", "console.lock"))
	defer lock.Close()
	tmux := func(args ...string) ([]byte, error) {
		return exec.Command("tmux", append([]string{"-f", "/dev/null", "-S", socket}, args...)...).CombinedOutput()
	}
	state, err := tmux("list-panes", "-t", session, "-F", "#{pane_dead}")
	alive := err == nil && strings.TrimSpace(string(state)) == "0"
	if !alive {
		if err == nil {
			// Retain diagnostics until the next explicit up. A dead pane is never a live console.
			if out, err := tmux("kill-session", "-t", session); err != nil {
				die("remove dead console: %s", out)
			}
		} else if isSocket(socket) && !strings.Contains(string(state), "no server running") && !strings.Contains(string(state), "can't find") && !strings.Contains(string(state), "no sessions") && !strings.Contains(string(state), "No such file") {
			die("inspect container console: %s", strings.TrimSpace(string(state)))
		}
		argv := []string{selfPath(), "container-run", c.Name, role, p.Harness, p.Model}
		// The runner gets the same cell root even when the tmux server was started elsewhere.
		command := "env CELLS_ROOT=" + bashQuote(filepath.Dir(c.Dir)) + " "
		for _, a := range argv {
			command += bashQuote(a) + " "
		}
		// Start a waiting pane first: remain-on-exit is installed before its process can fail.
		if out, err := tmux("new-session", "-d", "-s", session, "-c", c.Dir, "sleep 86400"); err != nil {
			die("create container console: %s", out)
		}
		if out, err := tmux("set-option", "-w", "-t", session, "remain-on-exit", "on"); err != nil {
			die("retain container errors: %s", out)
		}
		if out, err := tmux("respawn-pane", "-k", "-t", session, command); err != nil {
			die("start container console: %s", out)
		}
	}
	// Release only the short creation lock before interactive attach.
	_ = lock.Close()
	if isTTY(os.Stdin) {
		runForeground([]string{"tmux", "-S", socket, "attach-session", "-t", session}, os.Environ(), "")
	} else {
		fmt.Printf("[container] console ready; attach with cellctl desk %s %s\n", c.Name, role)
	}
}
func containerLock(path string) *os.File {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		die("container session lock: %v", err)
	}
	if err = deskkit.TryLockExclusive(f); err != nil {
		_ = f.Close()
		die("container session lock: %v (another session may be active)", err)
	}
	return f
}

// cmdContainerRun is the Go process held in the private tmux console. It owns
// the lifetime lock and rechecks Docker state after acquiring it, including recovery.
func cmdContainerRun(args []string) {
	if len(args) != 4 {
		die("container-run is an internal console entrypoint")
	}
	c := loadCell(args[0])
	if c.Kind != "container" || c.Env.Get("CELL_CONTAINER_CONFIG") == "" {
		die("container-run requires native container configuration")
	}
	if !valueIn(args[1], c.Roles) {
		die("role is not enabled")
	}
	p := c.nativeContainerPlan(args[1], args[2], args[3])
	path := p.HostLock
	if path == "" {
		path = filepath.Join(c.Dir, "run", p.RoleKey+".lock")
	}
	lock := containerLock(path)
	defer lock.Close()
	fmt.Printf("[container] run target=%s name=%s\n", p.Host, p.Name)
	if err := cellcontainer.Docker(p.Host).Run(p); err != nil {
		die("container run: %v", err)
	}
}

func newNativeContainer(root, name, repo, config, roles, roots string) {
	cfg, err := cellcontainer.Load(config)
	if err != nil {
		die("new: %v", err)
	}
	cell, ok := cfg.Cells[name]
	if !ok {
		die("new: cell is absent from container config")
	}
	if repo != "" && repo != cell.Repo {
		die("new: --repo differs from container config")
	}
	for _, role := range strings.Fields(roles) {
		if _, err = cfg.RoleKey(name, role); err != nil {
			die("new: %v", err)
		}
	}
	d := filepath.Join(root, name)
	if exists(d) {
		die("%s already exists (cellctl new never overwrites a cell)", d)
	}
	// One registry harness today; mixed role harnesses use explicit desk --harness.
	key, err := cfg.RoleKey(name, strings.Fields(roles)[0])
	if err != nil {
		die("new: %v", err)
	}
	harness := cell.Roles[key].Harness
	body := "# Native container cell; lifecycle is implemented by Go cellctl.\n" +
		"CELL=" + bashQuote(name) + "\nCELL_KIND=container\nCELL_REPO=" + bashQuote(cell.Repo) + "\nCELL_CONTAINER_CONFIG=" + bashQuote(config) + "\nROLES=" + bashQuote(roles) + "\nCELL_ROOTS=" + bashQuote(roots) + "\nCELL_HARNESS=" + harness + "\nDESKD=0\n"
	for _, role := range strings.Fields(roles) {
		key, _ := cfg.RoleKey(name, role)
		r := cell.Roles[key]
		if r.Harness != harness {
			die("new: configured roles use different harnesses; register separately or use explicit desk overrides")
		}
		model := r.Models[harness]
		if model == "" {
			die("new: each registered role needs its harness model pin")
		}
		if role == "the-desk" && harness == "claude" {
			refuseOpusForTheDesk(model)
		}
		prefix := "DESK_MODEL_"
		if harness == "codex" {
			prefix = "CODEX_MODEL_"
		}
		body += prefix + underscore(role) + "=" + bashQuote(model) + "\n"
	}
	mustMkdirAll(root)
	if err = os.Mkdir(d, 0700); err != nil {
		die("new: %v", err)
	}
	if err = os.WriteFile(filepath.Join(d, "cell.env"), []byte(body), 0600); err != nil {
		die("new: %v", err)
	}
	fmt.Printf("[new] registered native container %s — cellctl check %s, then cellctl up %s\n", name, name, name)
}
