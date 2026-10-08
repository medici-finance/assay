package deskkit

// verifyoutcomes.go — #882: the per-file verify-outcome record layer.
//
// WHY THIS EXISTS. Every verify Evidence PR used to append one line to the single shared
// docs/streams/verify-outcomes.jsonl. The forge merges pull requests server-side with no
// merge=union driver, so every landing turned every sibling Evidence PR touching that one path
// CONFLICTING (19 of 40 open on 2026-09-27). This file is the pure, forge-free and git-free
// record layer that replaces the shared log with one NEW file per outcome — a shape concurrent
// PRs never collide on, because two PRs only ever add the SAME path when they carry
// byte-identical content (RecordName is a pure function of the record's bytes), and two
// identical adds merge cleanly.
//
// LAYERING. This is the pure record layer: name a record from its bytes (RecordName), parse one
// (ParseRecord), and read every record under a root — both the new per-file layout and the
// legacy shared log, unioned and deduped (ReadVerifyOutcomes), then reduce to one winner per
// brief (LatestPerBrief). It touches no forge and no git, so it is tested with neither. statusgen
// keeps an independent copy of the read half in its own module (a separate Go module cannot
// import this package) — keep the two in sync by hand; a structural test in each module (the
// choke-point guard) pins that module's single reader.
//
// THE TRANSITION WINDOW. The legacy log is frozen (deskevidence refuses ever appending to it
// again, #882 Task step 5) but stays on disk, unretired, until every open PR that
// still touches it has landed (Task step 8). ReadVerifyOutcomes reads BOTH layouts for as long as
// that window is open, and dedupes by content digest so a migrated record and its still-present
// source log line count once.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// OutcomeRecordsDir is the repo-relative directory verify-outcome records live under, one file
// per outcome, per-stream subdirectory (docs/streams/verify-outcomes/<stream>/*.json).
const OutcomeRecordsDir = "docs/streams/verify-outcomes"

// legacyOutcomesGlob matches the retired shared log AND any rotation shard of it
// (verify-outcomes.jsonl, verify-outcomes-2026-10.jsonl, …), directly under docs/streams/. The
// reader keeps unioning every row already committed there through the whole transition window.
const legacyOutcomesGlob = "verify-outcomes*.jsonl"

var (
	outcomeStreamRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	outcomeNumRe    = regexp.MustCompile(`^[0-9]+$`)
)

// OutcomeRecord is one verify outcome, read generically enough that a downstream reader
// (verifyloop's outcomeRow, or a verify-wake-v1 receipt) can unmarshal Raw into its own richer
// type. Raw is the CANONICAL line — the record's single JSON object, trimmed of surrounding
// whitespace, with NO trailing newline — so two records with the same canonical line always
// compare digest-equal regardless of which layout (a record file, whose bytes are the canonical
// line plus a trailing newline, or a legacy log line) produced them.
type OutcomeRecord struct {
	Brief  string // the "brief" field: "<stream>/<NN>"
	TS     string // the "ts" field, RFC 3339
	Raw    []byte // the canonical JSON line, no trailing newline
	Name   string // RecordName's basename for this record's bytes (used for LatestPerBrief's tie-break)
	Source string // "record:<path>" or "legacy:<path>", for diagnostics only
	Digest string // RecordDigest(Raw) — the same digest term RecordName embeds in the path
}

