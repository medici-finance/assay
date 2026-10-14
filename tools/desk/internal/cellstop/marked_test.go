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
