package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// The brief ruling-link lane: a pull request that ADDS or CHANGES a brief's
// frontmatter ruling: link (or the decision-issue: it points into) has that link
// resolved on the forge, whatever the brief's gate or status. The decision-gate
// hold resolves a ruling only when the change moves, relabels or drops a
// gate: human brief; a ruling link recorded in a change of its own was read by
// no one, so a bot's, an edited or a missing comment could stand in the brief as
// a human ruling. Resolution is the hold's own (resolveBriefRuling →
// resolveRulingOn): one resolver, one set of named reasons (registers-v1 §7.5).
// A link that cannot be resolved is refused, never skipped.

// briefRulingSpecRef is the citation every refusal of this lane carries.
const briefRulingSpecRef = "lifecycle-v1 §4.5; registers-v1 §7.5"

// briefRulingTouch is one brief file the change touches, read on both sides.
type briefRulingTouch struct {
	Path    string // repo-relative brief file after the change
	Board   string // <stream>/<NN> from the path
	Head    gateBrief
	HeadErr string // the brief after the change could not be read ("" = read)
	Base    *gateBrief
	BaseErr string // the brief at the base could not be read ("" = read or absent)
}

// briefRulingBoardID is the board id a brief file's path spells, or "" when the
// path is not a brief file on the board (gateBoardPathRe).
func briefRulingBoardID(rel string) string {
	m := gateBoardPathRe.FindStringSubmatch(rel)
	if m == nil || m[3] == "README.md" {
		return ""
	}
	bm := briefNameRe.FindStringSubmatch(filepath.Base(rel))
	if bm == nil {
		return ""
	}
	return m[2] + "/" + bm[1]
}

// briefRulingTouched lists the brief files the working tree at root changes
// against base, by git's own diff (renames split into their halves), with each
// side's frontmatter read. deleted holds the base side of every brief file the
// change removes, so a file moved (into done/, or renumbered) with its ruling
// unchanged is not read as a new ruling.
func briefRulingTouched(root, base string) (touched []briefRulingTouch, deleted []gateBrief, err error) {
	out, err := exec.Command("git", "-C", root, "diff", "--name-status", "--no-renames", "-z",
		base, "--", "docs/streams", "docs/archive").Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git diff --name-status %s: %w", base, err)
	}
	fields := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	readBase := func(rel string) (*gateBrief, string) {
		raw, err := gitShowObject(root, base, rel)
		if err != nil {
			return nil, fmt.Sprintf("git show %s:%s: %v", base, rel, err)
		}
		gb := &gateBrief{}
		readGateFrontmatter(gb, raw)
		return gb, ""
	}
	for i := 0; i+1 < len(fields); i += 2 {
		status, rel := fields[i], fields[i+1]
		board := briefRulingBoardID(rel)
		if board == "" {
			continue
		}
		if strings.HasPrefix(status, "D") {
			if b, why := readBase(rel); why == "" {
				deleted = append(deleted, *b)
			}
			continue
		}
		t := briefRulingTouch{Path: rel, Board: board}
		fi, lerr := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		switch {
		case lerr != nil:
			t.HeadErr = lerr.Error()
		case !fi.Mode().IsRegular():
			t.HeadErr = "not a plain file"
		default:
			raw, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if rerr != nil {
				t.HeadErr = rerr.Error()
			} else {
				readGateFrontmatter(&t.Head, raw)
			}
		}
		if !strings.HasPrefix(status, "A") {
			t.Base, t.BaseErr = readBase(rel)
		}
		touched = append(touched, t)
	}
	sort.Slice(touched, func(i, j int) bool { return touched[i].Path < touched[j].Path })
	return touched, deleted, nil
}

// sameBriefRuling reports whether two readings of a brief record the same
// ruling link into the same decision issue.
func sameBriefRuling(a, b gateBrief) bool {
	return a.Ruling == b.Ruling && a.DecisionIssue == b.DecisionIssue
}

