// Package chat is the Ask Assay chat client: an agent loop over deskd's
// READ-ONLY MCP tools plus a subscription model backend (design §7.5 phase 1,
// §13.8).
//
// # What this package is for
//
// Every figure the console renders today was computed by the pane that
// rendered it. This package is the first thing that lets a MODEL narrate onto
// a decision surface, and the moment it does, one sentence of plausible prose
// is all that stands between an operator and a fabricated number. So the whole
// package is built around a single question: where can a rendered digit have
// come from?
//
// The answer is enforced in four independent places, each failing on a
// DIFFERENT signal in a DIFFERENT component:
//
//  1. The NUMBERS RULE, imported — not copied — from the public module at
//     github.com/medici-finance/assay/tools/desk/askassay. An [askassay.Answer]
//     holds its value in an unexported field and renders from its STATE, so a
//     figure with no complete declared source renders `could-not-check`. There
//     is exactly ONE binary of that rule in this house; see figure.go.
//  2. The TOOL BINDING in this file: a closed read-only allow-list with no env
//     var, flag, or exported mutator that can widen it. A fabricated "action"
//     has no tool to land on. Fails at bind time, on a tool-registry signal.
//  3. The RENDER (design §6.6): chat output is escaped text in an
//     html/template, never markup, never a form, never a link that performs an
//     action. Fails in the template layer, on an output-encoding signal.
//  4. The IMPORT GRAPH: this package has no path to internal/custody, which is
//     a COMPILE-TIME impossibility asserted by internal/mcp's `go list -deps`
//     check. The chat cannot answer a gate, adopt, or act as the human
//     identity, because the credential that would let it is not reachable from
//     here.
//
// # What a figure can be built from
//
// A TOOL RESULT, and nothing else. [Client.Ask] carries the model's words to
// [Answer.Text] and builds [Answer.Figures] exclusively from the JSON returned
// by a bound read tool. The model's own numbers are, structurally, just text:
// there is no code path from [Answer.Text] to a [Figure].
package chat

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// boundToolNames is the CLOSED read-only allow-list — the entire set of MCP
// tools the Ask Assay chat may call (design §7.5 phase 1).
//
// It is a fixed-size ARRAY, not a slice, and it is unexported. Both matter:
//
//   - an array cannot be appended to, so no code in this package can widen the
//     binding at run time even by accident;
//   - unexported, with [BoundTools] returning a COPY, means no code OUTSIDE
//     this package can reach the backing storage at all.
//
// There is deliberately no env var, no flag, and no exported mutator that adds
// a rung. Widening this binding is a source edit in a reviewed diff on a
// `gate: human` surface — which is the only place a decision to let a model
// reach a write tool belongs.
//
// The absences are the contract, and they are of two different kinds:
//
//   - desk__escalate and desk__report_file MUTATE the platform. A model that
//     could call them could file as the desk App off its own narration.
//   - desk__claim, desk__team_view and desk__improve_list do not mutate the
//     platform, and are still out: desk__claim takes a coordination LEASE
//     (a write to fleet state by another name), and the other two are simply
//     not needed to answer §7.5's "what's blocked and why". An allow-list that
//     grows to whatever happens to be read-only is not an allow-list.
//   - desk__answer and desk__adopt do not exist on the surface AT ALL
//     (internal/mcp's two deliberate absences). They are named in
//     [forbiddenChatTools] anyway, so that a future registry that grew one
//     could not quietly satisfy this binding.
var boundToolNames = [6]string{
	"desk__inbox_list",
	"desk__item_get",
	"desk__metrics_query",
	"desk__portfolio",
	"desk__roadmap",
	"desk__status",
}

// forbiddenChatTools are names that must NEVER appear in a chat binding, even
// if a future registry advertised them as reads. Keeping them enumerated —
// rather than relying on "they are not in the allow-list" — means the refusal
// message names the actual hazard instead of reporting a generic miss.
var forbiddenChatTools = map[string]string{
	"desk__answer":      "answering an escalation is the human's act, authored by the human identity (design §7.1, §6.2)",
	"desk__adopt":       "adopting a process change is the human's act (design §7.1, §6.2)",
	"desk__escalate":    "files on the platform as the desk App — a model must not be able to file off its own narration",
	"desk__report_file": "writes a classified report on the platform as the desk App",
	"desk__claim":       "takes a coordination lease — a write to fleet state, even though it touches no platform object",
}

