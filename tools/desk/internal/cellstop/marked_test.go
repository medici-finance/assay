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
	for _, p := range markedIn(all, m, map[int]bool{900: true, 70: true}, []int{60}) {
		got[p.PID] = p
	}
	want := map[int]bool{10: true, 11: true, 20: false}
	if len(got) != len(want) {
		t.Fatalf("marked set = %+v, want pids %v", got, want)
	}
	for pid, orphan := range want {
		p, ok := got[pid]
		if !ok || p.Orphan != orphan || p.Role != "worker-desk" {
			t.Fatalf("pid %d: %+v (listed %v), want orphan=%v", pid, p, ok, orphan)
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
	got := markedIn(all, m, map[int]bool{900: true, 800: true}, []int{202})
	if len(got) != 1 || got[0].PID != 300 || !got[0].Orphan {
		t.Fatalf("want only the real orphan 300, got %+v", got)
	}
}
