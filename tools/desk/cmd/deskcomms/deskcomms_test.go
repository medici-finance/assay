package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeGateway is the injected transport under test: it records what it was handed
// and returns a configured Receipt / error, so a verb's whole path runs with no
// socket. The record is what lets a test PROVE the client signed and addressed the
// message it claims to have sent.
type fakeGateway struct {
	submitted [][]byte
	receipt   Receipt
	err       error
	polled    []Notice
	acked     []string
}

func (f *fakeGateway) Submit(raw []byte) (Receipt, error) {
	f.submitted = append(f.submitted, append([]byte(nil), raw...))
	if f.err != nil {
		return Receipt{}, f.err
	}
	return f.receipt, nil
}

func (f *fakeGateway) Poll(cell, role string) ([]Notice, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.polled, nil
}

func (f *fakeGateway) Ack(cell, role, id string) error {
	if f.err != nil {
		return f.err
	}
	f.acked = append(f.acked, id)
	return nil
}

// testDeps builds a fully-injected deps: fixed identity from context (cell-a /
// worker-desk), a real ed25519 signer, a fake gateway that accepts, deterministic
// id/nonce, and no rate-limit gate. The returned public key verifies the assertion
// the client mints.
func testDeps(t *testing.T, payload string) (*deps, *fakeGateway, ed25519.PublicKey, *bytes.Buffer) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating keypair: %v", err)
	}
	gw := &fakeGateway{receipt: Receipt{ID: "gw-1", Accepted: true}}
	out := &bytes.Buffer{}
	d := &deps{
		stdin:    strings.NewReader(payload),
		stdout:   out,
		now:      func() time.Time { return time.Unix(1700000000, 0).UTC() },
		cell:     "cell-a",
		self:     "worker-desk",
		signer:   comms.Ed25519Signer{Key: priv},
		gateway:  gw,
		newID:    func() string { return "msg-test-1" },
		newNonce: func() string { return "nonce-test-1" },
	}
	return d, gw, pub, out
}

// --- happy path: the client signs and addresses a real, verifiable envelope ----

func TestSendHappyPathSignsAndAddresses(t *testing.T) {
	d, gw, pub, out := testDeps(t, "please pick up brief 7")
	oc, err := cmdSend(d, []string{"--to", "pr-review-desk", "--verb", "handoff"})
	if err != nil {
		t.Fatalf("want a clean send, got error: %v", err)
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d", deskkit.ExitCodeOf(err))
	}
	if len(gw.submitted) != 1 {
		t.Fatalf("want exactly one submission, got %d", len(gw.submitted))
	}
	if oc.bucket != "cell-a/pr-review-desk" {
		t.Fatalf("want bucket cell-a/pr-review-desk, got %q", oc.bucket)
	}
	if !strings.Contains(out.String(), "gw-1") {
		t.Fatalf("want the receipt id echoed, got %q", out.String())
	}

	// The submitted bytes must parse AND authenticate against the signer's key —
	// proving the assertion was minted over THIS envelope, not merely attached.
	env, perr := comms.ParseEnvelope(gw.submitted[0])
	if perr != nil {
		t.Fatalf("submitted envelope does not parse: %v", perr)
	}
	trust := comms.Ed25519TrustStore{"cell-a": pub}
	if verr := comms.VerifyEnvelope(env, d.now(), trust, time.Minute, comms.NewMemReplayGuard()); verr != nil {
		t.Fatalf("submitted envelope does not verify: %v", verr)
	}
	if env.From.Cell != "cell-a" || env.From.Role != "worker-desk" {
		t.Fatalf("sender identity not taken from context: got %s/%s", env.From.Cell, env.From.Role)
	}
	if env.To.Cell != "cell-a" || env.To.Role != "pr-review-desk" {
		t.Fatalf("destination not addressed as given: got %s/%s", env.To.Cell, env.To.Role)
	}
}

