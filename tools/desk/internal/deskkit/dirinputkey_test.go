package deskkit

// dirinputkey_test.go — #4865: a receipt for a declared directory never presents a member's
// content digest to the pattern sweep as <identifier>":"<64 hex>", and every member stays
// covered by a digest.

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const dirKeyDir = "tools/desk/internal/cellapi/testdata/deskdimage"

// The three member names the issue reported, plus one with no trigger word.
var dirKeyMembers = []string{"secret-mount.fixture", "credential-copy.fixture", "missing-secret-scan.fixture", "plain.fixture"}

func dirKeyBrief() string { return "## Context\nfiles:\n- `" + dirKeyDir + "/`\n" }

func dirKeyTree() fakeTree {
	tree := fakeTree{files: map[string]string{wakeTestBriefPath: dirKeyBrief()}}
	for i, m := range dirKeyMembers {
		tree.files[dirKeyDir+"/"+m] = "fixture " + string(rune('a'+i)) + "\n"
	}
	return tree
}

// dirKeyRecord is the writer's own record for a brief declaring only dirKeyDir/.
func dirKeyRecord(t *testing.T) ([]byte, WakeReceipt) {
	t.Helper()
	in := wakeTestInput()
	in.LandedBrief = []byte(dirKeyBrief())
	raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), in, dirKeyTree())
	if err != nil {
		t.Fatal(err)
	}
	var r WakeReceipt
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	return raw, r
}

// gitleaksSweep runs the pinned scanner the way the pattern-sweep job does — `gitleaks dir .`
// from a tree root carrying the repository's own .gitleaks.toml — over one record placed at
// the per-outcome-record allowlist's path. It returns the generic-api-key findings.
func gitleaksSweep(t *testing.T, bin string, record []byte) []string {
	t.Helper()
	config, err := os.ReadFile(filepath.Join("..", "..", "..", "..", ".gitleaks.toml"))
	if err != nil {
		t.Fatalf("read the repository's .gitleaks.toml: %v", err)
	}
	root := t.TempDir()
	rel := "docs/streams/verify-outcomes/demo/01-20261029T000000Z-0123456789ab.json"
	for p, b := range map[string][]byte{".gitleaks.toml": config, rel: append(record, '\n')} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	report := filepath.Join(t.TempDir(), "report.json")
	cmd := exec.Command(bin, "dir", ".", "--config", ".gitleaks.toml", "--no-banner", "--exit-code", "9",
		"--report-format", "json", "--report-path", report)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 9) {
		t.Fatalf("gitleaks did not complete: %v\n%s", err, out)
	}
	b, rerr := os.ReadFile(report)
	if rerr != nil {
		t.Fatalf("gitleaks wrote no report: %v\n%s", rerr, out)
	}
	var findings []struct{ RuleID, Match, File string }
	if err := json.Unmarshal(b, &findings); err != nil {
		t.Fatalf("gitleaks report: %v", err)
	}
	var got []string
	for _, f := range findings {
		if f.File != rel {
			t.Fatalf("finding outside the record: %+v", f)
		}
		got = append(got, f.RuleID+" "+f.Match)
	}
	if (len(got) == 0) != (err == nil) {
		t.Fatalf("gitleaks exit %v disagrees with its %d findings", err, len(got))
	}
	sort.Strings(got)
	return got
}

