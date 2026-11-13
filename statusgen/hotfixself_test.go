//go:build unix

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
		{"running file owned by another uid, manifest root-owned", "hotfix overlay not root-owned", func(f *sgSelfFixture) {
			mine, other := sgHotfixOwnedBy(uint32(os.Getuid())), sgHotfixOwnedBy(uint32(os.Getuid())+1)
			f.in.Owner = func(p string) error {
				if p == f.exe {
					return other(p)
				}
				return mine(p)
			}
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

// TestStatusgenHotfixNetnsRoot: the check:ci network-off helper runs under `unshare --net
// --map-root-user`, where the caller's uid reads as 0 and an unmapped uid, root's included,
// reads as the overflow uid. sgNamespaceRootUID finds the uid root carries there; the helper,
// and only the helper, checks ownership by that uid, and never takes uid 0 there for root.
func TestStatusgenHotfixNetnsRoot(t *testing.T) {
	for _, c := range []struct {
		name, uidMap, overflow string
		want                   uint32
		ok                     bool
	}{
		{"initial namespace", "         0          0 4294967295\n", "65534\n", 0, true},
		{"map-root-user, root unmapped", "         0       1000          1\n", "65534\n", 65534, true},
		{"root mapped to another inside uid", "1000 0 1\n0 1000 1\n", "65534", 1000, true},
		{"root mapped in a subuid range", "0 0 1\n1 100000 65536\n", "65534", 0, true},
		{"overflow uid is itself mapped", "0 1000 70000\n", "65534", 0, false},
		{"empty map", "", "65534", 0, false},
		{"malformed map line", "0 1000\n", "65534", 0, false},
		{"zero-length extent", "0 1000 0\n", "65534", 0, false},
		{"malformed overflow", "0 1000 1\n", "nobody", 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, ok := sgNamespaceRootUID(c.uidMap, c.overflow)
			if got != c.want || ok != c.ok {
				t.Fatalf("sgNamespaceRootUID(%q, %q) = %d, %v; want %d, %v", c.uidMap, c.overflow, got, ok, c.want, c.ok)
			}
		})
	}

	me := uint32(os.Getuid())
	if me == 0 {
		t.Skip("running as root: the owner probes below need a non-root file owner")
	}
	f := filepath.Join(t.TempDir(), "statusgen")
	sgFixWrite(t, f, "x\n", 0o755)
	procFor := func(uidMap, overflow string) func(string) ([]byte, error) {
		return func(p string) ([]byte, error) {
			switch p {
			case sgProcUIDMap:
				return []byte(uidMap), nil
			case sgProcOverflowUID:
				return []byte(overflow), nil
			}
			return nil, os.ErrNotExist
		}
	}
	helper := []string{"statusgen", netnsHelperArg, "net:[1]", "sh", "-c", "true"}
	// A namespace in which root reads as this test's uid: the file's own owner check passes,
	// and the parent walk then meets a uid-0 directory, which in such a namespace is the
	// mapped caller and is refused.
	asRoot := procFor(fmt.Sprintf("0 %d 1\n", me+1), fmt.Sprint(me))
	notRoot := fmt.Sprintf("is owned by uid %d, not 0", me)
	err := sgActiveOwnerFor(helper, asRoot)(f)
	if err == nil || strings.Contains(err.Error(), notRoot) || !strings.Contains(err.Error(), "parent ") ||
		!strings.Contains(err.Error(), "owned by uid 0, not "+fmt.Sprint(me)) {
		t.Fatalf("helper in a namespace where root reads as uid %d: %v", me, err)
	}
	// The same file under root's uid 0 check: every other invocation, a helper whose mapping
	// cannot be read, and a helper whose mapping is ambiguous.
	for name, owner := range map[string]func(string) error{
		"ordinary invocation":   sgActiveOwnerFor([]string{"statusgen", "verifyrun"}, asRoot),
		"helper argv not first": sgActiveOwnerFor([]string{"statusgen", "verifyrun", netnsHelperArg}, asRoot),
		"helper, no uid_map":    sgActiveOwnerFor(helper, func(string) ([]byte, error) { return nil, os.ErrPermission }),
		"helper, initial ns":    sgActiveOwnerFor(helper, procFor("0 0 4294967295\n", fmt.Sprint(me))),
		"helper, ambiguous map": sgActiveOwnerFor(helper, procFor(fmt.Sprintf("0 %d 70000\n", me+1), "1")),
		"default argv":          sgActiveOwner(),
	} {
		if err := owner(f); err == nil || !strings.Contains(err.Error(), notRoot) {
			t.Errorf("%s: owner check on a uid-%d file = %v, want %q", name, me, err, notRoot)
		}
	}
	// In the --map-root-user namespace root reads as the overflow uid; a file owned by anyone
	// else, the caller included, is refused.
	if err := sgActiveOwnerFor(helper, procFor(fmt.Sprintf("0 %d 1\n", me), "65534"))(f); err == nil ||
		!strings.Contains(err.Error(), fmt.Sprintf("is owned by uid %d, not 65534", me)) {
		t.Fatalf("helper, root unmapped: %v", err)
	}
}

// TestStatusgenHotfixPinReader: sgHotfixPin reads a pin line the way deskkit's lookupPin and
// NearestPinRoot do, so layers 1 to 3 agree on it: the name must be followed by a literal
// space, and the nearest existing .assay-versions wins even when it cannot be read.
func TestStatusgenHotfixPinReader(t *testing.T) {
	sum := strings.Repeat("1", 64)
	t.Run("tab-separated line resolves nothing", func(t *testing.T) {
		d := t.TempDir()
		sgFixPins(t, d, "statusgen\t"+sgFixBase+" "+sum+"\n")
		if got := sgHotfixPin(d, "linux-amd64"); got != "" {
			t.Fatalf("tab-separated statusgen line = %q, want none", got)
		}
		sgFixPins(t, d, "statusgen\t"+sgFixBase+" "+sum+"\nstatusgen-linux-amd64 v1.0.40 "+sum+"\n")
		if got := sgHotfixPin(d, "linux-amd64"); got != "v1.0.40" {
			t.Fatalf("tab-separated bare line beside a platform line = %q, want v1.0.40", got)
		}
	})
	t.Run("an unreadable nearer pin file is not walked past", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("running as root: a mode-000 file stays readable")
		}
		outer := t.TempDir()
		inner := filepath.Join(outer, "repo")
		sgFixPins(t, outer, "statusgen "+sgFixBase+" "+sum+"\n")
		sgFixPins(t, inner, "statusgen "+sgFixBase+" "+sum+"\n")
		if got := sgHotfixPin(inner, "linux-amd64"); got != sgFixBase {
			t.Fatalf("readable nearer pin = %q", got)
		}
		p := filepath.Join(inner, ".assay-versions")
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
		if got := sgHotfixPin(inner, "linux-amd64"); got != "" {
			t.Fatalf("unreadable nearer pin file walked past to %q", got)
		}
	})
}
