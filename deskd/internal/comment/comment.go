// Package comment is the PER-USER comment write path: the ONE
// place a viewer's own GitHub token posts an issue comment authored as THAT
// viewer. It is the sibling of internal/answer — that package is the ONE operator
// write path (custody-backed, comment+close); this is the ONE user write path
// (session-token-backed, comment-ONLY).
//
// # Comment-only is STRUCTURAL, not behavioural
//
// Two properties are enforced by the shape of this package, not by a runtime
// check a diff could quietly drop:
//
//  1. No custody import. Nothing in this package imports internal/custody, and
//     nothing it imports does either — so a commenter can never reach the operator
//     credential. internal/mcp's authority test is extended (the capability model Verify 1)
//     to assert internal/comment has NO import path to internal/custody, exactly
//     as it asserts the MCP surface does not.
//  2. No close construction. The ONE outbound request builder here (github.go)
//     builds a single POST to .../issues/{n}/comments and NOTHING else — no
//     PATCH state:closed, no reaction, no other verb. TestOneUserWritePath
//     parses the package and asserts exactly one request-building file, and a
//     companion test asserts it constructs no issue-state PATCH. A comment can
//     never carry a close, whatever its body text says (§6.6 injection posture).
package comment

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors, first-class so the handler maps each to a status.
var (
	// ErrNoAuthor — no authenticated viewer (no session token to author with).
	ErrNoAuthor = errors.New("no authenticated viewer: sign in to comment")
	// ErrEmptyComment — an empty comment body.
	ErrEmptyComment = errors.New("a comment must carry a body")
	// ErrBadRef — the escalation has no usable platform ref.
	ErrBadRef = errors.New("the escalation has no usable platform ref to comment on")
	// ErrUnknownItem — the id resolves to nothing in the trusted index.
	ErrUnknownItem = errors.New("no open escalation with that id in the index")
)

// Ref is the issue a comment is posted on: owner/name#number. It is resolved
// SERVER-SIDE from the escalation id against the trusted index — never from the
// request body, exactly as the answer path resolves its ref.
type Ref struct {
	Repo   string // owner/name
	Number int
}

func (r Ref) valid() bool { return r.Repo != "" && r.Number > 0 }

// RefLookup resolves an escalation id to its trusted ref. deskd backs it with
// the derived index; the ref NEVER comes from the request.
type RefLookup interface {
	Ref(id string) (Ref, bool, error)
}

// userWriter is what the Service depends on: post a comment authored with the
// caller's token. github.go's UserWriter satisfies it; a test supplies a fake.
// The interface has ONE method — Comment — so there is structurally nothing to
// route a close or any other write through.
type userWriter interface {
	Comment(ctx context.Context, token string, ref Ref, body string) error
}

// Service is the user comment write path.
type Service struct {
	writer userWriter
	lookup RefLookup
}

// NewService builds the comment path.
func NewService(w userWriter, lookup RefLookup) *Service {
	return &Service{writer: w, lookup: lookup}
}

// Comment posts an issue comment on the escalation id, authored with the
// viewer's own session token. login is the AUTHENTICATED login (used only for
// the audit line the caller may add and for a nil-token guard); the platform
// attributes the comment to whoever the token belongs to — GitHub's own
// enforcement, a layer this package does not and cannot override.
func (s *Service) Comment(ctx context.Context, login, token string, id, body string) error {
	if token == "" {
		return ErrNoAuthor
	}
	if strings.TrimSpace(body) == "" {
		return ErrEmptyComment
	}
	if s.lookup == nil {
		return fmt.Errorf("no escalation lookup configured")
	}
	ref, found, err := s.lookup.Ref(id)
	if err != nil {
		return fmt.Errorf("ref lookup failed: %w", err)
	}
	if !found {
		return ErrUnknownItem
	}
	if !ref.valid() {
		return ErrBadRef
	}
	return s.writer.Comment(ctx, token, ref, body)
}
