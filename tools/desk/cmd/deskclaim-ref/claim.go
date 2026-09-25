package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// out / errOut are the tool's stdout and stderr sinks. They are package vars ONLY so a test
// can capture the exact lines this tool emits — the `show`/`list` output is a wire contract
// its Go readers (desksupervise, deskdispatch) parse, so it must be asserted byte-for-byte.
var (
	out    io.Writer = os.Stdout
	errOut io.Writer = os.Stderr
)

// Exit codes — the deskkit contract, named here so the port reads against the script it
// mirrors (dispatch-claim.sh: OK/REFUSED/UNVERIFIABLE).
const (
	exitOK           = deskkit.ExitOK           // 0
	exitRefused      = deskkit.ExitRefused      // 5
	exitUnverifiable = deskkit.ExitUnverifiable // 6
)

// refPrefix is the claim ref namespace on the forge — IDENTICAL to the bash script's
// REF_PREFIX. See main.go's SCOPE NOTE on why it is not deskkit.ClaimRefsPrefix.
const refPrefix = "refs/dispatch"

// logPrefix keeps the script's exact stdout/stderr line shape (`dispatch-claim: …`). It is
// load-bearing OUTPUT parity, not cosmetic: desksupervise/live.go and
// deskdispatch/dispatch.go parse this tool's `show` output (state=/age=/owner=/branch= and
// the "FREE <key>" marker), and a changed prefix or message body would break those readers.
const logPrefix = "dispatch-claim"

func logf(format string, a ...any) { fmt.Fprintf(out, logPrefix+": "+format+"\n", a...) }
func errf(format string, a ...any) { fmt.Fprintf(errOut, logPrefix+": "+format+"\n", a...) }

// claimedTTL / dispatchedTTL are the two-phase TTLs, env-overridable exactly as the script
// (CLAIMED_TTL_MIN / DISPATCHED_TTL_MIN). state=claimed but never advanced to dispatched is
// dead after the short TTL; state=dispatched after the long one.
func claimedTTL() int    { return envMin("CLAIMED_TTL_MIN", 20) }
func dispatchedTTL() int { return envMin("DISPATCHED_TTL_MIN", 120) }

func envMin(name string, def int) int {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

// --- the claim-store seam ---------------------------------------------------
//
// The storage surface this tool drives is deskkit.ClaimStore (internal/deskkit/claimstore.go):
// it was lifted there from this package, unchanged, so more than one backend can sit behind it
// and deskkit.ResolveClaimStore can decide which one a cell uses. The forge-ref store — an
// in-process git-smart-HTTP transport (gogit.go, over go-git), NOT a `gh` (or any) CLI — is one
// implementation of it. The store is a package var, mirroring the old `ghRun` seam this port
// replaced, so a test drives an in-memory store with no live remote and no external process.
// Removing the CLI closes the forge-surface violation the ban (internal/forgeban) exists to
// catch: this binary reaches the forge only through the enumerated git transport, never a
// shell-out. The verbs below are pure decision logic over the store's three-state results.

// causeSuffix renders the store's last transport cause as a ": <host>: <error>" suffix for a
// fail-closed message, or "" when there is nothing to attribute. It is the ONE place the
// attribution is composed so every "unverifiable:" line carries it uniformly.
func causeSuffix() string {
	if c := store.TransportCause(); c != "" {
		return ": " + c
	}
	return ""
}

// store is the resolved claim store. dispatchVerb installs what deskkit.ResolveClaimStore
// returns; tests swap it through buildStore.
var store deskkit.ClaimStore

// --- helpers ----------------------------------------------------------------

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// validID rejects keys git itself would reject as a ref component and requires the
// mandatory <repo>-- prefix. It mirrors the bash `valid_id` case patterns exactly.
func validID(id string) bool {
	if !strings.Contains(id, "--") {
		return false
	}
	if strings.ContainsAny(id, " ~^:?*[\\") {
		return false
	}
	if strings.Contains(id, "..") || strings.Contains(id, "@{") {
		return false
	}
	if strings.HasPrefix(id, ".") || strings.HasSuffix(id, ".lock") {
		return false
	}
	return true
}

// claimMessage builds the annotated-tag message body — the WIRE CONTRACT shared with the bash
// script (tools/dispatch-claim.sh) and its REST-minted tags. The grammar is exact: a bash
// `dispatch-claim.sh show` parses this out of a Go-minted tag, and this tool's reader parses it
// out of a bash/REST-minted one. Do not reorder or re-space the fields.
func claimMessage(id, owner, state, branch, note string) string {
	msg := "dispatch-claim " + id + " owner=" + owner + " state=" + state + " branch=" + dashIfEmpty(branch)
	if note != "" {
		msg += " note=" + sanitizeNote(note)
	}
	return msg
}

// fieldOf pulls `key=value` out of a space-separated claim message (bash `field_of`).
func fieldOf(msg, key string) string {
	for _, tok := range strings.Fields(msg) {
		if v, ok := strings.CutPrefix(tok, key+"="); ok {
			return v
		}
	}
	return ""
}

// ageMinutes parses the claim's stamped ISO date and returns whole minutes since, mirroring the
// bash `age_minutes`. ok=false when the date is unparseable — the caller treats that as
// unverifiable, never as age 0.
func ageMinutes(iso string) (int, bool) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(iso))
	if err != nil {
		return 0, false
	}
	return int(time.Since(t).Minutes()), true
}

