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

// nativeEngine is the Docker engine constructor, replaced in tests.
var nativeEngine = cellcontainer.Docker

func (c *Cell) nativeContainerConfig() *cellcontainer.Config {
	cfg, err := cellcontainer.Load(c.Env.Get("CELL_CONTAINER_CONFIG"))
	if err != nil {
		die("container config: %v", err)
	}
	return cfg
}
func (c *Cell) nativeContainerPlan(role, harness, model string) *cellcontainer.Plan {
	// This also protects the internal runner, which can be invoked without desk.
	policy, _, policyErr := c.cellModelPolicy()
	if policyErr != nil {
		die("%v", policyErr)
	}
	if policy != nil || c.Env.Get("CELL_PROVIDER") != "" {
		die("native containers do not accept host model policies or providers")
	}
	cfg := c.nativeContainerConfig()
	key, err := cfg.RoleKey(c.Name, role)
	if err != nil {
		die("%v", err)
	}
	p, err := cfg.Plan(c.Name, key, harness, model)
	if err != nil {
		die("%v", err)
	}
	// The cell directory holds cell.env and the console socket; CheckFiles refuses a directory
	// mount that is, or contains, it (and so also the cells root above it).
	p.Protected = append(p.Protected, c.Dir)
	if role == "the-desk" && p.Harness == "claude" {
		refuseOpusForTheDesk(p.Model)
	}
	return p
}
func (c *Cell) nativeContainer(args ...string) {
	switch verb := args[0]; verb {
	case "desk":
		if len(args) != 6 || args[2] != "--harness" || args[4] != "--model" {
			die("invalid native container desk arguments")
		}
		p := c.nativeContainerPlan(args[1], args[3], args[5])
		if c.Env.Get("DRY_RUN") == "1" {
			fmt.Printf("[dry-run] native container %s target=%s name=%s harness=%s model=%s\n", verb, p.Host, p.Name, p.Harness, p.Model)
			for _, a := range append([]string{"docker", "--host", p.Host}, p.RunArgs()...) {
				fmt.Printf("%s ", bashQuote(a))
			}
			fmt.Println()
			return
		}
		fmt.Printf("[container] %s target=%s name=%s\n", verb, p.Host, p.Name)
		c.nativeContainerUp(p, args[1])
	case "check":
		// check is the preflight for up, so it plans from the registered pins.
		var failed []string
		for _, role := range c.Roles {
			rm := c.resolveRoleModel(role, c.Harness)
			if !rm.OK {
				die("%s", rm.Src)
			}
			p := c.nativeContainerPlan(role, c.Harness, rm.Model)
			if c.Env.Get("DRY_RUN") == "1" {
				fmt.Printf("[dry-run] native container %s target=%s name=%s harness=%s model=%s\n", verb, p.Host, p.Name, p.Harness, p.Model)
				continue
			}
			fmt.Printf("[container] %s target=%s name=%s\n", verb, p.Host, p.Name)
			if err := nativeEngine(p.Host).Check(p); err != nil {
				fmt.Fprintf(os.Stderr, "cellctl: container check: %v\n", err)
				failed = append(failed, p.Name)
				continue
			}
			fmt.Printf("[check] %s ready (no model or forge calls)\n", p.Name)
		}
		if len(failed) > 0 {
			die("container check failed for %s", strings.Join(failed, ", "))
		}
	case "status", "down":
		c.nativeLifecycle(verb)
	default:
		die("unsupported native container operation %s", verb)
	}
}

// nativeLifecycle reports or stops what is actually running for every enabled role. Each
// container is validated against the harness and model it was launched with, so a per-launch
// override never strands it, and a refusal for one role never skips the others.
func (c *Cell) nativeLifecycle(verb string) {
	cfg := c.nativeContainerConfig()
	var failed []string
	for _, role := range c.Roles {
		key, err := cfg.RoleKey(c.Name, role)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellctl: container %s: %v\n", verb, err)
			failed = append(failed, role)
			continue
		}
		name := cellcontainer.ContainerName(c.Name, key)
		if c.Env.Get("DRY_RUN") == "1" {
			fmt.Printf("[dry-run] native container %s target=%s name=%s\n", verb, cfg.DockerHost, name)
			continue
		}
		fmt.Printf("[container] %s target=%s name=%s\n", verb, cfg.DockerHost, name)
		if err = c.nativeLifecycleRole(nativeEngine(cfg.DockerHost), cfg, verb, key); err != nil {
			fmt.Fprintf(os.Stderr, "cellctl: container %s: %v\n", verb, err)
			failed = append(failed, name)
		}
	}
	if len(failed) > 0 {
		die("container %s refused for %s", verb, strings.Join(failed, ", "))
	}
}
func (c *Cell) nativeLifecycleRole(e cellcontainer.Engine, cfg *cellcontainer.Config, verb, key string) error {
	name := cellcontainer.ContainerName(c.Name, key)
	p, s, err := cfg.Running(e, c.Name, key)
	if err != nil {
		return err
	}
	if verb == "status" {
		if s == nil {
			fmt.Printf("%s: stopped\n", name)
		} else {
			fmt.Printf("%s: %s (%s) harness=%s model=%s\n", name, s.State.Status, s.ID, p.Harness, p.Model)
		}
		return nil
	}
	// Stop the verified container before removing its console. Volumes are never removed.
	if s != nil {
		if _, err = e.Output("stop", s.ID); err != nil {
			return err
		}
	}
	socket, session := c.nativeConsole(key)
	if isSocket(socket) {
		out, err := exec.Command("tmux", "-S", socket, "kill-session", "-t", session).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[container] console cleanup: %s\n", strings.TrimSpace(string(out)))
		}
	}
	fmt.Printf("%s: stopped; workspace volume retained\n", name)
	return nil
}
func (c *Cell) nativeConsole(key string) (string, string) {
	return filepath.Join(c.Dir, "run", "container.sock"), c.Name + "-" + key
}

// nativeConsoleArgv is the runner command for tmux. It is passed as separate arguments, so tmux
// executes it directly and no shell (the operator's default-shell included) parses a value.
func (c *Cell) nativeConsoleArgv(p *cellcontainer.Plan, role string) []string {
	// The runner gets the same cell root even when the tmux server was started elsewhere.
	return []string{"env", "CELLS_ROOT=" + filepath.Dir(c.Dir), selfPath(), "container-run", c.Name, role, p.Harness, p.Model}
}
func (c *Cell) nativeContainerUp(p *cellcontainer.Plan, role string) {
	if !onPath("tmux") {
		die("native container console requires tmux")
	}
	e := nativeEngine(p.Host)
	// Do all fallible state checks in the visible caller as well as in the runner.
	if err := e.Check(p); err != nil {
		die("container preflight: %v", err)
	}
	socket, session := c.nativeConsole(p.RoleKey)
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
		// Start a waiting pane first: remain-on-exit is installed before its process can fail.
		if out, err := tmux("new-session", "-d", "-s", session, "-c", c.Dir, "sleep 86400"); err != nil {
			die("create container console: %s", out)
		}
		if out, err := tmux("set-option", "-w", "-t", session, "remain-on-exit", "on"); err != nil {
			die("retain container errors: %s", out)
		}
		if out, err := tmux(append([]string{"respawn-pane", "-k", "-t", session}, c.nativeConsoleArgv(p, role)...)...); err != nil {
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
	if err := nativeEngine(p.Host).Run(p); err != nil {
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
