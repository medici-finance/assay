package forgeban

import (
	"fmt"
	"sort"
	"strings"
)

// check.go — reading a scan against the two registers. Kept out of the test file so a CI
// job, a `go vet`-style driver, or a future `deskban` verb can run the same reconciliation
// the test runs; one implementation, so the gate and the command can never disagree.

// Report is the reconciliation of a scan against AllowedInvocations and UnresolvedArgv.
// Every field is a REASON THE BAN IS NOT PROVEN CLEAN, so an empty Report is the only pass.
type Report struct {
	// Unallowed are forge-CLI invocations with no register entry. The ban proper.
	Unallowed []Finding
	// UnregisteredUnresolved are exec sites with a non-constant argv[0] and no ledger entry.
	// Could-not-check, reported as itself.
	UnregisteredUnresolved []Finding
	// StaleAllowed are permit rows that match nothing in the tree. A permit for a call site
	// that no longer exists is the shape an allowlist rots into: it stops describing the
	// code and starts being a place to park a name.
	StaleAllowed []string
	// StaleUnresolved are ledger rows that match nothing in the tree, for the same reason.
	StaleUnresolved []string
	// Allowed and Unresolved are the matched counts, so a caller can report the ban's
	// current strength rather than only its failures.
	Allowed    int
	Unresolved int
}

// OK reports whether the scan reconciles: nothing unallowed, nothing unregistered, nothing stale.
func (r Report) OK() bool {
	return len(r.Unallowed) == 0 && len(r.UnregisteredUnresolved) == 0 &&
		len(r.StaleAllowed) == 0 && len(r.StaleUnresolved) == 0
}

// Ceiling exposes the ratchet constant so a caller outside this package can report it.
func Ceiling() int { return allowedInvocationCeiling }

// Check reconciles findings against the two registers.
func Check(findings []Finding) Report {
	allowed := index(AllowedInvocations)
	unresolved := index(UnresolvedArgv)

	hitAllowed := map[string]bool{}
	hitUnresolved := map[string]bool{}
	var rep Report

	for _, f := range findings {
		k := f.Key()
		switch f.Kind {
		case KindInvocation:
			if _, ok := allowed[k]; ok {
				hitAllowed[k] = true
				rep.Allowed++
				continue
			}
			rep.Unallowed = append(rep.Unallowed, f)
		case KindUnresolved:
			if _, ok := unresolved[k]; ok {
				hitUnresolved[k] = true
				rep.Unresolved++
				continue
			}
			rep.UnregisteredUnresolved = append(rep.UnregisteredUnresolved, f)
		}
	}

	for k := range allowed {
		if !hitAllowed[k] {
			rep.StaleAllowed = append(rep.StaleAllowed, k)
		}
	}
	for k := range unresolved {
		if !hitUnresolved[k] {
			rep.StaleUnresolved = append(rep.StaleUnresolved, k)
		}
	}
	sort.Strings(rep.StaleAllowed)
	sort.Strings(rep.StaleUnresolved)
	return rep
}

func index(rows []Allowance) map[string]Allowance {
	out := make(map[string]Allowance, len(rows))
	for _, a := range rows {
		out[a.Key] = a
	}
	return out
}

// Explain renders a Report as the diagnosis a failing gate should print: what was found,
// where, and what the reader is expected to do about it.
func (r Report) Explain() string {
	var b strings.Builder
	if len(r.Unallowed) > 0 {
		fmt.Fprintf(&b, "%d forge-CLI invocation(s) with no register entry:\n", len(r.Unallowed))
		for _, f := range r.Unallowed {
			fmt.Fprintf(&b, "  %s:%d — %s invokes %q via %s\n", f.File, f.Line, f.Func, f.Bin, f.Via)
		}
		b.WriteString("  Desk tools reach a forge through the enumerated Forge interface, never a CLI.\n" +
			"  Route the call through a typed op (adding one requires converting its consuming call site in\n" +
			"  the same change — the spec §6 freeze rule), or, if it genuinely cannot move yet, add a row to\n" +
			"  AllowedInvocations naming what has to happen first and raise allowedInvocationCeiling.\n")
	}
	if len(r.UnregisteredUnresolved) > 0 {
		fmt.Fprintf(&b, "%d exec site(s) whose argv[0] is not a compile-time constant and are not in the ledger:\n",
			len(r.UnregisteredUnresolved))
		for _, f := range r.UnregisteredUnresolved {
			fmt.Fprintf(&b, "  %s:%d — %s launches via %s\n", f.File, f.Line, f.Func, f.Via)
		}
		b.WriteString("  This is could-not-check, not a violation: the checker cannot see what these launch.\n" +
			"  Either spell argv[0] as a literal or a package constant so it resolves, or add a row to\n" +
			"  UnresolvedArgv saying what it launches and why that is not a forge CLI.\n")
	}
	if len(r.StaleAllowed) > 0 {
		fmt.Fprintf(&b, "%d permit row(s) match no call site — delete them and lower allowedInvocationCeiling:\n  %s\n",
			len(r.StaleAllowed), strings.Join(r.StaleAllowed, "\n  "))
	}
	if len(r.StaleUnresolved) > 0 {
		fmt.Fprintf(&b, "%d ledger row(s) match no exec site — delete them:\n  %s\n",
			len(r.StaleUnresolved), strings.Join(r.StaleUnresolved, "\n  "))
	}
	return b.String()
}
