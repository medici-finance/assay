package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A protected record (#4499) is a --acceptance-record holding only the
// Verification-Table line: a delivery that added a supplemental row and
// changed no original command. verifyrun runs the original rows as delivered,
// derives the supplemental rows from git at the recorded coordinates, and
// witnesses those author-added with no acceptance credit. The tests below pin
// the route (TestProtectedRecordRun) and its three negative controls: an
// original row whose definition moved is refused before any row runs
// (TestProtectedRecordRefusal), and a supplemental row cannot be promoted to
// credit nor the record survive a missing marker (TestProtectedRecordCredit).

type protectedRecordFixture struct {
	root, path, base, delivery, record string
}

const recordSupplemental = "| 3 | `printf supplemental` | exit 0 |\n"

// newRecordFixture commits the original table (rows 1-2), then a delivery
// that adds row 3; deliver may also rewrite the delivered Verify section.
func newRecordFixture(t *testing.T, deliver func(string) string) protectedRecordFixture {
	t.Helper()
	o := vfDefaults()
	o.status = "verified"
	o.verified = "2026-08-13 human:alex"
	o.evidence = ""
	root, _ := vfFixture(t, o)
	path := vfBrief(root)
	base := vfRev(t, root, "HEAD")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	verify := vfVerify + recordSupplemental
	if deliver != nil {
		verify = deliver(verify)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(raw), vfVerify, verify, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailWorker)
	delivery := vfRev(t, root, "HEAD")
	vfGit(t, root, vfMailVerifier, "config", "user.name", vfRunner)
	vfGit(t, root, vfMailVerifier, "config", "user.email", vfMailVerifier)
	record := filepath.Join(t.TempDir(), "record.txt")
	line := "Verification-Table: delivery=" + delivery + " base=" + base + " original=" + base + "\n"
	if err := os.WriteFile(record, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	return protectedRecordFixture{root, path, base, delivery, record}
}

func (f protectedRecordFixture) run(t *testing.T, extra ...string) (int, string) {
	t.Helper()
	args := append([]string{"--brief", f.path, "--root", f.root, "--acceptance-record", f.record}, extra...)
	res, diag := captureVerifyrun(t, args)
	resetReadMemo()
	return res.code, diag
}

// closes reports whether every closure reader accepts the brief as verified.
func (f protectedRecordFixture) closes(t *testing.T) bool {
	t.Helper()
	resetReadMemo()
	code, _, _ := captureVerifyclosure(t, []string{"--root", f.root, "--brief", "vf/01"})
	res, _ := captureVerifyrun(t, []string{"--check", "--brief", f.path})
	info := &briefInfo{File: vfBriefRel, Row: &briefInfoRow{Status: "verified", Verified: "2026-08-13 human:alex"}}
	infoErr := checkBriefInfoVerified(f.root, info)
	streams, _, err := loadStreams(f.root)
	if err != nil {
		t.Fatal(err)
	}
	released := evaluateCoverage(f.root, streams, coverageOptions{Revision: vfRev(t, f.root, "HEAD")})["vf/01"].Released
	if (code == 0) != (res.code == 0) || (code == 0) != (infoErr == nil) || (code == 0) != released {
		t.Fatalf("closure readers disagree: verifyclosure=%d check=%d info=%v released=%v", code, res.code, infoErr, released)
	}
	return code == 0
}

// regression: #4499
func TestProtectedRecordRun(t *testing.T) {
	f := newRecordFixture(t, nil)
	if code, diag := f.run(t); code != 0 {
		t.Fatalf("protected record run: %d %s", code, diag)
	}
	_, ev, err := briefSections(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ev, "\nVerification-Table: delivery="+f.delivery+" base="+f.base+" original="+f.base+"\n") {
		t.Fatalf("no Verification-Table marker written:\n%s", ev)
	}
	rows := parseEvidenceRows(ev)
	for id, want := range map[string]string{"1": "pass exit=0", "2": "pass exit=1", "3": "author-added; execution exit=0; no acceptance credit"} {
		if len(rows[id]) != 1 || !strings.Contains(rows[id][0].Text, "| "+want+" |") {
			t.Fatalf("row %s: want %q, got %+v", id, want, rows[id])
		}
	}
	vfAppend(t, f.root, "\n\n"+vfPass)
	vfCommit(t, f.root, vfMailVerifier)
	if !f.closes(t) {
		t.Fatal("completed protected record does not close")
	}
}

// regression: #4499
func TestProtectedRecordRefusal(t *testing.T) {
	cases := []struct {
		name, want string
		deliver    func(string) string
		after      func(t *testing.T, f protectedRecordFixture)
		extra      []string
	}{
		{name: "original-edited-after", want: "differs from immutable delivery",
			after: func(t *testing.T, f protectedRecordFixture) { editVerify(t, f, "`true`", "`true || :`") }},
		{name: "added-edited-after", want: "differs from immutable delivery",
			after: func(t *testing.T, f protectedRecordFixture) {
				editVerify(t, f, "`printf supplemental`", "`printf other`")
			}},
		{name: "command-at-delivery", want: "original row 1 differs from its delivered definition",
			deliver: func(v string) string { return strings.Replace(v, "`true`", "`true || :`", 1) }},
		{name: "expect-at-delivery", want: "original row 2 differs from its delivered definition",
			deliver: func(v string) string { return strings.Replace(v, "| exit 1 |", "| exit 0 |", 1) }},
		{name: "row-list-line", want: "nothing else",
			after: func(t *testing.T, f protectedRecordFixture) { appendRecord(t, f, "Rows: 1,2") }},
		{name: "transition-no-forge", want: "requires --acceptance-forge",
			after: func(t *testing.T, f protectedRecordFixture) {
				appendRecord(t, f, "Acceptance-Transition: amendment="+f.delivery+" approval=x")
			}},
		{name: "check-mode", want: "requires host execution", extra: []string{"--check"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRecordFixture(t, c.deliver)
			if c.after != nil {
				c.after(t, f)
			}
			before, err := os.ReadFile(f.path)
			if err != nil {
				t.Fatal(err)
			}
			code, diag := f.run(t, c.extra...)
			if code == 0 || !strings.Contains(diag, c.want) {
				t.Fatalf("want refusal %q, got %d %s", c.want, code, diag)
			}
			if after, _ := os.ReadFile(f.path); string(after) != string(before) {
				t.Fatal("a refused protected record wrote Evidence")
			}
		})
	}
}

// regression: #4499
func TestProtectedRecordCredit(t *testing.T) {
	marker := func(f protectedRecordFixture) string {
		return "Verification-Table: delivery=" + f.delivery + " base=" + f.base + " original=" + f.base + "\n"
	}
	cases := []struct {
		name   string
		closes bool
		edit   func(f protectedRecordFixture, body string) string
	}{
		{"as-run", true, nil},
		{"added-promoted", false, func(_ protectedRecordFixture, b string) string {
			return strings.Replace(b, "author-added; execution exit=0; no acceptance credit", "pass exit=0", 1)
		}},
		{"original-demoted", false, func(_ protectedRecordFixture, b string) string {
			return strings.Replace(b, "pass exit=0", "author-added; execution exit=0; no acceptance credit", 1)
		}},
		{"marker-missing", false, func(f protectedRecordFixture, b string) string {
			return strings.Replace(b, marker(f), "", 1)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRecordFixture(t, nil)
			if code, diag := f.run(t); code != 0 {
				t.Fatalf("protected record run: %d %s", code, diag)
			}
			vfAppend(t, f.root, "\n\n"+vfPass)
			if c.edit != nil {
				raw, err := os.ReadFile(f.path)
				if err != nil {
					t.Fatal(err)
				}
				edited := c.edit(f, string(raw))
				if edited == string(raw) {
					t.Fatal("edit did not apply")
				}
				if err := os.WriteFile(f.path, []byte(edited), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			vfCommit(t, f.root, vfMailVerifier)
			if got := f.closes(t); got != c.closes {
				t.Fatalf("closure accepted=%v want=%v", got, c.closes)
			}
		})
	}
}

// editVerify commits a post-delivery edit to the brief's Verify section.
func editVerify(t *testing.T, f protectedRecordFixture, old, next string) {
	t.Helper()
	raw, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.path, []byte(strings.Replace(string(raw), old, next, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, f.root, vfMailHuman)
}

// appendRecord adds a line to the --acceptance-record file.
func appendRecord(t *testing.T, f protectedRecordFixture, line string) {
	t.Helper()
	raw, err := os.ReadFile(f.record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.record, append(raw, []byte(line+"\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
}
