package deskkit

// ackbeacon.go — the receipt-line beacon append behind the `deskack` verb.
//
// WHAT THIS IS. When a desk acts on a human-typed message it first prints a receipt —
// `ack <role>@<repo-short>: <restatement>` — so the operator sees the message was read
// (and reads back the desk's ACTUAL interpretation, which is what lets a correction be
// paired with the receipt it corrects). The receipt is also RECORDED: one `{ts, role,
// repo, restatement}` entry appended to this session's roster beacon
// (<StateDir>/roster/<session>.json), which is the file deskroster already keeps a
// session's open-work in. the receipt-pairing metric reads these to pair a correction with the
// receipt it corrects; opmetrics can pair them against duplicate sends (a re-send with
// no receipt between == an unacknowledged message).
//
// WHY READ-MODIFY-WRITE PRESERVING UNKNOWN FIELDS. deskroster and deskack are separate
// binaries that BOTH own fields on the one beacon file (deskroster: session/role/
// updated/open_work; deskack: acks). If either rewrote the object from its own struct it
// would drop the other's fields on the floor — so the write here merges into the raw
// JSON object (a map of RawMessage) and touches only `acks` and `session`, leaving every
// other key's value intact. All beacon writers use MutateRosterBeacon so loading the
// object, changing owned fields, and publishing the replacement form one transaction.

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"time"
)

// AckRecord is one receipt: the timestamp, the role that acknowledged (the desk loop
// name from $DESK_LOOP), the short repo label the message concerned (empty when none was
// named), and the desk's own one-line restatement of the message.
type AckRecord struct {
	TS          string `json:"ts"`
	Role        string `json:"role"`
	Repo        string `json:"repo,omitempty"`
	Restatement string `json:"restatement"`
}

// AckBeaconPath is the beacon file a session's receipts append to. It is the SAME file
// deskroster stores a session's open-work in, under <StateDir>/roster/<session>.json.
func AckBeaconPath(session string) (string, error) {
	if !validRosterSession(session) {
		return "", Refused("roster session must be a safe filename segment")
	}
	base, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "roster", session+".json"), nil
}

// Line renders the fixed receipt line for a record — `ack <role>@<repo>: <restatement>`,
// or `ack <role>: <restatement>` when the record names no repo. It is the same shape
// deskack prints and records, so a correction composed from a beacon record reproduces the
// receipt the operator saw verbatim.
func (r AckRecord) Line() string {
	if r.Repo == "" {
		return "ack " + r.Role + ": " + r.Restatement
	}
	return "ack " + r.Role + "@" + r.Repo + ": " + r.Restatement
}

// LastAckWithin returns the most recent receipt on session's beacon whose timestamp is
// within window of now, or (nil, nil) when the beacon carries no such receipt — no beacon
// file, no acks array, or a newest receipt older than the window. A beacon that cannot be
// read or parsed, or whose newest receipt has an unparseable timestamp, is the
// could-not-check THIRD state and returns an error (fail closed): a correction must never be
// composed against a receipt the tool could not actually confirm, and a corrupt beacon must
// not read as "no receipt" (which would round a could-not-check down to a clean refusal).
//
// The last element of the acks array is the newest by construction — AppendAck only ever
// appends — so this reads it directly rather than re-sorting.
func LastAckWithin(session string, window time.Duration, now time.Time) (*AckRecord, error) {
	path, err := AckBeaconPath(session)
	if err != nil {
		return nil, err
	}
	obj, err := ReadRosterBeacon(session)
	if err != nil {
		return nil, err
	}
	raw, ok := obj["acks"]
	if !ok || len(raw) == 0 {
		return nil, nil
	}
	var acks []AckRecord
	if uerr := json.Unmarshal(raw, &acks); uerr != nil {
		return nil, Unverifiable("cannot parse the acks array in the roster beacon at "+path, uerr)
	}
	if len(acks) == 0 {
		return nil, nil
	}
	last := acks[len(acks)-1]
	ts, terr := time.Parse(time.RFC3339, last.TS)
	if terr != nil {
		return nil, Unverifiable("the newest receipt on the beacon at "+path+
			" has an unparseable ts "+strconv.Quote(last.TS), terr)
	}
	if now.Sub(ts) > window {
		return nil, nil // stale — a receipt exists but not within the window
	}
	return &last, nil
}

// AppendAck appends rec to session's roster beacon, creating the file (and the roster
// directory) if absent, and PRESERVING every field the beacon already carries. It
// returns the path written so the caller can name it. rec.TS is stamped here when empty,
// so a caller cannot forget it.
func AppendAck(session string, rec AckRecord) (path string, err error) {
	if rec.TS == "" {
		rec.TS = time.Now().UTC().Format(time.RFC3339)
	}
	return MutateRosterBeacon(session, func(obj map[string]json.RawMessage) (BeaconAction, error) {
		var acks []AckRecord
		if raw, ok := obj["acks"]; ok {
			if err := json.Unmarshal(raw, &acks); err != nil {
				return BeaconKeep, Unverifiable("cannot parse the acks array in the roster beacon", err)
			}
		}
		acks = append(acks, rec)
		acksRaw, err := json.Marshal(acks)
		if err != nil {
			return BeaconKeep, Unverifiable("cannot encode the acks array", err)
		}
		obj["acks"] = acksRaw
		if _, ok := obj["session"]; !ok {
			sraw, _ := json.Marshal(session)
			obj["session"] = sraw
		}
		return BeaconWrite, nil
	})
}
