package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// regression: #2701
func TestCursorRepoGenerated(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	findings, err := HarnessCursorCheck(HarnessOptions{BundleDir: filepath.Join(root, "plugins", "assay"), RepoRoot: root, Forge: "github"})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("generated checkout drift: %v", findings)
	}
}

// regression: #2701
func TestCursorOwnedRemoval(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt", "install"}, nil, nil, false, false)
	repo := t.TempDir()
	opts := HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}
	if err := HarnessCursorPlace(opts, io.Discard); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(repo, ".cursor", "skills", "local", "SKILL.md")
	mustWrite(t, local, "unmanaged local skill\n")
	mustWrite(t, filepath.Join(bundle, "cursor", "packaging.md"), cursorPackagingMarker+"\nadopt\n-->\n")
	if err := os.RemoveAll(filepath.Join(bundle, "skills", "install")); err != nil {
		t.Fatal(err)
	}
	findings, err := HarnessCursorCheck(opts)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Path, "/local/") {
			t.Fatalf("unmanaged content classified as generated: %v", f)
		}
		if strings.Contains(f.Path, "/install/") {
			found = true
		}
	}
	if !found {
		t.Fatal("removed packaged skill was not reported as stale")
	}
	if err := HarnessCursorPlace(opts, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".cursor", "skills", "install", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("stale generated skill survived: %v", err)
	}
	if raw, err := os.ReadFile(local); err != nil || string(raw) != "unmanaged local skill\n" {
		t.Fatalf("unmanaged skill changed: %q %v", raw, err)
	}
	findings, err = HarnessCursorCheck(opts)
	if err != nil || len(findings) != 0 {
		t.Fatalf("post-sync check: %v %v", findings, err)
	}
}

// regression: #2701
func TestCursorUnmanagedCollision(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	repo := t.TempDir()
	local := filepath.Join(repo, ".cursor", "skills", "adopt", "SKILL.md")
	mustWrite(t, local, "locally authored skill\n")
	before := treeSnapshot(t, repo)
	err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard)
	if err == nil {
		t.Fatal("unmanaged collision silently overwritten")
	}
	after := treeSnapshot(t, repo)
	if len(before) != len(after) || after[filepath.Join(".cursor", "skills", "adopt", "SKILL.md")] != "locally authored skill\n" {
		t.Fatal("refused sync wrote to destination")
	}
}

// regression: #2701
func TestCursorSourceDrift(t *testing.T) {
	for _, change := range []string{"changed", "added", "removed"} {
		t.Run(change, func(t *testing.T) {
			bundle := cursorFixtureBundle(t, []string{"adopt", "install"}, nil, nil, false, false)
			repo := t.TempDir()
			opts := HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}
			if err := HarnessCursorPlace(opts, io.Discard); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "changed":
				mustWrite(t, filepath.Join(bundle, "skills", "install", "SKILL.md"), "changed canonical skill\n")
			case "added":
				mustWrite(t, filepath.Join(bundle, "skills", "second", "SKILL.md"), "new canonical skill\n")
				mustWrite(t, filepath.Join(bundle, "cursor", "packaging.md"), cursorPackagingMarker+"\nadopt\ninstall\nsecond\n-->\n")
			case "removed":
				if err := os.RemoveAll(filepath.Join(bundle, "skills", "install")); err != nil {
					t.Fatal(err)
				}
				mustWrite(t, filepath.Join(bundle, "cursor", "packaging.md"), cursorPackagingMarker+"\nadopt\n-->\n")
			}
			before := treeSnapshot(t, repo)
			findings, err := HarnessCursorCheck(opts)
			if err != nil || len(findings) == 0 {
				t.Fatalf("canonical %s not detected: %v %v", change, findings, err)
			}
			if !reflect.DeepEqual(before, treeSnapshot(t, repo)) {
				t.Fatal("drift check mutated destination")
			}
			if err := HarnessCursorPlace(opts, io.Discard); err != nil {
				t.Fatal(err)
			}
			findings, err = HarnessCursorCheck(opts)
			if err != nil || len(findings) != 0 {
				t.Fatalf("generated result drifts: %v %v", findings, err)
			}
		})
	}
}

// regression: #2701
func TestCursorStaleEditRefuses(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt", "install"}, nil, nil, false, false)
	repo := t.TempDir()
	opts := HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}
	if err := HarnessCursorPlace(opts, io.Discard); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, ".cursor", "skills", "install", "SKILL.md"), "local changes\n")
	if err := os.RemoveAll(filepath.Join(bundle, "skills", "install")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(bundle, "cursor", "packaging.md"), cursorPackagingMarker+"\nadopt\n-->\n")
	before := treeSnapshot(t, repo)
	if err := HarnessCursorPlace(opts, io.Discard); err == nil {
		t.Fatal("modified stale generated content was retired")
	}
	if !reflect.DeepEqual(before, treeSnapshot(t, repo)) {
		t.Fatal("refusal changed destination")
	}
}

