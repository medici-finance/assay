package main

import (
	"fmt"
	"sort"
)

// streamArchiveReadyNotices implements the stream-LEVEL "archive-ready" surface
// (#947): a NON-BLOCKING NOTICE for every stream whose work is entirely complete,
// so the desk can see the whole stream is a candidate to move off the active
// board. It is the stream-level counterpart to checkArchivedPlaceholders' NOTICE
// (a), which flags a single retired placeholder still at the stream root.
//
// DETECT, DON'T AUTO-FLIP. This function only SURFACES the candidate — the actual
// archival stays a desk confirmation. The archival act is the one the rest of
// statusgen already enforces: set the stream's frontmatter `status: done` and move
// the stream directory out of docs/streams/ into docs/archive/ (checks.go rejects a
// `done` stream that is still under docs/streams/ — "done streams must be moved to
// docs/archive/"). This is a whole-STREAM move, distinct from the per-placeholder
// <stream>/done/ sweep that archivecheck.go governs. Archiving removes the stream
// from the active board (a visibility act), a "done" stream often has a fast-follow
// or a finding that reopens it, and auto-archiving on "all briefs done" would turn
// the metric into a target (Goodhart). So the pattern is: statusgen surfaces
// archive-ready → the desk confirms and archives. Notice-only by design: it never
// returns problems and so never changes the exit code.
//
// The predicate is deliberately strict and conservative:
//   - the stream must carry at least one real README-table brief (a
//     placeholder-only stream — e.g. the perpetual issue-loop intake stream — has
//     no README-table work to archive and is never a stream-archival candidate);
//   - EVERY brief in the stream must be `status: done`. The scan spans the whole
//     brief set — real README rows AND the synthetic placeholder rows
//     attachPlaceholders appends — so a single open placeholder issue (todo,
//     in-progress, blocked, …) correctly holds the stream off the archive-ready
//     list even when every README row is done.
//
// The signal clears when the stream leaves docs/streams/. loadStreams walks only
// docs/streams/, so once the desk moves the archived stream directory into
// docs/archive/ the stream is no longer loaded and the notice stops firing. Note
// s.Briefs is parsed from the README table (parseBriefTable), not the filesystem,
// so moving individual brief FILES does not empty it and would not clear the
// signal — the whole-stream move is what clears it. (An archived placeholder,
// globbed from the stream root by attachPlaceholders, does drop out of s.Briefs on
// its own, but that only concerns the placeholder half of the set.)
func streamArchiveReadyNotices(streams []*Stream) []string {
	var notices []string
	for _, s := range streams {
		realBriefs := 0
		allDone := true
		for _, b := range s.Briefs {
			if b.Schema != "placeholder-v1" {
				realBriefs++
			}
			if b.Status != "done" {
				allDone = false
			}
		}
		// A placeholder-only stream (no README-table brief) is never a
		// stream-archival candidate; a stream with any non-done brief is still
		// live work.
		if realBriefs == 0 || !allDone {
			continue
		}
		noun := "briefs"
		if realBriefs == 1 {
			noun = "brief"
		}
		notices = append(notices, fmt.Sprintf(
			"stream %s: all %d %s done — archive candidate; the desk confirms and archives (set the stream `status: done` and move docs/streams/%s to docs/archive/), never auto-flipped",
			s.Name, realBriefs, noun, s.Name))
	}
	sort.Strings(notices)
	return notices
}
