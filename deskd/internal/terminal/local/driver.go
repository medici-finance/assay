package local

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/creack/pty"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// defaultBindAddr mirrors cmd/deskd/main.go's own --addr default (P3's bind
// gate): a Driver constructed with New() assumes deskd is running with that
// default until a caller (the future brief that wires this driver into
// cmd/deskd) sets BindAddr/AuthMode/ValidatorConfigured from the real flags.
const defaultBindAddr = "127.0.0.1:8787"

// defaultArgv is what an empty TargetSpec.Command resolves to: a login shell
// in the operator's own HOME (the local driver facts: "a desk with kind: local and no
// command gets the default bash -l in the operator's HOME").
var defaultArgv = []string{"bash", "-l"}

// deniedEnvNeedles are the substrings (checked case-insensitively) that make
// an environment variable name unsafe to hand to a desk's shell, beyond the
// DESKD_ prefix.
var deniedEnvNeedles = []string{"TOKEN", "SECRET", "KEY"}

// configuredSecretEnv is the set of EXACT variable names (upper-cased) the
// running configuration declares as credentials — every cell's chat
// `key_env`. A configured name need not contain TOKEN, SECRET or KEY (the
// schema accepts any upper-case identifier), so the needle list alone cannot
// keep it out of a desk shell; [DenyEnvNames] registers it explicitly.
var (
	configuredSecretMu  sync.RWMutex
	configuredSecretEnv = map[string]bool{}
)

// DenyEnvNames adds exact variable names to the set every desk shell's
// environment is stripped of, on top of the DESKD_ prefix and the needle
// list. cmd/deskd calls it once at startup with every configured credential
// variable name, before any session is attached. Blank names are ignored.
func DenyEnvNames(names ...string) {
	configuredSecretMu.Lock()
	defer configuredSecretMu.Unlock()
	for _, n := range names {
		if n = strings.ToUpper(strings.TrimSpace(n)); n != "" {
			configuredSecretEnv[n] = true
		}
	}
}

// Driver implements terminal.Target by spawning TargetSpec.Command on a PTY in
// this process's own tree. It is stateless with respect to any one desk (one
// Driver value serves every kind: local desk, per the [terminal.Target]
// contract) but caches PATH resolutions across desks, which is why New
// returns a *Driver rather than a value.
type Driver struct {
	// BindAddr, AuthMode and ValidatorConfigured mirror the three arguments
	// deskd's own P3 bind gate (internal/identity.CheckBind) runs at startup.
	// Attach re-runs the identical check with these values — the driver's own
	// belt-and-braces copy of the bind gate, not a read of deskd's live flags
	// (this package cannot see cmd/deskd's flag.FlagSet). New defaults them to
	// deskd's own defaults (loopback, no bearer validator) so a Driver used
	// without reconfiguration behaves exactly as a freshly started deskd does.
	BindAddr            string
	AuthMode            identity.AuthMode
	ValidatorConfigured bool

	resolveMu sync.Mutex
	resolved  map[string]string // argv[0] name -> resolved absolute path, cached once
}

// New returns a Driver configured with deskd's own startup defaults
// (loopback bind, no bearer validator needed). A caller that knows deskd's
// real --addr/--auth should set BindAddr/AuthMode/ValidatorConfigured before
// the first Attach.
func New() *Driver {
	return &Driver{
		BindAddr: defaultBindAddr,
		AuthMode: identity.AuthLoopback,
		resolved: make(map[string]string),
	}
}

func init() {
	// Self-registration: the terminal package cannot import
	// this one without a cycle (this package imports terminal for
	// Target/Session/ErrUnreachable), so — like database/sql drivers — this
	// package registers itself when linked. A future brief's cmd/deskd wiring
	// looks the driver up via terminal.DriverFor(schema.TargetLocal) and sets
	// its BindAddr/AuthMode/ValidatorConfigured from the real flags before
	// attaching.
	//
	// RegisterDriver takes a factory (func() (terminal.Target, error)) so
	// that a driver whose construction CAN fail (the pod driver's kubeconfig
	// discovery) can surface that at DriverFor time rather than at package
	// init. New() never fails, so this factory always returns a nil error.
	terminal.RegisterDriver(schema.TargetLocal, func() (terminal.Target, error) {
		d := New()
		gateMu.Lock()
		if gateSet {
			d.BindAddr, d.AuthMode, d.ValidatorConfigured = gateAddr, gateMode, gateValidator
		}
		gateMu.Unlock()
		return d, nil
	})
}

var (
	gateMu        sync.Mutex
	gateSet       bool
	gateAddr      string
	gateMode      identity.AuthMode
	gateValidator bool
)

// SetBindGate records deskd's REAL bind arguments (--addr, --auth, whether a
// bearer validator is configured) for every Driver the registered factory
// builds from now on, so Attach's P3 re-check runs against what deskd is
// actually bound to rather than New()'s loopback defaults. cmd/deskd calls it
// once, from the file that links the driver set, before the attach supervisor
// starts (the service contract).
func SetBindGate(addr string, mode identity.AuthMode, validatorConfigured bool) {
	gateMu.Lock()
	defer gateMu.Unlock()
	gateSet, gateAddr, gateMode, gateValidator = true, addr, mode, validatorConfigured
}

