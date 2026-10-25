package main

// recordedruling_test.go — #2079. A gate: human brief whose human had already ruled, with the
// ruling transcribed as a decision record, could not be dispatched: the decision gate only ever
// ran the consumer's decision script, which derives a NEW decision issue from the brief. The
// gate now accepts a `**Ruling (<date>): option <N>.**` line linking a record under
// docs/streams/decisions/, provided the record exists and its ruling corroborates through
// `statusgen --corroborate --ruling-record --ruling-brief` as a ruling on THIS brief. Every other
// shape refuses, naming the line.
//
// Nothing here reaches a forge: statusgen is answered by the harness's exec seam.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	rrBriefRel  = "docs/streams/example/brief-01-example.md"
	rrRecordRel = "docs/streams/decisions/DR-example-rec.md"
	rrGoodLine  = "**Ruling (2026-09-30): option 1.** Transcribed as [DR-example-rec](../decisions/DR-example-rec.md)."
)

// rrTree writes a brief whose body is body under root, plus the decision record, and returns the
// brief's absolute path.
func rrTree(t *testing.T, root, body string) string {
	t.Helper()
	brief := filepath.Join(root, filepath.FromSlash(rrBriefRel))
	rec := filepath.Join(root, filepath.FromSlash(rrRecordRel))
	for _, p := range []string{brief, rec} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(rec, []byte("---\nid: DR-example-rec\n---\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brief, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return brief
}

func rrBrief(lines ...string) string {
	return "---\nbrief: example/01\ngate: human\n---\n\n# Example\n\n## Human decision\n\n" +
		strings.Join(lines, "\n") + "\n"
}

// regression: #2079
func TestRecordedRulingParsesLinkedLine(t *testing.T) {
	root := t.TempDir()
	brief := rrTree(t, root, rrBrief("Option 1 or option 2?", "", rrGoodLine))
	r, err := readRecordedRuling(root, brief, rrBriefRel)
	if err != nil {
		t.Fatalf("a well-formed linked Ruling line was refused: %v", err)
	}
	if r == nil {
		t.Fatal("a well-formed linked Ruling line was not recognised as a recorded ruling")
	}
	if r.record != rrRecordRel || r.repoRoot != root || r.lineNo != 12 || r.brief != rrBriefRel {
		t.Fatalf("parsed %+v, want record %s for brief %s in %s at line 12", r, rrRecordRel, rrBriefRel, root)
	}
}

// A brief that records no ruling — or a Ruling line linking no decision record — keeps the gate
// exactly as it was: the script path runs.
func TestRecordedRulingAbsentLeavesGate(t *testing.T) {
	for name, body := range map[string]string{
		"no-ruling":       rrBrief("Option 1 or option 2?"),
		"unlinked-ruling": rrBrief("**Ruling (2026-09-30): option 1.** Answered on the decision issue."),
		"non-record-link": rrBrief("**Ruling (2026-09-30): option 1.** See [the spec](../../spec.md)."),
		"fenced":          rrBrief("```", rrGoodLine, "```"),
		"recorded-label":  rrBrief("**Ruling recorded (2026-09-30):** see [DR](../decisions/DR-example-rec.md)."),
		"relayed-label":   rrBrief("**Ruling relayed from the driver** (2026-09-30): [DR](../decisions/DR-example-rec.md)."),
		"frontmatter":     "---\nnote: \"" + strings.ReplaceAll(rrGoodLine, `"`, ``) + "\"\n---\n# x\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			brief := rrTree(t, root, body)
			r, err := readRecordedRuling(root, brief, rrBriefRel)
			if err != nil || r != nil {
				t.Fatalf("want (nil, nil) — no recorded ruling — got (%+v, %v)", r, err)
			}
		})
	}
}

// regression: #2079
func TestRecordedRulingRefusesBadLine(t *testing.T) {
	for name, tc := range map[string]struct {
		body, want string
		noRecord   bool
	}{
		"malformed-shape":  {body: rrBrief("**Ruling (2026-09-30) option 1** as [DR](../decisions/DR-example-rec.md)."), want: "accepted shape"},
		"bad-date":         {body: rrBrief("**Ruling (2026-13-45): option 1.** [DR](../decisions/DR-example-rec.md)"), want: "calendar date"},
		"missing-record":   {body: rrBrief(rrGoodLine), want: "does not exist", noRecord: true},
		"two-records":      {body: rrBrief("**Ruling (2026-09-30): option 1.** [a](../decisions/DR-example-rec.md) [b](../decisions/DR-example-two.md)"), want: "exactly one"},
		"url-link":         {body: rrBrief("**Ruling (2026-09-30): option 1.** [DR](https://example.com/o/r/blob/main/docs/streams/decisions/DR-example-rec.md)"), want: "by URL"},
		"escapes-checkout": {body: rrBrief("**Ruling (2026-09-30): option 1.** [DR](../../../../decisions/DR-example-rec.md)"), want: "does not resolve"},
		"two-rulings":      {body: rrBrief(rrGoodLine, "", strings.Replace(rrGoodLine, "option 1", "option 2", 1)), want: "more than one ruling"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			brief := rrTree(t, root, tc.body)
			if tc.noRecord {
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(rrRecordRel))); err != nil {
					t.Fatal(err)
				}
			}
			r, err := readRecordedRuling(root, brief, rrBriefRel)
			if err == nil {
				t.Fatalf("a Ruling line this gate cannot accept was accepted: %+v", r)
			}
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
				t.Fatalf("rc = %d, want %d (refused): %v", deskkit.ExitCodeOf(err), deskkit.ExitRefused, err)
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "line 10") {
				t.Fatalf("the refusal must name the line (line 10) and say %q: %v", tc.want, err)
			}
		})
	}
}

