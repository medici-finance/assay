package main

// packet_review.go — the review kit's packet: what a reviewer otherwise spends its first
// dozen calls fetching, read once at dispatch (#2437).
//
// SECTIONS, in order: the change's facts; its description; the brief, when the dispatch
// names one; check and status states at the head; the earlier verdicts of THIS lane in full
// and a one-line index of every other review; the diff; the post-change text of each touched
// file.
//
// EVERYTHING IS A FORGE READ. This file starts no process and runs no git: it reads through
// the resolved forge under the review dispatcher's own credential — the one the stamp step
// already uses — and the brief from the path the dispatch resolved. It writes nothing to the
// forge.
//
// NOTHING HERE IS A GATE. The packet reports what the forge returned and who the forge says
// wrote it. Which review counts, whether a check is required, whether the change may merge —
// those stay with the tools that decide them. In particular the lane sort below is for
// LAYOUT only.

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/packet"
)

func init() { registerPacketProvider("review", reviewPacket) }

// The review packet's own limits, stated in its header beside the shared caps.
const (
	// reviewPacketDiffCap is the diff's cap. A diff is the one item a reviewer must see
	// whole, so it gets three times a file's allowance; past that it is listed as omitted
	// and read from the worktree, never shown in part.
	reviewPacketDiffCap = 192 << 10
	// reviewPacketMaxFiles bounds how many touched files are read — one forge call each.
	reviewPacketMaxFiles = 60
	// reviewPacketReadBudget bounds the time spent reading files: the dispatch holds its
	// claim while the packet is built.
	reviewPacketReadBudget = 60 * time.Second
	// reviewPacketMaxRows bounds each tool-written list (checks, the review index, the
	// touched-file list). A list that hits it says how many rows it left out.
	reviewPacketMaxRows = 200
)

// reviewPacketForge is the part of the forge the review packet reads.
type reviewPacketForge interface {
	GetPullRequest(repo deskkit.ForgeRepo, number int) (*deskkit.PullRequest, error)
	ChecksAtHead(repo deskkit.ForgeRepo, sha string) (*deskkit.ChecksAtHead, error)
	ReviewsAtHead(repo deskkit.ForgeRepo, number int) ([]deskkit.Review, error)
	ListChangedFiles(repo deskkit.ForgeRepo, number int) ([]deskkit.ChangedFile, error)
	ChangeDiff(repo deskkit.ForgeRepo, number int) (string, error)
	ReadFile(repo deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error)
}

// reviewPacketForgeFn resolves the forge the packet reads from. A seam for tests; the
// default is the same resolution, under the same role, as the model-stamp step.
var reviewPacketForgeFn = func(repo string) (reviewPacketForge, deskkit.ForgeRepo, error) {
	fr, err := forgeRepoOf(repo)
	if err != nil {
		return nil, fr, err
	}
	fg, _, err := deskkit.ResolveForge(fr, deskkit.ReviewDispatcherRole)
	if err != nil {
		return nil, fr, err
	}
	return fg, fr, nil
}

// Review lanes, for layout.
const (
	laneCorrectness = "correctness"
	laneSecurity    = "security"
)

// reviewLaneOf reads the lane of a review dispatch off its claim key: a key whose suffix
// after "--pr-<N>" has a "security" segment is the security lane; any other key is the
// correctness lane. The dispatch carries no other statement of its lane. The packet says
// which lane it assumed and from which key, and the other lane's reviews are still indexed,
// so a wrong guess moves text between two headings and hides nothing.
func reviewLaneOf(claimKey string, pr int) string {
	marker := fmt.Sprintf("--pr-%d", pr)
	i := strings.LastIndex(claimKey, marker)
	if i < 0 {
		return laneCorrectness
	}
	suffix := claimKey[i+len(marker):]
	if suffix != "" && !strings.HasPrefix(suffix, "--") {
		return laneCorrectness // "--pr-770" is another change's key, not a suffix of this one's
	}
	for _, seg := range strings.Split(suffix, "--") {
		if strings.EqualFold(seg, laneSecurity) {
			return laneSecurity
		}
	}
	return laneCorrectness
}

