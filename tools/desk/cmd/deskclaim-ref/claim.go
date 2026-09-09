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
// Every forge access goes through the claimStore, an in-process git-smart-HTTP transport
// (gogit.go, over go-git) — NOT a `gh` (or any) CLI. It is a package var, mirroring the old
// `ghRun` seam this port replaced, so a test drives an in-memory forge with no live remote and
// no external process. Removing the CLI closes the forge-surface violation the ban
// (internal/forgeban) exists to catch: this binary now reaches the forge only through the
// enumerated git transport, never a shell-out.

// claimStatus is the three-state result of a claim read (bash read_claim's rc).
type claimStatus int

const (
	claimHeld         claimStatus = iota // a holder exists
	claimFree                            // no such ref
	claimUnverifiable                    // the read itself failed (fail-closed)
)

// writeOutcome is the three-state result of a claim WRITE. A rejection is the SERVER's
// compare-and-swap losing (the ref already exists on a create, or its value moved under an
// update/steal) — an expected race the caller acts on, never a could-not-check.
type writeOutcome int

const (
	writeApplied      writeOutcome = iota // the server applied the update
	writeRejected                         // the server refused: the CAS old no longer matches
	writeUnverifiable                     // transport/auth/net failure — fail closed
)

// claimRef is a held claim as read off the forge: the tag object's sha (used as the CAS `old`
// on the next write), its message body and its tagger date (RFC3339).
type claimRef struct {
	sha  string
	msg  string
	date string
}

// claimStore is the git-data surface deskclaim-ref drives. All mint/CAS/transport mechanics
// live behind it; the verbs below are pure decision logic over its results.
type claimStore interface {
	// read returns refs/dispatch/<id>'s payload and status.
	read(id string) (claimRef, claimStatus)
	// createIfAbsent mints a claim tag carrying msg (stamped now) and CAS-creates the ref from
	// ZERO. writeRejected == a holder already exists.
	createIfAbsent(id, msg string) writeOutcome
	// updateFrom mints a claim tag carrying msg and CAS-updates the ref from oldSHA.
	// writeRejected == the ref no longer holds oldSHA (advanced or stolen under this caller).
	updateFrom(id, oldSHA, msg string) writeOutcome
	// remove deletes refs/dispatch/<id> (reading the current value in the same session).
	// writeApplied == deleted OR already absent (a release is idempotent); existed reports
	// which, so release can log the script's exact "no claim — no-op" line.
	remove(id string) (outcome writeOutcome, existed bool)
	// list enumerates the present claim ids.
	list() ([]string, claimStatus)
	// branchExists reports heads/<branch> presence; verifiable=false is could-not-check.
	branchExists(branch string) (exists, verifiable bool)
	// transportCause reports "<host>: <error>" for the store's most recent transport failure,
	// or "" when the last operation did not fail at the transport layer. The verb layer appends
	// it to a fail-closed (exit 6) message so the operator sees WHERE the tool dialed and WHY it
	// failed, rather than a bare "unverifiable" that points at the wrong suspects (#727).
	transportCause() string
}

// causeSuffix renders the store's last transport cause as a ": <host>: <error>" suffix for a
// fail-closed message, or "" when there is nothing to attribute. It is the ONE place the
// attribution is composed so every "unverifiable:" line carries it uniformly.
func causeSuffix() string {
	if c := store.transportCause(); c != "" {
		return ": " + c
	}
	return ""
}

// store is the live forge seam. main() installs the go-git store (gogit.go); tests swap it.
var store claimStore

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
	switch store.createIfAbsent(id, claimMessage(id, owner, "claimed", branch, "")) {
	case writeApplied:
		logf("acquired %s (owner=%s state=claimed) — %s/%s", id, owner, refPrefix, id)
		return exitOK
	case writeUnverifiable:
		errf("unverifiable: could not create the claim %s/%s%s", refPrefix, id, causeSuffix())
		return exitUnverifiable
	}
	// The create was REJECTED: the ref already exists. Exactly one benign cause — someone else
	// holds it. Read the holder; anything unreadable is unverifiable and fails closed.
	ref, status := store.read(id)
	switch status {
	case claimFree:
		errf("unverifiable: creating %s/%s was rejected but no claim exists", refPrefix, id)
		return exitUnverifiable
	case claimUnverifiable:
		errf("unverifiable: creating %s/%s was rejected and the claim could not be read%s", refPrefix, id, causeSuffix())
		return exitUnverifiable
	}
	state := fieldOf(ref.msg, "state")
	hbranch := fieldOf(ref.msg, "branch")
	hage, aok := ageMinutes(ref.date)
	if !aok {
		errf("unverifiable: holder of %s has an unreadable claim date (%s)", id, ref.date)
		return exitUnverifiable
	}
	if exists, verifiable := store.branchExists(hbranch); verifiable && exists {
		reportHolder(id, ref.msg, ref.date)
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
	reportHolder(id, ref.msg, ref.date)
	logf("  live (age %dm < %dm TTL for state=%s)", hage, ttl, dashOrValue(state))
	return exitRefused
}

func cmdProgress(id, owner, branch string) int {
	ref, status := store.read(id)
	switch status {
	case claimFree:
		errf("refused: %s has no claim to advance (acquire first)", id)
		return exitRefused
	case claimUnverifiable:
		errf("unverifiable: could not read the claim on %s%s", id, causeSuffix())
		return exitUnverifiable
	}
	if holder := fieldOf(ref.msg, "owner"); holder != "" && holder != owner {
		errf("refused: %s is held by %s, not %s — only the holder advances its own claim", id, holder, owner)
		return exitRefused
	}
	switch store.updateFrom(id, ref.sha, claimMessage(id, owner, "dispatched", branch, "")) {
	case writeApplied:
		logf("progressed %s (state=dispatched branch=%s owner=%s) — TTL now %dm", id, dashIfEmpty(branch), owner, dispatchedTTL())
		return exitOK
	case writeRejected:
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
	outcome, existed := store.remove(id)
	switch outcome {
	case writeApplied:
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
	ref, status := store.read(id)
	switch status {
	case claimUnverifiable:
		errf("unverifiable: could not read %s before stealing it%s", id, causeSuffix())
		return exitUnverifiable
	case claimFree:
		// Nothing holds it — a steal collapses to a create. A racing create in the gap is the
		// CAS losing, reported as "re-claimed during the steal".
		switch store.createIfAbsent(id, msg) {
		case writeApplied:
			logf("stole %s (owner=%s reason=%s)", id, owner, reason)
			return exitOK
		case writeRejected:
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
	switch store.updateFrom(id, ref.sha, msg) {
	case writeApplied:
		logf("stole %s (owner=%s reason=%s)", id, owner, reason)
		return exitOK
	case writeRejected:
		errf("refused: %s was re-claimed by another desk during the steal", id)
		return exitRefused
	default:
		errf("unverifiable: could not mint the replacement claim for %s", id)
		return exitUnverifiable
	}
}

func cmdShow(id string) int {
	ref, status := store.read(id)
	switch status {
	case claimFree:
		logf("FREE %s (no %s/%s in the repo)", id, refPrefix, id)
		return exitOK
	case claimUnverifiable:
		errf("unverifiable: could not read the claim on %s%s", id, causeSuffix())
		return exitUnverifiable
	}
	ageStr := "?"
	if age, ok := ageMinutes(ref.date); ok {
		ageStr = strconv.Itoa(age)
	}
	logf("HELD %s — %s at=%s age=%sm", id, ref.msg, ref.date, ageStr)
	return exitOK
}

func cmdList() int {
	ids, status := store.list()
	switch status {
	case claimUnverifiable:
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
