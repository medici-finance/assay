package main

// verdictbold_test.go — every VERIFY verdict a dispatched agent is TOLD to write is in the
// form the gate READS.
//
// THE DEFECT. The verifier kit asked for "one clear line: `VERIFY: PASS` or `VERIFY: FAIL`" —
// unbolded — while the statusgen gate that advances an item (hasVerifyPass, and every caller
// that shares its regex) reads ONLY the bold marker, `**VERIFY: PASS**`. A verifier that
// followed the kit to the letter produced a real PASS the gate could not see, and the item
// never moved. The writer and the reader of one marker disagreed, and nothing compared them.
//
// THE CLASS. Any instruction text a dispatched agent receives — an embedded kit, or the
// assignment prose prompt.go writes ahead of the kits — that mentions a `VERIFY: PASS` /
// `VERIFY: FAIL` verdict OUTSIDE a match of the ratified bold-marker regex. Each such mention
// is a place an agent can copy an unreadable form from.
//
// THE GUARD. The ratified regex is not copied here: it is READ out of its one home,
// statusgen/verifyissues.go, at test time, so a change to the marker the gate reads is
// immediately a change to what this test demands of the instructions. Then every embedded
// kit (the whole references/ glob, so a kit added later is covered without editing this file)
// and the fully assembled verifier prompt are scanned, and every verdict mention must sit
// inside a bold-marker match. A positive control runs the same checker over planted text so
// a matcher that silently stops matching fails instead of reporting clean.
//
// BOUND. Instruction text only. Evidence bodies already written in older surface forms are
// statusgen's business (lastVerifyVerdict reads them leniently by design), and refusing an
// unbolded verdict at landing time is a separate change to the landing tool.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ratifiedRegexHome is where the gate's bold-marker regex is declared, repo-relative.
const ratifiedRegexHome = "statusgen/verifyissues.go"

// ratifiedRegexDecl captures the raw-string literal of the gate's declaration. Matching the
// declaration line exactly (name included) is deliberate: a rename or a move of the ratified
// regex must re-point this test, never leave it reading something else.
var ratifiedRegexDecl = regexp.MustCompile(
	"(?m)^var verifyVerdictBoldRe = regexp\\.MustCompile\\(`([^`]+)`\\)\\s*$")

// verdictMention is ANY verdict token, however it is dressed — the same shape statusgen's
// lenient reader accepts. Every one of these in instruction text must be inside a bold match.
var verdictMention = regexp.MustCompile(`VERIFY:[ \t]*(PASS|FAIL)`)

// ratifiedVerdictRe loads and compiles the gate's own regex from statusgen.
//
// It walks up by filepath.Dir to the checkout root rather than carrying a relative literal,
// the same way verifyloop's kit coupling test finds its kit. A tree that does not carry
// statusgen at all (a sliced file set shipping only this module) has no gate to couple to and
// SKIPS; a tree that carries statusgen but not the declaration FAILS — the regex moved or was
// renamed, and this test must be re-pointed rather than go quiet.
func ratifiedVerdictRe(t *testing.T) *regexp.Regexp {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	home := filepath.FromSlash(ratifiedRegexHome)
	for {
		if _, err := os.Stat(filepath.Join(dir, home)); err == nil {
			break
		}
		if _, err := os.Stat(filepath.Join(dir, "statusgen", "go.mod")); err == nil {
			t.Fatalf("statusgen is in this tree at %s but %s is not — the ratified VERIFY regex "+
				"moved; re-point ratifiedRegexHome", dir, ratifiedRegexHome)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skipf("%s is not in this tree — no gate to couple the verdict instructions to", ratifiedRegexHome)
		}
		dir = parent
	}
	raw, err := os.ReadFile(filepath.Join(dir, home))
	if err != nil {
		t.Fatalf("read %s: %v", ratifiedRegexHome, err)
	}
	m := ratifiedRegexDecl.FindSubmatch(raw)
	if m == nil {
		t.Fatalf("%s no longer declares verifyVerdictBoldRe as a raw-string regexp.MustCompile — "+
			"the ratified marker moved or changed shape; re-point ratifiedRegexDecl", ratifiedRegexHome)
	}
	re, err := regexp.Compile(string(m[1]))
	if err != nil {
		t.Fatalf("the ratified regex %q from %s does not compile: %v", m[1], ratifiedRegexHome, err)
	}
	// Sanity on what was loaded: it is the BOLD reader, not some other VERIFY regex.
	if !re.MatchString("**VERIFY: PASS**") || re.MatchString("VERIFY: PASS") {
		t.Fatalf("the regex loaded from %s (%q) does not behave as the bold-marker reader — "+
			"it must match **VERIFY: PASS** and must not match a plain VERIFY: PASS", ratifiedRegexHome, m[1])
	}
	return re
}

