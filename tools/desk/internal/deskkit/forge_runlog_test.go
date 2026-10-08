package deskkit

import (
	"reflect"
	"strings"
	"testing"
)

// TestForgeNoPassthroughAfterRunLogRetry pins the two operations forge-neutral brief 17 adds to the frozen
// Forge interface: each takes exactly (ForgeRepo, RunRef) — a repo and a run identity — and
// no caller-supplied address, on the interface and on both backends. A log or retry verb that
// took a path or URL would be an arbitrary-request method under a narrow name.
func TestForgeNoPassthroughAfterRunLogRetry(t *testing.T) {
	want := map[string]struct{ out []reflect.Type }{
		"RunLog":   {out: []reflect.Type{reflect.TypeOf([]RunLogPart(nil)), reflect.TypeOf((*error)(nil)).Elem()}},
		"RetryRun": {out: []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()}},
	}
	ifaceT := reflect.TypeOf((*Forge)(nil)).Elem()
	for name, w := range want {
		for label, typ := range map[string]reflect.Type{
			"interface": ifaceT,
			"github":    reflect.TypeOf(&GitHubForge{}),
			"gitlab":    reflect.TypeOf(&GitLabForge{}),
		} {
			m, ok := typ.MethodByName(name)
			if !ok {
				t.Errorf("%s does not carry %s", label, name)
				continue
			}
			ft := m.Type
			in := ft.NumIn()
			first := 0
			if label != "interface" {
				first = 1 // skip the receiver
			}
			if in-first != 2 || ft.In(first) != reflect.TypeOf(ForgeRepo{}) || ft.In(first+1) != reflect.TypeOf(RunRef{}) {
				t.Errorf("%s.%s must take exactly (ForgeRepo, RunRef), has %v", label, name, ft)
			}
			if ft.NumOut() != len(w.out) {
				t.Errorf("%s.%s returns %d values, want %d", label, name, ft.NumOut(), len(w.out))
				continue
			}
			for i, o := range w.out {
				if ft.Out(i) != o {
					t.Errorf("%s.%s result %d is %v, want %v", label, name, i, ft.Out(i), o)
				}
			}
		}
		if why, bad := isPassthroughName(name); bad {
			t.Errorf("%s reads as a passthrough name: %s", name, why)
		}
	}
	if !strings.Contains(reflect.TypeOf(RunLogPart{}).Field(0).Name, "Name") {
		t.Errorf("RunLogPart should lead with the job name")
	}
}

// TestCapRunLogTextKeepsTail pins the cap's direction: the TAIL survives, because a CI failure
// is at the end of a job log, and the truncated flag is set.
func TestCapRunLogTextKeepsTail(t *testing.T) {
	small, trunc := capRunLogText([]byte("short"))
	if small != "short" || trunc {
		t.Fatalf("a log under the cap must pass untouched, got %q trunc=%v", small, trunc)
	}
	big := strings.Repeat("a", RunLogPartCap) + "THE-FAILURE"
	got, trunc := capRunLogText([]byte(big))
	if !trunc {
		t.Fatal("a log over the cap must be flagged truncated")
	}
	if !strings.HasSuffix(got, "THE-FAILURE") {
		t.Fatalf("the cap must keep the tail, got ...%q", got[max(0, len(got)-20):])
	}
	if len(got) > RunLogPartCap {
		t.Fatalf("capped text is %d bytes, over the %d cap", len(got), RunLogPartCap)
	}
}
