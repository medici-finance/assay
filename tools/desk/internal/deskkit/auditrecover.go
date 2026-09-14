package deskkit

// auditrecover.go — non-destructive recovery of a corrupt audit.jsonl.
//
// THE DEFECT THIS CLOSES. `audit.jsonl` is load-bearing state, not merely a record: the
// rate-limit counter, the circuit breaker, and the idempotency store (AlreadyDoneIn) all
// derive from it. A single malformed line — a partial append from `kill -9`, a disk-full
// write, a sync-tool rewrite — makes LoadEntries refuse (exit 6), which halts every desk
// tool. The sanctioned remedy printed in that refusal used to be "move the file aside to
// audit.jsonl.corrupt-<ts>". That is a FULL STATE RESET: moving the whole file aside
// returns every budget to full AND makes the idempotency store forget every prior write,
// so re-runs post duplicates. Corruption recovery is thus a rotation, and a rotation is a
// reset — the exact trap this file removes.
//
// THE FIX. Quarantine the malformed LINE, not the whole FILE. RecoverCorruptAudit rewrites
// audit.jsonl carrying forward every still-parseable entry and moving only the unparseable
// lines into a corrupt-<ts> sidecar. Because the counter and the idempotency store are pure
// functions of the surviving entries, both CARRY OVER across the recovery — a recovered log
// bills the same budget and remembers the same completed writes it did before the bad line
// landed. The only thing lost is the bad line itself, which no meter could read anyway.
//
// This is a DELIBERATE, EXPLICIT operation (the `deskaudit recover` verb), never an
// automatic repair on a read or write path: a read tool silently rewriting shared state is
// its own failure mode, and it would paper over corruption whose cause a human should see.
// The append-only invariant that governs normal operation (Log's O_APPEND) is not weakened —
// recovery is the one sanctioned rewrite, taken under the audit flock, exactly as the human
// `mv` was a sanctioned out-of-band act, only now it preserves the state the meters need.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AuditRecovery reports what RecoverCorruptAudit did.
type AuditRecovery struct {
	// Carried is the number of parseable entries preserved in the rewritten audit.jsonl —
	// the state (counter + idempotency) that survives the recovery.
	Carried int
	// Quarantined is the number of unparseable lines moved to the sidecar.
	Quarantined int
	// QuarantinePath is the corrupt-<ts> sidecar the bad lines were moved to, or "" when
	// there were none (the file was already clean and nothing was rewritten).
	QuarantinePath string
	// Rewrote reports whether audit.jsonl was actually rewritten. False when the file was
	// already clean (recovery is then a no-op that preserves the append-only file untouched)
	// or absent.
	Rewrote bool
}

