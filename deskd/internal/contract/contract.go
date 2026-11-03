// Package contract is the TYPED, VERSIONED wire between the deskd service and
// its clients — the client↔service seam ruled on #1819 (2026-08-30): typed
// HTTP/JSON with server-sent events (protocol option 1), NOT MCP and NOT ACP.
// MCP stays the AGENT surface (internal/mcp); ACP is reserved for a later
// chat-pane seam. This package is the single home of the request/response
// shapes, the SSE event envelope, the answer-record envelope, and the
// signed-intent envelope so the service and every client host
// speak one contract that a diff can break rather than a review
// argues.
//
// # Leaf, deliberately
//
// The package imports only the standard library and internal/domain (the pure
// domain leaf). It imports NONE of internal/answer, internal/custody, or
// internal/mcp — TestLeafImports asserts that on every build — so importing
// the contract can never drag one binary toward the other's identity. Both the
// service and the client import it; neither is dragged by it.
//
// # Custody rule (b), encoded here
//
// The service MAY act on the person's behalf, but ONLY for the ENUMERATED
// named actions in [NamedActions] ({answer, ack, send, session} — the set grew
// from two to four under the service contract, ruled option 1 on the decision
// issue, frozen again behind the same tests). Two rules make that a build
// property rather than a convention:
//
//   - [TestNamedActionSetIsClosed] asserts the set of human-authored write
//     routes declared in [Routes] equals the [NamedActions] literal. A new
//     human-write route is a deliberate contract change, never a config flag.
//   - The per-request write credential rides the HTTP Authorization header, it
//     is NEVER a body field. [TestNoCredentialShapedFields] walks every wire
//     type and fails on a field named or json-tagged like a credential, so the
//     contract cannot grow a place to persist one. The signed-intent path
//     ([SignedIntent]) carries a SIGNATURE over a record, never a raw token.
//
// Neither rule is the service's not-at-rest guarantee itself — that is a
// runtime persistence audit landed with the service in the v1 contract — but together
// they are the shape of the seam that makes the guarantee expressible.
package contract

