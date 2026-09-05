package improve

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/askassay"
)

// Row is one line in a strip. Every row can be asked for its ID and can refuse
// to render — a row that cannot state what it is, or that is missing the field
// that makes it meaningful, takes its whole strip to could-not-check rather
// than rendering as a shorter list.
type Row interface {
	RowID() string
	Validate() error
	Render() string
}

// Class is the good/bad/ugly taxonomy (§5.3). It is rendered as DATA and is
// disputable like any other field — §13.5b records that the taxonomy has not
// been hardened against a cadence of real reports, and that who assigns the
// class is an unruled question. What is enforced here is only that the class
// is one of the three declared values: an unrecognised class is refused rather
// than silently bucketed, because silently bucketing is how a taxonomy freezes
// without anybody deciding to freeze it.
type Class string

const (
	// ClassGood — what worked; patterns worth promoting into skills and
	// conventions.
	ClassGood Class = "good"
	// ClassBad — failures with evidence; the change-failure numerator,
	// itemised. Tunes existing machinery.
	ClassBad Class = "bad"
	// ClassUgly — process-integrity incidents. The ONLY class allowed to
	// motivate new enforcement, which is what keeps the system from growing
	// controls in response to noise.
	ClassUgly Class = "ugly"
)

var declaredClasses = map[Class]bool{ClassGood: true, ClassBad: true, ClassUgly: true}

// Classes returns the declared taxonomy, in the order §5.3 states it.
func Classes() []Class { return []Class{ClassGood, ClassBad, ClassUgly} }

// ReportRow is one filed report in the good/bad/ugly stream.
type ReportRow struct {
	ID string
	// Class is the taxonomy value. Unrecognised values are refused.
	Class Class
	// Title is what the report says in words.
	Title string
	// Program and Epic are the scopes the strip filters on. Empty means
	// unscoped, which is legal and is rendered as unscoped — never as a
	// default program.
	Program string
	Epic    string
	// Evidence is the links this report rests on. §7.3: "every report links
	// its evidence." A report with none does not render, because a report
	// with no evidence is an opinion with a class attached.
	Evidence []string
}

func (r ReportRow) RowID() string { return r.ID }

// Validate refuses a report that cannot honestly render.
func (r ReportRow) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("report has no ID")
	}
	if strings.TrimSpace(r.Title) == "" {
		return fmt.Errorf("%s: report has no title", r.ID)
	}
	if !declaredClasses[r.Class] {
		return fmt.Errorf("%s: %q is not one of the declared classes (%s) — an unrecognised class is refused rather than bucketed, so that adding a class is a decision rather than a side effect", r.ID, r.Class, classList())
	}
	if len(nonEmpty(r.Evidence)) == 0 {
		return fmt.Errorf("%s: report carries no evidence link — the strip's contract is that every report links its evidence, so a report with none is refused rather than rendered as an unsupported claim", r.ID)
	}
	return nil
}

func (r ReportRow) Render() string {
	return fmt.Sprintf("report %s · class=%s · program=%s · epic=%s · evidence=%s · %s",
		r.ID, r.Class, scope(r.Program), scope(r.Epic),
		strings.Join(nonEmpty(r.Evidence), ","), r.Title)
}

// ClusterRow is one candidate systemic issue: a grouping of report IDs that
// the analysis pass asserts are one problem.
type ClusterRow struct {
	ID string
	// Title is the assertion in words ("3 tier-downgrade reports, 2 programs,
	// 2 weeks").
	Title string
	// MemberIDs are the reports this cluster groups. A cluster with none is
	// not a cluster.
	MemberIDs []string
	// Window is the span over which recurrence was judged. It is part of the
	// claim, not chrome.
	Window string
}

func (c ClusterRow) RowID() string { return c.ID }

func (c ClusterRow) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return errors.New("cluster has no ID")
	}
	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("%s: cluster has no title — a cluster is an assertion that N signals are one issue, and an untitled cluster does not make it", c.ID)
	}
	if len(nonEmpty(c.MemberIDs)) == 0 {
		return fmt.Errorf("%s: cluster groups no members — a cluster of nothing is not an empty cluster, it is a grouping that failed", c.ID)
	}
	if strings.TrimSpace(c.Window) == "" {
		return fmt.Errorf("%s: cluster states no window — three reports in two weeks and three reports in two years are different findings, and without the window the row does not say which one it is", c.ID)
	}
	return nil
}

func (c ClusterRow) Render() string {
	return fmt.Sprintf("cluster %s · members=%d · window=%s · %s",
		c.ID, len(nonEmpty(c.MemberIDs)), c.Window, c.Title)
}

// Unresolved is a reference a strip could not join. It is REPORTED, never
// dropped and never counted as clean: dropping it makes a cluster look tidier
// than it is, and a tidy render of a broken join is the failure mode this
// whole package guards.
type Unresolved struct {
	// From is the row that held the reference.
	From string
	// Ref is the reference that did not resolve.
	Ref string
	// Why says what was looked for and where.
	Why string
}

