package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var resetNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

// resetCell writes a cell directory with the given cell.env body and pin record, and returns a
// Cell whose Env is read from that cell.env plus the tier map the reset reads its cheap default
// from.
func resetCell(t *testing.T, envBody string, pins map[string]pinRecord) *Cell {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cell.env"), []byte(envBody), 0o600); err != nil {
		t.Fatal(err)
	}
	savePinFile(dir, pinFile{Pins: pins})
	e := envWith(map[string]string{"TIER_MODEL_MID_CLAUDE": "sonnet", "TIER_MODEL_MID_CODEX": "gpt-mid"})
	return &Cell{Dir: dir, Env: e}
}

func stamp(daysAgo int) string {
	return resetNow.Add(-time.Duration(daysAgo) * 24 * time.Hour).Format(time.RFC3339)
}

func envLine(t *testing.T, c *Cell, key string) string {
	t.Helper()
	return effectiveCellEnv(filepath.Join(c.Dir, "cell.env"), nil).Get(key)
}

func TestModelResetSparesExplicitPin(t *testing.T) {
	body := "CELL=demo\nDESK_MODEL_worker_desk=fable\nDESK_MODEL_DEFAULT=fable\n"
	c := resetCell(t, body, map[string]pinRecord{
		"DESK_MODEL_worker_desk": {Kind: pinKindExplicit, Value: "fable", At: stamp(60)},
		// DESK_MODEL_DEFAULT has no record at all: unknown provenance reads as explicit too.
	})
	notices := c.resetAgedDefaultPins(7*24*time.Hour, resetNow, true)
	if len(notices) != 0 {
		t.Fatalf("explicit or unrecorded pins must not be repinned, got notices %q", notices)
	}
	got, _ := os.ReadFile(filepath.Join(c.Dir, "cell.env"))
	if string(got) != body {
		t.Errorf("cell.env changed:\n%s\nwant:\n%s", got, body)
	}
	if c.Env.Get("DESK_MODEL_worker_desk") == "sonnet" {
		t.Error("in-memory env was repinned")
	}
}

func TestModelResetRepinsAgedDefault(t *testing.T) {
	body := "CELL=demo\nDESK_MODEL_worker_desk=fable\nDESK_MODEL_pr_review_desk=fable\nDESK_MODEL_the_desk=fable\n"
	c := resetCell(t, body, map[string]pinRecord{
		"DESK_MODEL_worker_desk":    {Kind: pinKindDefault, Value: "fable", At: stamp(10)},
		"DESK_MODEL_pr_review_desk": {Kind: pinKindDefault, Value: "fable", At: stamp(2)}, // young
		"DESK_MODEL_the_desk":       {Kind: pinKindDefault, Value: "fable", At: stamp(30)},
	})
	notices := c.resetAgedDefaultPins(7*24*time.Hour, resetNow, true)
	if len(notices) != 1 {
		t.Fatalf("want exactly one notice, got %q", notices)
	}
	n := notices[0]
	if strings.Contains(n, "\n") || !strings.Contains(n, "worker-desk") ||
		!strings.Contains(n, "fable") || !strings.Contains(n, "sonnet") {
		t.Errorf("notice must be one line naming role, old and new model: %q", n)
	}
	if got := envLine(t, c, "DESK_MODEL_worker_desk"); got != "sonnet" {
		t.Errorf("aged default pin = %q, want sonnet", got)
	}
	if got := envLine(t, c, "DESK_MODEL_pr_review_desk"); got != "fable" {
		t.Errorf("young default pin moved to %q", got)
	}
	if got := envLine(t, c, "DESK_MODEL_the_desk"); got != "fable" {
		t.Errorf("the-desk pin moved to %q; the coordinator stays at the top tier", got)
	}
	// The repin restarts the clock: a second pass moves nothing.
	if again := c.resetAgedDefaultPins(7*24*time.Hour, resetNow, true); len(again) != 0 {
		t.Errorf("second pass repinned again: %q", again)
	}
}

func TestModelResetHandEditRestartsClock(t *testing.T) {
	// The record says sonnet; cell.env was hand-edited to fable. The old age says nothing about
	// the new value, so the clock restarts rather than repinning on the stale stamp.
	c := resetCell(t, "DESK_MODEL_DEFAULT=fable\n", map[string]pinRecord{
		"DESK_MODEL_DEFAULT": {Kind: pinKindDefault, Value: "sonnet", At: stamp(90)},
	})
	if n := c.resetAgedDefaultPins(7*24*time.Hour, resetNow, true); len(n) != 0 {
		t.Fatalf("hand-edited pin repinned on a stale stamp: %q", n)
	}
	rec := loadPinFile(c.Dir).Pins["DESK_MODEL_DEFAULT"]
	if rec.Value != "fable" || rec.At != resetNow.Format(time.RFC3339) {
		t.Errorf("record not restarted: %+v", rec)
	}
}

