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

// TestRunLogSinkKeepsTail pins the sink's direction: the TAIL survives however the bytes
// arrive (one write, or many small ones across its compaction), the truncated flag is exact at
// the keep boundary, and a write past the read bound fails instead of being dropped silently.
func TestRunLogSinkKeepsTail(t *testing.T) {
	small := &runLogSink{limit: 1 << 20, keep: 8}
	_, _ = small.Write([]byte("short"))
	if got, trunc := small.tail(); got != "short" || trunc {
		t.Fatalf("a log under the cap must pass untouched, got %q trunc=%v", got, trunc)
	}
	exact := &runLogSink{limit: 1 << 20, keep: 8}
	_, _ = exact.Write([]byte("12345678"))
	if got, trunc := exact.tail(); got != "12345678" || trunc {
		t.Fatalf("a log exactly at the cap is whole, got %q trunc=%v", got, trunc)
	}
	for _, chunk := range []int{1, 3, 7, 64} {
		s := &runLogSink{limit: 1 << 20, keep: 8}
		log := strings.Repeat("a", 100) + "FAILURE!"
		for i := 0; i < len(log); i += chunk {
			if _, err := s.Write([]byte(log[i:min(i+chunk, len(log))])); err != nil {
				t.Fatalf("chunk %d: %v", chunk, err)
			}
		}
		if got, trunc := s.tail(); got != "FAILURE!" || !trunc {
			t.Fatalf("chunk %d: want the last 8 bytes flagged truncated, got %q trunc=%v", chunk, got, trunc)
		}
	}
	over := &runLogSink{limit: 10, keep: 8}
	if _, err := over.Write([]byte("0123456789")); err != nil {
		t.Fatalf("a write reaching the bound exactly must pass: %v", err)
	}
	if _, err := over.Write([]byte("x")); err != errRunLogOverRead {
		t.Fatalf("a write past the bound must fail with errRunLogOverRead, got %v", err)
	}
}
