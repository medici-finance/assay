package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/attestation"
	"github.com/medici-finance/assay/statusgen/evaluation"
)

// The container bracket (desktools-v2/71). A runner inside a --network none
// container cannot reach the forge, so it never runs verifier admission
// itself. The host brackets the run instead, and one tree digest binds the
// bytes the container ran to the admitted home three times:
//
//  1. host open: `verifyrun --open-bracket` runs `deskdispatch
//     --check-verifier`, derives the executing runner on the host, digests the
//     home, and writes the receipt file (the check's JSON verbatim, `tree`,
//     `opened`, `runner`, `runner_source`) outside the home;
//  2. container: `verifyrun --sandbox=container-netns --admission-receipt
//     <file> --container-image-digest sha256:<…>` reads that receipt (no
//     admission and no identity derivation of its own), digests its own root
//     and refuses unless it equals the receipt's tree, runs the rows under the
//     receipt's runner, digests its root again, and only when the rows left it
//     unchanged records Tree-Digest, Container-Image and the attestation line
//     under the table;
//  3. host close: `verifyrun --close-bracket` re-runs the check, compares it
//     with the witness's attestation line, re-derives the runner on the host
//     and requires it to equal the receipt's and every container row's Runner
//     cell, recomputes the home's digest, and only when it equals both the
//     receipt's tree and the witness's Tree-Digest appends an
//     Admission-Bracket line whose witness= is bracketDigest of the span above
//     it (the Runner cells are inside that digest).
//
// A version-2 (cross-repository) receipt is admitted on the same three steps,
// with two more bindings. Its brief lives in the tracking repository and its
// rows run in the deliverable home, so the tree digest binds only the home:
// the brief each step reads must lie outside the home, end in the binding's
// repository-relative brief path and hold exactly the bytes the binding's
// BriefSHA256 names (crossRepoBindingProblem). Both repositories are named in
// the attestation line (repo#issue for the tracking repository, source-repo=
// for the deliverable), so they are inside the witness digest; the open and
// close steps also require the host check's admitted home to be --root and the
// close step's host binding to equal the receipt's, field for field.
//
// A container-sandboxed row is credited only inside a closed bracket. The
// bracket digest is keyless and computed from the file's own bytes, so a
// match shows only that the bracket is consistent with its span, not who
// wrote it. A row is credited only when the bracket line and every span line
// were landed by the verifier's landing step, the span's attestation matches
// the verifier's attestation record for that run, and that attestation names
// the brief that contains it and the source under verification (rule 45);
// nothing in this file checks those conditions.

const (
	containerImagePrefix   = "Container-Image: "
	treeDigestPrefix       = "Tree-Digest: "
	admissionBracketPrefix = "Admission-Bracket: "
	attestationPrefix      = "Verification-Attestation: "
)

var (
	sha256DigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	hex64Re        = regexp.MustCompile(`^[0-9a-f]{64}$`)
	hex40Re        = regexp.MustCompile(`^[0-9a-f]{40}$`)
	// attestationV1Re is the version-1 attestation line verifierAdmissionReceipt
	// renders. The version-2 form (source-repo=, tracking-source=) never matches.
	attestationV1Re = regexp.MustCompile(`^Verification-Attestation: (\S+)#([0-9]+) run=(\S+) source=(\S+) brief=(\S+) model=(\S+) tier=(\S+)$`)
	// attestationV2Re is the version-2 (cross-repository) line: repo#issue is
	// the tracking repository, source-repo= the deliverable repository whose
	// commit source= names, tracking-source= the tracking repository's commit.
	attestationV2Re = regexp.MustCompile(`^Verification-Attestation: (\S+)#([0-9]+) run=(\S+) source-repo=(\S+) source=(\S+) tracking-source=(\S+) brief=(\S+) model=(\S+) tier=(\S+)$`)
)

// containerAttestation is what the bracket reads from a container witness's
// attestation line, in either version. repo#issue names the attestation record
// in the tracking repository; source is the deliverable source commit and brief
// the repository-relative brief. sourceRepo and trackingSource are set on a
// version-2 (cross-repository) line only.
type containerAttestation struct {
	version               int
	repo, issue, run      string
	sourceRepo, source    string
	trackingSource, brief string
	model, tier           string
}

// containerAttestationFields names the fields values returns, in its order.
var containerAttestationFields = []string{"repo", "issue", "run", "source-repo", "source", "tracking-source", "brief", "model", "tier"}

// values is every field of the line, in containerAttestationFields' order; a
// version-1 line has empty source-repo and tracking-source.
func (a containerAttestation) values() []string {
	return []string{a.repo, a.issue, a.run, a.sourceRepo, a.source, a.trackingSource, a.brief, a.model, a.tier}
}

// parseContainerAttestation reads a version-1 or version-2 attestation line. A
// version-2 line must name two different owner/name repositories, the tracking
// one and the deliverable one; any other line is not a container attestation.
func parseContainerAttestation(line string) (containerAttestation, bool) {
	if m := attestationV1Re.FindStringSubmatch(line); m != nil {
		return containerAttestation{version: 1, repo: m[1], issue: m[2], run: m[3], source: m[4], brief: m[5], model: m[6], tier: m[7]}, true
	}
	m := attestationV2Re.FindStringSubmatch(line)
	if m == nil || !repoFrontmatterRe.MatchString(m[1]) || !repoFrontmatterRe.MatchString(m[4]) || strings.EqualFold(m[1], m[4]) {
		return containerAttestation{}, false
	}
	return containerAttestation{version: 2, repo: m[1], issue: m[2], run: m[3], sourceRepo: m[4], source: m[5], trackingSource: m[6], brief: m[7], model: m[8], tier: m[9]}, true
}

// bracketReceipt is the open step's receipt file. Runner, RunnerSource,
// RunnerTree and OnBehalfOf are every part of a witness Runner cell, as the
// HOST derived them at the open step (executingRunner, briefTreeSHA and
// onBehalfOfSuffix on the home), never values the container derives or a
// caller types (#4208): a home cut as a linked worktree keeps its git
// directory outside the mounted tree, and the container's own roster and git
// config are not the host's. The close step re-derives all four on the host
// and refuses unless they equal these and every row of the container's table
// carries exactly the cell they render.
type bracketReceipt struct {
	Check        json.RawMessage `json:"check"`
	Tree         string          `json:"tree"`
	Opened       string          `json:"opened"`
	Runner       string          `json:"runner"`
	RunnerSource string          `json:"runner_source"`
	RunnerTree   string          `json:"runner_tree"`
	OnBehalfOf   string          `json:"on_behalf_of"`
}

