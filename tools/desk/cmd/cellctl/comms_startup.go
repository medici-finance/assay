package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
)

func (c *Cell) commsDir() string       { return filepath.Join(c.Dir, "run", "comms") }
func (c *Cell) commsLaunchDir() string { return filepath.Join(c.Dir, "run", "comms-launch") }

// A receipt reserves a single surface while its shell starts. An incomplete
// receipt after partial creation is uncertainty, never permission to duplicate.
type commsSurface struct {
	Cockpit   string `json:"cockpit"`
	Handle    string `json:"handle"`
	Label     string `json:"label"`
	RuntimeID string `json:"runtime_id,omitempty"`
}

func (c *Cell) commsCmdIn(sh paneShell, self string) string {
	root, name := c.reenter()
	return sh.invoke(sh.quote(self)) + " --cells-root " + sh.quote(root) + " comms " + sh.quote(name) + " run"
}

func (c *Cell) commsPreflight() (bool, error) {
	cfg, err := c.loadDeskComms()
	if err != nil || cfg == nil {
		return false, err
	}
	if c.KindOverride != "" && c.Env.GetOr("CELL_KIND", "k8s") != "house" {
		return false, fmt.Errorf("comms requires a persisted house cell kind")
	}
	if _, err := c.commsCommands(); err != nil {
		return false, err
	}
	dirs := []string{cfg.QueueDir}
	if runtime.GOOS != "windows" {
		dirs = append(dirs, filepath.Dir(cfg.Socket))
	}
	for _, dir := range dirs {
		if _, err := os.Lstat(dir); err == nil {
			if err := privateCommsDir(dir); err != nil {
				return false, err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	if _, err := os.Lstat(c.commsLaunchDir()); err == nil {
		if err := c.checkCommsLaunchDir(); err != nil {
			return false, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	owned := false
	if _, err := os.Lstat(c.commsDir()); err == nil {
		var err error
		owned, err = c.commsOwned()
		if err != nil {
			return false, err
		}
	}
	if runtime.GOOS != "windows" && !owned {
		if _, err := os.Lstat(cfg.Socket); err == nil {
			return false, fmt.Errorf("comms endpoint already exists; inspect the prior gateway before restarting")
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return true, nil
}

func (c *Cell) commsState() (cellcadence.State, error) {
	s, err := cellcadence.Read(c.commsDir())
	if err == nil && (s.Cell != c.Name || s.Role != "comms") {
		return s, fmt.Errorf("foreign comms checkpoint scope")
	}
	return s, err
}

// A lease establishes ownership, never a PID or a recent timestamp. An
// unfinished checkpoint without a live lease requires explicit recovery.
func (c *Cell) commsOwned() (bool, error) {
	lease, err := cellcadence.Acquire(c.commsDir())
	if err != nil && !errors.Is(err, cellcadence.ErrBusy) {
		return false, err
	}
	if lease != nil {
		defer lease.Close()
	}
	s, readErr := c.commsState()
	if readErr != nil && !errors.Is(readErr, cellcadence.ErrNoState) {
		return false, readErr
	}
	if errors.Is(err, cellcadence.ErrBusy) {
		if readErr != nil || !s.Running {
			return false, fmt.Errorf("comms owner is starting or stopping: %w", cellcadence.ErrBusy)
		}
		return true, nil
	}
	if s.Running {
		return false, cellcadence.ErrUnfinished
	}
	return false, nil
}

func (c *Cell) checkCommsLaunchDir() error {
	if err := privateCommsDir(c.commsLaunchDir()); err != nil {
		return err
	}
	return commsLaunchOwner(c.commsLaunchDir())
}

func (c *Cell) acquireCommsLaunch() (*cellcadence.Lease, error) {
	if err := os.MkdirAll(c.commsLaunchDir(), 0700); err != nil {
		return nil, err
	}
	if err := c.checkCommsLaunchDir(); err != nil {
		return nil, err
	}
	return cellcadence.Acquire(c.commsLaunchDir())
}

func (c *Cell) readCommsSurface() (*commsSurface, error) {
	if err := c.checkCommsLaunchDir(); err != nil {
		return nil, err
	}
	p := filepath.Join(c.commsLaunchDir(), "surface.json")
	if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err := privateCommsFile(p); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s commsSurface
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if !valueIn(s.Cockpit, []string{"tmux", "herdr", "orca"}) {
		return nil, fmt.Errorf("invalid comms surface cockpit")
	}
	return &s, nil
}

// Called once by up before roles, never by a cadence pass.
func (c *Cell) startComms(cockpit string) error {
	launch, err := c.acquireCommsLaunch()
	if err != nil {
		return fmt.Errorf("comms startup already in progress: %w", err)
	}
	defer launch.Close()
	owned, err := c.commsOwned()
	if err != nil {
		return err
	}
	if owned {
		fmt.Println("[comms] reusing the running service")
		return nil
	}
	surface, err := c.readCommsSurface()
	if err != nil {
		return err
	}
	if surface != nil {
		return fmt.Errorf("comms surface already reserved; run cellctl down %s before retrying", c.Name)
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	surface = &commsSurface{Cockpit: cockpit, Label: c.Name + "-comms-" + hex.EncodeToString(nonce[:])}
	p := filepath.Join(c.commsLaunchDir(), "surface.json")
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err := c.writeCommsSurface(f, surface); err != nil {
		return errors.Join(err, f.Close())
	}
	launchErr := c.openCommsSurface(surface)
	writeErr := c.writeCommsSurface(f, surface)
	if err := errors.Join(launchErr, writeErr, f.Close()); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		owned, err := c.commsOwned()
		if owned {
			break
		}
		if err != nil && !errors.Is(err, cellcadence.ErrBusy) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("comms surface opened but supervisor ownership is unconfirmed; inspect before retrying")
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("[comms] opened %s-comms in %s\n", c.Name, cockpit)
	return nil
}

func (c *Cell) writeCommsSurface(f *os.File, surface *commsSurface) error {
	if err := c.checkCommsLaunchDir(); err != nil {
		return err
	}
	if err := privateCommsFile(f.Name()); err != nil {
		return err
	}
	opened, err := f.Stat()
	if err != nil {
		return err
	}
	named, err := os.Lstat(f.Name())
	if err != nil {
		return err
	}
	if !os.SameFile(opened, named) {
		return fmt.Errorf("comms launch receipt changed; inspect before retrying")
	}
	b, err := json.Marshal(surface)
	if err != nil {
		return err
	}
	if _, err := f.WriteAt(b, 0); err != nil {
		return err
	}
	if err := f.Truncate(int64(len(b))); err != nil {
		return err
	}
	return f.Sync()
}

func (c *Cell) openCommsSurface(surface *commsSurface) error {
	cockpit := surface.Cockpit
	cmd := c.commsCmdIn(paneShellFor(runtime.GOOS, cockpit), selfPath())
	label := surface.Label
	switch cockpit {
	case "tmux":
		out, err := exec.Command("tmux", "new-window", "-P", "-F", "#{window_id}", "-t", c.Session, "-n", label, "-c", c.Dir, cmd).Output()
		if err != nil {
			return fmt.Errorf("comms tmux launch: %w", err)
		}
		handle := strings.TrimSpace(string(out))
		if !strings.HasPrefix(handle, "@") {
			return fmt.Errorf("tmux comms create returned no window identity")
		}
		surface.Handle = handle
		return nil
	case "herdr":
		if !helpHas("create", "herdr", "tab") || !helpHas("run", "herdr", "pane") {
			return fmt.Errorf("herdr cannot create and run a comms tab")
		}
		ws := herdrWindowID()
		defaultTab := ""
		if ws == "" {
			ws, defaultTab = c.herdrStartWindow(label)
		}
		out, err := exec.Command("herdr", "tab", "create", "--workspace", ws, "--label", label).Output()
		if err != nil {
			return err
		}
		var d struct {
			Result struct {
				Tab struct {
					TabID string `json:"tab_id"`
				} `json:"tab"`
				RootPane struct {
					PaneID string `json:"pane_id"`
				} `json:"root_pane"`
			} `json:"result"`
		}
		if json.Unmarshal(out, &d) != nil || d.Result.Tab.TabID == "" || d.Result.RootPane.PaneID == "" {
			return fmt.Errorf("herdr comms create returned no tab/pane identity; inspect %s", label)
		}
		surface.Handle = d.Result.Tab.TabID
		if err := exec.Command("herdr", "pane", "run", d.Result.RootPane.PaneID, cmd).Run(); err != nil {
			return err
		}
		if defaultTab != "" {
			_ = exec.Command("herdr", "tab", "close", defaultTab).Run()
		}
		return nil
	case "orca":
		return c.openOrcaComms(cmd, surface)
	}
	return fmt.Errorf("unsupported comms cockpit %q", cockpit)
}

func (c *Cell) stopComms() error {
	// No manifest lookup: disabling config must not orphan a running service.
	if _, err := os.Lstat(c.commsDir()); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	launch, err := c.acquireCommsLaunch()
	if err != nil {
		return err
	}
	defer launch.Close()
	surface, err := c.readCommsSurface()
	if err != nil {
		return err
	}
	owned, err := c.commsOwned()
	if err != nil {
		return err
	}
	if !owned && surface != nil {
		if _, err := c.commsState(); errors.Is(err, cellcadence.ErrNoState) {
			return fmt.Errorf("comms surface has not confirmed supervisor startup; inspect it before recovery")
		}
	}
	if owned {
		f, err := os.OpenFile(filepath.Join(c.commsDir(), "STOP"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		if f != nil {
			if err := f.Close(); err != nil {
				return err
			}
		}
		deadline := time.Now().Add(20 * time.Second)
		for {
			owned, err = c.commsOwned()
			if err != nil && !errors.Is(err, cellcadence.ErrBusy) {
				return err
			}
			if !owned && err == nil {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("comms has not stopped; cockpit left open for inspection")
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	if state, err := c.commsState(); err != nil && !errors.Is(err, cellcadence.ErrNoState) {
		return err
	} else if err == nil && state.Outcome != "ok" {
		return fmt.Errorf("comms last shutdown failed; inspect the retained checkpoint before recovery")
	}
	if surface == nil {
		return nil
	}
	if surface.Handle == "" {
		return fmt.Errorf("comms terminal identity is unknown; close the %s-comms surface and inspect before removing its launch receipt", c.Name)
	}
	err = c.closeCommsSurface(surface)
	if err != nil {
		return fmt.Errorf("comms surface close failed: %w", err)
	}
	return os.Remove(filepath.Join(c.commsLaunchDir(), "surface.json"))
}

func (c *Cell) runCommsUntilStop(ctx context.Context, commands [][]string, env []string) cellcadence.Result {
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := make(chan cellcadence.Result, 1)
	go func() { finished <- runCommsPair(child, commands, env) }()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	requested := false
	var stopErr error
	for {
		select {
		case result := <-finished:
			if requested && !result.Uncertain && errors.Is(result.Err, context.Canceled) && commsCancelOnly(result.Err) {
				result.Err, result.ExitCode = nil, 0
			}
			result.Err = errors.Join(result.Err, stopErr)
			return result
		case <-tick.C:
			if _, err := os.Lstat(filepath.Join(c.commsDir(), "STOP")); err == nil {
				requested = true
				cancel()
			} else if !errors.Is(err, os.ErrNotExist) {
				stopErr = err
				cancel()
			}
		}
	}
}

func commsCancelOnly(err error) bool {
	if err == nil || err == context.Canceled {
		return true
	}
	if es, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range es.Unwrap() {
			if !commsCancelOnly(e) {
				return false
			}
		}
		return true
	}
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode() == -1 || (runtime.GOOS == "windows" && e.ExitCode() == 1)
	}
	return false
}

func (c *Cell) openOrcaComms(cmd string, surface *commsSurface) error {
	label := surface.Label
	verb, ok := orcaTerminalVerb()
	if !ok {
		return fmt.Errorf("orca has no terminal create verb")
	}
	h := helpText("orca", "terminal", verb)
	commandFlag, commandOK := firstFlag(h, "--command", "--cmd", "--exec", "--run")
	nameFlag, nameOK := firstFlag(h, "--name", "--title", "--label")
	_, jsonOK := firstFlag(h, "--json")
	_, closeOK := firstFlag(helpText("orca", "terminal", "close"), "--terminal")
	if !commandOK || !nameOK || !jsonOK || !closeOK {
		return fmt.Errorf("orca comms needs named JSON terminal creation and single-terminal close")
	}
	dir := orDefault(c.Repo, c.Dir)
	args := []string{"terminal", verb}
	if flag, ok := firstFlag(h, "--worktree"); ok {
		if helpHas("add", "orca", "repo") {
			if err := exec.Command("orca", "repo", "add", "--path", dir).Run(); err != nil {
				return err
			}
		}
		args = append(args, flag, "path:"+dir)
	} else if flag, ok := firstFlag(h, "--cwd", "--path", "--directory"); ok {
		args = append(args, flag, dir)
	}
	if runtime.GOOS == "windows" {
		if flag, ok := firstFlag(h, "--shell"); ok {
			args = append(args, flag, "powershell")
		} else {
			return fmt.Errorf("orca cannot select the PowerShell shell required by the comms command")
		}
	}
	args = append(args, nameFlag, label, commandFlag, cmd, "--json")
	out, err := exec.Command("orca", args...).Output()
	if err != nil {
		return err
	}
	id, err := readOrcaComms(out)
	if err != nil {
		return err
	}
	surface.Handle, surface.RuntimeID = id.Result.Terminal.Handle, id.Meta.RuntimeID
	return nil
}

type orcaCommsReply struct {
	OK     bool `json:"ok"`
	Result struct {
		Terminal struct {
			Handle string `json:"handle"`
			Title  string `json:"title"`
		} `json:"terminal"`
	} `json:"result"`
	Meta struct {
		RuntimeID string `json:"runtimeId"`
	} `json:"_meta"`
}

func readOrcaComms(out []byte) (orcaCommsReply, error) {
	var d orcaCommsReply
	if json.Unmarshal(out, &d) != nil || !d.OK || d.Result.Terminal.Handle == "" || d.Meta.RuntimeID == "" {
		return d, fmt.Errorf("orca returned no successful terminal/runtime identity")
	}
	return d, nil
}

func (c *Cell) closeCommsSurface(s *commsSurface) error {
	switch s.Cockpit {
	case "tmux":
		out, err := exec.Command("tmux", "list-windows", "-a", "-F", "#{window_id}\t#{window_name}").Output()
		if err != nil {
			return fmt.Errorf("cannot inspect comms tmux window: %w", err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			id, name, ok := strings.Cut(line, "\t")
			if id != s.Handle {
				continue
			}
			if !ok || name != s.Label {
				return fmt.Errorf("comms tmux window identity changed")
			}
			return exec.Command("tmux", "kill-window", "-t", s.Handle).Run()
		}
		return nil // an enumerated absence, not a failed inspection
	case "herdr":
		out, err := exec.Command("herdr", "tab", "list").Output()
		if err != nil {
			return fmt.Errorf("cannot inspect comms herdr tab: %w", err)
		}
		var d struct {
			Result struct {
				Tabs *[]struct {
					ID    string `json:"tab_id"`
					Label string `json:"label"`
				} `json:"tabs"`
			} `json:"result"`
		}
		if json.Unmarshal(out, &d) != nil || d.Result.Tabs == nil {
			return fmt.Errorf("cannot inspect comms herdr tab list")
		}
		for _, tab := range *d.Result.Tabs {
			if tab.ID != s.Handle {
				continue
			}
			if tab.Label != s.Label {
				return fmt.Errorf("comms herdr tab identity changed")
			}
			return exec.Command("herdr", "tab", "close", s.Handle).Run()
		}
		return nil
	case "orca":
		out, err := exec.Command("orca", "terminal", "show", "--terminal", s.Handle, "--json").Output()
		if err != nil {
			return fmt.Errorf("cannot inspect comms orca terminal: %w", err)
		}
		observed, err := readOrcaComms(out)
		if err != nil {
			return err
		}
		if observed.Meta.RuntimeID != s.RuntimeID || observed.Result.Terminal.Handle != s.Handle || observed.Result.Terminal.Title != s.Label {
			return fmt.Errorf("comms orca terminal identity changed")
		}
		return exec.Command("orca", "terminal", "close", "--terminal", s.Handle).Run()
	}
	return fmt.Errorf("unsupported comms surface")
}
