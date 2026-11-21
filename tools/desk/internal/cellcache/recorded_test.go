//go:build darwin || linux

package cellcache

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// regression: #5371 — the read-only report reader is bounded like every other reader of
// report.json: a FIFO, an oversized file and a link all fail fast, never stall or flood.
func TestRecordedReportBounded(t *testing.T) {
	p := warmed(t)
	if r, err := RecordedReport(p); err != nil || r.After.LogicalBytes == nil || *r.After.LogicalBytes != 100 {
		t.Fatalf("recorded report not read: %+v %v", r, err)
	}
	report := p.ReportPath()
	cases := map[string]func(t *testing.T){
		"fifo": func(t *testing.T) {
			if err := syscall.Mkfifo(report, 0o600); err != nil {
				t.Skip("mkfifo:", err)
			}
		},
		"oversized": func(t *testing.T) {
			if err := os.WriteFile(report, []byte(strings.Repeat("x", 65<<10)), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"link": func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "elsewhere.json")
			if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, report); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, plant := range cases {
		t.Run(name, func(t *testing.T) {
			if err := os.Remove(report); err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatal(err)
			}
			plant(t)
			done := make(chan error, 1)
			go func() { _, err := RecordedReport(p); done <- err }()
			select {
			case err := <-done:
				if err == nil {
					t.Fatalf("%s at report.json was read as a report", name)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("%s at report.json stalled the reader", name)
			}
		})
	}
	if err := os.Remove(report); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordedReport(p); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing report = %v, want not-exist", err)
	}
}