// briefRulingLane judges every ruling link the working tree at root adds or
// changes against base. Each such link must pass layer one (a decision issue
// recorded, a well-formed issue-comment URL on it) and then resolve on the
// forge through resolveBriefRuling. The forge client, login map and aliases are
// read only when a link reaches layer two.
func briefRulingLane(root, repo, base string) []decisionGateResult {
	touched, deleted, err := briefRulingTouched(root, base)
	if err != nil {
		return []decisionGateResult{{Board: "(briefs)", Refused: true,
			Detail: fmt.Sprintf("the brief files this change touches could not be listed against the base %s (%v), so no added or changed ruling link can be judged — fail-closed (%s)", base, err, briefRulingSpecRef)}}
	}
	var (
		out     []decisionGateResult
		c       *ghClient
		logins  map[string]string
		aliases []string
		ready   bool
	)
	refuse := func(t briefRulingTouch, format string, a ...any) {
		out = append(out, decisionGateResult{Board: t.Board, Refused: true,
			Detail: fmt.Sprintf("%s: ", t.Path) + fmt.Sprintf(format, a...) + " (" + briefRulingSpecRef + ")"})
	}
	for _, t := range touched {
		if t.HeadErr != "" {
			refuse(t, "the brief file could not be read (%s), so whether this change adds a ruling link cannot be judged — fail-closed", t.HeadErr)
			continue
		}
		if t.Head.FMErr {
			refuse(t, "the brief's frontmatter does not parse, so whether this change adds a ruling link cannot be judged — fail-closed")
			continue
		}
		if t.Head.Ruling == "" {
			continue // no ruling cited after the change: nothing to resolve
		}
		if t.BaseErr != "" {
			// An unreadable base cannot show the link was already there: judge it.
			fmt.Fprintf(os.Stderr, "statusgen: brief ruling: %s: base unreadable (%s) — judging its ruling link as added\n", t.Path, t.BaseErr)
		} else if t.Base != nil && !t.Base.FMErr && sameBriefRuling(*t.Base, t.Head) {
			continue
		} else if t.Base == nil {
			moved := false
			for _, d := range deleted {
				if !d.FMErr && sameBriefRuling(d, t.Head) {
					moved = true
					break
				}
			}
			if moved {
				continue
			}
		}
		if t.Head.DecisionIssue == 0 {
			refuse(t, "this change cites ruling: %s, but no decision issue is recorded (frontmatter decision-issue: is absent), so the link has no issue to rule on", t.Head.Ruling)
			continue
		}
		link, ok := parseRulingURL(t.Head.Ruling)
		if !ok {
			refuse(t, "this change cites ruling: %q, which is not an issue-comment URL of the form %s — %s", t.Head.Ruling, rulingURLGrammar, rulingMalformed)
			continue
		}
		if link.Issue != t.Head.DecisionIssue {
			refuse(t, "this change cites ruling: %s, a comment on #%d, not on the recorded decision issue #%d — %s", t.Head.Ruling, link.Issue, t.Head.DecisionIssue, rulingUnrelatedIssue)
			continue
		}
		if repo == "" {
			refuse(t, "this change cites ruling: %s; the repository could not be determined from the origin remote, so the link cannot be resolved — %s", t.Head.Ruling, rulingUnresolvable)
			continue
		}
		if !ready {
			c, logins, aliases = decisionGateClientFn(), decisionGateLoginsFn(), decisionGateAliases(root, base, repo)
			ready = true
		}
		var briefIDs []string
		if t.Head.BriefID != "" {
			briefIDs = []string{t.Head.BriefID}
		}
		o := resolveBriefRuling(c, repo, t.Path, t.Head.Ruling, []string{t.Board}, briefIDs, logins, aliases)
		if o.Present && o.Reason == rulingOK {
			out = append(out, decisionGateResult{Board: t.Board,
				Detail: fmt.Sprintf("%s: ruling: link added or changed by this change, ruled by %s (%s)", t.Path, o.Author, o.Detail)})
			continue
		}
		refuse(t, "this change cites ruling: %s, which does not stand as a human ruling on decision issue #%d: %s — %s", t.Head.Ruling, t.Head.DecisionIssue, o.Reason, o.Detail)
	}
	return out
}
