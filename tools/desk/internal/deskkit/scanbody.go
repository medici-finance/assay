package deskkit

// scanbody.go DERIVES the issue-loop scan PR's title and body from the branch's own
// diff, so the recorded created/retired counts cannot drift from the change they
// describe.
//
// The defect it closes (#685, third recurrence after #592 and #627): the intake-desk
// coalesces every scan of a session into ONE running PR and pushes to it repeatedly, but
// the title/body enumerating created/retired placeholders is HAND-WRITTEN ONCE. On #627
// the body still claimed 29/29 while the diff had grown to 48/32 across 12 coalesced
// commits; the reviewer flagged it twice and each re-push added more scans past the stale
// body. The placeholder DATA was correct every time — only the recorded counts were
// stale. Two prior attempts to fix this with discipline ("remember to regenerate the body
// on every push") are exactly the discipline that failed.
//
// So the counts get one declared source — `git diff <merge-base>..HEAD --
// docs/streams/issue-loop/` — and the body is REGENERATED from it at push time. Nothing
// hand-maintains a second copy.
//
// The parsing here is deliberately over a unified diff rather than --name-status: a
// RETIREMENT is not a file operation, it is a `status:` flip inside a file, and
// --name-status cannot see it.

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ScanDir is the stream directory the scan PR is scoped to.
const ScanDir = "docs/streams/issue-loop"

// ScanBodyMarker identifies a derived scan body. Its presence is what tells the next push
// that this body is machine-owned; a body without it was hand-written and is the thing
// being replaced.
const ScanBodyMarker = "<!-- desk-scanbody v1 -->"

// ScanEntry is one placeholder the diff created or retired.
type ScanEntry struct {
	Path string // repo-relative path
	Why  string // how it was classified — shown in the body so the count is auditable
}

// ScanCounts is the derived record: what this branch's diff actually did.
type ScanCounts struct {
	Created []ScanEntry
	Retired []ScanEntry
}

// diffFileRe captures the a/ and b/ paths of a `diff --git` header.
var diffFileRe = regexp.MustCompile(`^diff --git a/(\S+) b/(\S+)$`)

// statusDoneRe matches an ADDED frontmatter line flipping a placeholder to done.
var statusDoneRe = regexp.MustCompile(`^\+\s*status:\s*done\s*$`)

// ParseScanDiff classifies a unified diff (`git diff -U0 -M <merge-base> HEAD -- <dir>`)
// into created and retired placeholders.
//
//   - CREATED — a new file under dir that is not the stream README, not already in the
//     done/ archive, and not born carrying `status: done`. A placeholder born already
//     closed never was live work, so it is not a creation for this count — the same rule
//     the done/ archive already got, applied to the frontmatter shape of the same state.
//   - RETIRED — a file whose diff ADDS `status: done`, or a rename whose destination is
//     under done/ (the scanner's archive move). Either is a retirement; a scan that flips
//     and archives in one pass must count once, so the two are unioned by path.
//
// The two columns are therefore disjoint: no path can appear in both.
//
// dir is matched as a path prefix; the README and any non-.md file are ignored.
func ParseScanDiff(diff, dir string) ScanCounts {
	dir = strings.Trim(strings.TrimSpace(dir), "/")
	created := map[string]string{}
	retired := map[string]string{}

	var curOld, curNew string
	var isNew, isRename bool

	flush := func() {
		if curNew == "" {
			return
		}
		// retired[curNew] is already settled here: the status-flip shape is recorded
		// while the file's hunk lines are scanned, and flush runs after them.
		if isNew && scanCountable(curNew, dir) && !scanArchived(curNew) && retired[curNew] == "" {
			created[curNew] = "new placeholder file"
		}
		if isRename && scanCountable(curNew, dir) && scanArchived(curNew) && !scanArchived(curOld) {
			retired[curNew] = "archived into " + dir + "/done/"
		}
	}

	for _, ln := range strings.Split(strings.ReplaceAll(diff, "\r\n", "\n"), "\n") {
		if m := diffFileRe.FindStringSubmatch(ln); m != nil {
			flush()
			curOld, curNew = m[1], m[2]
			isNew, isRename = false, false
			continue
		}
		if curNew == "" {
			continue
		}
		switch {
		case strings.HasPrefix(ln, "new file mode "):
			isNew = true
		case strings.HasPrefix(ln, "rename to "):
			isRename = true
		case statusDoneRe.MatchString(ln):
			if scanCountable(curNew, dir) {
				retired[curNew] = "status flipped to done"
			}
		}
	}
	flush()

	return ScanCounts{Created: scanEntries(created), Retired: scanEntries(retired)}
}

// scanCountable rejects anything that is not a placeholder/brief markdown file under dir.
func scanCountable(p, dir string) bool {
	if dir != "" && !strings.HasPrefix(p, dir+"/") {
		return false
	}
	if !strings.HasSuffix(p, ".md") {
		return false
	}
	return path.Base(p) != "README.md"
}

func scanArchived(p string) bool {
	return strings.Contains(p, "/done/")
}

