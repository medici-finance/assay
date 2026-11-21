package deskkit

import (
	"bytes"
	"fmt"
	"path"
	"strings"
)

// companionPlan checks in.Companions' paths: each repo-relative, clean,
// distinct and not File itself. Both backends run it before any request.
func companionPlan(in WriteFileInput) error {
	seen := map[string]bool{in.File: true}
	for _, c := range in.Companions {
		if c.File == "" || path.IsAbs(c.File) || path.Clean(c.File) != c.File || c.File == ".." || strings.HasPrefix(c.File, "../") || seen[c.File] {
			return Refused(fmt.Sprintf("refusing a write to %s on %s: companion path %q is not a distinct, clean repo-relative path", in.File, in.Branch, c.File))
		}
		seen[c.File] = true
	}
	return nil
}

// companionWanted reads one companion as the base holds it: true when it
// must be added, false when the base already holds the same bytes, and a
// refusal when the base holds anything else there.
func companionWanted(read func(ReadFileInput) (*FileContent, error), c CompanionFile, branch, ref string) (bool, error) {
	cur, err := read(ReadFileInput{File: c.File, Ref: ref})
	if err != nil {
		if IsForgeNotFound(err) {
			return true, nil
		}
		return false, err
	}
	if cur == nil || !cur.Exists {
		return true, nil
	}
	if cur.Kind == FileKindRegular && bytes.Equal(cur.Content, c.Content) {
		return false, nil
	}
	return false, Refused(fmt.Sprintf("refusing a paired write: %s on %s already holds other content than the companion file this commit adds", c.File, branch))
}
