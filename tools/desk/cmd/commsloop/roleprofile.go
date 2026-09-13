package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/acp"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// roleprofile.go — per-target-role default-deny permission fences for the executor
// dispatch leg (brief 08). An executor session is fired AT one of the five
// desk roles (the envelope's To.Role); each role gets its OWN compiled PermissionPolicy
// + FileAccessPolicy, because "worker profile is not the verify profile" (brief facts) —
// a worker mutates code inside its own worktree, a verifier never mutates at all, and so
// on. An UNKNOWN role refuses dispatch outright: there is no permissive default profile
// to fall back to (fail closed, same posture as internal/acp's own
// DefaultRefusePermission).
//
// This deliberately duplicates the SHAPE of cmd/verifyloop/dispatch_native.go's
// verifyDeskAllows / rootScopedFileAccess rather than importing across loop packages —
// routing.go (this same package) already establishes why: a shared function would
// collapse two independently-reviewable fences into one, which proves nothing about
// either fence holding on its own. Each loop's role/permission logic stays owned by that
// loop.

// KnownRoles is the closed set of target desk roles the executor leg can fire an ACP
// session at — the five desk roles a cell runs (the-desk, intake-desk, worker-desk,
// pr-review-desk, verify-desk). A role outside this set is refused at
// resolveRoleProfile, before any worktree or spawn — "unknown role = refuse dispatch,
// never a permissive default profile" (the executor dispatch leg's own ground rule).
var KnownRoles = map[string]bool{
	"the-desk":       true,
	"intake-desk":    true,
	"worker-desk":    true,
	"pr-review-desk": true,
	"verify-desk":    true,
}

