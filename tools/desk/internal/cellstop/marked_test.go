package cellstop

import "testing"

// regression: #2880
func TestMarkedOrphanOwnership(t *testing.T) {
	m := Marker{Cell: "example", Dir: "/cells/example"}
	mark := func(role string) map[string]string {
		return map[string]string{EnvCell: "example", EnvCellDir: "/cells/example", EnvCellRole: role}
	}
	all := map[int]process{
		1:   {pid: 1, parent: 0, identity: "init", env: map[string]string{}},
		10:  {pid: 10, parent: 1, identity: "orphan", env: mark("worker-desk")},
		11:  {pid: 11, parent: 10, identity: "orphan child", env: mark("worker-desk")},
		20:  {pid: 20, parent: 30, identity: "live parent", env: mark("worker-desk")},
		30:  {pid: 30, parent: 1, identity: "unmarked shell", env: map[string]string{}},
		40:  {pid: 40, parent: 1, identity: "same name, other cell", env: map[string]string{EnvCell: "example", EnvCellDir: "/other/example", EnvCellRole: "worker-desk"}},
		50:  {pid: 50, parent: 1, identity: "path only", env: map[string]string{"HOME": "/cells/example/home"}},
		60:  {pid: 60, parent: 1, identity: "live lease holder", env: mark("the-desk")},
		61:  {pid: 61, parent: 60, identity: "holder child", env: mark("worker-desk")},
		70:  {pid: 70, parent: 1, identity: "invoker ancestor", env: mark("intake-desk")},
		80:  {pid: 80, parent: 1, identity: "no role", env: map[string]string{EnvCell: "example", EnvCellDir: "/cells/example"}},
		900: {pid: 900, parent: 70, identity: "invoker", env: mark("intake-desk")},
	}
	got := map[int]Marked{}
	listed, err := markedIn(all, m, map[int]bool{900: true, 70: true}, []int{60})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range listed {
		got[p.PID] = p
	}
	// Listed: every live marked process of this cell, so that no record is ever classed dead
	// while one runs. Orphan (signal candidate) only where proven; shielded ones are flagged.
	want := map[int]bool{10: true, 11: true, 20: false, 60: false, 61: false, 70: false, 900: false}
	if len(got) != len(want) {
		t.Fatalf("marked set = %+v, want pids %v", got, want)
	}
	for pid, orphan := range want {
		p, ok := got[pid]
		if !ok || p.Orphan != orphan || p.Orphan && p.Shielded {
			t.Fatalf("pid %d: %+v (listed %v), want orphan=%v", pid, p, ok, orphan)
		}
	}
	for _, pid := range []int{60, 61, 70, 900} {
		if !got[pid].Shielded {
			t.Fatalf("pid %d not flagged shielded: %+v", pid, got[pid])
		}
	}
}

// regression: #2880 (security S1) — a marked parent-1 process ABOVE a live lease holder (a tmux
// server started from inside a role window, its pane shells, another cell's supervisor on it) is
// never an orphan: anything with a shielded descendant is shielded too.
func TestMarkedNeverAboveLiveHolder(t *testing.T) {
	m := Marker{Cell: "a", Dir: "/cells/a"}
	mark := map[string]string{EnvCell: "a", EnvCellDir: "/cells/a", EnvCellRole: "the-desk"}
	all := map[int]process{
		1:   {pid: 1, identity: "init", env: map[string]string{}},
		100: {pid: 100, parent: 1, identity: "tmux server", env: mark},
		101: {pid: 101, parent: 100, identity: "pane shell", env: mark},
		102: {pid: 102, parent: 101, identity: "other cell supervisor", env: mark},
		201: {pid: 201, parent: 100, identity: "pane shell of live holder", env: mark},
		202: {pid: 202, parent: 201, identity: "live worker-desk holder", env: mark},
		203: {pid: 203, parent: 202, identity: "live role child", env: mark},
		300: {pid: 300, parent: 1, identity: "real orphan", env: mark},
		800: {pid: 800, parent: 1, identity: "invoker shell", env: map[string]string{}},
		900: {pid: 900, parent: 800, identity: "invoker", env: map[string]string{}},
	}
	got, err := markedIn(all, m, map[int]bool{900: true, 800: true}, []int{202})
	if err != nil {
		t.Fatal(err)
	}
	if o := orphansOf(got); len(o) != 1 || o[0] != 300 {
		t.Fatalf("want only the real orphan 300, got %+v", got)
	}
}

