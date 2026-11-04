package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// linkDeliveryFixture builds a delivery that changes web/fixed.go and adds the
// given symlinks (path -> target text) in the same commit. When gitlink is
// set, that path is also added as a submodule entry. It returns the root, the
// delivery's first parent and the delivery.
func linkDeliveryFixture(t *testing.T, links map[string]string, gitlink string) (string, string, string) {
	t.Helper()
	root := consumersFixture(t, []string{"web/fixed.go: fixed-here"}, map[string]string{"web/fixed.go": "before\n", "web/a/b/keep.go": "kept\n"})
	mustGitInit(t, root)
	runGit(t, root, "commit", "--allow-empty", "-qm", "parent anchor")
	base := mustGitOut(t, root, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "delivered\n")
	for p, target := range links {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, full); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", "-A")
	if gitlink != "" {
		runGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+base+","+gitlink)
	}
	runGit(t, root, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "delivery with links")
	return root, base, mustGitOut(t, root, "rev-parse", "HEAD")
}

func runLinkDelivery(t *testing.T, root, base, delivery string) (int, string, string) {
	t.Helper()
	var code int
	var out string
	diag := captureStderr(t, func() {
		out = captureStdout(t, func() { code = runDeliveryConsumers(root, base, "alpha/01", delivery) })
	})
	return code, out, diag
}

// regression: #5476
func TestDeliveryLinkInTree(t *testing.T) {
	for name, links := range map[string]map[string]string{
		"sibling file":     {"web/link.go": "fixed.go"},
		"skill folder":     {".agents/skills/web": "../../web"},
		"dangling":         {"web/dangle.go": "nothere.go"},
		"via dir link":     {"ldir": "web/a/b", "web/x.go": "../ldir/keep.go"},
		"chain inside":     {"web/l1.go": "l2.go", "web/l2.go": "fixed.go"},
		"to the root dir":  {"web/a/up": "../.."},
		"dot components":   {"web/dot.go": "./a/./b/../b/keep.go"},
		"in-tree up-down":  {"web/a/b/up": "..", "web/a/b/x.go": "up/b/keep.go"},
		"trailing slashes": {"web/sl": "a/b/"},
	} {
		t.Run(name, func(t *testing.T) {
			root, base, delivery := linkDeliveryFixture(t, links, "")
			code, out, diag := runLinkDelivery(t, root, base, delivery)
			if code != 0 || !strings.Contains(out, "summary: 1 corroborated, 0 disproved, 0 unchecked") {
				t.Fatalf("in-tree links %v: exit %d\nstdout: %s\nstderr: %s", links, code, out, diag)
			}
		})
	}
}

// regression: #5476
func TestDeliveryLinkNotFollowed(t *testing.T) {
	// A second brief file that is only a link to the first. Read through the
	// link, it would be a second alpha/01 and the brief would no longer resolve
	// uniquely. The checks read the link record only, so it is not a brief.
	root, base, delivery := linkDeliveryFixture(t, map[string]string{"docs/streams/alpha/brief-01-alias.md": "brief-01-claims.md"}, "")
	code, out, diag := runLinkDelivery(t, root, base, delivery)
	if code != 0 || !strings.Contains(out, "summary: 1 corroborated, 0 disproved, 0 unchecked") {
		t.Fatalf("a link was read through: exit %d\nstdout: %s\nstderr: %s", code, out, diag)
	}
}

// regression: #5476
func TestDeliveryLinkEscapes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		links   map[string]string
		gitlink string
		want    string
	}{
		{"absolute", map[string]string{"web/abs.go": "/etc/hosts"}, "", "web/abs.go"},
		{"dotdot outside", map[string]string{"web/up.go": "../../outside.go"}, "", "web/up.go"},
		{"dotdot from root", map[string]string{"top.go": "../x"}, "", "top.go"},
		{"chain ends outside", map[string]string{"web/l1.go": "l2.go", "web/l2.go": "../../out.go"}, "", "web/l1.go"},
		{"chain via abs", map[string]string{"web/l1.go": "l2.go", "web/l2.go": "/tmp"}, "", "web/l1.go"},
		{"through dir link", map[string]string{"web/a/b/up": "..", "web/a/b/x.go": "up/../../.."}, "", "web/a/b/x.go"},
		{"loop", map[string]string{"web/la": "lb", "web/lb": "la"}, "", "web/la"},
		{"self loop", map[string]string{"web/self": "self/x"}, "", "web/self"},
		{"backslash", map[string]string{"web/bs.go": `..\..\x`}, "", "web/bs.go"},
		{"drive prefix", map[string]string{"web/dr.go": "C:/x"}, "", "web/dr.go"},
		{"submodule", nil, "web/sub", "web/sub"},
		{"submodule beside link", map[string]string{"web/link.go": "fixed.go"}, "web/sub", "web/sub"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, base, delivery := linkDeliveryFixture(t, tc.links, tc.gitlink)
			code, out, diag := runLinkDelivery(t, root, base, delivery)
			want := "statusgen: --consumers --delivery: COULD-NOT-CHECK: delivery snapshot contains a symlink or submodule: " + tc.want + "\n"
			if code != 2 || diag != want || out != "" {
				t.Fatalf("escape %v: exit %d\nstdout: %q\nstderr: %q\nwant stderr: %q", tc.links, code, out, diag, want)
			}
		})
	}
}

// regression: #5476
func TestDeliveryLinkHopBound(t *testing.T) {
	chain := func(n int) map[string]string {
		links := map[string]string{}
		for i := 1; i < n; i++ {
			links[fmt.Sprintf("l%02d", i)] = fmt.Sprintf("l%02d", i+1)
		}
		links[fmt.Sprintf("l%02d", n)] = "web/fixed.go"
		return links
	}
	if p := escapingDeliveryLink(chain(maxDeliveryLinkHops)); p != "" {
		t.Fatalf("a %d-link chain inside the root was refused at %s", maxDeliveryLinkHops, p)
	}
	if p := escapingDeliveryLink(chain(maxDeliveryLinkHops + 1)); p != "l01" {
		t.Fatalf("a %d-link chain was accepted (got %q)", maxDeliveryLinkHops+1, p)
	}
}