// knownRoleNames returns KnownRoles's members, sorted, for refusal messages.
func knownRoleNames() []string {
	out := make([]string, 0, len(KnownRoles))
	for r := range KnownRoles {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// roleAllowsMutation reports whether role's profile permits a mutating tool call
// (edit/create/delete/move/rename) AT ALL. Only worker-desk authors code inside its own
// worktree; every other role's profile is read + role-scoped-command only — the same
// default-deny posture verify-desk's own profile already uses
// (cmd/verifyloop/dispatch_native.go's verifyDeskAllows), generalized per role here.
var roleAllowsMutation = map[string]bool{
	"the-desk":       false,
	"intake-desk":    false,
	"worker-desk":    true,
	"pr-review-desk": false,
	"verify-desk":    false,
}

// roleCommandPrefixes is the allowlist of leading tokens an execute/other tool call may
// use, per role. NOT the isolation boundary (same caveat as verifyloop's
// verifyCommandPrefixes doc): FSRoot is what fences fs/* callbacks to the worktree; this
// keeps a role's turn from reaching for an obviously out-of-role tool in the common
// case. Absent-from-map is unreachable — an unknown role never gets this far
// (resolveRoleProfile refuses first).
var roleCommandPrefixes = map[string][]string{
	"the-desk": {
		"gh issue", "gh pr", "git status", "git log", "git diff", "git show",
		"ls", "cat", "head", "tail", "grep", "rg", "wc",
	},
	"intake-desk": {
		"gh issue", "git status", "git log", "git diff", "git show",
		"ls", "cat", "head", "tail", "grep", "rg", "wc",
	},
	"worker-desk": {
		"go build", "go test", "go run", "go vet", "gofmt",
		"git status", "git diff", "git show", "git log", "git add", "git commit", "git rev-parse",
		"ls", "cat", "head", "tail", "grep", "rg", "wc", "true",
	},
	"pr-review-desk": {
		"gh pr", "go build", "go test", "go vet",
		"git diff", "git show", "git log", "git status",
		"ls", "cat", "head", "tail", "grep", "rg", "wc",
	},
	"verify-desk": {
		"go build", "go test", "go run", "go vet", "gofmt",
		"git diff", "git show", "git status", "git log", "git rev-parse", "git ls-files",
		"ls", "cat", "head", "tail", "grep", "rg", "wc", "true",
	},
}

// RoleProfile is one target role's compiled fence: a PermissionPolicy (session/
// request_permission callbacks) and a FileAccessPolicy (fs/read_text_file,
// fs/write_text_file callbacks). Both are compiled at dispatch time from the role name
// plus the session's own worktree — every role's FileAccessPolicy scopes to the SAME
// worktree (the isolation floor is the session's worktree, not a role distinction; brief
// facts: "FSRoot = the session's worktree"), while PermissionPolicy differs per role.
type RoleProfile struct {
	Role             string
	PermissionPolicy acp.PermissionPolicy
	FileAccessPolicy acp.FileAccessPolicy
}

// resolveRoleProfile refuses on any role outside KnownRoles. Called BEFORE a worktree is
// created or a runner resolved: an unknown role never reaches acp.Spawn at all.
func resolveRoleProfile(role string) error {
	if !KnownRoles[strings.TrimSpace(role)] {
		return deskkit.Refused(fmt.Sprintf(
			"roleprofile: unknown target role %q — refusing dispatch (no permissive default profile; known roles: %s)",
			role, strings.Join(knownRoleNames(), ", ")))
	}
	return nil
}

// roleProfileFor compiles role's RoleProfile against one session's worktree. role MUST
// already be validated by resolveRoleProfile — this does not re-refuse, it only compiles.
func (l *Loop) roleProfileFor(role string, it loopengine.Item, worktree string) RoleProfile {
	return RoleProfile{
		Role:             role,
		PermissionPolicy: l.rolePermissionPolicy(role, it, worktree),
		FileAccessPolicy: rootScopedFileAccessComms(worktree),
	}
}

// rolePermissionPolicy is role's compiled PermissionPolicy: stop-flags / kill-switch
// re-checked on EVERY callback (brief facts — mirrors verifyloop's own permissionPolicy),
// then the pure roleAllows decision, with every decision — allow AND refuse — audited
// for the daily lane-violation sweep (09).
func (l *Loop) rolePermissionPolicy(role string, it loopengine.Item, worktree string) acp.PermissionPolicy {
	return func(_ context.Context, req acp.PermissionRequest) acp.PermissionDecision {
		if err := l.guard(); err != nil {
			l.auditNative("permission", deskkit.ResultRefused, it,
				"role="+role+" stop/kill active mid-turn — refusing tool call ["+req.Kind+"] "+req.Title)
			return acp.PermissionDecision{Allow: false}
		}
		allow, reason := roleAllows(role, req)
		result := deskkit.ResultRefused
		if allow {
			result = deskkit.ResultOK
		}
		l.auditNative("permission", result, it, "role="+role+" "+reason+" ["+req.Kind+"] "+req.Title)
		return acp.PermissionDecision{Allow: allow}
	}
}

// roleAllows is the pure decision function (no I/O, no audit), unit-testable directly —
// the SAME shape as verifyloop's verifyDeskAllows, generalized to take role into account.
// Default-deny: an unrecognised role would refuse every kind, but roleAllows is only ever
// called with a role already validated by resolveRoleProfile.
func roleAllows(role string, req acp.PermissionRequest) (allow bool, reason string) {
	switch req.Kind {
	case "read", "fetch":
		// Reads are allowed for every role; the fs fence (FSRoot=worktree) scopes fs/*
		// callbacks to the worktree independently, so an out-of-worktree read is refused
		// there even if a read tool call is permitted here.
		return true, "allow-read"
	case "edit", "delete", "move", "rename", "create":
		if roleAllowsMutation[role] {
			return true, "allow-mutation"
		}
		// This is the fence a mis-routed dispatch (a worker-shaped payload fired under a
		// non-mutating role's profile) hits: the profile, not the payload, decides.
		return false, "refuse-mutation-out-of-profile"
	}
	cmd := extractCommand(req)
	if strings.TrimSpace(cmd) == "" {
		return false, "refuse-command-unparseable"
	}
	if isRoleCommand(role, cmd) {
		return true, "allow-role-command"
	}
	return false, "refuse-non-role-command"
}

// isRoleCommand reports whether cmd is a single, simple invocation this role's
// allowlist covers. Any shell metacharacter that could chain to a non-allowed command
// makes it fail closed (same rule as verifyloop's isVerifyCommand).
func isRoleCommand(role, cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	if strings.ContainsAny(cmd, ";|&<>`\n") || strings.Contains(cmd, "$(") {
		return false
	}
	for _, p := range roleCommandPrefixes[role] {
		if cmd == p || strings.HasPrefix(cmd, p+" ") {
			return true
		}
	}
	return false
}

// wireExecutorPermissionToolCall / extractCommand pull the command out of a
// session/request_permission payload — a package-local copy of verifyloop's own
// extractCommand (dispatch_native.go), duplicated rather than imported for the same
// reason roleAllows is: each loop's permission fence is independently reviewable.
type wireExecutorPermissionToolCall struct {
	ToolCall struct {
		Title    string `json:"title"`
		RawInput struct {
			Command string `json:"command"`
			Cmd     string `json:"cmd"`
		} `json:"rawInput"`
	} `json:"toolCall"`
}

func extractCommand(req acp.PermissionRequest) string {
	var w wireExecutorPermissionToolCall
	if err := json.Unmarshal(req.Raw, &w); err == nil {
		if c := strings.TrimSpace(w.ToolCall.RawInput.Command); c != "" {
			return c
		}
		if c := strings.TrimSpace(w.ToolCall.RawInput.Cmd); c != "" {
			return c
		}
	}
	// Default-deny: no recognized command key means no fallback to the agent-chosen
	// Title (an agent could title a hidden command as an allowed one).
	return ""
}

// rootScopedFileAccessComms is this package's own root-scoped FileAccessPolicy — a copy
// of internal/acp's unexported rootScopedFileAccess, needed because RoleProfile compiles
// its OWN FileAccessPolicy value per the brief's "PermissionPolicy + FileAccessPolicy"
// contract rather than leaving it to acp.Opts's FSRoot default. Behaviourally identical:
// allow only a path (lexically) inside root.
func rootScopedFileAccessComms(root string) acp.FileAccessPolicy {
	return func(_ context.Context, req acp.FileAccessRequest) bool {
		if root == "" {
			return false
		}
		return pathUnderComms(root, req.Path)
	}
}

func pathUnderComms(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
