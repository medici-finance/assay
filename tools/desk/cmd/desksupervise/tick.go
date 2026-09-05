package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// tick.go — `desksupervise tick`: one sweep, either offline (--claims-fixture +
// --observations-fixture, what every Verify row runs) or live (claims read via live.go,
// probes via loopengine.HouseProbes()).

// cmdTick implements `desksupervise tick [--root DIR] [--repo OWNER/NAME] [--dry-run]
// [--now RFC3339] [--claims-fixture FILE] [--observations-fixture FILE]`.
func cmdTick(args []string) (err error) {
	ac := &auditCtx{verb: "tick"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("tick", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	root := fs.String("root", "", "repo root to read a live claim tool against (default: cwd)")
	repo := fs.String("repo", "", "owner/name of the repo whose dispatch claims to sweep (live mode)")
	dryRun := fs.Bool("dry-run", false, "classify and print only — never release a claim, file an issue, or journal")
	nowStr := fs.String("now", "", "RFC3339 clock override — Verify-fixture use only; a live run always uses the real clock")
	claimsFixture := fs.String("claims-fixture", "", "JSON array of claim records — bypasses the live claim tool")
	obsFixture := fs.String("observations-fixture", "", "JSON object keyed by claim key — bypasses the forge and audit file")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("refused: bad flags: " + err.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: tick takes no positional arguments")
	}
	if (*claimsFixture == "") != (*obsFixture == "") {
		return deskkit.Refused("refused: --claims-fixture and --observations-fixture are given together or not at all")
	}
	ac.repo = *repo

	now := time.Now().UTC()
	if *nowStr != "" {
		parsed, perr := time.Parse(time.RFC3339, *nowStr)
		if perr != nil {
			return deskkit.Refused("refused: --now " + *nowStr + " is not RFC3339")
		}
		now = parsed
	}

	pol, perr := loadPolicy()
	if perr != nil {
		return perr
	}

	var (
		claims    []claimRecord
		obsSource observationSource
	)
	if *claimsFixture != "" {
		claims, err = loadClaimsFixture(*claimsFixture)
		if err != nil {
			return err
		}
		obsByKey, oerr := loadObservationsFixture(*obsFixture)
		if oerr != nil {
			return oerr
		}
		obsSource = fixtureObservationSource(obsByKey)
	} else {
		dir := *root
		if dir == "" {
			wd, gerr := os.Getwd()
			if gerr != nil {
				return deskkit.Unverifiable("cannot resolve working directory", gerr)
			}
			dir = wd
		}
		liveClaims, cerr := readLiveClaims(dir, *repo, now)
		if cerr != nil {
			return cerr
		}
		claims = liveClaims
		obsSource = liveObservationSource(loopengine.HouseProbes())
	}

	results, anyBlind, serr := sweep(claims, obsSource, pol, now, *dryRun, doReclaim, doFileBlockedTimeout, doArmRunStop, os.Stdout)
	if serr != nil {
		return serr
	}

	ac.detail = fmt.Sprintf("%d claim(s) classified, blind=%t, dry-run=%t", len(results), anyBlind, *dryRun)
	if len(results) == 0 {
		ac.successResult = deskkit.ResultNoop
	}
	if anyBlind {
		// The tick itself ran cleanly (every claim WAS classified — see the printed lines),
		// but the READING is incomplete: at least one claim's liveness could not be
		// positively established. Exit 6, not 0 — could-not-check is never a pass, and a
		// caller scripting on exit code must see the difference between "every claim alive"
		// and "some claim unknown".
		return deskkit.Unverifiable(fmt.Sprintf(
			"%d of %d claim(s) were COULD-NOT-CHECK (action=BLIND) — the tick ran, the reading is incomplete",
			blindCount(results), len(results)), nil)
	}
	return nil
}

func blindCount(results []sweepResult) int {
	n := 0
	for _, r := range results {
		if r.Blind {
			n++
		}
	}
	return n
}

// policyOverrideFile is the JSON shape of <StateDir>/liveness.json — a PARTIAL override of
// loopengine.DefaultLivenessPolicy(): an absent field keeps the default. Durations are
// plain strings ("10m") for operator readability, parsed with time.ParseDuration, never
// raw nanosecond integers.
type policyOverrideFile struct {
	ScheduleToStart string            `json:"scheduleToStart,omitempty"`
	StartToClose    map[string]string `json:"startToClose,omitempty"` // tier name -> duration
	HeartbeatGap    string            `json:"heartbeatGap,omitempty"`
}

// loadPolicy returns loopengine.DefaultLivenessPolicy(), overlaid with
// <StateDir>/liveness.json when it exists. A missing file is not an error (the documented
// default applies); an unreadable or malformed one IS — a knob this tool cannot positively
// read is never silently ignored in favour of a default the operator may not have intended.
func loadPolicy() (loopengine.LivenessPolicy, error) {
	pol := loopengine.DefaultLivenessPolicy()

	stateDir, err := deskkit.StateDir()
	if err != nil {
		return pol, deskkit.Unverifiable("cannot resolve the desk-tools state directory for liveness.json", err)
	}
	path := filepath.Join(stateDir, "liveness.json")
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return pol, nil
		}
		return pol, deskkit.Unverifiable("cannot read "+path, rerr)
	}

	var ov policyOverrideFile
	if merr := json.Unmarshal(b, &ov); merr != nil {
		return pol, deskkit.Unverifiable("cannot parse "+path+" as liveness policy JSON", merr)
	}
	if ov.ScheduleToStart != "" {
		d, derr := time.ParseDuration(ov.ScheduleToStart)
		if derr != nil {
			return pol, deskkit.Unverifiable(path+": scheduleToStart "+ov.ScheduleToStart+" is not a duration", derr)
		}
		pol.ScheduleToStart = d
	}
	if ov.HeartbeatGap != "" {
		d, derr := time.ParseDuration(ov.HeartbeatGap)
		if derr != nil {
			return pol, deskkit.Unverifiable(path+": heartbeatGap "+ov.HeartbeatGap+" is not a duration", derr)
		}
		pol.HeartbeatGap = d
	}
	if len(ov.StartToClose) > 0 {
		merged := map[loopengine.Tier]time.Duration{}
		for k, v := range pol.StartToClose {
			merged[k] = v
		}
		for name, s := range ov.StartToClose {
			tier, terr := tierOf(name)
			if terr != nil {
				return pol, deskkit.Unverifiable(path+": startToClose key "+name+": "+terr.Error(), terr)
			}
			d, derr := time.ParseDuration(s)
			if derr != nil {
				return pol, deskkit.Unverifiable(path+": startToClose["+name+"] "+s+" is not a duration", derr)
			}
			merged[tier] = d
		}
		pol.StartToClose = merged
	}
	return pol, nil
}
