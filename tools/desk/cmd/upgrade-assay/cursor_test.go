package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cursorbundle"
)

func cursorRelease(t *testing.T, version string, skill ...string) []byte {
	t.Helper()
	bundle := t.TempDir()
	files := map[string]string{
		"cursor/packaging.md":        "<!-- assay:cursor-packaging\nadopt\n-->\n",
		"cursor/assay.mdc":           "---\nalwaysApply: true\n---\nfixture rule\n",
		"skills/adopt/SKILL.md":      "---\nname: adopt\n---\nselected release skill\n",
		"references/desk-shell.md":   "fixture reference\n",
		"codex/AGENTS-assay.md":      "fixture Assay bindings\n",
		".claude-plugin/plugin.json": `{"version":"` + strings.TrimPrefix(version, "v") + `"}`,
	}
	if len(skill) > 0 {
		files["skills/adopt/SKILL.md"] = skill[0]
	}
	for name, body := range files {
		p := filepath.Join(bundle, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := cursorbundle.Pack(bundle, version, &out); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func materializeCursor(t *testing.T, root string, raw []byte) {
	t.Helper()
	dir := filepath.Join(root, "releases")
	if err := os.WriteFile(filepath.Join(dir, "v0.13.0."+cursorbundle.AssetName), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	checksums := checksumsFor('2') + fmt.Sprintf("%x  %s\n", sha256.Sum256(raw), cursorbundle.AssetName)
	if err := os.WriteFile(filepath.Join(dir, "v0.13.0.checksums.txt"), []byte(checksums), 0o644); err != nil {
		t.Fatal(err)
	}
}

// regression: #2701
func TestCursorUpgrade(t *testing.T) {
	root := writeDerivedFixture(t)
	materializeCursor(t, root, cursorRelease(t, "v0.13.0"))
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("local instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"--root", root, "--to", "v0.13.0", "--harness", "cursor", "--forge", "gitlab"}
	before := snapshot(t, root)
	if code, out := run2(t, append(args, "--dry-run")...); code != 0 || !strings.Contains(out, "would write .cursor/skills/adopt/SKILL.md") || strings.Contains(out, "/plugin marketplace") {
		t.Fatalf("preview exit %d: %s", code, out)
	}
	if snapshot(t, root) != before {
		t.Fatal("preview changed adopter")
	}
	if code, out := run2(t, args...); code != 0 {
		t.Fatalf("apply exit %d: %s", code, out)
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil || !strings.HasPrefix(string(agents), "local instructions\n") || !strings.Contains(string(agents), "`glab`") {
		t.Fatalf("bindings: %s %v", agents, err)
	}
	if code, out := run2(t, append(args, "--check")...); code != 0 {
		t.Fatalf("selected-version check exit %d: %s", code, out)
	}
	applied := snapshot(t, root)
	if code, out := run2(t, args...); code != 0 {
		t.Fatalf("idempotent apply exit %d: %s", code, out)
	}
	if snapshot(t, root) != applied {
		t.Fatal("identical apply changed adopter")
	}
	skill := filepath.Join(root, ".cursor", "skills", "adopt", "SKILL.md")
	if err = os.WriteFile(skill, []byte("locally edited skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before = snapshot(t, root)
	if code, out := run2(t, append(args, "--check")...); code != 1 || !strings.Contains(out, "content-differs") {
		t.Fatalf("same-version drift bypassed: %d %s", code, out)
	}
	if code, out := run2(t, args...); code == 0 {
		t.Fatalf("local edits silently overwritten: %s", out)
	}
	if snapshot(t, root) != before {
		t.Fatal("refused apply changed adopter")
	}
}

// regression: #2701
func TestCursorUpgradeRefusals(t *testing.T) {
	for _, kind := range []string{"legacy", "missing", "tampered", "substituted", "stale", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			root := writeDerivedFixture(t)
			raw := cursorRelease(t, "v0.13.0")
			if kind == "stale" {
				raw = cursorRelease(t, "v0.12.0")
			}
			if kind != "legacy" {
				materializeCursor(t, root, raw)
			}
			asset := filepath.Join(root, "releases", "v0.13.0."+cursorbundle.AssetName)
			switch kind {
			case "missing":
				if err := os.Remove(asset); err != nil {
					t.Fatal(err)
				}
			case "substituted":
				if err := os.WriteFile(asset, cursorRelease(t, "v0.13.0", "substituted selected-version skill\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "tampered":
				if err := os.WriteFile(asset, append(raw, '!'), 0o644); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.Remove(asset); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(asset, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshot(t, root)
			for _, mode := range []string{"--dry-run", "--check", ""} {
				args := []string{"--root", root, "--to", "v0.13.0", "--harness", "cursor"}
				if mode != "" {
					args = append(args, mode)
				}
				if code, out := run2(t, args...); code != exitArtifactsGone {
					t.Fatalf("%s got %d: %s", mode, code, out)
				}
				if snapshot(t, root) != before {
					t.Fatal("refusal changed adopter")
				}
			}
		})
	}
}

// regression: #2701
func TestCursorFetchOptIn(t *testing.T) {
	root := writeDerivedFixture(t)
	raw := cursorRelease(t, "v0.13.0")
	materializeCursor(t, root, raw)
	if err := os.Remove(filepath.Join(root, "releases", "v0.13.0."+cursorbundle.AssetName)); err != nil {
		t.Fatal(err)
	}
	old := fetchFunc
	t.Cleanup(func() { fetchFunc = old })
	calls := 0
	var stdout, stderr bytes.Buffer
	url := "https://github.com/medici-finance/assay/releases/download/v0.13.0/" + cursorbundle.AssetName
	fetchFunc = func(got string) ([]byte, error) {
		calls++
		if got != url {
			t.Fatalf("wrong selected release URL %s", got)
		}
		if !strings.Contains(stderr.String(), "fetching "+url) {
			t.Fatal("fetch occurred before target announcement")
		}
		return raw, nil
	}
	args := []string{"--root", root, "--to", "v0.13.0", "--harness", "cursor", "--dry-run"}
	before := snapshot(t, root)
	if code := run(args, &stdout, &stderr); code != exitArtifactsGone || calls != 0 {
		t.Fatalf("offline request: exit %d calls %d", code, calls)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(append(args, "--fetch"), &stdout, &stderr); code != 0 || calls != 1 {
		t.Fatalf("explicit request: exit %d calls %d: %s", code, calls, stderr.String())
	}
	if snapshot(t, root) != before {
		t.Fatal("fetched preview changed adopter")
	}
}