import (
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// Version is the wire version. Every route lives under "/" + Version.
const Version = "v1"

// Prefix is the path prefix every route shares.
const Prefix = "/" + Version

// Method is an HTTP verb the contract uses. Reads are GET; every write is POST,
// with exactly ONE exception: DELETE /v1/sessions/{id}, which closes a custom
// terminal session (the service contract). [TestNamedActionSetIsClosed] pins that
// DELETE to that one route, so no other resource can grow a delete.
type Method string

const (
	GET    Method = "GET"
	POST   Method = "POST"
	DELETE Method = "DELETE"
)

// Action is a NAMED human action the service may perform on the person's
// behalf under custody rule (b). The set is CLOSED: see [NamedActions].
type Action string

const (
	// ActionAnswer commits a human decision — a comment carrying the answer
	// record plus a close (the answer flow semantics, unchanged).
	ActionAnswer Action = "answer"
	// ActionAck posts the 👀 read-receipt reaction as the person.
	ActionAck Action = "ack"
	// ActionSend types one line (or key sequence) into ONE desk's live
	// terminal as the person — optionally waiting, under the handle's send
	// lock, until the desk is at its prompt (the service contract). It is not a
	// GitHub write: its identity is the per-request VIEWER identity, never the
	// answerer write credential, and it can create no review, ratify no gate and
	// mint no role.
	ActionSend Action = "send"
	// ActionSession opens (POST /v1/sessions) or closes (DELETE
	// /v1/sessions/{id}) a CUSTOM terminal session on a configured desk's own
	// target (the service contract). Two routes carry this one action; the set of
	// ACTIONS is what is closed.
	ActionSession Action = "session"
)

// NamedActions is the ENUMERATED allow-list of human-authored writes the
// service exposes — the closed set of custody rule (b). There is no general
// comment/close/react route on any resource; adding an action here AND a route
// for it in [Routes] is the only way to grow the surface, and it is a contract
// change by construction. [TestNamedActionSetIsClosed] pins [Routes] to this
// literal AND pins the literal itself to exactly these four, so growing the
// set is a test edit a human gate reviews — never a one-line addition here.
var NamedActions = []Action{ActionAnswer, ActionAck, ActionSend, ActionSession}

// IsNamedAction reports whether a is in the closed allow-list.
func IsNamedAction(a Action) bool {
	for _, x := range NamedActions {
		if x == a {
			return true
		}
	}
	return false
}

// RouteKind classifies a route by what it does — read, event stream, local
// (service-side) state, an enumerated named action, or an idempotent nudge.
// Only [KindNamedAction] routes act as the person: answer/ack are GitHub
// writes authored as the person; send/session are the person's own input into a
// live desk terminal, carried on their viewer identity and touching no platform.
type RouteKind string

const (
	KindRead        RouteKind = "read"
	KindEvent       RouteKind = "event"
	KindLocalState  RouteKind = "local-state"  // service-local SQLite; no platform write
	KindNamedAction RouteKind = "named-action" // the ONLY human-authored platform writes
	KindNudge       RouteKind = "nudge"        // App-identity re-fetch; authors nothing
)

// Route is one registered endpoint of the v1 contract.
type Route struct {
	Method Method
	// Path is the full path including [Prefix]. "{id}" marks the id segment
	// (an escalation id, a cell name, a desk id or a session id — the route
	// family says which).
	Path string
	Kind RouteKind
	// Action names the named action a [KindNamedAction] route performs; empty
	// for every other kind.
	Action Action
	// Summary is a one-line description for humans and the OpenAPI generator.
	Summary string
}

// Routes is the complete v1 contract surface. Reads mirror the read surface and altitude panes
// read set (each has an MCP twin — a parity test lands with the service);
// events is the SSE stream; the two local-state writes are ruling D7; the two
// named actions are the closed custody-(b) allow-list; refresh is the nudge.
//
// NOT present in any version: a general human write on an arbitrary resource
// (comment/close/react). The allow-list is closed by construction — see the
// package doc.
var Routes = []Route{
	{GET, Prefix + "/cells", KindRead, "", "configured cells and their repos, with per-repo health"},
	{GET, Prefix + "/inbox", KindRead, "", "escalations for a cell, urgency-then-age, with index age and confidence"},
	{GET, Prefix + "/escalations/{id}", KindRead, "", "one escalation's full item view, with derived outcome"},
	{GET, Prefix + "/roadmap", KindRead, "", "stream roadmap (delegated to statusgen; could-not-check until wired)"},
	{GET, Prefix + "/portfolio", KindRead, "", "portfolio strip (altitude panes)"},
	{GET, Prefix + "/metrics", KindRead, "", "metrics query (read set)"},
	{GET, Prefix + "/desks/{id}/terminal", KindRead, "", "one desk's terminal handle: agent state, target kind, scrollback cursor (the service contract)"},
	// The desk registry read (the service contract): the rail's data source. It
	// joins the cell's configured loops, each loop's App identity, and the live
	// attach state from the terminal registry into one row per desk — present
	// even when the desk is unreachable (spec §5.2, the silent-cell failure
	// mode). Its MCP twin is desk__desks, shared with the terminal-handle route
	// above (parity is keyed by path, not by tool name).
	{GET, Prefix + "/desks", KindRead, "", "the desk registry for a cell (?cell=): one row per configured desk (master/desk/custom/peer), never dropped for being unreachable (the service contract)"},
	// The terminal stream (the service contract): the read half of the terminal
	// seam (spec §5.1). A KindEvent route — SSE, protocol option 1, never a
	// socket upgrade — so it has no MCP twin (parity walks KindRead only) and it is
	// NOT a named action: it takes no body and cannot write to the desk. The
	// write half (`send`) is the send route's contract change, not this route's.
	{GET, Prefix + "/desks/{id}/terminal/stream", KindEvent, "", "SSE: one desk's terminal — scrollback from a cursor, then live output (terminal.output, terminal.state, resync); Last-Event-ID = the byte cursor (the service contract)"},
	{GET, Prefix + "/events", KindEvent, "", "SSE: badge counts, escalation.transition, index.health; Last-Event-ID resume"},
	{POST, Prefix + "/escalations/{id}/read", KindLocalState, "", "mark read — service-local state (ruling D7), no platform write"},
	{POST, Prefix + "/escalations/{id}/answer", KindNamedAction, ActionAnswer, "answer a decision — comment + close carrying the answer record"},
	{POST, Prefix + "/escalations/{id}/ack", KindNamedAction, ActionAck, "acknowledge — the 👀 reaction, authored as the person"},
	{POST, Prefix + "/escalations/{id}/refresh", KindNudge, "", "nudge the service to re-fetch this issue now (App-identity read, idempotent)"},
	// The terminal named actions (the service contract): the WRITE half of the
	// terminal seam. Each is the person's own act on their per-request viewer
	// identity, scoped to the one desk the path names and allowed only for the
	// cell's answerer; neither touches GitHub. `session` is two routes (open,
	// close) carrying ONE action — the closed set is a set of actions.
	{POST, Prefix + "/desks/{id}/terminal/send", KindNamedAction, ActionSend, "type one line or key sequence into this desk's live terminal as the person, optionally waiting (atomically, under the handle's send lock) for the prompt first; a timeout writes nothing"},
	{POST, Prefix + "/sessions", KindNamedAction, ActionSession, "open a custom terminal session on a configured desk's own target (memory-only; a restart forgets it)"},
	{DELETE, Prefix + "/sessions/{id}", KindNamedAction, ActionSession, "close a custom terminal session — never a configured desk"},
	// Cell overrides (the service contract): the GET is a read with an MCP twin
	// (desk__overrides); the POST is service-local SQLite (ruling §10.5) — it
	// writes deskd's own table and files a record PR under a SEPARATE App, so it
	// is KindLocalState, NOT a KindNamedAction human-authored platform write, and
	// the closed named-action allow-list is untouched.
	{GET, Prefix + "/cells/{id}/overrides", KindRead, "", "read a cell's operator override table (dispatch_width, hold_merges, pause_verify, model_floor, quiet_hours) with as_of"},
	{POST, Prefix + "/cells/{id}/overrides", KindLocalState, "", "set a cell's operator overrides — service-local state (ruling §10.5), mirrored to cells.yaml by a record PR; no human-authored platform write"},
}

// HumanWriteActions returns the SET of named actions [Routes] declares as
// human-authored writes, each action once, in first-declared order.
// [TestNamedActionSetIsClosed] asserts it equals [NamedActions].
//
// It deduplicates because one action may be carried by more than one route
// (`session` is open + close): the closed set is a set of ACTIONS, and a
// per-route list would fail the length compare on the second `session` route —
// the failure the brief's pre-mortem names, whose wrong fix is dropping a
// route.
func HumanWriteActions() []Action {
	return humanWriteActions(Routes)
}

func humanWriteActions(routes []Route) []Action {
	seen := map[Action]bool{}
	var out []Action
	for _, r := range routes {
		if r.Kind == KindNamedAction && !seen[r.Action] {
			seen[r.Action] = true
			out = append(out, r.Action)
		}
	}
	return out
}

// Meta is the staleness envelope every read response carries (design §10:
// staleness is displayed, never hidden). ETag is transported in the HTTP
// header, so it is not a JSON field.
type Meta struct {
	AsOf            *time.Time `json:"as_of"`
	IndexAgeSeconds *int64     `json:"index_age_seconds"`
	ETag            string     `json:"-"`
}

// InboxResponse is the GET /v1/inbox body.
type InboxResponse struct {
	Meta
	Cell  string              `json:"cell"`
	Items []domain.Escalation `json:"items"`
	Total int                 `json:"total"`
}

// EscalationResponse is the GET /v1/escalations/{id} body. Outcome is the
// service's derivation of the answer record it parsed from GitHub on poll (nil
// while the escalation is unanswered).
type EscalationResponse struct {
	Meta
	Item    domain.Escalation `json:"item"`
	Outcome *Outcome          `json:"outcome"`
}

// Outcome is the answered state the service derives FROM GITHUB by parsing the
// answer-record envelope on poll — never from a side channel (§4: GitHub is
// the record).
type Outcome struct {
	AnsweredBy string          `json:"answered_by"`
	Option     domain.OptionID `json:"option"`
	OptionsRev string          `json:"options_rev"`
	// Stale is true when the answered record's options_rev differs from the
	// escalation's current rev (displayed as answered-stale-options, §10).
	Stale bool `json:"stale_options,omitempty"`
	// Unverified is true when the answer marker on GitHub could not be
	// verified as the named human's answer (the authorization contract gap 3, item
	// (5)): a bot-authored record whose signature, key, action, answer_as,
	// issue or time window fails, or a human-authored record naming someone
	// else. AnsweredBy is then the comment's real AUTHOR and Option is empty —
	// an anomaly to surface, never a decision any agent may act on.
	Unverified bool `json:"unverified,omitempty"`
}

// AnswerRequest is the body of POST /v1/escalations/{id}/answer. It carries
// what the human decided and what they last saw (for the answer-race check).
// It does NOT carry a credential: the per-request write credential rides the
// Authorization header. When Intent is set, the service acts on the verified
// signed intent INSTEAD of a header credential (custody (b) alternative).
type AnswerRequest struct {
	Option            domain.OptionID `json:"option,omitempty"`
	OptionsRev        string          `json:"options_rev"`
	Text              string          `json:"text,omitempty"`
	LastSeenUpdatedAt time.Time       `json:"last_seen_updated_at"`
	Intent            *SignedIntent   `json:"intent,omitempty"`
}

// AckRequest is the body of POST /v1/escalations/{id}/ack. Like AnswerRequest
// it carries no credential; Intent is the signed-intent alternative.
type AckRequest struct {
	Intent *SignedIntent `json:"intent,omitempty"`
}

// ActionResult is what a named action reports back.
type ActionResult struct {
	ID         string `json:"id"`
	Action     Action `json:"action"`
	AuthoredAs string `json:"authored_as"`
	// SignedBy is the human whose verified signed intent the action carried
	// (the authorization contract ruling 2): the write is authored as the gateway bot
	// (AuthoredAs) and the decision is the signer's. Empty on the per-request
	// credential path, where AuthoredAs is the human.
	SignedBy   string          `json:"signed_by,omitempty"`
	Option     domain.OptionID `json:"option,omitempty"`
	OptionsRev string          `json:"options_rev,omitempty"`
	Closed     bool            `json:"closed,omitempty"`
	At         time.Time       `json:"at"`
}

// TerminalHandle is GET /v1/desks/{id}/terminal: the read-only view of one
// desk's terminal, as the cockpit registry resolved it (the service contract).
//
// It is a READ shape — the service derives every field; a client SENDS
// nothing here but the desk id in the path. The security property this shape
// carries (SPOF layer (a)) is that there is NO field a client could use to
// name or select a target: the driver a desk id resolves to is chosen by the
// registry from the parsed cells.yaml `target:`, never by the caller (spec
// §5.1). [TerminalHandle.TargetKind] is a CATEGORY (`container`/`pod`/`local`,
// or "" when the desk has no attachable target), never a target address —
// knowing the kind lets a client render a label, never attach anywhere.
// [TestNoTargetShapedRequest] walks the wire types and fails on any field that
// could carry a target NAME/ADDRESS, exactly as [TestNoCredentialShapedFields]
// walks them for a credential.
//
// A desk with no attachable target (no `target:` in its cells.yaml loop) is a
// could-not-check ROW, never a 404 (spec §5.2, deskd rule 1): Connected is
// false, State is "could-not-check", and the row still renders — an
// unreachable desk is a could-not-check button, never a missing one.
type TerminalHandle struct {
	DeskID string `json:"desk_id"`
	// TargetKind is the driver CATEGORY the registry resolved for this desk
	// (`container`/`pod`/`local`), or "" when no `target:` is configured. It
	// is a category label, never an address a client could send — see the
	// type doc and [TestNoTargetShapedRequest].
	TargetKind string `json:"target_kind"`
	// AgentIdentity is the identity the desk skill advertised on the OSC
	// title, or "" when nothing is attached / no title has been seen.
	AgentIdentity string `json:"agent_identity"`
	// Title is the last OSC title parsed from the desk's byte stream.
	Title string `json:"title"`
	// Connected is true only while a driver session is attached and alive.
	Connected bool `json:"connected"`
	// Writable is true only while a live driver session is attached, i.e.
	// while a `send` (the service contract) could be delivered. It is a READ of
	// the handle's state and grants nothing: the send route re-checks identity,
	// tier, cell and epoch on every request. The rule is the same whoever
	// attached the session: deskd's attach lifecycle for a configured desk
	// (the service contract, which never sets or reads this field) or a `session`
	// open for a custom one.
	Writable bool `json:"writable"`
	// Epoch names the live driver session this handle is attached to; it
	// changes on every attach, detach and close, and carries a per-process
	// boot nonce so a deskd restart never re-issues an old value. A `send`
	// must carry the epoch it was composed against: a mismatch refuses the
	// send (not-sent), so queued input is never delivered to a replacement
	// session. "" while nothing is attached. This top-level epoch is the one
	// SendRequest.Epoch carries; it is not attach.epoch (AttachStatus.Epoch,
	// the per-process cursor-discard signal).
	Epoch string `json:"epoch"`
	// State is the derived agent state: live | busy | waiting | idle |
	// could-not-check. It is derived from TWO signals (OSC title + PTY
	// prompt/idle) and is could-not-check when they disagree — never the
	// optimistic reading (spec §5.1).
	State string `json:"state"`
	// Cursor is the latest scrollback cursor (a monotonic byte offset per
	// handle); a stream client resumes from it.
	Cursor uint64 `json:"cursor"`
	// AsOf is when the handle was last derived, nil when never attached.
	AsOf *time.Time `json:"as_of"`
	// Attach is deskd's own attach lifecycle for this desk (the service contract):
	// whether deskd holds a session, and when it does not, the closed-enum
	// reason why. Any status other than "attached" means State is
	// could-not-check.
	Attach AttachStatus `json:"attach"`
}

// AttachStatus is deskd's attach lifecycle for one desk (the service contract),
// carried on [TerminalHandle] and [DeskRow] and so, by shared type, on the MCP
// desk__desks twin. Every field is derived by the service. A client sends none
// of it, and nothing in it names a target.
//
// Status and Reason are CLOSED enums. The driver's error text, which names the
// container or pod, goes to deskd's log and never onto the wire.
//
//   - Status: attached | retrying | failed | off
//   - Reason: "" | no-target | no-driver | driver-init | unreachable |
//     unauthorized | unsupported | exited | disabled
//
// "failed" is terminal for this deskd process (a local desk that exited, a
// cluster desk with no access grant, an unsupported host, a missing driver).
// "retrying" carries NextAttemptAt. "off" means this deskd does not attach the
// row at all: the start-up switch is off (reason disabled), or it is a peer row
// (reason no-target).
type AttachStatus struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
	// Attempts is the number of attach attempts deskd has made for this desk
	// since it started.
	Attempts int `json:"attempts"`
	// NextAttemptAt is when the next attempt is due; nil unless Status is
	// "retrying".
	NextAttemptAt *time.Time `json:"next_attempt_at"`
	// Since is when Status/Reason last changed.
	Since time.Time `json:"since"`
	// Epoch is the deskd process start. It changes only when deskd restarts,
	// which is the one event that resets every desk's scrollback cursor to 0.
	// A client that sees attach.epoch change between two reads discards its
	// cursor before resuming. It is per PROCESS, not per session: a re-attach
	// inside one deskd process keeps both it and the cursor. It is not the
	// send binding: a `send` carries the top-level TerminalHandle.Epoch.
	Epoch time.Time `json:"epoch"`
	// Capability is what this deskd host can do for the desk's kind, decided
	// before any attempt is made (report F-2).
	Capability AttachCapability `json:"capability"`
}

