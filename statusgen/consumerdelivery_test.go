package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func deliveryFixture(t *testing.T, routes []string) (string, string, string) {
	t.Helper()
	root := consumersFixture(t, routes, map[string]string{"web/fixed.go": "before\n", "web/untouched.go": "before\n", "web/excluded.go": "before\n"})
	mustGitInit(t, root)
	runGit(t, root, "commit", "--allow-empty", "-qm", "parent anchor")
	base := mustGitOut(t, root, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "delivered\n")
	mustGitCommitAll(t, root, "implementation without brief edit")
	return root, base, mustGitOut(t, root, "rev-parse", "HEAD")
}

// regression: #1915
func TestDeliveryInherited(t *testing.T) {
	root, base, delivery := deliveryFixture(t, []string{"web/fixed.go: fixed-here", "web/excluded.go: out-of-scope (unchanged reader contract)"})
	if got := runDeliveryConsumers(root, base, "alpha/01", delivery); got != 0 {
		t.Fatalf("inherited delivery claims: got %d, want 0", got)
	}
	// Later committed and dirty claims/files cannot substitute for delivery inputs.
	p := filepath.Join(root, "docs/streams/alpha/brief-01-claims.md")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	mustGitCommitAll(t, root, "later removes brief")
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "dirty\n")
	if got := runDeliveryConsumers(root, base, "alpha/01", delivery); got != 0 {
		t.Fatalf("latest tree masked delivery: got %d", got)
	}
}

// regression: #1915
func TestDeliveryPlantedRoutes(t *testing.T) {
	for _, route := range []string{"web/untouched.go: fixed-here", "web/fixed.go: out-of-scope (must remain unchanged)"} {
		t.Run(route, func(t *testing.T) {
			root, base, delivery := deliveryFixture(t, []string{"web/fixed.go: fixed-here", route})
			if got := runDeliveryConsumers(root, base, "alpha/01", delivery); got != 1 {
				t.Fatalf("planted route %q: got %d, want DISPROVED", route, got)
			}
		})
	}
}

// regression: #1915
func TestDeliveryBounds(t *testing.T) {
	root, base, delivery := deliveryFixture(t, []string{"web/fixed.go: fixed-here"})
	for _, v := range [][3]string{{base, "", delivery}, {base, "alpha/99", delivery}, {delivery, "alpha/01", delivery}, {base, "alpha/01", "HEAD"}, {"main", "alpha/01", delivery}, {mustGitOut(t, root, "rev-parse", base+"^"), "alpha/01", delivery}, {base, "alpha/01", strings.Repeat("a", 40)}} {
		if got := runDeliveryConsumers(root, v[0], v[1], v[2]); got != 2 {
			t.Fatalf("invalid bounds %v: got %d", v, got)
		}
	}
	// Missing parent objects and shallow history never count as a clean run.
	gitdir := mustGitOut(t, root, "rev-parse", "--git-dir")
	mustWrite(t, filepath.Join(root, gitdir, "shallow"), base+"\n")
	if got := runDeliveryConsumers(root, base, "alpha/01", delivery); got != 2 {
		t.Fatalf("shallow got %d", got)
	}
	if err := os.Remove(filepath.Join(root, gitdir, "shallow")); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"link", "export-ignore"} {
		r, b, _ := deliveryFixture(t, []string{"web/fixed.go: fixed-here"})
		if kind == "link" {
			if err := os.Symlink("fixed.go", filepath.Join(r, "web/link.go")); err != nil {
				t.Fatal(err)
			}
		} else {
			mustWrite(t, filepath.Join(r, ".gitattributes"), "web/fixed.go export-ignore\n")
		}
		mustGitCommitAll(t, r, "snapshot interpretation control")
		d := mustGitOut(t, r, "rev-parse", "HEAD")
		b = mustGitOut(t, r, "rev-parse", "HEAD^1")
		if got := runDeliveryConsumers(r, b, "alpha/01", d); got != 2 {
			t.Fatalf("%s snapshot got %d", kind, got)
		}
	}

	runGit(t, root, "checkout", "--detach", base)
	if got := runDeliveryConsumers(root, base, "alpha/01", delivery); got != 2 {
		t.Fatalf("nonancestor got %d", got)
	}
}

// regression: #1915
func TestDeliveryNoEvidence(t *testing.T) {
	root, base, delivery := deliveryFixture(t, []string{"remote reader: out-of-scope (not visible in this repository)"})
	if got := runDeliveryConsumers(root, base, "alpha/01", delivery); got != 2 {
		t.Fatalf("unchecked only got %d", got)
	}
}

// regression: #1915
func TestDeliveryCLI(t *testing.T) {
	root, base, delivery := deliveryFixture(t, []string{"web/fixed.go: fixed-here"})
	bin := buildStatusgenBinary(t)
	out, err := exec.Command(bin, "--root", root, "--consumers", "--brief", "alpha/01", "--delivery", delivery, "--base", base).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "summary: 1 corroborated, 0 disproved, 0 unchecked") {
		t.Fatalf("delivery CLI: %v: %s", err, out)
	}
	out, err = exec.Command(bin, "--root", root, "--delivery", delivery).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "--delivery requires --consumers") {
		t.Fatalf("delivery without mode: %v: %s", err, out)
	}
}
