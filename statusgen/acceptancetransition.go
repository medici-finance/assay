package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// The transport is deliberately unavailable until a command explicitly opts in.
// Tests inject the existing HTTP client boundary, never an approval boolean.
var transitionClient func() *ghClient
var transitionForge bool

func transitionFlags(fs *flag.FlagSet) func() {
	old := transitionForge
	fs.BoolVar(&transitionForge, "acceptance-forge", false, "explicitly corroborate acceptance transitions on the forge; otherwise transitions remain held")
	return func() { transitionForge = old }
}

// transitionWitnessTree is the execution revision a fresh witness of the
// current run names (#4299). A host transition run records the full HEAD; a
// container bracket row of the same run records the receipt's 12-hex tree
// token. The short token stands for the run's full sha only when it is at
// least treeSHALen long, an exact prefix of runSHA, and git resolves it, on
// its own, to exactly runSHA. Otherwise the token is returned unchanged and
// the caller's full-object-id check refuses it.
func transitionWitnessTree(dir, tree, runSHA string) string {
	if isObjectID(tree) || !isObjectID(runSHA) || len(tree) < treeSHALen || !strings.HasPrefix(runSHA, tree) {
		return tree
	}
	// tree is a hex prefix of runSHA here, so it can never read as an option.
	out, err := historyGit(dir, "rev-parse", "--verify", "--quiet", tree+"^{commit}").Output()
	if err != nil || strings.TrimSpace(string(out)) != runSHA {
		return tree
	}
	return runSHA
}

