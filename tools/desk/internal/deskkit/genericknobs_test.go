package deskkit

// Property tests for the three knobs that finish the mechanism-ships /
// policy-is-supplied split across the desk tree: the DISPLAY taxonomy
// (ASSAY_REPO_ALIASES), the RELEASE home (ASSAY_RELEASE_REPO) and the writeguard
// dangerous-command CALLOUT (ASSAY_WRITEGUARD_CALLOUT).
//
// They differ in stakes and the tests say so. The taxonomy is display-only; the
// release home selects a target that IsAllowedRepo still screens; the callout sits in
// front of a write guard. What they share is the property that matters most for a
// shipped tool: UNSET is a complete configuration, not a degraded one, and each knob's
// effective value is visible in the P3 echo whether it was set, unset, or refused.

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// ---- unset is the shipped default, not a degraded state -----------------------

// TestGenericKnobsUnsetAreShippedDefaults — an adopter who configures none of the
// three gets empty values, NOT problems. This is the property that lets the tools
// publish at all: a fresh adopter must not have to configure a taxonomy, a release
// home and a callout before anything runs.
func TestGenericKnobsUnsetAreShippedDefaults(t *testing.T) {
	withRoster(t, goldenRoster())
	c := EffectiveConfig()
	if len(c.Problems) != 0 {
		t.Fatalf("an unset taxonomy/release/callout refused the configuration: %v", c.Problems)
	}
	if !c.Configured() {
		t.Fatal("the roster is not configured with the three new knobs unset")
	}
	if len(c.RepoAliases) != 0 {
		t.Fatalf("aliases = %v, want none", c.RepoAliases)
	}
	if c.ReleaseRepo != "" || ConfiguredReleaseRepo() != "" {
		t.Fatalf("release repo = %q, want empty so the consumer applies its shipped default", c.ReleaseRepo)
	}
	if c.WriteguardCallout != "" || WriteguardCalloutPath() != "" {
		t.Fatalf("callout = %q, want empty (compiled generic indicators only)", c.WriteguardCallout)
	}
}

// NOTE: ASSAY_REPO_ALIASES parsing, override resolution and its fail-closed /
// injectivity properties are pinned by the shared-resolver suite in
// repoalias_test.go (this unified branch takes #817's taxonomy base — one deskkit
// resolver behind both boards — rather than #814's per-call-site derivation). This
// file keeps only the RELEASE_REPO and WRITEGUARD_CALLOUT knobs plus the
// cross-cutting "unset is the shipped default" and P3-echo properties that span all
// three.

// ---- ASSAY_RELEASE_REPO ------------------------

func TestReleaseRepoParse(t *testing.T) {
	base := goldenRoster()
	base[EnvReleaseRepo] = "fork-owner/fork-repo"
	withRoster(t, base)
	if got := ConfiguredReleaseRepo(); got != "fork-owner/fork-repo" {
		t.Fatalf("ConfiguredReleaseRepo() = %q", got)
	}
}

// TestReleaseRepoMalformedRefused — a release tool that picked its target by parse
// order, or accepted a pattern naming a SET of repos, would be choosing what gets
// released on nobody's authority.
func TestReleaseRepoMalformedRefused(t *testing.T) {
	for name, value := range map[string]string{
		"a list":         "one/alpha,two/beta",
		"space list":     "one/alpha two/beta",
		"no owner":       "alpha",
		"trailing slash": "one/",
		"leading slash":  "/alpha",
		"a pattern":      "one/*",
		"too many parts": "one/two/three",
	} {
		t.Run(name, func(t *testing.T) {
			base := goldenRoster()
			base[EnvReleaseRepo] = value
			withRoster(t, base)
			c := EffectiveConfig()
			if len(c.Problems) == 0 {
				t.Fatalf("%s (%q) was accepted as a release home", name, value)
			}
			if c.ReleaseRepo != "" {
				t.Fatalf("a refused configuration still carries a release home: %q", c.ReleaseRepo)
			}
		})
	}
}

// ---- ASSAY_WRITEGUARD_CALLOUT ------------------------

func TestWriteguardCalloutParse(t *testing.T) {
	p := filepath.Join(t.TempDir(), "callout")
	base := goldenRoster()
	base[EnvWriteguardCallout] = p
	withRoster(t, base)
	if got := WriteguardCalloutPath(); got != p {
		t.Fatalf("WriteguardCalloutPath() = %q, want %q", got, p)
	}
}

