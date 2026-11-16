package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
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
		if err := writeReadSetFile(state, storeDigest, []byte("recorded\n"), randomTempSuffix); err == nil {
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
		if err := writeReadSetFile(state, storeDigest, []byte("recorded\n"), randomTempSuffix); err == nil {
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
	if err := writeReadSetFile(state, storeDigest, []byte("recorded\n"), randomTempSuffix); err != nil {
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
	if err := writeReadSetFile(state, storeDigest, []byte("recorded\n"), randomTempSuffix); err != nil {
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
	// Exactly the cap remains: the file just written plus the newest
	// readSetMaxFiles-1 planted, name(0)..name(readSetMaxFiles-2).
	if hex != readSetMaxFiles {
		t.Fatalf("%d read-set files remain, want exactly %d", hex, readSetMaxFiles)
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
	if !have[name(readSetMaxFiles-2)] {
		t.Fatal("the oldest planted file within the count cap was pruned")
	}
	if have[name(readSetMaxFiles-1)] {
		t.Fatal("a planted file beyond the count cap was kept")
	}
	if have[name(readSetMaxFiles)] {
		t.Fatal("the oldest fresh file over the count cap was kept")
	}
	if !have["README"] {
		t.Fatal("a file not named like a read set was pruned")
	}
}

// regression: #6692
// TestReadSetPruneAgeBoundary: pruneReadSets at a fixed now keeps a file
// younger than readSetMaxAge and removes one aged exactly readSetMaxAge or
// more, read-set files and leftover temp files alike; the file named keep is
// exempt whatever its age.
func TestReadSetPruneAgeBoundary(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1_800_000_000, 0)
	name := func(i int) string { return fmt.Sprintf("%064x", i+1) }
	ages := map[string]time.Duration{
		name(0):    readSetMaxAge - time.Second,     // stays
		name(1):    readSetMaxAge - 24*time.Hour,    // 29 days: stays
		name(2):    readSetMaxAge,                   // exactly the max: goes
		name(3):    readSetMaxAge + time.Second,     // goes
		".a.1.tmp": readSetMaxAge - time.Second,     // stays
		".a.2.tmp": readSetMaxAge,                   // goes
		name(4):    readSetMaxAge + 10*24*time.Hour, // keep: stays
	}
	for n, age := range ages {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte(n), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(-age)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	pruneReadSets(dir, name(4), now)
	want := map[string]bool{name(0): true, name(1): true, ".a.1.tmp": true, name(4): true}
	for n := range ages {
		_, err := os.Lstat(filepath.Join(dir, n))
		if kept := err == nil; kept != want[n] {
			t.Errorf("%s aged %v: kept=%v, want kept=%v", n, ages[n], kept, want[n])
		}
	}
}

// regression: #6692
// TestReadSetTempRefusesPlant: the temp file is opened O_EXCL|O_NOFOLLOW
// under the name suffix gives, so a regular file or a symlink planted at
// that name is refused and left as it was, and nothing is written through it.
func TestReadSetTempRefusesPlant(t *testing.T) {
	fixed := func() (string, error) { return "planted", nil }
	for _, kind := range []string{"file", "link"} {
		t.Run(kind, func(t *testing.T) {
			state, outside := t.TempDir(), t.TempDir()
			dir := filepath.Join(state, evaluation.ReadSetStateDir)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			tmp := filepath.Join(dir, "."+storeDigest+".planted.tmp")
			victim := filepath.Join(outside, "victim")
			if err := os.WriteFile(victim, []byte("untouched\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if kind == "file" {
				if err := os.WriteFile(tmp, []byte("untouched\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				victim = tmp
			} else {
				symlinkOrSkip(t, victim, tmp)
			}
			if err := writeReadSetFile(state, storeDigest, []byte("recorded\n"), fixed); err == nil {
				t.Fatalf("a %s planted at the temp name was opened, want a refusal", kind)
			}
			if b, _ := os.ReadFile(victim); string(b) != "untouched\n" {
				t.Fatalf("the planted %s's bytes changed to %q", kind, b)
			}
			if _, err := os.Lstat(readSetStatePath(state, storeDigest)); err == nil {
				t.Fatal("a read-set file was renamed into place from a planted temp name")
			}
		})
	}
}

// regression: #6692
// TestReadSetTempRemovedOnFail: when the rename into place fails, the temp
// file is removed. The suffix seam plants a directory at the destination
// after the destination check, so the rename fails.
func TestReadSetTempRemovedOnFail(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, evaluation.ReadSetStateDir)
	dst := readSetStatePath(state, storeDigest)
	racer := func() (string, error) { return "racer", os.MkdirAll(filepath.Join(dst, "sub"), 0o755) }
	if err := writeReadSetFile(state, storeDigest, []byte("recorded\n"), racer); err == nil {
		t.Fatal("the rename onto a directory succeeded, want an error")
	}
	if _, err := os.Lstat(filepath.Join(dir, "."+storeDigest+".racer.tmp")); err == nil {
		t.Fatal("the temp file was left behind after a failed rename")
	}
}

// readSetStateWriters lists the non-test functions in f that name the
// read-set state directory (evaluation.ReadSetStateDir, readSetStatePath, or
// a string literal holding the directory's name) other than the one writer,
// the path helper and the constant's own declaration, keyed
// path:function with the path relative to statusgen.
func readSetStateWriters(fset *token.FileSet, f *ast.File) []string {
	allowed := map[string]bool{
		"readset_record.go:writeReadSetFile":    true,
		"readset_record.go:readSetStatePath":    true,
		"evaluation/readset.go:<package scope>": true,
	}
	file := filepath.ToSlash(fset.Position(f.Pos()).Filename)
	var sites []string
	for _, d := range f.Decls {
		key := file + ":<package scope>"
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil {
			key = file + ":" + fn.Name.Name
		} else if ok {
			key = file + ":<method " + fn.Name.Name + ">"
		}
		ast.Inspect(d, func(n ast.Node) bool {
			hit := false
			switch x := n.(type) {
			case *ast.Ident:
				hit = x.Name == "ReadSetStateDir" || x.Name == "readSetStatePath"
			case *ast.BasicLit:
				v, err := strconv.Unquote(x.Value)
				hit = x.Kind == token.STRING && err == nil && strings.Contains(v, evaluation.ReadSetStateDir)
			}
			if hit && !allowed[key] {
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
// writes by rename. Planted second writers, by identifier, by string literal
// and in a sub-package, must each be flagged.
func TestReadSetStateOneWriter(t *testing.T) {
	plants := map[string]string{
		"readset_record.go": `package main
import ("os"; "github.com/medici-finance/assay/statusgen/evaluation")
func plantedWriter(d string) error { return os.WriteFile(d+"/"+evaluation.ReadSetStateDir+"/x", nil, 0o644) }
`,
		"readset_lit.go": `package main
import "os"
func plantedWriter(d string) error { return os.WriteFile(d+"/assay-read-sets/x", nil, 0o644) }
`,
		"verifyqueue/planted.go": `package verifyqueue
import ("os"; "path/filepath"; "github.com/medici-finance/assay/statusgen/evaluation")
func plantedWriter(d string) error { return os.WriteFile(filepath.Join(d, evaluation.ReadSetStateDir, "x"), nil, 0o644) }
`,
	}
	fset := token.NewFileSet()
	for file, src := range plants {
		pf, err := parser.ParseFile(fset, file, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := readSetStateWriters(fset, pf); len(got) != 1 || !strings.HasSuffix(got[0], file+":plantedWriter") {
			t.Fatalf("POSITIVE CONTROL (%s): guard flagged %q, want exactly plantedWriter", file, got)
		}
	}
	var bad []string
	seen := map[string]bool{}
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return err
		}
		seen[filepath.ToSlash(p)] = true
		bad = append(bad, readSetStateWriters(fset, f)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if !seen["readset_record.go"] || !seen["evaluation/readset.go"] {
		t.Fatal("readset_record.go or evaluation/readset.go not found: the guard is not looking at this tree")
	}
	if len(bad) > 0 {
		t.Fatalf("the read-set state directory is named outside writeReadSetFile:\n  %s", strings.Join(bad, "\n  "))
	}
}