// receiptRunnerRe is the shape of a runner token executingRunner derives: a
// roster App slug with `[bot]`, a GitLab username, a CI actor, a git bot name
// or `human:<name>`. No whitespace, `|`, parentheses or control character can
// reach a witness Runner cell through a receipt.
var receiptRunnerRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:+\[\]-]{0,127}$`)

// receiptTreeRe is the shape of a tree token treeSHA renders: `no-git`, or a
// short commit SHA with an optional `+dirty` or `+unknown` qualifier.
var receiptTreeRe = regexp.MustCompile(`^(?:no-git|[0-9a-f]{7,40}(?:\+dirty|\+unknown)?)$`)

// receiptOnBehalfOfRe is the shape of an annotation onBehalfOfSuffix renders.
var receiptOnBehalfOfRe = regexp.MustCompile(`^on-behalf-of human:[^\s()|]+$`)

// receiptRunnerProblem validates the Runner-cell parts a receipt carries. An
// absent runner is could-not-attribute, exactly as on the host lane: the
// container never derives one of its own and never invents a placeholder.
func receiptRunnerProblem(r bracketReceipt) error {
	if r.Runner == "" && r.RunnerSource == "" {
		return fmt.Errorf("could-not-attribute — the receipt carries no runner identity from the host open step. A container run never derives its own identity (a linked worktree's git directory is not mounted) and never invents one; re-open the bracket on the host with a statusgen that records the runner")
	}
	if !receiptRunnerRe.MatchString(r.Runner) {
		return fmt.Errorf("the receipt's runner %q is not a runner identity the host derives", r.Runner)
	}
	switch r.RunnerSource {
	case runnerSourceForge, runnerSourceCIEnv, runnerSourceGitConfig:
	default:
		return fmt.Errorf("the receipt's runner source %q is not one of %s, %s or %s", r.RunnerSource, runnerSourceForge, runnerSourceCIEnv, runnerSourceGitConfig)
	}
	if !receiptTreeRe.MatchString(r.RunnerTree) {
		return fmt.Errorf("the receipt's runner tree %q is not a tree token the host derives (no-git, or a short SHA with an optional +dirty or +unknown); re-open the bracket on the host with a statusgen that records it", r.RunnerTree)
	}
	if r.OnBehalfOf != "" && !receiptOnBehalfOfRe.MatchString(r.OnBehalfOf) {
		return fmt.Errorf("the receipt's on-behalf-of %q is not an annotation the host renders", r.OnBehalfOf)
	}
	return nil
}

// hostRunnerCell derives, on the host, every part of the Runner cell a witness
// row taken on home for brief carries: the same executingRunner, briefTreeSHA and
// onBehalfOfSuffix (for the repo the row is written into) a host-lane row uses.
// ok is false when no executing identity is available.
func hostRunnerCell(home, brief string) (r bracketReceipt, ok bool) {
	r.Runner, r.RunnerSource, ok = executingRunner(home)
	if !ok {
		return bracketReceipt{}, false
	}
	r.RunnerTree = briefTreeSHA(home, brief)
	r.OnBehalfOf = onBehalfOfSuffix(r.Runner, witnessAnnotationRepo(witnessTargetRepo(brief), witnessOriginRepo(home)))
	return r, true
}

// receiptRunnerCell is the Runner cell a container row records under r.
func receiptRunnerCell(r bracketReceipt) string {
	return runnerCellText(r.Runner, r.RunnerTree, r.OnBehalfOf, r.RunnerSource)
}

// containerBracketRefusal validates the bracket flags of a row-running
// invocation. It is called right after sandboxRefusal, so the CI-lane and
// --in-container refusals keep their own wording. container-netns needs both
// the host receipt and the image digest, whatever DESK_LOOP says and whether
// or not an attestation file is visible: an unattested container run is
// refused before any row runs.
func containerBracketRefusal(mode, receipt, digest string) string {
	if mode != sandboxContainerNetns {
		if receipt != "" || digest != "" {
			return "--admission-receipt and --container-image-digest apply only with --sandbox=" + sandboxContainerNetns + " (and --admission-receipt with --close-bracket)"
		}
		return ""
	}
	if receipt == "" {
		return "refusing --sandbox=" + sandboxContainerNetns + " without --admission-receipt <file>: a container run reads the host open step's receipt, read-only and outside the home, and never runs admission itself. An unattested container run is not a witness; no row ran"
	}
	if digest == "" {
		return "refusing --sandbox=" + sandboxContainerNetns + " without --container-image-digest sha256:<64 hex>: the host names the image the rows ran in, and a run whose image cannot be named is could-not-run. No row ran"
	}
	if !sha256DigestRe.MatchString(digest) {
		return fmt.Sprintf("refusing --container-image-digest %q: want sha256: followed by 64 lowercase hex digits", digest)
	}
	return ""
}

// readBracketReceipt reads and validates an open step's receipt file. The file
// must sit outside the home, carry a sha256 tree and a parseable opened time,
// and embed a complete version-1 check receipt, or a version-2 one that passes
// crossRepoBindingProblem for this home and brief. It returns the receipt, the
// attestation line verifierAdmissionReceipt renders for it, and the receipt's
// repository-relative brief.
func readBracketReceipt(path, home, brief string) (bracketReceipt, string, string, error) {
	var r bracketReceipt
	if rel, ok := relWithin(home, path); ok {
		return r, "", "", fmt.Errorf("the receipt %s sits inside the home (%s); hand it over read-only from outside the home", path, rel)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return r, "", "", err
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, "", "", fmt.Errorf("unreadable receipt: %w", err)
	}
	if !sha256DigestRe.MatchString(r.Tree) {
		return r, "", "", fmt.Errorf("the receipt carries no sha256 tree digest from the open step")
	}
	if _, err := time.Parse(time.RFC3339, r.Opened); err != nil {
		return r, "", "", fmt.Errorf("the receipt carries no RFC 3339 opened time from the open step")
	}
	if err := receiptRunnerProblem(r); err != nil {
		return r, "", "", err
	}
	b, err := checkBinding(r.Check)
	if err != nil {
		return r, "", "", fmt.Errorf("unreadable check output in the receipt: %w", err)
	}
	switch b.Version {
	case 1:
		if b.Repo == "" || !hex64Re.MatchString(b.Run) || !hex40Re.MatchString(b.Source) || b.Brief == "" || !filepath.IsLocal(b.Brief) || filepath.ToSlash(filepath.Clean(b.Brief)) != b.Brief {
			return r, "", "", fmt.Errorf("incomplete receipt: it needs a repo, a 64-hex run, a 40-hex source and a home-relative brief")
		}
	case 2:
		if err := crossRepoBindingProblem(b, home, brief); err != nil {
			return r, "", "", err
		}
	default:
		return r, "", "", fmt.Errorf("unsupported receipt version %d", b.Version)
	}
	line, _, admitted, err := verifierAdmissionReceipt(r.Check, brief)
	if err != nil {
		return r, "", "", err
	}
	if b.Version == 2 {
		// The receipt's BriefRoot and SourceHome are host paths the container
		// does not have; verifierAdmissionReceipt has required them absolute,
		// and the open and close steps bind them on the host. Here the home is
		// bound by the tree digest and the brief by crossRepoBindingProblem.
		if admitted == "" {
			return r, "", "", fmt.Errorf("the version-2 receipt names no admitted deliverable home")
		}
		return r, line, b.Brief, nil
	}
	if admitted != "" {
		return r, "", "", fmt.Errorf("a cross-repository receipt is not admitted in container mode")
	}
	if rel, ok := relWithin(home, brief); !ok || rel != b.Brief {
		return r, "", "", fmt.Errorf("the receipt admits brief %s, but this run's brief is %s", b.Brief, brief)
	}
	return r, line, b.Brief, nil
}

// checkBinding reads the Binding of a `deskdispatch --check-verifier` receipt.
func checkBinding(check []byte) (attestation.Binding, error) {
	var peek struct{ Binding attestation.Binding }
	if err := json.Unmarshal(check, &peek); err != nil {
		return attestation.Binding{}, err
	}
	return peek.Binding, nil
}

// crossRepoBindingProblem is the bracket's check of a version-2
// (cross-repository) binding against the home the rows run in and the brief
// this step reads. Every field a version-1 container receipt must carry is
// still required, and on top of that:
//
//   - both repositories are named, as owner/name, and they differ: Repo is the
//     tracking repository the brief and its issue live in, SourceRepo the
//     deliverable repository the home is a checkout of;
//   - run, source and tracking source are the full hex forms;
//   - the binding passes the shared offline judgment
//     (attestation.BindingProblem), digests and checkout conversions included.
//     The repository set was decided by the host check that wrote the
//     receipt; this step has none of its own, so it does not re-decide it;
//   - the brief lies OUTSIDE the home (it is the tracking repository's file,
//     and the home's tree digest does not cover it), its resolved path ends
//     in the binding's brief path, and it holds exactly the bytes the
//     binding's BriefSHA256 names.
func crossRepoBindingProblem(b attestation.Binding, home, brief string) error {
	switch {
	case !repoFrontmatterRe.MatchString(b.Repo):
		return fmt.Errorf("incomplete cross-repository receipt: its tracking repository %q is not an owner/name", b.Repo)
	case !repoFrontmatterRe.MatchString(b.SourceRepo):
		return fmt.Errorf("incomplete cross-repository receipt: its source repository %q is not an owner/name", b.SourceRepo)
	case strings.EqualFold(b.Repo, b.SourceRepo):
		return fmt.Errorf("the cross-repository receipt names %s as both its tracking and its source repository; a version-2 receipt binds two different repositories", b.Repo)
	case !hex64Re.MatchString(b.Run) || !hex40Re.MatchString(b.Source) || !hex40Re.MatchString(b.TrackingSource):
		return fmt.Errorf("incomplete cross-repository receipt: it needs a 64-hex run, a 40-hex source and a 40-hex tracking source")
	case b.Brief == "" || !filepath.IsLocal(b.Brief) || filepath.ToSlash(filepath.Clean(b.Brief)) != b.Brief:
		return fmt.Errorf("incomplete cross-repository receipt: it needs a repository-relative brief")
	}
	if p := attestation.BindingProblem(b, b.Repo, attestation.GeneralV1V2, true); p != "" {
		return fmt.Errorf("the cross-repository receipt's binding is refused: %s", p)
	}
	if rel, ok := relWithin(home, brief); ok {
		return fmt.Errorf("the brief %s sits inside the home (%s): a cross-repository run reads its tracking repository's brief from outside the deliverable home. Pass --root <the deliverable home> and the tracking brief", brief, rel)
	}
	if !strings.HasSuffix(filepath.ToSlash(resolveExisting(brief)), "/"+b.Brief) {
		return fmt.Errorf("the receipt admits brief %s, but this run's brief is %s", b.Brief, brief)
	}
	raw, err := os.ReadFile(brief)
	if err != nil {
		return fmt.Errorf("the cross-repository brief: %w", err)
	}
	if sum := sha256.Sum256(raw); hex.EncodeToString(sum[:]) != b.BriefSHA256 {
		return fmt.Errorf("the brief %s does not hold the bytes the receipt admits (sha256 %s, receipt %s)", brief, hex.EncodeToString(sum[:]), b.BriefSHA256)
	}
	return nil
}

// containerAdmit is what a container run takes from the host's receipt: the
// attestation line, the admitted tree digest, and every part of the Runner
// cell the host derived at the open step.
type containerAdmit struct {
	attestation, tree string
	cell              bracketReceipt // Runner, RunnerSource, RunnerTree, OnBehalfOf
}

// containerAdmission is the container half: the host's receipt replaces
// admission and identity derivation, and the mounted root must digest to the
// receipt's tree before any row runs.
func containerAdmission(receiptPath, root, brief string) (containerAdmit, error) {
	r, line, _, err := readBracketReceipt(receiptPath, root, brief)
	if err != nil {
		return containerAdmit{}, fmt.Errorf("container admission refused; no Verify rows executed: %w", err)
	}
	tree, err := homeTreeDigest(root)
	if err != nil {
		return containerAdmit{}, fmt.Errorf("container admission refused; no Verify rows executed: %w", err)
	}
	if tree != r.Tree {
		return containerAdmit{}, fmt.Errorf("container admission refused; no Verify rows executed: the mounted root %s digests to %s, not the %s the host recorded at the open step. The container is not running the admitted home", root, tree, r.Tree)
	}
	cell := bracketReceipt{Check: r.Check, Runner: r.Runner, RunnerSource: r.RunnerSource, RunnerTree: r.RunnerTree, OnBehalfOf: r.OnBehalfOf}
	return containerAdmit{attestation: line, tree: tree, cell: cell}, nil
}

// relWithin returns p's slash path relative to root when p lies inside root,
// after resolving symlinks on both. A path that does not exist yet (a receipt
// or witness about to be written) is resolved through its nearest existing
// ancestor, so a symlinked parent directory that leads into root counts as
// inside it.
func relWithin(root, p string) (string, bool) {
	rel, err := filepath.Rel(resolveExisting(root), resolveExisting(p))
	if err != nil || !filepath.IsLocal(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// resolveExisting makes s absolute and resolves it one component at a time,
// as the kernel does: each existing prefix has its symlinks resolved before
// the next component, so a ".." after a symlink steps out of the link's
// target, never out of the link's own directory. Once a component does not
// exist, the rest is kept as written (cleaned lexically).
func resolveExisting(s string) string {
	if !filepath.IsAbs(s) {
		if wd, err := os.Getwd(); err == nil {
			s = wd + string(filepath.Separator) + s
		}
	}
	vol := filepath.VolumeName(s)
	cur := vol + string(filepath.Separator)
	if real, err := filepath.EvalSymlinks(cur); err == nil {
		cur = real
	}
	parts := strings.Split(filepath.ToSlash(s[len(vol):]), "/")
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		next := filepath.Join(cur, part)
		real, err := filepath.EvalSymlinks(next)
		if err != nil {
			return filepath.Join(append([]string{cur}, parts[i:]...)...)
		}
		cur = real
	}
	return cur
}

// homeTreeDigest is the one tree digest every bracket step uses: sha256 over
// the path-sorted list of `<path> NUL <mode> NUL <sha256 of content or link
// target> NUL` for every regular file and symlink under root, excluding the
// top-level .git entry. Paths are slash-separated and relative to root; mode
// is git's (100644, 100755 or 120000). Admission already refuses any untracked
// or ignored file in the home, so on an admitted home the set is exactly the
// tracked tree at the attested source. It reads the file system, not git,
// because a container need not see a linked worktree's git directory. Any
// other kind of entry (a socket, a device) refuses.
func homeTreeDigest(root string) (string, error) {
	type entry struct{ path, mode, sum string }
	var entries []entry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var content []byte
		mode := "100644"
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			content, mode = []byte(target), "120000"
		case info.Mode().IsRegular():
			if content, err = os.ReadFile(p); err != nil {
				return err
			}
			if info.Mode()&0o111 != 0 {
				mode = "100755"
			}
		default:
			return fmt.Errorf("%s is neither a regular file nor a symlink", rel)
		}
		sum := sha256.Sum256(content)
		entries = append(entries, entry{rel, mode, hex.EncodeToString(sum[:])})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("tree digest of %s: %w", root, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	h := sha256.New()
	for _, e := range entries {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", e.path, e.mode, e.sum)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// containerWitnessLines are the lines a container run writes under its
// witness table: Tree-Digest between the table and the attestation line (Task
// 3), the image digest beside it, the attestation last.
func containerWitnessLines(image, tree, attestation string) string {
	return treeDigestPrefix + tree + "\n" + containerImagePrefix + image + "\n" + attestation + "\n"
}

// writeContainerWitness writes a container run's witness to its own new file
// outside the home. Writing it into the brief would change the home, and the
// close step's tree digest would then refuse the bracket.
func writeContainerWitness(path, home, table string) error {
	if rel, ok := relWithin(home, path); ok {
		return fmt.Errorf("refusing --witness %s: it sits inside the home (%s), which would change the tree the bracket binds", path, rel)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(strings.TrimRight(table, "\n") + "\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// runOpenBracket is `verifyrun --open-bracket`: the host's first half. It
// writes the receipt file only when the host derives an executing identity
// for the home and the host check admits it, as version 1 for this home and
// brief, or as version 2 with this home as its admitted deliverable home.
func runOpenBracket(home, brief, out string, stdout, stderr io.Writer, historyOut ...string) int {
	refuse := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "statusgen verifyrun --open-bracket: refused, no receipt written: "+format+"\n", a...)
		return verifyrunExitCouldNot
	}
	if rel, ok := relWithin(home, out); ok {
		return refuse("--out %s sits inside the home (%s); the receipt must be outside it", out, rel)
	}
	// The Runner cell is derived here, on the host, where the home's git
	// directory and the roster are readable. The container reads it from the
	// receipt.
	cell, ok := hostRunnerCell(home, brief)
	if !ok {
		return refuse("could-not-attribute — no executing identity is available on the host for %s (no forge role, no GITHUB_ACTOR under GitHub Actions, no git user.name/user.email). The container run would have no runner to record; set the home's git identity and re-open", home)
	}
	if err := receiptRunnerProblem(cell); err != nil {
		return refuse("%v", err)
	}
	check, err := hostCheckVerifier(home, brief)
	if err != nil {
		return refuse("%v", err)
	}
	_, hostBrief, admitted, err := verifierAdmissionReceipt(check, brief)
	if err != nil {
		return refuse("%v", err)
	}
	if admitted != "" {
		// A cross-repository receipt: the check admitted a deliverable home and
		// moved the brief to its isolated tracking checkout. The home must be
		// --root, and the brief this step was given and the host's tracking
		// brief must each pass the binding check; the Runner cell is then
		// derived for the tracking brief, as the host lane derives it.
		if !sameDirectory(admitted, home) {
			return refuse("the host check admitted the deliverable home %s, not --root %s; a cross-repository container run digests and runs only the admitted home", admitted, home)
		}
		b, err := checkBinding(check)
		if err != nil {
			return refuse("%v", err)
		}
		for _, p := range []string{brief, hostBrief} {
			if err := crossRepoBindingProblem(b, home, p); err != nil {
				return refuse("%v", err)
			}
		}
		if cell, ok = hostRunnerCell(home, hostBrief); !ok {
			return refuse("could-not-attribute — no executing identity is available on the host for %s", home)
		}
		if err := receiptRunnerProblem(cell); err != nil {
			return refuse("%v", err)
		}
	}
	tree, err := homeTreeDigest(home)
	if err != nil {
		return refuse("%v", err)
	}
	opened := nowFunc().UTC().Format(time.RFC3339)
	if len(historyOut) > 0 && historyOut[0] != "" {
		binding, err := checkBinding(check)
		if err != nil {
			return refuse("%v", err)
		}
		if canonicalPath(historyOut[0]) == canonicalPath(out) {
			return refuse("history bundle and admission receipt need different output files")
		}
		if err := exportContainerHistory(home, historyOut[0], binding.Source); err != nil {
			return refuse("%v", err)
		}
	}
	quoted := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	body := `{"check":` + string(check) + `,"tree":"` + tree + `","opened":"` + opened +
		`","runner":` + quoted(cell.Runner) + `,"runner_source":` + quoted(cell.RunnerSource) +
		`,"runner_tree":` + quoted(cell.RunnerTree) + `,"on_behalf_of":` + quoted(cell.OnBehalfOf) + `}` + "\n"
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return refuse("%v", err)
	}
	fmt.Fprintf(stdout, "opened %s tree=%s runner=%s (%s) receipt=%s\n", opened, tree, cell.Runner, cell.RunnerSource, out)
	return verifyrunExitPass
}

// hostCheckVerifier runs the shared admission check on the host and returns
// its JSON output, trimmed and validated as JSON.
func hostCheckVerifier(home, brief string) ([]byte, error) {
	out, err := exec.Command("deskdispatch", "--check-verifier", "--root", home, "--brief", brief).Output()
	if err != nil {
		detail := ""
		if exit, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(exit.Stderr))
		}
		return nil, fmt.Errorf("the host admission check failed: %v: %s", err, detail)
	}
	out = bytes.TrimSpace(out)
	if !json.Valid(out) {
		return nil, fmt.Errorf("the host admission check returned no JSON receipt")
	}
	return out, nil
}

