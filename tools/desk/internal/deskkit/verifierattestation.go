package deskkit

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const VerifierAttestationTitle = "[verification-attestation] "
const verifierRecordName = "assay-verifier-attestation.json"

// VerifierAttestationRecord is the file name of a verifier home's local admission
// record, kept in the home's worktree admin dir. deskwt prune reads it, by this one
// name, to hold an admitted home until its Evidence can land.
const VerifierAttestationRecord = verifierRecordName

// VerifierBinding identifies one selected runner against one immutable source.
// This is dispatch provenance, NEVER a Verify result or evidence of execution.
// Checkout pins the line-ending conversion the home was checked out with, so
// admission renders attested blobs from the commit and this record only.
type VerifierBinding struct {
	Version                                                                              int
	SourceRepo, TrackingSource, TrackingHomeSHA256, TrackingCheckout                     string `json:",omitempty"`
	Run, Repo, Source, Brief, BriefSHA256, PlanSHA256, HomeSHA256, Model, Tier, Checkout string
}

// verifierPhase names the admission checkpoint. Before execution every source
// file, the stream index included, must hold its attested bytes; at Evidence
// time the stream index may also carry the attested brief's own lifecycle edit.
type verifierPhase int

const (
	verifierExecution verifierPhase = iota
	verifierEvidence
)

type VerifierReceipt struct {
	Binding VerifierBinding
	Issue   int
	URL     string
	// BriefRoot is local-only routing returned after admission, never part of the issue body.
	BriefRoot string `json:",omitempty"`
	// SourceHome is the admitted deliverable home a version-2 run's Verify rows
	// execute in, returned after admission and local-only like BriefRoot. The
	// executor runs its rows there and refuses any other row root: BriefRoot is
	// the tracking tree, so a root derived from the brief's location would run
	// the rows against a tree the attestation does not name.
	SourceHome string `json:",omitempty"`
}
type verifierLocal struct {
	VerifierReceipt
	Home         string
	TrackingHome string `json:",omitempty"`
	Attempted    bool
}

func verifierDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// verifierEnv is the environment for every admission git read: inherited GIT_*
// variables (which can redirect the repository, index, object store, config or
// replace-ref base) are dropped, replacement objects are disabled, system
// attributes and system config are ignored, and git may not prompt or run an
// askpass helper, so the reads resolve the dispatched commit's own objects, and
// the index the Verify rows read, in the home itself. An admitted run's rows
// read with the same settings, so a home admission probes is read the same way.
func verifierEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "GIT_") {
			env = append(env, kv)
		}
	}
	return append(env, "GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=")
}

// VerifierReadEnv gives outcome readers the same Git environment as admission.
// A caller cannot redirect a tracking-history read after its source check.
func VerifierReadEnv() []string { return verifierEnv() }

