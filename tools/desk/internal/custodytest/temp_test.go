package custodytest

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateTempDir(t *testing.T) {
	dir := PrivateTempDir(t)
	file := filepath.Join(dir, "example.token")
	if err := os.WriteFile(file, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	got := deskkit.ClassifyCustodyOwnerOnly(file)
	if got.State != deskkit.CustodyVerified {
		t.Fatalf("custody=%v: %v", got.State, got.Err)
	}
}