// containerBlock is the container witness at the end of a witness file.
type containerBlock struct {
	lines       []string // the file's lines, without the final newline's empty tail
	attestation string
	brief       string // the attestation's home-relative brief
	tree        string
	rows        []string // the table rows between its own header and Tree-Digest
}

// findContainerBlock reads a container witness file: it must END with the
// witness table and then the Tree-Digest, Container-Image and version-1 or
// version-2 attestation lines, in that order, with at least one sandbox=container-netns
// row in the table directly above. A file that already carries a bracket line
// is refused.
func findContainerBlock(text string) (containerBlock, error) {
	if strings.Contains(text, "\r") {
		return containerBlock{}, fmt.Errorf("the witness contains carriage returns; a container run writes LF lines")
	}
	if !strings.HasSuffix(text, "\n") {
		return containerBlock{}, fmt.Errorf("the witness does not end with a newline")
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), strings.TrimSpace(admissionBracketPrefix)) {
			return containerBlock{}, fmt.Errorf("this witness already carries an %sline", admissionBracketPrefix)
		}
	}
	n := len(lines)
	if n < 4 || !strings.HasPrefix(lines[n-3], treeDigestPrefix) || !strings.HasPrefix(lines[n-2], containerImagePrefix) || !strings.HasPrefix(lines[n-1], attestationPrefix) {
		return containerBlock{}, fmt.Errorf("the witness does not end with the %s, %sand attestation lines a container run records", strings.TrimSpace(treeDigestPrefix), containerImagePrefix)
	}
	tree := strings.TrimPrefix(lines[n-3], treeDigestPrefix)
	if !sha256DigestRe.MatchString(tree) {
		return containerBlock{}, fmt.Errorf("malformed %q", lines[n-3])
	}
	if !sha256DigestRe.MatchString(strings.TrimPrefix(lines[n-2], containerImagePrefix)) {
		return containerBlock{}, fmt.Errorf("malformed %q", lines[n-2])
	}
	att, ok := parseContainerAttestation(lines[n-1])
	if !ok {
		return containerBlock{}, fmt.Errorf("the witness's attestation line is not the version-1 or version-2 form a container run records: %q", lines[n-1])
	}
	// The run, source and tree are compared with the lines inside the span
	// that ends at the append position, never with another table's: a span
	// holding more than one of them is refused before anything else is read.
	if _, header := witnessSpanDigest(lines, n); header >= 0 {
		if _, _, problem := spanOwnLines(lines[header:n]); problem != "" {
			return containerBlock{}, fmt.Errorf("the span that ends at the append position is not bound to one run: %s", problem)
		}
	}
	containerRows, ownHeader := 0, false
	var rows []string
	for i := n - 4; i >= 0 && strings.HasPrefix(lines[i], "|"); i-- {
		if isWitnessHeaderLine(lines[i]) {
			ownHeader = true
			break
		}
		rows = append(rows, lines[i])
		if witnessSandboxOf(lines[i]) == sandboxContainerNetns {
			containerRows++
		}
	}
	if containerRows == 0 {
		return containerBlock{}, fmt.Errorf("no witness row with sandbox=%s sits directly above the %sline", sandboxContainerNetns, treeDigestPrefix)
	}
	if !ownHeader {
		if _, header := witnessSpanDigest(lines, n); header < 0 {
			return containerBlock{}, fmt.Errorf("no witness table header lies above the append position, so the bracket would have no span")
		}
		// The span would start at another table's header and take in that
		// table's Tree-Digest, image and attestation lines.
		return containerBlock{}, fmt.Errorf("the container rows above the %sline carry no witness table header of their own, so the bracket's span would start at another table", treeDigestPrefix)
	}
	return containerBlock{lines: lines, attestation: lines[n-1], brief: att.brief, tree: tree, rows: rows}, nil
}

