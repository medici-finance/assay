package main

// rule51.go — the rule-51 witness lane of `statusgen verifyrun` (#4823).
//
// Brief rule 51 (docs/brief-rules.md) credits a Verify row whose Expect is a
// clean repo-wide statusgen lint when `statusgen --lint --acceptance-forge` is
// clean at the pinned revision, witnessed in the host lane. Until now nothing
// wrote that witness: the offline lint is red wherever a closed brief carries an
// Acceptance-Transition record, and a hand citation in Evidence prose is not a
// witness any tool reads.
//
// `verifyrun --rule51` writes it. For a row in the rule-51 class (rule51Class)
// whose offline run failed, an admitted host run also runs THIS binary as
// `--root . --lint --acceptance-forge` under the run's verifier read
// credential, and stamps the row's own witness with what that run found. The
// row's authored command is never rewritten: the witness records the command
// as authored and its offline result, and the stamp sits beside them in the
// Result cell.
//
// checkWitnesses credits a stamped row as passing only when the stamp is well
// formed, its digest binds it to this row's command, offline result, tree and
// date, the forge run was clean (rc 0 with a `LINT: PASS` line), the pin is a
// release tag, and the tree is a clean commit. Anything else stays the
// failure the offline run recorded.
//
// What the stamp is not: its digest is unkeyed, so like every witness row it
// is a tool FORMAT, not a signature. It stops a stamp being copied onto another
// row, tree or command, and it stops a hand citation counting at all, but a
// person who writes a whole well-formed stamp by hand can forge it exactly as
// they can forge any witness row. The bound on that is the one every witness
// already has: admission and review of the Evidence diff.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// rule51CredClass names the credential class the forge run reads under: the
// verifier's run-scoped read-only credential (runcredential.go). It is the only
// class there is; the token is recorded so a later class cannot be mistaken
// for this one.
const rule51CredClass = "verifier-run-read"

// rule51StampVersion versions the digest input.
const rule51StampVersion = "rule51-v1"

// rule51Executable is the statusgen the forge run executes: this binary, so
// the pin the stamp records is the version that ran. A variable only so a test
// can point it at a stub.
var rule51Executable = os.Executable

// rule51Class reports whether a Verify row is in the rule-51 class: a row
// whose command is exactly a repo-wide statusgen lint, and whose Expect asks
// for a clean exit.
//
// The command must be, token for token after `\|` unescaping, one of
//
//	statusgen [--root . | --root=.] --lint            (flags in either order)
//	cd statusgen && go run . [--root .. | --root=..] --lint
//
// with `--lint` exactly once, at most one root flag, and nothing else: no
// `--diff-base` (a diff-scoped lint is not repo-wide), no `--acceptance-forge`
// (that row is already the forge form), no `--json`, no subcommand, no
// redirection, pipe, `;`, `$?` or other shell text. In the `cd` form the root
// is required, since `go run .` inside statusgen/ otherwise lints statusgen/.
//
// The Expect must decide on exit 0 (the default or an explicit 0), with no
// count bound and no required line other than `LINT: PASS`. A row asking for
// a named NOTICE or a problem count is outside rule 51.
func rule51Class(command, expect string) bool {
	if !rule51CommandClass(command) {
		return false
	}
	e := parseExpect(expect)
	return e.exitCode == 0 && !e.minSet && (e.wantLine == "" || e.wantLine == "LINT: PASS")
}

// rule51CommandClass is the command half of rule51Class.
func rule51CommandClass(command string) bool {
	cmd := unescapePipes(command)
	if strings.ContainsAny(cmd, "|;<>$`\\()'\"*?{}[]~#!\n\r") {
		return false
	}
	toks := strings.Fields(cmd)
	var rest []string
	var root string
	switch {
	case len(toks) >= 1 && toks[0] == "statusgen":
		rest, root = toks[1:], "."
	case len(toks) >= 6 && toks[0] == "cd" && toks[1] == "statusgen" && toks[2] == "&&" &&
		toks[3] == "go" && toks[4] == "run" && toks[5] == ".":
		rest, root = toks[6:], ".."
	default:
		return false
	}
	lint, roots := 0, 0
	for i := 0; i < len(rest); i++ {
		switch t := rest[i]; {
		case t == "--lint":
			lint++
		case t == "--root" && i+1 < len(rest) && rest[i+1] == root:
			roots++
			i++
		case t == "--root="+root:
			roots++
		default:
			return false
		}
	}
	if lint != 1 || roots > 1 {
		return false
	}
	return root == "." || roots == 1
}

