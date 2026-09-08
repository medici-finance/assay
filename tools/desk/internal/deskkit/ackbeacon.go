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
// other key byte-for-byte as it found it. deskroster carries the acks field as an opaque
// RawMessage for the same reason: to round-trip it through ITS writes untouched.

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	base, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "roster", session+".json"), nil
}

// AppendAck appends rec to session's roster beacon, creating the file (and the roster
// directory) if absent, and PRESERVING every field the beacon already carries. It
// returns the path written so the caller can name it. rec.TS is stamped here when empty,
// so a caller cannot forget it.
func AppendAck(session string, rec AckRecord) (path string, err error) {
	if rec.TS == "" {
		rec.TS = time.Now().UTC().Format(time.RFC3339)
	}
	path, err = AckBeaconPath(session)
	if err != nil {
		return "", err
	}
	// Load the existing object as raw keys so no field this binary does not know about is
	// dropped on write. A missing file is an empty object; a malformed one is an error
	// (fail closed — never silently blank another writer's beacon).
	obj := map[string]json.RawMessage{}
	if data, rerr := os.ReadFile(path); rerr == nil {
		if len(data) > 0 {
			if uerr := json.Unmarshal(data, &obj); uerr != nil {
				return path, Unverifiable("cannot parse the roster beacon at "+path+
					" — refusing to overwrite it and lose another writer's fields", uerr)
			}
		}
	} else if !os.IsNotExist(rerr) {
		return path, Unverifiable("cannot read the roster beacon at "+path, rerr)
	}

	// Decode the existing acks array (absent → empty), append, re-encode.
	var acks []AckRecord
	if raw, ok := obj["acks"]; ok && len(raw) > 0 {
		if uerr := json.Unmarshal(raw, &acks); uerr != nil {
			return path, Unverifiable("cannot parse the acks array in the roster beacon at "+path, uerr)
		}
	}
	acks = append(acks, rec)
	acksRaw, merr := json.Marshal(acks)
	if merr != nil {
		return path, Unverifiable("cannot encode the acks array", merr)
	}
	obj["acks"] = acksRaw
	// Keep the session field present so a beacon deskack creates is well-formed for
	// deskroster's reader (which keys the file by its session name anyway).
	if _, ok := obj["session"]; !ok {
		sraw, _ := json.Marshal(session)
		obj["session"] = sraw
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return path, Unverifiable("cannot create the roster directory "+dir, err)
	}
	out, merr := json.MarshalIndent(obj, "", "  ")
	if merr != nil {
		return path, Unverifiable("cannot encode the roster beacon", merr)
	}
	if werr := os.WriteFile(path, append(out, '\n'), 0o600); werr != nil {
		return path, Unverifiable("cannot write the roster beacon at "+path, werr)
	}
	return path, nil
}
