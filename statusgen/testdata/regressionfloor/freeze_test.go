package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// regression: graph-execution/29
func TestFreezeObjects(t *testing.T) {
	good := strings.Repeat("a", 40)
	if object, err := pinnedObject(good, "file_test.go"); err != nil || object != good+":file_test.go" {
		t.Fatalf("valid pinned object refused: %s %v", object, err)
	}
	for _, source := range []string{"-p", "HEAD", "", strings.Repeat("a", 39)} {
		if _, err := pinnedObject(source, "file_test.go"); err == nil {
			t.Fatalf("invalid pinned source accepted: %q", source)
		}
	}
	for _, path := range []string{"", "/file_test.go", "../file_test.go"} {
		if _, err := pinnedObject(good, path); err == nil {
			t.Fatalf("invalid pinned path accepted: %q", path)
		}
	}
}

// regression: graph-execution/29
func TestFreezeFunctions(t *testing.T) {
	parse := func(source string) string {
		t.Helper()
		f, err := parser.ParseFile(token.NewFileSet(), "fixture.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		digest, err := pinnedBody(f, "TestChosen")
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}
	// The method precedes the function, so a name-only first match picks it.
	function := "package fixture\nfunc TestChosen(t *T) { println(1) }\n"
	both := "package fixture\nfunc (R) TestChosen(t *T) { println(2) }\nfunc TestChosen(t *T) { println(1) }\n"
	if parse(function) != parse(both) {
		t.Fatal("method selected instead of retained test function")
	}
	f, err := parser.ParseFile(token.NewFileSet(), "method.go", "package fixture\nfunc (R) TestChosen(t *T) {}", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pinnedBody(f, "TestChosen"); err == nil {
		t.Fatal("method-only declaration accepted")
	}
}
