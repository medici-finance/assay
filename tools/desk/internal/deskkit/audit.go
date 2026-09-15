package deskkit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Audit result values (the `result` field of an audit line).
const (
	ResultOK           = "ok"
	ResultNoop         = "noop"
	ResultRefused      = "refused"
	ResultDisabled     = "disabled"
	ResultRateLimited  = "ratelimited"
	ResultUnverifiable = "unverifiable"
	// ResultUnwritten (#448) — a precondition could not be positively
	// verified, and the failure occurred BEFORE any outward write was attempted: a GET
	// that 403'd or timed out, a trust-gate read, a local CI/diff determination that came
	// back pending/short. Exit code is still ExitUnverifiable (6) — the operator-facing
	// contract is unchanged — but the audited RESULT is distinct from ResultUnverifiable
	// because the two must be billed differently. See ratelimit.go's chargesBudget for
	// why: ResultUnverifiable charges on the theory that the call may have reached the
	// remote, and that theory is false for every ResultUnwritten line by construction —
	// nothing downstream of it ever calls the mutating endpoint.
	//
	// Distinct from ResultRefused, which is also a no-write outcome but is a POSITIVE,
	// compiled-in determination ("this is disallowed"). ResultUnwritten is the opposite
	// epistemic state: the precondition could NOT be determined at all. Both share the
	// same non-charging budget treatment for the same reason (no remote amplification to
	// cap) and the same non-progress breaker treatment (a repeated failed precondition
	// check is exactly the spinning-caller shape the breaker exists to stop).
	ResultUnwritten = "unwritten"
	// ResultDryRun — the invocation carried --dry-run: it validated, it read the
	// remote, and it STOPPED BEFORE THE WRITE. Distinct from ResultNoop, which means
	// the write was attempted and idempotency short-circuited it (#214).
	//
	// The distinction earns its keep in two places, both of which read `noop` as
	// something a dry run is not:
	//
	//   - the rate limiter's breaker counts `noop` as non-progress, so five rehearsals
	//     of a release used to open a 15-minute breaker against the real one;
	//   - AlreadyDoneIn counts `noop` as done, so in any tool whose idempotency is
	//     anchored on the audit log (deskpost's is; deskrelease's is anchored on the
	//     remote) a rehearsal would make the real act look already-performed.
	//
	// A tool may emit this ONLY on a path that provably performed no outward write.
	// Emitting it from a path that writes would launder a real write past both meters
	// — which is why the flag that selects it must also be the flag that suppresses
	// the write.
	ResultDryRun = "dryrun"
)

// Entry is one audit line. Only outward-write verbs set BodyDigest; it
// is omitted otherwise so the on-disk line matches the schema exactly for
// non-body verbs. PR and HeadSHA are pointers so they serialise as JSON null when a
// verb has no PR / head.
type Entry struct {
	TS         string  `json:"ts"`
	Tool       string  `json:"tool"`
	Verb       string  `json:"verb"`
	ArgsDigest string  `json:"argsDigest"`
	BodyDigest string  `json:"bodyDigest,omitempty"`
	Repo       string  `json:"repo"`
	PR         *int    `json:"pr"`
	HeadSHA    *string `json:"headSHA"`
	Result     string  `json:"result"`
	Detail     string  `json:"detail"`
	SourceSHA  string  `json:"sourceSHA"`
	BuiltAt    string  `json:"builtAt"`
	SessionTag string  `json:"sessionTag"`
	// Title is the item title as it stood when the entry was written, so a later run
	// can detect a title-rename-after-bless (trust.go: lastEditedAt does not track
	// renames). Omitted for verbs that have no item title. Log sanitizes it through
	// StripControl — the audit log is replayed into terminals and agent context, so a
	// title is public-origin text there too.
	Title string `json:"title,omitempty"`
}

func auditPath() (string, error) {
	dir, err := deskDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "audit.jsonl"), nil
}

