package main

// kits.go — the prompt-kit reader.
//
// WHY THE KITS ARE EMBEDDED. Each kit under references/ carries the clauses a dispatched
// agent must receive VERBATIM — the isolation floor, the no-evasion rule, the
// security-gate refusal, the offline envelope, the body-file rule, the Evidence format.
// Every one of them is a rule that has already failed in the field; the wording IS the
// fix. If the binary read them off disk at run time, a machine with a stale checkout would
// hand its agents a stale rule and nothing would say so. Embedding makes the kit a
// property of the BINARY, so `deskdispatch --version` and the kit text move together and
// a fleet on one pinned release is a fleet on one set of clauses.
//
// PUBLIC-TREE RULE. These files ship in a public tree. Every clause is written GENERIC:
// no private repository name, no issue or PR reference, no internal document path, no
// stream or item identifier, no personal name. Where a house rule is inseparable from
// private context, the kit carries the generic statement of the rule and the private
// detail stays where it came from. A kit that names a private artifact publishes a map to
// it, so kittext_test.go enforces the property mechanically rather than leaving it to a
// reviewer's eye.

import (
	"embed"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

//go:embed references/*.md
var kitFS embed.FS

// commonKitPath holds the clauses EVERY dispatched agent receives, whatever its class.
//
// It is a separate file, emitted on every dispatch AHEAD of the class kit, so there is
// exactly ONE wording of the isolation floor, the no-evasion rule and the offline envelope
// rather than three copies in three kits that drift apart. It is deliberately NOT
// selectable as a --kit value: it is not an agent class, and a dispatch that emitted only
// the common clauses would be one with no class instructions at all.
const commonKitPath = "references/common-clauses.md"

// kitFile maps the --kit value to its CLASS file. The vocabulary is CLOSED: a kit name the
// binary does not carry is refused rather than silently producing a prompt with no
// clauses in it — an agent dispatched without the clauses is exactly the failure the kits
// exist to prevent, and it would look like a successful dispatch.
var kitFile = map[string]string{
	"worker":   "references/worker-prompt.md",
	"review":   "references/review-prompt.md",
	"verifier": "references/verifier-prompt.md",
}

func kitNames() []string {
	out := make([]string, 0, len(kitFile))
	for k := range kitFile {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func joinKits() string { return strings.Join(kitNames(), ", ") }

// kitText returns one kit's verbatim text. A missing embedded file is UNVERIFIABLE, not a
// silently empty kit: the binary was built without a clause set it claims to carry, and
// that is a build defect the caller must see rather than route around.
func kitText(name string) (string, error) {
	path, ok := kitFile[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return "", deskkit.Refused("unknown --kit " + name + " (want one of: " + joinKits() + ")")
	}
	return readKit(name, path)
}

// commonKitText returns the common clauses. Its absence is UNVERIFIABLE for the same
// reason a class kit's is, and more so: a dispatch missing the common clauses is a
// dispatch missing the isolation floor every other clause rests on.
func commonKitText() (string, error) { return readKit("common", commonKitPath) }

func readKit(name, path string) (string, error) {
	b, err := kitFS.ReadFile(path)
	if err != nil {
		return "", deskkit.Unverifiable("kit "+name+" is not embedded in this binary — it was built "+
			"without the clause set it advertises; do not dispatch on it", err)
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		return "", deskkit.Unverifiable("kit "+name+" is EMPTY — an agent dispatched with an empty "+
			"clause set looks like a successful dispatch and is not one", nil)
	}
	return text, nil
}
