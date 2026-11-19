package main

// readset.go — the landing half of the read-set introduction binding
// (#2026). A witness row whose Runner cell carries
// a `(reads sha256:<hex>)` token names a read-set file
// `<brief dir>/reads/<hex>`; coverage honours that file only when it arrives
// in the same commit as its row. The recorder writes the file under the run's
// state directory, `<git dir>/assay-read-sets/<hex>`, never beside the brief,
// so the landing takes it from there and commits it with the row:
//
//   - captureReadSets runs before any forge call. It resolves the state
//     directory of --root with `git rev-parse --git-path` and copies every
//     named file into memory, checking its digest. This is the copy before
//     cleanup: a linked worktree's git dir, and the state directory in it, is
//     deleted with the worktree, so nothing after this point reads the disk.
//   - pairReadSets runs before the write. Each digest the commit introduces
//     (named by the landed content, not by the branch's) must have a verified
//     copy; one absent from the state directory, or hashing to another digest,
//     refuses the landing before any write. The rest ride the write as
//     companions, which the forge commits with the brief in ONE commit.
//
// A digest the branch already names was introduced by an earlier commit and
// needs no file here. A could-not-check token names no file and lands as any
// other row does.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// readSetTokenRe matches a Runner cell's read-set token naming a file.
var readSetTokenRe = regexp.MustCompile(`\(reads sha256:([0-9a-f]{64})\)`)

// readSetDigests lists the distinct read-set digests content names.
func readSetDigests(content []byte) []string {
	seen := map[string]bool{}
	for _, m := range readSetTokenRe.FindAllSubmatch(content, -1) {
		seen[string(m[1])] = true
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// readSetCapture is what captureReadSets copied from the state directory.
type readSetCapture struct {
	dir    string            // the resolved state directory, for messages
	files  map[string][]byte // digest -> the bytes that hash to it
	broken map[string]string // digest -> why no verified copy exists
	err    error             // the state directory could not be resolved
}

// readSetStateDirFn resolves root's state directory; a test seam.
var readSetStateDirFn = readSetStateDir

// readSetStateDir asks git through deskkit's verifier read seam (no new git
// spawn), so the answer is the per-worktree git dir the recorder writes under,
// never the common dir. root is made absolute first, so the process cwd never
// decides which checkout is asked.
func readSetStateDir(root string) (string, error) {
	abs, aerr := filepath.Abs(root)
	if aerr != nil {
		return "", fmt.Errorf("git rev-parse --git-path %s in %s did not name a directory", evaluation.ReadSetStateDir, root)
	}
	dir, err := deskkit.VerifierGitPath(abs, evaluation.ReadSetStateDir)
	if err != nil || !filepath.IsAbs(dir) {
		return "", fmt.Errorf("git rev-parse --git-path %s in %s did not name a directory", evaluation.ReadSetStateDir, root)
	}
	return dir, nil
}

// captureReadSets copies, before any forge call, every read-set file content
// names from root's state directory (see the file comment).
func captureReadSets(content []byte, root string) *readSetCapture {
	c := &readSetCapture{files: map[string][]byte{}, broken: map[string]string{}}
	digests := readSetDigests(content)
	if len(digests) == 0 {
		return c
	}
	dir, err := readSetStateDirFn(root)
	if err != nil {
		c.err = err
		return c
	}
	c.dir = dir
	for _, d := range digests {
		b, why := readStateFile(filepath.Join(dir, d))
		switch {
		case why != "":
			c.broken[d] = why
		case evaluation.ReadSetDigest(b) != d:
			c.broken[d] = "hashes to " + evaluation.ReadSetDigest(b)
		default:
			c.files[d] = b
		}
	}
	return c
}

// readStateFile reads one regular file without following a link at its name.
func readStateFile(p string) ([]byte, string) {
	fi, err := os.Lstat(p)
	if err != nil {
		return nil, "is absent"
	}
	if !fi.Mode().IsRegular() {
		return nil, "is not a regular file"
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, "cannot be read"
	}
	defer f.Close()
	if held, err := f.Stat(); err != nil || !os.SameFile(fi, held) {
		return nil, "changed while it was read"
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		return nil, "cannot be read"
	}
	return buf.Bytes(), ""
}

// pairReadSets returns the read-set files the landing commits with target, or
// a refusal before any write when one the commit introduces has no verified
// copy (see the file comment).
func pairReadSets(c *readSetCapture, commit, remote []byte, target string) ([]deskkit.CompanionFile, error) {
	landed := map[string]bool{}
	for _, d := range readSetDigests(remote) {
		landed[d] = true
	}
	var introduced []string
	for _, d := range readSetDigests(commit) {
		if !landed[d] {
			introduced = append(introduced, d)
		}
	}
	if len(introduced) == 0 {
		return nil, nil
	}
	if c == nil || c.err != nil {
		why := "no state directory was read"
		if c != nil {
			why = c.err.Error()
		}
		return nil, deskkit.Refused("refused: " + target + " introduces read-set file(s) " + strings.Join(introduced, ",") +
			" and the run's state directory cannot be located: " + why + ". Land from the verifier execution checkout, before the worktree is removed")
	}
	stream := path.Dir(target)
	var broken []string
	var out []deskkit.CompanionFile
	for _, d := range introduced {
		b, ok := c.files[d]
		if !ok {
			why := c.broken[d]
			if why == "" {
				why = "was not read"
			}
			broken = append(broken, filepath.Join(c.dir, d)+" "+why)
			continue
		}
		out = append(out, deskkit.CompanionFile{File: evaluation.ReadSetPath(stream, d), Content: b})
	}
	if len(broken) > 0 {
		return nil, deskkit.Refused("refused: " + target + " names read-set files it cannot land with: " +
			strings.Join(broken, "; ") + ". The landing takes each file from the run's state directory (git rev-parse --git-path " +
			evaluation.ReadSetStateDir + "), which is deleted with a linked worktree: land before the worktree is removed")
	}
	return out, nil
}
