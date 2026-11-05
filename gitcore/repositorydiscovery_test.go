package gitcore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

func TestToplevelHeadPrefixBoundary(t *testing.T) {
	f := gittest.NewFixture(t)
	nested := &gittest.Fixture{Dir: filepath.Join(f.Dir, "sub")}
	if err := os.Mkdir(nested.Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := nested.Git("init", "-q", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	for pad := 240; pad <= 260; pad++ {
		t.Run(fmt.Sprint(pad), func(t *testing.T) {
			body := "ref:" + strings.Repeat(" ", pad) + "refs/heads/main\n"
			if err := os.WriteFile(filepath.Join(nested.Dir, ".git", "HEAD"), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			want, err := nested.Git("rev-parse", "--show-toplevel")
			if err != nil {
				t.Fatal(err)
			}
			if got, err := Toplevel(nested.Dir); err != nil || got != want {
				t.Fatalf("HEAD prefix padding %d: discovery=%q %v native=%q", pad, got, err, want)
			}
		})
	}
}

func TestToplevelIgnoresStrayDotGitDirectory(t *testing.T) {
	for _, shape := range []string{"empty", "head-only", "objects-and-refs-without-head", "empty-head", "garbage-head"} {
		t.Run(shape, func(t *testing.T) {
			f := gittest.NewFixture(t)
			nested := filepath.Join(f.Dir, "sub")
			entry := filepath.Join(nested, ".git")
			if err := os.MkdirAll(entry, 0755); err != nil {
				t.Fatal(err)
			}
			if shape == "head-only" {
				if err := os.WriteFile(filepath.Join(entry, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if shape == "objects-and-refs-without-head" || shape == "empty-head" || shape == "garbage-head" {
				for _, part := range []string{"objects", "refs"} {
					if err := os.Mkdir(filepath.Join(entry, part), 0755); err != nil {
						t.Fatal(err)
					}
				}
			}
			if shape == "empty-head" || shape == "garbage-head" {
				body := ""
				if shape == "garbage-head" {
					body = "not a reference\n"
				}
				if err := os.WriteFile(filepath.Join(entry, "HEAD"), []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := f.Git("-C", nested, "rev-parse", "--show-toplevel")
			if err != nil {
				t.Fatalf("native parent discovery fixture: %v", err)
			}
			got, err := Toplevel(nested)
			if err != nil || got != want {
				t.Fatalf("Toplevel=%q %v native=%q", got, err, want)
			}
			wantCommon, err := f.Git("-C", nested, "rev-parse", "--path-format=absolute", "--git-common-dir")
			if err != nil {
				t.Fatal(err)
			}
			if common, err := CommonDir(nested); err != nil || common != wantCommon {
				t.Fatalf("CommonDir=%q %v native=%q", common, err, wantCommon)
			}
		})
	}
}

func TestToplevelKeepsNearestValidRepository(t *testing.T) {
	f := gittest.NewFixture(t)
	nested := &gittest.Fixture{Dir: filepath.Join(f.Dir, "sub")}
	if err := os.MkdirAll(nested.Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := nested.Git("init", "-q", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	want, err := nested.Git("rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Toplevel(nested.Dir); err != nil || got != want {
		t.Fatalf("valid nested repository skipped: %q %v native=%q", got, err, want)
	}
}

func TestToplevelRejectsInvalidGitdirFile(t *testing.T) {
	f := gittest.NewFixture(t)
	nested := filepath.Join(f.Dir, "sub")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, ".git"), []byte("not a gitdir pointer\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Git("-C", nested, "rev-parse", "--show-toplevel"); err == nil {
		t.Fatal("native invalid-pointer control unexpectedly succeeded")
	}
	if got, err := Toplevel(nested); err == nil {
		t.Fatalf("invalid pointer silently fell through to parent: %q", got)
	}
}
