package main

// Verify row CLASSES (verdict-lane/02).
//
// Every Verify row is authored as prose+shell and re-interpreted at run time.
// The verdict lane (R-6) needs the common case to be DETERMINISTIC — a machine
// re-executes it, a human trusts the run — so each row now declares its class at
// brief-author time, in an optional `Class` column:
//
//	| # | Class | Command | Expect |
//
// Four classes, two kinds:
//
//	check:ci     HERMETIC. Tree-only, no environment beyond the checkout. The
//	             lane RE-EXECUTES the row network-off against the candidate tree
//	             and refuses the whole verdict on mismatch (rulings.md R-6 c.6).
//	             Hermeticity is ENFORCED at execution (verifyrun disables the
//	             network), never merely declared.
//	check        DETERMINISTIC but ENV-BOUND — needs a live PEM, a real queue, a
//	             tool on PATH. A runner executes it; CI does not. It rests on the
//	             verdict's authorship+signature (R-6 c.1–3), not on re-execution.
//	check:cluster A SUBCLASS of `check` (verdict-lane/07): env-bound to a LIVE
//	             cluster, whose runner must be the privileged pod runner. The
//	             OFFLINE lane cannot execute it — it records could-not-check with a
//	             stable, greppable marker naming the probe, and the brief is
//	             code-verified/cluster-pending until the pod lane completes it.
//	gate:model   JUDGMENT — a model reads the row and decides.
//	gate:human   JUDGMENT — a human reads the row and decides; stays on the
//	             verify-gate issue pair, outside the transcription lane.
//
// THE LEGACY DEFAULT is the compatibility hinge: a Verify table with NO `Class`
// column is legacy, and every one of its rows is treated as `check`
// (runner-executed). Nothing in the inherited corpus breaks on day one — the
// 300-plus existing tables keep the exact behaviour they had before the column
// existed, and only a table that OPTS IN by adding the column gains the new
// routing.
//
// SCRIPT CONVENTION. A hermetic (`check:ci`) or env-bound (`check`) row may be a
// reviewed SCRIPT rather than an inline command:
//
//	docs/streams/<stream>/verify.d/brief-NN/row-K.sh   (executable, exit 0 = PASS)
//
// and then the row's Command cell IS that script path. The reviewer who approves
// the brief approves the script — the trust decision lives where it already is.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
)

// The row classes. Stable identifiers — the lint, verifyrun and the
// verdict lane all select on these exact strings.
const (
	classCheckCI      = "check:ci"      // hermetic, tree-only, CI re-executes network-off
	classCheck        = "check"         // deterministic but env-bound — runner-executed
	classCheckCluster = "check:cluster" // env-bound to a live cluster — the pod runner only (verdict-lane/07)
	classGateModel    = "gate:model"    // judgment: a model decides
	classGateHuman    = "gate:human"    // judgment: a human decides (verify-gate)
)

// classCheckCluster is a fifth class, a strict SUBCLASS of `check` (verdict-lane/07,
// the pod/online verify lane). A cluster row is deterministic but env-bound to a
// LIVE cluster — a probe run against a real participant/validator — whose runner
// MUST be the privileged pod runner; the OFFLINE verify lane holds no cluster
// access and can never execute it. Two facts flow from that:
//
//   - the offline lane records a cluster row could-not-check with a stable,
//     greppable marker (clusterPendingMarker) naming the probe, distinct from an
//     ordinary env-bound `check` skip — so "parked awaiting the pod" is
//     distinguishable from "skipped, will run later in CI"; and
//   - a brief whose only unrun Verify rows are cluster rows is code-verified but
//     cluster-pending — the pod runner's worklist, derived by --cluster-pending-queue.
//
// The probe a cluster row names is validated against the documented-probe
// registry (docs/streams/.pod-probes): an unknown probe routes the row at a
// script no pod runner provides, so it is a lint PROBLEM, never a silent pass.

// legacyRowClass is what a row resolves to when the table declares no `Class`
// column (and what an explicit-but-empty cell falls back to): `check`,
// runner-executed. This is the single fact that makes the whole column additive
// — see the file header.
const legacyRowClass = classCheck

// knownRowClasses is the closed set the lint validates against. A class outside
// it is a PROBLEM (a typo, or a class nobody has defined), never a silent pass.
var knownRowClasses = map[string]bool{
	classCheckCI:      true,
	classCheck:        true,
	classCheckCluster: true,
	classGateModel:    true,
	classGateHuman:    true,
}

