package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// `statusgen --corroborate --ruling-record <record> --ruling-brief <brief>`
// corroborates ONE decision record's human ruling, outside a pull request, AS A
// RULING ON ONE gate: human BRIEF (#2079).
//
// The PR mode of --corroborate gates the records a PR touches, and a real
// decided-by name there can corroborate against the PR's own reviews. A caller
// that holds no PR — a dispatcher checking that a gate: human brief's recorded
// ruling stands before it starts work — has no such anchor. The only anchor left
// is the record's ruling: link (registers-v1 §7.5), so this mode demands one and
// resolves it through resolveRulingOn, the check every PR-mode record and the
// decision-gate hold go through: same link grammar, same repository, an
// unedited non-bot comment by a mapped human that names the record.
//
// What it adds is the BRIEF. A record whose ruling stands is a ruling on THAT
// record; it is not a ruling on whichever brief links it. So the ruling's
// decision issue must be one for both — it carries the record's decision-gate
// marker (or one for a brief whose design: cites it: issueRulesOnRecord) AND
// the brief's own marker (briefIssueRulesOn, the decision-gate hold's binding) —
// and the comment must name the brief as well as the record, because an issue
// body is editable after the ruling and so cannot alone tie the comment to a
// brief (the hold requires the same of its own rulings).
//
// It checks THAT a human ruled on this brief, never WHAT they ruled: the option a
// brief's Ruling line names is not compared with anything (the record carries no
// chosen-option field), exactly as the hold confirms a ruling without reading it.
//
// Exit codes: 0 = the ruling corroborates for this brief; 1 = it does not (or the
// record is unreadable, or carries no ruling: link); 2 = could-not-check (usage,
// an unreadable brief, an undeterminable repository, or a forge read that failed).

// rulingRecordClientFn builds the forge client this mode reads through — a seam
// so a test can serve the forge from an httptest server.
var rulingRecordClientFn = rulingForgeClient

// rulingBrief is the brief a recorded ruling must rule on.
type rulingBrief struct {
	boardIDs []string // "<stream>/<NN>" as the file name spells it, and unpadded when that differs
	fmIDs    []string // the frontmatter brief: value, when it has one
}

// names is every id a ruling comment may name the brief by.
func (b rulingBrief) names() []string {
	return append(append([]string{}, b.boardIDs...), b.fmIDs...)
}

// loadRulingBrief reads the brief at the repo-relative rel. why is non-empty when
// rel is not a brief file of this checkout: docs/streams/<stream>/brief-<NN>-*.md,
// active or archived under done/.
func loadRulingBrief(root, rel string) (b rulingBrief, why string) {
	raw := strings.TrimSpace(rel)
	if raw == "" {
		return b, "is required with --ruling-record: the gate: human brief the ruling must rule on"
	}
	clean := path.Clean(filepath.ToSlash(raw))
	parts := strings.Split(clean, "/")
	if path.IsAbs(clean) || clean != filepath.ToSlash(raw) || len(parts) < 4 || parts[0] != "docs" || parts[1] != "streams" ||
		len(parts) > 5 || (len(parts) == 5 && parts[3] != archiveDirName) {
		return b, "is not a repo-relative brief path (docs/streams/<stream>/brief-<NN>-<slug>.md, or under its done/)"
	}
	m := briefNameRe.FindStringSubmatch(parts[len(parts)-1])
	if m == nil {
		return b, "is not a brief file (brief-<NN>-<slug>.md)"
	}
	full := filepath.Join(root, filepath.FromSlash(clean))
	if fi, err := os.Lstat(full); err != nil || !fi.Mode().IsRegular() {
		return b, "is not a regular file in " + root
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return b, "could not be read: " + err.Error()
	}
	stream, num := parts[2], m[1]
	b.boardIDs = []string{stream + "/" + num}
	if n := normBriefNum(num); n != num {
		b.boardIDs = append(b.boardIDs, stream+"/"+n)
	}
	var gb gateBrief
	readGateFrontmatter(&gb, content)
	if gb.BriefID != "" {
		b.fmIDs = []string{gb.BriefID}
	}
	return b, ""
}

