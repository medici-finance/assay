package main

// packet_verifier.go — the verifier kit's dispatch packet (issue 2439).
//
// WHAT IT SAVES. A dispatched verifier makes a handful of reads before it runs its first
// row: the brief, its frontmatter gate and risk lines, the Verify table, a look at the
// history for the change that delivered the work. Measured on one adopter fleet that is
// about four tool calls a run, 7 to 16 percent of a run's requests. The packet hands those
// over in one file. It does not, and must not, shorten the rest: running the rows and
// reading the delivered work to judge them.
//
// THE RULE THAT SHAPES IT. A verifier is the control that turns "merged" into "verified",
// so nothing here may make a verdict easier to reach without running the rows. The packet
// therefore holds NOTHING that could be copied into an observed cell:
//
//   - the brief is carried only up to its Evidence heading. Earlier observed results, and
//     every section after them, are never copied — the packet says how large the Evidence
//     section is and stops there;
//   - a row contributes its `#` and Command cells, as the table writes them. Never a result,
//     never a prediction, never output cached from an earlier run. The dispatcher runs no
//     row and interprets no command;
//   - a commit is named by hash and changed paths. Its subject, message and date are not
//     copied: an evidence-landing commit's subject states an earlier verdict;
//   - a Verify table that itself has a result-like column (observed, exit, status, …) is not
//     a table this tool will quote. The provider declines and the dispatch carries on with
//     no packet.
//
// The cut at the Evidence heading is deliberately the FIRST such heading at any level and
// takes no account of code fences. That errs toward carrying less: a brief that quotes an
// "Evidence" heading in an example is cut early and the verifier reads the rest at the
// source, where a fence-aware cut could be talked past its real Evidence section.
//
// WHERE IT GOES. Outside the verifier home, always: an additional file in the home refuses
// the pre-work check (--check-verifier). packet.go already writes beside the prompt file or
// in the cache directory; this provider declines when the prompt file itself sits in the
// home.
//
// WHAT IT IS BOUND TO. The home's HEAD commit, read once and re-checked after the sections
// are built. The brief is read from that commit's tree, not from the working tree, so the
// packet describes exactly the commit its header records.
//
// IT STARTS NO PROCESS. Every read goes through internal/gitcore, in this process: no git
// binary, no hook, no helper, no configured diff or text-conversion program. This package's
// subprocess inventory (reviewguard_test.go) is therefore unchanged by this file.

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/packet"
)

func init() { registerPacketProvider("verifier", verifierPacket) }

const (
	// verifierPacketCommits bounds the commits listed; verifierPacketFiles bounds the changed
	// paths listed for one commit. Both are stated in the packet header.
	verifierPacketCommits = 6
	verifierPacketFiles   = 60

	// verifierPacketScan bounds how many commits back the history is searched for those
	// commits, and is stated in the packet header too. It is a COUNT so that the same home
	// gives the same packet however busy the machine is; about half a millisecond a commit
	// makes it roughly a second at the bound. verifierPacketBudget is the backstop behind it:
	// how long the search and the changed-path lists may take in all. A packet is a
	// convenience; neither may hold up a dispatch.
	verifierPacketScan   = 2000
	verifierPacketBudget = 10 * time.Second
)

var (
	vpCommitRe   = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	vpBriefIDRe  = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)
	vpEvidenceRe = regexp.MustCompile(`(?i)^#{1,6}[ \t]+evidence\b`)
	vpVerifyRe   = regexp.MustCompile(`(?i)^##[ \t]+verify\b`)
	vpSectionRe  = regexp.MustCompile(`^#{1,2}[ \t]+\S`)
	vpTableSepRe = regexp.MustCompile(`^\s*\|?[\s:|-]*-[\s:|-]*\|?\s*$`)
	vpWordRe     = regexp.MustCompile(`[a-z]+`)
)