// AttachCapability is the host-side answer to "can this deskd attach a desk of
// this kind at all?" (the service contract, report F-2). It is decided from the
// host platform and the desk's kind BEFORE any attempt, so an unsupported
// combination never retries, spawns or falls back to another shell or
// identity.
//
//   - Backend: "" | docker-exec | kube-exec | host-pty
//   - Profile: "" | trusted-host | cluster-rbac. "trusted-host" is hygiene on
//     a host the operator already trusts (deskd's own OS user, the local
//     driver's env deny-list). It is NOT isolation from a hostile worker.
//   - Support: supported | unsupported | could-not-check
//   - Reason: "" | no-target | no-pty-backend | no-cluster-grant | unknown-kind
type AttachCapability struct {
	HostOS   string `json:"host_os"`
	HostArch string `json:"host_arch"`
	Backend  string `json:"backend"`
	Profile  string `json:"profile"`
	Support  string `json:"support"`
	Reason   string `json:"reason"`
}

// DeskRow is one row of GET /v1/desks (the service contract): the join of a
// cell's configured loop (what exists), its App identity (who it files as,
// spec §5.2), and the terminal registry's live attach state (how it is
// doing). A row exists for EVERY configured desk even when its target is
// unreachable — the silent-cell failure mode (design §10) is a dead desk
// read as "not configured", which the row-count invariant in
// internal/terminal refuses by construction (SPOF layer a in the brief's
// Context: the row set is built from the loops FIRST, state is joined on).
//
// It is a READ shape exactly like [TerminalHandle]: nothing in it is a field
// a client could use to name or select a target (see [TestNoTargetShapedRequest]).
type DeskRow struct {
	// ID is "<cell>/<role>" for a configured desk, "<cell>/custom/<uuid>" for
	// a session, or "<peer-cell>/master" for a peer row.
	ID string `json:"id"`
	// Cell is the row's owning cell — the entered cell for master/desk/custom
	// rows, the OTHER cell for a peer row.
	Cell string `json:"cell"`
	// Role is the loop role ("review"/"verify"/"dispatch"/"metrics"), or
	// "master" for a peer row.
	Role string `json:"role"`
	// Group discriminates the rail's four sections: master | desk | custom |
	// peer. Exactly one configured loop per cell is "master" — the loop
	// [schema.Cell.MasterRoleOrDefault] names, a CONFIG KEY never a heuristic.
	Group string `json:"group"`
	// App is the loop's effective identity — [schema.Loop.WriteAppOrDefault],
	// never the desk-tools roster (facts: the service's role→App map is
	// cells.yaml `loops[].app`). Empty for a peer row: this deskd does not
	// know the peer's App.
	App string `json:"app"`
	// TargetKind is the driver category the terminal registry resolved
	// (container/pod/local), or "" when the desk has no attachable target.
	TargetKind string `json:"target_kind"`
	// State is the derived agent state: live | busy | waiting | idle |
	// could-not-check. A peer row is always could-not-check in this brief —
	// no cross-deskd read happens here (peer reachability makes peers reachable).
	State string `json:"state"`
	// Unread is the D7-style unread-marker count scoped to this desk. Always
	// 0 in this build: no producer of a desk-scoped unread count exists yet
	// (facts) — this is a real zero, not a could-not-check masquerading as one,
	// because the field itself is honestly unimplemented rather than a read
	// that failed.
	Unread int `json:"unread"`
	// AgentIdentity is the OSC-title identity the terminal registry parsed,
	// "" if nothing is attached or no title has been seen.
	AgentIdentity string `json:"agent_identity"`
	// LastTransition is when the row's state was last derived, nil if the
	// desk has never been attached.
	LastTransition *time.Time `json:"last_transition"`
	// Multiplexed reports whether more than one driver session backs this
	// desk id. Always false in this build: the terminal registry attaches at
	// most one session per handle (the service contract); a real multiplexed
	// target is a later brief's concern.
	Multiplexed bool `json:"multiplexed"`
	// Cursor is the latest scrollback cursor a stream client resumes from.
	Cursor uint64 `json:"cursor"`
	// URL is set only on a peer row: the peer deskd's base url, so a client
	// can go there directly — deskd never proxies (spec §7).
	URL string `json:"url,omitempty"`
	// Attach is deskd's attach lifecycle for the desk, the same value
	// [TerminalHandle.Attach] carries (the service contract). The rail reads
	// Attach.Reason for a greyed button's tooltip. A peer row reads
	// status "off", reason "no-target": this deskd never attaches a peer's
	// desk.
	Attach AttachStatus `json:"attach"`
}