// segmentPattern matches a ROTATED audit segment name EXACTLY — `audit.jsonl.<YYYY-MM-DD>`
// with an optional `.N` disambiguator. Nothing else in the state directory can satisfy it:
// `audit.lock`, `audit.jsonl.corrupt-<ts>`, the kill-switch flags and anything a future
// change adds are all outside it, so rotation and segment enumeration can never sweep one
// in. A prefix match here would be exactly that defect.
var segmentPattern = regexp.MustCompile(`^audit\.jsonl\.\d{4}-\d{2}-\d{2}(\.\d+)?$`)

// auditNow is the clock rotation reads. A test hook in the dirOverride mould: production
// uses time.Now, white-box tests drive a day boundary without waiting for one. Like
// dirOverride it is deliberately NOT wired to any env var or flag.
var auditNow = time.Now

// segmentPaths returns every file the ledger currently spans, OLDEST FIRST: the rotated
// segments in chronological order, then the live audit.jsonl (which is listed whether or
// not it exists yet — a missing file is empty history to every reader here).
//
// Segment names sort lexicographically into chronological order because the date is
// zero-padded ISO, and `audit.jsonl.2026-09-13` sorts before `audit.jsonl.2026-09-13.1`
// which sorts before `audit.jsonl.2026-09-14`.
//
// THIS LIST IS WHY ROTATION IS NOT A RESET. auditrecover.go's package comment states the
// case plainly: the rate-limit counter, the circuit breaker and the idempotency store are
// pure functions of the entries, so moving the file aside returns every budget to full and
// makes the store forget every prior write. Rotation avoids that not by copying state
// forward but by deleting nothing and making every reader read the whole span — so the
// slice LoadEntries returns is identical, row for row and in order, to the one it would
// have returned had the ledger never been rotated.
func segmentPaths() ([]string, error) {
	dir, err := deskDir()
	if err != nil {
		return nil, Unverifiable("cannot resolve audit path (HOME missing?)", err)
	}
	live := filepath.Join(dir, "audit.jsonl")
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{live}, nil
		}
		return nil, Unverifiable("cannot list desk-tools dir", err)
	}
	var segs []string
	for _, e := range ents {
		if e.IsDir() || !segmentPattern.MatchString(e.Name()) {
			continue
		}
		segs = append(segs, e.Name())
	}
	sort.Strings(segs)
	out := make([]string, 0, len(segs)+1)
	for _, s := range segs {
		out = append(out, filepath.Join(dir, s))
	}
	return append(out, live), nil
}

// rotationDue reports whether an append-only file last written at mtime belongs to an
// earlier UTC day than now. An append-only file's mtime IS the instant of its last append,
// so this costs one os.Stat and reads no bytes.
func rotationDue(mtime, now time.Time) bool {
	return mtime.UTC().Format("2006-01-02") < now.UTC().Format("2006-01-02")
}

// rotateIfNeeded renames audit.jsonl to audit.jsonl.<its last day> when its last append
// fell on an earlier UTC day, so the file every append and every tail read touches stays
// one day long. Log calls it before it opens the file; Log's existing O_CREATE re-creates
// the live file, so there is no window in which the ledger is absent to a lock holder.
//
// BEST-EFFORT, NEVER LOAD-BEARING. Every failure path returns silently and the append
// proceeds against whatever file is there: the worst case of a rotation that cannot happen
// is a file that stays long — the state this work starts from — never a lost or unwritten
// row and never a changed exit code.
//
// The lock is the audit flock every other writer and `deskaudit recover` take, acquired
// with a SINGLE non-blocking attempt rather than lockAudit's 60-second wait: rotation is
// due at most once a day, and a rotation that blocked a writer for a minute would have
// converted a cleanup into an outage. A busy lock simply means someone else is writing;
// the next invocation checks again.
func rotateIfNeeded() {
	dir, err := deskDir()
	if err != nil {
		return
	}
	path := filepath.Join(dir, "audit.jsonl")
	fi, err := os.Stat(path)
	if err != nil || !rotationDue(fi.ModTime(), auditNow()) {
		return
	}

	unlock, ok := tryLockAudit(dir)
	if !ok {
		return
	}
	defer unlock()

	// Re-check under the lock: the loser of a race between two processes that both saw a
	// due rotation must do nothing rather than rotate a file the winner already replaced.
	fi, err = os.Stat(path)
	if err != nil || !rotationDue(fi.ModTime(), auditNow()) {
		return
	}

	base := "audit.jsonl." + fi.ModTime().UTC().Format("2006-01-02")
	target := filepath.Join(dir, base)
	for i := 1; ; i++ {
		if _, serr := os.Stat(target); os.IsNotExist(serr) {
			break
		}
		target = filepath.Join(dir, fmt.Sprintf("%s.%d", base, i))
	}
	_ = os.Rename(path, target)
}

