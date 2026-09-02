package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// send.go — `deskcomms send`: hand a message to the local cell gateway after a
// fail-fast client preflight. The preflight runs the SAME internal/comms parse and
// lane-ACL the gateway re-runs authoritatively, so a refusal here is behaviour-
// identical to the gateway's and saves a round trip on the obvious ones. It is
// NOT the enforcement boundary — that is the gateway's, because not every
// participating agent runs this verb. A refusal is a STOP the desk reports, never
// routes around.
//
// IDENTITY IS NEVER SELF-CLAIMED. The sender's {cell, desk-role} come from the
// session context (client.go: resolveIdentity), never from an argument — a caller
// cannot name themselves on the command line. Only the DESTINATION is addressed by
// argument (--to / --to-cell): you say who a message is FOR, never who it is FROM.

// stringList is a repeatable flag value, for --ref given more than once.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type sendArgs struct {
	to     string
	toCell string
	verb   string
	class  string
	refs   stringList
}

// bindSend registers the send arguments. The sender's own identity is deliberately
// absent from this set — see the file header. `--to` names the destination
// desk-role; `--to-cell` an optional other cell; `--verb` the message verb.
func bindSend(fs *flag.FlagSet) *sendArgs {
	a := &sendArgs{}
	fs.StringVar(&a.to, "to", "", "destination desk-role (within this cell unless --to-cell is given)")
	fs.StringVar(&a.toCell, "to-cell", "", "destination cell (default: this session's own cell)")
	fs.StringVar(&a.verb, "verb", "", "message verb (must be a lane-ACL vocabulary member)")
	fs.StringVar(&a.class, "class", "routine", "handling class: routine | sensitive")
	fs.Var(&a.refs, "ref", "cross-reference id (repeatable)")
	return a
}

func cmdSend(d *deps, args []string) (*outcome, error) {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	a := bindSend(fs)
	if err := fs.Parse(args); err != nil {
		return &outcome{detail: "bad arguments"}, deskkit.Refused("send: " + err.Error())
	}
	return runSend(d, a)
}

