package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInitInto runs `qualgen init --root dir` and returns combined stdout for
// assertions. It fails the test on a non-zero exit.
func runInitInto(t *testing.T, dir string) string {
	t.Helper()
	var out, errb strings.Builder
	if rc := runInit([]string{"--root", dir}, &out, &errb); rc != 0 {
		t.Fatalf("qualgen init --root %s exit %d; stderr:\n%s", dir, rc, errb.String())
	}
	return out.String()
}

// TestInitEmitsWorkflowAndPin: init scaffolds the two report-pack files into an
// empty repo (pack contract criterion 2). Row 3's "the emitted workflow exists".
func TestInitEmitsWorkflowAndPin(t *testing.T) {
	dir := t.TempDir()
	runInitInto(t, dir)
	for _, rel := range []string{
		".github/workflows/assay-qualgen.yml",
		".assay-versions",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("init did not create %s: %v", rel, err)
		}
	}
}

// TestInitWorkflowNamesNoProducingRepoPath is Verify row 3's core assertion: the
// EMITTED adopter workflow acquires qualgen through channel E only and names NO
// path specific to the producing repository. An adopter has no qualgen source
// tree, so any build-from-source shape (a `qualgen/` source dir, `go build`, a
// `cd qualgen`) in the emitted workflow would be the producing-repo self-host
// seam leaking into an adopter's CI.
func TestInitWorkflowNamesNoProducingRepoPath(t *testing.T) {
	dir := t.TempDir()
	runInitInto(t, dir)
	wf, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "assay-qualgen.yml"))
	if err != nil {
		t.Fatalf("read emitted workflow: %v", err)
	}
	body := string(wf)
	for _, forbidden := range []string{
		"go build",   // build-from-source
		"cd qualgen", // producing-repo source dir
		"qualgen/",   // any producing-repo source path
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("emitted adopter workflow names a producing-repo build-from-source marker %q — an adopter consumes the pinned binary, not the source:\n%s", forbidden, body)
		}
	}
	// Positive: it must acquire the PINNED binary (channel E) and hash-verify it.
	for _, want := range []string{
		"qualgen-$plat",  // per-platform pinned artifact name
		".assay-versions", // the pin file
		"shasum -a 256 -c -", // hash verification
	} {
		if !strings.Contains(body, want) {
			t.Errorf("emitted workflow missing channel-E marker %q:\n%s", want, body)
		}
	}
}

// TestInitWorkflowIsSingleWriter is the pack contract's criterion 3, and the
// shape Verify rows 4–6 exercise: the pull-request half renders and discards
// (no --write), while ONLY the push-to-main half authorizes the write with
// QUALGEN_QUALITY_WRITER=ci and runs --write. A --write on the PR half would
// let a branch modify the committed view.
func TestInitWorkflowIsSingleWriter(t *testing.T) {
	dir := t.TempDir()
	runInitInto(t, dir)
	wf, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "assay-qualgen.yml"))
	if err != nil {
		t.Fatalf("read emitted workflow: %v", err)
	}
	body := string(wf)

	// Assert on EXECUTABLE lines only, not the header comment (which documents
	// the two-half shape in prose and legitimately names the --write command).
	// A YAML comment line's trimmed text starts with '#'.
	var cmdLines []string
	for _, ln := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "#") {
			continue
		}
		cmdLines = append(cmdLines, ln)
	}
	cmds := strings.Join(cmdLines, "\n")

	// The render (PR) job must run `qualgen report --out .` and must NOT carry
	// --write anywhere in its render step. Assert the discard command is present
	// and that the ONLY --write invocation is under the ci-writer env.
	if !strings.Contains(cmds, "qualgen report --out .") {
		t.Errorf("emitted workflow does not render with `qualgen report --out .`:\n%s", body)
	}
	writes := strings.Count(cmds, "qualgen report --out . --write")
	if writes != 1 {
		t.Errorf("expected exactly ONE executable --write invocation (the push-to-main regen), got %d:\n%s", writes, body)
	}
	if !strings.Contains(cmds, "QUALGEN_QUALITY_WRITER: ci") {
		t.Errorf("emitted workflow does not authorize the committed-view write with QUALGEN_QUALITY_WRITER=ci:\n%s", body)
	}
	// The render job (github.event_name == 'pull_request') must not sit above a
	// --write. Split the executable lines on the regen job's ci-writer env and
	// assert the PR half is clean.
	prHalf := cmds
	if i := strings.Index(cmds, "QUALGEN_QUALITY_WRITER: ci"); i >= 0 {
		prHalf = cmds[:i]
	}
	if strings.Contains(prHalf, "--write") {
		t.Errorf("the pull-request render half carries a --write — a branch could modify the committed QUALITY.md:\n%s", prHalf)
	}
}

// TestInitNeverOverwrites: init fills gaps, it does not clobber. A pre-existing
// .assay-versions (a repo already pinning another pack) is left byte-for-byte
// unchanged and reported as skipped, with the note to add the qualgen line.
func TestInitNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	const existing = "statusgen-linux-amd64  v0.9.1  deadbeef\n"
	if err := os.WriteFile(filepath.Join(dir, ".assay-versions"), []byte(existing), 0o644); err != nil {
		t.Fatalf("seed .assay-versions: %v", err)
	}
	out := runInitInto(t, dir)
	got, err := os.ReadFile(filepath.Join(dir, ".assay-versions"))
	if err != nil {
		t.Fatalf("read .assay-versions: %v", err)
	}
	if string(got) != existing {
		t.Errorf(".assay-versions was overwritten:\ngot:  %q\nwant: %q", string(got), existing)
	}
	if !strings.Contains(out, "exists   .assay-versions") {
		t.Errorf("init did not report the pre-existing .assay-versions as skipped:\n%s", out)
	}
	if !strings.Contains(out, "the qualgen pin\nwas NOT added") {
		t.Errorf("init did not print the existing-pin note telling the adopter to add the qualgen line:\n%s", out)
	}
	// The workflow, being absent, must still be created.
	if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "assay-qualgen.yml")); err != nil {
		t.Errorf("init skipped the workflow too — it should fill the gap: %v", err)
	}
}

// TestInitDryRunWritesNothing: --dry-run previews and mutates no tree.
func TestInitDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	var out, errb strings.Builder
	if rc := runInit([]string{"--root", dir, "--dry-run"}, &out, &errb); rc != 0 {
		t.Fatalf("dry-run exit %d; stderr:\n%s", rc, errb.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("--dry-run wrote %d entr(y/ies) into the tree; want 0", len(entries))
	}
	if !strings.Contains(out.String(), "would create  .github/workflows/assay-qualgen.yml") {
		t.Errorf("--dry-run did not preview the workflow:\n%s", out.String())
	}
}
