package main

import (
	"encoding/json"
	"testing"
)

// Fail-first tests for #5529, safe:idiom. The retired-idiom list is swapped for a fixture
// list (the shipped list is empty); everything else goes through gatherRowFacts and Classify
// against a real checkout, in the shape of TestSafeRename.

// toolScript exits 0 only when its first argument is accept. An accept of "!" makes every
// call exit 127, the shell's could-not-run code.
func toolScript(accept string) string {
	if accept == "!" {
		return "#!/bin/sh\nexit 127\n"
	}
	return "#!/bin/sh\n[ \"$1\" = \"" + accept + "\" ] && echo ok && exit 0\nexit 2\n"
}

// withIdioms replaces the usable list with what loadRetiredIdioms keeps of raw.
func withIdioms(t *testing.T, raw string) {
	t.Helper()
	saved := retiredIdioms
	retiredIdioms = loadRetiredIdioms([]byte(raw))
	t.Cleanup(func() { retiredIdioms = saved })
}

const oldFlagList = `{"idioms":[{"idiom":"--old-flag","replacement":"--new-flag","ruling":"#123"}]}`

// idiomCase runs row `sh tool.sh <arg>` (Expect exit 0) against a tool that accepts accept.
func idiomCase(t *testing.T, arg, accept string) (RowFacts, Classification, verifyRow) {
	t.Helper()
	root := gitRepo(t)
	writeFile(t, root, "tool.sh", toolScript(accept))
	gitCommit(t, root, "tool")
	row := verifyRow{Num: 1, Class: "check", Command: "sh tool.sh " + arg, Expect: "exit 0", ExpectCell: "exit 0"}
	f := gatherRowFacts(root, row, false, "")
	return f, Classify(f), row
}

// TestSafeIdiom — the row uses an idiom a listed ruling retired, and the row rewritten to
// the replacement passes its own Expect → safe:idiom, and --open writes that rewrite.
//
// regression: #5529
func TestSafeIdiom(t *testing.T) {
	withIdioms(t, oldFlagList)
	f, got, row := idiomCase(t, "--old-flag", "--new-flag")
	if got.Verdict != SafeIdiom {
		t.Fatalf("listed retired idiom: got %s (%q), want %s\nfacts=%+v", got.Verdict, got.Reason, SafeIdiom, f)
	}
	if f.RetiredIdiom != "--old-flag" || f.IdiomRuling != "#123" {
		t.Fatalf("RetiredIdiom=%q IdiomRuling=%q, want --old-flag and #123", f.RetiredIdiom, f.IdiomRuling)
	}
	cmd, _, changed := computeNewRow("", row, f, got)
	if !changed || cmd != "sh tool.sh --new-flag" {
		t.Fatalf("computeNewRow command = %q (%t), want sh tool.sh --new-flag", cmd, changed)
	}
}

// TestIdiomRefusals — near-misses: an idiom not in the list, a list entry with no ruling
// reference or a malformed one, a longer word that only starts with the idiom, and a
// rewritten row that still fails or cannot run.
//
// regression: #5529
func TestIdiomRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, list, arg, accept string
		wantIdiom               string
	}{
		{"not in list", `{"idioms":[{"idiom":"--other","replacement":"--new-flag","ruling":"#123"}]}`, "--old-flag", "--new-flag", ""},
		{"no ruling", `{"idioms":[{"idiom":"--old-flag","replacement":"--new-flag"}]}`, "--old-flag", "--new-flag", ""},
		{"blank ruling", `{"idioms":[{"idiom":"--old-flag","replacement":"--new-flag","ruling":" "}]}`, "--old-flag", "--new-flag", ""},
		{"prose ruling", `{"idioms":[{"idiom":"--old-flag","replacement":"--new-flag","ruling":"agreed in chat"}]}`, "--old-flag", "--new-flag", ""},
		{"token boundary", oldFlagList, "--old-flag-x", "--new-flag-x", ""},
		{"rewrite fails", oldFlagList, "--old-flag", "--third-flag", "--old-flag"},
		{"rewrite cannot run", oldFlagList, "--old-flag", "!", "--old-flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withIdioms(t, tc.list)
			f, got, _ := idiomCase(t, tc.arg, tc.accept)
			if got.Verdict.IsSafe() {
				t.Fatalf("got %s (%q), want a refusal\nfacts=%+v", got.Verdict, got.Reason, f)
			}
			if f.RetiredIdiom != tc.wantIdiom {
				t.Fatalf("RetiredIdiom=%q, want %q", f.RetiredIdiom, tc.wantIdiom)
			}
		})
	}
}

// TestIdiomListLoads — the checked-in list parses and every usable entry names a ruling;
// a duplicated idiom is dropped entirely.
//
// regression: #5529
func TestIdiomListLoads(t *testing.T) {
	var doc struct {
		Idioms []json.RawMessage `json:"idioms"`
	}
	if err := json.Unmarshal(retiredIdiomsJSON, &doc); err != nil {
		t.Fatalf("retired-idioms.json does not parse: %v", err)
	}
	for _, e := range loadRetiredIdioms(retiredIdiomsJSON) {
		if !rulingRef.MatchString(e.Ruling) {
			t.Fatalf("usable entry %+v has no ruling reference", e)
		}
	}
	dup := `{"idioms":[{"idiom":"--a","replacement":"--b","ruling":"#1"},{"idiom":"--a","replacement":"--c","ruling":"#2"}]}`
	if got := loadRetiredIdioms([]byte(dup)); len(got) != 0 {
		t.Fatalf("duplicated idiom kept: %+v", got)
	}
	if got := loadRetiredIdioms([]byte(oldFlagList)); len(got) != 1 || got[0].Ruling != "#123" {
		t.Fatalf("well-formed entry dropped: %+v", got)
	}
}