// regression: #2880 (security round 2, S1a) — the shield walk crosses processes of other uids:
// a marked host above a live holder stays shielded when a root-owned login or sudo sits between.
// A parent missing from the listing altogether is could-not-check, never a gap to walk past.
func TestMarkedShieldCrossesOtherUID(t *testing.T) {
	m := Marker{Cell: "a", Dir: "/cells/a"}
	mark := map[string]string{EnvCell: "a", EnvCellDir: "/cells/a", EnvCellRole: "the-desk"}
	all := map[int]process{
		100: {pid: 100, parent: 1, identity: "host", env: mark},
		101: {pid: 101, parent: 100, identity: "other window shell", env: mark},
		150: {pid: 150, parent: 100, identity: "root-owned login", foreign: true, env: map[string]string{}},
		201: {pid: 201, parent: 150, identity: "shell", env: mark},
		202: {pid: 202, parent: 201, identity: "live holder", env: mark},
		900: {pid: 900, parent: 1, identity: "invoker", env: map[string]string{}},
	}
	got, err := markedIn(all, m, map[int]bool{900: true}, []int{202})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if p.Orphan {
			t.Fatalf("host of a live holder listed as an orphan across another uid: %+v", got)
		}
	}
	delete(all, 150)
	if _, err := markedIn(all, m, map[int]bool{900: true}, []int{202}); err == nil {
		t.Fatal("a parent missing from the listing was walked past instead of could-not-check")
	}
}

// regression: #2880 (security round 2, S1b) — another cell's live window under a host that
// inherited THIS cell's marker shields the host: by its cellctl supervisor, or by a role child
// whose readable environment carries another cell's marker.
func TestMarkedShieldOtherCellWindow(t *testing.T) {
	m := Marker{Cell: "a", Dir: "/cells/a"}
	markA := map[string]string{EnvCell: "a", EnvCellDir: "/cells/a", EnvCellRole: "the-desk"}
	markB := map[string]string{EnvCell: "b", EnvCellDir: "/cells/b", EnvCellRole: "worker-desk"}
	base := func() map[int]process {
		return map[int]process{
			100: {pid: 100, parent: 1, identity: "tmux server", env: markA, envRead: true, comm: "tmux"},
			101: {pid: 101, parent: 100, identity: "pane", env: markA, envRead: true, comm: "zsh"},
			102: {pid: 102, parent: 101, identity: "cell b supervisor", env: markA, envRead: true, comm: "/opt/bin/cellctl"},
			103: {pid: 103, parent: 102, identity: "cell b role child", env: markB, envRead: true, comm: "claude"},
			300: {pid: 300, parent: 1, identity: "real orphan", env: markA, envRead: true, comm: "node"},
			900: {pid: 900, parent: 1, identity: "invoker", env: map[string]string{}},
		}
	}
	for name, drop := range map[string]int{"supervisor and child": 0, "supervisor between passes": 103} {
		all := base()
		delete(all, drop)
		got, err := markedIn(all, m, map[int]bool{900: true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if o := orphansOf(got); len(o) != 1 || o[0] != 300 {
			t.Fatalf("%s: want only the real orphan 300, got %+v", name, got)
		}
	}
	// A child whose environment is withheld does not shield; that keeps a hand-closed loop
	// stoppable. Its readable cellctl supervisor above it still does (case above).
	all := base()
	delete(all, 102)
	delete(all, 103)
	all[104] = process{pid: 104, parent: 101, identity: "withheld env", env: map[string]string{}, comm: "sleep"}
	got, _ := markedIn(all, m, map[int]bool{900: true}, nil)
	orphans := 0
	for _, p := range got {
		if p.Orphan {
			orphans++
		}
	}
	if orphans != 3 { // 100, 101, 300: a withheld child alone proves nothing either way
		t.Fatalf("withheld-environment child changed attribution: %+v", got)
	}
}

func orphansOf(got []Marked) []int {
	var out []int
	for _, p := range got {
		if p.Orphan {
			if p.Shielded {
				return []int{-1}
			}
			out = append(out, p.PID)
		}
	}
	return out
}

// regression: #2880 (correctness round 2, F2) — shielding means never signal, not never see.
// Behind a Linux child subreaper shared with the invoker, marked processes of a free role are
// still listed (non-orphan, shielded), so their record can never be classed dead.
func TestMarkedSubreaperStillListed(t *testing.T) {
	m := Marker{Cell: "a", Dir: "/cells/a"}
	mark := map[string]string{EnvCell: "a", EnvCellDir: "/cells/a", EnvCellRole: "worker-desk"}
	all := map[int]process{
		1:   {pid: 1, identity: "init", env: map[string]string{}},
		50:  {pid: 50, parent: 1, identity: "systemd --user", env: map[string]string{}},
		60:  {pid: 60, parent: 50, identity: "terminal", env: map[string]string{}},
		800: {pid: 800, parent: 60, identity: "shell", env: map[string]string{}},
		900: {pid: 900, parent: 800, identity: "invoker", env: map[string]string{}},
		300: {pid: 300, parent: 50, identity: "marked loop", env: mark},
		301: {pid: 301, parent: 300, identity: "marked child", env: mark},
	}
	got, err := markedIn(all, m, map[int]bool{900: true, 800: true, 60: true, 50: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].PID != 300 || got[1].PID != 301 || got[0].Orphan || got[1].Orphan {
		t.Fatalf("subreaper-hosted marked processes must be listed as non-orphans, got %+v", got)
	}
}
