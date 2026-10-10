package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var resetNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

const week = 7 * 24 * time.Hour

// resetCell writes a cell directory with the given cell.env body and pin record, and returns a
// Cell whose Env is what the files say plus the launch-env tier entries loadCell would add.
func resetCell(t *testing.T, envBody string, pins map[string]pinRecord) *Cell {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "cell.env")
	if err := os.WriteFile(p, []byte(envBody), 0o600); err != nil {
		t.Fatal(err)
	}
	savePinFile(dir, pinFile{Pins: pins})
	e := effectiveCellEnv(p, nil)
	for k, v := range tierModelDefaults {
		if e.Get(k) == "" {
			e.Put(k, v)
		}
	}
	return &Cell{Dir: dir, Env: e}
}

func stamp(daysAgo int) string {
	return resetNow.Add(-time.Duration(daysAgo) * 24 * time.Hour).Format(time.RFC3339)
}

func aged(v string) pinRecord { return pinRecord{Kind: pinKindDefault, Value: v, At: stamp(30)} }

func envLine(t *testing.T, c *Cell, key string) string {
	t.Helper()
	return effectiveCellEnv(filepath.Join(c.Dir, "cell.env"), nil).Get(key)
}

func liveReset(t *testing.T, c *Cell) []pinMove {
	t.Helper()
	moves, _, err := c.resetAgedDefaultPins(week, resetNow, true, "")
	if err != nil {
		t.Fatal(err)
	}
	return moves
}

