package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The merged-main shape of the consumer check: the delivery is an ancestor of
// HEAD, the verifier's Evidence commit records it in a Verification-Table marker,
// and the checkout's origin/main already equals HEAD, so the branch diff is empty.

const derivedBrief = "docs/streams/alpha/brief-01-claims.md"

func derivedMarker(delivery, base, original string) string {
	return "Verification-Table: delivery=" + delivery + " base=" + base + " original=" + original + "\n"
}

// derivedFixture returns a merged-main checkout whose committed Evidence carries
// marker(base, delivery), plus the base (delivery's first parent) and delivery.
func derivedFixture(t *testing.T, routes []string, marker func(base, delivery string) string) (root, base, delivery string) {
	t.Helper()
	root, base, delivery = deliveryFixture(t, routes)
	text := "\n## Evidence\n\n"
	if marker != nil {
		text += marker(base, delivery)
	}
	appendBrief(t, root, text)
	mustGitCommitAll(t, root, "verifier Evidence commit")
	runGit(t, root, "update-ref", remoteMainRef, "HEAD")
	return root, base, delivery
}

func appendBrief(t *testing.T, root, text string) {
	t.Helper()
	p := filepath.Join(root, derivedBrief)
	mustWrite(t, p, readFileT(t, p)+text)
}

func readFileT(t *testing.T, p string) string {
	t.Helper()
	b, err := exec.Command("cat", p).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func goodMarker(base, delivery string) string { return derivedMarker(delivery, base, base) }

func runDerivedCLI(t *testing.T, bin, root string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"--root", root, "--consumers"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	return code, stdout.String(), stderr.String()
}

// regression: #1281
func TestDerivedDeliveryPassesOnMergedMain(t *testing.T) {
	root, base, delivery := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, goodMarker)
	if got := runConsumersDerived(root, "alpha/01"); got != 0 {
		t.Fatalf("merged brief with a recorded delivery: got %d, want 0", got)
	}
	bin := buildStatusgenBinary(t)
	code, stdout, stderr := runDerivedCLI(t, bin, root, "--brief", "alpha/01")
	if code != 0 || !strings.Contains(stdout, "summary: 1 corroborated, 0 disproved, 0 unchecked") {
		t.Fatalf("bare --consumers --brief on merged main: exit %d\n%s\n%s", code, stdout, stderr)
	}
	// The run is pinned: its label names the delivery range it read.
	if !strings.Contains(stdout, base+".."+delivery+" (immutable delivered snapshot)") {
		t.Fatalf("the derived run must name its pinned range:\n%s", stdout)
	}
}

// regression: #1281
func TestDerivedDeliveryPlantedSecondInstance(t *testing.T) {
	// The same defect class, a second instance: a claim the delivered diff does
	// not make true must still be DISPROVED on merged main, never waved through.
	for _, route := range []string{"web/untouched.go: fixed-here", "web/fixed.go: out-of-scope (must remain unchanged)"} {
		t.Run(route, func(t *testing.T) {
			root, _, _ := derivedFixture(t, []string{"web/fixed.go: fixed-here", route}, goodMarker)
			if got := runConsumersDerived(root, "alpha/01"); got != 1 {
				t.Fatalf("planted route %q on merged main: got %d, want DISPROVED", route, got)
			}
		})
	}
}

// regression: #1281
func TestDerivedDeliveryPinsRecordedDeliveryParent(t *testing.T) {
	// Two deliveries land. The marker names the SECOND, which did not touch
	// web/fixed.go; its own first parent is the pin, not the older baseline.
	root, base, _ := deliveryFixture(t, []string{"web/fixed.go: fixed-here"})
	mustWrite(t, filepath.Join(root, "web/untouched.go"), "later delivery\n")
	second := mustGitCommitAll(t, root, "second delivery")
	second = mustGitOut(t, root, "rev-parse", second)
	appendBrief(t, root, "\n## Evidence\n\n"+derivedMarker(second, mustGitOut(t, root, "rev-parse", second+"^1"), base))
	mustGitCommitAll(t, root, "verifier Evidence commit")
	runGit(t, root, "update-ref", remoteMainRef, "HEAD")
	if got := runConsumersDerived(root, "alpha/01"); got != 1 {
		t.Fatalf("pin must be the recorded delivery's first parent: got %d, want DISPROVED", got)
	}
}

