package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

// regression: #3353
func TestCellLaunchTelemetry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("offline launch fixture uses Unix shell")
	}
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "off", true: "config-only"}[enabled], func(t *testing.T) {
			f := newPolicyFixture(t, "2.1.278")
			f.prepareLocalLaunch(t)
			script := "#!/bin/sh\ncase $1 in\n--version) echo '2.1.278 (stub)';exit 0;;\nplugin) exit 0;;\nesac\nexit 0\n"
			must(t, os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(script), 0700))
			path := filepath.Join(f.cellDir, "cell.env")
			base, err := os.ReadFile(path)
			must(t, err)
			value := "0"
			if enabled {
				value = "1"
			}
			must(t, os.WriteFile(path, append(base, []byte("\nASSAY_TELEMETRY="+value+"\n")...), 0600))
			ambient := "0"
			if !enabled {
				ambient = "1"
			}
			r := f.run(t, []string{"CELLCTL_DESKWT=0", "ASSAY_TELEMETRY=" + ambient}, "desk", "example", "worker-desk", "--cadence", "off")
			if r.code != 0 {
				t.Fatalf("launch exit=%d stderr=%s", r.code, r.stderr)
			}
			rows := perfRowsIn(t, filepath.Join(f.cellDir, ".config", "assay"))
			got := verbsOf(rows)
			if !enabled {
				if len(got) != 0 {
					t.Fatal("no opt-in wrote telemetry", got)
				}
				return
			}
			for _, step := range []string{"launch-preparation", "launch-scratch-lock-wait", "launch-scratch-admission", "launch-cache-enrollment", "launch-child-start", "launch-first-start", "launch-all-starts"} {
				if !slices.Contains(got, "step:"+step) {
					t.Fatalf("config-only launch omitted %s; got %v", step, got)
				}
			}
			if !slices.Contains(got, "desk") {
				t.Fatal("config-only consent omitted parent invocation", got)
			}
			n := 0
			for _, v := range got {
				if v == "step:launch-preparation" {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("shared preparation counted %d times", n)
			}
		})
	}
	// Incomplete/failed starts cannot be recorded as first/all confirmed starts.
	c := preparedCell(t)
	c.Env.Put("ASSAY_TELEMETRY", "1")
	rows := c.convergeRoles([]upProbe{{Role: "worker-desk", Kind: probeAbsent}}, nil, func(string, time.Time) (string, error) { return "", os.ErrInvalid }, func(string) error { return nil })
	if upRowsOK(rows) {
		t.Fatal("unknown child start became confirmed")
	}
}
