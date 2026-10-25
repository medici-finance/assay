package deskkit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/hotfixreaders"
)

// regression: #4405
// TestHotfixStamp: the hotfix tag grammar is exact — `vX.Y.Z+hotfix-` + 12 lowercase hex —
// and nothing else is a hotfix; a release build carries no `+` and no HotfixExpires.
func TestHotfixStamp(t *testing.T) {
	t.Run("hotfix", func(t *testing.T) {
		tag := "v1.0.41+hotfix-0123456789ab"
		if !IsHotfix(tag) {
			t.Fatalf("IsHotfix(%q) = false", tag)
		}
		if base, ok := HotfixBase(tag); !ok || base != "v1.0.41" {
			t.Fatalf("HotfixBase(%q) = %q, %v; want v1.0.41", tag, base, ok)
		}
		if sha := HotfixSHA(tag); sha != "0123456789ab" {
			t.Fatalf("HotfixSHA(%q) = %q", tag, sha)
		}
		if got := HotfixTag("v1.0.41", "0123456789abcdef0123456789abcdef01234567"); got != tag {
			t.Fatalf("HotfixTag = %q, want %q", got, tag)
		}
	})
	t.Run("not-hotfix", func(t *testing.T) {
		for _, tag := range []string{"v1.0.41", "dev", "", "v1.0.41+hotfix-xyz", "v1.0.41+build.7",
			"v1.0.41+hotfix-0123456789AB", "v1.0.41+hotfix-0123456789abc", "desk-tools/v1.0.41+hotfix-0123456789ab"} {
			if IsHotfix(tag) {
				t.Errorf("IsHotfix(%q) = true", tag)
			}
			if _, ok := HotfixBase(tag); ok {
				t.Errorf("HotfixBase(%q) ok", tag)
			}
			if HotfixSHA(tag) != "" {
				t.Errorf("HotfixSHA(%q) non-empty", tag)
			}
		}
	})
	t.Run("never-a-pin", func(t *testing.T) {
		for _, tag := range []string{"v1.0.41+hotfix-0123456789ab", "v1.0.41+build.7"} {
			if artifactTagPattern.MatchString(tag) {
				t.Errorf("pin grammar accepts %q", tag)
			}
		}
	})
	t.Run("release-build-unchanged", func(t *testing.T) {
		if HotfixExpires != "" {
			t.Fatalf("an unstamped build carries HotfixExpires %q", HotfixExpires)
		}
		if strings.Contains(ReleaseTagOrDev(), "+") {
			t.Fatalf("an unstamped build's tag %q carries a +", ReleaseTagOrDev())
		}
	})
	t.Run("ruled-constants", func(t *testing.T) {
		if HotfixSourceRule != "merged-main" || HotfixMaxTTL != 7*24*time.Hour ||
			HotfixDefaultTTL != 3*24*time.Hour || !HotfixRequireSudo {
			t.Fatalf("ruled constants moved: %q %s %s %v", HotfixSourceRule, HotfixMaxTTL, HotfixDefaultTTL, HotfixRequireSudo)
		}
	})
}

// regression: #4405
// TestDeskkitHotfixReaders: deskkit's tag classification reaches the shared matrix's verdict.
func TestDeskkitHotfixReaders(t *testing.T) {
	m, err := hotfixreaders.Load("deskkit")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.Cases {
		got := "neither"
		switch {
		case IsHotfix(c.Tag):
			got = "hotfix"
		case artifactTagPattern.MatchString(c.Tag):
			got = "pin"
		}
		if want := c.Verdicts["deskkit"]; got != want {
			t.Errorf("deskkit reads %q as %s, matrix says %s", c.Tag, got, want)
		}
	}
}

