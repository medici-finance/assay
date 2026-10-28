package deskkit

// The P3 effective-config echo: how much of it a run prints.
//
// Every tool that makes a trust decision echoes the effective configuration once per run
// (EchoEffectiveConfig), so a settings change appears in run output and in CI history whether
// it widened or narrowed the set. The full block is about twenty lines. Printed on every run of
// every tool — usage errors and read-only verbs included — it trained operators and agents to
// filter the `assay-config:` prefix away wholesale, which would hide a REFUSED line too.
//
// So the DEFAULT is one line carrying a digest of the full block, and the full block prints
// whenever any of these holds:
//
//   - the configuration carries a problem or an unrecognised key — those lines are never quieter
//     than they were (the REFUSED and unrecognised-keys lines are part of the full block);
//   - the run is in CI (CI=true, or GITHUB_ACTIONS=true), so CI history keeps the whole block;
//   - ASSAY_CONFIG_ECHO=full is set — or ASSAY_CONFIG_ECHO holds any value other than
//     "compact": an unrecognised value never makes the echo quieter. This forced form is
//     exactly the echo before the digest existed: it neither reads nor writes the state file;
//   - the digest differs from the last RECORDED digest, kept in a small state file beside the
//     roster — so the first stateful run after a change prints in full. Only that stateful
//     path records, and only once every line of the full block was written without error (a
//     closed or failing stderr leaves the previous record, so the next run prints in full too).
//     A forced, CI or refused run prints in full without recording, so a return to the
//     recorded configuration after such a run prints compact (its digest still shows);
//   - that state file cannot be read or cannot be written: a run that cannot prove it has
//     already shown this configuration shows it.
//
// A run whose stderr nobody sees neither reads nor records the state: a harness hook calls
// EchoEffectiveConfigUnsurfaced, and a caller that captures a child's stderr and strips the echo
// gives the child UnsurfacedEchoEnv. Otherwise the unseen run would mark the digest "seen", and
// the next run someone does see would print compact after a change. A desk tool's OTHER captured
// children (its per-verb token mint, for one) start after the tool's own echo at the top of main,
// from the same config home and, outside CI, the same write class: the parent has already shown
// that digest (or printed in full without recording, which the child inherits when forced), so
// the child is never the first run to see it.
//
// The digest is sha256 over the canonical full block (EffectiveConfigLines, newline-joined and
// newline-terminated — exactly the bytes the full echo prints), so the one-line form still
// moves on every change, in both directions, on every run.
//
// RESIDUAL, stated. The state file lives in the config-home directory, which the roster loader
// already requires to be owner-only. Forging a "seen" digest so a change prints compact needs
// write access to that directory — the same access that can rewrite the roster itself — and the
// compact line still carries the CURRENT digest, so the change remains visible in it.
//
// TEST BINARIES. Inside a `go test` binary the echo neither reads nor writes the state file and
// always prints the full block (configEchoStateful), so a test that runs tool code against the
// operator's own config home neither leaves a state file there nor prints a different form
// depending on what an earlier real run recorded. This package's own echo tests opt back in.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// EnvConfigEcho selects the echo form. "full" forces the full block; "compact" (or unset) is the
// default digest form; any other value is treated as "full".
const EnvConfigEcho = "ASSAY_CONFIG_ECHO"

// configEchoStateName is the last-seen-digest file, kept beside the roster in the config home.
const configEchoStateName = "config-echo.state"

// configEchoStateMax bounds the state-file read; the file holds one short line.
const configEchoStateMax = 512

// configEchoStateful reports whether the echo may consult and record the state file. It is false
// inside a test binary (see the file comment); this package's echo tests set it.
var configEchoStateful = !testing.Testing()

// configEchoStatePath is where the last-seen digest for this config home is recorded.
func configEchoStatePath() string {
	return filepath.Join(filepath.Dir(ConfigHomePath()), configEchoStateName)
}

// EchoEffectiveConfig writes the P3 echo: the one-line digest by default, the full block when
// any full-echo condition holds (see the file comment). Every tool that makes a trust decision
// calls this once per run. It never looks at argv: a verb that wants a help screen without the
// configured values says so itself, through EchoConfigHelpNotice.
func EchoEffectiveConfig(w io.Writer) {
	writeConfigEcho(w, EffectiveConfig(), configEchoStatePath())
}