// rule51Stamp is the rule-51 lane's record of the forge run.
type rule51Stamp struct {
	State     string // pass | fail | could-not-run
	ForgeRC   int    // -1 when the forge form never ran
	ForgeOut  string // first 12 hex of sha256 of the forge run's combined output
	Pin       string // the statusgen version that ran the forge form
	CredClass string // rule51CredClass
	CredSum   string // first 12 hex of sha256 of the credential value, or "-"
}

// rule51Tokens renders the stamp for the witness Result cell, with the digest
// that binds it to the row.
func (s rule51Stamp) tokens(w witness) string {
	return fmt.Sprintf("rule51=%s forge-rc=%s forge-out=sha256:%s pin=%s cred=%s cred-digest=%s stamp=sha256:%s",
		s.State, exitCell(s.ForgeRC), s.ForgeOut, s.Pin, s.CredClass, rule51CredCell(s.CredSum),
		rule51Digest(s, w.ID, w.Command, w.State, exitCell(w.Exit), w.OutHash, w.Tree, w.Date))
}

func rule51CredCell(sum string) string {
	if sum == "" || sum == "-" {
		return "-"
	}
	return "sha256:" + sum
}

// rule51Digest is the stamp's binding: every field of the stamp and of the
// witness it sits in that a credit reads.
func rule51Digest(s rule51Stamp, id, command, state, exit, outHash, tree, date string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		rule51StampVersion, id, normalizeCommandText(command), state, exit, outHash, tree, date,
		s.State, exitCell(s.ForgeRC), s.ForgeOut, s.Pin, s.CredClass, rule51CredCell(s.CredSum),
	}, "\n")))
	return hex.EncodeToString(sum[:])
}

// rule51PinToken is the version token the stamp records: the binary's own
// stamped version, or "unstamped" for one a token cannot carry.
var rule51PinRe = regexp.MustCompile(`^[A-Za-z0-9._+/-]+$`)

func rule51PinToken() string {
	v := statusgenVersion
	if !rule51PinRe.MatchString(v) {
		return "unstamped"
	}
	return v
}

// rule51ForgeState decides the forge run's state from its exit and output, as
// rule 51 states it: a clean run is rc 0 with a `LINT: PASS` line; a run that
// says an authority read did not complete is could-not-run; anything else is
// fail. Only pass is ever credited.
func rule51ForgeState(res runResult) string {
	switch {
	case res.couldNotRun:
		return stateCouldNotRun
	case res.exit == 0 && outputHasLine(res.output, "LINT: PASS"):
		return statePass
	case bytes.Contains(res.output, []byte("could-not-check repository authority")),
		bytes.Contains(res.output, []byte("human approval could not be corroborated")):
		return stateCouldNotRun
	default:
		return stateFail
	}
}

// runRule51 runs the forge form of an in-class row for the witness w and
// returns the stamp's tokens, plus a console note. sandbox is the run's
// network-off mode: a run inside a network-none container cannot reach the
// forge, and a run with no verifier read credential (one not admitted) has no
// credential to read it with, so both stamp could-not-run without running.
func runRule51(plan shellPlan, sandbox, root string, w witness, timeout time.Duration) (string, string) {
	s := rule51Stamp{State: stateCouldNotRun, ForgeRC: -1, ForgeOut: hashOutput(nil), Pin: rule51PinToken(), CredClass: rule51CredClass, CredSum: "-"}
	note := ""
	switch {
	case sandbox == sandboxContainerNetns:
		note = "rule-51 lane: a network-none container cannot reach the forge; run the host lane"
	case plan.readCred == nil:
		note = "rule-51 lane: the run is not admitted, so it holds no verifier read credential"
	default:
		exe, err := rule51Executable()
		if err != nil || exe == "" {
			note = fmt.Sprintf("rule-51 lane: cannot name this statusgen binary: %v", err)
			break
		}
		_, secret, cerr := plan.readCred.ensure()
		if cerr != nil {
			note = "rule-51 lane: the verifier read credential is unavailable: " + cerr.Error()
			break
		}
		sum := sha256.Sum256([]byte(secret))
		s.CredSum = hex.EncodeToString(sum[:])[:12]
		res := runVerifyCommandWith(root, shellQuoteArg(exe)+" --root . --lint --acceptance-forge", timeout, nil, plan, rowShellSh)
		s.State = rule51ForgeState(res)
		s.ForgeRC = res.exit
		s.ForgeOut = hashOutput(res.output)
		note = "rule-51 lane: --acceptance-forge " + s.State + " (rc " + exitCell(res.exit) + ")"
		if res.couldNotRun && res.reason != "" {
			note += ": " + res.reason
		}
	}
	return s.tokens(w), note
}

