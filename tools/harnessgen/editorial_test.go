package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Writing projects install this directory on its own. Verify its complete payload,
// not just its SKILL.md files: missing references break the prompts, and an extra
// hook or skill changes what a fresh install loads.
func TestEditorialPluginContents(t *testing.T) {
	root := filepath.Join("..", "..")
	pkg := filepath.Join(root, "plugins", "editorial")
	files := map[string]string{
		"LICENSE":                      "LICENSE",
		"NOTICE":                       "NOTICE",
		"README.md":                    "",
		"skills/humanizer/SKILL.md":    "",
		"skills/humanizer/LICENSE.txt": "",
		"skills/humanizer/references/writing-patterns.md": "",
	}
	for _, member := range []string{
		"skills/deck-author/SKILL.md",
		"skills/deck-author/workflow.md",
		"skills/deck-author/assets/deck.html",
		"skills/deck-author/references/brand-contract.md",
		"skills/deck-author/tools/check_deck.py",
		"skills/deck-author/tools/render-deck-pdf.cjs",
		"skills/deck-author/tools/verify-pdf-footer.cjs",
		"skills/deck-author/tools/verify-pdf-uris.cjs",
		"skills/deck-typography/SKILL.md",
		"skills/slop-lint/SKILL.md",
		"skills/staging/LINEAGE.md",
		"skills/staging/SKILL.md",
		"skills/staging/workflow.md",
		"skills/staging/references/audience.md",
		"skills/staging/references/creative-development.md",
		"skills/staging/references/executive-presence.md",
		"skills/staging/references/genres/STUBS.md",
		"skills/staging/references/genres/executive-explainer.md",
		"skills/staging/references/genres/practitioner-explainer.md",
		"skills/staging/references/genres/product-pitch.md",
		"skills/staging/references/genres/teach-workshop.md",
		"skills/staging/references/genres/technical-deep-dive.md",
		"skills/staging/references/motion-roles.md",
		"skills/staging/references/motion-spec.md",
		"skills/staging/references/seam-gate.md",
		"skills/staging/references/slide-functions.md",
		"skills/staging/references/transitions.md",
		"skills/staging/references/video-editorial-review.md",
		"skills/staging/references/vocabulary-map.md",
		"skills/staging/tests/fixtures/bad-diagram-names-a-chart.yaml",
		"skills/staging/tests/fixtures/bad-mechanism-too-late.yaml",
		"skills/staging/tests/fixtures/bad-missing-number-gate.yaml",
		"skills/staging/tests/fixtures/bad-withheld-figure-on-slot.yaml",
		"skills/staging/tests/fixtures/good.yaml",
		"skills/staging/tests/test_list_syntax.py",
		"skills/staging/tools/LICENSE-hyperframes-Apache-2.0.txt",
		"skills/staging/tools/seam-gate-hard-cut-test.mjs",
		"skills/staging/tools/seam-gate.mjs",
		"skills/staging/tools/staging_lint.py",
		"skills/video-author/SKILL.md",
		"skills/video-author/workflow.md",
		"skills/video-author/references/driver-pattern.md",
		"skills/video-author/references/native-rendering.md",
		"skills/video-author/references/render-check.md",
		"skills/video-author/references/screen-grammar.md",
		"skills/video-author/references/voice-delivery.md",
		"skills/video-author/tests/test_native_export.py",
		"skills/video-author/tools/deliver_gate.py",
		"skills/video-author/tools/native_export.py",
		"skills/video-author/tools/player.js",
		"skills/video-author/tools/render-check.mjs",
		"skills/video-author/tools/script_scenes.py",
		"skills/video-design/SKILL.md",
	} {
		files[member] = ""
	}
	for _, skill := range []string{"article-craft", "prose-cadence"} {
		source := filepath.Join("plugins", "assay", "skills", skill)
		files["skills/"+skill+"/SKILL.md"] = filepath.Join(source, "SKILL.md")
		err := fs.WalkDir(os.DirFS(filepath.Join(root, source)), ".", func(name string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				files["skills/"+skill+"/"+name] = filepath.Join(source, name)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	var metadata map[string]any
	for _, harness := range []string{"claude", "codex", "cursor"} {
		rel := "." + harness + "-plugin/plugin.json"
		files[rel] = ""
		buf, err := os.ReadFile(filepath.Join(pkg, rel))
		if err != nil {
			t.Fatal(err)
		}
		var manifest map[string]any
		if err := json.Unmarshal(buf, &manifest); err != nil {
			t.Fatal(err)
		}
		if harness == "claude" {
			version, _ := manifest["version"].(string)
			if manifest["name"] != "editorial" || version == "" {
				t.Fatalf("%s must identify the versioned editorial plugin", rel)
			}
			for field := range manifest {
				switch field {
				case "name", "version", "description", "author", "license":
				default:
					t.Errorf("unexpected field %q in skills-only editorial manifest", field)
				}
			}
			metadata = manifest
		} else {
			if manifest["skills"] != "./skills/" {
				t.Errorf("%s does not load the standalone skills directory", rel)
			}
			delete(manifest, "skills")
			if !reflect.DeepEqual(manifest, metadata) {
				t.Errorf("%s metadata differs from the Claude manifest", rel)
			}
		}
	}
	err := fs.WalkDir(os.DirFS(pkg), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		src, ok := files[name]
		if !ok || !entry.Type().IsRegular() {
			t.Errorf("unexpected editorial package member %q", name)
		}
		delete(files, name)
		if src != "" {
			source, err := os.ReadFile(filepath.Join(root, src))
			if err != nil {
				return err
			}
			packed, err := os.ReadFile(filepath.Join(pkg, name))
			if err != nil {
				return err
			}
			if !bytes.Equal(source, packed) {
				t.Errorf("%s differs from %s", name, src)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name := range files {
		t.Errorf("missing editorial package member %q", name)
	}
}

// Project-local installs also redistribute this notice.
func TestEditorialHumanizerLicense(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "plugins", "editorial", "skills", "humanizer", "LICENSE.txt"))
	if err != nil {
		t.Fatal(err)
	}
	const upstream = "4ac4810254ab36d45419141aeb8e69bf50652cfafe5b2dab947d06d44e5cbf96"
	if fmt.Sprintf("%x", sha256.Sum256(data)) != upstream {
		t.Error("Humanizer license differs from its pinned upstream notice")
	}
}

// Claude and Codex read the Claude catalog; Cursor imports its own projection.
func TestEditorialMarketplaceEntries(t *testing.T) {
	for _, catalog := range []string{".claude-plugin", ".cursor-plugin"} {
		buf, err := os.ReadFile(filepath.Join("..", "..", catalog, "marketplace.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m cursorMarketplace
		if err := json.Unmarshal(buf, &m); err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, entry := range m.Plugins {
			if entry.Name == "editorial" {
				count++
				if entry.Source != "./plugins/editorial" {
					t.Errorf("%s points editorial at %q", catalog, entry.Source)
				}
			}
		}
		if m.Name != "assay" || count != 1 {
			t.Errorf("%s must list editorial@assay exactly once", catalog)
		}
	}
}
