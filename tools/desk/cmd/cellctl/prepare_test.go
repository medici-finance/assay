package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellscratch"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func preparedCell(t *testing.T) *Cell {
	t.Helper()
	return &Cell{Name: "fixture", Kind: "house", Dir: t.TempDir(), Config: t.TempDir(), Env: envWith(map[string]string{})}
}
func retainedHistory(t *testing.T, c *Cell, n int) map[string][32]byte {
	t.Helper()
	s, err := cellscratch.Open(filepath.Join(c.Dir, "run", "scratch"))
	must(t, err)
	defer s.Close()
	hashes := map[string][32]byte{}
	for i := 0; i < n; i++ {
		r, err := s.Begin("old", "worker", "source")
		must(t, err)
		for j := 0; j < 50; j++ {
			path := filepath.Join(r.Work(), fmt.Sprintf("dir/file-%d", j))
			must(t, os.MkdirAll(filepath.Dir(path), 0700))
			b := []byte(strings.Repeat("protected", 100))
			must(t, os.WriteFile(path, b, 0600))
			hashes[path] = sha256.Sum256(b)
		}
		if i%3 == 0 {
			must(t, os.WriteFile(filepath.Join(r.Work(), ".git"), []byte("gitdir: retained"), 0600))
		}
		must(t, r.Finish(0, true))
		must(t, r.Close())
	}
	return hashes
}
func assertHistory(t *testing.T, hashes map[string][32]byte) {
	t.Helper()
	for path, want := range hashes {
		b, err := os.ReadFile(path)
		if err != nil || sha256.Sum256(b) != want {
			t.Fatalf("protected history changed: %v", err)
		}
	}
}

// regression: #3353
func TestCellPrepBounded(t *testing.T) {
	t.Run("actual up", cellPrepUp)
	t.Run("walking callers", startupWalkGuard)
	t.Run("cache enrollment", TestCellCacheEnrollmentBounded)
	for _, history := range []int{1, 10} {
		for _, roles := range []int{1, 5} {
			t.Run(fmt.Sprintf("history%d/roles%d", history, roles), func(t *testing.T) {
				c := preparedCell(t)
				hashes := retainedHistory(t, c, history)
				fresh, err := c.prepareLaunch()
				must(t, err)
				if !fresh {
					t.Fatal("cold preparation was not recorded")
				}
				// Hold the production maintenance lease as if inventory were paused mid-scan.
				f, err := os.OpenFile(filepath.Join(c.Dir, "run", "scratch", "maintenance.lock"), os.O_CREATE|os.O_RDWR, 0600)
				must(t, err)
				must(t, deskkit.TryLockExclusive(f))
				defer f.Close()
				began := time.Now()
				probe := perfrec.SystemProbe()
				measurement := perfrec.Begin(probe, probe.Mark(), nil)
				var lockWait time.Duration
				var probes []upProbe
				for i := 0; i < roles; i++ {
					probes = append(probes, upProbe{Role: knownRoles[i], Kind: probeAbsent})
				}
				rows := c.convergeRoles(probes, nil, func(_ string, _ time.Time) (string, error) { return "", nil }, func(role string) error {
					env := fixtureProcessEnv("ASSAY_SOURCE_REVISION=source", "DESK_LOOP="+role)
					s, _, err := c.beginScratch(role, env)
					if err != nil {
						return err
					}
					result := cellcadence.Result{ExitCode: 0}
					s.finish(&result)
					lockWait += s.store.LockWait()
					if result.Err != nil {
						return result.Err
					}
					if s.store.Traversals() != 0 {
						t.Fatal("startup/finish recursively walked history")
					}
					return nil
				})
				if !upRowsOK(rows) || len(rows) != roles {
					t.Fatal(rows)
				}
				for i := 0; i < 3; i++ {
					fresh, err := c.prepareLaunch()
					must(t, err)
					if fresh {
						t.Fatal("compatible preparation repeated")
					}
				}
				env := fixtureProcessEnv("CELL_SCRATCH_FIXTURE=1", "ASSAY_SOURCE_REVISION=source", "DESK_LOOP=worker-desk", "SCRATCH_EVIDENCE="+filepath.Join(t.TempDir(), "result"))
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				result := c.executeCadencePass(ctx, "worker-desk", "claude", []string{os.Args[0], "fixture"}, env, t.TempDir())
				if result.Err != nil || result.ExitCode != 0 {
					t.Fatal(result)
				}
				assertHistory(t, hashes)
				sample := measurement.Finish("cellctl", "desk", "none", 0, "")
				cpu := "could-not-check"
				if sample.CPUMs != nil {
					cpu = fmt.Sprint(*sample.CPUMs)
				}
				t.Logf("build=current workload=protected-files:%d roles:%d samples=1 wall=%s cpu_ms=%s cpu_coverage=%s lock_wait=%s", history*50, roles, time.Since(began), cpu, sample.CPU, lockWait)
			})
		}
	}
}