func (u Unresolved) String() string {
	return fmt.Sprintf("%s references %s, which did not resolve: %s", u.From, u.Ref, u.Why)
}

// ResolveClusterMembers joins clusters to the reports they group. Every member
// ID that has no matching report is returned as [Unresolved]; none is removed
// from its cluster's member list.
//
// The second return is the drill-down map §7.3 asks for — a cluster one click
// from its members — and it contains ONLY resolved members. The two returns
// together are the honest picture: what you can click through to, and what you
// cannot.
func ResolveClusterMembers(clusters []ClusterRow, reports []ReportRow) (map[string][]ReportRow, []Unresolved) {
	byID := make(map[string]ReportRow, len(reports))
	for _, r := range reports {
		byID[r.ID] = r
	}
	members := make(map[string][]ReportRow, len(clusters))
	var unresolved []Unresolved
	for _, c := range clusters {
		for _, id := range c.MemberIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				unresolved = append(unresolved, Unresolved{From: c.ID, Ref: "(empty)",
					Why: "the cluster carries an empty member reference — an empty slot is a lost member, not an absent one"})
				continue
			}
			r, ok := byID[id]
			if !ok {
				unresolved = append(unresolved, Unresolved{From: c.ID, Ref: id,
					Why: "no report with this ID is in the rendered report set — it may be outside the strip's filter, outside its window, or absent from the source. The member stays counted in the cluster and is NOT clickable"})
				continue
			}
			members[c.ID] = append(members[c.ID], r)
		}
	}
	sort.Slice(unresolved, func(i, j int) bool {
		if unresolved[i].From != unresolved[j].From {
			return unresolved[i].From < unresolved[j].From
		}
		return unresolved[i].Ref < unresolved[j].Ref
	})
	return members, unresolved
}

// ProposalRow is one open proposal in the retro queue: a process change
// awaiting the human gate, with the evidence that motivated it.
type ProposalRow struct {
	ID string
	// Title is the proposed change in words.
	Title string
	// IntakeRef is where the proposal lives in the append-only register. The
	// pane renders it; the pane never writes it.
	IntakeRef string
	// MotivatingEvidence is the measurement that motivated the change — the
	// deck's hard rule, "no change without the measurement that motivated it".
	// A proposal with none does not render.
	MotivatingEvidence []string
	// TargetMetric is the number the change is expected to move — the missing
	// twin, "no change without the measurement that judges it". A proposal
	// that names no target cannot be judged later, so it does not render.
	TargetMetric string
	// Adopt is the human-gate action. It routes; it does not write.
	Adopt AdoptAction
}

func (p ProposalRow) RowID() string { return p.ID }

func (p ProposalRow) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("proposal has no ID")
	}
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("%s: proposal has no title", p.ID)
	}
	if strings.TrimSpace(p.IntakeRef) == "" {
		return fmt.Errorf("%s: proposal names no register entry — a proposal the pane cannot point at in the append-only register is one the pane is holding on its own, which is exactly what it must not do", p.ID)
	}
	if len(nonEmpty(p.MotivatingEvidence)) == 0 {
		return fmt.Errorf("%s: proposal carries no motivating evidence — no change without the measurement that motivated it", p.ID)
	}
	if strings.TrimSpace(p.TargetMetric) == "" {
		return fmt.Errorf("%s: proposal names no target metric — no change without the measurement that JUDGES it, and a change with no named target can never appear honestly in the did-it-work strip", p.ID)
	}
	if err := p.Adopt.Validate(p.ID); err != nil {
		return fmt.Errorf("%s: %w", p.ID, err)
	}
	return nil
}

func (p ProposalRow) Render() string {
	return fmt.Sprintf("proposal %s · register=%s · target=%s · evidence=%s · adopt=%s · %s",
		p.ID, p.IntakeRef, p.TargetMetric,
		strings.Join(nonEmpty(p.MotivatingEvidence), ","),
		p.Adopt.Render(), p.Title)
}

// Verdict is the did-it-work answer for one adopted change.
type Verdict string

const (
	// VerdictMoved — both figures were measured and the metric moved in the
	// direction the change named.
	VerdictMoved Verdict = "moved"
	// VerdictNoMovement — both figures were measured and the metric did not
	// move. This is the verdict the strip exists to make visible, and it is
	// only reachable when BOTH figures are real.
	VerdictNoMovement Verdict = "no-movement"
	// VerdictRegressed — both figures were measured and the metric moved
	// against the change's stated direction.
	VerdictRegressed Verdict = "regressed"
	// VerdictUndetermined — at least one figure does not exist. NOT
	// no-movement: they look identical on a screen and mean opposite things.
	VerdictUndetermined Verdict = "undetermined"
)

