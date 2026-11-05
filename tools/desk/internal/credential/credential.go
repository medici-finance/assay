// Package credential is the role-credential provider: the one implementation of GitHub
// App installation-token mint-or-reuse and GitLab PAT custody read / rotate-on-mint that
// both the desktoken CLI and the in-process lookup (deskkit.RoleTokenForOwner) run.
//
// WHY A PACKAGE. Until it existed the mint lived only in cmd/desktoken, and every desk verb
// that needed a role token forked that binary to get one — a process launch per (role,
// account) per verb, a PATH lookup that resolved whatever `desktoken` happened to be first,
// and stderr scraping to recover the refusal. The provider is the same code reached as a
// function call: the CLI is now a thin path-printing shell over it.
//
// WHAT IT DOES NOT IMPORT. Nothing here imports deskkit. Configuration, the kill-switch
// guard, the audit ledger, the custody-file checks and the advisory lock all arrive through
// Adapters, so deskkit can import this package (it owns the in-process caller) without a
// cycle, and so every control the CLI applied is applied by the provider through the SAME
// deskkit implementation the CLI used. A missing adapter is a refusal, never a skipped check.
//
// WHAT IT NEVER DOES. It launches no process, reads no ambient identity, falls back to no
// other role, account or host, and returns no serializable secret: the token travels as a
// Secret whose String, GoString, MarshalJSON and MarshalText never yield the value.
package credential

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Class is the exit class of a provider failure. The provider never decides an exit CODE —
// that is the caller's contract — only which class of failure occurred, so the CLI and the
// in-process caller map it to the same codes they always used.
type Class int

const (
	// ClassRefused is a constraint refusal: the request itself is not admissible.
	ClassRefused Class = iota + 1
	// ClassUnverifiable is a fail-closed refusal: state could not be positively verified.
	ClassUnverifiable
)

// Error is a classified provider failure. Error() renders "msg: cause" exactly as the
// desk-tools error type does, so a message reads the same before and after the caller maps
// it, and the audit row the provider writes carries the text the caller will print.
type Error struct {
	Class Class
	Msg   string
	Err   error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

// Unwrap exposes the cause for errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.Err }

func refused(msg string) *Error { return &Error{Class: ClassRefused, Msg: msg} }

func unverifiable(msg string, err error) *Error {
	return &Error{Class: ClassUnverifiable, Msg: msg, Err: err}
}

// Outcome is what an operation actually did. It is typed so a caller can tell a reuse (no
// act, no audit row) from a mint or a rotation (an act, one audit row) without parsing text.
type Outcome string

const (
	// OutcomeReused: an on-disk cache was handed back; nothing was contacted or changed.
	OutcomeReused Outcome = "reused"
	// OutcomeMinted: a new GitHub installation token was minted and cached.
	OutcomeMinted Outcome = "minted"
	// OutcomePathOnly: the cache path was resolved (desktoken --ttl); nothing was read.
	OutcomePathOnly Outcome = "path-only"
	// OutcomeCustodyRead: a GitLab custody file was verified and read; no rotation.
	OutcomeCustodyRead Outcome = "custody-read"
	// OutcomeRotated: a GitLab PAT was rotated in place and self-checked.
	OutcomeRotated Outcome = "rotated"
)

// Secret holds a credential value in memory only. Its zero value is empty. Every
// formatting and encoding path redacts — a Secret printed, logged or marshalled by accident
// yields a marker, never the token — and Reveal is the one spelling that returns the value.
type Secret struct{ v string }

// Reveal returns the credential value. It is the only way to obtain it.
func (s Secret) Reveal() string { return s.v }

// Empty reports whether the secret holds no value.
func (s Secret) Empty() bool { return s.v == "" }

const redacted = "[redacted credential]"

func (Secret) String() string   { return redacted }
func (Secret) GoString() string { return redacted }

// errNotSerializable is returned by every encoder hook: a credential is never serialized.
var errNotSerializable = errors.New("credential: a role credential is not serializable")

func (Secret) MarshalJSON() ([]byte, error) { return nil, errNotSerializable }
func (Secret) MarshalText() ([]byte, error) { return nil, errNotSerializable }

// Credential is a provider result.
type Credential struct {
	// Secret is the token value, in memory. Empty for OutcomePathOnly, and for a GitLab
	// rotation's caller that asked for the path only it is still populated.
	Secret Secret
	// Path is the custody file the value lives in — the only thing the CLI prints.
	Path string
	// ExpiresAt is the credential's expiry: the forge's own expires_at for a mint or a
	// rotation, the recorded mint expiry for a reuse, and for a LEGACY cache (minted before
	// expiry was recorded) the conservative reuse bound mtime + ReuseWindow. Zero when the
	// forge reports none (a GitLab PAT under group policy).
	ExpiresAt time.Time
	// Outcome is what the operation did.
	Outcome Outcome
	// InstallID is the GitHub installation the token belongs to ("" on GitLab).
	InstallID string
}

// Record is one audit record. The provider writes exactly one per real act — a mint, a
// rotation, a custody read — and one per refusal past the guard. A cache reuse writes none
// (it contacts nothing and changes nothing), and a guard refusal writes none here because
// the guard records its own.
type Record struct {
	Verb   string // "mint", "rotate" or "custody-read"
	Err    error  // nil on success
	Detail string // success detail; a refusal's detail is Err.Error()
}

// Custody is the custody-file check adapter (deskkit.LstatCustody / VerifyCustodyOwnerOnly).
type Custody struct {
	// Lstat returns the file a custody check must judge and read. sameDirLink admits the
	// documented same-directory link layout (GitLab); false refuses every link.
	Lstat func(path string, sameDirLink bool) (target string, fi os.FileInfo, err error)
	// IsLinkRefusal reports whether an Lstat error is a refused link (as opposed to a
	// missing or unreadable file).
	IsLinkRefusal func(err error) bool
	// OwnerOnly enforces owner-only custody (0600 on unix, the owner-only ACL on Windows).
	OwnerOnly func(path string, fi os.FileInfo) error
}

// Locker is the advisory-lock adapter (deskkit.TryLockExclusive / UnlockFile / ErrLockBusy).
type Locker struct {
	TryLock func(f *os.File) error
	Unlock  func(f *os.File) error
	IsBusy  func(err error) bool
}

// Adapters carries every capability the provider does not own. Each one is REQUIRED for the
// operations that use it; a nil one is a refusal (see check), never a skipped control.
type Adapters struct {
	// Guard is the live kill-switch / STOP / heartbeat check, run once per operation before
	// any custody is touched. It records its own refusal.
	Guard func() error
	// Audit receives the operation's one record (see Record).
	Audit func(Record)
	// Custody is the custody-file check.
	Custody Custody
	// Lock serialises GitLab rotation per role. Required only for a rotation.
	Lock Locker
	// HTTP is the transport every forge call goes through. Nothing falls back to a default.
	HTTP *http.Client
	// Now is the clock. Cache ages and expiry are read from it.
	Now func() time.Time
	// Stderr receives operator-facing notices (the GitLab pre-contact announcement). Nil
	// discards them.
	Stderr io.Writer
}

// check returns the refusal for the first missing adapter an operation needs.
func (a Adapters) check(needLock bool) *Error {
	missing := ""
	switch {
	case a.Guard == nil:
		missing = "Guard"
	case a.Audit == nil:
		missing = "Audit"
	case a.Custody.Lstat == nil || a.Custody.IsLinkRefusal == nil || a.Custody.OwnerOnly == nil:
		missing = "Custody"
	case a.HTTP == nil:
		missing = "HTTP"
	case a.Now == nil:
		missing = "Now"
	case needLock && (a.Lock.TryLock == nil || a.Lock.Unlock == nil || a.Lock.IsBusy == nil):
		missing = "Lock"
	}
	if missing == "" {
		return nil
	}
	return refused("credential provider: the " + missing + " adapter is not wired — refusing rather than " +
		"running the operation without that control")
}

func (a Adapters) stderr() io.Writer {
	if a.Stderr == nil {
		return io.Discard
	}
	return a.Stderr
}

// Paths is the App-credential search path snapshot: the directories searched in order
// (the head is where new files are written), the knob that prepends one, and the doc that
// records where a deployment provisions its credentials. Messages name all three.
type Paths struct {
	Dirs []string
	Knob string
	Doc  string
}

// find returns the first existing regular <dir>/<name> across the search path, or the
// head-of-path candidate with found=false — the same answer deskkit.FindConfigFile gives.
func (p Paths) find(name string) (string, bool) {
	for _, d := range p.Dirs {
		path := filepath.Join(d, name)
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path, true
		}
	}
	if len(p.Dirs) == 0 {
		return name, false
	}
	return filepath.Join(p.Dirs[0], name), false
}

