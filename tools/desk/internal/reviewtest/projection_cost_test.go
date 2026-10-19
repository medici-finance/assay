package reviewtest_test

import (
	"context"
	"encoding/json"
	q "github.com/medici-finance/assay/tools/desk/internal/reviewtest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: class issue-projection
func TestProjectionCost(t *testing.T) {
	bin := build(t)
	type sample struct {
		Arm      string  `json:"arm"`
		Size     int     `json:"size"`
		Cycle    int     `json:"cycle"`
		WallMS   float64 `json:"wallMs"`
		OwnCPUMS float64 `json:"directOwnCpuMs"`
		HTTP     int     `json:"httpAttempts"`
	}
	var samples []sample
	for _, size := range []int{8, 80} {
		for cycle := 0; cycle < 3; cycle++ {
			for _, arm := range []string{"legacy", "subject"} {
				s := sample{Arm: arm, Size: size, Cycle: cycle}
				began := time.Now()
				cases := []string{"legacy", "legacy", "legacy-inbox"}
				if arm == "subject" {
					cases = []string{"worker", "intake", "inbox"}
				}
				for _, name := range cases {
					dir := t.TempDir()
					request, response := filepath.Join(dir, "request.json"), filepath.Join(dir, "response.json")
					raw, _ := json.Marshal(q.Request{Case: name, Size: size, Root: dir, Home: dir})
					if err := os.WriteFile(request, raw, 0600); err != nil {
						t.Fatal(err)
					}
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					cmd := exec.CommandContext(ctx, filepath.Join(bin, "issueboard.test"), "-test.run=^TestProjectionCostProbe$", "-test.v")
					cmd.Env = append(isolatedEnv(dir), q.RequestEnv+"="+request, q.ResponseEnv+"="+response)
					log, e := cmd.CombinedOutput()
					cancel()
					if e != nil || !strings.Contains(string(log), "--- PASS: TestProjectionCostProbe ") {
						t.Fatalf("cost production path %s: %v %s", name, e, log)
					}
					s.OwnCPUMS += float64((cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()).Nanoseconds()) / 1e6
					raw, e = os.ReadFile(response)
					if e != nil {
						t.Fatal(e)
					}
					var got q.Response
					if e = json.Unmarshal(raw, &got); e != nil {
						t.Fatal(e)
					}
					if got.Exit != 0 || !got.HTTPComplete {
						t.Fatalf("incomplete cost observation: %+v", got)
					}
					for _, a := range got.Attempts {
						if a.Points != nil {
							t.Fatal("invented fixture points")
						}
						if a.Status != 200 {
							t.Fatal("failed fixture attempt")
						}
					}
					s.HTTP += len(got.Attempts)
				}
				s.WallMS = float64(time.Since(began).Nanoseconds()) / 1e6
				samples = append(samples, s)
				t.Logf("%s size=%d cycle=%d: wall=%.3fms direct-child-own-cpu=%.3fms actual fake-provider attempts=%d", arm, size, cycle, s.WallMS, s.OwnCPUMS, s.HTTP)
			}
		}
	}
	for i := 0; i < len(samples); i += 2 {
		if samples[i+1].HTTP >= samples[i].HTTP {
			t.Fatal("complete-cycle subject did not remove provider work")
		}
	}
	if path := os.Getenv("ASSAY_ISSUE_REPORT"); path != "" {
		raw, _ := json.MarshalIndent(samples, "", "  ")
		if e := os.WriteFile(path, append(raw, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Log("coverage: actual production HTTP attempts against socket-free fake provider; setup/build excluded; serial complete worker+intake+addressed-inbox cycle; direct-child own CPU only (test harness included); inclusive/descendant/model CPU and billable GraphQL points unknown; synthetic cold/steady cohorts, not fleet performance or an accepted latency regression allowance")
}
