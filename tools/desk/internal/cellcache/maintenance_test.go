package cellcache

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCacheMaintenanceCancellation(t *testing.T) {
	p := Policy{Root: filepath.Join(t.TempDir(), "cache"), Domain: strings.Repeat("a", 64), Budget: 4096}
	if _, err := Prepare(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Root, "build", "completed-result"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(p.Root, "report.json")
	before, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	began := time.Now()
	_, err = Maintain(ctx, p)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(began) > time.Second {
		t.Fatalf("maintenance did not cancel during settling: %v elapsed=%s", err, time.Since(began))
	}
	after, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("canceled maintenance published accounting")
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "completed-result")); err != nil {
		t.Fatal("canceled maintenance changed retained payload", err)
	}
}

func TestCacheMaintenanceFresh(t *testing.T) {
	p := Policy{Root: filepath.Join(t.TempDir(), "cache"), Domain: strings.Repeat("a", 64), Budget: 4096}
	if _, err := Prepare(p); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * AdmitMtimeSlack)
	for _, name := range []string{"build", "mod"} {
		if err := os.Chtimes(filepath.Join(p.Root, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	reportPath := filepath.Join(p.Root, "report.json")
	before, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Maintain(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if r.Admission != "precheck" || !bytes.Equal(before, after) {
		t.Fatal("fresh between-pass accounting was recursively refreshed")
	}
}
