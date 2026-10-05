package cellscratch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScratchInputParents(t *testing.T) {
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "actual"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "actual", "data"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("actual", filepath.Join(source, "alias")); err != nil {
		t.Fatal(err)
	}
	s := scratchStore(t)
	r := scratchRun(t, s)
	if err := r.Inputs(source, []string{"alias/data"}, 4096); err == nil {
		t.Fatal("nonliteral source parent admitted")
	}
	if err := r.Inputs(source, []string{"actual/data"}, 4096); err != nil {
		t.Fatal("literal required input rejected", err)
	}
}

func TestScratchInputLinks(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "data"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(source, "data"), filepath.Join(source, "copy")); err != nil {
		t.Fatal(err)
	}
	r := scratchRun(t, scratchStore(t))
	if err := r.Inputs(source, []string{"copy"}, 4096); err == nil {
		t.Fatal("multiply-linked input admitted")
	}
}
