package main

import (
	"path/filepath"
	"testing"
	"time"
)

// These tests pin the intake board line's three-state honesty when the per-entry
// docs/streams/intake/ directory is absent. Before the monolithic fallback, a
// missing directory made loadIntake return an empty set, which the board rendered
// as "the front door is clear" — a confident negative over a register that was
// never read (docs/three-state-instrument-rule.md, sub-rule 1). The findings
// register already reads its monolithic docs/streams/FINDINGS.md in that case;
// intake now does the same via docs/streams/INTAKE.md.

const intakeLegacyWithUntriaged = `# Intake

Front door for raw ideas. Same append-only + sequence rules as FINDINGS.

## I-legacy-new — 2026-07-08 — An untriaged idea

A raw idea nobody has triaged yet.

Disposition: new

## I-legacy-scoped — 2026-07-09 — An already-triaged idea

This one was scoped into a stream, so it must NOT count as untriaged.

Disposition: scoped → somestream
`

// TestLoadIntakeLegacyFallbackNotClear is the fail-first guard: the per-entry
// directory is absent but the monolithic INTAKE.md carries a Disposition: new
// entry. loadIntake must read that entry (not round to an empty set), and the
// board line must report the untriaged depth rather than "the front door is
// clear". Before the fix, loadIntake returned (nil, nil) here and the board
// falsely read clear.
func TestLoadIntakeLegacyFallbackNotClear(t *testing.T) {
	root := t.TempDir()
	streamsDir := filepath.Join(root, "docs", "streams")
	mustMkdirAll(t, streamsDir)
	writeTemp(t, streamsDir, "INTAKE.md", intakeLegacyWithUntriaged)
	// Note: NO docs/streams/intake/ directory is created.

	entries, err := loadIntake(root)
	if err != nil {
		t.Fatalf("loadIntake returned an error over a readable monolithic register: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries parsed from the monolithic INTAKE.md, got %d: %+v", len(entries), entries)
	}

	res := intakeAlarm(entries, time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC))
	if res.Untriaged != 1 {
		t.Errorf("expected exactly 1 untriaged entry (I-legacy-new; the scoped one must not count), got %d", res.Untriaged)
	}
	line := intakeBoardLine(res)
	if line == "_0 untriaged entries — the front door is clear._" {
		t.Errorf("board line falsely reads clear over a monolithic register with an untriaged entry: %q", line)
	}
	if res.Unreadable {
		t.Errorf("a readable monolithic register must be a measured read, not could-not-check")
	}
}

// TestLoadIntakeNeitherRegisterCouldNotCheck: with NEITHER the per-entry
// directory NOR the monolithic INTAKE.md present, the untriaged set is genuinely
// undetermined. loadIntake must return an error (which the board renders as
// could-not-check) rather than an empty set that reads clear.
func TestLoadIntakeNeitherRegisterCouldNotCheck(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "docs", "streams"))
	// No intake/ directory, no INTAKE.md.

	if _, err := loadIntake(root); err == nil {
		t.Fatalf("loadIntake must error (could-not-check) when neither intake register exists, got nil error")
	}
}

// TestLoadIntakeLegacyEmptyMonolithClear guards against the opposite regression:
// a repo whose monolithic INTAKE.md genuinely holds no entries (this repo's own
// bootstrap state) must still read as a clean, MEASURED clear — not a false alarm.
func TestLoadIntakeLegacyEmptyMonolithClear(t *testing.T) {
	root := t.TempDir()
	streamsDir := filepath.Join(root, "docs", "streams")
	mustMkdirAll(t, streamsDir)
	writeTemp(t, streamsDir, "INTAKE.md", "# Intake\n\nFront door for raw ideas.\n\nNo intake entries yet.\n")

	entries, err := loadIntake(root)
	if err != nil {
		t.Fatalf("loadIntake errored over an empty-but-present monolithic register: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries from an empty monolithic register, got %d: %+v", len(entries), entries)
	}
	res := intakeAlarm(entries, time.Now())
	if res.Unreadable {
		t.Errorf("an empty-but-present register is a measured empty, not could-not-check")
	}
	if got := intakeBoardLine(res); got != "_0 untriaged entries — the front door is clear._" {
		t.Errorf("empty register board line = %q, want the clear line", got)
	}
}

