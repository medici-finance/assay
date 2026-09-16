package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// mailbox_test.go — the POLL path of a routed message (#1166). The drain must
// hand every accepted, in-lane, routed message to the ADDRESSEE ROLE's
// mailbox via commsqueue.DeliverToMailbox, so `deskcomms poll` as that role
// sees it and `deskcomms ack <id>` clears it. Before this landed the mailbox
// had no production writer: a message reached the executor leg (when armed)
// but a desk window polling its mailbox received nothing.
//
// The reads below go through commsqueue.PollMailbox / AckMailbox — the exact
// calls the gateway's poll/ack ops (../commsgw/socket.go handlePoll/handleAck,
// via mailbox.go's aliases) make on behalf of `deskcomms poll` / `deskcomms
// ack`, whose (cell, role) come from the polling session's own identity.

// cellRoles is the within-cell desk mesh (internal/comms/laneacl.yaml).
var cellRoles = []string{"the-desk", "intake-desk", "worker-desk", "pr-review-desk", "verify-desk"}

func pollAs(t *testing.T, root, cell, role string) []commsqueue.Notice {
	t.Helper()
	got, err := commsqueue.PollMailbox(root, cell, role)
	if err != nil {
		t.Fatalf("PollMailbox(%s/%s): %v", cell, role, err)
	}
	return got
}

// assertOnlyAddresseeSees pins role-scoped visibility: the addressee's poll
// returns exactly the one notice; every other role in the cell (the sender
// included) polls empty.
func assertOnlyAddresseeSees(t *testing.T, root, cell, addressee, id string) commsqueue.Notice {
	t.Helper()
	got := pollAs(t, root, cell, addressee)
	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("poll as the addressee (%s) must see exactly the routed message %q, got %+v", addressee, id, got)
	}
	for _, other := range cellRoles {
		if other == addressee {
			continue
		}
		if n := pollAs(t, root, cell, other); len(n) != 0 {
			t.Fatalf("poll as %s must NOT see a message addressed to %s, got %+v", other, addressee, n)
		}
	}
	return got[0]
}

// TestDeliveredToAddresseeMailbox routes one accepted message per in-lane
// verb (handoff / notify / ask) and proves: the addressee polls it (1), every
// other role polls empty (0), ack clears it (0) and moves it to the acked
// partition (never deletes), and the message otherwise lands exactly as
// before (accepted-queue retired, journal line, nothing held).
func TestDeliveredToAddresseeMailbox(t *testing.T) {
	cases := []struct {
		verb, class, action string
	}{
		{"handoff", "routine", ActionRouteWorkDispatch}, // -> session tier
		{"notify", "routine", ActionLandReport},         // -> local tier
		{"ask", "sensitive", ActionRouteReview},         // -> session tier
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.verb, func(t *testing.T) {
			loop, root, filer := newTestLoop(t)
			loop.Router = &Router{Question: routerQuestion, Advisor: &spyAdvisor{answer: tc.action}, Journal: &memJournal{}, Budget: deskkit.NewBudget(0, 0)}
			id := "mb-" + tc.verb
			payload := `{"kind":"` + tc.verb + `","note":"for the addressee"}`
			env := routedEnv(id, "cell-a", "the-desk", "cell-a", "worker-desk", tc.verb, tc.class, payload)
			plantAccepted(t, root, env)

			if n := pollAs(t, root, "cell-a", "worker-desk"); len(n) != 0 {
				t.Fatalf("mailbox must be empty before the drain runs, got %+v", n)
			}

			drainOnce(t, loop)

			// The message lands exactly as before: retired from accepted,
			// journalled, never held, nothing filed.
			if acc := acceptedIDs(t, root); len(acc) != 0 {
				t.Fatalf("a routed message must leave the accepted-queue, still queued: %v", acc)
			}
			if held, _ := commsqueue.ListHeld(root); len(held) != 0 || filer.calls != 0 {
				t.Fatalf("a routed in-lane message must not quarantine: held=%v filed=%d", held, filer.calls)
			}
			if !strings.Contains(journalLog(t, root), "landed id="+id) {
				t.Fatalf("the landing must still be journalled")
			}

			// Role-scoped visibility, field-for-field.
			n := assertOnlyAddresseeSees(t, root, "cell-a", "worker-desk", id)
			if n.From != env.From || n.Verb != tc.verb || n.Class != tc.class || !sameJSON(t, n.Payload, payload) {
				t.Fatalf("the delivered notice must carry the envelope's from/verb/class/payload: got %+v", n)
			}
			if n.Sent != env.Sent {
				t.Fatalf("the notice's Sent must be the envelope's, got %v want %v", n.Sent, env.Sent)
			}

			// ack <id> clears it — a MOVE to the acked partition, never a delete.
			if err := commsqueue.AckMailbox(root, "cell-a", "worker-desk", id); err != nil {
				t.Fatalf("AckMailbox: %v", err)
			}
			if after := pollAs(t, root, "cell-a", "worker-desk"); len(after) != 0 {
				t.Fatalf("poll after ack must be empty, got %+v", after)
			}
			acked := filepath.Join(commsqueue.MailboxAckedDir(root, "cell-a", "worker-desk"), id+".json")
			if _, err := os.Stat(acked); err != nil {
				t.Fatalf("ack must MOVE the notice into the acked partition (%s): %v", acked, err)
			}
		})
	}
}