// containerRowRunnerProblem is the close step's check of the Runner cell of
// every row of the container's own table (rows run upward from the Tree-Digest
// line, so the last one is the header's separator line, which is skipped):
// each must be a witness row whose Runner cell is exactly want, the cell the
// host derived, byte for byte: runner, tree token, on-behalf-of annotation and
// source tag. A row the container wrote under any other identity, tree or
// principal, or with none, refuses the close.
func containerRowRunnerProblem(rows []string, want string) string {
	for i, row := range rows {
		if i == len(rows)-1 && witnessSeparatorLine(row) {
			continue
		}
		cells := witnessCells(row)
		if cells == nil {
			return fmt.Sprintf("the container table carries a row that is not a witness row: %q", row)
		}
		// The row's read-set token is the container's own record of what that
		// row read (build-less-brittle/15); the host derives no such token, so
		// the cell compares without it and must match byte for byte otherwise.
		if cell := evaluation.StripReadSetToken(strings.TrimSpace(cells[witnessCellRunner])); cell != want {
			return fmt.Sprintf("row %s's Runner cell %q is not the cell the host derived and recorded at the open step, %q", strings.TrimSpace(cells[0]), cell, want)
		}
	}
	return ""
}

// witnessSeparatorLine reports whether a table line is a header separator
// (`|---|---|…`).
func witnessSeparatorLine(l string) bool {
	return strings.Trim(strings.TrimSpace(l), "|-: ") == ""
}

