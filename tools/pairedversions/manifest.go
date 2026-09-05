package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Paths, relative to the repo root, of the two records this checker compares.
// They are the front door: `assay:install` reads BOTH — the plugin version from
// the first, the statusgen tag and per-platform sha256 from the second — so a
// disagreement between them installs a tool the shipped skills were never
// tested against.
const (
	pluginJSONPath = "plugins/assay/.claude-plugin/plugin.json"
	manifestPath   = "plugins/assay/paired-versions.yaml"
)

// section is one half of the pairing manifest: a release home, the pinned tag,
// and the per-platform channel-E pin lines that name the artifact, repeat the
// tag, and carry the sha256 the installer verifies the download against.
type section struct {
	ReleaseHome string            `yaml:"release_home"`
	Tag         string            `yaml:"tag"`
	Platforms   map[string]string `yaml:"platforms"`
}

// manifest is plugins/assay/paired-versions.yaml.
type manifest struct {
	Schema    string  `yaml:"schema"`
	Plugin    string  `yaml:"plugin"`
	Statusgen section `yaml:"statusgen"`
	DeskTools section `yaml:"desk-tools"`
}

// pin is one parsed channel-E platform line: `<artifact> <tag> <sha256>`.
type pin struct {
	Platform string
	Artifact string
	Tag      string
	SHA256   string
}

var (
	semverRe = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// readPluginVersion returns the `version` field of the plugin manifest. That
// file is the declared OWNER of the plugin's version — the field the plugin
// platform itself resolves — so it is read as-is and never reconciled against a
// second guess.
func readPluginVersion(root string) (string, error) {
	p := filepath.Join(root, pluginJSONPath)
	b, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", pluginJSONPath, err)
	}
	var doc struct {
		Version *string `json:"version"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return "", fmt.Errorf("cannot parse %s: %w", pluginJSONPath, err)
	}
	if doc.Version == nil {
		return "", fmt.Errorf("%s carries no `version` field", pluginJSONPath)
	}
	return *doc.Version, nil
}

// readManifest parses the pairing manifest.
func readManifest(root string) (*manifest, error) {
	p := filepath.Join(root, manifestPath)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", manifestPath, err)
	}
	var m manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", manifestPath, err)
	}
	return &m, nil
}

// pins parses a section's platform lines into structured pins. A line that does
// not have the exact three-field channel-E shape is returned as an error rather
// than skipped: a pin the checker cannot read is a pin it has not checked, and
// an unread pin is never reported clean.
func (s section) pins() ([]pin, []error) {
	var out []pin
	var errs []error
	for _, plat := range sortedKeys(s.Platforms) {
		raw := strings.TrimSpace(s.Platforms[plat])
		f := strings.Fields(raw)
		if len(f) != 3 {
			errs = append(errs, fmt.Errorf("platform %q: pin line %q is not the channel-E `<artifact> <tag> <sha256>` shape (%d fields, want 3)", plat, raw, len(f)))
			continue
		}
		if !sha256Re.MatchString(f[2]) {
			errs = append(errs, fmt.Errorf("platform %q: %q is not a bare lowercase 64-hex sha256", plat, f[2]))
			continue
		}
		out = append(out, pin{Platform: plat, Artifact: f[0], Tag: f[1], SHA256: f[2]})
	}
	return out, errs
}
