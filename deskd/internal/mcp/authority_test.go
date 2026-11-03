package mcp

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/chat"
)

// custodyPkg is the package holding the operator credential (the human-gate
// OAuth token). Named as a constant so a typo cannot make the check below pass
// vacuously — TestHumanCredentialUnreachable also asserts the write path DOES
// reach it, which would fail on a wrong path.
const custodyPkg = "github.com/medici-finance/assay/deskd/internal/custody"

// TestHumanCredentialUnreachable extends the §11 authority contract to
// the answer flow: the MCP surface — and everything it transitively imports — must
// have NO import path to the custody package, so the token that authors a
// operator write is code-path-unreachable from an agent tool call (design §7.1;
// gate-why: "the token is unreachable from the MCP code path (§11 test)").
//
// The check is on the real transitive import graph via `go list -deps`, not a
// grep, so it catches a leak introduced three imports deep. Its load-bearing-
// ness is guarded by the paired positive assertion: the answer package DOES
// reach custody, which fails if the forbidden path string is wrong.
func TestHumanCredentialUnreachable(t *testing.T) {
	mcpDeps := goListDeps(t, ".")
	if mcpDeps[custodyPkg] {
		t.Fatalf("the MCP surface imports %s — the operator credential must be unreachable from the MCP code path (§11, the answer flow). "+
			"An agent tool call could otherwise reach the human-gate token.", custodyPkg)
	}

	// Positive control: the write path (internal/answer) is where the credential
	// IS reachable. If custodyPkg were misspelled, this assertion would fail and
	// expose the negative check above as vacuous.
	answerDeps := goListDeps(t, "../answer")
	if !answerDeps[custodyPkg] {
		t.Fatalf("internal/answer does not reach %s — either the write path lost its credential or the checked path is wrong, "+
			"which would make the MCP-unreachability check vacuous", custodyPkg)
	}
}

// TestUserCommentPathUnreachableFromCustody extends the §11 authority contract
// to the capability model: the USER comment path (internal/comment) — and everything it
// transitively imports — must have NO import path to the custody package, so a
// commenter-tier viewer can never reach the operator credential. Comment-only is
// therefore a compile-time property (the capability model SPOF layer 2, Verify 1), not a
// runtime check.
//
// The paired positive control is internal/answer's known custody edge, checked
// in TestHumanCredentialUnreachable above — so a misspelt custodyPkg cannot
// make this pass vacuously.
func TestUserCommentPathUnreachableFromCustody(t *testing.T) {
	commentDeps := goListDeps(t, "../comment")
	if commentDeps[custodyPkg] {
		t.Fatalf("internal/comment imports %s — the user comment path must be comment-ONLY and can never reach the operator credential. "+
			"A commenter reaching the answer credential would defeat the whole ladder.", custodyPkg)
	}
	// It also must not reach the answer package (which DOES import custody): a
	// path comment → answer → custody would be a leak one hop removed.
	const answerPkg = "github.com/medici-finance/assay/deskd/internal/answer"
	if commentDeps[answerPkg] {
		t.Fatalf("internal/comment imports %s — the user comment path must not reach the operator answer path (it imports custody)", answerPkg)
	}
}

// deskclientDir is where the web console client host would
// live if it existed on this branch. It does not: the increment-3 audit
// (the authorization contract) named "TestClientBinaryServesNoMCP — go list -deps
// ./cmd/deskclient contains no internal/mcp" as owed, but the SAME audit also
// records task 5 as superseded, pending a re-home ruling
// (the authorization contract, needs-decision) — the human front end is now the
// Electron app (the service contract+), and cmd/deskclient is explicitly NOT
// being written until that fork resolves. Those two facts about ONE
// increment conflict on their face, so this test is written to be honest
// about BOTH rather than fabricating the excluded binary to force a pass.
const deskclientDir = "../../cmd/deskclient"