// EchoEffectiveConfigUnsurfaced writes the full block and never reads or records the state file.
// A tool whose stderr nobody sees on success calls it instead of EchoEffectiveConfig — a harness
// hook (writeguard; the cellctl model-policy hook), whose stderr Claude Code does not show when it
// exits 0. Recording there would mark a digest "seen" that nobody saw, and the next run someone
// does see would print compact.
func EchoEffectiveConfigUnsurfaced(w io.Writer) {
	for _, l := range EffectiveConfig().EffectiveConfigLines() {
		fmt.Fprintln(w, l)
	}
}

// EchoConfigHelpNotice is the OPT-IN help-screen echo (medici-finance/assay#3754). A verb calls it
// IN PLACE OF EchoEffectiveConfig only when its own predicate says the invocation is the help
// screen its dispatch returns before any work (UsageHelp, or a verb's own superset of it), and
// that predicate must be the SAME one the dispatch uses, so the two cannot disagree:
//
//	if deskkit.UsageHelp(os.Args[1:]) {
//		deskkit.EchoConfigHelpNotice(os.Stderr)
//	} else {
//		deskkit.EchoEffectiveConfig(os.Stderr)
//	}
//
// It prints no configured value (no login, id, slug, repo or path), because help output is read,
// pasted and kept by whoever asked for it and the help path consults no configuration. A
// configuration that needs attention is still never quieter than that: a refused configuration
// is reported by its problem count and an unrecognised key by its NAME; a clean configuration
// prints nothing. It neither reads nor records the digest state, so the next real run shows a
// changed configuration in full.
//
// A verb that does work under a help-shaped argv (one that ignores argv, or parses it as
// positionals) never calls it. The opt-in set is pinned by the module-root test helpecho_test.go.
func EchoConfigHelpNotice(w io.Writer) {
	writeConfigHelpNotice(w, EffectiveConfig())
}

func writeConfigHelpNotice(w io.Writer, c Config) {
	if len(c.Problems) > 0 {
		_, _ = fmt.Fprintf(w, "assay-config: REFUSED — %d problem(s); values are not shown on a help screen, run the verb to see them\n",
			len(c.Problems))
	}
	if len(c.UnknownKeys) > 0 {
		_, _ = fmt.Fprintf(w, "assay-config: unrecognised-keys=%s (IGNORED)\n", strings.Join(c.UnknownKeys, ","))
	}
}

// UnsurfacedEchoEnv returns env with ASSAY_CONFIG_ECHO=full in place of any existing setting. A
// caller that captures a child's stderr and strips the echo preamble from it (Run, deskboot's
// runner, the token minter) gives the child this environment, so the child prints the stateless
// full block and records no digest nobody saw. The value is appended last, which is the one
// os/exec keeps when a name repeats.
func UnsurfacedEchoEnv(env []string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, EnvConfigEcho+"=") {
			out = append(out, kv)
		}
	}
	return append(out, EnvConfigEcho+"=full")
}

// writeConfigEcho is EchoEffectiveConfig with its inputs explicit, so the decision is testable
// without the process cache.
//
// It is also the ONE place the digest is recorded, and it records only after every line of the
// full block was written without error to something other than the null device. A writer that
// refuses the block — or breaks partway through — has not shown it, and neither has the null
// device, which is where a closed stderr lands (the Go runtime reopens a closed fd 0-2 on
// /dev/null at startup, so those writes succeed). Either way the previous record stands and the
// next run prints in full too.
func writeConfigEcho(w io.Writer, c Config, statePath string) {
	lines := c.EffectiveConfigLines()
	sum := ConfigDigest(lines)
	full, record := configEchoForm(c, sum, statePath)
	if !full {
		fmt.Fprintln(w, compactConfigLine(c, sum))
		return
	}
	var werr error
	for _, l := range lines {
		if _, err := fmt.Fprintln(w, l); err != nil && werr == nil {
			werr = err
		}
	}
	if record && werr == nil && !nullSink(w) {
		// A failed state write is harmless: nothing recorded the digest, so the next run
		// prints in full as well.
		_ = writeConfigEchoState(statePath, sum)
	}
}

