package deskkit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The local hotfix overlay: one shared definition every layer reads.
//
// A hotfix is a desk-tools binary built by `sudo cellctl hotfix install` from a commit that is
// already on main, stamped `<pin>+hotfix-<sha12>`, installed root-owned beside the pinned
// install, and retired automatically when its pin moves or its expiry passes. Four layers
// check it, each on its own signal:
//
//	layer 1  cellctl shim generation  points a shim at the overlay only for a valid entry
//	layer 2  the binary's self-check  (this file's init) refuses an expired / pin-moved stamp
//	layer 3  deskboot `binary-pin`    classifies every shim by its target path, rechecks the forge
//	layer 4  deskboard                shows `HOTFIX:` on the board header, never a silent pin
//
// The ruled constants live in version.go beside the tag grammar (per the driver's ruling).

const (
	// HotfixOverlayEnv names the overlay directory; unset, DefaultHotfixOverlay.
	HotfixOverlayEnv = "DESK_TOOLS_HOTFIX"
	// DefaultHotfixOverlay is the overlay directory beside the default pinned install.
	DefaultHotfixOverlay = "/opt/desk-tools/hotfix"
	// DeskToolsBinEnv names the pinned install directory; unset, DefaultDeskToolsBin.
	DeskToolsBinEnv = "DESK_TOOLS_BIN"
	// DefaultDeskToolsBin is the pinned install directory desk-install writes.
	DefaultDeskToolsBin = "/opt/desk-tools/bin"
	// HotfixFallbackEnv carries the pinned binary a hotfix shim falls back to (layer 2).
	HotfixFallbackEnv = "DESK_HOTFIX_FALLBACK"
	// HotfixManifestName is the manifest file inside the overlay directory.
	HotfixManifestName = "manifest.json"
	// HotfixManifestSchema is the manifest's schema marker.
	HotfixManifestSchema = "hotfix-manifest-v1"

	// HotfixRepoURL is the ONLY source a hotfix is cloned from. Compiled in: no flag, env var
	// or git remote config changes it.
	HotfixRepoURL = "https://github.com/medici-finance/assay.git"
	// HotfixRepoSlug is the forge repository the source rule is decided against.
	HotfixRepoSlug = "medici-finance/assay"
	// HotfixForgeAPI is the forge API base the source rule is decided against, over HTTPS.
	HotfixForgeAPI = "https://api.github.com"
)

// The reasons a hotfix is refused. Every layer prints these exact phrases.
const (
	HotfixReasonNotRootOwned = "hotfix overlay not root-owned"
	HotfixReasonTampered     = "hotfix tampered"
	HotfixReasonOutlived     = "hotfix outlived its pin"
	HotfixReasonExpired      = "hotfix expired"
	HotfixReasonCeiling      = "hotfix exceeds the ruled ceiling"
	HotfixReasonSourceRule   = "hotfix source rule not the ruled one"
	HotfixReasonNotOnMain    = "hotfix source not on main"
	HotfixReasonUnstamped    = "unstamped binary in hotfix overlay"
	HotfixReasonIneligible   = "hotfix-ineligible"
	HotfixReasonNoPin        = "hotfix could-not-check: no pin resolves"
)

// HotfixOverlayDir is the overlay directory the environment names, else the default.
func HotfixOverlayDir() string {
	if v := strings.TrimSpace(os.Getenv(HotfixOverlayEnv)); v != "" {
		return v
	}
	return DefaultHotfixOverlay
}

// DeskToolsBinDir is the pinned install directory the environment names, else the default.
func DeskToolsBinDir() string {
	if v := strings.TrimSpace(os.Getenv(DeskToolsBinEnv)); v != "" {
		return v
	}
	return DefaultDeskToolsBin
}

// HotfixEntry is one manifest entry: the eleven fields install records per tool.
type HotfixEntry struct {
	Tool        string `json:"tool"`
	SHA256      string `json:"sha256"`
	SourceSHA   string `json:"source_sha"`
	BasePin     string `json:"base_pin"`
	BuiltAt     string `json:"built_at"`
	Expires     string `json:"expires"`
	SourceRule  string `json:"source_rule"`
	Ack         string `json:"ack"`
	InstalledBy string `json:"installed_by"`
	GoVersion   string `json:"go_version"`
	GoSumSHA256 string `json:"go_sum_sha256"`
}

// Tag is the stamp the entry's binary carries.
func (e HotfixEntry) Tag() string { return HotfixTag(e.BasePin, e.SourceSHA) }

