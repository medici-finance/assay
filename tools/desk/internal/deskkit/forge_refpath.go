package deskkit

import (
	"fmt"
	"strings"
)

// forge_refpath.go — the validator that keeps DeleteRef a TYPED operation rather than a
// passthrough wearing a typed signature.
//
// DeleteRef is the one frozen op whose argument is a path-shaped string, and that is exactly
// the shape an arbitrary-endpoint escape hatch takes: the callsite it replaces was
// `gh api -X DELETE repos/<owner>/<repo>/git/refs/dispatch/<key>`, a call that could as
// easily have addressed `/repos/<o>/<r>/branches/main/protection`. What stops the typed op
// from being the same hole with better manners is that the caller supplies only the tail
// INSIDE the repo's ref namespace, and that tail is checked here before any request is
// built. A ref that could traverse out of the namespace, carry a query string, or smuggle a
// second path segment past an encoder is REFUSED — the operation never runs, so a bad ref is
// a refusal rather than a request to somewhere else.
//
// The rules below are git's own check-ref-format restrictions, kept deliberately STRICTER
// than git where the extra strictness costs nothing: git permits a single-component ref,
// this refuses one, because every ref a desk tool addresses is namespaced
// ("heads/…", "tags/…", "dispatch/…") and a bare component is far more likely to be a
// caller that meant to pass a branch name.

// ValidateRefPath checks a ref path and returns it in the canonical form the backends
// address: the path WITHOUT a leading "refs/", e.g. "heads/topic" or "dispatch/item--01".
// Both spellings are accepted on input ("refs/heads/x" and "heads/x") so a caller holding a
// fully-qualified ref does not have to strip it and risk stripping the wrong prefix.
//
// It is exported because it is part of the DeleteRef contract a second backend must honour,
// and because a test outside this package must be able to prove the refusals.
func ValidateRefPath(ref string) (string, error) {
	r := strings.TrimPrefix(ref, "refs/")
	if r == "" {
		return "", Unverifiable("refusing an empty ref path — DeleteRef addresses a ref inside the repo, not an endpoint", nil)
	}
	if strings.HasPrefix(r, "/") || strings.HasSuffix(r, "/") {
		return "", Unverifiable(fmt.Sprintf("refusing ref %q: a ref path has no leading or trailing separator", ref), nil)
	}
	parts := strings.Split(r, "/")
	if len(parts) < 2 {
		return "", Unverifiable(fmt.Sprintf("refusing ref %q: a ref path must be namespaced (\"heads/<branch>\", \"tags/<tag>\", \"dispatch/<key>\")", ref), nil)
	}
	for _, p := range parts {
		if err := validateRefComponent(ref, p); err != nil {
			return "", err
		}
	}
	return r, nil
}

// refForbiddenRunes are the characters git's check-ref-format rejects outright, plus the
// three URL-significant ones ("?", "#", "%") that would let a ref reshape the request it is
// interpolated into rather than name an object inside it.
const refForbiddenRunes = " ~^:?*[\\\x7f%#\"'<>|&;$`\n\r\t"

func validateRefComponent(full, p string) error {
	switch {
	case p == "":
		return Unverifiable(fmt.Sprintf("refusing ref %q: it has an empty path component", full), nil)
	case p == "." || p == "..":
		return Unverifiable(fmt.Sprintf("refusing ref %q: %q would traverse out of the ref namespace", full, p), nil)
	case strings.HasPrefix(p, "."):
		return Unverifiable(fmt.Sprintf("refusing ref %q: a component may not begin with a dot", full), nil)
	case strings.HasPrefix(p, "-"):
		return Unverifiable(fmt.Sprintf("refusing ref %q: a component may not begin with a dash (it would read as an option)", full), nil)
	case strings.HasSuffix(p, ".lock"):
		return Unverifiable(fmt.Sprintf("refusing ref %q: a component may not end in .lock", full), nil)
	case strings.Contains(p, ".."):
		return Unverifiable(fmt.Sprintf("refusing ref %q: %q contains \"..\"", full, p), nil)
	case strings.Contains(p, "@{"):
		return Unverifiable(fmt.Sprintf("refusing ref %q: %q contains the reflog sequence \"@{\"", full, p), nil)
	}
	for _, r := range p {
		if r < 0x20 || strings.ContainsRune(refForbiddenRunes, r) {
			return Unverifiable(fmt.Sprintf("refusing ref %q: component %q carries the forbidden character %q", full, p, r), nil)
		}
	}
	return nil
}
