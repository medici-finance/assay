package main

// Certification provenance.
//
// A clean sweep prints a CERTIFICATE naming the tree's content hash. The hash is
// over the ASSEMBLED tree — every regular file's relative path and bytes — so a
// certificate is bound to the exact tree it was produced against and cannot be
// re-used for a different one. Change one byte in one file and the hash changes,
// which is the property that stops a stale "clean" result from authorising a tree
// it never saw.
//
// The tool also reads the pubmanifest stage marker (the sibling
// `<tree>.pubmanifest-stage` file, written by `pubmanifest stage`) when present,
// and records it in the certificate as provenance. This is how the gate composes
// with the stager: a tree the stager REFUSED to build is never written, so
// `leaksweep --tree <that path>` finds no tree and reports could-not-check — it
// cannot certify what the stager refused to produce. When a marker IS present and
// valid, the certificate says so; when it is absent (for example the source-repo
// self-test), the sweep still runs but the certificate records that the tree
// carries no stage provenance.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// contentHash is a stable digest of the assembled tree: for each file, in sorted
// path order, the relative path, a NUL, the file length, a NUL, and the bytes.
// Including the path and length as well as the content means a rename or a
// truncation changes the hash too, not only an edit to a file's body.
func contentHash(c *corpus) string {
	h := sha256.New()
	for _, f := range c.files {
		fmt.Fprintf(h, "%s\x00%d\x00", f.path, len(f.ex.text))
		// Hash the RAW bytes on disk, not the extracted text: two different PDFs
		// can extract to the same text but are not the same tree.
		data, err := os.ReadFile(filepath.Join(c.tree, filepath.FromSlash(f.path)))
		if err != nil {
			// Unreadable files are already surfaced as could-not-check; fold the
			// error into the hash so the digest still changes rather than silently
			// hashing nothing for that path.
			fmt.Fprintf(h, "<unreadable:%v>", err)
			continue
		}
		h.Write(data)
	}
	// Unsearchable binaries (#528) are part of the tree too. The sweep refuses to
	// certify any tree that contains one, so this never feeds a CERTIFICATE; but
	// the digest must still distinguish two trees that differ only in such a file,
	// or a NOT CERTIFIED line's hash could repeat across different dirty trees.
	for _, p := range c.unsearchable {
		fmt.Fprintf(h, "%s\x00", p)
		data, err := os.ReadFile(filepath.Join(c.tree, filepath.FromSlash(p)))
		if err != nil {
			fmt.Fprintf(h, "<unreadable:%v>", err)
			continue
		}
		fmt.Fprintf(h, "%d\x00", len(data))
		h.Write(data)
	}
	// Non-regular files (symlinks) too. git stores a symlink as a blob of its
	// target string, so that string — not any linked content — is what would ship;
	// hash it (via Readlink) so two trees whose symlinks differ in target do not
	// share a digest.
	for _, p := range c.nonRegular {
		fmt.Fprintf(h, "%s\x00", p)
		target, err := os.Readlink(filepath.Join(c.tree, filepath.FromSlash(p)))
		if err != nil {
			fmt.Fprintf(h, "<unreadable:%v>", err)
			continue
		}
		fmt.Fprintf(h, "%d\x00", len(target))
		h.Write([]byte(target))
	}
	return hex.EncodeToString(h.Sum(nil))
}

const stageMarkerSuffix = ".pubmanifest-stage"

// readProvenance returns a short description of the pubmanifest stage marker for
// the tree, or "" if there is none. It never fails the run: absence of a marker
// is not a defect (the tool runs against un-staged trees too), only a fact
// recorded in the certificate.
func readProvenance(tree string) string {
	abs, err := filepath.Abs(tree)
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(abs + stageMarkerSuffix)
	if err != nil {
		return ""
	}
	target := ""
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "target: ") {
			target = strings.TrimSpace(strings.TrimPrefix(line, "target: "))
		}
	}
	if target == abs {
		return "pubmanifest stage marker verified for this tree"
	}
	if target != "" {
		return fmt.Sprintf("pubmanifest stage marker present but authorises %q, not this tree", target)
	}
	return "pubmanifest stage marker present"
}