// TestWriteguardCalloutRequiresAbsolutePath is the load-bearing shape rule. A relative
// callout path resolves against whatever directory the guard's process was spawned in
// — caller-influenced input choosing the guard's own policy source. It is refused at
// LOAD (rather than blocking at invocation) because a misconfigured guard should be
// loud in the P3 echo, not silently refusing every write on the machine.
func TestWriteguardCalloutRequiresAbsolutePath(t *testing.T) {
	for name, value := range map[string]string{
		"bare name":    "callout.sh",
		"dot-relative": "./callout.sh",
		"parent":       "../tools/callout.sh",
		"a list":       "/a/callout,/b/callout",
	} {
		t.Run(name, func(t *testing.T) {
			base := goldenRoster()
			base[EnvWriteguardCallout] = value
			withRoster(t, base)
			c := EffectiveConfig()
			if len(c.Problems) == 0 {
				t.Fatalf("%s (%q) was accepted as a callout path", name, value)
			}
			if c.WriteguardCallout != "" {
				t.Fatalf("a refused configuration still carries a callout: %q", c.WriteguardCallout)
			}
		})
	}
}

// ---- P3: all three echo, and BOTH directions are visible ----------------------

// TestGenericKnobsEchoBothDirections — each knob's line appears in the P3 echo, and
// setting it CHANGES the echo while unsetting it changes it back. A knob whose
// narrowing is invisible is a knob whose removal is invisible, and for the callout
// that is the difference between "the adopter's check is running" and "it is not".
func TestGenericKnobsEchoBothDirections(t *testing.T) {
	for _, s := range []struct {
		name, key, value string
	}{
		{"repoAliases", EnvRepoAliases, "one/alpha=al:prod"},
		{"releaseRepo", EnvReleaseRepo, "fork-owner/fork-repo"},
		{"writeguardCallout", EnvWriteguardCallout, "/opt/adopter/writeguard-callout"},
	} {
		t.Run(s.name, func(t *testing.T) {
			withRoster(t, goldenRoster())
			var unset bytes.Buffer
			EchoEffectiveConfig(&unset)
			if !strings.Contains(unset.String(), s.key+"=") {
				t.Fatalf("the run echo carries no %s line when the knob is UNSET — an operator "+
					"cannot see that the surface exists, let alone that it is empty", s.key)
			}

			base := goldenRoster()
			base[s.key] = s.value
			withRoster(t, base)
			var set bytes.Buffer
			EchoEffectiveConfig(&set)
			if !strings.Contains(set.String(), s.value) {
				t.Fatalf("the run echo does not render the configured %s value:\n%s", s.key, set.String())
			}
			if set.String() == unset.String() {
				t.Fatalf("setting %s did not change the run echo — the change leaves no trace in "+
					"run output or CI history", s.key)
			}

			withRoster(t, goldenRoster())
			var again bytes.Buffer
			EchoEffectiveConfig(&again)
			if again.String() == set.String() {
				t.Fatalf("UNSETTING %s did not change the run echo. Removal must be as visible as "+
					"addition", s.key)
			}
		})
	}
}

// TestUnsetKnobsEchoTheirMeaning — the release home and the callout render their unset
// state as a named default rather than as a blank. A blank line beside a GUARD is the
// one place a reader must not have to guess whether the check is running.
func TestUnsetKnobsEchoTheirMeaning(t *testing.T) {
	withRoster(t, goldenRoster())
	var buf bytes.Buffer
	EchoEffectiveConfig(&buf)
	out := buf.String()
	for key, want := range map[string]string{
		EnvReleaseRepo:       "shipped default",
		EnvWriteguardCallout: "compiled generic indicators only",
	} {
		line := ""
		for _, l := range strings.Split(out, "\n") {
			if strings.Contains(l, key+"=") {
				line = l
			}
		}
		if line == "" {
			t.Fatalf("no echo line for %s", key)
		}
		if !strings.Contains(line, want) {
			t.Fatalf("the unset %s line does not say what unset MEANS (%q):\n%s", key, want, line)
		}
	}
}

// TestNewKnobsAreRecognisedKeys — the ASSAY_ namespace refuses any key it does not
// know, so a roster carrying all three must NOT be refused. This is the check that
// would have caught adding a variable to one reader's parser and not the other's.
func TestNewKnobsAreRecognisedKeys(t *testing.T) {
	base := goldenRoster()
	base[EnvRepoAliases] = "one/alpha=al:prod"
	base[EnvReleaseRepo] = "one/alpha"
	base[EnvWriteguardCallout] = "/opt/adopter/callout"
	withRoster(t, base)
	if p := EffectiveConfig().Problems; len(p) != 0 {
		t.Fatalf("a roster carrying the three new keys was refused: %v", p)
	}
}
