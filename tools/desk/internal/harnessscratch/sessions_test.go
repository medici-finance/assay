package harnessscratch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeID builds a session-id-shaped name from one hex digit, so the fixture holds no
// literal that reads as a real agent session id.
func fakeID(d string) string {
	r := func(n int) string { return strings.Repeat(d, n) }
	return r(8) + "-" + r(4) + "-4" + r(3) + "-8" + r(3) + "-" + r(12)
}

var (
	idLive   = fakeID("1")
	idEnded  = fakeID("2")
	idNoBeac = fakeID("3")
	idWorktr = fakeID("4")
	idRepo   = fakeID("5")
	idLinked = fakeID("6")
	idOther  = fakeID("8")
	idNone   = fakeID("7")
)

const (
	project   = "-fixture-project"
	freshness = time.Hour
)

var now = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func write(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func beacon(t *testing.T, dir, id string, updated time.Time) {
	write(t, filepath.Join(dir, id+".json"), `{"session":"`+id+`","updated":"`+updated.Format(time.RFC3339)+`"}`)
}

// fixture lays out a harness temp root and a roster, and returns both plus a directory
// outside the root that a planted link points at.
func fixture(t *testing.T) (root, roster, outside string) {
	base := t.TempDir()
	root, roster, outside = filepath.Join(base, "claude-501"), filepath.Join(base, "roster"), filepath.Join(base, "outside")
	s := func(id string) string { return filepath.Join(root, project, id) }
	write(t, filepath.Join(s(idLive), "scratchpad", "a"), "x")
	write(t, filepath.Join(s(idEnded), "scratchpad", "gocache", "b"), "x")
	write(t, filepath.Join(s(idNoBeac), "scratchpad", "c"), "x")
	write(t, filepath.Join(s(idWorktr), "scratchpad", "wt", ".git"), "gitdir: /elsewhere/.git/worktrees/wt\n")
	write(t, filepath.Join(s(idRepo), "scratchpad", "clone", ".git", "HEAD"), "ref: refs/heads/main\n")
	write(t, filepath.Join(root, project, "not-a-session", "d"), "x")
	write(t, filepath.Join(outside, "keep-me"), "x")
	// Another desk's session with a stale beacon: never listed, never touched.
	write(t, filepath.Join(root, "-other-desk", idOther, "e"), "x")
	beacon(t, roster, idOther, now.Add(-72*time.Hour))
	if err := os.Symlink(outside, s(idLinked)); err != nil {
		t.Fatal(err)
	}
	beacon(t, roster, idLive, now.Add(-5*time.Minute))
	for _, id := range []string{idEnded, idWorktr, idRepo, idLinked} {
		beacon(t, roster, id, now.Add(-72*time.Hour))
	}
	return root, roster, outside
}

func byID(t *testing.T, rows []Entry) map[string]Entry {
	t.Helper()
	m := map[string]Entry{}
	for _, e := range rows {
		k := e.Session
		if k == "" {
			k = e.Project
		}
		m[k] = e
	}
	return m
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

// regression: #7274
func TestSessionsDryRunRemovesNothing(t *testing.T) {
	root, roster, _ := fixture(t)
	rows, err := Sessions(root, project, BeaconLiveness(roster, freshness, now), false)
	if err != nil {
		t.Fatal(err)
	}
	got := byID(t, rows)
	want := map[string]struct{ state, action string }{
		idLive:          {"live", ActionKeep},
		idEnded:         {"ended", ActionCandidate},
		idNoBeac:        {"unknown", ActionKeep},
		idWorktr:        {"ended", ActionKeep},
		idRepo:          {"ended", ActionKeep},
		idLinked:        {"unknown", ActionKeep},
		"not-a-session": {"unknown", ActionKeep},
	}
	for id, w := range want {
		e, ok := got[id]
		if !ok {
			t.Fatalf("no row for %s: %+v", id, rows)
		}
		if e.State != w.state || e.Action != w.action || e.Reason == "" {
			t.Errorf("%s: state=%s action=%s reason=%q, want %s/%s with a reason", id, e.State, e.Action, e.Reason, w.state, w.action)
		}
		if !exists(e.Path) {
			t.Errorf("dry run removed %s", e.Path)
		}
	}
	if !got[idWorktr].Held || !strings.Contains(got[idWorktr].Reason, "registered git worktree") {
		t.Errorf("worktree session not reported held: %+v", got[idWorktr])
	}
	if !got[idRepo].Held {
		t.Errorf("repository session not reported held: %+v", got[idRepo])
	}
}

// regression: #7274
func TestSessionsApplyRemovesOnlyEnded(t *testing.T) {
	root, roster, outside := fixture(t)
	rows, err := Sessions(root, project, BeaconLiveness(roster, freshness, now), true)
	if err != nil {
		t.Fatal(err)
	}
	got := byID(t, rows)
	if e := got[idEnded]; e.Action != ActionRemoved || exists(e.Path) {
		t.Errorf("ended session not removed: %+v", e)
	}
	for _, id := range []string{idLive, idNoBeac, idWorktr, idRepo, idLinked, "not-a-session"} {
		if e := got[id]; e.Action != ActionKeep || !exists(e.Path) {
			t.Errorf("%s removed or not kept: %+v", id, e)
		}
	}
	if !exists(filepath.Join(outside, "keep-me")) {
		t.Error("removal followed a link out of the root")
	}
	if _, listed := got[idOther]; listed || !exists(filepath.Join(root, "-other-desk", idOther)) {
		t.Error("the pass reached another desk's project directory")
	}
}

// regression: #7274 — the pass is one desk's: it refuses anything but one project slug and
// treats a desk with no sessions yet as an empty report.
func TestSessionsScopedToOneDesk(t *testing.T) {
	root, roster, _ := fixture(t)
	l := BeaconLiveness(roster, freshness, now)
	for _, bad := range []string{"", ".", "..", "a/b", "../x"} {
		if rows, err := Sessions(root, bad, l, true); err == nil || rows != nil {
			t.Errorf("project %q accepted: %d rows, err %v", bad, len(rows), err)
		}
	}
	rows, err := Sessions(root, "-no-sessions-yet", l, true)
	if err != nil || len(rows) != 0 {
		t.Errorf("absent project: %v %v", rows, err)
	}
}

// regression: #7274
func TestSessionsRecheckBeforeRemoval(t *testing.T) {
	root, roster, _ := fixture(t)
	calls := map[string]int{}
	base := BeaconLiveness(roster, freshness, now)
	// The session comes back between planning and removal.
	live := func(id string) (State, string) {
		calls[id]++
		if id == idEnded && calls[id] > 1 {
			return Live, "beacon re-stamped"
		}
		return base(id)
	}
	rows, err := Sessions(root, project, live, true)
	if err != nil {
		t.Fatal(err)
	}
	if e := byID(t, rows)[idEnded]; e.Action != ActionKeep || !exists(e.Path) {
		t.Errorf("session that came back was removed: %+v", e)
	}
}

func TestSessionsWalkLimitKeeps(t *testing.T) {
	root, roster, _ := fixture(t)
	saved := walkLimit
	walkLimit = 1
	defer func() { walkLimit = saved }()
	rows, err := Sessions(root, project, BeaconLiveness(roster, freshness, now), true)
	if err != nil {
		t.Fatal(err)
	}
	if e := byID(t, rows)[idEnded]; e.Action != ActionKeep || !strings.Contains(e.Reason, "could-not-check") || !exists(e.Path) {
		t.Errorf("over-limit session not kept as could-not-check: %+v", e)
	}
}

func TestBeaconLiveness(t *testing.T) {
	_, roster, _ := fixture(t)
	write(t, filepath.Join(roster, idNoBeac+".json"), `{"updated":"yesterday"}`)
	l := BeaconLiveness(roster, freshness, now)
	for id, want := range map[string]State{idLive: Live, idEnded: Ended, idNoBeac: Unknown, idNone: Unknown, "../escape": Unknown} {
		if got, why := l(id); got != want || why == "" {
			t.Errorf("%s: %v %q, want %v", id, got, why, want)
		}
	}
	if got, _ := BeaconLiveness("", freshness, now)(idEnded); got != Unknown {
		t.Errorf("no roster read as %v, want unknown", got)
	}
}