// vpResultWords are the header words that mark a column as holding what a run OBSERVED. A
// Verify table carrying one is not quoted (see the file comment). The list is wider than
// any table this repository writes today on purpose: declining costs the verifier one read.
var vpResultWords = map[string]bool{
	"result": true, "results": true, "observed": true, "actual": true, "output": true,
	"exit": true, "status": true, "verdict": true, "outcome": true, "evidence": true,
	"runner": true, "date": true, "pass": true, "passed": true, "fail": true, "failed": true,
}

// verifierPacket is the provider: it resolves the commit and the brief, refuses the shapes
// described above, and returns the ordered sections. It reads the repository in-process and
// writes nothing.
func verifierPacket(in packetInput) (packet.Spec, error) {
	home := strings.TrimSpace(in.home)
	if home == "" {
		return packet.Spec{}, errors.New("no verifier home to read")
	}
	if pf := strings.TrimSpace(in.o.promptFile); pf != "" {
		if abs, err := filepath.Abs(pf); err == nil && vpWithin(home, abs) {
			return packet.Spec{}, errors.New("the prompt file sits inside the verifier home, and a packet beside it " +
				"would be an additional file there, which refuses the pre-work check")
		}
	}
	brief, err := vpBriefPath(in.o)
	if err != nil {
		return packet.Spec{}, err
	}
	repo, head, err := vpOpenAtHead(home)
	if err != nil {
		return packet.Spec{}, err
	}
	text, err := repo.FileAt(head, brief)
	if err != nil {
		return packet.Spec{}, fmt.Errorf("could not read %s at %s: %w", brief, head[:12], err)
	}
	doc := parseVerifierBrief(text)
	if col := doc.resultColumn(); col != "" {
		return packet.Spec{}, fmt.Errorf("the brief's Verify section has a table with a result-like column (%q); "+
			"this tool does not quote a table that may hold observed results", packet.Inline(col))
	}
	id := briefIDFromItem(in.o.item)
	if !vpBriefIDRe.MatchString(id) {
		id = ""
	}

	return packet.Spec{
		Head: head,
		CapNotes: []string{fmt.Sprintf("At most %d commits are listed, from a search of at most %d commits back, and "+
			"at most %d changed paths for one commit.", verifierPacketCommits, verifierPacketScan, verifierPacketFiles)},
		Sections: []packet.Section{
			packet.NewSection("Read this first: a reading aid, never evidence", func() (packet.Content, error) {
				var c packet.Content
				c.Textf("This packet was built by the dispatcher BEFORE any row ran. It holds no result of any "+
					"row, from this run or an earlier one. A row's result comes only from running the row at the "+
					"verified commit; nothing in this file is evidence, and no line of it belongs in an observed "+
					"cell. Run every row yourself.\n\n"+
					"- Brief: `%s`, read at the head commit above.\n"+
					"- If that commit is not the one you are verifying, say so and gather yourself.\n"+
					"- Where this file and the brief at the verified commit disagree, the brief governs.",
					packet.Inline(brief))
				return c, nil
			}),
			packet.NewSection("Gate and risk (frontmatter lines, as written)", func() (packet.Content, error) {
				var c packet.Content
				if len(doc.gateRisk) == 0 {
					c.Text("The brief's frontmatter at this commit has no `gate:`, `gate-why:` or `risk:` line. " +
						"An absent field is not a \"no\"; the kit says what it means.")
					return c, nil
				}
				c.Text("The brief's own `gate:`, `gate-why:` and `risk:` frontmatter lines. A field not shown " +
					"is absent at this commit, and an absent field is not a \"no\".")
				c.Untrusted("frontmatter gate and risk lines", []byte(strings.Join(doc.gateRisk, "\n")+"\n"))
				return c, nil
			}),
			packet.NewSection("Brief text before its Evidence section", func() (packet.Content, error) {
				var c packet.Content
				if strings.TrimSpace(doc.before) == "" {
					return c, errors.New("the brief has no text before its Evidence heading")
				}
				c.Text("Everything from the brief's first line up to, and not including, its first Evidence " +
					"heading. The Expect cells in it are the brief's pass criteria, not observations.")
				c.Untrusted(brief+" (before Evidence)", []byte(doc.before))
				return c, nil
			}),
			packet.NewSection("Verify rows: command text only", func() (packet.Content, error) {
				return doc.rowCommands(), nil
			}),
			packet.NewSection("Earlier Evidence: its size, never its rows", func() (packet.Content, error) {
				return doc.evidenceIndex(), nil
			}),
			packet.NewSection("Commits that changed or name this brief", func() (packet.Content, error) {
				return verifierPacketCommitsSection(repo, head, brief, id, verifierPacketScan)
			}),
		},
		Recheck: func() error {
			// Opened afresh, so nothing read while building can answer for the home now.
			_, now, err := vpOpenAtHead(home)
			if err != nil {
				return fmt.Errorf("could not re-read the verifier home's commit: %w", err)
			}
			if now != head {
				return fmt.Errorf("the verifier home moved from %s to %s while the packet was being built",
					head[:12], packet.Inline(now))
			}
			return nil
		},
	}, nil
}

