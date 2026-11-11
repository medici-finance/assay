package main

// decisiongatedelivery.go — a pull request that DELIVERS a brief is judged as the move it causes
// (#5507).
//
// THE DEFECT CLASS. A delivering pull request does not move its brief's README row. It names the
// brief in a `Brief:` trailer in its body; once it merges, reconcile credits it through that
// trailer and writes the row to implemented. The decision-gate hold (lifecycle-v1 §4.5) and the
// design-approval gate (§4.4) judge only what the change WRITES, so the delivering pull request
// passes both: it writes no move. The move happens after the merge, and reconcile's own lint hold
// then keeps the row back. The brief stays held on the board with no pull request left to fix the
// missing keys on.
//
// THE CHECK. On a pull_request event, --lint reads the event payload the runner wrote
// (GITHUB_EVENT_PATH): the pull request's number and body, and the repository. The body's brief
// is read exactly the way reconcile reads it: a body carrying exactly ONE `Brief:` trailer names
// one brief (singleBriefTrailer, ghfetch.go), reduced to its join key by canonicalBriefKey and
// matched against the board id as the row spells it. A body with no trailer or with two names
// nothing, and a value reconcile cannot join (`sdlc/018` against row 18, `sdlc/18.`) names
// nothing either: reconcile never credits it, so the merge moves no row. The named brief counts
// as moved to implemented by this change only when its row stands at a status reconcile writes
// from, todo or in-progress (reconcileMovesFrom). Both gates then judge that move as they judge a
// written one. The base, the cutover and the scope are unchanged.
//
// PARTIAL DELIVERIES. A brief whose `delivery:` list declares this pull request a partial delivery
// in this repository is not moved, because reconcile does not move it (briefdelivery.go). A
// partial claim whose `in:` cannot be resolved makes the brief derive unknown in reconcile, which
// moves nothing and refuses --apply; the gates likewise judge no move and the hold reports the
// reason as a notice. That is a residual: should the claim later resolve, reconcile may move the
// row after the merge with no pre-merge check of the keys.
//
// DIFFERENTIAL LINT. Under --diff-base the base tree is linted by a re-run of the binary with the
// PR event removed from its environment (withoutPREvent, difflint.go): the PR delivers to the head,
// so its refusals are never demoted as pre-existing on base.
//
// WHAT IT DOES NOT CHANGE. Outside a pull_request event (push, schedule, a local run, reconcile's
// hold) nothing is read and both gates behave as before. A brief already merged is never judged
// again: no pull request event names it. A held row already on main stays as it is, and a row at
// blocked, withdrawn, implemented or later is never judged as moved: reconcile does not write it.
// Only the boards in the tree being linted are read, so a delivery credited to a brief in another
// stream root is never judged here (a residual: that root's reconcile can still move it).
//
// THREE-STATE. A pull_request event whose payload cannot be read (no GITHUB_EVENT_PATH, an
// unreadable file, a payload that is not JSON, no pull request number, or a delivering body with
// no repository to resolve its partial claims against) cannot say what this change delivers. The
// decision-gate hold then refuses (fail-closed), whether or not the tree carries .git; it never
// reads that as "delivers nothing". The design gate reads the same payload and leaves that
// refusal to the hold rather than reporting it twice.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// gateDelivery is what a pull request's `Brief:` trailer delivers.
type gateDelivery struct {
	PR  int    // the pull request's number
	Key string // canonicalBriefKey of the one trailer reconcile credits; "" = delivers nothing
	// Undecided is why a partial `delivery:` claim on the named brief could not be resolved
	// against this repository: reconcile derives it unknown and moves nothing, so Key is "".
	Undecided string
}

// reconcileMovesFrom reports whether reconcile's writer moves a row standing at status: it
// writes only from todo or in-progress (reconcileapply.go). Every other status is immutable
// there, so a pull request naming such a brief moves nothing on merge.
func reconcileMovesFrom(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "todo", "in-progress":
		return true
	}
	return false
}

// delivers reports the pull request number that delivers the brief at stream/num, or 0. num is
// the board's own spelling: reconcile joins on it exactly.
func (d gateDelivery) delivers(stream, num string) int {
	if d.Key == "" || d.Key != canonicalBriefKey(stream+"/"+num) {
		return 0
	}
	return d.PR
}

// prDeliveryFn is the seam the tests replace; it reads the runner's event.
var prDeliveryFn = prDeliveryFromEvent

// prDeliveryFromEvent reads the pull request this lint runs on, if any. why is non-empty when a
// pull_request event's payload could not be read.
func prDeliveryFromEvent(root string) (d gateDelivery, why string) {
	switch os.Getenv("GITHUB_EVENT_NAME") {
	case "pull_request", "pull_request_target":
	default:
		return d, ""
	}
	p := os.Getenv("GITHUB_EVENT_PATH")
	if p == "" {
		return d, "GITHUB_EVENT_PATH is unset"
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return d, fmt.Sprintf("the event payload could not be read (%v)", err)
	}
	var ev struct {
		PullRequest *struct {
			Number int     `json:"number"`
			Body   *string `json:"body"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return d, fmt.Sprintf("the event payload is not JSON (%v)", err)
	}
	if ev.PullRequest == nil || ev.PullRequest.Number <= 0 {
		return d, "the event payload carries no pull request number"
	}
	body := ""
	if ev.PullRequest.Body != nil {
		body = *ev.PullRequest.Body
	}
	return deliveryFromBody(root, ev.Repository.FullName, ev.PullRequest.Number, body)
}

// deliveryFromBody reads the body's brief the way reconcile credits it: exactly one `Brief:`
// trailer (singleBriefTrailer), keyed by canonicalBriefKey. A trailer the brief's own `delivery:`
// list declares a partial delivery of this pull request (in this repository) is dropped, as
// reconcile drops it; one whose partial claim cannot be resolved is dropped with the reason.
func deliveryFromBody(root, repo string, number int, body string) (gateDelivery, string) {
	d := gateDelivery{PR: number}
	ref, ok := singleBriefTrailer(body)
	if !ok {
		return d, "" // unlinked or multi-linked: reconcile credits no brief
	}
	if repo == "" {
		return d, "the event payload names no repository, so the brief's partial delivery: claims cannot be resolved"
	}
	key := canonicalBriefKey(ref)
	partials := readDeclaredPartials(root, repo)
	if partials.partial(number, ref) {
		return d, ""
	}
	ids := make([]string, 0, len(partials.undecided))
	for id := range partials.undecided {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if canonicalBriefKey(id) == key {
			d.Undecided = partials.undecided[id]
			return d, ""
		}
	}
	d.Key = key
	return d, ""
}

// withDelivery returns head with the brief d delivers moved to implemented, when its row stands
// at a status reconcile writes from (reconcileMovesFrom). head itself is not changed.
func (d gateDelivery) withDelivery(head gateSnapshot) gateSnapshot {
	if d.Key == "" {
		return head
	}
	out := make(gateSnapshot, len(head))
	for k, v := range head {
		out[k] = v
	}
	keys := make([]string, 0, len(head))
	for k := range head {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, hk := range keys {
		h := head[hk]
		if canonicalBriefKey(h.Board) != d.Key || !h.hasRow() || h.Unreadable != "" || !reconcileMovesFrom(h.Status) {
			continue
		}
		moved := *h
		moved.Status = "implemented"
		moved.DeliveredBy = d.PR
		moved.DeliveredFrom = h.Status
		out[hk] = &moved
	}
	return out
}
