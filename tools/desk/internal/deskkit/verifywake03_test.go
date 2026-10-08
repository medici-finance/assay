package deskkit

// verifywake03_test.go — verify-reset/03: the receipt writer (BuildOutcomeRecord) and the two
// independent hold reads (Unchanged, ReadBlocker).

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// fakeTree is an OutcomeTree over an in-memory map of repo-relative files at one commit.
type fakeTree struct {
	files map[string]string
	fail  bool // every read errors: the commit cannot be read
}

func (f fakeTree) PathKind(rel string) (string, error) {
	if f.fail {
		return "", errors.New("bad object")
	}
	if _, ok := f.files[rel]; ok {
		return "blob", nil
	}
	for p := range f.files {
		if strings.HasPrefix(p, rel+"/") {
			return "tree", nil
		}
	}
	return "", nil
}

func (f fakeTree) ReadFile(rel string) ([]byte, error) {
	if f.fail {
		return nil, errors.New("bad object")
	}
	b, ok := f.files[rel]
	if !ok {
		return nil, errors.New("does not exist")
	}
	return []byte(b), nil
}

func (f fakeTree) ListFiles(rel string) ([]string, error) {
	var out []string
	for p := range f.files {
		if strings.HasPrefix(p, rel+"/") {
			out = append(out, p)
		}
	}
	return out, nil
}

const wakeTestBriefPath = "docs/streams/demo/brief-01-thing.md"

const wakeTestBrief = "# 01\n\n## Context\n\nfiles:\n" +
	"- `tools/a.go` (edit)\n- `tools/pkg/` (new dir)\n- `tools/planned.go` (planned, absent)\n" +
	"- `docs/<slug>.md` (placeholder)\n\nfacts:\n- `tools/not-declared.go` is outside the block\n"

func wakeTestTree() fakeTree {
	return fakeTree{files: map[string]string{
		wakeTestBriefPath:       wakeTestBrief,
		"tools/a.go":            "package a\n",
		"tools/pkg/one.go":      "package pkg\n",
		"tools/pkg/sub/two.go":  "package sub\n",
		"tools/not-declared.go": "package x\n",
	}}
}

func wakeTestRecord(outcome, kind, ref string) []byte {
	m := map[string]any{
		"ts": "2026-10-08T10:00:00Z", "brief": "assay:at:demo:01", "outcome": outcome,
		"sha": "0123456789abcdef0123456789abcdef01234567", "verifier": "assay-verifier-app[bot]",
		"rows": []int{1, 2}, "extra_key": "kept",
	}
	if kind != "" {
		m["blocker_kind"] = kind
	}
	if ref != "" {
		m["blocker_ref"] = ref
	}
	b, _ := json.Marshal(m)
	return b
}

func wakeTestInput() OutcomeReceiptInput {
	return OutcomeReceiptInput{BriefPath: wakeTestBriefPath, LandedBrief: []byte(wakeTestBrief + "\nlanded\n"), ToolVersion: "v9.9.9"}
}

// Verify row 1: a verify-fail record carries a complete receipt with one file: key per declared
// path (every file under a declared directory; absent paths skipped) plus the brief and tool
// keys; a verified record carries none.
func TestOutcomeRecordWritesReceipt(t *testing.T) {
	out, err := BuildOutcomeRecord(wakeTestRecord("verify-fail", BlockerImplementation, "#42"), wakeTestInput(), wakeTestTree())
	if err != nil {
		t.Fatalf("verify-fail record refused: %v", err)
	}
	var r WakeReceipt
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	if !r.Complete() {
		t.Fatalf("receipt is not complete: %s", out)
	}
	if r.WakePredicate != WakeRelevantInputChanged || r.Schema != SchemaWakeV1 || r.ToolVersion != "v9.9.9" {
		t.Errorf("predicate/schema/tool_version = %q/%q/%q", r.WakePredicate, r.Schema, r.ToolVersion)
	}
	want := map[string]string{
		"file:tools/a.go":           Sha256Hex([]byte("package a\n")),
		"file:tools/pkg/one.go":     Sha256Hex([]byte("package pkg\n")),
		"file:tools/pkg/sub/two.go": Sha256Hex([]byte("package sub\n")),
		"file:" + wakeTestBriefPath: Sha256Hex(wakeTestInput().LandedBrief),
		"tool":                      "v9.9.9",
	}
	if len(r.Inputs) != len(want) {
		t.Errorf("inputs has %d keys, want %d: %v", len(r.Inputs), len(want), r.Inputs)
	}
	for k, v := range want {
		if r.Inputs[k] != v {
			t.Errorf("inputs[%q] = %q, want %q", k, r.Inputs[k], v)
		}
	}
	if !strings.Contains(string(out), `"extra_key":"kept"`) {
		t.Errorf("an unrelated key did not survive: %s", out)
	}
	again, _ := BuildOutcomeRecord(wakeTestRecord("verify-fail", BlockerImplementation, "#42"), wakeTestInput(), wakeTestTree())
	if string(again) != string(out) {
		t.Errorf("the writer is not deterministic")
	}

	pass := wakeTestRecord("verified", "", "")
	got, err := BuildOutcomeRecord(pass, wakeTestInput(), wakeTestTree())
	if err != nil {
		t.Fatalf("verified record refused: %v", err)
	}
	if string(got) != string(pass) || strings.Contains(string(got), "wake_schema") || strings.Contains(string(got), "inputs") {
		t.Errorf("a verified record must carry no receipt: %s", got)
	}
}

