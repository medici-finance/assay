package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

// The launcher already qualified the planner result. Check scope, freshness, generation
// and every subject here before any checkpoint can advance in the offline transport.
func readObservedFeedback(observationPath, feedbackPath string, repos []string) (map[string]prSnapshot, error) {
	ob, err := os.ReadFile(observationPath)
	if err != nil {
		return nil, err
	}
	obs, err := rf.DecodeObservation(ob)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(feedbackPath)
	if err != nil {
		return nil, err
	}
	var batch deskkit.MonitorBatch
	if err = json.Unmarshal(b, &batch); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if _, err := rf.PlanObservation(obs, rf.ObservationPolicy{Now: now, Oldest: now.Add(-5 * time.Minute), Policy: obs.Inputs.Policy}); err != nil {
		return nil, err
	}
	if batch.Rows == nil || batch.Policy != obs.Inputs.Policy || batch.Generation != obs.Inputs.Collection.Start.Format(time.RFC3339Nano) {
		return nil, fmt.Errorf("inapplicable feedback generation")
	}
	out := map[string]prSnapshot{}
	for _, r := range obs.Inputs.Admitted {
		if r.Provider != "github" || r.Host != "github.com" {
			return nil, fmt.Errorf("unsupported observation source")
		}
		out[r.Repository] = prSnapshot{source: r.Provider, feedback: map[int]deskkit.MonitorFeedback{}}
	}
	if len(repos) != len(out) {
		return nil, fmt.Errorf("incomplete feedback scope")
	}
	seen := map[string]bool{}
	for _, repo := range repos {
		key := strings.ToLower(repo)
		if _, ok := out[key]; !ok || seen[key] {
			return nil, fmt.Errorf("missing or duplicate feedback repository")
		}
		seen[key] = true
	}
	for _, c := range obs.Inputs.Changes {
		id := c.Identity
		repo := id.Repository.Repository
		snap, ok := out[repo]
		key := deskkit.MonitorSubject(id.Repository.String(), id.Number, id.Head)
		fb, found := batch.Rows[key]
		if !ok || !found || !validFeedback(fb.Reviews) || !validFeedback(fb.Checks) {
			return nil, fmt.Errorf("missing feedback for observed change")
		}
		delete(batch.Rows, key)
		draft := "ready"
		if c.Facts.Change.IsDraft {
			draft = "draft"
		}
		snap.rows = append(snap.rows, prRow{number: id.Number, line: fmt.Sprintf("%d %s %s OPEN %s", id.Number, id.Head, draft, c.Facts.Change.MergeStateStatus)})
		snap.feedback[id.Number] = fb
		out[repo] = snap
	}
	if len(batch.Rows) != 0 {
		return nil, fmt.Errorf("feedback includes unobserved changes")
	}
	return out, nil
}

func validFeedback(v string) bool {
	if v == "" {
		return true
	}
	if len(v) != 64 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}

// Keep the original five-column checkpoint byte-compatible with previous binaries.
// The companion stores no private feedback text. Incomplete dimensions inherit only
// observations of the same source and head; a push already wakes the shepherd.
//
// Every failure is returned through this one wrapper, so each names the companion file
// it concerns: an operator can then restore or remove exactly that file.
func checkpointFeedback(dir, repo string, snap prSnapshot, primary []string, out io.Writer) error {
	if snap.source == "" {
		return nil
	}
	side := filepath.Join(dir, "feedback-v1")
	path := statePath(side, repo)
	if err := advanceFeedback(side, path, repo, snap, primary, out); err != nil {
		return precondition("monitor feedback checkpoint '%s': %s; restore or remove that file to reseed %s", path, diagnostic(err), repo)
	}
	return nil
}

func advanceFeedback(side, path, repo string, snap prSnapshot, primary []string, out io.Writer) error {
	lines, err := readStateLines(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	old := map[string][]string{}
	for i, l := range lines {
		f := strings.Fields(l)
		if len(f) != 5 || (f[3] != "-" && !validFeedback(f[3])) || (f[4] != "-" && !validFeedback(f[4])) {
			return fmt.Errorf("invalid baseline entry %d", i+1)
		}
		old[f[0]] = f
	}
	heads := map[string]string{}
	for _, l := range primary {
		f := prFields(l)
		heads[f[0]] = f[1]
	}
	next := []string{}
	var events []string
	for _, row := range snap.rows {
		f := prFields(row.line)
		num, head := f[0], f[1]
		mark := snap.feedback[row.number]
		if head == "null" || head == "" {
			continue
		}
		values := []string{mark.Reviews, mark.Checks}
		for i, kind := range []string{"review", "checks"} {
			if values[i] == "" {
				values[i] = "-"
			}
			prev := old[num]
			if len(prev) != 5 || prev[1] != snap.source || prev[2] != head || heads[num] != head {
				continue
			}
			known := prev[i+3]
			if values[i] == "-" {
				values[i] = known
				continue
			}
			if known != "-" && known != values[i] {
				events = append(events, fmt.Sprintf("PR-EVENT: %s#%s %s %s -> %s", repo, num, kind, known, values[i]))
			}
		}
		if values[0] != "-" || values[1] != "-" {
			next = append(next, strings.Join([]string{strconv.Itoa(row.number), snap.source, head, values[0], values[1]}, " "))
		}
	}
	next = sortC(next)
	if slices.Equal(lines, next) {
		return nil
	}
	if err := os.MkdirAll(side, 0700); err != nil {
		return err
	}
	if err := writeState(side, repo, next); err != nil {
		return err
	}
	for _, ev := range events {
		fmt.Fprintln(out, ev)
	}
	return nil
}