// sanitizeNote mirrors the bash `tr ' \n' '_ ' | tr -d '\n'`: space -> '_', newline -> ' '.
func sanitizeNote(note string) string {
	var b strings.Builder
	for _, r := range note {
		switch r {
		case ' ':
			b.WriteByte('_')
		case '\n':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// reportHolder is the DEDUP LOG: a second dispatch attempt must say what it deduplicated
// against (which desk, when), never no-op silently (bash report_holder).
func reportHolder(id, msg, date string) {
	owner := fieldOf(msg, "owner")
	state := fieldOf(msg, "state")
	branch := fieldOf(msg, "branch")
	age, ok := ageMinutes(date)
	ageStr := "?"
	if ok {
		ageStr = strconv.Itoa(age)
	}
	logf("DEDUP %s — already claimed by owner=%s state=%s branch=%s at %s (%sm ago); not dispatching",
		id, dashOrValue(owner), dashOrValue(state), dashIfEmpty(branch), date, ageStr)
}

func dashOrValue(s string) string {
	if s == "" {
		return "?"
	}
	return s
}

// --- verbs ------------------------------------------------------------------

func cmdAcquire(id, owner, branch string) int {
	switch store.CreateIfAbsent(id, claimMessage(id, owner, "claimed", branch, "")) {
	case deskkit.ClaimWriteApplied:
		logf("acquired %s (owner=%s state=claimed) — %s/%s", id, owner, refPrefix, id)
		return exitOK
	case deskkit.ClaimWriteUnverifiable:
		errf("unverifiable: could not create the claim %s/%s%s", refPrefix, id, causeSuffix())
		return exitUnverifiable
	}
	// The create was REJECTED: the ref already exists. Exactly one benign cause — someone else
	// holds it. Read the holder; anything unreadable is unverifiable and fails closed.
	ref, status := store.Read(id)
	switch status {
	case deskkit.ClaimReadFree:
		errf("unverifiable: creating %s/%s was rejected but no claim exists%s", refPrefix, id, causeSuffix())
		return exitUnverifiable
	case deskkit.ClaimReadUnverifiable:
		errf("unverifiable: creating %s/%s was rejected and the claim could not be read%s", refPrefix, id, causeSuffix())
		return exitUnverifiable
	}
	state := fieldOf(ref.Msg, "state")
	hbranch := fieldOf(ref.Msg, "branch")
	hage, aok := ageMinutes(ref.Date)
	if !aok {
		errf("unverifiable: holder of %s has an unreadable claim date (%s)", id, ref.Date)
		return exitUnverifiable
	}
	if exists, verifiable := store.BranchExists(hbranch); verifiable && exists {
		reportHolder(id, ref.Msg, ref.Date)
		logf("  branch-as-claim: %s exists on the remote — the work is in flight, not stalled", hbranch)
		return exitRefused
	}
	ttl := dispatchedTTL()
	if state == "claimed" {
		ttl = claimedTTL()
	}
	if hage >= ttl {
		logf("stale claim on %s: state=%s age=%dm >= %dm TTL — reclaiming", id, dashOrValue(state), hage, ttl)
		return cmdSteal(id, owner, fmt.Sprintf("TTL: state=%s age=%dm >= %dm", dashOrValue(state), hage, ttl))
	}
	reportHolder(id, ref.Msg, ref.Date)
	logf("  live (age %dm < %dm TTL for state=%s)", hage, ttl, dashOrValue(state))
	return exitRefused
}

func cmdProgress(id, owner, branch string) int {
	ref, status := store.Read(id)
	switch status {
	case deskkit.ClaimReadFree:
		errf("refused: %s has no claim to advance (acquire first)", id)
		return exitRefused
	case deskkit.ClaimReadUnverifiable:
		errf("unverifiable: could not read the claim on %s%s", id, causeSuffix())
		return exitUnverifiable
	}
	if holder := fieldOf(ref.Msg, "owner"); holder != "" && holder != owner {
		errf("refused: %s is held by %s, not %s — only the holder advances its own claim", id, holder, owner)
		return exitRefused
	}
	switch store.UpdateFrom(id, ref.Version, claimMessage(id, owner, "dispatched", branch, "")) {
	case deskkit.ClaimWriteApplied:
		logf("progressed %s (state=dispatched branch=%s owner=%s) — TTL now %dm", id, dashIfEmpty(branch), owner, dispatchedTTL())
		return exitOK
	case deskkit.ClaimWriteRejected:
		// BEHAVIOUR CHANGE (documented): the old port advanced with `PATCH force=true`, which
		// resurrected a claim that had been stolen out from under this session between the read
		// and the write. The compare-and-swap from the exact value read closes that race — a
		// claim no longer held by this session is REFUSED, not clobbered back into existence.
		errf("refused: %s was advanced or stolen by another desk since it was read — not the holder any more", id)
		return exitRefused
	default:
		errf("unverifiable: could not advance %s/%s%s", refPrefix, id, causeSuffix())
		return exitUnverifiable
	}
}

func cmdRelease(id string) int {
	outcome, existed := store.Remove(id)
	switch outcome {
	case deskkit.ClaimWriteApplied:
		if existed {
			logf("released %s", id)
		} else {
			logf("released %s (no claim — no-op)", id)
		}
		return exitOK
	default:
		errf("unverifiable: could not delete %s/%s%s", refPrefix, id, causeSuffix())
		return exitUnverifiable
	}
}

func cmdSteal(id, owner, reason string) int {
	if reason == "" {
		errf("refused: steal requires --reason (a takeover with no recorded reason is a hand-delete)")
		return exitRefused
	}
	msg := claimMessage(id, owner, "claimed", "", reason)
	ref, status := store.Read(id)
	switch status {
	case deskkit.ClaimReadUnverifiable:
		errf("unverifiable: could not read %s before stealing it%s", id, causeSuffix())
		return exitUnverifiable
	case deskkit.ClaimReadFree:
		// Nothing holds it — a steal collapses to a create. A racing create in the gap is the
		// CAS losing, reported as "re-claimed during the steal".
		switch store.CreateIfAbsent(id, msg) {
		case deskkit.ClaimWriteApplied:
			logf("stole %s (owner=%s reason=%s)", id, owner, reason)
			return exitOK
		case deskkit.ClaimWriteRejected:
			errf("refused: %s was re-claimed by another desk during the steal", id)
			return exitRefused
		default:
			errf("unverifiable: could not mint the replacement claim for %s%s", id, causeSuffix())
			return exitUnverifiable
		}
	}
	// Held: replace the current tag with an explicit-old CAS update. A stale old means another
	// desk moved it first — the steal loses cleanly rather than clobbering (closing the
	// old DELETE-then-POST race window).
	switch store.UpdateFrom(id, ref.Version, msg) {
	case deskkit.ClaimWriteApplied:
		logf("stole %s (owner=%s reason=%s)", id, owner, reason)
		return exitOK
	case deskkit.ClaimWriteRejected:
		errf("refused: %s was re-claimed by another desk during the steal", id)
		return exitRefused
	default:
		errf("unverifiable: could not mint the replacement claim for %s", id)
		return exitUnverifiable
	}
}

func cmdShow(id string) int {
	ref, status := store.Read(id)
	switch status {
	case deskkit.ClaimReadFree:
		logf("FREE %s (no %s/%s in the repo)", id, refPrefix, id)
		return exitOK
	case deskkit.ClaimReadUnverifiable:
		errf("unverifiable: could not read the claim on %s%s", id, causeSuffix())
		return exitUnverifiable
	}
	ageStr := "?"
	if age, ok := ageMinutes(ref.Date); ok {
		ageStr = strconv.Itoa(age)
	}
	logf("HELD %s — %s at=%s age=%sm", id, ref.Msg, ref.Date, ageStr)
	return exitOK
}

func cmdList() int {
	ids, status := store.List()
	switch status {
	case deskkit.ClaimReadUnverifiable:
		errf("unverifiable: could not list dispatch claims%s", causeSuffix())
		return exitUnverifiable
	}
	if len(ids) == 0 {
		logf("(no dispatch claims in the repo)")
		return exitOK
	}
	rc := exitOK
	for _, id := range ids {
		if c := cmdShow(id); c != exitOK {
			rc = c
		}
	}
	return rc
}
