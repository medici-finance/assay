package commsqueue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// refusal.go — the gateway's REFUSAL journal line (#1165). A refused inbound
// used to leave no durable trace at all: the refusal was asserted on the
// client receipt only, so the daily lane-violation sweep (cmd/commsloop
// sweep.go) could not see refused attempts, and a sustained probe against
// the gateway was invisible out of band. Every refusal now appends ONE line
// of this shape to the SAME journal.log the sweep already reads.
//
// The shape lives HERE, in the package both binaries share, for the same
// reason the queue shape does (package doc): cmd/commsgw writes it and
// cmd/commsloop reads it, and they are separate processes that cannot
// import one another, so this is the one place the writer and the reader are
// held to a single definition.
//
// NEVER THE PAYLOAD. A journal line carries the refusal kind, the lane pair
// and the sender identity AS PRESENTED (the unverified from/to the raw bytes
// claimed — a refused message has, by definition, not passed verification,
// so nothing here is trusted identity), a timestamp, and a digest of the raw
// bytes for correlation with a receipt. It carries no payload bytes and no
// free-text error detail: the house ruling that the journal records a
// digest only, never raw untrusted text, binds a refusal exactly as it binds
// a prose consult. RefusalRecord has no payload field by construction, and
// TestRefusalRecordNeverCarriesPayload pins that.

// JournalKindRefused is the `kind` a refusal journal line carries. It is the
// discriminator sweep.go's structured-line reader switches on; a line with
// this kind is a refusal record, never a landed/quarantined/spawn one.
const JournalKindRefused = "refused"

// RefusalKind is the closed vocabulary of gateway refusal kinds. Each maps
// one-to-one onto a DISTINCT typed refusal in cmd/commsgw/precheck.go (and
// the comms identity errors it wraps), so the sweep and an incident review
// can count each failure mode separately — never a collapsed "rejected".
type RefusalKind string

const (
	// RefusalPeerUnauthenticated — the transport presented no verified peer.
	RefusalPeerUnauthenticated RefusalKind = "peer-unauthenticated"
	// RefusalCarrierMalformed — the transport carrier (socket request line,
	// A2A message parts) did not yield envelope bytes at all.
	RefusalCarrierMalformed RefusalKind = "carrier-malformed"
	// RefusalEnvelopeParse — the bytes do not parse as a cellmsg-v1 envelope.
	RefusalEnvelopeParse RefusalKind = "envelope-parse"
	// RefusalUnknownCell — the asserting cell is not in the trust store.
	RefusalUnknownCell RefusalKind = "unknown-cell"
	// RefusalBadSignature — the assertion signature does not verify.
	RefusalBadSignature RefusalKind = "bad-signature"
	// RefusalExpired — the assertion is past its validity window.
	RefusalExpired RefusalKind = "expired"
	// RefusalNotYetValid — the assertion claims issuance beyond the skew.
	RefusalNotYetValid RefusalKind = "not-yet-valid"
	// RefusalReplay — the assertion nonce was already consumed.
	RefusalReplay RefusalKind = "replay"
	// RefusalIdentityMismatch — the assertion does not match the declared
	// sender.
	RefusalIdentityMismatch RefusalKind = "identity-mismatch"
	// RefusalAssertionInvalid — the assertion failed verification for a
	// reason none of the finer kinds above names (fail-closed catch-all for
	// that stage; never silently one of the others).
	RefusalAssertionInvalid RefusalKind = "assertion-invalid"
	// RefusalLaneDenied — within-cell (from, verb, to) is not in the ACL.
	RefusalLaneDenied RefusalKind = "lane-denied"
	// RefusalCrossCellPair — cross-cell pair is not the ruled desk pair.
	RefusalCrossCellPair RefusalKind = "cross-cell-pair"
	// RefusalCrossCellVerb — cross-cell verb is outside the allow-set.
	RefusalCrossCellVerb RefusalKind = "cross-cell-verb"
	// RefusalDuplicate — the message id is already claimed.
	RefusalDuplicate RefusalKind = "duplicate"
	// RefusalBudgetExhausted — the sender is over its rate/budget.
	RefusalBudgetExhausted RefusalKind = "budget-exhausted"
	// RefusalRateLimiterUnconfigured — the gateway could not check the budget
	// at all (fail closed).
	RefusalRateLimiterUnconfigured RefusalKind = "rate-limiter-unconfigured"
	// RefusalKillSwitch — the desk kill switch is armed.
	RefusalKillSwitch RefusalKind = "kill-switch"
	// RefusalOther — a refusal the gateway could not classify into any of
	// the kinds above. It is a member of the vocabulary, not an absence: an
	// unclassifiable refusal is still journalled (and counted by the sweep),
	// never dropped for want of a label.
	RefusalOther RefusalKind = "other"
)