// TestNotDeliveredWhenRefused pins the negative half: a message refused BEFORE
// routing — by the routing-boundary ACL re-check or by the router's own
// quarantine default — is held and never reaches any mailbox.
func TestNotDeliveredWhenRefused(t *testing.T) {
	t.Run("out-of-lane at the routing boundary", func(t *testing.T) {
		loop, root, _ := newTestLoop(t)
		loop.Router = &Router{Question: routerQuestion, Advisor: &spyAdvisor{answer: ActionRouteWorkDispatch}, Journal: &memJournal{}, Budget: deskkit.NewBudget(0, 0)}
		// "status" is a cross-cell-only verb used within-cell: ACL-refused.
		plantAccepted(t, root, routedEnv("mb-refused-lane", "cell-a", "the-desk", "cell-a", "worker-desk", "status", "routine", `{}`))
		drainOnce(t, loop)
		if held := heldIDsOf(t, root); !held["mb-refused-lane"] {
			t.Fatalf("the out-of-lane message must be held, held=%v", held)
		}
		for _, role := range cellRoles {
			if n := pollAs(t, root, "cell-a", role); len(n) != 0 {
				t.Fatalf("a refused message must never be delivered; %s polled %+v", role, n)
			}
		}
	})
	t.Run("quarantined by the router", func(t *testing.T) {
		loop, root, _ := newTestLoop(t)
		// An empty (malformed) advisor answer bounds to the default: quarantine.
		loop.Router = &Router{Question: routerQuestion, Advisor: &spyAdvisor{answer: ""}, Journal: &memJournal{}, Budget: deskkit.NewBudget(0, 0)}
		plantAccepted(t, root, routedEnv("mb-refused-router", "cell-a", "the-desk", "cell-a", "worker-desk", "handoff", "routine", `{}`))
		drainOnce(t, loop)
		if held := heldIDsOf(t, root); !held["mb-refused-router"] {
			t.Fatalf("the router-quarantined message must be held, held=%v", held)
		}
		for _, role := range cellRoles {
			if n := pollAs(t, root, "cell-a", role); len(n) != 0 {
				t.Fatalf("a quarantined message must never be delivered; %s polled %+v", role, n)
			}
		}
	})
}

// TestDeliveryIsAdditiveToExecutorLeg proves the executor leg is untouched by
// delivery: with Native armed (fake ACP agent, dispatch_native_test.go) the
// session still fires exactly once AND the addressee's mailbox holds the
// notice. With Native at its zero value (production today) nothing fires and
// the mailbox is still written — delivery is the always-on path.
func TestDeliveryIsAdditiveToExecutorLeg(t *testing.T) {
	t.Run("native armed: session fires and mailbox is written", func(t *testing.T) {
		executorTestHome(t)
		loop, root, _ := newTestLoop(t)
		loop.Router = &Router{Question: routerQuestion, Advisor: &spyAdvisor{answer: ActionRouteWorkDispatch}, Journal: &memJournal{}, Budget: deskkit.NewBudget(0, 0)}
		wt := t.TempDir()
		spawns := 0
		loop.Native = true
		loop.RunnerCmd = []string{os.Args[0]}
		loop.NativeEnv = []string{"COMMSLOOP_FAKE_ACP=roundtrip"}
		loop.NativeTimeout = 20 * time.Second
		loop.MakeWorktree = func(loopengine.Item) (string, func(), error) {
			spawns++
			return wt, func() {}, nil
		}
		plantAccepted(t, root, routedEnv("mb-native", "cell-a", "the-desk", "cell-a", "worker-desk", "handoff", "routine", `{"kind":"request-act"}`))
		drainOnce(t, loop)
		if spawns != 1 {
			t.Fatalf("the executor leg must fire exactly once with Native armed, spawns=%d", spawns)
		}
		assertOnlyAddresseeSees(t, root, "cell-a", "worker-desk", "mb-native")
	})
	t.Run("native off: nothing fires, mailbox is still written", func(t *testing.T) {
		loop, root, _ := newTestLoop(t)
		loop.Router = &Router{Question: routerQuestion, Advisor: &spyAdvisor{answer: ActionRouteWorkDispatch}, Journal: &memJournal{}, Budget: deskkit.NewBudget(0, 0)}
		loop.MakeWorktree = func(loopengine.Item) (string, func(), error) {
			t.Fatal("Native is off: no worktree/session may be created")
			return "", nil, nil
		}
		plantAccepted(t, root, routedEnv("mb-inert", "cell-a", "the-desk", "cell-a", "worker-desk", "handoff", "routine", `{}`))
		drainOnce(t, loop)
		assertOnlyAddresseeSees(t, root, "cell-a", "worker-desk", "mb-inert")
		if !strings.Contains(journalLog(t, root), "no session fired") {
			t.Fatalf("with Native off the landing must still record that no session fired")
		}
	})
}

