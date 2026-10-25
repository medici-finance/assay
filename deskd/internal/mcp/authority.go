package mcp

import (
	"fmt"
	"sort"
	"strings"
)

// The §11 authority test, as CODE the registry is measured against rather than
// prose a reviewer skims. Design §7.1 names two deliberate absences —
// desk__answer and desk__adopt — and the load-bearing property beneath them:
// "no MCP path can produce a operator-authored mutation." This file is the
// enumeration that proves it for the whole tool set at once, on every build.

// forbiddenTools are the tool names that MUST NOT exist on this surface. Their
// absence is the contract: an agent may file and draft, never answer or adopt
// (design §7.1, §6.2).
var forbiddenTools = []string{"desk__answer", "desk__adopt"}

// AuthorityFinding is one violation of the authority contract.
type AuthorityFinding struct {
	Tool   string
	Reason string
}

// AuthorityResult is the outcome of enumerating a tool set against §11.
type AuthorityResult struct {
	Findings []AuthorityFinding
	// Summary is the human/CI-legible verdict. When there are no findings it
	// is EXACTLY "no human-authored mutation path" — the phrase the Verify row
	// greps for, so the check is a string a machine can assert on.
	Summary string
}

// Clean reports whether the tool set satisfies the authority contract.
func (r AuthorityResult) Clean() bool { return len(r.Findings) == 0 }

// AuthorityReport enumerates every tool and asserts:
//
//   - no forbidden tool (desk__answer / desk__adopt) is present;
//   - no tool authors a platform write as a HUMAN identity ([HumanIdentity()]);
//   - every MUTATING tool authors as [AppIdentity] and nothing else — a
//     mutation with an empty or foreign author is a write nobody signed;
//   - every READ tool authors as nothing — a read that names a write identity
//     is a mislabelled write.
//
// The result carries the phrase "no human-authored mutation path" when clean,
// so the Verify row can assert on output rather than an exit code alone.
func AuthorityReport(tools []Tool) AuthorityResult {
	var findings []AuthorityFinding
	byName := map[string]bool{}
	for _, t := range tools {
		byName[t.Name] = true
	}

	for _, forbidden := range forbiddenTools {
		if byName[forbidden] {
			findings = append(findings, AuthorityFinding{
				Tool:   forbidden,
				Reason: "forbidden tool is present — " + forbidden + " is a deliberate absence (design §7.1): agents file and draft, they do not answer or adopt",
			})
		}
	}

	for _, t := range tools {
		switch {
		case t.AuthorsAs != "" && t.AuthorsAs == HumanIdentity():
			findings = append(findings, AuthorityFinding{
				Tool:   t.Name,
				Reason: fmt.Sprintf("authors as the human identity %q — no MCP tool may produce a %s-authored mutation (§11)", HumanIdentity(), HumanIdentity()),
			})
		case t.Mutates && t.AuthorsAs == "":
			findings = append(findings, AuthorityFinding{
				Tool:   t.Name,
				Reason: "mutates the platform but names no author — a write nobody signed",
			})
		case t.Mutates && t.AuthorsAs != AppIdentity:
			findings = append(findings, AuthorityFinding{
				Tool:   t.Name,
				Reason: fmt.Sprintf("mutates as %q; the only permitted write identity is the service App %q", t.AuthorsAs, AppIdentity),
			})
		case !t.Mutates && t.AuthorsAs != "":
			findings = append(findings, AuthorityFinding{
				Tool:   t.Name,
				Reason: fmt.Sprintf("is a read tool but names a write author %q — a read that claims a write identity is mislabelled", t.AuthorsAs),
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Tool != findings[j].Tool {
			return findings[i].Tool < findings[j].Tool
		}
		return findings[i].Reason < findings[j].Reason
	})

	res := AuthorityResult{Findings: findings}
	if len(findings) == 0 {
		res.Summary = "no human-authored mutation path"
		return res
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d authority violation(s): ", len(findings))
	for i, f := range findings {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s: %s", f.Tool, f.Reason)
	}
	res.Summary = b.String()
	return res
}

// ChatBindingReport extends the §11 authority contract to the Ask Assay chat
// binding (the service contract).
//
// The chat is the first component that lets a MODEL choose which of these
// tools to call, so "the registry is clean" is no longer the whole question:
// what matters additionally is WHICH tools the model was handed. This function
// answers that, by enumerating the bound names against the shipped registry
// and asserting each one:
//
//   - exists on the surface — a binding naming a tool the registry does not
//     have is a binding nobody can audit against anything;
//   - does not MUTATE — a model may not reach a platform write;
//   - authors as NOTHING — a bound tool that names any write identity, App or
//     human, is a mislabelled write on the surface a model can reach;
//   - is not one of the forbidden names, whether or not it is registered.
//
// It takes the bound names as a []string rather than importing the chat
// package, so the production import edge stays mcp → (nothing new). The test
// supplies internal/chat's own BoundTools(), which is what makes the check
// bind the SHIPPED binding rather than a copy of it that could drift.
func ChatBindingReport(bound []string, tools []Tool) AuthorityResult {
	byName := map[string]Tool{}
	for _, t := range tools {
		byName[t.Name] = t
	}
	var findings []AuthorityFinding
	seen := map[string]bool{}
	for _, name := range bound {
		if seen[name] {
			findings = append(findings, AuthorityFinding{Tool: name, Reason: "is bound to the chat twice"})
			continue
		}
		seen[name] = true
		for _, forbidden := range forbiddenTools {
			if name == forbidden {
				findings = append(findings, AuthorityFinding{
					Tool:   name,
					Reason: "is bound to the chat and is a forbidden tool — " + forbidden + " is a deliberate absence (§7.1)",
				})
			}
		}
		t, ok := byName[name]
		if !ok {
			findings = append(findings, AuthorityFinding{
				Tool:   name,
				Reason: "is bound to the chat but is not registered on this surface — the binding cannot be audited against the registry",
			})
			continue
		}
		if t.Mutates {
			findings = append(findings, AuthorityFinding{
				Tool:   name,
				Reason: "is bound to the chat and MUTATES the platform — a model must not be able to reach a write tool (§7.5 phase 1 binds reads only)",
			})
		}
		if t.AuthorsAs != "" {
			findings = append(findings, AuthorityFinding{
				Tool: name,
				Reason: fmt.Sprintf("is bound to the chat and authors as %q — a tool a model can call must author as nobody, "+
					"and one that names the human identity %q would let a narration act as the human", t.AuthorsAs, HumanIdentity()),
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Tool != findings[j].Tool {
			return findings[i].Tool < findings[j].Tool
		}
		return findings[i].Reason < findings[j].Reason
	})

	res := AuthorityResult{Findings: findings}
	if len(findings) == 0 {
		res.Summary = "no model-reachable write path"
		return res
	}
	var cb strings.Builder
	fmt.Fprintf(&cb, "%d chat-binding violation(s): ", len(findings))
	for i, f := range findings {
		if i > 0 {
			cb.WriteString("; ")
		}
		fmt.Fprintf(&cb, "%s: %s", f.Tool, f.Reason)
	}
	res.Summary = cb.String()
	return res
}