// HotfixManifest is the overlay's manifest.json.
type HotfixManifest struct {
	Schema  string        `json:"schema"`
	Entries []HotfixEntry `json:"entries"`
}

// Entry returns the manifest's entry for tool.
func (m *HotfixManifest) Entry(tool string) (HotfixEntry, bool) {
	if m == nil {
		return HotfixEntry{}, false
	}
	for _, e := range m.Entries {
		if e.Tool == tool {
			return e, true
		}
	}
	return HotfixEntry{}, false
}

// HotfixManifestPath is the manifest's path inside overlay.
func HotfixManifestPath(overlay string) string { return filepath.Join(overlay, HotfixManifestName) }

// HotfixBinaryPath is tool's binary path inside overlay.
func HotfixBinaryPath(overlay, tool string) string { return filepath.Join(overlay, "bin", tool) }

// ReadHotfixManifest reads overlay's manifest. A missing manifest is (nil, os.ErrNotExist).
func ReadHotfixManifest(overlay string) (*HotfixManifest, error) {
	b, err := os.ReadFile(HotfixManifestPath(overlay))
	if err != nil {
		return nil, err
	}
	var m HotfixManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("manifest unreadable: %w", err)
	}
	if m.Schema != HotfixManifestSchema {
		return nil, fmt.Errorf("manifest schema %q, want %q", m.Schema, HotfixManifestSchema)
	}
	return &m, nil
}