// TestClientBinaryServesNoMCP is the per-binary §11 assertion for the web
// console client host (facts: "go list -deps ./cmd/deskclient contains no
// internal/mcp" — the agent surface must be unreachable from whatever binary
// serves a human's browser, exactly as [TestHumanCredentialUnreachable]
// keeps the custody credential unreachable from the agent surface's own
// import graph, in the other direction).
//
// cmd/deskclient does not exist on this branch (task 5 is superseded/pending
// re-home, the authorization contract) — building it just to satisfy this test would
// violate that explicit scope boundary. So this test is honest about its own
// applicability: it SKIPS, naming the tracked decision, for as long as the
// binary is absent, and asserts the real invariant the moment a re-home
// lands it — zero further edits needed, and CI can never silently forget to
// re-enable this check because the assertion is already here, gated on a
// fact (the directory's existence) rather than on a human remembering to
// come back to a TODO.
func TestClientBinaryServesNoMCP(t *testing.T) {
	if _, err := os.Stat(deskclientDir); os.IsNotExist(err) {
		t.Skipf("could-not-check: %s does not exist on this branch — task 5 (the web console client host) is superseded, "+
			"pending the re-home ruling on the authorization contract; this assertion activates the moment that binary lands", deskclientDir)
	}
	deps := goListDeps(t, deskclientDir)
	const mcpPkg = "github.com/medici-finance/assay/deskd/internal/mcp"
	if deps[mcpPkg] {
		t.Fatalf("cmd/deskclient imports %s — the client binary serving a human's browser must not reach the agent surface (§11)", mcpPkg)
	}
}

// goListDeps returns the transitive dependency set of the package in dir.
func goListDeps(t *testing.T, dir string) map[string]bool {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps in %s failed: %v\n%s", dir, err, out)
	}
	deps := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		deps[strings.TrimSpace(line)] = true
	}
	return deps
}

// TestAuthority is the §11 authority test (Verify row 1): enumerate every tool
// and assert none can author as a human identity. The output carries the exact
// phrase the Verify row greps for.
func TestAuthority(t *testing.T) {
	res := AuthorityReport(registry())
	if !res.Clean() {
		t.Fatalf("the shipped tool set violates the authority contract: %s", res.Summary)
	}

	// The phrase is load-bearing: the Verify row asserts on it, running
	// `go test` WITHOUT -v — so it must reach stdout unconditionally, not via
	// t.Log (which prints only under -v or on failure).
	const phrase = "no human-authored mutation path"
	if !strings.Contains(res.Summary, phrase) {
		t.Fatalf("authority summary %q does not contain %q", res.Summary, phrase)
	}
	fmt.Println("authority (§11):", res.Summary)

	// Every mutating tool authors as the App and nothing else; every read tool
	// authors as nothing. Stated per-tool so a regression names the offender.
	for _, tool := range registry() {
		switch {
		case tool.AuthorsAs == HumanIdentity():
			t.Fatalf("%s authors as the human identity %q", tool.Name, HumanIdentity())
		case tool.Mutates && tool.AuthorsAs != AppIdentity:
			t.Fatalf("%s mutates but does not author as the App (%q)", tool.Name, tool.AuthorsAs)
		case !tool.Mutates && tool.AuthorsAs != "":
			t.Fatalf("%s is a read tool but names write author %q", tool.Name, tool.AuthorsAs)
		}
	}

	// The two deliberate absences are absent.
	for _, name := range []string{"desk__answer", "desk__adopt"} {
		if _, err := (&Server{tools: toolMap(registry())}).lookup(name); err == nil {
			t.Fatalf("%s must not exist on this surface", name)
		}
	}

	// ---- the chat binding (the service contract, Verify row 6) ---------------
	//
	// The chat is the first component that hands a MODEL a choice of these
	// tools, so the authority contract now also covers WHICH tools it was
	// handed. The bound set comes from internal/chat's own BoundTools() — the
	// shipped binding itself, not a copy of it that could drift.
	chatRes := ChatBindingReport(chat.BoundTools(), registry())
	if !chatRes.Clean() {
		t.Fatalf("the chat's bound tool set violates the authority contract: %s", chatRes.Summary)
	}
	fmt.Println("chat binding (§11 / the service contract):", chatRes.Summary, "—", chat.BoundTools())

	// internal/chat has no import path to internal/custody. This is a
	// COMPILE-TIME impossibility, not a runtime check: the credential that
	// authors a operator write is not reachable from the package a model drives.
	// The paired positive control below keeps it from passing vacuously.
	chatDeps := goListDeps(t, "../chat")
	if chatDeps[custodyPkg] {
		t.Fatalf("internal/chat imports %s — the human-gate credential must be unreachable from the package a MODEL "+
			"drives (the service contract). A chat that could reach it could answer a gate off its own narration.", custodyPkg)
	}
	const answerPkg = "github.com/medici-finance/assay/deskd/internal/answer"
	if chatDeps[answerPkg] {
		t.Fatalf("internal/chat imports %s — the answer path imports custody, so this would be the same leak one hop removed", answerPkg)
	}

	// Positive control, restated here so THIS assertion is proven capable of
	// failing on its own rather than borrowing another test's proof: the
	// answer path DOES reach custody. If custodyPkg were misspelled, the
	// negative check above would be vacuous and this would fail.
	if !goListDeps(t, "../answer")[custodyPkg] {
		t.Fatalf("internal/answer does not reach %s — the checked path is wrong, which would make the internal/chat "+
			"unreachability assertion vacuous", custodyPkg)
	}
}

