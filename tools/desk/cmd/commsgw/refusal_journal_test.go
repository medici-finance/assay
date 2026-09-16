package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// refusal_journal_test.go — the FAIL-FIRST drill for #1165: N refusals of
// each kind, driven through the REAL transport entry points (the socket
// server's handleSubmit and the A2A agent's Execute), must leave exactly N
// journal lines of that kind on the gateway's journal.log, attributed to the
// sender and lane AS PRESENTED, stamped with the gateway clock, and carrying
// no payload bytes.
//
// DELIBERATELY written against the symbols the merge-base already had
// (SocketServer, GatewayAgent, handleSubmit, the fixture) and string
// literals for the kind names, so that run against the unfixed code it
// compiles and reports "0 lines" — a clean red — rather than a compile
// error. The finer contracts (classification exhaustiveness, the
// peer-unauthenticated line, the gateway cell stamp, the write-failure
// receipt) live in refusal_test.go and use the new symbols.

// journaledRefusal is a LOCAL decode of one journal line — not the shared
// record type — so this file has no dependency on the fix's own symbols.
type journaledRefusal struct {
	Time    time.Time      `json:"time"`
	Kind    string         `json:"kind"`
	ID      string         `json:"id"`
	Cell    string         `json:"cell"`
	Refusal string         `json:"refusal"`
	From    comms.SenderID `json:"from"`
	To      comms.Lane     `json:"to"`
	Verb    string         `json:"verb"`
}

// readRefusalJournal returns every line of root/journal.log (raw text and
// decoded). A missing journal is an empty result — that IS the merge-base
// state this drill reddens on.
func readRefusalJournal(t *testing.T, root string) (rawLines []string, recs []journaledRefusal) {
	t.Helper()
	f, err := os.Open(filepath.Join(root, "journal.log"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		t.Fatalf("open journal.log: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		rawLines = append(rawLines, line)
		var r journaledRefusal
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("journal line does not decode as JSON: %v\n%s", err, line)
		}
		recs = append(recs, r)
	}
	return rawLines, recs
}

// payloadCanary is a string that appears ONLY in the refused messages'
// payloads; its presence anywhere in journal.log is the "payload leaked"
// signal.
const payloadCanary = "CANARY-PAYLOAD-MUST-NEVER-BE-JOURNALLED-7f3a"

// addPayload injects the canary payload into a raw envelope AFTER signing.
// The assertion signs only {cell, role, msg id, nonce, iat, exp}
// (identity.go's canonicalAssertionBytes), so the signature stays valid and
// the refusal under drill is still the stage's own, not a parse failure.
// Done on a generic map so this file never depends on wireEnvelope carrying
// a payload field.
func addPayload(t *testing.T, raw []byte) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("addPayload: unmarshal: %v", err)
	}
	m["payload"] = json.RawMessage(`{"note":"` + payloadCanary + `"}`)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("addPayload: marshal: %v", err)
	}
	return out
}

// refusalDrill is one kind's drill: build N raw envelopes that the gateway
// must refuse with that kind, and the presented sender/lane the journal must
// attribute them to.
type refusalDrill struct {
	kind string
	// setup mutates the fixture/server before the drill (rate limiter,
	// guard, clock).
	setup func(f *testFixture, s *SocketServer)
	// envelopes returns the N raw messages to submit, in order, and the
	// presented (from, to, verb) every refusal line must carry.
	envelopes func(t *testing.T, f *testFixture, s *SocketServer, n int) (raws [][]byte, from comms.SenderID, to comms.Lane, verb string)
	// receipt is the substring the client receipt must carry (the refusal
	// itself is unchanged by the journal line).
	receipt string
}

var (
	drillFrom = comms.SenderID{Cell: "cell-a", Role: "the-desk"}
	drillTo   = comms.Lane{Cell: "cell-a", Role: "worker-desk"}
)

// resign re-mints the envelope's assertion by hand with the given
// (cell, role, id, nonce) and re-marshals — the shape precheck_test.go's
// replay / unknown-cell cases already use.
func resign(t *testing.T, f *testFixture, raw []byte, cell, role, id, nonce string) []byte {
	t.Helper()
	var we wireEnvelope
	if err := json.Unmarshal(raw, &we); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	a, err := comms.Mint(cell, role, id, nonce, f.now, 0, comms.Ed25519Signer{Key: f.priv})
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	we.Sig = a
	out, err := json.Marshal(we)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return out
}

