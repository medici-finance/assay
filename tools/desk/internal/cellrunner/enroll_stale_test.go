//go:build darwin || linux

package cellrunner

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
)

// regression: #6593
//
// Final consumer enrollment is the boundary every supervised child crosses,
// including the hidden `cellctl cache-run` pane supervisor (it calls
// RunInteractive with no preparation of its own). On a recorded measurement
// that is aged, future-dated, unstamped or missing it must refuse WITHOUT
// re-measuring: no child starts, report.json is left byte-for-byte as it was
// (a clean would rewrite it), no consumer record is left behind, and the
// refusal hands the clean to the operator. Re-measuring belongs to the launch
// owner's preparation, before enrollment; see docs/cellctl-go-cache.md,
// "Consumer enrollment refuses a stale measurement".
func TestEnrollStaleRefusesNoWalk(t *testing.T) {
	for _, kind := range []string{"aged", "future", "zero", "missing"} {
		t.Run(kind, func(t *testing.T) {
			cell, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			p, err := cellcache.Resolve(cell, func(k string) string {
				switch k {
				case "CELL_GO_CACHE":
					return "on"
				case "CELL_GO_CACHE_MIN_FREE":
					return "0"
				}
				return ""
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = cellcache.Prepare(*p); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(p.ReportPath())
			if err != nil {
				t.Fatal(err)
			}
			var rep cellcache.Report
			if err = json.Unmarshal(b, &rep); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "aged":
				rep.After.At = time.Now().Add(-cellcache.AdmitMaxAge - time.Minute)
			case "future":
				rep.After.At = time.Now().Add(time.Hour)
			case "zero":
				rep.After.At = time.Time{}
			}
			if kind == "missing" {
				err = os.Remove(p.ReportPath())
			} else {
				b, _ = json.Marshal(rep)
				err = os.WriteFile(p.ReportPath(), b, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, beforeErr := os.ReadFile(p.ReportPath())
			marker := filepath.Join(cell, "child-ran")
			started := false
			_, _, err = RunObserved(t.Context(), []string{"/bin/sh", "-c", `: > "$1"`, "sh", marker}, p.Env(), "", io.Discard, io.Discard, func(int) error { started = true; return nil })
			var held *cellcache.Deferred
			if !errors.As(err, &held) {
				t.Fatalf("enrollment did not refuse a %s measurement: %v", kind, err)
			}
			if started {
				t.Fatal("a child started on an unproven measurement")
			}
			if _, e := os.Stat(marker); !errors.Is(e, os.ErrNotExist) {
				t.Fatalf("child ran: %v", e)
			}
			if !strings.Contains(err.Error(), "run cellctl cache <cell> clean") {
				t.Fatalf("refusal does not hand the clean to the operator: %v", err)
			}
			after, afterErr := os.ReadFile(p.ReportPath())
			if !bytes.Equal(before, after) || (beforeErr == nil) != (afterErr == nil) {
				t.Fatalf("enrollment re-measured: report.json changed (%v -> %v)", beforeErr, afterErr)
			}
			left, err := filepath.Glob(filepath.Join(p.Root, "active-*"))
			if err != nil || len(left) != 0 {
				t.Fatalf("refused enrollment left a consumer record: %v %v", left, err)
			}
		})
	}
}
