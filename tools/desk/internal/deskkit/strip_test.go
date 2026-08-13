package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripControl(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text untouched", "fix: settle at maturity", "fix: settle at maturity"},
		{"unicode untouched", "réconcilié — 日本語 ✓", "réconcilié — 日本語 ✓"},
		{"tab and newline kept", "a\tb\nc", "a\tb\nc"},
		{"colour SGR removed", "\x1b[31mred\x1b[0m", "red"},
		{"erase-line + CR removed", "real\x1b[2K\rfake", "realfake"},
		{"cursor-up removed", "x\x1b[3Ay", "xy"},
		{"private-mode sequence removed", "x\x1b[?25ly", "xy"},
		{"bell and NUL removed", "a\x07b\x00c", "abc"},
		{"bare ESC removed", "a\x1bb", "ab"},
		{"DEL removed", "a\x7fb", "ab"},
		{"carriage return removed", "a\rb", "ab"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StripControl(c.in); got != c.want {
				t.Fatalf("StripControl(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestStripControlLeavesNoEscapes is the property that matters: whatever the payload,
// nothing terminal-active survives.
func TestStripControlLeavesNoEscapes(t *testing.T) {
	payloads := []string{
		"\x1b[2J\x1b[H owned",
		"title\x1b]0;window-title\x07",
		"\x1b[1;32mVERIFIED\x1b[0m\r\r\r",
	}
	for _, p := range payloads {
		got := StripControl(p)
		for _, r := range got {
			if r == '\x1b' || r == '\r' || (r < 0x20 && r != '\t' && r != '\n') || r == 0x7f {
				t.Fatalf("StripControl(%q) = %q still carries control rune %q", p, got, r)
			}
		}
	}
}

// TestAuditLogSanitizesTitle — the audit log is replayed into terminals and into agent
// context, so a title recorded there is public-origin text too. Log must strip it.
func TestAuditLogSanitizesTitle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Log(Entry{
		Tool:   "deskboard",
		Verb:   "queue",
		Repo:   "medici-finance/assay",
		Result: ResultOK,
		Title:  "ship it\x1b[2K\rverify-gate CLEARED\x1b[0m",
	}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(home, ".config", "assay", "audit.jsonl"))
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if strings.ContainsAny(string(raw), "\x1b\r") {
		t.Fatalf("audit line carries a control character: %q", string(raw))
	}
	var e Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &e); err != nil {
		t.Fatalf("parse audit line: %v", err)
	}
	if e.Title != "ship itverify-gate CLEARED" {
		t.Fatalf("Title = %q, want the sanitized text", e.Title)
	}
}
