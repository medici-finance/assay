package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/medici-finance/assay/gitcore"
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

type transitionLink struct {
	number                           int
	amendment, approval, predecessor string
	preflight                        bool
}

// transitionLinks validates the entire chain grammar before history or authority
// reads. The legacy form continues through the unchanged single-link grammar.
func transitionLinks(run string) ([]transitionLink, string) {
	markers := protectedMarkersNamed(normalizedProtectedEvidence(run), "Acceptance-Transition:")
	chain := false
	for _, m := range markers {
		chain = chain || strings.Contains(m.text, "link=")
	}
	if !chain {
		return nil, ""
	}
	links := make([]transitionLink, 0, len(markers))
	seen := map[string]bool{}
	for i, m := range markers {
		k := i + 1
		fail := func(s string) ([]transitionLink, string) {
			return nil, fmt.Sprintf("acceptance transition: link %d: %s", k, s)
		}
		f := strings.Fields(m.text)
		if !m.literal || len(f) != 4 || f[1] != fmt.Sprintf("link=%d", k) || !strings.HasPrefix(f[2], "amendment=") || !strings.HasPrefix(f[3], "approval=") {
			return fail("expected contiguous literal chain markers in link order")
		}
		a, u := strings.TrimPrefix(f[2], "amendment="), strings.TrimPrefix(f[3], "approval=")
		if !isObjectID(a) {
			return fail("a separate immutable amendment is required")
		}
		parsed, ok := parseRulingURL(u)
		if !ok {
			return fail("a valid approval URL is required")
		}
		key := fmt.Sprintf("%s/%d", strings.ToLower(parsed.Repo), parsed.CommentID)
		if seen[key] {
			return fail("an approval URL cannot serve two links")
		}
		seen[key] = true
		links = append(links, transitionLink{number: k, amendment: a, approval: u})
	}
	return links, ""
}

func acceptanceTransition(path, verify, full, run, delivery, base, original, baseline, delivered string, audit bool, read func(string) (string, error)) (string, map[string]string, string) {
	links, why := transitionLinks(run)
	if why != "" {
		return "", nil, why
	}
	if len(links) == 0 {
		return acceptanceStep(path, verify, full, run, delivery, base, original, baseline, delivered, audit, read, nil)
	}
	fail := func(k int, s string) (string, map[string]string, string) {
		return "", nil, fmt.Sprintf("acceptance transition: link %d: %s", k, s)
	}
	dir, file := filepath.Dir(path), filepath.Base(path)
	version := func(rev string) (int, error) {
		raw, e := historyGit(dir, "show", "--end-of-options", rev+":./"+file).Output()
		if e != nil {
			return 0, e
		}
		bf, ok, e := parseBriefFileBytes(path, raw)
		if e != nil || !ok || bf.Version < 1 {
			return 0, fmt.Errorf("transition requires a versioned brief-v2 definition")
		}
		return bf.Version, nil
	}
	// Account for every Verify OR version change, including a reverted edit.
	history, e := historyGit(dir, "rev-list", "--first-parent", "--reverse", "HEAD", "^"+delivery).Output()
	if e != nil {
		return fail(1, "cannot read amendment ancestry")
	}
	previous, previousTable := delivery, delivered
	previousVersion, e := version(delivery)
	if e != nil {
		return fail(1, e.Error())
	}
	position := 0
	for _, rev := range strings.Fields(string(history)) {
		table, e := read(rev)
		if e != nil {
			return fail(position+1, "cannot read acceptance history")
		}
		v, e := version(rev)
		if e != nil {
			return fail(position+1, e.Error())
		}
		if strings.TrimSpace(table) != strings.TrimSpace(previousTable) || v != previousVersion {
			if position >= len(links) || links[position].amendment != rev {
				return fail(position+1, fmt.Sprintf("link %d missing at %.12s", position+1, rev))
			}
			links[position].predecessor = previous
			previous = rev
			position++
		}
		previousTable, previousVersion = table, v
	}
	if position != len(links) {
		return fail(position+1, "amendment is not an ordered first-parent acceptance change")
	}
	final := links[len(links)-1].amendment
	finalTable, e := read(final)
	if e != nil || strings.TrimSpace(verify) != strings.TrimSpace(finalTable) {
		return fail(len(links), "current Verify differs from the approved amendment")
	}
	current, ok, e := parseBriefFile(path)
	if e != nil || !ok || current.Version != previousVersion {
		return fail(len(links), "current acceptance version differs from amendment")
	}
	// Validate all immutable structure before the first forge read. The preflight
	// projects every candidate; actual approval selection is reapplied below from
	// the original obligations, so omitted Expect drift never gains authority.
	for _, preflight := range []bool{true, false} {
		effective, prior := baseline, delivered
		revisions := map[string]string{}
		for i, link := range links {
			link.preflight = preflight
			amended, e := read(link.amendment)
			if e != nil {
				return fail(i+1, "cannot read amendment")
			}
			marker := fmt.Sprintf("Acceptance-Transition: amendment=%s approval=%s", link.amendment, link.approval)
			stepLines := strings.Split(run, "\n")
			var stepRun []string
			inserted := false
			for _, line := range stepLines {
				if strings.HasPrefix(line, "Acceptance-Transition:") {
					if !inserted {
						stepRun = append(stepRun, marker)
						inserted = true
					}
					continue
				}
				stepRun = append(stepRun, line)
			}
			next, revs, why := acceptanceStep(path, amended, full, strings.Join(stepRun, "\n"), delivery, base, original, effective, prior, audit && i == len(links)-1, read, &link)
			if why != "" {
				return "", nil, why
			}
			effective, prior = next, amended
			for id, rev := range revs {
				revisions[id] = rev
			}
		}
		if !preflight {
			return effective, revisions, ""
		}
	}
	panic("unreachable transition planning phase")
}

