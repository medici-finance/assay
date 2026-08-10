package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	validStreamStatus = map[string]bool{"active": true, "paused": true, "done": true}
	validPriority     = map[string]bool{"P0": true, "P1": true, "P2": true}
	validTrack        = map[string]bool{"": true, "product": true, "platform": true, "ecosystem": true}
	validBriefStatus  = map[string]bool{"todo": true, "in-progress": true, "implemented": true, "verified": true, "done": true, "blocked": true}
)

// ackRe matches a well-formed desk acknowledgement: "YYYY-MM-DD <who>" (F-09).
var ackRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \S`)

// check returns hard problems (exit 1) and non-fatal notices (printed, exit 0).
// Notices exist for exactly one case today: an unresolved finding targeting an
// in-flight brief that the desk has not acked — visible, but never blocking
// (the F-09 demotion-DoS mitigation, methodology/04).
//
// This is the whole-house entry point: per-stream checks and the findings
// affects: known-stream validation both run against the same `streams` set.
func check(streams []*Stream, findings []Finding) ([]string, []string) {
	return checkScoped(streams, streams, findings)
}

// checkScoped runs the source checks with product-scoping (mm/31+32) applied.
// `streams` is the (possibly narrowed) scoped set that drives the per-stream and
// per-brief checks; `allStreams` is the FULL stream universe used ONLY to
// validate that a finding's affects: names a real stream. Findings legitimately
// affect ANY product's stream regardless of which product a PR touches, so the
// known-stream existence check must resolve against all streams, never the
// scoped subset — otherwise a single-product PR falsely flags every finding
// referencing another product's stream as "unknown stream" (#834). Per-brief
// checks (demotion, unknown-brief) stay scoped: they only run for streams in the
// scoped `streams` set, so one product's in-flight finding never gates another's PR.
func checkScoped(streams, allStreams []*Stream, findings []Finding) ([]string, []string) {
	var problems, notices []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	// allByName carries the full stream universe for the findings affects:
	// known-stream existence check only (#834); it is never narrowed by scope.
	allByName := map[string]*Stream{}
	for _, s := range allStreams {
		allByName[s.Name] = s
	}
	byName := map[string]*Stream{}
	for _, s := range streams {
		byName[s.Name] = s
		if s.Name != filepath.Base(s.Dir) {
			add("%s: frontmatter stream %q != directory name %q", s.Dir, s.Name, filepath.Base(s.Dir))
		}
		if !validStreamStatus[s.Status] {
			add("%s: invalid stream status %q", s.Name, s.Status)
		}
		if s.Status == "done" {
			add("%s: done streams must be moved to docs/archive/", s.Name)
		}
		if s.Status == "active" && s.Priority == "" {
			add("%s: an active stream requires a priority", s.Name)
		}
		if s.Priority != "" && !validPriority[s.Priority] {
			add("%s: invalid priority %q", s.Name, s.Priority)
		}
		if !validTrack[s.Track] {
			add("%s: invalid track %q", s.Name, s.Track)
		}
		if s.Tiering != nil && strings.TrimSpace(*s.Tiering) == "" {
			add("%s: tiering present but empty — omit the field or give it a value", s.Name)
		}
		if s.MaxConcurrent != nil {
			n := *s.MaxConcurrent
			if n < 1 || n > perStreamCap {
				add("%s: max-concurrent %d out of range — must be 1..%d (methodology-metrics/13)", s.Name, n, perStreamCap)
			}
		}
		seen := map[string]bool{}
		for _, b := range s.Briefs {
			if seen[b.Num] {
				add("%s: duplicate brief number %q", s.Name, b.Num)
			}
			seen[b.Num] = true
			if !validBriefStatus[b.Status] {
				add("%s/brief-%s: invalid status %q", s.Name, b.Num, b.Status)
			}
			// Placeholder-v1 rows are exempt from the Verified/Reviewed gate —
			// their lifecycle is issue-driven (issue-loop/04 close-out), not
			// human-verified. A done placeholder carries `resolved: issue-close`
			// instead.
			if b.Schema != "placeholder-v1" {
				if b.Status == "done" && (b.Verified == "" || b.Reviewed == "") {
					add("%s/brief-%s: status done requires Verified and Reviewed entries", s.Name, b.Num)
				}
				if b.Status == "verified" && b.Verified == "" {
					add("%s/brief-%s: status verified requires a Verified entry", s.Name, b.Num)
				}
			}
		}
	}
	for _, f := range findings {
		// Resolved findings are fully inert — even a malformed Ack in an
		// archived entry must not hard-error (nobody should be forced to
		// edit register history to make the checker pass).
		if !f.Resolved && f.Ack != "" && !ackRe.MatchString(f.Ack) {
			add("%s: malformed Ack %q — want `Ack: YYYY-MM-DD <who>`", f.ID, f.Ack)
		}
		for _, a := range f.Affects {
			parts := strings.SplitN(a, "/", 2)
			s, ok := byName[parts[0]]
			if !ok {
				// The stream is not in the scoped set. It may still be a real
				// stream that is simply outside this PR's product scope — a
				// finding legitimately affects any product's stream (#834), so
				// resolve existence against the FULL universe, not the scoped
				// subset. Only a stream absent from allByName is truly unknown.
				// A resolved finding's refs are historical: its stream may since
				// have been archived (e.g. F-05 → operator). Only an UNRESOLVED
				// finding must point at a live stream. Per-brief checks below are
				// intentionally skipped for out-of-scope streams (scoping intent).
				if !f.Resolved {
					if _, known := allByName[parts[0]]; !known {
						add("%s: Affects references unknown stream %q", f.ID, parts[0])
					}
				}
				continue
			}
			if len(parts) == 2 {
				num := strings.TrimPrefix(parts[1], "brief-")
				found := false
				for _, b := range s.Briefs {
					if b.Num == num {
						found = true
						if f.Resolved || (b.Status != "in-progress" && b.Status != "implemented") {
							continue
						}
						// A malformed Ack is not an ack: it already errored
						// above and falls through to the awaits-ack notice.
						if ackRe.MatchString(f.Ack) {
							add("%s/brief-%s: unresolved %s (desk-acked) — demote to todo (re-gate) or resolve the finding", s.Name, num, f.ID)
						} else {
							notices = append(notices, fmt.Sprintf("%s/brief-%s: unresolved %s awaits desk ack", s.Name, num, f.ID))
						}
					}
				}
				if !found && !f.Resolved {
					add("%s: Affects references unknown brief %q in stream %s", f.ID, parts[1], s.Name)
				}
			}
		}
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

func applyFindings(streams []*Stream, findings []Finding) {
	for _, f := range findings {
		if f.Resolved {
			continue
		}
		for _, a := range f.Affects {
			parts := strings.SplitN(a, "/", 2)
			for _, s := range streams {
				if s.Name != parts[0] {
					continue
				}
				for i := range s.Briefs {
					if len(parts) == 1 || s.Briefs[i].Num == strings.TrimPrefix(parts[1], "brief-") {
						s.Briefs[i].StaleRef = f.ID
					}
				}
			}
		}
	}
}

// freshnessCheckNotices returns a NOTICE for each brief-v1 file in a hardening stream
// (stream name contains "hardening"), authored after the freshness-rule cutoff date,
// whose sources: lacks a `freshness-checked` token. This is an advisory reminder that
// the authoring pre-flight rule in the author-brief skill applies — the skill rule is
// the real enforcement, this NOTICE just surfaces the gap so the desk sees it.
// methodology/26.
func freshnessCheckNotices(streams []*Stream) []string {
	// Cutoff: briefs authored on or after this date are expected to carry a
	// freshness-checked token. Matches the date the methodology/26 rule merged.
	const cutoff = "2026-07-11"
	cutoffDate, err := time.Parse("2006-01-02", cutoff)
	if err != nil {
		return nil // should be impossible with a literal
	}

	var notices []string
	for _, s := range streams {
		if !strings.Contains(s.Name, "hardening") {
			continue
		}
		for _, path := range briefFilePaths(s) {
			bf, ok, ferr := parseBriefFile(path)
			if ferr != nil || !ok {
				continue // malformed/legacy are handled by other checks
			}
			// Extract the date portion of the authored field. Format is
			// "YYYY-MM-DD by <who>..." — take the first 10 characters.
			authoredDate := ""
			if len(bf.Authored) >= 10 {
				authoredDate = bf.Authored[:10]
			}
			authoredTime, parseErr := time.Parse("2006-01-02", authoredDate)
			if parseErr != nil {
				continue // can't determine date; skip
			}
			if !authoredTime.After(cutoffDate) {
				continue // authored before the rule existed
			}
			hasFresh := false
			for _, src := range bf.Sources {
				if strings.HasPrefix(src, "freshness-checked") {
					hasFresh = true
					break
				}
			}
			if !hasFresh {
				notices = append(notices, fmt.Sprintf(
					"%s: brief %s in hardening stream %s, authored %s, lacks a freshness-checked token in sources — add `freshness-checked YYYY-MM-DD @ <short-sha>` per the author-brief skill (methodology/26)",
					path, bf.Brief, s.Name, authoredDate))
			}
		}
	}
	sort.Strings(notices)
	return notices
}
