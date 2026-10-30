package drainloop

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Receipt identifies a successful operation in the authoritative external store.
// A found receipt must have the requested Key and a nonempty Ref.
type Receipt struct {
	Key string
	Ref string
	At  time.Time
}

// Effect owns one external operation. Key must bind logical run, operation, target
// and subject, excluding attempt. Apply must pass that same key to any provider
// dedupe facility. Receipt must give an authoritative answer: an unreadable or
// unsupported lookup is an error, never found=false. False,nil means definitively
// not applied and safe to apply under the provider's enforced boundary. Providers
// with neither safe dedupe nor an authoritative absence proof must return error.
// Concurrent executors still require provider dedupe/fencing: this scheduler cannot
// turn a lookup followed by an external write into an atomic provider transaction.
type Effect interface {
	Key() string
	Receipt(context.Context, string) (Receipt, bool, error)
	Apply(context.Context) (Receipt, error)
}

// OperationKey builds unambiguous stable coordinates without attempt identity.
// Empty coordinates are rejected; its format is versioned for durable consumers.
func OperationKey(run, operation, target, subject string) (string, error) {
	parts := []string{run, operation, target, subject}
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return "", fmt.Errorf("effect coordinates must be nonempty")
		}
	}
	b, err := json.Marshal(parts)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("effect-v1:%x", sha256.Sum256(b)), nil
}

// EffectFailure marks a provider-proven failure with no external change. Any
// unclassified Apply error is unknown and holds; it is never automatically retried.
type EffectFailure struct{ Err error }

func (e EffectFailure) Error() string { return fmt.Sprintf("effect failed: %v", e.Err) }
func (e EffectFailure) Unwrap() error { return e.Err }

func (c Config) effectContext() context.Context {
	if c.Context != nil {
		return c.Context
	}
	return context.Background()
}

func effectResult(it Item, verdict Verdict, name string, err error) Result {
	return Result{Item: it, Verdict: verdict, Rows: []EvidenceRow{{Name: name, Output: err.Error()}}}
}

func validReceipt(r Receipt, key string) bool { return r.Key == key && strings.TrimSpace(r.Ref) != "" }

// reconcileBeforeApply always looks up the authoritative receipt, including fresh
// deliveries. This subsumes Retry and lost-journal-intent detection without assuming
// a write-only Journal can read history. No process-local cache authorizes a write.
func (c Config) reconcileBeforeApply(it Item, effects []Effect) Result {
	ctx := c.effectContext()
	if strings.TrimSpace(it.RunID) == "" || strings.TrimSpace(it.AttemptID) == "" {
		return effectResult(it, VerdictError, "effect identity", fmt.Errorf("RunID and AttemptID required"))
	}
	keys := map[string]bool{}
	operationKeys := make([]string, len(effects))
	for i, e := range effects {
		if e == nil {
			return effectResult(it, VerdictError, "effect identity", fmt.Errorf("nil effect"))
		}
		key := e.Key()
		if strings.TrimSpace(key) == "" || keys[key] {
			return effectResult(it, VerdictError, "effect identity", fmt.Errorf("effects require unique nonempty operation keys"))
		}
		keys[key] = true
		operationKeys[i] = key
	}
	var generation string
	pending := map[string]Intent{}
	handoff, transactional := c.Journal.(TransactionalJournal)
	if transactional {
		var err error
		generation, err = handoff.Generation(ctx, it.ID)
		if err != nil || strings.TrimSpace(generation) == "" {
			return effectResult(it, VerdictHold, "handoff could-not-check", fmt.Errorf("claim generation unavailable: %v", err))
		}
		entries, err := handoff.Pending(ctx, it.RunID)
		if err != nil {
			return effectResult(it, VerdictHold, "handoff could-not-check", err)
		}
		for _, entry := range entries {
			e := entry.Event
			if e.Kind != "EFFECT-INTENT" || e.RunID != it.RunID || e.ItemID != it.ID || e.Generation == "" || !keys[e.Key] {
				return effectResult(it, VerdictHold, "handoff could-not-check", fmt.Errorf("unmatched pending intent"))
			}
			if _, duplicate := pending[e.Key]; duplicate {
				return effectResult(it, VerdictHold, "handoff could-not-check", fmt.Errorf("duplicate pending intent"))
			}
			pending[e.Key] = entry
		}
	}
	res := Result{Item: it, Verdict: VerdictPass}
	for i, effect := range effects {
		key := operationKeys[i]
		event := Event{ItemID: it.ID, RunID: it.RunID, AttemptID: it.AttemptID, Key: key, Generation: generation, Time: time.Now()}
		r, found, err := effect.Receipt(ctx, key)
		if err != nil {
			return effectResult(it, VerdictHold, "reconcile could-not-check", err)
		}
		if found {
			if !validReceipt(r, key) {
				return effectResult(it, VerdictHold, "reconcile could-not-check", fmt.Errorf("invalid authoritative receipt"))
			}
			if old, ok := pending[key]; ok {
				event.Generation = old.Event.Generation
			}
			event.Kind, event.Ref = "RECONCILE", r.Ref
			c.recordEffect(event)
			c.log("RECONCILE %s %s", it.ID, r.Ref)
			event.Kind = "EFFECT-DONE"
			c.recordEffect(event)
			res.Rows = append(res.Rows, EvidenceRow{Name: "reconcile", Output: r.Ref})
			continue
		}
		if old, ok := pending[key]; ok && old.Event.Generation != generation {
			return effectResult(it, VerdictHold, "stale handoff", fmt.Errorf("pending intent belongs to a superseded claim generation"))
		}
		if err := ctx.Err(); err != nil {
			return effectResult(it, VerdictHold, "effect cancelled", err)
		}
		event.Kind = "EFFECT-INTENT"
		if transactional {
			err = handoff.CommitIntent(ctx, Intent{Event: event})
		} else {
			err = c.Journal.Record(event)
		}
		if err != nil {
			return effectResult(it, VerdictError, "effect intent journal", err)
		}
		r, err = effect.Apply(ctx)
		if err != nil {
			var failure EffectFailure
			var failurePtr *EffectFailure
			if errors.As(err, &failure) || errors.As(err, &failurePtr) {
				return effectResult(it, VerdictFail, "effect failed", err)
			}
			return effectResult(it, VerdictHold, "effect outcome unknown", err)
		}
		if !validReceipt(r, key) {
			return effectResult(it, VerdictHold, "effect outcome unknown", fmt.Errorf("Apply returned invalid receipt"))
		}
		event.Kind, event.Ref, event.Time = "EFFECT-DONE", r.Ref, time.Now()
		c.recordEffect(event)
		res.Rows = append(res.Rows, EvidenceRow{Name: "effect", Output: r.Ref})
	}
	return res
}

func (c Config) recordEffect(event Event) {
	if err := c.Journal.Record(event); err != nil {
		c.log("JOURNAL %s %s — sink error (non-fatal): %v", event.Kind, event.ItemID, err)
	}
}
