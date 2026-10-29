package deskkit

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/deskcore/attestation"
)

// Cross-repository runs have two independent immutable trees. Repo names the
// tracking repository, which owns the brief, attestation and Evidence. Source
// and SourceRepo name the deliverable being observed. No tracking file enters
// the deliverable tree; its isolated checkout lives outside both source roots.
// Only the local record carries its path. The issue binds a digest of that path.
func verifierContains(root, file string) bool {
	root, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	file, err = filepath.EvalSymlinks(file)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, file)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func verifierOrigin(root string) (string, error) {
	raw, err := verifierGit(root, "config", "--get", "remote.origin.url")
	if err != nil {
		return "", err
	}
	return RemoteRepoSlug(raw)
}

func verifierVisibility(tracking, source string) error {
	if RepoVisibility(tracking) == VisibilityUnknown || RepoVisibility(source) == VisibilityUnknown ||
		(RepoVisibility(tracking) == VisibilityPublic && RepoVisibility(source) != VisibilityPublic) {
		return Refused("verifier tracking destination cannot publish the deliverable repository binding")
	}
	return nil
}

func verifierRepoIdentity(home, repo string) error {
	actual, err := verifierOrigin(home)
	if err != nil {
		return Unverifiable("cannot establish verifier repository identity", err)
	}
	if !strings.EqualFold(actual, repo) || !IsAllowedRepo(repo) {
		return Refused("verifier repository identity differs from its binding")
	}
	return nil
}

