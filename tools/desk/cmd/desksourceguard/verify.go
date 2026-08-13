// Command desksourceguard verifies that the desk-tools SOURCE tree a consumer
// materialised is the exact commit its `.assay-versions` pins — and that that
// commit is the one the hash-pinned BINARIES were built from.
//
// WHY THIS EXISTS (#519). The consumer install path pins two things
// two different ways. The binary half is tag + sha256: `.assay-versions` records
// a per-platform digest and the consumer hard-exits on a mismatch, so moving a
// release tag cannot change which bytes run. The source half was tag ONLY — the
// consumer cloned `tools/desk` at `--branch <tag>` with no hash, and that tree is
// what its `gate-pins` job compiles and runs. A release tag is a mutable ref, so
// an actor able to repoint one could change the source a consumer's own gate
// compiles, with nothing in the consumer's diff to show for it: the reviewed tree
// and the compiled tree could differ, after review.
//
// The property this guard enforces is "the tree compiled is the tree that was
// reviewed, and it cannot be re-resolved afterwards". A commit SHA is content-
// addressed and immutable, so it is the anchor; the tag stays in the pin only as
// the human-legible name, and is no longer trusted to answer WHICH commit.
//
// Three agreements are required, all fail-closed:
//
//  1. `desk-tools-source` and `desk-tools-<platform>` name the SAME tag — the two
//     halves of one pin can never drift into naming different releases.
//  2. The materialised clone's HEAD equals the pinned commit — a repointed tag
//     goes RED here instead of silently substituting a tree.
//  3. The pinned commit matches the `SourceSHA` stamped into THIS binary. The
//     binary arrived inside a tarball whose sha256 the consumer already verified
//     against `.assay-versions`, so its stamp is hash-anchored: it cannot be
//     re-resolved, and it is what ties the source pin to the binary pin. Without
//     (3) a consumer could pin a source commit unrelated to the binaries it runs
//     and every other check would still pass.
//
// Ordering is load-bearing: this guard is a hash-verified BINARY, so it runs
// before any line of the cloned source is compiled or executed. A check living
// in the cloned tree could be edited by the same repoint it is meant to catch.
//
// Exit codes follow the desk-tools contract: 0 agree, 5 refused (a check
// positively failed), 6 unverifiable (a precondition could not be read). Nothing
// collapses to 0.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// sourceArtifact is the `.assay-versions` artifact name carrying the source pin.
// It is deliberately a sibling of the `desk-tools-<platform>` binary lines and
// uses the identical three-field shape (artifact / tag / digest), so the pin file
// stays one grammar. The digest column holds a git commit SHA rather than a
// sha256 — both are content addresses of the thing named.
const sourceArtifact = "desk-tools-source"

// fullSHA matches a complete 40-hex git object name. Abbreviations are refused:
// a short SHA is a prefix query against the repository, so it can become
// ambiguous as history grows, and an ambiguous anchor is not an anchor.
var fullSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// shortSHA is the shape release-desk.yml stamps in (`git rev-parse --short HEAD`).
// Seven hex is git's floor; the guard refuses anything shorter as too weak to bind
// a commit.
var shortSHA = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// execCommand is the seam for testing — production binds exec.Command.
var execCommand = exec.Command

type options struct {
	repoRoot string
	source   string
	platform string
}

// defaultPlatform maps the running binary back to the `.assay-versions` artifact
// suffix. It is derived rather than defaulted to a constant so a guard running on
// an unexpected runner image refuses instead of reading someone else's pin.
func defaultPlatform() (string, error) {
	arch := runtime.GOARCH
	switch arch {
	case "amd64", "arm64":
	default:
		return "", deskkit.Unverifiable("unsupported GOARCH "+arch+
			" — no desk-tools artifact is published for it, so no pin can be read", nil)
	}
	switch runtime.GOOS {
	case "linux", "darwin":
	default:
		return "", deskkit.Unverifiable("unsupported GOOS "+runtime.GOOS+
			" — no desk-tools artifact is published for it, so no pin can be read", nil)
	}
	return runtime.GOOS + "-" + arch, nil
}

// pin is one `.assay-versions` line: artifact, tag, digest.
type pin struct {
	tag    string
	digest string
}

// readPin returns the pin for exactly one artifact name. The trailing space in the
// prefix match is load-bearing and matches how every consumer workflow selects a
// pin: without it `desk-tools-source` would also match `desk-tools-source-foo`,
// and `statusgen ` would match `statusgen-linux-amd64`.
func readPin(pinFile, artifact string) (pin, error) {
	raw, err := os.ReadFile(pinFile)
	if err != nil {
		return pin{}, deskkit.Unverifiable("cannot read "+pinFile+
			" — the pin file is the only record of which release this consumer runs", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, artifact+" ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return pin{}, deskkit.Unverifiable(fmt.Sprintf(
				"malformed %q pin in %s: %q — expected `<artifact> <tag> <digest>`",
				artifact, pinFile, line), nil)
		}
		return pin{tag: fields[1], digest: fields[2]}, nil
	}
	return pin{}, deskkit.Unverifiable("no "+artifact+" pin in "+pinFile, nil)
}