// unboldVerdicts returns each verdict mention in text that does not sit inside a match of
// bold, as the line it appears on.
func unboldVerdicts(bold *regexp.Regexp, text string) []string {
	covered := bold.FindAllStringIndex(text, -1)
	var bad []string
	for _, loc := range verdictMention.FindAllStringIndex(text, -1) {
		inside := false
		for _, c := range covered {
			if loc[0] >= c[0] && loc[1] <= c[1] {
				inside = true
				break
			}
		}
		if inside {
			continue
		}
		start := strings.LastIndex(text[:loc[0]], "\n") + 1
		end := strings.Index(text[loc[1]:], "\n")
		if end < 0 {
			end = len(text)
		} else {
			end += loc[1]
		}
		bad = append(bad, strings.TrimSpace(text[start:end]))
	}
	return bad
}

// The positive control. The same checker, over text that certainly carries the defect, must
// flag it — and must pass the canonical form, with a qualifier after the asterisks.
func TestVerdictCheckerIsLive(t *testing.T) {
	bold := ratifiedVerdictRe(t)
	if got := unboldVerdicts(bold, "Report one clear line: `VERIFY: PASS` or `VERIFY: FAIL`."); len(got) != 2 {
		t.Fatalf("planted unbolded verdicts not flagged (got %d, want 2) — the checker is broken "+
			"and every clean result below is meaningless", len(got))
	}
	if got := unboldVerdicts(bold, "`**VERIFY: PASS**` or `**VERIFY: FAIL** — row 3: exit 1`"); len(got) != 0 {
		t.Fatalf("the canonical bold form was flagged: %q", got)
	}
}

// Every embedded kit, and the assembled verifier prompt, asks only for the bold form.
func TestVerdictInstructionsAreBold(t *testing.T) {
	bold := ratifiedVerdictRe(t)

	kits, err := fs.Glob(kitFS, "references/*.md")
	if err != nil || len(kits) == 0 {
		t.Fatalf("no embedded kits found (err=%v) — this check would prove nothing", err)
	}
	sources := map[string]string{}
	for _, p := range kits {
		b, err := kitFS.ReadFile(p)
		if err != nil {
			t.Fatalf("read embedded %s: %v", p, err)
		}
		sources[p] = string(b)
	}
	// The assembled prompt carries prompt.go's own assignment prose, which no kit file holds.
	sources["assembled --kit verifier prompt"] = dispatchPrompt(t, "verifier", "verify-item-9")

	for name, text := range sources {
		for _, line := range unboldVerdicts(bold, text) {
			t.Errorf("%s asks for a VERIFY verdict the gate cannot read — every verdict a dispatched "+
				"agent is told to write must be the bold marker (**VERIFY: PASS** / **VERIFY: FAIL**, "+
				"qualifiers after the closing asterisks):\n  %s", name, line)
		}
	}

	// The verifier kit must still SHOW the agent the form, for both verdicts — a kit that
	// dropped the example would pass the scan above vacuously.
	vk := sources[kitFile["verifier"]]
	for _, verdict := range []string{"PASS", "FAIL"} {
		found := false
		for _, m := range bold.FindAllStringSubmatch(vk, -1) {
			if m[1] == verdict {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the verifier kit no longer carries a bold **VERIFY: %s** example the gate's regex "+
				"matches — the agent is not shown the form it must write", verdict)
		}
	}
}
