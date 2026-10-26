package verdict

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Crypto stdlib dependencies include platform support; this package exposes
// only supplied-byte/public-key operations and must never acquire authority.
func TestNoAuthorityAdapters(t *testing.T) {
	entries, e := os.ReadDir(".")
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, e := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if e != nil {
			t.Fatal(e)
		}
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if p == "os" || p == "os/exec" || p == "net" || strings.HasPrefix(p, "net/") || strings.Contains(p, "/tools/desk") || strings.Contains(p, "/custody") || strings.Contains(p, "/adapters") {
				t.Fatalf("%s imports authority/effect adapter %s", name, p)
			}
		}
	}
}
