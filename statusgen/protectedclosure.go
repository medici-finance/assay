package main

// Protected-table closure uses immutable Git snapshots, never an exemption
// inferred from a witness's prose. These reads authenticate object contents and
// ancestry, not a forge identity. The existing verifier-attribution and admission
// checks remain the authority for who produced the Evidence.
import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

const stateAuthorAdded = "author-added"

type protectedPlan struct {
	Verify   string
	Evidence string // only the selected current run; earlier records remain on disk
	Added    []checkFinding
	Original string // authenticated immutable acceptance-definition revision
}

func closureSatisfied(f checkFinding) bool {
	return f.State == statePass || f.State == stateAuthorAdded
}

// protectedAcceptance returns only owed original rows plus explicit uncredited
// supplemental dispositions. Without a live marker, the ordinary table is owed
// in full. A quoted example, hash or author-added label grants no exemption.
func protectedAcceptance(verify, evidence string, paths ...string) (protectedPlan, string) {
	stripped, _ := stripRowComments(evidence)
	live := stripCode(stripped)
	protected := false
	for _, line := range strings.Split(live, "\n") {
		protected = protected || strings.HasPrefix(line, "Verification-Table:")
	}
	if !protected {
		return protectedPlan{Verify: verify, Evidence: evidence}, ""
	}
	// Callers that already selected p.Run supply no verdict. Complete Evidence
	// histories use the same strict current-run selector as flip and reconcile.
	if len(flipVerdictTokens(strings.Split(stripped, "\n"))) > 0 {
		p, err := flipLatestPass(evidence)
		if err != nil {
			return protectedPlan{}, "protected table: " + err.Error()
		}
		// A newer unfinished record must not borrow the last completed PASS.
		// Comments and quoted examples after it do not start a live run.
		for _, line := range strings.Split(live, "\n")[p.Marks[len(p.Marks)-1]+1:] {
			line = strings.TrimSpace(strikethroughRe.ReplaceAllString(line, ""))
			if line != "" && !strings.HasPrefix(line, ">") {
				return protectedPlan{}, "protected table: live content follows the latest PASS; complete the current run before closing"
			}
		}
		evidence = p.Run
		stripped, _ = stripRowComments(evidence)
	}
	rawLines := strings.Split(stripped, "\n")
	var marker string
	for i, line := range strings.Split(stripCode(stripped), "\n") {
		if !strings.HasPrefix(line, "Verification-Table:") {
			continue
		}
		if line != rawLines[i] {
			return protectedPlan{}, "protected table: Verification-Table must be literal live text, without code spans"
		}
		if marker != "" {
			return protectedPlan{}, "protected table: multiple Verification-Table markers in the current run"
		}
		marker = strings.TrimSpace(line)
	}
	if marker == "" {
		return protectedPlan{}, "protected table: the current run has no Verification-Table marker"
	}
	fields := strings.Fields(marker)
	if len(fields) != 4 || !strings.HasPrefix(fields[1], "delivery=") || !strings.HasPrefix(fields[2], "base=") || !strings.HasPrefix(fields[3], "original=") {
		return protectedPlan{}, "protected table: expected Verification-Table: delivery=<full-sha> base=<full-sha> original=<full-sha>"
	}
	delivery, base, original := strings.TrimPrefix(fields[1], "delivery="), strings.TrimPrefix(fields[2], "base="), strings.TrimPrefix(fields[3], "original=")
	if !isObjectID(delivery) || !isObjectID(base) || !isObjectID(original) || len(paths) != 1 || paths[0] == "" {
		return protectedPlan{}, "protected table: full object ids and the brief's repository path are required"
	}
	dir, file := filepath.Dir(paths[0]), filepath.Base(paths[0])
	refuse := func(err any) (protectedPlan, string) { return protectedPlan{}, fmt.Sprint("protected table: ", err) }
	// Reuse the qualified source-provenance primitive. This authenticates the
	// delivery and baseline snapshots, not whether a witness applies to code:
	// that remains the independent content comparison in witnessTreeApplies.
	head, err := historyGit(dir, "rev-parse", "--verify", "HEAD^{commit}").Output()
	if err != nil || !isObjectID(strings.TrimSpace(string(head))) {
		return refuse("cannot resolve HEAD as an immutable commit")
	}
	bases, err := flipMergeBases(dir, delivery, strings.TrimSpace(string(head)))
	if err != nil || len(bases) != 1 || bases[0] != delivery {
		return refuse("delivery is absent or not an ancestor of HEAD")
	}
	out, err := historyGit(dir, "show", "-s", "--format=%P", "--end-of-options", delivery).Output()
	if err != nil {
		return refuse("cannot read delivery parents")
	}
	parents := strings.Fields(string(out))
	var expected string
	switch len(parents) {
	case 1:
		expected = parents[0] // squash: the exact pre-delivery main snapshot
	case 2:
		bases, err := flipMergeBases(dir, parents[0], parents[1])
		if err != nil || len(bases) != 1 {
			return refuse("delivery has no unique merge base")
		}
		expected = bases[0]
	default:
		return refuse("delivery must have one parent (squash) or two parents (merge)")
	}
	bases, err = flipMergeBases(dir, original, base)
	if base != expected || err != nil || len(bases) != 1 || bases[0] != original {
		return refuse("base is not the exact delivery baseline, or original is not its ancestor")
	}
	read := func(rev string) (string, error) {
		b, err := historyGit(dir, "show", "--end-of-options", rev+":./"+file).Output()
		if err != nil {
			return "", err
		}
		body := strings.ReplaceAll(string(b), "\r\n", "\n")
		if first, _, _ := strings.Cut(body, "\n"); strings.TrimSpace(first) == "---" {
			if _, content, err := splitFrontmatter(body); err == nil {
				body = content
			}
		}
		section := extractSectionByPrefix(body, "Verify")
		if why := unterminatedCommentIn(section, ""); why != "" {
			return "", fmt.Errorf("%s", why)
		}
		if len(verifyUnplacedRows(section)) != 0 {
			return "", fmt.Errorf("unplaced Verify rows")
		}
		seen := map[string]bool{}
		for _, row := range briefVerifyRows(section) {
			if seen[row.ID] {
				return "", fmt.Errorf("duplicate Verify row %s", row.ID)
			}
			seen[row.ID] = true
		}
		return section, nil
	}
	baseline, err := read(base)
	if err != nil {
		return refuse("cannot read original baseline: " + err.Error())
	}
	if len(parents) == 2 {
		mainTable, err := read(parents[0])
		if err != nil || !reflect.DeepEqual(briefVerifyRows(mainTable), briefVerifyRows(baseline)) {
			return refuse("pre-delivery main acceptance differs from the divergence baseline; independent re-derivation is required")
		}
	}
	old, err := read(original)
	if err != nil {
		return refuse("cannot read original table: " + err.Error())
	}
	delivered, err := read(delivery)
	if err != nil {
		return refuse("cannot read delivered table: " + err.Error())
	}
	if !reflect.DeepEqual(briefVerifyRows(old), briefVerifyRows(baseline)) {
		return refuse("original table differs from pre-delivery baseline; independent re-derivation is required")
	}
	if strings.TrimSpace(verify) != strings.TrimSpace(delivered) {
		return refuse("current Verify section differs from immutable delivery")
	}
	originals := briefVerifyRows(baseline)
	if len(originals) == 0 {
		return refuse("original table has no acceptance rows")
	}
	current := map[string]verifyRow{}
	for _, row := range briefVerifyRows(delivered) {
		current[row.ID] = row
	}
	for _, row := range originals {
		if got, ok := current[row.ID]; !ok || got.Class != row.Class {
			return refuse("original row " + row.ID + " was removed, renumbered or had its class changed; restore its original acceptance contract")
		}
		delete(current, row.ID)
	}
	rows := parseEvidenceRows(evidence)
	for _, row := range originals {
		var latest string
		for _, er := range rows[row.ID] {
			if isWitnessRow(er.Text) {
				latest = er.Text
			}
		}
		cells := witnessCells(latest)
		if len(cells) > 0 && strings.HasPrefix(strings.TrimSpace(cells[witnessCellResult]), stateAuthorAdded) {
			return refuse("original row " + row.ID + " cannot be reported author-added")
		}
		// Even when a delivered Expect was edited, the original process exit
		// remains owed. Output assertions retain the ordinary witness contract:
		// the committed verifier record and its output hash need independent review.
		if len(cells) > 0 && witnessStateOf(latest) == statePass {
			exit := witnessResultRe.FindStringSubmatch(cells[witnessCellResult])
			if len(exit) != 2 || exit[1] != fmt.Sprint(parseExpect(row.Expect).exitCode) {
				return refuse("original row " + row.ID + " witness exit contradicts the original Expect")
			}
		}
	}
	var added []checkFinding
	for _, row := range briefVerifyRows(delivered) {
		if _, supplemental := current[row.ID]; !supplemental {
			continue
		}
		var latest string
		for _, er := range rows[row.ID] {
			if isWitnessRow(er.Text) {
				latest = er.Text
			}
		}
		cells := witnessCells(latest)
		if len(cells) == 0 || normalizeCommandText(witnessCommandOf(latest)) != normalizeCommandText(row.Command) || witnessStateOf(latest) != "" || !strings.HasPrefix(strings.TrimSpace(cells[witnessCellResult]), "author-added;") || !strings.HasSuffix(strings.TrimSpace(cells[witnessCellResult]), "no acceptance credit") {
			return refuse("row " + row.ID + " requires its latest matching witness to say author-added; no acceptance credit")
		}
		added = append(added, checkFinding{row.ID, stateAuthorAdded, "supplemental row; no acceptance credit (immutable original table preserved)"})
	}
	return protectedPlan{Verify: baseline, Evidence: evidence, Added: added, Original: original}, ""
}

// The lint shares exactly the same original-row audit as the closure verbs.
// This also refuses malformed or forged markers in a newly asserted closure.
func protectedClosureProblems(streams []*Stream) []string {
	var problems []string
	for _, s := range streams {
		for _, b := range s.Briefs {
			if b.Status != "verified" && b.Status != "done" {
				continue
			}
			art, ok := loadBriefArtifacts(s, b.Num)
			if !ok || !strings.Contains(art.Evidence, "Verification-Table:") {
				continue
			}
			findings, refusal := closureWitnesses(art.Verify, art.Evidence, art.Path)
			if refusal != "" {
				problems = append(problems, s.Name+"/"+b.Num+": "+refusal)
				continue
			}
			for _, f := range findings {
				if !closureSatisfied(f) {
					problems = append(problems, fmt.Sprintf("%s/%s: protected acceptance row %s: %s", s.Name, b.Num, f.ID, f.State))
				}
			}
		}
	}
	return problems
}
