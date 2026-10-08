//go:build unix

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// canary is a FIFO at one location the key resolver consults, watched by a detector
// goroutine. The detector repeatedly opens the FIFO write-only and non-blocking: that open
// succeeds only while some reader has the FIFO open (it fails with ENXIO otherwise), so a
// success means something tried to read a key there. On success the detector closes its end
// at once, so the reader sees EOF instead of blocking.
type canary struct {
	path  string
	fired atomic.Bool
	stop  chan struct{}
	wg    sync.WaitGroup
}

func newCanary(t *testing.T, path string) *canary {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("mkfifo %s: %v", path, err)
	}
	c := &canary{path: path, stop: make(chan struct{})}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.stop:
				return
			default:
			}
			if f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				c.fired.Store(true)
				f.Close()
			}
			time.Sleep(time.Millisecond)
		}
	}()
	t.Cleanup(c.halt)
	return c
}

func (c *canary) halt() {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	c.wg.Wait()
}

// canaryEnv pins HOME to H and ASSAY_CONFIG_HOME to C, and places the three canaries:
// A (named by VERIFIER_PEM in the arms that set it), B at C/verifier-app.pem and
// C at H/.config/assay/verifier-app.pem.
type canaryEnv struct {
	home, cfg string
	a, b, c   *canary
	aPath     string
}

func newCanaryEnv(t *testing.T) *canaryEnv {
	t.Helper()
	e := &canaryEnv{home: t.TempDir(), cfg: t.TempDir()}
	e.aPath = filepath.Join(t.TempDir(), "named-by-env.pem")
	e.a = newCanary(t, e.aPath)
	e.b = newCanary(t, filepath.Join(e.cfg, "verifier-app.pem"))
	e.c = newCanary(t, filepath.Join(e.home, ".config", "assay", "verifier-app.pem"))
	t.Setenv("HOME", e.home)
	return e
}

// runBounded runs fn and fails the test if it has not returned within 30s (a reader blocked
// on a canary nobody released); -timeout bounds the whole run on top.
func runBounded(t *testing.T, name string, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		t.Fatalf("%s: run did not return within 30s", name)
		return nil
	}
}

// Row 2.
func TestUnsignedOutNeverOpensVerifierPEMCanary(t *testing.T) {
	root := demoRoot(t)

	unsigned := func(name string) error {
		out := filepath.Join(t.TempDir(), "payload.json")
		var buf bytes.Buffer
		err := runBounded(t, name, func() error {
			return runVerdict(verdictRunConfig{root: root, repo: exRepo, head: exHead,
				exec: fakeExec, out: &buf, unsignedOut: out})
		})
		if err == nil {
			if _, serr := os.Stat(out); serr != nil {
				t.Fatalf("%s: payload file not written: %v", name, serr)
			}
		}
		return err
	}
	signed := func(name string) error {
		var buf bytes.Buffer
		return runBounded(t, name, func() error {
			return runVerdict(verdictRunConfig{root: root, repo: exRepo, head: exHead,
				dryRun: true, window: time.Hour, exec: fakeExec, out: &buf})
		})
	}
	// settle gives a detector time to notice a reader that opened in the last poll interval.
	settle := func() { time.Sleep(50 * time.Millisecond) }

	t.Run("U1-all-three-live", func(t *testing.T) {
		e := newCanaryEnv(t)
		t.Setenv("VERIFIER_PEM", e.aPath)
		t.Setenv("ASSAY_CONFIG_HOME", e.cfg)
		if err := unsigned("U1"); err != nil {
			t.Fatalf("U1: unsigned run = %v, want nil", err)
		}
		settle()
		for name, c := range map[string]*canary{"A (VERIFIER_PEM)": e.a, "B (ASSAY_CONFIG_HOME)": e.b, "C (HOME)": e.c} {
			if c.fired.Load() {
				t.Fatalf("U1: canary %s fired — the unsigned run opened a verifier key location", name)
			}
		}
	})

	t.Run("U2-home-only", func(t *testing.T) {
		e := newCanaryEnv(t)
		t.Setenv("VERIFIER_PEM", "")
		t.Setenv("ASSAY_CONFIG_HOME", "")
		if err := unsigned("U2"); err != nil {
			t.Fatalf("U2: unsigned run = %v, want nil", err)
		}
		settle()
		if e.c.fired.Load() {
			t.Fatalf("U2: canary C (HOME) fired — the unsigned run opened ~/.config/assay/verifier-app.pem")
		}
	})

	// CONTROL arms: the signed path opens each location, so each detector is proven live.
	for _, k := range []struct {
		name     string
		env, cfg func(e *canaryEnv) string
		want     func(e *canaryEnv) *canary
	}{
		{"K1-env", func(e *canaryEnv) string { return e.aPath }, func(e *canaryEnv) string { return e.cfg }, func(e *canaryEnv) *canary { return e.a }},
		{"K2-config-home", func(*canaryEnv) string { return "" }, func(e *canaryEnv) string { return e.cfg }, func(e *canaryEnv) *canary { return e.b }},
		{"K3-home", func(*canaryEnv) string { return "" }, func(*canaryEnv) string { return "" }, func(e *canaryEnv) *canary { return e.c }},
	} {
		t.Run(k.name, func(t *testing.T) {
			e := newCanaryEnv(t)
			t.Setenv("VERIFIER_PEM", k.env(e))
			t.Setenv("ASSAY_CONFIG_HOME", k.cfg(e))
			err := signed(k.name)
			if !k.want(e).fired.Load() {
				t.Fatalf("%s: control canary never fired (err %v) — the detector is broken, so the unsigned arms prove nothing", k.name, err)
			}
			if err == nil {
				t.Fatalf("%s: signed run read an empty key from the canary and still returned nil", k.name)
			}
		})
	}
}
