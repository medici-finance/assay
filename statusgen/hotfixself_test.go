//go:build unix

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// sgSelfFixture is a temp overlay holding one statusgen hotfix with its manifest entry, a
// pinned install dir and a checkout whose .assay-versions pins statusgen. The owner check is
// bound to the test user's uid (the production check is root's).
type sgSelfFixture struct {
	overlay, pinned, repo, exe string
	in                         sgHotfixInput
}

const (
	sgFixSource = "0123456789abcdef0123456789abcdef01234567"
	sgFixBase   = "v1.0.41"
)

func sgFixWrite(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func sgFixPins(t *testing.T, repo, body string) {
	t.Helper()
	sgFixWrite(t, filepath.Join(repo, ".assay-versions"), body, 0o644)
}

func newSgSelfFixture(t *testing.T) *sgSelfFixture {
	t.Helper()
	base := t.TempDir()
	f := &sgSelfFixture{
		overlay: filepath.Join(base, "overlay"),
		pinned:  filepath.Join(base, "pinned"),
		repo:    filepath.Join(base, "repo"),
	}
	f.exe = filepath.Join(f.overlay, "bin", "statusgen")
	content := "hotfix build of statusgen\n"
	sgFixWrite(t, f.exe, content, 0o755)
	sgFixWrite(t, filepath.Join(f.pinned, "statusgen"), "pinned\n", 0o755)
	sum := sha256.Sum256([]byte(content))
	m := map[string]any{
		"schema": sgHotfixManifestSchema,
		"entries": []map[string]string{{
			"tool": "statusgen", "sha256": hex.EncodeToString(sum[:]), "source_sha": sgFixSource,
			"base_pin": sgFixBase, "built_at": "2026-10-26T00:00:00Z", "expires": "2026-10-28T00:00:00Z",
			"source_rule": sgHotfixSourceRule, "ack": "per the driver's ruling", "installed_by": "operator",
			"go_version": "go1.27.2", "go_sum_sha256": "00",
		}},
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	sgFixWrite(t, filepath.Join(f.overlay, sgHotfixManifestName), string(b), 0o644)
	sgFixPins(t, f.repo, "desk-tools v1.0.42 "+strings.Repeat("0", 64)+"\nstatusgen "+sgFixBase+" "+strings.Repeat("1", 64)+"\n")
	f.in = sgHotfixInput{
		Tag: sgFixBase + "+hotfix-" + sgFixSource[:12], Expires: "2026-10-28T00:00:00Z", BuiltAt: "2026-10-26T00:00:00Z",
		Exe: f.exe, Overlay: f.overlay, DefaultOverlay: sgDefaultHotfixOverlay, PinnedBin: f.pinned,
		Owner: sgHotfixOwnedBy(uint32(os.Getuid())), Cwd: f.repo, Now: mustSgTime(t, "2026-10-27T00:00:00Z"),
		Platform: "linux-amd64",
	}
	return f
}

func mustSgTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// regression: #4405
// TestStatusgenHotfixSelf: statusgen's own layer 2, with layers 1 and 3 bypassed (no shim, no
// boot step runs). A valid hotfix over statusgen's own pin line proceeds; the desk-tools line
// is never its pin. Expired, pin-moved, tampered, over-ceiling, wrong-rule, unstamped and
// unprovable-pin hotfixes are refused: they exec DESK_HOTFIX_FALLBACK when it is under the
// pinned dir, else exit 6 naming the reason. A release build outside the overlay skips.
func TestStatusgenHotfixSelf(t *testing.T) {
	t.Run("valid over its own pin line", func(t *testing.T) {
		f := newSgSelfFixture(t)
		v := sgEvaluateHotfix(f.in)
		if v.Skip || v.Reason != "" || v.Pin != sgFixBase {
			t.Fatalf("valid hotfix verdict = %+v", v)
		}
	})
	t.Run("release build outside the overlay skips", func(t *testing.T) {
		f := newSgSelfFixture(t)
		f.in.Tag, f.in.Expires, f.in.Exe = "v1.0.41", "", filepath.Join(f.pinned, "statusgen")
		if v := sgEvaluateHotfix(f.in); !v.Skip {
			t.Fatalf("release build outside the overlay: %+v", v)
		}
		if sgHotfixApplies("dev", filepath.Join(f.pinned, "statusgen"), f.overlay, sgDefaultHotfixOverlay) {
			t.Fatal("a dev build outside the overlay applies the check")
		}
		if !sgHotfixApplies("dev", f.exe, f.overlay, sgDefaultHotfixOverlay) {
			t.Fatal("a binary under the overlay skips the check")
		}
	})
	refused := []struct {
		name, reason string
		edit         func(f *sgSelfFixture)
	}{
		{"unstamped in the overlay", "unstamped binary in hotfix overlay", func(f *sgSelfFixture) { f.in.Tag, f.in.Expires = "v1.0.41", "" }},
		{"stamp with no expiry", "unstamped binary in hotfix overlay", func(f *sgSelfFixture) { f.in.Expires = "" }},
		{"expired", "hotfix expired", func(f *sgSelfFixture) { f.in.Now = mustSgTime(t, "2026-10-29T00:00:00Z") }},
		{"over the ceiling", "hotfix exceeds the ruled ceiling", func(f *sgSelfFixture) { f.in.BuiltAt = "2026-10-01T00:00:00Z" }},
		{"statusgen pin moved, desk-tools pin at the base", "hotfix outlived its pin", func(f *sgSelfFixture) {
			sgFixPins(t, f.repo, "desk-tools "+sgFixBase+" "+strings.Repeat("0", 64)+"\nstatusgen v1.0.42 "+strings.Repeat("1", 64)+"\n")
		}},
		{"platform line moved", "hotfix outlived its pin", func(f *sgSelfFixture) {
			sgFixPins(t, f.repo, "statusgen-linux-amd64 v1.0.42 "+strings.Repeat("1", 64)+"\n")
		}},
		{"only a desk-tools line, no pinned install", "hotfix could-not-check: no pin resolves", func(f *sgSelfFixture) {
			sgFixPins(t, f.repo, "desk-tools "+sgFixBase+" "+strings.Repeat("0", 64)+"\n")
		}},
		{"outside any checkout, no pinned install", "hotfix could-not-check: no pin resolves", func(f *sgSelfFixture) {
			f.in.Cwd = filepath.Dir(f.overlay)
		}},
		{"one byte changed", "hotfix tampered", func(f *sgSelfFixture) {
			sgFixWrite(t, f.exe, "hotfix build of statusgeN\n", 0o755)
		}},
		{"stamp differs from the entry", "hotfix tampered", func(f *sgSelfFixture) { f.in.Tag = "v1.0.41+hotfix-ffffffffffff" }},
		{"no owner check", "hotfix overlay not root-owned", func(f *sgSelfFixture) { f.in.Owner = nil }},
		{"overlay owned by another uid", "hotfix overlay not root-owned", func(f *sgSelfFixture) {
			f.in.Owner = sgHotfixOwnedBy(uint32(os.Getuid()) + 1)
		}},
		{"source rule not the ruled one", "hotfix source rule not the ruled one", func(f *sgSelfFixture) {
			p := filepath.Join(f.overlay, sgHotfixManifestName)
			b, _ := os.ReadFile(p)
			sgFixWrite(t, p, strings.Replace(string(b), sgHotfixSourceRule, "approved-pr", 1), 0o644)
		}},
		{"stamped copy outside the overlay, no root-owned manifest there", "hotfix overlay not root-owned", func(f *sgSelfFixture) {
			other := filepath.Join(filepath.Dir(f.overlay), "elsewhere")
			cp := filepath.Join(other, "bin", "statusgen")
			b, _ := os.ReadFile(f.exe)
			sgFixWrite(t, cp, string(b), 0o755)
			f.in.Exe, f.in.Overlay = cp, other
		}},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			f := newSgSelfFixture(t)
			c.edit(f)
			v := sgEvaluateHotfix(f.in)
			if v.Skip || !strings.Contains(v.Reason, c.reason) || v.Fallback != "" {
				t.Fatalf("verdict = %+v, want a refusal naming %q and no fallback", v, c.reason)
			}
			f.in.Fallback = filepath.Join(f.pinned, "statusgen")
			if v := sgEvaluateHotfix(f.in); v.Fallback != f.in.Fallback {
				t.Fatalf("with the pinned fallback set: %+v", v)
			}
		})
	}
	t.Run("outside a pinned checkout is could-not-check", func(t *testing.T) {
		f := newSgSelfFixture(t)
		f.in.Cwd = filepath.Dir(f.overlay)
		// a working pinned install is present, and is still never read as the pin
		sgFixWrite(t, filepath.Join(f.pinned, "statusgen"), "#!/bin/sh\necho "+sgFixBase+"\n", 0o755)
		if v := sgEvaluateHotfix(f.in); v.Skip || !strings.Contains(v.Reason, "no pin resolves") || v.Fallback != "" {
			t.Fatalf("no statusgen pin line above the cwd: %+v", v)
		}
	})
	t.Run("fallback outside the pinned dir is refused", func(t *testing.T) {
		f := newSgSelfFixture(t)
		f.in.Now = mustSgTime(t, "2026-10-29T00:00:00Z")
		f.in.Fallback = f.exe
		v := sgEvaluateHotfix(f.in)
		if v.Fallback != "" || !strings.Contains(v.Reason, "is not under the pinned dir") {
			t.Fatalf("an overlay fallback: %+v", v)
		}
	})
	t.Run("staging file", func(t *testing.T) {
		f := newSgSelfFixture(t)
		staged := filepath.Join(f.overlay, "bin", ".statusgen.staging-123")
		sgFixWrite(t, staged, "staged\n", 0o755)
		f.in.Exe = staged
		if v := sgEvaluateHotfix(f.in); v.Reason != "" {
			t.Fatalf("the install's staged probe is refused: %+v", v)
		}
		f.in.Now = mustSgTime(t, "2026-10-29T00:00:00Z")
		if v := sgEvaluateHotfix(f.in); !strings.Contains(v.Reason, "hotfix expired") {
			t.Fatalf("an expired staged file: %+v", v)
		}
	})
	t.Run("verdict acts", func(t *testing.T) {
		var errb bytes.Buffer
		code := -1
		sgApplyHotfixVerdict(sgHotfixVerdict{Reason: "hotfix expired 2026-10-28T00:00:00Z"}, "v1.0.41+hotfix-0123456789ab", &errb,
			func(string, []string, []string) int { t.Fatal("exec with no fallback"); return 0 }, func(c int) { code = c })
		if code != 6 || !strings.Contains(errb.String(), "statusgen: refusing to run: hotfix expired") {
			t.Fatalf("no-fallback refusal: code %d, %q", code, errb.String())
		}
		errb.Reset()
		ran := ""
		sgApplyHotfixVerdict(sgHotfixVerdict{Reason: "hotfix expired", Fallback: "/pinned/statusgen"}, "v1.0.41+hotfix-0123456789ab", &errb,
			func(p string, _ []string, env []string) int {
				ran = p
				for _, kv := range env {
					if strings.HasPrefix(kv, sgHotfixFallbackEnv+"=") {
						t.Errorf("fallback env still carries %s", kv)
					}
				}
				return 0
			}, func(c int) { code = c })
		if ran != "/pinned/statusgen" || code != 0 || !strings.Contains(errb.String(), "statusgen NOTICE: hotfix v1.0.41+hotfix-0123456789ab refused (hotfix expired); running pinned /pinned/statusgen") {
			t.Fatalf("fallback: ran %q code %d %q", ran, code, errb.String())
		}
	})
}