// headOf resolves the commit a materialised checkout is actually at. It asks git
// rather than reading a file so a detached, shallow, sparse clone — which is what
// the consumer action produces — answers correctly.
func headOf(dir string) (string, error) {
	cmd := execCommand("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", deskkit.Unverifiable("cannot resolve HEAD of the materialised source at "+dir+
			" — without it the compiled tree cannot be identified", err)
	}
	head := strings.TrimSpace(string(out))
	if !fullSHA.MatchString(head) {
		return "", deskkit.Unverifiable("git reported a HEAD that is not a 40-hex commit for "+dir+
			": "+fmt.Sprintf("%q", head), nil)
	}
	return head, nil
}

// verify performs the three agreements. stampedSHA is the SourceSHA compiled into
// this binary; it is a parameter rather than a direct read of deskkit.SourceSHA so
// the negative cases are testable without rebuilding.
func verify(opts options, stampedSHA string, out *strings.Builder) error {
	pinFile := filepath.Join(opts.repoRoot, ".assay-versions")

	binaryPin, err := readPin(pinFile, "desk-tools-"+opts.platform)
	if err != nil {
		return err
	}
	sourcePin, err := readPin(pinFile, sourceArtifact)
	if err != nil {
		return err
	}

	// (1) one release, not two.
	if sourcePin.tag != binaryPin.tag {
		return deskkit.Refused(fmt.Sprintf(
			"pin split: %s names tag %s but desk-tools-%s names tag %s in %s. "+
				"The source and the binaries must come from ONE release; bump both lines together.",
			sourceArtifact, sourcePin.tag, opts.platform, binaryPin.tag, pinFile))
	}

	if !fullSHA.MatchString(sourcePin.digest) {
		return deskkit.Unverifiable(fmt.Sprintf(
			"%s pin in %s holds %q, which is not a full 40-hex commit SHA. "+
				"An abbreviated or non-SHA anchor can be re-resolved, which is the whole point of the pin.",
			sourceArtifact, pinFile, sourcePin.digest), nil)
	}

	// (3) the source pin and the hash-pinned binaries name the same commit.
	// Checked BEFORE touching the clone: it needs no filesystem and it is the
	// agreement that makes the pinned SHA trustworthy in the first place.
	if stampedSHA == "" {
		return deskkit.Unverifiable(
			"this desksourceguard binary carries no SourceSHA stamp — it is an unstamped local "+
				"build (`go run`/`go build` without the release -ldflags), so it cannot bind the "+
				"source pin to a hash-verified artifact. Run the binary from the pinned release tarball.", nil)
	}
	if !shortSHA.MatchString(stampedSHA) {
		return deskkit.Unverifiable(fmt.Sprintf(
			"this binary's SourceSHA stamp %q is not >=7 hex characters — too weak to bind a commit",
			stampedSHA), nil)
	}
	if !strings.HasPrefix(sourcePin.digest, stampedSHA) {
		return deskkit.Refused(fmt.Sprintf(
			"binary/source pin disagreement: the sha256-verified desk-tools binaries were built from "+
				"commit %s…, but %s pins commit %s in %s. The pinned source is NOT the source these "+
				"binaries came from.",
			stampedSHA, sourceArtifact, sourcePin.digest, pinFile))
	}

	// (2) the tree that will be compiled is the pinned commit.
	head, err := headOf(opts.source)
	if err != nil {
		return err
	}
	if head != sourcePin.digest {
		return deskkit.Refused(fmt.Sprintf(
			"SOURCE PIN MISMATCH: the tree materialised at %s is commit %s, but %s pins %s "+
				"(tag %s) in %s.\n"+
				"A release tag is a mutable ref. Either that tag has been REPOINTED since the pin "+
				"was reviewed — in which case the tree about to be compiled is not the reviewed one, "+
				"and this is a supply-chain event, not a chore — or the pin is stale and must be "+
				"bumped deliberately. This guard refuses rather than compile an unreviewed tree.",
			opts.source, head, sourceArtifact, sourcePin.digest, sourcePin.tag, pinFile))
	}

	fmt.Fprintf(out, "desksourceguard: OK — %s at %s\n", sourcePin.tag, head)
	fmt.Fprintf(out, "  source pin (%s)   : %s\n", pinFile, sourcePin.digest)
	fmt.Fprintf(out, "  materialised HEAD    : %s\n", head)
	fmt.Fprintf(out, "  binary stamp SourceSHA: %s (from the sha256-verified tarball)\n", stampedSHA)
	return nil
}
