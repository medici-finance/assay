package gitmaintenance

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2525
func TestTempAbsenceProof(t *testing.T) {
	for _, afterCAS := range []bool{false, true} {
		for _, state := range []string{"unreadable", "malformed", "absent"} {
			t.Run(state+map[bool]string{false: "/initial", true: "/after-CAS"}[afterCAS], func(t *testing.T) {
				root, old := refWorld(t)
				tmp := seedTemp(t, root, old)
				refPath := filepath.Join(root, ".git", filepath.FromSlash(tmp.Name))
				corrupt := func() {
					if err := os.Remove(refPath); err != nil {
						t.Fatal(err)
					}
					switch state {
					case "unreadable":
						// A directory cannot be read as a loose reference, even as root. The
						// upstream reader reports it missing; no permission-dependent skip.
						if err := os.Mkdir(refPath, 0700); err != nil {
							t.Fatal(err)
						}
					case "malformed":
						if err := os.WriteFile(refPath, []byte("broken\n"), 0600); err != nil {
							t.Fatal(err)
						}
					}
				}
				prev := refProcess
				calls := 0
				casErr := errors.New("fixture CAS failed")
				if afterCAS {
					refProcess = func(context.Context, []string, []string, string, io.Writer, io.Writer) (int, bool, error) {
						calls++
						corrupt()
						return 1, false, casErr
					}
					t.Cleanup(func() { refProcess = prev })
				} else {
					corrupt()
				}
				row, err := tmp.Complete(context.Background())
				if afterCAS && calls != 1 {
					t.Fatalf("CAS calls=%d, row=%+v err=%v", calls, row, err)
				}
				if state == "absent" {
					if err != nil || row.State != "absent" {
						t.Fatalf("proved absence=%+v,%v", row, err)
					}
					if _, err := os.Stat(tmp.path); !os.IsNotExist(err) {
						t.Fatalf("absent receipt was not drained: %v", err)
					}
					return
				}
				if row.State != "held" || err == nil || !strings.Contains(row.Reason, "could-not-check") || deskkit.ExitCodeOf(err) != 6 {
					t.Errorf("uncertain ref=%+v,%v (exit=%d)", row, err, deskkit.ExitCodeOf(err))
				}
				if afterCAS && !errors.Is(err, casErr) {
					t.Errorf("lost CAS failure cause: %v", err)
				}
				if _, statErr := os.Lstat(refPath); statErr != nil {
					t.Errorf("uncertain ref lost: %v", statErr)
				}
				body, readErr := os.ReadFile(tmp.path)
				var receipt tempReceipt
				if readErr != nil || json.Unmarshal(body, &receipt) != nil || !receipt.Complete || receipt.CleanupPending || receipt.Hash != old {
					t.Fatalf("recovery custody lost: receipt=%+v read=%v", receipt, readErr)
				}
				if err := os.Remove(refPath); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(refPath, []byte(old+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				refProcess = prev
				rows, err := RecoverTempRefs(context.Background(), root, 256)
				if err != nil || len(rows) != 1 || rows[0].State != "deleted" {
					t.Fatalf("restored recovery=%+v,%v", rows, err)
				}
				assertRef(t, root, tmp.Name, "")
				if got := refGit(t, root, "cat-file", "-t", old); got != "commit" {
					t.Fatalf("object lost: %s", got)
				}
			})
		}
	}
}

// regression: #2525
func TestTempInvalidHeadName(t *testing.T) {
	root, old := refWorld(t)
	tmp := seedTemp(t, root, old)
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/bad..name\n"), 0600); err != nil {
		t.Fatal(err)
	}
	row, err := tmp.Complete(context.Background())
	if err == nil || row.State != "held" {
		t.Fatalf("invalid HEAD name admitted: %+v,%v", row, err)
	}
	if _, err := os.Stat(tmp.path); err != nil {
		t.Fatalf("invalid HEAD drained receipt: %v", err)
	}
	assertRef(t, root, tmp.Name, old)
}