// Attach implements terminal.Target. It refuses (wrapping
// [terminal.ErrUnreachable]) a spec of the wrong kind, a deskd bound
// off-loopback without auth configured (the P3 re-check), or an argv[0] that
// is neither absolute nor resolvable on PATH; otherwise it spawns the argv on
// a PTY sized to size and returns a live *Session.
func (d *Driver) Attach(ctx context.Context, spec *terminal.TargetSpec, size terminal.Size) (terminal.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("local: %w: %v", terminal.ErrUnreachable, err)
	}
	if spec == nil || spec.Kind != schema.TargetLocal {
		return nil, fmt.Errorf("local: %w: spec is not kind %q", terminal.ErrUnreachable, schema.TargetLocal)
	}

	// The P3 re-check (belt and braces): the SAME condition deskd's bind gate
	// enforces at startup (internal/identity.CheckBind), re-run here because a
	// local PTY runs as the deskd process's own user — a different component,
	// on a different signal, than the listener bind.
	if err := identity.CheckBind(d.BindAddr, d.AuthMode, d.ValidatorConfigured); err != nil {
		return nil, fmt.Errorf("local: %w: %v", terminal.ErrUnreachable, err)
	}

	argv := spec.Command
	homeDefault := false
	if len(argv) == 0 {
		argv = defaultArgv
		homeDefault = true
	}

	resolved, err := d.resolveArgv0(argv[0])
	if err != nil {
		return nil, fmt.Errorf("local: %w: %v", terminal.ErrUnreachable, err)
	}

	cmd := exec.Command(resolved, argv[1:]...)
	cmd.Env = filteredEnv(os.Environ())
	// the authorization contract: no human is present to act on a Claude Code prompt
	// suggestion inside a desk's spawned shell either, so it is forced off —
	// regardless of what deskd's own process inherited (an operator's shell
	// profile, a launcher script). overrideEnv REMOVES any existing entry
	// first rather than merely appending: a name that appears twice in a
	// child's environ resolves to whichever occurrence getenv(3) finds first
	// (the first, on glibc/Linux), so appending "false" after an inherited
	// "true" would silently lose to it.
	cmd.Env = overrideEnv(cmd.Env, "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION", "false")
	if homeDefault {
		if home, err := os.UserHomeDir(); err == nil {
			cmd.Dir = home
		}
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: clampWinsize(size.Rows),
		Cols: clampWinsize(size.Cols),
	})
	if err != nil {
		return nil, fmt.Errorf("local: %w: spawn %q: %v", terminal.ErrUnreachable, resolved, err)
	}

	return newSession(cmd, ptmx), nil
}

// resolveArgv0 resolves name to an absolute path, once, and caches the
// result — a later PATH mutation cannot redirect an already-resolved name to
// a different binary (the single-point-of-failure control this driver is the
// layer behind: the registry/schema only checks Command is non-empty).
func (d *Driver) resolveArgv0(name string) (string, error) {
	if filepath.IsAbs(name) {
		return name, nil
	}

	d.resolveMu.Lock()
	defer d.resolveMu.Unlock()

	if p, ok := d.resolved[name]; ok {
		return p, nil
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%q is not an absolute path and not resolvable on PATH: %w", name, err)
	}
	d.resolved[name] = p
	return p, nil
}

// filteredEnv returns env (as produced by os.Environ()) with every DESKD_*
// variable, every variable whose name contains TOKEN, SECRET or KEY
// (case-insensitive), and every name registered with [DenyEnvNames] removed.
// It is the deny-list a test asserts directly: deskd's own GitHub read token
// (DESKD_GITHUB_TOKEN) must never reach a desk session's shell.
func filteredEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, ok := strings.Cut(kv, "=")
		if !ok {
			name = kv
		}
		if isDeniedEnvName(name) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// overrideEnv returns env with any existing entry named name removed and
// name=value appended once. Used instead of a plain append for a value this
// driver must WIN regardless of what the parent process's environment
// already carries — see the CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION call site in
// Attach for why simply appending is not enough.
func overrideEnv(env []string, name, value string) []string {
	prefix := name + "="
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, prefix+value)
}

func isDeniedEnvName(name string) bool {
	upper := strings.ToUpper(name)
	if strings.HasPrefix(upper, "DESKD_") {
		return true
	}
	configuredSecretMu.RLock()
	configured := configuredSecretEnv[upper]
	configuredSecretMu.RUnlock()
	if configured {
		return true
	}
	for _, needle := range deniedEnvNeedles {
		if strings.Contains(upper, needle) {
			return true
		}
	}
	return false
}

// clampWinsize floors a possibly-zero/negative size to 1: a 0x0 PTY is legal
// to the kernel but useless (and some shells behave oddly at it), and
// terminal.Size carries no validation of its own.
func clampWinsize(n int) uint16 {
	if n < 1 {
		return 1
	}
	if n > 65535 {
		return 65535
	}
	return uint16(n)
}
