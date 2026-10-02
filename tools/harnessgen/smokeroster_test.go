package main

// Class guard for the Codex smoke protocol's skill roster
// (docs/codex-smoke-protocol.md, harness-portability/07).
//
// Defect class: the protocol's Step 3 names the skills a live run must invoke,
// and that hand-kept list drifts from the bundle's skills tree whenever a skill
// is added or removed — a live run then skips a shipped skill (or chases one
// that no longer ships) and still reads as complete. It has drifted twice: once
// fixed by hand for issue 938, and again when `system-demo` joined the bundle
// without joining the list. This test holds Step 3's `assay:<name>` list equal to
// plugins/assay/skills/*/SKILL.md, so the next drift is red in CI instead of
// being found on a live run.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const smokeProtocolPath = "../../docs/codex-smoke-protocol.md"

var smokeSkillToken = regexp.MustCompile("`assay:([a-z0-9][a-z0-9-]*)`")

// smokeStep3 returns the text of the protocol's Step 3 section: from its
// `#### Step 3` heading up to the next `#### ` heading. ok is false when the
// heading is absent.
func smokeStep3(protocol string) (string, bool) {
	start := strings.Index(protocol, "\n#### Step 3")
	if start < 0 {
		return "", false
	}
	rest := protocol[start+1:]
	if end := strings.Index(rest[len("#### Step 3"):], "\n#### "); end >= 0 {
		rest = rest[:len("#### Step 3")+end]
	}
	return rest, true
}

// smokeRosterViolations compares Step 3's `assay:<name>` list with the skills on
// disk. Every mismatch is one message; a missing Step 3 or an empty list is a
// violation too, so a broken matcher cannot read as clean.
func smokeRosterViolations(protocol string, disk map[string]bool) []string {
	step3, ok := smokeStep3(protocol)
	if !ok {
		return []string{"no `#### Step 3` section found in the smoke protocol"}
	}
	listed := map[string]bool{}
	for _, m := range smokeSkillToken.FindAllStringSubmatch(step3, -1) {
		listed[m[1]] = true
	}
	if len(listed) == 0 {
		return []string{"Step 3 names no `assay:<skill>` tokens"}
	}
	var msgs []string
	for _, name := range sortedKeysBool(disk) {
		if !listed[name] {
			msgs = append(msgs, "skill on disk missing from Step 3: assay:"+name)
		}
	}
	for _, name := range sortedKeysBool(listed) {
		if !disk[name] {
			msgs = append(msgs, "Step 3 names a skill not on disk: assay:"+name)
		}
	}
	return msgs
}

// TestSmokeRosterMatchesDisk is the class guard over the committed protocol.
func TestSmokeRosterMatchesDisk(t *testing.T) {
	b, err := os.ReadFile(filepath.FromSlash(smokeProtocolPath))
	if err != nil {
		t.Fatalf("could-not-check: reading %s: %v", smokeProtocolPath, err)
	}
	disk := diskSkillSet(filepath.FromSlash("../../plugins/assay/skills"))
	if len(disk) == 0 {
		t.Fatal("could-not-check: no plugins/assay/skills/*/SKILL.md found")
	}
	for _, m := range smokeRosterViolations(string(b), disk) {
		t.Errorf("%s — update Step 3 of docs/codex-smoke-protocol.md", m)
	}
}

// TestSmokeRosterFlagsPlants is the positive control: planted drift in each
// direction, and a protocol with no Step 3, must all be flagged.
func TestSmokeRosterFlagsPlants(t *testing.T) {
	disk := map[string]bool{"adopt": true, "install": true}
	doc := func(step3 string) string {
		return "# p\n\n#### Step 2 — x\n\n`assay:ghost` outside step 3\n\n#### Step 3 — y\n\n" +
			step3 + "\n\n#### Step 4 — z\n\n`assay:later`\n"
	}
	cases := []struct {
		name, protocol, want string
	}{
		{"clean", doc("`assay:adopt`, `assay:install`"), ""},
		{"missing", doc("`assay:adopt`"), "missing from Step 3: assay:install"},
		{"extra", doc("`assay:adopt`, `assay:install`, `assay:gone`"), "not on disk: assay:gone"},
		{"no step 3", "# p\n\n#### Step 1 — x\n", "no `#### Step 3` section"},
		{"empty list", doc("no tokens here"), "names no `assay:<skill>` tokens"},
	}
	for _, c := range cases {
		got := strings.Join(smokeRosterViolations(c.protocol, disk), "\n")
		if c.want == "" {
			if got != "" {
				t.Errorf("%s: want clean, got %q", c.name, got)
			}
			continue
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: want a violation containing %q, got %q", c.name, c.want, got)
		}
	}
}