// seedAccepted submits raw and asserts it cleared PreCheck. With a nil prose
// gate RunOutbound then HOLDS it (fail closed), so "cleared PreCheck" reads
// as a prose-gate hold on the receipt — which is exactly the signal that no
// deterministic stage refused it.
func seedAccepted(t *testing.T, s *SocketServer, raw []byte) {
	t.Helper()
	resp := s.handleSubmit(gwRequest{Op: "submit", Message: raw})
	if resp.Receipt == nil || !strings.Contains(resp.Receipt.Detail, "prose gate") {
		t.Fatalf("seed delivery must clear PreCheck (a nil gate then holds it), got %+v", resp)
	}
}

func refusalDrills() []refusalDrill {
	simple := func(n int, t *testing.T, f *testFixture, id string, mutate func(*wireEnvelope)) [][]byte {
		raws := make([][]byte, 0, n)
		for i := 0; i < n; i++ {
			raws = append(raws, addPayload(t, f.envelope(t, id+"-"+string(rune('a'+i)), mutate)))
		}
		return raws
	}
	return []refusalDrill{
		{
			kind: "envelope-parse",
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				raws := make([][]byte, 0, n)
				for i := 0; i < n; i++ {
					// Well-formed JSON, addressing present, but a schema this
					// reader refuses — parse fails AFTER the addressing is
					// readable, so the line must still attribute it.
					raws = append(raws, []byte(`{"schema":"not-cellmsg","id":"parse-`+string(rune('a'+i))+`","from":{"cell":"cell-a","role":"the-desk"},"to":{"cell":"cell-a","role":"worker-desk"},"verb":"handoff","payload":{"note":"`+payloadCanary+`"}}`))
				}
				return raws, drillFrom, drillTo, "handoff"
			},
			receipt: ErrEnvelopeParse.Error(),
		},
		{
			kind: "unknown-cell",
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				raws := make([][]byte, 0, n)
				for i := 0; i < n; i++ {
					id := "ghost-" + string(rune('a'+i))
					raw := f.envelope(t, id, func(we *wireEnvelope) {
						we.Cell = "cell-ghost"
						we.From.Cell = "cell-ghost"
					})
					raws = append(raws, addPayload(t, resign(t, f, raw, "cell-ghost", "the-desk", id, id+"-nonce")))
				}
				return raws, comms.SenderID{Cell: "cell-ghost", Role: "the-desk"}, drillTo, "handoff"
			},
			receipt: comms.ErrUnknownCell.Error(),
		},
		{
			kind: "bad-signature",
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				raws := make([][]byte, 0, n)
				for i := 0; i < n; i++ {
					var we wireEnvelope
					if err := json.Unmarshal(f.envelope(t, "badsig-"+string(rune('a'+i)), nil), &we); err != nil {
						t.Fatal(err)
					}
					we.Sig.Sig = append([]byte(nil), we.Sig.Sig...)
					we.Sig.Sig[0] ^= 0xFF
					out, _ := json.Marshal(we)
					raws = append(raws, addPayload(t, out))
				}
				return raws, drillFrom, drillTo, "handoff"
			},
			receipt: comms.ErrBadSignature.Error(),
		},
		{
			kind: "expired",
			setup: func(f *testFixture, s *SocketServer) {
				later := f.now.Add(comms.DefaultTTL + time.Minute)
				s.Now = func() time.Time { return later }
			},
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				return simple(n, t, f, "expired", nil), drillFrom, drillTo, "handoff"
			},
			receipt: comms.ErrExpired.Error(),
		},
		{
			kind: "not-yet-valid",
			setup: func(f *testFixture, s *SocketServer) {
				earlier := f.now.Add(-(comms.DefaultSkew + time.Minute))
				s.Now = func() time.Time { return earlier }
			},
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				return simple(n, t, f, "early", nil), drillFrom, drillTo, "handoff"
			},
			receipt: comms.ErrNotYetValid.Error(),
		},
		{
			kind: "replay",
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				// One accepted delivery consumes the nonce; every later
				// message re-minted on the SAME nonce is a replay.
				seedAccepted(t, s, addPayload(t, resign(t, f, f.envelope(t, "replay-seed", nil), "cell-a", "the-desk", "replay-seed", "shared-nonce")))
				raws := make([][]byte, 0, n)
				for i := 0; i < n; i++ {
					id := "replay-" + string(rune('a'+i))
					raws = append(raws, addPayload(t, resign(t, f, f.envelope(t, id, nil), "cell-a", "the-desk", id, "shared-nonce")))
				}
				return raws, drillFrom, drillTo, "handoff"
			},
			receipt: comms.ErrReplay.Error(),
		},
		{
			kind: "identity-mismatch",
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				raws := make([][]byte, 0, n)
				for i := 0; i < n; i++ {
					id := "mismatch-" + string(rune('a'+i))
					// Assertion binds the-desk; the envelope DECLARES
					// worker-desk — the presented (unverified) sender.
					var we wireEnvelope
					if err := json.Unmarshal(f.envelope(t, id, nil), &we); err != nil {
						t.Fatal(err)
					}
					we.From.Role = "worker-desk"
					we.To.Role = "the-desk"
					out, _ := json.Marshal(we)
					raws = append(raws, addPayload(t, out))
				}
				return raws, comms.SenderID{Cell: "cell-a", Role: "worker-desk"}, comms.Lane{Cell: "cell-a", Role: "the-desk"}, "handoff"
			},
			receipt: comms.ErrIdentityMismatch.Error(),
		},
		{
			kind: "lane-denied",
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				raws := simple(n, t, f, "lane", func(we *wireEnvelope) { we.To.Role = "janitor" })
				return raws, drillFrom, comms.Lane{Cell: "cell-a", Role: "janitor"}, "handoff"
			},
			receipt: ErrLaneDenied.Error(),
		},
		{
			kind: "cross-cell-pair",
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				raws := simple(n, t, f, "xpair", func(we *wireEnvelope) {
					we.From.Role = "worker-desk"
					we.To = comms.Lane{Cell: "cell-b", Role: "the-desk"}
					we.Verb = "status"
				})
				return raws, comms.SenderID{Cell: "cell-a", Role: "worker-desk"}, comms.Lane{Cell: "cell-b", Role: "the-desk"}, "status"
			},
			receipt: ErrCrossCellPair.Error(),
		},
		{
			kind: "cross-cell-verb",
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				raws := simple(n, t, f, "xverb", func(we *wireEnvelope) {
					we.To = comms.Lane{Cell: "cell-b", Role: "the-desk"}
					we.Verb = "handoff"
				})
				return raws, drillFrom, comms.Lane{Cell: "cell-b", Role: "the-desk"}, "handoff"
			},
			receipt: ErrCrossCellVerb.Error(),
		},
		{
			kind: "duplicate",
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				raws := make([][]byte, 0, n)
				for i := 0; i < n; i++ {
					id := "dup-" + string(rune('a'+i))
					seedAccepted(t, s, addPayload(t, resign(t, f, f.envelope(t, id, nil), "cell-a", "the-desk", id, id+"-nonce-1")))
					// Same id, FRESH nonce: not a replay — a duplicate id.
					raws = append(raws, addPayload(t, resign(t, f, f.envelope(t, id, nil), "cell-a", "the-desk", id, id+"-nonce-2")))
				}
				return raws, drillFrom, drillTo, "handoff"
			},
			receipt: ErrDuplicateMessage.Error(),
		},
		{
			kind: "budget-exhausted",
			setup: func(f *testFixture, s *SocketServer) {
				s.Deps.RateLimiter = NewRateLimiter(1, time.Minute)
			},
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				seedAccepted(t, s, addPayload(t, f.envelope(t, "budget-seed", nil)))
				return simple(n, t, f, "budget", nil), drillFrom, drillTo, "handoff"
			},
			receipt: ErrBudgetExhausted.Error(),
		},
		{
			kind: "rate-limiter-unconfigured",
			setup: func(f *testFixture, s *SocketServer) {
				s.Deps.RateLimiter = nil
			},
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				return simple(n, t, f, "nolimiter", nil), drillFrom, drillTo, "handoff"
			},
			receipt: ErrRateLimiterUnconfigured.Error(),
		},
		{
			kind: "kill-switch",
			setup: func(f *testFixture, s *SocketServer) {
				f.guard = func() error { return errors.New("STOP armed (drill)") }
			},
			envelopes: func(t *testing.T, f *testFixture, s *SocketServer, n int) ([][]byte, comms.SenderID, comms.Lane, string) {
				return simple(n, t, f, "ks", nil), drillFrom, drillTo, "handoff"
			},
			receipt: ErrKillSwitch.Error(),
		},
	}
}

