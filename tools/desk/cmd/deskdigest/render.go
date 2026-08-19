package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// render.go — the digest markdown. Everything a reader needs to check the arithmetic is
// IN the document: the repos read, the labels that did not exist, the repos that did not
// answer, and a coverage line reconciling inputs against rendered rows.

// digestMarker is the machine-readable header. It is a secondary confirmation for the
// weekly lifecycle (which keys on the exact title) and the thing a human can grep for.
const digestMarkerPrefix = "<!-- desk-decision-digest v1 week="

// defaultAnswerDays is the brief's stated horizon for an human-only item that HAS a safe
// default: 14 days out.
const defaultAnswerDays = 14

// row is one rendered queue line.
type row struct {
	Item    *item
	Verdict verdict
	Rec     string
	HasRec  bool
	Def     string
	HasDef  bool
	AgeDays int
}

// digest is a fully-computed report, ready to render or post.
type digest struct {
	Week      string
	Now       time.Time
	Coll      *collection
	SignOff   signOff
	Rows      []row
	Decisions []r3Decision
}

// build turns a collection into a digest. Pure apart from the clock it is handed.
func build(c *collection, so signOff, now time.Time, week string) *digest {
	d := &digest{Week: week, Now: now.UTC(), Coll: c, SignOff: so, Decisions: r3Decisions(c)}
	for _, it := range c.Items {
		r := row{Item: it, Verdict: classifyItem(it), AgeDays: wholeDays(it.CreatedAt, now)}
		r.Rec, r.HasRec = recommendation(it)
		r.Def, r.HasDef = declaredDefault(it)
		d.Rows = append(d.Rows, r)
	}
	return d
}

// Counts tallies the rows by class — the numbers the digest states about itself.
func (d *digest) Counts() map[class]int {
	m := map[class]int{classReversible: 0, classHumanOnly: 0, classUnclassified: 0, classCouldNotRead: 0}
	for _, r := range d.Rows {
		m[r.Verdict.Class]++
	}
	return m
}

// title is the weekly issue title. The week id is the whole identity: the same week
// updates in place, a new week is a new issue, and nothing else in the title varies.
func (d *digest) title() string { return "Decision digest " + d.Week }

// Render writes the digest markdown.
func (d *digest) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s -->\n", digestMarkerPrefix, d.Week)
	fmt.Fprintf(&b, "# Decision digest %s\n\n", d.Week)
	fmt.Fprintf(&b, "Generated %s by `deskdigest`. This is a REPORT. It decides nothing, closes nothing, "+
		"flips nothing and applies no label — every row below is an item waiting on a human, or an item "+
		"whose state could not be established.\n\n", d.Now.Format(time.RFC3339))

	d.renderStanding(&b)
	d.renderQueue(&b)
	d.renderDecisions(&b)
	d.renderGaps(&b)
	d.renderCoverage(&b)
	d.renderBoundary(&b)
	return b.String()
}

func (d *digest) renderStanding(b *strings.Builder) {
	fmt.Fprintf(b, "## Standing of %s (reversible-decision default)\n\n", r3ID)
	switch d.SignOff.State {
	case signOffUnsigned:
		fmt.Fprintf(b, "**%s is UNSIGNED.** %s\n\n", r3ID, d.SignOff.Note)
	case signOffClaimed:
		fmt.Fprintf(b, "**%s records a sign-off artifact:** %s\n\n%s\n\n", r3ID, d.SignOff.URL, d.SignOff.Note)
	default:
		fmt.Fprintf(b, "**%s standing: COULD NOT READ.** %s\n\n", r3ID, d.SignOff.Note)
	}
}

