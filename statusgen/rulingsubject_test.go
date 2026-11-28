package main

import (
	"strconv"
	"testing"

	"github.com/medici-finance/assay/statusgen/workref"
)

// Ruling-subject binding through the real decision-gate hold (milestone-support/09,
// design point 4). Each case drives `statusgen --decision-gate` on a throwaway
// repository against the faked forge of decisionruling_test.go.

const (
	subjRegistry = "schema: graph-repos-v1\ncell: example\nself: ex\nrepos:\n" +
		"  ex:        {cell: example, repo: example-org/example-repo}\n" +
		"  elsewhere: {cell: example, repo: example-org/other-repo}\n"
	subjBrief  = "example:ex:sdlc:18"
	subjPerm   = "00000000-0000-4000-8000-000000000018"
	subjMarker = "<!-- decision-gate: " + subjBrief + " -->"
)

func subjFM(version, extra string) string {
	fm := "---\nbrief: " + subjBrief + "\nid: " + subjPerm + "\ntitle: Brief 18\nwave: 1\neffort: M\ngate: human\n"
	if version != "" {
		fm += "version: " + version + "\n"
	}
	return fm + extra + "---\n\n# Brief 18\n"
}

func subjCard(version int) string {
	return `<!-- assay-card: {"schema":"gate-card/v1","purpose":"decision","brief":"` + subjBrief + `","local":"sdlc/18","repository":"` + rlRepo + `","version":` + strconv.Itoa(version) + `} -->`
}

// subjRun moves brief sdlc/18 to implemented with a recorded ruling, against a
// forge whose comment text, creation time and issue body are given.
func subjRun(t *testing.T, version, comment, created, issue string) (int, string) {
	t.Helper()
	dhSeams(t, &rlForge{
		commentStatus: 200,
		commentBody:   rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, comment, created, created),
		issueStatus:   200,
		issueBody:     rlIssueJSON(issue),
	})
	root := dhFixture(t, map[string]string{
		"docs/streams/graph-repos.yaml": subjRegistry,
		dhReadmePath:                    dhReadme(dhRow("18", "todo")),
		dhBriefPath:                     subjFM(version, "decision-issue: 41\n"),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  subjFM(version, dhRuledLines()),
	})
	return dhGate(t, root)
}

// milestone-support/09: a ruling binds the exact work subject the
// human named; an alias, revision or card it did not name never carries it.
// regression: #7088
func TestDecisionGateRulingSubject(t *testing.T) {
	const pre = rlPosted                // before the cutover
	const post = "2026-12-02T10:00:00Z" // after the cutover
	line := func(ref, card string) string {
		return "Option 1 for " + dhBoardID + ".\n\nRuling-Subject: " + ref + " card=" + card + "\n"
	}
	issue := "A human decision is needed.\n\n" + subjMarker + "\n"
	withCard := issue + "\n" + subjCard(1) + "\n"
	cases := []struct {
		name, version, comment, created, issue string
		pass                                   bool
		want                                   string
	}{
		{"typed line, no card", "1", line("example:ex:sdlc:b-18@1", "none"), pre, issue, true, ""},
		{"typed line after cutover", "1", line("sdlc:b-18@1", "none"), post, issue, true, ""},
		{"typed line, matching card", "1", line("example:ex:sdlc:b-18@1", workref.CardDigest(subjCard(1))), pre, withCard, true, ""},
		{"wrong revision", "2", line("example:ex:sdlc:b-18@1", "none"), pre, issue, false, "work-subject"},
		{"absent explicit version", "", line("example:ex:sdlc:b-18@1", "none"), pre, issue, false, "brief-version"},
		{"other root, same stream and number", "1", line("example:elsewhere:sdlc:b-18@1", "none"), pre, issue, false, "work-subject"},
		{"other brief", "1", line("example:ex:sdlc:b-19@1", "none"), pre, issue, false, "work-subject"},
		{"duplicate subject lines", "1", line("example:ex:sdlc:b-18@1", "none") + "Ruling-Subject: example:ex:sdlc:b-18@1 card=none\n", pre, issue, false, "subject-line"},
		{"malformed subject line", "1", "Option 1 for " + dhBoardID + ".\nRuling-Subject: example:ex:sdlc:b-18@1\n", pre, issue, false, "subject-line"},
		{"card added after card=none", "1", line("example:ex:sdlc:b-18@1", "none"), pre, withCard, false, "work-subject-could-not-check"},
		{"card removed, marker remains", "1", line("example:ex:sdlc:b-18@1", workref.CardDigest(subjCard(1))), pre, issue, false, "work-subject-could-not-check"},
		{"card changed to version 2", "2", line("example:ex:sdlc:b-18@2", workref.CardDigest(subjCard(1))), pre, issue + "\n" + subjCard(2) + "\n", false, "work-subject-could-not-check"},
		{"marker of another cell", "1", line("example:ex:sdlc:b-18@1", "none"), pre, "A human decision is needed.\n\n<!-- decision-gate: other:ex:sdlc:18 -->\n", false, "work-subject"},
		{"no line, before cutover", "", "Option 1 for " + dhBoardID + ".", pre, issue, true, ""},
		{"no line, after cutover", "", "Option 1 for " + dhBoardID + ".", post, issue, false, "cutover"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out := subjRun(t, tc.version, tc.comment, tc.created, tc.issue)
			if tc.pass {
				if code != 0 {
					t.Fatalf("exit %d, want 0:\n%s", code, out)
				}
				wantContains(t, "--decision-gate report", out, dhBoardID+" RULED")
				return
			}
			if code == 0 {
				t.Fatalf("exit 0, want a refusal:\n%s", out)
			}
			wantContains(t, "--decision-gate report", out, dhBoardID+" REFUSED", tc.want)
		})
	}
}
