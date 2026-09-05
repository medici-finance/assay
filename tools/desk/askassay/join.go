package askassay

import (
	"fmt"
	"sort"
)

// THE UNJOINABLE ROW
// ------------------
// No single board verb forms the key a PR answer needs, which is
// (repo, pr, head, verb):
//
//   - `deskboard actions` gives the verb but carries NO head SHA on its rows
//     (see the actionRow struct in cmd/deskboard/board.go).
//   - `deskboard prs` gives the head SHA but carries NO review state (see the
//     prRow struct in the same file).
//
// So a pane that answers "how many PRs need review at their current head" has
// to JOIN them, and a join has a failure mode a single read does not: rows
// that do not match. This file says exactly what happens to those rows, which
// is the question the brief asks to be answered out loud:
//
//	An unjoinable row is emitted as UNRESOLVED. It is never dropped, never
//	counted into a clean total, and never rendered as a zero. If EVERY row is
//	unresolved, the derived count is could-not-check rather than 0.
//
// Dropping unjoinable rows is the tempting behaviour and it is the one that
// manufactures a false number: the total shrinks silently and the answer looks
// cleaner the worse the data gets.

// ActionRow is one row of `deskboard actions`. It deliberately has no head
// SHA field, mirroring the verb's actual output — the absence is the point.
type ActionRow struct {
	Repo   string
	Number int
	Action string
}

// PRRow is one row of `deskboard prs`. It deliberately has no review-state
// field, mirroring the verb's actual output.
type PRRow struct {
	Repo    string
	Number  int
	HeadSHA string
}

// Resolution is why a joined row is or is not usable as a
// (repo, pr, head, verb) key.
type Resolution string

const (
	// Resolved means both sides matched and a head SHA is present.
	Resolved Resolution = "resolved"
	// UnresolvedNoPRRow means an action exists for a PR the inventory did not
	// return — commonly a scope difference or a short read between the two
	// probes, and never evidence that the PR is gone.
	UnresolvedNoPRRow Resolution = "unresolved: no PR-inventory row for this action, so no head SHA exists to key it to"
	// UnresolvedNoActionRow means the inventory returned a PR the action verb
	// did not classify.
	UnresolvedNoActionRow Resolution = "unresolved: no action row for this PR, so no verb exists to key to its head"
	// UnresolvedNoHeadSHA means both sides matched but the inventory row's
	// head SHA was empty.
	UnresolvedNoHeadSHA Resolution = "unresolved: the PR-inventory row carries an empty head SHA, so the key cannot be formed"
)

// JoinedRow is one outer-join result. Every input row on either side produces
// exactly one JoinedRow, so the join can lose information but cannot lose
// rows.
type JoinedRow struct {
	Repo       string
	Number     int
	HeadSHA    string
	Action     string
	Resolution Resolution
}

// OK reports whether this row forms a usable (repo, pr, head, verb) key.
func (j JoinedRow) OK() bool { return j.Resolution == Resolved }

// Join outer-joins action rows against PR-inventory rows on (repo, number).
// The output length is the size of the union of the two key sets: a row that
// does not join is carried through marked, never discarded.
func Join(actions []ActionRow, prs []PRRow) []JoinedRow {
	type key struct {
		repo string
		num  int
	}
	prByKey := make(map[key]PRRow, len(prs))
	for _, p := range prs {
		prByKey[key{p.Repo, p.Number}] = p
	}
	actByKey := make(map[key]ActionRow, len(actions))
	for _, a := range actions {
		actByKey[key{a.Repo, a.Number}] = a
	}

	seen := map[key]bool{}
	var out []JoinedRow

	for _, a := range actions {
		k := key{a.Repo, a.Number}
		seen[k] = true
		p, ok := prByKey[k]
		switch {
		case !ok:
			out = append(out, JoinedRow{Repo: a.Repo, Number: a.Number, Action: a.Action, Resolution: UnresolvedNoPRRow})
		case p.HeadSHA == "":
			out = append(out, JoinedRow{Repo: a.Repo, Number: a.Number, Action: a.Action, Resolution: UnresolvedNoHeadSHA})
		default:
			out = append(out, JoinedRow{Repo: a.Repo, Number: a.Number, HeadSHA: p.HeadSHA, Action: a.Action, Resolution: Resolved})
		}
	}
	for _, p := range prs {
		k := key{p.Repo, p.Number}
		if seen[k] {
			continue
		}
		out = append(out, JoinedRow{Repo: p.Repo, Number: p.Number, HeadSHA: p.HeadSHA, Resolution: UnresolvedNoActionRow})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		return out[i].Number < out[j].Number
	})
	return out
}

// Tally counts resolved and unresolved rows. Both numbers are returned
// because reporting only the first is how the unjoinable rows disappear.
func Tally(rows []JoinedRow) (resolved, unresolved int) {
	for _, r := range rows {
		if r.OK() {
			resolved++
			continue
		}
		unresolved++
	}
	return resolved, unresolved
}

// CountJoined derives an answer from a join. It is could-not-check when there
// were rows and none of them resolved — a join that resolved nothing has
// measured nothing, and "0 PRs need review" would be a fabricated result.
// When SOME rows resolved, the count is real but carries the unresolved
// remainder as a caveat, because a partial join is a partial answer.
func CountJoined(q Question, rows []JoinedRow, match func(JoinedRow) bool, st Stamp) Answer {
	resolved, unresolved := Tally(rows)
	if len(rows) > 0 && resolved == 0 {
		return Unavailable(q, fmt.Sprintf(
			"the join resolved 0 of %d rows — every row lacked either a head SHA or a counterpart, so no (repo, pr, head, verb) key could be formed. A count over an empty join is not zero, it is unmeasured",
			len(rows)), st)
	}
	var n int64
	for _, r := range rows {
		if r.OK() && match(r) {
			n++
		}
	}
	a := Computed(q, n, st)
	if unresolved > 0 && a.state == Checked {
		a.caveats = append(a.caveats, fmt.Sprintf(
			"PARTIAL JOIN: %d of %d rows are UNRESOLVED and are excluded from this figure — they are not counted as clean and not counted as failing. The true figure is at least %d and at most %d",
			unresolved, len(rows), n, n+int64(unresolved)))
	}
	return a
}
