package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// regression: #3353
func TestCellLaunchTelemetry(t *testing.T) {
	for _, sink := range []string{"normal", "failed", "slow"} {
		t.Run(sink, func(t *testing.T) {
			state := perfFixture(t)
			if sink == "failed" {
				if err := os.WriteFile(filepath.Join(state, "perf"), []byte("blocked"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			oldWriter := perfWriter
			if sink == "slow" {
				perfWriter = &perfrec.Writer{Now: func() time.Time { time.Sleep(20 * time.Millisecond); return perfTestNow }}
			}
			t.Cleanup(func() { perfWriter = oldWriter })
			for _, enabled := range []bool{false, true} {
				ambient := "1"
				if enabled {
					ambient = "0"
				}
				t.Setenv("ASSAY_TELEMETRY", ambient)
				gotOut, gotErr := captureStd(t, func() {
					code := PerfRunMeasuredConfig("cellctl", []string{"desk"}, []string{"desk"}, func(_ []string, _ *perfrec.Session) (int, perfrec.Context) {
						PerfSelectConsent(enabled)
						PerfStep("cellctl", "launch-child-start", time.Millisecond, 0)
						fmt.Fprintln(os.Stdout, "normal result")
						return 17, perfrec.ResolveContext(perfrec.ContextInput{})
					})
					if code != 17 {
						t.Fatalf("%s sink changed exit to %d", sink, code)
					}
				})
				if gotOut != "normal result\n" || gotErr != "" {
					t.Fatalf("sink or consent changed output %q/%q", gotOut, gotErr)
				}
				if !enabled && sink != "failed" {
					if _, err := os.Stat(filepath.Join(state, "perf")); !os.IsNotExist(err) {
						t.Fatal("resolved off inherited ambient consent")
					}
				}
			}
			if sink != "failed" {
				rows, _ := perfLines(t, state)
				if len(rows) != 2 {
					t.Fatalf("config-only invocation+phase rows=%d", len(rows))
				}
				found := false
				for _, r := range rows {
					if r["verb"] == "desk" {
						found = true
					}
				}
				if !found {
					t.Fatal("parent invocation missing")
				}
			}
		})
	}
}
