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
// brief (LatestPerBrief). It touches no forge and no git, so it is tested with neither.
//
// SHARED PROTOCOL. The codec, naming rule, per-brief reduction and the
// reader itself are NOT defined here: they live once, in statusgen/evaluation (outcome.go),
// which tools/desk already links through its replace directive, and statusgen uses the same
// code. What this file keeps is the adapter surface callers already import by its old names:
// one-line delegations, a root-to-fs.FS source adapter, and a wall-clock adapter. A structural
// test in each module (the choke-point guard) pins that module's single reader, and
// TestOutcomeProtocolClassGuard pins that no second copy of the protocol's rules reappears.
//
// THE TRANSITION WINDOW. The legacy log is frozen (deskevidence refuses ever appending to it
// again, #882 Task step 5) but stays on disk, unretired, until every open PR that
// still touches it has landed (Task step 8). ReadVerifyOutcomes reads BOTH layouts for as long as
// that window is open, and dedupes by content digest so a migrated record and its still-present
// source log line count once.

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
	"github.com/medici-finance/assay/statusgen/evaluation/outcomerecord"
)

// OutcomeRecordsDir is the repo-relative directory verify-outcome records live under, one file
// per outcome, per-stream subdirectory (docs/streams/verify-outcomes/<stream>/*.json).
const OutcomeRecordsDir = outcomerecord.OutcomeRecordsDir

// MaxClockSkew bounds how far ahead of "now" a verify-outcome record's `ts` may be before it is
// treated as untrustworthy rather than merely "the newest" (#1803 SR-1803-2). The writer
// (cmdOutcomeRecordWrite) refuses to land a record whose `ts` is more than MaxClockSkew ahead
// of its own clock; the shared reduction additionally never lets a future-dated record already
// on disk win the comparison.
const MaxClockSkew = outcomerecord.MaxClockSkew

// OutcomeRecord is one verify outcome (the shared outcomerecord.Record). Raw is the CANONICAL
// line — the record's single JSON object, trimmed of surrounding whitespace, with NO trailing
// newline.
type OutcomeRecord = outcomerecord.Record

// ParseRecord parses one verify-outcome record's raw bytes; see outcomerecord.ParseRecord.
func ParseRecord(raw []byte) (OutcomeRecord, error) { return outcomerecord.ParseRecord(raw) }

// RecordDigest is the first 12 hex digits of the SHA-256 of canonicalLine plus a newline.
func RecordDigest(canonicalLine []byte) string { return outcomerecord.RecordDigest(canonicalLine) }

// CanonicalBytes returns the exact bytes a record file holds for canonicalLine.
func CanonicalBytes(canonicalLine []byte) []byte { return outcomerecord.CanonicalBytes(canonicalLine) }

// SplitBriefKey splits a "<stream>/<NN>" brief key, refusing any other shape.
func SplitBriefKey(brief string) (stream, num string, err error) {
	return evaluation.SplitBriefKey(brief)
}

// RecordName computes the repo-relative path a verify-outcome record's bytes land at; a pure
// function of the bytes (see outcomerecord.RecordName).
func RecordName(recordBytes []byte) (string, error) { return outcomerecord.RecordName(recordBytes) }

// FutureTS reports whether ts lands more than MaxClockSkew ahead of now.
func FutureTS(ts string, now time.Time) bool { return outcomerecord.FutureTS(ts, now) }

// ReadVerifyOutcomes reads every verify-outcome record under root, both layouts, deduped by
// digest. Three-state (common-clause C4): absent history is an empty set; an unreadable file or
// directory is an error (could-not-check); the legacy log keeps its tolerance for a malformed
// LINE. The rules live in outcomerecord.ReadRecords; this is the filesystem-root adapter.
func ReadVerifyOutcomes(root string) ([]OutcomeRecord, error) {
	fsRoot := root
	if fsRoot == "" {
		fsRoot = "."
	}
	records, _, err := outcomerecord.ReadRecords(os.DirFS(fsRoot), root)
	return records, err
}

// LatestPerBrief reduces records to one winner per brief key at the current wall clock; see
// outcomerecord.LatestPerBrief for the rules (newest ts, name tie-break, future-ts exclusion).
func LatestPerBrief(records []OutcomeRecord) (latest map[string]OutcomeRecord, futureByBrief map[string]bool) {
	return outcomerecord.LatestPerBrief(records, time.Now())
}

