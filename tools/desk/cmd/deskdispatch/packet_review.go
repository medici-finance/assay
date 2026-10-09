package main

// packet_review.go — the review kit's packet: what a reviewer otherwise spends its first
// dozen calls fetching, read once at dispatch (#2437).
//
// SECTIONS, in order: the change's facts; its description; the brief, when the dispatch
// names one; check and status states at the head; the reviewer identity's earlier verdicts
// of THIS lane in full, a one-line index of its other reviews, and the reviews of every
// other account under their own heading; the diff; the post-change text of each touched
// file.
//
// EVERY VALUE THE FORGE RETURNED IS SHOWN AS A VALUE. A title, a login, a branch, a state, a
// check name, a file name, an error message: each goes through packet.Code, so it sits in a
// code span it cannot close, and the packet's preamble tells the reader a code span is data.
// The only bare words in this file's output are this file's own.
//
// EVERYTHING IS A FORGE READ. This file starts no process and runs no git: it reads through
// the resolved forge under the review dispatcher's own credential — the one the stamp step
// already uses — and the brief from the path the dispatch resolved. It writes nothing to the
// forge.
//
// NOTHING HERE IS A GATE. The packet reports what the forge returned and who the forge says
// wrote it. Which review counts, whether a check is required, whether the change may merge —
// those stay with the tools that decide them. The lane sort below is for LAYOUT, with one
// rule the layout must not get wrong: only a review whose author is the reviewer identity is
// ever put under a heading that calls it a verdict, or counted as one.

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
	// reviewPacketMaxOtherBodies and reviewPacketOtherBodyCap bound the review bodies quoted
	// from accounts other than the reviewer identity. Anyone who can review the change can
	// post one, so they get a small fixed share of the packet and cannot crowd out the diff.
	reviewPacketMaxOtherBodies = 5
	reviewPacketOtherBodyCap   = 16 << 10
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

// reviewPacketReviewerFn names the ONE account whose reviews are verdicts: the reviewer
// role's login, the same binding the posting verb matches a review's author against. ok is
// false when the role is not bound here; the section then lists no review as a verdict. A
// seam for tests.
var reviewPacketReviewerFn = func() (string, bool) { return deskkit.RoleAppLogin(deskkit.ReviewDispatcherRole) }

// byReviewer reports whether the forge names the reviewer identity as a review's author.
// An empty login matches nothing: a review whose author the forge no longer reports is not
// the reviewer's.
func byReviewer(r deskkit.Review, reviewer string) bool {
	return reviewer != "" && r.Author.Login != "" && r.Author.Login == reviewer
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
		packet.NewSection("Change", func() (packet.Content, error) {
			c := reviewChangeSection(in.repo, change)
			c.Text("Not in this packet: the base commit (the base is given as a branch name only), and the change's " +
				"conversation comments and inline review comments. Read them at the source if you need them.")
			return c, nil
		}),
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
			c.Textf("The specification this dispatch names (%s), as it stands in the dispatcher's checkout.",
				packet.Code(briefArg(in.o)))
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
			fmt.Sprintf("At most %d review bodies from accounts other than the reviewer identity are quoted, up to %d bytes each.",
				reviewPacketMaxOtherBodies, reviewPacketOtherBodyCap),
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

// codeOrNone renders a forge value as a code span, or says the forge gave none.
func codeOrNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(not reported)"
	}
	return packet.Code(s)
}