// ParseRecord parses one verify-outcome record's raw bytes — a record file's whole content
// (the canonical line plus its trailing newline) or a single line of the legacy log — and
// returns the record with Raw holding the canonical line. A record with no `brief` key, or
// bytes that are not one JSON object, is refused: a verify-outcome record is never partially
// understood.
func ParseRecord(raw []byte) (OutcomeRecord, error) {
	line := bytes.TrimSpace(raw)
	if len(line) == 0 {
		return OutcomeRecord{}, errors.New("empty verify-outcome record")
	}
	var head struct {
		Brief string `json:"brief"`
		TS    string `json:"ts"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return OutcomeRecord{}, fmt.Errorf("invalid verify-outcome record JSON: %w", err)
	}
	if strings.TrimSpace(head.Brief) == "" {
		return OutcomeRecord{}, errors.New("verify-outcome record has no brief key")
	}
	canon := append([]byte{}, line...)
	return OutcomeRecord{
		Brief:  head.Brief,
		TS:     head.TS,
		Raw:    canon,
		Digest: RecordDigest(canon),
	}, nil
}

// RecordDigest is the first 12 hex digits of the SHA-256 of canonicalLine plus a trailing
// newline — the record file's exact on-disk bytes ("the content is exactly today's row, one
// line, unchanged schema", per the brief's record-layout decision). A migrated record's bytes
// therefore equal its source log line plus "\n", and the digest of each agrees.
func RecordDigest(canonicalLine []byte) string {
	buf := make([]byte, 0, len(canonicalLine)+1)
	buf = append(buf, canonicalLine...)
	buf = append(buf, '\n')
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])[:12]
}

// CanonicalBytes returns the exact bytes a record file holds for canonicalLine: the line itself
// plus one trailing newline.
func CanonicalBytes(canonicalLine []byte) []byte {
	out := make([]byte, 0, len(canonicalLine)+1)
	out = append(out, canonicalLine...)
	return append(out, '\n')
}

// SplitBriefKey splits a "<stream>/<NN>" brief key into its stream name and number, refusing
// (never sanitising) any other shape: stream must match ^[a-z0-9][a-z0-9-]*$ and num must match
// ^[0-9]+$. A malformed key (a directory-traversal segment, an empty component, an uppercase
// letter) is refused here rather than silently reaching a filesystem path.
func SplitBriefKey(brief string) (stream, num string, err error) {
	parts := strings.SplitN(brief, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid brief key %q: want <stream>/<NN>", brief)
	}
	stream, num = parts[0], parts[1]
	if !outcomeStreamRe.MatchString(stream) {
		return "", "", fmt.Errorf("invalid brief stream %q in key %q", stream, brief)
	}
	if !outcomeNumRe.MatchString(num) {
		return "", "", fmt.Errorf("invalid brief number %q in key %q", num, brief)
	}
	return stream, num, nil
}

// RecordName computes the repo-relative path a verify-outcome record's bytes land at:
//
//	docs/streams/verify-outcomes/<stream>/<NN>-<YYYYMMDDTHHMMSSZ>-<digest12>.json
//
// recordBytes is the record's single JSON line (with or without a trailing newline — ParseRecord
// trims it). RecordName is a PURE function of the bytes: two callers computing it for the same
// content always agree, which is what makes two concurrent PRs' adds collision-free (they only
// ever land the same path when they carry byte-identical content, and two identical adds merge
// cleanly). `brief` and `ts` are the only fields RecordName reads; every other field is opaque to
// it and travels through unparsed.
func RecordName(recordBytes []byte) (string, error) {
	rec, err := ParseRecord(recordBytes)
	if err != nil {
		return "", err
	}
	stream, num, err := SplitBriefKey(rec.Brief)
	if err != nil {
		return "", err
	}
	ts, err := time.Parse(time.RFC3339, rec.TS)
	if err != nil {
		return "", fmt.Errorf("invalid verify-outcome record ts %q for brief %s: %w", rec.TS, rec.Brief, err)
	}
	compact := ts.UTC().Format("20060102T150405Z")
	return path.Join(OutcomeRecordsDir, stream, fmt.Sprintf("%s-%s-%s.json", num, compact, rec.Digest)), nil
}

// ReadVerifyOutcomes reads every verify-outcome record under root: every per-file record under
// docs/streams/verify-outcomes/<stream>/*.json AND every line of any legacy
// docs/streams/verify-outcomes*.jsonl, deduped by the digest of their canonical bytes so a record
// present in both layouts (the migration's own steady state through the transition window) is
// returned once.
//
// Three-state (common-clause C4): an ABSENT records directory and an absent legacy log together
// are an EMPTY set, never an error — an adopter tree with no verify-outcome history yet. An
// UNREADABLE file or directory (a permission error, or any read failure that is not "does not
// exist") is an ERROR — could-not-check, never a skipped record — for the per-file layout, whose
// records are load-bearing accounting. The legacy log keeps its pre-existing tolerance for a
// malformed LINE (skipped, never fatal to the read) but not for an unreadable FILE.
func ReadVerifyOutcomes(root string) ([]OutcomeRecord, error) {
	var records []OutcomeRecord
	seen := map[string]bool{}

	recordsDir := filepath.Join(root, filepath.FromSlash(OutcomeRecordsDir))
	streamDirs, err := os.ReadDir(recordsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("could-not-check: cannot read %s: %w", recordsDir, err)
		}
		streamDirs = nil
	}
	// Deterministic order: sort stream directories and, within each, their files.
	sort.Slice(streamDirs, func(i, j int) bool { return streamDirs[i].Name() < streamDirs[j].Name() })
	for _, sd := range streamDirs {
		if !sd.IsDir() {
			continue
		}
		streamDir := filepath.Join(recordsDir, sd.Name())
		files, ferr := os.ReadDir(streamDir)
		if ferr != nil {
			return nil, fmt.Errorf("could-not-check: cannot read %s: %w", streamDir, ferr)
		}
		names := make([]string, 0, len(files))
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
				names = append(names, f.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			p := filepath.Join(streamDir, name)
			raw, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil, fmt.Errorf("could-not-check: cannot read record %s: %w", p, rerr)
			}
			rec, perr := ParseRecord(raw)
			if perr != nil {
				return nil, fmt.Errorf("could-not-check: malformed verify-outcome record %s: %w", p, perr)
			}
			rec.Name = name
			rec.Source = "record:" + filepath.ToSlash(p)
			if !seen[rec.Digest] {
				seen[rec.Digest] = true
				records = append(records, rec)
			}
		}
	}

	matches, gerr := filepath.Glob(filepath.Join(root, "docs", "streams", legacyOutcomesGlob))
	if gerr != nil {
		return nil, fmt.Errorf("could-not-check: bad legacy verify-outcomes glob: %w", gerr)
	}
	sort.Strings(matches)
	for _, p := range matches {
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			if os.IsNotExist(rerr) {
				continue
			}
			return nil, fmt.Errorf("could-not-check: cannot read %s: %w", p, rerr)
		}
		for _, line := range bytes.Split(raw, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			rec, perr := ParseRecord(line)
			if perr != nil {
				continue // legacy tolerance: a malformed line is skipped, never fatal (pre-existing behaviour)
			}
			if name, nerr := RecordName(line); nerr == nil {
				rec.Name = filepath.Base(name)
			} else {
				continue // legacy tolerance: a line RecordName cannot name is skipped, never fatal
			}
			rec.Source = "legacy:" + filepath.ToSlash(p)
			if !seen[rec.Digest] {
				seen[rec.Digest] = true
				records = append(records, rec)
			}
		}
	}

	return records, nil
}

// MaxClockSkew bounds how far ahead of "now" a verify-outcome record's `ts` may be before it is
// treated as untrustworthy rather than merely "the newest" (#1803 SR-1803-2). Before this bound
// existed, "newest ts wins" (LatestPerBrief, below) was UNBOUNDED: a record with a fabricated or
// clock-skewed future `ts` — whether from a skewed writer clock, a locally-timed value
// mislabelled `Z`, or a hand-built record — stayed "current" for its brief until that instant
// actually passed, silently shadowing every later, genuine outcome. The writer
// (cmdOutcomeRecordWrite, tools/desk/cmd/deskevidence/outcomerecord.go) refuses to land a record
// whose `ts` is more than MaxClockSkew ahead of its own clock; LatestPerBrief additionally never
// lets a future-dated record already on disk (from before the writer refusal existed, or from
// any other lane) win the comparison.
const MaxClockSkew = 5 * time.Minute

// FutureTS reports whether ts — parsed as RFC 3339 — lands more than MaxClockSkew ahead of now.
// An unparsable ts is never "future" here: LatestPerBrief's own tsOf already sorts it as the
// oldest possible instant, so it cannot win on its own account either.
func FutureTS(ts string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false
	}
	return t.After(now.Add(MaxClockSkew))
}

// LatestPerBrief reduces records to one winner per brief key: the newest `ts` wins; ties are
// broken by the lexically greatest record NAME (never by line position or slice order — the
// defect this whole layout retires). A record whose `ts` fails to parse sorts as the oldest
// possible instant, so it never silently outranks a well-formed record for the same brief; if it
// is the ONLY record for that brief it still wins (there is nothing else to prefer). Uses the
// current wall clock for the future-ts bound; see LatestPerBriefAt for the injectable form tests
// use.
func LatestPerBrief(records []OutcomeRecord) (latest map[string]OutcomeRecord, futureByBrief map[string]bool) {
	return LatestPerBriefAt(records, time.Now())
}

// LatestPerBriefAt is LatestPerBrief with an explicit "now", so a test never depends on wall-clock
// time. A record whose `ts` is more than MaxClockSkew ahead of now (SR-1803-2) is NEVER a
// candidate winner: it is excluded from the reduction entirely, and its brief key is reported in
// the second return value so a caller can classify that brief as could-not-check rather than
// either (a) silently letting the future record win, or (b) silently falling back to whatever
// non-future record happens to be left with no signal that anything was excluded.
func LatestPerBriefAt(records []OutcomeRecord, now time.Time) (latest map[string]OutcomeRecord, futureByBrief map[string]bool) {
	out := make(map[string]OutcomeRecord, len(records))
	future := map[string]bool{}
	tsOf := func(r OutcomeRecord) time.Time {
		t, err := time.Parse(time.RFC3339, r.TS)
		if err != nil {
			return time.Time{}
		}
		return t
	}
	for _, rec := range records {
		if FutureTS(rec.TS, now) {
			future[rec.Brief] = true
			continue
		}
		cur, ok := out[rec.Brief]
		if !ok {
			out[rec.Brief] = rec
			continue
		}
		rt, ct := tsOf(rec), tsOf(cur)
		switch {
		case rt.After(ct):
			out[rec.Brief] = rec
		case rt.Equal(ct) && rec.Name > cur.Name:
			out[rec.Brief] = rec
		}
	}
	return out, future
}

// UnderOutcomeRecordsDir is the writer's INDEPENDENT, defense-in-depth check on a record target
// path (#1803 SR-1803-3): run AFTER RecordName has already computed targetRepoPath, it shares NO
// code with RecordName/SplitBriefKey's own regex validation, so a regression in either of those
// (for example outcomeNumRe loosened to admit a traversal-shaped brief number) does not also
// blind this check. It requires p to be relative, and its path.Clean'd form to sit EXACTLY two
// path segments below OutcomeRecordsDir (<stream-dir>/<file>.json), with neither segment empty
// or a "..".
func UnderOutcomeRecordsDir(p string) error {
	if p == "" {
		return errors.New("empty verify-outcome record target path")
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("verify-outcome record target %q is an absolute path", p)
	}
	clean := path.Clean(p)
	prefix := OutcomeRecordsDir + "/"
	if !strings.HasPrefix(clean, prefix) {
		return fmt.Errorf("verify-outcome record target %q resolves to %q, outside %s", p, clean, OutcomeRecordsDir)
	}
	rest := strings.TrimPrefix(clean, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || parts[0] == ".." || parts[1] == ".." {
		return fmt.Errorf("verify-outcome record target %q does not have the expected <stream>/<file> depth under %s", p, OutcomeRecordsDir)
	}
	return nil
}

// --- the wake-receipt writer (verify-reset/03) ----------------------------------------------
//
// Every verify-fail and blocked record carries a complete verify-wake-v1 receipt, derived here
// rather than hand-written, so the planner can hold the brief until something it depends on
// moves. A record that cannot derive one is refused, naming the field; it is never written
// incomplete. A pass (verified) record carries no receipt.

// OutcomeTree reads the repository at one commit: the record's own sha.
type OutcomeTree interface {
	// PathKind is "blob" for a file, "tree" for a directory, "" when rel is absent at the
	// commit. An error means the commit could not be read.
	PathKind(rel string) (string, error)
	// ReadFile returns rel's bytes at the commit.
	ReadFile(rel string) ([]byte, error)
	// ListFiles returns every file under directory rel at the commit, repo-relative.
	ListFiles(rel string) ([]string, error)
}

// OutcomeReceiptInput is what the writer needs beyond the record and the tree.
type OutcomeReceiptInput struct {
	BriefPath   string // the brief's repo-relative path (docs/streams/<stream>/brief-<NN>-*.md)
	LandedBrief []byte // the brief as it lands on the target branch: its hash is the brief input
	ToolVersion string // the desk-tools version that wrote the record: the "tool" input
}

var (
	filesBlockStartRe = regexp.MustCompile(`(?m)^files:\s*$`)
	backtickSpanRe    = regexp.MustCompile("`([^`]+)`")
)

// BriefDeclaredFiles returns the distinct backtick-quoted spans inside a brief's `## Context`
// `files:` block, in first-seen order. The block runs from the bare `files:` line to the first
// blank line. Every span is a candidate path; one that does not exist at the record's sha (a
// planned file, a placeholder such as `<slug>`) simply resolves absent. No `files:` line is an
// empty set, never an error.
func BriefDeclaredFiles(content string) []string {
	loc := filesBlockStartRe.FindStringIndex(content)
	if loc == nil {
		return nil
	}
	block := content[loc[1]:]
	if end := strings.Index(block, "\n\n"); end >= 0 {
		block = block[:end]
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range backtickSpanRe.FindAllStringSubmatch(block, -1) {
		p := strings.TrimSpace(m[1])
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// checkNonPassBlocker refuses a non-pass record whose caller-supplied blocker fields cannot
// hold it: blocker_kind outside the closed set, or a blocker_ref that is not a reference.
func checkNonPassBlocker(r WakeReceipt) error {
	if !blockerKinds[r.BlockerKind] {
		return Refused("refused: blocker_kind " + strconv.Quote(r.BlockerKind) + " is not one of " +
			"implementation, check-definition, human-action, environment, unknown — a " + r.Outcome +
			" record names its blocker class")
	}
	_, err := ParseBlockerRef(r.BlockerRef, "", "")
	return err
}

// BuildOutcomeRecord returns the record bytes to land for raw (one JSON verify-outcome record).
// A pass record is returned unchanged. A verify-fail or blocked record gets a complete
// verify-wake-v1 receipt:
//
//   - inputs: one file:<path> key per declared path that exists at the record's sha (every
//     file under a declared directory), read from the brief AT that sha; plus
//     file:<brief-path> hashed as the brief lands; plus tool = in.ToolVersion;
//   - wake_predicate relevant-input-changed, wake_schema, a stable receipt_id, tool_version.
//
// blocker_kind and blocker_ref are the caller's, carried in raw, and required: an unknown kind
// or a reference that is not #<N>, <owner>/<repo>#<N> or a forge URL is refused, naming the
// field. Every other key in raw travels through unchanged. A commit that cannot be read is
// could-not-check; every other gap is a refusal naming the field.
func BuildOutcomeRecord(raw []byte, in OutcomeReceiptInput, tree OutcomeTree) ([]byte, error) {
	line := bytes.TrimSpace(raw)
	var r WakeReceipt
	if err := json.Unmarshal(line, &r); err != nil {
		return nil, Refused("refused: invalid verify-outcome record JSON: " + err.Error())
	}
	if !r.IsFailedOrBlocked() {
		return append([]byte{}, line...), nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(line, &fields); err != nil {
		return nil, Refused("refused: invalid verify-outcome record JSON: " + err.Error())
	}
	if err := checkNonPassBlocker(r); err != nil {
		return nil, err
	}
	sha := strings.TrimSpace(r.SHA)
	if sha == "" {
		return nil, Refused("refused: sha is empty — the receipt's inputs are read at the record's sha")
	}
	if strings.TrimSpace(in.BriefPath) == "" || len(in.LandedBrief) == 0 {
		return nil, Refused("refused: brief — the brief as it lands is required to hash its revision")
	}
	if strings.TrimSpace(in.ToolVersion) == "" {
		return nil, Refused("refused: tool_version is empty")
	}
	if tree == nil {
		return nil, Unverifiable("could-not-check: no reader for the tree at "+sha, nil)
	}

	briefAtSHA, err := tree.ReadFile(in.BriefPath)
	if err != nil {
		return nil, Unverifiable("could-not-check: inputs — cannot read "+in.BriefPath+" at "+sha, err)
	}
	inputs := map[string]string{}
	for _, decl := range BriefDeclaredFiles(string(briefAtSHA)) {
		p := strings.TrimSuffix(decl, "/")
		if p == "" {
			continue
		}
		kind, kerr := tree.PathKind(p)
		if kerr != nil {
			return nil, Unverifiable("could-not-check: inputs — cannot resolve "+p+" at "+sha, kerr)
		}
		var files []string
		switch kind {
		case "":
			continue // absent at this sha: a planned file or a placeholder, never an input
		case "blob":
			files = []string{p}
		case "tree":
			if files, err = tree.ListFiles(p); err != nil {
				return nil, Unverifiable("could-not-check: inputs — cannot list "+p+" at "+sha, err)
			}
			if len(files) == 0 {
				return nil, Refused("refused: inputs — declared directory " + p + " holds no file at " + sha)
			}
		default:
			return nil, Refused("refused: inputs — declared path " + p + " is a " + kind + " at " + sha)
		}
		for _, f := range files {
			b, rerr := tree.ReadFile(f)
			if rerr != nil {
				return nil, Unverifiable("could-not-check: inputs — cannot read "+f+" at "+sha, rerr)
			}
			inputs[inputKeyFilePrefix+f] = Sha256Hex(b)
		}
	}
	inputs[inputKeyFilePrefix+in.BriefPath] = Sha256Hex(in.LandedBrief)
	inputs[inputKeyTool] = in.ToolVersion

	id := Sha256Hex([]byte(strings.Join([]string{r.Brief, sha, r.TS, r.Outcome, r.BlockerKind, r.BlockerRef}, "\x00")))
	r.Schema, r.ID, r.Inputs, r.ToolVersion, r.WakePredicate = SchemaWakeV1, "vw1-"+id[:16], inputs, in.ToolVersion, WakeRelevantInputChanged
	if !r.Complete() {
		return nil, Refused("refused: the derived receipt for " + r.Brief + " is incomplete")
	}
	for k, v := range map[string]any{
		"wake_schema": r.Schema, "receipt_id": r.ID, "inputs": r.Inputs,
		"tool_version": r.ToolVersion, "wake_predicate": r.WakePredicate,
	} {
		b, merr := json.Marshal(v)
		if merr != nil {
			return nil, merr
		}
		fields[k] = b
	}
	return json.Marshal(fields)
}
