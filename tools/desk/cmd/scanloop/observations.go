package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
//
// Retired is a recorded TERMINAL disposition for one event, written by the drain
// after the trust gate read it: "gone" (the forge answered a definite not-found
// for an issue in a repo the poller read cleanly this pass) or "quarantined"
// (read cleanly, untrusted and unblessed at its current content). A retired event
// stays in the history and is never replayed. A newer event on the same issue is
// a new key and is read and gated again, so a later blessing or edit is not lost.
type intakeObservation struct {
	Item    string    `json:"item"`
	Opened  time.Time `json:"opened"`
	Retired string    `json:"retired,omitempty"`
}
type intakeObservations struct {
	Schema string              `json:"schema"`
	Events []intakeObservation `json:"events"`
}

const (
	retiredGone        = "gone"
	retiredQuarantined = "quarantined"
)

// observationRetirement is one terminal disposition the drain records for an event.
type observationRetirement struct {
	Item   string
	Opened time.Time
	Reason string
}

func observationKey(item string, opened time.Time) string {
	return strings.ToLower(item) + "@" + opened.UTC().Format(time.RFC3339Nano)
}

// replayCap bounds how many RETAINED (replayed) events per repo one pass presents.
// It reuses the poller's own per-repo burst knob, so retention cannot route around
// the bound the poller already applies to a fresh delta. The rest stay retained and
// are named as deferred and presented oldest first once an earlier retained event
// is discharged (settled judgment, accountable rejection) or retired. An event that
// stays owed or unreadable keeps its slot, which is why the deferral is named on
// every pass rather than silent.
func replayCap() int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("INBOUND_MONITOR_BURST_CAP"))); err == nil && v > 0 {
		return v
	}
	return 25
}

// settledHandoff reports whether a recorded accountable judgment discharges the
// event from the routing queue. Only a recorded, assessed judgment does; labels,
// closure, a scan PR or a mechanical v1 exit never do. Owed, held or unreadable
// states stay.
func settledHandoff(state string) bool {
	return state == "observed" || state == "external-wait"
}

// judgedRejection reports whether an intake-exit-v1 record is an accountable
// rejection of one issue event: the exit is rejected-watching and the intake
// lane's judgment, not a mechanical default, decided it. The companion
// vocabulary has no rejection, and a rejected issue has no successor to read
// back, so this recorded judgment is its discharge.
func judgedRejection(r IntakeExitRecord) bool {
	return r.Source == "issue" && r.Exit == ExitRejectedWatching && r.DecidedBy == decidedJudgment && r.Opened != ""
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
		switch e.Retired {
		case "", retiredGone, retiredQuarantined:
		default:
			return out, fmt.Errorf("unknown intake observation disposition")
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

// writeObservations replaces the history atomically. Callers hold the audit lock.
func writeObservations(dir string, obs intakeObservations) error {
	raw, err := json.Marshal(obs)
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
		for _, e := range previous.Events {
			seen[observationKey(e.Item, e.Opened)] = true
		}
		changed := false
		for _, item := range incoming {
			e := intakeObservation{Item: item.ID(), Opened: item.UpdatedAt.UTC()}
			if !issueItemRe.MatchString(e.Item) || e.Opened.IsZero() {
				return fmt.Errorf("invalid inbound source event")
			}
			if k := observationKey(e.Item, e.Opened); !seen[k] {
				previous.Events = append(previous.Events, e)
				seen[k] = true
				changed = true
			}
		}
		result = previous
		if !changed {
			return nil
		}
		return writeObservations(dir, previous)
	})
	if err != nil {
		return result, deskkit.Unverifiable("intake observations could not be retained", err)
	}
	return result, nil
}

