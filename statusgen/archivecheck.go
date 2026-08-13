package main

import (
	"fmt"
	"path/filepath"
	"sort"
)

// checkArchivedPlaceholders implements the two ADDITIVE lint checks
// for the per-stream `done/` archive convention (D3). It lives OUTSIDE
// registers.go — the tombstone/anti-falsification guard is a human-gate surface
// this brief must not touch — and is wired into the whole-house aggregation in
// main.go alongside checkPlaceholderFiles.
//
//   - NOTICE (a): a `status: done` placeholder still sitting at a stream ROOT is
//     an archive candidate — the scanner's RETIRE/sweep will move it into done/.
//     Non-blocking (exit stays 0): the drain is a scan action, not a lint fix.
//   - PROBLEM (b): a brief or placeholder file UNDER a `done/` subfolder whose
//     status is NOT done is active work parked out of sight — the board never
//     shows it (root-only discovery), so it must never be archived while live.
//
// Discovery for (b) is deliberately its own recursion-free glob of `<stream>/done/`:
// the normal brief/placeholder discovery globs the stream ROOT only, so nothing
// else in the pipeline ever reads the archive.
func checkArchivedPlaceholders(streams []*Stream) (problems, notices []string) {
	prob := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	note := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }

	for _, s := range streams {
		// (a) NOTICE: a done placeholder still at the stream ROOT — the scanner
		// archives it on the next --scan-issues sweep. s.Placeholders is the
		// ROOT-only parsed set (attachPlaceholders), so any done one is a ghost.
		for _, ph := range s.Placeholders {
			if ph.Status == "done" {
				note("%s: retired placeholder (status: done) at the stream root — archive candidate, run `statusgen --scan-issues` to move it into %s/", ph.Path, archiveDirName)
			}
		}

		// (b) PROBLEM: a placeholder under done/ that is NOT done.
		for _, path := range archivedPlaceholderFilePaths(s) {
			ph, ok, err := parsePlaceholderFile(path)
			if err != nil {
				prob("%s", err.Error()) // already path-prefixed
				continue
			}
			if !ok {
				continue
			}
			if ph.Status != "done" {
				prob("%s: placeholder under %s/ has status %q — only `status: done` work may be archived (no parking active work out of sight)", path, archiveDirName, ph.Status)
			}
		}

		// (b) PROBLEM: a brief under done/ whose README row status is NOT done.
		// A brief file carries no self-status (the README table is the record),
		// so correlate the archived file's brief number to its stream row.
		byNum := map[string]string{} // brief Num → README-row status
		for _, b := range s.Briefs {
			byNum[b.Num] = b.Status
		}
		matches, _ := filepath.Glob(filepath.Join(s.Dir, archiveDirName, "brief-*.md"))
		sort.Strings(matches)
		for _, path := range matches {
			m := briefNameRe.FindStringSubmatch(filepath.Base(path))
			if m == nil {
				continue
			}
			status, known := byNum[m[1]]
			if known && status != "done" {
				prob("%s: brief under %s/ has README-row status %q — only `status: done` work may be archived (no parking active work out of sight)", path, archiveDirName, status)
			}
		}
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}
