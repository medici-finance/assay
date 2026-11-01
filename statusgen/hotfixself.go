package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Layer 2 of the local hotfix overlay, for statusgen: the binary's own self-check, run from
// this package's init before main does anything else. statusgen is its own Go module and does
// not import the desk tools' deskkit package, so this is statusgen's own copy of deskkit's
// layer 2 (tools/desk/internal/deskkit/hotfixself.go), held to deskkit's compiled values by
// TestStatusgenHotfixParity. Two differences, both because statusgen ships as its own release
// asset: the pin is statusgen's own `.assay-versions` line (the bare `statusgen` line, else this
// platform's `statusgen-<os>-<arch>` line), never the desk-tools line; and outside a checkout
// the pinned install's `statusgen --version` prints the bare tag on one line.
//
// A release build outside the overlay directories skips it entirely (no `+hotfix-` stamp, not
// under the overlay the environment names nor the compiled-in one). ANY binary under either
// overlay directory must carry a hotfix stamp, and EVERY stamped binary is checked: the root
// ownership of the overlay, its bin, its manifest and the running file; the running file's
// sha256 and stamp against the manifest entry; the source rule; its own expiry; its lifetime
// against the ruled ceiling; and its base against the pin. It makes no network call. On a
// failure it execs DESK_HOTFIX_FALLBACK (the pinned binary its shim named) with a NOTICE; with
// no usable fallback it exits 6 naming the reason. It never proceeds on its expiry alone: a pin
// that does not resolve is could-not-check.

// statusgenHotfixExpires and statusgenBuiltAt are stamped by `cellctl hotfix install` only;
// the release build never sets them, so a release binary carries neither.
var (
	statusgenHotfixExpires = ""
	statusgenBuiltAt       = ""
)

// The overlay's compiled values, equal to deskkit's (TestStatusgenHotfixParity).
const (
	sgHotfixOverlayEnv     = "DESK_TOOLS_HOTFIX"
	sgDefaultHotfixOverlay = "/opt/desk-tools/hotfix"
	sgDeskToolsBinEnv      = "DESK_TOOLS_BIN"
	sgDefaultDeskToolsBin  = "/opt/desk-tools/bin"
	sgHotfixFallbackEnv    = "DESK_HOTFIX_FALLBACK"
	sgHotfixManifestName   = "manifest.json"
	sgHotfixManifestSchema = "hotfix-manifest-v1"
	// sgHotfixSourceRule and sgHotfixMaxTTL are the driver's ruled values: compiled, never a
	// flag, env var or file.
	sgHotfixSourceRule = "merged-main"
	sgHotfixMaxTTL     = 7 * 24 * time.Hour
	// sgHotfixSeamRootUID is read only in a binary built with the test-only hotfixseam tag.
	sgHotfixSeamRootUID = "ASSAY_HOTFIXSEAM_ROOT_UID"
	sgExitUnverifiable  = 6
)

// The refusal phrases, equal to deskkit's so every layer prints the same words.
const (
	sgReasonNotRootOwned = "hotfix overlay not root-owned"
	sgReasonTampered     = "hotfix tampered"
	sgReasonOutlived     = "hotfix outlived its pin"
	sgReasonExpired      = "hotfix expired"
	sgReasonCeiling      = "hotfix exceeds the ruled ceiling"
	sgReasonSourceRule   = "hotfix source rule not the ruled one"
	sgReasonUnstamped    = "unstamped binary in hotfix overlay"
	sgReasonNoPin        = "hotfix could-not-check: no pin resolves"
)

