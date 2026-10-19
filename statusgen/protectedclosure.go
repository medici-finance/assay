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
	Verify    string
	Evidence  string // only the selected current run; earlier records remain on disk
	Added     []checkFinding
	Original  string            // authenticated immutable acceptance-definition revision
	Revisions map[string]string // approved per-row amendments; omitted rows retain Original
}

func closureSatisfied(f checkFinding) bool {
	return f.State == statePass || f.State == stateAuthorAdded
}

// normalizedProtectedEvidence identifies a comment-removed marker-scan view.
// Comment removal is not idempotent. Selected-run auditing must preserve the
// full record's classification through closureRunWitnesses, not a raw string.
type normalizedProtectedEvidence string

func normalizeProtectedEvidence(raw string) normalizedProtectedEvidence {
	stripped, _ := stripRowComments(raw)
	return normalizedProtectedEvidence(stripped)
}

type protectedMarker struct {
	text    string
	literal bool
}

// protectedMarkers is the single scan for both classification and validation.
// Its typed input distinguishes raw Evidence from the already-normalized run.
func protectedMarkers(text normalizedProtectedEvidence) []protectedMarker {
	return protectedMarkersNamed(text, "Verification-Table:")
}

func protectedMarkersNamed(text normalizedProtectedEvidence, prefix string) []protectedMarker {
	rawLines := strings.Split(string(text), "\n")
	var markers []protectedMarker
	for i, line := range strings.Split(stripCode(string(text)), "\n") {
		if strings.HasPrefix(line, prefix) {
			markers = append(markers, protectedMarker{strings.TrimSpace(line), line == rawLines[i]})
		}
	}
	return markers
}

func hasProtectedTableMarker(evidence string) bool {
	text := normalizeProtectedEvidence(evidence)
	return len(protectedMarkers(text)) > 0 || len(protectedMarkersNamed(text, "Acceptance-Transition:")) > 0
}

// closureRunWitnesses owns the full-record/selected-run boundary. Protected
// records retain full history for the canonical audit to select its current run.
// Ordinary records audit only normalizedRun. If that canonical audit would newly
// interpret the selected string as protected, the record is ambiguous: refuse,
// never grant an exemption or return full ordinary history to fill missing rows.
func closureRunWitnesses(verify, full, normalizedRun string, paths ...string) ([]checkFinding, string) {
	if hasProtectedTableMarker(full) {
		return closureWitnesses(verify, full, paths...)
	}
	if hasProtectedTableMarker(normalizedRun) {
		return nil, "protected table: selected run changes marker classification; record literal current provenance and re-run the audit"
	}
	return closureWitnesses(verify, normalizedRun, paths...)
}

// protectedAcceptance returns only owed original rows plus explicit uncredited
// supplemental dispositions. Without a live marker, the ordinary table is owed
// in full. A quoted example, hash or author-added label grants no exemption.
func protectedAcceptance(verify, evidence string, paths ...string) (protectedPlan, string) {
	return protectedAcceptanceFor(verify, evidence, "", paths...)
}

// proposal is a coordinate-only execution request, never an approval or witness.
func protectedAcceptanceFor(verify, evidence, proposal string, paths ...string) (protectedPlan, string) {
	fullEvidence := evidence
	if proposal != "" {
		evidence = proposal
	}
	text := normalizeProtectedEvidence(evidence)
	markers := protectedMarkers(text)
	if len(markers) == 0 {
		if proposal != "" || len(protectedMarkersNamed(text, "Acceptance-Transition:")) > 0 {
			return protectedPlan{}, "protected table: acceptance transition requires immutable delivery provenance"
		}
		return protectedPlan{Verify: verify, Evidence: evidence}, ""
	}
	// Selected-run callers preserve record classification through closureRunWitnesses.
	// Select its strict current run here; earlier markers establish protected
	// context but cannot supply missing current provenance or witnesses.
	if proposal == "" && len(flipVerdictTokens(strings.Split(string(text), "\n"))) > 0 {
		p, err := flipLatestPass(evidence)
		if err != nil {
			return protectedPlan{}, "protected table: " + err.Error()
		}
		evidence = p.Run
		// flipLatestPass normalized the complete Evidence once before slicing
		// this run. Scan that view directly, without another comment removal.
		markers = protectedMarkers(normalizedProtectedEvidence(p.Run))
	}
	if len(markers) == 0 {
		return protectedPlan{}, "protected table: the current run has no Verification-Table marker"
	}
	if len(markers) != 1 {
		return protectedPlan{}, "protected table: multiple Verification-Table markers in the current run"
	}
	if !markers[0].literal {
		return protectedPlan{}, "protected table: Verification-Table must be literal live text, without code spans"
	}
	marker := markers[0].text
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
	// An arbitrary ancestor can belong to a merged side branch. Only a
	// delivery on the observed HEAD's first-parent chain gives parents[0]
	// the mainline role used by the baseline checks below. Excluding that
	// parent bounds the walk without treating side-branch reachability as
	// delivery proof. historyGit retains replace/graft isolation.
	if !isObjectID(parents[0]) {
		return refuse("delivery first parent is not an immutable object id")
	}
	mainline, err := historyGit(dir, "rev-list", "--first-parent", "--end-of-options", strings.TrimSpace(string(head)), "^"+parents[0]).Output()
	if err != nil {
		return refuse("cannot read the immutable first-parent delivery history")
	}
	onMainline := false
	for _, id := range strings.Fields(string(mainline)) {
		onMainline = onMainline || id == delivery
	}
	if !onMainline {
		return refuse("delivery is not on the observed HEAD's first-parent chain; record the actual mainline delivery and re-derive its baseline")
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
	var revisions map[string]string
	if strings.Contains(evidence, "Acceptance-Transition:") {
		var why string
		baseline, revisions, why = acceptanceTransition(paths[0], verify, fullEvidence, evidence, delivery, base, original, baseline, delivered, proposal == "", read)
		if why != "" {
			return refuse(why)
		}
		delivered = verify
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
		if proposal == "" && (len(cells) == 0 || normalizeCommandText(witnessCommandOf(latest)) != normalizeCommandText(row.Command) || witnessStateOf(latest) != "" || !strings.HasPrefix(strings.TrimSpace(cells[witnessCellResult]), "author-added;") || !strings.HasSuffix(strings.TrimSpace(cells[witnessCellResult]), "no acceptance credit")) {
			return refuse("row " + row.ID + " requires its latest matching witness to say author-added; no acceptance credit")
		}
		added = append(added, checkFinding{row.ID, stateAuthorAdded, "supplemental row; no acceptance credit (immutable original table preserved)"})
	}
	return protectedPlan{Verify: baseline, Evidence: evidence, Added: added, Original: original, Revisions: revisions}, ""
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
			if !ok || (!strings.Contains(art.Evidence, "Verification-Table:") && !strings.Contains(art.Evidence, "Acceptance-Transition:")) {
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
