package deskkit

// selfcontain_synthid_test.go — the session-id arm's synthetic-fixture exemption (#2217): a
// UUID whose FIRST group is exactly eight zeros, written into a FILE, is a synthetic fixture
// id, not a session id. The agent tooling mints random v4 session ids, and a minted id has
// an all-zero first group with probability about 2^-32, so the shape is decided by content
// alone, with no path condition. Every other UUID the arm refused it still refuses —
// including a first group that is all zeros but one digit — and every surface that is not
// a file's content (a commit message, a PR body, a comment) gets no exemption at all.
//
// Every UUID below is ASSEMBLED AT RUN TIME (sxUUID), the exempt ones too: the push-path
// check guarding the change that carries this file may run a desk binary built before the
// exemption existed, and the refused ones must never appear as a literal in any file.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// sxUUID assembles a v4-shaped UUID (version nibble 4, variant 8) from a first group and a
// counter for the last group, the shape the brief-event fixtures use.
func sxUUID(head string, n int) string {
	return head + "-" + "0000" + "-" + "4000" + "-" + "8000" + "-" + fmt.Sprintf("%012d", n)
}

var (
	sxZero1 = sxUUID("0000"+"0000", 1) // the exempt shape, last group counting up from 1
	sxZero2 = sxUUID("0000"+"0000", 2)
	// sxRandom is a random-looking v4 id: non-zero first group, version 4, variant b.
	sxRandom = "9f3c" + "2a7e" + "-" + "51b0" + "-" + "4d8a" + "-" + "b6e2" + "-" + "0c7d" + "91e4a3f5"
	sxOneOff = sxUUID("0000"+"0001", 1) // all zeros except one digit
)

// sxNonZero is every first-group shape that must stay refused: a random v4 id, and first
// groups one digit away from all-zero at either end, in a letter digit, and in the middle.
var sxNonZero = []string{
	sxRandom,
	sxOneOff,
	sxUUID("1000"+"0000", 1),
	sxUUID("0000"+"000a", 1),
	sxUUID("0000"+"f000", 1),
}

// sxKinds are the four file kinds the acceptance names, each with a path of that kind and a
// line shape a real file of that kind carries an id in.
var sxKinds = []struct {
	kind, path string
	line       func(id string) string
}{
	{"test file", "internal/events/event_test.go",
		func(id string) string { return "\twant := Event{ID: \"" + id + "\", Kind: \"created\"}" }},
	{"testdata file", "internal/events/testdata/events.jsonl",
		func(id string) string { return `{"event_id":"` + id + `","kind":"created"}` }},
	{"schema", "schemas/brief-event.schema.json",
		func(id string) string { return `      "examples": ["` + id + `"]` }},
	{"spec doc", "spec/brief-event.md",
		func(id string) string { return "Each event carries an `event_id`, for example `" + id + "`." }},
}

// sxFile is a small file holding the given lines, between two neutral lines.
func sxFile(lines ...string) string {
	return "header line\n" + strings.Join(lines, "\n") + "\ntrailer line\n"
}

// TestSynthIDAdmittedInFiles is acceptance 1: an all-zero-first-group v4 fixture id in a test
// file, a testdata file, a schema and a spec doc is NOT refused — one id or several, with or
// without the file's full content supplied (the exemption reads content, never a path or a
// source).
func TestSynthIDAdmittedInFiles(t *testing.T) {
	bfSetup(t)
	for _, k := range sxKinds {
		t.Run(k.kind, func(t *testing.T) {
			for name, text := range map[string]string{
				"one id":       sxFile(k.line(sxZero1)),
				"several ids":  sxFile(k.line(sxZero1), k.line(sxZero2)),
				"two per line": sxFile(k.line(sxZero1) + " " + k.line(sxZero2)),
			} {
				for _, withSrc := range []bool{true, false} {
					if err := bfCheck(t, k.path, text, text, withSrc); err != nil {
						t.Fatalf("%s (source supplied: %v): a synthetic fixture id was REFUSED: %v",
							name, withSrc, err)
					}
				}
			}
		})
	}
}

// TestSynthIDNonZeroStillRefused is acceptance 2: in each of the four file kinds, a
// random-looking v4 id and every first group that is not exactly eight zeros (`00000001-…`
// among them) still refuses — alone, and beside an exempt id, where the arm reports the
// first NON-exempt match.
func TestSynthIDNonZeroStillRefused(t *testing.T) {
	bfSetup(t)
	for _, k := range sxKinds {
		for _, id := range sxNonZero {
			t.Run(k.kind+"/"+id[:8], func(t *testing.T) {
				alone := sxFile(k.line(id))
				bfWantRefused(t, bfCheck(t, k.path, alone, alone, true), id)
				beside := sxFile(k.line(sxZero1), k.line(id), k.line(sxZero2))
				bfWantRefused(t, bfCheck(t, k.path, beside, beside, true), id)
			})
		}
	}
}

