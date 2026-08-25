package main

import (
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// land.go — the one-tracked-exit ledger.
//
// The whole reason this desk exists is that every inbound item leaves by exactly ONE of five
// tracked exits. Land is where that stops being an aspiration: an item that lands with NO exit is
// the front door leaking, and an item that lands with TWO is two downstream owners for one piece of
// work. Both are refusals here rather than warnings, because a warning on a drain is a line nobody
// reads.

// Exit is one of the five tracked exits, plus the explicit not-yet-routed marker.
type Exit string

const (
	// ExitPlaceholder — the item became a placeholder brief carried by a scan PR.
	ExitPlaceholder Exit = "placeholder"
	// ExitBug — the item became a filed issue.
	ExitBug Exit = "bug"
	// ExitFinding — the item became a finding entry in the register.
	ExitFinding Exit = "finding"
	// ExitNeedsDecision — the item became an entry in the single human-decision queue.
	ExitNeedsDecision Exit = "needs-decision"
	// ExitRejectedWatching — the item was explicitly rejected, or recorded as watched.
	ExitRejectedWatching Exit = "rejected-watching"

	// ExitUnrouted is NOT an exit. It is what the judgment lane returns before a model tier has
	// chosen, and Land refuses it: "we emitted it and stopped thinking about it" is precisely the
	// leak the five-exit rule exists to prevent.
	ExitUnrouted Exit = "unrouted"
)

// trackedExits is the closed set, in report order.
var trackedExits = []Exit{ExitPlaceholder, ExitBug, ExitFinding, ExitNeedsDecision, ExitRejectedWatching}

// Tracked reports whether e is one of the five.
func (e Exit) Tracked() bool {
	for _, t := range trackedExits {
		if e == t {
			return true
		}
	}
	return false
}

// Filed reports whether this exit is produced by the issue-filing lane.
func (e Exit) Filed() bool { return e == ExitBug || e == ExitNeedsDecision }

// Labels are the labels the filing verb stamps for this exit, beyond the raised-by stamp it applies
// on its own. A repo can be missing one; the filing verb prints the one-off creation command rather
// than failing, so a missing label degrades to an unstamped filing and never to a lost item.
func (e Exit) Labels() []string {
	switch e {
	case ExitBug:
		return []string{"bug"}
	case ExitNeedsDecision:
		return []string{"needs-decision"}
	default:
		return nil
	}
}

func exitNames() []string {
	out := make([]string, 0, len(trackedExits))
	for _, e := range trackedExits {
		out = append(out, string(e))
	}
	return out
}

func filedExitNames() []string {
	var out []string
	for _, e := range trackedExits {
		if e.Filed() {
			out = append(out, string(e))
		}
	}
	sort.Strings(out)
	return out
}

// ExitOf resolves the ONE exit a landed result records, reconciling the two places an exit can come
// from: the lane that executed (a mechanical dispatch knows its own exit) and the structured result
// fed back for a judgment item (a model tier's routing decision).
//
// Disagreement between them is a REFUSAL, not a precedence rule. A silent winner would make the two
// sources drift apart forever, and the first anyone would learn of it is a downstream owner acting
// on an exit nobody chose.
func ExitOf(fromLane, fromResult Exit) (Exit, error) {
	laneOK := fromLane.Tracked()
	resultOK := fromResult.Tracked()

	switch {
	case laneOK && resultOK && fromLane != fromResult:
		return "", deskkit.Refused("two tracked exits for one item: the lane recorded " + string(fromLane) +
			" and the result recorded " + string(fromResult) + ". One inbound item leaves by exactly one exit; " +
			"two exits is two downstream owners for one piece of work.")
	case resultOK:
		return fromResult, nil
	case laneOK:
		return fromLane, nil
	}

	unrouted := fromResult == ExitUnrouted || fromLane == ExitUnrouted
	if unrouted {
		return "", deskkit.Refused("the item was emitted for routing and came back UNROUTED. " +
			"Emitting is not an exit — route it to one of: " + strings.Join(exitNames(), ", "))
	}
	return "", deskkit.Refused("no tracked exit was recorded for this item. " +
		"An inbound item that lands with no exit is the front door leaking; the exits are: " +
		strings.Join(exitNames(), ", "))
}

// ExitRecord is the durable line Land writes per item.
type ExitRecord struct {
	ItemID   string
	Exit     Exit
	Lane     LaneName
	Artifact string
	Verdict  string
}

// ExitLedger accumulates the pass's records and answers the only question that matters at the end
// of a cycle: did anything come in and not leave?
type ExitLedger struct {
	records map[string]ExitRecord
	order   []string
}

func NewExitLedger() *ExitLedger { return &ExitLedger{records: map[string]ExitRecord{}} }

// Record files one exit. Recording a SECOND exit for an item already in the ledger is refused for
// the same reason ExitOf refuses a disagreement.
func (l *ExitLedger) Record(r ExitRecord) error {
	if !r.Exit.Tracked() {
		return deskkit.Refused("ledger: " + string(r.Exit) + " is not a tracked exit")
	}
	if prev, ok := l.records[r.ItemID]; ok {
		if prev.Exit == r.Exit {
			return nil // idempotent re-land of the same exit
		}
		return deskkit.Refused("ledger: " + r.ItemID + " already left by " + string(prev.Exit) +
			"; refusing to also record " + string(r.Exit))
	}
	l.records[r.ItemID] = r
	l.order = append(l.order, r.ItemID)
	return nil
}

// Records returns the pass's exits in the order they landed.
func (l *ExitLedger) Records() []ExitRecord {
	out := make([]ExitRecord, 0, len(l.order))
	for _, id := range l.order {
		out = append(out, l.records[id])
	}
	return out
}

// Unexited names the queued items that never reached the ledger. It is the leak detector, and it is
// reported on EVERY cycle rather than only when it is non-empty: a leak check that prints nothing
// when it is clean is indistinguishable from a leak check that did not run.
func (l *ExitLedger) Unexited(queued []string) []string {
	var out []string
	for _, id := range queued {
		if _, ok := l.records[id]; !ok {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// CountByExit summarises the pass.
func (l *ExitLedger) CountByExit() map[Exit]int {
	c := map[Exit]int{}
	for _, r := range l.records {
		c[r.Exit]++
	}
	return c
}

// auditExit writes the exit to the shared audit stream so the record survives the session that made
// it. A failure to write is returned, never swallowed: an exit nobody can read afterwards is not a
// record, and the drain would otherwise report a clean pass on an unwritten ledger.
func auditExit(tool string, r ExitRecord) error {
	return deskkit.Log(deskkit.Entry{
		Tool:   tool,
		Verb:   "land",
		Repo:   repoOfItemID(r.ItemID),
		Result: string(r.Exit),
		Detail: string(r.Lane) + " -> " + r.Artifact,
	})
}

// repoOfItemID recovers the repo slug from the "<owner>/<name>#<num>" item key.
func repoOfItemID(id string) string {
	slug, _, ok := strings.Cut(id, "#")
	if !ok {
		return ""
	}
	return slug
}

// resultExit reads the exit a fed result carries. The engine's Result type is frozen, so the
// routing decision rides on the Item payload it hands back rather than on a new field — heterogeneity
// lives in the adapter, not in the contract.
func resultExit(r loopengine.Result) Exit {
	if r.Item.Payload == nil {
		return ""
	}
	return Exit(strings.TrimSpace(r.Item.Payload["exit"]))
}