func acceptanceTransition(path, verify, full, run, delivery, base, original, baseline, delivered string, audit bool, read func(string) (string, error)) (string, map[string]string, string) {
	fail := func(s string) (string, map[string]string, string) { return "", nil, "acceptance transition: " + s }
	markers := protectedMarkersNamed(normalizedProtectedEvidence(run), "Acceptance-Transition:")
	if len(markers) != 1 || !markers[0].literal {
		return fail("one literal live Acceptance-Transition marker is required")
	}
	marker := markers[0].text
	fields := strings.Fields(marker)
	if len(fields) != 3 || !strings.HasPrefix(fields[1], "amendment=") || !strings.HasPrefix(fields[2], "approval=") {
		return fail("expected Acceptance-Transition: amendment=<full-sha> approval=<ruling-url>")
	}
	amendment := strings.TrimPrefix(fields[1], "amendment=")
	approval := strings.TrimPrefix(fields[2], "approval=")
	if !isObjectID(amendment) || amendment == delivery {
		return fail("a separate immutable amendment is required")
	}
	dir, file := filepath.Dir(path), filepath.Base(path)
	ancestors, e := historyGit(dir, "rev-list", "--first-parent", "HEAD", "^"+delivery).Output()
	if e != nil {
		return fail("cannot read amendment ancestry")
	}
	found := false
	for _, id := range strings.Fields(string(ancestors)) {
		found = found || id == amendment
	}
	if !found {
		return fail("amendment is not after delivery on the observed first-parent chain")
	}
	parents, e := historyGit(dir, "show", "-s", "--format=%P", "--end-of-options", amendment).Output()
	pp := strings.Fields(string(parents))
	if e != nil || (len(pp) != 1 && len(pp) != 2) {
		return fail("amendment must have one or two parents")
	}
	before, e := read(pp[0])
	if e != nil || strings.TrimSpace(before) != strings.TrimSpace(delivered) {
		return fail("acceptance changed between delivery and amendment")
	}
	if len(pp) == 2 {
		bases, err := flipMergeBases(dir, pp[0], pp[1])
		if err != nil || len(bases) != 1 {
			return fail("amendment has no unique merge base")
		}
		divergence, err := read(bases[0])
		if err != nil || !reflect.DeepEqual(briefVerifyRows(divergence), briefVerifyRows(before)) {
			return fail("amendment divergence differs from pre-merge main acceptance")
		}
	}
	amended, e := read(amendment)
	if e != nil || strings.TrimSpace(amended) != strings.TrimSpace(verify) {
		return fail("current Verify differs from the approved amendment")
	}
	oldRows, newRows := briefVerifyRows(delivered), briefVerifyRows(amended)
	if len(oldRows) != len(newRows) {
		return fail("row addition, deletion or renumbering is unsupported")
	}
	originals := map[string]verifyRow{}
	for _, r := range briefVerifyRows(baseline) {
		originals[r.ID] = r
	}
	candidates := map[string]verifyRow{}
	required := map[string]bool{}
	columns := map[string]string{}
	for i, old := range oldRows {
		next := newRows[i]
		if old.ID != next.ID {
			return fail("row order or identity changed")
		}
		source, owed := originals[old.ID]
		if owed && expectOnlyChange(source, next) {
			// An Expect-only original change may have been delivered already or
			// be introduced by the amendment. It receives no credit unless the
			// exact human approval explicitly names this row. An older command-
			// only approval therefore still owes the original Expect.
			candidates[old.ID], columns[old.ID] = next, "Expect"
			continue
		}
		if reflect.DeepEqual(old, next) {
			continue
		}
		if !owed {
			return fail("supplemental rows cannot be amended or promoted")
		}
		oldContext, oldOK := transitionContext(old)
		nextContext, nextOK := transitionContext(next)
		if !oldOK || !nextOK || oldContext != nextContext {
			return fail("changed commands require one unambiguous code span and identical surrounding context")
		}
		comparable := next
		comparable.Command = old.Command
		comparable.CommandCell = old.CommandCell
		comparable.RiskText = old.RiskText
		if next.Command == old.Command || !reflect.DeepEqual(old, comparable) {
			return fail("only command-only or Expect-only original-row changes are supported")
		}
		if !reflect.DeepEqual(source, old) {
			return fail("a changed command must still carry its complete original delivered definition")
		}
		candidates[old.ID], columns[old.ID], required[old.ID] = next, "Command", true
	}
	if len(candidates) == 0 {
		return fail("no changed original command or Expect")
	}
	// Bind the existing brief-v2 acceptance version, not an invented counter.
	versions := map[string]int{}
	for _, rev := range []string{pp[0], amendment} {
		raw, err := historyGit(dir, "show", "--end-of-options", rev+":./"+file).Output()
		if err != nil {
			return fail("cannot read acceptance version")
		}
		bf, ok, err := parseBriefFileBytes(path, raw)
		if err != nil || !ok || bf.Version < 1 {
			return fail("transition requires a versioned brief-v2 definition")
		}
		versions[rev] = bf.Version
	}
	if versions[amendment] != versions[pp[0]]+1 {
		return fail("amendment must increment the acceptance version exactly once")
	}
	current, ok, err := parseBriefFile(path)
	if err != nil || !ok || current.Version != versions[amendment] {
		return fail("current acceptance version differs from amendment")
	}
	historical, e := flipEvidenceAt(dir, pp[0], file)
	if e != nil {
		return fail("cannot read historical Evidence")
	}
	deliveredEvidence, err := flipEvidenceAt(dir, delivery, file)
	if err != nil || !strings.HasPrefix(strings.TrimSpace(historical), strings.TrimSpace(deliveredEvidence)) {
		return fail("pre-amendment history lost delivered Evidence")
	}
	atAmendment, e := flipEvidenceAt(dir, amendment, file)
	if e != nil || atAmendment != historical {
		return fail("amendment changed historical Evidence")
	}
	if !strings.HasPrefix(strings.TrimSpace(full), strings.TrimSpace(historical)) {
		return fail("historical Evidence was not preserved")
	}
	if audit {
		p, e := flipLatestPass(full)
		if e != nil {
			return fail("a fresh completed independent run is required")
		}
		if e = flipProvenance(path, full, p.Marks, p.RunStart); e != nil {
			return fail(e.Error())
		}
		for _, r := range briefVerifyRows(baseline) {
			var latest string
			for _, er := range parseEvidenceRows(run)[r.ID] {
				if isWitnessRow(er.Text) {
					latest = er.Text
				}
			}
			tree := transitionWitnessTree(dir, witnessTreeOf(latest), p.SHA)
			if !isObjectID(tree) {
				return fail("fresh witnesses must name full immutable execution revisions")
			}
			mb, err := flipMergeBases(dir, amendment, tree)
			if err != nil || len(mb) != 1 || mb[0] != amendment {
				return fail("every original row needs a fresh witness at or after amendment")
			}
		}
	}

	// The ruling's body itself binds every immutable coordinate and changed row.
	// A bot relay or a general APPROVED review cannot satisfy this exact act.
	rootRaw, e := historyGit(dir, "rev-parse", "--show-toplevel").Output()
	if e != nil {
		return fail("cannot resolve repository")
	}
	root := strings.TrimSpace(string(rootRaw))
	canonicalPath, e := filepath.EvalSymlinks(path)
	if e != nil {
		return fail("cannot resolve canonical brief path")
	}
	rel, e := filepath.Rel(root, canonicalPath)
	if e != nil || !filepath.IsLocal(rel) {
		return fail("cannot resolve brief path")
	}
	rel = filepath.ToSlash(rel)
	origin, err := historyGit(root, "remote", "get-url", "origin").Output()
	if err != nil || remoteHost(string(origin)) != "github.com" {
		return fail("could-not-check repository authority: approval requires a github.com origin")
	}
	repo := originOwnerRepo(strings.TrimSpace(string(origin)))
	if repo == "" {
		return fail("cannot bind approval to the repository origin")
	}
	payloadPrefix := fmt.Sprintf("Approve acceptance-transition v1\nrepo=%s\nbrief=%s\ndelivery=%s\nbase=%s\noriginal=%s\namendment=%s\nrows=", repo, rel, delivery, base, original, amendment)
	if !transitionForge {
		return fail("could-not-check human authority: --acceptance-forge is required")
	}
	fmt.Fprintln(os.Stderr, "acceptance transition: reading approval", approval)
	client := transitionClient
	if client == nil {
		client = rulingForgeClient
	}
	rec := decisionRecordStamp{ID: "acceptance-transition", Ruling: approval}
	out := resolveRulingOn(client(), repo, rec, scanEffectiveConfig().HumanLogins, rulingIssueBinding{what: "acceptance transition for " + rel, rules: func(iss rulingIssue, _ rulingLink) bool {
		return strings.Contains(iss.Body, "<!-- acceptance-transition: "+rel+" -->")
	}})
	if !out.Present || out.Reason != rulingOK {
		return fail("human approval could not be corroborated: " + out.Detail)
	}
	approvalBody := strings.TrimSpace(strings.ReplaceAll(out.Body, "\r\n", "\n"))
	rawIDs, matched := strings.CutPrefix(approvalBody, payloadPrefix)
	selected := map[string]bool{}
	for _, id := range strings.Split(rawIDs, ",") {
		if _, ok := candidates[id]; !ok || selected[id] {
			return fail("human approval does not name the exact transition payload")
		}
		selected[id] = true
	}
	changes := map[string]verifyRow{}
	var ids []string
	for _, old := range oldRows {
		if required[old.ID] && !selected[old.ID] {
			return fail("human approval omits a changed original command")
		}
		if selected[old.ID] {
			changes[old.ID] = candidates[old.ID]
			ids = append(ids, old.ID)
		}
	}
	if !matched || len(ids) == 0 || strings.Join(ids, ",") != rawIDs {
		return fail("human approval does not name the exact transition payload")
	}
	// Rewrite only approved columns in the original table. Unselected Expect
	// drift retains its original obligation; a command change is never optional.
	lines := strings.Split(baseline, "\n")
	commandCol, expectCol := -1, -1
	rewritten := map[string]bool{}
	for i, line := range lines {
		cells := splitRowEscaped(strings.TrimSpace(line))
		for j := range cells {
			cells[j] = strings.TrimSpace(cells[j])
		}
		if len(cells) < 2 {
			continue
		}
		if cells[0] == "#" {
			for j, c := range cells {
				if c == "Command" {
					commandCol = j
				}
				if c == "Expect" {
					expectCol = j
				}
			}
			continue
		}
		changed, ok := changes[cells[0]]
		if !ok {
			continue
		}
		if commandCol < 0 || commandCol >= len(cells) {
			return fail("cannot locate the original command cell")
		}
		if columns[cells[0]] == "Expect" {
			if expectCol < 0 || expectCol >= len(cells) || cells[expectCol] != originals[cells[0]].Expect {
				return fail("cannot locate the original Expect cell")
			}
			cells[expectCol] = changed.Expect
		} else {
			originalCell := originals[cells[0]].CommandCell
			if cells[commandCol] != originalCell {
				return fail("original command cell does not match its parsed definition")
			}
			cells[commandCol] = strings.Replace(originalCell, codeSpanOf(originals[cells[0]].Command), codeSpanOf(changed.Command), 1)
		}
		rewritten[cells[0]] = true
		lines[i] = "| " + strings.Join(cells, " | ") + " |"
	}
	if len(rewritten) != len(changes) {
		return fail("initial transitions require an explicit first-column # header and every changed row ID")
	}
	effective := strings.Join(lines, "\n")
	effectiveRows := briefVerifyRows(effective)
	if len(effectiveRows) != len(originals) {
		return fail("effective table lost original obligations")
	}
	for _, row := range effectiveRows {
		old, ok := originals[row.ID]
		if !ok {
			return fail("effective table added an obligation")
		}
		want := old
		if changed, ok := changes[row.ID]; ok {
			want = changed
		}
		if !reflect.DeepEqual(row, want) {
			return fail("effective table changed an unchanged obligation or command context")
		}
	}
	revisions := map[string]string{}
	for id := range changes {
		revisions[id] = amendment
	}
	return effective, revisions, ""
}

// transitionContext binds both sides of the unique executable span separately:
// concatenating them would let context move across the command. Reuse the
// renderer and execution parser; do not infer a command from surrounding prose.
func transitionContext(row verifyRow) ([2]string, bool) {
	spans, plain := renderedCodeSpans(row.CommandCell)
	if !plain || len(spans) != 1 || !spans[0].cleanEnd || spans[0].text != row.Command || row.ProseLed != "" {
		return [2]string{}, false
	}
	before, after, found := strings.Cut(row.CommandCell, codeSpanOf(row.Command))
	if !found || strings.ContainsAny(before+after, "`") {
		return [2]string{}, false
	}
	return [2]string{before, after}, true
}

// expectOnlyChange retains the complete parsed command, class, shell and
// obligations. RiskText is derived from the two authored cells, not independent
// authority; it must be recomputed whenever the Expect cell changes.
func expectOnlyChange(original, next verifyRow) bool {
	if next.Expect == original.Expect {
		return false
	}
	comparable := next
	comparable.Expect, comparable.RiskText = original.Expect, original.RiskText
	return reflect.DeepEqual(original, comparable)
}
