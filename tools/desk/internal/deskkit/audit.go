package deskkit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	path, err := auditPath()
	if err != nil {
		return nil, Unverifiable("cannot resolve audit path (HOME missing?)", err)
	}
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
			return nil, Unverifiable(
				fmt.Sprintf("malformed audit line %d — run `deskaudit recover` (quarantines the bad line and carries good entries forward; a plain move resets the budget + idempotency)", n), err)
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
