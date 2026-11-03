package deskkit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