// KnownRefusalKinds is the closed set the sweep validates a refusal line's
// kind against; a line naming a kind outside it is corrupt (could-not-check),
// never silently counted under some other label.
var KnownRefusalKinds = map[RefusalKind]bool{
	RefusalPeerUnauthenticated:     true,
	RefusalCarrierMalformed:        true,
	RefusalEnvelopeParse:           true,
	RefusalUnknownCell:             true,
	RefusalBadSignature:            true,
	RefusalExpired:                 true,
	RefusalNotYetValid:             true,
	RefusalReplay:                  true,
	RefusalIdentityMismatch:        true,
	RefusalAssertionInvalid:        true,
	RefusalLaneDenied:              true,
	RefusalCrossCellPair:           true,
	RefusalCrossCellVerb:           true,
	RefusalDuplicate:               true,
	RefusalBudgetExhausted:         true,
	RefusalRateLimiterUnconfigured: true,
	RefusalKillSwitch:              true,
	RefusalOther:                   true,
}

// RefusalRecord is ONE refusal journal line. Its JSON is deliberately a
// subset-compatible shape with the sweep's structured record (`time`, `kind`,
// `id`, `cell`, `from`, `to`, `verb`) plus the refusal-specific `refusal` and
// `rawDigest` fields, so the sweep's single structured-line reader parses it
// with no second parser.
//
// There is NO payload field, NO error-detail field and NO assertion field on
// this struct — see the file doc. Adding one is a ruling change, not an
// enhancement.
type RefusalRecord struct {
	Time time.Time `json:"time"`
	// Kind is always JournalKindRefused.
	Kind string `json:"kind"`
	// ID is the message id AS PRESENTED by the raw bytes when one could be
	// read, else a synthetic `refused-<digest16>` so every line carries a
	// non-empty id (the sweep's reader treats an id-less structured line as
	// corrupt).
	ID string `json:"id"`
	// Cell is the GATEWAY's own cell — the cell whose sweep this refusal is
	// in scope for even when the presented from/to are empty or forged.
	Cell string `json:"cell,omitempty"`
	// Refusal is the distinct refusal kind.
	Refusal RefusalKind `json:"refusal"`
	// From / To / Verb are the sender identity and lane pair AS PRESENTED —
	// unverified, possibly forged, empty when the bytes did not parse far
	// enough to read them. They are the attribution the sweep counts by.
	From comms.SenderID `json:"from,omitempty"`
	To   comms.Lane     `json:"to,omitempty"`
	Verb string         `json:"verb,omitempty"`
	// RawDigest is sha256-hex of the raw inbound bytes: enough to correlate a
	// line with a receipt or a capture, never enough to reconstruct content.
	RawDigest string `json:"rawDigest,omitempty"`
}

// SenderKey is the per-sender attribution key the sweep counts refusals by:
// `<cell>/<role>` as presented, or "(unattributed)" when neither could be
// read from the bytes — an unattributable flood still counts as a flood.
func (r RefusalRecord) SenderKey() string {
	if r.From.Cell == "" && r.From.Role == "" {
		return "(unattributed)"
	}
	return r.From.Cell + "/" + r.From.Role
}

// LaneKey is the per-lane attribution key: the DESTINATION lane
// `<to cell>/<to role>` as presented (a Lane IS a destination inbox —
// comms.Lane's doc), or "(unattributed)" when none was presented. It is the
// second axis on purpose: several senders each under their own threshold
// but converging on ONE inbox is a probe of that inbox, and only a
// destination-keyed count sees it. The from/to PAIR is recorded on every
// line; the key just chooses which end to count by.
func (r RefusalRecord) LaneKey() string {
	if r.To.Cell == "" && r.To.Role == "" {
		return "(unattributed)"
	}
	return r.To.Cell + "/" + r.To.Role
}

// AppendRefusal appends rec as one line to <root>/journal.log — the SAME
// append-only journal Land writes and the sweep reads. It refuses to write a
// record whose Kind is not JournalKindRefused or whose Refusal is not in the
// closed vocabulary, so a caller can never plant a line the sweep would
// have to read as corrupt.
func AppendRefusal(root string, rec RefusalRecord) error {
	if rec.Kind != JournalKindRefused {
		return fmt.Errorf("commsqueue: refusal record kind %q is not %q", rec.Kind, JournalKindRefused)
	}
	if !KnownRefusalKinds[rec.Refusal] {
		return fmt.Errorf("commsqueue: refusal kind %q is not in the closed vocabulary", rec.Refusal)
	}
	if rec.ID == "" {
		return fmt.Errorf("commsqueue: refusal record has an empty id")
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("commsqueue: cannot marshal refusal record %s: %w", rec.ID, err)
	}
	return AppendJournal(root, string(raw))
}
