// Package cellstop stops attributable local consumers before destructive cell
// maintenance. Environment values are inspected in memory and never logged.
package cellstop

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type Scope struct {
	Cell, Directory, Roots, CacheRoot string
	Worktrees                         []string
}
type process struct {
	pid, parent int
	group       int    // process group id
	foreign     bool   // owned by another uid: listed for the tree walk only, never a candidate
	envRead     bool   // its environment was actually readable (not withheld)
	comm        string // the process table's command name
	identity    string
	start       string // kernel start time (deskkit.ProcessStart); set only for a signal target
	env         map[string]string
}

func within(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func (s Scope) owns(p process) bool {
	for _, key := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		if within(p.env[key], s.CacheRoot) {
			return true
		}
	}
	session := p.env["DESK_SESSION"]
	if strings.HasPrefix(session, s.Cell+"-") &&
		(s.Roots != "" && p.env["DESK_ROOTS"] == s.Roots ||
			within(p.env["HOME"], s.Directory) || within(p.env["ASSAY_CONFIG_HOME"], s.Directory)) {
		return true
	}
	for _, root := range s.Worktrees {
		if within(p.env["PWD"], root) {
			return true
		}
	}
	return false
}

// Stop repeatedly inventories the cell, revalidates identity before signalling,
// and catches children born during shutdown. The invoking process and its
// ancestors are never signalled. A failure to establish quiescence is a refusal.
func Stop(ctx context.Context, scope Scope) error {
	if scope.Cell == "" || !filepath.IsAbs(scope.Directory) {
		return fmt.Errorf("invalid stop scope")
	}
	deadline := time.Now().Add(15 * time.Second)
	owned := map[int]string{}
	quiet := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		all, err := inventory()
		if err != nil {
			return err
		}
		for pid, identity := range owned {
			p, ok := all[pid]
			if !ok || p.identity != identity {
				delete(owned, pid)
			}
		}
		excluded := ancestors(all)
		for pid, p := range all {
			if !excluded[pid] && scope.owns(p) {
				owned[pid] = p.identity
			}
		}
		for changed := true; changed; {
			changed = false
			for pid, p := range all {
				if !excluded[pid] {
					if _, ok := owned[p.parent]; ok {
						if _, exists := owned[pid]; !exists {
							owned[pid] = p.identity
							changed = true
						}
					}
				}
			}
		}
		live := 0
		for pid, identity := range owned {
			p, ok := all[pid]
			if !ok || p.identity != identity {
				delete(owned, pid)
				continue
			}
			live++
			if err := terminate(p, time.Until(deadline) < 10*time.Second); err != nil {
				return err
			}
		}
		if live == 0 {
			quiet++
		} else {
			quiet = 0
		}
		if quiet == 2 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("cell consumers did not stop; refusing maintenance")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
