package acp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
)

// PermissionOption is one choice offered in a session/request_permission
// callback. Kind is one of the ACP-defined kinds ("allow_once",
// "allow_always", "reject_once", "reject_always"); this package does not
// further validate the value, it only groups by the "allow"/"reject" prefix
// when translating a PermissionDecision back into an OptionID (see
// selectOption).
type PermissionOption struct {
	OptionID string
	Name     string
	Kind     string
}

// PermissionRequest is the session/request_permission callback payload
// handed to a PermissionPolicy.
type PermissionRequest struct {
	SessionID  SessionID
	ToolCallID string
	Title      string
	Kind       string
	Options    []PermissionOption
	// Raw is the full wire toolCall object (rawInput, locations, content,
	// ...) for policies that need more than Title/Kind.
	Raw json.RawMessage
}

// PermissionDecision is what a PermissionPolicy returns for a
// session/request_permission callback. Allow=false (the zero value) refuses.
type PermissionDecision struct {
	Allow bool
}

// PermissionPolicy decides a session/request_permission callback. Invoked
// once per callback, synchronously, from the connection's read loop -- keep
// it fast and non-blocking on external I/O.
//
// The default (DefaultRefusePermission, used when Opts.PermissionPolicy is
// nil) always refuses, per the brief's ground rule to fail closed.
type PermissionPolicy func(ctx context.Context, req PermissionRequest) PermissionDecision

// DefaultRefusePermission is the fail-closed PermissionPolicy: it never
// allows a tool call.
func DefaultRefusePermission(context.Context, PermissionRequest) PermissionDecision {
	return PermissionDecision{Allow: false}
}

// selectOption translates a PermissionDecision into the wire outcome: the
// OptionID of a matching "allow"/"reject" option (preferring the "_once"
// variant over "_always" -- least privilege), or cancel=true if the agent
// offered no option of the requested kind at all. cancel=true is itself a
// fail-safe: this package never falls through to picking an option of the
// wrong polarity just because that's all that was offered.
func selectOption(opts []PermissionOption, allow bool) (optionID string, cancel bool) {
	prefix := "reject"
	if allow {
		prefix = "allow"
	}
	var onceID, anyID string
	for _, o := range opts {
		if !strings.HasPrefix(o.Kind, prefix) {
			continue
		}
		if anyID == "" {
			anyID = o.OptionID
		}
		if strings.HasSuffix(o.Kind, "_once") {
			onceID = o.OptionID
		}
	}
	if onceID != "" {
		return onceID, false
	}
	if anyID != "" {
		return anyID, false
	}
	return "", true
}

// FileAccessRequest describes an fs/read_text_file or fs/write_text_file
// callback.
//
// Observed live (see README.md): the adapter under test never actually
// issued these even though this client advertised both fs capabilities --
// it performed file writes itself and gated them exclusively through
// session/request_permission on the owning tool call. Implement this policy
// anyway (defense in depth / other agents may differ), but do not treat it
// as the primary enforcement point.
type FileAccessRequest struct {
	SessionID SessionID
	Path      string // absolute path the agent wants to read/write
	Write     bool   // false = read, true = write
}

// FileAccessPolicy decides an fs/* callback: true allows, false refuses.
type FileAccessPolicy func(ctx context.Context, req FileAccessRequest) bool

// rootScopedFileAccess is the default FileAccessPolicy used when
// Opts.FileAccessPolicy is nil: it allows a path only when Opts.FSRoot is
// non-empty and the path is (lexically) contained in it, and refuses
// everything otherwise.
func rootScopedFileAccess(root string) FileAccessPolicy {
	return func(_ context.Context, req FileAccessRequest) bool {
		if root == "" {
			return false
		}
		return pathUnder(root, req.Path)
	}
}

// pathUnder reports whether path is root itself or lexically nested under
// it, after Clean-ing both. Uses an explicit separator boundary check so
// "/root-evil" does not falsely match root "/root".
func pathUnder(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
