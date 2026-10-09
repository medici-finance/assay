package deskkit

// dispatchrecord.go — one structured line per dispatch, beside the audit log.
//
// WHY THIS EXISTS. A dispatch (an agent launched onto a brief or a pull request) left no durable
// trace of which brief, which pull request, which session and which tier it was: only a label on
// the pull request, once one existed, and one free-text audit line. Nothing could ask "how often
// does a strong-tier worker need a second dispatch?" or join a verify failure back to the dispatch
// that produced the work. deskdispatch now writes one `dispatched` line per dispatch, and
// deskclaim-ref writes one `released` line per release that removed a held claim, to
// <StateDir>/dispatch-records.jsonl (mode 0600, append-only, never committed to git).
//
// dispatch_ref — THE JOIN KEY. `<claim_key>@<YYYYMMDDTHHMMSSZ>.<12 lowercase hex>`:
// the claim key (per ITEM — every re-dispatch, re-review and stale reclaim reuses it), the UTC
// second deskdispatch read right after claim-acquire succeeded, and a 48-bit nonce from a
// cryptographically secure source. The timestamp alone cannot make a per-run id (two runs of one
// item can share a second); the nonce does. The ref embeds the claim key, so it is LOCAL state:
// in clear only here and in the agent worktree's `git config --worktree assay.dispatchRef`, never
// on a non-private surface, where a consumer writes at most a sha256 of the full ref.
//
// WHAT IS NEVER RECORDED. No prompt text, brief body, PR/issue text, tool output, transcript,
// vendor model name or per-person metric. There is no free-text field: every string is a key, an
// identifier or a closed-vocabulary value, bounded to 256 bytes with no control character, and
// the tier fields accept only the dispatch tier vocabulary, so a model slug can never land in
// one. These records are for aggregate analysis per brief / tier / kit, never for ranking people
// or agents.

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// DispatchRecordSchema is the record's schema id; a reader refuses any other value.
const DispatchRecordSchema = "dispatch-record-v1"

// The two events. `dispatched` is written by deskdispatch after its model-stamp step; `released`
// by deskclaim-ref after a release that removed an existing claim.
const (
	DispatchEventDispatched = "dispatched"
	DispatchEventReleased   = "released"
)

// The model-stamp outcomes a `dispatched` line records (deskdispatch step 5).
const (
	ModelStampApplied = "applied"
	ModelStampPending = "pending"
	ModelStampSkipped = "skipped"
)

// DispatchRecordsFile is the record store's file name inside StateDir, beside audit.jsonl.
const DispatchRecordsFile = "dispatch-records.jsonl"

// dispatchRecordMaxBytes bounds every string field and the dispatch_ref as a whole.
const dispatchRecordMaxBytes = 256

// dispatchRefNonceBytes is the nonce size: 6 bytes = 48 bits = 12 lowercase hex characters.
const dispatchRefNonceBytes = 6

// dispatchRefTimeLayout is ISO-8601 basic form, UTC: YYYYMMDDTHHMMSSZ.
const dispatchRefTimeLayout = "20060102T150405Z"

// claimKeyRe bounds a claim key to the prefix grammar every dispatch_ref consumer parses: 1-226
// printable ASCII bytes with no `@`, no space and no control byte. 226 = 256 - len("@" + 16-byte
// timestamp + "." + 12-char nonce), so a ref built from any accepted key fits the 256-byte cap.
var claimKeyRe = regexp.MustCompile(`^[\x21-\x3F\x41-\x7E]{1,226}$`)

// dispatchRefSuffixRe is everything after the claim key: "@" timestamp "." nonce.
var dispatchRefSuffixRe = regexp.MustCompile(`^@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$`)

// DispatchRecord is one line of the dispatch record store. Nullable fields are pointers so a
// missing value serializes as JSON null — never as a guessed default. The JSON keys and their
// order are the schema.
type DispatchRecord struct {
	Schema      string  `json:"schema"`
	Event       string  `json:"event"`
	TS          string  `json:"ts"`
	DispatchRef *string `json:"dispatch_ref"`
	ClaimKey    string  `json:"claim_key"`
	Repo        string  `json:"repo"`
	Item        *string `json:"item"`
	Brief       *string `json:"brief"`
	Kit         *string `json:"kit"`
	Branch      *string `json:"branch"`
	PR          *int    `json:"pr"`
	SessionTag  string  `json:"session_tag"`
	Tier        *string `json:"tier"`
	BriefExec   *string `json:"brief_exec_tier"`
	BriefEffort *string `json:"brief_effort"`
	ModelStamp  *string `json:"model_stamp"`
	// AttemptLocal is 1 + the number of earlier `dispatched` lines with the same claim_key in
	// THIS store. It is a LOCAL ordinal — a second machine dispatching the same item keeps its
	// own count — never a global attempt number.
	AttemptLocal *int `json:"attempt_local"`
}

