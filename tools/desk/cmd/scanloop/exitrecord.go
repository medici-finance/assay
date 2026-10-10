package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// exitrecord.go — the intake-exit-v1 record, issue-lane writer.
//
// The schema is spec/intake-exit-v1.md. statusgen writes the same record
// for the intake register, in another Go module that cannot import this one, so both carry a test
// (TestIntakeExitSchema_MatchesDoc) that pins their JSON keys and closed role set to that
// document's tables. Change the document and both writers together.
//
// The record is deliberately text-free. It has no key for a title, a body, a comment, an author or
// a model name, and Validate checks every identity-bearing value against a CLOSED set rather than a
// pattern: a pattern that admits a role slug also admits a person's login.

// intakeExitSchema is the record's schema tag.
const intakeExitSchema = "intake-exit-v1"

// intakeExitsFile is the record file's name inside the desk state directory. It is local state on
// the desk host and is never committed.
const intakeExitsFile = "intake-exits.jsonl"

// IntakeExitRecord is one triaged item's exit. The field order is the schema document's order, and
// every key is always written: an unknown value is an empty string, never an omitted key.
type IntakeExitRecord struct {
	Schema      string `json:"schema"`
	Source      string `json:"source"`
	Item        string `json:"item"`
	Repo        string `json:"repo"`
	Exit        Exit   `json:"exit"`
	Detail      string `json:"detail"`
	Artifact    string `json:"artifact"`
	DecidedBy   string `json:"decided_by"`
	TriagerRole string `json:"triager_role"`
	TriagerTier string `json:"triager_tier"`
	Opened      string `json:"opened"`
	Triaged     string `json:"triaged"`
	Kind        string `json:"kind"`
	Trust       string `json:"trust"`
	SessionTag  string `json:"session_tag"`
	DispatchRef string `json:"dispatch_ref"`
}

// Values of decided_by.
const (
	decidedMechanical = "mechanical"
	decidedJudgment   = "judgment"
)

// Values of triager_tier. A vendor model name is never one of them.
const (
	tierNone   = "none"
	tierAny    = "any"
	tierStrong = "strong"
)

// intakeExitRoles is the CLOSED role set: the five canonical desk loop names plus `driver`, the
// neutral token for a stamp a human wrote by hand. It is a literal here and in statusgen (which
// cannot import deskkit); both are pinned to the schema document's "Closed role set" table.
var intakeExitRoles = []string{"the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver"}

// classifierKinds are the reasons classify returns. A `kind` outside them is refused.
var classifierKinds = []string{kindNewIssue, kindUpdate, kindUnreadableState, kindNoScanTarget, kindOutsideBoundary}

// admissionStates are the trust gate's states, the only values `trust` may take on an issue record.
var admissionStates = []string{string(AdmissionAdmitted), string(AdmissionQuarantined), string(AdmissionCouldNotCheck)}