// regression: #1281
func TestDerivedDeliveryDiffCannotBeTaken(t *testing.T) {
	// The first miss site: the branch diff cannot be taken at all. The recorded,
	// landed delivery still pins the run.
	root, _, _ := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, goodMarker)
	orig := changedPathsSince
	changedPathsSince = func(root, base string) ([]string, error) { return nil, fmt.Errorf("simulated diff failure") }
	defer func() { changedPathsSince = orig }()
	if got := runConsumersDerived(root, "alpha/01"); got != 0 {
		t.Fatalf("untakeable diff with a recorded delivery: got %d, want 0", got)
	}
}

// regression: #1281
func TestDerivedDeliveryRequiresLandedDelivery(t *testing.T) {
	// A marker committed only in this checkout is not a landed delivery. With no
	// fetched mainline ref there is nothing to land against; with a ref that
	// does not yet contain the named delivery the commit is still local.
	cases := []struct {
		name  string
		cli   bool // a ref behind the delivery leaves the brief in the branch diff, which is not the derivation
		after func(t *testing.T, root, base, delivery string)
	}{
		{"no mainline ref", true, func(t *testing.T, root, base, delivery string) {
			runGit(t, root, "update-ref", "-d", remoteMainRef)
		}},
		{"mainline ref behind the delivery", false, func(t *testing.T, root, base, delivery string) {
			runGit(t, root, "update-ref", remoteMainRef, base)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, base, delivery := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, goodMarker)
			c.after(t, root, base, delivery)
			if d, _, why := deriveConsumerDelivery(root, derivedBrief); d != "" || !strings.Contains(why, "not reachable from the fetched mainline ref") {
				t.Fatalf("a never-landed delivery must be refused: delivery %q why %q", d, why)
			}
			if !c.cli {
				return
			}
			code, stdout, stderr := runDerivedCLI(t, buildStatusgenBinary(t), root, "--brief", "alpha/01")
			if code != 2 || !strings.Contains(stderr, "COULD-NOT-CHECK") || strings.Contains(stdout, "summary:") {
				t.Fatalf("exit %d, want 2 COULD-NOT-CHECK with no verdict\n%s\n%s", code, stdout, stderr)
			}
		})
	}
	// Positive control: the same checkout with the delivery landed derives.
	root, base, delivery := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, goodMarker)
	if d, b, why := deriveConsumerDelivery(root, derivedBrief); why != "" || d != delivery || b != base {
		t.Fatalf("positive control: %q %q %q", d, b, why)
	}
}

// regression: #1281
func TestDerivedDeliveryRefusalBranches(t *testing.T) {
	// One case per refusal branch of deriveConsumerDelivery, each reached with
	// everything else valid, so a weakened branch changes the answer.
	root, _, _ := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, goodMarker)
	for _, rel := range []string{"", "../outside.md", "/etc/passwd"} {
		if d, _, why := deriveConsumerDelivery(root, rel); d != "" || !strings.Contains(why, "not inside the repository root") {
			t.Errorf("path guard: %q gave delivery %q why %q", rel, d, why)
		}
	}
	good := func(base, delivery string) string { return strings.TrimSpace(goodMarker(base, delivery)) }
	cases := []struct {
		name   string
		marker func(base, delivery string) string
		want   string
	}{
		// The code span leaves the line still starting with the marker prefix, so
		// only the literal-text check can refuse it.
		{"trailing code span", func(b, d string) string { return good(b, d) + " `note`\n" }, "literal live text"},
		{"five fields", func(b, d string) string { return good(b, d) + " extra=1\n" }, "expected Verification-Table"},
		{"three fields", func(b, d string) string { return "Verification-Table: delivery=" + d + " base=" + b + "\n" }, "expected Verification-Table"},
		{"fields out of order", func(b, d string) string {
			return "Verification-Table: base=" + b + " delivery=" + d + " original=" + b + "\n"
		}, "expected Verification-Table"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, _, _ := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, c.marker)
			if d, _, why := deriveConsumerDelivery(r, derivedBrief); d != "" || !strings.Contains(why, c.want) {
				t.Fatalf("delivery %q why %q, want a refusal naming %q", d, why, c.want)
			}
		})
	}
}