// Direction is the movement a change said it wanted.
type Direction string

const (
	// DirectionUp — the change expected the metric to rise.
	DirectionUp Direction = "up"
	// DirectionDown — the change expected the metric to fall.
	DirectionDown Direction = "down"
)

var declaredDirections = map[Direction]bool{DirectionUp: true, DirectionDown: true}

// RetroActionRow is one ADOPTED process change with the before/after of the
// metric it named. Before and After are answer-layer figures, not integers:
// that is what makes "the metric could not be read" representable at all.
type RetroActionRow struct {
	ID string
	// Title is the change in words.
	Title string
	// AdoptedBy names the identity that committed the adoption, and Cadence
	// which cadence consumed it. Both are rendered: an adoption with no named
	// human is not a gold gate that was passed, it is one that was skipped.
	AdoptedBy string
	Cadence   string
	// TargetMetric is the number the change said it would move.
	TargetMetric string
	// Direction is the movement it said it wanted.
	Direction Direction
	// Before and After are the two readings. Either may be could-not-check,
	// and when either is, the verdict is undetermined.
	Before askassay.Answer
	After  askassay.Answer
}

func (a RetroActionRow) RowID() string { return a.ID }

func (a RetroActionRow) Validate() error {
	if strings.TrimSpace(a.ID) == "" {
		return errors.New("retro action has no ID")
	}
	if strings.TrimSpace(a.Title) == "" {
		return fmt.Errorf("%s: retro action has no title", a.ID)
	}
	if strings.TrimSpace(a.AdoptedBy) == "" {
		return fmt.Errorf("%s: retro action names no adopter — adoption is a human gate, and an adopted change with no named human is a gate that was not passed", a.ID)
	}
	if strings.TrimSpace(a.Cadence) == "" {
		return fmt.Errorf("%s: retro action names no cadence — one adoption per cadence is unenforceable if a row does not say which cadence it consumed", a.ID)
	}
	if strings.TrimSpace(a.TargetMetric) == "" {
		return fmt.Errorf("%s: retro action names no target metric, so there is nothing for this strip to judge it against", a.ID)
	}
	if !declaredDirections[a.Direction] {
		return fmt.Errorf("%s: %q is not a declared direction (up, down) — without one, a moved number cannot be told from a regressed one and every change looks successful", a.ID, a.Direction)
	}
	return nil
}

// Verdict is the judgement. It returns [VerdictUndetermined] whenever either
// figure is absent, and it is the single most load-bearing branch in this
// package: the alternative — treating a missing figure as no movement — turns
// an unwired metric into a verdict about a process change.
func (a RetroActionRow) Verdict() Verdict {
	before, okBefore := a.Before.Value()
	after, okAfter := a.After.Value()
	if !okBefore || !okAfter {
		return VerdictUndetermined
	}
	if before == after {
		return VerdictNoMovement
	}
	rose := after > before
	if (a.Direction == DirectionUp && rose) || (a.Direction == DirectionDown && !rose) {
		return VerdictMoved
	}
	return VerdictRegressed
}

// VerdictReason says why the verdict is what it is. For an undetermined row it
// names WHICH figure was missing and quotes that figure's own reason, so the
// operator learns what to go and wire rather than that the change did nothing.
func (a RetroActionRow) VerdictReason() string {
	_, okBefore := a.Before.Value()
	_, okAfter := a.After.Value()
	switch {
	case !okBefore && !okAfter:
		return fmt.Sprintf("UNDETERMINED — neither figure exists. before: %s. after: %s. This is not no-movement: nothing was measured", reasonOf(a.Before), reasonOf(a.After))
	case !okBefore:
		return fmt.Sprintf("UNDETERMINED — the BEFORE figure does not exist (%s), so there is no baseline to judge the after against. This is not no-movement", reasonOf(a.Before))
	case !okAfter:
		return fmt.Sprintf("UNDETERMINED — the AFTER figure does not exist (%s), so the change has not been judged at all. This is not no-movement", reasonOf(a.After))
	}
	return fmt.Sprintf("both figures measured; expected direction %s", a.Direction)
}

func reasonOf(ans askassay.Answer) string {
	if r := strings.TrimSpace(ans.Reason()); r != "" {
		return r
	}
	return "no reason recorded"
}

func (a RetroActionRow) Render() string {
	return fmt.Sprintf("retro-action %s · verdict=%s · target=%s · direction=%s · adopted-by=%s · cadence=%s · %s\n    before: %s\n    after: %s\n    verdict-reason: %s",
		a.ID, a.Verdict(), a.TargetMetric, a.Direction, a.AdoptedBy, a.Cadence, a.Title,
		a.Before.Render(), a.After.Render(), a.VerdictReason())
}

func classList() string {
	out := make([]string, 0, len(declaredClasses))
	for c := range declaredClasses {
		out = append(out, string(c))
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func scope(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unscoped"
	}
	return s
}