// TestChatBindingReportCatchesAWriteToolInTheBinding is the positive control
// for ChatBindingReport. A binding that DID hand a model a write tool must
// fail the report — a check that cannot fail is not load-bearing.
func TestChatBindingReportCatchesAWriteToolInTheBinding(t *testing.T) {
	cases := map[string][]string{
		"a mutating tool":      append(chat.BoundTools(), "desk__escalate"),
		"a forbidden tool":     append(chat.BoundTools(), "desk__answer"),
		"an unregistered tool": append(chat.BoundTools(), "desk__invented"),
	}
	for name, bound := range cases {
		res := ChatBindingReport(bound, registry())
		if res.Clean() {
			t.Fatalf("%s: ChatBindingReport passed %v — the check is not load-bearing", name, bound)
		}
	}

	// And a read tool that names a write identity, which the mutation flag
	// alone would not catch.
	tampered := registry()
	for i := range tampered {
		if tampered[i].Name == "desk__status" {
			tampered[i].AuthorsAs = HumanIdentity()
		}
	}
	if res := ChatBindingReport(chat.BoundTools(), tampered); res.Clean() {
		t.Fatal("ChatBindingReport passed a bound read tool authoring as the human identity")
	}
}

// TestAuthorityCatchesAHumanAuthoredTool is the positive control: a tool set
// that DID contain a human-authored mutation must fail the report. A check that
// cannot fail is not load-bearing.
func TestAuthorityCatchesAHumanAuthoredTool(t *testing.T) {
	tampered := append(registry(), Tool{
		Name:      "desk__answer", // both a forbidden name AND a human author
		Mutates:   true,
		AuthorsAs: HumanIdentity(),
	})
	res := AuthorityReport(tampered)
	if res.Clean() {
		t.Fatalf("authority report passed a human-authored desk__answer — the check is not load-bearing")
	}
	if !strings.Contains(res.Summary, "desk__answer") {
		t.Fatalf("summary should name the offending tool: %s", res.Summary)
	}
}

// toolMap / lookup are tiny helpers so the absence assertion can probe the
// registry the way Dispatch does.
func toolMap(tools []Tool) map[string]Tool {
	m := map[string]Tool{}
	for _, t := range tools {
		m[t.Name] = t
	}
	return m
}

func (s *Server) lookup(name string) (Tool, error) {
	t, ok := s.tools[name]
	if !ok {
		return Tool{}, &ErrUnknownTool{Name: name}
	}
	return t, nil
}
