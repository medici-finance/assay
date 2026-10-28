package deskkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Layer 2: the binary's own self-check, run from this package's init — before any tool's
// main does anything else. A release build outside the overlay directories skips it entirely
// (no `+hotfix-` stamp, not under the overlay the environment names nor the compiled-in one),
// so every pinned binary behaves exactly as before. ANY binary under either overlay directory
// must carry a hotfix stamp, and EVERY stamped binary, wherever it runs from, is checked: the
// root ownership of the overlay, its bin, its manifest and the running file, the running file's
// sha256 and stamp against the manifest entry, the source rule the entry records, its own
// expiry, its lifetime against the ruled ceiling, and its base against the pin. A stamped binary
// outside the overlay, or an overlay the environment redirects elsewhere, has no root-owned
// entry matching it and is refused.
// It shares no check call with the shim (ChooseHotfixShim) or the boot (ValidateHotfix): it
// hashes and owner-checks the running file itself. It makes no network call: the forge
// decision is the install's and the boot's (layer 3), so an offline tool start never depends
// on it. On a
// failure it execs DESK_HOTFIX_FALLBACK (the pinned binary its shim named) with a NOTICE;
// with no usable fallback it exits 6 naming the reason. It never proceeds on its expiry alone:
// a pin that does not resolve is could-not-check.

// hotfixSelfInput is everything the self-check reads, gathered once so the decision is a
// pure function a test drives directly.
type hotfixSelfInput struct {
	Tag, Expires, BuiltAt string
	Exe                   string // the running executable, symlinks resolved
	Overlay               string // the overlay directory
	// DefaultOverlay is the compiled-in overlay directory. A binary resolving under it is a
	// hotfix-overlay binary whatever the environment names as the overlay.
	DefaultOverlay string
	PinnedBin      string // the pinned install directory
	// Owner is the ownership check layer 2 applies to the running file, the manifest and the
	// overlay. Nil is a refusal, never a pass.
	Owner    HotfixOwnerCheck
	Cwd      string
	Fallback string // DESK_HOTFIX_FALLBACK
	Now      time.Time
	Platform string
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
	inOverlay := WithinDir(in.Exe, in.Overlay) || WithinDir(in.Exe, in.DefaultOverlay)
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
	// Every stamped binary is integrity-checked, wherever it runs from and whatever the
	// environment names as the overlay: a stamped copy outside the overlay, or an overlay the
	// environment redirects, has no root-owned manifest entry matching it and is refused.
	if reason := selfIntegrity(in, tool); reason != "" {
		return fail(reason)
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

// stagingNameRE is the name `cellctl hotfix install` gives the file it stages in the overlay's
// bin directory (os.CreateTemp of "."+tool+".staging-"): a dot, the tool, ".staging-" and
// digits. Install runs that file's --version once, as the invoking user, BEFORE its manifest
// entry exists (the manifest is written last), so no digest or entry can exist to check. The
// file is created by the root install step, chowned to root before the probe, removed on every
// exit path, and no shim, boot classification or manifest entry ever names it; layers 1 and 3
// refuse any path that is not exactly the overlay's tool binary. Only the manifest, entry,
// digest, stamp and source-rule checks are skipped for it: the owner checks on the overlay, its
// bin and the file itself, and expiry, ceiling and pin, still apply.
var stagingNameRE = regexp.MustCompile(`^\.[A-Za-z0-9_-]+\.staging-[0-9]+$`)

func isHotfixStagingFile(in hotfixSelfInput) bool {
	return filepath.Dir(in.Exe) == filepath.Join(in.Overlay, "bin") && stagingNameRE.MatchString(filepath.Base(in.Exe))
}

// selfIntegrity is layer 2's own ownership, sha256 and source-rule decision for every stamped
// binary, checked against the overlay it is told of; "" means it holds. Its own code, not
// ValidateHotfix: a fault in the boot's or the shim's check leaves this one standing.
func selfIntegrity(in hotfixSelfInput, tool string) string {
	if in.Owner == nil {
		return HotfixReasonNotRootOwned + ": no owner check supplied"
	}
	for _, p := range []string{in.Overlay, filepath.Join(in.Overlay, "bin"), in.Exe} {
		if err := in.Owner(p); err != nil {
			return HotfixReasonNotRootOwned + ": " + err.Error()
		}
	}
	if isHotfixStagingFile(in) {
		return ""
	}
	if err := in.Owner(HotfixManifestPath(in.Overlay)); err != nil {
		return HotfixReasonNotRootOwned + ": " + err.Error()
	}
	m, err := ReadHotfixManifest(in.Overlay)
	if err != nil {
		return HotfixReasonTampered + ": " + err.Error()
	}
	e, ok := m.Entry(tool)
	if !ok {
		return HotfixReasonTampered + ": " + in.Exe + " has no manifest entry"
	}
	got, err := selfDigest(in.Exe)
	if err != nil {
		return HotfixReasonTampered + ": sha256 of " + in.Exe + " unreadable: " + err.Error()
	}
	if got != e.SHA256 {
		return HotfixReasonTampered + ": sha256 of " + in.Exe + " is " + got + ", manifest says " + e.SHA256
	}
	if e.Tag() != in.Tag {
		return HotfixReasonTampered + ": " + in.Exe + " is stamped " + quoteOrDev(in.Tag) + ", manifest says " + e.Tag()
	}
	if e.SourceRule != HotfixSourceRule {
		return HotfixReasonSourceRule + ": manifest source_rule " + e.SourceRule + ", the ruled rule is " + HotfixSourceRule
	}
	return ""
}

func selfDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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

// hotfixSelfApplies is the self-check's gate: a stamped binary anywhere, or any binary under
// the overlay the environment names OR the compiled-in one, so the environment alone never
// turns the check off.
func hotfixSelfApplies(tag, exe, defaultOverlay string) bool {
	return IsHotfix(tag) || WithinDir(exe, HotfixOverlayDir()) || WithinDir(exe, defaultOverlay)
}

func runHotfixSelfCheck() {
	exe := executablePath()
	if !hotfixSelfApplies(ReleaseTag, exe, DefaultHotfixOverlay) {
		return // a release build outside the overlay: the check is skipped entirely
	}
	cwd, _ := os.Getwd()
	in := hotfixSelfInput{
		Tag: ReleaseTag, Expires: HotfixExpires, BuiltAt: BuiltAt,
		Exe: exe, Overlay: HotfixOverlayDir(), DefaultOverlay: DefaultHotfixOverlay, PinnedBin: DeskToolsBinDir(), Owner: ActiveHotfixOwner(),
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