func TestHotfixIneligibleMatch(t *testing.T) {
	for p, want := range map[string]string{
		"migrations/0003-x.md":                     "migrations/",
		"schemas/brief-v2.json":                    "schemas/",
		"tools/desk/internal/deskkit/rosteracl.go": "tools/desk/internal/deskkit/rosteracl.go",
		"tools/desk/internal/deskkit/version.go":   "tools/desk/internal/deskkit/version.go",
		"tools/desk/cmd/deskpr/main.go":            "",
		"migrationsX/a":                            "",
	} {
		if got := HotfixIneligibleMatch(p); got != want {
			t.Errorf("HotfixIneligibleMatch(%q) = %q, want %q", p, got, want)
		}
	}
	l := HotfixIneligiblePaths()
	l[0] = "mutated"
	if HotfixIneligiblePaths()[0] == "mutated" {
		t.Fatal("HotfixIneligiblePaths returns the compiled slice, not a copy")
	}
}

// selfFixture is a temp layout: an overlay dir, a pinned dir, a checkout pinning pin.
type selfFixture struct{ overlay, pinned, checkout, outside string }

func newSelfFixture(t *testing.T, pin string) selfFixture {
	t.Helper()
	d := t.TempDir()
	f := selfFixture{
		overlay: filepath.Join(d, "hotfix"), pinned: filepath.Join(d, "bin"),
		checkout: filepath.Join(d, "repo"), outside: filepath.Join(d, "elsewhere"),
	}
	for _, p := range []string{filepath.Join(f.overlay, "bin"), f.pinned, f.checkout, f.outside} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	hfWriteFile(t, filepath.Join(f.checkout, AssayVersionsFile), "desk-tools "+pin+" "+strings.Repeat("a", 64)+"\n")
	hfWriteFile(t, filepath.Join(f.pinned, "deskpr"), "#!/bin/sh\n")
	hfWriteFile(t, filepath.Join(f.overlay, "bin", "deskpr"), "#!/bin/sh\n")
	return f
}

func hfWriteFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// regression: #4405
// TestHotfixSelfCheck: layer 2 alone, with the shim (layer 1) and the boot (layer 3) bypassed
// — the binary itself refuses an expired or pin-moved stamp, falling back to the pinned binary
// when its shim named one and exiting 6 when not; it never proceeds on its expiry alone.
func TestHotfixSelfCheck(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	built := now.Add(-24 * time.Hour).Format(time.RFC3339)
	live := now.Add(48 * time.Hour).Format(time.RFC3339)
	f := newSelfFixture(t, "v1.0.41")
	// The manifest entry the running file is checked against: its own digest and stamp.
	exe := filepath.Join(f.overlay, "bin", "deskpr")
	for _, p := range []string{f.overlay, filepath.Join(f.overlay, "bin"), exe} {
		_ = os.Chmod(p, 0o755)
	}
	sum, err := FileSHA256(exe)
	if err != nil {
		t.Fatal(err)
	}
	entry := HotfixEntry{Tool: "deskpr", SHA256: sum, SourceSHA: "0123456789ab" + strings.Repeat("0", 28),
		BasePin: "v1.0.41", BuiltAt: built, Expires: live, SourceRule: HotfixSourceRule}
	hfWriteManifest(t, f.overlay, entry)
	owner := HotfixOwnedBy(uint32(os.Getuid()))
	base := func() hotfixSelfInput {
		return hotfixSelfInput{
			Tag: "v1.0.41+hotfix-0123456789ab", Expires: live, BuiltAt: built,
			Exe: exe, Overlay: f.overlay, PinnedBin: f.pinned, Owner: owner,
			Cwd: f.checkout, Now: now, Platform: "linux-amd64",
			PinnedVersion: func(string) (string, error) { return "", os.ErrNotExist },
		}
	}
	fallback := filepath.Join(f.pinned, "deskpr")

	t.Run("valid-proceeds", func(t *testing.T) {
		v := evaluateHotfixSelf(base())
		if v.Skip || v.Reason != "" {
			t.Fatalf("valid hotfix: %+v", v)
		}
	})
	t.Run("expired-execs-fallback", func(t *testing.T) {
		in := base()
		in.Now = now.Add(72 * time.Hour)
		in.Fallback = fallback
		v := evaluateHotfixSelf(in)
		if v.Fallback != fallback || !strings.Contains(v.Reason, HotfixReasonExpired) {
			t.Fatalf("expired with fallback: %+v", v)
		}
		var stderr bytes.Buffer
		t.Setenv(HotfixFallbackEnv, fallback)
		var execPath string
		var execEnv []string
		code := -1
		applyHotfixSelfVerdict(v, in.Tag, &stderr, func(p string, _, env []string) int {
			execPath, execEnv = p, env
			return 0
		}, func(c int) { code = c })
		if execPath != fallback || code != 0 {
			t.Fatalf("exec %q code %d, want %q 0", execPath, code, fallback)
		}
		for _, kv := range execEnv {
			if strings.HasPrefix(kv, HotfixFallbackEnv+"=") {
				t.Fatalf("fallback exec keeps %s", kv)
			}
		}
		if !strings.Contains(stderr.String(), "NOTICE") || !strings.Contains(stderr.String(), "expired") {
			t.Fatalf("NOTICE missing: %q", stderr.String())
		}
	})
	t.Run("pin-moved-execs-fallback", func(t *testing.T) {
		g := newSelfFixture(t, "v1.0.42")
		in := base()
		in.Cwd = g.checkout
		in.Fallback = fallback
		v := evaluateHotfixSelf(in)
		if v.Fallback != fallback || !strings.Contains(v.Reason, HotfixReasonOutlived) {
			t.Fatalf("pin moved: %+v", v)
		}
	})
	t.Run("no-fallback-exits-6", func(t *testing.T) {
		in := base()
		in.Now = now.Add(72 * time.Hour)
		v := evaluateHotfixSelf(in)
		var stderr bytes.Buffer
		code := -1
		applyHotfixSelfVerdict(v, in.Tag, &stderr, func(string, []string, []string) int {
			t.Fatal("exec without a fallback")
			return 0
		}, func(c int) { code = c })
		if code != ExitUnverifiable || !strings.Contains(stderr.String(), HotfixReasonExpired) {
			t.Fatalf("code %d stderr %q", code, stderr.String())
		}
	})
	t.Run("fallback-outside-pinned-dir-refused", func(t *testing.T) {
		in := base()
		in.Now = now.Add(72 * time.Hour)
		in.Fallback = filepath.Join(f.outside, "deskpr")
		v := evaluateHotfixSelf(in)
		if v.Fallback != "" || v.Reason == "" {
			t.Fatalf("fallback outside the pinned dir accepted: %+v", v)
		}
	})
	t.Run("outside-checkout-no-pinned-install-could-not-check", func(t *testing.T) {
		in := base()
		in.Cwd = f.outside
		v := evaluateHotfixSelf(in)
		if v.Reason == "" || !strings.Contains(v.Reason, "could-not-check") {
			t.Fatalf("no pin resolves, yet: %+v", v)
		}
	})
	t.Run("outside-checkout-pinned-install-answers", func(t *testing.T) {
		in := base()
		in.Cwd = f.outside
		in.PinnedVersion = func(string) (string, error) { return "v1.0.41", nil }
		if v := evaluateHotfixSelf(in); v.Reason != "" {
			t.Fatalf("pinned install names the pin, yet: %+v", v)
		}
	})
	t.Run("bare-tag-in-overlay-refused", func(t *testing.T) {
		in := base()
		in.Tag, in.Expires = "v1.0.41", ""
		v := evaluateHotfixSelf(in)
		if !strings.Contains(v.Reason, HotfixReasonUnstamped) {
			t.Fatalf("bare tag in overlay: %+v", v)
		}
	})
	t.Run("ceiling", func(t *testing.T) {
		in := base()
		in.BuiltAt = now.Add(-5 * 24 * time.Hour).Format(time.RFC3339)
		in.Expires = now.Add(5 * 24 * time.Hour).Format(time.RFC3339)
		if v := evaluateHotfixSelf(in); !strings.Contains(v.Reason, HotfixReasonCeiling) {
			t.Fatalf("10-day lifetime: %+v", v)
		}
	})
	// Layer 2 on its own: ValidateHotfix, ChooseHotfixShim and the boot are not called, and
	// each fault is one the upper layers also catch — this layer refuses it by its own check.
	t.Run("own-integrity-checks", func(t *testing.T) {
		for name, c := range map[string]struct {
			mutate func(t *testing.T, g selfFixture, in *hotfixSelfInput)
			reason string
		}{
			"wrong-owner": {func(_ *testing.T, _ selfFixture, in *hotfixSelfInput) {
				in.Owner = HotfixOwnedBy(uint32(os.Getuid()) + 1)
			}, HotfixReasonNotRootOwned},
			"no-owner-check": {func(_ *testing.T, _ selfFixture, in *hotfixSelfInput) { in.Owner = nil }, HotfixReasonNotRootOwned},
			"writable-by-group": {func(t *testing.T, _ selfFixture, in *hotfixSelfInput) {
				if err := os.Chmod(in.Exe, 0o775); err != nil {
					t.Fatal(err)
				}
			}, HotfixReasonNotRootOwned},
			"wrong-sha256": {func(t *testing.T, _ selfFixture, in *hotfixSelfInput) {
				hfWriteFile(t, in.Exe, "#!/bin/sh\necho swapped\n")
			}, HotfixReasonTampered},
			"no-manifest-entry": {func(t *testing.T, g selfFixture, _ *hotfixSelfInput) {
				e := entry
				e.Tool = "deskboard"
				hfWriteManifest(t, g.overlay, e)
			}, HotfixReasonTampered},
			"manifest-missing": {func(t *testing.T, g selfFixture, _ *hotfixSelfInput) {
				if err := os.Remove(HotfixManifestPath(g.overlay)); err != nil {
					t.Fatal(err)
				}
			}, HotfixReasonNotRootOwned},
			"stamp-not-the-manifest-one": {func(_ *testing.T, _ selfFixture, in *hotfixSelfInput) {
				in.Tag = "v1.0.41+hotfix-ffffffffffff"
			}, HotfixReasonTampered},
			"source-rule": {func(t *testing.T, g selfFixture, _ *hotfixSelfInput) {
				e := entry
				e.SourceRule = "approved-pr"
				hfWriteManifest(t, g.overlay, e)
			}, HotfixReasonSourceRule},
		} {
			t.Run(name, func(t *testing.T) {
				g := newSelfFixture(t, "v1.0.41")
				gexe := filepath.Join(g.overlay, "bin", "deskpr")
				for _, p := range []string{g.overlay, filepath.Join(g.overlay, "bin"), gexe} {
					_ = os.Chmod(p, 0o755)
				}
				gsum, _ := FileSHA256(gexe)
				ge := entry
				ge.SHA256 = gsum
				hfWriteManifest(t, g.overlay, ge)
				in := base()
				in.Exe, in.Overlay, in.PinnedBin, in.Cwd = gexe, g.overlay, g.pinned, g.checkout
				in.Fallback = filepath.Join(g.pinned, "deskpr")
				if v := evaluateHotfixSelf(in); v.Reason != "" {
					t.Fatalf("control: the unmutated fixture is refused: %+v", v)
				}
				c.mutate(t, g, &in)
				v := evaluateHotfixSelf(in)
				if !strings.Contains(v.Reason, c.reason) {
					t.Fatalf("reason %q, want %q", v.Reason, c.reason)
				}
				if v.Fallback != in.Fallback {
					t.Fatalf("fallback %q, want the pinned %q", v.Fallback, in.Fallback)
				}
			})
		}
	})
	// The install's own probe: its staged file runs --version before any manifest entry
	// exists, so that exact name is checked for expiry, ceiling and pin only.
	t.Run("install-staging-file-exempt-only-by-exact-name", func(t *testing.T) {
		g := newSelfFixture(t, "v1.0.41")
		stage := filepath.Join(g.overlay, "bin", ".deskpr.staging-123456")
		hfWriteFile(t, stage, "#!/bin/sh\n")
		in := base()
		in.Exe, in.Overlay, in.PinnedBin, in.Cwd = stage, g.overlay, g.pinned, g.checkout
		in.Owner = HotfixOwnedBy(uint32(os.Getuid()) + 1)
		if v := evaluateHotfixSelf(in); v.Reason != "" {
			t.Fatalf("staged install probe refused: %+v", v)
		}
		in.Now = now.Add(72 * time.Hour)
		if v := evaluateHotfixSelf(in); !strings.Contains(v.Reason, HotfixReasonExpired) {
			t.Fatalf("staged probe skips expiry too: %+v", v)
		}
		in.Now = now
		for _, name := range []string{"deskpr.staging-123456", ".deskpr.staging-x", ".deskpr.staging-1.sh", "deskpr"} {
			other := filepath.Join(g.overlay, "bin", name)
			hfWriteFile(t, other, "#!/bin/sh\n")
			in.Exe = other
			if v := evaluateHotfixSelf(in); !strings.Contains(v.Reason, HotfixReasonNotRootOwned) {
				t.Fatalf("%s exempt from the integrity check: %+v", name, v)
			}
		}
		nested := filepath.Join(g.overlay, "bin", "sub", ".deskpr.staging-1")
		if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
			t.Fatal(err)
		}
		hfWriteFile(t, nested, "#!/bin/sh\n")
		in.Exe = nested
		if v := evaluateHotfixSelf(in); !strings.Contains(v.Reason, HotfixReasonNotRootOwned) {
			t.Fatalf("nested staging name exempt: %+v", v)
		}
	})
	t.Run("release-build-outside-overlay-skipped", func(t *testing.T) {
		in := base()
		in.Tag, in.Expires = "v1.0.41", ""
		in.Exe = filepath.Join(f.pinned, "deskpr")
		in.Cwd = f.outside
		if v := evaluateHotfixSelf(in); !v.Skip {
			t.Fatalf("release build outside the overlay checked: %+v", v)
		}
	})
}

