package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// baseTagLen is how many hex characters of the base's digest a shortened name keeps. The tag
// stands in for the part of the base that was cut, so two items whose keys share a long
// prefix still get different names after the cut.
const baseTagLen = 8

// sessionScopedName is the ONE place a worker or verifier worktree name gains its session
// suffix. base must already match worktreeNameRe; sess is dispatchSessionSuffix()'s value.
//
//   - No session: the bare base (the pre-session behaviour).
//   - base-sess fits worktreeNameMax: that name, unchanged.
//   - Otherwise the BASE is shortened, never the session: <base prefix>-<tag>-<sess>, where
//     tag is the first baseTagLen hex characters of sha256(base). The session suffix is what
//     keeps two sessions' worktrees apart, so it is kept whole; the tag keeps two items with a
//     shared long prefix apart.
//   - A session so long that not even one character of base fits beside the tag is refused
//     with an error. The caller never falls back to the bare base: that name is shared by
//     every session dispatching the same item, which is the isolation loss this guards.
func sessionScopedName(base, sess string) (string, error) {
	if sess == "" {
		return base, nil
	}
	if full := base + "-" + sess; worktreeNameRe.MatchString(full) {
		return full, nil
	}
	sum := sha256.Sum256([]byte(base))
	tag := hex.EncodeToString(sum[:])[:baseTagLen]
	keep := worktreeNameMax - len(sess) - len(tag) - 2
	if keep < 1 {
		return "", fmt.Errorf("the session id %q (%d characters) leaves no room for the item in a "+
			"%d-character worktree name, so no session-unique name fits; use a shorter $DESK_SESSION",
			sess, len(sess), worktreeNameMax)
	}
	if keep > len(base) {
		keep = len(base)
	}
	name := base[:keep] + "-" + tag + "-" + sess
	if !worktreeNameRe.MatchString(name) {
		return "", fmt.Errorf("the shortened worktree name %q is outside the worktree-name grammar", name)
	}
	return name, nil
}