// MintDispatchRef builds `<claimKey>@<t as YYYYMMDDTHHMMSSZ>.<12 lowercase hex>` from an injected
// instant and an injected entropy source (production passes time.Now().UTC() and
// crypto/rand.Reader). A short or failing read is an ERROR: the caller proceeds with no ref, and
// this never falls back to a weaker source or invents a nonce.
func MintDispatchRef(claimKey string, t time.Time, rnd io.Reader) (string, error) {
	if !claimKeyRe.MatchString(claimKey) {
		return "", fmt.Errorf("claim key %q is outside the dispatch_ref prefix grammar (1-226 printable ASCII bytes, no '@')", claimKey)
	}
	if rnd == nil {
		return "", errors.New("no entropy source")
	}
	var b [dispatchRefNonceBytes]byte
	if _, err := io.ReadFull(rnd, b[:]); err != nil {
		return "", fmt.Errorf("reading the dispatch_ref nonce: %w", err)
	}
	return claimKey + "@" + t.UTC().Format(dispatchRefTimeLayout) + "." + hex.EncodeToString(b[:]), nil
}

// DispatchRefClaimKey returns the part of ref before its "@" — the claim key a ref was minted for
// (a claim key never contains "@") — and whether ref had one.
func DispatchRefClaimKey(ref string) (string, bool) {
	i := strings.IndexByte(ref, '@')
	if i <= 0 {
		return "", false
	}
	return ref[:i], true
}

// ValidateDispatchRecord refuses a record that is not exactly the schema: an unknown schema or
// event, a timestamp that is not RFC3339, a claim key outside the prefix grammar, a dispatch_ref
// that is not `<the record's OWN claim_key>@YYYYMMDDTHHMMSSZ.<12 lowercase hex>`, a tier outside
// the dispatch tier vocabulary, an effort outside S/M/L, a model_stamp outside
// applied/pending/skipped, any string over 256 bytes or carrying a control character, and the
// per-event field shape (a `released` line carries none of the dispatch-only fields).
func ValidateDispatchRecord(r DispatchRecord) error {
	if r.Schema != DispatchRecordSchema {
		return fmt.Errorf("schema %q is not %q", r.Schema, DispatchRecordSchema)
	}
	if r.Event != DispatchEventDispatched && r.Event != DispatchEventReleased {
		return fmt.Errorf("event %q is not %q or %q", r.Event, DispatchEventDispatched, DispatchEventReleased)
	}
	strs := []struct {
		name string
		v    *string
	}{
		{"schema", &r.Schema}, {"event", &r.Event}, {"ts", &r.TS}, {"dispatch_ref", r.DispatchRef},
		{"claim_key", &r.ClaimKey}, {"repo", &r.Repo}, {"item", r.Item}, {"brief", r.Brief},
		{"kit", r.Kit}, {"branch", r.Branch}, {"session_tag", &r.SessionTag}, {"tier", r.Tier},
		{"brief_exec_tier", r.BriefExec}, {"brief_effort", r.BriefEffort}, {"model_stamp", r.ModelStamp},
	}
	for _, s := range strs {
		if s.v == nil {
			continue
		}
		if err := checkRecordString(s.name, *s.v); err != nil {
			return err
		}
	}
	if _, err := time.Parse(time.RFC3339, r.TS); err != nil {
		return fmt.Errorf("ts %q is not RFC3339", r.TS)
	}
	if !claimKeyRe.MatchString(r.ClaimKey) {
		return fmt.Errorf("claim_key %q is outside the prefix grammar (1-226 printable ASCII bytes, no '@')", r.ClaimKey)
	}
	if strings.TrimSpace(r.Repo) == "" || !strings.Contains(r.Repo, "/") {
		return fmt.Errorf("repo %q is not owner/name", r.Repo)
	}
	if strings.TrimSpace(r.SessionTag) == "" {
		return errors.New("session_tag is empty")
	}
	if r.DispatchRef != nil {
		ref := *r.DispatchRef
		if !strings.HasPrefix(ref, r.ClaimKey+"@") {
			return fmt.Errorf("dispatch_ref %q does not start with the record's own claim_key %q + \"@\"", ref, r.ClaimKey)
		}
		if !dispatchRefSuffixRe.MatchString(ref[len(r.ClaimKey):]) {
			return fmt.Errorf("dispatch_ref %q is not <claim_key>@YYYYMMDDTHHMMSSZ.<12 lowercase hex>", ref)
		}
	}
	for _, t := range []struct {
		name string
		v    *string
	}{{"tier", r.Tier}, {"brief_exec_tier", r.BriefExec}} {
		if t.v != nil && !isDispatchTier(*t.v) {
			return fmt.Errorf("%s %q is not one of %s", t.name, *t.v, strings.Join(DispatchTiers(), "|"))
		}
	}
	if r.BriefEffort != nil {
		switch *r.BriefEffort {
		case "S", "M", "L":
		default:
			return fmt.Errorf("brief_effort %q is not S|M|L", *r.BriefEffort)
		}
	}
	if r.ModelStamp != nil {
		switch *r.ModelStamp {
		case ModelStampApplied, ModelStampPending, ModelStampSkipped:
		default:
			return fmt.Errorf("model_stamp %q is not applied|pending|skipped", *r.ModelStamp)
		}
	}
	if r.PR != nil && *r.PR <= 0 {
		return fmt.Errorf("pr %d is not a positive number", *r.PR)
	}
	switch r.Event {
	case DispatchEventDispatched:
		for _, req := range []struct {
			name string
			v    *string
		}{{"item", r.Item}, {"kit", r.Kit}, {"tier", r.Tier}, {"model_stamp", r.ModelStamp}} {
			if req.v == nil || *req.v == "" {
				return fmt.Errorf("a dispatched record needs %s", req.name)
			}
		}
		if r.AttemptLocal == nil || *r.AttemptLocal < 1 {
			return errors.New("a dispatched record needs attempt_local >= 1")
		}
	case DispatchEventReleased:
		if r.Item != nil || r.Brief != nil || r.Kit != nil || r.Branch != nil || r.PR != nil ||
			r.Tier != nil || r.BriefExec != nil || r.BriefEffort != nil || r.ModelStamp != nil || r.AttemptLocal != nil {
			return errors.New("a released record carries no dispatch-only field (item, brief, kit, branch, pr, tier, brief_exec_tier, brief_effort, model_stamp, attempt_local must be null)")
		}
	}
	return nil
}

