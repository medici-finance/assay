package main

// opmetricsstale.go — a --lint NOTICE when the operator-load day-file has gone
// stale.
//
// The mm/40 opmetrics collector runs on the operator's machine and lands one
// day-file per day under docs/reports/daily/<date>/opmetrics.json. A daily-skill
// roster line is a REMINDER, not a control: if the operator forgets the run, the
// board goes quietly unmeasured and nobody is told. This check is the CODE that
// makes the gap loud — a NOTICE in every --lint run when the newest day-file is
// older than the freshness window, so a missed run surfaces on its own.
//
// SEVERITY IS NOTICE, deliberately. A stale day-file is not a board falsification
// and never blocks the pipeline (exit stays 0); it is a prompt to run the
// collector. It follows the same advisory-first precedent as the other
// freshness/aging notices in this package.
//
// A ROOT THAT HAS NEVER CARRIED ONE IS SILENT. An adopter who has not stood the
// collector up is not nagged about a routine they never adopted — the NOTICE
// fires only on a root that HAS a day-file, and only when the newest has aged out.

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// opmetricsStaleDays is the freshness window: a day-file whose date is more than
// this many days before `now` is stale. The collector is a daily routine, so a
// window of 3 tolerates a weekend or a skipped day before it complains.
const opmetricsStaleDays = 3

// opmetricsStaleNotices returns a single NOTICE when the newest opmetrics day-file
// under root is older than opmetricsStaleDays. It returns nil (silent) when the
// root carries no day-file at all, and nil when the newest is fresh. now is passed
// so the check is deterministic under test.
func opmetricsStaleNotices(root string, now time.Time) []string {
	newest, ok := newestOpmetricsDate(root)
	if !ok {
		return nil // a root that has never carried one is not nagged
	}
	// Measure from the day-file's date (midnight, its own nominal day) to now.
	dayStart := time.Date(newest.Year(), newest.Month(), newest.Day(), 0, 0, 0, 0, now.Location())
	ageDays := int(now.Sub(dayStart).Hours() / 24)
	if ageDays <= opmetricsStaleDays {
		return nil
	}
	return []string{fmt.Sprintf(
		"opmetrics-stale: the newest operator-load day-file (docs/reports/daily/%s/opmetrics.json) is %d days old (> %d) — run the mm/40 opmetrics collector so the operator-load metric and the adoption ladder's rungs 3–4 stop reading unmeasured",
		newest.Format("2006-01-02"), ageDays, opmetricsStaleDays)}
}

// newestOpmetricsDate returns the most recent day (by directory name) under
// docs/reports/daily/ that actually contains an opmetrics.json, and whether any
// exists at all.
func newestOpmetricsDate(root string) (time.Time, bool) {
	dir := filepath.Join(root, "docs", "reports", "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, false
	}
	var newest time.Time
	found := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d, perr := time.Parse("2006-01-02", e.Name())
		if perr != nil {
			continue
		}
		if _, serr := os.Stat(filepath.Join(dir, e.Name(), "opmetrics.json")); serr != nil {
			continue
		}
		if !found || d.After(newest) {
			newest, found = d, true
		}
	}
	return newest, found
}