// regression: #3353
func TestCellPrepFences(t *testing.T) {
	t.Run("actual concurrent starts and down", cellConcurrentStarts)
	t.Run("interrupted owner", TestInteractiveOwnerCrashRefusesCadence)
	t.Run("profile content", cellProfileChanges)
	t.Run("cache accounting", TestCellCacheEnrollmentBounded)
	c := preparedCell(t)
	c.Env.Put("CELL_SCRATCH_MAX_BYTES", "invalid")
	if _, err := c.prepareLaunch(); err == nil {
		t.Fatal("invalid scratch policy published preparation")
	}
	if _, err := os.Stat(filepath.Join(c.Dir, "run", "scratch", "preparation.json")); !os.IsNotExist(err) {
		t.Fatal("failed preparation left a reusable receipt")
	}
	c.Env.Put("CELL_SCRATCH_MAX_BYTES", "")
	hashes := retainedHistory(t, c, 10)
	var wg sync.WaitGroup
	fresh := make(chan bool, 8)
	fail := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); b, e := c.prepareLaunch(); fresh <- b; fail <- e }()
	}
	wg.Wait()
	close(fresh)
	close(fail)
	n := 0
	for b := range fresh {
		if b {
			n++
		}
	}
	for e := range fail {
		must(t, e)
	}
	if n != 1 {
		t.Fatalf("compatible preparations=%d", n)
	}
	for _, key := range []string{"CELL_PROVIDER_TOKEN", "MODEL_API_KEY"} {
		c.Env.Put(key, "fixture-secret")
		b, e := c.prepareLaunch()
		must(t, e)
		if b {
			t.Fatalf("credential-bearing environment entered preparation binding: %s", key)
		}
	}
	for _, generation := range []string{"one", "two"} {
		c.Env.Put("ASSAY_OPERATOR_GENERATION", generation)
		b, e := c.prepareLaunch()
		must(t, e)
		if !b {
			t.Fatal("generation reused stale preparation")
		}
	}
	c.Env.Put("CELL_SCRATCH_MAX_BYTES", "1")
	b, e := c.prepareLaunch()
	must(t, e)
	if !b {
		t.Fatal("changed configuration reused receipt")
	}
	role := "worker-desk"
	lease, err := cellcadence.AcquireRole(c.cadenceDir(role), cellcadence.RoleLock{Cell: c.Name, Role: role})
	must(t, err)
	defer lease.Close()
	if second, e := cellcadence.AcquireRole(c.cadenceDir(role), cellcadence.RoleLock{Cell: c.Name, Role: role}); e == nil {
		second.Close()
		t.Fatal("preparation bypassed current role lease")
	}
	must(t, os.MkdirAll(c.Config, 0700))
	must(t, os.WriteFile(filepath.Join(c.Config, "STOP"), nil, 0600))
	if c.cadenceGuard(role) == nil {
		t.Fatal("prepared launch bypassed current stop")
	}
	maintainScratch(t, c)
	assertHistory(t, hashes)
}

// regression: #3353
func TestCellCacheEnrollmentBounded(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("managed cache platform contract excludes Windows")
	}
	c := preparedCell(t)
	c.Env.Put("CELL_GO_CACHE", "on")
	c.Env.Put("CELL_GO_CACHE_MIN_FREE", "0")
	p, err := c.cachePolicy()
	must(t, err)
	_, err = cellcache.Prepare(*p)
	must(t, err)
	before, err := os.ReadFile(p.ReportPath())
	must(t, err)
	l, err := cellcache.Acquire(p.Env())
	must(t, err)
	must(t, l.Finish(true))
	// Empty cold directories may be re-accounted with bounded reads; all other
	// enrollment preserves the full report. A stale report must always refuse.
	var rep cellcache.Report
	must(t, json.Unmarshal(before, &rep))
	rep.After.At = time.Now().Add(-time.Hour)
	raw, err := json.Marshal(rep)
	must(t, err)
	must(t, os.WriteFile(p.ReportPath(), raw, 0600))
	if l, err = cellcache.Acquire(p.Env()); err == nil {
		l.Finish(true)
		t.Fatal("stale accounting admitted a child")
	}
	for _, body := range [][]byte{[]byte("corrupt"), nil} {
		must(t, os.WriteFile(p.ReportPath(), body, 0600))
		if _, err = cellcache.Prepare(*p); err == nil {
			t.Fatal("missing/corrupt accounting treated as zero")
		}
	}
}

func cellProfileChanges(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	c := preparedCell(t)
	c.Env.Put("CELL_MODEL_POLICY", f.policy)
	fresh, err := c.prepareLaunch()
	must(t, err)
	if !fresh {
		t.Fatal("profile preparation absent")
	}
	f.rewritePolicy(t, func(m map[string]any) {
		roles := m["roles"].(map[string]any)
		worker := roles["worker-desk"].(map[string]any)
		worker["tier"] = "top"
	})
	fresh, err = c.prepareLaunch()
	must(t, err)
	if !fresh {
		t.Fatal("changed profile content reused stale receipt")
	}
	must(t, os.WriteFile(f.policy, []byte("corrupt"), 0600))
	if _, err = c.prepareLaunch(); err == nil {
		t.Fatal("corrupt profile published preparation success")
	}
}
