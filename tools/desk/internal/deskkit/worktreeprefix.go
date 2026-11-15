package deskkit

import (
	"path/filepath"
	"strings"
)

// worktreeprefix.go — the ONE statement of where a desk worktree may live.
//
// deskwt refuses to remove (or add, or lock) a worktree outside two sanctioned prefixes, and that
// refusal is the control that stops it deleting an arbitrary directory. Every desk tool that CREATES
// a worktree has to place it under one of the same prefixes, or the sanctioned removal path has no
// route to it: a scanloop scan worktree cut beside the desk checkout was exactly that, and a retained
// one could then only be removed by hand. The rule used to be written out separately in each tool
// that read it; it is stated here once so a tool's placement and deskwt's check cannot disagree.
//
// This file names the prefixes. It never widens them: a caller that needs a location outside them is
// asking for a different control, not for a third entry here.

// SanctionedWorktreeTmpBase is the parent directory of the `tracker-*` sanctioned prefix. It is a
// compiled-in constant, deliberately not read from an env var or a flag: an operator-relocatable
// prefix would defeat a fixed allowlist.
const SanctionedWorktreeTmpBase = "/private/tmp"

// SanctionedWorktreeLeafPrefix is the leaf-name prefix a worktree directly under
// SanctionedWorktreeTmpBase must carry.
const SanctionedWorktreeLeafPrefix = "tracker-"

// InSanctionedWorktreePrefix reports whether a RESOLVED path is a sanctioned worktree location: a
// direct `tracker-*` child of the resolved tmp base, or strictly under the resolved
// `<repo-root>/.claude/worktrees` directory. All three arguments must already be resolved (symlinks
// evaluated) by the caller, the same way: the comparison is purely lexical, so a resolved candidate
// against an unresolved prefix (macOS `/tmp` against `/private/tmp`) is refused, which is the safe
// direction.
func InSanctionedWorktreePrefix(resolved, tmpDir, worktreesDir string) bool {
	if filepath.Dir(resolved) == tmpDir && strings.HasPrefix(filepath.Base(resolved), SanctionedWorktreeLeafPrefix) {
		return true
	}
	return strings.HasPrefix(resolved, worktreesDir+string(filepath.Separator))
}