// DesksResponse is GET /v1/desks: one cell's desk registry with the staleness
// envelope every v1 read carries.
type DesksResponse struct {
	Meta
	Cell  string    `json:"cell"`
	Desks []DeskRow `json:"desks"`
}

// WireTypes is the registry of exported request/response/envelope types the
// contract puts on the wire. [TestNoCredentialShapedFields] walks it, and a
// future OpenAPI generator (deferred to the contract-doc follow-up) reads it,
// so the two never drift from the shipped types.
func WireTypes() []any {
	return []any{
		InboxResponse{},
		EscalationResponse{},
		Outcome{},
		AnswerRequest{},
		AckRequest{},
		ActionResult{},
		TerminalHandle{},
		SignedIntent{},
		Event{},
		BadgeData{},
		TransitionData{},
		IndexHealthData{},
		// The terminal stream's two payloads (the service contract).
		TerminalOutputData{},
		TerminalStateData{},
		// Overrides is the cell-override wire type (the service contract). It lives
		// in internal/domain (the pure leaf this contract already imports) so the
		// one struct is shared by the HTTP write, the MCP read and the record PR
		// — a rename in one is a compile error in the others. Registering it here
		// puts it in the generated OpenAPI components.
		domain.Overrides{},
		OverridesResponse{},
		// DeskRow/DesksResponse are the desk registry read wire types
		// (the service contract). Registering them here puts them in the
		// generated OpenAPI components.
		DeskRow{},
		DesksResponse{},
		// The attach lifecycle status and host capability (the service contract).
		AttachStatus{},
		AttachCapability{},
		// The terminal named actions' wire types (the service contract).
		SendRequest{},
		SendResult{},
		SessionRequest{},
		SessionRow{},
	}
}