// reviewVerdictLine is the verdict-line grammar the posting verb enforces on a review body,
// repeated here so a posted review can be sorted under a lane heading.
// TestReviewVerdictLineMatchesThePostingVerb holds the two spellings together.
var reviewVerdictLine = regexp.MustCompile(`(?mi)^[ \t]*(Verdict|Security-Review):[ \t]*(approve|request-changes|pass|fail)[ \t]*$`)

// reviewBodyLane names the lane a review body's verdict line puts it in: correctness,
// security, "both" (two kinds in one body) or "" (no verdict line).
func reviewBodyLane(body string) string {
	var c, s bool
	for _, m := range reviewVerdictLine.FindAllStringSubmatch(body, -1) {
		if strings.EqualFold(m[1], "Verdict") {
			c = true
		} else {
			s = true
		}
	}
	switch {
	case c && s:
		return "both"
	case c:
		return laneCorrectness
	case s:
		return laneSecurity
	}
	return ""
}

// reviewPacketSafePath reports whether a touched file's path may be put in a forge read.
// The path comes from the change, so its author chose it, and a forge backend may place it
// in a request path as given. Anything that could change what is requested — a query or
// fragment mark, a percent sign, a backslash, a dot segment, a leading slash, white space, a
// control character — is not sent; the file is listed as omitted instead.
func reviewPacketSafePath(p string) bool {
	if p == "" || len(p) > 1024 || strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	for _, r := range p {
		switch {
		case r == '/' || unicode.IsLetter(r) || unicode.IsDigit(r):
		case strings.ContainsRune("._-+@=,~()[]", r):
		default:
			return false
		}
	}
	return true
}

func reviewPacket(in packetInput) (packet.Spec, error) {
	pr := in.o.pr
	if pr <= 0 {
		return packet.Spec{}, errNoPacket
	}
	fg, fr, err := reviewPacketForgeFn(in.repo)
	if err != nil {
		return packet.Spec{}, fmt.Errorf("the forge for %s could not be reached: %s", in.repo, firstLine(err.Error()))
	}
	change, err := fg.GetPullRequest(fr, pr)
	if err != nil {
		return packet.Spec{}, fmt.Errorf("%s#%d could not be read: %s", in.repo, pr, firstLine(err.Error()))
	}
	if change == nil || strings.TrimSpace(change.HeadSHA) == "" {
		return packet.Spec{}, fmt.Errorf("%s#%d was read without a head commit", in.repo, pr)
	}
	head := change.HeadSHA
	lane := reviewLaneOf(in.plan.claimKey, pr)

	// The touched-file list feeds two sections; it is read once.
	var (
		files     []deskkit.ChangedFile
		filesErr  error
		filesRead bool
	)
	changed := func() ([]deskkit.ChangedFile, error) {
		if !filesRead {
			files, filesErr = fg.ListChangedFiles(fr, pr)
			filesRead = true
		}
		return files, filesErr
	}

	sections := []packet.Section{
		packet.NewSection("Change", func() (packet.Content, error) { return reviewChangeSection(in.repo, change), nil }),
		packet.NewSection("Description", func() (packet.Content, error) {
			var c packet.Content
			if strings.TrimSpace(change.Body) == "" {
				c.Text("_The change has no description._")
				return c, nil
			}
			c.Text("The change's description, as its author wrote it.")
			c.Untrusted("description", []byte(change.Body))
			return c, nil
		}),
	}
	if brief := strings.TrimSpace(in.o.brief); brief != "" {
		sections = append(sections, packet.NewSection("Brief", func() (packet.Content, error) {
			var c packet.Content
			raw, err := os.ReadFile(brief)
			if err != nil {
				return c, fmt.Errorf("the brief could not be read: %v", err)
			}
			c.Textf("The specification this dispatch names (`%s`), as it stands in the dispatcher's checkout.",
				packet.Inline(briefArg(in.o)))
			c.Untrusted("brief "+briefArg(in.o), raw)
			return c, nil
		}))
	}
	sections = append(sections,
		packet.NewSection("Checks at head", func() (packet.Content, error) { return reviewChecksSection(fg, fr, head) }),
		packet.NewSection("Earlier verdicts", func() (packet.Content, error) {
			return reviewVerdictsSection(fg, fr, pr, head, lane, in.plan.claimKey)
		}),
		packet.NewSection("Diff", func() (packet.Content, error) { return reviewDiffSection(fg, fr, pr, change, changed) }),
		packet.NewSection("Files at head", func() (packet.Content, error) {
			return reviewFilesSection(fg, fr, change, changed)
		}),
	)

	return packet.Spec{
		Head: head,
		CapNotes: []string{
			fmt.Sprintf("The diff may be up to %d bytes.", reviewPacketDiffCap),
			fmt.Sprintf("At most %d touched files are read, within %s.", reviewPacketMaxFiles, reviewPacketReadBudget),
		},
		Sections: sections,
		Recheck: func() error {
			again, err := fg.GetPullRequest(fr, pr)
			if err != nil {
				return fmt.Errorf("the head could not be re-read after the build, so the packet may be stale: %s", firstLine(err.Error()))
			}
			if again == nil || again.HeadSHA != head {
				now := "(none)"
				if again != nil {
					now = again.HeadSHA
				}
				return fmt.Errorf("the head moved from %s to %s while the packet was read", head, now)
			}
			return nil
		},
	}, nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(not reported)"
	}
	return packet.Inline(s)
}