func reviewChangeSection(repo string, c *deskkit.PullRequest) packet.Content {
	var out packet.Content
	var b strings.Builder
	fmt.Fprintf(&b, "- **Change:** %s#%d — %s\n", packet.Inline(repo), c.Number, codeOrNone(c.Title))
	author := codeOrNone(c.Author.Login)
	if t := strings.TrimSpace(c.Author.Type); t != "" {
		author += " (" + packet.Code(t) + ")"
	}
	fmt.Fprintf(&b, "- **Author:** %s\n", author)
	fmt.Fprintf(&b, "- **Base:** %s\n", codeOrNone(c.BaseRef))
	fmt.Fprintf(&b, "- **Head branch:** %s\n", codeOrNone(c.HeadRef))
	fmt.Fprintf(&b, "- **Head commit:** %s\n", codeOrNone(c.HeadSHA))
	fmt.Fprintf(&b, "- **State:** %s\n", codeOrNone(c.State))
	fmt.Fprintf(&b, "- **Draft:** %s\n", yesNo(c.Draft))
	fmt.Fprintf(&b, "- **Mergeable:** %s\n", codeOrNone(c.Mergeable))
	if c.CrossRepo != "" {
		fmt.Fprintf(&b, "- **Head repository:** %s\n", packet.Code(c.CrossRepo))
	}
	if c.ChangedFiles > 0 {
		fmt.Fprintf(&b, "- **Files changed (the forge's count):** %d\n", c.ChangedFiles)
	} else {
		b.WriteString("- **Files changed (the forge's count):** 0, or not reported\n")
	}
	if len(c.Labels) > 0 {
		labels := make([]string, 0, len(c.Labels))
		for _, l := range c.Labels {
			labels = append(labels, packet.Code(l))
		}
		fmt.Fprintf(&b, "- **Labels:** %s\n", strings.Join(labels, ", "))
	}
	fmt.Fprintf(&b, "- **Last updated:** %s\n", codeOrNone(c.UpdatedAt))
	if c.URL != "" {
		fmt.Fprintf(&b, "- **URL:** %s\n", packet.Code(c.URL))
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
	fmt.Fprintf(&b, "States at head %s, as the forge reports them now. Which of them are required is not stated here.\n\n",
		packet.Code(head))
	fmt.Fprintf(&b, "- **Combined status:** %s\n", codeOrNone(checks.CombinedState))
	fmt.Fprintf(&b, "- **Commit statuses:** %d listed of %d\n", len(checks.Statuses), checks.StatusTotalCount)
	for i, s := range checks.Statuses {
		if i == reviewPacketMaxRows {
			fmt.Fprintf(&b, "  - … %d more not shown\n", len(checks.Statuses)-i)
			break
		}
		fmt.Fprintf(&b, "  - %s — %s\n", codeOrNone(s.Context), codeOrNone(s.State))
	}
	fmt.Fprintf(&b, "- **Check runs:** %d listed of %d\n", len(checks.CheckRuns), checks.CheckRunsTotalCount)
	for i, r := range checks.CheckRuns {
		if i == reviewPacketMaxRows {
			fmt.Fprintf(&b, "  - … %d more not shown\n", len(checks.CheckRuns)-i)
			break
		}
		state := codeOrNone(r.Status)
		if strings.TrimSpace(r.Conclusion) != "" {
			state += " / " + packet.Code(r.Conclusion)
		}
		fmt.Fprintf(&b, "  - %s — %s\n", codeOrNone(r.Name), state)
	}
	out.Text(b.String())
	return out, nil
}

// reviewLine is one review's facts on one line, every forge value in a code span. The
// commit is the one the forge records on the review; whether it equals the current head is
// stated as that comparison and nothing more.
func reviewLine(r deskkit.Review, head string) string {
	at := "the forge's record; not the current head"
	if r.CommitID == head {
		at = "the forge's record; equal to the current head"
	}
	return fmt.Sprintf("review %d — state %s — commit %s (%s) — submitted %s — by %s",
		r.ID, codeOrNone(r.State), codeOrNone(r.CommitID), at, codeOrNone(r.SubmittedAt), codeOrNone(r.Author.Login))
}

// reviewKind says, in the tool's words, what verdict line a review's body carries.
func reviewKind(r deskkit.Review) string {
	switch kind := reviewBodyLane(r.Body); kind {
	case "":
		return "no verdict line"
	case "both":
		return "both verdict lines"
	default:
		return kind + " lane"
	}
}

// reviewIndex writes one line per review, up to the row limit.
func reviewIndex(out *packet.Content, reviews []deskkit.Review, head string, note func(deskkit.Review) string) {
	var b strings.Builder
	for i, r := range reviews {
		if i == reviewPacketMaxRows {
			fmt.Fprintf(&b, "- … %d more not shown\n", len(reviews)-i)
			break
		}
		fmt.Fprintf(&b, "- %s — %s\n", reviewLine(r, head), note(r))
	}
	out.Text(b.String())
}

// reviewVerdictsSection lays out every review on the change. WHO POSTED A REVIEW DECIDES
// WHAT IT IS CALLED: a review is listed and counted as an earlier verdict only when the
// forge names the reviewer identity as its author. A review by any other account goes
// under its own heading, which says it is not a verdict, whatever its body says; a body
// there that has a verdict-shaped line is quoted between boundary lines so the reader can
// see what was claimed, and it is never counted. When the reviewer identity cannot be
// resolved the section fails closed: no review is listed as a verdict and no body is
// quoted.
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
	reviewer, known := reviewPacketReviewerFn()
	reviewer = strings.TrimSpace(reviewer)
	if !known || reviewer == "" {
		out.Textf("This dispatch is the **%s** lane (read off its claim key %s). **The reviewer identity could not be "+
			"resolved here, so NO review below is listed as an earlier verdict:** this packet cannot tell the reviewer's "+
			"reviews from anyone else's. Every review on the change is indexed, oldest first, one line each, with no body. "+
			"Read the earlier verdicts at the source.", lane, packet.Code(claimKey))
		out.Textf("### Reviews on the change — %d, none listed as a verdict", len(reviews))
		if len(reviews) == 0 {
			out.Text("_None._")
			return out, nil
		}
		reviewIndex(&out, reviews, head, reviewKind)
		return out, nil
	}

	out.Textf("This dispatch is the **%s** lane (read off its claim key %s). Every review on the change is below, oldest "+
		"first, under the author the forge reports. A review is listed as an earlier verdict only when that author is the "+
		"reviewer identity (%s); it is then filed under a lane by the verdict line in its body. A review by any other "+
		"account is not a verdict, whatever its body says, and is not counted. Nothing else about a review is checked "+
		"here — not its commit, and not whether it still counts.",
		lane, packet.Code(claimKey), packet.Code(reviewer))

	var mine, rest, others []deskkit.Review
	for _, r := range reviews {
		switch {
		case !byReviewer(r, reviewer):
			others = append(others, r)
		case reviewBodyLane(r.Body) == lane:
			mine = append(mine, r)
		default:
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
	out.Textf("### Other reviews by the reviewer identity — %d, one line each (the %s lane's among them)", len(rest), other)
	if len(rest) == 0 {
		out.Text("_None._")
	} else {
		reviewIndex(&out, rest, head, reviewKind)
	}
	out.Textf("### Reviews by other accounts — not verdicts — %d", len(others))
	if len(others) == 0 {
		out.Text("_None._")
		return out, nil
	}
	out.Text("None of these is a verdict of either lane and none is counted above. A body with a line shaped like a " +
		"verdict line is quoted so you can see what it says; the line has no effect.")
	reviewIndex(&out, others, head, func(r deskkit.Review) string {
		if reviewBodyLane(r.Body) == "" {
			return "no verdict line"
		}
		return "its body has a line shaped like a verdict line; it is not a verdict"
	})
	quoted := 0
	for _, r := range others {
		if reviewBodyLane(r.Body) == "" {
			continue
		}
		label := fmt.Sprintf("review %d body (another account's; not a verdict)", r.ID)
		if quoted == reviewPacketMaxOtherBodies {
			out.Omit(label, int64(len(r.Body)), fmt.Sprintf("past the %d-body limit for reviews by other accounts", reviewPacketMaxOtherBodies))
			continue
		}
		quoted++
		out.UntrustedCapped(label, []byte(r.Body), reviewPacketOtherBodyCap)
	}
	return out, nil
}

func reviewDiffSection(fg reviewPacketForge, fr deskkit.ForgeRepo, pr int, change *deskkit.PullRequest,
	changed func() ([]deskkit.ChangedFile, error)) (packet.Content, error) {
	var out packet.Content
	diff, derr := fg.ChangeDiff(fr, pr)
	if derr == nil && strings.TrimSpace(diff) != "" {
		out.Textf("The forge's unified diff of the whole change, base %s to head %s.", codeOrNone(change.BaseRef), codeOrNone(change.HeadSHA))
		out.UntrustedCapped("diff", []byte(diff), reviewPacketDiffCap)
		return out, nil
	}
	// why is the tool's own words; the forge's error text, when there is one, is a value.
	why, shown := "it returned an empty diff", "it returned an empty diff"
	if derr != nil {
		why = firstLine(derr.Error())
		shown = "its error: " + packet.Code(why)
	}
	files, ferr := changed()
	if ferr != nil {
		return out, fmt.Errorf("no whole diff (%s) and no per-file patches (%s)", why, firstLine(ferr.Error()))
	}
	out.Textf("The forge did not serve one diff for the whole change (%s). Its per-file patches follow instead; "+
		"a file the forge gave no patch for is listed as omitted.", shown)
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
	switch {
	case change.ChangedFiles <= 0 && len(files) > 0:
		// A count of zero beside a non-empty list is a count the forge did not report, not
		// agreement: say the list could not be checked rather than print "it counts 0".
		fmt.Fprintf(&b, "The forge lists %d touched file(s). **It reports no file count on the change, so whether "+
			"this list is the whole change is not known.**", len(files))
	case len(files) != change.ChangedFiles:
		fmt.Fprintf(&b, "The forge lists %d touched file(s); it counts %d on the change. **The two differ: this list "+
			"is not the whole change.** Read the rest at the source.", len(files), change.ChangedFiles)
	default:
		fmt.Fprintf(&b, "The forge lists %d touched file(s); it counts %d on the change.", len(files), change.ChangedFiles)
	}
	b.WriteString(" Each file still present at the head follows in full, as it reads AFTER the change.\n\n")
	for i, f := range files {
		if i == reviewPacketMaxRows {
			fmt.Fprintf(&b, "- … %d more not shown\n", len(files)-i)
			break
		}
		fmt.Fprintf(&b, "- %s — %s", codeOrNone(f.Filename), codeOrNone(f.Status))
		if f.PreviousFilename != "" && f.PreviousFilename != f.Filename {
			fmt.Fprintf(&b, " (was %s)", packet.Code(f.PreviousFilename))
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
			out.OmitDetail(name, packet.SizeUnknown, "could not be read", firstLine(rerr.Error()))
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