// TestLoadIntakePerEntryDirWins confirms the per-entry directory remains the
// source of truth when it exists: an empty per-entry directory is a legitimate
// measured empty and must NOT trip the monolithic fallback (even if an INTAKE.md
// view is also present).
func TestLoadIntakePerEntryDirWins(t *testing.T) {
	root := t.TempDir()
	streamsDir := filepath.Join(root, "docs", "streams")
	mustMkdirAll(t, filepath.Join(streamsDir, "intake"))
	// A stale monolithic view with an entry must be IGNORED while the per-entry
	// directory exists — the directory is authoritative.
	writeTemp(t, streamsDir, "INTAKE.md", intakeLegacyWithUntriaged)

	entries, err := loadIntake(root)
	if err != nil {
		t.Fatalf("loadIntake errored over an empty per-entry directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("per-entry directory is authoritative and empty; expected 0 entries, got %d: %+v", len(entries), entries)
	}
}

// TestParseIntakeLegacyDispositionCaseInsensitive verifies that parseIntakeLegacy
// matches the disposition key case-insensitively (e.g. "disposition: accepted" or
// "Disposition: accepted"), so a lowercase or mixed-case key is not silently
// dropped and converted to the untriaged "new" default (issue #915).
func TestParseIntakeLegacyDispositionCaseInsensitive(t *testing.T) {
	cases := []struct {
		name          string
		keyLine       string
		wantDisp      string
		wantUntriaged int
	}{
		{
			name:          "canonical TitleCase Disposition:",
			keyLine:       "Disposition: scoped → somestream",
			wantDisp:      "scoped → somestream",
			wantUntriaged: 0,
		},
		{
			name:          "lowercase disposition:",
			keyLine:       "disposition: accepted",
			wantDisp:      "accepted",
			wantUntriaged: 0,
		},
		{
			name:          "mixed case DisPosition:",
			keyLine:       "DisPosition: rejected",
			wantDisp:      "rejected",
			wantUntriaged: 0,
		},
		{
			name:          "lowercase disposition: new",
			keyLine:       "disposition: new",
			wantDisp:      "new",
			wantUntriaged: 1,
		},
		{
			name:          "leading whitespace before key",
			keyLine:       "  disposition: accepted",
			wantDisp:      "accepted",
			wantUntriaged: 0,
		},
		{
			name:          "tab separator after colon",
			keyLine:       "Disposition:\taccepted",
			wantDisp:      "accepted",
			wantUntriaged: 0,
		},
		{
			name:          "no colon does not match and defaults to new",
			keyLine:       "Disposition accepted",
			wantDisp:      "new",
			wantUntriaged: 1,
		},
		{
			name:          "longer key prefix does not match and defaults to new",
			keyLine:       "DispositionX: accepted",
			wantDisp:      "new",
			wantUntriaged: 1,
		},
		{
			name:          "missing disposition key defaults to new",
			keyLine:       "",
			wantDisp:      "new",
			wantUntriaged: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			streamsDir := filepath.Join(root, "docs", "streams")
			mustMkdirAll(t, streamsDir)
			content := "# Intake\n\n## I-case-test — 2026-07-08 — Test entry\n\nSome body.\n\n"
			if tc.keyLine != "" {
				content += tc.keyLine + "\n"
			}
			writeTemp(t, streamsDir, "INTAKE.md", content)

			entries, err := loadIntake(root)
			if err != nil {
				t.Fatalf("loadIntake error: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}
			if entries[0].Disposition != tc.wantDisp {
				t.Errorf("expected Disposition %q, got %q", tc.wantDisp, entries[0].Disposition)
			}
			res := intakeAlarm(entries, time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC))
			if res.Untriaged != tc.wantUntriaged {
				t.Errorf("expected Untriaged = %d, got %d", tc.wantUntriaged, res.Untriaged)
			}
		})
	}
}