// TestSynthIDOtherArmsArmed: the exemption is on the session-id arm only. A file whose only
// UUID is exempt still refuses an agent id, a machine path or a worktree name.
func TestSynthIDOtherArmsArmed(t *testing.T) {
	bfSetup(t)
	for _, c := range []struct{ rule, span string }{
		{"agent-id", "agent-" + "0a1b2c3d" + "4e5f"},
		{"absolute-machine-path", "/Us" + "ers/someone/notes.md"},
		{"scratch-worktree-name", "trac" + "ker-demo-item"},
	} {
		t.Run(c.rule, func(t *testing.T) {
			text := sxFile(`{"event_id":"`+sxZero1+`"}`, "see "+c.span)
			err := bfCheck(t, "internal/events/testdata/events.jsonl", text, text, true)
			if err == nil || !obRuleMatches(err.Error(), RuleSelfContainPrefix+c.rule) ||
				!strings.Contains(err.Error(), c.span) {
				t.Fatalf("want a %s%s refusal naming %q, got %v", RuleSelfContainPrefix, c.rule, c.span, err)
			}
		})
	}
}

// TestSynthIDBodiesStillRefused: the exemption is for a FILE's content. The same synthetic id
// in a PR body, an issue, a comment, a review or a commit message refuses exactly as before.
func TestSynthIDBodiesStillRefused(t *testing.T) {
	bfSetup(t)
	body := "The fixture uses " + sxZero1 + " as its first event id."
	if err := SelfContainCheck("PR body", []byte(body),
		SelfContainOpts{Repo: obPublic, NumberHint: 9000, Notices: &bytes.Buffer{}}); err == nil ||
		!strings.Contains(err.Error(), sxZero1) {
		t.Fatalf("PR body carrying a synthetic id: want a session-id refusal, got %v", err)
	}
	for _, kind := range []string{OutboundKindChange, OutboundKindIssue, OutboundKindComment,
		OutboundKindReview, OutboundKindCommit, OutboundKindRef} {
		t.Run(kind, func(t *testing.T) {
			err := OutboundCheck(OutboundWrite{Role: "worker", Repo: obPublic, Kind: kind,
				Fields: []OutboundField{{Name: "body", Text: body}}})
			bfWantRefused(t, err, sxZero1)
		})
	}
}

// TestSynthIDPushPath runs the exemption on OutboundCheckPush over a real branch diff, which
// reads only ADDED lines and supplies no full content for a non-brief file.
func TestSynthIDPushPath(t *testing.T) {
	bfSetup(t)
	p := newObPushRepo(t)
	push := func(name string, files map[string]string, msg string) error {
		t.Helper()
		p.git("checkout", "-q", "-b", name, "main")
		for _, k := range sxKinds {
			if c, ok := files[k.path]; ok {
				p.write(k.path, c)
				p.git("add", k.path)
			}
		}
		p.git("commit", "-q", "-m", msg)
		p.git("checkout", "-q", "main")
		return OutboundCheckPush(OutboundPush{Dir: p.dir, Repo: obPublic, Base: "main", Head: name,
			Branch: name, Role: "worker"})
	}

	all := map[string]string{}
	for _, k := range sxKinds {
		all[k.path] = sxFile(k.line(sxZero1), k.line(sxZero2))
	}
	if err := push("new-fixtures", all, "add brief-event fixtures"); err != nil {
		t.Fatalf("new files carrying synthetic fixture ids were REFUSED on the push path: %v", err)
	}

	// A MODIFIED file: main carries it; the branch adds one more fixture line.
	td := sxKinds[1]
	p.write(td.path, sxFile(td.line(sxZero1)))
	p.git("add", td.path)
	p.git("commit", "-q", "-m", "seed a fixture file")
	if err := push("add-line", map[string]string{td.path: sxFile(td.line(sxZero1), td.line(sxZero2))},
		"add a fixture line"); err != nil {
		t.Fatalf("adding a synthetic id line to an existing file was REFUSED: %v", err)
	}

	for _, id := range []string{sxRandom, sxOneOff} {
		bfWantRefused(t, push("nonzero-"+id[:8], map[string]string{
			td.path: sxFile(td.line(sxZero1), td.line(id)),
		}, "add a fixture"), id)
	}

	err := push("commit-id", map[string]string{sxKinds[3].path: sxFile("prose")},
		"add a spec\n\nfirst event "+sxZero1)
	bfWantRefused(t, err, sxZero1)
	if !strings.Contains(err.Error(), "(commit write to") {
		t.Fatalf("the commit-message refusal is not a commit write: %v", err)
	}
}

// TestSynthIDForgeWriteFile runs the exemption on the Forge file-write seam.
func TestSynthIDForgeWriteFile(t *testing.T) {
	bfSetup(t)
	write := func(file, content, msg string) (*outboundRecordingForge, error) {
		fake := &outboundRecordingForge{}
		_, err := OutboundChecked(fake, "worker").WriteFile(obRepo(obPublic), WriteFileInput{
			File: file, Branch: "main", Content: []byte(content), Message: msg})
		return fake, err
	}
	for _, k := range sxKinds {
		fake, err := write(k.path, sxFile(k.line(sxZero1)), "add a fixture")
		if err != nil || len(fake.calls) != 1 {
			t.Fatalf("%s: a synthetic id was refused at WriteFile (calls %d): %v", k.kind, len(fake.calls), err)
		}
		_, err = write(k.path, sxFile(k.line(sxOneOff)), "add a fixture")
		bfWantRefused(t, err, sxOneOff)
	}
	_, err := write(sxKinds[0].path, sxFile("prose"), "first event "+sxZero1)
	bfWantRefused(t, err, sxZero1)
}
