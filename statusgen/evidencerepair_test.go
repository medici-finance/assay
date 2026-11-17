package main

import (
	"fmt"
	"strings"
	"testing"
)

// repairDelivered is delivered Evidence in the pre-repair shape of #6616:
// one combined Date / Runner column, an escaped pipe in a Command cell.
const repairDelivered = "\n| # | Command | Expected | Observed (exit + key output) | Date / Runner |\n" +
	"|---|---------|----------|------------------------------|---------------|\n" +
	"| 1 | go test ./x | exit 0 | exit 0; ok | 2026-10-16 verifier-app[bot] (model verifier) |\n" +
	"| 11 | run x 2>&1 \\| grep -c y | 1 | printed 1 | 2026-10-16 verifier-app[bot] (model verifier) |\n"

const (
	repairPR      = 77
	repairComment = 9101
	repairMessage = "Merge pull request #77 from example/repair"
)

// repairSplit is the sanctioned editorial repair: split Date and Runner, add
// the principal and one prose paragraph.
func repairSplit(body string) string {
	r := strings.NewReplacer(
		"| Date / Runner |\n|---|---------|----------|------------------------------|---------------|",
		"| Date | Runner |\n|---|---------|----------|------------------------------|------|--------|",
		"| 2026-10-16 verifier-app[bot] (model verifier) |",
		"| 2026-10-16 | verifier-app[bot] (model verifier) (on-behalf-of human:alex) |",
	)
	body = r.Replace(body)
	return strings.Replace(body, "Historical result retained.\n", "Historical result retained.\n\nEditorial repair only: separate Date and Runner.\n", 1)
}

func repairApproval(login, typ, body, updated string) string {
	return rlCommentJSONWith(login, typ, rlRepo, repairPR, body, rlPosted, updated)
}

const repairApprovalText = "For PR #77 only, I authorize separating Date and Runner in the vf/01 Evidence."

// repairChain builds the #6616 shape: link 1, then the repair, then link 2.
func repairChain(t *testing.T, edit func(string) string, message string) (*chainFixture, string) {
	t.Helper()
	var repair string
	f := chainFixtureWith(t, "repair", func(f *chainFixture) {
		f.write(t, edit(f.body))
		vfGit(t, f.root, vfMailHuman, "add", "-A")
		vfGit(t, f.root, vfMailHuman, "commit", "-q", "-m", message)
		repair = vfRev(t, f.root, "HEAD")
	})
	login, _ := HumanLogin("alex")
	f.extra = map[int]string{repairComment: repairApproval(login, "User", repairApprovalText, rlPosted)}
	f.issues = map[int]string{repairPR: fmt.Sprintf(`{"number":%d,"state":"closed","body":"repair","pull_request":{"url":"https://api.github.com/repos/%s/pulls/%d"}}`, repairPR, rlRepo, repairPR)}
	return f, repair
}

func (f *chainFixture) withRepair(commit string) {
	f.record()
	f.descriptor += fmt.Sprintf("\nEvidence-Repair: commit=%s approval=https://github.com/%s/pull/%d#issuecomment-%d", commit, rlRepo, repairPR, repairComment)
}

// regression: #6616
func TestEvidenceRepairChain(t *testing.T) {
	f, repair := repairChain(t, repairSplit, repairMessage)
	// Without a repair line the chain refuses exactly as before.
	f.refuse(t, "link 1: historical Evidence was not preserved", true)
	f.withRepair(repair)
	f.reads = 0
	if _, why := f.plan(t); why != "" {
		t.Fatal(why)
	}
	if f.reads == 0 {
		t.Fatal("repair approval was never read")
	}
}

// regression: #6616
func TestEvidenceRepairRefusals(t *testing.T) {
	login, _ := HumanLogin("alex")
	for _, c := range []struct {
		name, want, message string
		edit                func(string) string
		reply               string
		structural          bool
	}{
		{name: "observed", want: "changed the Observed (exit + key output) cell of row 1", structural: true,
			edit: func(b string) string {
				return strings.Replace(repairSplit(b), "| exit 0; ok |", "| exit 0; all ok |", 1)
			}},
		{name: "verify", want: "link 2 missing", structural: true,
			edit: func(b string) string {
				return strings.Replace(repairSplit(b), "| 3 | `true && true` | exit 0 |", "| 3 | `true && true` | exit 0 again |", 1)
			}},
		{name: "unnamed", want: "does not name its approval", structural: true, message: "fixture"},
		{name: "bot", want: "a bot account is not a human", reply: repairApproval("helper[bot]", "Bot", repairApprovalText, rlPosted)},
		{name: "edited", want: "was edited after it was posted", reply: repairApproval(login, "User", repairApprovalText, "2026-09-02T10:00:00Z")},
		{name: "brief", want: "does not name the record vf/01", reply: repairApproval(login, "User", "For PR #77 only, I authorize this.", rlPosted)},
	} {
		t.Run(c.name, func(t *testing.T) {
			edit, message := c.edit, c.message
			if edit == nil {
				edit = repairSplit
			}
			if message == "" {
				message = repairMessage
			}
			f, repair := repairChain(t, edit, message)
			if c.reply != "" {
				f.extra[repairComment] = c.reply
			}
			f.withRepair(repair)
			f.refuse(t, c.want, c.structural)
		})
	}
}

// regression: #6616
func TestEvidenceRepairPlacement(t *testing.T) {
	f, _ := repairChain(t, repairSplit, repairMessage)
	f.withRepair(f.base)
	f.refuse(t, "is not after the amendment's parent", true)
	f.record()
	f.descriptor += "\nEvidence-Repair: commit=" + f.base + " approval=https://example.com/x"
	f.refuse(t, "a valid issue or pull-request comment approval URL is required", true)
}
