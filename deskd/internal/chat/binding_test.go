package chat

import (
	"errors"
	"strings"
	"testing"
)

// fullRegistry mirrors what internal/mcp advertises today (query.go's
// registry()), including the tools the chat must NOT bind. Keeping the write
// tools in the fixture is the point: a binding that is only ever offered read
// tools has not been shown to refuse anything.
func fullRegistry() []ToolDescriptor {
	return []ToolDescriptor{
		{Name: "desk__escalate", Mutates: true, AuthorsAs: "assay-desk-app[bot]"},
		{Name: "desk__report_file", Mutates: true, AuthorsAs: "assay-desk-app[bot]"},
		{Name: "desk__claim"},
		{Name: "desk__inbox_list"},
		{Name: "desk__item_get"},
		{Name: "desk__status"},
		{Name: "desk__metrics_query"},
		{Name: "desk__roadmap"},
		{Name: "desk__portfolio"},
		{Name: "desk__team_view"},
		{Name: "desk__improve_list"},
	}
}

// TestChatBindsOnlyReadTools is Verify row 3. The bound set equals the closed
// allow-list, the four named write/answer tools are absent, and — the subtests
// that make this load-bearing — injecting a write tool into the binding is
// REFUSED.
func TestChatBindsOnlyReadTools(t *testing.T) {
	t.Run("the bound set equals the closed allow-list", func(t *testing.T) {
		bound, err := Bind(fullRegistry())
		if err != nil {
			t.Fatalf("Bind refused the shipped registry: %v", err)
		}
		got := make([]string, 0, len(bound))
		for _, b := range bound {
			got = append(got, b.Name)
		}
		want := BoundTools()
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("bound set is %v, want exactly the allow-list %v", got, want)
		}
	})

	t.Run("no bound tool mutates or names a write identity", func(t *testing.T) {
		bound, err := Bind(fullRegistry())
		if err != nil {
			t.Fatalf("Bind: %v", err)
		}
		for _, b := range bound {
			if b.Mutates {
				t.Fatalf("%s is bound and mutates", b.Name)
			}
			if b.AuthorsAs != "" {
				t.Fatalf("%s is bound and authors as %q", b.Name, b.AuthorsAs)
			}
		}
	})

	// The four names the brief calls out. desk__answer and desk__adopt do not
	// exist on the surface at all; desk__escalate and desk__claim do, and are
	// still absent from the binding.
	t.Run("escalate, claim, answer and adopt are absent", func(t *testing.T) {
		bound, err := Bind(fullRegistry())
		if err != nil {
			t.Fatalf("Bind: %v", err)
		}
		for _, b := range bound {
			switch b.Name {
			case "desk__escalate", "desk__claim", "desk__answer", "desk__adopt":
				t.Fatalf("%s must never be bound to the chat", b.Name)
			}
		}
	})

	// ---- the negative paths: the refusal must fire -------------------------

	// A write tool injected INTO the binding. bindNames is the injection point
	// that exists for exactly this proof.
	//
	// The injected tools are described here EXACTLY AS THE REAL REGISTRY
	// DESCRIBES THEM, and that detail is load-bearing. An earlier version of
	// this subtest injected every one of them with Mutates:true, which made it
	// pass for the wrong reason: the mutation check caught them all, and the
	// allow-list check it was supposed to be pinning was never exercised. The
	// mutation script (mutate-chat-controls.sh) caught that — the allow-list
	// could be deleted outright and this subtest stayed green.
	//
	// So the cases below are split by WHICH refusal must fire:
	//   - desk__escalate / desk__report_file mutate, and are also unlisted;
	//   - desk__claim, desk__team_view and desk__improve_list are honest READ
	//     tools that mutate nothing and author as nobody. NOTHING but the
	//     allow-list can refuse them, so they are what proves it load-bearing.
	//   - desk__answer / desk__adopt are not registered at all.
	t.Run("a write tool injected into the binding is refused", func(t *testing.T) {
		registry := fullRegistry()
		for _, injected := range []string{"desk__escalate", "desk__report_file", "desk__answer", "desk__adopt"} {
			assertBindingRefused(t, injected, append(BoundTools(), injected), append(registry, ToolDescriptor{
				Name: injected, Mutates: true, AuthorsAs: "assay-desk-app[bot]",
			}))
		}
	})

	// The case only the allow-list can catch. If the allow-list check is
	// removed from binding.go, this subtest goes RED on its own — no other
	// refusal in bindNames applies to a genuine read tool.
	t.Run("a non-mutating tool that is merely not allow-listed is refused", func(t *testing.T) {
		registry := fullRegistry()
		for _, injected := range []string{"desk__claim", "desk__team_view", "desk__improve_list", "desk__invented"} {
			for _, d := range registry {
				if d.Name == injected && (d.Mutates || d.AuthorsAs != "") {
					t.Fatalf("%s is not a clean read in the fixture, so it cannot isolate the allow-list check", injected)
				}
			}
			assertBindingRefused(t, injected, append(BoundTools(), injected),
				append(registry, ToolDescriptor{Name: injected})) // no Mutates, no AuthorsAs
		}
	})

	// A write advertised UNDER A READ NAME. This is the substitution the
	// allow-list alone cannot catch — the name is on the list, so only the
	// Mutates check refuses it. Removing that check turns this subtest RED
	// while the one above stays green, which is why both exist.
	t.Run("a mutating tool advertised under an allow-listed name is refused", func(t *testing.T) {
		tampered := fullRegistry()
		for i := range tampered {
			if tampered[i].Name == "desk__metrics_query" {
				tampered[i].Mutates = true
			}
		}
		if _, err := Bind(tampered); err == nil {
			t.Fatal("Bind accepted a MUTATING desk__metrics_query — a write advertised under a read name would be bound")
		}
	})

	t.Run("a read tool naming a write identity is refused", func(t *testing.T) {
		tampered := fullRegistry()
		for i := range tampered {
			if tampered[i].Name == "desk__status" {
				tampered[i].AuthorsAs = "operator"
			}
		}
		if _, err := Bind(tampered); err == nil {
			t.Fatal("Bind accepted a read tool that authors as the human identity")
		}
	})

	t.Run("a binding the server cannot satisfy is refused, not silently narrowed", func(t *testing.T) {
		var narrowed []ToolDescriptor
		for _, d := range fullRegistry() {
			if d.Name == "desk__metrics_query" {
				continue // the server no longer offers the figure source
			}
			narrowed = append(narrowed, d)
		}
		if _, err := Bind(narrowed); err == nil {
			t.Fatal("Bind succeeded with desk__metrics_query missing — a chat running with a silently narrower " +
				"tool set would answer questions it has no source for")
		}
	})
}

