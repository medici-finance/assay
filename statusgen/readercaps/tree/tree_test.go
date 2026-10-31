package tree

import (
	"github.com/medici-finance/assay/statusgen/readercaps"
	"os"
	"path/filepath"
	"testing"
)

// regression: #4947
func TestReaderCapsTree(t *testing.T) {
	reader := readercaps.Advertisement{Schema: readercaps.AdvertisementSchema, Provider: "reader", Version: "dev", Operations: []readercaps.Operation{}}
	requirement := "---\n\"reader-\\x72equirements\": {schema: \"reader-\\x72equirements-v1\", required: [{operation: dispatch, schemas: [], semantics: [future-v1]}]}\n---\n"
	for _, base := range []string{"streams", "archive"} {
		t.Run(base, func(t *testing.T) {
			root := t.TempDir()
			if got := Check([]string{root}, reader); got.State != readercaps.Supported {
				t.Fatal(got)
			}
			dir := filepath.Join(root, "docs", base, "example")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"README.md", "brief-01-example.md"} {
				path := filepath.Join(dir, name)
				if err := os.WriteFile(path, []byte(requirement), 0644); err != nil {
					t.Fatal(err)
				}
				if got := Check([]string{root}, reader); got.State != readercaps.Unsupported {
					t.Fatalf("%s: %+v", name, got)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte(requirement), 0644); err != nil {
				t.Fatal(err)
			}
			if got := Check([]string{root}, reader); got.State != readercaps.Supported {
				t.Fatal("body document became an opt-in:", got)
			}
			if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("---\nreader-requirements: null\n---\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if got := Check([]string{root}, reader); got.State != readercaps.CouldNotCheck {
				t.Fatal("malformed requirements certified:", got)
			}
		})
	}
	for _, kind := range []string{"symlink", "broken-symlink", "file"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			docs := filepath.Join(root, "docs")
			if err := os.MkdirAll(docs, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(docs, "streams")
			if kind == "file" {
				if err := os.WriteFile(path, []byte("not a directory"), 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				target := filepath.Join(root, "target")
				if kind == "symlink" {
					if err := os.Mkdir(target, 0755); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			if got := Check([]string{root}, reader); got.State != readercaps.CouldNotCheck {
				t.Fatalf("%s certified: %+v", kind, got)
			}
		})
	}
}