// acceptanceStep is the shared per-link policy for both record forms. A nil
// chain retains legacy verdicts; chain preflight has no authority-read effects.
func acceptanceStep(path, verify, full, run, delivery, base, original, baseline, delivered string, audit bool, read func(string) (string, error), chain *transitionLink) (string, map[string]string, string) {
	fail := func(s string) (string, map[string]string, string) {
		prefix := "acceptance transition: "
		if chain != nil {
			prefix += fmt.Sprintf("link %d: ", chain.number)
		}
		return "", nil, prefix + s
	}
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
	predecessor := delivery
	if chain != nil {
		predecessor = chain.predecessor
	}
	ancestors, e := historyGit(dir, "rev-list", "--first-parent", "HEAD", "^"+predecessor).Output()
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
	pp, e := provenanceParents(dir, amendment)
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
	if chain != nil && transitionLayout(delivered) != transitionLayout(amended) {
		return fail("table-layout changes are unsupported")
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
		if chain != nil && source.CommandCell != next.CommandCell && source.Expect != next.Expect {
			comparable := next
			comparable.Command, comparable.CommandCell, comparable.ProseLed = source.Command, source.CommandCell, source.ProseLed
			comparable.Expect, comparable.RiskText = source.Expect, source.RiskText
			if !reflect.DeepEqual(source, comparable) {
				return fail("combined changes must retain row identity, class, shell and obligations")
			}
			if !reflect.DeepEqual(source, old) {
				return fail("a changed command must still carry its complete prior definition")
			}
			candidates[old.ID], columns[old.ID], required[old.ID] = next, "Command+Expect", true
			continue
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
	if chain == nil && (err != nil || !ok || current.Version != versions[amendment]) {
		return fail("current acceptance version differs from amendment")
	}
	historical, e := flipEvidenceAt(dir, pp[0], file)
	if e != nil {
		return fail("cannot read historical Evidence")
	}
	deliveredEvidence, err := flipEvidenceAt(dir, delivery, file)
	if err != nil || !transitionDeliveredEvidencePreserved(historical, deliveredEvidence) {
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

	selected := map[string]bool{}
	rawIDs, matched, typed := "", true, false
	if chain != nil && chain.preflight {
		for _, row := range oldRows {
			if _, ok := candidates[row.ID]; ok {
				selected[row.ID] = true
				if rawIDs != "" {
					rawIDs += ","
				}
				rawIDs += row.ID
			}
		}
	} else {
		// The ruling's body itself binds every immutable coordinate and changed row.
		// A bot relay or a general APPROVED review cannot satisfy this exact act.
		rootRaw, e := provenanceRepoTop(dir)
		if e != nil {
			return fail("cannot resolve repository")
		}
		// Git returns an absolute worktree root, while callers may supply a
		// relative brief path (notably --root .). Compare physical absolute paths
		// on both sides so aliases cannot change the ruling's repository coordinate.
		root, e := filepath.EvalSymlinks(strings.TrimSpace(string(rootRaw)))
		if e != nil {
			return fail("cannot resolve canonical repository path")
		}
		absolutePath, e := filepath.Abs(path)
		if e != nil {
			return fail("cannot resolve absolute brief path")
		}
		canonicalPath, e := filepath.EvalSymlinks(absolutePath)
		if e != nil {
			return fail("cannot resolve canonical brief path")
		}
		rel, e := filepath.Rel(root, canonicalPath)
		if e != nil || !filepath.IsLocal(rel) {
			return fail("cannot resolve brief path")
		}
		rel = filepath.ToSlash(rel)
		// Command-priority config could replace even a snapshot's explicit reset.
		// It is outside the snapshot profile and cannot supply approval authority.
		if gitcore.OriginConfigInjected() {
			return fail("could-not-check repository authority: injected Git configuration is unsupported")
		}
		// The owner's error names a fixed unavailability class, never a URL,
		// credential or path, so it is reported verbatim.
		origin, err := gitcore.OriginAuthority(root)
		if err != nil {
			return fail("could-not-check repository authority: " + err.Error())
		}
		if remoteHost(origin) != "github.com" {
			return fail("could-not-check repository authority: approval requires a github.com origin")
		}
		repo := originOwnerRepo(strings.TrimSpace(origin))
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
			if chain != nil && out.Reason == rulingUnresolvable {
				return fail("could-not-check human authority: " + out.Detail)
			}
			return fail("human approval could not be corroborated: " + out.Detail)
		}
		approvalBody := strings.TrimSpace(strings.ReplaceAll(out.Body, "\r\n", "\n"))
		if chain != nil {
			v1 := strings.HasPrefix(approvalBody, "Approve acceptance-transition v1\n")
			if v1 {
				if chain.number != 1 {
					return fail("only link 1 may cite a v1 approval")
				}
				for _, column := range columns {
					if column == "Command+Expect" {
						return fail("v1 approval cannot authorize combined columns")
					}
				}
			} else {
				approvalBody = strings.ReplaceAll(out.Body, "\r\n", "\n")
				payloadPrefix = fmt.Sprintf("Approve acceptance-transition v2\nrepo=%s\nbrief=%s\ndelivery=%s\nbase=%s\noriginal=%s\nlink=%d\npredecessor=%s\namendment=%s\nrows=", repo, rel, delivery, base, original, chain.number, chain.predecessor, amendment)
				typed = true
			}
		}
		rawIDs, matched = strings.CutPrefix(approvalBody, payloadPrefix)
		for _, item := range strings.Split(rawIDs, ",") {
			id := item
			if typed {
				var column string
				id, column, _ = strings.Cut(item, ":")
				if column != columns[id] {
					return fail("human approval does not name the exact transition payload")
				}
			}
			if _, ok := candidates[id]; !ok || selected[id] {
				return fail("human approval does not name the exact transition payload")
			}
			selected[id] = true
		}

	}
	changes := map[string]verifyRow{}
	var ids []string
	for _, old := range oldRows {
		if required[old.ID] && !selected[old.ID] {
			if chain != nil {
				return fail("human approval omits a changed original command: row " + old.ID)
			}
			return fail("human approval omits a changed original command")
		}
		if selected[old.ID] {
			changes[old.ID] = candidates[old.ID]
			id := old.ID
			if typed {
				id += ":" + columns[old.ID]
			}
			ids = append(ids, id)
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
		if columns[cells[0]] == "Expect" || columns[cells[0]] == "Command+Expect" {
			if expectCol < 0 || expectCol >= len(cells) || cells[expectCol] != originals[cells[0]].Expect {
				return fail("cannot locate the original Expect cell")
			}
			cells[expectCol] = changed.Expect
		}
		if columns[cells[0]] == "Command+Expect" {
			cells[commandCol] = changed.CommandCell
		} else if columns[cells[0]] == "Command" {
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

// transitionDeliveredEvidencePreserved retains the byte-prefix rule for every
// substantive result. Only the exact generated empty-Evidence placeholder may
// instead survive as an exact Markdown quotation (#4563, human ruling
// https://github.com/medici-finance/assay/issues/4563#issuecomment-6528575948).
// Amendment and current-history preservation are checked separately, unchanged.
func transitionDeliveredEvidencePreserved(historical, delivered string) bool {
	if strings.HasPrefix(strings.TrimSpace(historical), strings.TrimSpace(delivered)) {
		return true
	}
	const placeholder = "<!-- Independent verifier appends one row per Verify command after implementation. -->"
	if strings.TrimSpace(delivered) != placeholder {
		return false
	}
	for _, line := range strings.Split(historical, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ">") {
			continue
		}
		for strings.HasPrefix(line, ">") {
			line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
		}
		if line == placeholder {
			return true
		}
	}
	return false
}

// transitionLayout binds the explicit table header, including column order.
func transitionLayout(table string) string {
	for _, line := range strings.Split(table, "\n") {
		cells := splitRowEscaped(strings.TrimSpace(line))
		if len(cells) > 0 && strings.TrimSpace(cells[0]) == "#" {
			for i := range cells {
				cells[i] = strings.TrimSpace(cells[i])
			}
			return strings.Join(cells, "|")
		}
	}
	return ""
}
