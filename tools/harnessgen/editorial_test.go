package main

import (
	"bytes"
	"encoding/json"
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
		"LICENSE":   "LICENSE",
		"NOTICE":    "NOTICE",
		"README.md": "",
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
