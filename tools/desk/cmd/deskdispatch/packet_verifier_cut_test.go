package main

// packet_verifier_cut_test.go — the verifier packet finds a brief's headings at least as
// widely as the repository's own readers do, and never says a brief records no result.
//
// THE RISK. The packet carries a brief up to its Evidence heading and leaves the rest out.
// If the tool looks for that heading more narrowly than the tools that READ the section —
// they take the white space off a line before they compare it — then a brief whose heading
// is indented by one space keeps its Evidence section in the packet, observed rows and
// verdict line included, under a tool sentence saying there is none. The same holds for the
// Verify heading (the result-column check would never run) and for the heading that ends
// the Verify section.
//
// THE ORACLE. There is no shared function to call: the readers live in another module and
// in other commands' main packages. So their tests of a line are restated below, each with
// the place it was read from, and every line any of them accepts must cut the packet.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/packet"
)

// vpRender returns one section's content as the shared builder writes it.
func vpRender(t *testing.T, c packet.Content) string {
	t.Helper()
	p, err := packet.Build(packet.Spec{Head: "0123abcd", Token: "t0ken", Sections: []packet.Section{
		packet.NewSection("One section", func() (packet.Content, error) { return c, nil })}})
	if err != nil {
		t.Fatal(err)
	}
	return p.Text
}

// vpEvidenceReaders are the tests the repository's own tools apply to one line to decide it
// is the Evidence heading.
var vpEvidenceReaders = []struct {
	where string
	finds func(line string) bool
}{
	// statusgen: brieffile.go, verifyrun.go, evidenceactor.go, transcribeverdict.go; and
	// tools/desk: cmd/verifyloop/frontmatter.go.
	{"the line with its white space off is the heading", func(l string) bool { return strings.TrimSpace(l) == "## Evidence" }},
	// tools/desk: internal/deskkit/verifierattestation.go.
	{"a level-two mark, then a title that trims to the word", func(l string) bool {
		return strings.HasPrefix(l, "## ") && strings.TrimSpace(strings.TrimPrefix(l, "## ")) == "Evidence"
	}},
	// statusgen: gitinfo.go.
	{"the line less a carriage return is the heading", func(l string) bool { return strings.TrimRight(l, "\r") == "## Evidence" }},
}

// vpVerifyReaders are the same for the Verify heading.
var vpVerifyReaders = []struct {
	where string
	finds func(line string) bool
}{
	// statusgen: verifyissues.go (the section lifted by prefix).
	{"trimmed, a level-two mark, then a title that begins with the word", func(l string) bool {
		rest, ok := strings.CutPrefix(strings.TrimSpace(l), "## ")
		return ok && strings.HasPrefix(rest, "Verify")
	}},
	// tools/desk: cmd/verifyloop/frontmatter.go and cmd/deskrebaseline/brief.go.
	{"trimmed, the heading alone or with a qualifier", func(l string) bool {
		t := strings.TrimSpace(l)
		return t == "## Verify" || strings.HasPrefix(t, "## Verify ") || strings.HasPrefix(t, "## Verify(")
	}},
	// tools/desk: cmd/deskpathguard/rederive.go.
	{"trimmed, the heading in any letter case", func(l string) bool { return strings.EqualFold(strings.TrimSpace(l), "## Verify") }},
	// tools/desk: internal/testledger/ledger.go.
	{"the untrimmed line begins with the heading", func(l string) bool { return strings.HasPrefix(l, "## Verify") }},
}

const vpCutTable = "| # | Class | Command | Expect |\n|---|---|---|---|\n| 1 | unit | `true` | ok |\n\n"

// TestVerifierPacketCutsWhereverARepositoryReaderFindsEvidence: a line any reader takes for
// the Evidence heading cuts the packet, wherever it sits and whatever stands between it and
// the Verify section.
func TestVerifierPacketCutsWhereverARepositoryReaderFindsEvidence(t *testing.T) {
	for _, line := range []string{
		"## Evidence",
		" ## Evidence",      // one leading space
		"   ## Evidence",    // three: still a heading in Markdown
		"\t## Evidence",     // a tab
		" ## Evidence",      // a no-break space
		"\u0085## Evidence", // a next-line character
		"## Evidence  ",     // trailing white space
		"##   Evidence",     // a wide gap after the mark
		"##   Evidence \t",  // both
		"  ## Evidence   ",  // mixed
	} {
		found := false
		for _, r := range vpEvidenceReaders {
			found = found || r.finds(line)
		}
		if !found {
			t.Fatalf("control: no repository reader takes %q for the Evidence heading", line)
		}
		doc := parseVerifierBrief("intro KEEP\n\n## Verify\n\n" + vpCutTable + "## Notes\n\nNOTES-KEPT\n\n" +
			line + "\n\nDROP-SENTINEL an earlier run observed this\n")
		if !doc.hasEv {
			t.Errorf("%q: a repository reader finds the Evidence heading and the packet does not", line)
		}
		if strings.Contains(doc.before, "DROP-SENTINEL") {
			t.Errorf("%q: the carried text holds the Evidence section: %q", line, doc.before)
		}
		if !strings.Contains(doc.before, "NOTES-KEPT") {
			t.Errorf("%q: the carried text lost the section ahead of the heading: %q", line, doc.before)
		}
	}
}