func fileOf(t *testing.T, c *Cell) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(c.Dir, "cell.env"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestModelResetSparesExplicitPin(t *testing.T) {
	body := "CELL=demo\nDESK_MODEL_worker_desk=fable\nDESK_MODEL_verify_desk=fable\n"
	c := resetCell(t, body, map[string]pinRecord{
		"DESK_MODEL_worker_desk": {Kind: pinKindExplicit, Value: "fable", At: stamp(60)},
		// DESK_MODEL_verify_desk has no record at all: unknown provenance reads as explicit too.
	})
	if moves := liveReset(t, c); len(moves) != 0 {
		t.Fatalf("explicit or unrecorded pins must not be repinned, got %v", moves)
	}
	if got := fileOf(t, c); got != body {
		t.Errorf("cell.env changed:\n%s\nwant:\n%s", got, body)
	}
	if c.Env.Get("DESK_MODEL_worker_desk") == "sonnet" {
		t.Error("in-memory env was repinned")
	}
}

func TestModelResetRepinsAgedDefault(t *testing.T) {
	body := "CELL=demo\nDESK_MODEL_worker_desk=fable\nDESK_MODEL_pr_review_desk=fable\nDESK_MODEL_the_desk=fable\n"
	c := resetCell(t, body, map[string]pinRecord{
		"DESK_MODEL_worker_desk":    aged("fable"),
		"DESK_MODEL_pr_review_desk": {Kind: pinKindDefault, Value: "fable", At: stamp(2)}, // young
		"DESK_MODEL_the_desk":       aged("fable"),
	})
	moves := liveReset(t, c)
	if len(moves) != 1 {
		t.Fatalf("want exactly one move, got %v", moves)
	}
	n := moves[0].notice()
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
	if again := liveReset(t, c); len(again) != 0 {
		t.Errorf("second pass repinned again: %v", again)
	}
}

func TestModelResetHandEditRestartsClock(t *testing.T) {
	// The record says sonnet; cell.env was hand-edited to fable. The old age says nothing about
	// the new value, so the clock restarts rather than repinning on the stale stamp.
	c := resetCell(t, "DESK_MODEL_worker_desk=fable\n", map[string]pinRecord{
		"DESK_MODEL_worker_desk": {Kind: pinKindDefault, Value: "sonnet", At: stamp(90)},
	})
	if n := liveReset(t, c); len(n) != 0 {
		t.Fatalf("hand-edited pin repinned on a stale stamp: %v", n)
	}
	rec := loadPinFile(c.Dir).Pins["DESK_MODEL_worker_desk"]
	if rec.Value != "fable" || rec.At != resetNow.Format(time.RFC3339) {
		t.Errorf("record not restarted: %+v", rec)
	}
}

func TestModelResetDryRunWritesNothing(t *testing.T) {
	body := "DESK_MODEL_worker_desk=fable\n"
	c := resetCell(t, body, map[string]pinRecord{"DESK_MODEL_worker_desk": aged("fable")})
	before, _ := os.ReadFile(filepath.Join(c.Dir, modelPinsFile))
	moves, backup, err := c.resetAgedDefaultPins(week, resetNow, false, "")
	if err != nil || len(moves) != 1 || backup != "" {
		t.Fatalf("dry run should plan the repin and write no backup: %v %q %v", moves, backup, err)
	}
	if got := fileOf(t, c); got != body {
		t.Errorf("dry run wrote cell.env: %s", got)
	}
	if after, _ := os.ReadFile(filepath.Join(c.Dir, modelPinsFile)); string(after) != string(before) {
		t.Errorf("dry run rewrote the record:\n%s", after)
	}
	if c.Env.Get("DESK_MODEL_worker_desk") != "sonnet" {
		t.Error("dry run must apply the repin to the in-memory env so the plan matches the launch")
	}
}

func TestModelResetTTLOffAndParse(t *testing.T) {
	c := resetCell(t, "DESK_MODEL_worker_desk=fable\n", map[string]pinRecord{"DESK_MODEL_worker_desk": aged("fable")})
	if n, _, _ := c.resetAgedDefaultPins(0, resetNow, true, ""); len(n) != 0 {
		t.Errorf("ttl 0 is off, got %v", n)
	}
	for in, want := range map[string]time.Duration{"7d": week, "7": week, "36h": 36 * time.Hour, "0": 0, "1.5d": 36 * time.Hour} {
		if got, err := parseModelTTL(in); err != nil || got != want {
			t.Errorf("parseModelTTL(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	assertDies(t, "malformed cell.env TTL", func() { cellModelTTL(envWith(map[string]string{modelTTLKey: "soon"})) })
}

// regression: #2425 C3
func TestModelTTLRefusesOddForms(t *testing.T) {
	for _, bad := range []string{"", "soon", "-3d", "NaN", "nan", "Inf", "+Inf", "1e30", "1e3", "1_0", "1_0d", "100000d", " 7d", "-1h"} {
		if d, err := parseModelTTL(bad); err == nil {
			t.Errorf("parseModelTTL(%q) accepted as %v", bad, d)
		}
	}
	// `set` refuses the same values, and --force cannot bypass a value rule.
	assertDies(t, "set CELL_MODEL_TTL_DAYS=soon", func() { validateEnvKey(modelTTLKey, "soon", true) })
	assertDies(t, "set CELL_MODEL_TTL_DAYS=NaN", func() { validateEnvKey(modelTTLKey, "NaN", false) })
	validateEnvKey(modelTTLKey, "14d", false)
}

// regression: #2425 S1
func TestResetSparesTheDeskDefault(t *testing.T) {
	// the-desk has no per-role pin, so it resolves through DESK_MODEL_DEFAULT: that key is the
	// coordinator's model and must not move.
	c := resetCell(t, "DESK_MODEL_DEFAULT=fable\n", map[string]pinRecord{"DESK_MODEL_DEFAULT": aged("fable")})
	if moves := liveReset(t, c); len(moves) != 0 {
		t.Fatalf("the harness default the-desk resolves through was repinned: %v", moves)
	}
	if rm := c.resolveRoleModel("the-desk", "claude"); rm.Model != "fable" {
		t.Errorf("the-desk resolves to %q after the reset, want fable", rm.Model)
	}
	// With the-desk pinned on its own key the harness default is an ordinary default pin.
	c = resetCell(t, "DESK_MODEL_the_desk=fable\nDESK_MODEL_DEFAULT=fable\n", map[string]pinRecord{"DESK_MODEL_DEFAULT": aged("fable")})
	if moves := liveReset(t, c); len(moves) != 1 || moves[0].Key != "DESK_MODEL_DEFAULT" {
		t.Fatalf("harness default with a pinned the-desk: %v", moves)
	}
	if rm := c.resolveRoleModel("the-desk", "claude"); rm.Model != "fable" {
		t.Errorf("the-desk resolves to %q, want its own pin fable", rm.Model)
	}
}

// regression: #2425 S2
func TestResetIgnoresNonPinKeys(t *testing.T) {
	body := "DESK_MODEL_OVERRIDE=fable\nDESK_MODEL_FLOOR_OVERRIDE=fable\nDESK_MODEL_worker_desk=fable\n"
	c := resetCell(t, body, map[string]pinRecord{
		"DESK_MODEL_OVERRIDE":       aged("fable"),
		"DESK_MODEL_FLOOR_OVERRIDE": aged("fable"),
		"DESK_MODEL_worker_desk":    aged("fable"),
	})
	moves := liveReset(t, c)
	if len(moves) != 1 || moves[0].Key != "DESK_MODEL_worker_desk" {
		t.Fatalf("only the real pin may move: %v", moves)
	}
	want := "DESK_MODEL_OVERRIDE=fable\nDESK_MODEL_FLOOR_OVERRIDE=fable\nDESK_MODEL_worker_desk=sonnet\n"
	if got := fileOf(t, c); got != want {
		t.Errorf("cell.env =\n%s\nwant\n%s", got, want)
	}
	for _, k := range []string{"DESK_MODEL_OVERRIDE", "CODEX_MODEL_OVERRIDE", "DESK_MODEL_FLOOR_OVERRIDE", "DESK_MODEL_nobody"} {
		if isModelPinKey(k) {
			t.Errorf("%s read as a model pin", k)
		}
	}
}

// regression: #2425 S3
func TestResetBacksUpAndKeepsPrev(t *testing.T) {
	body := "CELL=demo\nDESK_MODEL_worker_desk=fable\n"
	c := resetCell(t, body, map[string]pinRecord{"DESK_MODEL_worker_desk": aged("fable")})
	_, backup, err := c.resetAgedDefaultPins(week, resetNow, true, "")
	if err != nil || backup == "" {
		t.Fatalf("live reset wrote no backup: %q %v", backup, err)
	}
	if raw, _ := os.ReadFile(backup); string(raw) != body {
		t.Errorf("backup holds %q, want the pre-reset cell.env", raw)
	}
	if rec := loadPinFile(c.Dir).Pins["DESK_MODEL_worker_desk"]; rec.Prev != "fable" || rec.Value != "sonnet" {
		t.Errorf("record after reset = %+v, want value sonnet prev fable", rec)
	}
}

// regression: #2425 S4
func TestResetHeldLockWritesNothing(t *testing.T) {
	body := "DESK_MODEL_worker_desk=fable\n"
	c := resetCell(t, body, map[string]pinRecord{"DESK_MODEL_worker_desk": aged("fable")})
	lockdir := filepath.Join(c.Dir, modelPinsLock)
	if err := os.Mkdir(lockdir, 0o700); err != nil {
		t.Fatal(err)
	}
	writePid(lockdir, strconv.Itoa(os.Getpid())) // a live pid: the lock is held, not stale
	old := pinLockTries
	pinLockTries = 1
	t.Cleanup(func() { pinLockTries = old })
	out := captureStdout(t, func() { c.bootModelReset(resetNow, "") })
	if !strings.HasPrefix(out, "[model-reset] skipped this boot:") {
		t.Errorf("boot with a held lock printed %q", out)
	}
	if got := fileOf(t, c); got != body {
		t.Errorf("a reset under a held lock wrote cell.env: %s", got)
	}
	// No temp file is left beside cell.env by an atomic write.
	if err := os.RemoveAll(lockdir); err != nil {
		t.Fatal(err)
	}
	liveReset(t, c)
	ents, _ := os.ReadDir(c.Dir)
	for _, e := range ents {
		if strings.Contains(e.Name(), ".tmp-") || e.Name() == modelPinsLock {
			t.Errorf("left %s in the cell directory", e.Name())
		}
	}
}

// regression: #2425 S5
func TestResetTargetFromCellFile(t *testing.T) {
	c := resetCell(t, "DESK_MODEL_worker_desk=fable\n", map[string]pinRecord{"DESK_MODEL_worker_desk": aged("fable")})
	c.Env.Put("TIER_MODEL_MID_CLAUDE", "exported-in-shell") // the launch env, not the cell's files
	if moves := liveReset(t, c); len(moves) != 1 || moves[0].To != "sonnet" {
		t.Fatalf("target must come from the cell's files or the compiled map: %v", moves)
	}
	c = resetCell(t, "TIER_MODEL_MID_CLAUDE=claude-sonnet-5\nDESK_MODEL_worker_desk=fable\n", map[string]pinRecord{"DESK_MODEL_worker_desk": aged("fable")})
	if moves := liveReset(t, c); len(moves) != 1 || moves[0].To != "claude-sonnet-5" {
		t.Fatalf("cell.env tier entry not used: %v", moves)
	}
}

// regression: #2425 C1
func TestResetNeverRaisesAPin(t *testing.T) {
	for _, v := range []string{"haiku", "claude-haiku-4-5", "some-unranked-model", "claude-sonnet-5"} {
		body := "DESK_MODEL_worker_desk=" + v + "\n"
		c := resetCell(t, body, map[string]pinRecord{"DESK_MODEL_worker_desk": aged(v)})
		if moves := liveReset(t, c); len(moves) != 0 {
			t.Errorf("pin %q at or below the cheap default (or unranked) was moved: %v", v, moves)
		}
		if got := fileOf(t, c); got != body {
			t.Errorf("cell.env changed for %q: %s", v, got)
		}
	}
	// An empty value is not a pin: never written as one, and its record is dropped.
	body := "DESK_MODEL_worker_desk=\n"
	c := resetCell(t, body, map[string]pinRecord{"DESK_MODEL_worker_desk": aged("")})
	if moves := liveReset(t, c); len(moves) != 0 {
		t.Fatalf("empty pin turned into a per-role pin: %v", moves)
	}
	if got := fileOf(t, c); got != body {
		t.Errorf("cell.env changed: %s", got)
	}
	if _, has := loadPinFile(c.Dir).Pins["DESK_MODEL_worker_desk"]; has {
		t.Error("record for an empty pin kept")
	}
}

// regression: #2425 C2
func TestResetSkipsProviderCell(t *testing.T) {
	body := "CELL_PROVIDER=glm\nDESK_MODEL_worker_desk=fable\n"
	c := resetCell(t, body, map[string]pinRecord{"DESK_MODEL_worker_desk": aged("fable")})
	if moves := liveReset(t, c); len(moves) != 0 {
		t.Fatalf("claude pin repinned on a provider cell: %v", moves)
	}
	// A provider chosen at launch (--provider) counts the same as one in cell.env.
	c = resetCell(t, "DESK_MODEL_worker_desk=fable\n", map[string]pinRecord{"DESK_MODEL_worker_desk": aged("fable")})
	if moves, _, _ := c.resetAgedDefaultPins(week, resetNow, true, "kimi"); len(moves) != 0 {
		t.Fatalf("claude pin repinned under a launch provider: %v", moves)
	}
}

// regression: #2425 advisory
func TestResetBoundaryAndStaleRecord(t *testing.T) {
	exact := pinRecord{Kind: pinKindDefault, Value: "fable", At: resetNow.Add(-week).Format(time.RFC3339)}
	c := resetCell(t, "DESK_MODEL_worker_desk=fable\n", map[string]pinRecord{"DESK_MODEL_worker_desk": exact})
	if moves := liveReset(t, c); len(moves) != 0 {
		t.Errorf("a pin exactly at the TTL is not older than it: %v", moves)
	}
	// A record whose line was removed is dropped, so a re-added line starts a fresh clock.
	c = resetCell(t, "CELL=demo\n", map[string]pinRecord{"DESK_MODEL_worker_desk": aged("fable")})
	liveReset(t, c)
	if _, has := loadPinFile(c.Dir).Pins["DESK_MODEL_worker_desk"]; has {
		t.Error("record outlived its cell.env line")
	}
}

// regression: #2425 ruling
func TestSetRecordsPinKind(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "cell.env")
	if err := os.WriteFile(p, []byte("CELL=demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := envWith(map[string]string{})
	captureStdout(t, func() {
		applyEnvKVs(e, p, false, []string{"DESK_MODEL_DEFAULT=fable"})
		applyEnvKVsKind(e, p, false, []string{"DESK_MODEL_worker_desk=fable", "DESK_MODEL_verify_desk='fable'"}, pinKindDefault)
	})
	pins := loadPinFile(dir).Pins
	if pins["DESK_MODEL_DEFAULT"].Kind != pinKindExplicit {
		t.Errorf("plain set recorded %+v, want explicit", pins["DESK_MODEL_DEFAULT"])
	}
	if pins["DESK_MODEL_worker_desk"].Kind != pinKindDefault {
		t.Errorf("--default set recorded %+v, want default", pins["DESK_MODEL_worker_desk"])
	}
	if v := pins["DESK_MODEL_verify_desk"].Value; v != "fable" {
		t.Errorf("quoted value recorded as %q, want the loader's fable", v)
	}
	// A later plain set is an operator's choice: the pin becomes explicit.
	captureStdout(t, func() { applyEnvKVs(e, p, false, []string{"DESK_MODEL_worker_desk=haiku"}) })
	if rec := loadPinFile(dir).Pins["DESK_MODEL_worker_desk"]; rec.Kind != pinKindExplicit || rec.Value != "haiku" {
		t.Errorf("plain set over a default pin: %+v, want explicit/haiku", rec)
	}
	// Clearing a pin drops its record; a non-pin key, forced or not, is never recorded.
	captureStdout(t, func() {
		applyEnvKVs(e, p, false, []string{"DESK_MODEL_verify_desk=", "CELL_COCKPIT=tmux"})
		applyEnvKVs(e, p, true, []string{"DESK_MODEL_OVERRIDE=fable"})
	})
	got := loadPinFile(dir).Pins
	for _, k := range []string{"DESK_MODEL_verify_desk", "CELL_COCKPIT", "DESK_MODEL_OVERRIDE"} {
		if _, has := got[k]; has {
			t.Errorf("%s recorded: %+v", k, got[k])
		}
	}
}

// regression: #2425 ruling
func TestSetDefaultFlagOnly(t *testing.T) {
	root := defaultsRoot(t)
	writeCell(t, root, "demo", demoCell)
	captureStdout(t, func() { cmdSet("demo", []string{"worker-desk", "--model", "fable"}) })
	captureStdout(t, func() { cmdSet("demo", []string{"DESK_MODEL_verify_desk=fable", "--default"}) })
	pins := loadPinFile(filepath.Join(root, "demo")).Pins
	if pins["DESK_MODEL_worker_desk"].Kind != pinKindExplicit {
		t.Errorf("plain role-form set recorded %+v, want explicit", pins["DESK_MODEL_worker_desk"])
	}
	if pins["DESK_MODEL_verify_desk"].Kind != pinKindDefault {
		t.Errorf("set --default recorded %+v, want default", pins["DESK_MODEL_verify_desk"])
	}
	msg := refusal(t, "set --default with no pin", func() { cmdSet("demo", []string{"CELL_COCKPIT=tmux", "--default"}) })
	if !strings.Contains(msg, "sets no model pin") {
		t.Errorf("refusal = %q", msg)
	}
}

func TestBootModelResetPrintsOneLine(t *testing.T) {
	c := resetCell(t, "DESK_MODEL_worker_desk=fable\n", map[string]pinRecord{"DESK_MODEL_worker_desk": aged("fable")})
	out := captureStdout(t, func() { c.bootModelReset(resetNow, "") })
	if strings.Count(out, "\n") != 1 || !strings.HasPrefix(out, "[model-reset] worker-desk: fable -> sonnet") {
		t.Errorf("boot notice = %q", out)
	}
	if quiet := captureStdout(t, func() { c.bootModelReset(resetNow, "") }); quiet != "" {
		t.Errorf("a boot with nothing to repin must print nothing, got %q", quiet)
	}
}
