package main

import (
	"fmt"
	"strings"
	"testing"
)

// roundTripFixture lays a delivered brief-v2 brief, then a first-parent span
// that edits its Verify table on main without an acceptance link (a single-
// parent commit, as a repo-wide sweep lands) and a merge that restores it.
// restore maps the delivered body to the body the merge lands; returning it
// unchanged is the byte-exact restore. The returned fixture has no links yet.
func roundTripFixture(t *testing.T, edit, restore func(string) string) (*chainFixture, string) {
	t.Helper()
	o := vfDefaults()
	o.evidence = ""
	o.status = "verified"
	o.verified = "2026-08-13 human:alex"
	o.verify = "| # | Command | Expect | Class | Shell |\n|---|---|---|---|---|\n" +
		"| 1 | `true` | exit 0 | check | sh |\n| 2 | `true` | exit 0 | check | sh |\n| 3 | `true` | exit 0 | check | sh |\n"
	root, _ := vfFixture(t, o)
	path := vfBrief(root)
	f := &chainFixture{root: root, path: path, replies: map[int]string{}, issue: rlIssueJSON("<!-- acceptance-transition: " + vfBriefRel + " -->")}
	raw := readFileT(t, path)
	f.write(t, strings.Replace(raw, "schema: brief-v1", "schema: brief-v2\nversion: 1", 1))
	vfGit(t, root, vfMailHuman, "remote", "add", "origin", "https://github.com/"+rlRepo+".git")
	vfCommit(t, root, vfMailHuman)
	f.base = vfRev(t, root, "HEAD")
	f.write(t, strings.Replace(f.body, "## Evidence\n", "## Evidence\nHistorical result retained.\n", 1))
	vfCommit(t, root, vfMailWorker)
	f.delivery = vfRev(t, root, "HEAD")
	delivered := f.body
	// The sweep: one single-parent commit rewrites a delivered row.
	f.write(t, edit(delivered))
	vfCommit(t, root, vfMailHuman)
	swept := vfRev(t, root, "HEAD")
	// The restore lands as a merge, as the restoring sweep did.
	vfGit(t, root, vfMailHuman, "checkout", "-q", "-b", "restore")
	f.write(t, restore(delivered))
	vfCommit(t, root, vfMailHuman)
	vfGit(t, root, vfMailHuman, "checkout", "-q", "main")
	vfGit(t, root, vfMailHuman, "merge", "--no-ff", "-q", "-m", "restore the delivered row", "restore")
	f.write(t, readFileT(t, path))
	return f, swept
}

func sweepRow2(body string) string {
	return strings.Replace(body, "| 2 | `true` | exit 0 |", "| 2 | `true || true` | exit 0 |", 1)
}

func sameBody(body string) string { return body }

// regression: #6599
func TestChainRoundTripNoop(t *testing.T) {
	f, _ := roundTripFixture(t, sweepRow2, sameBody)
	f.amend(t, strings.Replace(f.body, "| 1 | `true` |", "| 1 | `true && true` |", 1), "1:Command", true)
	f.bind(t)
	p, why := f.plan(t)
	if why != "" {
		t.Fatalf("a byte-exact round trip before link 1 must collapse to a no-op: %s", why)
	}
	if p.Changes != "1=Command" || p.Revisions["1"] != f.amendments[0] || f.reads != 2 {
		t.Fatalf("round trip changed the plan: %+v reads=%d", p, f.reads)
	}
	// The collapsed span grants no version step: link 1's predecessor stays
	// the delivery, so an approval naming the restore as predecessor fails.
	f.payloads[0] = strings.Replace(f.payloads[0], "\npredecessor="+f.delivery+"\n", "\npredecessor="+vfRev(t, f.root, f.amendments[0]+"^1")+"\n", 1)
	f.refuse(t, "acceptance transition: link 1: human approval does not name the exact transition payload", false)
}

