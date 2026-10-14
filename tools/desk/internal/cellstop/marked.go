package cellstop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The cell marker cellctl stamps on every child it launches for a role (and on
// the comms service). Children inherit it, so a background loop a harness left
// behind still names the cell, its directory and the role it was started for.
// The marker is the ONLY ownership proof a reconcile accepts: matching a path or
// a command line is not proof that a process belongs to a cell.
const (
	EnvCell     = "ASSAY_CELL"
	EnvCellDir  = "ASSAY_CELL_DIR"
	EnvCellRole = "ASSAY_CELL_ROLE"
)

// Marker names one cell: its name and its directory. Two cells may share a
// name under different registries, so the directory is part of the identity.
type Marker struct{ Cell, Dir string }

// Env is the marker for one role, as KEY=VALUE pairs for a child environment.
func (m Marker) Env(role string) []string {
	return []string{EnvCell + "=" + m.Cell, EnvCellDir + "=" + m.Dir, EnvCellRole + "=" + role}
}

func (m Marker) carries(p process) bool {
	if m.Cell == "" || m.Dir == "" || p.env[EnvCell] != m.Cell || p.env[EnvCellRole] == "" {
		return false
	}
	return sameDir(p.env[EnvCellDir], m.Dir)
}

func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// Marked is one live process of the invoking user that carries a cell's marker.
type Marked struct {
	PID, Parent int
	Identity    string // start time and command, as the process table reports them
	Start       string // the kernel's start time, re-checked immediately before any signal
	Role        string
	// Orphan is true when the process was re-parented to pid 1 (its launching
	// harness is gone), or descends from such a process through marked parents.
	Orphan bool
	// Shielded is true when it is, is above, or is hosted alongside something live: it is
	// listed so that its role's record can never be classed dead while it runs, and it is
	// never a signal target.
	Shielded bool
}

// Unattributed is a live process in a dead role child's process group that does not carry a
// readable cell marker (macOS withholds the environment of its platform binaries; a process
// started before the marker existed carries none). Its descent from the role child is evident,
// its ownership is not: it is reported, and it is never signalled.
type Unattributed struct {
	PID, Parent, Group int
	Identity           string
}

// Scan is MarkedProcesses plus, for each process-group id in groups (the recorded, now-dead
// role children: each was its own group leader), the live members that are not listed as
// marked. One inventory serves both.
func Scan(m Marker, protect []int, groups []int) ([]Marked, map[int][]Unattributed, error) {
	if m.Cell == "" || !filepath.IsAbs(m.Dir) {
		return nil, nil, fmt.Errorf("invalid cell marker")
	}
	all, err := inventoryAll()
	if err != nil {
		return nil, nil, err
	}
	listedRaw, err := markedIn(all, m, ancestors(all), protect)
	if err != nil {
		return nil, nil, err
	}
	marked, err := withKernelStart(listedRaw)
	if err != nil {
		return nil, nil, err
	}
	listed := map[int]bool{}
	for _, p := range marked {
		listed[p.PID] = true
	}
	want := map[int]bool{}
	for _, g := range groups {
		if g > 1 {
			want[g] = true
		}
	}
	var unattributed map[int][]Unattributed
	for pid, p := range all {
		if !want[p.group] || listed[pid] || pid == os.Getpid() {
			continue
		}
		if unattributed == nil {
			unattributed = map[int][]Unattributed{}
		}
		unattributed[p.group] = append(unattributed[p.group], Unattributed{PID: pid, Parent: p.parent, Group: p.group, Identity: p.identity})
	}
	for _, u := range unattributed {
		sort.Slice(u, func(i, j int) bool { return u[i].PID < u[j].PID })
	}
	return marked, unattributed, nil
}

// MarkedProcesses lists every live process of the invoking user that carries
// m's marker. The invoking process and every process in protect (the verified
// live lease holders) are never listed, nor is anything above them or anything
// else hosted by what is above them. An error is
// could-not-check, never an empty cell.
func MarkedProcesses(m Marker, protect []int) ([]Marked, error) {
	if m.Cell == "" || !filepath.IsAbs(m.Dir) {
		return nil, fmt.Errorf("invalid cell marker")
	}
	all, err := inventoryAll()
	if err != nil {
		return nil, err
	}
	listed, err := markedIn(all, m, ancestors(all), protect)
	if err != nil {
		return nil, err
	}
	return withKernelStart(listed)
}

// withKernelStart records each listed process's kernel start time, dropping any that exited.
// An unreadable start time is could-not-check for the whole listing: nothing is signalled on
// an identity that could not be measured.
func withKernelStart(listed []Marked) ([]Marked, error) {
	out := listed[:0]
	for _, p := range listed {
		if p.PID == os.Getpid() {
			continue // the invoker: its exec-time marker is the one it inherited and dropped
		}
		start, gone, err := deskkit.ProcessStart(p.PID)
		if gone {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could-not-check pid %d's start time: %w", p.PID, err)
		}
		p.Start = start
		out = append(out, p)
	}
	return out, nil
}

func markedIn(all map[int]process, m Marker, excluded map[int]bool, protect []int) ([]Marked, error) {
	// A process is shielded when it is, sits above, or is hosted alongside anything that may be
	// live and not this cell's: the invoking process, this cell's live lease holders, and — for
	// every other cell and user on the host, whose leases cannot be enumerated from here — every
	// process of another uid, every cellctl (any cell's supervisor), and every process whose
	// environment was READ and does not carry this cell's marker (another cell's role child, an
	// unrelated program). A process whose environment is withheld (a macOS platform binary) is
	// not a root: it cannot be told apart, and every live window has a readable cellctl
	// supervisor above it, which is. Shielding runs over the full tree of every uid, both ways —
	// up from every root, then down from every shielded process. Pid 1 is never a shielding root
	// for its subtree: it is everyone's parent. A walk that meets a parent missing from the
	// listing cannot prove what it would have shielded: could-not-check.
	shielded := map[int]bool{}
	var roots []int
	for pid := range excluded {
		roots = append(roots, pid)
	}
	roots = append(roots, protect...)
	for pid, p := range all {
		if p.foreign || filepath.Base(p.comm) == "cellctl" || p.envRead && !m.carries(p) {
			roots = append(roots, pid)
		}
	}
	for _, pid := range roots {
		for seen := 0; pid > 1 && seen <= len(all); seen++ {
			if shielded[pid] {
				break // the rest of this chain was walked from another root
			}
			shielded[pid] = true
			p, ok := all[pid]
			if !ok {
				return nil, fmt.Errorf("could-not-check: pid %d is not in the process listing, so what it hosts cannot be shielded", pid)
			}
			pid = p.parent
		}
	}
	for changed := true; changed; {
		changed = false
		for pid, p := range all {
			if !shielded[pid] && p.parent > 1 && shielded[p.parent] {
				shielded[pid] = true
				changed = true
			}
		}
	}
	orphan := map[int]bool{}
	for pid, p := range all {
		if !shielded[pid] && !p.foreign && m.carries(p) && p.parent == 1 {
			orphan[pid] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for pid, p := range all {
			if !shielded[pid] && !p.foreign && !orphan[pid] && orphan[p.parent] && m.carries(p) {
				orphan[pid] = true
				changed = true
			}
		}
	}
	var out []Marked
	for pid, p := range all {
		if p.foreign || !m.carries(p) {
			continue
		}
		out = append(out, Marked{PID: pid, Parent: p.parent, Identity: p.identity, Role: p.env[EnvCellRole], Orphan: orphan[pid], Shielded: shielded[pid]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out, nil
}

// StopMarked ends the given marked processes: SIGTERM to each, a bounded
// grace for them to exit, then SIGKILL to any survivor, then a re-check by pid
// AND start identity. Every signal is preceded by a fresh inventory that must
// still show the same pid with the same start identity carrying the same
// marker, so a reused pid is never signalled. It returns an error naming any
// process that is still running when the bound expires.
//
// reprove recomputes the whole listing — live holders, their ancestors and hosted trees, every
// shield root — and is called immediately before EACH signal: the prompt in front of this call
// can wait for any length of human time, and a live window may open beneath a listed process in
// the meantime. A target that is still running but is no longer listed as the same orphan (same
// pid, identity and kernel start) stops the run with nothing further signalled.
func StopMarked(ctx context.Context, m Marker, targets []Marked, grace time.Duration, reprove func() ([]Marked, error)) error {
	if reprove == nil {
		return fmt.Errorf("refusing to signal: no way to re-prove the shield at signal time")
	}
	if len(targets) == 0 {
		return nil
	}
	if grace <= 0 {
		grace = 5 * time.Second
	}
	procs := make([]process, 0, len(targets))
	for _, t := range targets {
		if t.PID <= 1 || t.PID == os.Getpid() || t.Identity == "" || t.Start == "" || !t.Orphan || t.Shielded {
			return fmt.Errorf("refusing to signal pid %d: not a provable cell orphan", t.PID)
		}
		procs = append(procs, process{pid: t.PID, parent: t.Parent, identity: t.Identity, start: t.Start})
	}
	signal := func(p process, sig signalKind) error {
		if err := stillOrphan(p, reprove); err != nil {
			return err
		}
		return signalMarked(p, m, sig)
	}
	for _, p := range procs {
		if err := signal(p, sigTerm); err != nil {
			return err
		}
	}
	if left, err := waitGone(ctx, procs, grace); err != nil || len(left) == 0 {
		return err
	}
	for _, p := range procs {
		if err := signal(p, sigKill); err != nil {
			return err
		}
	}
	left, err := waitGone(ctx, procs, 5*time.Second)
	if err != nil {
		return err
	}
	if len(left) > 0 {
		return fmt.Errorf("orphaned cell processes still running after SIGKILL: %v", left)
	}
	return nil
}

// waitGone polls until none of procs is running with its recorded identity.
func waitGone(ctx context.Context, procs []process, bound time.Duration) ([]int, error) {
	deadline := time.Now().Add(bound)
	for {
		all, err := inventory()
		if err != nil {
			return nil, err
		}
		var left []int
		for _, p := range procs {
			if cur, ok := all[p.pid]; ok && cur.identity == p.identity {
				left = append(left, p.pid)
			}
		}
		if len(left) == 0 || time.Now().After(deadline) {
			return left, nil
		}
		select {
		case <-ctx.Done():
			return left, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// stillOrphan re-proves, from a fresh listing, that p is still the same unshielded orphan. A
// target that has exited (or whose pid now names another process) is not an error: signalMarked
// will not signal it. A target still running but no longer listed as that orphan is.
func stillOrphan(p process, reprove func() ([]Marked, error)) error {
	listed, err := reprove()
	if err != nil {
		return fmt.Errorf("re-proving the shield before signalling pid %d: %w", p.pid, err)
	}
	for _, t := range listed {
		if t.PID == p.pid && t.Orphan && !t.Shielded && t.Identity == p.identity && t.Start == p.start {
			return nil
		}
	}
	all, err := inventory()
	if err != nil {
		return err
	}
	if cur, ok := all[p.pid]; ok && cur.identity == p.identity {
		return fmt.Errorf("pid %d is no longer a provable orphan (it is now shielded by a live window or holder); nothing further was signalled", p.pid)
	}
	return nil
}