// The writer's record for a declared directory holding the reported trigger-word members
// passes the repository's pattern sweep with its allowlist unchanged. The legacy shape of the
// same record (one file: key per member) is swept too, as the control that proves the harness
// fires: it must report exactly the three trigger-word members.
// regression: #4865
func TestDirKeysPassPatternSweep(t *testing.T) {
	bin, err := exec.LookPath("gitleaks")
	if err != nil {
		t.Skip("gitleaks is not on PATH; the leaksweep-pattern job scans the committed writer golden (TestDirKeysGoldenRecord) with the pinned scanner instead")
	}
	raw, r := dirKeyRecord(t)
	if got := gitleaksSweep(t, bin, raw); len(got) != 0 {
		t.Fatalf("the writer's record trips the pattern sweep:\n%s\n%s", strings.Join(got, "\n"), raw)
	}

	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	legacy := map[string]string{}
	for k, v := range r.Inputs {
		legacy[k] = v
	}
	for _, m := range dirKeyMembers {
		legacy[inputKeyFilePrefix+dirKeyDir+"/"+m] = Sha256Hex([]byte(dirKeyTree().files[dirKeyDir+"/"+m]))
	}
	fields["inputs"] = legacy
	twin, _ := json.Marshal(fields)
	got := gitleaksSweep(t, bin, twin)
	if len(got) != 3 {
		t.Fatalf("control: the legacy per-file shape should trip exactly the three trigger-word members, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	for _, want := range []string{"credential-copy.fixture", "missing-secret-scan.fixture", "secret-mount.fixture"} {
		if !strings.Contains(strings.Join(got, "\n"), want) {
			t.Errorf("control: %s did not trip:\n%s", want, strings.Join(got, "\n"))
		}
	}
}

// dirKeyGolden is the writer's committed output for dirKeyTree. It lives outside the
// per-outcome-record allowlist path, so the leaksweep-pattern job's whole-tree scan reads it
// with no allowlist at all: a writer that puts a member digest back after a trigger-word key
// reddens that job even where gitleaks is absent from the test environment.
const dirKeyGolden = "testdata/dirinputkey/record.json"

// regression: #4865
func TestDirKeysGoldenRecord(t *testing.T) {
	raw, _ := dirKeyRecord(t)
	raw = append(raw, '\n')
	if os.Getenv("DIRKEY_GOLDEN_UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(dirKeyGolden), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dirKeyGolden, raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(dirKeyGolden)
	if err != nil {
		t.Fatalf("read %s (regenerate with DIRKEY_GOLDEN_UPDATE=1): %v", dirKeyGolden, err)
	}
	if string(want) != string(raw) {
		t.Fatalf("the writer drifted from %s (regenerate with DIRKEY_GOLDEN_UPDATE=1 only after the pattern sweep passes):\n got %s\nwant %s", dirKeyGolden, raw, want)
	}
}

// The record's keys for a declared directory: the unchanged scope:<dir>/ membership digest,
// one file:<dir>/ digest over every member, and no file:<member> key at all.
// regression: #4865
func TestDirKeyRecordShape(t *testing.T) {
	_, r := dirKeyRecord(t)
	tree := dirKeyTree()
	scope, err := ResolveDeclaredScope([]string{dirKeyDir}, tree)
	if err != nil {
		t.Fatal(err)
	}
	if r.Inputs[InputKeyScopePrefix+dirKeyDir+"/"] != scope.Shapes[InputKeyScopePrefix+dirKeyDir+"/"] {
		t.Fatalf("the scope: membership digest moved: %v", r.Inputs)
	}
	var members []string
	for _, m := range dirKeyMembers {
		members = append(members, dirKeyDir+"/"+m)
	}
	want, _ := DirInputDigest(members, func(f string) (string, error) { return Sha256Hex([]byte(tree.files[f])), nil })
	if got := r.Inputs[DirInputKey(dirKeyDir)]; got != want || !scopeDigestFormat.MatchString(got) {
		t.Fatalf("file:%s/ = %q, want %q", dirKeyDir, got, want)
	}
	for k := range r.Inputs {
		if strings.HasPrefix(k, inputKeyFilePrefix+dirKeyDir+"/") && k != DirInputKey(dirKeyDir) {
			t.Errorf("a member key survived: %s", k)
		}
	}
	if len(r.Inputs) != 4 {
		t.Errorf("inputs = %v, want scope, directory, brief and tool keys", r.Inputs)
	}
}

// Coverage is unchanged: every member is still bound by a digest. Editing, adding, removing
// or renaming any one member wakes a held record; the untouched tree holds it.
// regression: #4865
func TestDirKeyCoversEveryMember(t *testing.T) {
	_, r := dirKeyRecord(t)
	r.Brief = "demo/01"
	root := scopeKeyRoot(t, dirKeyTree().files)
	rr := NewRootRevisionReader(root, wakeTestInput().ToolVersion)
	if state, why := Unchanged(r, rr); state != InputsUnchanged {
		t.Fatalf("untouched tree: %v %s", state, why)
	}
	for _, m := range dirKeyMembers {
		full := filepath.Join(root, dirKeyDir, m)
		before, err := os.ReadFile(full)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, append(before, 'x'), 0644); err != nil {
			t.Fatal(err)
		}
		state, why := Unchanged(r, rr)
		if state != InputsChanged || !strings.Contains(why, dirKeyDir+"/") {
			t.Errorf("content edit of %s held: %v %s", m, state, why)
		}
		if err := os.Rename(full, full+".moved"); err != nil {
			t.Fatal(err)
		}
		if state, why := Unchanged(r, rr); state != InputsChanged {
			t.Errorf("rename of %s held: %v %s", m, state, why)
		}
		if err := os.Remove(full + ".moved"); err != nil {
			t.Fatal(err)
		}
		if state, why := Unchanged(r, rr); state != InputsChanged {
			t.Errorf("removal of %s held: %v %s", m, state, why)
		}
		if err := os.WriteFile(full, before, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if state, why := Unchanged(r, rr); state != InputsUnchanged {
		t.Fatalf("restored tree: %v %s", state, why)
	}
	if err := os.WriteFile(filepath.Join(root, dirKeyDir, "new.fixture"), []byte("added\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if state, why := Unchanged(r, rr); state != InputsChanged {
		t.Errorf("an added member held: %v %s", state, why)
	}
	// The directory digest alone (scope: membership dropped) still wakes on content.
	if err := os.Remove(filepath.Join(root, dirKeyDir, "new.fixture")); err != nil {
		t.Fatal(err)
	}
	got, ok := rr.Revision(DirInputKey(dirKeyDir))
	if !ok || got != r.Inputs[DirInputKey(dirKeyDir)] {
		t.Fatalf("reader digest %q, writer %q", got, r.Inputs[DirInputKey(dirKeyDir)])
	}
}

// A record written before #4865 (one file: key per member, no file:<dir>/ key) still reads
// unchanged on an untouched tree and still wakes on a member edit: no forced re-verification.
// regression: #4865
func TestDirKeyLegacyRecordReads(t *testing.T) {
	_, r := dirKeyRecord(t)
	r.Brief = "demo/01"
	tree := dirKeyTree()
	delete(r.Inputs, DirInputKey(dirKeyDir))
	for _, m := range dirKeyMembers {
		r.Inputs[inputKeyFilePrefix+dirKeyDir+"/"+m] = Sha256Hex([]byte(tree.files[dirKeyDir+"/"+m]))
	}
	root := scopeKeyRoot(t, tree.files)
	rr := NewRootRevisionReader(root, wakeTestInput().ToolVersion)
	if state, why := Unchanged(r, rr); state != InputsUnchanged {
		t.Fatalf("legacy record on an untouched tree: %v %s", state, why)
	}
	if err := os.WriteFile(filepath.Join(root, dirKeyDir, dirKeyMembers[0]), []byte("edited\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if state, why := Unchanged(r, rr); state != InputsChanged {
		t.Fatalf("legacy record held across a member edit: %v %s", state, why)
	}
}

// A file:<dir>/ value sits where the pattern sweep's generic rule does not look, so the
// reader refuses any value that is not the writer's digest, like a scope: value; and the
// key never resolves outside a canonical in-root directory.
// regression: #4865
func TestDirKeyValueMustBeDigest(t *testing.T) {
	_, r := dirKeyRecord(t)
	r.Brief = "demo/01"
	root := scopeKeyRoot(t, dirKeyTree().files)
	rr := NewRootRevisionReader(root, wakeTestInput().ToolVersion)
	for _, bad := range []string{"", "not-a-digest", strings.Repeat("A", 64), strings.Repeat("a", 63), "ghp_" + randomBase62(t, 36)} {
		r.Inputs[DirInputKey(dirKeyDir)] = bad
		if err := rr.CheckReceiptScope(r); err == nil || !strings.Contains(err.Error(), "not a directory digest") {
			t.Errorf("value %q accepted: %v", bad, err)
		}
		if state, _ := Unchanged(r, rr); state == InputsUnchanged {
			t.Errorf("value %q read as unchanged", bad)
		}
	}
	for _, key := range []string{"file:/", "file:" + dirKeyDir + "//", "file:../outside/", "file:./" + dirKeyDir + "/", "file:" + dirKeyDir + "/" + dirKeyMembers[0] + "/", "file:tools/absent/"} {
		if _, ok := rr.Revision(key); ok {
			t.Errorf("%s resolved", key)
		}
	}
}

// A declared directory that holds the brief keeps one key per member, so a pending landing
// can still substitute the brief's own key; every other declared directory folds.
// regression: #4865
func TestDirKeyBriefDirKeepsMembers(t *testing.T) {
	brief := "## Context\nfiles:\n- `docs/streams/demo/`\n- `" + dirKeyDir + "/`\n"
	tree := dirKeyTree()
	tree.files[wakeTestBriefPath] = brief
	tree.files["docs/streams/demo/notes.md"] = "notes\n"
	in := wakeTestInput()
	in.LandedBrief = []byte(brief)
	raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), in, tree)
	if err != nil {
		t.Fatal(err)
	}
	var r WakeReceipt
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Inputs[DirInputKey("docs/streams/demo")]; ok {
		t.Fatalf("the brief's directory folded: %v", r.Inputs)
	}
	if r.Inputs["file:docs/streams/demo/notes.md"] == "" || r.Inputs["file:"+wakeTestBriefPath] != Sha256Hex(in.LandedBrief) {
		t.Fatalf("the brief's directory lost its member keys: %v", r.Inputs)
	}
	if r.Inputs[DirInputKey(dirKeyDir)] == "" {
		t.Fatalf("the other directory did not fold: %v", r.Inputs)
	}
	scope, err := ResolveDeclaredScope([]string{"docs/streams/demo", dirKeyDir}, tree)
	if err != nil {
		t.Fatal(err)
	}
	if missing := scope.Uncovered(r.Inputs); len(missing) != 0 {
		t.Fatalf("uncovered: %v", missing)
	}
}
