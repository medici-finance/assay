package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
)

// regression: #4172 — actual reaped child, cache Finish and supervisor clock;
// no operator Check/clean occurs between the two process enrollments.
func TestCadenceCacheMaintenance(t *testing.T) {
	for _, mode := range []string{"build", "age-during-interval", "maintenance-failure"} {
		t.Run(mode, func(t *testing.T) {
			c := codexEnvironmentCell(t)
			c.Name, c.Kind = "sample", "house"
			c.Cadence = &cadenceOptions{Interval: 250 * time.Millisecond, Budget: 5 * time.Second}
			c.Env.Put("CELL_GO_CACHE", "on")
			c.Env.Put("CELL_GO_CACHE_MIN_FREE", "0")
			c.Env.Put("CELL_GO_CACHE_BYTES", "1048576")
			must(t, os.MkdirAll(c.Dir, 0700))
			must(t, c.cacheAdmission())
			p, err := c.cachePolicy()
			must(t, err)
			// Warm fixture timestamps are unambiguous; the build variant creates
			// its new entry at real current filesystem time in the managed child.
			old := time.Now().Add(-2 * cellcache.AdmitMtimeSlack)
			for _, name := range []string{"build", "mod"} {
				must(t, os.Chtimes(filepath.Join(p.Root, name), old, old))
			}
			l, err := cellcadence.Acquire(c.cadenceDir("worker-desk"))
			must(t, err)
			defer l.Close()
			c.cadenceLease = l
			exe, err := os.Executable()
			must(t, err)
			wt := t.TempDir()
			record := filepath.Join(t.TempDir(), "passes")
			env := c.cacheEnv([]string{"ASSAY_SOURCE_REVISION=fixture-revision", "DESK_LOOP=worker-desk", "DESK_ROOTS=example/repo=/a path", "ASSAY_COCKPIT=herdr", "FIXTURE_CWD=" + wt, "FIXTURE_RECORD=" + record})
			if mode == "build" {
				env = append(env, "FIXTURE_CACHE_WRITE=1")
			}
			for _, k := range []string{"SYSTEMROOT", "SystemRoot"} {
				if v := os.Getenv(k); v != "" {
					env = append(env, k+"="+v)
				}
			}
			args, env, err := prepareTickLaunch("claude", []string{exe, "--cellctl-cadence-fixture", "Invoke skill"}, env, time.Second)
			must(t, err)
			cfg := c.cadenceConfig("worker-desk")
			cfg.Heartbeat = 5 * time.Millisecond
			stop := errors.New("two enrollment attempts complete")
			aged := false
			cfg.Guard = func() error {
				s, err := cellcadence.Read(c.cadenceDir("worker-desk"))
				if err != nil {
					return nil
				}
				if s.Sequence == 2 && !s.Running {
					return stop
				}
				if mode == "age-during-interval" && s.Sequence == 1 && !s.Running && s.Maintenance == "checked-clean" && !aged {
					// Cross the real persisted report age boundary after post-pass
					// maintenance and before the next due pass, without a 5m sleep.
					b, err := os.ReadFile(filepath.Join(p.Root, "report.json"))
					if err != nil {
						return err
					}
					var r cellcache.Report
					if err = json.Unmarshal(b, &r); err != nil {
						return err
					}
					r.After.At = time.Now().Add(-cellcache.AdmitMaxAge - time.Second)
					b, err = json.Marshal(r)
					if err != nil {
						return err
					}
					if err = os.WriteFile(filepath.Join(p.Root, "report.json"), b, 0600); err != nil {
						return err
					}
					aged = true
				}
				return nil
			}
			attempts := 0
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			err = l.Run(ctx, cfg, func(ctx context.Context) cellcadence.Result {
				attempts++
				r := c.executeCadencePass(ctx, "worker-desk", "claude", args, env, wt)
				if attempts == 1 && mode == "maintenance-failure" {
					must(t, os.WriteFile(filepath.Join(p.Root, "owner"), []byte("corrupt ownership evidence"), 0600))
				}
				return r
			})
			if !errors.Is(err, stop) {
				t.Fatalf("supervisor did not reach second enrollment: %v", err)
			}
			b, err := os.ReadFile(record)
			must(t, err)
			state, err := cellcadence.Read(c.cadenceDir("worker-desk"))
			must(t, err)
			if mode == "maintenance-failure" {
				if string(b) != "pass\n" || state.Outcome != "could-not-check" || state.Maintenance != "could-not-check" {
					t.Fatalf("maintenance failure authorized enrollment: passes=%q state=%+v", b, state)
				}
			} else if string(b) != "pass\npass\n" || state.Outcome != "noop" {
				t.Fatalf("between-pass maintenance did not restore next child admission (%s): passes=%q state=%+v", mode, b, state)
			}
			if mode == "age-during-interval" && !aged {
				t.Fatal("age boundary was not crossed")
			}
			if strings.Contains(state.Summary, "maintenance") {
				t.Fatal("maintenance replaced child completion evidence")
			}
		})
	}
}