// vpOpenAtHead opens the verifier home and returns it with the full id of its HEAD commit.
func vpOpenAtHead(home string) (*gitcore.Repo, string, error) {
	repo, err := gitcore.Open(home)
	if err != nil {
		return nil, "", fmt.Errorf("could not open the verifier home as a repository: %w", err)
	}
	hash, err := repo.Resolve("HEAD")
	if err != nil {
		return nil, "", fmt.Errorf("could not read the verifier home's commit: %w", err)
	}
	head := hash.String()
	if !vpCommitRe.MatchString(head) {
		return nil, "", fmt.Errorf("the verifier home's commit %q is not a full commit id", packet.Inline(head))
	}
	if _, err := repo.ParentHashes(head); err != nil {
		return nil, "", fmt.Errorf("the verifier home's HEAD %s is not a readable commit: %w", head[:12], err)
	}
	return repo, head, nil
}

// vpBriefPath returns the brief as a slash-separated path relative to --root: the path the
// verifier home, a worktree of the same repository, carries it at. A brief that is not
// inside the dispatched repository has no content at the verified commit to read.
func vpBriefPath(o dispatchOpts) (string, error) {
	b := strings.TrimSpace(o.brief)
	if b == "" {
		return "", errors.New("the dispatch names no brief")
	}
	if filepath.IsAbs(b) {
		rel, err := filepath.Rel(resolvePath(o.root), resolvePath(b))
		if err != nil {
			return "", fmt.Errorf("the brief path does not relate to --root: %w", err)
		}
		b = rel
	}
	b = filepath.ToSlash(filepath.Clean(b))
	if b == "." || b == ".." || strings.HasPrefix(b, "../") || strings.HasPrefix(b, "/") || strings.HasPrefix(b, "-") ||
		strings.ContainsAny(b, ":\n\r\x00") {
		return "", errors.New("the brief is not a file inside the dispatched repository, so there is nothing to " +
			"read at the verified commit")
	}
	return b, nil
}