// nullSink reports whether w is the null device, so the echo written to it was seen by no one. A
// writer that is a file it cannot stat counts as one too (it cannot be shown to be visible); any
// other writer — a pipe, a terminal, a regular file, a buffer — does not. On Windows it always
// answers false: there os.SameFile reports every character device (a console included) as the
// same file as NUL, and the runtime does not reopen a closed standard handle on NUL, so the
// echo records there as it did before this check.
func nullSink(w io.Writer) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return true
	}
	null, err := os.Stat(os.DevNull)
	if err != nil {
		return false
	}
	return os.SameFile(fi, null)
}

// ConfigDigest is the hex sha256 of the full echo block exactly as printed: the lines joined
// and terminated by newlines.
func ConfigDigest(lines []string) string {
	h := sha256.New()
	for _, l := range lines {
		io.WriteString(h, l)
		io.WriteString(h, "\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// compactConfigLine is the default one-line echo. It carries the class, source and configured
// state the full block's first line carries, the leading 12 hex digits of the digest, and the
// sizes of the identity and scope sets.
func compactConfigLine(c Config, sum string) string {
	return fmt.Sprintf("assay-config: class=%s source=%s configured=%t digest=sha256:%s "+
		"humans=%d bots=%d roles=%d repos=%d (full: %s=full)",
		c.Class, c.Source, c.Configured(), sum[:12],
		len(c.Humans), len(c.BotIdents), len(c.RoleBots), len(c.Repos)+len(c.RepoPatterns),
		EnvConfigEcho)
}

// echoInCI reports whether the process runs on a CI runner, for the echo only. It is wider than
// InCI (which selects the environment roster transport and stays GitHub-Actions-only): a wrong
// answer here can only make the echo louder.
func echoInCI() bool {
	ci := strings.ToLower(strings.TrimSpace(os.Getenv("CI")))
	return ci == "true" || ci == "1" || InCI()
}

// configEchoForm decides the echo form: full is true on every condition the file comment lists,
// and record is true only when the full block is due on the stateful default path because the
// digest differs from the last recorded one (or the record is unreadable). It reads the state file
// but never writes it; writeConfigEcho records once the block is out.
func configEchoForm(c Config, sum, statePath string) (full, record bool) {
	if len(c.Problems) > 0 || len(c.UnknownKeys) > 0 {
		return true, false
	}
	if echoInCI() {
		// CI history keeps the whole block; a runner's home is not a place to keep state.
		return true, false
	}
	if !configEchoStateful {
		return true, false
	}
	if mode := strings.ToLower(strings.TrimSpace(os.Getenv(EnvConfigEcho))); mode != "" && mode != "compact" {
		// Forced full: the pre-digest echo, with no state read or written.
		return true, false
	}
	last, err := readConfigEchoState(statePath)
	if err != nil || last != sum {
		return true, true
	}
	return false, false
}

// readConfigEchoState returns the recorded digest. Any failure — absent, unreadable, oversized,
// malformed, a symlink, a reparse point or any other non-regular file — is an error, and the
// caller prints in full. It opens the file the way the roster beacon is opened (no-follow,
// non-blocking, checked regular by handle).
func readConfigEchoState(path string) (string, error) {
	f, err := openRosterBeaconFile(path, false)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, configEchoStateMax+1))
	if err != nil {
		return "", err
	}
	if len(b) > configEchoStateMax {
		return "", errors.New("config-echo state file is oversized")
	}
	v := strings.TrimSpace(string(b))
	sum, ok := strings.CutPrefix(v, "sha256:")
	if !ok || len(sum) != sha256.Size*2 {
		return "", errors.New("config-echo state file is malformed")
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return "", errors.New("config-echo state file is malformed")
	}
	return sum, nil
}

// writeConfigEchoState atomically replaces the state file: a same-directory temporary (created
// 0600) renamed over the old file, so a reader never sees a partial digest. It never creates the
// config-home directory — a missing directory is a write failure.
func writeConfigEchoState(path, sum string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".config-echo-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()
	if _, err := f.WriteString("sha256:" + sum + "\n"); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return replaceRosterBeacon(tmp, path)
}