// TestDeliveryFailureFailsClosed: a message that cannot be placed in the
// addressee's mailbox is NOT landed — Dispatch refuses (unverifiable), the
// item stays in the accepted-queue for the engine's retry, and no PASS is
// synthesized for a message nobody can poll.
func TestDeliveryFailureFailsClosed(t *testing.T) {
	loop, root, _ := newTestLoop(t)
	loop.Router = &Router{Question: routerQuestion, Advisor: &spyAdvisor{answer: ActionRouteWorkDispatch}, Journal: &memJournal{}, Budget: deskkit.NewBudget(0, 0)}
	// A regular FILE where the mailbox tree must be created makes every
	// mailbox write fail (ENOTDIR), independent of permissions/uid.
	if err := os.WriteFile(filepath.Join(root, "mailbox"), []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	plantAccepted(t, root, routedEnv("mb-undeliverable", "cell-a", "the-desk", "cell-a", "worker-desk", "handoff", "routine", `{}`))

	items, err := loop.SelectQueue()
	if err != nil || len(items) != 1 {
		t.Fatalf("SelectQueue: items=%d err=%v", len(items), err)
	}
	tier, err := loop.TierPolicy(items[0])
	if err != nil || tier == loopengine.TierHuman {
		t.Fatalf("TierPolicy: tier=%v err=%v (want a dispatchable tier)", tier, err)
	}
	h, err := loop.Dispatch(items[0], tier)
	if err == nil || h != nil {
		t.Fatalf("Dispatch must refuse when the mailbox write fails, got handle=%v err=%v", h, err)
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("an undeliverable message is could-not-deliver (unverifiable), got %v", err)
	}
	if acc := acceptedIDs(t, root); len(acc) != 1 || acc[0] != "mb-undeliverable" {
		t.Fatalf("the undelivered message must stay in the accepted-queue, got %v", acc)
	}
	if strings.Contains(journalLog(t, root), "landed id=mb-undeliverable") {
		t.Fatalf("an undelivered message must never be journalled as landed")
	}
}

// TestRedispatchNeverResurrectsAckedNotice: the engine may call Dispatch again
// for the same item (a retry after a rate-limited executor dispatch). A notice
// the addressee has ALREADY acked must not reappear in its mailbox.
func TestRedispatchNeverResurrectsAckedNotice(t *testing.T) {
	loop, root, _ := newTestLoop(t)
	loop.Router = &Router{Question: routerQuestion, Advisor: &spyAdvisor{answer: ActionRouteWorkDispatch}, Journal: &memJournal{}, Budget: deskkit.NewBudget(0, 0)}
	plantAccepted(t, root, routedEnv("mb-again", "cell-a", "the-desk", "cell-a", "worker-desk", "handoff", "routine", `{}`))
	items, err := loop.SelectQueue()
	if err != nil || len(items) != 1 {
		t.Fatalf("SelectQueue: items=%d err=%v", len(items), err)
	}
	tier, _ := loop.TierPolicy(items[0])
	if _, err := loop.Dispatch(items[0], tier); err != nil {
		t.Fatalf("first Dispatch: %v", err)
	}
	assertOnlyAddresseeSees(t, root, "cell-a", "worker-desk", "mb-again")
	if err := commsqueue.AckMailbox(root, "cell-a", "worker-desk", "mb-again"); err != nil {
		t.Fatalf("AckMailbox: %v", err)
	}
	if _, err := loop.Dispatch(items[0], tier); err != nil {
		t.Fatalf("second Dispatch: %v", err)
	}
	if n := pollAs(t, root, "cell-a", "worker-desk"); len(n) != 0 {
		t.Fatalf("a re-dispatch must not resurrect an acked notice, got %+v", n)
	}
}

// --- local helpers (names chosen not to collide with the bypass battery's) ---

func acceptedIDs(t *testing.T, root string) []string {
	t.Helper()
	items, err := commsqueue.ListAccepted(root)
	if err != nil {
		t.Fatalf("ListAccepted: %v", err)
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Envelope.ID)
	}
	return out
}

func heldIDsOf(t *testing.T, root string) map[string]bool {
	t.Helper()
	held, err := commsqueue.ListHeld(root)
	if err != nil {
		t.Fatalf("ListHeld: %v", err)
	}
	out := map[string]bool{}
	for _, h := range held {
		out[h.Envelope.ID] = true
	}
	return out
}

func journalLog(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "journal.log"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read journal.log: %v", err)
	}
	return string(raw)
}

// sameJSON compares a delivered payload to the planted one modulo whitespace:
// the queue's atomic writer re-indents the raw JSON on disk (the same shape
// every accepted-queue write already has), so the payload is carried opaquely
// but not byte-identically.
func sameJSON(t *testing.T, got json.RawMessage, want string) bool {
	t.Helper()
	var g, w bytes.Buffer
	if err := json.Compact(&g, got); err != nil {
		t.Fatalf("delivered payload is not JSON: %v (%q)", err, got)
	}
	if err := json.Compact(&w, []byte(want)); err != nil {
		t.Fatalf("planted payload is not JSON: %v", err)
	}
	return bytes.Equal(g.Bytes(), w.Bytes())
}