// runSend is the preflight pipeline, factored out so the argument parsing and the
// checks are testable apart. Order: reserved-verb (identity-independent, so it can
// fail fast even before a session has an identity) → identity → parse → ACL →
// bodycheck → ratelimit → mint → submit. The kill switch (DISABLED > STOP >
// STOP.<loop>) is enforced by deskkit.Guard() ahead of this in run(); it is the
// mandatory first gate of every desk tool, so it precedes even the reserved-verb
// check here.
func runSend(d *deps, a *sendArgs) (*outcome, error) {
	oc := &outcome{}

	// 1. Reserved (human-gate) verb — refused up front, with its own message,
	//    before any identity is required. approve / flip / merge / ready / sign
	//    name moves only a human may make; the lane ACL can never carry one (it
	//    refuses them at load), so the gateway would refuse too. Checking it here
	//    gives the distinct, self-explaining refusal instead of a bare unknown-verb.
	verb := strings.TrimSpace(a.verb)
	if verb == "" {
		return oc, deskkit.Refused("send: --verb is required")
	}
	if root, bad := deskkit.ReservedMember(verb); bad {
		oc.detail = "reserved-verb: " + verb
		return oc, deskkit.Refused(fmt.Sprintf(
			"refused: reserved-verb — %q names a human-gate action (%q). deskcomms carries "+
				"desk-to-desk messages, never a move only a human may make; this verb can never "+
				"be on a lane, so it is refused before it is sent.", verb, root))
	}
	if strings.TrimSpace(a.to) == "" {
		return oc, deskkit.Refused("send: --to is required")
	}

	// 2. Session identity — from context, never from an argument.
	if d.cell == "" || d.self == "" {
		return oc, deskkit.Refused("send: session identity is not established (see DESK_CELL / DESK_ROLE)")
	}
	toCell := strings.TrimSpace(a.toCell)
	if toCell == "" {
		toCell = d.cell
	}
	oc.bucket = toCell + "/" + strings.TrimSpace(a.to)

	// Read the payload from stdin. It is carried as an opaque JSON string body; the
	// bodycheck below scans the RAW text (not the JSON-quoted form).
	payloadText, err := io.ReadAll(d.stdin)
	if err != nil {
		return oc, deskkit.Unverifiable("send: could not read payload from stdin", err)
	}
	payloadJSON, err := json.Marshal(string(payloadText))
	if err != nil {
		return oc, deskkit.Unverifiable("send: could not encode payload", err)
	}

	msgID := d.newID()
	env := comms.Envelope{
		Schema:  comms.Schema,
		ID:      msgID,
		Cell:    d.cell,
		From:    comms.SenderID{Cell: d.cell, Role: d.self, App: deskkit.RoleAppLoginOrEmpty(d.self)},
		To:      comms.Lane{Cell: toCell, Role: strings.TrimSpace(a.to)},
		Verb:    verb,
		Class:   strings.TrimSpace(a.class),
		Refs:    []string(a.refs),
		Payload: payloadJSON,
		Sent:    d.now().UTC(),
	}

	// 3. Parse — behaviour-identical to the gateway's ingress parse, by calling the
	//    SAME comms.ParseEnvelope (size caps, absent-triple, unknown-verb/class).
	raw, err := json.Marshal(env)
	if err != nil {
		return oc, deskkit.Unverifiable("send: could not encode envelope", err)
	}
	parsed, err := comms.ParseEnvelope(raw)
	if err != nil {
		oc.detail = "parse: " + err.Error()
		return oc, deskkit.Refused("refused: parse — " + err.Error())
	}

	// 4. Lane ACL — the SAME compiled matrix the gateway enforces. Absent is
	//    refused: a tuple the matrix does not explicitly permit is denied, with no
	//    default-allow and no wildcard. Cross-cell verbs ship empty, so every
	//    cross-cell message is refused until a recorded ruling opens the set.
	acl := comms.Compiled()
	if !acl.Allow(parsed.From.Cell, parsed.From.Role, parsed.Verb, parsed.To.Cell, parsed.To.Role) {
		oc.detail = "acl-out-of-lane"
		return oc, deskkit.Refused(fmt.Sprintf(
			"refused: acl — lane (%s/%s) --%s--> (%s/%s) is not permitted (deny-by-default; "+
				"cross-cell reach is the-desk <-> the-desk and its verb set ships empty)",
			parsed.From.Cell, parsed.From.Role, parsed.Verb, parsed.To.Cell, parsed.To.Role))
	}

	// 5. Content scan — the tokens-only preflight over the payload text. The
	//    gateway's prose gate is the independent second layer; this catches the
	//    obvious credential-shaped body before a round trip.
	if err := deskkit.BodyCheck(payloadText); err != nil {
		oc.detail = "bodycheck: " + err.Error()
		return oc, deskkit.Refused("refused: bodycheck — " + err.Error())
	}

	// 6. Rate limit + circuit breaker — one more outbound message must be within
	//    the shared budget and the breaker closed. A refusal here is exit 4 with a
	//    retry-after; it is not routed around.
	if d.rateCheck != nil {
		if err := d.rateCheck(oc.bucket); err != nil {
			oc.detail = "ratelimit: " + deskkit.StripControl(err.Error())
			return oc, err
		}
	}

	// 7. Mint the signed assertion binding {cell, role, msg-id}. The signer is this
	//    desk-role's custody-managed key. buildDeps (client.go) leaves d.signer nil and
	//    defers the custody-key load to HERE — after every refusal check above — so a
	//    reserved-verb, out-of-lane, oversize, or credential-shaped send from a session
	//    with no key still gets its own specific refusal, never a could-not-mint that
	//    would hide it. Only once a send has earned the right to be minted do we read the
	//    key. The identity that selects the key is the session context (loadSigner reads
	//    the custody path $DESK_COMMS_KEY established at token-mint time, the same context
	//    that supplied d.cell / d.self), never a flag. A signer that cannot be loaded — no
	//    key configured, unreadable, wrong mode, or malformed — surfaces loadSigner's own
	//    typed refusal (could-not-mint / custody-mode refused): fail closed, never an
	//    unsigned message. A test injects d.signer directly and so bypasses the load; the
	//    production path arrives here with d.signer nil and loads it now.
	signer := d.signer
	if signer == nil {
		loaded, lerr := loadSigner()
		if lerr != nil {
			oc.detail = deskkit.StripControl(lerr.Error())
			return oc, lerr
		}
		signer = loaded
	}
	assertion, err := comms.Mint(d.cell, d.self, msgID, d.newNonce(), d.now(), 0, signer)
	if err != nil {
		oc.detail = "mint: " + err.Error()
		return oc, deskkit.Unverifiable("could-not-mint: "+err.Error(), err)
	}
	parsed.Sig = assertion
	signed, err := json.Marshal(parsed)
	if err != nil {
		return oc, deskkit.Unverifiable("send: could not encode signed envelope", err)
	}

	// 8. Submit to the local gateway. There is NO local-spool fallback: an
	//    unreachable gateway is could-not-submit (exit 6, fail closed), never a
	//    silent on-disk queue reported as delivered.
	receipt, err := d.gateway.Submit(signed)
	if err != nil {
		if errors.Is(err, ErrGatewayUnreachable) {
			oc.detail = "gateway-unreachable"
			return oc, deskkit.Unverifiable("could-not-submit: "+err.Error()+" (no local fallback — fail closed)", err)
		}
		oc.detail = "gateway-refused"
		return oc, deskkit.Refused("refused: " + err.Error())
	}
	if !receipt.Accepted {
		oc.detail = "gateway-refused"
		return oc, deskkit.Refused("refused: gateway did not accept the message: " + receipt.Detail)
	}

	oc.detail = fmt.Sprintf("sent %s --%s--> %s/%s (id %s)", d.self, verb, toCell, a.to, receipt.ID)
	fmt.Fprintf(d.stdout, "%s\n", oc.detail)
	return oc, nil
}