// SendWait names what a `send` waits for before it writes.
type SendWait string

const (
	// SendWaitPrompt waits until the desk's derived state is idle or waiting —
	// a settled prompt, including a desk asking a human.
	SendWaitPrompt SendWait = "prompt"
	// SendWaitIdle waits until no output has arrived for the idle window
	// (spike ANSWER 2's silence threshold).
	SendWaitIdle SendWait = "idle"
	// SendWaitNone writes as soon as the send lock is held.
	SendWaitNone SendWait = "none"
)

// SendOutcome is what the service knows happened to one send's bytes. The
// three states are kept apart on purpose: a lost acknowledgment after the
// write is outcome-unknown, never "not sent", so no client retries it blindly.
type SendOutcome string

const (
	// SendNotSent: nothing was written (refused, timed out, epoch changed,
	// authority revoked during the wait).
	SendNotSent SendOutcome = "not-sent"
	// SendSent: the bytes were handed to the live session in one write.
	SendSent SendOutcome = "sent"
	// SendOutcomeUnknown: the write was attempted and did not report success;
	// some or all bytes may have reached the desk. Never replayed.
	SendOutcomeUnknown SendOutcome = "outcome-unknown"
)

// SendRequest is the body of POST /v1/desks/{id}/terminal/send. It carries the
// input and the wait condition — never an identity (the viewer identity rides
// the request: cookie on loopback, the viewer bearer off-loopback) and never a
// target (the desk id in the path is resolved by the registry).
type SendRequest struct {
	// Input is written in ONE write once the wait condition holds.
	Input string `json:"input"`
	// WaitFor is prompt | idle | none.
	WaitFor SendWait `json:"wait_for"`
	// TimeoutMS bounds the wait (lock + condition). On expiry NOTHING is
	// written and the answer is 409 {error: "timeout", state}. 0 = the
	// service default.
	TimeoutMS int `json:"timeout_ms"`
	// Epoch is the handle epoch the input was composed against: the
	// top-level `epoch` string of GET /v1/desks/{id}/terminal, never
	// `attach.epoch`. Required: a send never binds to whatever session
	// happens to be attached when it arrives.
	Epoch string `json:"epoch"`
	// RequestID is the client's idempotency key. A retry carrying a request id
	// the handle already has a receipt for returns that receipt and writes
	// nothing; receipts are session-local and memory-only.
	RequestID string `json:"request_id,omitempty"`
}