// ---------------------------------------------------------------------------
// Obligation classes (mistake-proofing/03, spec §4 B2 + §3 D7)
// ---------------------------------------------------------------------------
//
// AXIS DECISION — recorded here, in source, before the code, because it is the
// central design call this brief makes.
//
// The five execution classes above answer ONE question: WHO EXECUTES a row (a
// hermetic re-run, a runner, the pod runner, a model, a human). The four classes
// below answer a DIFFERENT, orthogonal question: WHAT OBLIGATION the row
// discharges — a mutation proves a guard reddens, a flow crosses a component
// boundary, a dereference resolves a claim, a neighbour exercises an untouched
// sibling. The two axes are independent: a mutation row is still executed by
// someone. Collapsing them into ONE closed set — adding `mutation` to
// knownRowClasses — would force every author to choose between naming WHO runs a
// row and naming WHAT it discharges, and would silently exempt every
// obligation-classed row from execution routing (its class() would resolve to an
// unknown execution value and route nowhere). So the obligation is a SECOND,
// independent closed set, carried on the SAME cell by a compound encoding.
//
// ENCODING — a COMPOUND CELL, chosen over a second table column. The `Class`
// cell carries an execution value, then zero or more `+`-prefixed obligation
// tokens, whitespace-separated. A brief-shaped worked example:
//
//	| # | Class                   | Command                          | Expect |
//	|---|-------------------------|----------------------------------|--------|
//	| 1 | check:ci +mutation      | `go test ./x -run TestGuard`     | exit 0; reddens on the mutation entry |
//	| 2 | check +flow             | docs/streams/x/verify.d/b-1/f.sh | exit 0 |
//	| 3 | gate:model +dereference | prose — the named symbol resolves | the claim resolves against the tree |
//
// Why a compound cell and NOT a second column: the execution column is OPTIONAL
// and its ABSENCE is the legacy-default hinge (a table with no `Class` column is
// legacy `check`). A second obligation column would add a THIRD table shape
// (no-class / class-only / class+obligation) and force verifyRowTable to grow a
// second optional-column search — multiplying the ways the legacy hinge could
// shift. The compound cell keeps the table's column set byte-identical to
// today's: the whole obligation axis lives INSIDE the one optional `Class` cell,
// so a column-less legacy table is untouched, an execution-only cell parses
// exactly as before, and only a cell that opts in by writing a `+token` gains an
// obligation. briefdoc.SplitRowClassCell is the single seam that separates the two axes.
//
// PRESENCE, NOT ADEQUACY (spec §3 D7). This encoding, and the derivation that
// reads it (obligationderivation.go), carry only whether an obligation ROW IS
// PRESENT. Whether a mutation row actually reddens a guard, whether a flow row
// actually crosses a boundary, whether a dereference row actually dereferences —
// that ADEQUACY is not decidable from row text and stays the reviewer's call.
// Presence is the control; adequacy is review. Every message this surface emits
// says which half it covers.
//
// NEIGHBOUR IS VALIDATED BUT NOT DERIVED. The `+neighbour` token is a defined,
// closed-set obligation — an author may write it and it validates — but no
// honest path-only signal distinguishes "a sibling the change did not touch"
// from "any of the many files this change happens not to touch", so its
// derivation TRIGGER is deferred to a follow-up (task 3 explicitly sanctions
// this). mutation, flow and dereference are derived; neighbour is carried.
const (
	classMutation    = "mutation"    // breaks the guarded thing and proves the guard reddens (spec D1)
	classDereference = "dereference" // resolves a claim rather than counting its presence
	classNeighbour   = "neighbour"   // exercises a sibling site the change did not touch (validated, not yet derived)
)

// knownObligations is the closed obligation set — the SECOND axis, deliberately
// separate from knownRowClasses. An obligation token outside it is a hard
// PROBLEM, exactly as an unknown execution class is: an unrecognised token
// routes nowhere. Kept unknown-fatal so a typo'd `+mutaton` cannot pass as a
// silently-absent obligation.
var knownObligations = map[string]bool{
	classMutation:    true,
	classFlow:        true,
	classDereference: true,
	classNeighbour:   true,
}

// obligationSeparator prefixes each obligation token in a compound Class cell
// (`check:ci +mutation`), distinguishing it from the single execution value.
const obligationSeparator = "+"

// obligations returns the KNOWN obligation tokens a row's Class cell carries.
// nil for a legacy (unclassed) table, an execution-only cell, or a cell whose
// only obligation tokens are unrecognised — those are caught as PROBLEMs by
// verifyRowClassProblems; here we report only what a consumer can route on.
func (r verifyRowCells) obligations() []string {
	if !r.Classed {
		return nil
	}
	_, obs := briefdoc.SplitRowClassCell(r.Class)
	var known []string
	for _, o := range obs {
		if knownObligations[o] {
			known = append(known, o)
		}
	}
	return known
}

