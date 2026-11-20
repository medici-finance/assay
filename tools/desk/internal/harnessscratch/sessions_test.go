package harnessscratch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	idLive    = "11111111-1111-4111-8111-111111111111"
	idEnded   = "22222222-2222-4222-8222-222222222222"
	idNoBeac  = "33333333-3333-4333-8333-333333333333"
	idWorktr  = "44444444-4444-4444-8444-444444444444"
	idRepo    = "55555555-5555-4555-8555-555555555555"
	idLinked  = "66666666-6666-4666-8666-666666666666"
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
	rows, err := Sessions(root, BeaconLiveness(roster, freshness, now), false)
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
	rows, err := Sessions(root, BeaconLiveness(roster, freshness, now), true)
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
	rows, err := Sessions(root, live, true)
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
	rows, err := Sessions(root, BeaconLiveness(roster, freshness, now), true)
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
	for id, want := range map[string]State{idLive: Live, idEnded: Ended, idNoBeac: Unknown, "77777777-7777-4777-8777-777777777777": Unknown, "../escape": Unknown} {
		if got, why := l(id); got != want || why == "" {
			t.Errorf("%s: %v %q, want %v", id, got, why, want)
		}
	}
	if got, _ := BeaconLiveness("", freshness, now)(idEnded); got != Unknown {
		t.Errorf("no roster read as %v, want unknown", got)
	}
}