// RecoverCorruptAudit performs the non-destructive recovery described above, under the audit
// flock so it cannot race a concurrent write. It:
//
//   - reads audit.jsonl line by line, classifying each non-blank line as a parseable Entry
//     (carried forward) or an unparseable line (quarantined);
//   - if there are no bad lines, leaves the file exactly as it was (append-only preserved)
//     and returns Rewrote=false;
//   - otherwise writes the bad lines to audit.jsonl.corrupt-<ts> (0600) and atomically
//     replaces audit.jsonl with only the carried lines (temp file + rename, 0600), so the
//     counter and idempotency state carry over while the corruption is quarantined.
//
// A missing audit.jsonl is not corruption: nothing to recover, returns a zero AuditRecovery.
func RecoverCorruptAudit() (AuditRecovery, error) {
	dir, err := deskDir()
	if err != nil {
		return AuditRecovery{}, Unverifiable("cannot resolve desk-tools dir (HOME missing?)", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return AuditRecovery{}, Unverifiable("cannot create desk-tools dir", err)
	}

	unlock, err := lockAudit(dir)
	if err != nil {
		return AuditRecovery{}, err
	}
	defer unlock()

	// EVERY SEGMENT, not just the live file. Since the ledger rotates daily
	// (audit.go § rotateIfNeeded), a malformed line can sit in a rotated segment, and a
	// recovery that only ever looked at audit.jsonl would leave it there permanently —
	// every LoadEntries still refusing, with the printed remedy already run.
	paths, err := segmentPaths()
	if err != nil {
		return AuditRecovery{}, err
	}

	// good[i]/bad[i] are the raw lines of paths[i], kept byte-for-byte (no
	// reserialisation → no schema drift).
	good := make([][]string, len(paths))
	bad := make([][]string, len(paths))
	carried, quarantined := 0, 0
	for i, path := range paths {
		f, oerr := os.Open(path)
		if oerr != nil {
			if os.IsNotExist(oerr) {
				continue // a segment that is not there is not corruption
			}
			return AuditRecovery{}, Unverifiable("cannot read audit file for recovery: "+filepath.Base(path), oerr)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			raw := sc.Text()
			if strings.TrimSpace(raw) == "" {
				continue // blank lines are not corruption; LoadEntries skips them too
			}
			var e Entry
			if json.Unmarshal([]byte(strings.TrimSpace(raw)), &e) != nil {
				bad[i] = append(bad[i], raw)
				continue
			}
			good[i] = append(good[i], raw)
		}
		if scErr := sc.Err(); scErr != nil {
			f.Close()
			return AuditRecovery{}, Unverifiable("error scanning audit file for recovery: "+filepath.Base(path), scErr)
		}
		f.Close()
		carried += len(good[i])
		quarantined += len(bad[i])
	}

	if quarantined == 0 {
		// Already clean: do NOT rewrite — every append-only file stays untouched.
		return AuditRecovery{Carried: carried, Quarantined: 0, Rewrote: false}, nil
	}

	// Move the bad lines into ONE corrupt-<ts> sidecar, then atomically replace only the
	// files that carried one.
	stamp := time.Now().UTC().Format("20060102T150405Z")
	quarantine := filepath.Join(dir, "audit.jsonl.corrupt-"+stamp)
	// A second recovery in the same second must not clobber the first sidecar.
	for i := 1; ; i++ {
		if _, statErr := os.Stat(quarantine); os.IsNotExist(statErr) {
			break
		}
		quarantine = filepath.Join(dir, fmt.Sprintf("audit.jsonl.corrupt-%s.%d", stamp, i))
	}
	var allBad []string
	for i := range paths {
		allBad = append(allBad, bad[i]...)
	}
	if err := os.WriteFile(quarantine, []byte(strings.Join(allBad, "\n")+"\n"), 0o600); err != nil {
		return AuditRecovery{}, Unverifiable("cannot write quarantine sidecar", err)
	}

	for i, path := range paths {
		if len(bad[i]) == 0 {
			continue // this segment was clean — leave it exactly as it is
		}
		tmp := filepath.Join(dir, fmt.Sprintf(".%s.recover-%s", filepath.Base(path), stamp))
		var body string
		if len(good[i]) > 0 {
			body = strings.Join(good[i], "\n") + "\n"
		}
		if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
			return AuditRecovery{}, Unverifiable("cannot write recovered audit file: "+filepath.Base(path), err)
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return AuditRecovery{}, Unverifiable("cannot replace audit file with recovered copy: "+filepath.Base(path), err)
		}
	}

	return AuditRecovery{
		Carried:        carried,
		Quarantined:    quarantined,
		QuarantinePath: quarantine,
		Rewrote:        true,
	}, nil
}

// tryLockAudit takes the same audit.lock as lockAudit with ONE non-blocking attempt, for
// the caller whose work is optional. Rotation is that caller: it is due at most once a day
// and must never make a writer wait, so a busy lock is simply "someone else is writing"
// and the next invocation checks again. ok=false is never permission to proceed without the
// lock — it is permission to do nothing.
func tryLockAudit(dir string) (unlock func(), ok bool) {
	lf, err := os.OpenFile(filepath.Join(dir, "audit.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false
	}
	if lerr := TryLockExclusive(lf); lerr != nil {
		lf.Close()
		return nil, false
	}
	return func() { _ = UnlockFile(lf); _ = lf.Close() }, true
}

// lockAudit takes the shared audit.lock exclusive advisory lock, the same lock the
// outward-write flow holds across LoadEntries → AllowWrite → append, so recovery cannot race
// a live write. It fails closed: a lock it cannot take is never permission to proceed.
func lockAudit(dir string) (unlock func(), err error) {
	lf, err := os.OpenFile(filepath.Join(dir, "audit.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, Unverifiable("cannot open audit lock", err)
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		lerr := TryLockExclusive(lf)
		if lerr == nil {
			return func() { _ = UnlockFile(lf); _ = lf.Close() }, nil
		}
		if lerr != ErrLockBusy {
			lf.Close()
			return nil, Unverifiable("cannot acquire audit lock", lerr)
		}
		if time.Now().After(deadline) {
			lf.Close()
			return nil, Unverifiable("audit lock held >60s (stale lock?) — another desk write may be stuck; retry, or a human clears it", nil)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