// resolveRecordForBrief is condition 6 for a record AND a brief, then the brief
// named in the comment. Every other condition is resolveRulingOn's.
func resolveRecordForBrief(c *ghClient, root, repo string, rec decisionRecordStamp, b rulingBrief, logins map[string]string) rulingOutcome {
	aliases := decisionRepoAliases(root, repo)
	citing := decisionCitingBriefs(root, rec.ID)
	names := b.names()
	if rec.LoadErr == "" && strings.TrimSpace(rec.Ruling) != "" {
		fmt.Fprintf(os.Stderr, "statusgen --corroborate: resolving ruling link %s for %s as a ruling on brief %s\n",
			strings.TrimSpace(rec.Ruling), rec.File, strings.Join(names, " / "))
	}
	o := resolveRulingOn(c, repo, rec, logins, rulingIssueBinding{
		what: rec.ID + " (or a brief whose design: cites it) together with one for brief " + strings.Join(names, " / "),
		rules: func(iss rulingIssue, link rulingLink) bool {
			return issueRulesOnRecord(iss.Body, rec.ID, link.Repo, aliases, citing) &&
				briefIssueRulesOn(iss.Body, repo, b.boardIDs, b.fmIDs, aliases)
		},
	})
	if !o.Present || o.Reason != rulingOK {
		return o
	}
	for _, n := range names {
		if commentNamesRecord(o.Body, n) {
			return o
		}
	}
	o.Reason = rulingRecordNotNamed
	o.Detail = fmt.Sprintf("the ruling comment %s names the record but not the brief (%s) — a ruling on a record is a ruling on the brief only when the human's comment names that brief",
		o.Detail, strings.Join(names, " / "))
	return o
}

func runCorroborateRecord(root, rel, briefRel string) int {
	rel = filepath.ToSlash(filepath.Clean(strings.TrimSpace(rel)))
	if !isDecisionRecordPath(rel) {
		fmt.Fprintf(os.Stderr, "statusgen: --ruling-record %q is not a decision record (a repo-relative docs/streams/%s/<name>.md, README.md excluded)\n", rel, decisionsDirName)
		return 2
	}
	brief, why := loadRulingBrief(root, briefRel)
	if why != "" {
		fmt.Fprintf(os.Stderr, "statusgen: --ruling-brief %q %s\n", briefRel, why)
		return 2
	}
	repo := repoFromOriginAtFn(root)
	if repo == "" {
		fmt.Fprintf(os.Stderr, "statusgen: --ruling-record %s: COULD-NOT-CHECK — cannot determine the GitHub repository from the origin remote of %s\n", rel, root)
		return 2
	}
	rec := loadDecisionRecordStamp(root, rel)
	o := resolveRecordForBrief(rulingRecordClientFn(), root, repo, rec, brief, scanEffectiveConfig().HumanLogins)
	switch {
	case o.Present && o.Reason == rulingOK:
		fmt.Printf("%s: CORROBORATED for %s — ruling comment by %s (mapped human %q) naming the record and the brief, on a decision issue for both: %s (provenance only: what the ruling chose is not compared)\n",
			rel, strings.Join(brief.names(), " / "), o.Author, o.Name, o.Detail)
		return 0
	case o.Reason == rulingUnresolvable:
		fmt.Printf("%s: COULD-NOT-CHECK — ruling %s: %s\n", rel, o.Reason, o.Detail)
		return 2
	case !o.Present:
		// No ruling: link. A PR's own reviews can anchor a real decided-by name; outside a PR
		// nothing can, so the record does not corroborate whatever its decided-by says.
		fmt.Printf("%s: MISSING-CORROBORATION — ruling %s: the record carries no ruling: link; outside a pull request only a ruling: link to the human's ruling comment can corroborate it (registers-v1 §7.5)\n",
			rel, rulingPlaceholderUnratified)
		return 1
	case o.Reason == rulingDeleted:
		// A private repository answers 404 to a credential that cannot read it, too. Still a
		// refusal (fail closed); the message says which repository the read was against.
		fmt.Printf("%s: MISSING-CORROBORATION — ruling %s: %s (a 404 also answers a credential that cannot read %s)\n", rel, o.Reason, o.Detail, repo)
		return 1
	default:
		fmt.Printf("%s: MISSING-CORROBORATION — ruling %s: %s\n", rel, o.Reason, o.Detail)
		return 1
	}
}
