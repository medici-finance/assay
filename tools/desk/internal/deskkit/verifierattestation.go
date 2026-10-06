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
	"strings"
)

const VerifierAttestationTitle = "[verification-attestation] "
const verifierRecordName = "assay-verifier-attestation.json"

// VerifierBinding identifies one selected runner against one immutable source.
// This is dispatch provenance, NEVER a Verify result or evidence of execution.
type VerifierBinding struct {
	Version                                                                    int
	Run, Repo, Source, Brief, BriefSHA256, PlanSHA256, HomeSHA256, Model, Tier string
}
type VerifierReceipt struct {
	Binding VerifierBinding
	Issue   int
	URL     string
}
type verifierLocal struct {
	VerifierReceipt
	Home      string
	Attempted bool
}

func verifierDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// verifierEnv is the environment for every admission git read: inherited GIT_*
// variables (which can redirect the repository, index, object store, config or
// replace-ref base) are dropped, and replacement objects are disabled, so the
// reads resolve the dispatched commit's own objects in the home itself.
func verifierEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "GIT_") {
			env = append(env, kv)
		}
	}
	return append(env, "GIT_NO_REPLACE_OBJECTS=1")
}
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
		return r, Refused("no pre-work verifier attestation in this worktree; dispatch a verifier before executing Verify rows")
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
func verifierLocalCheck(root, brief string, r verifierLocal) error {
	home, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	home, err = filepath.EvalSymlinks(home)
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
	path := brief
	if !filepath.IsAbs(path) {
		path = filepath.Join(home, path)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(home, path)
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
	// Admission compares the bytes a Verify row will read, never git's own view
	// of the worktree: index flags, replacement objects and clean filters can all
	// make that view report no change. Evidence and status edits target the
	// brief and its stream index; other outputs belong outside this home.
	if err := verifierNoReplacements(home); err != nil {
		return err
	}
	index := filepath.ToSlash(filepath.Join(filepath.Dir(b.Brief), "README.md"))
	if err := verifierSourceClosure(home, b.Source, map[string]bool{b.Brief: true, index: true}); err != nil {
		return err
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
	home, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return err
	}
	path := brief
	if !filepath.IsAbs(path) {
		path = filepath.Join(home, path)
	}
	rel, err := filepath.Rel(home, path)
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
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	r := verifierLocal{Home: home, VerifierReceipt: VerifierReceipt{Binding: VerifierBinding{Version: 1, Run: verifierDigest(nonce), HomeSHA256: verifierDigest([]byte(home)), Repo: repo, Source: head, Brief: filepath.ToSlash(rel), BriefSHA256: verifierDigest(original), PlanSHA256: verifierDigest(withoutEvidence(original)), Model: stamp.Model, Tier: stamp.Tier}}}
	record, err := verifierRecordPath(home)
	if err != nil {
		return err
	}
	if _, err = os.Stat(record); err == nil {
		return Refused("verifier run already prepared; recover this run instead of replacing it")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = verifierLocalCheck(home, rel, r); err != nil {
		return err
	}
	return verifierSave(home, r)
}
func verifierBody(b VerifierBinding) string {
	data, _ := json.MarshalIndent(b, "", "  ")
	return "Dispatcher-owned pre-work attestation. This record contains NO verification result.\n\n" + string(data) + "\n"
}
func verifierAuthority(login string) bool {
	desk, ok := RoleAppLogin(DispatcherRole)
	verifier, bound := RoleAppLogin("verifier")
	return ok && bound && !SameActor(desk, verifier) && SameActor(login, desk)
}
func verifierIssue(f Forge, repo ForgeRepo, r verifierLocal) (*Issue, error) {
	issue, err := f.GetIssueTyped(repo, r.Issue, TargetIssue)
	if err != nil {
		return nil, Unverifiable("cannot read verifier attestation target", err)
	}
	if issue == nil || issue.IsPullRequest || !verifierAuthority(issue.Author.Login) || issue.Title != VerifierAttestationTitle+r.Binding.Run || issue.Body != verifierBody(r.Binding) {
		return nil, Refused("verifier target is not the exact dispatcher-authored run record")
	}
	trust, err := f.IssueTrustEvents(repo, r.Issue)
	if err != nil {
		return nil, Unverifiable("cannot establish immutable verifier record", err)
	}
	if trust == nil || !trust.BodyHistoryKnown || !trust.Complete || !trust.BodyEdited.IsZero() {
		return nil, Refused("verifier record edited or its history is incomplete")
	}
	return issue, nil
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
	if err = verifierLocalCheck(root, r.Binding.Brief, r); err != nil {
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

func CheckVerifierAttestationWithForge(root, brief string, f Forge) (VerifierReceipt, error) {
	r, err := verifierLoad(root)
	if err != nil {
		return r.VerifierReceipt, err
	}
	if err = verifierLocalCheck(root, brief, r); err != nil {
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
	return fmt.Sprintf("Verification-Attestation: %s#%d run=%s source=%s brief=%s model=%s tier=%s", r.Binding.Repo, r.Issue, r.Binding.Run, r.Binding.Source, r.Binding.Brief, r.Binding.Model, r.Binding.Tier)
}
func IsVerifierAttestation(title, author string) bool {
	return strings.HasPrefix(title, VerifierAttestationTitle) && verifierAuthority(author)
}

// CheckVerifierEvidence reuses pre-work admission after Evidence/status edits
// and binds the landing target to the attested brief: an attestation for one
// brief never admits Evidence, a stream-index edit or an outcome record for
// another.
func CheckVerifierEvidence(root, repo, target string) (VerifierReceipt, error) {
	if root == "" {
		root = "."
	}
	r, err := verifierLoad(root)
	if err != nil {
		return r.VerifierReceipt, err
	}
	if r.Binding.Repo != repo {
		return r.VerifierReceipt, Refused("evidence repository differs from attested run")
	}
	if err := r.VerifierReceipt.CheckEvidenceTarget(target); err != nil {
		return r.VerifierReceipt, err
	}
	return CheckVerifierAttestation(root, r.Binding.Brief)
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

// PlanVerifierAttestation validates the existing local binding without forge access.
func PlanVerifierAttestation(root, brief string) error {
	r, err := verifierLoad(root)
	if err != nil {
		return err
	}
	if brief == "" {
		brief = r.Binding.Brief
	}
	return verifierLocalCheck(root, brief, r)
}