// runCloseBracket is `verifyrun --close-bracket`: the host's second half. It
// appends the Admission-Bracket line only when the host check re-admits
// exactly the run the witness and the receipt name AND the home's recomputed
// tree digest equals both the receipt's tree and the witness's Tree-Digest.
// Any mismatch or refusal writes nothing.
func runCloseBracket(home, brief, receiptPath, witnessPath string, stdout, stderr io.Writer) int {
	refuse := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "statusgen verifyrun --close-bracket: refused, nothing appended: "+format+"\n", a...)
		return verifyrunExitCouldNot
	}
	if rel, ok := relWithin(home, witnessPath); ok {
		return refuse("the witness %s sits inside the home (%s)", witnessPath, rel)
	}
	raw, err := os.ReadFile(witnessPath)
	if err != nil {
		return refuse("%v", err)
	}
	info, err := os.Stat(witnessPath)
	if err != nil {
		return refuse("%v", err)
	}
	block, err := findContainerBlock(string(raw))
	if err != nil {
		return refuse("%v", err)
	}
	receipt, receiptLine, _, err := readBracketReceipt(receiptPath, home, brief)
	if err != nil {
		return refuse("%v", err)
	}
	if receiptLine != block.attestation {
		return refuse("the receipt does not name the run the witness names:\n  witness: %s\n  receipt: %s", block.attestation, receiptLine)
	}
	// The home must be the tree the open step admitted before anything is
	// re-derived from it: a moved home refuses here, by digest.
	tree, err := homeTreeDigest(home)
	if err != nil {
		return refuse("%v", err)
	}
	if tree != receipt.Tree {
		return refuse("the home's tree digest %s is not the %s recorded at the open step: the home moved between open and close", tree, receipt.Tree)
	}
	if tree != block.tree {
		return refuse("the home's tree digest %s is not the witness's Tree-Digest %s: the witness was not taken on this home", tree, block.tree)
	}
	// The Runner cell is re-derived on the host, from the same sources the
	// open step read, never taken from the receipt or the witness: a receipt
	// altered after the open step, or a container that wrote another
	// identity, tree or principal into its rows, refuses here. The container
	// witness and the receipt both sit outside the home, whose digest has
	// just matched the receipt's, so an honest home renders the same tree
	// token at close as at open. For a version-1 run none of this starts
	// deskdispatch.
	att, _ := parseContainerAttestation(block.attestation)
	cellBrief := brief
	var check []byte
	if att.version == 2 {
		// A cross-repository run's Runner cell is derived for the tracking
		// brief the host check admits, as at the open step, so the check runs
		// first. Its admitted home must be this home, its binding must be the
		// receipt's field for field (both repositories and both commits
		// included), and the brief given here and the host's tracking brief
		// must each hold the admitted bytes outside the home.
		if check, err = hostCheckVerifier(home, brief); err != nil {
			return refuse("%v", err)
		}
		_, hostBrief, hostHome, err := verifierAdmissionReceipt(check, brief)
		if err != nil {
			return refuse("%v", err)
		}
		if hostHome == "" {
			return refuse("the host check returned a version-1 receipt, but the witness names a cross-repository run")
		}
		if !sameDirectory(hostHome, home) {
			return refuse("the host check admitted the deliverable home %s, not --root %s", hostHome, home)
		}
		hostBinding, err := checkBinding(check)
		if err != nil {
			return refuse("%v", err)
		}
		receiptBinding, err := checkBinding(receipt.Check)
		if err != nil {
			return refuse("%v", err)
		}
		if hostBinding != receiptBinding {
			return refuse("the host check's binding is not the receipt's (repository, source, tracking source, brief, digest, model or tier differ): the receipt was not written for this run, or it was altered")
		}
		for _, p := range []string{brief, hostBrief} {
			if err := crossRepoBindingProblem(hostBinding, home, p); err != nil {
				return refuse("%v", err)
			}
		}
		cellBrief = hostBrief
	}
	host, ok := hostRunnerCell(home, cellBrief)
	if !ok {
		return refuse("could-not-attribute — no executing identity is available on the host for %s, so the runner the receipt names cannot be confirmed", home)
	}
	if host.Runner != receipt.Runner || host.RunnerSource != receipt.RunnerSource {
		return refuse("the receipt names runner %s (%s), but the host derives %s (%s): the receipt was not written by this home's open step, or it was altered", receipt.Runner, receipt.RunnerSource, host.Runner, host.RunnerSource)
	}
	if host.RunnerTree != receipt.RunnerTree || host.OnBehalfOf != receipt.OnBehalfOf {
		return refuse("the receipt names runner tree %q and on-behalf-of %q, but the host derives %q and %q: the receipt was altered, or the home moved between open and close", receipt.RunnerTree, receipt.OnBehalfOf, host.RunnerTree, host.OnBehalfOf)
	}
	if problem := containerRowRunnerProblem(block.rows, receiptRunnerCell(host)); problem != "" {
		return refuse("%s", problem)
	}
	if check == nil {
		if check, err = hostCheckVerifier(home, brief); err != nil {
			return refuse("%v", err)
		}
	}
	hostLine, _, hostHome, err := verifierAdmissionReceipt(check, brief)
	if err != nil {
		return refuse("%v", err)
	}
	if att.version == 1 && hostHome != "" {
		return refuse("the host check returned a cross-repository receipt, but the witness names a version-1 run")
	}
	if hostLine != block.attestation {
		return refuse("the host check does not re-admit the run the witness names (run, source, brief, model or tier differ):\n  witness: %s\n  host:    %s", block.attestation, hostLine)
	}
	opened, _ := time.Parse(time.RFC3339, receipt.Opened)
	closed := nowFunc().UTC()
	if !closed.Truncate(time.Second).After(opened) {
		return refuse("the close at %s is not after the receipt's opened time %s; the bracket's opened= must precede its closed=, so re-run the close once the clock has passed the open", closed.Format(time.RFC3339), receipt.Opened)
	}
	above := strings.Join(block.lines, "\n") + "\n"
	witness, err := bracketDigest(above)
	if err != nil {
		return refuse("%v", err)
	}
	line := fmt.Sprintf("%srun=%s source=%s tree=%s witness=%s opened=%s closed=%s before=receipt after=check-verifier",
		admissionBracketPrefix, att.run, att.source, tree, witness, receipt.Opened, closed.Format(time.RFC3339))
	// Temp file and rename, so an interrupted close never truncates a witness
	// that was already complete.
	if err := writeAtomicPerm(witnessPath, []byte(above+line+"\n"), info.Mode().Perm()); err != nil {
		return refuse("%v", err)
	}
	fmt.Fprintln(stdout, line)
	return verifyrunExitPass
}

