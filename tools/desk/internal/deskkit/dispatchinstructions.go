package deskkit

// dispatchinstructions.go — the one value cellctl composes and deskdispatch reads to quote a
// role's own dispatch-instruction file into every prompt of a kit (desk-containers/30).
//
// cellctl resolves a role's `dispatch_instructions` declaration (kit → file) at launch, reads each
// file and sets ASSAY_DISPATCH_INSTRUCTIONS in the window environment. deskdispatch, run from that
// window, looks up its own kit, re-checks custody, re-reads the file and refuses when the bytes no
// longer hash to the digest cellctl recorded. Both binaries read the grammar from here, and the kit
// names from DispatchKits(), so neither restates either.

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// EnvDispatchInstructions carries the role's dispatch-instruction files from the launcher to
// deskdispatch: comma-separated `<kit>=<absolute path>@sha256:<64 lowercase hex>`.
const EnvDispatchInstructions = "ASSAY_DISPATCH_INSTRUCTIONS"

// dispatchInstructionsDigestSep separates an entry's path from its digest. An entry splits on its
// LAST occurrence, and a path that itself contains it cannot be carried.
const dispatchInstructionsDigestSep = "@sha256:"

// dispatchInstructionsDigestRe is the digest grammar, shared with the dispatch record's
// dispatch_instructions_sha256 field.
var dispatchInstructionsDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// DispatchInstruction is one entry of the value: the kit it serves, the file's absolute path and
// the sha256 of the bytes read at launch.
type DispatchInstruction struct {
	Kit    string
	Path   string
	SHA256 string
}

// DispatchInstructionPathCarriable reports why path cannot be carried in the value, or nil. A
// path containing `,` would split into two entries and one containing `@sha256:` would split in
// the wrong place; no escaping exists, so such a file must be renamed.
func DispatchInstructionPathCarriable(path string) error {
	for _, bad := range []string{",", dispatchInstructionsDigestSep} {
		if strings.Contains(path, bad) {
			return fmt.Errorf("the path %q contains %q, which %s cannot carry — rename the file", path, bad, EnvDispatchInstructions)
		}
	}
	return nil
}

// FormatDispatchInstructions composes the value from entries, in the order given.
func FormatDispatchInstructions(entries []DispatchInstruction) string {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, e.Kit+"="+e.Path+dispatchInstructionsDigestSep+e.SHA256)
	}
	return strings.Join(parts, ",")
}

// ParseDispatchInstructions parses the value. An empty value is no entries. Every entry must
// split into exactly one kit, one absolute path and one digest; a kit outside DispatchKits(), a
// kit given twice, a relative path and a digest that is not 64 lowercase hex are each an error.
func ParseDispatchInstructions(v string) ([]DispatchInstruction, error) {
	if v == "" {
		return nil, nil
	}
	var out []DispatchInstruction
	seen := map[string]bool{}
	for i, entry := range strings.Split(v, ",") {
		n := i + 1
		kit, rest, ok := strings.Cut(entry, "=")
		if !ok || kit == "" {
			return nil, fmt.Errorf("%s entry %d (%q) is not <kit>=<absolute path>@sha256:<digest>", EnvDispatchInstructions, n, entry)
		}
		if !containsString(dispatchKits, kit) {
			return nil, fmt.Errorf("%s entry %d names kit %q, which is not one of %s", EnvDispatchInstructions, n, kit, strings.Join(dispatchKits, ", "))
		}
		if seen[kit] {
			return nil, fmt.Errorf("%s names kit %q more than once", EnvDispatchInstructions, kit)
		}
		seen[kit] = true
		at := strings.LastIndex(rest, dispatchInstructionsDigestSep)
		if at < 0 {
			return nil, fmt.Errorf("%s entry %d (kit %s) carries no %s<digest> after its path", EnvDispatchInstructions, n, kit, dispatchInstructionsDigestSep)
		}
		path, digest := rest[:at], rest[at+len(dispatchInstructionsDigestSep):]
		if err := DispatchInstructionPathCarriable(path); err != nil {
			return nil, fmt.Errorf("%s entry %d (kit %s): %v", EnvDispatchInstructions, n, kit, err)
		}
		if path == "" || !filepath.IsAbs(path) {
			return nil, fmt.Errorf("%s entry %d (kit %s): the path %q is not absolute", EnvDispatchInstructions, n, kit, path)
		}
		if !dispatchInstructionsDigestRe.MatchString(digest) {
			return nil, fmt.Errorf("%s entry %d (kit %s, file %s): the digest %q is not 64 lowercase hex", EnvDispatchInstructions, n, kit, path, digest)
		}
		out = append(out, DispatchInstruction{Kit: kit, Path: path, SHA256: digest})
	}
	if len(out) == 0 {
		return nil, errors.New(EnvDispatchInstructions + " holds no entry")
	}
	return out, nil
}

// ValidDispatchInstructionsDigest reports whether d is 64 lowercase hex.
func ValidDispatchInstructionsDigest(d string) bool { return dispatchInstructionsDigestRe.MatchString(d) }