// vpWithin reports whether path is dir or sits under it, comparing symlink-resolved paths.
func vpWithin(dir, path string) bool {
	rel, err := filepath.Rel(resolvePath(dir), resolvePath(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// verifierBrief is a brief split for the packet. Only `before` and what is derived from it
// is ever written; `evidence` is measured and never quoted.
type verifierBrief struct {
	before    string   // the text ahead of the first Evidence heading
	evidence  []string // the Evidence section's lines, heading included; counted only
	following int      // sections after the Evidence section, counted only
	hasEv     bool
	gateRisk  []string
	hasVerify bool
	tables    []vpTable // every table in the Verify section, in order
}

// vpTable is one Markdown table: its header cells and each row's cells.
type vpTable struct {
	header []string
	rows   [][]string
}

func parseVerifierBrief(text string) verifierBrief {
	var doc verifierBrief
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	cut := len(lines)
	for i, l := range lines {
		if vpEvidenceRe.MatchString(l) {
			cut, doc.hasEv = i, true
			break
		}
	}
	before := lines[:cut]
	doc.before = strings.Join(before, "\n")
	if doc.hasEv {
		level := len(lines[cut]) - len(strings.TrimLeft(lines[cut], "#"))
		end := len(lines)
		for i := cut + 1; i < len(lines); i++ {
			if n := len(lines[i]) - len(strings.TrimLeft(lines[i], "#")); n >= 1 && n <= level &&
				strings.HasPrefix(strings.TrimLeft(lines[i], "#"), " ") {
				if end == len(lines) {
					end = i
				}
				doc.following++
			}
		}
		doc.evidence = lines[cut:end]
	}
	doc.gateRisk = vpGateRiskLines(doc.before)

	start := -1
	for i, l := range before {
		if vpVerifyRe.MatchString(l) {
			start = i
			break
		}
	}
	if start < 0 {
		return doc
	}
	doc.hasVerify = true
	end := len(before)
	for i := start + 1; i < len(before); i++ {
		if vpSectionRe.MatchString(before[i]) {
			end = i
			break
		}
	}
	section := before[start+1 : end]
	for i := 0; i+1 < len(section); i++ {
		if !strings.HasPrefix(strings.TrimSpace(section[i]), "|") || !vpTableSepRe.MatchString(section[i+1]) {
			continue
		}
		t := vpTable{header: vpSplitRow(section[i])}
		j := i + 2
		for ; j < len(section) && strings.HasPrefix(strings.TrimSpace(section[j]), "|"); j++ {
			t.rows = append(t.rows, vpSplitRow(section[j]))
		}
		doc.tables = append(doc.tables, t)
		i = j - 1
	}
	return doc
}

// vpGateRiskLines returns the frontmatter's top-level `gate:`, `gate-why:` and `risk:`
// lines as written, each with the indented lines that continue it (a block-style value).
func vpGateRiskLines(text string) []string {
	var out []string
	keep := false
	for _, l := range strings.Split(briefFrontmatterBlock(text), "\n") {
		switch {
		case strings.HasPrefix(l, "gate:"), strings.HasPrefix(l, "gate-why:"), strings.HasPrefix(l, "risk:"):
			keep = true
		case strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t"):
			// a continuation of whichever key came last
		default:
			keep = false
		}
		if keep && strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// vpSplitRow splits one Markdown table row into its cells. A pipe written `\|` is part of
// its cell and is left exactly as written — this tool copies a cell, it does not decode it.
func vpSplitRow(line string) []string {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == '|':
			cur.WriteString(`\|`)
			i++
		case s[i] == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(s[i])
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		cells = append(cells, strings.TrimSpace(cur.String()))
	}
	return cells
}

// vpHeaderKey folds a header cell for comparison: lower case, emphasis and code marks off.
func vpHeaderKey(cell string) string {
	return strings.ToLower(strings.TrimSpace(strings.Trim(strings.TrimSpace(cell), "*_`")))
}

// resultColumn names the first header cell, in any table of the Verify section, that reads
// as a column of observed results; "" when there is none.
func (d verifierBrief) resultColumn() string {
	for _, t := range d.tables {
		for _, h := range t.header {
			for _, w := range vpWordRe.FindAllString(strings.ToLower(h), -1) {
				if vpResultWords[w] {
					return h
				}
			}
		}
	}
	return ""
}

// rowCommands is the "command text only" section: each row's `#` cell and Command cell,
// copied as the table writes them, and the row count. It writes no Expect cell. When a row
// does not have its header's number of cells (an unescaped pipe inside a cell does that)
// the tool cannot say where the command ends, so it lists nothing rather than guess.
func (d verifierBrief) rowCommands() packet.Content {
	var c packet.Content
	if !d.hasVerify {
		c.Text("No `## Verify` heading was found ahead of the brief's Evidence heading at this commit. Read the brief.")
		c.Omit("Verify row commands", packet.SizeUnknown, "no Verify section found before the Evidence heading")
		return c
	}
	var list strings.Builder
	rows := 0
	for _, t := range d.tables {
		num, cmd := -1, -1
		for i, h := range t.header {
			switch vpHeaderKey(h) {
			case "#":
				num = i
			case "command":
				cmd = i
			}
		}
		if num < 0 || cmd < 0 {
			continue
		}
		for _, r := range t.rows {
			rows++
			if len(r) != len(t.header) {
				c.Textf("The Verify table has a row whose cells do not match its header (row %d of the table; an "+
					"unescaped `|` inside a cell does that). The tool will not guess where a command ends, so "+
					"no command list is given. Read the table in the brief text above.", rows)
				c.Omit("Verify row commands", packet.SizeUnknown, "a row's cells do not match the table header")
				return c
			}
			fmt.Fprintf(&list, "[%s] %s\n", r[num], r[cmd])
		}
	}
	if rows == 0 {
		c.Text("No table with a `#` and a `Command` column was found in the Verify section at this commit. Read the brief.")
		c.Omit("Verify row commands", packet.SizeUnknown, "no row table found in the Verify section")
		return c
	}
	c.Textf("The Verify section has %d row(s) at this commit; your Evidence needs one row for each. Below is each "+
		"row's `#` cell and its Command cell, copied character for character (a pipe written `\\|` is left as the "+
		"table writes it). The dispatcher ran none of them. There is no Expect cell and no result here; a command "+
		"quoted here is not a command that was run.", rows)
	c.Untrusted("Verify row commands (the # and Command cells only)", []byte(list.String()))
	return c
}

// evidenceIndex says that an Evidence section exists and how large it is. It quotes none of
// it: no row, no date, no heading text.
func (d verifierBrief) evidenceIndex() packet.Content {
	var c packet.Content
	if !d.hasEv {
		c.Text("The brief has no Evidence heading at this commit.")
		return c
	}
	size, tableLines := 0, 0
	for _, l := range d.evidence {
		size += len(l) + 1
		if strings.HasPrefix(strings.TrimSpace(l), "|") && !vpTableSepRe.MatchString(l) {
			tableLines++
		}
	}
	c.Textf("The brief has an Evidence section at this commit: %d line(s), %d of them table lines. None of it is "+
		"copied here. What an earlier run observed is not a result of this run; read the section in the brief if "+
		"you need to see what was recorded or where your rows go. %d further section(s) follow it and are not "+
		"copied either.", len(d.evidence), tableLines, d.following)
	c.Omit("the Evidence section and the sections after it", int64(size),
		"earlier observed results are never copied into a packet")
	return c
}

// vpCommit is one listed commit and why it is listed.
type vpCommit struct {
	sha     string
	parents []string
	why     []string
}

// verifierPacketCommitsSection lists the newest commits, reachable from head, that changed
// the brief file or whose message carries a `Brief: <id>` line, each with the paths it
// changed against its first parent. The history is read in ONE walk, which ends at the
// commit that added the brief file, at the listed-commit limit, at the search bound or when
// the time allowed is spent, whichever comes first. scan is the search bound, a parameter so
// a test can reach it without building thousands of commits.
func verifierPacketCommitsSection(repo *gitcore.Repo, head, brief, id string, scan int) (packet.Content, error) {
	var c packet.Content
	var commits []vpCommit
	started := time.Now()
	searched, stopped := 0, ""
	err := repo.PathHistory(head, brief, func(pc gitcore.PathCommit) bool {
		if searched == scan {
			stopped = fmt.Sprintf("the history search stops after %d commits", searched)
			return false
		}
		if time.Since(started) > verifierPacketBudget {
			stopped = "the time allowed for this section ran out"
			return false
		}
		searched++
		k := vpCommit{sha: pc.Hash, parents: pc.Parents}
		changed, added := vpChangedBrief(pc)
		if changed {
			k.why = append(k.why, "changed the brief file")
		}
		if id != "" && vpNamesBrief(pc.Message, id) {
			k.why = append(k.why, "its message has a `Brief: "+id+"` line")
		}
		if len(k.why) > 0 {
			commits = append(commits, k)
		}
		// Nothing older than the commit that added the brief file is searched, and nothing
		// past the limit is listed.
		return !added && len(commits) < verifierPacketCommits
	})
	if err != nil {
		// A clone whose history stops short (a shallow one) still yields what was read
		// before the gap; the packet says the search ended there.
		stopped = "the history could not be read further in this clone"
	}
	if stopped != "" {
		c.Omit("commits older than the "+fmt.Sprint(searched)+" searched", packet.SizeUnknown, stopped)
	}
	if len(commits) == 0 {
		c.Text("No commit searched changed the brief file or names it. Find the change that delivered this " +
			"item yourself.")
		return c, nil
	}
	c.Textf("The dispatcher does not know which commit delivered this item's work. These are the newest commits "+
		"reachable from the head commit that changed the brief file, or whose message names the brief: at most "+
		"%d, newest first, each with the paths it changed against its first parent. The search goes no further "+
		"back than the commit that added the brief file. No commit subject, message or date is copied, because "+
		"a message may state an earlier verdict and that is not evidence. Decide for yourself which change "+
		"delivered the work, and read its diff at the source.", verifierPacketCommits)
	for _, k := range commits {
		kind := ""
		if len(k.parents) > 1 {
			kind = "; a merge commit"
		}
		c.Textf("- `%s`: %s%s", k.sha, strings.Join(k.why, " and "), kind)
		label := "paths changed by " + k.sha[:12]
		if len(k.parents) == 0 {
			c.Omit(label, packet.SizeUnknown, "the commit has no parent to compare against")
			continue
		}
		if time.Since(started) > verifierPacketBudget {
			c.Omit(label, packet.SizeUnknown, "the time allowed for this section ran out")
			continue
		}
		paths, err := repo.DiffNames(k.parents[0], k.sha)
		if err != nil {
			c.Omit(label, packet.SizeUnknown, "its first parent could not be read in this clone")
			continue
		}
		if len(paths) == 0 {
			c.Text("  It changed no path against its first parent.")
			continue
		}
		names := strings.Join(paths, "\n")
		if len(paths) > verifierPacketFiles {
			c.Omit(label, int64(len(names)), fmt.Sprintf("%d paths, over the %d-path limit", len(paths), verifierPacketFiles))
			continue
		}
		c.Untrusted(label, []byte(names+"\n"))
	}
	return c, nil
}

// vpChangedBrief reports whether the brief file at a commit differs from what EVERY parent
// carries — so a merge that only brings an already-listed change across is not listed a
// second time — and whether the commit ADDED it: it carries the file and no parent does. A
// parentless commit changed, and added, the file when it carries it.
func vpChangedBrief(pc gitcore.PathCommit) (changed, added bool) {
	if len(pc.ParentPathIDs) == 0 {
		return pc.PathID != "", pc.PathID != ""
	}
	changed, added = true, pc.PathID != ""
	for _, p := range pc.ParentPathIDs {
		if p == pc.PathID {
			changed = false
		}
		if p != "" {
			added = false
		}
	}
	return changed, added
}

// vpNamesBrief reports whether a commit message has a line that is exactly `Brief: <id>`,
// trailing space aside. The message is read to answer that one question and is not kept.
func vpNamesBrief(msg, id string) bool {
	want := "Brief: " + id
	for _, l := range strings.Split(msg, "\n") {
		if strings.TrimRight(l, " \t\r") == want {
			return true
		}
	}
	return false
}
