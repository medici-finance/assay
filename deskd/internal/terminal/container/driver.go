package container

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/creack/pty"

	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// dockerBinary is the docker CLI this driver execs — the ONLY external
// process this package ever spawns, and (with the docker client library
// refused, see doc.go) the ONLY place `docker` is touched anywhere in the
// cockpit (Verify row 9's grep).
const dockerBinary = "docker"

// attachCmd is the ONLY interactive command this driver ever execs — a login
// shell, never a value a caller supplies (the container driver SPOF layer 2). It is `bash
// -l` behind a fixed one-line preamble: the preamble prints the shell's own
// in-container pid, then `exec`s `bash -l` in the same process, so the shell
// in the container IS `bash -l` (same pid, argv `bash -l`). The session
// records that pid and signals it on Close (session.go), because the
// container engine does not end an exec'd process when its client goes away
// (the service contract review finding F-cor-1). The preamble runs under
// [posixBash], so a container-side $BASH_ENV cannot print a decoy pid line
// ahead of it. [terminal.TargetSpec]
// (the service contract) carries
// no per-desk command-override field: there is no `attach_cmd` key on
// schema.TargetSpec today, so there is no path — not even a misconfigured
// cells.yaml — by which a value could choose the exec'd argv. A configurable
// attach command is a schema.TargetSpec change for a later brief; this one
// does not invent that field, it ships the one default the spec text names.
var attachCmd = posixBash(`echo "$$"; exec bash -l`)

// Driver implements [terminal.Target] by shelling out to `docker exec -i -t`
// under a locally-allocated pty. A Driver value is stateless with respect to
// any one desk (target.go's contract) — one Driver serves every
// container-kind desk; there is no per-instance state to construct, so
// [NewDriver] takes nothing.
type Driver struct{}

// NewDriver returns a ready-to-use Driver. There is no client handle to
// build: the mechanism is the `docker` binary on PATH, invoked fresh on every
// Attach.
func NewDriver() *Driver { return &Driver{} }

// Attach implements [terminal.Target]. A spec of the wrong kind for this
// driver, or a container docker cannot reach, is an error wrapping
// [terminal.ErrUnreachable] — target.go's contract — never a bare error the
// registry would treat as a programming fault.
func (d *Driver) Attach(ctx context.Context, spec *terminal.TargetSpec, size terminal.Size) (terminal.Session, error) {
	if spec == nil || spec.Kind != "container" || spec.Container == "" {
		return nil, fmt.Errorf("container driver: spec is not an addressable container target: %w", terminal.ErrUnreachable)
	}

	// A synchronous reachability check BEFORE spawning the interactive exec.
	// `docker exec -it` under a pty fails ASYNCHRONOUSLY — the process starts,
	// then the docker CLI prints a daemon error to the pty and exits non-zero
	// — so classifying unreachable from that would mean racing the exit
	// against a timeout. `docker inspect` answers synchronously instead.
	running, inspectErr := containerIsRunning(ctx, spec.Container)
	if inspectErr != nil && permissionDenied(inspectErr) {
		// The container engine refused deskd's user (the service contract):
		// ErrUnauthorized, retried slowly, never ErrUnreachable.
		return nil, fmt.Errorf("container driver: %q: the container engine refused on permission grounds (%v): %w",
			spec.Container, inspectErr, terminal.ErrUnauthorized)
	}
	if inspectErr != nil || !running {
		return nil, fmt.Errorf("container driver: %q not reachable (running=%v, inspect error: %v): %w",
			spec.Container, running, inspectErr, terminal.ErrUnreachable)
	}

	args := append([]string{"exec", "-i", "-t", spec.Container}, attachCmd...)
	// No -u/--user, no -e/--env, no --privileged: the exec runs with the
	// container's OWN user and environment (SPOF layer 2). astguard_test.go
	// audits this package's source for exactly those flags.
	cmd := exec.Command(dockerBinary, args...)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(size.Rows),
		Cols: uint16(size.Cols),
	})
	if err != nil {
		return nil, fmt.Errorf("container driver: starting docker exec on %q: %w (%v)", spec.Container, terminal.ErrUnreachable, err)
	}

	return newSession(cmd, ptmx, spec.Container), nil
}

// containerIsRunning is the ONE other docker-cli call this driver makes: it
// answers the reachability/liveness question `docker inspect` can answer
// synchronously, before ever attaching an interactive exec.
func containerIsRunning(ctx context.Context, name string) (bool, error) {
	out, err := exec.CommandContext(ctx, dockerBinary, "inspect", "--type=container", "--format={{.State.Running}}", name).Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// permissionDenied reports a `permission denied` from the docker CLI, which is
// what it prints when deskd's user cannot reach the engine's socket.
func permissionDenied(err error) bool {
	text := err.Error()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		text += " " + string(exitErr.Stderr)
	}
	return strings.Contains(strings.ToLower(text), "permission denied")
}