// regression: #1281
func TestDerivedDeliveryRefusesAmbiguousBrief(t *testing.T) {
	// Two brief files claiming the same brief id: the derivation must not pick
	// the first one.
	root, _, _ := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, goodMarker)
	dir := filepath.Join(root, "docs", "streams", "alpha")
	dup := strings.Replace(readFileT(t, filepath.Join(dir, "brief-02-target.md")), "brief: alpha/02", "brief: alpha/01", 1)
	if !strings.Contains(dup, "brief: alpha/01") {
		t.Fatal("fixture: the duplicate brief id was not planted")
	}
	mustWrite(t, filepath.Join(dir, "brief-02-target.md"), dup)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if code, why := runDerivedConsumers(root, streams, "alpha/01"); code != 2 || !strings.Contains(why, "exactly one file") {
		t.Fatalf("ambiguous brief: code %d why %q, want 2 naming exactly one file", code, why)
	}
}

// regression: #1281
func TestDerivedDeliveryCouldNotCheck(t *testing.T) {
	bin := buildStatusgenBinary(t)
	const zeros = "0000000000000000000000000000000000000000"
	cases := []struct {
		name   string
		marker func(base, delivery string) string
		after  func(t *testing.T, root, base, delivery string)
		brief  string
		want   string
	}{
		{name: "no marker in Evidence", want: "no Verification-Table marker"},
		{name: "no base ref and no marker", after: func(t *testing.T, root, base, delivery string) {
			runGit(t, root, "update-ref", "-d", remoteMainRef)
		}, want: "no Verification-Table marker"},
		{name: "code-span marker", marker: func(base, delivery string) string {
			return "`" + strings.TrimSpace(goodMarker(base, delivery)) + "`\n"
		}, want: "no Verification-Table marker"},
		{name: "malformed marker", marker: func(base, delivery string) string {
			return "Verification-Table: delivery=" + delivery + "\n"
		}, want: "expected Verification-Table"},
		{name: "short ids", marker: func(base, delivery string) string {
			return derivedMarker(delivery[:12], base, base)
		}, want: "full 40-hex"},
		{name: "unknown delivery", marker: func(base, delivery string) string {
			return derivedMarker(strings.Repeat("a", 40), base, base)
		}, want: "not an ancestor"},
		{name: "wrong recorded base", marker: func(base, delivery string) string {
			return derivedMarker(delivery, delivery, base)
		}, want: "exact delivery baseline"},
		{name: "two conflicting deliveries", marker: func(base, delivery string) string {
			return goodMarker(base, delivery) + derivedMarker(base, zeros, base)
		}, want: "more than one delivery"},
		{name: "unknown brief", brief: "alpha/99", want: "no brief file for"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, base, delivery := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, c.marker)
			if c.after != nil {
				c.after(t, root, base, delivery)
			}
			brief := c.brief
			if brief == "" {
				brief = "alpha/01"
			}
			code, stdout, stderr := runDerivedCLI(t, bin, root, "--brief", brief)
			if code != 2 || !strings.Contains(stderr, "COULD-NOT-CHECK") || !strings.Contains(stderr, c.want) {
				t.Fatalf("exit %d, want 2 with COULD-NOT-CHECK naming %q\n%s\n%s", code, c.want, stdout, stderr)
			}
			// A refusal is never a verdict: no entry was corroborated or disproved.
			if strings.Contains(stdout, "summary:") {
				t.Fatalf("a could-not-check run must not print a verdict summary:\n%s", stdout)
			}
		})
	}
}

// regression: #1281
func TestDerivedDeliveryRefusesSideBranchDelivery(t *testing.T) {
	// A delivery reachable only through a merged side branch is not on HEAD's
	// first-parent chain; recording it cannot choose the range.
	root := consumersFixture(t, []string{"web/fixed.go: fixed-here"}, map[string]string{"web/fixed.go": "before\n", "web/untouched.go": "before\n"})
	mustGitInit(t, root)
	runGit(t, root, "commit", "--allow-empty", "-qm", "anchor")
	anchor := mustGitOut(t, root, "rev-parse", "HEAD")
	mainBranch := mustGitOut(t, root, "rev-parse", "--abbrev-ref", "HEAD")
	runGit(t, root, "checkout", "-qb", "side")
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "delivered\n")
	mustGitCommitAll(t, root, "side change")
	side := mustGitOut(t, root, "rev-parse", "HEAD")
	runGit(t, root, "checkout", "-q", mainBranch)
	mustWrite(t, filepath.Join(root, "web/untouched.go"), "mainline\n")
	mustGitCommitAll(t, root, "mainline change")
	runGit(t, root, "-c", "commit.gpgsign=false", "merge", "--no-ff", "-qm", "merge side", "side")
	appendBrief(t, root, "\n## Evidence\n\n"+derivedMarker(side, anchor, anchor))
	mustGitCommitAll(t, root, "verifier Evidence commit")
	runGit(t, root, "update-ref", remoteMainRef, "HEAD")
	code, _, stderr := runDerivedCLI(t, buildStatusgenBinary(t), root, "--brief", "alpha/01")
	if code != 2 || !strings.Contains(stderr, "first-parent chain") {
		t.Fatalf("side-branch delivery: exit %d\n%s", code, stderr)
	}
}

