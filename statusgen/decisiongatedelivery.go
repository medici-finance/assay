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
// (GITHUB_EVENT_PATH): the pull request's number and body, and the repository. Each brief the
// body names in a `Brief:` trailer, read the way reconcile reads it (parseLinkTrailers), counts as
// moved to implemented by this change when its row is on the board below implemented. Both gates
// then judge that move as they judge a written one. The base, the cutover and the scope are
// unchanged. A brief whose `delivery:` list declares this pull request a partial delivery is not
// moved, because reconcile does not move it either (briefdelivery.go).
//
// WHAT IT DOES NOT CHANGE. Outside a pull_request event (push, schedule, a local run, reconcile's
// hold) nothing is read and both gates behave as before. A brief already merged is never judged
// again: no pull request event names it. A held row already on main stays as it is.
//
// THREE-STATE. A pull_request event whose payload cannot be read cannot say what this change
// delivers. The decision-gate hold then refuses (fail-closed); it never reads that as "delivers
// nothing".

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// gateDelivery is what a pull request's `Brief:` trailers deliver.
type gateDelivery struct {
	PR   int            // the pull request's number
	Keys map[string]int // board key (<stream>/<normalised NN>, lowercased) → PR number
}

// delivers reports the pull request number that delivers the brief at stream/num, or 0.
func (d gateDelivery) delivers(stream, num string) int {
	if len(d.Keys) == 0 {
		return 0
	}
	return d.Keys[strings.ToLower(stream)+"/"+normBriefNum(num)]
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
	return deliveryFromBody(root, ev.Repository.FullName, ev.PullRequest.Number, body), ""
}

// deliveryFromBody maps the body's `Brief:` trailers to board keys. Every trailer counts, so a body
// naming two briefs is judged for both. A trailer the brief's own `delivery:` list declares a
// partial delivery of this pull request (in this repository) is dropped, as reconcile drops it.
func deliveryFromBody(root, repo string, number int, body string) gateDelivery {
	d := gateDelivery{PR: number, Keys: map[string]int{}}
	_, briefs, _ := parseLinkTrailers(body)
	if len(briefs) == 0 {
		return d
	}
	var partials declaredPartials
	if repo != "" {
		partials = readDeclaredPartials(root, repo)
	}
	for _, b := range briefs {
		stream, num, ok := briefStreamNum(b)
		if !ok {
			continue // not a board id; reconcile cannot credit it either
		}
		if partials.partial(number, b) {
			continue
		}
		d.Keys[strings.ToLower(stream)+"/"+normBriefNum(num)] = number
	}
	return d
}

// withDelivery returns head with every brief d delivers moved to implemented, when its row stands
// on the board below implemented. head itself is not changed.
func (d gateDelivery) withDelivery(head gateSnapshot) gateSnapshot {
	if len(d.Keys) == 0 {
		return head
	}
	out := make(gateSnapshot, len(head))
	for k, v := range head {
		out[k] = v
	}
	keys := make([]string, 0, len(d.Keys))
	for k := range d.Keys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for hk, h := range head {
			if strings.ToLower(hk) != k || !h.hasRow() || h.Unreadable != "" || gateStatusRank(h.Status) >= 1 {
				continue
			}
			moved := *h
			moved.Status = "implemented"
			moved.DeliveredBy = d.Keys[k]
			moved.DeliveredFrom = h.Status
			out[hk] = &moved
		}
	}
	return out
}
