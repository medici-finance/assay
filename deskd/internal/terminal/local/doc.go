// Package local is the cockpit terminal seam's LOCAL-PTY driver
// (the service contract): the simplest of the three [terminal.Target]
// implementations (container/13, pod/14, local/15) — it spawns
// [schema.TargetSpec.Command] on a PTY in deskd's OWN process tree. This is
// the target for a laptop-hosted desk (the master desk sharing the operator's
// machine with deskd) and for a hand-opened custom session: no
// container runtime, no cluster, nothing but a PTY.
//
// # The one spawn site
//
// [Driver.Attach] is, by design, the ONLY place in this package a process is
// started (norest_test.go's sibling in this package, and Verify row 6 of the
// brief, grep exec/pty spawn calls outside driver packages and expect zero).
//
// # PATH resolution happens ONCE, then is cached forever
//
// A [schema.TargetSpec] names a command by argv, e.g. `["bash", "-l"]`; when
// the name is not an absolute path, [Driver.Attach] resolves it against
// deskd's own PATH via [exec.LookPath] and CACHES the resolved absolute path
// keyed by the name. A later PATH mutation therefore cannot redirect an
// already-resolved command to a different binary — the failure mode this
// closes is a `PATH` change (deliberate or not) silently swapping what a
// desk's shell actually is. An unresolvable name is refused at Attach with an
// error wrapping [terminal.ErrUnreachable] (an exec-time failure in the
// driver), never at config load — schema validation only requires a
// non-empty `command:`, not that it resolves (single-point-of-failure note,
// the local driver).
//
// # The deny-list: deskd's own secrets never reach a desk's shell
//
// The child inherits deskd's environment MINUS every `DESKD_*` variable and
// every variable whose name contains `TOKEN`, `SECRET`, or `KEY`
// (case-insensitive). deskd's own GitHub read token (`DESKD_GITHUB_TOKEN`)
// is exactly the credential this exists to keep out of every desk session's
// shell — a desk operator typing `env` must never see it.
//
// # Loopback re-check — the same P3 condition, a second component
//
// A local PTY runs AS THE DESKD PROCESS'S OWN USER. deskd already refuses to
// bind off-loopback without bearer-token validation configured (P3,
// internal/identity.CheckBind, cmd/deskd/main.go's bind gate). This driver
// re-checks the IDENTICAL condition at Attach — belt and braces, in a
// different component, so a future bind-gate bypass upstream does not also
// hand a remote, unauthenticated caller a shell as the deskd user.
// [Driver.BindAddr] / [Driver.AuthMode] / [Driver.ValidatorConfigured] mirror
// the three CheckBind parameters; a future brief that wires this driver into
// cmd/deskd sets them from the same flags CheckBind already runs against at
// startup.
//
// # Transport bounds and limits
//
// The byte transport performs no transcoding. The registry bounds scrollback
// by both bytes and lines; cumulative byte cursors resume exactly or report
// eviction explicitly. Idle detection uses output silence and a prompt regex
// when the producer supplies no structured prompt mark. Browser IME behavior
// needs separate client qualification and is not established by these tests.
//
// # No scrollback at rest
//
// This package holds no ring and writes no file: bytes flow from the PTY to
// the registry's ring (a different package, a different at-rest guarantee) or
// out through Close/Kill. norest_test.go in this directory is the same source
// audit internal/terminal/norest_test.go runs, scoped to this package, so the
// scrollback-at-rest guarantee covers the local driver too (the local driver Verify
// row 7).
package local