// assertBindingRefused asserts bindNames refuses `want`, and that the refusal
// is a real one — carrying the sentinel and naming the offender, rather than
// an unrelated error that happens to be non-nil.
func assertBindingRefused(t *testing.T, injected string, want []string, available []ToolDescriptor) {
	t.Helper()
	got, err := bindNames(want, available)
	if err == nil {
		t.Fatalf("bindNames accepted %s into the binding and returned %d tools — the refusal is not load-bearing",
			injected, len(got))
	}
	if !errors.Is(err, ErrBindingRefused) {
		t.Fatalf("%s: refusal does not carry ErrBindingRefused: %v", injected, err)
	}
	if !strings.Contains(err.Error(), injected) {
		t.Fatalf("%s: refusal does not name the offending tool: %v", injected, err)
	}
}

// TestBoundToolsCannotBeWidenedByACaller pins the "no exported mutator"
// property: BoundTools returns a COPY, so appending to the result changes
// nothing about what Bind will accept.
func TestBoundToolsCannotBeWidenedByACaller(t *testing.T) {
	got := BoundTools()
	got = append(got, "desk__escalate")
	got[0] = "desk__escalate"

	bound, err := Bind(fullRegistry())
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	for _, b := range bound {
		if b.Name == "desk__escalate" {
			t.Fatal("mutating the slice BoundTools returned widened the actual binding")
		}
	}
	if n := len(BoundTools()); n != len(boundToolNames) {
		t.Fatalf("BoundTools now returns %d names, want %d — the backing array was reached", n, len(boundToolNames))
	}
}
