package cellcadence

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A launch that meets a busy role lease names the lease and its holder and stays
// ErrBusy for every caller; a status probe still answers at once.
func TestRoleLeaseBusyNamed(t *testing.T) {
	dir := t.TempDir()
	owner, err := AcquireRole(dir, RoleLock{Cell: "fixture", Role: "worker-desk"})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if err = owner.RecordChild(0); err != nil {
		t.Fatal(err)
	}
	if _, err = Acquire(dir); err != ErrBusy {
		t.Fatalf("probe: want bare ErrBusy, got %v", err)
	}
	_, err = AcquireRole(dir, RoleLock{Cell: "fixture", Role: "worker-desk", Wait: 50 * time.Millisecond, OptIn: "--end-stale-lock-holder"})
	var busy *deskkit.BusyError
	if !errors.Is(err, ErrBusy) || !errors.As(err, &busy) {
		t.Fatalf("want ErrBusy wrapping a BusyError, got %v", err)
	}
	for _, want := range []string{`"role lease"`, "owner.lock", "holder:", "holder current:", "action:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("busy role lease lacks %q: %v", want, err)
		}
	}
}