func verifierGitBytes(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"--no-replace-objects", "-C", root}, args...)...)
	cmd.Env = verifierEnv()
	return cmd.Output()
}
func verifierGit(root string, args ...string) (string, error) {
	out, err := verifierGitBytes(root, args...)
	if err != nil {
		return "", Unverifiable("cannot establish verifier source: git "+strings.Join(args, " "), err)
	}
	for _, arg := range args {
		if arg == "-z" {
			return string(out), nil
		}
	}
	return strings.TrimSpace(string(out)), nil
}
func verifierRecordPath(root string) (string, error) {
	return verifierGit(root, "rev-parse", "--path-format=absolute", "--git-path", verifierRecordName)
}
func verifierSave(root string, r verifierLocal) error {
	path, err := verifierRecordPath(root)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".attestation-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
func verifierLoad(root string) (verifierLocal, error) {
	var r verifierLocal
	path, err := verifierRecordPath(root)
	if err != nil {
		return r, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return r, Unverifiable("missing original pre-work verifier record: run nonce, repository/source, brief/plan, home and model provenance are unavailable; cannot recover or execute this run", err)
	}
	if err = json.Unmarshal(b, &r); err != nil {
		return r, Unverifiable("unreadable verifier attestation", err)
	}
	return r, nil
}

// withoutEvidence permits only the Evidence section to change after execution.
// Status lives in the separate stream index; Verify commands and all brief inputs
// remain bound to the dispatched source.
func withoutEvidence(b []byte) []byte {
	var out []string
	inEvidence := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "## ") {
			inEvidence = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == "Evidence"
		}
		if !inEvidence {
			out = append(out, line)
		}
	}
	return []byte(strings.TrimSpace(strings.Join(out, "\n")))
}
func verifierLocalCheck(root, brief string, r verifierLocal, phase verifierPhase) error {
	if r.Binding.Version == 2 {
		return verifierCrossRepoCheck(root, brief, r, phase)
	}
	given, home, err := verifierHome(root)
	if err != nil {
		return err
	}
	if home != r.Home || verifierDigest([]byte(home)) != r.Binding.HomeSHA256 {
		return Refused("verifier attestation belongs to a different worktree")
	}
	b := r.Binding
	if b.Version != 1 || len(b.Run) != 64 || len(b.Source) != 40 || b.Repo == "" || b.Brief == "" {
		return Refused("invalid verifier binding")
	}
	if _, err := hex.DecodeString(b.Run); err != nil {
		return Refused("invalid verifier run identifier")
	}
	if _, err := ModelStampLabels(b.Model, b.Tier); err != nil {
		return err
	}
	checkout, err := parseVerifierCheckout(b.Checkout)
	if err != nil {
		return err
	}
	path := brief
	if !filepath.IsAbs(path) {
		path = filepath.Join(given, path)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(given, path)
	if err != nil {
		return err
	}
	if filepath.ToSlash(rel) != b.Brief {
		return Refused("verifier attestation belongs to a different brief")
	}
	head, err := verifierGit(home, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	branch, err := verifierGit(home, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if head != b.Source || branch != "HEAD" {
		return Refused("verifier requires its attested detached source commit")
	}
	if err := verifierOwnWorkTree(home); err != nil {
		return err
	}
	if err := verifierRowGitReads(home); err != nil {
		return err
	}
	// Admission compares the bytes a Verify row will read, never git's own view
	// of the worktree: index flags, replacement objects, clean filters and the
	// home's own attributes or conversion config can all make that view report
	// no change. Evidence edits target the brief's Evidence section; the stream
	// index is source until Evidence time, when only the attested brief's own
	// row may move. Other outputs belong outside this home.
	if err := verifierNoReplacements(home); err != nil {
		return err
	}
	if err := verifierNoLocalAttributes(home); err != nil {
		return err
	}
	allowed := map[string]bool{b.Brief: true}
	index := filepath.ToSlash(filepath.Join(filepath.Dir(b.Brief), "README.md"))
	if phase == verifierEvidence {
		allowed[index] = true
	}
	if err := verifierSourceClosure(home, b.Source, checkout, allowed); err != nil {
		return err
	}
	if phase == verifierEvidence {
		_, nn, _ := r.VerifierReceipt.verifierBriefKey()
		if err := verifierIndexEdit(home, b.Source, index, nn, checkout); err != nil {
			return err
		}
	}

	source, err := verifierGitBytes(home, "cat-file", "blob", b.Source+":"+b.Brief)
	if err != nil {
		return Unverifiable("cannot read attested brief at source", err)
	}
	if verifierDigest(source) != b.BriefSHA256 || verifierDigest(withoutEvidence(source)) != b.PlanSHA256 {
		return Refused("attested source brief digest mismatch")
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if verifierDigest(withoutEvidence(current)) != b.PlanSHA256 {
		return Refused("brief inputs or Verify commands changed since dispatch")
	}
	return nil
}

// PrepareVerifierAttestation persists the nonce BEFORE the first forge write.
func PrepareVerifierAttestation(root, repo, brief, model, tier string) error {
	if filepath.IsAbs(brief) && !verifierContains(root, brief) {
		return prepareVerifierCrossRepo(root, repo, brief, model, tier)
	}
	desk, _ := RoleAppLogin(DispatcherRole)
	if !verifierAuthority(desk) {
		return Refused("pre-work attestation requires distinct configured desk and verifier actors")
	}
	if !IsAllowedRepo(repo) {
		return Refused("verifier repo outside configured repo set")
	}
	labels, err := ModelStampLabels(model, tier)
	if err != nil {
		return err
	}
	stamp, _ := ModelStampOf(labels)
	given, home, err := verifierHome(root)
	if err != nil {
		return err
	}
	path := brief
	if !filepath.IsAbs(path) {
		path = filepath.Join(given, path)
	}
	rel, err := filepath.Rel(given, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return Refused("verifier brief must be inside its source worktree")
	}
	head, err := verifierGit(home, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	main, err := verifierGit(home, "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return err
	}
	if head != main {
		return Refused("fresh verifier source is not origin/main")
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	checkout, err := readVerifierCheckout(home)
	if err != nil {
		return err
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	r := verifierLocal{Home: home, VerifierReceipt: VerifierReceipt{Binding: VerifierBinding{Version: 1, Run: verifierDigest(nonce), HomeSHA256: verifierDigest([]byte(home)), Repo: repo, Source: head, Brief: filepath.ToSlash(rel), BriefSHA256: verifierDigest(original), PlanSHA256: verifierDigest(withoutEvidence(original)), Model: stamp.Model, Tier: stamp.Tier, Checkout: checkout.String()}}}
	record, err := verifierRecordPath(home)
	if err != nil {
		return err
	}
	if _, err = os.Stat(record); err == nil {
		return Refused("verifier run already prepared; recover this run instead of replacing it")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = verifierLocalCheck(home, rel, r, verifierExecution); err != nil {
		return err
	}
	return verifierSave(home, r)
}
func verifierBody(b VerifierBinding) string {
	data, _ := json.MarshalIndent(b, "", "  ")
	return "Dispatcher-owned pre-work attestation. This record contains NO verification result.\n\n" + string(data) + "\n"
}
func verifierAuthority(login string) bool { return attestationAuthority("verifier")(login) }
func verifierIssue(f Forge, repo ForgeRepo, r verifierLocal) (*Issue, error) {
	return attestationRecord(f, repo, r.Issue, VerifierAttestationTitle+r.Binding.Run, verifierBody(r.Binding), verifierAuthority, "verifier")
}
func verifierRepo(slug string) (ForgeRepo, error) {
	owner, name, ok := strings.Cut(slug, "/")
	if !ok || owner == "" || name == "" {
		return ForgeRepo{}, Refused("invalid verifier repository")
	}
	return ForgeRepo{Owner: owner, Name: name}, nil
}

// IssueVerifierAttestation recovers the persisted run without claims, allocations
// or worker launch. An uncertain create may only be recovered by finding its exact
// nonce; absence is not permission to create a duplicate.
func IssueVerifierAttestation(root string, f Forge) (VerifierReceipt, error) {
	r, err := verifierLoad(root)
	if err != nil {
		return r.VerifierReceipt, err
	}
	if err = verifierLocalCheck(root, r.Binding.Brief, r, verifierExecution); err != nil {
		return r.VerifierReceipt, err
	}
	repo, err := verifierRepo(r.Binding.Repo)
	if err != nil {
		return r.VerifierReceipt, err
	}
	if r.Issue == 0 && r.Attempted {
		hits, e := f.SearchIssues(repo, SearchIssuesInput{Query: r.Binding.Run})
		if e != nil {
			return r.VerifierReceipt, Unverifiable("cannot recover verifier issue creation", e)
		}
		for _, hit := range hits {
			if hit.Title == VerifierAttestationTitle+r.Binding.Run {
				if r.Issue != 0 {
					return r.VerifierReceipt, Refused("duplicate verifier records require reconciliation")
				}
				r.Issue = hit.Number
				r.URL = hit.URL
			}
		}
		if r.Issue == 0 {
			return r.VerifierReceipt, Unverifiable("previous issue creation outcome unknown; retry recovery after the exact run record becomes readable, never create another run to bypass it", nil)
		}
		if err = verifierSave(root, r); err != nil {
			return r.VerifierReceipt, err
		}
	}
	if r.Issue == 0 {
		r.Attempted = true
		if err = verifierSave(root, r); err != nil {
			return r.VerifierReceipt, err
		}
		ref, e := f.FileIssue(repo, IssueInput{Title: VerifierAttestationTitle + r.Binding.Run, Body: verifierBody(r.Binding)})
		if e != nil {
			return r.VerifierReceipt, Unverifiable("verifier issue create outcome uncertain; recover the same run", e)
		}
		if ref.Number <= 0 {
			return r.VerifierReceipt, Unverifiable("verifier issue create returned no target", nil)
		}
		r.Issue = ref.Number
		r.URL = ref.URL
		if err = verifierSave(root, r); err != nil {
			return r.VerifierReceipt, err
		}
	}
	issue, err := verifierIssue(f, repo, r)
	if err != nil {
		return r.VerifierReceipt, err
	}
	labels, _ := ModelStampLabels(r.Binding.Model, r.Binding.Tier)
	if err = ApplyVerifiedModelStamp(f, repo, r.Issue, TargetIssue, labels, verifierAuthority); err != nil {
		return r.VerifierReceipt, err
	}
	if issue.State != "closed" {
		if err = f.CloseIssueTyped(repo, r.Issue, TargetIssue, "completed"); err != nil {
			return r.VerifierReceipt, Unverifiable("cannot close attestation record (not a verification result)", err)
		}
	}
	return CheckVerifierAttestationWithForge(root, r.Binding.Brief, f)
}

// CheckVerifierAttestationWithForge is pre-execution admission: every source
// file, the stream index included, holds its attested bytes.
func CheckVerifierAttestationWithForge(root, brief string, f Forge) (VerifierReceipt, error) {
	return checkVerifierAttestation(root, brief, f, verifierExecution)
}
func checkVerifierAttestation(root, brief string, f Forge, phase verifierPhase) (VerifierReceipt, error) {
	r, err := verifierLoad(root)
	if err != nil {
		return r.VerifierReceipt, err
	}
	if err = verifierLocalCheck(root, brief, r, phase); err != nil {
		return r.VerifierReceipt, err
	}
	if r.Issue <= 0 {
		return r.VerifierReceipt, Refused("verifier stamp is pending; no Verify rows admitted")
	}
	repo, err := verifierRepo(r.Binding.Repo)
	if err != nil {
		return r.VerifierReceipt, err
	}
	issue, err := verifierIssue(f, repo, r)
	if err != nil {
		return r.VerifierReceipt, err
	}
	if issue.State != "closed" {
		return r.VerifierReceipt, Refused("pre-work attestation has not completed")
	}
	tl, err := StampTimelineFor(f, repo, r.Issue, TargetIssue)
	if err != nil {
		return r.VerifierReceipt, err
	}
	stamp, state := AttestedModelStampOf(tl, verifierAuthority)
	if state != ModelStamped || stamp.Model != r.Binding.Model || stamp.Tier != r.Binding.Tier {
		return r.VerifierReceipt, Refused("missing or mismatched dispatcher stamp; no Verify rows admitted")
	}
	if r.Binding.Version == 2 {
		r.BriefRoot, r.SourceHome = r.TrackingHome, r.Home
	}
	return r.VerifierReceipt, nil
}
func CheckVerifierAttestation(root, brief string) (VerifierReceipt, error) {
	r, err := verifierLoad(root)
	if err != nil {
		return r.VerifierReceipt, err
	}
	repo, err := verifierRepo(r.Binding.Repo)
	if err != nil {
		return r.VerifierReceipt, err
	}
	f, _, err := ResolveForge(repo, "verifier")
	if err != nil {
		return r.VerifierReceipt, err
	}
	return CheckVerifierAttestationWithForge(root, brief, f)
}
func RecoverVerifierAttestation(root string) (VerifierReceipt, error) {
	r, err := verifierLoad(root)
	if err != nil {
		return r.VerifierReceipt, err
	}
	repo, err := verifierRepo(r.Binding.Repo)
	if err != nil {
		return r.VerifierReceipt, err
	}
	f, _, err := ResolveForge(repo, DispatcherRole)
	if err != nil {
		return r.VerifierReceipt, err
	}
	return IssueVerifierAttestation(root, f)
}
func (r VerifierReceipt) EvidenceBinding() string {
	if r.Binding.Version == 2 {
		return fmt.Sprintf("Verification-Attestation: %s#%d run=%s source-repo=%s source=%s tracking-source=%s brief=%s model=%s tier=%s", r.Binding.Repo, r.Issue, r.Binding.Run, r.Binding.SourceRepo, r.Binding.Source, r.Binding.TrackingSource, r.Binding.Brief, r.Binding.Model, r.Binding.Tier)
	}
	return fmt.Sprintf("Verification-Attestation: %s#%d run=%s source=%s brief=%s model=%s tier=%s", r.Binding.Repo, r.Issue, r.Binding.Run, r.Binding.Source, r.Binding.Brief, r.Binding.Model, r.Binding.Tier)
}
func IsVerifierAttestation(title, author string) bool {
	return strings.HasPrefix(title, VerifierAttestationTitle) && verifierAuthority(author)
}

// ReadVerifierAttestation admits an existing remote record without requiring the
// original local verifier home. Scheduling readers use the same immutable record
// and dispatcher stamp gates as execution and Evidence admission; this is never
// authority to execute rows, land Evidence, or advance lifecycle.
func ReadVerifierAttestation(f Forge, repo ForgeRepo, n int) (VerifierReceipt, error) {
	r := VerifierReceipt{Issue: n}
	if n <= 0 {
		return r, Refused("verifier attestation number must be positive")
	}
	issue, err := f.GetIssueTyped(repo, n, TargetIssue)
	if err != nil {
		return r, Unverifiable("cannot read verifier attestation", err)
	}
	if issue == nil {
		return r, Refused("missing verifier attestation")
	}
	_, raw, ok := strings.Cut(issue.Body, "\n\n")
	if !ok {
		return r, Refused("verifier attestation body is not a binding")
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r.Binding); err != nil {
		return r, Refused("verifier attestation body is not a binding")
	}
	b := r.Binding
	if (b.Version != 1 && b.Version != 2) || b.Repo != repo.Slug() || !workerHex64.MatchString(b.Run) ||
		len(b.Source) != 40 || !workerHexCommit.MatchString(b.Source) || !workerHex64.MatchString(b.HomeSHA256) || !workerHex64.MatchString(b.BriefSHA256) || !workerHex64.MatchString(b.PlanSHA256) {
		return r, Refused("invalid verifier attestation binding")
	}
	if err := r.CheckEvidenceTarget(b.Brief); err != nil {
		return r, err
	}
	if b.Version == 1 && (b.SourceRepo != "" || b.TrackingSource != "" || b.TrackingHomeSHA256 != "" || b.TrackingCheckout != "") {
		return r, Refused("same-repository verifier has cross-repository fields")
	}
	if b.Version == 2 && (!IsAllowedRepo(b.SourceRepo) || strings.EqualFold(b.SourceRepo, b.Repo) || len(b.TrackingSource) != 40 || !workerHexCommit.MatchString(b.TrackingSource) || !workerHex64.MatchString(b.TrackingHomeSHA256)) {
		return r, Refused("invalid cross-repository verifier binding")
	}
	if b.Version == 2 {
		if _, err := parseVerifierCheckout(b.TrackingCheckout); err != nil {
			return r, err
		}
	}
	if _, err := ModelStampLabels(b.Model, b.Tier); err != nil {
		return r, err
	}
	if _, err := parseVerifierCheckout(b.Checkout); err != nil {
		return r, err
	}
	issue, err = attestationRecord(f, repo, n, VerifierAttestationTitle+b.Run, verifierBody(b), verifierAuthority, "verifier")
	if err != nil {
		return r, err
	}
	if issue.State != "closed" {
		return r, Refused("verifier pre-work attestation has not completed")
	}
	tl, err := StampTimelineFor(f, repo, n, TargetIssue)
	if err != nil {
		return r, err
	}
	stamp, state := AttestedModelStampOf(tl, verifierAuthority)
	if state != ModelStamped || stamp.Model != b.Model || stamp.Tier != b.Tier {
		return r, Refused("missing or mismatched dispatcher verifier stamp")
	}
	r.URL = issue.URL
	return r, nil
}

// CheckVerifierEvidence reuses pre-work admission at landing time, after the
// verifier's edit to the attested brief's Evidence section, and binds the
// landing target to the attested brief: an attestation for one brief never
// admits Evidence, a stream-index edit or an outcome record for another. Only
// at this phase may the home's stream index differ from its attested bytes,
// and then only in the attested brief's own row's lifecycle cells (the
// row-scoped landing writes them); execution-phase admission refuses any
// stream-index change.
func CheckVerifierEvidence(root, repo, target string) (VerifierReceipt, error) {
	return CheckVerifierEvidenceWithForge(root, repo, target, nil)
}

// CheckVerifierEvidenceWithForge is CheckVerifierEvidence against f; a nil f
// resolves the verifier's own forge.
func CheckVerifierEvidenceWithForge(root, repo, target string, f Forge) (VerifierReceipt, error) {
	if root == "" {
		root = "."
	}
	r, err := verifierLoad(root)
	if err != nil {
		record, perr := verifierRecordPath(root)
		if _, serr := os.Stat(record); perr != nil || serr != nil {
			return r.VerifierReceipt, Refused("no pre-work verifier attestation at " + root + "; land Evidence with --root naming the dispatched verifier home, not the desk checkout, and never dispatch another run to obtain one")
		}
		return r.VerifierReceipt, err
	}
	if r.Binding.Repo != repo {
		return r.VerifierReceipt, Refused("evidence repository differs from attested run")
	}
	if err := r.VerifierReceipt.CheckEvidenceTarget(target); err != nil {
		return r.VerifierReceipt, err
	}
	if f == nil {
		fr, err := verifierRepo(r.Binding.Repo)
		if err != nil {
			return r.VerifierReceipt, err
		}
		if f, _, err = ResolveForge(fr, "verifier"); err != nil {
			return r.VerifierReceipt, err
		}
	}
	return checkVerifierAttestation(root, r.Binding.Brief, f, verifierEvidence)
}

// verifierBriefKey returns the attested brief's <stream> and <NN> when it sits
// at docs/streams/<stream>/brief-<NN>-*.md.
func (r VerifierReceipt) verifierBriefKey() (stream, nn string, ok bool) {
	dir, base := path.Split(cleanRepoPath(r.Binding.Brief))
	stream, ok = strings.CutPrefix(strings.TrimSuffix(dir, "/"), "docs/streams/")
	if !ok || stream == "" || strings.Contains(stream, "/") || !isBriefFileName(base) {
		return "", "", false
	}
	nn, _, _ = strings.Cut(strings.TrimPrefix(base, "brief-"), "-")
	return stream, nn, true
}

// CheckEvidenceTarget admits only the attested brief itself, its stream index,
// or a verify-outcome record keyed to that brief's <stream>/<NN>.
func (r VerifierReceipt) CheckEvidenceTarget(target string) error {
	t, brief := cleanRepoPath(target), cleanRepoPath(r.Binding.Brief)
	if t == "" || brief == "" || strings.HasPrefix(t, "/") || strings.HasPrefix(t, "../") {
		return Refused("Evidence target " + target + " is not bound to the attested brief")
	}
	if t == brief || t == path.Join(path.Dir(brief), "README.md") {
		return nil
	}
	if rest, ok := strings.CutPrefix(t, OutcomeRecordsDir+"/"); ok {
		recStream, file, _ := strings.Cut(rest, "/")
		recNN, _, _ := strings.Cut(file, "-")
		if stream, nn, ok := r.verifierBriefKey(); ok && recStream == stream && recNN == nn && !strings.Contains(file, "/") {
			return nil
		}
	}
	return Refused("Evidence target " + target + " is not bound to the attested brief " + r.Binding.Brief)
}

// CheckEvidenceContent refuses a landing that would change the attested brief's
// inputs, and binds a stream-index landing to the attested brief's own row:
// rows names the index rows the landing may touch, and each must be that row.
func (r VerifierReceipt) CheckEvidenceContent(target string, content []byte, rows []string) error {
	if r.Binding.Brief == "" {
		return nil
	}
	t, brief := cleanRepoPath(target), cleanRepoPath(r.Binding.Brief)
	if t == brief && verifierDigest(withoutEvidence(content)) != r.Binding.PlanSHA256 {
		return Refused("Evidence landing would change attested brief inputs or Verify commands")
	}
	if t == path.Join(path.Dir(brief), "README.md") {
		_, nn, ok := r.verifierBriefKey()
		if !ok || len(rows) == 0 {
			return Refused("stream-index landing for an attested run must name the attested brief's row")
		}
		for _, row := range rows {
			if strings.TrimSpace(row) != nn {
				return Refused("stream-index row " + row + " is not the attested brief's row " + nn)
			}
		}
	}
	return nil
}

// CheckObservedBrief binds a scheduling reader's immutable source bytes to the
// exact original brief and plan admitted before this verifier run.
func (r VerifierReceipt) CheckObservedBrief(content []byte) error {
	if verifierDigest(content) != r.Binding.BriefSHA256 || verifierDigest(withoutEvidence(content)) != r.Binding.PlanSHA256 {
		return Refused("attested source brief digest mismatch")
	}
	return nil
}

// CheckVerifierSourceTree establishes unchanged source bytes for a scheduling
// hold that has no wake receipt. It reuses admission's tree/checkout checks;
// HEAD identity alone says nothing about modified, ignored or untracked bytes.
func CheckVerifierSourceTree(root, source string) error {
	checkout, err := readVerifierCheckout(root)
	if err != nil {
		return err
	}
	if err := verifierNoReplacements(root); err != nil {
		return err
	}
	if err := verifierNoLocalAttributes(root); err != nil {
		return err
	}
	return verifierSourceClosure(root, source, checkout, nil)
}

// VerifierEvidenceBindings returns real binding lines inside Evidence. Fenced
// examples and mentions elsewhere never establish an Evidence run. Shared by
// outcome admission and queue scheduling so the two read the same contract.
func VerifierEvidenceBindings(content []byte) []string {
	var lines []string
	inEvidence, fence := false, ""
	fenceLine := regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	for _, line := range strings.Split(string(content), "\n") {
		if match := fenceLine.FindStringSubmatch(line); match != nil {
			if fence == "" {
				fence = match[1]
			} else if match[1][0] == fence[0] && len(match[1]) >= len(fence) && strings.TrimSpace(match[2]) == "" {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inEvidence = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == "Evidence"
		}
		line = strings.TrimRight(line, " \t\r")
		if inEvidence && strings.HasPrefix(line, "Verification-Attestation: ") {
			lines = append(lines, line)
		}
	}
	return lines
}

// PlanVerifierAttestation validates the existing local binding without forge access.
func PlanVerifierAttestation(root, brief string) error {
	r, err := verifierLoad(root)
	if err != nil {
		return err
	}
	if brief == "" {
		brief = r.Binding.Brief
	}
	return verifierLocalCheck(root, brief, r, verifierExecution)
}
