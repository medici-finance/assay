package deskkit

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Layer 2: the binary's own self-check, run from this package's init — before any tool's
// main does anything else. A release build outside the overlay directory skips it entirely
// (no `+hotfix-` stamp, not under the overlay), so every pinned binary behaves exactly as
// before. A hotfix build, or ANY binary resolving under the overlay directory, is checked:
// its own expiry, its lifetime against the ruled ceiling, and its base against the pin. On a
// failure it execs DESK_HOTFIX_FALLBACK (the pinned binary its shim named) with a NOTICE;
// with no usable fallback it exits 6 naming the reason. It never proceeds on its expiry alone:
// a pin that does not resolve is could-not-check.

// hotfixSelfInput is everything the self-check reads, gathered once so the decision is a
// pure function a test drives directly.
type hotfixSelfInput struct {
	Tag, Expires, BuiltAt string
	Exe                   string // the running executable, symlinks resolved
	Overlay               string // the overlay directory
	PinnedBin             string // the pinned install directory
	Cwd                   string
	Fallback              string // DESK_HOTFIX_FALLBACK
	Now                   time.Time
	Platform              string
	// PinnedVersion reads a pinned install's release tag (its --version); a test fakes it.
	PinnedVersion func(path string) (string, error)
}

// hotfixSelfVerdict is the self-check's decision: Skip (not a hotfix, not in the overlay),
// proceed (Reason == ""), exec Fallback, or exit 6 with Reason.
type hotfixSelfVerdict struct {
	Skip     bool
	Reason   string
	Fallback string
	Pin      string
}

func evaluateHotfixSelf(in hotfixSelfInput) hotfixSelfVerdict {
	inOverlay := WithinDir(in.Exe, in.Overlay)
	hot := IsHotfix(in.Tag)
	if !hot && !inOverlay {
		return hotfixSelfVerdict{Skip: true}
	}
	v := hotfixSelfVerdict{}
	tool := strings.TrimSuffix(filepath.Base(in.Exe), ".exe")
	fail := func(reason string) hotfixSelfVerdict {
		v.Reason = reason
		if in.Fallback == "" {
			return v
		}
		if WithinDir(in.Fallback, in.PinnedBin) && !WithinDir(in.Fallback, in.Overlay) {
			v.Fallback = in.Fallback
			return v
		}
		v.Reason = reason + " (and " + HotfixFallbackEnv + " " + in.Fallback + " is not under the pinned dir " + in.PinnedBin + ")"
		return v
	}
	if !hot || strings.TrimSpace(in.Expires) == "" {
		return fail(HotfixReasonUnstamped + ": " + in.Exe + " carries tag " + quoteOrDev(in.Tag) + " and no hotfix expiry")
	}
	expires, err := time.Parse(time.RFC3339, in.Expires)
	if err != nil {
		return fail(HotfixReasonUnstamped + ": expiry " + in.Expires + " is not RFC 3339")
	}
	if !in.Now.Before(expires) {
		return fail(HotfixReasonExpired + " " + expires.UTC().Format(time.RFC3339))
	}
	if built, err := time.Parse(time.RFC3339, in.BuiltAt); err != nil || expires.Sub(built) > HotfixMaxTTL {
		return fail(HotfixReasonCeiling + " (built " + in.BuiltAt + ", expires " + in.Expires + ")")
	}
	pin := ""
	if root := NearestPinRoot(in.Cwd); root != "" {
		pin, _, _ = DeskToolsReleasePin(root, in.Platform)
	}
	if pin == "" && in.PinnedVersion != nil && in.PinnedBin != "" {
		if t, err := in.PinnedVersion(filepath.Join(in.PinnedBin, tool)); err == nil && artifactTagPattern.MatchString(t) {
			pin = t
		}
	}
	if pin == "" {
		return fail(HotfixReasonNoPin + " (no .assay-versions above " + in.Cwd + ", no pinned " + tool + " under " + in.PinnedBin + ")")
	}
	v.Pin = pin
	base, _ := HotfixBase(in.Tag)
	if !SameReleaseTag(base, pin) {
		return fail(HotfixReasonOutlived + " (built over " + base + ", the pin is " + pin + ")")
	}
	return v
}

func quoteOrDev(tag string) string {
	if tag == "" {
		return devRelease
	}
	return tag
}

// pinnedVersionReal runs a pinned install's --version with the fallback variable unset.
func pinnedVersionReal(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Env = hotfixEnvWithout(os.Environ(), HotfixFallbackEnv)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return ParseReleaseTagOutput(string(out)), nil
}

func hotfixEnvWithout(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func init() { runHotfixSelfCheck() }

func runHotfixSelfCheck() {
	exe := executablePath()
	if !IsHotfix(ReleaseTag) && !WithinDir(exe, HotfixOverlayDir()) {
		return // a release build outside the overlay: the check is skipped entirely
	}
	cwd, _ := os.Getwd()
	in := hotfixSelfInput{
		Tag: ReleaseTag, Expires: HotfixExpires, BuiltAt: BuiltAt,
		Exe: exe, Overlay: HotfixOverlayDir(), PinnedBin: DeskToolsBinDir(),
		Cwd: cwd, Fallback: os.Getenv(HotfixFallbackEnv), Now: time.Now(),
		Platform: runtime.GOOS + "-" + runtime.GOARCH, PinnedVersion: pinnedVersionReal,
	}
	applyHotfixSelfVerdict(evaluateHotfixSelf(in), ReleaseTag, os.Stderr, hotfixExecFallback, os.Exit)
}

// applyHotfixSelfVerdict acts on the verdict: proceed, exec the pinned fallback with a
// NOTICE (the fallback variable stripped so the pinned binary never re-checks against it),
// or exit 6 naming the reason.
func applyHotfixSelfVerdict(v hotfixSelfVerdict, tag string, stderr io.Writer,
	execFn func(path string, args, env []string) int, exit func(int)) {
	if v.Skip || v.Reason == "" {
		return
	}
	if v.Fallback != "" {
		fmt.Fprintf(stderr, "desk-tools NOTICE: hotfix %s refused (%s); running pinned %s\n", quoteOrDev(tag), v.Reason, v.Fallback)
		exit(execFn(v.Fallback, os.Args[1:], hotfixEnvWithout(os.Environ(), HotfixFallbackEnv)))
		return
	}
	fmt.Fprintf(stderr, "desk-tools: refusing to run: %s\n", v.Reason)
	exit(ExitUnverifiable)
}

func executablePath() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
