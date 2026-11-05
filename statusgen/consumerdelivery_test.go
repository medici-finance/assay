package main

import (
	"bytes"
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
			// An in-tree link is recorded, not refused (#5476); this control
			// keeps the refusal for a link whose target leaves the root.
			if err := os.Symlink("../../outside.go", filepath.Join(r, "web/link.go")); err != nil {
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

// regression: #5375
func TestDeliveryIdentitySwapped(t *testing.T) {
	// Each producer's follow-up routes corroborate at either delivery. That
	// topology must not be presented as proof of which producer was delivered.
	root := consumersFixture(t, []string{"web/fixed.go: follow-up alpha/01", "web/excluded.go: out-of-scope (unchanged reader contract)"}, map[string]string{"web/fixed.go": "before\n", "web/untouched.go": "before\n", "web/excluded.go": "before\n"})
	p := filepath.Join(root, "docs/streams/alpha/brief-02-target.md")
	text := readFileT(t, p)
	text = strings.Replace(text, "sources: [\"fixture\"]\n", "sources: [\"fixture\"]\nconsumers:\n  - \"web/untouched.go: follow-up alpha/02\"\n  - \"web/excluded.go: out-of-scope (unchanged reader contract)\"\n", 1)
	mustWrite(t, p, text)
	mustGitInit(t, root)
	base := mustGitOut(t, root, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "producer one\n")
	mustGitCommitAll(t, root, "first producer delivery")
	one := mustGitOut(t, root, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(root, "web/untouched.go"), "producer two\n")
	mustGitCommitAll(t, root, "second producer delivery")
	two := mustGitOut(t, root, "rev-parse", "HEAD")
	for _, tc := range []struct{ name, brief, base, delivery string }{
		{"first-correct", "alpha/01", base, one},
		{"second-correct", "alpha/02", one, two},
		{"first-swapped", "alpha/01", one, two},
		{"second-swapped", "alpha/02", base, one},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var code int
			var out string
			diagnostic := captureStderr(t, func() {
				out = captureStdout(t, func() { code = runDeliveryConsumers(root, tc.base, tc.brief, tc.delivery) })
			})
			if code != 0 || !strings.Contains(out, "summary: 1 corroborated, 0 disproved, 1 unchecked") {
				t.Fatalf("routing semantics changed: exit %d\n%s", code, out)
			}
			if strings.Contains(out, "PRODUCER-DELIVERY:") || !strings.Contains(diagnostic, "PRODUCER-DELIVERY: COULD-NOT-CHECK") || !strings.Contains(diagnostic, "follow-up routes prove target existence and reference-back only") {
				t.Fatalf("producer identity needs stderr qualification without changing stdout:\nstdout: %s\nstderr: %s", out, diagnostic)
			}
		})
	}
}

// regression: #5375
func TestDeliveryIdentityDerived(t *testing.T) {
	root, _, _ := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, goodMarker)
	var code int
	var out string
	diagnostic := captureStderr(t, func() {
		out = captureStdout(t, func() { code = runConsumersDerived(root, "alpha/01") })
	})
	if code != 0 || !strings.Contains(out, "summary: 1 corroborated, 0 disproved, 0 unchecked") || strings.Contains(out, "PRODUCER-DELIVERY:") || !strings.Contains(diagnostic, "PRODUCER-DELIVERY: COULD-NOT-CHECK") {
		t.Fatalf("derived delivery needs the same stderr identity limit: exit %d\nstdout: %s\nstderr: %s", code, out, diagnostic)
	}
}

// regression: #5375 F1
func TestDeliveryStdoutStable(t *testing.T) {
	root, base, delivery := deliveryFixture(t, []string{"web/fixed.go: fixed-here"})
	var code int
	var out string
	_ = captureStderr(t, func() {
		out = captureStdout(t, func() { code = runDeliveryConsumers(root, base, "alpha/01", delivery) })
	})
	want := "consumers corroboration — base " + base + ".." + delivery + " (immutable delivered snapshot), 1 brief(s)\n" +
		"\nalpha/01\n  CORROBORATED  web/fixed.go: fixed-here\n" +
		"\nsummary: 1 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing\n" +
		"LIMIT: only the entries written above were judged — an OMITTED consumer is invisible to this gate.\n"
	if code != 0 || out != want {
		t.Fatalf("historical routing stdout changed: exit %d\ngot: %q\nwant: %q", code, out, want)
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
	// Extracting the shared emitter must preserve the ordinary branch output.
	briefPath := filepath.Join(root, "docs/streams/alpha/brief-01-claims.md")
	briefText, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, briefPath, strings.ReplaceAll(string(briefText), "web/fixed.go", "web/untouched.go"))
	branch := exec.Command(bin, "--root", root, "--consumers", "--brief", "alpha/01", "--base", base)
	var stdout, stderr bytes.Buffer
	branch.Stdout, branch.Stderr = &stdout, &stderr
	err = branch.Run()
	var failureLine string
	for _, line := range strings.Split(stderr.String(), "\n") {
		if strings.HasPrefix(line, "statusgen: --consumers:") {
			failureLine = line
		}
	}
	if err == nil || failureLine != "statusgen: --consumers: 1 routing claim(s) DISPROVED by the diff against "+base {
		t.Fatalf("branch stderr compatibility: %v: %q", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "base "+base+" (committed diff + uncommitted working tree), 1 brief(s)") {
		t.Fatalf("branch header compatibility: %s", stdout.String())
	}
	out, err = exec.Command(bin, "--root", root, "--delivery", delivery).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "--delivery requires --consumers") {
		t.Fatalf("delivery without mode: %v: %s", err, out)
	}
}
