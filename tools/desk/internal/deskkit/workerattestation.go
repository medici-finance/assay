package deskkit

// workerattestation.go — the pre-work attestation for a FRESH worker dispatch (#3056).
//
// A fresh worker dispatch (a brief or an issue, no --pr) has no change to stamp: the PR does not
// exist until the worker opens it, so its model stamp is PENDING until the coordinator runs
// --stamp-only after the PR opens. A driver that admits a worker only on a SUCCESSFUL, read-back
// stamp receipt before any work is spent can therefore never admit one. This file gives that
// dispatch the same shape the verifier kit already has (verifierattestation.go): a typed,
// dispatcher-authored issue whose immutable body binds the run, stamped with the AUTHORING pair
// (authored-model:/authored-tier:) under dispatcher authority, closed, and read back before the
// prompt is emitted.
//
// What the record is NOT: a stamp on the change. The PR's own authoring stamp is still applied by
// --stamp-only after the PR opens, from the coordinator's explicit selection; the attestation only
// lets that call REFUSE a selection, branch or repository that disagrees with the admitted run. A
// model is never inferred from the record. The record carries no claim of execution surveillance.

import (
	"github.com/medici-finance/assay/deskcore/attestation"

	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// WorkerAttestationTitle is the reserved title prefix of a worker pre-work attestation record. It
// is followed by the run identifier, exactly as the verifier record's title is.
const WorkerAttestationTitle = "[worker-attestation] "

// WorkerAttestationLinePrefix starts the PR-body line that binds an opened change to the
// attestation record its worker was admitted under.
const WorkerAttestationLinePrefix = "Worker-Attestation:"

// workerAgentRole is the roster role whose actor must differ from the dispatcher's for a worker
// attestation to be authority-bearing: the worker App never becomes stamp authority.
const workerAgentRole = "worker"

// WorkerBinding identifies one selected worker run against its source. It is dispatch
// provenance, NEVER evidence of what the worker did. Exactly one of Brief or SourceIssue names
// the specification the run was handed.
type WorkerBinding struct {
	Version int
	// Run is a 64-hex digest of a fresh random nonce: a re-dispatch is a different run.
	Run string
	// Repo is the owner/name the claim, the worktree branch and the PR live in.
	Repo string
	// Item is the dispatch claim key (the item as the claim names it).
	Item string
	// Kit is the dispatched worker kit (worker or worker-objective).
	Kit string
	// Branch and Base are the worktree's branch and its commit at admission.
	Branch, Base string
	// Brief is the specification's path relative to its own repository root, and BriefSHA256
	// the digest of the bytes the prompt names. Empty for an issue item.
	Brief, BriefSHA256 string
	// SourceIssue is the tracked issue of an issue-<N> item, and SourceIssueSHA256 the digest of
	// its body as read at admission. Zero for a brief item.
	SourceIssue       int
	SourceIssueSHA256 string
	// Model and Tier are the dispatcher's explicit selection, normalised by the authored-pair
	// reader, so the record and the stamp it carries spell them the same way.
	Model, Tier string
}

// WorkerReceipt is a read-back, closed and stamped worker attestation record.
type WorkerReceipt struct {
	Binding WorkerBinding
	Issue   int
	URL     string
}

// PRBinding is the line the worker puts in its PR body. --stamp-only reads it back.
func (r WorkerReceipt) PRBinding() string {
	return fmt.Sprintf("%s %s#%d run=%s", WorkerAttestationLinePrefix, r.Binding.Repo, r.Issue, r.Binding.Run)
}

// Receipt is the one-line admission summary the dispatch step prints.
func (r WorkerReceipt) Receipt() string {
	b := r.Binding
	source := "brief=" + b.Brief
	if b.SourceIssue > 0 {
		source = fmt.Sprintf("issue=%d", b.SourceIssue)
	}
	return fmt.Sprintf("%s item=%s branch=%s base=%s %s model=%s tier=%s kit=%s",
		r.PRBinding(), b.Item, b.Branch, b.Base, source, b.Model, b.Tier, b.Kit)
}

// attestationAuthority is the dispatcher authority shared by every pre-work attestation record:
// the dispatcher (desk) and the dispatched agent's role are both bound, are distinct actors, and
// login is the dispatcher. A configuration binding the two to one actor has no independent
// stamp authority, so it refuses rather than let an agent attest its own run.
func attestationAuthority(agentRole string) func(string) bool {
	return func(login string) bool {
		desk, ok := RoleAppLogin(DispatcherRole)
		agent, bound := RoleAppLogin(agentRole)
		return ok && bound && !SameActor(desk, agent) && SameActor(login, desk)
	}
}

// workerAuthority is attestationAuthority for the worker role.
func workerAuthority(login string) bool { return attestationAuthority(workerAgentRole)(login) }

// WorkerAttestationAuthorityReady reports whether the roster can carry a worker attestation at
// all: desk and worker bound to distinct actors. Checked before any durable state exists.
func WorkerAttestationAuthorityReady() error {
	desk, ok := RoleAppLogin(DispatcherRole)
	if !ok || !workerAuthority(desk) {
		return Refused("worker pre-work attestation requires distinct configured desk and worker actors")
	}
	return nil
}

// IsWorkerAttestation reports whether an issue is a worker attestation record: the reserved title
// AND authored by the bound dispatcher. A look-alike title by anyone else stays ordinary work.
func IsWorkerAttestation(title, author string) bool {
	return strings.HasPrefix(title, WorkerAttestationTitle) && workerAuthority(author)
}

// IsAttestationRecord is the single listing policy for every dispatcher-owned pre-work record:
// neither is open work, so no intake, board or sweep lists it.
func IsAttestationRecord(title, author string) bool {
	return IsVerifierAttestation(title, author) || IsWorkerAttestation(title, author)
}

func workerBody(b WorkerBinding) string {
	data, _ := json.MarshalIndent(b, "", "  ")
	return "Dispatcher-owned pre-work attestation for a worker run. This record contains NO work result and is not a stamp on any change.\n\n" + string(data) + "\n"
}

var workerHexCommit = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
var workerHex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// validate checks a binding's shape. It never reads the forge.
func (b WorkerBinding) validate() error {
	if b.Version != 1 || !workerHex64.MatchString(b.Run) || !ValidRepoSlug(b.Repo) || strings.TrimSpace(b.Item) == "" ||
		strings.TrimSpace(b.Branch) == "" || b.Branch == "HEAD" || !workerHexCommit.MatchString(b.Base) {
		return Refused("invalid worker attestation binding")
	}
	if b.Kit != "worker" && b.Kit != "worker-objective" {
		return Refused("worker attestation binds only a worker kit")
	}
	hasBrief := b.Brief != "" && workerHex64.MatchString(b.BriefSHA256)
	hasIssue := b.SourceIssue > 0 && workerHex64.MatchString(b.SourceIssueSHA256)
	if hasBrief == hasIssue || (b.Brief == "") != (b.BriefSHA256 == "") || (b.SourceIssue == 0) != (b.SourceIssueSHA256 == "") {
		return Refused("worker attestation binds exactly one source: a brief or an issue")
	}
	if filepath.IsAbs(b.Brief) || strings.HasPrefix(b.Brief, "../") || strings.Contains(b.Brief, `\`) {
		return Refused("worker attestation brief must be repository-relative")
	}
	stamp, state := StampAuthored.StampOf(mustLabels(StampAuthored.Labels(b.Model, b.Tier)))
	if state != ModelStamped || stamp.Model != b.Model || stamp.Tier != b.Tier {
		return Refused("worker attestation model/tier is not a normalised authoring selection")
	}
	return nil
}

func mustLabels(labels []string, err error) []string {
	if err != nil {
		return nil
	}
	return labels
}

// WorkerBindingInput is what the dispatch knows at admission time.
type WorkerBindingInput struct {
	Repo, Item, Kit, Branch string
	// Home is the allocated worktree; its HEAD and branch are read, never supplied.
	Home string
	// BriefPath is the specification file the prompt names (absolute), or "" for an issue item.
	BriefPath string
	// SourceIssue is the issue-<N> item's tracked issue, or 0.
	SourceIssue int
	Model, Tier string
}

// PrepareWorkerBinding derives a binding from the allocated worktree and the source, with a fresh
// run identifier. An issue source is read through f (the dispatcher's forge) so the record binds
// the body the worker was handed. Nothing is written.
func PrepareWorkerBinding(f Forge, in WorkerBindingInput) (WorkerBinding, error) {
	var b WorkerBinding
	if err := WorkerAttestationAuthorityReady(); err != nil {
		return b, err
	}
	if !IsAllowedRepo(in.Repo) {
		return b, Refused("worker attestation repo outside configured repo set")
	}
	labels, err := StampAuthored.Labels(in.Model, in.Tier)
	if err != nil {
		return b, Refused(err.Error())
	}
	stamp, _ := StampAuthored.StampOf(labels)
	head, err := verifierGit(in.Home, "rev-parse", "HEAD")
	if err != nil {
		return b, err
	}
	branch, err := verifierGit(in.Home, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return b, err
	}
	if branch != in.Branch {
		return b, Refused(fmt.Sprintf("worker worktree is on %q, not the dispatched branch %q", branch, in.Branch))
	}
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		return b, Unverifiable("cannot mint worker attestation run identifier", err)
	}
	b = WorkerBinding{Version: 1, Run: verifierDigest(nonce), Repo: in.Repo, Item: in.Item, Kit: in.Kit,
		Branch: branch, Base: head, Model: stamp.Model, Tier: stamp.Tier}
	switch {
	case in.BriefPath != "" && in.SourceIssue == 0:
		raw, err := os.ReadFile(in.BriefPath)
		if err != nil {
			return b, Unverifiable("cannot read the worker brief", err)
		}
		rel, err := repoRelative(in.BriefPath)
		if err != nil {
			return b, err
		}
		b.Brief, b.BriefSHA256 = rel, verifierDigest(raw)
	case in.BriefPath == "" && in.SourceIssue > 0:
		repo, err := verifierRepo(in.Repo)
		if err != nil {
			return b, err
		}
		issue, err := f.GetIssueTyped(repo, in.SourceIssue, TargetIssue)
		if err != nil || issue == nil {
			return b, Unverifiable("cannot read the worker's source issue", err)
		}
		if issue.IsPullRequest {
			return b, Refused("worker source issue is a change, not an issue")
		}
		b.SourceIssue, b.SourceIssueSHA256 = in.SourceIssue, verifierDigest([]byte(issue.Body))
	default:
		return b, Refused("worker attestation binds exactly one source: --brief or an issue-<N> item")
	}
	return b, b.validate()
}

// repoRelative spells an absolute file path relative to the root of the repository holding it,
// so a record never carries a local absolute path.
func repoRelative(abs string) (string, error) {
	dir, base := filepath.Split(abs)
	prefix, err := verifierGit(dir, "rev-parse", "--show-prefix")
	if err != nil {
		return "", err
	}
	rel := filepath.ToSlash(filepath.Join(prefix, base))
	if rel == "" || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) {
		return "", Refused("worker brief is not inside a repository")
	}
	return rel, nil
}

// attestationRecord reads an attestation record and checks it is exactly the dispatcher-authored,
// never-edited record for this title and body.
func attestationRecord(f Forge, repo ForgeRepo, n int, title, body string, authority func(string) bool, what string) (*Issue, error) {
	issue, err := f.GetIssueTyped(repo, n, TargetIssue)
	if err != nil {
		return nil, Unverifiable("cannot read "+what+" attestation target", err)
	}
	return attestationRecordObserved(f, repo, n, issue, title, body, authority, what)
}

// attestationTarget checks a typed observation without silently fetching a different one.
func attestationTarget(issue *Issue, title, body string, authority func(string) bool, what string) error {
	if issue == nil {
		return Refused(what + " target is not the exact dispatcher-authored run record")
	}
	if why := attestation.TargetProblem(true, issue.IsPullRequest, authority(issue.Author.Login), issue.Title, issue.Body, title, body); why != "" {
		return Refused(what + " target is not the exact dispatcher-authored run record")
	}
	return nil
}

type attestationRecordReader interface {
	IssueTrustEvents(ForgeRepo, int) (*TrustPayload, error)
}

func attestationRecordObserved(f attestationRecordReader, repo ForgeRepo, n int, issue *Issue, title, body string, authority func(string) bool, what string) (*Issue, error) {
	if err := attestationTarget(issue, title, body, authority, what); err != nil {
		return nil, err
	}
	trust, err := f.IssueTrustEvents(repo, n)
	if err != nil {
		return nil, Unverifiable("cannot establish immutable "+what+" record", err)
	}
	if trust == nil || attestation.HistoryProblem(trust.BodyHistoryKnown, trust.Complete, !trust.BodyEdited.IsZero()) != "" {
		return nil, Refused(what + " record edited or its history is incomplete")
	}
	return issue, nil
}

// IssueWorkerAttestation creates the record, stamps it with the authoring pair under dispatcher
// authority, closes it and reads it back. Any failure returns an error and the caller emits no
// prompt. A record left behind by a failure stays OPEN: an open record never binds a PR, and the
// listing policy keeps it out of every work queue until it is reconciled.
func IssueWorkerAttestation(f Forge, b WorkerBinding) (WorkerReceipt, error) {
	r := WorkerReceipt{Binding: b}
	if err := b.validate(); err != nil {
		return r, err
	}
	if err := WorkerAttestationAuthorityReady(); err != nil {
		return r, err
	}
	repo, err := verifierRepo(b.Repo)
	if err != nil {
		return r, err
	}
	ref, err := f.FileIssue(repo, IssueInput{Title: WorkerAttestationTitle + b.Run, Body: workerBody(b)})
	if err != nil {
		return r, Unverifiable("worker attestation create failed or its outcome is unknown; this run is abandoned, dispatch a new run", err)
	}
	if ref == nil || ref.Number <= 0 {
		return r, Unverifiable("worker attestation create returned no target", nil)
	}
	r.Issue, r.URL = ref.Number, ref.URL
	if _, err = attestationRecord(f, repo, r.Issue, WorkerAttestationTitle+b.Run, workerBody(b), workerAuthority, "worker"); err != nil {
		return r, err
	}
	labels, _ := StampAuthored.Labels(b.Model, b.Tier)
	if err = ApplyVerifiedStamp(f, repo, r.Issue, TargetIssue, StampAuthored, labels, workerAuthority); err != nil {
		return r, err
	}
	if err = f.CloseIssueTyped(repo, r.Issue, TargetIssue, "completed"); err != nil {
		return r, Unverifiable("cannot close worker attestation record", err)
	}
	got, err := ReadWorkerAttestation(f, repo, r.Issue)
	if err != nil {
		return r, err
	}
	if got.Binding != b {
		return r, Refused("worker attestation readback binds a different run")
	}
	if got.URL == "" {
		got.URL = r.URL
	}
	return got, nil
}

// ReadWorkerAttestation reads record n back and admits it only when it is the exact,
// dispatcher-authored, never-edited, CLOSED record carrying the authoring pair its body binds,
// applied under dispatcher authority. The binding is recovered from the body and re-rendered, so
// a body that does not round-trip refuses.
func ReadWorkerAttestation(f Forge, repo ForgeRepo, n int) (WorkerReceipt, error) {
	r := WorkerReceipt{Issue: n}
	if n <= 0 {
		return r, Refused("worker attestation number must be positive")
	}
	issue, err := f.GetIssueTyped(repo, n, TargetIssue)
	if err != nil {
		return r, Unverifiable("cannot read worker attestation target", err)
	}
	if issue == nil || !strings.HasPrefix(issue.Title, WorkerAttestationTitle) {
		return r, Refused("target is not a worker attestation record")
	}
	_, raw, ok := strings.Cut(issue.Body, "\n\n")
	if !ok {
		return r, Refused("worker attestation body is not a binding")
	}
	var b WorkerBinding
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return r, Refused("worker attestation body is not a binding")
	}
	r.Binding = b
	if err := b.validate(); err != nil {
		return r, err
	}
	if b.Repo != repo.Slug() {
		return r, Refused("worker attestation belongs to a different repository")
	}
	issue, err = attestationRecord(f, repo, n, WorkerAttestationTitle+b.Run, workerBody(b), workerAuthority, "worker")
	if err != nil {
		return r, err
	}
	r.URL = issue.URL
	if issue.State != "closed" {
		return r, Refused("worker pre-work attestation has not completed (record still open)")
	}
	tl, err := StampTimelineFor(f, repo, n, TargetIssue)
	if err != nil {
		return r, err
	}
	stamp, state := StampAuthored.AttestedStampOf(tl, workerAuthority)
	if state != ModelStamped || stamp.Model != b.Model || stamp.Tier != b.Tier {
		return r, Refused("missing or mismatched dispatcher authoring stamp on the worker attestation")
	}
	return r, nil
}

// WorkerAttestationRef is a parsed PR-body binding line.
type WorkerAttestationRef struct {
	Repo  string
	Issue int
	Run   string
}

var workerLineRe = regexp.MustCompile(`^Worker-Attestation:\s+([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#([1-9][0-9]*)\s+run=([0-9a-f]{64})\s*$`)

// ParseWorkerAttestationLine finds the PR body's binding line. No line is (zero, false, nil); a
// malformed line or more than one line refuses, so an ambiguous body never binds silently.
func ParseWorkerAttestationLine(body string) (WorkerAttestationRef, bool, error) {
	var ref WorkerAttestationRef
	found := false
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if !strings.HasPrefix(line, WorkerAttestationLinePrefix) {
			continue
		}
		m := workerLineRe.FindStringSubmatch(line)
		if m == nil {
			return ref, false, Refused("malformed " + WorkerAttestationLinePrefix + " line in the change description")
		}
		if found {
			return ref, false, Refused("more than one " + WorkerAttestationLinePrefix + " line in the change description")
		}
		n, _ := strconv.Atoi(m[2])
		ref, found = WorkerAttestationRef{Repo: m[1], Issue: n, Run: m[3]}, true
	}
	return ref, found, nil
}

// CheckWorkerChangeBinding refuses a post-open stamp whose selection, repository, kit or source
// branch disagrees with the admitted run. It never supplies a value: every argument is the
// coordinator's own explicit selection or the change's own read.
func CheckWorkerChangeBinding(r WorkerReceipt, repo, kit, model, tier, headRef string) error {
	b := r.Binding
	labels, err := StampAuthored.Labels(model, tier)
	if err != nil {
		return Refused(err.Error())
	}
	want, _ := StampAuthored.StampOf(labels)
	var diffs []string
	if b.Repo != repo {
		diffs = append(diffs, fmt.Sprintf("repo %s != attested %s", repo, b.Repo))
	}
	if b.Kit != kit {
		diffs = append(diffs, fmt.Sprintf("kit %s != attested %s", kit, b.Kit))
	}
	if want.Model != b.Model || want.Tier != b.Tier {
		diffs = append(diffs, fmt.Sprintf("selection %s/%s != attested %s/%s", want.Model, want.Tier, b.Model, b.Tier))
	}
	if headRef != b.Branch {
		diffs = append(diffs, fmt.Sprintf("change branch %q != attested %q", headRef, b.Branch))
	}
	if len(diffs) > 0 {
		return Refused("stamp-only: the change disagrees with its worker attestation " +
			fmt.Sprintf("%s#%d: %s; nothing written", b.Repo, r.Issue, strings.Join(diffs, "; ")))
	}
	return nil
}
