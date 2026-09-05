package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// errUnreachable stands in for a transport failure against the release home —
// distinct from errNoRelease, which is a real answer.
var errUnreachable = errors.New("dial tcp: connection refused")

// fakeSource is a releaseSource backed by an in-memory table, so every
// release-shaped failure mode (tag never cut, draft, unreachable release home)
// is exercised without a network.
type fakeSource struct {
	rels map[string]*release
	err  error
}

func (f fakeSource) Release(home, tag string) (*release, error) {
	if f.err != nil {
		return nil, f.err
	}
	r, ok := f.rels[home+"@"+tag]
	if !ok {
		return nil, errNoRelease
	}
	return r, nil
}

const (
	fakeHome = "example-org/example-releases"
	shaA     = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shaB     = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	shaC     = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	shaD     = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
)

// publishedV123 is the release the testdata manifests are pinned against.
func publishedV123() *release {
	return &release{
		Tag: "v1.2.3",
		Assets: []string{
			checksumsAsset,
			"statusgen-darwin-arm64", "statusgen-linux-amd64",
			"desk-tools-darwin-arm64.tar.gz", "desk-tools-linux-amd64.tar.gz",
		},
		Checksums: map[string]string{
			"statusgen-darwin-arm64":         shaA,
			"statusgen-linux-amd64":          shaB,
			"desk-tools-darwin-arm64.tar.gz": shaC,
			"desk-tools-linux-amd64.tar.gz":  shaD,
		},
	}
}

func srcWith(r *release) fakeSource {
	return fakeSource{rels: map[string]*release{fakeHome + "@v1.2.3": r}}
}