// The witness digest of an Admission-Bracket line (desktools-v2/71 Task 1).
// The close step writes it as witness= and the credit rule recomputes it with
// the read-only `statusgen verifyrun --bracket-digest --witness <file>`, so both
// go through witnessSpanDigest.
//
//   - header: a line equal, after trimming, to the first line of witnessHeader
//     or witnessHeaderFailFirst.
//   - span: fixed by the bracket's position alone. It starts at the nearest
//     witness header ABOVE the position and ends at the line immediately
//     before it. For an existing Admission-Bracket line the position is that
//     line; for the close step it is the line about to be appended. Other
//     witness tables in the same file, above or below, are outside the span.
//     A position with no header above it has no span.
//   - normalisation: trailing whitespace stripped, blank lines dropped, lines
//     joined with LF (no final LF), and each table row's Runner cell (the
//     column the span's header names Runner) rewritten in three steps: every
//     exact `(on-behalf-of human:<subject>)` removed (case-sensitive; subject
//     one or more characters other than whitespace and parentheses), every run
//     of spaces and tabs collapsed to one space, the cell trimmed. The bytes
//     between the cell's two delimiters become one space, the value and one
//     space; every other byte of the row is kept. deskevidence stamps exactly
//     that form, before any trailing source tag such as `(git-config)`, so the
//     stamp is outside the digest while every other byte of the span is in it.
//
// It is never a raw sha256sum of the landed bytes, which the stamp changes.