func reviewChangeSection(repo string, c *deskkit.PullRequest) packet.Content {
	var out packet.Content
	var b strings.Builder
	fmt.Fprintf(&b, "- **Change:** %s#%d — %s\n", packet.Inline(repo), c.Number, orNone(c.Title))
	author := orNone(c.Author.Login)
	if t := strings.TrimSpace(c.Author.Type); t != "" {
		author += " (" + packet.Inline(t) + ")"
	}
	fmt.Fprintf(&b, "- **Author:** %s\n", author)
	fmt.Fprintf(&b, "- **Base:** `%s`\n", orNone(c.BaseRef))
	fmt.Fprintf(&b, "- **Head branch:** `%s`\n", orNone(c.HeadRef))
	fmt.Fprintf(&b, "- **Head commit:** `%s`\n", packet.Inline(c.HeadSHA))
	fmt.Fprintf(&b, "- **State:** %s\n", orNone(c.State))
	fmt.Fprintf(&b, "- **Draft:** %s\n", yesNo(c.Draft))
	fmt.Fprintf(&b, "- **Mergeable:** %s\n", orNone(c.Mergeable))
	if c.CrossRepo != "" {
		fmt.Fprintf(&b, "- **Head repository:** %s\n", packet.Inline(c.CrossRepo))
	}
	fmt.Fprintf(&b, "- **Files changed (the forge's count):** %d\n", c.ChangedFiles)
	if len(c.Labels) > 0 {
		labels := make([]string, 0, len(c.Labels))
		for _, l := range c.Labels {
			labels = append(labels, "`"+packet.Inline(l)+"`")
		}
		fmt.Fprintf(&b, "- **Labels:** %s\n", strings.Join(labels, ", "))
	}
	fmt.Fprintf(&b, "- **Last updated:** %s\n", orNone(c.UpdatedAt))
	if c.URL != "" {
		fmt.Fprintf(&b, "- **URL:** %s\n", packet.Inline(c.URL))
	}
	out.Text(b.String())
	return out
}

