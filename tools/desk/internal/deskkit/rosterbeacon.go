package deskkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BeaconAction is the disposition of a locked roster-beacon transaction.
type BeaconAction uint8

const (
	BeaconKeep   BeaconAction = iota // leave the existing file (or its absence) unchanged
	BeaconWrite                      // atomically publish the callback's object
	BeaconDelete                     // remove the beacon, retaining its stable lock file
)

// Burst startup can queue several writers behind synced replacement of a large
// beacon. Allow that queue to drain while still bounding a stuck writer's impact.
const rosterBeaconLockWait = 10 * time.Second

// Keep both the beacon and its lock a single portable filename. In particular,
// Windows alternate data streams and device names must not alias another file.
func validRosterSession(session string) bool {
	if !ValidSessionSegment(session) || strings.ContainsAny(session, `<>:"|?*`) || strings.HasSuffix(session, ".") || strings.HasSuffix(session, " ") {
		return false
	}
	for _, r := range session {
		if r < 32 {
			return false
		}
	}
	base, _, _ := strings.Cut(strings.ToUpper(session), ".")
	base = strings.TrimRight(base, " ")
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return false
	}
	for _, prefix := range []string{"COM", "LPT"} {
		suffix := []rune(strings.TrimPrefix(base, prefix))
		if strings.HasPrefix(base, prefix) && len(suffix) == 1 && strings.ContainsRune("123456789¹²³", suffix[0]) {
			return false
		}
	}
	return true
}

// ReadRosterBeacon reads one complete snapshot without taking the writer lock.
// A missing file returns nil, nil; every existing file must contain a JSON object.
// Never use this snapshot for a later write: MutateRosterBeacon owns the entire
// read-modify-write transaction, including its read.
func ReadRosterBeacon(session string) (map[string]json.RawMessage, error) {
	path, err := AckBeaconPath(session)
	if err != nil {
		return nil, err
	}
	return readRosterBeacon(path)
}

func readRosterBeacon(path string) (map[string]json.RawMessage, error) {
	f, err := openRosterBeaconFile(path, false)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, Unverifiable("cannot read the roster beacon at "+path, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, Unverifiable("cannot read the roster beacon at "+path, err)
	}
	obj, err := decodeRosterBeacon(data)
	if err != nil {
		return nil, Unverifiable("cannot parse the roster beacon at "+path+
			" — refusing to overwrite it or treat corruption as missing state", err)
	}
	return obj, nil
}

func decodeRosterBeacon(data []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, errors.New("beacon must contain a JSON object")
	}
	obj := make(map[string]json.RawMessage)
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("beacon object key must be a string")
		}
		if _, exists := obj[key]; exists {
			return nil, fmt.Errorf("duplicate beacon field %q", key)
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		obj[key] = value
	}
	if _, err := dec.Token(); err != nil { // closing brace
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("beacon contains trailing data")
	}
	return obj, nil
}

// MutateRosterBeacon serializes every cooperating writer to one session's beacon.
// The callback runs after the current object has been read under a cross-process
// lock; a missing file supplies an empty object. Returning an error leaves the file
// untouched. Preserve keys the caller does not own. The callback must not recursively
// call a beacon writer for this session.
//
// The lock is a separate, stable file: locking the beacon itself would stop protecting
// it after atomic replacement. Never unlink the lock, including when deleting a beacon.
func MutateRosterBeacon(session string, fn func(map[string]json.RawMessage) (BeaconAction, error)) (path string, err error) {
	path, err = AckBeaconPath(session)
	if err != nil {
		return "", err
	}
	if fn == nil {
		return path, Unverifiable("roster beacon mutation requires a callback", nil)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return path, Unverifiable("cannot create the roster directory", err)
	}
	lock, err := openRosterBeaconFile(path+".lock", true)
	if err != nil {
		return path, Unverifiable("cannot open the roster beacon lock", err)
	}
	defer lock.Close()
	deadline := time.Now().Add(rosterBeaconLockWait)
	for {
		lockErr := TryLockExclusive(lock)
		if lockErr == nil {
			break
		}
		if !errors.Is(lockErr, ErrLockBusy) {
			return path, Unverifiable("cannot lock the roster beacon", lockErr)
		}
		if !time.Now().Before(deadline) {
			return path, Unverifiable("timed out waiting for the roster beacon lock; retry after the active writer finishes", lockErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer UnlockFile(lock)

	obj, err := readRosterBeacon(path)
	if err != nil {
		return path, err
	}
	if obj == nil {
		obj = make(map[string]json.RawMessage)
	}
	action, err := fn(obj)
	if err != nil {
		return path, err
	}
	switch action {
	case BeaconKeep:
		return path, nil
	case BeaconDelete:
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return path, Unverifiable("cannot remove the roster beacon", err)
		}
		return path, nil
	case BeaconWrite:
		out, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return path, Unverifiable("cannot encode the roster beacon", err)
		}
		if err := writeRosterBeacon(path, append(out, '\n')); err != nil {
			return path, Unverifiable("cannot atomically write the roster beacon", err)
		}
		return path, nil
	default:
		return path, Unverifiable("invalid roster beacon mutation action", fmt.Errorf("action %d", action))
	}
}

func writeRosterBeacon(path string, data []byte) error {
	// A same-directory temporary keeps replacement on the same filesystem. CreateTemp
	// creates it with 0600, so replacement also repairs overly broad old-file modes.
	f, err := os.CreateTemp(filepath.Dir(path), ".roster-beacon-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return replaceRosterBeacon(tmp, path)
}