// regression: #1281
func TestDerivedDeliveryIgnoresWorkingTreeMarker(t *testing.T) {
	// Only the committed brief names a delivery. A dirty marker naming another
	// valid range, or a marker that was never committed, is not read.
	root, base, delivery := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, goodMarker)
	mustWrite(t, filepath.Join(root, "web/untouched.go"), "later delivery\n")
	later := mustGitOut(t, root, "rev-parse", mustGitCommitAll(t, root, "later delivery"))
	appendBrief(t, root, derivedMarker(later, delivery, base))
	got, gotBase, why := deriveConsumerDelivery(root, derivedBrief)
	if why != "" || got != delivery || gotBase != base {
		t.Fatalf("committed marker must win: got %q base %q why %q, want %q base %q", got, gotBase, why, delivery, base)
	}
	bare, _, _ := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, nil)
	b2 := mustGitOut(t, bare, "rev-parse", "HEAD~2")
	appendBrief(t, bare, derivedMarker(mustGitOut(t, bare, "rev-parse", "HEAD~1"), b2, b2))
	if d, _, why := deriveConsumerDelivery(bare, derivedBrief); d != "" || why == "" {
		t.Fatalf("an uncommitted marker must not name a delivery: %q %q", d, why)
	}
}

// regression: #1281
func TestExplicitBaseIsUnchanged(t *testing.T) {
	// A recorded delivery does not change what an explicit --base means: the
	// branch diff, which carries no evidence about a merged brief, is
	// COULD-NOT-CHECK exactly as before, and --delivery keeps its own contract.
	root, base, delivery := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, goodMarker)
	bin := buildStatusgenBinary(t)
	code, _, stderr := runDerivedCLI(t, bin, root, "--brief", "alpha/01", "--base", remoteMainRef)
	if code != 2 || !strings.Contains(stderr, "alpha/01 is not in the diff against") || strings.Contains(stderr, "delivery could not be derived") {
		t.Fatalf("explicit --base must not derive: exit %d\n%s", code, stderr)
	}
	if got := runConsumers(root, remoteMainRef, "alpha/01"); got != 2 {
		t.Fatalf("runConsumers with an explicit base: got %d, want 2", got)
	}
	code, stdout, stderr := runDerivedCLI(t, bin, root, "--brief", "alpha/01", "--delivery", delivery, "--base", base)
	if code != 0 || !strings.Contains(stdout, "summary: 1 corroborated, 0 disproved, 0 unchecked") {
		t.Fatalf("explicit --delivery: exit %d\n%s\n%s", code, stdout, stderr)
	}
	// Without --brief nothing is derived: the all-briefs scan stays diff-scoped.
	code, stdout, _ = runDerivedCLI(t, bin, root)
	if code != 0 || strings.Contains(stdout, "immutable delivered snapshot") {
		t.Fatalf("no --brief must stay diff-scoped: exit %d\n%s", code, stdout)
	}
}

// regression: #1281
func TestDerivedBranchDiffStillWins(t *testing.T) {
	// A brief the branch diff does carry is checked against that diff, with or
	// without a marker: derivation is only for a diff with no evidence.
	root, base, _ := deliveryFixture(t, []string{"web/fixed.go: fixed-here"})
	runGit(t, root, "update-ref", remoteMainRef, base)
	appendBrief(t, root, "\nbranch edit\n")
	mustGitCommitAll(t, root, "branch edits the brief")
	want := runConsumers(root, remoteMainRef, "alpha/01")
	if got := runConsumersDerived(root, "alpha/01"); got != want || want != 0 {
		t.Fatalf("branch diff path: derived %d, explicit %d, want both 0", got, want)
	}
}