func reviewChecksSection(fg reviewPacketForge, fr deskkit.ForgeRepo, head string) (packet.Content, error) {
	var out packet.Content
	checks, err := fg.ChecksAtHead(fr, head)
	if err != nil {
		return out, errors.New(firstLine(err.Error()))
	}
	if checks == nil {
		return out, errors.New("the forge returned no check record")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "States at head `%s`, as the forge reports them now. Which of them are required is not stated here.\n\n",
		packet.Inline(head))
	fmt.Fprintf(&b, "- **Combined status:** %s\n", orNone(checks.CombinedState))
	fmt.Fprintf(&b, "- **Commit statuses:** %d listed of %d\n", len(checks.Statuses), checks.StatusTotalCount)
	for i, s := range checks.Statuses {
		if i == reviewPacketMaxRows {
			fmt.Fprintf(&b, "  - … %d more not shown\n", len(checks.Statuses)-i)
			break
		}
		fmt.Fprintf(&b, "  - `%s` — %s\n", packet.Inline(s.Context), orNone(s.State))
	}
	fmt.Fprintf(&b, "- **Check runs:** %d listed of %d\n", len(checks.CheckRuns), checks.CheckRunsTotalCount)
	for i, r := range checks.CheckRuns {
		if i == reviewPacketMaxRows {
			fmt.Fprintf(&b, "  - … %d more not shown\n", len(checks.CheckRuns)-i)
			break
		}
		state := orNone(r.Status)
		if strings.TrimSpace(r.Conclusion) != "" {
			state += " / " + packet.Inline(r.Conclusion)
		}
		fmt.Fprintf(&b, "  - `%s` — %s\n", packet.Inline(r.Name), state)
	}
	out.Text(b.String())
	return out, nil
}

func reviewLine(r deskkit.Review, head string) string {
	at := "not the current head"
	if r.CommitID == head {
		at = "the current head"
	}
	return fmt.Sprintf("review %d — state `%s` — commit `%s` (%s) — submitted %s — by %s",
		r.ID, orNone(r.State), orNone(r.CommitID), at, orNone(r.SubmittedAt), orNone(r.Author.Login))
}

func reviewVerdictsSection(fg reviewPacketForge, fr deskkit.ForgeRepo, pr int, head, lane, claimKey string) (packet.Content, error) {
	var out packet.Content
	reviews, err := fg.ReviewsAtHead(fr, pr)
	if err != nil {
		return out, errors.New(firstLine(err.Error()))
	}
	other := laneSecurity
	if lane == laneSecurity {
		other = laneCorrectness
	}
	out.Textf("This dispatch is the **%s** lane (read off its claim key `%s`). Every review on the change is below, oldest "+
		"first, under the author the forge reports. A review is filed under a lane by the verdict line in its body and "+
		"nothing else: this packet does not check who posted it or whether it counts.",
		lane, packet.Inline(claimKey))

	var mine, rest []deskkit.Review
	for _, r := range reviews {
		if reviewBodyLane(r.Body) == lane {
			mine = append(mine, r)
		} else {
			rest = append(rest, r)
		}
	}
	out.Textf("### This lane (%s) — %d earlier verdict(s), in full", lane, len(mine))
	if len(mine) == 0 {
		out.Text("_None._")
	}
	for _, r := range mine {
		out.Text("- " + reviewLine(r, head))
		out.Untrusted(fmt.Sprintf("review %d body", r.ID), []byte(r.Body))
	}
	out.Textf("### Other reviews — %d, one line each (the %s lane's among them)", len(rest), other)
	if len(rest) == 0 {
		out.Text("_None._")
		return out, nil
	}
	var b strings.Builder
	for i, r := range rest {
		if i == reviewPacketMaxRows {
			fmt.Fprintf(&b, "- … %d more not shown\n", len(rest)-i)
			break
		}
		kind := reviewBodyLane(r.Body)
		switch kind {
		case "":
			kind = "no verdict line"
		case "both":
			kind = "both verdict lines"
		default:
			kind += " lane"
		}
		fmt.Fprintf(&b, "- %s — %s\n", reviewLine(r, head), kind)
	}
	out.Text(b.String())
	return out, nil
}

