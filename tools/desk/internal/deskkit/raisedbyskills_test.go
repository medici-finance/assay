package deskkit

// raisedbyskills_test.go — the DIFF half of derive-or-diff for the `raised-by:<role>`
// vocabulary.
//
// The roster's role-bindings are the ONE declared source (raisedby.go). The desk SKILL.md
// files are copies: each one names its own role in prose, in a `--raised-by <role>`
// instruction a human wrote. Prose copies of a declared set drift — that is the whole
// premise of the rule — and this drift is the silent kind, because a skill naming a role
// nobody bound produces no error until an agent runs it and `deskfile` refuses at exit 5,
// mid-filing, with the finding already in hand.
//
// So the copies get DIFFED against the source. The skills stay hand-written (which desk
// files where is a judgement about the process, not a mechanical consequence of a roster
// entry) but a skill that contradicts the roster is now visible instead of silent — the
// same shape as statusgen/mergedstatus.go's reconciliation of status cells against merges.
//
// A NAMING TRAP THIS ALSO CLOSES. A sibling brief in this tree had to rename a decision
// class and its GitHub label away from a person's handle and given name, because the
// tree-sweep blocks personal identifiers on a push. A `raised-by:` label is published on
// every issue it touches, so the same trap is live here — and the roster's role names are
// function names by construction. Constraining the skills to that vocabulary is what keeps
// a person's name out of a published label.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// skillRaisedByRe matches the `--raised-by <role>` instruction as the skills write it.
// The role half is deliberately permissive (any run of non-space, non-backtick characters)
// so a MISSPELLED role is captured and then fails the vocabulary check, rather than
// failing to match and passing silently.
var skillRaisedByRe = regexp.MustCompile(`--raised-by[\s\n]+([^\s` + "`" + `]+)`)

// skillsDir is the repo-relative skills root, from this package's directory.
const skillsDir = "../../../../.claude/skills"

// TestSkillsRaisedByVocabularyMatchesTheRoster is the diff.
func TestSkillsRaisedByVocabularyMatchesTheRoster(t *testing.T) {
	skipIfFixtureAbsent(t, skillsDir,
		".claude/ is not part of this repository's published file set; no desk skills ship")
	bound := map[string]bool{}
	for _, r := range RaisedByRoles() {
		bound[r] = true
	}
	if len(bound) == 0 {
		t.Fatal("the fixture roster binds no roles — this check would pass vacuously")
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", skillsDir, err)
	}

	found := map[string][]string{} // role -> skills naming it
	scanned := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			continue // not every skill directory carries a SKILL.md
		}
		scanned++
		for _, m := range skillRaisedByRe.FindAllStringSubmatch(string(raw), -1) {
			role := strings.ToLower(strings.Trim(m[1], "\"'.,;:"))
			// `--raised-by <role>` is the placeholder form used in explanatory prose,
			// not a claim about a specific desk.
			if role == "<role>" || role == "" {
				continue
			}
			found[role] = append(found[role], e.Name())
		}
	}
	if scanned == 0 {
		t.Fatalf("scanned 0 SKILL.md files under %s — this check proved nothing", skillsDir)
	}

	// THE DIFF. Every role a skill names must be one the roster binds.
	var roles []string
	for r := range found {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	for _, role := range roles {
		if bound[role] {
			continue
		}
		sort.Strings(found[role])
		t.Errorf("SKILL.md %v instructs `--raised-by %s`, which is NOT a role the roster binds "+
			"(bound: %s).\nThe label vocabulary is DERIVED from the roster's role-bindings — the "+
			"`role=` prefixes on %s. A skill naming an unbound role fails at exit 5 inside "+
			"`deskfile new`, mid-filing, with the finding already in hand. Fix the skill, or bind "+
			"the role.", found[role], role, strings.Join(RaisedByRoles(), ", "), EnvTrustedBotSlugs)
	}

	// NOT VACUOUS. If every stamping instruction were deleted from every skill the loop
	// above would pass over an empty set, and the guard would report clean on a fleet that
	// stamps nothing.
	if len(roles) == 0 {
		t.Fatalf("no SKILL.md under %s carries a `--raised-by <role>` instruction. Either the "+
			"stamping convention was removed from every filing desk — in which case the by-desk "+
			"issue metric now reads UNKNOWN for everything — or this scanner stopped matching the "+
			"form the skills use. Both are findings; neither is clean.", skillsDir)
	}
}

// TestSkillsRaisedByRolesCarryNoPersonalIdentifier — the same trap raisedby.go's own test
// covers, applied to the copies. A published label must be role-shaped: lowercase letters,
// digits and dashes. A handle, a given name or an id cannot survive that shape.
func TestSkillsRaisedByRolesCarryNoPersonalIdentifier(t *testing.T) {
	skipIfFixtureAbsent(t, skillsDir,
		".claude/ is not part of this repository's published file set; no desk skills ship")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", skillsDir, err)
	}
	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(skillsDir, e.Name(), "SKILL.md"))
		if rerr != nil {
			continue
		}
		for _, m := range skillRaisedByRe.FindAllStringSubmatch(string(raw), -1) {
			role := strings.ToLower(strings.Trim(m[1], "\"'.,;:"))
			if role == "<role>" || role == "" {
				continue
			}
			checked++
			for _, r := range role {
				if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
					t.Errorf("%s/SKILL.md names role %q, which carries %q. Role labels are "+
						"lowercase/dash only, so a person's handle or given name cannot reach a "+
						"published `raised-by:` label", e.Name(), role, string(r))
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no concrete `--raised-by <role>` instruction was found to check")
	}
}