// regression: #1281
func TestDerivedDeliveryMergeCommitPinsFirstParent(t *testing.T) {
	// A merge delivery records the divergence baseline in its marker, but the
	// range judged is the merge against its FIRST parent (the mainline).
	root := consumersFixture(t, []string{"web/fixed.go: fixed-here"}, map[string]string{"web/fixed.go": "before\n", "web/untouched.go": "before\n"})
	mustGitInit(t, root)
	runGit(t, root, "commit", "--allow-empty", "-qm", "anchor")
	anchor := mustGitOut(t, root, "rev-parse", "HEAD")
	mainBranch := mustGitOut(t, root, "rev-parse", "--abbrev-ref", "HEAD")
	runGit(t, root, "checkout", "-qb", "side")
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "delivered\n")
	mustGitCommitAll(t, root, "side change")
	runGit(t, root, "checkout", "-q", mainBranch)
	mustWrite(t, filepath.Join(root, "web/untouched.go"), "mainline\n")
	mustGitCommitAll(t, root, "mainline change")
	mainTip := mustGitOut(t, root, "rev-parse", "HEAD")
	runGit(t, root, "-c", "commit.gpgsign=false", "merge", "--no-ff", "-qm", "merge delivery", "side")
	merge := mustGitOut(t, root, "rev-parse", "HEAD")
	appendBrief(t, root, "\n## Evidence\n\n"+derivedMarker(merge, anchor, anchor))
	mustGitCommitAll(t, root, "verifier Evidence commit")
	runGit(t, root, "update-ref", remoteMainRef, "HEAD")
	got, base, why := deriveConsumerDelivery(root, derivedBrief)
	if why != "" || got != merge || base != mainTip {
		t.Fatalf("merge delivery: got %q base %q why %q, want %q base %q", got, base, why, merge, mainTip)
	}
	if code := runConsumersDerived(root, "alpha/01"); code != 0 {
		t.Fatalf("merge delivery judged against its first parent: got %d, want 0", code)
	}
}

// decoyRepoEnv builds a repository that is NOT the fixture (it holds none of
// the fixture's commits) and returns the repository-locating variables that
// would redirect a git read to it.
func decoyRepoEnv(t *testing.T) map[string]string {
	t.Helper()
	decoy := t.TempDir()
	mustWrite(t, filepath.Join(decoy, "decoy.txt"), "decoy\n")
	mustGitInit(t, decoy)
	runGit(t, decoy, "commit", "--allow-empty", "-qm", "decoy only")
	return map[string]string{
		"GIT_DIR":              filepath.Join(decoy, ".git"),
		"GIT_WORK_TREE":        decoy,
		"GIT_COMMON_DIR":       filepath.Join(decoy, ".git"),
		"GIT_OBJECT_DIRECTORY": filepath.Join(decoy, ".git", "objects"),
	}
}

// regression: #1281
func TestDerivedDeliveryIgnoresRepoLocatingEnv(t *testing.T) {
	// The environment must not redirect the delivery derivation: with a
	// repository-locating variable pointing at a decoy repository the derived
	// run still reads the real checkout and reaches the same verdict.
	root, base, delivery := derivedFixture(t, []string{"web/fixed.go: fixed-here"}, goodMarker)
	bin := buildStatusgenBinary(t)
	if code, _, _ := runDerivedCLI(t, bin, root, "--brief", "alpha/01"); code != 0 {
		t.Fatalf("positive control: clean environment exit %d, want 0", code)
	}
	for name, val := range decoyRepoEnv(t) {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, val)
			code, stdout, stderr := runDerivedCLI(t, bin, root, "--brief", "alpha/01")
			if code != 0 || !strings.Contains(stdout, base+".."+delivery+" (immutable delivered snapshot)") {
				t.Fatalf("%s=decoy redirected the derivation: exit %d\n%s\n%s", name, code, stdout, stderr)
			}
		})
	}
}

// regression: #1281
func TestDerivedDeliveryEnvIsCleanedPerVariable(t *testing.T) {
	// Every repository-locating variable is removed from a provenance read's
	// environment (an alternate object directory, an index file, ... have no
	// end-to-end effect on a clean fixture, so each is pinned here directly).
	want := []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"}
	for _, name := range want {
		t.Setenv(name, "/nonexistent/decoy")
	}
	t.Setenv("GIT_UNRELATED_KEPT", "kept")
	env := historyGit(t.TempDir(), "rev-parse", "HEAD").Env
	has := func(k string) bool {
		for _, kv := range env {
			if strings.HasPrefix(kv, k+"=") {
				return true
			}
		}
		return false
	}
	for _, name := range want {
		if has(name) {
			t.Errorf("%s survives into the provenance git environment", name)
		}
	}
	if !has("GIT_UNRELATED_KEPT") || !has("GIT_GRAFT_FILE") {
		t.Errorf("unrelated variables must be kept and the graft pin set: %v", env)
	}
}
