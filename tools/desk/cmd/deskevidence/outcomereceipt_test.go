package main

// outcomereceipt_test.go — verify-reset/03: --outcome-record derives the verify-wake-v1 receipt
// for every verify-fail / blocked record, from the brief's files: block at the record's sha.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// issueForge adds the one forge read check (b) needs to the behavioural fake.
type issueForge struct{ *fakeForge }

func (issueForge) GetIssue(_ deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	return &deskkit.Issue{Number: n, State: "open"}, nil
}

const receiptBriefPath = "docs/streams/example-stream/brief-14-x.md"

// receiptGitRoot is a git checkout holding a brief that declares one file and one directory;
// it returns the root and the commit the record names.
func receiptGitRoot(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	rvGitIn(t, dir, "init", "-q", "-b", "main")
	rvWriteAndAdd(t, dir, receiptBriefPath, briefWithFiles("foo/bar.go", "foo/pkg/", "foo/planned.go"))
	rvWriteAndAdd(t, dir, "foo/bar.go", "package foo\n")
	rvWriteAndAdd(t, dir, "foo/pkg/a.go", "package pkg\n")
	rvWriteAndAdd(t, dir, "foo/pkg/b.go", "package pkg // b\n")
	rvGitIn(t, dir, "commit", "-qm", "base")
	return dir, rvGitIn(t, dir, "rev-parse", "HEAD")
}

func TestDeriveReceiptFromGit(t *testing.T) {
	root, sha := receiptGitRoot(t)
	f := &fakeForge{}
	landed := briefWithFiles("foo/bar.go", "foo/pkg/", "foo/planned.go") + "\n### Evidence appended\n"
	f.setFile(receiptBriefPath, landed)
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	line := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"` + sha +
		`","blocker_kind":"implementation","blocker_ref":"#7"}`

	out, err := deriveOutcomeReceipt([]byte(line), root, issueForge{f}, fr, "main")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	var wr deskkit.WakeReceipt
	if err := json.Unmarshal(out, &wr); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"file:foo/bar.go", "file:foo/pkg/a.go", "file:foo/pkg/b.go", "file:" + receiptBriefPath, "tool"} {
		if wr.Inputs[k] == "" {
			t.Errorf("inputs has no %s: %v", k, wr.Inputs)
		}
	}
	if len(wr.Inputs) != 5 {
		t.Errorf("inputs = %v, want exactly 5 keys (the planned path is absent at the sha)", wr.Inputs)
	}
	if wr.Inputs["file:"+receiptBriefPath] != deskkit.Sha256Hex([]byte(landed)) {
		t.Errorf("the brief input is not the brief as it lands")
	}
	// The derived receipt passes the three checks a hand-written one is held to.
	if err := validateReceipt(root, issueForge{f}, fr, "main", "example-org/tracker", wr); err != nil {
		t.Errorf("the derived receipt fails validateReceipt: %v", err)
	}

	pass := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verified","sha":"` + sha + `"}`
	if got, err := deriveOutcomeReceipt([]byte(pass), root, issueForge{f}, fr, "main"); err != nil || string(got) != pass {
		t.Errorf("a verified record must pass through unchanged: %s, %v", got, err)
	}
}

func TestOutcomeWriteLandsReceipt(t *testing.T) {
	f, errBuf := setupFake(t)
	outcomeReceiptFn = deriveOutcomeReceipt
	forgeForFn = func(owner, name string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return deskkit.OutboundChecked(issueForge{f}, "verifier"), deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	root, sha := receiptGitRoot(t)
	f.setFile(receiptBriefPath, briefWithFiles("foo/bar.go", "foo/pkg/", "foo/planned.go"))
	line := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"blocked","sha":"` + sha +
		`","blocker_kind":"human-action","blocker_ref":"example-org/tracker#7"}`
	plain, _ := deskkit.RecordName([]byte(line))
	recFile := writeRepoFile(t, "record.json", line+"\n")

	code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
	}
	if len(f.writes) != 1 {
		t.Fatalf("writes = %v, want exactly one", f.writes)
	}
	w := f.writes[0]
	if w.File == plain || !sameBriefSlot(w.File, plain) {
		t.Errorf("landed at %s; want a re-derived name in %s's slot", w.File, plain)
	}
	if want, _ := deskkit.RecordName(w.Content); want != w.File {
		t.Errorf("landed at %s but its bytes name %s", w.File, want)
	}
	if !strings.Contains(string(w.Content), `"wake_schema":"`+deskkit.SchemaWakeV1+`"`) ||
		!strings.Contains(string(w.Content), `"file:foo/pkg/b.go"`) {
		t.Errorf("landed record carries no derived receipt: %s", w.Content)
	}
}

func TestOutcomeWriteRefusesBadRef(t *testing.T) {
	f, errBuf := setupFake(t)
	outcomeReceiptFn = deriveOutcomeReceipt
	root, sha := receiptGitRoot(t)
	f.setFile(receiptBriefPath, briefWithFiles("foo/bar.go"))
	for _, extra := range []string{
		`"blocker_kind":"implementation","blocker_ref":"to file"`,
		`"blocker_kind":"implementation"`,
		`"blocker_ref":"#7"`,
	} {
		errBuf.Reset()
		line := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"` + sha + `",` + extra + `}`
		recFile := writeRepoFile(t, "record.json", line+"\n")
		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile, "--root", root})
		if code != deskkit.ExitRefused || !strings.Contains(errBuf.String(), "blocker_") {
			t.Errorf("%s: exit %d (stderr %q), want %d naming the blocker field", extra, code, errBuf.String(), deskkit.ExitRefused)
		}
	}
	if len(f.writes) != 0 {
		t.Errorf("a refused record landed: %v", f.writes)
	}
}
