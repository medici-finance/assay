package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- manifest reader, against the format fixture -------------------------------

// TestLoadManifestReadsTheFormatFixture parses the shipped releases/example.yaml fixture
// and pins the format fields: `umbrella:` and `artifacts:[].tag`. It is the Go-side
// companion to the `yq` check — the manifest is readable by more than one parser.
func TestLoadManifestReadsTheFormatFixture(t *testing.T) {
	// Repo root is four levels up from this package (tools/desk/cmd/deskrelease/).
	path := filepath.Join("..", "..", "..", "..", "releases", "example.yaml")
	m, err := loadManifest(path)
	if err != nil {
		t.Fatalf("loadManifest(%s): %v", path, err)
	}
	if m.Umbrella != "assay/v0.9.0" {
		t.Fatalf("umbrella = %q, want %q", m.Umbrella, "assay/v0.9.0")
	}
	if len(m.Artifacts) == 0 {
		t.Fatal("fixture parsed with zero artifacts")
	}
	for _, a := range m.Artifacts {
		if _, _, ok := splitArtifactTag(a.Tag); !ok {
			t.Fatalf("fixture artifact tag %q does not match <component>/vX.Y.Z", a.Tag)
		}
	}
}

// --- negative test 1: umbrella regression is refused ----------------------------

// TestCheckUmbrellaMonotonic_RefusesRegression pins the umbrella's own per-line
// monotonicity (authority rule 4: "the umbrella is its own line"): a new manifest whose
// umbrella version does not sort above the previous umbrella's is refused.
func TestCheckUmbrellaMonotonic_RefusesRegression(t *testing.T) {
	prev := Manifest{Umbrella: "assay/v0.9.0"}

	cases := []struct {
		name string
		next Manifest
	}{
		{name: "lower patch", next: Manifest{Umbrella: "assay/v0.8.9"}},
		{name: "equal version — a manifest is never re-released under its own number", next: Manifest{Umbrella: "assay/v0.9.0"}},
		{name: "lower major", next: Manifest{Umbrella: "assay/v0.8.99"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkUmbrellaMonotonic(prev, tc.next)
			if err == nil {
				t.Fatalf("checkUmbrellaMonotonic(prev=%s, next=%s) accepted a non-increasing umbrella version",
					prev.Umbrella, tc.next.Umbrella)
			}
			if !strings.Contains(err.Error(), "sort above") {
				t.Fatalf("error %q does not name the sorts-above invariant", err.Error())
			}
		})
	}
}

// TestCheckUmbrellaMonotonic_AcceptsIncrease is the positive half — a genuine bump is not
// refused, so the row above cannot be satisfied by a function that refuses everything.
func TestCheckUmbrellaMonotonic_AcceptsIncrease(t *testing.T) {
	prev := Manifest{Umbrella: "assay/v0.9.0"}
	next := Manifest{Umbrella: "assay/v0.10.0"}
	if err := checkUmbrellaMonotonic(prev, next); err != nil {
		t.Fatalf("checkUmbrellaMonotonic(prev=%s, next=%s) refused a genuine increase: %v",
			prev.Umbrella, next.Umbrella, err)
	}
}

// --- negative test 2: per-artifact / umbrella independence ----------------------

// TestPerArtifactCutIsUnaffectedByUmbrellaTagsAndViceVersa is the independence property:
// cutting a per-artifact tag succeeds whether or not an umbrella tag
// already exists, and cutting the umbrella tag succeeds whether or not a per-artifact tag
// already exists — the two namespaces do not read or gate each other anywhere in `cut.go`.
// Exercised end-to-end through `run` and the fake GitHub harness, not just the regex, so it
// pins the property at the layer where a future coupling (e.g. "refuse a component cut
// unless an umbrella exists") would actually get introduced.
func TestPerArtifactCutIsUnaffectedByUmbrellaTagsAndViceVersa(t *testing.T) {
	t.Run("a per-artifact cut succeeds with an umbrella tag already present", func(t *testing.T) {
		h := newHarness(t)
		h.gh.refs["tags/assay/v0.9.0"] = mainSHA

		if code := run([]string{"cut", "statusgen/v0.8.1"}); code != deskkit.ExitOK {
			t.Fatalf("exit %d, want 0; stderr=%s", code, h.errb.String())
		}
		if got := h.gh.refs["tags/statusgen/v0.8.1"]; got != mainSHA {
			t.Fatalf("statusgen tag points at %q, want %q", got, mainSHA)
		}
		// The umbrella tag it started with is untouched.
		if got := h.gh.refs["tags/assay/v0.9.0"]; got != mainSHA {
			t.Fatalf("pre-existing umbrella tag moved to %q", got)
		}
	})

	t.Run("the umbrella cut succeeds with a per-artifact tag already present", func(t *testing.T) {
		h := newHarness(t)
		h.gh.refs["tags/statusgen/v0.8.0"] = mainSHA

		if code := run([]string{"cut", "assay/v0.9.0"}); code != deskkit.ExitOK {
			t.Fatalf("exit %d, want 0; stderr=%s", code, h.errb.String())
		}
		if got := h.gh.refs["tags/assay/v0.9.0"]; got != mainSHA {
			t.Fatalf("assay tag points at %q, want %q", got, mainSHA)
		}
		if got := h.gh.refs["tags/statusgen/v0.8.0"]; got != mainSHA {
			t.Fatalf("pre-existing per-artifact tag moved to %q", got)
		}
	})
}