func reviewDiffSection(fg reviewPacketForge, fr deskkit.ForgeRepo, pr int, change *deskkit.PullRequest,
	changed func() ([]deskkit.ChangedFile, error)) (packet.Content, error) {
	var out packet.Content
	diff, derr := fg.ChangeDiff(fr, pr)
	if derr == nil && strings.TrimSpace(diff) != "" {
		out.Textf("The forge's unified diff of the whole change, base `%s` to head `%s`.", orNone(change.BaseRef), packet.Inline(change.HeadSHA))
		out.UntrustedCapped("diff", []byte(diff), reviewPacketDiffCap)
		return out, nil
	}
	why := "it returned an empty diff"
	if derr != nil {
		why = firstLine(derr.Error())
	}
	files, ferr := changed()
	if ferr != nil {
		return out, fmt.Errorf("no whole diff (%s) and no per-file patches (%s)", why, firstLine(ferr.Error()))
	}
	out.Textf("The forge did not serve one diff for the whole change (%s). Its per-file patches follow instead; "+
		"a file the forge gave no patch for is listed as omitted.", packet.Inline(why))
	for _, f := range files {
		if f.PatchAbsent || f.Patch == "" {
			out.Omit("patch of "+f.Filename, packet.SizeUnknown, "the forge served no patch for it (binary, too large, or no content change)")
			continue
		}
		out.Untrusted("patch of "+f.Filename, []byte(f.Patch))
	}
	return out, nil
}

func reviewFilesSection(fg reviewPacketForge, fr deskkit.ForgeRepo, change *deskkit.PullRequest,
	changed func() ([]deskkit.ChangedFile, error)) (packet.Content, error) {
	var out packet.Content
	files, err := changed()
	if err != nil {
		return out, errors.New(firstLine(err.Error()))
	}
	head := change.HeadSHA

	var b strings.Builder
	fmt.Fprintf(&b, "The forge lists %d touched file(s); it counts %d on the change.", len(files), change.ChangedFiles)
	if change.ChangedFiles > 0 && len(files) != change.ChangedFiles {
		b.WriteString(" **The two differ: this list is not the whole change.** Read the rest at the source.")
	}
	b.WriteString(" Each file still present at the head follows in full, as it reads AFTER the change.\n\n")
	for i, f := range files {
		if i == reviewPacketMaxRows {
			fmt.Fprintf(&b, "- … %d more not shown\n", len(files)-i)
			break
		}
		fmt.Fprintf(&b, "- `%s` — %s", packet.Inline(f.Filename), orNone(f.Status))
		if f.PreviousFilename != "" && f.PreviousFilename != f.Filename {
			fmt.Fprintf(&b, " (was `%s`)", packet.Inline(f.PreviousFilename))
		}
		b.WriteByte('\n')
	}
	out.Text(b.String())

	start := packetNow()
	read, total := 0, 0
	for _, f := range files {
		if strings.EqualFold(f.Status, "removed") || strings.EqualFold(f.Status, "deleted") {
			continue // nothing at the head to show; it is in the list above and in the diff
		}
		name := "file " + f.Filename
		switch {
		case !reviewPacketSafePath(f.Filename):
			out.Omit(name, packet.SizeUnknown, "its path holds characters this tool does not put in a forge request")
			continue
		case read >= reviewPacketMaxFiles:
			out.Omit(name, packet.SizeUnknown, fmt.Sprintf("past the %d-file read limit", reviewPacketMaxFiles))
			continue
		case total >= packet.DefaultOverallCap:
			out.Omit(name, packet.SizeUnknown, "the files already read fill the packet's overall cap")
			continue
		case packetNow().Sub(start) > reviewPacketReadBudget:
			out.Omit(name, packet.SizeUnknown, fmt.Sprintf("the %s read budget ran out", reviewPacketReadBudget))
			continue
		}
		read++
		fc, rerr := fg.ReadFile(fr, deskkit.ReadFileInput{File: f.Filename, Ref: head})
		switch {
		case rerr != nil && deskkit.IsForgeNotFound(rerr), rerr == nil && (fc == nil || !fc.Exists):
			out.Omit(name, packet.SizeUnknown, "the forge has no such file at the head commit")
		case rerr != nil:
			out.Omit(name, packet.SizeUnknown, "could not be read: "+firstLine(rerr.Error()))
		case len(fc.Content) == 0:
			out.Omit(name, 0, "the forge returned no inline content (an empty file, or one over its inline-read limit)")
		default:
			if len(fc.Content) <= packet.DefaultPerItemCap {
				total += len(fc.Content)
			}
			out.Untrusted(name, fc.Content)
		}
	}
	return out, nil
}
