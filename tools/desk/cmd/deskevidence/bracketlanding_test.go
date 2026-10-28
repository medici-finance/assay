package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// regression: #4071
// TestBracketedWitnessSurvivesLanding: a bracketed container witness whose App
// Runner cells carry no principal, one of them ending in a (git-config) source
// tag and one with an escaped \| in its Command cell (never a delimiter, in
// cellSpans here and in statusgen's digest alike), goes through this tool's landing stamp (stampEvidencePrincipal, resolved
// from a fixture roster through the principalSubjectFn seam, no forge), which
// adds the on-behalf-of annotation before the tag. `statusgen verifyrun
// --bracket-digest`, built from this tree, run on the stamped Evidence block
// still reports verdict=match for the bracket, so the landing never breaks an
// honest bracket; the same block with one Result cell edited reports mismatch,
// so the digest still binds the rows.
func TestBracketedWitnessSurvivesLanding(t *testing.T) {
	statusgen := buildStatusgen(t)
	setupStamp(t, true)
	resolved := 0
	old := principalSubjectFn
	principalSubjectFn = func(repo string) (string, error) { resolved++; return old(repo) }
	t.Cleanup(func() { principalSubjectFn = old })

	raw, err := os.ReadFile(filepath.Join("testdata", "container-bracket", "bracketed-witness.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	base := content[:strings.Index(content, "## Evidence")]
	stamped, rows, err := stampEvidencePrincipal([]byte(content), []byte(base), func() (string, error) {
		return principalSubjectFn(stampPublicRepo)
	})
	if err != nil {
		t.Fatalf("stamp: %v", err)
	}
	if resolved != 1 || len(rows) != 2 || strings.Count(string(stamped), "(on-behalf-of human:") != 2 || !strings.Contains(string(stamped), ") (git-config) |") {
		t.Fatalf("the landing stamp did not annotate both App rows (resolved %d, stamped %d):\n%s", resolved, len(rows), stamped)
	}
	if string(stamped) == content {
		t.Fatal("the stamp left the bytes unchanged; the landing is untested")
	}

	if !strings.Contains(string(stamped), `-count=1 .`+"`"+` | pass exit=0`) || !regexp.MustCompile(`\(on-behalf-of human:[^\s()]+\) \(forge-identity\) \|`).MatchString(string(stamped)) {
		t.Fatalf("the stamp did not land in the Runner cell of the escaped-pipe row:\n%s", stamped)
	}
	if verdict, code := bracketVerdict(t, statusgen, string(stamped)); verdict != "match" || code != 0 {
		t.Fatalf("--bracket-digest of the stamped brief: verdict %q exit %d, want match exit 0", verdict, code)
	}
	edited := strings.Replace(string(stamped), "| pass exit=0 sandbox=container-netns |", "| fail exit=1 sandbox=container-netns |", 1)
	if edited == string(stamped) {
		t.Fatal("could not edit the fixture's Result cell")
	}
	if verdict, code := bracketVerdict(t, statusgen, edited); verdict != "mismatch" || code != 1 {
		t.Fatalf("an edited Result cell: verdict %q exit %d, want mismatch exit 1", verdict, code)
	}
}

// buildStatusgen builds this tree's statusgen into a scratch directory. The
// test fails, never skips, when it cannot: an unbuilt digest proves nothing.
func buildStatusgen(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "statusgen"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "go.mod")); err != nil {
		t.Fatalf("statusgen source not beside this module: %v", err)
	}
	bin := filepath.Join(t.TempDir(), "statusgen")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = src
	if out, err := goquiet.Cmd(build).CombinedOutput(); err != nil {
		t.Fatalf("build statusgen: %v\n%s", err, out)
	}
	return bin
}

// bracketVerdict runs `statusgen verifyrun --bracket-digest` on text, which
// must carry exactly one bracket, and returns its verdict and the exit code.
func bracketVerdict(t *testing.T, statusgen, text string) (string, int) {
	t.Helper()
	f := filepath.Join(t.TempDir(), "evidence.md")
	if err := os.WriteFile(f, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(statusgen, "verifyrun", "--bracket-digest", "--witness", f).Output()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("--bracket-digest: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	m := regexp.MustCompile(`^bracket=L\d+ header=L\d+ witness=sha256:[0-9a-f]{64} recorded=sha256:[0-9a-f]{64} verdict=(\S+) span-lines=tree-digest:1,container-image:1,attestation:1$`).FindStringSubmatch(lines[0])
	if len(lines) != 1 || m == nil {
		t.Fatalf("--bracket-digest printed %q, want one bracket line", out)
	}
	return m[1], code
}