// TestSplitArtifactTagRejectsMalformed is a small characterisation test on the shape check
// Verify row 5 and the manifest reader both lean on — one clearly-labelled pin, not a
// re-test of tagPattern (which cut_test.go already covers).
func TestSplitArtifactTagRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "v0.9.0", "assay", "ASSAY/v0.9.0", "assay/0.9.0", "assay/v0.9"} {
		if _, _, ok := splitArtifactTag(bad); ok {
			t.Fatalf("splitArtifactTag(%q) accepted a malformed tag", bad)
		}
	}
}

// TestSplitArtifactTagRejectsLeadingZeros pins the "no leading zeros" half of
// version-scheme.md's tag grammar, which names releaseguard's versionRE as canonical.
// Before this was enforced, `[0-9]+` per field accepted every case below.
func TestSplitArtifactTagRejectsLeadingZeros(t *testing.T) {
	for _, bad := range []string{
		"statusgen/v01.2.3", // major
		"statusgen/v0.08.3", // minor
		"statusgen/v0.8.03", // patch
		"assay/v00.9.0",     // doubled zero
		"assay/v0.9.0-rc.1", // pre-release: refused here on purpose (strict subset of versionRE)
	} {
		if _, _, ok := splitArtifactTag(bad); ok {
			t.Fatalf("splitArtifactTag(%q) accepted a tag version-scheme.md's grammar forbids", bad)
		}
	}
}

// TestSplitArtifactTagAcceptsWellFormed is the no-cry-wolf half: the tightened pattern must
// still accept every shape the scheme calls valid, including a multi-digit and a
// hyphenated-component line (a future plugin and container-image line each add one).
func TestSplitArtifactTagAcceptsWellFormed(t *testing.T) {
	cases := map[string][3]int{
		"assay/v0.9.0":          {0, 9, 0},
		"statusgen/v0.8.2":      {0, 8, 2},
		"desk-tools/v0.2.6":     {0, 2, 6},
		"daily-harvest/v0.1.0":  {0, 1, 0},
		"assay/v0.10.0":         {0, 10, 0},
		"harness-image/v12.0.7": {12, 0, 7},
	}
	for tag, want := range cases {
		_, got, ok := splitArtifactTag(tag)
		if !ok {
			t.Fatalf("splitArtifactTag(%q) refused a well-formed tag", tag)
		}
		if got != want {
			t.Fatalf("splitArtifactTag(%q) = %v, want %v", tag, got, want)
		}
	}
}

// TestLeadingZeroTagsDoNotCollideWithTheirCanonicalForm is the harm the rule exists to stop:
// under `[0-9]+`, "v0.8.03" and "v0.8.3" both parsed to [0 8 3], so two distinct git tags
// became one version tuple and checkUmbrellaMonotonic could no longer tell a bump from a
// duplicate. Asserting the collision is impossible pins the property, not just the regex.
func TestLeadingZeroTagsDoNotCollideWithTheirCanonicalForm(t *testing.T) {
	_, canonical, ok := splitArtifactTag("assay/v0.9.0")
	if !ok {
		t.Fatal("the canonical form must parse")
	}
	if _, colliding, ok := splitArtifactTag("assay/v0.09.0"); ok {
		t.Fatalf("assay/v0.09.0 parsed to %v, colliding with assay/v0.9.0's %v — two tags, one tuple",
			colliding, canonical)
	}
}

// TestCheckUmbrellaMonotonic_FailsClosedOnAnUnparseableUmbrella proves the ordering check
// reports could-not-check rather than "no violation" when it cannot read its inputs. A
// monotonicity check that returns nil on a tag it failed to parse is the fail-OPEN shape:
// it would wave through exactly the malformed umbrella it exists to judge.
func TestCheckUmbrellaMonotonic_FailsClosedOnAnUnparseableUmbrella(t *testing.T) {
	good := Manifest{Umbrella: "assay/v0.9.0"}

	cases := []struct {
		name       string
		prev, next Manifest
		wantIn     string
	}{
		{"unparseable previous", Manifest{Umbrella: "garbage"}, good, "previous manifest's umbrella"},
		{"unparseable next", good, Manifest{Umbrella: "garbage"}, "next manifest's umbrella"},
		{"empty previous", Manifest{}, good, "previous manifest's umbrella"},
		{"empty next", good, Manifest{}, "next manifest's umbrella"},
		{"leading-zero next", good, Manifest{Umbrella: "assay/v0.10.00"}, "next manifest's umbrella"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkUmbrellaMonotonic(tc.prev, tc.next)
			if err == nil {
				t.Fatalf("checkUmbrellaMonotonic(prev=%q, next=%q) returned nil — an unreadable input "+
					"was reported as 'no violation'", tc.prev.Umbrella, tc.next.Umbrella)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("error %q does not say WHICH side could not be read (want %q)", err.Error(), tc.wantIn)
			}
		})
	}
}

// TestLoadManifestFailsClosedOnUnreadableInput is the same property one layer out: a missing
// or malformed manifest is an error, never a zero-value Manifest that a caller would then
// read as "an umbrella naming no artifacts".
func TestLoadManifestFailsClosedOnUnreadableInput(t *testing.T) {
	dir := t.TempDir()

	if _, err := loadManifest(filepath.Join(dir, "does-not-exist.yaml")); err == nil {
		t.Fatal("loadManifest on a missing file returned nil error")
	}

	bad := filepath.Join(dir, "malformed.yaml")
	if err := os.WriteFile(bad, []byte("umbrella: [this is not a string\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(bad); err == nil {
		t.Fatal("loadManifest on malformed YAML returned nil error")
	}

	// A directory is unreadable-as-a-file: still an error, not an empty manifest.
	if _, err := loadManifest(dir); err == nil {
		t.Fatal("loadManifest on a directory returned nil error")
	}
}
