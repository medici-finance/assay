package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// --- Launch readiness view (assay-launch/04) ---
//
// --launch walks the transitive depends: closure of the go-live gate
// (assay-launch/05 by default, overridable via --launch-target) and prints a
// single-panel readiness table — every dependency and its live status from the
// brief tables — plus a one-line verdict. Diagnostic only; never reads or writes
// STATUS.md.

// launchDep is one node in the transitive depends closure.
type launchDep struct {
	ID     string // "stream/NN"
	Title  string
	Status string
}

// statusMark returns a readiness mark for the dependency status.
// done/verified = ready; in-progress/implemented = in-flight; else = not-started.
func statusMark(status string) string {
	switch status {
	case "done", "verified":
		return "✅" // ✅
	case "in-progress", "implemented":
		return "⏳" // ⏳
	default:
		return "❌" // ❌
	}
}

// launchTransitiveClosure walks the forward depends: graph from the target brief
// ID, collecting every reachable dep. Cycle-safe via a visited set.
func launchTransitiveClosure(streams []*Stream, target string) ([]launchDep, error) {
	// Build an index: brief ID -> (title, status, depends list).
	type node struct {
		title   string
		status  string
		depends []string
	}
	index := map[string]node{}
	for _, s := range streams {
		for _, b := range s.Briefs {
			id := s.Name + "/" + b.Num
			index[id] = node{
				title:   b.Title,
				status:  b.Status,
				depends: b.Depends,
			}
		}
	}

	targetNode, ok := index[target]
	if !ok {
		return nil, fmt.Errorf("target %q not found in any stream", target)
	}

	// Walk transitive closure.
	visited := map[string]bool{}
	var deps []launchDep
	stack := append([]string(nil), targetNode.depends...)
	visited[target] = true // never include the target itself

	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[id] {
			continue
		}
		visited[id] = true

		n, ok := index[id]
		if !ok {
			// Unresolvable dep — include it as an unknown entry.
			deps = append(deps, launchDep{ID: id, Title: "(unresolved)", Status: "unknown"})
			continue
		}
		deps = append(deps, launchDep{ID: id, Title: n.title, Status: n.status})
		for _, d := range n.depends {
			if !visited[d] {
				stack = append(stack, d)
			}
		}
	}

	// Sort by ID for deterministic output.
	sort.Slice(deps, func(i, j int) bool { return deps[i].ID < deps[j].ID })

	return deps, nil
}

// renderLaunchView renders the readiness table + verdict.
func renderLaunchView(target string, deps []launchDep) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Launch readiness for %s\n", target)
	fmt.Fprintf(&b, "Readiness per the board — does not verify deploys happened.\n\n")

	// Table header.
	fmt.Fprintf(&b, "%-35s %-6s %s\n", "DEPENDENCY", "STATUS", "TITLE")
	fmt.Fprintf(&b, "%-35s %-6s %s\n", strings.Repeat("-", 35), strings.Repeat("-", 6), strings.Repeat("-", 50))

	readyCount := 0
	notReady := []string{}

	for _, d := range deps {
		mark := statusMark(d.Status)
		statusDisplay := d.Status
		if d.Status == "unknown" {
			mark = "❓" // ❓
		}
		fmt.Fprintf(&b, "%-35s %-2s %-6s %s\n", d.ID, mark, statusDisplay, d.Title)
		if d.Status == "done" || d.Status == "verified" {
			readyCount++
		} else if d.Status != "unknown" {
			notReady = append(notReady, d.ID)
		}
	}

	fmt.Fprintf(&b, "\n---\n")
	total := len(deps)
	if total == 0 {
		fmt.Fprintf(&b, "READY: no dependencies — nothing blocks the gate.\n")
	} else if len(notReady) == 0 {
		fmt.Fprintf(&b, "READY: all %d deps done.\n", total)
	} else {
		fmt.Fprintf(&b, "BLOCKED: %d of %d deps not done: %s\n", len(notReady), total, strings.Join(notReady, ", "))
	}

	return b.String()
}

// runLaunch is the --launch entrypoint. It never reads or writes STATUS.md —
// a self-contained diagnostic sub-command, same discipline as --dora/--trend/--roadmap.
func runLaunch(root, target string) int {
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	attachPlaceholders(streams)
	// Wire Depends from brief-v1 frontmatter so the transitive walk works.
	// Problems are ignored — this is a diagnostic view, not a validation gate.
	checkBriefFiles(streams)

	deps, err := launchTransitiveClosure(streams, target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}

	fmt.Print(renderLaunchView(target, deps))
	return 0
}