// retireObservations records terminal dispositions under the same lock and atomic
// replacement. It never deletes an event: the retired event remains history. An
// event already retired keeps its first recorded disposition.
func retireObservations(dir string, rs []observationRetirement) error {
	err := withAuditLock(func(_ string) error {
		obs, err := readObservations(dir)
		if err != nil {
			return err
		}
		index := map[string]int{}
		for i, e := range obs.Events {
			index[observationKey(e.Item, e.Opened)] = i
		}
		changed := false
		for _, r := range rs {
			if !issueItemRe.MatchString(r.Item) || r.Opened.IsZero() || (r.Reason != retiredGone && r.Reason != retiredQuarantined) {
				return fmt.Errorf("invalid intake observation disposition")
			}
			k := observationKey(r.Item, r.Opened)
			i, ok := index[k]
			if !ok {
				obs.Events = append(obs.Events, intakeObservation{Item: r.Item, Opened: r.Opened.UTC(), Retired: r.Reason})
				index[k] = len(obs.Events) - 1
				changed = true
				continue
			}
			if obs.Events[i].Retired == "" {
				obs.Events[i].Retired = r.Reason
				changed = true
			}
		}
		if !changed {
			return nil
		}
		return writeObservations(dir, obs)
	})
	if err != nil {
		return deskkit.Unverifiable("intake observation dispositions could not be recorded", err)
	}
	return nil
}

func retainingMonitor(dir string, scope []string, poll func() (*MonitorReport, error)) func() (*MonitorReport, error) {
	return func() (*MonitorReport, error) {
		var judgments []issueflow.IntakeHandoff
		var exits []IntakeExitRecord
		if err := withAuditLock(func(state string) error {
			var err error
			if judgments, err = readHandoffRecords(filepath.Join(state, handoffRecordsFile)); err != nil {
				return err
			}
			exits, err = readExitRecords(filepath.Join(state, intakeExitsFile))
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
		delta := map[string]bool{}
		for _, item := range report.Inbound {
			visible = append(visible, intakeObservation{Item: item.ID(), Opened: item.UpdatedAt})
			delta[observationKey(item.ID(), item.UpdatedAt)] = true
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
		retired := map[string]bool{}
		for _, e := range old.Events {
			if e.Retired != "" {
				retired[observationKey(e.Item, e.Opened)] = true
			}
		}
		rejected := map[string]bool{}
		for _, r := range exits {
			if !judgedRejection(r) {
				continue
			}
			opened, err := time.Parse(time.RFC3339, r.Opened)
			if err != nil {
				continue
			}
			rejected[observationKey(r.Item, opened)] = true
		}
		var current []Inbound
		for _, item := range latest {
			// The LATEST event decides: a recorded terminal disposition, a settled
			// accountable judgment or an accountable rejection of it discharges the
			// issue from replay. A newer event is a different key, so it is
			// presented again.
			key := observationKey(item.ID(), item.UpdatedAt)
			if retired[key] || rejected[key] || settledHandoff(item.HandoffState) {
				continue
			}
			current = append(current, item)
		}
		sort.Slice(current, func(i, j int) bool {
			a, b := current[i], current[j]
			if a.UpdatedAt.Equal(b.UpdatedAt) {
				return a.ID() < b.ID()
			}
			return a.UpdatedAt.Before(b.UpdatedAt)
		})
		limit := replayCap()
		presented, deferred := map[string]int{}, map[string]int{}
		report.Inbound = nil
		for _, item := range current {
			// This poll's delta is already bounded by the poller; only retained events count.
			if !delta[observationKey(item.ID(), item.UpdatedAt)] {
				if presented[item.Repo] >= limit {
					deferred[item.Repo]++
					continue
				}
				presented[item.Repo]++
			}
			report.Inbound = append(report.Inbound, item)
		}
		for _, repo := range scope {
			if n := deferred[repo]; n > 0 {
				report.Deferred = append(report.Deferred, fmt.Sprintf("%s %d retained observation(s) deferred (replay cap %d per repo; retained, oldest first)", repo, n, limit))
			}
		}
		report.retire = func(rs []observationRetirement) error { return retireObservations(dir, rs) }
		return report, nil
	}
}