// verifyRowCells is one Verify-table data row, as located by verifyRowTable.
//
// Class is the RAW cell text (markdown intact); resolveRowClass normalises it.
// Classed records whether the TABLE carried a `Class` column at all — the one
// bit that separates an explicitly-declared `check` (env-bound, skipped in CI)
// from a legacy-default `check` (the inherited corpus, which CI still
// re-executes). Collapsing the two would silently exempt every legacy row from
// the scheduled main-rerun, which is the opposite of what the column is for.
type verifyRowCells struct {
	Num     string
	Class   string
	Command string
	Expect  string
	Classed bool
	// Shell is the RAW `Shell` cell text (issue #1424): the per-row shell a
	// Verify row is executed under. Empty — or a table with no `Shell` column —
	// resolves to the default `sh` (POSIX bash), which is byte-for-byte today's
	// behaviour for the whole inherited corpus. resolveRowShell normalises it.
	Shell string
	// Env is the RAW `Env` cell text (issue #6065): WHERE the row runs — the
	// checkout alone, the witness's host, a named non-production service
	// instance, or a live cluster. EnvCol records whether the TABLE carried an
	// `Env` column at all; the environment lint reads a table only when it has
	// opted in, so the inherited corpus is untouched. resolveRowEnv normalises it.
	Env    string
	EnvCol bool
}

// resolveRowClass normalises a Class cell to one of the class constants and
// reports whether it names a KNOWN class.
//
// An empty cell — or a row from a table with no Class column — resolves to the
// legacy default and is known. A non-empty cell that matches no class is
// returned verbatim (lowercased) with known=false, so the lint can name exactly
// what the author wrote.
func resolveRowClass(raw string) (class string, known bool) {
	s := strings.ToLower(strings.TrimSpace(briefdoc.NormalizeRowID(raw))) // strip backticks/emphasis, trim
	if s == "" {
		return legacyRowClass, true
	}
	return s, knownRowClasses[s]
}

// class returns the resolved class for a parsed row: the declared class when the
// table opted into the column, else the legacy default. It does NOT report
// known-ness — the lint (verifyRowClassProblems) is the sole place an unknown
// class is judged; every other consumer wants a class to route on.
func (r verifyRowCells) class() string {
	if !r.Classed {
		return legacyRowClass
	}
	// A compound cell (`check +mutation`) routes on its EXECUTION token only; the
	// obligation tokens are a second axis every execution consumer ignores.
	execCell, _ := briefdoc.SplitRowClassCell(r.Class)
	if execCell == "" {
		return legacyRowClass
	}
	c, _ := resolveRowClass(execCell)
	return c
}

// ---------------------------------------------------------------------------
// Verify row SHELLS (issue #1424)
// ---------------------------------------------------------------------------
//
// A Verify row is executed IN a shell, and which shell is a PER-ROW property the
// author declares — never a heuristic guessed from the command text. Sniffing a
// row for `findstr`, `cmd`-specific flags or backslash paths to pick a shell is
// exactly the guess this design forbids: a row that happens to mention `findstr`
// in a comment, or a bash row that uses a backslash, would misroute.
//
// The default — and the resolved value for EVERY inherited row — is `sh` (POSIX
// bash). Some rows are authored in NATIVE-Windows syntax, e.g.
//
//	findstr /c:"…" docs\streams\<stream>\spec.md
//
// which means what the author intended only under `cmd.exe`: run under bash the
// `*`, the quotes and the backslash path are interpreted by bash BEFORE the
// Windows tool ever sees them, so the row fails with a bash-level error while the
// identical row passes under `cmd /c`. That is a WRONG-SHELL row, not a failing
// check. The author marks such a row's shell in an optional `Shell` column:
//
//	| # | Shell | Command | Expect |
//
// Three shells, DEFAULT `sh`:
//
//	sh    POSIX bash — the default when the column is absent OR the cell is empty.
//	      Every inherited row is `sh`; its behaviour is byte-for-byte unchanged.
//	cmd   Windows cmd.exe (`cmd /d /s /c <command>`). could-not-run off Windows.
//	pwsh  PowerShell (`powershell -NoProfile -Command <command>`). could-not-run
//	      off Windows.
//
// A row marked for a shell the runner's OS does not provide (a `cmd`/`pwsh` row
// on Linux/macOS) is could-not-run WITH the reason — never `fail`, and never a
// silent rewrite of the command into another shell's syntax. See verifyrun.go's
// shellDispatch. The `Shell` column is INDEPENDENT of the `Class` column: a
// legacy table (no `Class` column) may still carry a `Shell` column, and vice
// versa.

