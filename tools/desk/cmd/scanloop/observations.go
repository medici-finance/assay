package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/domain"
	"github.com/medici-finance/assay/deskcore/issueflow"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const observationFile = "intake-observations-v1.json"

// These are observations of source events, not acknowledgements or assignments.
// Retain all event identities. The queue presents one current item, while the
// history remains available even after an issue closes or a scan carrier stops.
type intakeObservation struct {
	Item   string    `json:"item"`
	Opened time.Time `json:"opened"`
}
type intakeObservations struct {
	Schema string              `json:"schema"`
	Events []intakeObservation `json:"events"`
}

func readObservations(dir string) (intakeObservations, error) {
	out := intakeObservations{Schema: "intake-observations-v1"}
	raw, err := os.ReadFile(filepath.Join(dir, observationFile))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := domain.DecodeStrict(raw, &out); err != nil {
		return out, err
	}
	if out.Schema != "intake-observations-v1" {
		return out, fmt.Errorf("unsupported intake observation version")
	}
	for _, e := range out.Events {
		if !issueItemRe.MatchString(e.Item) || e.Opened.IsZero() {
			return out, fmt.Errorf("invalid intake source event")
		}
	}
	return out, nil
}

// Baselines are the poller's existing complete open-issue keysets. Reading them
// spends no additional forge request. The pre-poll save closes the crash window
// after a previous poll advanced its cursor but before its delta was consumed.
func baselineObservations(dir string, scope []string) ([]Inbound, error) {
	var out []Inbound
	for _, repo := range scope {
		raw, err := os.ReadFile(filepath.Join(dir, strings.ReplaceAll(repo, "/", "__")+".state"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			report := ParseMonitorOutput("INBOUND: " + line)
			if len(report.Inbound) != 1 || len(report.Unparsed) != 0 || report.Inbound[0].Repo != repo || report.Inbound[0].UpdatedAt.IsZero() {
				return nil, fmt.Errorf("invalid baseline event in %s", repo)
			}
			out = append(out, report.Inbound[0])
		}
	}
	return out, nil
}

// Save by atomic replacement under the existing audit lock; no second lock
// protocol or mutable lifecycle store. Re-read inside the lock so overlapping
// polls cannot overwrite each other's observations. A failed write is loud and
// leaves the old complete history intact.
func retainObservations(dir string, incoming []Inbound) (intakeObservations, error) {
	var result intakeObservations
	err := withAuditLock(func(_ string) error {
		previous, err := readObservations(dir)
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		key := func(e intakeObservation) string {
			return strings.ToLower(e.Item) + "@" + e.Opened.UTC().Format(time.RFC3339Nano)
		}
		for _, e := range previous.Events {
			seen[key(e)] = true
		}
		changed := false
		for _, item := range incoming {
			e := intakeObservation{item.ID(), item.UpdatedAt.UTC()}
			if !issueItemRe.MatchString(e.Item) || e.Opened.IsZero() {
				return fmt.Errorf("invalid inbound source event")
			}
			if !seen[key(e)] {
				previous.Events = append(previous.Events, e)
				seen[key(e)] = true
				changed = true
			}
		}
		result = previous
		if !changed {
			return nil
		}
		raw, err := json.Marshal(previous)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		f, err := os.CreateTemp(dir, ".intake-observations-*")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		if _, err := f.Write(raw); err != nil {
			f.Close()
			return err
		}
		if err := f.Sync(); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		return os.Rename(f.Name(), filepath.Join(dir, observationFile))
	})
	if err != nil {
		return result, deskkit.Unverifiable("intake observations could not be retained", err)
	}
	return result, nil
}

func retainingMonitor(dir string, scope []string, poll func() (*MonitorReport, error)) func() (*MonitorReport, error) {
	return func() (*MonitorReport, error) {
		var judgments []issueflow.IntakeHandoff
		if err := withAuditLock(func(state string) error {
			var err error
			judgments, err = readHandoffRecords(filepath.Join(state, handoffRecordsFile))
			return err
		}); err != nil {
			return nil, deskkit.Unverifiable("cannot read accountable intake dispositions", err)
		}

		before, err := baselineObservations(dir, scope)
		if err != nil {
			return nil, deskkit.Unverifiable("cannot read intake baseline before polling", err)
		}
		old, err := retainObservations(dir, before)
		if err != nil {
			return nil, err
		}
		report, err := poll()
		if err != nil {
			return nil, err
		}
		if report == nil {
			return nil, deskkit.Unverifiable("poll returned no intake report", nil)
		}
		after, err := baselineObservations(dir, scope)
		if err != nil {
			return nil, deskkit.Unverifiable("cannot read intake baseline after polling", err)
		}
		_, err = retainObservations(dir, append(after, report.Inbound...))
		if err != nil {
			return nil, err
		}
		// One queue/claim entry per issue. Event history is never collapsed on disk.
		replay := map[string]bool{}
		for _, e := range old.Events {
			replay[strings.ToLower(e.Item)] = true
		}
		latest := map[string]Inbound{}
		// First-time arming still seeds silently. The saved seed becomes visible
		// on the next pass; only old observations and this poll's delta queue now.
		visible := append([]intakeObservation(nil), old.Events...)
		for _, item := range report.Inbound {
			visible = append(visible, intakeObservation{item.ID(), item.UpdatedAt})
		}
		for _, e := range visible {
			parsed := ParseMonitorOutput("INBOUND: " + e.Item + " " + e.Opened.Format(time.RFC3339Nano))
			if len(parsed.Inbound) != 1 {
				return nil, deskkit.Unverifiable("cannot reconstruct intake event", nil)
			}
			item := parsed.Inbound[0]
			key := strings.ToLower(e.Item)
			item.Replayed = replay[key]
			item.HandoffState = "disposition-owed"
			revision := 0
			for _, h := range judgments {
				if !strings.EqualFold(h.Item, e.Item) || h.Opened != e.Opened.UTC().Format(time.RFC3339) || h.Revision <= revision {
					continue
				}
				revision = h.Revision
				a := issueflow.AssessIntakeHandoff(h, issueflow.IntakeFacts{Item: e.Item, Opened: h.Opened, Complete: true})
				item.HandoffState, item.HandoffRoute, item.HandoffSuccessor = a.State, a.Route, h.Successor
			}

			if prev, ok := latest[key]; !ok || item.UpdatedAt.After(prev.UpdatedAt) {
				latest[key] = item
			}
		}
		report.Inbound = nil
		for _, item := range latest {
			report.Inbound = append(report.Inbound, item)
		}
		sort.Slice(report.Inbound, func(i, j int) bool {
			a, b := report.Inbound[i], report.Inbound[j]
			if a.UpdatedAt.Equal(b.UpdatedAt) {
				return a.ID() < b.ID()
			}
			return a.UpdatedAt.Before(b.UpdatedAt)
		})
		return report, nil
	}
}
