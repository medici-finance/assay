//go:build darwin || linux

package cellprocess

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// regression: #3353
func TestCacheEnrollmentTelemetry(t *testing.T) {
	// Exercise the public final custody boundary directly, including a config-only
	// child opt-in opposing the ambient process value. No helper binary may bind
	// this oracle to another source tree during a copied mutation run.
	t.Setenv("ASSAY_TELEMETRY", "0")
	root := t.TempDir()
	t.Setenv("HOME", root)
	for _, enabled := range []bool{false, true} {
		value := "0"
		if enabled {
			value = "1"
		}
		code, uncertain, err := Run(context.Background(), []string{"/bin/sh", "-c", "printf child-output; exit 0"}, []string{"ASSAY_TELEMETRY=" + value}, "", io.Discard, io.Discard)
		if code != 0 || uncertain || err != nil {
			t.Fatalf("telemetry changed child result: %d %v %v", code, uncertain, err)
		}
		count := 0
		err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			dec := json.NewDecoder(f)
			for {
				var row struct {
					Verb string `json:"verb"`
				}
				err := dec.Decode(&row)
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				if row.Verb == "step:launch-cache-enrollment" {
					count++
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if enabled {
			want = 1
		}
		if count != want {
			t.Fatalf("final enrollment telemetry count=%d want=%d (resolved consent=%v)", count, want, enabled)
		}
	}
}