// The row shells. Stable identifiers — verifyrun dispatch and the lint select on
// these exact strings.
const (
	rowShellCmd  = "cmd"  // Windows cmd.exe
	rowShellPwsh = "pwsh" // PowerShell
)

// legacyRowShell is what a row resolves to when no `Shell` column is present, or
// its cell is empty: `sh`. This is the single fact that makes the column additive
// — an unmarked corpus runs exactly as it did before the column existed.
const legacyRowShell = rowShellSh

// knownRowShells is the closed set the lint validates against. A `Shell` cell
// outside it (a typo like `bash` or `powershell`, or `shell`) is a PROBLEM — a
// shell the tool cannot resolve would route nowhere — never a silent default.
var knownRowShells = map[string]bool{
	rowShellSh:   true,
	rowShellCmd:  true,
	rowShellPwsh: true,
}

// resolveRowShell normalises a `Shell` cell to one of the shell constants and
// reports whether it names a KNOWN shell. An empty cell — or a row from a table
// with no `Shell` column — resolves to the legacy default `sh` and is known. A
// non-empty cell that matches no shell is returned verbatim (lowercased) with
// known=false, so the lint can name exactly what the author wrote.
func resolveRowShell(raw string) (shell string, known bool) {
	s := strings.ToLower(strings.TrimSpace(briefdoc.NormalizeRowID(raw))) // strip backticks/emphasis, trim
	if s == "" {
		return legacyRowShell, true
	}
	return s, knownRowShells[s]
}

// shell returns the resolved shell for a parsed row: the declared shell when the
// table opted into the `Shell` column, else the legacy default `sh`. Like
// class(), it does NOT report known-ness — the lint (verifyRowClassProblems) is
// the sole place an unknown shell is judged; every other consumer wants a shell
// to route on.
func (r verifyRowCells) shell() string {
	s, _ := resolveRowShell(r.Shell)
	return s
}

// ---------------------------------------------------------------------------
// Verify row ENVIRONMENTS (issue #6065)
// ---------------------------------------------------------------------------
//
// The class says WHO executes a row; nothing said WHERE. A row that needs a live
// database or a project credential could be classed `check:ci` — which CI
// re-executes network-off on the checkout alone, so it can only fail or be
// faked — and an env-bound `check` row could name no environment at all, which
// leaves the non-implementer who must witness it with nowhere to run it. The
// author then reports their own run, and the reviewer is right to refuse it.
//
// So a row may state its environment in an optional `Env` column, located by
// header NAME like `Class` and `Shell`:
//
//	| # | Class | Env | Command | Expect |
//
// A CLOSED grammar, lower-cased, backticks stripped:
//
//	tree             the checkout alone, network-off — what CI re-executes.
//	host[:<name>]    the witness's own machine: installed tools, local config.
//	service:<name>   a NAMED NON-PRODUCTION service instance — a database, an
//	                 API, a staging deployment. <name> is the identifier the
//	                 brief's Context (or its spec) describes; never production.
//	cluster:<name>   a live cluster, run by the pod runner (check:cluster).
//
// <name> is lower-case letters, digits, `.`, `_` and `-`. An empty cell names no
// environment. The column is ADDITIVE: a table without it lints exactly as it
// did before, and every consumer other than the lint ignores it.
//
// The rules (lint arm (d) in verifyRowClassProblems), all in a table WITH the
// column:
//
//   - an Env cell outside the grammar is a PROBLEM — it routes nowhere;
//   - a `check:ci` row naming anything but `tree` is a PROBLEM — CI reaches no
//     host, service or cluster;
//   - a row naming a non-tree environment in a table with NO `Class` column is
//     a PROBLEM — an unclassed row is legacy `check`, which the scheduled main
//     re-run still executes in CI;
//   - an explicitly classed `check` or `check:cluster` row with an empty Env is
//     a PROBLEM — an env-bound row must say where it is witnessed.
//
// Gate rows are judged, not executed, so their Env cell is informational.
//
// DECLARED NAMES (desk-containers/35). A `service:<name>` cell may also carry the
// cell dev-env variable names the row needs:
//
//	service:<name>:<NAME>[,<NAME>…]   e.g. service:pg-local:DEVENV_PGHOST,DEVENV_PGPORT
//
// <name> admits neither `:` nor `,`, so the form parses without ambiguity. A row
// that declares no names receives none from the cell's dev-env file
// (containers/secrets.md §4.5). Names keep the case they are written in. The
// allow-list denies by default: a declarable name matches `DEVENV_[A-Z0-9_]+`
// and nothing else, a namespace no shell, loader, interpreter, forge client, git
// or Assay component reads on its own. In every table with the column, a
// malformed name, a duplicate name, a name outside the allow-list (naming its
// class where it is a path, loader, interpreter, tool-configuration,
// agent-socket or Assay name) and declared names on any kind but `service` are
// each a PROBLEM, whatever the row's class.