// WriteHotfixManifest writes m to overlay's manifest atomically, mode 0644.
func WriteHotfixManifest(overlay string, m *HotfixManifest) error {
	m.Schema = HotfixManifestSchema
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(overlay, ".manifest-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, HotfixManifestPath(overlay))
}

// FileSHA256 is the lowercase hex sha256 of the file at p.
func FileSHA256(p string) (string, error) {
	f, err := os.Open(p)
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

// SameReleaseTag reports whether two release tags name the same release, ignoring an
// optional `<artifact>/` prefix (deskboard's normalizeTag rule). A hotfix tag never equals
// its base: the `+hotfix-` suffix is part of the comparison.
func SameReleaseTag(a, b string) bool {
	strip := func(t string) string {
		if i := strings.LastIndex(t, "/"); i >= 0 {
			return t[i+1:]
		}
		return t
	}
	a, b = strip(strings.TrimSpace(a)), strip(strings.TrimSpace(b))
	return a != "" && a == b
}

// HotfixOwnerCheck refuses a path that is not owned by the hotfix owner (root) or that it or
// a parent leaves group- or world-writable.
type HotfixOwnerCheck func(path string) error

// HotfixRootOwned is the real, unseamed owner check: every hotfix file is owned by uid 0.
func HotfixRootOwned() HotfixOwnerCheck { return HotfixOwnedBy(0) }

// HotfixError is a hotfix refusal: Reason is one of the HotfixReason* phrases, Detail the
// specifics, Code the exit class (1 refused, 6 could-not-check).
type HotfixError struct {
	Reason string
	Detail string
	Code   int
}

func (e *HotfixError) Error() string {
	if e.Detail == "" {
		return e.Reason
	}
	return e.Reason + ": " + e.Detail
}

func hotfixErr(code int, reason, format string, a ...any) *HotfixError {
	return &HotfixError{Reason: reason, Detail: fmt.Sprintf(format, a...), Code: code}
}

// HotfixCodeOf is the exit class of a hotfix error: its Code, else 6 for any other error.
func HotfixCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var he *HotfixError
	if errors.As(err, &he) && he.Code != 0 {
		return he.Code
	}
	return 6
}

// ErrNoHotfix reports that the overlay carries no hotfix for the tool asked about.
var ErrNoHotfix = errors.New("no hotfix for this tool")

// ValidateHotfix runs every OFFLINE check on tool's overlay entry against pin at now. It
// returns ErrNoHotfix when the overlay carries neither a binary nor an entry for tool, the
// valid entry, or a *HotfixError naming the first failed check. The forge re-check is the
// caller's (it needs the network); everything here reads only local files.
func ValidateHotfix(overlay, tool, pin string, now time.Time, owner HotfixOwnerCheck) (HotfixEntry, error) {
	bin := HotfixBinaryPath(overlay, tool)
	_, binErr := os.Lstat(bin)
	binExists := binErr == nil
	if _, err := os.Stat(overlay); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return HotfixEntry{}, ErrNoHotfix
		}
		return HotfixEntry{}, hotfixErr(6, HotfixReasonNotRootOwned, "overlay %s unreadable: %v", overlay, err)
	}
	if err := owner(overlay); err != nil {
		return HotfixEntry{}, hotfixErr(6, HotfixReasonNotRootOwned, "%v", err)
	}
	mpath := HotfixManifestPath(overlay)
	if _, err := os.Lstat(mpath); err != nil {
		if errors.Is(err, os.ErrNotExist) && !binExists {
			return HotfixEntry{}, ErrNoHotfix
		}
		if errors.Is(err, os.ErrNotExist) {
			return HotfixEntry{}, hotfixErr(1, HotfixReasonTampered, "%s present with no manifest", bin)
		}
		return HotfixEntry{}, hotfixErr(6, HotfixReasonTampered, "manifest %s unreadable: %v", mpath, err)
	}
	if err := owner(mpath); err != nil {
		return HotfixEntry{}, hotfixErr(6, HotfixReasonNotRootOwned, "%v", err)
	}
	m, err := ReadHotfixManifest(overlay)
	if err != nil {
		return HotfixEntry{}, hotfixErr(1, HotfixReasonTampered, "%v", err)
	}
	e, ok := m.Entry(tool)
	if !ok {
		if !binExists {
			return HotfixEntry{}, ErrNoHotfix
		}
		return HotfixEntry{}, hotfixErr(1, HotfixReasonTampered, "%s has no manifest entry", bin)
	}
	if !binExists {
		return e, hotfixErr(1, HotfixReasonTampered, "manifest entry for %s but no binary at %s", tool, bin)
	}
	if err := owner(bin); err != nil {
		return e, hotfixErr(6, HotfixReasonNotRootOwned, "%v", err)
	}
	sum, err := FileSHA256(bin)
	if err != nil {
		return e, hotfixErr(6, HotfixReasonTampered, "sha256 of %s unreadable: %v", bin, err)
	}
	if sum != e.SHA256 {
		return e, hotfixErr(1, HotfixReasonTampered, "sha256 of %s is %s, manifest says %s", bin, sum, e.SHA256)
	}
	if !fullSHA.MatchString(e.SourceSHA) {
		return e, hotfixErr(1, HotfixReasonTampered, "manifest source_sha %q is not a 40-hex commit", e.SourceSHA)
	}
	if strings.TrimSpace(pin) == "" {
		return e, hotfixErr(6, HotfixReasonNoPin, "no desk-tools pin to compare base %s against", e.BasePin)
	}
	if !SameReleaseTag(e.BasePin, pin) {
		return e, hotfixErr(1, HotfixReasonOutlived, "%s is built over %s, the pin is now %s", tool, e.BasePin, pin)
	}
	built, err1 := time.Parse(time.RFC3339, e.BuiltAt)
	expires, err2 := time.Parse(time.RFC3339, e.Expires)
	if err1 != nil || err2 != nil {
		return e, hotfixErr(1, HotfixReasonTampered, "manifest built_at %q / expires %q not RFC 3339", e.BuiltAt, e.Expires)
	}
	if !now.Before(expires) {
		return e, hotfixErr(1, HotfixReasonExpired, "%s expired %s", e.Tag(), expires.UTC().Format(time.RFC3339))
	}
	if expires.Sub(built) > HotfixMaxTTL {
		return e, hotfixErr(1, HotfixReasonCeiling, "%s lives %s, the ruled ceiling is %s", e.Tag(), expires.Sub(built), HotfixMaxTTL)
	}
	if e.SourceRule != HotfixSourceRule {
		return e, hotfixErr(1, HotfixReasonSourceRule, "manifest source_rule %q, the ruled rule is %q", e.SourceRule, HotfixSourceRule)
	}
	return e, nil
}

var fullSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// IsFullSHA reports whether s is a 40-hex lowercase commit id.
func IsFullSHA(s string) bool { return fullSHA.MatchString(s) }

// ParseReleaseTagOutput extracts the `releaseTag=` value a desk tool's --version prints, or
// "" when the output carries none.
func ParseReleaseTagOutput(out string) string {
	for _, f := range strings.Fields(out) {
		if v, ok := strings.CutPrefix(f, "releaseTag="); ok {
			return v
		}
	}
	return ""
}