// LatestPerBriefAt is LatestPerBrief with an explicit "now", so a test never depends on
// wall-clock time.
func LatestPerBriefAt(records []OutcomeRecord, now time.Time) (latest map[string]OutcomeRecord, futureByBrief map[string]bool) {
	return outcomerecord.LatestPerBrief(records, now)
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
	// Optional cross-repository observation; the whole immutable deliverable
	// commit is a conservative input scope, alongside the tracking file keys.
	SourceRepo, Source string
	BriefPath          string // the brief's repo-relative path (docs/streams/<stream>/brief-<NN>-*.md)
	LandedBrief        []byte // the brief as it lands on the target branch: its hash is the brief input
	ToolVersion        string // the desk-tools version that wrote the record: the "tool" input
	// Repo is the landing repository ("owner/name"). It is stamped as the record's repo key
	// when the record carries none, so a bare #N blocker_ref resolves against the repository the
	// record lands in — the same repository the writer's own reference check resolved it in.
	Repo string
	// SiblingSources is the attested source commit of each further sibling
	// repository the brief declares, by owner/repo (#4876): one repo-source key
	// each. It only ever comes from the run's attestation binding.
	SiblingSources map[string]string
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

// CheckSuppliedReceipt is the completeness gate for a non-pass record that arrives already
// carrying a verify-wake-v1 receipt (declared decision 3: validated as written, never
// re-derived). It refuses — exit 5, naming the first missing or invalid field — exactly the
// records BuildOutcomeRecord would refuse to produce, so the pass-through can never land a
// receipt the planner then reads as incomplete. A pass record, and a record with no
// wake_schema, are not its concern (nil).
func CheckSuppliedReceipt(r WakeReceipt) error {
	if !r.IsFailedOrBlocked() || r.Schema != SchemaWakeV1 {
		return nil
	}
	named := func(field, why string) error {
		return Refused("refused: " + field + " — " + why + " (a " + r.Outcome +
			" record carrying wake_schema " + SchemaWakeV1 + " lands only with a complete receipt)")
	}
	if strings.TrimSpace(r.ID) == "" {
		return named("receipt_id", "empty")
	}
	if err := checkNonPassBlocker(r); err != nil {
		return err
	}
	if !wakePredicates[r.WakePredicate] {
		return named("wake_predicate", strconv.Quote(r.WakePredicate)+" is not a wake predicate")
	}
	switch r.WakePredicate {
	case WakeRelevantInputChanged:
		if len(r.Inputs) == 0 {
			return named("inputs", "empty, so unchangedness cannot be established")
		}
	case WakeDeadlineReached:
		if strings.TrimSpace(r.Deadline) == "" {
			return named("deadline", "empty for "+WakeDeadlineReached)
		}
	case WakeExplicitRecheck:
		if strings.TrimSpace(r.RecheckReason) == "" {
			return named("recheck_reason", "empty for "+WakeExplicitRecheck)
		}
	}
	if !r.Complete() {
		return named("wake_schema", "the receipt is incomplete")
	}
	return nil
}

// BuildOutcomeRecord returns the record bytes to land for raw (one JSON verify-outcome record).
// A pass record is returned unchanged. A verify-fail or blocked record gets a complete
// verify-wake-v1 receipt:
//
//   - inputs: one scope: membership key per declaration and one file:<path> key per declared
//     file that exists at the record's sha, read from the brief AT that sha; a declared
//     directory is one file:<dir>/ key over every member (DirInputDigest) unless it holds the
//     brief; plus file:<brief-path> hashed as the brief lands; plus tool = in.ToolVersion;
//   - wake_predicate relevant-input-changed, wake_schema, a stable receipt_id, tool_version;
//   - repo = in.Repo (the landing repository) when the record names none, so the planner reads
//     a bare #N blocker_ref in the repository the record lands in.
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
	declarations, err := BriefDeclaredFiles(string(briefAtSHA))
	if err != nil {
		return nil, err
	}
	// A ../<sibling>/ declaration is never read from this tree; it is covered
	// below by that sibling's own attested source commit, or refused.
	local, siblings := SplitSiblingDeclarations(declarations)
	scope, err := ResolveDeclaredScope(local, tree)
	if err != nil {
		return nil, err
	}
	inputs := scope.Shapes
	digests := map[string]string{}
	digestOf := func(f string) (string, error) {
		if d, ok := digests[f]; ok {
			return d, nil
		}
		b, rerr := tree.ReadFile(f)
		if rerr != nil {
			return "", Unverifiable("could-not-check: inputs — cannot read "+f+" at "+sha, rerr)
		}
		digests[f] = Sha256Hex(b)
		return digests[f], nil
	}
	// #4865: a declared directory's members are covered by ONE file:<dir>/ digest over
	// their paths and content, never by one file:<member> key each, so no member name can
	// put a 64-hex digest after a credential-shaped identifier. A directory holding the
	// brief keeps one key per member: the brief's own file: key is the one input a pending
	// landing substitutes, and the brief's bytes must not hide inside a directory digest.
	folded := map[string]bool{}
	for dir, members := range scope.Dirs {
		holdsBrief := false
		for _, f := range members {
			holdsBrief = holdsBrief || f == in.BriefPath
		}
		if holdsBrief {
			continue
		}
		d, derr := DirInputDigest(members, digestOf)
		if derr != nil {
			return nil, derr
		}
		inputs[DirInputKey(dir)] = d
		for _, f := range members {
			folded[f] = true
		}
	}
	for _, f := range scope.Files {
		if folded[f] {
			continue
		}
		d, derr := digestOf(f)
		if derr != nil {
			return nil, derr
		}
		inputs[inputKeyFilePrefix+f] = d
	}
	inputs[inputKeyFilePrefix+in.BriefPath] = Sha256Hex(in.LandedBrief)
	inputs[inputKeyTool] = in.ToolVersion
	if in.SourceRepo != "" || in.Source != "" {
		_, sourceErr := hex.DecodeString(in.Source)
		if !IsAllowedRepo(in.SourceRepo) || len(in.Source) != 40 || sourceErr != nil || strings.EqualFold(in.SourceRepo, in.Repo) {
			return nil, Refused("incomplete cross-repository outcome source")
		}
		inputs[InputKeyRepoSourcePrefix+in.SourceRepo] = in.Source
	}
	for repo, commit := range in.SiblingSources {
		_, commitErr := hex.DecodeString(commit)
		if !IsAllowedRepo(repo) || len(commit) != 40 || commitErr != nil || in.SourceRepo == "" ||
			strings.EqualFold(repo, in.Repo) || strings.EqualFold(repo, in.SourceRepo) || !declaresSibling(siblings, repo) {
			return nil, Refused("incomplete cross-repository sibling source " + repo)
		}
		inputs[InputKeyRepoSourcePrefix+repo] = commit
	}
	// A single-repository run holds no source commit for any sibling (#5097):
	// record each sibling declaration as declared and unbound, never as read.
	// A cross-repository run gets no marker, so its unbound sibling still refuses.
	if in.SourceRepo == "" && in.Source == "" && len(in.SiblingSources) == 0 {
		for _, decl := range siblings {
			inputs[InputKeyDeclaredPrefix+decl] = DeclaredUnbound
		}
	}
	if err := CheckSiblingCoverage(siblings, inputs); err != nil {
		return nil, err
	}

	id := Sha256Hex([]byte(strings.Join([]string{r.Brief, sha, r.TS, r.Outcome, r.BlockerKind, r.BlockerRef}, "\x00")))
	r.Schema, r.ID, r.Inputs, r.ToolVersion, r.WakePredicate = SchemaWakeV1, "vw1-"+id[:16], inputs, in.ToolVersion, WakeRelevantInputChanged
	if !r.Complete() {
		return nil, Refused("refused: the derived receipt for " + r.Brief + " is incomplete")
	}
	stamp := map[string]any{
		"wake_schema": r.Schema, "receipt_id": r.ID, "inputs": r.Inputs,
		"tool_version": r.ToolVersion, "wake_predicate": r.WakePredicate,
	}
	if strings.TrimSpace(r.Repo) == "" && strings.TrimSpace(in.Repo) != "" {
		stamp["repo"] = strings.TrimSpace(in.Repo)
	}
	for k, v := range stamp {
		b, merr := json.Marshal(v)
		if merr != nil {
			return nil, merr
		}
		fields[k] = b
	}
	return json.Marshal(fields)
}