// TestRefusalJournalEveryKind drives N refusals of every kind through the
// socket server and finds N journal lines of that kind — 0 on the
// merge-base, where a refusal wrote nothing.
func TestRefusalJournalEveryKind(t *testing.T) {
	const n = 3
	for _, d := range refusalDrills() {
		d := d
		t.Run(d.kind, func(t *testing.T) {
			f := newFixture(t)
			root := t.TempDir()
			s := &SocketServer{Root: root, Deps: f.deps, Emitter: NoOpInboxEmitter{}, Now: func() time.Time { return f.now }}
			if d.setup != nil {
				d.setup(f, s)
			}
			raws, from, to, verb := d.envelopes(t, f, s, n)
			want := s.Now()
			for i, raw := range raws {
				resp := s.handleSubmit(gwRequest{Op: "submit", Message: raw})
				if resp.Receipt == nil || resp.Receipt.Accepted {
					t.Fatalf("[%d] must be refused, got %+v", i, resp)
				}
				if !strings.Contains(resp.Receipt.Detail, d.receipt) {
					t.Fatalf("[%d] receipt must still carry the refusal %q, got %q", i, d.receipt, resp.Receipt.Detail)
				}
			}

			rawLines, recs := readRefusalJournal(t, root)
			got := 0
			for _, r := range recs {
				if r.Refusal == d.kind {
					got++
				}
			}
			if got != n {
				t.Fatalf("%d refusals of kind %q must leave %d journal lines of that kind, found %d (journal has %d lines: %v)", n, d.kind, n, got, len(rawLines), rawLines)
			}
			if len(recs) != n {
				t.Fatalf("ONLY refusals journal (an accepted seed leaves no refusal line): want %d lines, got %d: %v", n, len(recs), rawLines)
			}
			for _, r := range recs {
				if r.Kind != "refused" {
					t.Fatalf("every refusal line carries kind \"refused\", got %+v", r)
				}
				if r.ID == "" {
					t.Fatalf("every refusal line carries a non-empty id, got %+v", r)
				}
				if r.From != from || r.To != to || r.Verb != verb {
					t.Fatalf("line must carry the sender/lane AS PRESENTED (%v -> %v verb %q), got %+v", from, to, verb, r)
				}
				if !r.Time.Equal(want) {
					t.Fatalf("line must be stamped with the gateway clock %v, got %v", want, r.Time)
				}
			}
			for _, line := range rawLines {
				if strings.Contains(line, payloadCanary) {
					t.Fatalf("a refusal line must NEVER carry payload bytes, found the canary in: %s", line)
				}
			}
		})
	}
}