// tree materialises a repo root carrying the two records Check reads, from the
// named testdata fixtures. An empty name omits that file entirely.
func tree(t *testing.T, manifestFixture, pluginFixture string) string {
	t.Helper()
	root := t.TempDir()
	copyFixture := func(fixture, dest string) {
		if fixture == "" {
			return
		}
		b, err := os.ReadFile(filepath.Join("testdata", fixture))
		if err != nil {
			t.Fatalf("reading fixture %s: %v", fixture, err)
		}
		full := filepath.Join(root, dest)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	copyFixture(manifestFixture, manifestPath)
	copyFixture(pluginFixture, pluginJSONPath)
	return root
}

func run(t *testing.T, root string, src releaseSource) (bool, string) {
	t.Helper()
	var buf bytes.Buffer
	ok := Check(root, src, &buf)
	return ok, buf.String()
}

// TestCheckPasses is the only case that may return true. Everything else in this
// file asserts the guard goes RED — which is the property that matters, since a
// guard that cannot fail is decoration.
func TestCheckPasses(t *testing.T) {
	ok, out := run(t, tree(t, "ok.yaml", "plugin-0.5.0.json"), srcWith(publishedV123()))
	if !ok {
		t.Fatalf("consistent front door reported a problem:\n%s", out)
	}
	if !strings.Contains(out, "pairedversions: OK") {
		t.Errorf("want an OK line, got:\n%s", out)
	}
}

// TestCheckFails walks every failure mode. Each case names the substring the
// report must carry, so a check that reddens for the WRONG reason still fails
// the test.
func TestCheckFails(t *testing.T) {
	draft := publishedV123()
	draft.Draft = true

	// A release whose checksums.txt omits an artifact the release nonetheless
	// publishes: the hash authority cannot corroborate the pin — could-not-check,
	// never a pass.
	noEntry := publishedV123()
	delete(noEntry.Checksums, "statusgen-linux-amd64")

	cases := []struct {
		name     string
		manifest string
		plugin   string
		src      releaseSource
		want     string
	}{{
		// (a) — the defect this guard was written for.
		name:     "plugin version disagrees with the pairing",
		manifest: "version-mismatch.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      srcWith(publishedV123()),
		want:     "version (0.5.0) != plugins/assay/paired-versions.yaml `plugin` (0.4.0)",
	}, {
		name:     "pairing manifest declares no plugin at all",
		manifest: "no-plugin-field.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      srcWith(publishedV123()),
		want:     "carries no `plugin` field",
	}, {
		name:     "plugin manifest carries no version",
		manifest: "ok.yaml",
		plugin:   "plugin-no-version.json",
		src:      srcWith(publishedV123()),
		want:     "carries no `version` field",
	}, {
		// (b) — the tag was never cut.
		name:     "paired tag is not a published release",
		manifest: "ok.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      fakeSource{rels: map[string]*release{}},
		want:     "is NOT a published release",
	}, {
		// (b) — cut, but not published.
		name:     "paired tag is a draft release",
		manifest: "ok.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      srcWith(draft),
		want:     "is a DRAFT release",
	}, {
		// (c) — the hand-invented / locally-built hash.
		name:     "pinned sha256 disagrees with checksums.txt",
		manifest: "sha-mismatch.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      srcWith(publishedV123()),
		want:     "pinned sha256 for statusgen-darwin-arm64",
	}, {
		name:     "pin line repeats a tag the section does not pin",
		manifest: "pin-tag-mismatch.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      srcWith(publishedV123()),
		want:     "pin line names tag v1.0.0 but the section pins v1.2.3",
	}, {
		name:     "pin names an artifact the release does not publish",
		manifest: "unknown-artifact.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      srcWith(publishedV123()),
		want:     `publishes no asset named "statusgen-linux-arm64"`,
	}, {
		name:     "pin line is not the channel-E shape",
		manifest: "malformed-pin.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      srcWith(publishedV123()),
		want:     "is not the channel-E",
	}, {
		name:     "section pins no tag",
		manifest: "no-tag.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      srcWith(publishedV123()),
		want:     "carries no `tag`",
	}, {
		name:     "artifact has no checksums.txt entry",
		manifest: "ok.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      srcWith(noEntry),
		want:     "has no entry in the checksums.txt",
	}, {
		// could-not-check is reported as itself and is still not green.
		name:     "release home cannot be read",
		manifest: "ok.yaml",
		plugin:   "plugin-0.5.0.json",
		src:      fakeSource{err: errUnreachable},
		want:     "could-not-check",
	}, {
		name:     "pairing manifest is missing",
		manifest: "",
		plugin:   "plugin-0.5.0.json",
		src:      srcWith(publishedV123()),
		want:     "cannot read plugins/assay/paired-versions.yaml",
	}, {
		name:     "plugin manifest is missing",
		manifest: "ok.yaml",
		plugin:   "",
		src:      srcWith(publishedV123()),
		want:     "cannot read plugins/assay/.claude-plugin/plugin.json",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, out := run(t, tree(t, tc.manifest, tc.plugin), tc.src)
			if ok {
				t.Fatalf("guard passed on %s — it cannot fail, so it guards nothing:\n%s", tc.name, out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("report does not name the problem.\nwant substring: %s\ngot:\n%s", tc.want, out)
			}
		})
	}
}

// TestCheckReportsEveryProblemAtOnce pins the no-short-circuit property: a
// re-pin should be one edit, not a sequence of one-failure-per-CI-round.
func TestCheckReportsEveryProblemAtOnce(t *testing.T) {
	// version-mismatch.yaml trips (a); the empty source trips (b) for BOTH
	// sections.
	_, out := run(t, tree(t, "version-mismatch.yaml", "plugin-0.5.0.json"), fakeSource{rels: map[string]*release{}})
	if n := strings.Count(out, "pairedversions: FAIL"); n < 3 {
		t.Errorf("want at least 3 problems reported in one run, got %d:\n%s", n, out)
	}
	for _, sec := range []string{"`statusgen`", "`desk-tools`"} {
		if !strings.Contains(out, sec) {
			t.Errorf("report does not cover section %s:\n%s", sec, out)
		}
	}
}

// TestPrereleaseIsANoteNotAFailure — a prerelease IS published and installable,
// so it is surfaced rather than failed. Silence would be the bug.
func TestPrereleaseIsANoteNotAFailure(t *testing.T) {
	pre := publishedV123()
	pre.Prerelease = true
	ok, out := run(t, tree(t, "ok.yaml", "plugin-0.5.0.json"), srcWith(pre))
	if !ok {
		t.Fatalf("a prerelease pin should not fail the run:\n%s", out)
	}
	if !strings.Contains(out, "PRE-release") {
		t.Errorf("a prerelease pin must not be silent:\n%s", out)
	}
}