// TestVerifierPacketFileNeverCarriesAnIndentedEvidenceSection is the same at the level of
// the file a verifier is handed: the fixture's Evidence section, with a section between it
// and the Verify table and its heading indented, and every sentinel a result could leak by.
func TestVerifierPacketFileNeverCarriesAnIndentedEvidenceSection(t *testing.T) {
	for _, tc := range []struct{ name, indent string }{
		{"one space", " "}, {"three spaces", "   "}, {"a tab", "\t"}, {"a no-break space", " "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := vpRepo(t, vpBriefBefore+"## Notes\n\nNOTES-MARKER nothing more to do.\n\n"+tc.indent+vpBriefAfter)
			text := vpBuild(t, vpInput(root))
			for _, s := range vpSentinels {
				if strings.Contains(text, s) {
					t.Errorf("the packet carries %q from the Evidence section", s)
				}
			}
			if !strings.Contains(text, "NOTES-MARKER") {
				t.Errorf("control: the section ahead of the Evidence heading is not in the packet")
			}
			if !strings.Contains(text, "The brief has an Evidence section at this commit") {
				t.Errorf("the packet does not say the brief has an Evidence section:\n%s", text)
			}
		})
	}
}

// TestVerifierPacketDeclinesAResultColumnUnderAnyVerifyHeadingAReaderFinds: the check that
// refuses a table with an observed-results column runs for every shape of Verify heading a
// repository reader accepts, not only the unindented one.
func TestVerifierPacketDeclinesAResultColumnUnderAnyVerifyHeadingAReaderFinds(t *testing.T) {
	for _, line := range []string{
		"## Verify",
		" ## Verify (executable — no prose-only items)",
		"   ## Verify",
		"\t## Verify",
		" ## Verify (executable)",
		"## Verify(executable)",
		"## Verifying the work",
		"## verify",
		"## Verify  ",
	} {
		found := false
		for _, r := range vpVerifyReaders {
			found = found || r.finds(line)
		}
		if !found {
			t.Fatalf("control: no repository reader takes %q for the Verify heading", line)
		}
		doc := parseVerifierBrief("intro\n\n" + line + "\n\n| # | Command | Expect | Observed |\n|---|---|---|---|\n" +
			"| 1 | `true` | ok | exit 0 OBSERVED-SENTINEL |\n\n## Evidence\n\nlater\n")
		if !doc.hasVerify {
			t.Errorf("%q: a repository reader finds the Verify heading and the packet does not", line)
		}
		if doc.resultColumn() == "" {
			t.Errorf("%q: a table with an Observed column under this heading is not declined", line)
		}
	}

	// And through the provider: no packet at all.
	root := vpRepo(t, strings.Replace(vpBriefBefore, "## Verify (executable", " ## Verify (executable", 1)+
		"| # | Command | Expect | Observed |\n|---|---|---|---|\n| 9 | `true` | ok | exit 0 OBSERVED-SENTINEL |\n\n"+vpBriefAfter)
	if _, err := verifierPacket(vpInput(root)); err == nil || !strings.Contains(err.Error(), "result-like column") {
		t.Errorf("verifierPacket over an indented Verify heading with an Observed column: err = %v, want a decline", err)
	}
}

// TestVerifierPacketVerifySectionEndsWhereTheReadersEndIt: the readers end the Verify
// section at the next level-two heading with its white space off, so a table under an
// indented heading that follows is not a Verify table and its rows are not Verify rows.
func TestVerifierPacketVerifySectionEndsWhereTheReadersEndIt(t *testing.T) {
	for _, next := range []string{"## Notes", " ## Notes", "   ## Notes", "\t## Notes", " ## Notes"} {
		doc := parseVerifierBrief("## Verify\n\n" + vpCutTable + next + "\n\n" +
			"| # | Command | Why |\n|---|---|---|\n| 7 | `not a verify row` | n |\n| 8 | `nor this` | n |\n\n## Evidence\n")
		got := vpRender(t, doc.rowCommands())
		if !strings.Contains(got, "has 1 row(s)") || strings.Contains(got, "not a verify row") {
			t.Errorf("after %q the row list is not the Verify section's one row:\n%s", next, got)
		}
	}
	// A line that only looks like a level-one heading (a shell comment in an example) does
	// not end the section for the readers, so it does not end it here.
	doc := parseVerifierBrief("## Verify\n\n```sh\n# build first\n```\n\n" + vpCutTable + "## Evidence\n")
	if got := vpRender(t, doc.rowCommands()); !strings.Contains(got, "has 1 row(s)") {
		t.Errorf("a comment line inside the Verify section ended it:\n%s", got)
	}
}

