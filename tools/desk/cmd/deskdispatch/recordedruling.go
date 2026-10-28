package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A RECORDED RULING (#2079). A gate: human brief whose human has already ruled carries the
// ruling in its own text, transcribed as a decision record:
//
//	**Ruling (2026-09-30): option 1.** … transcribed as [DR-example](../decisions/DR-example.md) …
//
// Before this, the decision gate always ran the consumer's decision script with `ensure`, which
// derives a decision issue from the brief — and a consumer script may refuse a brief whose
// decision section now carries a link, so an item whose human HAD ruled could not be dispatched
// at all. The recorded ruling is now a second way through the gate: the linked record must exist
// in the brief's own checkout and its ruling must CORROBORATE, AS A RULING ON THIS BRIEF, through
// `statusgen --corroborate --ruling-record <record> --ruling-brief <brief>` — the forge check
// (registers-v1 §7.5) the PR lane applies to every decision record (an unedited, non-bot comment
// by a mapped human, naming the record), plus the decision-gate hold's binding to the brief: the
// comment's decision issue must carry THIS brief's decision-gate marker and the comment must name
// the brief. A record ruled for another brief does not pass. Anything short of that refuses,
// naming the line.
//
// What is checked is provenance — THAT the brief's human ruled — never WHAT they ruled: the
// option the line names is parsed for its shape and not compared with the record (the record
// carries no chosen-option field). The decision-gate hold confirms its rulings the same way.
//
// Scope, deliberately narrow so no brief that dispatched before changes path:
//   - a candidate is a line opening `**Ruling (` (an optional list marker first), outside the
//     frontmatter and outside fenced code; `**Ruling recorded …**`, `**Ruling relayed …**` and
//     prose mentions are not candidates;
//   - a candidate that links NO decision record leaves the gate exactly as it was (the script
//     runs);
//   - a candidate that links one is held to the full grammar and the full check.

// rulingCandidateRe opens a candidate line (after any list marker is stripped).
var rulingCandidateRe = regexp.MustCompile(`^\*\*Ruling\s*\(`)

// rulingLineRe is the one accepted shape: `**Ruling (<YYYY-MM-DD>): option <N>.**`, where N is a
// number or a single option letter.
var rulingLineRe = regexp.MustCompile(`^\*\*Ruling \((\d{4}-\d{2}-\d{2})\): option ([0-9]+|[A-Za-z])\.\*\*`)

// rulingListMarkerRe strips one leading list marker.
var rulingListMarkerRe = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+`)

// mdLinkRe matches an inline markdown link and captures its target (an optional quoted title
// after the target is allowed and ignored).
var mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)`)

// decisionsDirRel is the decision register this gate reads at runtime; the corpus guard lists
// its root as an operational stream for that reason.
const decisionsDirRel = "docs/streams/decisions"

// recordedRuling is a brief's recorded ruling, parsed offline before the claim.
type recordedRuling struct {
	lineNo   int    // 1-based line number in the brief
	line     string // the line as written, trimmed
	repoRoot string // the checkout the brief and the record live in
	brief    string // the brief's path relative to repoRoot — what the ruling must rule on
	record   string // the record's repo-relative path, docs/streams/decisions/<name>.md
}

// where names the line for every report and refusal.
func (r *recordedRuling) where(brief string) string {
	return fmt.Sprintf("%s line %d %q", brief, r.lineNo, clipLine(r.line))
}

