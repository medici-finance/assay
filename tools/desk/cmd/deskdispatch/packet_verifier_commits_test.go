package main

// packet_verifier_commits_test.go — the commit list says every place it stops.
//
// THE RISK. The list holds the newest few commits that changed or name the brief. On a brief
// verified more than once those are the Evidence landings, and the commit that delivered the
// work is older. A list that ends at its limit and says nothing reads as "these are all of
// them"; so does a list built without searching any message when its heading says messages
// were searched.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

const vpCommitsSection = " (section \"Commits that changed or name this brief\") — size unknown — "

var vpListedRe = regexp.MustCompile("(?m)^- `[0-9a-f]{40}`: ")

// vpLandings adds n commits that each change the brief's Evidence section, as a re-verify
// landing does. Their messages name no brief.
func vpLandings(t *testing.T, root string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		vpWrite(t, root, vpBriefRel, vpBriefBefore+vpBriefAfter+fmt.Sprintf("\nLanding %d.\n", i))
		vpGit(t, root, "add", ".")
		vpGit(t, root, "commit", "-q", "-m", fmt.Sprintf("verify: landing %d", i))
	}
}

// vpOmissionLines counts the two lines the shared builder writes for one omission — in the
// list at the top and in its section — and fails on a line that names it with other words.
func vpOmissionLines(t *testing.T, text, name, reason string) (listed, inSection int) {
	t.Helper()
	for _, l := range strings.Split(text, "\n") {
		switch l {
		case "- `" + name + "`" + vpCommitsSection + reason:
			listed++
		case "_Omitted: `" + name + "`" + vpCommitsSection + reason + "._":
			inSection++
		}
	}
	return listed, inSection
}

// TestVerifierPacketSaysHowManyCommitsItLeftOut: a brief with more commits than the list
// holds. The fixture's three commits (authored, delivered, verified) are followed by nine
// landings, so twelve commits change or name the brief, six are listed and six are not —
// the delivering commit among them. The packet says how many it left out and of what.
func TestVerifierPacketSaysHowManyCommitsItLeftOut(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	delivered := vpGit(t, root, "rev-parse", "HEAD~1")
	vpLandings(t, root, 9)
	text := vpBuild(t, vpInput(root))

	if got := len(vpListedRe.FindAllString(text, -1)); got != verifierPacketCommits {
		t.Fatalf("control: %d commits are listed, want the limit of %d:\n%s", got, verifierPacketCommits, text)
	}
	if strings.Contains(text, delivered) {
		t.Fatalf("control: the delivering commit is listed, so this fixture does not cut it:\n%s", text)
	}
	name := "6 older commits that changed or name the brief"
	reason := fmt.Sprintf("the list holds only the newest %d", verifierPacketCommits)
	if listed, inSection := vpOmissionLines(t, text, name, reason); listed != 1 || inSection != 1 {
		t.Errorf("the packet lists %d and its section %d omission line(s) for the commits past the limit, want one of each "+
			"naming %q:\n%s", listed, inSection, name, text)
	}

	// Control: a list that holds every such commit reports no cut.
	root = vpRepo(t, vpBriefBefore+vpBriefAfter)
	vpLandings(t, root, verifierPacketCommits-3)
	text = vpBuild(t, vpInput(root))
	if got := len(vpListedRe.FindAllString(text, -1)); got != verifierPacketCommits {
		t.Fatalf("control: %d commits are listed, want exactly %d:\n%s", got, verifierPacketCommits, text)
	}
	if strings.Contains(text, "older commit") {
		t.Errorf("control: a list that holds every commit reports older ones left out:\n%s", text)
	}
}

// TestVerifierPacketCountsPastTheLimitOnlyAsFarAsItSearched: when the search stops before
// the commit that added the brief, the count of commits left out is a floor, and the
// packet says "at least" beside the stop it already reports.
func TestVerifierPacketCountsPastTheLimitOnlyAsFarAsItSearched(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	vpLandings(t, root, 9)
	head := vpGit(t, root, "rev-parse", "HEAD")
	repo, _, err := vpOpenAtHead(root)
	if err != nil {
		t.Fatal(err)
	}
	// Eight commits searched: six listed, two more counted, four never reached.
	c, err := verifierPacketCommitsSection(repo, head, vpBriefRel, "example-stream/07", 8)
	if err != nil {
		t.Fatal(err)
	}
	text := vpRender(t, c)
	for _, want := range []string{
		"`at least 2 older commits that changed or name the brief`",
		fmt.Sprintf("the list holds only the newest %d", verifierPacketCommits),
		"`commits older than the 8 searched`",
		"the history search stops after 8 commits",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the section does not say %q:\n%s", want, text)
		}
	}
}

// TestVerifierPacketSaysWhenNoMessageWasSearched: an item key that yields no brief id. No
// commit message can be matched, so the packet says none was searched and does not describe
// its list as holding commits "whose message names the brief".
func TestVerifierPacketSaysWhenNoMessageWasSearched(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	delivered := vpGit(t, root, "rev-parse", "HEAD~1")
	for _, item := range []string{"assay:at:example-stream:07", "07", "example-stream/07`x"} {
		t.Run(item, func(t *testing.T) {
			in := vpInput(root)
			in.o.item = item
			text := vpBuild(t, in)
			if strings.Contains(text, "- `"+delivered+"`") {
				t.Fatalf("control: the commit that only NAMES the brief is listed, so a message was searched:\n%s", text)
			}
			if !strings.Contains(text, "No commit message was searched") {
				t.Errorf("the packet does not say that no commit message was searched:\n%s", text)
			}
			if strings.Contains(text, "whose message names the brief") {
				t.Errorf("the packet describes a search of messages it did not make:\n%s", text)
			}
			tool, _ := vpToolLines(t, text)
			if strings.Contains(strings.Join(tool, "\n"), "`x") {
				t.Errorf("an item key that is not a brief id reached a tool line:\n%s", text)
			}
		})
	}

	// Control: with a brief id the delivering commit is listed and messages are named.
	text := vpBuild(t, vpInput(root))
	if !strings.Contains(text, "- `"+delivered+"`: its message has a `Brief: example-stream/07` line") {
		t.Errorf("control: the delivering commit is not listed for a well-formed id:\n%s", text)
	}
	if strings.Contains(text, "No commit message was searched") {
		t.Errorf("control: a search with a brief id says no message was searched:\n%s", text)
	}
}

// TestVerifierPacketSaysWhenItsTimeRanOut: the backstop behind the count bound. With no time
// left the section lists nothing, and says the search stopped, not that nothing exists.
func TestVerifierPacketSaysWhenItsTimeRanOut(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	orig := verifierPacketBudget
	verifierPacketBudget = -time.Second
	t.Cleanup(func() { verifierPacketBudget = orig })
	text := vpBuild(t, vpInput(root))
	if got := len(vpListedRe.FindAllString(text, -1)); got != 0 {
		t.Fatalf("control: %d commits are listed with no time to search:\n%s", got, text)
	}
	if listed, inSection := vpOmissionLines(t, text, "commits older than the 0 searched",
		"the time allowed for this section ran out"); listed != 1 || inSection != 1 {
		t.Errorf("the packet does not say its time ran out (%d listed, %d in the section):\n%s", listed, inSection, text)
	}
}
