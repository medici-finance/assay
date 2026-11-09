package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// hostBudget is the sole lease owner. Files are stable slot names, never
// ownership markers: only an exclusive OS lock proves admission. Never unlink
// a slot file, since that would allow two owners to lock different inodes.
type hostBudget struct {
	ctx   context.Context
	dir   string
	total int
	log   io.Writer
	mu    sync.Mutex
}

func loadBudget(ctx context.Context, lookup func(string) (string, bool), log io.Writer) (*hostBudget, error) {
	dir, haveDir := lookup("MUHAR_LEASE_DIR")
	value, haveTotal := lookup("MUHAR_TOTAL_SLOTS")
	if !haveDir && !haveTotal {
		return nil, nil
	}
	total, err := strconv.Atoi(value)
	if !haveDir || !haveTotal || dir == "" || err != nil || total <= 0 {
		return nil, fmt.Errorf("MUHAR_LEASE_DIR and positive MUHAR_TOTAL_SLOTS must both be explicitly set")
	}
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("MUHAR_LEASE_DIR must be absolute")
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("MUHAR_LEASE_DIR must name an accessible existing directory")
	}
	// Validate every slot before any work. No fallback directory or partial pool.
	for i := 0; i < total; i++ {
		f, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("slot-%d.lock", i)), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("MUHAR_LEASE_DIR slot %d: %w", i, err)
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}
	return &hostBudget{ctx: ctx, dir: dir, total: total, log: log}, nil
}

func (b *hostBudget) note(format string, args ...any) {
	if b == nil || b.log == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	fmt.Fprintf(b.log, format, args...)
}

func (b *hostBudget) clamp(jobs int) int {
	if b != nil && jobs > b.total {
		b.note("muhar: budget: clamping %d workers to MUHAR_TOTAL_SLOTS=%d\n", jobs, b.total)
		return b.total
	}
	return jobs
}

// acquire polls nonblocking locks while holding NO other slot. It returns one
// release function for one independently progressing worker/command, never a
// reservation for a future pool. Queue time ends before build/test timing starts.
func (b *hostBudget) acquire() (func(), error) {
	if b == nil {
		return func() {}, nil
	}
	start := time.Now()
	waiting := false
	for {
		if err := b.ctx.Err(); err != nil {
			return nil, err
		}
		for i := 0; i < b.total; i++ {
			f, err := os.OpenFile(filepath.Join(b.dir, fmt.Sprintf("slot-%d.lock", i)), os.O_RDWR, 0600)
			if err != nil {
				return nil, fmt.Errorf("budget slot %d: %w", i, err)
			}
			err = deskkit.TryLockExclusive(f)
			if err == nil {
				if waiting {
					b.note("muhar: budget: admitted after queue-wait=%s\n", time.Since(start).Round(time.Millisecond))
				}
				var once sync.Once
				return func() { once.Do(func() { _ = deskkit.UnlockFile(f); _ = f.Close() }) }, nil
			}
			_ = f.Close()
			if !errors.Is(err, deskkit.ErrLockBusy) {
				return nil, fmt.Errorf("budget slot %d lock: %w", i, err)
			}
		}
		if !waiting {
			b.note("muhar: budget: waiting for a host slot (queue time, outside command timeout)\n")
			waiting = true
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-b.ctx.Done():
			timer.Stop()
			return nil, b.ctx.Err()
		case <-timer.C:
		}
	}
}