// LastEntry returns the newest entry in the ledger. It reads at most one tailBlockSize
// block per segment it has to touch, whatever the ledger's size — which is the whole
// point: Guard's disarm-transition check asks one question of one line and used to pay a
// whole-file parse for it.
//
// ok=false for a missing ledger, a wholly empty one, or a final line that does not parse.
// That is exactly lastResultWas' existing best-effort contract (killswitch.go): corruption
// is surfaced as a refusal by the outward-write flow's LoadEntries, not by the guard.
//
// When the LIVE file is empty — the state immediately after a rotation — it continues into
// the newest prior segment, so a UTC day boundary cannot blind the disarm check.
func LastEntry() (Entry, bool) {
	paths, err := segmentPaths()
	if err != nil {
		return Entry{}, false
	}
	for i := len(paths) - 1; i >= 0; i-- {
		var out Entry
		found, malformed := false, false
		serr := scanBackwards(paths[i], func(line []byte) bool {
			var e Entry
			if json.Unmarshal(line, &e) != nil {
				malformed = true
				return false
			}
			out, found = e, true
			return false
		})
		if malformed {
			return Entry{}, false
		}
		if serr != nil {
			if os.IsNotExist(serr) {
				continue // this segment is gone; an older one may still answer
			}
			return Entry{}, false
		}
		if found {
			return out, true
		}
	}
	return Entry{}, false
}

// SessionTag names the AGENT this process is acting as: $DESK_SESSION if set, else
// $CLAUDE_CODE_SESSION_ID (the variable the Claude Code harness actually exports), else
// the legacy $CLAUDE_SESSION_ID, else "unknown". It is self-reported: forensics, not
// enforcement.
//
// WHY $DESK_SESSION COMES FIRST. A dispatched agent is a CHILD PROCESS of the session that
// fanned it out, so it inherits the harness's session id verbatim: every agent in a
// fan-out reports the same $CLAUDE_CODE_SESSION_ID, the dispatcher's. A tag read from that
// alone therefore names the DISPATCHER, not the actor — which is wrong for an audit trail
// (a fan-out's whole output attributed to one id) and wrong for anything keyed on it, since
// a per-session budget then covers the fan-out rather than the agent. $DESK_SESSION is the
// desk tools' own per-agent session id, set per dispatched agent and per role window, and
// it is what distinguishes siblings; deskwt and deskroster already resolve it ahead of the
// harness id, so preferring it here makes the tools agree on who "this session" is rather
// than answering it two ways.
//
// The harness ids remain the fallback, so a plain human-driven session that never sets
// $DESK_SESSION is unaffected.
func SessionTag() string {
	if s := strings.TrimSpace(os.Getenv("DESK_SESSION")); s != "" {
		return s
	}
	if s := os.Getenv("CLAUDE_CODE_SESSION_ID"); s != "" {
		return s
	}
	if s := os.Getenv("CLAUDE_SESSION_ID"); s != "" { // legacy fallback
		return s
	}
	return "unknown"
}

// Sha256Hex returns the lowercase-hex sha256 of b.
func Sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ArgsDigest returns the sha256 hex of the tool's args (the audit `argsDigest`
// field). Args are NUL-joined so a boundary can't be forged by concatenation.
func ArgsDigest(args []string) string {
	return Sha256Hex([]byte(strings.Join(args, "\x00")))
}

