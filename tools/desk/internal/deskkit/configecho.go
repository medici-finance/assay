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
//   - the digest differs from the last one this config home saw, recorded in a small state
//     file beside the roster — so the first run after ANY change prints in full;
//   - that state file cannot be read or cannot be written: a run that cannot prove it has
//     already shown this configuration shows it.
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
// calls this once per run.
func EchoEffectiveConfig(w io.Writer) {
	writeConfigEcho(w, EffectiveConfig(), configEchoStatePath())
}

// writeConfigEcho is EchoEffectiveConfig with its inputs explicit, so the decision is testable
// without the process cache.
func writeConfigEcho(w io.Writer, c Config, statePath string) {
	lines := c.EffectiveConfigLines()
	sum := ConfigDigest(lines)
	if !configEchoFull(c, sum, statePath) {
		fmt.Fprintln(w, compactConfigLine(c, sum))
		return
	}
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
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

// configEchoFull decides the echo form and records the digest when the full block is shown
// outside CI. It returns true (full) on every condition the file comment lists.
func configEchoFull(c Config, sum, statePath string) bool {
	if len(c.Problems) > 0 || len(c.UnknownKeys) > 0 {
		return true
	}
	if echoInCI() {
		// CI history keeps the whole block; a runner's home is not a place to keep state.
		return true
	}
	if !configEchoStateful {
		return true
	}
	if mode := strings.ToLower(strings.TrimSpace(os.Getenv(EnvConfigEcho))); mode != "" && mode != "compact" {
		// Forced full: the pre-digest echo, with no state read or written.
		return true
	}
	last, err := readConfigEchoState(statePath)
	if err != nil || last != sum {
		// The full block is being shown, so this digest is now "seen". A failed write is
		// harmless here (the block prints anyway) and keeps the next run full as well,
		// because nothing recorded the digest.
		_ = writeConfigEchoState(statePath, sum)
		return true
	}
	return false
}

// readConfigEchoState returns the recorded digest. Any failure — absent, unreadable, oversized,
// malformed — is an error, and the caller prints in full.
func readConfigEchoState(path string) (string, error) {
	f, err := os.Open(path)
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