func TestModelResetDryRunWritesNothing(t *testing.T) {
	body := "DESK_MODEL_DEFAULT=fable\n"
	c := resetCell(t, body, map[string]pinRecord{
		"DESK_MODEL_DEFAULT": {Kind: pinKindDefault, Value: "fable", At: stamp(20)},
	})
	if n := c.resetAgedDefaultPins(7*24*time.Hour, resetNow, false); len(n) != 1 {
		t.Fatalf("dry run should still plan the repin: %q", n)
	}
	if got, _ := os.ReadFile(filepath.Join(c.Dir, "cell.env")); string(got) != body {
		t.Errorf("dry run wrote cell.env: %s", got)
	}
	if c.Env.Get("DESK_MODEL_DEFAULT") != "sonnet" {
		t.Error("dry run must apply the repin to the in-memory env so the plan matches the launch")
	}
}

func TestModelResetTTLOffAndParse(t *testing.T) {
	c := resetCell(t, "DESK_MODEL_DEFAULT=fable\n", map[string]pinRecord{
		"DESK_MODEL_DEFAULT": {Kind: pinKindDefault, Value: "fable", At: stamp(400)},
	})
	if n := c.resetAgedDefaultPins(0, resetNow, true); len(n) != 0 {
		t.Errorf("ttl 0 is off, got %q", n)
	}
	for in, want := range map[string]time.Duration{"7d": 7 * 24 * time.Hour, "7": 7 * 24 * time.Hour, "36h": 36 * time.Hour, "0": 0} {
		if got, err := parseModelTTL(in); err != nil || got != want {
			t.Errorf("parseModelTTL(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "soon", "-3d"} {
		if _, err := parseModelTTL(bad); err == nil {
			t.Errorf("parseModelTTL(%q) accepted", bad)
		}
	}
	assertDies(t, "malformed cell.env TTL", func() { cellModelTTL(envWith(map[string]string{modelTTLKey: "soon"})) })
}

func TestSetRecordsPinKind(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cell.env")
	if err := os.WriteFile(p, []byte("CELL=demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := envWith(map[string]string{})
	captureStdout(t, func() {
		applyEnvKVs(e, p, false, []string{"DESK_MODEL_DEFAULT=fable"})
		applyEnvKVsKind(e, p, false, []string{"DESK_MODEL_worker_desk=fable"}, pinKindExplicit)
	})
	pins := loadPinFile(dir).Pins
	if pins["DESK_MODEL_DEFAULT"].Kind != pinKindDefault {
		t.Errorf("plain set recorded %+v, want default", pins["DESK_MODEL_DEFAULT"])
	}
	if pins["DESK_MODEL_worker_desk"].Kind != pinKindExplicit {
		t.Errorf("--explicit set recorded %+v, want explicit", pins["DESK_MODEL_worker_desk"])
	}
	// An explicit record is sticky: a later plain set updates the value, never the owner.
	captureStdout(t, func() { applyEnvKVs(e, p, false, []string{"DESK_MODEL_worker_desk=haiku"}) })
	rec := loadPinFile(dir).Pins["DESK_MODEL_worker_desk"]
	if rec.Kind != pinKindExplicit || rec.Value != "haiku" {
		t.Errorf("plain set over an explicit pin: %+v, want explicit/haiku", rec)
	}
	// A non-pin key leaves the record alone.
	captureStdout(t, func() { applyEnvKVs(e, p, false, []string{"CELL_COCKPIT=tmux"}) })
	if _, has := loadPinFile(dir).Pins["CELL_COCKPIT"]; has {
		t.Error("a non-pin key was recorded as a pin")
	}
}

func TestBootModelResetPrintsOneLine(t *testing.T) {
	c := resetCell(t, "DESK_MODEL_worker_desk=fable\n", map[string]pinRecord{
		"DESK_MODEL_worker_desk": {Kind: pinKindDefault, Value: "fable", At: stamp(20)},
	})
	out := captureStdout(t, func() { c.bootModelReset(resetNow) })
	if strings.Count(out, "\n") != 1 || !strings.HasPrefix(out, "[model-reset] worker-desk: fable -> sonnet") {
		t.Errorf("boot notice = %q", out)
	}
	if quiet := captureStdout(t, func() { c.bootModelReset(resetNow) }); quiet != "" {
		t.Errorf("a boot with nothing to repin must print nothing, got %q", quiet)
	}
}
