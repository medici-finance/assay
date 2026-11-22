package main

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Store only content digests and delivery times; read current action text from
// the forge on every poll. Pending represents what may already have been shown,
// so stale offers can be withdrawn even when their acknowledgement was lost.
type deliveredAction struct {
	Digest string
	At     time.Time
}

type pendingChange struct {
	Kind   string
	Digest string
}

type changeState struct {
	Version   int
	Delivered map[string]deliveredAction
	Pending   map[string]pendingChange
	Receipt   string
	LastAck   string
}

type changeEvent struct {
	ID     string        `json:"id"`
	Kind   string        `json:"kind"`
	Action *changeAction `json:"action,omitempty"`
}

type changeBatch struct {
	Receipt string        `json:"receipt,omitempty"`
	Events  []changeEvent `json:"events"`
}

func withChangesState(dir string, fn func(string, *changeState) error) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := deskkit.TryLockExclusive(lock); err != nil {
		return fmt.Errorf("cannot acquire inbox cursor lock: %w", err)
	}
	defer deskkit.UnlockFile(lock)
	path := filepath.Join(dir, "cursor.json")
	s := changeState{Version: 1, Delivered: make(map[string]deliveredAction)}
	raw, err := os.ReadFile(path)
	if err == nil {
		var saved changeState
		if err := json.Unmarshal(raw, &saved); err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if saved.Version != 1 || saved.Delivered == nil || (saved.Receipt != "") != (len(saved.Pending) > 0) {
			return fmt.Errorf("invalid inbox cursor %s", path)
		}
		s = saved
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return fn(path, &s)
}

func saveChanges(path string, state *changeState) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cursor-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = json.NewEncoder(tmp).Encode(state); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	// Flush the directory entry as well on filesystems that support it.
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func acknowledgeChanges(path string, state *changeState, receipt string, now time.Time) error {
	if receipt != "" && receipt == state.LastAck {
		return nil
	}
	if receipt == "" || receipt != state.Receipt {
		return deskkit.Refused("unknown or superseded delivery receipt; read changes again")
	}
	for id, change := range state.Pending {
		if change.Kind == "withdrawn" {
			delete(state.Delivered, id)
		} else {
			state.Delivered[id] = deliveredAction{Digest: change.Digest, At: now.UTC()}
		}
	}
	state.LastAck, state.Receipt, state.Pending = receipt, "", nil
	return saveChanges(path, state)
}

// calculatePending compares against both the last successful delivery and the
// unacknowledged offer. A retry reproduces the same event. A superseding
// observation replaces it, including when an earlier change has since reverted.
func calculatePending(s *changeState, live map[string]changeAction, now time.Time, remindAfter time.Duration) map[string]pendingChange {
	next := make(map[string]pendingChange)
	for id, action := range live {
		// The title is context for a human, outside the delivery identity.
		action.Title = ""
		raw, _ := json.Marshal(action)
		sum := digest(string(raw))
		pending, offered := s.Pending[id]
		acked, delivered := s.Delivered[id]
		var kind string
		switch {
		case offered && sum == pending.Digest:
			kind = pending.Kind
		case offered:
			kind = "changed"
		case !delivered:
			kind = "ready"
		case sum != acked.Digest:
			kind = "changed"
		case remindAfter > 0 && now.Sub(acked.At) >= remindAfter:
			kind = "reminder"
		default:
			continue
		}
		next[id] = pendingChange{Kind: kind, Digest: sum}
	}
	for id := range s.Delivered {
		if _, ok := live[id]; !ok {
			next[id] = pendingChange{Kind: "withdrawn"}
		}
	}
	for id := range s.Pending {
		if _, ok := live[id]; !ok {
			next[id] = pendingChange{Kind: "withdrawn"}
		}
	}
	return next
}

func deliverChanges(out io.Writer, path string, s *changeState, live map[string]changeAction, now time.Time, remindAfter time.Duration) error {
	next := calculatePending(s, live, now, remindAfter)
	if !maps.Equal(next, s.Pending) {
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return err
		}
		s.Receipt, s.Pending = hex.EncodeToString(nonce[:]), next
		// Stage before output. A crash or broken pipe can re-offer this exact
		// receipt; only explicit acknowledgement advances the delivery baseline.
		if err := saveChanges(path, s); err != nil {
			return err
		}
	}
	result := changeBatch{Receipt: s.Receipt, Events: make([]changeEvent, 0, len(next))}
	// Withdraw stale instructions first, then present live acts in the inbox's
	// urgency/age order. A stable ID breaks ties across repositories.
	ids := slices.Collect(maps.Keys(next))
	slices.SortFunc(ids, func(x, y string) int {
		a, xLive := live[x]
		b, yLive := live[y]
		switch {
		case xLive && !yLive:
			return 1
		case !xLive && yLive:
			return -1
		default:
			return cmp.Or(cmp.Compare(a.rank, b.rank), cmp.Compare(a.createdAt, b.createdAt), cmp.Compare(x, y))
		}
	})
	for _, id := range ids {
		e := changeEvent{ID: id, Kind: next[id].Kind}
		if action, ok := live[id]; ok {
			e.Action = &action
		}
		result.Events = append(result.Events, e)
	}
	return json.NewEncoder(out).Encode(result)
}