func clipLine(s string) string {
	const max = 160
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// briefRepoRoot is the checkout a brief lives in: the parent of the nearest `docs/streams`
// ancestor of its directory, or "" when the brief is not under one.
func briefRepoRoot(brief string) string {
	for d := filepath.Dir(brief); ; {
		if filepath.Base(d) == "streams" && filepath.Base(filepath.Dir(d)) == "docs" {
			return filepath.Dir(filepath.Dir(d))
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// readRecordedRuling parses brief for a recorded ruling. It returns (nil, nil) when the brief
// carries no candidate line that links a decision record — the gate then runs as before. A
// candidate that links one but cannot be accepted is REFUSED, the refusal naming the line. An
// shown is the brief as reports spell it (briefArg). An
// unreadable brief returns (nil, nil): the brief was already resolved, and the script path
// (unchanged) is what reports on it.
func readRecordedRuling(root, brief, shown string) (*recordedRuling, error) {
	path := brief
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, brief)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	repoRoot := briefRepoRoot(path)
	var found []*recordedRuling
	var refusals []string
	inFence, fence := false, ""
	lines := strings.Split(string(raw), "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}
	for i := start; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if f := fenceOpen(t); f != "" {
			switch {
			case !inFence:
				inFence, fence = true, f
				continue
			case strings.HasPrefix(t, fence) && strings.TrimLeft(t, fence[:1]) == "":
				inFence = false
				continue
			}
		}
		if inFence {
			continue
		}
		body := rulingListMarkerRe.ReplaceAllString(t, "")
		if !rulingCandidateRe.MatchString(body) {
			continue
		}
		r := &recordedRuling{lineNo: i + 1, line: t}
		records, why := linkedRecords(path, repoRoot, body)
		if len(records) == 0 && why == "" {
			continue // links no decision record: not a recorded ruling, the gate is unchanged
		}
		switch {
		case why != "":
		case len(records) > 1:
			why = fmt.Sprintf("it links %d decision records (%s) — a recorded ruling transcribes to exactly one", len(records), strings.Join(records, ", "))
		case !rulingLineRe.MatchString(body):
			why = "it does not open with the accepted shape `**Ruling (<YYYY-MM-DD>): option <N>.**`"
		default:
			m := rulingLineRe.FindStringSubmatch(body)
			if _, derr := time.Parse("2006-01-02", m[1]); derr != nil {
				why = fmt.Sprintf("its date %q is not a calendar date", m[1])
			}
		}
		if why == "" {
			r.repoRoot, r.record = repoRoot, records[0]
			if rel, rerr := filepath.Rel(repoRoot, path); rerr == nil {
				r.brief = filepath.ToSlash(rel)
			}
			full := filepath.Join(repoRoot, filepath.FromSlash(r.record))
			if fi, serr := os.Stat(full); serr != nil || !fi.Mode().IsRegular() {
				why = fmt.Sprintf("the decision record it links, %s, does not exist in %s", r.record, repoRoot)
			}
		}
		if why != "" {
			refusals = append(refusals, fmt.Sprintf("%s: %s", r.where(shown), why))
			continue
		}
		found = append(found, r)
	}
	if len(refusals) > 0 {
		return nil, deskkit.Refused(fmt.Sprintf(
			"step %s: the brief records a ruling this gate cannot accept — %s. Nothing was claimed.",
			stepDecisionGate, strings.Join(refusals, "; ")))
	}
	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return found[0], nil
	}
	var ws []string
	for _, r := range found {
		ws = append(ws, r.where(shown))
	}
	return nil, deskkit.Refused(fmt.Sprintf(
		"step %s: the brief records more than one ruling (%s) — which one stands cannot be read from it. "+
			"Keep one `**Ruling (<date>): option <N>.**` line. Nothing was claimed.",
		stepDecisionGate, strings.Join(ws, "; ")))
}

// fenceOpen returns the fence run (``` or ~~~, three or more) a trimmed line opens with, or "".
func fenceOpen(t string) string {
	for _, c := range []string{"`", "~"} {
		if strings.HasPrefix(t, c+c+c) {
			n := len(t) - len(strings.TrimLeft(t, c))
			return strings.Repeat(c, n)
		}
	}
	return ""
}

// linkedRecords returns the repo-relative decision-record paths the line links. why is non-empty
// when a link aims at the decision register but cannot be taken as an in-checkout record: by URL
// (the file on disk is the one corroborated, so the link must name it), from a brief outside any
// docs/streams checkout, or resolving outside the register.
func linkedRecords(briefPath, repoRoot, line string) (records []string, why string) {
	seen := map[string]bool{}
	for _, m := range mdLinkRe.FindAllStringSubmatch(line, -1) {
		target := m[1]
		if u, err := url.Parse(target); err == nil && (u.Scheme != "" || u.Host != "") {
			if strings.Contains(u.Path, "/"+decisionsDirRel+"/") {
				return nil, fmt.Sprintf("it links the decision record %s by URL — link it by its path in this checkout so the record corroborated is the one on disk", target)
			}
			continue
		}
		if i := strings.IndexAny(target, "#?"); i >= 0 {
			target = target[:i]
		}
		if p, err := url.PathUnescape(target); err == nil {
			target = p
		}
		if target == "" || !strings.HasSuffix(strings.ToLower(target), ".md") {
			continue
		}
		var abs string
		if strings.HasPrefix(target, "/") {
			if repoRoot == "" {
				continue
			}
			abs = filepath.Join(repoRoot, filepath.FromSlash(target))
		} else {
			abs = filepath.Join(filepath.Dir(briefPath), filepath.FromSlash(target))
		}
		abs = filepath.Clean(abs)
		if filepath.Base(filepath.Dir(abs)) != "decisions" {
			continue // not a decision-register link
		}
		if repoRoot == "" {
			return nil, "it links a decision record, but the brief is not under a docs/streams directory, so the record's checkout cannot be named"
		}
		rel, err := filepath.Rel(repoRoot, abs)
		rel = filepath.ToSlash(rel)
		if err != nil || filepath.Dir(rel) != decisionsDirRel || strings.EqualFold(filepath.Base(rel), "README.md") {
			return nil, fmt.Sprintf("its link %s does not resolve to a record in %s/ of the brief's own checkout", m[1], decisionsDirRel)
		}
		if !seen[rel] {
			seen[rel] = true
			records = append(records, rel)
		}
	}
	return records, ""
}

// statusgenBinary is the corroborating tool's name on PATH.
const statusgenBinary = "statusgen"

// checkRecordedRuling corroborates a recorded ruling as a ruling on THIS brief through `statusgen
// --corroborate --ruling-record --ruling-brief`, under the credential the claim step resolved
// (env, minted for --repo). Exit 0 passes; exit 1 is the ruling failing a condition (REFUSED,
// naming the line and the repository the credential was minted for, since a credential that
// cannot read the brief's checkout reads a private record's ruling as deleted); anything else —
// the tool missing, a forge read failing — is a could-not-check (UNVERIFIABLE). Never a pass by
// default.
func checkRecordedRuling(o dispatchOpts, r *recordedRuling, env []string) (string, error) {
	bin, err := lookPath(statusgenBinary)
	if err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: %s records a ruling, but %s is not on PATH, so the ruling cannot be corroborated — this "+
				"refuses rather than take the ruling on the brief's word", stepDecisionGate, r.where(briefArg(o)), statusgenBinary), err)
	}
	res := runCmdEnv(r.repoRoot, env, bin, "--root", r.repoRoot, "--corroborate", "--ruling-record", r.record, "--ruling-brief", r.brief)
	said := firstLine(res.stdout)
	if strings.TrimSpace(res.stdout) == "" {
		said = res.run.Said()
	}
	if res.err == nil {
		return fmt.Sprintf("OK: ruling recorded — %s, transcribed as %s, corroborated as a human ruling on this brief "+
			"(provenance only; the option it names is not compared) (%s); no decision issue ensured",
			r.where(briefArg(o)), r.record, said), nil
	}
	if exitCodeOf(res.err) == 1 {
		return "", deskkit.Refused(fmt.Sprintf(
			"step %s: %s records a ruling transcribed as %s, but that record's ruling does not corroborate as a "+
				"ruling on this brief: %s (read under the credential for %s). Fix the record's ruling: link, rule on "+
				"this brief on its decision issue, or remove the line to take the decision-issue path.",
			stepDecisionGate, r.where(briefArg(o)), r.record, said, o.repo))
	}
	return "", res.run.FailVerbatim(deskkit.ExitUnverifiable, fmt.Sprintf(
		"step %s: %s records a ruling transcribed as %s, and corroborating it could not be completed (%s) — "+
			"this fails closed", stepDecisionGate, r.where(briefArg(o)), r.record, res.run.Said()))
}