const (
	rowEnvTree    = "tree"
	rowEnvHost    = "host"
	rowEnvService = "service"
	rowEnvCluster = "cluster"
)

// rowEnvNameRe is the <name> half of a `kind:<name>` Env cell.
var rowEnvNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// resolveRowEnv normalises an `Env` cell and reports its kind and whether it is
// well-formed. An empty cell is well-formed with kind "" (no environment named).
// The normalised cell is returned verbatim so the lint names what was written.
func resolveRowEnv(raw string) (env, kind string, known bool) {
	env = strings.ToLower(strings.TrimSpace(briefdoc.NormalizeRowID(raw)))
	if env == "" {
		return "", "", true
	}
	k, name, named := strings.Cut(env, ":")
	switch k {
	case rowEnvTree:
		return env, k, !named
	case rowEnvHost:
		return env, k, !named || rowEnvNameRe.MatchString(name)
	case rowEnvService, rowEnvCluster:
		return env, k, named && rowEnvNameRe.MatchString(name)
	}
	return env, k, false
}

// rowEnvVarNameRe is the shape of a declared variable name: upper-case
// letters, digits and `_`, not starting with a digit. A name of another shape
// is malformed before the allow-list is consulted.
var rowEnvVarNameRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// rowEnvAllowedNameRe is the deny-by-default allow-list of declarable names
// (desk-containers/35 Task 3): the DEVENV_ namespace and nothing else.
var rowEnvAllowedNameRe = regexp.MustCompile(`^DEVENV_[A-Z0-9_]+$`)

// splitRowEnvNames separates an Env cell into its `kind[:<name>]` part and the
// names it declares. declared is true when the cell carries a names segment
// (a second `:`), even an empty one, so `service:pg:` is a malformed
// declaration rather than a silent `service:pg`. Names are returned verbatim
// and in the order written; base goes on to resolveRowEnv.
func splitRowEnvNames(raw string) (base string, names []string, declared bool) {
	cell := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), "`"))
	kind, rest, ok := strings.Cut(cell, ":")
	if !ok {
		return raw, nil, false
	}
	name, list, ok := strings.Cut(rest, ":")
	if !ok {
		return raw, nil, false
	}
	for _, n := range strings.Split(list, ",") {
		names = append(names, strings.TrimSpace(n))
	}
	return kind + ":" + name, names, true
}

// rowEnvNameClass names the class a refused name belongs to (desk-containers/35
// Task 3), or "" for a name in no named class, which the allow-list still
// refuses by default. The tool-configuration class reuses the shell-startup and
// forge-token sets the verifier's admitted row environment strips
// (verifieradmission.go), so the two never drift apart.
func rowEnvNameClass(name string) string {
	u := strings.ToUpper(name)
	switch {
	case u == "PATH" || u == "HOME" || strings.HasPrefix(u, "XDG_"):
		return "path-and-home"
	case strings.HasPrefix(u, "LD_") || strings.HasPrefix(u, "DYLD_"):
		return "dynamic-loader"
	case slices.Contains([]string{"NODE_OPTIONS", "NODE_PATH", "PYTHONPATH", "PYTHONHOME", "PYTHONSTARTUP", "PERL5LIB", "PERL5OPT", "PERLLIB", "RUBYOPT", "RUBYLIB"}, u):
		return "interpreter-options"
	case strings.HasPrefix(u, "GIT_") || strings.HasPrefix(u, "GH_") || admittedRowForgeToken(u) || admittedRowShellStartup(u):
		return "tool-configuration"
	case u == "SSH_AUTH_SOCK" || u == "GPG_AGENT_INFO":
		return "agent-socket"
	case strings.HasPrefix(u, "ASSAY_"):
		return "Assay"
	}
	return ""
}