// TestVerifierPacketNeverSaysABriefRecordsNoResult: the tool cuts at a heading; it does not
// read the text it carries for meaning. So where it found no Evidence heading it says that
// it found none and could not determine whether the carried text records an earlier result
// — never that there is none — and at no point does the file claim to hold no result.
func TestVerifierPacketNeverSaysABriefRecordsNoResult(t *testing.T) {
	const recorded = "## Run log\n\nRUNLOG-MARKER **VERIFY: PASS** — 3 of 3 rows, exit 0.\n"
	for _, tc := range []struct {
		name, brief string
		cut         bool
	}{
		{"a heading the tool does not take for Evidence", vpBriefBefore + recorded, false},
		{"no later section at all", vpBriefBefore + "## Out of scope\n\nNothing else.\n", false},
		{"an Evidence heading", vpBriefBefore + vpBriefAfter, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := vpBuild(t, vpInput(vpRepo(t, tc.brief)))
			tool, _ := vpToolLines(t, text)
			own := strings.Join(tool, "\n")
			for _, never := range []string{"holds no result", "has no Evidence heading", "no result here"} {
				if strings.Contains(own, never) {
					t.Errorf("the tool's own text says %q, which it cannot know:\n%s", never, own)
				}
			}
			if !strings.Contains(own, "nothing in this file is a result of this run") {
				t.Errorf("the tool's own text no longer says what the file is not:\n%s", own)
			}
			const cannotTell = "could not determine whether"
			if tc.cut && strings.Contains(own, cannotTell) {
				t.Errorf("with an Evidence heading found, the packet still says it could not tell:\n%s", own)
			}
			if !tc.cut && strings.Count(own, cannotTell) < 2 {
				t.Errorf("with no Evidence heading found, the packet must say %q in its first section and in "+
					"its Evidence section:\n%s", cannotTell, own)
			}
			if tc.cut != strings.Contains(own, "from its first Evidence heading to its end") {
				t.Errorf("the first section's statement of what was cut is wrong for this brief:\n%s", own)
			}
		})
	}
}

// TestVerifierPacketEvidenceTitleShapes: a numbered, emphasised or code-marked Evidence
// title cuts too. Cutting early costs a read; cutting late could carry results.
func TestVerifierPacketEvidenceTitleShapes(t *testing.T) {
	for _, line := range []string{
		"## 5. Evidence", "## 5) Evidence", "## **Evidence**", "## `Evidence`", "## _Evidence_ (run 2)",
		"## Evidence:", "###\tEVIDENCE", "## 5. **Evidence**",
	} {
		doc := parseVerifierBrief("KEEP\n\n" + line + "\n\nDROP\n")
		if !doc.hasEv || strings.Contains(doc.before, "DROP") {
			t.Errorf("%q does not cut the packet: hasEv=%v before=%q", line, doc.hasEv, doc.before)
		}
	}
	for _, line := range []string{
		"# Brief 3 — Evidence rules", "## What counts as evidence", "## Evidenced claims", "#Evidence", "Evidence",
	} {
		if doc := parseVerifierBrief("KEEP\n\n" + line + "\n\nALSO-KEPT\n"); doc.hasEv {
			t.Errorf("control: %q cut the packet, and it is not an Evidence heading", line)
		}
	}
}

// TestVerifierPacketDeclinesAResultColumnAnywhereInTheCarriedText: the check covers every
// table in the text the packet would carry — a table outside the Verify section, or under
// a second Verify heading, holds observed results just as well as one inside the first.
func TestVerifierPacketDeclinesAResultColumnAnywhereInTheCarriedText(t *testing.T) {
	for _, tc := range []struct{ name, extra string }{
		{"a table in a later section", "## Mutations\n\n| # | Mutation | Result |\n|---|---|---|\n| 1 | drop the guard | red OBSERVED-SENTINEL |\n\n"},
		{"a second Verify section", "## Notes\n\nn\n\n## Verify (re-run)\n\n| # | Command | Observed |\n|---|---|---|\n| 1 | `true` | exit 0 OBSERVED-SENTINEL |\n\n"},
		{"a table ahead of the Verify section", ""},
	} {
		brief := vpBriefBefore + tc.extra + vpBriefAfter
		if tc.extra == "" {
			brief = strings.Replace(vpBriefBefore, "## Verify (executable",
				"| Check | Status |\n|---|---|\n| earlier | passed OBSERVED-SENTINEL |\n\n## Verify (executable", 1) + vpBriefAfter
		}
		if _, err := verifierPacket(vpInput(vpRepo(t, brief))); err == nil || !strings.Contains(err.Error(), "result-like column") {
			t.Errorf("%s: err = %v, want the provider to decline", tc.name, err)
		}
	}
	// Control: the fixture's own tables, none with a result-like column, still build.
	if _, err := verifierPacket(vpInput(vpRepo(t, vpBriefBefore+vpBriefAfter))); err != nil {
		t.Errorf("control: the plain fixture is declined: %v", err)
	}
}
