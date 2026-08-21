package deskkit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadComposition_PlainTag — a plain vX.Y.Z umbrella maps to <tag>.yaml and
// resolves per-artifact tags by explicit name.
func TestLoadComposition_PlainTag(t *testing.T) {
	c, err := LoadComposition(filepath.Join("testdata", "marker-known", ReleasesDir), "v0.11.0")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Umbrella != "v0.11.0" {
		t.Errorf("umbrella = %q", c.Umbrella)
	}
	if tag, ok := c.TagFor("statusgen"); !ok || tag != "v0.11.0" {
		t.Errorf("statusgen tag = %q ok=%v, want v0.11.0", tag, ok)
	}
	if _, ok := c.TagFor("nonesuch"); ok {
		t.Errorf("unnamed artifact should not resolve")
	}
}

// TestLoadComposition_NamespacedTagBackCompat — an entry with only a namespaced
// tag still resolves its component name from the tag (older manifests read).
func TestLoadComposition_NamespacedTagBackCompat(t *testing.T) {
	dir := t.TempDir()
	body := "umbrella: assay/v0.9.0\nartifacts:\n  - tag: statusgen/v0.8.2\n"
	if err := os.WriteFile(filepath.Join(dir, "assay-v0.9.0.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadComposition(dir, "assay/v0.9.0")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if tag, ok := c.TagFor("statusgen"); !ok || tag != "statusgen/v0.8.2" {
		t.Errorf("namespaced entry: statusgen tag = %q ok=%v", tag, ok)
	}
}

// TestLoadComposition_MissingIsUnverifiable — a missing manifest is fail-closed.
func TestLoadComposition_MissingIsUnverifiable(t *testing.T) {
	if _, err := LoadComposition(t.TempDir(), "v9.9.9"); !IsUnverifiable(err) {
		t.Errorf("missing manifest should be unverifiable, got %v", err)
	}
}