// Log appends exactly one JSON line to ~/.config/assay/audit.jsonl. The file is
// only ever appended to (O_APPEND); tools never truncate or rewrite it. The
// directory is created 0700 and the file 0600 on first use. Unset fields are filled:
// TS = now (RFC3339 UTC), SessionTag from the environment, SourceSHA/BuiltAt from the
// embedded version stamp. A missing Result is itself an Unverifiable programming
// error (every line must classify its outcome).
func Log(e Entry) error {
	if e.Result == "" {
		return Unverifiable("audit entry missing result (internal)", nil)
	}
	// Record the CANONICAL tool key so a line written under a variant spelling — most often
	// a guard() line keyed off the running binary's basename (a test build, a locally built
	// or renamed copy) — lands in the same bucket the write path counts, instead of splitting
	// the trail and escaping the budget (audittoolkey.go). Best-effort: a key that resolves
	// to no known tool is left exactly as given (CanonicalToolKeyOr), so recording never
	// fails closed and a non-tool binary's guard line still records verbatim for forensics.
	e.Tool = CanonicalToolKeyOr(e.Tool)
	if e.TS == "" {
		e.TS = time.Now().UTC().Format(time.RFC3339)
	}
	e.Title = StripControl(e.Title)
	if e.SessionTag == "" {
		e.SessionTag = SessionTag()
	}
	if e.SourceSHA == "" || e.BuiltAt == "" {
		s, b := Version()
		if e.SourceSHA == "" {
			e.SourceSHA = s
		}
		if e.BuiltAt == "" {
			e.BuiltAt = b
		}
	}

	dir, err := deskDir()
	if err != nil {
		return Unverifiable("cannot resolve desk-tools dir (HOME missing?)", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Unverifiable("cannot create desk-tools dir", err)
	}
	// Daily rotation, best-effort: one os.Stat on the common path, and a rename only on
	// the first append of a new UTC day. It deletes nothing and every reader spans the
	// segments, so no meter observes it — see segmentPaths.
	rotateIfNeeded()
	path := filepath.Join(dir, "audit.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Unverifiable("cannot open audit file", err)
	}
	defer f.Close()

	line, err := json.Marshal(e)
	if err != nil {
		return Unverifiable("cannot marshal audit entry", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return Unverifiable("cannot append audit line", err)
	}
	return nil
}

// LoadEntries reads and parses the whole audit file. Semantics:
//   - a MISSING file is empty history — bootstrap, returns (nil, nil);
//   - an unreadable file, ANY malformed line, or a scan error is a REFUSAL
//     (Unverifiable → exit 6). The tools never skip or repair a bad line here; the
//     printed recovery is `deskaudit recover`, which quarantines the bad LINE and carries
//     every good entry forward (RecoverCorruptAudit) — NOT a plain `mv` of the whole file,
//     which resets the counter and idempotency store (see auditrecover.go).
//
// This is the canonical reader the outward-write flow calls (under its flock) BEFORE
// AllowWrite / AlreadyDoneIn, so corruption surfaces as a single exit-6 refusal.
func LoadEntries() ([]Entry, error) {
	paths, err := segmentPaths()
	if err != nil {
		return nil, err
	}
	var all []Entry
	for _, p := range paths {
		entries, lerr := loadEntriesFrom(p)
		if lerr != nil {
			return nil, lerr
		}
		all = append(all, entries...)
	}
	return all, nil
}

// loadEntriesFrom is LoadEntries over ONE file. The whole-ledger reader is the loop above
// it, so the file-level semantics — missing file is empty history, first malformed line is
// a refusal — stay stated in one place.
func loadEntriesFrom(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, Unverifiable("cannot read audit file — run `deskaudit recover` (quarantines the bad content and carries good entries forward; a plain move resets the budget + idempotency)", err)
	}
	defer f.Close()

	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		n++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			where := ""
			if base := filepath.Base(path); base != "audit.jsonl" {
				where = " of " + base
			}
			return nil, Unverifiable(
				fmt.Sprintf("malformed audit line %d%s — run `deskaudit recover` (quarantines the bad line and carries good entries forward; a plain move resets the budget + idempotency)", n, where), err)
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, Unverifiable("error scanning audit file", err)
	}
	return entries, nil
}

// FirstTS returns the ts of the first audit entry, for the deskboard reset banner
// (a suspicious audit-file reset — history lost — is visible). Empty string on
// empty history; Unverifiable on a corrupt file.
func FirstTS() (string, error) {
	entries, err := LoadEntries()
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	return entries[0].TS, nil
}