// The sender identity is the session's, regardless of arguments — there is no
// argument that names the sender, so the only identity a message can carry is the
// context one. This is the impersonation-precedent property expressed as a test.
func TestSendSenderIdentityIsFromContextNotArgument(t *testing.T) {
	d, gw, _, _ := testDeps(t, "hi")
	// Even trying to smuggle a role through the destination flags cannot change FROM.
	if _, err := cmdSend(d, []string{"--to", "pr-review-desk", "--verb", "notify"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, _ := comms.ParseEnvelope(gw.submitted[0])
	if env.From.Role != "worker-desk" || env.From.Cell != "cell-a" {
		t.Fatalf("sender identity should be the session's (cell-a/worker-desk), got %s/%s", env.From.Cell, env.From.Role)
	}
}

// --- refusal battery (each names a distinct refusing layer) --------------------

func TestRefusesOutOfLaneTriple(t *testing.T) {
	// worker-desk -> worker-desk within the same cell: a lane between a role and
	// itself is not a lane. The ACL refuses it (deny-by-default).
	d, gw, _, _ := testDeps(t, "x")
	_, err := cmdSend(d, []string{"--to", "worker-desk", "--verb", "handoff"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "acl") {
		t.Fatalf("want an ACL refusal, got %v", err)
	}
	if len(gw.submitted) != 0 {
		t.Fatalf("a refused message must never reach the gateway; got %d submissions", len(gw.submitted))
	}
}

func TestRefusesReservedVerb(t *testing.T) {
	d, gw, _, _ := testDeps(t, "x")
	_, err := cmdSend(d, []string{"--to", "pr-review-desk", "--verb", "approve"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "reserved-verb") {
		t.Fatalf("want a reserved-verb refusal naming the human-gate action, got %v", err)
	}
	if len(gw.submitted) != 0 {
		t.Fatalf("a reserved verb must never reach the gateway; got %d submissions", len(gw.submitted))
	}
}

func TestRefusesCrossCellFromNonCoordinator(t *testing.T) {
	// worker-desk reaching into cell-b: cross-cell reach is the-desk <-> the-desk,
	// and its verb set ships empty — so this is refused twice over.
	d, gw, _, _ := testDeps(t, "x")
	_, err := cmdSend(d, []string{"--to", "worker-desk", "--to-cell", "cell-b", "--verb", "handoff"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "acl") {
		t.Fatalf("want an ACL cross-cell refusal, got %v", err)
	}
	if len(gw.submitted) != 0 {
		t.Fatalf("a cross-cell message must never reach the gateway; got %d submissions", len(gw.submitted))
	}
}

func TestRefusesOversizePayload(t *testing.T) {
	// A payload over the structured-body cap is refused at parse (before bodycheck),
	// by the SAME comms.ParseEnvelope the gateway runs.
	big := strings.Repeat("a", comms.MaxPayloadBytes+512)
	d, gw, _, _ := testDeps(t, big)
	_, err := cmdSend(d, []string{"--to", "pr-review-desk", "--verb", "handoff"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Fatalf("want a parse/size refusal, got %v", err)
	}
	if len(gw.submitted) != 0 {
		t.Fatalf("an oversize message must never reach the gateway; got %d submissions", len(gw.submitted))
	}
}

func TestRefusesTokenShapedPayload(t *testing.T) {
	// A credential-shaped body trips the content scan (bodycheck) before the message
	// is ever minted or submitted. The token literal is split so the test source
	// itself carries no contiguous credential-shaped run.
	tokenish := "here is a token " + "ghp" + "_0123456789ABCDEFabcdef0123456789ABCD"
	d, gw, _, _ := testDeps(t, tokenish)
	_, err := cmdSend(d, []string{"--to", "pr-review-desk", "--verb", "handoff"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "bodycheck") {
		t.Fatalf("want a bodycheck refusal, got %v", err)
	}
	if len(gw.submitted) != 0 {
		t.Fatalf("a credential-shaped message must never reach the gateway; got %d submissions", len(gw.submitted))
	}
}

func TestRefusesBreakerTrip(t *testing.T) {
	// When the shared rate limit / circuit breaker refuses, the send is refused with
	// exit 4 and never reaches the gateway.
	d, gw, _, _ := testDeps(t, "x")
	d.rateCheck = func(bucket string) error {
		return deskkit.RateLimited("circuit breaker open for " + bucket)
	}
	_, err := cmdSend(d, []string{"--to", "pr-review-desk", "--verb", "handoff"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRateLimited {
		t.Fatalf("want exit 4, got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
	if len(gw.submitted) != 0 {
		t.Fatalf("a rate-limited message must never reach the gateway; got %d submissions", len(gw.submitted))
	}
}

func TestRefusesKillSwitchArmed(t *testing.T) {
	// The kill switch is the mandatory first gate (via run/Guard): an armed DISABLED
	// stops the verb before any message is built. HOME is redirected so the disabled
	// audit line lands in a temp dir, not the real one.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	rc := run([]string{"send", "--to", "pr-review-desk", "--verb", "handoff"})
	if rc != deskkit.ExitDisabled {
		t.Fatalf("want exit 3 (disabled), got %d", rc)
	}
}

func TestRefusesGatewayDownNoFallback(t *testing.T) {
	// An unreachable gateway is could-not-submit (exit 6), fail closed: there is no
	// local-spool path in the code at all, so "no fallback write" holds by
	// construction. We assert the exit and the fail-closed message.
	d, gw, _, _ := testDeps(t, "x")
	gw.err = fmt.Errorf("%w: dial unix: connection refused", ErrGatewayUnreachable)
	_, err := cmdSend(d, []string{"--to", "pr-review-desk", "--verb", "handoff"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("want exit 6 (could-not-submit), got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "no local fallback") {
		t.Fatalf("want a fail-closed message, got %v", err)
	}
}

func TestRefusesPollWhenGatewayDown(t *testing.T) {
	d, gw, _, _ := testDeps(t, "")
	gw.err = fmt.Errorf("%w: dial unix: connection refused", ErrGatewayUnreachable)
	_, err := cmdPoll(d, nil)
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("want exit 6 (could-not-poll), got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
}

// --- read side -----------------------------------------------------------------

func TestPollReadsOwnMailbox(t *testing.T) {
	d, gw, _, out := testDeps(t, "")
	gw.polled = []Notice{
		{ID: "n-1", From: comms.SenderID{Cell: "cell-a", Role: "the-desk"}, Verb: "notify", Class: "routine"},
	}
	oc, err := cmdPoll(d, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "n-1") {
		t.Fatalf("want the notice id printed, got %q", out.String())
	}
	if !strings.Contains(oc.detail, "1 notice") {
		t.Fatalf("want the count in the detail, got %q", oc.detail)
	}
}

func TestAckMovesNotice(t *testing.T) {
	d, gw, _, _ := testDeps(t, "")
	_, err := cmdAck(d, []string{"n-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gw.acked) != 1 || gw.acked[0] != "n-1" {
		t.Fatalf("want n-1 acked, got %v", gw.acked)
	}
}

func TestAckRequiresExactlyOneID(t *testing.T) {
	d, _, _, _ := testDeps(t, "")
	if _, err := cmdAck(d, nil); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want a refusal for a missing id, got %v", err)
	}
	if _, err := cmdAck(d, []string{"a", "b"}); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want a refusal for two ids, got %v", err)
	}
}
