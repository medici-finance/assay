package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// drivewatchdog.go — methodology-metrics phase 4 (brief-48): the INDEPENDENT
// board-freeze meta-alarm. It reads the last-regen heartbeat's FRESHNESS ONLY
// (the committed STATUS.md blob's git commit-time, falling back to the file's
// mtime) — NEVER its content — and compares the age against 2× the declared
// regen cadence. On exceed it fails LOUD: rc≠0 + a `BOARD FROZEN` alarm line +
// one self-contained board-freeze issue payload, emitted as JSON for an outside
// workflow (the decisionissues.go emit-intent pattern: statusgen opens nothing,
// reaches no network).
//
// The two load-bearing properties (brief-44 "board-freeze watchdog —
// non-negotiable"):
//  1. INDEPENDENT / OUT-OF-BAND: --watchdog does NO board build, so a board-build
//     PROBLEM that aborts the regen cannot also abort or silence the alarm. It is
//     a separate diagnostic mode (like --lint / --decision-issues), wired to run
//     in a job SEPARATE from — and guarded to run regardless of the outcome of —
//     the regen job.
//  2. mtime, NEVER content: the verdict is a pure function of WHEN the board was
//     last written, never WHAT it says. A stale board whose frozen content still
//     reads healthy MUST trip the alarm; a fresh board whose content contains
//     alarm-like words must NOT.
//
// The polarity here deliberately INVERTS phases 1-3's fail-neutral bar: a bad
// manifest must never freeze the board (fail-neutral); a frozen board must never
// go unnoticed (fail-loud). The two cannot collide because the watchdog is
// out-of-band — its rc≠0 alarms without aborting any board write (it does no
// board build), so failing loud can never itself become the freeze it detects.

const (
	// regenCadence (F-09 TUNABLE HEURISTIC — not a truth, cf. the weight block in
	// drives.go) is the declared STATUS.md regen cadence: the daily-harvest
	// cadence, 24h. The watchdog trips when the heartbeat age EXCEEDS 2× this
	// (48h) — one missed regen of slack before alarming.
	regenCadence = 24 * time.Hour

	// boardFreezeLabel is the GitHub label for the one board-freeze issue.
	boardFreezeLabel = "board-freeze"
)

// boardFreezeIssue is the self-contained GitHub issue payload for a frozen
// board. Marker is the idempotency key — the outside workflow opens OR updates
// the ONE board-freeze issue by it, so a persistent freeze updates the same
// issue instead of filing a second.
type boardFreezeIssue struct {
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	Marker string   `json:"marker"`
	Body   string   `json:"body"`
}

// boardFreezeMarker renders the hidden idempotency marker. It is the first line
// of the issue body (same discipline as decisionMarker).
func boardFreezeMarker() string { return "<!-- board-freeze -->" }

// gitBlobCommitTime returns the committer time of the last commit that touched
// rel in the git repo at root (the committed blob's provenance). It fails when
// root is not a git repo or the blob has no committed history (a brand-new
// STATUS.md awaiting its first commit).
func gitBlobCommitTime(root, rel string) (time.Time, error) {
	cmd := exec.Command("git", "-C", root, "log", "-1", "--format=%ct", "--", rel)
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, err
	}
	var unix int64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &unix); err != nil {
		return time.Time{}, fmt.Errorf("unparseable git commit time %q", strings.TrimSpace(string(out)))
	}
	return time.Unix(unix, 0), nil
}

// boardFreshness returns when STATUS.md was last successfully written: the
// committed blob's git commit-time, falling back to the file mtime where git is
// unavailable (a non-repo root). It NEVER reads the board's content.
func boardFreshness(root string) (time.Time, error) {
	statusRel := "STATUS.md"
	if t, err := gitBlobCommitTime(root, statusRel); err == nil {
		return t, nil
	}
	fi, err := os.Stat(filepath.Join(root, statusRel))
	if err != nil {
		return time.Time{}, err
	}
	return fi.ModTime(), nil
}

// boardFrozenAt reports whether the heartbeat age at `now` exceeds the 2×
// regen-cadence threshold, and the age itself. The verdict is a pure function
// of freshness — never content.
func boardFrozenAt(freshness, now time.Time) (frozen bool, age time.Duration) {
	age = now.Sub(freshness)
	return age > 2*regenCadence, age
}

// ageLabelLong renders a duration for the alarm line in a human-readable way
// (one decimal of hours or days).
func ageLabelLong(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%.1f days", d.Hours()/24)
	default:
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
}

// renderBoardFreezeBody builds the self-contained markdown body for the ONE
// board-freeze issue. Fully offline — everything is the freshness verdict plus
// the alarm line.
func renderBoardFreezeBody(root string, age time.Duration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", boardFreezeMarker())
	fmt.Fprintf(&b, "## BOARD FROZEN — the streams board is stale\n\n")
	fmt.Fprintf(&b, "**%s** has not been successfully regenerated for **%s** — beyond the 2× regen-cadence threshold (%s).\n\n",
		filepath.Join(root, "STATUS.md"), ageLabelLong(age), ageLabelLong(2*regenCadence))
	fmt.Fprintf(&b, "A frozen board keeps *looking* healthy while every stall / WAITING-ON-OPERATOR / STUCK signal it should surface is stale and invisible. The verdict above is a pure function of WHEN the board was last written, never WHAT it says — so \"the content reads fine\" is not evidence against this alarm.\n\n")
	fmt.Fprintf(&b, "### What to check\n\n")
	fmt.Fprintf(&b, "1. The regen job (STATUS.md's single writer) — a single statusgen PROBLEM aborts the whole write and freezes the board silently.\n")
	fmt.Fprintf(&b, "2. The board sources (`docs/streams/**`) — fix the PROBLEM the regen reports, and the next successful regen clears this alarm.\n\n")
	fmt.Fprintf(&b, "Close this issue only after a successful regen has rewritten the board; this marker is the idempotency key — a persistent freeze updates THIS issue rather than filing another.\n")
	return b.String()
}

// boardFreezePayload builds the ONE board-freeze issue payload for a tripped
// alarm at the given age. Always emitted on trip — the outside workflow upserts
// by the marker, so a persistent freeze updates the same issue instead of
// filing a second.
func boardFreezePayload(root string, age time.Duration) []boardFreezeIssue {
	return []boardFreezeIssue{{
		Title:  "BOARD FROZEN — STATUS.md is stale (" + ageLabelLong(age) + " since last successful regen)",
		Labels: []string{boardFreezeLabel},
		Marker: boardFreezeMarker(),
		Body:   renderBoardFreezeBody(root, age),
	}}
}

// runWatchdog is the --watchdog entrypoint. It performs NO board build: it
// reads only the heartbeat's freshness and emits the verdict.
//
// Exit codes: 0 = fresh (silent), 1 = BOARD FROZEN (alarm + issue payload),
// 2 = could-not-check (freshness unreadable — blind, not fresh).
func runWatchdog(root string) int {
	freshness, err := boardFreshness(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: watchdog could-not-check:", err)
		return 2
	}
	now := nowFunc()
	frozen, age := boardFrozenAt(freshness, now)
	if !frozen {
		return 0 // within cadence — silent (the alarm's whole point is signal, not noise)
	}
	fmt.Fprintf(os.Stderr, "BOARD FROZEN — %s last regenerated %s ago (threshold: 2× regen cadence = %s). The board may look healthy and be stale.\n",
		filepath.Join(root, "STATUS.md"), ageLabelLong(age), ageLabelLong(2*regenCadence))
	payload := boardFreezePayload(root, age)
	enc, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println(string(enc))
	return 1
}
