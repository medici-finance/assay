//go:build pirunner

package pirunner

import (
	"fmt"
	"sync"
)

// Ledger is the fixture-owned synthetic effect ledger and current-authority
// source. It lives in the Go test process, outside Pi, so a Pi replay can
// never vouch for its own authority: every consequential tool execution asks
// it again, including a replay that skips the beforeTool hook.
type Ledger struct {
	mu      sync.Mutex
	gen     map[string]uint64 // current owner generation per invocation key
	paused  map[string]bool
	budget  map[string]int // remaining effect budget; absent means unlimited
	applied map[string]int // logical effect id -> applications
	lost    map[string]bool

	UpperChecks int // beforeTool authorizations asked
	ExecChecks  int // execution-time rechecks asked
	Refusals    []string
	Attempts    int // effect requests received, duplicates included
	Duplicates  int
	// LostReply is called (asynchronously) after an effect whose reply is
	// dropped has been applied; the test kills the bridge there.
	LostReply func(key string)
}

// NewLedger returns an empty ledger.
func NewLedger() *Ledger {
	return &Ledger{gen: map[string]uint64{}, paused: map[string]bool{}, budget: map[string]int{}, applied: map[string]int{}, lost: map[string]bool{}}
}

// Grant records gen as the current owner generation for key unless a newer
// one is already current.
func (l *Ledger) Grant(key string, gen uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.gen[key] < gen {
		l.gen[key] = gen
	}
}

// Supersede makes gen current, so the running generation is stale.
func (l *Ledger) Supersede(key string, gen uint64) {
	l.mu.Lock()
	l.gen[key] = gen
	l.mu.Unlock()
}

// Pause and Resume toggle the invocation's pause.
func (l *Ledger) Pause(key string)  { l.mu.Lock(); l.paused[key] = true; l.mu.Unlock() }
func (l *Ledger) Resume(key string) { l.mu.Lock(); delete(l.paused, key); l.mu.Unlock() }

// SetBudget limits the effects key may still apply.
func (l *Ledger) SetBudget(key string, n int) { l.mu.Lock(); l.budget[key] = n; l.mu.Unlock() }

// LoseReplyFor drops the reply to the next application of this logical effect.
func (l *Ledger) LoseReplyFor(id string) { l.mu.Lock(); l.lost[id] = true; l.mu.Unlock() }

// Applied reports how many times a logical effect was applied.
func (l *Ledger) Applied(id string) int { l.mu.Lock(); defer l.mu.Unlock(); return l.applied[id] }

// EffectID is the stable logical identity of a synthetic effect: the
// invocation and the effect's own arguments, never a Pi task or call id that
// a replay could renumber.
func EffectID(key, action, target string) string { return key + "|" + action + "|" + target }

// authority answers whether the generation running key may act now. ranGen
// is the generation the run was launched under.
func (l *Ledger) authority(key string, ranGen uint64) string {
	cur, ok := l.gen[key]
	switch {
	case !ok:
		return "no-owner"
	case cur != ranGen:
		return "stale-generation"
	case l.paused[key]:
		return "paused"
	}
	if b, ok := l.budget[key]; ok && b <= 0 {
		return "budget-exhausted"
	}
	return "allowed"
}

func (l *Ledger) check(kind, key string, ranGen uint64) msg {
	l.mu.Lock()
	defer l.mu.Unlock()
	if kind == "authorize" {
		l.UpperChecks++
	} else {
		l.ExecChecks++
	}
	st := l.authority(key, ranGen)
	if st != "allowed" {
		l.Refusals = append(l.Refusals, fmt.Sprintf("%s:%s:%s", kind, key, st))
	}
	return msg{"status": st}
}

// effect applies one logical effect at most once under current authority.
func (l *Ledger) effect(key string, ranGen uint64, action, target string) msg {
	l.mu.Lock()
	l.Attempts++
	l.ExecChecks++
	if st := l.authority(key, ranGen); st != "allowed" {
		l.Refusals = append(l.Refusals, fmt.Sprintf("effect:%s:%s", key, st))
		l.mu.Unlock()
		return msg{"status": st}
	}
	id := EffectID(key, action, target)
	if l.applied[id] > 0 {
		l.Duplicates++
		l.mu.Unlock()
		return msg{"status": "duplicate", "effect": id}
	}
	l.applied[id]++
	if b, ok := l.budget[key]; ok {
		l.budget[key] = b - 1
	}
	lose := l.lost[id]
	delete(l.lost, id)
	cb := l.LostReply
	l.mu.Unlock()
	if lose {
		if cb != nil {
			go cb(key)
		}
		return nil // applied, but the reply never reaches the tool
	}
	return msg{"status": "applied", "effect": id}
}