func (d *digest) renderQueue(b *strings.Builder) {
	b.WriteString("## Queue\n\n")
	if len(d.Rows) == 0 {
		// THE EMPTY-WEEK INVARIANT. A digest that skipped a week with nothing in it
		// would be byte-identical, from the reader's side, to a digest whose generator
		// crashed, whose credential expired, or whose cron entry was deleted. So the
		// zero case is the case that must say the most: what was read, how, and how the
		// reader can tell this apart from silence.
		b.WriteString("**0 items.** Nothing is waiting on you this week.\n\n")
		b.WriteString("This section is here BECAUSE it is empty. A week with no decisions is a result, " +
			"and a digest that skipped it would be indistinguishable from a digest that failed to run — " +
			"same empty inbox either way. So the empty week reports itself, with its read scope:\n\n")
		for _, rr := range d.Coll.Repos {
			fmt.Fprintf(b, "- `%s` — %s\n", rr.Repo, repoReadSummary(rr))
		}
		if len(d.Coll.Repos) == 0 {
			b.WriteString("- (no repos in scope — the read scope resolved EMPTY, which is itself the finding: " +
				"a digest over nothing is not evidence that there is nothing)\n")
		}
		b.WriteString("\n")
		return
	}

	counts := d.Counts()
	fmt.Fprintf(b, "**%d items** — %d human-only, %d reversible, %d unclassified, %d could-not-read. "+
		"Oldest %d days.\n\n", len(d.Rows), counts[classHumanOnly], counts[classReversible],
		counts[classUnclassified], counts[classCouldNotRead], d.oldestAge())
	b.WriteString("| Item | Age | Question | Class | Why | Desk recommendation | Default if no answer |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, r := range d.Rows {
		fmt.Fprintf(b, "| [%s#%d](%s) | %dd | %s | `%s` | %s | %s | %s |\n",
			r.Item.Repo, r.Item.Number, r.Item.URL, r.AgeDays, oneLine(r.Item.Title),
			r.Verdict.Class, oneLine(r.Verdict.Why), d.recCell(r), d.defaultCell(r))
	}
	b.WriteString("\n")
	if counts[classUnclassified] > 0 || counts[classCouldNotRead] > 0 {
		b.WriteString("`unclassified` and `could-not-read` are NOT soft forms of `reversible`. " +
			"Nothing reaches the desk-decides lane by failing to be classified — the lane is entered only " +
			"by a positive signal, so an item the engine could not place stays with you and says so.\n\n")
	}
}

// recCell renders the recommendation column. An absent recommendation is stated as a
// debt the desk owes, not left blank: a blank cell reads as "nothing to say", and the
// truth is "nobody has said anything yet".
func (d *digest) recCell(r row) string {
	if r.HasRec {
		return r.Rec
	}
	return "_none on file — the desk owes one_"
}

// defaultCell renders the default-if-no-answer column per the brief: a date 14 days out
// where a safe default exists, and the literal `no default — blocks until answered`
// where none does. deskdigest never invents a default, so "exists" means somebody wrote
// one down.
func (d *digest) defaultCell(r row) string {
	switch r.Verdict.Class {
	case classCouldNotRead:
		return "unknown — the item could not be read"
	case classUnclassified:
		return "no default — needs classification first"
	case classReversible:
		if d.SignOff.State != signOffClaimed {
			return fmt.Sprintf("no default — %s unsigned, so the desk cannot decide it either", r3ID)
		}
		return fmt.Sprintf("desk may decide under %s; %d-day veto once recorded here", r3ID, vetoWindowDays)
	default: // human-only
		if !r.HasDef {
			return "no default — blocks until answered"
		}
		return fmt.Sprintf("%s — applies %s", r.Def, d.Now.AddDate(0, 0, defaultAnswerDays).Format("2006-01-02"))
	}
}

func (d *digest) renderDecisions(b *strings.Builder) {
	fmt.Fprintf(b, "## Desk decisions taken under %s (veto window)\n\n", r3ID)
	if len(d.Decisions) == 0 {
		if d.SignOff.State == signOffUnsigned {
			fmt.Fprintf(b, "None — %s is unsigned, so the desk has taken no decision under it. "+
				"This line is not an omission; it is the record for the week.\n\n", r3ID)
		} else {
			fmt.Fprintf(b, "None recorded this week. %s makes this digest the veto surface, so an absent "+
				"section would be an absent veto window — hence the explicit none.\n\n", r3ID)
		}
		return
	}
	fmt.Fprintf(b, "%d recorded. Veto by replying on the item; silence past the deadline lets the decision stand.\n\n",
		len(d.Decisions))
	b.WriteString("| Item | Decision | Recorded | Veto closes |\n|---|---|---|---|\n")
	for _, dec := range d.Decisions {
		fmt.Fprintf(b, "| [%s#%d](%s) | %s | %s | %s |\n",
			dec.Item.Repo, dec.Item.Number, dec.Item.URL, dec.Decision,
			dec.Taken.Format("2006-01-02"), dec.VetoEnds.Format("2006-01-02"))
	}
	b.WriteString("\n")
}