// regression: #4405
// TestStatusgenHotfixParity: statusgen is its own module and cannot import deskkit, so its
// copy of the overlay's compiled values must equal deskkit's literally: the overlay and
// pinned-dir defaults, the env names, the manifest schema, the ruled source rule and ceiling,
// and every refusal phrase. A change to either side without the other fails here.
func TestStatusgenHotfixParity(t *testing.T) {
	dk := filepath.Join("..", "tools", "desk", "internal", "deskkit")
	var src string
	for _, f := range []string{"hotfix.go", "version.go", "hotfixseam.go"} {
		b, err := os.ReadFile(filepath.Join(dk, f))
		if err != nil {
			t.Fatal(err)
		}
		src += string(b)
	}
	strs := map[string]string{
		"HotfixOverlayEnv": sgHotfixOverlayEnv, "DefaultHotfixOverlay": sgDefaultHotfixOverlay,
		"DeskToolsBinEnv": sgDeskToolsBinEnv, "DefaultDeskToolsBin": sgDefaultDeskToolsBin,
		"HotfixFallbackEnv": sgHotfixFallbackEnv, "HotfixManifestName": sgHotfixManifestName,
		"HotfixManifestSchema": sgHotfixManifestSchema, "HotfixSourceRule": sgHotfixSourceRule,
		"HotfixSeamRootUID":        sgHotfixSeamRootUID,
		"HotfixReasonNotRootOwned": sgReasonNotRootOwned, "HotfixReasonTampered": sgReasonTampered,
		"HotfixReasonOutlived": sgReasonOutlived, "HotfixReasonExpired": sgReasonExpired,
		"HotfixReasonCeiling": sgReasonCeiling, "HotfixReasonSourceRule": sgReasonSourceRule,
		"HotfixReasonUnstamped": sgReasonUnstamped, "HotfixReasonNoPin": sgReasonNoPin,
	}
	for name, mine := range strs {
		m := regexp.MustCompile(`(?m)^\s*` + name + `\s*=\s*"([^"]*)"`).FindStringSubmatch(src)
		if m == nil {
			t.Errorf("deskkit has no string constant %s", name)
			continue
		}
		if m[1] != mine {
			t.Errorf("%s: deskkit %q, statusgen %q", name, m[1], mine)
		}
	}
	m := regexp.MustCompile(`(?m)^\s*HotfixMaxTTL\s*=\s*(\d+)\s*\*\s*24\s*\*\s*time\.Hour`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("deskkit has no HotfixMaxTTL = <n> * 24 * time.Hour")
	}
	if n, err := strconv.Atoi(m[1]); err != nil || time.Duration(n)*24*time.Hour != sgHotfixMaxTTL {
		t.Errorf("HotfixMaxTTL: deskkit %s days, statusgen %s", m[1], sgHotfixMaxTTL)
	}
}
