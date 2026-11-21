package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// supersedeFixture plants an older and a newer outcome record for one brief on the fake's
// branch and pins the branch head GetCommit resolves.
func supersedeFixture(t *testing.T) (f *fakeForge, errBuf interface{ String() string }, older, newer, head string) {
	t.Helper()
	ff, buf := setupFake(t)
	oldLine := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
	newLine := completeFail(t, `{"ts":"2026-09-08T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000003"}`)
	var err error
	if older, err = deskkit.RecordName([]byte(oldLine)); err != nil {
		t.Fatal(err)
	}
	if newer, err = deskkit.RecordName([]byte(newLine)); err != nil {
		t.Fatal(err)
	}
	ff.setFile(older, oldLine+"\n")
	ff.setFile(newer, newLine+"\n")
	head = strings.Repeat("c", 40)
	ff.commitSHAs = map[string]string{"main": head}
	ff.commitAuthorLogin = "assay-verifier-app[bot]"
	return ff, buf, older, newer, head
}

// A superseded record is removed by ONE pinned, conditional forge commit as the verifier App:
// the fake records a deletion carrying the file's content id and the branch head it was judged
// on, and the landed commit resolves to the verifier App.
func TestRemoveSupersededLandsPinnedForgeCommit(t *testing.T) {
	f, errBuf, older, newer, head := supersedeFixture(t)
	code := run([]string{"example-org/tracker", "main", "--remove-superseded", older, "--superseded-by", newer})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
	}
	if len(f.writes) != 1 {
		t.Fatalf("writes = %v, want exactly one forge write", f.writes)
	}
	w := f.writes[0]
	if !w.Delete || w.File != older || w.ExpectedHead != head || w.ExpectedSHA != "sha-"+older || len(w.Content) != 0 {
		t.Fatalf("write = %+v, want a pinned conditional removal of %s at %s", w, older, head)
	}
	if !strings.Contains(w.Message, "Superseded-by: "+newer) {
		t.Fatalf("commit message %q does not name the superseding record", w.Message)
	}
	if _, still := f.files[older]; still {
		t.Fatal("the superseded record is still on the fake branch")
	}
	if _, kept := f.files[newer]; !kept {
		t.Fatal("the superseding record was removed")
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "assay-verifier-app[bot]") || !strings.Contains(d, "removed "+older) {
		t.Fatalf("audit detail %q does not record a verifier-attributed removal", d)
	}
}

func TestRemoveSupersededRefusals(t *testing.T) {
	cases := []struct {
		name string
		args func(older, newer string) []string
		want string
	}{
		{"the newer record is named as the one to remove", func(o, n string) []string {
			return []string{"--remove-superseded", n, "--superseded-by", o}
		}, "does not supersede"},
		{"a path outside the outcome records", func(o, n string) []string {
			return []string{"--remove-superseded", "docs/streams/example-stream/14-brief.md", "--superseded-by", n}
		}, "outcome record path"},
		{"the retired shared log", func(o, n string) []string {
			return []string{"--remove-superseded", "docs/streams/verify-outcomes.jsonl", "--superseded-by", n}
		}, "outcome record path"},
		{"a traversal out of the records dir", func(o, n string) []string {
			return []string{"--remove-superseded", deskkit.OutcomeRecordsDir + "/example-stream/../../x/14-a.json", "--superseded-by", n}
		}, "clean repo-relative"},
		{"a record of another brief", func(o, n string) []string {
			return []string{"--remove-superseded", o, "--superseded-by", deskkit.OutcomeRecordsDir + "/example-stream/15-20260909T020000Z-000000000000.json"}
		}, "same brief"},
		{"a superseding record that is not on the branch", func(o, n string) []string {
			return []string{"--remove-superseded", o, "--superseded-by", strings.Replace(n, "20260908", "20260910", 1)}
		}, "is not on the branch"},
		{"no superseding record named", func(o, n string) []string {
			return []string{"--remove-superseded", o}
		}, "given together"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, errBuf, older, newer, _ := supersedeFixture(t)
			code := run(append([]string{"example-org/tracker", "main"}, c.args(older, newer)...))
			if code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, deskkit.ExitRefused, errBuf.String())
			}
			if f.putCalls != 0 {
				t.Fatalf("refusal wrote %d time(s)", f.putCalls)
			}
			if !strings.Contains(errBuf.String(), c.want) {
				t.Fatalf("stderr %q does not name %q", errBuf.String(), c.want)
			}
		})
	}
	t.Run("a record whose bytes do not derive its path", func(t *testing.T) {
		f, errBuf, older, newer, _ := supersedeFixture(t)
		f.setFile(older, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verified","sha":"9999999"}`+"\n")
		code := run([]string{"example-org/tracker", "main", "--remove-superseded", older, "--superseded-by", newer})
		if code != deskkit.ExitRefused || f.putCalls != 0 || !strings.Contains(errBuf.String(), "does not hold the outcome record") {
			t.Fatalf("exit = %d puts = %d stderr %q", code, f.putCalls, errBuf.String())
		}
	})
}
