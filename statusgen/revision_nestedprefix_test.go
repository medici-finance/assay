package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #2395
func TestRevisionOrdinaryNestedPrefix(t *testing.T) {
	root, _ := revisionFixture(t)
	nested := filepath.Join(root, "sub")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	// Native oracle is collected before the launcher is poisoned.
	want, err := exec.Command("git", "-C", nested, "rev-parse", "--show-prefix").Output()
	if err != nil || !bytes.Equal(want, []byte("sub/\n")) {
		t.Fatalf("native nested prefix: %q %v", want, err)
	}
	marker := revisionNativeGuard(t)
	got, err := gitPrefixOutput(nested)
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("ordinary nested prefix=(%q,%v), native=%q", got, err, want)
	}
	why := verifyrunRootBelowToplevel(nested)
	if !strings.HasPrefix(why, "refusing --root "+nested+":") || !strings.Contains(why, "it is the subdirectory sub, below its repository's toplevel") {
		t.Errorf("nested root guard did not retain refusal: %q", why)
	}
	revisionNoStarts(t, marker)
}