// rowEnvNameProblems lints the names an Env cell declares (desk-containers/35):
// a malformed name, a duplicate, and a name outside the DEVENV_ allow-list.
func rowEnvNameProblems(path, where, env string, names []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, n := range names {
		switch {
		case !rowEnvVarNameRe.MatchString(n):
			out = append(out, fmt.Sprintf("%s: %s declares name %q in environment %q, which is not a well-formed variable name — a declared name is upper-case letters, digits and `_`, written `service:<name>:<NAME>[,<NAME>…]` with no empty entry; fix the cell — desk-containers/35", path, where, n, env))
		case seen[n]:
			out = append(out, fmt.Sprintf("%s: %s declares name %q twice in environment %q — each name is declared once; drop the duplicate — desk-containers/35", path, where, n, env))
		case !rowEnvAllowedNameRe.MatchString(n):
			class := "it is in no named class, and every name outside the allow-list refuses (deny by default)"
			if c := rowEnvNameClass(n); c != "" {
				class = "it is in the " + c + " class, which a shell, loader, interpreter, tool or Assay component reads on its own"
			}
			out = append(out, fmt.Sprintf("%s: %s declares name %q in environment %q, which is outside the dev-env allow-list — a declarable name matches `DEVENV_[A-Z0-9_]+` and nothing else; %s. Rename it into the DEVENV_ namespace and pass it to the tool explicitly in the row's command — desk-containers/35", path, where, n, env, class))
		}
		seen[n] = true
	}
	return out
}

// rowEnvProblem is lint arm (d) for one row of a table carrying an `Env`
// column: nil when the row is clean, else the PROBLEM texts (issue #6065;
// declared names, desk-containers/35).
func rowEnvProblem(path, where string, r verifyRowCells) []string {
	base, names, declared := splitRowEnvNames(r.Env)
	env, kind, known := resolveRowEnv(base)
	if declared {
		full := strings.TrimSpace(strings.Trim(strings.TrimSpace(r.Env), "`"))
		if kind != rowEnvService {
			return []string{fmt.Sprintf("%s: %s declares names in environment %q, but only a `service:<name>` environment may carry declared names (`service:<name>:<NAME>[,<NAME>…]`); a %q environment receives nothing from the cell dev-env file. Move the names to a `service:<name>` row or drop them — desk-containers/35", path, where, full, kind)}
		}
		if known {
			probs := rowEnvNameProblems(path, where, full, names)
			if msg := rowEnvKindProblem(path, where, env, kind, r); msg != "" {
				probs = append(probs, msg)
			}
			return probs
		}
	}
	if !known {
		return []string{fmt.Sprintf("%s: %s declares environment %q, which is not a defined Verify-row environment — the recognised forms are `tree` (the checkout alone, network-off: what CI re-executes), `host` or `host:<name>` (the witness's own machine), `service:<name>` (a named non-production service instance: a database, an API, a staging deployment) and `cluster:<name>` (a live cluster), where <name> is lower-case letters, digits, `.`, `_` or `-`. An environment the tool cannot resolve routes the row nowhere; fix the cell — issue #6065", path, where, env)}
	}
	if msg := rowEnvKindProblem(path, where, env, kind, r); msg != "" {
		return []string{msg}
	}
	return nil
}

// rowEnvKindProblem is the class-versus-environment half of lint arm (d) for a
// well-formed Env cell (issue #6065): "" when the row's class fits its kind.
func rowEnvKindProblem(path, where, env, kind string, r verifyRowCells) string {
	offTree := kind != "" && kind != rowEnvTree
	cls := r.class()
	switch {
	case offTree && !r.Classed:
		return fmt.Sprintf("%s: %s names environment %q in a table with no `Class` column — an unclassed row is legacy `check`, which the scheduled main re-run still executes in CI, where that environment does not exist. Add a `Class` column and class the row `check` (or `check:cluster` for a cluster probe) — issue #6065", path, where, env)
	case offTree && cls == classCheckCI:
		return fmt.Sprintf("%s: %s is classed `check:ci` but its Env cell names %q — a `check:ci` row is re-executed by CI network-off on the checkout alone, which reaches no host, service or cluster. Class it `check` (or `check:cluster` for a cluster probe) so a non-implementer witnesses it where that environment exists, or make it a tree-only command with Env `tree` — issue #6065", path, where, env)
	case kind == "" && r.Classed && (cls == classCheck || cls == classCheckCluster):
		return fmt.Sprintf("%s: %s is classed `%s` but names no environment — an env-bound row must say where a non-implementer witnesses it. Name it in the Env cell (`host`, `host:<name>`, `service:<name>` for a non-production service instance, or `cluster:<name>`), or class the row `check:ci` with Env `tree` if the checkout alone is enough — issue #6065", path, where, cls)
	}
	return ""
}

// ---------------------------------------------------------------------------
// verify.d scripted rows
// ---------------------------------------------------------------------------