func hfWriteManifest(t *testing.T, overlay string, e HotfixEntry) {
	t.Helper()
	if err := WriteHotfixManifest(overlay, &HotfixManifest{Entries: []HotfixEntry{e}}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateHotfix(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	owner := HotfixOwnedBy(uint32(os.Getuid()))
	mk := func(t *testing.T) (string, HotfixEntry) {
		f := newSelfFixture(t, "v1.0.41")
		sum, _ := FileSHA256(HotfixBinaryPath(f.overlay, "deskpr"))
		for _, p := range []string{f.overlay, filepath.Join(f.overlay, "bin"), HotfixBinaryPath(f.overlay, "deskpr")} {
			_ = os.Chmod(p, 0o755)
		}
		e := HotfixEntry{Tool: "deskpr", SHA256: sum, SourceSHA: strings.Repeat("0123456789", 4),
			BasePin: "v1.0.41", BuiltAt: now.Add(-time.Hour).Format(time.RFC3339),
			Expires: now.Add(47 * time.Hour).Format(time.RFC3339), SourceRule: HotfixSourceRule}
		hfWriteManifest(t, f.overlay, e)
		return f.overlay, e
	}
	t.Run("valid", func(t *testing.T) {
		o, _ := mk(t)
		if _, err := ValidateHotfix(o, "deskpr", "v1.0.41", now, owner); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("none", func(t *testing.T) {
		o, _ := mk(t)
		if _, err := ValidateHotfix(o, "deskboard", "v1.0.41", now, owner); err != ErrNoHotfix {
			t.Fatalf("err = %v, want ErrNoHotfix", err)
		}
	})
	for name, c := range map[string]struct {
		mutate func(o string, e *HotfixEntry)
		pin    string
		reason string
	}{
		"tampered":    {func(o string, e *HotfixEntry) { _ = os.WriteFile(HotfixBinaryPath(o, "deskpr"), []byte("x"), 0o755) }, "v1.0.41", HotfixReasonTampered},
		"outlived":    {func(string, *HotfixEntry) {}, "v1.0.42", HotfixReasonOutlived},
		"expired":     {func(_ string, e *HotfixEntry) { e.Expires = now.Add(-time.Minute).Format(time.RFC3339) }, "v1.0.41", HotfixReasonExpired},
		"ceiling":     {func(_ string, e *HotfixEntry) { e.BuiltAt = now.Add(-8 * 24 * time.Hour).Format(time.RFC3339) }, "v1.0.41", HotfixReasonCeiling},
		"source-rule": {func(_ string, e *HotfixEntry) { e.SourceRule = "approved-pr" }, "v1.0.41", HotfixReasonSourceRule},
	} {
		t.Run(name, func(t *testing.T) {
			o, e := mk(t)
			c.mutate(o, &e)
			hfWriteManifest(t, o, e)
			_, err := ValidateHotfix(o, "deskpr", c.pin, now, owner)
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("err = %v, want %q", err, c.reason)
			}
		})
	}
	t.Run("not-root-owned", func(t *testing.T) {
		o, _ := mk(t)
		_, err := ValidateHotfix(o, "deskpr", "v1.0.41", now, HotfixRootOwned())
		if os.Getuid() == 0 {
			t.Skip("running as root")
		}
		if err == nil || !strings.Contains(err.Error(), HotfixReasonNotRootOwned) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestHotfixForge(t *testing.T) {
	status := map[string]string{}
	files := map[string][]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/repos/medici-finance/assay/compare/")
		st, ok := status[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var fl []map[string]string
		for _, f := range files[key] {
			fl = append(fl, map[string]string{"filename": f})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": st, "files": fl})
	}))
	defer srv.Close()
	f := HotfixForge{BaseURL: srv.URL, Repo: HotfixRepoSlug}
	sha := strings.Repeat("ab", 20)
	status["main..."+sha] = "behind"
	if err := f.CheckSourceOnMain(sha); err != nil {
		t.Fatalf("behind: %v", err)
	}
	status["main..."+sha] = "diverged"
	if err := f.CheckSourceOnMain(sha); HotfixCodeOf(err) != 1 || !strings.Contains(err.Error(), HotfixSourceRule) {
		t.Fatalf("diverged: %v", err)
	}
	if err := f.CheckSourceOnMain(strings.Repeat("cd", 20)); HotfixCodeOf(err) != 1 {
		t.Fatalf("unknown sha: %v", err)
	}
	status["v1.0.41..."+sha] = "ahead"
	files["v1.0.41..."+sha] = []string{"tools/desk/cmd/deskpr/main.go"}
	if err := f.CheckEligible("v1.0.41", sha); err != nil {
		t.Fatalf("eligible: %v", err)
	}
	files["v1.0.41..."+sha] = []string{"tools/desk/cmd/deskpr/main.go", "migrations/0009-x.md"}
	if err := f.CheckEligible("v1.0.41", sha); HotfixCodeOf(err) != 1 || !strings.Contains(err.Error(), "hotfix-ineligible") || !strings.Contains(err.Error(), "migrations/0009-x.md") {
		t.Fatalf("ineligible: %v", err)
	}
	dead := HotfixForge{BaseURL: "http://127.0.0.1:1", Repo: HotfixRepoSlug}
	if err := dead.CheckSourceOnMain(sha); HotfixCodeOf(err) != 6 {
		t.Fatalf("unreachable: %v", err)
	}
}