// regression: #6599
func TestChainRoundTripRefusals(t *testing.T) {
	for _, mode := range []string{"inexact", "trailing-blank", "version", "unapproved-bump", "open"} {
		t.Run(mode, func(t *testing.T) {
			edit, restore := sweepRow2, sameBody
			switch mode {
			case "inexact":
				restore = func(b string) string {
					return strings.Replace(b, "| 2 | `true` | exit 0 |", "| 2 | `true` | exit 0  |", 1)
				}
			case "trailing-blank":
				// Equal once trimmed, but not byte-identical.
				restore = func(b string) string { return strings.Replace(b, "\n## Evidence", "\n\n## Evidence", 1) }
			case "version":
				edit = func(b string) string { return strings.Replace(sweepRow2(b), "version: 1", "version: 2", 1) }
				restore = func(b string) string { return strings.Replace(b, "version: 1", "version: 2", 1) }
			case "open":
				restore = func(b string) string { return strings.Replace(b, "| 2 | `true` |", "| 2 | `true || false` |", 1) }
			}
			f, swept := roundTripFixture(t, edit, restore)
			missing := swept
			if mode == "unapproved-bump" {
				// An unlinked version bump after the round trip is not a no-op.
				f.write(t, strings.Replace(strings.Replace(f.body, "version: 1", "version: 2", 1), "| 3 | `true` |", "| 3 | `true && true` |", 1))
				vfCommit(t, f.root, vfMailHuman)
				missing = vfRev(t, f.root, "HEAD")
			}
			body := strings.Replace(f.body, "| 1 | `true` |", "| 1 | `true && true` |", 1)
			if mode == "version" || mode == "unapproved-bump" {
				body = strings.Replace(body, "version: 2", "version: 3", 1)
			}
			f.amend(t, body, "1:Command", true)
			f.bind(t)
			f.refuse(t, fmt.Sprintf("acceptance transition: link 1: link 1 missing at %.12s", missing), true)
		})
	}
}

// regression: #6599
func TestChainRoundTripThenBump(t *testing.T) {
	// The round trip collapses; a later real bump is still its own link and
	// needs its own approval, with the delivery as its predecessor.
	f, _ := roundTripFixture(t, sweepRow2, sameBody)
	f.amend(t, strings.Replace(f.body, "| 3 | `true` |", "| 3 | `true && true` |", 1), "3:Command", false)
	f.amend(t, strings.Replace(f.body, "| 1 | `true` |", "| 1 | `true && true` |", 1), "1:Command", true)
	f.bind(t)
	p, why := f.plan(t)
	if why != "" || p.Changes != "1=Command,3=Command" || f.reads != 4 {
		t.Fatalf("approved bump after a round trip refused: %s %+v reads=%d", why, p, f.reads)
	}
	// Dropping the bump's own link leaves it unaccounted.
	lines := strings.Split(f.descriptor, "\n")
	f.descriptor = lines[0] + "\n" + strings.Replace(lines[2], "link=2", "link=1", 1)
	f.refuse(t, fmt.Sprintf("link 1 missing at %.12s", f.amendments[0]), true)
}

// regression: #6599
func TestChainRoundTripBetweenLinks(t *testing.T) {
	// The same span between two links, restored by a single-parent commit,
	// collapses against link 1's accepted state, not the delivery's.
	f, _ := roundTripFixture(t, sweepRow2, sameBody)
	f.amend(t, strings.Replace(f.body, "| 3 | `true` |", "| 3 | `true && true` |", 1), "3:Command", false)
	accepted := f.body
	f.write(t, sweepRow2(accepted))
	vfCommit(t, f.root, vfMailHuman)
	f.write(t, accepted)
	vfCommit(t, f.root, vfMailHuman)
	f.amend(t, strings.Replace(f.body, "| 1 | `true` |", "| 1 | `true && true` |", 1), "1:Command", false)
	f.bind(t)
	p, why := f.plan(t)
	if why != "" || p.Changes != "1=Command,3=Command" || p.Revisions["1"] != f.amendments[1] {
		t.Fatalf("round trip between links refused: %s %+v", why, p)
	}
}
