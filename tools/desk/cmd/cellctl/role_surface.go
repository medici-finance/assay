package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The per-role surface receipt (desk-containers/18): what `up` launched for one role, so a
// per-role `down <cell> <role...>` can close exactly that one window, tab or terminal and never
// a sibling's or another cell's. It is a work-safety check against a stale receipt closing the
// wrong window. A process under the same account could close any window directly, so it is not
// an authorization control.

const surfaceReceiptSchema = "cellctl-surface-receipt-v1"

type surfaceReceipt struct {
	Schema    string `json:"schema"`
	Cell      string `json:"cell"`
	Role      string `json:"role"`
	Cockpit   string `json:"cockpit"`             // tmux | herdr | orca
	Session   string `json:"session,omitempty"`   // tmux: the cell's session
	Workspace string `json:"workspace,omitempty"` // herdr: the cell's workspace id
	Runtime   string `json:"runtime,omitempty"`   // orca: the runtime id
	Surface   string `json:"surface"`             // tmux window id, herdr tab id, orca terminal handle
	Pane      string `json:"pane,omitempty"`      // herdr: the tab's root pane
	Name      string `json:"name"`                // <cell>-<role>
	// HarnessPID / HarnessStart are the role-lease holder: the process `cellctl desk` runs as in
	// the window, which supervises the harness (and, interactive, its child).
	HarnessPID   int          `json:"harness_pid"`
	HarnessStart string       `json:"harness_start"`
	Helpers      []helperProc `json:"helpers,omitempty"` // long-lived children present at launch
	Written      string       `json:"written"`
}

type helperProc struct {
	PID   int    `json:"pid"`
	Start string `json:"start"`
}

func surfaceReceiptPath(dir string) string { return filepath.Join(dir, "surface.json") }

func surfaceName(cell, role string) string { return cell + "-" + role }

func surfaceKind(cockpit string) string {
	switch cockpit {
	case "tmux":
		return "window"
	case "herdr":
		return "tab"
	}
	return "terminal"
}

// writeSurfaceReceipt replaces the role's receipt atomically (0600, exclusive temporary name,
// no symlink followed, 0700 directory).
func writeSurfaceReceipt(dir string, rec surfaceReceipt) error {
	rec.Schema = surfaceReceiptSchema
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateFile(surfaceReceiptPath(dir), append(b, '\n'))
}

// readSurfaceReceipt is (nil, nil) when the role has no receipt; anything else unreadable is an
// error, never "no receipt".
func readSurfaceReceipt(dir string) (*surfaceReceipt, error) {
	p := surfaceReceiptPath(dir)
	st, err := os.Lstat(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", p)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var rec surfaceReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	if rec.Schema != surfaceReceiptSchema {
		return nil, fmt.Errorf("%s: schema %q", p, rec.Schema)
	}
	return &rec, nil
}

