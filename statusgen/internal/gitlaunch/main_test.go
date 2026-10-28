package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #2395
func TestLaunchCounter(t *testing.T) {
	for name, src := range map[string]string{
		"git-out-seam":        `package p;func planted(){gitOut(root,"status")}`,
		"typed-parent-seam":   `package p;func planted(){provenanceParents(root,"HEAD")}`,
		"typed-prefix-seam":   `package p;func planted(){provenanceRootPrefix(root)}`,
		"mixed-known-unknown": `package p;import "os/exec";func f(arg string){bin:="echo";bin=arg;exec.Command(bin,"status")}`,
		"shell-wrapper":       `package p;import "os/exec";func f(){exec.Command("sh","-c","git status")}`,
		"dynamic":             `package p;import "os/exec";func planted(bin string){exec.Command(bin,"status")}`,
		"parenthesized":       `package p;import "os/exec";func planted(){(exec.Command)("git","status")}`,
		"dot-import":          `package p;import . "os/exec";func planted(){Command("git","status")}`,
		"absolute":            `package p;import "os/exec";func planted(){exec.Command("/usr/bin/git","status")}`,
		"reassigned":          `package p;import "os/exec";func planted(){bin:="echo";bin="git";exec.Command(bin,"status")}`,
		"direct":              `package p;import "os/exec";func planted(){exec.Command("git","status")}`,
		"context":             `package p;import x "os/exec";func planted(){x.CommandContext(ctx,"git","status")}`,
		"literal-alias":       `package p;import "os/exec";const bin="git";func planted(){exec.Command(bin,"status")}`,
		"call-alias":          `package p;import "os/exec";func planted(){run:=exec.Command;run("git","status")}`,
		"seam":                `package p;func planted(){historyGit(root,"status")}`,
		"seam-alias":          `package p;func planted(){run:=historyGit;run(root,"status")}`,
		"typed-compatibility": `package p;func planted(){gitShowObject(root,"HEAD","file")}`,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := scanFile("planted.go", []byte(src))
			if err != nil || len(got) != 1 {
				t.Fatalf("planted native operation not counted: %v %v", got, err)
			}
			if err := verify(got, inventory{Reasons: map[string]reason{}}); err == nil || !strings.Contains(err.Error(), "unrecorded") {
				t.Fatalf("plant not rejected: %v", err)
			}
		})
	}
	clean := `package p;const example="exec.Command(\"git\")";func notGit(){ println("historyGit") }`
	if got, err := scanFile("clean.go", []byte(clean)); err != nil || len(got) != 0 {
		t.Fatalf("non-call counted: %v %v", got, err)
	}
}

// TestPrintGitSites is a source-inventory diagnostic; it changes no file.
func TestPrintGitSites(t *testing.T) {
	root := os.Getenv("GITLAUNCH_DIAGNOSTIC_SOURCE")
	if root == "" {
		root = filepath.Join("..", "..")
	}
	sites, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(sites)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("GIT-SITES %s\n", b)
}

// regression: #2395
func TestGitCeilingOnlyDown(t *testing.T) {
	if err := ceilingDown(inventory{Dynamic: 1}, inventory{Dynamic: 2}); err == nil {
		t.Fatal("dynamic executable ceiling increase accepted")
	}
	if err := ceilingDown(inventory{Direct: 1, Seams: 1}, inventory{Direct: 2, Seams: 1}); err == nil {
		t.Fatal("direct ceiling increase accepted")
	}
	if err := ceilingDown(inventory{Direct: 1, Seams: 1}, inventory{Direct: 1, Seams: 2}); err == nil {
		t.Fatal("seam ceiling increase accepted")
	}
	if err := ceilingDown(inventory{Direct: 2, Seams: 2}, inventory{Direct: 1, Seams: 1}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "git-launches.json"))
	if err != nil {
		t.Fatal(err)
	}
	var current inventory
	if err := json.Unmarshal(raw, &current); err != nil {
		t.Fatal(err)
	}
	if err := checkPrior(root, current); err != nil {
		t.Fatal(err)
	}
}

// regression: #2395
func TestInventoryPlants(t *testing.T) {
	root := t.TempDir()
	write := func(name, source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("baseline.go", "package p\n")
	got, err := scan(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("clean source inventory: %v %v", got, err)
	}
	if err := verify(got, inventory{}); err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"direct":        `package p;import "os/exec";func extra(){exec.Command("git","status")}`,
		"existing-seam": `package p;func extra(){historyGit(root,"status")}`,
		"dynamic":       `package p;import "os/exec";func extra(bin string){exec.Command(bin,"status")}`,
	} {
		t.Run(name, func(t *testing.T) {
			write("added.go", source)
			planted, err := scan(root)
			if err != nil || len(planted) != 1 {
				t.Fatalf("new non-test file was not inventoried: %v %v", planted, err)
			}
			if err := verify(planted, inventory{}); err == nil || !strings.Contains(err.Error(), "unrecorded") {
				t.Fatalf("new operation not rejected: %v", err)
			}
			if err := os.Remove(filepath.Join(root, "added.go")); err != nil {
				t.Fatal(err)
			}
		})
	}
	write("example_test.go", `package p;func testOnly(){historyGit(root,"status")}`)
	after, err := scan(root)
	if err != nil || len(after) != 0 {
		t.Fatalf("test-only source counted: %v %v", after, err)
	}
}

// regression: #2395
func TestCrossFileLaunch(t *testing.T) {
	root := t.TempDir()
	for name, src := range map[string]string{"aliases.go": `package p;import "os/exec";var run=exec.Command`, "caller.go": `package p;func planted(){run("git","status")}`} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
	}
	sites, err := scan(root)
	if err != nil || len(sites) != 1 {
		t.Fatalf("cross-file alias not inventoried: %v %v", sites, err)
	}
	if err := verify(sites, inventory{}); err == nil {
		t.Fatal("cross-file Git construction not rejected")
	}
}

// regression: #2395
func TestLocalNativeSeam(t *testing.T) {
	source := `package p;func f(){git:=func(args ...string){historyGit(root,args...)};git("status");run:=git;run("log")}`
	got, err := scanFile("local.go", []byte(source))
	if err != nil || len(got) != 3 {
		t.Fatalf("local native seam callers not inventoried: %v %v", got, err)
	}
	if err := verify(got, inventory{}); err == nil {
		t.Fatal("local seam calls not rejected")
	}
}