// witnessSpanDigest digests the span that ends just above lines[pos]. header
// is the span's header line index, -1 when no witness header lies above pos.
func witnessSpanDigest(lines []string, pos int) (digest string, header int) {
	header = -1
	for i := pos - 1; i >= 0; i-- {
		if isWitnessHeaderLine(lines[i]) {
			header = i
			break
		}
	}
	if header < 0 {
		return "", -1
	}
	runner := witnessRunnerColumn(lines[header])
	var kept []string
	for i := header; i < pos; i++ {
		l := strings.TrimRight(lines[i], " \t\r\v\f")
		if l == "" {
			continue
		}
		if i > header && strings.HasPrefix(strings.TrimSpace(l), "|") {
			l = normaliseRunnerCell(l, runner)
		}
		kept = append(kept, l)
	}
	sum := sha256.Sum256([]byte(strings.Join(kept, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:]), header
}

// bracketDigest is the close step's digest: the span above the line it is
// about to append to text.
func bracketDigest(text string) (string, error) {
	lines := strings.Split(text, "\n")
	digest, header := witnessSpanDigest(lines, len(lines))
	if header < 0 {
		return "", fmt.Errorf("no witness table header lies above the append position, so the bracket would have no span")
	}
	return digest, nil
}

// bracketCheck is one `--bracket-digest` output line.
type bracketCheck struct {
	bracket, header   int // 1-based line numbers; header 0 when there is none
	witness, recorded string
	verdict           string // match | mismatch | no-table | shared
	why               string // a digest match the span's own lines do not support
	// counts is the span's Tree-Digest, Container-Image and attestation line
	// counts, the lines the credit rule needs one of each of; nil with no span.
	counts []int
}

func (c bracketCheck) String() string {
	header, witness := "none", "none"
	if c.header > 0 {
		header, witness = fmt.Sprintf("L%d", c.header), c.witness
	}
	spanLines := "none"
	if c.counts != nil {
		spanLines = fmt.Sprintf("tree-digest:%d,container-image:%d,attestation:%d", c.counts[0], c.counts[1], c.counts[2])
	}
	return fmt.Sprintf("bracket=L%d header=%s witness=%s recorded=%s verdict=%s span-lines=%s", c.bracket, header, witness, c.recorded, c.verdict, spanLines)
}

var bracketWitnessRe = regexp.MustCompile(`(?:^|\s)witness=(\S+)`)

// bracketChecks checks every Admission-Bracket line in text, in file order.
// A bracket whose nearest header another bracket also has is `shared` (its
// rows would lie in two spans, so neither credits them).
func bracketChecks(text string) []bracketCheck {
	lines := strings.Split(text, "\n")
	var checks []bracketCheck
	byHeader := map[int]int{}
	for i, l := range lines {
		if !strings.HasPrefix(strings.TrimSpace(l), strings.TrimSpace(admissionBracketPrefix)) {
			continue
		}
		c := bracketCheck{bracket: i + 1, recorded: "none"}
		if m := bracketWitnessRe.FindStringSubmatch(l); m != nil {
			c.recorded = m[1]
		}
		digest, header := witnessSpanDigest(lines, i)
		if header >= 0 {
			trees, images, attestations := spanLineCounts(lines[header:i])
			c.counts = []int{len(trees), images, len(attestations)}
			if digest == c.recorded {
				c.why = spanBindingProblem(lines[header:i], l)
			}
		}
		switch {
		case header < 0:
			c.verdict = "no-table"
		case digest == c.recorded && c.why == "":
			c.header, c.witness, c.verdict = header+1, digest, "match"
		default:
			c.header, c.witness, c.verdict = header+1, digest, "mismatch"
		}
		if header >= 0 {
			byHeader[header]++
		}
		checks = append(checks, c)
	}
	for i := range checks {
		if checks[i].header > 0 && byHeader[checks[i].header-1] > 1 {
			checks[i].verdict = "shared"
		}
	}
	return checks
}

// containerRowsOutsideSpans returns the 1-based line numbers of the table rows
// that record sandbox=container-netns and lie in no bracket's span (from a
// bracket's header to the line before it). Such a row is never credited, so
// --bracket-digest does not exit 0 while one is in the file.
func containerRowsOutsideSpans(text string, checks []bracketCheck) []int {
	lines := strings.Split(text, "\n")
	inSpan := make([]bool, len(lines))
	for _, c := range checks {
		if c.header > 0 {
			for i := c.header - 1; i < c.bracket-1 && i < len(lines); i++ {
				inSpan[i] = true
			}
		}
	}
	var out []int
	for i, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "|") && !isWitnessHeaderLine(l) && witnessSandboxOf(l) == sandboxContainerNetns && !inSpan[i] {
			out = append(out, i+1)
		}
	}
	return out
}

// bracketLineRe is the whole Admission-Bracket line the close step writes,
// field for field, after trimming: run, source, tree, witness, opened and
// closed, then the fixed before=receipt after=check-verifier.
var bracketLineRe = regexp.MustCompile(`^Admission-Bracket: run=(\S+) source=(\S+) tree=(\S+) witness=(\S+) opened=(\S+) closed=(\S+) before=receipt after=check-verifier$`)

