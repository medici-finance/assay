package terminal

import (
	"context"
	"errors"

	"github.com/medici-finance/assay/deskd/config/schema"
)

// TargetSpec is the parsed cells.yaml `target:` the registry resolves a desk id
// to. It is [schema.TargetSpec] — the ONE definition, in the config layer that
// parses and validates it under KnownFields(true) — re-exported here so a
// reader of the seam sees the type the [Registry] reads without importing the
// schema package by name. The registry reads ONLY this parsed value to choose a
// driver; no request field feeds it (spec §5.1).
type TargetSpec = schema.TargetSpec

// ErrUnreachable is the typed error a [Target] returns when it cannot attach —
// the container is gone, the pod is not scheduled, the local command will not
// spawn. The registry renders it as agent state could-not-check, never as a
// missing desk (spec §5.2: an unreachable desk is a could-not-check row, never
// a 404). A driver MUST return an error that wraps this (errors.Is) rather than
// a bare failure, so the registry can tell "cannot reach" from a programming
// error.
var ErrUnreachable = errors.New("terminal target unreachable")

// ErrUnauthorized is the typed error a [Target] returns when the platform
// refused the attach on permission grounds: `permission denied` on the
// container engine's socket for the container driver (the service contract). The
// supervisor also reports a `kind: pod` desk as unauthorized without asking
// any driver (DR-deskd-attach-start). It is distinct from [ErrUnreachable]
// because it does not heal in a second. The supervisor retries it at a fixed
// slow interval, not on the backoff ladder, and the wire reason reads
// "unauthorized". Like ErrUnreachable it renders as a could-not-check row,
// never a missing one.
var ErrUnauthorized = errors.New("terminal target refused the attach: unauthorized")

// Size is a terminal window size a session is opened at and resized to.
type Size struct {
	Cols int
	Rows int
}

// Target is the contract every "where does a desk run" driver implements — the
// container driver (13), the pod driver (14) and the local-PTY driver (15).
// It is the ONE place in the whole cockpit a process is ever spawned to reach a
// desk; a reviewer proves that by grepping exec.Command outside the driver
// packages and finding zero (brief Review gate). A driver that cannot attach
// returns an error wrapping [ErrUnreachable].
//
// A Target is stateless with respect to a desk: it is handed the [TargetSpec]
// to attach to on each call, so one Target value can serve every desk of its
// kind. The registry owns per-desk state (the ring, the derived agent state);
// the Target owns only the byte pipe.
type Target interface {
	// Attach opens a session against spec at the given initial size. A spec of
	// the wrong kind for this driver, or an unreachable target, is an error
	// wrapping [ErrUnreachable].
	Attach(ctx context.Context, spec *TargetSpec, size Size) (Session, error)
}

// Session is one live attachment to a desk's terminal. Read yields the desk's
// output bytes; Write sends input (the send route gates the write route, but the
// transport is here); Resize follows a window resize; Alive reports whether the
// underlying process is still attached; Close tears the attachment down.
//
// Read returns a channel the registry's pump drains; the driver closes it when
// the attachment ends. The registry, not the driver, timestamps and stamps a
// cursor onto the bytes — the driver owns bytes, not bookkeeping.
type Session interface {
	Read() <-chan []byte
	Write(p []byte) error
	Resize(cols, rows int) error
	Alive() bool
	Close() error
}