// rrDispatch runs a full human-gated dispatch of the recorded-ruling brief with statusgen on the
// (stubbed) PATH answering code, and returns the rc and the harness.
func rrDispatch(t *testing.T, statusgenOnPath bool, code int, out string) (int, *stub) {
	t.Helper()
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	rrTree(t, root, rrBrief(rrGoodLine))
	s.replies = append([]reply{{match: "--ruling-record", stdout: out, stderr: out, code: code}},
		happyReplies(filepath.Join(t.TempDir(), "worker-home"))...)
	lookPath = func(name string) (string, error) {
		if statusgenOnPath && name == statusgenBinary {
			return "/example/bin/statusgen", nil
		}
		return "", exec.ErrNotFound
	}
	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--gate-human", "--brief", rrBriefRel,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	return rc, s
}

// regression: #2079
func TestRecordedRulingDispatches(t *testing.T) {
	rc, s := rrDispatch(t, true, 0, rrRecordRel+": CORROBORATED — example")
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0 — a corroborated recorded ruling must pass the gate", rc)
	}
	if !s.ran("--corroborate --ruling-record " + rrRecordRel) {
		t.Fatalf("the recorded ruling was not corroborated through statusgen: %v", s.calls)
	}
	if s.ran("decision-issue.sh") {
		t.Fatalf("a decision issue was ensured although the ruling is recorded and corroborated: %v", s.calls)
	}
}

// The dispatched brief reaches statusgen with the record, so the ruling is checked against THIS
// brief: a record whose human ruled on a different brief must not open this brief's gate.
// regression: #2079
func TestRecordedRulingPassesBrief(t *testing.T) {
	rc, s := rrDispatch(t, true, 0, rrRecordRel+": CORROBORATED — example")
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}
	if !s.ran("--ruling-record " + rrRecordRel + " --ruling-brief " + rrBriefRel) {
		t.Fatalf("statusgen was not told which brief the ruling must rule on: %v", s.calls)
	}
}

// regression: #2079
func TestRecordedRulingRefusesUncorroborated(t *testing.T) {
	rc, s := rrDispatch(t, true, 1, rrRecordRel+": MISSING-CORROBORATION — ruling wrong-author: example")
	if rc != deskkit.ExitRefused {
		t.Fatalf("dispatch rc = %d, want %d (refused) — a recorded ruling that does not corroborate must refuse", rc, deskkit.ExitRefused)
	}
	if s.ran("decision-issue.sh") {
		t.Fatalf("the gate fell back to the decision script after the ruling failed to corroborate: %v", s.calls)
	}
}

func TestRecordedRulingCouldNotCheck(t *testing.T) {
	rc, s := rrDispatch(t, true, 2, rrRecordRel+": COULD-NOT-CHECK — example")
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("dispatch rc = %d, want %d (unverifiable)", rc, deskkit.ExitUnverifiable)
	}
	if s.ran("decision-issue.sh") {
		t.Fatalf("the gate fell back to the decision script on a could-not-check: %v", s.calls)
	}
}

// With statusgen absent the ruling cannot be corroborated: refuse BEFORE the claim.
func TestRecordedRulingNoStatusgen(t *testing.T) {
	rc, s := rrDispatch(t, false, 0, "")
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("dispatch rc = %d, want %d (unverifiable)", rc, deskkit.ExitUnverifiable)
	}
	if s.ran("acquire") || s.ran("decision-issue.sh") || s.ran("--ruling-record") {
		t.Fatalf("a child ran although the ruling could not be corroborated: %v", s.calls)
	}
}

// A malformed recorded ruling refuses before the claim, naming the line.
func TestRecordedRulingRefusesPreClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	rrTree(t, root, rrBrief("**Ruling (2026-09-30) option 1** as [DR](../decisions/DR-example-rec.md)."))
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	err := cmdDispatch([]string{"item-1", "--root", root, "--repo", allowedRepo, "--gate-human", "--brief", rrBriefRel,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused): %v", deskkit.ExitCodeOf(err), deskkit.ExitRefused, err)
	}
	if !strings.Contains(err.Error(), rrBriefRel+" line 10") {
		t.Errorf("the refusal does not name the line it could not parse: %v", err)
	}
	if s.ran("acquire") || s.ran("decision-issue.sh") {
		t.Fatalf("a child ran although the brief's ruling line was refused: %v", s.calls)
	}
}