// spanBindingProblem says why a bracket whose digest matches is still not
// bound to its own span: the Tree-Digest, Container-Image and attestation lines
// credited with it must be the ones inside that span, one of each, and the
// bracket line must be the close step's form with its tree= equal to that
// span's Tree-Digest and its run= and source= equal to that span's
// attestation, never a line from another bracket's span. Its opened= and
// closed= must be UTC RFC 3339 times with opened strictly before closed, and
// the span's Tree-Digest and Container-Image values sha256 digests. The span's
// digest does not cover the bracket line itself, so without these checks an
// edit to run=, source=, opened= or closed= after landing would go unnoticed.
// It is "" when the span binds.
func spanBindingProblem(span []string, bracket string) string {
	tree, attestation, problem := spanOwnLines(span)
	if problem != "" {
		return problem
	}
	if !sha256DigestRe.MatchString(tree) {
		return fmt.Sprintf("its own span's %s %s is not a sha256:<64 hex> digest", strings.TrimSpace(treeDigestPrefix), tree)
	}
	if image := spanImage(span); !sha256DigestRe.MatchString(image) {
		return fmt.Sprintf("its own span's %s %s is not a sha256:<64 hex> digest", strings.TrimSpace(containerImagePrefix), image)
	}
	b := bracketLineRe.FindStringSubmatch(strings.TrimSpace(bracket))
	if b == nil {
		return "the bracket line is not the form the close step writes (run= source= tree= witness= opened= closed= before=receipt after=check-verifier)"
	}
	if b[3] != tree {
		return fmt.Sprintf("the bracket's tree=%s is not its own span's %s %s", b[3], strings.TrimSpace(treeDigestPrefix), tree)
	}
	a, ok := parseContainerAttestation(attestation)
	if !ok {
		return "its own span's attestation line is not the version-1 or version-2 form a container run records"
	}
	if b[1] != a.run || b[2] != a.source {
		return fmt.Sprintf("the bracket's run=%s source=%s are not its own span's attestation run=%s source=%s", b[1], b[2], a.run, a.source)
	}
	opened, err := parseBracketTime(b[5])
	if err != nil {
		return fmt.Sprintf("the bracket's opened=%s %v", b[5], err)
	}
	closed, err := parseBracketTime(b[6])
	if err != nil {
		return fmt.Sprintf("the bracket's closed=%s %v", b[6], err)
	}
	if !closed.After(opened) {
		return fmt.Sprintf("the bracket's opened=%s is not before its closed=%s", b[5], b[6])
	}
	return ""
}

// parseBracketTime reads an opened= or closed= value: the UTC RFC 3339 form
// the open and close steps write, and nothing else.
func parseBracketTime(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil || t.UTC().Format(time.RFC3339) != v {
		return time.Time{}, fmt.Errorf("is not a UTC RFC 3339 time")
	}
	return t, nil
}

// spanOwnLines reads a span's Tree-Digest, Container-Image and attestation
// lines. It returns the span's one Tree-Digest value and its one attestation
// line, or why the span does not hold exactly one line of each kind.
func spanOwnLines(span []string) (tree, attestation, problem string) {
	trees, images, attestations := spanLineCounts(span)
	if len(trees) != 1 || images != 1 || len(attestations) != 1 {
		return "", "", fmt.Sprintf("the span holds %d %s, %d %s and %d attestation lines, not one of each", len(trees), strings.TrimSpace(treeDigestPrefix), images, strings.TrimSpace(containerImagePrefix), len(attestations))
	}
	return trees[0], attestations[0], ""
}

// spanLineCounts returns a span's Tree-Digest values, its Container-Image line
// count and its attestation lines, each trimmed.
func spanLineCounts(span []string) (trees []string, images int, attestations []string) {
	for _, l := range span {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, strings.TrimSpace(treeDigestPrefix)):
			trees = append(trees, strings.TrimSpace(strings.TrimPrefix(l, strings.TrimSpace(treeDigestPrefix))))
		case strings.HasPrefix(l, strings.TrimSpace(containerImagePrefix)):
			images++
		case strings.HasPrefix(l, attestationPrefix):
			attestations = append(attestations, l)
		}
	}
	return trees, images, attestations
}

// spanImage returns the value of a span's first Container-Image line, trimmed.
func spanImage(span []string) string {
	for _, l := range span {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, strings.TrimSpace(containerImagePrefix)) {
			return strings.TrimSpace(strings.TrimPrefix(l, strings.TrimSpace(containerImagePrefix)))
		}
	}
	return ""
}

// isWitnessHeaderLine reports whether a line, trimmed, is the first line of a
// verifyrun witness header, plain or fail-first.
func isWitnessHeaderLine(l string) bool {
	l = strings.TrimSpace(l)
	for _, header := range []string{witnessHeader, witnessHeaderFailFirst} {
		if first, _, _ := strings.Cut(header, "\n"); l == first {
			return true
		}
	}
	return false
}

// witnessRunnerColumn is the Runner cell's index in a witness header line.
func witnessRunnerColumn(header string) int {
	for i, c := range splitRowEscaped(header) {
		if strings.TrimSpace(c) == "Runner" {
			return i
		}
	}
	return -1
}

// onBehalfOfStampRe is exactly the stamp deskevidence and verifyrun write.
// Nothing looser matches: `(On-Behalf-Of human:x)`, `(on-behalf-of human: x)`
// or `(on-behalf-of <desk role>)` stay in the digest.
var onBehalfOfStampRe = regexp.MustCompile(`\(on-behalf-of human:[^\s()]+\)`)

var cellWhitespaceRe = regexp.MustCompile(`[ \t]+`)

// normaliseRunnerCell rewrites a table row's Runner cell (cell index col,
// counted as splitRowEscaped counts them: `\|` is cell content): stamp
// removed, whitespace runs collapsed, trimmed. The rest of the row is kept
// byte for byte. A row with too few cells is returned unchanged.
func normaliseRunnerCell(row string, col int) string {
	if col < 0 {
		return row
	}
	var pipes []int
	for i := 0; i < len(row); i++ {
		switch {
		case row[i] == '\\' && i+1 < len(row) && row[i+1] == '|':
			i++
		case row[i] == '|':
			pipes = append(pipes, i)
		}
	}
	// The row's leading pipe opens cell 0, so cell col sits between pipes
	// col and col+1.
	if !strings.HasPrefix(strings.TrimSpace(row), "|") || len(pipes) < col+2 {
		return row
	}
	cell := onBehalfOfStampRe.ReplaceAllString(row[pipes[col]+1:pipes[col+1]], "")
	cell = strings.TrimSpace(cellWhitespaceRe.ReplaceAllString(cell, " "))
	return row[:pipes[col]+1] + " " + cell + " " + row[pipes[col+1]:]
}