// writePath is where a NEW file is written: the head of the search path.
func (p Paths) writePath(name string) (string, bool) {
	if len(p.Dirs) == 0 || strings.TrimSpace(p.Dirs[0]) == "" {
		return "", false
	}
	return filepath.Join(p.Dirs[0], name), true
}

// --- Roles ------------------------------------------------------------------------------

// knownRoles is the fixed set of desk roles a credential can be obtained for. A role's
// custody is parameterised by its name (<role>-token-<install>, gitlab-<role>.token), so a
// role outside this set has no custody to read and is refused rather than turned into a
// file name.
//
// cell-issues is the write-issues App identity and release-runner the dedicated run
// credential — both selectable only by name, never a loop's default. auditor is the
// read-only hardening identity: it never posts, files or writes anything.
var knownRoles = []string{
	"auditor", "cell-issues", "desk", "intake-loop", "issue-loop", "release-runner",
	"reviewer", "verifier", "worker",
}

// KnownRoles returns a fresh copy of the admitted role set.
func KnownRoles() map[string]bool {
	out := make(map[string]bool, len(knownRoles))
	for _, r := range knownRoles {
		out[r] = true
	}
	return out
}

// KnownRole reports whether role is an admitted desk role.
func KnownRole(role string) bool {
	for _, r := range knownRoles {
		if r == role {
			return true
		}
	}
	return false
}

// RoleNames returns the admitted roles, sorted, for messages.
func RoleNames() []string {
	out := append([]string(nil), knownRoles...)
	sort.Strings(out)
	return out
}

func refuseRole(role string) *Error {
	return refused("unknown role " + role + "; valid: " + strings.Join(RoleNames(), ", "))
}

// audit writes the operation's record when an Audit adapter is wired. It is the one place a
// Record is emitted, so "exactly one row per act" is a property of the call sites below.
func (a Adapters) audit(verb string, err error, detail string) {
	if a.Audit == nil {
		return
	}
	a.Audit(Record{Verb: verb, Err: err, Detail: detail})
}

// finish audits the operation's terminal result and returns err unchanged. A nil err with
// suppress set (a cache reuse) writes nothing.
func (a Adapters) finish(verb string, err error, detail string, suppress bool) error {
	if err == nil && suppress {
		return nil
	}
	if err != nil {
		a.audit(verb, err, err.Error())
		return err
	}
	a.audit(verb, nil, detail)
	return nil
}