// checkRecordString enforces the per-field bound: at most 256 bytes, valid UTF-8, no control
// character (a newline would split the line; an escape sequence would rewrite a terminal).
func checkRecordString(name, v string) error {
	if len(v) > dispatchRecordMaxBytes {
		return fmt.Errorf("%s is %d bytes, over the %d-byte cap", name, len(v), dispatchRecordMaxBytes)
	}
	if !utf8.ValidString(v) {
		return fmt.Errorf("%s is not valid UTF-8", name)
	}
	for _, c := range v {
		if unicode.IsControl(c) {
			return fmt.Errorf("%s holds a control character (%U)", name, c)
		}
	}
	return nil
}

// DispatchRecordsPath is <StateDir>/dispatch-records.jsonl.
func DispatchRecordsPath() (string, error) {
	dir, err := deskDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DispatchRecordsFile), nil
}

// AppendDispatchRecord stamps the schema (and, when empty, ts and session_tag), computes
// attempt_local for a `dispatched` line, validates, and appends ONE line to
// <StateDir>/dispatch-records.jsonl (mode 0600, O_APPEND). A record that does not validate is
// never written. No rotation: one line per dispatch is low volume.
func AppendDispatchRecord(r *DispatchRecord) error {
	if r == nil {
		return errors.New("nil dispatch record")
	}
	r.Schema = DispatchRecordSchema
	if r.TS == "" {
		r.TS = time.Now().UTC().Format(time.RFC3339)
	}
	if r.SessionTag == "" {
		r.SessionTag = SessionTag()
	}
	dir, err := deskDir()
	if err != nil {
		return fmt.Errorf("cannot resolve the desk-tools state dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create the desk-tools state dir: %w", err)
	}
	path := filepath.Join(dir, DispatchRecordsFile)
	if r.Event == DispatchEventDispatched {
		n, err := countDispatched(path, r.ClaimKey)
		if err != nil {
			return err
		}
		a := n + 1
		r.AttemptLocal = &a
	}
	if err := ValidateDispatchRecord(*r); err != nil {
		return fmt.Errorf("refusing an invalid dispatch record: %w", err)
	}
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// countDispatched counts the `dispatched` lines for claimKey already in the store. An absent file
// is zero; an unreadable one is an error (a guessed ordinal is worse than none). A line that does
// not parse is skipped — the store is append-only and one torn line must not wedge every later
// dispatch's record.
func countDispatched(path, claimKey string) (int, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var rec struct {
			Event    string `json:"event"`
			ClaimKey string `json:"claim_key"`
		}
		if json.Unmarshal(sc.Bytes(), &rec) != nil {
			continue
		}
		if rec.Event == DispatchEventDispatched && rec.ClaimKey == claimKey {
			n++
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return n, nil
}

// WorktreeDispatchRef returns the `assay.dispatchRef` deskdispatch recorded in the current
// worktree's config, or "" when there is none (not a worktree, the extension is off, the key is
// absent). Like the run key, it is read from the worktree the verb runs in, never from a flag.
func WorktreeDispatchRef() string {
	out, err := exec.Command("git", "config", "--worktree", "assay.dispatchRef").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