func removeSurfaceReceipt(dir string) error {
	err := os.Remove(surfaceReceiptPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// removeAllSurfaceReceipts is cell-wide `down`'s part: every role's receipt goes with the
// session.
func (c *Cell) removeAllSurfaceReceipts() {
	for _, role := range knownRoles {
		if err := removeSurfaceReceipt(c.cadenceDir(role)); err != nil {
			fmt.Fprintf(os.Stderr, "NOTICE: removing %s's surface receipt: %v\n", role, err)
		}
	}
}

// --- binding at launch ---------------------------------------------------------------------

// noteSurface records what a cockpit just opened for role; bindSurface completes it into the
// receipt once the role's lease confirms the harness started.
func (c *Cell) noteSurface(role string, rec surfaceReceipt) {
	if c.pendingSurfaces == nil {
		c.pendingSurfaces = map[string]surfaceReceipt{}
	}
	rec.Cell, rec.Role, rec.Name = c.Name, role, surfaceName(c.Name, role)
	c.pendingSurfaces[role] = rec
}

// bindSurface writes role's receipt from the surface noted at open and the lease holder now
// running in it. A role whose surface identity was not returned by the cockpit gets no receipt,
// and its stale receipt is removed: per-role close then refuses per-role-close-unsupported.
func (c *Cell) bindSurface(role string) {
	dir := c.cadenceDir(role)
	rec, ok := c.pendingSurfaces[role]
	delete(c.pendingSurfaces, role)
	if !ok || rec.Surface == "" {
		_ = removeSurfaceReceipt(dir)
		return
	}
	h, err := cellcadence.Holder(dir)
	if err != nil || h == nil || h.PID <= 0 || h.Start == "" {
		_ = removeSurfaceReceipt(dir)
		fmt.Fprintf(os.Stderr, "NOTICE: %s: no lease holder identity to bind its surface receipt; per-role down will refuse\n", role)
		return
	}
	rec.HarnessPID, rec.HarnessStart = h.PID, h.Start
	rec.Written = cellop.Stamp(time.Now())
	procs := psTable{}
	if desc, err := procs.Descendants(h.PID); err == nil {
		for _, pid := range desc {
			if pid == h.ChildPID {
				continue
			}
			if start, gone, err := procs.Start(pid); err == nil && !gone {
				rec.Helpers = append(rec.Helpers, helperProc{PID: pid, Start: start})
			}
		}
	}
	if err := writeSurfaceReceipt(dir, rec); err != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: %s: writing its surface receipt: %v; per-role down will refuse\n", role, err)
	}
}

// --- the process table ---------------------------------------------------------------------

type procTable interface {
	// Start is the process's start time in deskkit.ProcessStart's form; gone when it has exited.
	Start(pid int) (start string, gone bool, err error)
	// Descendants is pid's whole subtree, pid itself excluded.
	Descendants(pid int) ([]int, error)
}

type psTable struct{}

func (psTable) Start(pid int) (string, bool, error) { return deskkit.ProcessStart(pid) }

func (psTable) Descendants(pid int) ([]int, error) {
	if runtime.GOOS == "windows" {
		return nil, errors.New("the process tree cannot be read on windows")
	}
	out, err := exec.Command("ps", "-axo", "pid=,ppid=").Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}
	return descendantsOf(string(out), pid)
}

// descendantsOf walks a `ps -o pid=,ppid=` listing for root's subtree.
func descendantsOf(listing string, root int) ([]int, error) {
	kids := map[int][]int{}
	for _, line := range strings.Split(listing, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 2 {
			return nil, fmt.Errorf("ps line %q", line)
		}
		pid, err1 := strconv.Atoi(f[0])
		ppid, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("ps line %q", line)
		}
		kids[ppid] = append(kids[ppid], pid)
	}
	var out []int
	seen := map[int]bool{root: true}
	queue := []int{root}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, k := range kids[p] {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
				queue = append(queue, k)
			}
		}
	}
	return out, nil
}

// --- the cockpit: live identity and the single-surface close -------------------------------

// liveSurface is what the cockpit reports now for a receipt's surface id.
type liveSurface struct {
	Found     bool
	Session   string // tmux
	Workspace string // herdr
	Runtime   string // orca
	Name      string
	// Roots are the processes the surface runs (tmux pane pid, herdr shell and foreground
	// pids, orca terminal pid). The harness must be one of them or under one.
	Roots []int
}

type surfaceCockpit interface {
	// CloseUnsupported is why this cockpit build cannot close one surface ("" when it can).
	CloseUnsupported(rec surfaceReceipt) string
	Live(rec surfaceReceipt) (liveSurface, error)
	Close(rec surfaceReceipt) error
}

// liveCockpit drives the real cockpit CLIs; run and help are injectable so the parsing is tested
// against stub cockpits.
type liveCockpit struct {
	run  func(name string, args ...string) ([]byte, error)
	help func(tok string, cmd ...string) bool
}

func (k liveCockpit) exec(name string, args ...string) ([]byte, error) {
	if k.run != nil {
		return k.run(name, args...)
	}
	return exec.Command(name, args...).Output()
}

func (k liveCockpit) has(tok string, cmd ...string) bool {
	if k.help != nil {
		return k.help(tok, cmd...)
	}
	return helpHas(tok, cmd...)
}

