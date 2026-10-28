package gitmaintenance

import (
	"os"
	"path/filepath"
	"testing"
)

// regression: #2513
func TestStateReadsStayReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state")
	if _, err := openMaintenanceRegular(path); err == nil {
		t.Fatal("missing state read clean")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("read created missing state")
	}
	if err := os.WriteFile(path, []byte("record"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openMaintenanceRegular(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write([]byte("overwrite")); err == nil {
		t.Fatal("read handle carries write capability")
	}
	if _, err := openMaintenanceRegular(dir); err == nil {
		t.Fatal("directory admitted as state")
	}
}