// SendResult is what a `send` reports back.
type SendResult struct {
	DeskID      string      `json:"desk_id"`
	RequestID   string      `json:"request_id"`
	Epoch       string      `json:"epoch"`
	Outcome     SendOutcome `json:"outcome"`
	WaitedMS    int64       `json:"waited_ms"`
	CursorAfter uint64      `json:"cursor_after"`
	StateAfter  string      `json:"state_after"`
	// Duplicate is true when this is the recorded receipt of an earlier send
	// with the same request id — nothing was written by THIS request.
	Duplicate bool `json:"duplicate,omitempty"`
}

// SessionRequest is the body of POST /v1/sessions. On names a CONFIGURED desk
// id; the session opens on that desk's own target — the client never names a
// target (the service contract's single point of failure). Exactly one of Skill or
// Prompt may be set.
type SessionRequest struct {
	On     string `json:"on"`
	Skill  string `json:"skill,omitempty"`
	Prompt string `json:"prompt,omitempty"`
	Title  string `json:"title,omitempty"`
}

// SessionRow is one custom terminal session. It lives in memory only: a deskd
// restart forgets it (the session's process dies with the service).
type SessionRow struct {
	// ID is "<cell>/custom/<random>"; it is also the desk id the terminal read,
	// stream and send routes take.
	ID         string    `json:"id"`
	On         string    `json:"on"`
	TargetKind string    `json:"target_kind"`
	Title      string    `json:"title,omitempty"`
	OpenedAt   time.Time `json:"opened_at"`
	OpenedBy   string    `json:"opened_by"`
	// PromptOutcome reports the delivery of a `prompt` typed into the new
	// session (a send bound to its first epoch); empty when none was asked.
	PromptOutcome SendOutcome `json:"prompt_outcome,omitempty"`
}

// OverridesResponse is the GET /v1/cells/{id}/overrides body and the shape the
// POST echoes back: the cell's current override table plus its staleness
// envelope. It embeds [domain.Overrides] so each set knob appears at the top
// level (dispatch_width, hold_merges, …), the same shape the MCP desk__overrides
// read returns.
type OverridesResponse struct {
	Meta
	Cell string `json:"cell"`
	domain.Overrides
	// RecordPR reports the mirror-to-cells.yaml record PR's state: a filed PR's
	// ref, or an honest could-not-check when the write-issues App is not
	// configured (the filer never falls back to another identity).
	RecordPR string `json:"record_pr,omitempty"`
}