var (
	issueItemRe = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+)#[1-9][0-9]*$`)
	intakeIDRe  = regexp.MustCompile(`^I-[A-Za-z0-9][A-Za-z0-9-]*$`)
	repoRe      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`)
	sessionRe   = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

	// artifactRes are the typed refs this writer accepts. The register-only `<stream>` shape is
	// not among them: the issue lane never produces a bare stream, and admitting any slug here
	// would admit free text.
	artifactRes = []*regexp.Regexp{
		regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[0-9]{2,4}$`),                        // <stream>/<NN>
		regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+#[1-9][0-9]*$`), // owner/repo#N
		regexp.MustCompile(`^#[1-9][0-9]*$`),                                         // #N
		regexp.MustCompile(`^F-[A-Za-z0-9][A-Za-z0-9-]*$`),                           // F-<slug>
		regexp.MustCompile(`^scan-pr:[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`),     // a new scan PR
	}

	// dispatchRefRe is the dispatch_ref grammar (spec/intake-exit-v1.md), `<claim_key>@YYYYMMDDTHHMMSSZ.<12 hex>`.
	// This record does not know its own claim key, so the claim-key part is checked for the
	// shape claimKeyFor produces: item-key characters, always containing the `--` separator.
	dispatchRefRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*--[A-Za-z0-9._/-]+@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$`)
)

// scanPRArtifactSuffix is how the scan-carrier lane names a NEW draft scan PR whose number it did
// not read back. The audit line keeps that text; the record turns it into the typed scan-pr ref.
const scanPRArtifactSuffix = " (new draft scan PR)"

func memberOf(v string, set []string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// typedArtifact reports whether a is one of the typed refs.
func typedArtifact(a string) bool {
	for _, re := range artifactRes {
		if re.MatchString(a) {
			return true
		}
	}
	return false
}

// recordArtifact turns a lane's artifact string into the record's typed ref. Anything it does not
// recognise is passed through unchanged, so Validate refuses it rather than this guessing.
func recordArtifact(raw string) string {
	raw = strings.TrimSpace(raw)
	if repo, ok := strings.CutSuffix(raw, scanPRArtifactSuffix); ok && repoRe.MatchString(repo) {
		return "scan-pr:" + repo
	}
	return raw
}

// clipValue quotes at most n bytes of an offending value, so a long pasted string is not echoed.
func clipValue(v string, n int) string {
	if len(v) > n {
		v = v[:n] + "…"
	}
	return fmt.Sprintf("%q", v)
}

func refuseRecord(msg string) error {
	return deskkit.Refused("intake-exit-v1: " + msg)
}

// Validate refuses (deskkit.Refused, exit 5) a record that breaks the schema. It runs before
// anything is written.
func (r IntakeExitRecord) Validate() error {
	if r.Schema != intakeExitSchema {
		return refuseRecord("schema must be " + intakeExitSchema + ", got " + clipValue(r.Schema, 32))
	}
	switch r.Source {
	case "issue":
		m := issueItemRe.FindStringSubmatch(r.Item)
		if m == nil {
			return refuseRecord("an issue item must be owner/repo#N, got " + clipValue(r.Item, 64))
		}
		if r.Repo != m[1] {
			return refuseRecord("repo " + clipValue(r.Repo, 64) + " is not the item's repo " + m[1])
		}
	case "intake":
		if !intakeIDRe.MatchString(r.Item) {
			return refuseRecord("an intake item must be I-<slug>, got " + clipValue(r.Item, 64))
		}
		if r.Repo != "" && !repoRe.MatchString(r.Repo) {
			return refuseRecord("repo must be owner/repo, got " + clipValue(r.Repo, 64))
		}
		if r.Kind != "" || r.Trust != "" {
			return refuseRecord("kind and trust are issue-lane fields; an intake record leaves them empty")
		}
	default:
		return refuseRecord("source must be issue or intake, got " + clipValue(r.Source, 16))
	}

	if !r.Exit.Tracked() {
		return refuseRecord(clipValue(string(r.Exit), 32) + " is not a tracked exit; the exits are " +
			strings.Join(exitNames(), ", "))
	}
	if r.Exit == ExitRejectedWatching {
		if r.Detail != "rejected" && r.Detail != "watching" {
			return refuseRecord("exit rejected-watching needs detail rejected or watching, got " + clipValue(r.Detail, 16))
		}
	} else if r.Detail != "" {
		return refuseRecord("detail is only for exit rejected-watching, not " + string(r.Exit))
	}
	switch {
	case r.Artifact == "" && r.Exit != ExitRejectedWatching:
		return refuseRecord("exit " + string(r.Exit) + " needs an artifact: what the item became")
	case r.Artifact != "" && !typedArtifact(r.Artifact):
		return refuseRecord("artifact " + clipValue(r.Artifact, 64) + " is not a typed ref " +
			"(<stream>/<NN>, owner/repo#N, #N, F-<slug>, scan-pr:owner/repo)")
	}

	if !memberOf(r.TriagerRole, intakeExitRoles) {
		return refuseRecord("triager_role must be one of the closed role set (" +
			strings.Join(intakeExitRoles, ", ") + "); got " + clipValue(r.TriagerRole, len("pr-review-desk")))
	}
	switch r.DecidedBy {
	case decidedMechanical:
		if r.TriagerTier != tierNone {
			return refuseRecord("a mechanical record's triager_tier is none, got " + clipValue(r.TriagerTier, 16))
		}
	case decidedJudgment:
		if r.TriagerTier != tierAny && r.TriagerTier != tierStrong {
			return refuseRecord("a judgment record's triager_tier is any or strong, got " + clipValue(r.TriagerTier, 16))
		}
	default:
		return refuseRecord("decided_by must be mechanical or judgment, got " + clipValue(r.DecidedBy, 16))
	}

	triaged, err := time.Parse(time.RFC3339, r.Triaged)
	if err != nil {
		return refuseRecord("triaged must be RFC3339, got " + clipValue(r.Triaged, 40))
	}
	if r.Opened != "" {
		opened, err := time.Parse(time.RFC3339, r.Opened)
		if err != nil {
			return refuseRecord("opened must be RFC3339 or empty, got " + clipValue(r.Opened, 40))
		}
		if opened.After(triaged) {
			return refuseRecord("opened " + r.Opened + " is after triaged " + r.Triaged)
		}
	}

	if r.Source == "issue" {
		if r.Kind != "" && !memberOf(r.Kind, classifierKinds) {
			return refuseRecord("kind must be one of the classifier reasons (" + strings.Join(classifierKinds, ", ") +
				"); got " + clipValue(r.Kind, 40))
		}
		if r.Trust != "" && !memberOf(r.Trust, admissionStates) {
			return refuseRecord("trust must be an admission state (" + strings.Join(admissionStates, ", ") +
				"); got " + clipValue(r.Trust, 24))
		}
	}
	if r.SessionTag != "" && !sessionRe.MatchString(r.SessionTag) {
		return refuseRecord("session_tag has characters outside [A-Za-z0-9._:-] or is too long")
	}
	if r.DispatchRef != "" && !dispatchRefRe.MatchString(r.DispatchRef) {
		return refuseRecord("dispatch_ref does not parse as <claim_key>@YYYYMMDDTHHMMSSZ.<12 lowercase hex> " +
			"(spec/intake-exit-v1.md); got " + clipValue(r.DispatchRef, 64))
	}
	return nil
}

// recordSessionTag is the session tag the record carries: the audit log's own value, dropped to
// empty when it would not validate, so an odd DESK_SESSION never fails a drain.
func recordSessionTag() string {
	s := deskkit.SessionTag()
	if !sessionRe.MatchString(s) {
		return ""
	}
	return s
}

// intakeExitsPath is <desk state dir>/intake-exits.jsonl.
func intakeExitsPath() (string, error) {
	dir, err := deskkit.StateDir()
	if err != nil {
		return "", deskkit.Unverifiable("cannot resolve the desk state dir (HOME missing?)", err)
	}
	return filepath.Join(dir, intakeExitsFile), nil
}

// exitLockWait bounds how long a writer waits for the shared audit lock.
const exitLockWait = 60 * time.Second

// withAuditLock runs fn holding the desk audit lock (<state dir>/audit.lock), the same advisory
// lock every desk write takes, so a read-check-append cannot interleave with another writer's.
func withAuditLock(fn func(dir string) error) error {
	dir, err := deskkit.StateDir()
	if err != nil {
		return deskkit.Unverifiable("cannot resolve the desk state dir (HOME missing?)", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return deskkit.Unverifiable("cannot create the desk state dir", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return deskkit.Unverifiable("cannot open the audit lock", err)
	}
	defer f.Close()
	deadline := time.Now().Add(exitLockWait)
	for {
		lerr := deskkit.TryLockExclusive(f)
		if lerr == nil {
			break
		}
		if !errors.Is(lerr, deskkit.ErrLockBusy) {
			return deskkit.Unverifiable("cannot acquire the audit lock", lerr)
		}
		if time.Now().After(deadline) {
			return deskkit.Unverifiable(fmt.Sprintf("the audit lock was held for more than %s; "+
				"another desk write may be stuck; retry, or a human clears it", exitLockWait), nil)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer func() { _ = deskkit.UnlockFile(f) }()
	return fn(dir)
}

// readExitRecords reads every record in the file. An absent file is an empty record set; a line
// that does not parse is could-not-check (exit 6), because one-exit-per-item cannot be held over a
// file that cannot be read.
func readExitRecords(path string) ([]IntakeExitRecord, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read "+intakeExitsFile, err)
	}
	defer f.Close()
	var out []IntakeExitRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		var r IntakeExitRecord
		if err := json.Unmarshal([]byte(raw), &r); err != nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("%s line %d does not parse", intakeExitsFile, line), err)
		}
		out = append(out, r)
	}
	if err := sc.Err(); err != nil {
		return nil, deskkit.Unverifiable("cannot read "+intakeExitsFile, err)
	}
	return out, nil
}

// appendExitRecord validates r and appends it as one line, holding the audit lock across the
// read-check-append. It holds one exit per item across sessions: the same item with the same exit
// is a no-op (wrote=false), the same item with a DIFFERENT exit is refused and the file is left
// unchanged. A write failure is returned, never swallowed. afterWrite, when non-nil, runs under the
// same lock only after a line was written (the land verb writes its audit line there).
func appendExitRecord(r IntakeExitRecord, afterWrite func() error) (wrote bool, err error) {
	if err := r.Validate(); err != nil {
		return false, err
	}
	line, err := json.Marshal(r)
	if err != nil {
		return false, deskkit.Unverifiable("cannot encode the exit record", err)
	}
	err = withAuditLock(func(dir string) error {
		path := filepath.Join(dir, intakeExitsFile)
		existing, err := readExitRecords(path)
		if err != nil {
			return err
		}
		for _, prev := range existing {
			if prev.Item != r.Item {
				continue
			}
			if prev.Exit == r.Exit {
				return nil
			}
			return deskkit.Refused(fmt.Sprintf("%s already left by %s; refusing to also record %s. "+
				"One inbound item leaves by exactly one exit.", r.Item, prev.Exit, r.Exit))
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return deskkit.Unverifiable("cannot open "+intakeExitsFile, err)
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			f.Close()
			return deskkit.Unverifiable("cannot write "+intakeExitsFile, err)
		}
		if err := f.Close(); err != nil {
			return deskkit.Unverifiable("cannot close "+intakeExitsFile, err)
		}
		wrote = true
		if afterWrite != nil {
			return afterWrite()
		}
		return nil
	})
	return wrote, err
}

// memberMeta is what Land needs about one inbound item to write its record. It is captured when
// the item is classified, because the batched scan dispatch's payload does not carry it per member.
// It holds no title, body or author.
type memberMeta struct {
	Kind   string
	Trust  string
	Opened time.Time
}

// landRecord builds the issue-lane record for one landed member.
func landRecord(item string, exit Exit, lane LaneName, artifact, detail, execTier string, meta memberMeta, now time.Time) IntakeExitRecord {
	r := IntakeExitRecord{
		Schema:      intakeExitSchema,
		Source:      "issue",
		Item:        item,
		Repo:        repoOfItemID(item),
		Exit:        exit,
		Artifact:    recordArtifact(artifact),
		DecidedBy:   decidedMechanical,
		TriagerRole: LoopName,
		TriagerTier: tierNone,
		Triaged:     now.UTC().Format(time.RFC3339),
		Kind:        meta.Kind,
		Trust:       meta.Trust,
		SessionTag:  recordSessionTag(),
	}
	if lane == LaneRouting {
		// A routing result was fed back by a model tier: the exit is a judgment.
		r.DecidedBy = decidedJudgment
		r.TriagerTier = strings.TrimSpace(execTier)
	}
	if exit == ExitRejectedWatching {
		r.Detail = strings.TrimSpace(detail)
	}
	if !meta.Opened.IsZero() && !meta.Opened.After(now) {
		r.Opened = meta.Opened.UTC().Format(time.RFC3339)
	}
	return r
}