func prepareVerifierCrossRepo(root, repo, brief, model, tier string) error {
	desk, _ := RoleAppLogin(DispatcherRole)
	if !verifierAuthority(desk) {
		return Refused("pre-work attestation requires distinct configured desk and verifier actors")
	}
	labels, err := ModelStampLabels(model, tier)
	if err != nil {
		return err
	}
	stamp, _ := ModelStampOf(labels)
	_, home, err := verifierHome(root)
	if err != nil {
		return err
	}
	record, err := verifierRecordPath(home)
	if err != nil {
		return err
	}
	if _, err := os.Stat(record); err == nil {
		return Refused("verifier run already exists; recover the same run rather than replacing its provenance")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := verifierRepoIdentity(home, repo); err != nil {
		return err
	}
	// Resolve the actual containing repository, never a caller-supplied repo
	// label or a lexical ../ allowance. Symlinks cannot select a different brief.
	abs, err := filepath.Abs(brief)
	if err != nil {
		return err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil || filepath.Join(parent, filepath.Base(abs)) != real {
		return Refused("verifier tracking brief must not be a symlink")
	}
	tracking, err := verifierGit(filepath.Dir(real), "rev-parse", "--show-toplevel")
	if err != nil {
		return Unverifiable("missing tracking repository provenance", err)
	}
	_, tracking, err = verifierHome(tracking)
	if err != nil {
		return err
	}
	trackingRepo, err := verifierOrigin(tracking)
	if err != nil {
		return Unverifiable("missing tracking repository identity", err)
	}
	if !IsAllowedRepo(trackingRepo) || strings.EqualFold(repo, trackingRepo) {
		return Refused("external verifier brief must belong to a distinct allowed tracking repository")
	}
	// The tracking destination must not disclose a more restricted source.
	// Unknown visibility is not evidence that a publication is safe.
	if err := verifierVisibility(trackingRepo, repo); err != nil {
		return err
	}
	rel, err := filepath.Rel(tracking, real)
	if err != nil || !verifierContains(tracking, real) {
		return Refused("tracking brief escapes its repository")
	}
	head, err := verifierGit(home, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	main, err := verifierGit(home, "rev-parse", "refs/remotes/origin/main")
	if err != nil || main != head {
		return Refused("verifier source is not fetched origin/main")
	}
	trackingHead, err := verifierGit(tracking, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	trackingMain, err := verifierGit(tracking, "rev-parse", "refs/remotes/origin/main")
	if err != nil || trackingMain != trackingHead {
		return Refused("verifier tracking source is not fetched origin/main")
	}
	original, err := verifierGitBytes(tracking, "cat-file", "blob", trackingHead+":"+filepath.ToSlash(rel))
	if err != nil {
		return Unverifiable("tracking brief is unavailable at its immutable revision", err)
	}
	current, err := os.ReadFile(real)
	if err != nil || verifierDigest(current) != verifierDigest(original) {
		return Refused("tracking brief differs from its immutable revision")
	}
	checkout, err := readVerifierCheckout(home)
	if err != nil {
		return err
	}
	// Allocate once, then persist the nonce and both roots before any forge call.
	// A failed publication retains this exact checkout for recovery.
	base, err := os.MkdirTemp("", "assay-verifier-tracking-")
	if err != nil {
		return err
	}
	retained := false
	defer func() {
		if !retained {
			_ = os.RemoveAll(base)
		}
	}()
	// The system temp directory is often reached through a link (macOS /var,
	// /tmp). Every check below compares git's resolved work tree to this path
	// as a string, so it is canonical from the first byte.
	if base, err = filepath.EvalSymlinks(base); err != nil {
		return err
	}
	trackingHome := filepath.Join(base, "tree")
	if _, err := verifierGit(tracking, "clone", "--no-hardlinks", "--no-checkout", "--", tracking, trackingHome); err != nil {
		return err
	}
	origin, err := verifierGit(tracking, "remote", "get-url", "origin")
	if err != nil {
		return err
	}
	if _, err := verifierGit(trackingHome, "remote", "set-url", "origin", origin); err != nil {
		return err
	}
	if err := checkoutVerifierTracking(trackingHome, trackingHead); err != nil {
		return err
	}
	_, trackingHome, err = verifierHome(trackingHome)
	if err != nil {
		return err
	}
	tc, err := readVerifierCheckout(trackingHome)
	if err != nil {
		return err
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	r := verifierLocal{Home: home, TrackingHome: trackingHome, VerifierReceipt: VerifierReceipt{Binding: VerifierBinding{
		Version: 2, Run: verifierDigest(nonce), Repo: trackingRepo, SourceRepo: repo, Source: head,
		TrackingSource: trackingHead, Brief: filepath.ToSlash(rel), BriefSHA256: verifierDigest(original), PlanSHA256: verifierDigest(withoutEvidence(original)),
		HomeSHA256: verifierDigest([]byte(home)), TrackingHomeSHA256: verifierDigest([]byte(trackingHome)),
		Model: stamp.Model, Tier: stamp.Tier, Checkout: checkout.String(), TrackingCheckout: tc.String(),
		SiblingSources: verifierSiblingSources(trackingRepo, repo, original),
	}}}
	if err := verifierLocalCheck(home, r.Binding.Brief, r, verifierExecution); err != nil {
		return err
	}
	if err := verifierSave(home, r); err != nil {
		return err
	}
	retained = true
	return nil
}

// verifierSiblingSources binds, for each further ../<sibling>/ repository the
// tracking brief declares, the one configured root of that name at its fetched
// origin/main (#4876); the run's own SourceRepo already covers its name. A
// sibling with no single clean, publishable, fetched root is left unbound, so
// its non-pass outcome record is refused exactly as before, never guessed.
func verifierSiblingSources(trackingRepo, sourceRepo string, brief []byte) string {
	decl, err := BriefDeclaredFiles(string(brief))
	if err != nil {
		return ""
	}
	_, siblings := SplitSiblingDeclarations(decl)
	roots, err := ConfiguredRoots()
	if err != nil || len(siblings) == 0 {
		return ""
	}
	bound := map[string]string{}
	seen := map[string]bool{}
	for _, d := range siblings {
		name, _, _ := SiblingDeclaration(d)
		if seen[strings.ToLower(name)] || declaresSibling([]string{d}, sourceRepo) || declaresSibling([]string{d}, trackingRepo) {
			continue
		}
		seen[strings.ToLower(name)] = true
		var match []RootConfig
		for _, root := range roots {
			if declaresSibling([]string{d}, root.Repo) {
				match = append(match, root)
			}
		}
		if len(match) != 1 {
			continue
		}
		if head, ok := verifierSiblingHead(trackingRepo, match[0]); ok {
			bound[match[0].Repo] = head
		}
	}
	return attestation.FormatSiblingSources(bound)
}

// verifierSiblingHead is a sibling root's HEAD when it is the fetched
// origin/main, by the same checks the wake reader applies to a repo-source key.
func verifierSiblingHead(trackingRepo string, r RootConfig) (string, bool) {
	if verifierVisibility(trackingRepo, r.Repo) != nil {
		return "", false
	}
	_, root, err := verifierHome(r.Path)
	if err != nil || verifierRepoIdentity(root, r.Repo) != nil {
		return "", false
	}
	head, err := verifierGit(root, "rev-parse", "HEAD")
	if err != nil {
		return "", false
	}
	if main, err := verifierGit(root, "rev-parse", "refs/remotes/origin/main"); err != nil || main != head {
		return "", false
	}
	checkout, err := readVerifierCheckout(root)
	if err != nil || verifierNoReplacements(root) != nil || verifierNoLocalAttributes(root) != nil || verifierOwnWorkTree(root) != nil || verifierSourceClosure(root, head, checkout, nil) != nil {
		return "", false
	}
	return head, true
}

func verifierCrossRepoCheck(root, brief string, r verifierLocal, phase verifierPhase) error {
	b := r.Binding
	if b.SourceRepo == "" || b.TrackingSource == "" || b.TrackingHomeSHA256 == "" || r.TrackingHome == "" || b.TrackingCheckout == "" {
		return Unverifiable("cross-repository verifier record lacks tracking/source repository, revision, checkout or home provenance; original run cannot be reconstructed", nil)
	}
	_, home, err := verifierHome(root)
	if err != nil {
		return err
	}
	if home != r.Home || verifierDigest([]byte(home)) != b.HomeSHA256 {
		return Refused("verifier attestation belongs to a different worktree")
	}
	if err := verifierRepoIdentity(home, b.SourceRepo); err != nil {
		return err
	}
	_, tracking, err := verifierHome(r.TrackingHome)
	if err != nil {
		return err
	}
	if tracking != r.TrackingHome || verifierDigest([]byte(tracking)) != b.TrackingHomeSHA256 || verifierContains(home, tracking) || verifierContains(tracking, home) {
		return Refused("tracking checkout differs from the attested isolated home")
	}
	if err := verifierRepoIdentity(tracking, b.Repo); err != nil {
		return err
	}
	if err := verifierVisibility(b.Repo, b.SourceRepo); err != nil {
		return err
	}
	if strings.EqualFold(b.Repo, b.SourceRepo) {
		return Refused("cross-repository binding names only one repository")
	}
	siblings, err := VerifierSiblingSources(b)
	if err != nil {
		return err
	}
	for sibling := range siblings {
		if err := verifierVisibility(b.Repo, sibling); err != nil {
			return err
		}
	}
	if err := verifierAssociation(tracking, b.TrackingSource, b.Brief, b.SourceRepo); err != nil {
		return err
	}
	// Check the deliverable with NO editable exceptions: its brief lives elsewhere.
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
	for _, check := range []func(string) error{verifierOwnWorkTree, verifierRowGitReads, verifierNoReplacements, verifierNoLocalAttributes} {
		if err := check(home); err != nil {
			return err
		}
	}
	checkout, err := parseVerifierCheckout(b.Checkout)
	if err != nil {
		return err
	}
	if err := verifierSourceClosure(home, b.Source, checkout, nil); err != nil {
		return err
	}
	// Reuse every original brief/index/digest check against the tracking tree.
	tr := r
	tr.Home, tr.TrackingHome = tracking, ""
	tr.Binding.Version, tr.Binding.Source, tr.Binding.SiblingSources = 1, b.TrackingSource, ""
	tr.Binding.HomeSHA256, tr.Binding.Checkout = b.TrackingHomeSHA256, b.TrackingCheckout
	return verifierLocalCheck(tracking, brief, tr, phase)
}

// VerifierBriefLocation reads local dispatch provenance without granting admission.
// Packet/prompt construction uses it; execution still requires the forge check.
func VerifierBriefLocation(root string) (briefRoot, brief, revision string, err error) {
	r, err := verifierLoad(root)
	if err != nil {
		return "", "", "", err
	}
	if err := verifierLocalCheck(root, r.Binding.Brief, r, verifierExecution); err != nil {
		return "", "", "", err
	}
	if r.Binding.Version == 2 {
		return r.TrackingHome, r.Binding.Brief, r.Binding.TrackingSource, nil
	}
	return r.Home, r.Binding.Brief, r.Binding.Source, nil
}

// EvidenceRoot and BriefSource identify the tracking tree after admission.
func (r VerifierReceipt) EvidenceRoot(sourceRoot string) string {
	if r.Binding.Version == 2 {
		return r.BriefRoot
	}
	return sourceRoot
}
func (r VerifierReceipt) BriefSource() string {
	if r.Binding.Version == 2 {
		return r.Binding.TrackingSource
	}
	return r.Binding.Source
}
