package local

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfdata"
)

// symlinkOrSkip plants a symbolic link, skipping where the platform or account cannot.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
}

// openOnce creates an empty store tree and closes it, so links can be planted inside.
func openOnce(t *testing.T, dir string) {
	t.Helper()
	mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)}).Close()
}

// regression: the spool followed a planted segment link and appended outside the store.
func TestStoreRefusesLinks(t *testing.T) {
	t.Run("control: a clean tree opens and writes", func(t *testing.T) {
		dir := t.TempDir()
		openOnce(t, dir)
		st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)})
		mustWrite(t, st, 1, day1)
		st.Close()
	})

	// Each case plants one link and names the outside file it must never touch.
	cases := []struct {
		name  string
		plant func(t *testing.T, root, outside string)
	}{
		{"a linked segment", func(t *testing.T, root, outside string) {
			os.WriteFile(outside, nil, 0o600)
			symlinkOrSkip(t, outside, filepath.Join(root, "segments", segmentName(1, 1, "2026-10-01")))
		}},
		{"a linked segments directory", func(t *testing.T, root, outside string) {
			os.Mkdir(outside, 0o700)
			os.Remove(filepath.Join(root, "segments"))
			symlinkOrSkip(t, outside, filepath.Join(root, "segments"))
		}},
		{"a linked lock file", func(t *testing.T, root, outside string) {
			os.WriteFile(outside, nil, 0o600)
			os.Remove(filepath.Join(root, "lock"))
			symlinkOrSkip(t, outside, filepath.Join(root, "lock"))
		}},
		{"a linked source file", func(t *testing.T, root, outside string) {
			b, _ := os.ReadFile(filepath.Join(root, "source.json"))
			os.WriteFile(outside, b, 0o600)
			os.Remove(filepath.Join(root, "source.json"))
			symlinkOrSkip(t, outside, filepath.Join(root, "source.json"))
		}},
		{"a linked store root", func(t *testing.T, root, outside string) {
			os.Rename(root, outside)
			symlinkOrSkip(t, outside, root)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name+" is refused and never written through", func(t *testing.T) {
			dir := t.TempDir()
			openOnce(t, dir)
			outside := filepath.Join(t.TempDir(), "outside")
			tc.plant(t, Root(dir), outside)
			before := snapshot(t, outside)
			st, err := Open(dir, armed, testPolicy(t, ""), Options{Now: fixed(day1)})
			if err == nil {
				st.Write([]perfdata.Draft{toolDraft(day1, 1)}, false)
				st.Close()
				t.Fatalf("Open accepted %s", tc.name)
			}
			if !errors.Is(err, ErrUnsafe) {
				t.Fatalf("want ErrUnsafe, got %v", err)
			}
			if after := snapshot(t, outside); after != before {
				t.Fatalf("the outside target changed through %s", tc.name)
			}
		})
	}

	t.Run("a link planted after Open is never written through", func(t *testing.T) {
		dir := t.TempDir()
		st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)})
		defer st.Close()
		outside := filepath.Join(t.TempDir(), "outside")
		os.WriteFile(outside, nil, 0o600)
		symlinkOrSkip(t, outside, filepath.Join(Root(dir), "segments", segmentName(1, 1, "2026-10-01")))
		if _, err := st.Write([]perfdata.Draft{toolDraft(day1, 1)}, false); !errors.Is(err, ErrUnsafe) {
			t.Fatalf("want ErrUnsafe from the append, got %v", err)
		}
		if b, _ := os.ReadFile(outside); len(b) != 0 {
			t.Fatalf("the append followed the link: %q", b)
		}
	})

	t.Run("the reader never follows a linked segment", func(t *testing.T) {
		dir := t.TempDir()
		st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)})
		mustWrite(t, st, 2, day1)
		st.Close()
		other := t.TempDir()
		ost := mustOpen(t, other, testPolicy(t, ""), Options{Now: fixed(day1)})
		mustWrite(t, ost, 3, day1)
		ost.Close()
		target := filepath.Join(Root(other), "segments", segmentsOf(t, other)[0].name)
		symlinkOrSkip(t, target, filepath.Join(Root(dir), "segments", segmentName(3, 1, "2026-10-01")))
		rr, err := Read(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(rr.Events) != 2 || rr.Coverage[0].State != perfdata.CoverageCorrupt {
			t.Fatalf("a linked segment must be counted, not read: events=%d %+v", len(rr.Events), rr.Coverage[0])
		}
	})

	t.Run("the reader refuses a linked store root", func(t *testing.T) {
		dir := t.TempDir()
		other := t.TempDir()
		st := mustOpen(t, other, testPolicy(t, ""), Options{Now: fixed(day1)})
		mustWrite(t, st, 1, day1)
		st.Close()
		symlinkOrSkip(t, Root(other), Root(dir))
		if _, err := Read(dir); !errors.Is(err, ErrUnsafe) {
			t.Fatalf("want ErrUnsafe for a linked root, got %v", err)
		}
	})
}

// regression: replacing the store directory after Open redirected later writes, alias
// map included, into the replacement.
func TestStoreBoundAtOpen(t *testing.T) {
	for _, sub := range []string{"", "segments"} {
		name := filepath.Join("store", sub)
		t.Run("replacing "+name+" after Open never redirects a write", func(t *testing.T) {
			dir := t.TempDir()
			st := mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)})
			defer st.Close()
			mustWrite(t, st, 1, day1)
			other := t.TempDir()
			openOnce(t, other)
			before := snapshot(t, Root(other))
			moved := filepath.Join(t.TempDir(), "moved")
			if err := os.Rename(filepath.Join(Root(dir), sub), moved); err != nil {
				t.Fatal(err)
			}
			symlinkOrSkip(t, filepath.Join(Root(other), sub), filepath.Join(Root(dir), sub))
			if _, err := st.Write([]perfdata.Draft{toolDraft(day1, 2)}, false); err != nil {
				t.Fatalf("the store must keep writing to the directory it opened: %v", err)
			}
			if after := snapshot(t, Root(other)); after != before {
				t.Fatalf("a write followed the replaced %s into another directory", name)
			}
			if n := strings.Count(snapshot(t, moved), `"schema":"`+perfdata.EventSchema+`"`); n != 2 {
				t.Fatalf("both events must land in the directory that was opened, found %d", n)
			}
		})
	}
}

// segmentsOf lists the parsed segment files of the store under dir, by path.
func segmentsOf(t *testing.T, dir string) []segment {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(Root(dir), "segments"))
	if err != nil {
		t.Fatal(err)
	}
	var out []segment
	for _, e := range ents {
		if sg, ok := parseSegment(e.Name()); ok {
			out = append(out, sg)
		}
	}
	return out
}

// snapshot is the content of a file, or a listing of a directory, as one string.
func snapshot(t *testing.T, p string) string {
	t.Helper()
	st, err := os.Lstat(p)
	if err != nil {
		return "absent"
	}
	if !st.IsDir() {
		b, _ := os.ReadFile(p)
		return "file:" + string(b)
	}
	out := "dir:"
	filepath.WalkDir(p, func(q string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			b, _ := os.ReadFile(q)
			out += q + "=" + string(b) + ";"
		}
		return nil
	})
	return out
}
