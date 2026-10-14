package cellstop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
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
	Role        string
	// Orphan is true when the process was re-parented to pid 1 (its launching
	// harness is gone), or descends from such a process through marked parents.
	Orphan bool
}

// MarkedProcesses lists every live process of the invoking user that carries
// m's marker. The invoking process, its ancestors and every process in
// protect (with their descendants) are never listed: protect holds the
// verified live lease holders, which are never ended. An error is
// could-not-check, never an empty cell.
func MarkedProcesses(m Marker, protect []int) ([]Marked, error) {
	if m.Cell == "" || !filepath.IsAbs(m.Dir) {
		return nil, fmt.Errorf("invalid cell marker")
	}
	all, err := inventory()
	if err != nil {
		return nil, err
	}
	return markedIn(all, m, ancestors(all), protect), nil
}

func markedIn(all map[int]process, m Marker, excluded map[int]bool, protect []int) []Marked {
	shielded := map[int]bool{}
	for pid := range excluded {
		shielded[pid] = true
	}
	for _, pid := range protect {
		shielded[pid] = true
	}
	// Descendants of a protected holder are part of its live session.
	for changed := true; changed; {
		changed = false
		for pid, p := range all {
			if !shielded[pid] && shielded[p.parent] && !excluded[p.parent] {
				shielded[pid] = true
				changed = true
			}
		}
	}
	orphan := map[int]bool{}
	for pid, p := range all {
		if !shielded[pid] && m.carries(p) && p.parent == 1 {
			orphan[pid] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for pid, p := range all {
			if !shielded[pid] && !orphan[pid] && orphan[p.parent] && m.carries(p) {
				orphan[pid] = true
				changed = true
			}
		}
	}
	var out []Marked
	for pid, p := range all {
		if shielded[pid] || !m.carries(p) {
			continue
		}
		out = append(out, Marked{PID: pid, Parent: p.parent, Identity: p.identity, Role: p.env[EnvCellRole], Orphan: orphan[pid]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

// StopMarked ends the given marked processes: SIGTERM to each, a bounded
// grace for them to exit, then SIGKILL to any survivor, then a re-check by pid
// AND start identity. Every signal is preceded by a fresh inventory that must
// still show the same pid with the same start identity carrying the same
// marker, so a reused pid is never signalled. It returns an error naming any
// process that is still running when the bound expires.
func StopMarked(ctx context.Context, m Marker, targets []Marked, grace time.Duration) error {
	if len(targets) == 0 {
		return nil
	}
	if grace <= 0 {
		grace = 5 * time.Second
	}
	procs := make([]process, 0, len(targets))
	for _, t := range targets {
		if t.PID <= 1 || t.PID == os.Getpid() || t.Identity == "" {
			return fmt.Errorf("refusing to signal pid %d: not a provable cell orphan", t.PID)
		}
		procs = append(procs, process{pid: t.PID, parent: t.Parent, identity: t.Identity})
	}
	for _, p := range procs {
		if err := signalMarked(p, m, sigTerm); err != nil {
			return err
		}
	}
	if left, err := waitGone(ctx, procs, grace); err != nil || len(left) == 0 {
		return err
	}
	for _, p := range procs {
		if err := signalMarked(p, m, sigKill); err != nil {
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