// WithinDir reports whether p is dir or lies under it, after resolving symlinks on both.
func WithinDir(p, dir string) bool {
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

// HotfixForge decides the source rule against the forge over HTTPS. Reads are
// unauthenticated: the public repository's compare endpoint needs no login, so no role
// credential is ever read for it.
type HotfixForge struct {
	BaseURL string
	Repo    string
	HTTP    *http.Client
}

// DefaultHotfixForge is the compiled forge: HotfixForgeAPI, HotfixRepoSlug.
func DefaultHotfixForge() HotfixForge {
	return HotfixForge{BaseURL: HotfixForgeAPI, Repo: HotfixRepoSlug, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

type hotfixCompare struct {
	Status string `json:"status"`
	Files  []struct {
		Filename         string `json:"filename"`
		PreviousFilename string `json:"previous_filename"`
	} `json:"files"`
}

// compare reads `base...head`. A 404/422 (the forge does not know a side) is refused (1);
// anything else unexpected is could-not-check (6).
func (f HotfixForge) compare(base, head string) (*hotfixCompare, error) {
	client := f.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	u := strings.TrimRight(f.BaseURL, "/") + "/repos/" + f.Repo + "/compare/" +
		url.PathEscape(base) + "..." + url.PathEscape(head)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, hotfixErr(6, HotfixReasonNotOnMain, "could-not-check: %v", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "assay-desk-tools-hotfix")
	resp, err := client.Do(req)
	if err != nil {
		return nil, hotfixErr(6, HotfixReasonNotOnMain, "could-not-check: forge unreachable: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, hotfixErr(6, HotfixReasonNotOnMain, "could-not-check: forge read: %v", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return nil, hotfixErr(1, HotfixReasonNotOnMain, "the forge does not know %s...%s (HTTP %d)", base, head, resp.StatusCode)
	default:
		return nil, hotfixErr(6, HotfixReasonNotOnMain, "could-not-check: forge answered HTTP %d for %s...%s", resp.StatusCode, base, head)
	}
	var c hotfixCompare
	if err := json.Unmarshal(body, &c); err != nil || c.Status == "" {
		return nil, hotfixErr(6, HotfixReasonNotOnMain, "could-not-check: ambiguous compare answer for %s...%s", base, head)
	}
	return &c, nil
}

// CheckSourceOnMain applies HotfixSourceRule: the forge must report `main...<sha>` as
// `behind` or `identical`. `ahead` / `diverged` is refused (1); an unknown sha is refused (1);
// an unreachable forge or an ambiguous answer is could-not-check (6), never a pass. A local
// ref is never consulted.
func (f HotfixForge) CheckSourceOnMain(sha string) error {
	if !IsFullSHA(sha) {
		return hotfixErr(1, HotfixReasonNotOnMain, "source %q is not a 40-hex commit", sha)
	}
	c, err := f.compare("main", sha)
	if err != nil {
		return err
	}
	switch c.Status {
	case "behind", "identical":
		return nil
	case "ahead", "diverged":
		return hotfixErr(1, HotfixReasonNotOnMain, "source rule %s: the forge reports main...%s as %s", HotfixSourceRule, sha, c.Status)
	}
	return hotfixErr(6, HotfixReasonNotOnMain, "could-not-check: forge status %q for main...%s", c.Status, sha)
}

// hotfixCompareFileCap is the forge's compare file-list cap; a diff that reaches it cannot be
// proven eligible, so it is could-not-check.
const hotfixCompareFileCap = 300

// CheckEligible refuses a source whose diff against pin touches a HotfixIneligiblePaths
// entry: exit 1 naming `hotfix-ineligible`, the entry and the first matching path. A source
// not ahead of its pin is refused too (a hotfix never rolls a tool back).
func (f HotfixForge) CheckEligible(pin, sha string) error {
	c, err := f.compare(pin, sha)
	if err != nil {
		var he *HotfixError
		if errors.As(err, &he) {
			he.Reason = HotfixReasonIneligible
		}
		return err
	}
	switch c.Status {
	case "ahead", "diverged":
	case "behind", "identical":
		return hotfixErr(1, HotfixReasonIneligible, "source %s is not ahead of its pin %s (forge: %s)", sha, pin, c.Status)
	default:
		return hotfixErr(6, HotfixReasonIneligible, "could-not-check: forge status %q for %s...%s", c.Status, pin, sha)
	}
	if len(c.Files) >= hotfixCompareFileCap {
		return hotfixErr(6, HotfixReasonIneligible, "could-not-check: %s...%s lists %d files, the forge cap; eligibility unprovable", pin, sha, len(c.Files))
	}
	for _, fl := range c.Files {
		for _, p := range []string{fl.Filename, fl.PreviousFilename} {
			if p == "" {
				continue
			}
			if rule := HotfixIneligibleMatch(p); rule != "" {
				return hotfixErr(1, HotfixReasonIneligible, "%s touches %s (HotfixIneligiblePaths entry %s)", sha, p, rule)
			}
		}
	}
	return nil
}
