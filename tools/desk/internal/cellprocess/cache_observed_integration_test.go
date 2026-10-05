//go:build darwin || linux

package cellprocess

import (
	"context"
	"errors"
	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"io"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheObservedIntegration(t *testing.T) {
	for _, interactive := range []bool{false, true} {
		t.Run(map[bool]string{false: "cadence", true: "interactive"}[interactive], func(t *testing.T) {
			cell, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			p, err := cellcache.Resolve(cell, func(k string) string {
				if k == "CELL_GO_CACHE" {
					return "on"
				}
				if k == "CELL_GO_CACHE_MIN_FREE" {
					return "0"
				}
				return ""
			})
			if err != nil {
				t.Fatal(err)
			}
			argv, env := fixture(t, "sleep")
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			called := false
			stop := errors.New("fixture enrollment stop")
			observe := func(pid int) error {
				called = true
				r, e := cellcache.Check(*p, true)
				if pid <= 0 || e != nil || len(r.SkippedActive) != 1 {
					t.Errorf("callback without cache custody: %+v %v", r, e)
				}
				return stop
			}
			run := func() (int, bool, error) {
				e := append(append([]string{}, env...), p.Env()...)
				if interactive {
					return RunInteractiveObserved(ctx, argv, e, cell, nil, io.Discard, io.Discard, observe)
				}
				return RunObserved(ctx, argv, e, cell, io.Discard, io.Discard, observe)
			}
			code, uncertain, err := run()
			if !called || code == 0 || uncertain || !errors.Is(err, stop) {
				t.Fatal("callback lost", called, code, uncertain, err)
			}
			r, err := cellcache.Check(*p, true)
			if err != nil || len(r.SkippedActive) != 0 {
				t.Fatal("finished custody retained", r, err)
			}
			p.Floor = ^uint64(0)
			called = false
			_, _, err = run()
			var deferred *cellcache.Deferred
			if called || !errors.As(err, &deferred) {
				t.Fatal("admission bypassed", called, err)
			}
		})
	}
}
