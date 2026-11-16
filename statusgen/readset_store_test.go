package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// The read-set state directory's write and retention rules
// (readset_record.go writeReadSetFile, pruneReadSets). Platform-neutral:
// the writer has no build constraint, so its rules are tested on every host.

const storeDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("SKIP (could-not-check, not a pass): this host cannot create a symlink: %v", err)
	}
}

// regression: #6670
// TestReadSetWriteRefusesLink: a symlink planted at the read-set file's
// name, or in place of the state directory itself, is refused; nothing is
// written through it and the link is left as it was.
func TestReadSetWriteRefusesLink(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		state, outside := t.TempDir(), t.TempDir()
		victim := filepath.Join(outside, "victim")
		if err := os.WriteFile(victim, []byte("untouched\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(state, evaluation.ReadSetStateDir), 0o755); err != nil {
			t.Fatal(err)
		}
		link := readSetStatePath(state, storeDigest)
		symlinkOrSkip(t, victim, link)
		if err := writeReadSetFile(state, storeDigest, []byte("recorded\n")); err == nil {
			t.Fatal("a symlink at the read-set file's name was written through, want a refusal")
		}
		if b, _ := os.ReadFile(victim); string(b) != "untouched\n" {
			t.Fatalf("the link's target was changed to %q", b)
		}
		if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("the planted link was replaced or removed: %v", err)
		}
	})
	t.Run("dir", func(t *testing.T) {
		state, outside := t.TempDir(), t.TempDir()
		symlinkOrSkip(t, outside, filepath.Join(state, evaluation.ReadSetStateDir))
		if err := writeReadSetFile(state, storeDigest, []byte("recorded\n")); err == nil {
			t.Fatal("a symlinked state directory was written through, want a refusal")
		}
		if ents, _ := os.ReadDir(outside); len(ents) != 0 {
			t.Fatalf("the link's target directory gained %d entries", len(ents))
		}
	})
}

// regression: #6670
// TestReadSetWriteReplaces: the file is written to a fresh temp file and
// renamed into place, so an existing file at the name is replaced, never
// rewritten in place: a hard link to it elsewhere keeps its bytes, and no
// temp file is left behind.
func TestReadSetWriteReplaces(t *testing.T) {
	state, outside := t.TempDir(), t.TempDir()
	dir := filepath.Join(state, evaluation.ReadSetStateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(outside, "other")
	if err := os.WriteFile(other, []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(other, readSetStatePath(state, storeDigest)); err != nil {
		t.Skipf("SKIP (could-not-check, not a pass): this host cannot create a hard link: %v", err)
	}
	if err := writeReadSetFile(state, storeDigest, []byte("recorded\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if b, _ := os.ReadFile(other); string(b) != "other\n" {
		t.Fatalf("the write went through the existing file in place: its other link now reads %q", b)
	}
	if b, _ := os.ReadFile(readSetStatePath(state, storeDigest)); string(b) != "recorded\n" {
		t.Fatalf("the read-set file reads %q", b)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || ents[0].Name() != storeDigest {
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Fatalf("state directory holds %q, want only the read-set file", names)
	}
}

// regression: #6670
// TestReadSetPruneBounded: the state directory is pruned on every write to
// files younger than readSetMaxAge and at most readSetMaxFiles of them,
// newest kept; the file just written is always kept, and anything not named
// like a read-set file is left alone.
func TestReadSetPruneBounded(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, evaluation.ReadSetStateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	name := func(i int) string { return fmt.Sprintf("%064x", i+1) }
	plant := func(n string, age time.Duration) {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte(n), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(-age)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	// Fresh files, one more than the cap, aged 1..cap+1 minutes.
	for i := 0; i <= readSetMaxFiles; i++ {
		plant(name(i), time.Duration(i+1)*time.Minute)
	}
	old := name(readSetMaxFiles + 10)
	plant(old, readSetMaxAge+time.Hour)
	plant("README", readSetMaxAge+time.Hour) // not a read-set name: never pruned
	if err := writeReadSetFile(state, storeDigest, []byte("recorded\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	hex := 0
	for _, e := range ents {
		have[e.Name()] = true
		if len(e.Name()) == 64 && strings.Trim(e.Name(), "0123456789abcdef") == "" {
			hex++
		}
	}
	if hex > readSetMaxFiles {
		t.Fatalf("%d read-set files remain, want at most %d", hex, readSetMaxFiles)
	}
	if have[old] {
		t.Fatal("a file older than readSetMaxAge was kept")
	}
	if !have[storeDigest] {
		t.Fatal("the file just written was pruned")
	}
	if !have[name(0)] {
		t.Fatal("the newest planted file was pruned")
	}
	if have[name(readSetMaxFiles)] {
		t.Fatal("the oldest fresh file over the count cap was kept")
	}
	if !have["README"] {
		t.Fatal("a file not named like a read set was pruned")
	}
}

// readSetStateWriters lists the non-test functions in f that name the
// read-set state directory (evaluation.ReadSetStateDir or readSetStatePath)
// other than the one writer and the path helper, keyed file:function.
func readSetStateWriters(fset *token.FileSet, f *ast.File) []string {
	allowed := map[string]bool{"readset_record.go:writeReadSetFile": true, "readset_record.go:readSetStatePath": true}
	file := filepath.Base(fset.Position(f.Pos()).Filename)
	var sites []string
	for _, d := range f.Decls {
		key := file + ":<package scope>"
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil {
			key = file + ":" + fn.Name.Name
		} else if ok {
			key = file + ":<method " + fn.Name.Name + ">"
		}
		ast.Inspect(d, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && (id.Name == "ReadSetStateDir" || id.Name == "readSetStatePath") && !allowed[key] {
				sites = append(sites, fset.Position(n.Pos()).String()+" in "+key)
			}
			return true
		})
	}
	return sites
}

// regression: #6670
// TestReadSetStateOneWriter: every write into the read-set state directory
// goes through writeReadSetFile, the one function that refuses links and
// writes by rename. A planted second writer must be flagged.
func TestReadSetStateOneWriter(t *testing.T) {
	plant := `package main
import ("os"; "github.com/medici-finance/assay/statusgen/evaluation")
func plantedWriter(d string) error { return os.WriteFile(d+"/"+evaluation.ReadSetStateDir+"/x", nil, 0o644) }
`
	fset := token.NewFileSet()
	pf, err := parser.ParseFile(fset, "readset_record.go", plant, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := readSetStateWriters(fset, pf); len(got) != 1 || !strings.HasSuffix(got[0], ":plantedWriter") {
		t.Fatalf("POSITIVE CONTROL: guard flagged %q, want exactly plantedWriter", got)
	}
	files, _ := filepath.Glob("*.go")
	var bad []string
	seen := false
	for _, p := range files {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		seen = seen || p == "readset_record.go"
		bad = append(bad, readSetStateWriters(fset, f)...)
	}
	if !seen {
		t.Fatal("readset_record.go not found: the guard is not looking at this package")
	}
	if len(bad) > 0 {
		t.Fatalf("the read-set state directory is named outside writeReadSetFile:\n  %s", strings.Join(bad, "\n  "))
	}
}
