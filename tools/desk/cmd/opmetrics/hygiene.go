package main

// hygiene.go — session hygiene from the desk-tools state directory.
//
// PATH NOTE (measured, 2026-08-13). The brief places roster beacons at
// `~/.claude/desk-tools/roster/` and claims at `~/.claude/desk-tools/claims/`. On this
// tree they are `<deskkit.StateDir()>/roster/` and `<deskkit.StateDir()>/claims/`,
// i.e. `~/.config/assay/` — see internal/deskkit/killswitch.go deskDir() and
// cmd/deskroster/roster.go rosterDir()/claimsDir(). The default here follows the CODE,
// not the brief, and --desk-tools overrides it. The correction is recorded in the PR.
//
// Beacon and claim reads are TOLERANT for the same reason deskkit's Claim reader is:
// three writers have written claims over this project's life and a metrics collector
// that only understood one of them would report a confident, wrong, low number.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// zombieStale is the beacon age past which a session with open work is a zombie: it
// registered work and then stopped reporting. 60 minutes is the brief's figure.
const zombieStale = 60 * time.Minute

// longSession is the transcript span past which a session counts as long-running —
// the "sessions >24h" hygiene signal.
const longSession = 24 * time.Hour

type beaconWork struct {
	Repo string `json:"repo"`
	PR   int    `json:"pr"`
}

type beaconFile struct {
	Session  string       `json:"session"`
	Updated  string       `json:"updated"`
	OpenWork []beaconWork `json:"open_work"`
}

// RosterRead is the aggregate of one pass over the roster beacons.
type RosterRead struct {
	Beacons     int
	Zombies     int
	Unparseable int
}

// ReadBeacons counts roster beacons and the subset that are stale WITH open work.
// A beacon with a stale timestamp but no open work is not a zombie — it is a session
// that finished and stopped beaconing, which is the normal end state.
func ReadBeacons(dir string, now time.Time) (RosterRead, error) {
	var out RosterRead
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out, fmt.Errorf("roster dir %s is not readable: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			out.Unparseable++
			continue
		}
		var b beaconFile
		if jerr := json.Unmarshal(raw, &b); jerr != nil {
			out.Unparseable++
			continue
		}
		out.Beacons++
		upd, terr := time.Parse(time.RFC3339, b.Updated)
		if terr != nil {
			// An unparseable timestamp cannot be proven fresh. It is counted as
			// unparseable and NOT as a zombie: inventing a zombie is as wrong as
			// missing one, and the unparseable count is what tells the reader the
			// answer is incomplete.
			out.Unparseable++
			continue
		}
		if len(b.OpenWork) > 0 && now.Sub(upd) > zombieStale {
			out.Zombies++
		}
	}
	return out, nil
}

// ClaimsRead is the aggregate of one pass over the dispatch claims.
type ClaimsRead struct {
	// FiledOnDate is claims whose timestamp falls on the reported day.
	FiledOnDate int
	// Open is every claim file present at read time, whatever its age.
	Open int
	// Owners is the number of DISTINCT claim owners — "sessions active". The owner
	// strings themselves are counted and discarded; they never reach the emit.
	Owners      int
	Unparseable int
}

// ReadClaims counts dispatch claims through deskkit's tolerant Claim reader, so
// claims written by the CLI, by loopengine, or by the legacy roster/bash idiom all
// count. A collector that understood only the canonical shape would under-report.
func ReadClaims(dir string, day time.Time) (ClaimsRead, error) {
	var out ClaimsRead
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out, fmt.Errorf("claims dir %s is not readable: %w", dir, err)
	}
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	owners := map[string]struct{}{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".claim" {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			out.Unparseable++
			continue
		}
		var c deskkit.Claim
		if jerr := json.Unmarshal(raw, &c); jerr != nil {
			out.Unparseable++
			continue
		}
		out.Open++
		if c.Owner != "" {
			owners[c.Owner] = struct{}{}
		}
		ts, terr := time.Parse(time.RFC3339, c.TS)
		if terr != nil {
			out.Unparseable++
			continue
		}
		ts = ts.In(day.Location())
		if !ts.Before(dayStart) && ts.Before(dayEnd) {
			out.FiledOnDate++
		}
	}
	out.Owners = len(owners)
	return out, nil
}

// countLongSessions returns how many transcript spans exceed 24 hours.
func countLongSessions(spans []SessionSpan) int {
	n := 0
	for _, s := range spans {
		if s.Last.Sub(s.First) > longSession {
			n++
		}
	}
	return n
}