// TestRefusalJournalA2APath proves the cross-cell transport journals a
// refusal through the SAME line shape: a forged-peer envelope refused by
// GatewayAgent.Execute leaves one line on the agent's root.
func TestRefusalJournalA2APath(t *testing.T) {
	const n = 3
	f := newFixture(t)
	root := t.TempDir()
	g := GatewayAgent{Root: root, Deps: f.deps, Emitter: NoOpInboxEmitter{}, Now: func() time.Time { return f.now }}

	for i := 0; i < n; i++ {
		id := "a2a-forged-" + string(rune('a'+i))
		raw := f.envelope(t, id, func(we *wireEnvelope) {
			we.Cell, we.From.Cell = "cell-ghost", "cell-ghost"
			we.To = comms.Lane{Cell: "cell-a", Role: "the-desk"}
			we.Verb = "status"
		})
		raw = addPayload(t, resign(t, f, raw, "cell-ghost", "the-desk", id, id+"-nonce"))
		ec := &a2asrv.ExecutorContext{Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewRawPart(raw))}
		var replies int
		for ev, err := range g.Execute(t.Context(), ec) {
			if err != nil {
				t.Fatalf("Execute yielded error: %v", err)
			}
			msg, ok := ev.(*a2a.Message)
			if !ok {
				t.Fatalf("want a terminal *a2a.Message, got %T", ev)
			}
			var r wireReply
			if err := json.Unmarshal(msg.Parts[0].Raw(), &r); err != nil {
				t.Fatalf("reply decode: %v", err)
			}
			if r.Accepted || !strings.Contains(r.Detail, comms.ErrUnknownCell.Error()) {
				t.Fatalf("forged peer must be refused as unknown-cell, got %+v", r)
			}
			replies++
		}
		if replies != 1 {
			t.Fatalf("exactly one terminal reply, got %d", replies)
		}
	}

	rawLines, recs := readRefusalJournal(t, root)
	if len(recs) != n {
		t.Fatalf("%d A2A refusals must leave %d journal lines, found %d: %v", n, n, len(recs), rawLines)
	}
	for _, r := range recs {
		if r.Refusal != "unknown-cell" || r.From.Cell != "cell-ghost" || r.To != (comms.Lane{Cell: "cell-a", Role: "the-desk"}) {
			t.Fatalf("A2A refusal line must carry kind + presented sender/lane, got %+v", r)
		}
	}
	for _, line := range rawLines {
		if strings.Contains(line, payloadCanary) {
			t.Fatalf("a refusal line must NEVER carry payload bytes, found the canary in: %s", line)
		}
	}
}