// renderGaps is the could-not-read section at REPO granularity. It is unconditional:
// "no gaps" is as much a finding as a list of them, and a section that appears only when
// something broke trains the reader to skim past its absence.
func (d *digest) renderGaps(b *strings.Builder) {
	b.WriteString("## Read gaps\n\n")
	var gaps []string
	for _, rr := range d.Coll.Repos {
		if rr.Err != "" {
			gaps = append(gaps, fmt.Sprintf("- `%s` — **did not answer**: %s. Its decision items are UNKNOWN "+
				"and are NOT counted as zero above.", rr.Repo, rr.Err))
		}
		for _, ml := range rr.MissingLabels {
			gaps = append(gaps, fmt.Sprintf("- `%s` — the label `%s` does not exist in this repo. "+
				"`gh issue list --label %s` returns an empty array and exit 0 there, so zero rows under it "+
				"means the label has never been applied, NOT that the queue is empty.", rr.Repo, ml, ml))
		}
	}
	for _, r := range d.Rows {
		if r.Verdict.Class == classCouldNotRead {
			gaps = append(gaps, fmt.Sprintf("- `%s#%d` — %s", r.Item.Repo, r.Item.Number, oneLine(r.Verdict.Why)))
		}
	}
	if len(gaps) == 0 {
		b.WriteString("None. Every repo in scope answered, every input label exists, and every item's " +
			"thread was read.\n\n")
		return
	}
	sort.Strings(gaps)
	b.WriteString(strings.Join(gaps, "\n") + "\n\n")
}

// renderCoverage states the arithmetic so a reader can check that no input fell out.
// The brief's review gate is "no item can silently drop out of the queue (every input
// issue appears or the digest names why not)" — this is that property, made checkable
// from the artifact rather than from the source.
func (d *digest) renderCoverage(b *strings.Builder) {
	b.WriteString("## Coverage\n\n")
	collected := len(d.Coll.Items)
	fmt.Fprintf(b, "- repos in scope: %d — %s\n", len(d.Coll.Scope), joinCode(d.Coll.Scope))
	fmt.Fprintf(b, "- input labels: `%s`\n", strings.Join(digestLabels, "`, `"))
	fmt.Fprintf(b, "- items collected: %d · rows rendered: %d\n", collected, len(d.Rows))
	if collected != len(d.Rows) {
		fmt.Fprintf(b, "- **COVERAGE DEFECT: %d collected items did not render.** Treat this digest as "+
			"incomplete and do not read the queue as exhaustive.\n", collected-len(d.Rows))
	} else {
		b.WriteString("- every collected item is rendered above; the two counts agree.\n")
	}
	b.WriteString("\n")
}

func (d *digest) renderBoundary(b *strings.Builder) {
	b.WriteString("## What produced this, and what it will not do\n\n")
	b.WriteString("`deskdigest` reports. It holds no authority to close, flip, label or decide, and " +
		"constructs no argv that could:\n\n")
	b.WriteString("- superseding last week's digest is a CLOSE, which belongs to `deskclose` — this tool " +
		"prints the command and runs nothing;\n")
	b.WriteString("- filing anything other than this one weekly issue belongs to `deskfile`;\n")
	b.WriteString("- the decision-SLA / escalation clock belongs to `issueboard` — the age column here is " +
		"plain item age, not a second threshold drifting alongside that one.\n")
}

func (d *digest) oldestAge() int {
	m := 0
	for _, r := range d.Rows {
		if r.AgeDays > m {
			m = r.AgeDays
		}
	}
	return m
}

// supersedeHint is the exact command a human or the desk runs to close LAST week's
// digest once this week's exists. Printed to stderr by --post and NEVER executed: the
// close belongs to deskclose's lane B, which has its own R-1 sign-off gate. The previous
// week's number is left as a placeholder on purpose — deskdigest does not go looking for
// an issue to close, not even to name it.
func supersedeHint(repo string, cur int) string {
	return fmt.Sprintf("deskclose superseded -R %s <last week's digest #> --by %d", repo, cur)
}

func repoReadSummary(rr repoRead) string {
	switch {
	case rr.Err != "":
		return "DID NOT ANSWER: " + rr.Err
	case len(rr.MissingLabels) > 0:
		return fmt.Sprintf("read; label(s) `%s` do not exist there, so nothing could have been filed under them",
			strings.Join(rr.MissingLabels, "`, `"))
	default:
		return "read; no open items under the input labels"
	}
}

func joinCode(ss []string) string {
	if len(ss) == 0 {
		return "(none)"
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = "`" + s + "`"
	}
	return strings.Join(out, ", ")
}

// wholeDays is the age in whole days, floored at 0 — a clock skew must not print a
// negative age.
func wholeDays(from, to time.Time) int {
	if from.IsZero() {
		return 0
	}
	d := int(to.Sub(from).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

// isoWeek renders the ISO-8601 year-week id used as the digest's identity.
func isoWeek(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w)
}