func scanEntries(m map[string]string) []ScanEntry {
	out := make([]ScanEntry, 0, len(m))
	for p, why := range m {
		out = append(out, ScanEntry{Path: p, Why: why})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// ScanPRTitle renders the derived title. The counts are formatted exactly as
// StatedScanCounts reads them back, so the drift lint below can compare a title/body
// against a fresh derivation without a second format to keep in sync.
func ScanPRTitle(date string, c ScanCounts) string {
	return fmt.Sprintf("chore(issue-loop): scan %s — %d created, %d retired",
		strings.TrimSpace(date), len(c.Created), len(c.Retired))
}

// scanListCap bounds the enumerations. A coalesced scan branch can carry hundreds of
// placeholders and a PR body over GitHub's limit fails the push that regenerates it —
// which would put the body back to being stale. The count itself is never truncated.
const scanListCap = 60

// ScanPRBody renders the derived body: the counts, the command they came from, and the
// enumerations. base is the merge-base ref the diff was taken from.
func ScanPRBody(date string, c ScanCounts, base string) string {
	var b strings.Builder
	b.WriteString(ScanBodyMarker + "\n")
	b.WriteString("Automated inbound scan. **This body is DERIVED — do not hand-edit it.**\n")
	b.WriteString("It is regenerated from the branch's own diff on every push, because a\n")
	b.WriteString("hand-maintained count drifts from the diff it describes (#685, #627, #592).\n\n")
	fmt.Fprintf(&b, "- **created:** %d\n", len(c.Created))
	fmt.Fprintf(&b, "- **retired:** %d\n", len(c.Retired))
	fmt.Fprintf(&b, "- **derived from:** `git diff %s..HEAD -- %s/` (%s)\n\n",
		strings.TrimSpace(base), ScanDir, strings.TrimSpace(date))
	scanSection(&b, "Created", c.Created)
	scanSection(&b, "Retired", c.Retired)
	b.WriteString("Review the created/retired placeholders, not the counts: the counts are the diff.\n")
	return b.String()
}

func scanSection(b *strings.Builder, heading string, entries []ScanEntry) {
	fmt.Fprintf(b, "### %s (%d)\n\n", heading, len(entries))
	if len(entries) == 0 {
		b.WriteString("_none_\n\n")
		return
	}
	shown := entries
	if len(shown) > scanListCap {
		shown = shown[:scanListCap]
	}
	for _, e := range shown {
		fmt.Fprintf(b, "- `%s` — %s\n", e.Path, e.Why)
	}
	if len(entries) > len(shown) {
		fmt.Fprintf(b, "- _… and %d more (list capped at %d; the count above is complete)_\n",
			len(entries)-len(shown), scanListCap)
	}
	b.WriteString("\n")
}

// statedCountsRe reads "<N> created, <M> retired" out of an existing title or body,
// tolerating the `**bold**` and bullet spellings the derived body uses.
var statedCountsRe = regexp.MustCompile(`(?i)(\d+)\s*\**\s*created\D{0,40}?(\d+)\s*\**\s*retired`)

// bulletCountsRe reads the derived body's own bullet form, where the number FOLLOWS the
// word ("**created:** 48").
var bulletCountsRe = regexp.MustCompile(`(?i)created\**\s*:?\**\s*(\d+)[\s\S]{0,120}?retired\**\s*:?\**\s*(\d+)`)

// StatedScanCounts extracts the counts a title/body ASSERTS. ok is false when the text
// states no counts at all — which is not a mismatch, it is simply nothing to diff.
func StatedScanCounts(text string) (created, retired int, ok bool) {
	if m := statedCountsRe.FindStringSubmatch(text); m != nil {
		c, _ := strconv.Atoi(m[1])
		r, _ := strconv.Atoi(m[2])
		return c, r, true
	}
	if m := bulletCountsRe.FindStringSubmatch(text); m != nil {
		c, _ := strconv.Atoi(m[1])
		r, _ := strconv.Atoi(m[2])
		return c, r, true
	}
	return 0, 0, false
}

// ScanBodyDrift is the belt-and-suspenders lint (#685 fix 2): given the text a PR
// currently carries and a fresh derivation, it returns a refusal when the asserted counts
// disagree with the diff.
//
// It exists even though the body is now derived, because the derivation is only in force
// on pushes that run through the emitter. A body edited by hand in the GitHub UI, or
// written by a session that skipped the tool, still lands — and this turns that back into
// a red gate instead of a reviewer catch, which is what #592 and #627 both relied on and
// both lost.
//
// Text stating no counts returns nil: absence is the emitter's business, not the lint's.
func ScanBodyDrift(text string, c ScanCounts) error {
	stated, statedRetired, ok := StatedScanCounts(text)
	if !ok {
		return nil
	}
	if stated == len(c.Created) && statedRetired == len(c.Retired) {
		return nil
	}
	return Refused(fmt.Sprintf(
		"scan PR text states %d created / %d retired but the diff has %d created / %d retired — "+
			"the recorded counts drifted from the change they describe (#685). Regenerate with "+
			"`deskscanbody emit` instead of hand-editing.",
		stated, statedRetired, len(c.Created), len(c.Retired)))
}