// Verify row 2: a placeholder blocker_ref is refused (exit 5), naming blocker_ref.
func TestOutcomeRecordRefusesPlaceholderBlockerRef(t *testing.T) {
	for _, ref := range []string{"to file", "", "action: owner to rule", "tracker#100"} {
		_, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerHumanAction, ref), wakeTestInput(), wakeTestTree())
		if err == nil {
			t.Errorf("blocker_ref %q: accepted, want refused", ref)
			continue
		}
		if ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "blocker_ref") {
			t.Errorf("blocker_ref %q: exit %d %q, want exit %d naming blocker_ref", ref, ExitCodeOf(err), err, ExitRefused)
		}
	}
}

func TestOutcomeRecordRefusesGaps(t *testing.T) {
	cases := []struct {
		name  string
		rec   []byte
		in    OutcomeReceiptInput
		tree  OutcomeTree
		exit  int
		field string
	}{
		{"unknown kind", wakeTestRecord("verify-fail", "flaky", "#1"), wakeTestInput(), wakeTestTree(), ExitRefused, "blocker_kind"},
		{"no kind", wakeTestRecord("verify-fail", "", "#1"), wakeTestInput(), wakeTestTree(), ExitRefused, "blocker_kind"},
		{"no tool", wakeTestRecord("verify-fail", BlockerUnknown, "#1"), OutcomeReceiptInput{BriefPath: wakeTestBriefPath, LandedBrief: []byte("x")}, wakeTestTree(), ExitRefused, "tool_version"},
		{"no brief", wakeTestRecord("verify-fail", BlockerUnknown, "#1"), OutcomeReceiptInput{ToolVersion: "dev"}, wakeTestTree(), ExitRefused, "brief"},
		{"unreadable sha", wakeTestRecord("verify-fail", BlockerUnknown, "#1"), wakeTestInput(), fakeTree{fail: true}, ExitUnverifiable, "inputs"},
	}
	for _, c := range cases {
		_, err := BuildOutcomeRecord(c.rec, c.in, c.tree)
		if err == nil || ExitCodeOf(err) != c.exit || !strings.Contains(err.Error(), c.field) {
			t.Errorf("%s: got %v (exit %d), want exit %d naming %q", c.name, err, ExitCodeOf(err), c.exit, c.field)
		}
	}
}

func TestUnchangedThreeState(t *testing.T) {
	r := WakeReceipt{Inputs: map[string]string{"file:a": "h1", "file:b": "h2"}}
	if s, why := Unchanged(r, staticRevs{"file:a": "h1", "file:b": "h2"}); s != InputsUnchanged {
		t.Errorf("all equal: %v %q", s, why)
	}
	if s, why := Unchanged(r, staticRevs{"file:a": "h1", "file:b": "zz"}); s != InputsChanged || why != "changed b" {
		t.Errorf("one changed: %v %q", s, why)
	}
	if s, _ := Unchanged(r, staticRevs{"file:a": "h1"}); s != InputsCouldNotCheck {
		t.Errorf("one unreadable: %v", s)
	}
	if s, _ := Unchanged(r, staticRevs{"file:a": "zz"}); s != InputsChanged {
		t.Errorf("a change outranks an unreadable sibling: %v", s)
	}
	if s, _ := Unchanged(WakeReceipt{}, staticRevs{}); s != InputsCouldNotCheck {
		t.Errorf("empty scope: %v", s)
	}
	if s, _ := Unchanged(r, nil); s != InputsCouldNotCheck {
		t.Errorf("nil tree: %v", s)
	}
}

type staticRevs map[string]string

func (s staticRevs) Revision(in string) (string, bool)   { v, ok := s[in]; return v, ok }
func (s staticRevs) ActionCompleted(string) (bool, bool) { return false, false }

type staticIssues map[string]BlockerState

func (s staticIssues) IssueState(ref BlockerRef) (BlockerState, string) {
	st, ok := s[ref.Owner+"/"+ref.Name+"#"+itoa(ref.Number)]
	if !ok {
		return BlockerCouldNotCheck, "unknown"
	}
	return st, ""
}

func TestReadBlockerThreeState(t *testing.T) {
	src := staticIssues{"o/r#1": BlockerOpen, "o/r#2": BlockerClosed, "x/y#3": BlockerOpen}
	cases := []struct {
		raw, def string
		src      IssueStateSource
		want     BlockerState
	}{
		{"#1", "o/r", src, BlockerOpen},
		{"#2", "o/r", src, BlockerClosed},
		{"x/y#3", "o/r", src, BlockerOpen},
		{"https://github.com/o/r/issues/2", "", src, BlockerClosed},
		{"#9", "o/r", src, BlockerCouldNotCheck},                                 // the source cannot say
		{"#1", "", src, BlockerCouldNotCheck},                                    // bare ref, no repo
		{"#1", "o/r", nil, BlockerCouldNotCheck},                                 // no source (--no-forge)
		{"to file", "o/r", src, BlockerCouldNotCheck},                            // not a reference
		{"https://github.com/o/r/actions/runs/7", "", src, BlockerCouldNotCheck}, // a run
	}
	for _, c := range cases {
		if got, why := ReadBlocker(c.raw, c.def, c.src); got != c.want {
			t.Errorf("ReadBlocker(%q, %q) = %v (%s), want %v", c.raw, c.def, got, why, c.want)
		}
	}
}
