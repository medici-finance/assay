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
// RunInteractive with no preparation of its own). On an existing root whose
// recorded measurement is aged, future-dated, unstamped or missing it must
// refuse WITHOUT
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
			p := smallPolicy(t, cell)
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

// smallPolicy resolves an enabled cache policy for cell with a 1 MiB budget
// and no floor, so the empty-root admission's free-space margin (floor plus a
// quarter of the budget) does not depend on the host's free temp space.
func smallPolicy(t *testing.T, cell string) *cellcache.Policy {
	t.Helper()
	p, err := cellcache.Resolve(cell, func(k string) string {
		switch k {
		case "CELL_GO_CACHE":
			return "on"
		case "CELL_GO_CACHE_BYTES":
			return "1048576"
		case "CELL_GO_CACHE_MIN_FREE":
			return "0"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// regression: #6593
//
// The one enrollment that admits with no prior measurement: when the cache
// root does not exist yet, enrollment creates it, so it is known empty without
// an inventory. It records a bounded-empty report after one free-space check
// and starts the child. docs/cellctl-go-cache.md, "Consumer enrollment refuses
// a stale measurement", states this exception.
func TestEnrollCreatedRootAdmits(t *testing.T) {
	cell, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := smallPolicy(t, cell)
	if _, err = os.Lstat(p.Root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cache root exists before enrollment: %v", err)
	}
	marker := filepath.Join(cell, "child-ran")
	started := false
	_, _, err = RunObserved(t.Context(), []string{"/bin/sh", "-c", `: > "$1"`, "sh", marker}, p.Env(), "", io.Discard, io.Discard, func(int) error { started = true; return nil })
	if err != nil {
		t.Fatalf("enrollment refused a root it created: %v", err)
	}
	if !started {
		t.Fatal("no child started on a created root")
	}
	if _, err = os.Stat(marker); err != nil {
		t.Fatalf("child did not run: %v", err)
	}
	b, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatalf("created-root admission recorded no report: %v", err)
	}
	var rep cellcache.Report
	if err = json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Admission != "bounded-empty" || rep.Outcome != "checked-clean" {
		t.Fatalf("created-root report = %s/%s, want bounded-empty/checked-clean", rep.Admission, rep.Outcome)
	}
}