// verifyScriptRe matches a Command cell whose command IS a verify.d script path
// (the script convention above). Anchored to the whole trimmed command so an
// inline command that merely mentions such a path (`cat …/row-1.sh`) is not
// mistaken for a scripted row — a scripted row's Command is the bare path and
// nothing else.
var verifyScriptRe = regexp.MustCompile(`^docs/streams/[^/\s]+/verify\.d/[^\s]+\.sh$`)

// scriptPath returns the verify.d script a row points at, or "" when the row is
// an inline command rather than a scripted one.
func scriptPath(command string) string {
	s := strings.TrimSpace(command)
	if verifyScriptRe.MatchString(s) {
		return s
	}
	return ""
}

// ---------------------------------------------------------------------------
// Lint (a) unknown class · (b) missing / non-executable script
// ---------------------------------------------------------------------------

// verifyRowClassProblems is the class lint. Two PROBLEM shapes, both hard:
//
//	(a) an unknown class — a Class cell that names no defined class. Always a
//	    PROBLEM: a class the tool does not recognise routes the row NOWHERE, and
//	    a row that routes nowhere is silently exempt from every gate. This fires
//	    on every brief-v1 file regardless of its README-row status, because a
//	    typo'd class is wrong the moment it is written.
//
//	(b) a `check:ci`/`check` SCRIPTED row whose script is missing or not
//	    executable. A scripted row's whole contract is "a runner re-executes this
//	    file"; a file that is absent, or present but not `chmod +x`, cannot be
//	    re-executed, so the row proves nothing. SCOPED to briefs the board has
//	    moved past `todo`: a `todo` brief listing its planned verify.d scripts is
//	    a PLAN, not a claim, and reddening the tree over scripts a future brief
//	    has not written yet would punish authoring the plan. Once a brief is
//	    in-progress/implemented/verified/done, its scripted rows must resolve to
//	    real, executable files.
//
// gate:model / gate:human rows are never scripted and are not checked here.
//
//	(c) an unknown SHELL (issue #1424) — a `Shell` cell that names no defined
//	    shell. Always a PROBLEM, on the same reasoning as (a): a shell the tool
//	    cannot resolve would route the row nowhere. The `Shell` column is
//	    independent of the `Class` column, so this fires whether or not the table
//	    declares a `Class` column.
//
//	(d) an environment mismatch (issue #6065), only in a table carrying an
//	    `Env` column — see rowEnvProblem: an unknown Env cell, a `check:ci` row
//	    naming an environment CI cannot reach, an unclassed row naming one, or
//	    an env-bound row naming none. Always a PROBLEM; no table on main carried
//	    the column when the arm landed, so it adds no debt to the corpus.
func verifyRowClassProblems(streams []*Stream) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	for _, s := range streams {
		// Map brief number → README status, so (b) can exempt todo briefs. A
		// brief file with no README row (status "") is treated as active for (b):
		// a scripted row in a file nobody lists is still a claim in the tree.
		status := map[string]string{}
		for _, br := range s.Briefs {
			status[br.Num] = br.Status
		}
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed reported by checkBriefFiles; legacy exempt
			}
			_, num, _ := expectedBriefID(path)
			briefStatus := status[num]
			enforceScripts := briefStatus != "" && briefStatus != "todo"
			verifyRowTable(bf.Verify, func(r verifyRowCells) {
				where := "a Verify row"
				if r.Num != "" {
					where = "Verify row " + r.Num
				}
				// (c) unknown shell (issue #1424). Independent of the Class
				// column and checked first: an empty cell (or no Shell column)
				// resolves to the default `sh` and is silent, so this only fires
				// on a cell that names a shell the tool does not recognise — a
				// typo like `bash`/`powershell`/`shell` that would route nowhere.
				if sh, known := resolveRowShell(r.Shell); !known {
					add("%s: %s declares shell %q, which is not a defined Verify row shell — the recognised shells are `sh` (POSIX bash, the default when the `Shell` column is absent or the cell is empty), `cmd` (Windows cmd.exe) and `pwsh` (PowerShell). A row whose shell the tool cannot resolve routes nowhere; fix the marker or leave the cell empty for the default `sh` — issue #1424", path, where, sh)
				}
				// (a) unknown class OR unknown obligation. A compound cell
				// (`check +mutation`, mistake-proofing/03) is split into its
				// execution token and its obligation tokens; each axis is
				// validated against its own closed set, and an unrecognised value
				// on EITHER axis is fatal — a token the tool cannot resolve routes
				// nowhere and is silently exempt from its gate.
				if r.Classed {
					execCell, obs := briefdoc.SplitRowClassCell(r.Class)
					if execCell != "" {
						if cls, known := resolveRowClass(execCell); !known {
							add("%s: %s declares class %q, which is not a defined Verify row class — the recognised classes are `check:ci` (hermetic, CI re-executes network-off), `check` (env-bound, runner-executed), `gate:model` and `gate:human`. A row whose class the tool cannot resolve routes nowhere and is silently exempt from every gate; fix the class or drop the column (an unclassed table is legacy `check`) — verdict-lane/02", path, where, cls)
							return // an unknown class makes the script check meaningless
						}
					}
					for _, ob := range obs {
						if !knownObligations[ob] {
							add("%s: %s declares obligation %q, which is not a defined Verify-row obligation — the recognised obligations are `+mutation`, `+flow`, `+dereference` and `+neighbour` (mistake-proofing/03). An obligation token the tool cannot resolve routes nowhere and is silently exempt from the obligation derivation; fix the token or drop it. This lint checks the PRESENCE of a typed obligation, not its adequacy, which stays the reviewer's call (spec §3 D7) — mistake-proofing/03", path, where, ob)
							return // an unknown obligation makes the script check meaningless
						}
					}
				}
				// (d) environment mismatch (issue #6065). After (a), so the
				// class it reads is a known one.
				if r.EnvCol {
					for _, msg := range rowEnvProblem(path, where, r) {
						add("%s", msg)
					}
				}
				// (b) missing / non-executable script.
				if !enforceScripts {
					return
				}
				cls := r.class()
				if cls != classCheckCI && cls != classCheck {
					return
				}
				sp := scriptPath(r.Command)
				if sp == "" {
					return // inline command, not a scripted row
				}
				abs := filepath.Join(s.Root, sp)
				info, statErr := os.Stat(abs)
				switch {
				case statErr != nil:
					add("%s: %s is a `%s` scripted row whose script %s does not exist — a scripted row's Command IS the script a runner re-executes, and a row pointing at a missing file proves nothing. Create the script (executable, exit 0 = PASS) or make the row an inline command — verdict-lane/02", path, where, cls, sp)
				case info.IsDir():
					add("%s: %s points its `%s` script cell at %s, which is a directory, not an executable file — verdict-lane/02", path, where, cls, sp)
				case info.Mode()&0o111 == 0:
					add("%s: %s is a `%s` scripted row whose script %s is not executable (no +x bit) — a runner cannot re-execute it. `chmod +x` the script and commit the mode — verdict-lane/02", path, where, cls, sp)
				}
			})
		}
	}
	sort.Strings(problems)
	return problems
}

