package deskkit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #3353
func TestMaintenanceScanBound(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 80; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("entry-%03d", i)), []byte("retain"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if entries, err := ReadDirBounded(f, 32); !errors.Is(err, ErrScanLimit) || entries != nil {
		t.Fatalf("partial listing gained authority: %d %v", len(entries), err)
	}
	seen := 0
	err = BoundedWalkDir(os.DirFS(root), ".", 32, func(_ string, _ fs.DirEntry, err error) error { seen++; return err })
	if !errors.Is(err, ErrScanLimit) || seen != 32 {
		t.Fatalf("wide scan not bounded: %d %v", seen, err)
	}
}

// regression: #3353
func TestMaintenanceBudgetRefusal(t *testing.T) {
	for _, value := range []string{"0", "-1", "1048577", "bad"} {
		t.Setenv("ASSAY_MAINTENANCE_ENTRY_LIMIT", value)
		if _, err := MaintenanceEntryLimit(); err == nil {
			t.Fatalf("invalid budget passed: %q", value)
		}
	}
}

// A paged walk visits every entry of a tree wider than one page, runs next between
// pages, stops on next's error, and still refuses a tree past the depth bound.
//
// regression: #5560
func TestPagedWalkDir(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 80; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("entry-%03d", i)), []byte("retain"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	seen, pages := 0, 0
	count := func(_ string, _ fs.DirEntry, err error) error { seen++; return err }
	err := PagedWalkDir(os.DirFS(root), ".", 32, func() error { pages++; return nil }, count)
	if err != nil || seen != 81 || pages != 2 {
		t.Fatalf("paged walk gave up at the bound: seen=%d pages=%d %v", seen, pages, err)
	}
	stop := errors.New("stopped")
	seen = 0
	if err := PagedWalkDir(os.DirFS(root), ".", 32, func() error { return stop }, count); !errors.Is(err, stop) || seen != 31 {
		t.Fatalf("paged walk ignored its stop: seen=%d %v", seen, err)
	}
	deep := filepath.Join(root, strings.Repeat("d/", 70))
	if err := os.MkdirAll(deep, 0700); err != nil {
		t.Fatal(err)
	}
	if err := PagedWalkDir(os.DirFS(root), ".", 32, nil, count); !errors.Is(err, ErrScanLimit) {
		t.Fatalf("paged walk ignored the depth bound: %v", err)
	}
}
