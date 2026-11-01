package main

// readset.go — the landing half of build-less-brittle/15's read-set
// introduction binding. A witness row whose Runner cell carries a
// `(reads sha256:<hex>)` token names a read-set file
// `<brief dir>/reads/<hex>`; coverage honours that file only when it
// arrives in the same commit as the row. The Contents-API write this tool
// lands through commits one file, so it cannot pair the two, and the landing
// refuses before any forge call:
//
//   - a named file absent from the local tree, or whose content hashes to
//     another digest, refuses as a broken pairing;
//   - a present and matching file still refuses, because the row and its file
//     cannot land in one commit here. Landing the row alone would leave a
//     token that names a file the branch does not hold.
//
// A could-not-check token names no file and lands as any other row does.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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

// refuseUnpairedReadSets is the pre-write landing check: nil when content
// names no read-set file, a refusal otherwise (see the file comment). root is
// the local tree the read-set files resolve against; target is the
// repository path the brief lands at.
func refuseUnpairedReadSets(content []byte, root, target string) error {
	digests := readSetDigests(content)
	if len(digests) == 0 {
		return nil
	}
	dir := path.Join(path.Dir(target), "reads")
	var broken []string
	for _, d := range digests {
		rel := dir + "/" + d
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		switch {
		case err != nil:
			broken = append(broken, rel+" is absent locally")
		default:
			sum := sha256.Sum256(b)
			if got := hex.EncodeToString(sum[:]); got != d {
				broken = append(broken, fmt.Sprintf("%s hashes to %s", rel, got))
			}
		}
	}
	if len(broken) > 0 {
		return deskkit.Refused("refused: " + target + " names read-set files it cannot land with: " +
			strings.Join(broken, "; ") + " — a row and its read-set file land together or not at all")
	}
	return deskkit.Refused("refused: " + target + " names read-set file(s) " + dir + "/{" + strings.Join(digests, ",") +
		"} and this landing commits one file, so the row and its read-set file cannot land in one commit " +
		"(build-less-brittle/15 Task step 7) — land them through an Evidence PR that adds both in one commit")
}
