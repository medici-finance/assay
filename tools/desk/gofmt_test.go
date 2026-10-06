package desk_test

// This is the GOFMT GUARD. The tools/desk module had no formatting gate, so unformatted
// files landed one PR at a time and a `gofmt -l` Verify row over a whole directory went red
// on files the brief under verification never touched. The drift moved between files as some
// were fixed and others arrived, which is the sign of a missing gate rather than one bad file.
//
// The guard formats every .go file in the module with go/format (the printer gofmt uses) and
// fails naming each file whose bytes change. testdata/ and vendor/ trees are skipped: a
// fixture may be deliberately malformed. A positive control proves the comparison still sees
// an unformatted file, so a broken check fails here rather than reporting clean.
//
// Fix a failure with `gofmt -w <file>`.

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// gofmtDrift reports whether src is not already in gofmt's canonical form. A parse error is
// returned as-is: an unparseable non-testdata .go file is a failure of its own.
func gofmtDrift(src []byte) (bool, error) {
	out, err := format.Source(src)
	if err != nil {
		return false, err
	}
	return !bytes.Equal(out, src), nil
}

func TestGofmtClean(t *testing.T) {
	// Positive control: a misaligned struct and a mis-indented body must read as drift, and
	// their canonical form must not.
	planted := []byte("package p\n\ntype T struct {\n\tA int\n\tLonger string\n}\n\nfunc f() {\n  return\n}\n")
	if drift, err := gofmtDrift(planted); err != nil || !drift {
		t.Fatalf("gofmt guard: the planted unformatted source was not seen as drift (drift=%v err=%v) — the scan below would report clean on anything", drift, err)
	}
	canonical, err := format.Source(planted)
	if err != nil {
		t.Fatalf("gofmt guard: positive control does not format: %v", err)
	}
	if drift, err := gofmtDrift(canonical); err != nil || drift {
		t.Fatalf("gofmt guard: canonical source read as drift (drift=%v err=%v) — the scan below would flag every file", drift, err)
	}

	var problems []string
	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" || (path != "." && strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		drift, err := gofmtDrift(src)
		switch {
		case err != nil:
			problems = append(problems, filepath.ToSlash(path)+": does not parse: "+err.Error())
		case drift:
			problems = append(problems, filepath.ToSlash(path)+": not gofmt-formatted — run `gofmt -w` on it")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("gofmt guard: could not walk the module: %v", err)
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error("gofmt guard: " + p)
	}
}