// sgPinTagPattern is a release pin tag: bare vX.Y.Z, optionally behind an artifact prefix.
var sgPinTagPattern = regexp.MustCompile(`^([a-z][a-z0-9-]*/)?v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// sgHotfixInput is everything the self-check reads, gathered once so the decision is a pure
// function a test drives directly.
type sgHotfixInput struct {
	Tag, Expires, BuiltAt string
	Exe                   string // the running executable, symlinks resolved
	Overlay               string // the overlay directory
	DefaultOverlay        string // the compiled-in overlay directory
	PinnedBin             string // the pinned install directory
	// Owner is the ownership check applied to the overlay, its bin, the manifest and the
	// running file. Nil is a refusal, never a pass.
	Owner    func(path string) error
	Cwd      string
	Fallback string // DESK_HOTFIX_FALLBACK
	Now      time.Time
	Platform string
}

// sgHotfixVerdict: Skip (not a hotfix, not in the overlay), proceed (Reason == ""), exec
// Fallback, or exit 6 with Reason.
type sgHotfixVerdict struct {
	Skip     bool
	Reason   string
	Fallback string
	Pin      string
}

// sgHotfixEntry is the manifest entry fields the self-check reads (deskkit's HotfixEntry).
type sgHotfixEntry struct {
	Tool       string `json:"tool"`
	SHA256     string `json:"sha256"`
	SourceSHA  string `json:"source_sha"`
	BasePin    string `json:"base_pin"`
	SourceRule string `json:"source_rule"`
}

type sgHotfixManifest struct {
	Schema  string          `json:"schema"`
	Entries []sgHotfixEntry `json:"entries"`
}

func sgEvaluateHotfix(in sgHotfixInput) sgHotfixVerdict {
	inOverlay := sgWithinDir(in.Exe, in.Overlay) || sgWithinDir(in.Exe, in.DefaultOverlay)
	base, hot := hotfixBase(in.Tag)
	if !hot && !inOverlay {
		return sgHotfixVerdict{Skip: true}
	}
	v := sgHotfixVerdict{}
	fail := func(reason string) sgHotfixVerdict {
		v.Reason = reason
		if in.Fallback == "" {
			return v
		}
		if sgWithinDir(in.Fallback, in.PinnedBin) && !sgWithinDir(in.Fallback, in.Overlay) && !sgWithinDir(in.Fallback, in.DefaultOverlay) {
			v.Fallback = in.Fallback
			return v
		}
		v.Reason = reason + " (and " + sgHotfixFallbackEnv + " " + in.Fallback + " is not under the pinned dir " + in.PinnedBin + ")"
		return v
	}
	if !hot || strings.TrimSpace(in.Expires) == "" {
		return fail(sgReasonUnstamped + ": " + in.Exe + " carries tag " + sgQuoteOrDev(in.Tag) + " and no hotfix expiry")
	}
	if reason := sgSelfIntegrity(in); reason != "" {
		return fail(reason)
	}
	expires, err := time.Parse(time.RFC3339, in.Expires)
	if err != nil {
		return fail(sgReasonUnstamped + ": expiry " + in.Expires + " is not RFC 3339")
	}
	if !in.Now.Before(expires) {
		return fail(sgReasonExpired + " " + expires.UTC().Format(time.RFC3339))
	}
	if built, err := time.Parse(time.RFC3339, in.BuiltAt); err != nil || expires.Sub(built) > sgHotfixMaxTTL {
		return fail(sgReasonCeiling + " (built " + in.BuiltAt + ", expires " + in.Expires + ")")
	}
	// Only a statusgen pin line proves the base. Unlike the desk tools' copy, this one never
	// asks the pinned install for its --version: that would be a new process launch in
	// statusgen, so outside a pinned checkout a statusgen hotfix is could-not-check.
	pin := sgHotfixPin(in.Cwd, in.Platform)
	if pin == "" {
		return fail(sgReasonNoPin + " (no statusgen pin line in a .assay-versions above " + in.Cwd + ")")
	}
	v.Pin = pin
	if sgBareTag(base) != sgBareTag(pin) {
		return fail(sgReasonOutlived + " (built over " + base + ", the statusgen pin is " + pin + ")")
	}
	return v
}

// sgHotfixPin is statusgen's pin from the nearest `.assay-versions` at or above cwd: the bare
// `statusgen` line, else (Windows) `statusgen-<platform>.exe`, else `statusgen-<platform>`.
// A winning line that is malformed or not a release tag resolves nothing: a broken record is
// never skipped in favour of another line. "" when none resolves.
func sgHotfixPin(cwd, platform string) string {
	if cwd == "" {
		return ""
	}
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	var raw []byte
	for {
		if b, err := os.ReadFile(filepath.Join(dir, ".assay-versions")); err == nil {
			raw = b
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	names := []string{"statusgen"}
	if p := strings.TrimSpace(platform); p != "" {
		if strings.HasPrefix(p, "windows-") {
			names = append(names, "statusgen-"+p+".exe")
		}
		names = append(names, "statusgen-"+p)
	}
	lines := strings.Split(string(raw), "\n")
	for _, n := range names {
		for _, line := range lines {
			f := strings.Fields(line)
			if len(f) == 0 || f[0] != n {
				continue
			}
			if len(f) < 3 || !sgPinTagPattern.MatchString(f[1]) {
				return ""
			}
			return f[1]
		}
	}
	return ""
}

// sgStagingNameRE is the name `cellctl hotfix install` gives the file it stages in the
// overlay's bin directory before its manifest entry exists (deskkit's stagingNameRE). Only the
// manifest, entry, digest, stamp and source-rule checks are skipped for it: the owner checks on
// the overlay, its bin and the file, and expiry, ceiling and pin, still apply.
var sgStagingNameRE = regexp.MustCompile(`^\.[A-Za-z0-9_-]+\.staging-[0-9]+$`)

func sgIsStagingFile(in sgHotfixInput) bool {
	return filepath.Dir(in.Exe) == filepath.Join(in.Overlay, "bin") && sgStagingNameRE.MatchString(filepath.Base(in.Exe))
}

// sgSelfIntegrity is the ownership, sha256, stamp and source-rule decision; "" means it holds.
func sgSelfIntegrity(in sgHotfixInput) string {
	if in.Owner == nil {
		return sgReasonNotRootOwned + ": no owner check supplied"
	}
	for _, p := range []string{in.Overlay, filepath.Join(in.Overlay, "bin"), in.Exe} {
		if err := in.Owner(p); err != nil {
			return sgReasonNotRootOwned + ": " + err.Error()
		}
	}
	if sgIsStagingFile(in) {
		return ""
	}
	mpath := filepath.Join(in.Overlay, sgHotfixManifestName)
	if err := in.Owner(mpath); err != nil {
		return sgReasonNotRootOwned + ": " + err.Error()
	}
	b, err := os.ReadFile(mpath)
	if err != nil {
		return sgReasonTampered + ": " + err.Error()
	}
	var m sgHotfixManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return sgReasonTampered + ": manifest unreadable: " + err.Error()
	}
	if m.Schema != sgHotfixManifestSchema {
		return sgReasonTampered + ": manifest schema " + strconv.Quote(m.Schema) + ", want " + strconv.Quote(sgHotfixManifestSchema)
	}
	var e *sgHotfixEntry
	for i := range m.Entries {
		if m.Entries[i].Tool == "statusgen" {
			e = &m.Entries[i]
			break
		}
	}
	if e == nil {
		return sgReasonTampered + ": " + in.Exe + " has no manifest entry"
	}
	got, err := sgFileDigest(in.Exe)
	if err != nil {
		return sgReasonTampered + ": sha256 of " + in.Exe + " unreadable: " + err.Error()
	}
	if got != e.SHA256 {
		return sgReasonTampered + ": sha256 of " + in.Exe + " is " + got + ", manifest says " + e.SHA256
	}
	want := e.BasePin + "+hotfix-"
	if len(e.SourceSHA) >= 12 {
		want += e.SourceSHA[:12]
	}
	if want != in.Tag {
		return sgReasonTampered + ": " + in.Exe + " is stamped " + sgQuoteOrDev(in.Tag) + ", manifest says " + want
	}
	if e.SourceRule != sgHotfixSourceRule {
		return sgReasonSourceRule + ": manifest source_rule " + e.SourceRule + ", the ruled rule is " + sgHotfixSourceRule
	}
	return ""
}

func sgFileDigest(path string) (string, error) {
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

func sgQuoteOrDev(tag string) string {
	if tag == "" {
		return "dev"
	}
	return tag
}

// sgBareTag strips an artifact prefix (`statusgen/v1.0.41` → `v1.0.41`).
func sgBareTag(t string) string {
	t = strings.TrimSpace(t)
	if i := strings.LastIndex(t, "/"); i >= 0 {
		return t[i+1:]
	}
	return t
}

// sgWithinDir reports whether p is dir or lies under it, after resolving symlinks on both.
func sgWithinDir(p, dir string) bool {
	if p == "" || dir == "" {
		return false
	}
	rp, err := filepath.EvalSymlinks(p)
	if err != nil {
		rp = filepath.Clean(p)
	}
	rd, err := filepath.EvalSymlinks(dir)
	if err != nil {
		rd = filepath.Clean(dir)
	}
	rel, err := filepath.Rel(rd, rp)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// sgFallbackEnv is the one environment the layer-2 check hands a process: the caller's own,
// with only the fallback variable removed. It reaches exactly one program, the pinned
// install replacing this process, and never a Verify row (TestRowEnvSingleSource lists it beside the other env-preserving wrappers).
func sgFallbackEnv() []string { return sgEnvWithout(os.Environ(), sgHotfixFallbackEnv) }

func sgEnvWithout(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func sgEnvDir(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// sgHotfixApplies is the self-check's gate: a stamped binary anywhere, or any binary under the
// overlay the environment names OR the compiled-in one, so the environment alone never turns
// the check off.
func sgHotfixApplies(tag, exe, overlay, defaultOverlay string) bool {
	_, hot := hotfixBase(tag)
	return hot || sgWithinDir(exe, overlay) || sgWithinDir(exe, defaultOverlay)
}

// sgActiveOwner is root's ownership check, unless a hotfixseam build names another uid.
func sgActiveOwner() func(string) error {
	if v := strings.TrimSpace(sgHotfixSeamGet(sgHotfixSeamRootUID)); v != "" {
		if uid, err := strconv.ParseUint(v, 10, 32); err == nil {
			return sgHotfixOwnedBy(uint32(uid))
		}
	}
	return sgHotfixOwnedBy(0)
}

func init() { sgRunHotfixSelfCheck() }

func sgRunHotfixSelfCheck() {
	exe := sgExecutablePath()
	overlay := sgEnvDir(sgHotfixOverlayEnv, sgDefaultHotfixOverlay)
	if !sgHotfixApplies(statusgenVersion, exe, overlay, sgDefaultHotfixOverlay) {
		return // a release build outside the overlay: the check is skipped entirely
	}
	cwd, _ := os.Getwd()
	in := sgHotfixInput{
		Tag: statusgenVersion, Expires: statusgenHotfixExpires, BuiltAt: statusgenBuiltAt,
		Exe: exe, Overlay: overlay, DefaultOverlay: sgDefaultHotfixOverlay,
		PinnedBin: sgEnvDir(sgDeskToolsBinEnv, sgDefaultDeskToolsBin), Owner: sgActiveOwner(),
		Cwd: cwd, Fallback: os.Getenv(sgHotfixFallbackEnv), Now: time.Now(),
		Platform: runtime.GOOS + "-" + runtime.GOARCH,
	}
	sgApplyHotfixVerdict(sgEvaluateHotfix(in), statusgenVersion, os.Stderr, sgExecFallback, exitProcess)
}

// sgApplyHotfixVerdict acts on the verdict: proceed, exec the pinned fallback with a NOTICE
// (the fallback variable stripped so the pinned binary never re-checks against it), or exit 6
// naming the reason.
func sgApplyHotfixVerdict(v sgHotfixVerdict, tag string, stderr io.Writer,
	execFn func(path string, args, env []string) int, exit func(int)) {
	if v.Skip || v.Reason == "" {
		return
	}
	if v.Fallback != "" {
		fmt.Fprintf(stderr, "statusgen NOTICE: hotfix %s refused (%s); running pinned %s\n", sgQuoteOrDev(tag), v.Reason, v.Fallback)
		exit(execFn(v.Fallback, os.Args[1:], sgFallbackEnv()))
		return
	}
	fmt.Fprintf(stderr, "statusgen: refusing to run: %s\n", v.Reason)
	exit(sgExitUnverifiable)
}

func sgExecutablePath() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