// shellQuoteArg quotes one argument for a POSIX shell.
func shellQuoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// rule51StampRe reads a stamp out of a witness Result cell.
var rule51StampRe = regexp.MustCompile(`(?:^|\s)rule51=(pass|fail|could-not-run) forge-rc=(-|\d+) forge-out=sha256:([0-9a-f]{12}) pin=(\S+) cred=(\S+) cred-digest=(-|sha256:[0-9a-f]{12}) stamp=sha256:([0-9a-f]{64})(?:\s|$)`)

// rule51Credit decides whether the latest witness of an in-class row is a
// credited rule-51 witness. It returns stamped=false when the witness carries
// no stamp at all (it is then an ordinary failure, unchanged), and otherwise
// credited plus the reason.
func rule51Credit(r verifyRow, latest string) (stamped, credited bool, why string) {
	cells := witnessCells(latest)
	if cells == nil {
		return false, false, ""
	}
	result := strings.TrimSpace(cells[witnessCellResult])
	if head, _, found := strings.Cut(result, " — "); found {
		result = head
	}
	if !strings.Contains(result, "rule51=") {
		return false, false, ""
	}
	if strings.Count(result, "rule51=") != 1 {
		return true, false, "the Result cell carries more than one rule-51 stamp"
	}
	m := rule51StampRe.FindStringSubmatch(result)
	if m == nil {
		return true, false, "the rule-51 stamp is malformed"
	}
	if !rule51Class(r.Command, r.Expect) {
		return true, false, "the row is outside the rule-51 class (a repo-wide `statusgen --lint` with no --diff-base, Expect a clean exit)"
	}
	if !strings.HasPrefix(result, stateFail+" exit=") {
		return true, false, "the stamp sits on a witness whose offline result is not a recorded failure"
	}
	s := rule51Stamp{State: m[1], ForgeOut: m[3], Pin: m[4], CredClass: m[5], CredSum: strings.TrimPrefix(m[6], "sha256:")}
	s.ForgeRC = -1
	if m[2] != "-" {
		s.ForgeRC, _ = strconv.Atoi(m[2])
	}
	exit := witnessResultRe.FindStringSubmatch(result)
	outHash := strings.TrimPrefix(witnessHashRe.FindString(cells[witnessCellOutput]), "sha256:")
	tree := witnessTreeOf(latest)
	date := strings.TrimSpace(cells[witnessCellDate])
	if len(exit) != 2 || rule51Digest(s, strings.TrimSpace(cells[0]), witnessCommandOf(latest), stateFail, exit[1], outHash, tree, date) != m[7] {
		return true, false, "the rule-51 stamp's digest does not bind it to this witness (row, command, offline result, tree and date)"
	}
	if s.CredClass != rule51CredClass || s.CredSum == "" || s.CredSum == "-" {
		return true, false, "the forge run did not read under the verifier's run-scoped read credential"
	}
	if s.State != statePass || s.ForgeRC != 0 {
		return true, false, fmt.Sprintf("the --acceptance-forge run is %s (rc %s), not clean", s.State, m[2])
	}
	if _, ok := gateParseVersion(s.Pin); !ok {
		return true, false, "the forge run's statusgen is " + s.Pin + ", not a pinned release"
	}
	if tree == "" || tree == "no-git" || strings.Contains(tree, "+") {
		return true, false, "the witness tree " + strconv.Quote(tree) + " is not a clean revision"
	}
	return true, true, "rule-51 lane: offline lint red; --acceptance-forge clean at " + s.Pin + " @ " + tree
}