// ToolDescriptor is the narrow view of an MCP tool this package needs: its
// name and the two fields that carry internal/mcp's authority contract.
//
// It is declared HERE rather than imported from internal/mcp on purpose. This
// package's import graph is itself one of the four controls (see the package
// doc), and the smaller it is, the less there is to audit. A struct of three
// scalars is a cheaper price than an import edge on a surface whose whole
// claim is what it cannot reach.
type ToolDescriptor struct {
	// Name is the MCP tool name.
	Name string
	// Description is the tool's own advertised description, passed to the
	// model so it can choose. It is DATA, never an instruction this package
	// acts on.
	Description string
	// Mutates is true when calling the tool can write the platform.
	Mutates bool
	// AuthorsAs names the identity any platform write is attributed to. A read
	// tool's is empty; a read tool that names one is a mislabelled write.
	AuthorsAs string
}

// ErrBindingRefused is the class every binding refusal carries, so a caller
// can distinguish "the chat may not run here" from a transport failure.
var ErrBindingRefused = errors.New("chat tool binding refused")

// BoundTools returns the closed allow-list, ID-sorted, as a fresh slice. It
// copies: a caller that appends to the result changes nothing.
func BoundTools() []string {
	out := make([]string, len(boundToolNames))
	copy(out, boundToolNames[:])
	sort.Strings(out)
	return out
}

// Bind resolves the closed allow-list against the tool set a server advertises.
//
// It takes NO tool list from its caller — the names come from
// [boundToolNames] and nowhere else — so there is no argument through which a
// caller could ask for a write tool. What the caller supplies is what the
// SERVER offers, and Bind's job is to refuse if that offer does not match the
// allow-list's terms.
func Bind(available []ToolDescriptor) ([]ToolDescriptor, error) {
	return bindNames(boundToolNames[:], available)
}

// bindNames is Bind's body with the wanted set made an argument, so that a
// test can inject a write tool INTO the binding and prove the refusal fires.
// It is unexported: the injection point exists for the test that proves the
// guard is load-bearing, and for nothing else.
func bindNames(want []string, available []ToolDescriptor) ([]ToolDescriptor, error) {
	byName := make(map[string]ToolDescriptor, len(available))
	for _, t := range available {
		byName[t.Name] = t
	}
	allowed := map[string]bool{}
	for _, n := range boundToolNames {
		allowed[n] = true
	}

	seen := map[string]bool{}
	out := make([]ToolDescriptor, 0, len(want))
	for _, name := range want {
		if why, forbidden := forbiddenChatTools[name]; forbidden {
			return nil, fmt.Errorf("%w: %s is forbidden on this surface — %s", ErrBindingRefused, name, why)
		}
		if !allowed[name] {
			return nil, fmt.Errorf("%w: %s is not on the closed read-only allow-list %v — "+
				"the Ask Assay chat binds these tools and no others, and widening the set is a source edit on a "+
				"human-gated surface, not a configuration", ErrBindingRefused, name, BoundTools())
		}
		if seen[name] {
			return nil, fmt.Errorf("%w: %s is bound twice", ErrBindingRefused, name)
		}
		seen[name] = true

		t, ok := byName[name]
		if !ok {
			// could-not-check is not a pass: a tool the server does not offer
			// is a binding that cannot be satisfied, so the chat does not run
			// with a silently smaller tool set than it believes it has.
			return nil, fmt.Errorf("%w: the server advertises no tool named %s — the binding cannot be satisfied, "+
				"and a chat running with a silently narrower tool set would answer questions it has no source for",
				ErrBindingRefused, name)
		}
		if t.Mutates {
			return nil, fmt.Errorf("%w: %s is advertised as MUTATING — the chat binds read tools only, and a write "+
				"advertised under a read name is exactly the substitution this check exists to catch",
				ErrBindingRefused, name)
		}
		if strings.TrimSpace(t.AuthorsAs) != "" {
			return nil, fmt.Errorf("%w: %s is advertised as authoring as %q — a read tool names no write identity, "+
				"so this is a mislabelled write", ErrBindingRefused, name, t.AuthorsAs)
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