// regression: #2701
func TestCursorPathAndBindings(t *testing.T) {
	for _, kind := range []string{"symlink", "bindings"} {
		t.Run(kind, func(t *testing.T) {
			bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
			repo := t.TempDir()
			if kind == "symlink" {
				outside := t.TempDir()
				mustWrite(t, filepath.Join(outside, "sentinel"), "untouched\n")
				if err := os.Symlink(outside, filepath.Join(repo, ".cursor")); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				before := treeSnapshot(t, outside)
				if err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard); err == nil {
					t.Fatal("symlinked destination admitted")
				}
				if !reflect.DeepEqual(before, treeSnapshot(t, outside)) {
					t.Fatal("wrote outside destination")
				}
			} else {
				mustWrite(t, filepath.Join(repo, "AGENTS.md"), "local\n"+bindingsBegin+"\nmalformed\n")
				before := treeSnapshot(t, repo)
				if err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard); err == nil {
					t.Fatal("malformed bindings admitted")
				}
				if !reflect.DeepEqual(before, treeSnapshot(t, repo)) {
					t.Fatal("refusal changed local bindings")
				}
			}
		})
	}
}

// regression: #2701
func TestCursorKeepsOutsideBindings(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	for _, existing := range []string{"local\n\n\n", "local no final newline", "before\n" + bindingsBegin + "\nold block\n<!-- assay:bindings:end -->\nafter\n\n"} {
		repo := t.TempDir()
		mustWrite(t, filepath.Join(repo, "AGENTS.md"), existing)
		if err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(existing, bindingsBegin) {
			if !strings.HasPrefix(string(raw), existing) {
				t.Fatalf("outside prefix changed: %q => %q", existing, raw)
			}
		} else if !strings.HasPrefix(string(raw), "before\n") || !strings.HasSuffix(string(raw), "after\n\n") {
			t.Fatalf("outside content changed: %q", raw)
		}
	}
}

// regression: #2701
func TestCursorSourceLinkRefuses(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	original := filepath.Join(bundle, "references")
	moved := filepath.Join(t.TempDir(), "references")
	if err := os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, original); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	repo := t.TempDir()
	if err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard); err == nil {
		t.Fatal("symlinked source parent admitted")
	}
	if len(treeSnapshot(t, repo)) != 0 {
		t.Fatal("refusal changed destination")
	}
}

// regression: #2701
func TestCursorReleaseCLI(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	mustWrite(t, filepath.Join(bundle, ".claude-plugin", "plugin.json"), `{"version":"1.2.3"}`)
	releases := t.TempDir()
	archive := filepath.Join(releases, "v1.2.3.cursor-bundle.tar.gz")
	var out bytes.Buffer
	if code := run([]string{"--harness", "cursor", "--bundle", bundle, "--release", "v1.2.3", "--pack", archive}, &out, &out); code != 0 {
		t.Fatalf("pack exit %d: %s", code, &out)
	}
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(releases, "v1.2.3.checksums.txt"), strings.Repeat("a", 64)+"  statusgen-linux-amd64\n"+fmt.Sprintf("%x  cursor-bundle.tar.gz\n", sha256.Sum256(raw)))
	repo := t.TempDir()
	args := []string{"--harness", "cursor", "--release", "v1.2.3", "--releases", releases, "--repo", repo, "--forge", "github"}
	for _, mode := range []string{"--dry-run", "", "--check"} {
		next := append([]string{}, args...)
		if mode != "" {
			next = append(next, mode)
		}
		out.Reset()
		if code := run(next, &out, &out); code != 0 {
			t.Fatalf("%s exit %d: %s", mode, code, &out)
		}
		if mode == "--dry-run" && len(treeSnapshot(t, repo)) != 0 {
			t.Fatal("release preview wrote destination")
		}
	}
	before := treeSnapshot(t, repo)
	mustWrite(t, archive, "tampered")
	for _, mode := range []string{"--dry-run", "", "--check"} {
		next := append([]string{}, args...)
		if mode != "" {
			next = append(next, mode)
		}
		out.Reset()
		if code := run(next, &out, &out); code == 0 {
			t.Fatalf("tampered %s admitted", mode)
		}
		if !reflect.DeepEqual(before, treeSnapshot(t, repo)) {
			t.Fatal("refusal changed adopter")
		}
	}
}

// regression: #2701
func TestCursorCanonicalMetadata(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	mustWrite(t, filepath.Join(bundle, "skills", "adopt", "component.yaml"), "component: assay/adopt\n")
	repo := t.TempDir()
	if err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".cursor", "skills", "adopt", "component.yaml")); !os.IsNotExist(err) {
		t.Fatalf("canonical component metadata duplicated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".cursor", "skills", "adopt", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}
