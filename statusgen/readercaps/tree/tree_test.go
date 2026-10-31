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
			// Only stream indices and direct/done brief records belong to this reader.
			for _, sub := range []string{"examples", "done"} {
				child := filepath.Join(dir, sub)
				if err := os.MkdirAll(child, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(child, "README.md"), []byte("---\nProse introduced by a horizontal rule.\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if got := Check([]string{root}, reader); got.State != readercaps.Supported {
					t.Fatal("non-record README stopped a legacy tree:", got)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "examples", "brief-99-example.md"), []byte(requirement), 0644); err != nil {
				t.Fatal(err)
			}
			if got := Check([]string{root}, reader); got.State != readercaps.Supported {
				t.Fatal("example document became an operational record:", got)
			}
			archived := filepath.Join(dir, "done", "brief-01-example.md")
			if err := os.WriteFile(archived, []byte(requirement), 0644); err != nil {
				t.Fatal(err)
			}
			if got := Check([]string{root}, reader); got.State != readercaps.Unsupported {
				t.Fatal("done brief omitted from admission:", got)
			}
			if err := os.Remove(archived); err != nil {
				t.Fatal(err)
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
	for _, base := range []string{"streams", "archive"} {
		for _, name := range []string{"intake", "requirements", "custom-register"} {
			t.Run(base+"-register-"+name, func(t *testing.T) {
				root := t.TempDir()
				dir := filepath.Join(root, "docs", base, name)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				raw := "---\nAn ordinary register index.\n"
				if name == "custom-register" {
					raw += "It is a register, not a stream.\n"
				}
				if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(raw), 0644); err != nil {
					t.Fatal(err)
				}
				if got := Check([]string{root}, reader); got.State != readercaps.Supported {
					t.Fatal("register index treated as a stream:", got)
				}
				if err := os.WriteFile(filepath.Join(dir, "brief-99-example.md"), []byte(requirement), 0644); err != nil {
					t.Fatal(err)
				}
				if got := Check([]string{root}, reader); got.State != readercaps.Unsupported {
					t.Fatal("register index hid a brief requirement:", got)
				}
			})
		}
	}
	t.Run("marker-cannot-hide-stream", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "docs", "streams", "example")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(requirement+"It is a register, not a stream.\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if got := Check([]string{root}, reader); got.State != readercaps.Unsupported {
			t.Fatal("valid stream requirements hidden by prose:", got)
		}
	})
	for _, kind := range []string{"directory", "markdown"} {
		t.Run("non-record-link-"+kind, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "docs", "streams")
			if err := os.MkdirAll(base, 0755); err != nil {
				t.Fatal(err)
			}
			target := t.TempDir()
			name := "linked-dir"
			if kind == "markdown" {
				target = filepath.Join(target, "note.md")
				name = "note.md"
				if err := os.WriteFile(target, []byte("---\nProse.\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(target, filepath.Join(base, name)); err != nil {
				t.Skip(err)
			}
			if got := Check([]string{root}, reader); got.State != readercaps.Supported {
				t.Fatal("ignored depth-one link became a record:", got)
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