func (k liveCockpit) CloseUnsupported(rec surfaceReceipt) string {
	switch rec.Cockpit {
	case "tmux":
		return ""
	case "herdr":
		if !k.has("close", "herdr", "tab") {
			return "this herdr build's 'herdr tab --help' advertises no 'close'"
		}
		return ""
	case "orca":
		if !k.has("--terminal", "orca", "terminal", "close") {
			return "this orca build's 'orca terminal close --help' advertises no '--terminal'"
		}
		return ""
	}
	return fmt.Sprintf("cockpit %q has no single-surface close", rec.Cockpit)
}

func (k liveCockpit) Live(rec surfaceReceipt) (liveSurface, error) {
	var l liveSurface
	switch rec.Cockpit {
	case "tmux":
		out, err := k.exec("tmux", "list-windows", "-a", "-F", "#{window_id}\t#{session_name}\t#{window_name}\t#{pane_pid}")
		if err != nil {
			return l, fmt.Errorf("tmux list-windows: %w", err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			f := strings.Split(line, "\t")
			if len(f) != 4 || f[0] != rec.Surface {
				continue
			}
			pid, err := strconv.Atoi(f[3])
			if err != nil {
				return l, fmt.Errorf("tmux pane pid %q", f[3])
			}
			return liveSurface{Found: true, Session: f[1], Name: f[2], Roots: []int{pid}}, nil
		}
		return l, nil
	case "herdr":
		out, err := k.exec("herdr", "tab", "list", "--workspace", rec.Workspace)
		if err != nil {
			return l, fmt.Errorf("herdr tab list: %w", err)
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
			return l, errors.New("herdr tab list: unreadable")
		}
		for _, t := range *d.Result.Tabs {
			if t.ID != rec.Surface {
				continue
			}
			l = liveSurface{Found: true, Workspace: rec.Workspace, Name: t.Label}
			roots, err := k.herdrPaneProcs(rec.Pane)
			if err != nil {
				return liveSurface{}, err
			}
			l.Roots = roots
			return l, nil
		}
		return l, nil
	case "orca":
		out, err := k.exec("orca", "terminal", "show", "--terminal", rec.Surface, "--json")
		if err != nil {
			return l, fmt.Errorf("orca terminal show: %w", err)
		}
		var d struct {
			OK     bool `json:"ok"`
			Result struct {
				Terminal struct {
					Handle string `json:"handle"`
					Title  string `json:"title"`
					PID    *int   `json:"pid"`
				} `json:"terminal"`
			} `json:"result"`
			Meta struct {
				RuntimeID string `json:"runtimeId"`
			} `json:"_meta"`
		}
		if json.Unmarshal(out, &d) != nil || !d.OK || d.Result.Terminal.Handle == "" {
			return l, errors.New("orca terminal show: unreadable")
		}
		if d.Result.Terminal.Handle != rec.Surface {
			return l, nil
		}
		if d.Result.Terminal.PID == nil {
			return l, errors.New("this orca build does not expose the terminal's process")
		}
		return liveSurface{Found: true, Runtime: d.Meta.RuntimeID, Name: d.Result.Terminal.Title, Roots: []int{*d.Result.Terminal.PID}}, nil
	}
	return l, fmt.Errorf("cockpit %q", rec.Cockpit)
}

// herdrPaneProcs reads `herdr pane process-info --pane <pane>`: the pane's shell pid and its
// foreground processes.
func (k liveCockpit) herdrPaneProcs(pane string) ([]int, error) {
	if pane == "" {
		return nil, errors.New("the receipt records no herdr pane")
	}
	out, err := k.exec("herdr", "pane", "process-info", "--pane", pane)
	if err != nil {
		return nil, fmt.Errorf("herdr pane process-info: %w", err)
	}
	var d struct {
		Result struct {
			ShellPID   int `json:"shell_pid"`
			Foreground []struct {
				PID int `json:"pid"`
			} `json:"foreground_processes"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &d) != nil || d.Result.ShellPID <= 0 {
		return nil, errors.New("herdr pane process-info: no shell pid")
	}
	roots := []int{d.Result.ShellPID}
	for _, p := range d.Result.Foreground {
		if p.PID > 0 {
			roots = append(roots, p.PID)
		}
	}
	return roots, nil
}

// Close issues exactly one single-surface close. It never closes a session or workspace.
func (k liveCockpit) Close(rec surfaceReceipt) error {
	var err error
	switch rec.Cockpit {
	case "tmux":
		_, err = k.exec("tmux", "kill-window", "-t", rec.Surface)
	case "herdr":
		_, err = k.exec("herdr", "tab", "close", rec.Surface)
	case "orca":
		_, err = k.exec("orca", "terminal", "close", "--terminal", rec.Surface)
	default:
		err = fmt.Errorf("cockpit %q", rec.Cockpit)
	}
	return err
}

// verifySurface checks every recorded value against the live surface. harnessMayBeGone is set
// only for a cadence-mode role re-verified after its own STOP made its supervisor exit; the
// surface's session, name and id must still match.
func (l *interimLifecycle) verifySurface(c *Cell, rec surfaceReceipt, harnessMayBeGone bool) error {
	mismatch := func(format string, a ...any) error {
		return refuseWith(cellop.ReasonSurfaceIdentityMismatch, "%s: %s", rec.Role, fmt.Sprintf(format, a...))
	}
	if rec.Cell != c.Name || !valueIn(rec.Role, knownRoles) || rec.Name != surfaceName(c.Name, rec.Role) {
		return mismatch("the receipt names %s/%s %q, not this cell's role", rec.Cell, rec.Role, rec.Name)
	}
	if rec.Cockpit == "tmux" && rec.Session != c.Session {
		return mismatch("the receipt's session %q is not this cell's %q", rec.Session, c.Session)
	}
	if rec.HarnessPID <= 0 || rec.HarnessStart == "" {
		return mismatch("the receipt records no harness identity")
	}
	start, gone, err := l.probes.procs.Start(rec.HarnessPID)
	if err != nil {
		return mismatch("harness pid %d start time unreadable: %v", rec.HarnessPID, err)
	}
	harnessGone := gone || start != rec.HarnessStart
	if harnessGone && !harnessMayBeGone {
		return refuseWith(cellop.ReasonSurfaceStale, "%s: the recorded harness pid %d is gone or restarted; re-launch the role or run cell-wide down", rec.Role, rec.HarnessPID)
	}
	live, err := l.probes.cockpit.Live(rec)
	if err != nil {
		return mismatch("%v", err)
	}
	if !live.Found {
		return mismatch("no %s %s %s is live", rec.Cockpit, surfaceKind(rec.Cockpit), rec.Surface)
	}
	if live.Name != rec.Name {
		return mismatch("%s %s is named %q, not %q", surfaceKind(rec.Cockpit), rec.Surface, live.Name, rec.Name)
	}
	switch rec.Cockpit {
	case "tmux":
		if live.Session != c.Session {
			return mismatch("window %s is in session %q, not %q", rec.Surface, live.Session, c.Session)
		}
	case "herdr":
		if live.Workspace != rec.Workspace {
			return mismatch("tab %s is not in workspace %s", rec.Surface, rec.Workspace)
		}
	case "orca":
		if live.Runtime != rec.Runtime {
			return mismatch("terminal %s is in runtime %q, not %q", rec.Surface, live.Runtime, rec.Runtime)
		}
	}
	if harnessGone {
		return nil
	}
	for _, root := range live.Roots {
		if root == rec.HarnessPID {
			return nil
		}
		desc, err := l.probes.procs.Descendants(root)
		if err != nil {
			return mismatch("process tree unreadable: %v", err)
		}
		for _, p := range desc {
			if p == rec.HarnessPID {
				return nil
			}
		}
	}
	return mismatch("harness pid %d is not in %s %s's process tree", rec.HarnessPID, surfaceKind(rec.Cockpit), rec.Surface)
}