// ---------------------------------------------------------------------------
// Lint (c) reviewer-conspicuity NOTICE on a verify.d diff
// ---------------------------------------------------------------------------

// changedVerifyScriptRe matches a changed repo-relative path that lives under a
// stream's verify.d/ tree — the scripts a runner will re-execute as the machine
// half of a verdict.
var changedVerifyScriptRe = regexp.MustCompile(`^docs/streams/[^/]+/verify\.d/`)

// verifyScriptDiffNotices raises a CONSPICUOUS NOTICE, naming each script, when a
// PR's diff touches any `docs/streams/*/verify.d/**` file.
//
// The reviewer is the trust anchor for verify scripts (R-6): there is no freeze
// rule and no automated gate that says a script edit is safe — the point is the
// opposite, that a human explicitly assesses every change to code a runner will
// later execute as the deterministic half of a verdict. So this fires whenever
// the diff touches the tree, listing the exact files, so the change cannot slip
// past review as an ordinary docs edit. It is advisory (never changes the exit
// code); its whole job is to be SEEN.
//
// It reads the `changed` set (the `--changed` file CI passes); with no changed
// set it is inert, exactly like the DAR check.
func verifyScriptDiffNotices(changed []string) []string {
	var scripts []string
	for _, p := range changed {
		p = strings.TrimSpace(p)
		if changedVerifyScriptRe.MatchString(p) {
			scripts = append(scripts, p)
		}
	}
	if len(scripts) == 0 {
		return nil
	}
	sort.Strings(scripts)
	return []string{fmt.Sprintf(
		"VERIFY-SCRIPT REVIEW: this PR changes %d verify.d script(s) — %s. These are executed VERBATIM by the verdict runner as the deterministic half of a verify verdict (verdict-lane), so a change here is a change to what the machine will trust. There is no freeze and no automated safety gate on them by design: the reviewer is the trust anchor. Read each diff as executable code, not as a docs edit, before approving — verdict-lane/02",
		len(scripts), strings.Join(scripts, ", "))}
}
