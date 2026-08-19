// Package main implements writeguard, a Claude Code PreToolUse hook that
// mechanically refuses writes to the SHARED checkout from sessions homed
// elsewhere (worktrees, scratchpad checkouts) — the write-isolation backstop.
//
// Decision model:
//
//   - sharedRoot  = the main checkout of the repo the session lives in,
//     derived as dirname(git rev-parse --git-common-dir) of the
//     session's project dir. In a linked worktree the common dir
//     is <shared>/.git, so this resolves to the shared checkout
//     from anywhere.
//   - session home = CLAUDE_PROJECT_DIR (hook env, = the session's project
//     root), falling back to the hook payload's cwd.
//   - EXEMPT (OPT-IN, #1035): session home == sharedRoot AND the payload
//     cwd genuinely resolves inside the shared checkout (outside
//     .claude/worktrees) AND the shared-OK claim is present — either the
//     WRITEGUARD_SHARED_OK env token (human shells) or the sentinel file
//     ~/.config/assay/writeguard-shared-ok (#1190: the env token alone is
//     unreachable from the Claude Code Bash tool, which starts a fresh shell
//     per call, so the sanctioned exemption had no way to be claimed).
//     CLAUDE_PROJECT_DIR alone is not enough: harness-worktree subagents
//     and EnterWorktree sessions inherit the shared project dir while
//     operating from a linked worktree — exactly the dispatcher population
//     (#1007). And being shared-homed alone is not enough either (#1035):
//     dispatched sessions booted by cd-ing into the shared checkout are
//     shared-homed by construction, so the exemption must be claimed
//     explicitly — coordinator/human shells export WRITEGUARD_SHARED_OK=1
//     (mirroring the .githooks ASSAY_MAIN_COMMIT_OK pattern); everything
//     else is blocked with isolation guidance. The token alone never
//     exempts a worktree-homed session.
//   - BLOCK (via permissionDecision:"deny" on exit 0, since go run does not
//     propagate child exit codes): Edit/Write/MultiEdit/NotebookEdit whose
//     target path is inside sharedRoot but NOT under sharedRoot/.claude/worktrees/;
//     Bash commands that mention the sharedRoot path (outside .claude/worktrees)
//     or run with cwd inside it AND carry a write indicator whose own
//     resolved WRITE TARGET lands inside the shared checkout (redirection,
//     cp/mv/rm/tee/sed -i, mutating git subcommand keyed on -C/--git-dir/
//     --work-tree or cwd, statusgen keyed on --root or cwd, a build tool keyed
//     on cwd — #1006: a pure READ mention of the shared path never arms an
//     indicator whose write lands elsewhere), or that cd/pushd into the
//     shared checkout at all — including a cd inside a quoted subshell
//     string (sh -c 'cd <shared> && …') and the env -C/--chdir
//     cd-equivalents (#1017 review).
//
// Three refinements keep "write indicator" meaning a WRITE (#1190):
//
//   - Indicators are matched against the command with LITERAL TEXT masked out
//     (quoted spans and heredoc bodies), so a `>` inside an awk program, a
//     markdown blockquote in a `gh issue create --body`, or the word
//     "statusgen" in an issue body is prose, not a redirect/invocation. Text
//     that is itself a command — the argument of `sh -c` / a heredoc fed to a
//     shell — stays visible (maskLiterals).
//   - The tool indicators (statusgen, the build tool) require an INVOCATION: the
//     name must end its own token (…/tools/statusgen/registers.go mentions it
//     mid-path and runs nothing) and sit in command position (`grep -rn
//     statusgen x` runs grep). Read-only modes (--lint/--check/--dry-run)
//     write nothing and never hit.
//   - Relative write targets resolve against the command's EFFECTIVE cwd —
//     the payload cwd advanced by any leading `cd`. An agent Bash call runs a
//     fresh shell whose cwd is the payload cwd, so a worker in its own
//     worktree writes `cd /abs/worktree && …`; blaming that write on the
//     payload cwd flagged the wrong tree.
//
// The guard FAILS OPEN: any parse/git error exits 0 so a broken guard can
// never brick every session.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Config describes one guard evaluation. Paths need not exist.
type Config struct {
	SharedRoot   string // main checkout root of the session's repo
	ProjectDir   string // session home (CLAUDE_PROJECT_DIR, fallback payload cwd)
	Cwd          string // hook payload cwd (the session's current directory)
	CwdTop       string // toplevel of the git worktree containing Cwd ("" if unknown)
	SharedOK     bool   // shared-OK claim present: env token or sentinel file (#1035 opt-in, #1190 reachability)
	SentinelPath string // path of the claim sentinel file; writes to it are refused from every session ("" disables)
	// Callout is the adopter's dangerous-command callout (ASSAY_WRITEGUARD_CALLOUT),
	// "" when none is configured. It is consulted only AFTER the compiled indicators
	// have declined to block, so it can only ADD refusals; every way it can fail to
	// answer is itself a refusal. See callout.go.
	Callout string
	// CalloutTimeout bounds one callout invocation; zero means the shared default
	// (deskkit.DefaultCalloutTimeout). It is a field rather than a constant so the
	// TIMEOUT failure mode is testable in milliseconds instead of costing the suite
	// the real deadline — an untested fail-closed branch is the one that rots.
	CalloutTimeout time.Duration
}

// Verdict is the guard's decision for one tool call.
type Verdict struct {
	Block  bool
	Reason string
}

func (c Config) worktreesDir() string {
	return filepath.Join(c.SharedRoot, ".claude", "worktrees")
}

// Exempt reports whether this call genuinely operates from the shared
// checkout itself AND has claimed the opt-in token, in which case the
// guard never blocks.
//
// The shared-homed exemption is OPT-IN (#1035): being homed in the shared
// checkout is no longer sufficient on its own, because sanctioned boot
// instructions ("cd into the checkout so the skill resolves") produce
// dispatched sessions that are shared-homed by construction — the exact
// dispatcher vector recurring through a sanctioned path. Sessions
// that legitimately operate on the shared checkout (the coordinator, a
// direct human shell) claim the exemption explicitly, mirroring the
// .githooks ASSAY_MAIN_COMMIT_OK main-commit gate; everything else gets
// blocked with isolation guidance. The claim alone never exempts a session
// that is not shared-homed.
//
// #1190: the claim is EITHER the WRITEGUARD_SHARED_OK env token (a human
// shell that exports it once) OR a short-lived sentinel file (see main.go).
// The env token alone made the sanctioned exemption unreachable from the
// Claude Code Bash tool — every call gets a fresh shell, and the hook reads
// its own process env, so an inline `WRITEGUARD_SHARED_OK=1 cmd` can never
// reach it.
func (c Config) Exempt() bool {
	return c.SharedOK && c.sharedHomed()
}

// sharedHomed reports whether this call genuinely operates from the shared
// checkout itself (session home AND payload cwd both resolve there).
//
// CLAUDE_PROJECT_DIR alone is NOT sufficient (#1007): subagents dispatched
// with isolation:worktree and EnterWorktree sessions inherit the shared
// checkout as CLAUDE_PROJECT_DIR while operating from a linked worktree
// (.claude/worktrees/<x> or a sibling tree) — exactly the population the
// backstop exists to guard. Shared-homed therefore requires BOTH the
// project dir AND the payload cwd to resolve into the shared checkout
// (outside the .claude/worktrees carve-out); CwdTop — the cwd's actual
// worktree toplevel, when the caller could determine it — additionally
// catches ad-hoc worktrees nested inside the shared root.
func (c Config) sharedHomed() bool {
	root := resolvePath(c.SharedRoot)
	if !samePath(resolvePath(c.ProjectDir), root) {
		return false
	}
	if c.CwdTop != "" && !samePath(resolvePath(c.CwdTop), root) {
		return false
	}
	if c.Cwd == "" {
		// No payload cwd to cross-check — fall back to the project-dir
		// answer alone (pre-#1007 behavior) rather than blocking a
		// coordinator session on a malformed payload.
		return true
	}
	cwd := resolvePath(c.Cwd)
	return underDir(cwd, root) && !underDir(cwd, resolvePath(c.worktreesDir()))
}

// home returns the directory the session should treat as its own tree, for
// block-message guidance. Normally ProjectDir; when ProjectDir is the shared
// root but the call operates from a linked worktree (#1007), that worktree
// is home — telling such a session to "write inside <shared>" would be
// telling it to commit the exact violation the guard exists to stop.
func (c Config) home() string {
	root := resolvePath(c.SharedRoot)
	if !samePath(resolvePath(c.ProjectDir), root) {
		return c.ProjectDir
	}
	if c.CwdTop != "" && !samePath(resolvePath(c.CwdTop), root) {
		return c.CwdTop
	}
	if c.Cwd == "" {
		return c.ProjectDir
	}
	wtd := resolvePath(c.worktreesDir())
	cwd := resolvePath(c.Cwd)
	if underDir(cwd, wtd) && !samePath(cwd, wtd) {
		if rel, err := filepath.Rel(wtd, cwd); err == nil {
			first := strings.Split(rel, string(filepath.Separator))[0]
			return filepath.Join(c.worktreesDir(), first)
		}
	}
	if !underDir(cwd, root) {
		return c.Cwd
	}
	return c.ProjectDir
}

// CheckFilePath evaluates an Edit/Write/MultiEdit/NotebookEdit target path.
func (c Config) CheckFilePath(tool, path string) Verdict {
	p := resolveToken(path, c.Cwd)
	root := resolvePath(c.SharedRoot)
	if underDir(p, root) && !underDir(p, resolvePath(c.worktreesDir())) {
		return c.block(tool, path)
	}
	return Verdict{}
}

// CheckBash evaluates a Bash command string. Crude by design: a blocked
// false positive with an explanation beats a silent leak. But the
// block must still key on the actual filesystem path a command writes —
// a write-shaped command run from a cwd that happens to sit inside the
// shared checkout is only a real hit if ITS OWN target argument resolves
// there too (see checkWriteIndicators). Commands with no local filesystem
// write at all (a bare `gh issue create`, etc.) never match any indicator
// and pass through untouched.
//
// Path mentions are resolved — relative paths (../.., ./sibling), dotted
// traversal (worktrees/../../), and home-relative paths (~/…, $HOME/…)
// are resolved against the payload cwd / the real home directory and
// tested with underDir, closing the evasion that string-matching the
// absolute root leaves open and correctly placing ~/.claude/** (which is
// NOT part of the shared checkout) outside it.
func (c Config) CheckBash(command string) Verdict {
	rroot := resolvePath(c.SharedRoot)
	wtd := resolvePath(c.worktreesDir())

	mentions := false
	for _, tok := range pathTokens(command) {
		p := resolveToken(tok, c.Cwd)
		if underDir(p, rroot) && !underDir(p, wtd) {
			mentions = true
			break
		}
	}

	cwdInShared := false
	if c.Cwd != "" {
		cwd := resolvePath(c.Cwd)
		cwdInShared = underDir(cwd, rroot) && !underDir(cwd, wtd)
	}

	if !mentions && !cwdInShared {
		return Verdict{}
	}

	if cdIntoDir(command, c.Cwd, rroot, wtd) {
		return c.block("Bash (cd into the shared checkout)", command)
	}

	if name, blocked := c.checkWriteIndicators(command, rroot, wtd); blocked {
		return c.block("Bash ("+name+" targeting the shared checkout)", command)
	}

	// The adopter's callout is consulted LAST, and that ordering is the only-widens
	// property rather than a preference: every compiled indicator has already had its
	// say, so no answer here can clear one. It is reached only for commands that are
	// already in scope (they name the shared checkout or run inside it) — the same
	// scope the compiled indicators are judged in — so an adopter's callout is never
	// asked about a command that has nothing to do with the protected tree.
	if reason, blocked := c.checkCallout(command); blocked {
		return Verdict{Block: true, Reason: reason}
	}

	return Verdict{}
}

// CheckClaimSentinel refuses EVERY tool-surface write to the shared-OK claim
// sentinel — creation, renewal, edit or removal — from every session,
// exempt or not, before any other check runs.
//
// #1190 review blocker 2: the sentinel file made the sanctioned exemption
// reachable, but reachable BY THE WRONG POPULATION. A dispatched worker can
// `touch` the file as easily as a human can, and dispatched workers are
// precisely who the isolation backstop exists to constrain (#1035: a worker booted by
// `cd <shared>` is shared-homed by construction). One Bash call from any
// session and every shared-homed session on the machine is exempt — with the
// block message itself handing over the recipe, to a model that has just been
// refused. An exemption its own target population can grant itself is not a
// gate.
//
// What actually separates a human act from an agent act on this machine is
// not intent, which the guard cannot read, but the TOOL SURFACE: every agent
// file write goes through a hook (Bash and Edit/Write/MultiEdit/NotebookEdit
// are all registered in .claude/settings.json), and a human typing in their
// own terminal goes through none. Refusing the sentinel path here therefore
// makes the claim un-self-issuable by construction rather than by
// instruction — the property WRITEGUARD_SHARED_OK had for free (the hook
// reads its own process env, which no Bash call can change) and which the
// file gave away. The env token keeps working for human shells; the file is
// how a human claims it for a session, made out of band.
//
// This is a hook-level control, not a sandbox: like every other rule here it
// keys on the command string, so an agent that writes a helper script and
// runs it, or reaches the filesystem from an interpreter (python -c, go run),
// is not stopped — the same limit the whole guard already has. It removes the
// one-token reflex path, which is the failure mode observed.
func (c Config) CheckClaimSentinel(tool, target string) Verdict {
	if c.SentinelPath == "" {
		return Verdict{}
	}
	sentinel := resolvePath(c.SentinelPath)
	hitsSentinel := func(p string) bool { return samePath(p, sentinel) }

	if tool == "Bash" {
		// failSafe=false: an unidentifiable write target is not evidence of a
		// write to THIS one path, and blocking every such command machine-wide
		// would be absurd. The write-indicator scan keeps its own fail-safe.
		if name, hit := c.scanWriteIndicators(target, hitsSentinel, false); hit {
			return c.sentinelBlock(tool+" ("+name+")", target)
		}
		return Verdict{}
	}
	if hitsSentinel(resolveToken(target, c.Cwd)) {
		return c.sentinelBlock(tool, target)
	}
	return Verdict{}
}

func (c Config) sentinelBlock(tool, target string) Verdict {
	sentinel := sentinelPathForMessage()
	return Verdict{
		Block: true,
		Reason: fmt.Sprintf(`writeguard: BLOCKED — the shared-OK claim is HUMAN-ONLY.
This %s call would write the writeguard claim sentinel (%s):
  %s
No session can issue its own exemption. A claim that any tool call could set is not a gate: dispatched workers are shared-homed by construction (#1035) and are exactly the population the isolation backstop exists to constrain, so the claim is settable only OUT OF BAND — by a human, in their own terminal, where no hook runs:
  mkdir -p %s
  printf '%%s\n' <shared-checkout-path> > %s   # scoped to one checkout
  # or, to claim any checkout on this machine: touch %s
The claim expires after %s; renew or drop it the same way (this guard refuses both from a session).
Dispatched/worker session? The claim is not yours to make — isolate instead: git worktree add ../tracker-<name> -b <branch> origin/main (absolute sibling path) and work there.`,
			tool, sentinel, target,
			filepath.Dir(sentinel), sentinel, sentinel, sentinelTTL()),
	}
}

// blockEvasionGuidance is appended to every block message (#1193): it names
// the ONE sanctioned exit for an out-of-checkout false positive (re-issue the
// SAME command with targets the guard can resolve) and forbids substituting a
// different command. The 2026-08-16 incident's worker answered a
// false-positive `rm` block with `find … -delete` instead of re-issuing —
// this line turns that confusion into a correct self-service re-issue.
const blockEvasionGuidance = "If your write targets a path OUTSIDE the shared checkout, re-issue the same command with absolute target paths (or one `cd <abs-dir> && …` chain) — the guard admits those. Do NOT substitute a different command to achieve the write: a block is a stop signal, and substitution is an escalation-worthy policy violation."

func (c Config) block(tool, target string) Verdict {
	if c.sharedHomed() {
		// Shared-homed session WITHOUT the opt-in token (#1035): with the
		// token it would have been exempt and never reached here. The
		// generic message below would name the shared checkout as "your own
		// worktree" — wrong guidance for this population.
		return Verdict{
			Block: true,
			Reason: fmt.Sprintf(`writeguard: BLOCKED — isolation backstop (shared-homed exemption is OPT-IN, #1035).
This session is homed in the SHARED checkout (%s) and this %s call targets it:
  %s
The shared-homed exemption must be CLAIMED explicitly (cf. ASSAY_MAIN_COMMIT_OK), and only a HUMAN can claim it — no session, including this one, can issue its own (#1190 review). Both mechanisms live outside the tool surface:
  - a human shell: export WRITEGUARD_SHARED_OK=1 (once per shell); or
  - a human terminal, outside Claude Code (the env var cannot be set from the Bash tool — fresh shell per call, #1190):
      mkdir -p %s
      printf '%%s\n' %s > %s   # scoped to this checkout; an empty file claims any
    The sentinel expires after %s; renewing and dropping it are human acts too — this guard refuses tool-surface writes to that path.
Dispatched/worker session? The claim is not yours to make — isolate instead: create your own worktree (git worktree add ../tracker-<name> -b <branch> origin/main, absolute sibling path) and work there.`,
				c.SharedRoot, tool, target,
				filepath.Dir(sentinelPathForMessage()), c.SharedRoot, sentinelPathForMessage(), sentinelTTL()) +
				"\n" + blockEvasionGuidance,
		}
	}
	home := c.home()
	return Verdict{
		Block: true,
		Reason: fmt.Sprintf(`writeguard: BLOCKED — isolation backstop.
This session is homed in %s, but this %s call targets the SHARED checkout (%s):
  %s
Write ONLY inside your own worktree (%s). Never write to the shared checkout via absolute paths, and never cd into it — read it with git -C / absolute-path reads if you must. If this operation is genuinely meant for the shared checkout, it must run from a session homed there (the coordinator or a direct human session) with WRITEGUARD_SHARED_OK=1 exported.`,
			home, tool, c.SharedRoot, target, home) +
			"\n" + blockEvasionGuidance,
	}
}

// --- path helpers ---

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func underDir(p, dir string) bool {
	p, dir = filepath.Clean(p), filepath.Clean(dir)
	return p == dir || strings.HasPrefix(p, dir+string(filepath.Separator))
}

// expandHome expands a leading "~", "~/…" or "$HOME"/"$HOME/…" prefix to the
// real home directory. The bool reports whether expansion happened, in which
// case the result must be treated as an absolute path — NOT joined against
// cwd. ~/.claude/** and $HOME/.claude/** are outside the shared checkout
// (they live under the user's home, not the repo), and must never be
// resolved as if they were relative to the session's cwd (#742).
func expandHome(p string) (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p, false
	}
	switch {
	case p == "~":
		return home, true
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(home, p[len("~/"):]), true
	case p == "$HOME":
		return home, true
	case strings.HasPrefix(p, "$HOME/"):
		return filepath.Join(home, p[len("$HOME/"):]), true
	}
	return p, false
}

// resolveToken resolves a token from a command/file-tool payload to an
// absolute, symlink-resolved path: ~/$HOME-prefixed tokens expand to the
// real home directory (never cwd-relative), already-absolute tokens are
// used as-is, and everything else is joined against cwd (if known).
func resolveToken(tok, cwd string) string {
	p, abs := expandHome(tok)
	if !abs && !filepath.IsAbs(p) && cwd != "" {
		p = filepath.Join(cwd, p)
	}
	return resolvePath(p)
}

// resolvePath resolves symlinks in the deepest existing ancestor of p and
// re-joins the rest, so nonexistent targets (a Write creating a file) still
// resolve /tmp-style symlinks. On any failure it returns the cleaned input.
func resolvePath(p string) string {
	p = filepath.Clean(p)
	rest := ""
	cur := p
	for {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(r, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// redirectOperatorRe matches a run of one or more ">" characters, used to
// split a space-less redirect (echo c>>/shared/file, echo c>/shared/file)
// so the glued-on target path is recognized as its own token (#751) instead
// of staying fused to the preceding word and being missed entirely.
var redirectOperatorRe = regexp.MustCompile(`>+`)

// pathTokens extracts tokens from a shell command that look like filesystem
// paths — absolute, relative, or containing a directory separator. Tokens
// starting with "-" are skipped (flags/options). Embedded paths in key=value
// options are extracted by splitting on "="; a target glued directly to a
// redirection operator with no space (word>>path) is extracted by further
// splitting on runs of ">" (#751) — this only widens the CANDIDATE token
// set feeding the outer mentions pre-filter in CheckBash; the actual block
// verdict still requires checkWriteIndicators' own target-aware extraction
// to resolve the argument inside the shared checkout, so this cannot by
// itself turn a safe command into a false-positive block.
func pathTokens(cmd string) []string {
	fields := strings.Fields(cmd)
	var tokens []string
	for _, f := range fields {
		for _, eqPart := range strings.Split(f, "=") {
			for _, part := range redirectOperatorRe.Split(eqPart, -1) {
				part = strings.Trim(part, `"'`)
				// Strip trailing shell metacharacters that strings.Fields
				// cannot split on (e.g. pushd /p; cmd → token "/p;").
				part = strings.TrimRight(part, `;|&`)
				if part == "" || strings.HasPrefix(part, "-") {
					continue
				}
				if strings.HasPrefix(part, "/") || strings.HasPrefix(part, "./") ||
					strings.HasPrefix(part, "../") || strings.HasPrefix(part, "~/") ||
					strings.Contains(part, "/") {
					tokens = append(tokens, part)
				}
			}
		}
	}
	return tokens
}

// cdIntoDir reports whether cmd cd/pushd-es into the shared checkout
// (outside .claude/worktrees). Targets are resolved against cwd so that
// relative paths (../.., ./sibling) and dotted traversal are caught.
//
// The prefix class includes quote characters so a cd inside a quoted
// subshell string (`sh -c 'cd <shared> && …'`, `bash -c "cd <shared> && …"`)
// is caught (#1017 review): before #1006 the bare git/statusgen indicators
// incidentally blocked those shapes on mention alone; target-aware
// indicators removed that cover, so the cd itself must match. Those shapes
// survive the literal-masking below because maskLiterals deliberately keeps
// shell -c / eval arguments and shell-fed heredoc bodies visible; a cd in
// ordinary quoted prose is masked away and no longer matches (#1259 review).
//
// `env -C <dir>` / `env --chdir[= ]<dir>` runs its wrapped command with
// <dir> as working directory — a cd equivalent, handled here explicitly
// (#1017 review). Only tokens in env's own option/assignment prefix are
// inspected; scanning stops at the wrapped command so `env make -C <dir>`
// is NOT treated as a chdir (make -C builds there; unchanged behavior).
// env is matched by BASENAME (#1026): the prefix class includes "/" so a
// path-prefixed invocation (/usr/bin/env, /bin/env, quoted "/usr/bin/env")
// is recognized; the class + \b keep dotenv/myenv-style names from matching
// (the char before their "env" is a word char, not in the class).
//
// cd/pushd deliberately keep a "/"-free prefix class: they are shell
// builtins, and a path-prefixed cd binary runs in a child process — it
// cannot change the invoking shell's directory, so it is not a chdir
// equivalent (and /usr/bin/pushd does not exist).
func cdIntoDir(cmd, cwd, sharedRoot, worktreesDir string) bool {
	// An unresolvable cd destination is NOT read as a cd into the shared
	// checkout: joining it onto cwd would invent <shared>/$S. That stays
	// fail-open safely because an unresolvable cd never advances the frame in
	// effectiveCwd either, so every relative write after it is still judged
	// against the payload cwd (pinned by the "cd variable then …" cases in
	// TestIssue1190TruePositivesPreserved). A cd whose destination the command
	// itself spells out (`D=<shared>; cd $D`) now resolves and blocks.
	inShared := func(target string, pos int) bool {
		t, ok := expandTargetHead(cmd, pos, target, cwd)
		if !ok {
			return false
		}
		p := resolveToken(t, cwd)
		return underDir(p, sharedRoot) && !underDir(p, worktreesDir)
	}

	// The cd scan runs over the LITERAL-MASKED command (#1259 review, B1).
	// Every other rule in this PR moved to masked text — indicators scan
	// maskLiterals, effectiveCwd scans maskQuoted — and this one still read the
	// raw command, so a cd inside quoted prose or a heredoc body was read as a
	// real cd, resolved against the shared cwd, and blocked commands that write
	// nothing: `grep -n 'cd ..' f.go`, `gh issue create --body "run: cd docs &&
	// ls"`, a `gh pr comment --body-file - <<EOF` body quoting a shell recipe.
	// That is #1190 finding 1 (a tool-shaped string inside an issue body)
	// surviving in the one code path masking never reached, and quoting a shell
	// recipe in an issue or PR body is a daily desk shape.
	//
	// maskLiterals — NOT maskQuoted — is what keeps the true positives: the
	// argument of `sh -c`/`bash -c`/`eval` and a shell-fed heredoc body stay
	// VISIBLE through it, so `sh -c 'cd <shared> && rm -rf docs'` and
	// `bash <<EOF … cd <shared> …` still block. maskLiterals preserves length
	// and byte offsets, so targets are still read from the RAW command.
	for _, idx := range cdTargetRe.FindAllStringSubmatchIndex(maskLiterals(cmd), -1) {
		target := cdTarget(cmd, idx)
		if target == "" {
			continue
		}
		if inShared(target, idx[0]) {
			return true
		}
	}

	envRe := regexp.MustCompile(`(?:^|[\s;&|('"/])env\b`)
	for _, loc := range envRe.FindAllStringIndex(cmd, -1) {
		fields := strings.Fields(segmentAfter(cmd, loc[1]))
	scan:
		for i := 0; i < len(fields); i++ {
			f := strings.Trim(fields[i], `"'`)
			switch {
			case f == "":
				// A bare quote token (e.g. the closing quote of a quoted
				// "/usr/bin/env") trims to empty — not the wrapped command,
				// keep scanning (#1026).
			case f == "-C" || f == "--chdir":
				if i+1 < len(fields) {
					i++
					if inShared(strings.Trim(fields[i], `"'`), loc[0]) {
						return true
					}
				}
			case strings.HasPrefix(f, "--chdir="):
				if inShared(strings.Trim(f[len("--chdir="):], `"'`), loc[0]) {
					return true
				}
			case f == "-u" || f == "-S" || f == "--split-string" || f == "--unset":
				i++ // option that consumes a value argument
			case strings.HasPrefix(f, "-"), strings.Contains(f, "="):
				// other env option / NAME=value assignment — keep scanning
			default:
				// the wrapped command itself — env's option prefix is over
				break scan
			}
		}
	}
	return false
}

// --- write indicators ---
//
// EVERY indicator is target-aware (#1006): we resolve the indicator's own
// write target (expanding ~/$HOME and joining relative paths against cwd)
// and only block if THAT target lands inside the shared checkout. A
// write-shaped command whose own argument points at ~/.claude/... (or
// anywhere else outside the checkout) is not a hit, no matter what cwd
// happens to be (#742) — and a pure READ mention of the shared path
// (git -C <shared> fetch, cat <shared>/x) never arms an indicator whose
// write lands in the caller's own worktree (#1006).
//
// Redirection, tee, cp/mv/rm-family, and sed -i carry explicit target
// arguments. Mutating git keys on its -C/--git-dir/--work-tree argument
// (falling back to cwd when absent); statusgen keys on --root (falling
// back to cwd); a build tool writes relative to cwd. Indicators that fall
// back to cwd return nil when cwd is unknown — a fail-safe hit.
type indicatorSpec struct {
	name        string
	re          *regexp.Regexp
	targetAware bool
	// extract returns the candidate (unresolved) target tokens for a match
	// spanning [matchStart, matchEnd) in cmd. A nil return means no target
	// could be identified — the caller treats that as a fail-safe hit.
	// Only called when targetAware.
	extract func(c Config, cmd string, matchStart, matchEnd int) []string
}

// cwdTargets is the target set for indicators that write relative to the
// session cwd with no explicit target argument in the command. Unknown cwd
// means the write destination cannot be established — return nil so the
// caller fails safe (#1006 makes indicators target-aware without ever
// guessing a target). "." is resolved by the caller against the EFFECTIVE
// cwd for the matched position (#1190), so a `cd <worktree> && …` prefix is
// honored — including when the payload cwd is unknown but the cd is absolute.
func (c Config) cwdTargets(cmd string, pos int) []string {
	if c.effectiveCwd(cmd, pos) == "" {
		return nil
	}
	return []string{"."}
}

// cdTargetRe matches a cd/pushd and captures its target — single-quoted (g2),
// double-quoted (g3) or bare (g4). Shared by cdIntoDir and effectiveCwd.
var cdTargetRe = regexp.MustCompile(`(?:^|[\s;&|('"])(cd|pushd)\s+(?:'([^']*)'|"([^"]*)"|([^\s;&|)]+))`)

// --- head-expansion resolution (#1190, and its review) ---
//
// A path token whose HEAD is a shell expansion ($SCRATCH/out, ${DIR},
// $(mktemp -d), `pwd`/x) has no literal prefix to judge, and joining it onto
// cwd INVENTS a path: from a cwd inside the shared checkout, `cd $S` and
// `… > "$S"/out.go` "resolved" to <shared>/$S and blocked commands that go
// somewhere else entirely (#1190).
//
// But treating every such head as unknowable was too broad, and it opened
// nine genuine shared-checkout writes the guard used to block (PR #1259
// review): `D=<shared>; echo pwn > $D/STATUS.md` and its rm/tee/sed/git -C
// siblings, `echo pwn > ` + "`pwd`" + `/STATUS.md`, and `D=$HOME/…`. Those
// heads are not unknowable at all — the command says what they are. So the
// guard now RESOLVES what it can, in this order:
//
//   - ~/$HOME prefixes (expandHome, as before);
//   - $(pwd) / `pwd` / $PWD → the effective cwd for the match position;
//   - $NAME / ${NAME} → the last literal NAME=value assignment made EARLIER in
//     the same command (read from the literal-masked command, so an assignment
//     quoted inside an issue body is prose and cannot define a variable),
//     recursively (D=$HOME/x), else the hook process environment, which the
//     Bash tool's shell inherits (so $CLAUDE_PROJECT_DIR resolves).
//
// Only a head that survives all of that is genuinely unknown, and callers
// choose what an unknown head means (see expandTargetHead's callers): the
// write-indicator scan falls back to the pre-#1190 cwd-relative resolution —
// fail-safe, blocking exactly when the frame is inside the shared checkout —
// while cd targets stay fail-open, which cannot launder a write because an
// unresolvable cd never advances the frame the following writes are judged in.
//
// Only a HEAD expansion is ever unknown. A token whose literal prefix already
// lands inside the shared checkout (<shared>/$SUB/x) still blocks: the
// expansion can only go deeper, and that prefix is the evidence.

var (
	// varHeadRe matches a whole-token variable reference: $NAME or ${NAME}.
	varHeadRe = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$|^\$([A-Za-z_][A-Za-z0-9_]*)$`)
	// assignmentRe matches a NAME=value shell assignment (bare or quoted value).
	assignmentRe = regexp.MustCompile(`(?:^|[\s;&|(])([A-Za-z_][A-Za-z0-9_]*)=('[^']*'|"[^"]*"|[^\s;&|)]*)`)
)

// pwdHeads are the expansions that mean "the current directory".
var pwdHeads = map[string]bool{"$(pwd)": true, "`pwd`": true, "$PWD": true, "${PWD}": true}

// shellDynamicVars are set per-shell by the shell itself. The hook process has
// its OWN value for them, which says nothing about the value in the Bash
// tool's shell, so they are never resolved from the environment — `cd $OLDPWD`
// moves somewhere this guard cannot model and must fail safe (#1259 red
// team), not be "resolved" to the hook's own OLDPWD.
var shellDynamicVars = map[string]bool{
	"OLDPWD": true, "_": true, "RANDOM": true, "SECONDS": true,
	"LINENO": true, "REPLY": true, "PPID": true, "BASHPID": true,
	"SHLVL": true, "PIPESTATUS": true, "FUNCNAME": true, "SRANDOM": true,
}

// expandTargetHead resolves a leading shell expansion in tok, using
// assignments made before pos in cmd and the hook's own environment. The bool
// reports whether the head is now literal; on false the token's destination is
// genuinely unknown.
func expandTargetHead(cmd string, pos int, tok, cwd string) (string, bool) {
	return expandTargetHeadDepth(cmd, pos, tok, cwd, 0)
}

func expandTargetHeadDepth(cmd string, pos int, tok, cwd string, depth int) (string, bool) {
	if p, expanded := expandHome(tok); expanded {
		tok = p
	}
	head, rest := tok, ""
	if i := strings.IndexByte(tok, '/'); i >= 0 {
		head, rest = tok[:i], tok[i:]
	}
	if !strings.ContainsAny(head, "$`") {
		return tok, true
	}
	if depth > 4 { // assignment cycle / pathological nesting
		return "", false
	}
	if pwdHeads[head] {
		if cwd == "" {
			return "", false
		}
		return cwd + rest, true
	}
	m := varHeadRe.FindStringSubmatch(head)
	if m == nil {
		return "", false
	}
	name := m[1] + m[2] // exactly one of the two alternatives matched
	val, ok := assignedValue(cmd, pos, name)
	if !ok {
		if shellDynamicVars[name] {
			return "", false
		}
		if val = os.Getenv(name); val == "" {
			return "", false
		}
	}
	resolved, ok := expandTargetHeadDepth(cmd, pos, val, cwd, depth+1)
	if !ok {
		return "", false
	}
	return resolved + rest, true
}

// assignedValue returns the value of the last NAME=value assignment made
// before pos in cmd. Assignments are located in the LITERAL-MASKED command —
// `gh issue create --body 'D=/tmp' && …` must not define D — while the value
// itself is read from the raw command at the same offsets (masking preserves
// length and position).
func assignedValue(cmd string, pos int, name string) (string, bool) {
	val, found := "", false
	for _, m := range assignmentRe.FindAllStringSubmatchIndex(maskLiterals(cmd), -1) {
		if m[0] >= pos {
			break
		}
		if cmd[m[2]:m[3]] != name {
			continue
		}
		val, found = strings.Trim(cmd[m[4]:m[5]], `"'`), true
	}
	return val, found
}

// cdTarget returns the target captured by a cdTargetRe submatch-index set.
func cdTarget(cmd string, idx []int) string {
	for g := 4; g <= 8; g += 2 {
		if g < len(idx) && idx[g] >= 0 {
			return cmd[idx[g]:idx[g+1]]
		}
	}
	return ""
}

// effectiveCwd returns the directory that relative paths at byte offset pos
// resolve against.
//
// #1190 repro 4: an agent Bash call runs a fresh shell started in the HOOK
// PAYLOAD cwd, which for a dispatched worker is frequently the shared
// checkout even though the worker owns a worktree elsewhere. Such a worker
// writes `cd /abs/worktree && go run ./tools/statusgen --root .` — resolving
// `.` against the payload cwd blamed the write on the shared checkout while
// it actually lands in the worktree.
//
// The frame therefore advances, but ONLY where shell semantics guarantee the
// shell's cwd really moved. The first version of this advanced on any
// TEXTUALLY PRECEDING cd, which is simply not what a shell does, and the
// red-team pass on PR #1259 executed eight shapes that exploited the gap —
// each one blocked before this PR, admitted at head, and each observed
// destroying tracked edits or untracked files in a fixture:
//
//	cd /path/does-not-exist; git clean -fdx      (the cd FAILS; ; runs anyway)
//	(cd <wt>) ; git clean -fdx                   (subshell)
//	sh -c 'cd <wt>' ; git clean -fdx             (child process)
//	pushd <wt>; popd; git checkout -- .          (pushed and popped back)
//	cd <wt> && cd $OLDPWD && git clean -fdx      (moved somewhere unmodelled)
//	if false; then cd <wt>; fi; echo pwn > STATUS.md   (branch not taken)
//	cd <wt> && cd ${X:-<shared>} && rm -rf docs  (unmodelled expansion)
//	S=<shared>; cd <wt>; cd $S; rm -rf docs      (; and an indirection)
//
// The first row is the one that matters most and is not an attack at all: a
// worker whose worktree path is stale or already pruned silently gets its
// `git clean` applied to the shared checkout. That is precisely the
// accidental-data-loss case the isolation backstop exists for.
//
// The rule that survives all eight: advance the frame only along a LEADING
// &&-CHAIN of literal cd commands — every top-level separator between the
// start of the command and the cd is `&&`, the cd is not inside quotes,
// parentheses or braces, and its target resolves. `&&` is what makes it
// sound: if the cd fails, nothing after it runs, so the guard's verdict about
// what follows is moot. ANY other cwd-changing construct before pos (a `;`- or
// newline-separated cd, a subshell cd, pushd/popd, an unresolvable target)
// means the guard cannot model the shell's cwd, and the frame FAILS SAFE back
// to the payload cwd — the pre-#1190 behaviour, which blocked all eight rows.
//
// The scan runs over the QUOTE-MASKED command: a cd inside any quoted span is
// either prose (`gh issue create --body 'cd /tmp' && echo x > STATUS.md` must
// not launder a relative shared write) or a child shell's cd (`sh -c 'cd …'`),
// and neither moves this shell. Indices are preserved by masking, so targets
// are still read from the raw command. A cd INTO the shared checkout is
// blocked outright by cdIntoDir before any indicator runs, so the frame can
// only ever move OUT of the shared checkout, never into it.
func (c Config) effectiveCwd(cmd string, pos int) string {
	masked := maskQuoted(cmd)
	frame := c.Cwd
	// Offsets of every visible cd/pushd/popd token, and whether it is a plain
	// top-level `cd` whose whole &&-chain from the start of the command is
	// intact (honorable) — see cdChainScan.
	for _, cd := range cdChainScan(masked) {
		if cd.at >= pos {
			break
		}
		if !cd.honorable {
			return c.Cwd
		}
		target := cdTarget(cmd, cd.idx)
		if target == "" || target == "-" {
			return c.Cwd
		}
		t, ok := expandTargetHead(cmd, cd.at, target, frame)
		if !ok {
			return c.Cwd // destination unmodelled: fail safe to the payload cwd
		}
		if _, homeAbs := expandHome(t); !homeAbs && !filepath.IsAbs(t) && frame == "" {
			return c.Cwd // relative cd from an unknown cwd
		}
		frame = resolveToken(t, frame)
	}
	return frame
}

// cdOccurrence is one visible cd/pushd/popd in a command.
type cdOccurrence struct {
	at        int   // byte offset of the cd/pushd/popd keyword
	idx       []int // cdTargetRe submatch indices ("" for pushd/popd matches)
	honorable bool  // a plain `cd` reached through an unbroken top-level &&-chain
}

// pushdPopdRe matches the cwd-changing builtins effectiveCwd refuses to model.
var pushdPopdRe = regexp.MustCompile(`(?:^|[\s;&|('"])(pushd|popd|dirs)\b`)

// cdChainScan finds every visible cwd-changing construct in the quote-masked
// command, in order, marking those that sit at depth 0 with only `&&`
// separators between them and the start of the command.
func cdChainScan(masked string) []cdOccurrence {
	// Top-level separators, in order, and the nesting depth at every byte.
	type sep struct {
		at    int
		isAnd bool
	}
	var seps []sep
	depth := make([]int, len(masked)+1)
	d := 0
	for i := 0; i < len(masked); i++ {
		depth[i] = d
		switch masked[i] {
		case '(', '{':
			d++
		case ')', '}':
			if d > 0 {
				d--
			}
		case '&':
			if d != 0 || (i > 0 && masked[i-1] == '&') {
				break // second byte of `&&`, or nested
			}
			if i+1 < len(masked) && masked[i+1] == '&' {
				seps = append(seps, sep{i, true})
			} else {
				seps = append(seps, sep{i, false}) // background `&`
			}
		case ';', '|', '\n':
			if d == 0 {
				seps = append(seps, sep{i, false})
			}
		}
	}
	depth[len(masked)] = d

	// A cd is honorable only when the shell is guaranteed to have moved for
	// everything that follows: every separator BEFORE it is `&&` (so it ran at
	// all) and the separator that joins it to the REST is `&&` (so the rest
	// runs only if the cd SUCCEEDED). `cd /stale-path; git clean -fdx` fails
	// the second test — the cd can fail and the clean still runs, in the
	// shared checkout.
	chainIntact := func(at int) bool {
		after := true
		for _, s := range seps {
			if s.at < at {
				if !s.isAnd {
					return false
				}
				continue
			}
			// first separator after the cd
			after = s.isAnd
			break
		}
		return after
	}

	var out []cdOccurrence
	for _, idx := range cdTargetRe.FindAllStringSubmatchIndex(masked, -1) {
		at := idx[2] // start of the cd/pushd keyword
		kw := masked[idx[2]:idx[3]]
		out = append(out, cdOccurrence{
			at:        at,
			idx:       idx,
			honorable: kw == "cd" && depth[at] == 0 && chainIntact(at),
		})
	}
	for _, loc := range pushdPopdRe.FindAllStringIndex(masked, -1) {
		out = append(out, cdOccurrence{at: loc[0], honorable: false})
	}
	sortOccurrences(out)
	return out
}

func sortOccurrences(o []cdOccurrence) {
	for i := 1; i < len(o); i++ {
		for j := i; j > 0 && o[j].at < o[j-1].at; j-- {
			o[j], o[j-1] = o[j-1], o[j]
		}
	}
}

// --- literal-text masking (#1190) ---
//
// A write indicator must mean a WRITE. Matching indicator keywords inside
// LITERAL TEXT — a quoted awk program (`awk 'NF>1 {print $1}'` is not a
// redirect), a `gh issue create --body '> quoted line'` (a markdown
// blockquote, not a redirect), an issue body naming the board generator —
// blocked commands that write nothing at all. maskLiterals blanks quoted
// spans and heredoc bodies (preserving length, so match indices still map
// onto the raw command) BEFORE the indicator regexes run.
//
// Carve-outs keep the genuine shapes visible, because some quoted/heredoc
// text IS a command: the argument of `sh -c` / `bash -c`, and a heredoc fed
// to a shell. `sh -c 'rm <shared>/x'` still blocks.

var (
	// shellCArgRe matches a shell -c invocation (or an `eval`) ending right at
	// a quote — the quoted span that follows is a command, not literal text.
	// `eval` was missing (#1190 review): `eval 'rm -rf <shared>/dist'` had its
	// argument masked away as prose and was admitted, while the identical
	// `sh -c 'rm -rf <shared>/dist'` blocked.
	shellCArgRe = regexp.MustCompile(`(^|[\s;&|(])(([^\s;&|()]*/)?(sh|bash|zsh|dash|ksh|ash)\s+(-\S+\s+)*-[a-zA-Z]*c|eval)\s+$`)
	// shellNameRe matches a bare shell command name (used for heredocs fed to
	// a shell, whose body is a script rather than literal text).
	shellNameRe = regexp.MustCompile(`^([^\s;&|()]*/)?(sh|bash|zsh|dash|ksh|ash)$`)
	// heredocRe matches a heredoc introducer: <<WORD, <<-WORD, <<'WORD', <<"WORD".
	heredocRe = regexp.MustCompile(`<<-?\s*(['"]?)([A-Za-z_][A-Za-z0-9_]*)['"]?`)
	// envAssignRe matches a leading NAME=value shell assignment.
	envAssignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
)

// maskLiterals blanks heredoc bodies then quoted spans. Length and byte
// offsets are preserved so a match found in the masked string indexes the
// same bytes in the raw command.
func maskLiterals(cmd string) string {
	return maskQuotedKeepShellArg(maskHeredocBodies(cmd))
}

// maskHeredocBodies blanks the body of every heredoc whose introducing
// command is not a shell (a heredoc piped into sh/bash is a script and stays
// visible). Newlines are preserved so line structure survives.
//
// The delimiter's QUOTING is load-bearing (#1259 delta review, blocking): with
// an UNQUOTED word (<<EOF) the shell expands the body — `$( … )` runs — before
// the introducing command is even executed, so a substitution there is
// executed code no matter what that command is. `cat <<EOF … $(rm -rf
// <shared>/docs) … EOF` genuinely deletes; `cat` never sees it. A QUOTED word
// (<<'EOF' or <<"EOF") suppresses expansion entirely, so its body really is
// literal text and blanking it stays correct — which is what keeps the
// `gh pr comment --body-file - <<'EOF' … EOF` prose case admitted.
//
// This is the same reasoning maskQuotedKeepShellArg applies to `$( … )` inside
// double quotes, on the other masking path. EXPANSION MEANS ALL EXPANSION:
// backticks are the same construct and are kept visible too (security review) —
// treating `$( … )` as the only executed form while the shell also runs
// “ ` … ` “ was a fail-open this PR introduced against main. `${ … }` is
// parameter expansion, not command execution, and stays masked.
//
// The blank/keep sets are computed for ALL heredocs before ANY byte is written
// (security review). Two heredocs on one line share a `bodyStart`, so the
// old per-match write let a later QUOTED delimiter's blanking overwrite the
// `$( … )` bytes an earlier UNQUOTED one deliberately preserved —
// `cat <<EOF <<'Z'` / `$(rm -rf <shared>/docs)` / `EOF` executed and was
// admitted. Keep wins globally, in either delimiter order: a byte any pass
// reads as executed code is never blanked by another.
func maskHeredocBodies(cmd string) string {
	b := []byte(cmd)
	blank := make([]bool, len(cmd))
	keep := make([]bool, len(cmd))
	// A `<<` that is itself inside a quoted span or a comment does not
	// introduce a heredoc (#1259 red team): detection used to run on raw
	// text with no such test, so one incidental mention — `# see <<EOF for the
	// format`, or `echo "uses <<EOF here"` — blanked every following line, and
	// a real `rm -rf <shared>/docs` underneath it went unseen. The scan itself
	// stays on the RAW command: a heredoc word is routinely quoted (<<'EOF'),
	// and quote-masking would erase the word before it could be read.
	masked := maskQuoted(cmd)
	for _, m := range heredocRe.FindAllStringSubmatchIndex(cmd, -1) {
		if inComment(masked, m[0]) || quoteStateAt(cmd, m[0]) != 0 {
			continue
		}
		// Group 1 is the opening quote of the heredoc word, captured but until
		// now discarded: non-empty means <<'EOF' / <<"EOF" (no expansion).
		quotedDelim := m[3] > m[2]
		word := cmd[m[4]:m[5]]
		lineStart := strings.LastIndexByte(cmd[:m[0]], '\n') + 1
		if fields := strings.Fields(cmd[lineStart:m[0]]); len(fields) > 0 &&
			shellNameRe.MatchString(strings.Trim(fields[0], `"'`)) {
			continue
		}
		nl := strings.IndexByte(cmd[m[1]:], '\n')
		if nl < 0 {
			continue // no body on this command line
		}
		bodyStart := m[1] + nl + 1
		end := len(cmd)
		for i := bodyStart; i < len(cmd); {
			line, next := cmd[i:], len(cmd)
			if le := strings.IndexByte(cmd[i:], '\n'); le >= 0 {
				line, next = cmd[i:i+le], i+le+1
			}
			if strings.TrimSpace(line) == word {
				end = i
				break
			}
			i = next
		}
		for j := bodyStart; j < end; j++ {
			blank[j] = true
		}
		if !quotedDelim {
			markExpansions(cmd, bodyStart, end, keep)
		}
	}
	for j := 0; j < len(cmd); j++ {
		if blank[j] && !keep[j] && cmd[j] != '\n' {
			b[j] = ' '
		}
	}
	return string(b)
}

// markExpansions marks every byte of every command substitution in
// cmd[start:end] — `$( … )` and “ ` … ` “ — as executed code that masking
// must leave visible.
//
// The `$( … )` walk is depth-counted so a nested substitution does not end the
// span early, and quote-aware inside the span so a literal `)` in the payload
// (`$(echo ')' ; rm -rf <shared>/docs)`) cannot end it either. Backticks do not
// nest, so theirs is a simple toggle. An unterminated span of either kind
// leaves the rest of the body visible, which fails closed.
func markExpansions(cmd string, start, end int, keep []bool) {
	subst := 0
	var inner byte
	tick := false
	for j := start; j < end; j++ {
		c := cmd[j]
		switch {
		case tick:
			keep[j] = true
			if c == '`' && !oddBackslashesBefore(cmd, j) {
				tick = false
			}
		case subst > 0:
			keep[j] = true
			switch {
			case inner != 0:
				if c == inner && !oddBackslashesBefore(cmd, j) {
					inner = 0
				}
			case c == '\'' || c == '"':
				inner = c
			case c == '(':
				subst++
			case c == ')':
				subst--
			}
		case c == '$' && j+1 < end && cmd[j+1] == '(' && !oddBackslashesBefore(cmd, j):
			keep[j], keep[j+1] = true, true
			subst, inner = 1, 0
			j++ // the '(' is accounted for by subst; don't count it twice
		case c == '`' && !oddBackslashesBefore(cmd, j):
			keep[j] = true
			tick = true
		}
	}
}

// opensComment reports whether the byte at i is a `#` that STARTS a shell
// comment: it must begin a word, and the caller must already know i is not
// inside a quoted span. `echo a#b` is one word, not a comment.
//
// #1259 security review: the quote scanners had no comment rule at all, so
// the apostrophe in `# don't do this by hand` opened a masking span and blanked
// every following line — including a real `rm -rf <shared>/docs`. A comment
// executes nothing, and a quote inside one is not a quote.
func opensComment(cmd string, i int) bool {
	return cmd[i] == '#' && (i == 0 || cmd[i-1] == ' ' || cmd[i-1] == '\t' || cmd[i-1] == '\n')
}

// closesQuote reports whether the byte at i closes the currently-open span.
// Inside double quotes `\"` is a literal, so an escaped quote does not close;
// inside single quotes the shell honours NO escapes, so `'a\'` really does end
// there. Getting this wrong desynchronises every later span.
func closesQuote(cmd string, i int, quote byte) bool {
	return cmd[i] == quote && !(quote == '"' && oddBackslashesBefore(cmd, i))
}

// opensQuote reports whether the byte at i opens a quoted span. A
// backslash-escaped quote is a literal character and opens nothing — `echo
// it\'s ; rm -rf <shared>/docs` is one command with balanced quoting, and
// reading that `'` as an opener blanked the `rm`.
func opensQuote(cmd string, i int) bool {
	return (cmd[i] == '\'' || cmd[i] == '"') && !oddBackslashesBefore(cmd, i)
}

// quoteStateAt returns the quote character enclosing offset i in cmd, or 0
// when i is outside any quoted span. Escape- and comment-aware, on the same
// rules as the maskers.
func quoteStateAt(cmd string, i int) byte {
	var quote byte
	for j := 0; j < i && j < len(cmd); j++ {
		switch {
		case quote != 0:
			if closesQuote(cmd, j, quote) {
				quote = 0
			}
		case opensComment(cmd, j):
			for j < i && j < len(cmd) && cmd[j] != '\n' {
				j++
			}
		case opensQuote(cmd, j):
			quote = cmd[j]
		}
	}
	return quote
}

// inComment reports whether offset i sits after an unquoted `#` that starts a
// word on the same line — i.e. inside a shell comment.
func inComment(masked string, i int) bool {
	start := strings.LastIndexByte(masked[:i], '\n') + 1
	for j := start; j < i; j++ {
		if masked[j] == '#' && (j == start || masked[j-1] == ' ' || masked[j-1] == '\t') {
			return true
		}
	}
	return false
}

// maskQuotedKeepShellArg blanks the contents of quoted spans (keeping the
// quote characters, like maskQuoted) EXCEPT three kinds of span whose contents
// are themselves a command: the argument of a shell -c invocation (or `eval`),
// a quoted COMMAND NAME, and a `$( … )` substitution inside double quotes.
//
// The command-name carve-out closes #1259 red-team: masking a quoted span
// unconditionally meant `"rm" -rf docs/streams` and `echo x | "tee" STATUS.md`
// lost their verb and were admitted, though both blocked before this PR. A
// quoted span in command position is never the prose the masking exists for.
//
// The substitution carve-out closes the #1259 delta review's blocking finding:
// a `$( … )` inside double quotes is neither prose nor an inert argument — the
// shell EXECUTES it before the surrounding command ever runs. Masking it
// admitted `echo "$(cd <shared> && rm -rf docs)"` and
// `echo "$(echo pwn > <shared>/STATUS.md)"`, both of which genuinely delete and
// overwrite. The neighbouring forms were already correct — unquoted
// `echo $(…)` is never masked, and `X="$(…)"` is kept by the command-position
// rule — so only "double-quoted substitution passed to a non-shell command"
// leaked. Single quotes need no equivalent: the shell performs no substitution
// inside them. BACKTICKS are the same construct and get the same carve-out:
// “ echo "`git -C <shared> clean -fdx`" “ executes too, and masking it was a
// fail-open this PR introduced against main (security review).
//
// THIS SCAN IS THE WRITE DETECTOR'S INPUT (scanWriteIndicators →
// maskLiterals), so every misjudgement here is an ADMIT. It therefore fails
// CLOSED in two ways the shell's own parser does not need to:
//
//   - a span is opened/closed on the shell's rules — an escaped `\'` opens
//     nothing, an escaped `\"` inside double quotes closes nothing, and a
//     quote inside a `#` comment is not a quote (security review);
//   - if the scan ends with a span still OPEN, the masker did not understand
//     the command, so it must not claim the tail was prose. The tail is
//     restored verbatim and the indicators judge it. Blanking on a parse the
//     masker could not finish is precisely the fail-open.
//
// Deliberately NOT mirrored in maskQuoted: that one feeds effectiveCwd, where a
// `cd` inside a substitution must stay HIDDEN. `$( … )` runs in a subshell, so
// its `cd` does not move the parent frame, and `echo "$(cd /tmp)" && rm -rf docs`
// must not be read as having moved it.
//
// This is defence in depth, not the sole thing standing between the guard and a
// fail-open (#1259 delta review — mirroring the carve-out into
// maskQuoted survived, and the reviewer chased down why): cdChainScan
// independently requires depth[at] == 0, and a revealed `$(` increments that
// depth, so the `cd` is non-honorable and the frame falls back to the payload cwd
// anyway. Keep both — but do not over-trust either one, or conclude the other is
// redundant.
func maskQuotedKeepShellArg(cmd string) string {
	b := []byte(cmd)
	var quote byte
	keep := false
	spanStart := 0 // opening quote of the span currently being blanked
	subst := 0     // open `$( … )` parens within the current double-quoted span
	var inner byte // quote open INSIDE that substitution, if any
	tick := false  // inside a backtick substitution within that span
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case tick:
			// Backticks do not nest; the span ends at the next unescaped one.
			// An unterminated backtick leaves the rest visible — fail closed.
			if c == '`' && !oddBackslashesBefore(cmd, i) {
				tick = false
			}
		case subst > 0:
			// Inside an executed substitution: leave every byte visible, and
			// do not let its own quoting close the enclosing span. Parens
			// inside the substitution's OWN quotes are literals, not
			// structure — counting them would let
			// `echo "$(echo ')' ; rm -rf <shared>/docs)"` end the span early
			// and hide the payload in the re-masked tail.
			switch {
			case inner != 0:
				if c == inner && !oddBackslashesBefore(cmd, i) {
					inner = 0
				}
			case c == '\'' || c == '"':
				inner = c
			case c == '(':
				subst++
			case c == ')':
				subst--
			}
		case quote != 0:
			if quote == '"' && c == '$' && i+1 < len(cmd) && cmd[i+1] == '(' && !oddBackslashesBefore(cmd, i) {
				subst, inner = 1, 0
				i++ // the '(' is accounted for by subst; don't count it twice
			} else if quote == '"' && c == '`' && !oddBackslashesBefore(cmd, i) {
				tick = true
			} else if closesQuote(cmd, i, quote) {
				quote = 0
			} else if !keep && c != '\n' {
				b[i] = ' '
			}
		case opensComment(cmd, i):
			// A comment executes nothing, so its text is prose and is blanked —
			// but its quotes must not open a span.
			j := i
			for j < len(cmd) && cmd[j] != '\n' {
				j++
			}
			for k := i; k < j; k++ {
				b[k] = ' '
			}
			i = j - 1 // the loop's i++ lands on the newline
		case opensQuote(cmd, i):
			quote = c
			spanStart = i
			keep = shellCArgRe.MatchString(cmd[:i]) || inCommandPosition(cmd, i)
		}
	}
	if quote != 0 && !keep {
		// Unterminated span: the masker never found the end, so everything it
		// blanked from spanStart on was a guess. Undo it and let the indicators
		// see the raw text — fail closed.
		copy(b[spanStart:], cmd[spanStart:])
	}
	return string(b)
}

// inCommandPosition reports whether offset i begins the first word of a shell
// segment (leading NAME=value assignments and command wrappers aside) — the
// position a command name occupies.
func inCommandPosition(cmd string, i int) bool {
	for _, f := range strings.Fields(segmentBefore(cmd, i)) {
		f = strings.Trim(f, `"'`)
		if f == "" || envAssignRe.MatchString(f) || commandWrappers[filepath.Base(f)] || strings.HasPrefix(f, "-") {
			continue
		}
		return false
	}
	return true
}

// --- invocation detection (#1190) ---

// isTokenBoundary reports whether c ends a shell word.
func isTokenBoundary(c byte) bool {
	switch c {
	case ' ', '\t', '\n', ';', '&', '|', ')', '(', '\'', '"', '`':
		return true
	}
	return false
}

// tokenStart walks back to the first byte of the shell word containing i.
func tokenStart(cmd string, i int) int {
	for i > 0 && !isTokenBoundary(cmd[i-1]) {
		i--
	}
	return i
}

// segmentBefore returns the text from the start of the current shell segment
// (after the previous unquoted ; & | ( { or newline) up to before.
func segmentBefore(cmd string, before int) string {
	if before <= 0 || before > len(cmd) {
		return ""
	}
	masked := maskQuoted(cmd)
	for i := before - 1; i >= 0; i-- {
		switch masked[i] {
		case ';', '&', '|', '(', '{':
			return cmd[i+1 : before]
		case '\n':
			// A line continuation joins the two lines into one segment.
			if !escapedNewline(masked, i) {
				return cmd[i+1 : before]
			}
		}
	}
	return cmd[:before]
}

// commandWrappers run the command that follows them, so a tool name after
// one is still in command position (`nohup statusgen &`, `sudo bazel build`).
// Kept small and exact: each of these takes the wrapped command as its next
// non-flag word.
var commandWrappers = map[string]bool{
	"env": true, "nohup": true, "time": true, "command": true,
	"exec": true, "sudo": true, "nice": true, "stdbuf": true,
	// #1259 red team: each of these was observed running the board
	// generator with the guard admitting the call.
	"timeout": true, "xargs": true, "caffeinate": true, "arch": true,
	"setsid": true, "ionice": true,
}

// wrapperValueArgs are wrappers whose first non-flag word is their OWN
// argument, not the wrapped command (`timeout 60 statusgen`).
var wrapperValueArgs = map[string]bool{"timeout": true}

// isInvocation reports whether a tool-name match spanning [start,end) is an
// actual invocation of that tool rather than a mention of its name:
//
//   - the name must END its own token — `<worktree>/tools/statusgen/x.go` is
//     a path that runs nothing; and
//
//   - the token must sit in COMMAND position — start of a segment (after
//     leading NAME=value assignments or `env`), or the package argument of
//     `go run`. `grep -rn statusgen <file>` runs grep, not the generator.
//
//     The "or the package argument of `go run`" clause is deliberately an
//     ALLOWLIST of exactly one go subcommand, not a denylist of read-only
//     ones: `go test ./tools/statusgen`, `go vet ./tools/statusgen`, `go
//     build ./tools/statusgen` all fail this test and are never invocations
//     (#147) — none of them execute the package's main(), so none of them
//     write STATUS.md, and the guard has no business asking whether their
//     nonexistent write target lands in the shared checkout.
func isInvocation(cmd string, start, end int) bool {
	if end < len(cmd) && !isTokenBoundary(cmd[end]) {
		return false
	}
	tokStart := tokenStart(cmd, start)
	// The tool name is the first word of a shell -c argument (`sh -c
	// 'statusgen'`, `bash -lc 'statusgen --root .'`): the masking carve-out
	// deliberately keeps that text visible BECAUSE it is a command, so the
	// command-position test must agree (#1259 red team).
	if tokStart > 0 && (cmd[tokStart-1] == '\'' || cmd[tokStart-1] == '"') &&
		shellCArgRe.MatchString(cmd[:tokStart-1]) {
		return true
	}
	fields := strings.Fields(segmentBefore(cmd, tokStart))
	i := 0
	for i < len(fields) {
		f := strings.Trim(fields[i], `"'`)
		// A bare quote token trims to empty — `"statusgen"` leaves a lone `"`
		// in front of the name. Not a command, keep scanning.
		if f == "" || envAssignRe.MatchString(f) || strings.HasPrefix(f, "-") {
			i++
			continue
		}
		if base := filepath.Base(f); commandWrappers[base] {
			i++
			if wrapperValueArgs[base] {
				for i < len(fields) && strings.HasPrefix(strings.Trim(fields[i], `"'`), "-") {
					i++ // wrapper's own flags
				}
				i++ // and its value argument (timeout's duration)
			}
			continue
		}
		break
	}
	if i > len(fields) {
		i = len(fields)
	}
	fields = fields[i:]
	if len(fields) == 0 {
		return true
	}
	if filepath.Base(strings.Trim(fields[0], `"'`)) == "go" {
		for _, f := range fields[1:] {
			if f == "run" {
				return true
			}
		}
	}
	return false
}

// gitReadOnlyModes are the modes of an otherwise-mutating git subcommand that
// write nothing (#1190 review). Each was observed REFUSED with the untrue
// reason "mutating git subcommand targeting the shared checkout"; each is
// paired with the genuine write it resembles in
// TestIssue1190TruePositivesPreserved (git stash, git clean -fd, git apply,
// git checkout -b), which must and does still block.
//
// Deliberately narrow: -n means --dry-run for `clean`, but --no-verify for
// `commit`, so the short-flag scan is keyed to the subcommand rather than
// applied globally.
func gitReadOnlyMode(sub string, args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true // `git checkout --help` prints a man page
		}
	}
	switch sub {
	case "stash":
		if len(args) > 0 && (args[0] == "list" || args[0] == "show") {
			return true
		}
	case "clean":
		for _, a := range args {
			if a == "--dry-run" {
				return true
			}
			// short-flag cluster containing n (-n, -nd, -ndx): dry run.
			if len(a) > 1 && a[0] == '-' && a[1] != '-' && strings.ContainsRune(a, 'n') {
				return true
			}
		}
	case "apply":
		for _, a := range args {
			if a == "--check" {
				return true
			}
		}
	}
	return false
}

// gitMutationTargets extracts the directory a mutating git invocation
// writes into: the values of -C, --git-dir, --work-tree (either "--flag
// value" or "--flag=value" form) within the matched invocation, falling
// back to cwd when none are present (git writes relative to cwd). Returns an
// empty (non-nil) set — "no write target, not a hit" — for the read-only
// modes above.
func gitMutationTargets(c Config, cmd string, matchStart, matchEnd int) []string {
	// The regex consumes the shell boundary that terminates the subcommand
	// token; trim it back so the subcommand and its arguments read correctly.
	if matchEnd > matchStart && isTokenBoundary(cmd[matchEnd-1]) {
		matchEnd--
	}
	fields := strings.Fields(cmd[matchStart:matchEnd])
	if len(fields) > 0 {
		sub := strings.Trim(fields[len(fields)-1], `"'`)
		if gitReadOnlyMode(sub, strings.Fields(segmentAfter(cmd, matchEnd))) {
			return []string{}
		}
	}
	var dirs []string
	for i := 0; i < len(fields); i++ {
		f := strings.Trim(fields[i], `"'`)
		switch {
		case f == "-C" || f == "--git-dir" || f == "--work-tree":
			if i+1 < len(fields) {
				i++
				dirs = append(dirs, strings.Trim(fields[i], `"'`))
			}
		case strings.HasPrefix(f, "--git-dir="):
			dirs = append(dirs, strings.Trim(f[len("--git-dir="):], `"'`))
		case strings.HasPrefix(f, "--work-tree="):
			dirs = append(dirs, strings.Trim(f[len("--work-tree="):], `"'`))
		case f == "-c":
			i++ // -c name=value is a config assignment, not a path
		}
	}
	if len(dirs) > 0 {
		return dirs
	}
	return c.cwdTargets(cmd, matchStart)
}

// statusgenReadOnlyFlags are the board-generator modes that write nothing
// (#1190, issue suggestion 3): --lint checks sources, --check is an advisory
// drift check, --dry-run reports what would be written. A read-only board
// read from a worker session is a legitimate, frequent operation.
//
// #147: `go test ./tools/statusgen` and `go vet ./tools/statusgen` name the
// package by path and so match the `\bstatusgen\b` indicator, but neither
// subcommand ever reaches this function at all — isInvocation's "only `go
// run` counts as an invocation" rule (see its doc comment) already screens
// them out before statusgenTargets runs, because compiling or vetting the
// package never executes its main() and so never writes STATUS.md. That is
// the general form of the fix #147 asked for (an allowlist of go test/vet
// specifically): it also covers `go build`/`go install`/`go doc` for free,
// and needs no flag list here. See TestIssue147_GoTestVetOnStatusgenPackage
// for the pinned regression coverage.
var statusgenReadOnlyFlags = map[string]bool{"--lint": true, "--check": true, "--dry-run": true}

// statusgenTargets extracts statusgen's write root: the value of --root
// (either "--root value" or "--root=value") in the same shell segment,
// falling back to the effective cwd when absent (statusgen defaults to
// --root .). Returns an empty (non-nil) set — "no write target, not a hit" —
// for a mere MENTION of the name and for read-only modes.
func statusgenTargets(c Config, cmd string, matchStart, matchEnd int) []string {
	if !isInvocation(cmd, matchStart, matchEnd) {
		return []string{}
	}
	fields := strings.Fields(segmentAfter(cmd, matchEnd))
	for i, f := range fields {
		f = strings.Trim(f, `"'`)
		if statusgenReadOnlyFlags[f] {
			return []string{}
		}
		if f == "--root" {
			if i+1 < len(fields) {
				// Keep scanning for a read-only flag after --root.
				for _, rest := range fields[i+1:] {
					if statusgenReadOnlyFlags[strings.Trim(rest, `"'`)] {
						return []string{}
					}
				}
				return []string{strings.Trim(fields[i+1], `"'`)}
			}
			return nil // --root with no visible value — fail safe
		}
		if strings.HasPrefix(f, "--root=") {
			for _, rest := range fields[i+1:] {
				if statusgenReadOnlyFlags[strings.Trim(rest, `"'`)] {
					return []string{}
				}
			}
			return []string{strings.Trim(f[len("--root="):], `"'`)}
		}
	}
	return c.cwdTargets(cmd, matchStart)
}

// copyLikeCommands write only their DESTINATION — their leading arguments
// are sources, i.e. reads (#1190). `cp <shared>/README.md /tmp/x` copies a
// file OUT of the shared checkout and writes nothing into it. mv/rm/dd and
// friends are NOT in this set: mv's source is removed, so every one of their
// arguments is a mutation target.
var copyLikeCommands = map[string]bool{"cp": true, "rsync": true, "install": true, "ln": true}

// sourceMutatingCopyFlag reports whether a copy-like command carries a flag
// that makes it write its SOURCE too, so the "sources are reads" premise no
// longer holds: `rsync -a --remove-source-files <shared>/docs/ /tmp/dest/`
// empties the shared directory it copies from (#1259 red team). rsync's
// --delete family deletes in the DESTINATION and is deliberately not here.
func sourceMutatingCopyFlag(segment string) bool {
	for _, f := range strings.Fields(segment) {
		if strings.Trim(f, `"'`) == "--remove-source-files" {
			return true
		}
	}
	return false
}

// targetDirFlag returns the value of -t/--target-directory in a segment, or
// "" — the flag inverts the usual "destination is last" convention.
func targetDirFlag(segment string) string {
	fields := strings.Fields(segment)
	for i, f := range fields {
		f = strings.Trim(f, `"'`)
		if (f == "-t" || f == "--target-directory") && i+1 < len(fields) {
			return strings.Trim(fields[i+1], `"'`)
		}
		if strings.HasPrefix(f, "--target-directory=") {
			return strings.Trim(f[len("--target-directory="):], `"'`)
		}
	}
	return ""
}

// findExecChild reports whether the shell word starting at verbStart is the
// CHILD COMMAND of a find -exec/-execdir/-ok/-okdir primary — i.e. the token
// immediately before it is one of those primaries (#1193). Such a verb
// operates on find's matches, not on paths of its own.
func findExecChild(cmd string, verbStart int) bool {
	j := verbStart
	for j > 0 && (cmd[j-1] == ' ' || cmd[j-1] == '\t') {
		j--
	}
	if j == 0 {
		return false
	}
	prev := strings.Trim(cmd[tokenStart(cmd, j-1):j], `"'`)
	switch prev {
	case "-exec", "-execdir", "-ok", "-okdir":
		return true
	}
	return false
}

// dropFindExecPlaceholders removes find's -exec syntax tokens from a child
// command's argument list: `{}` (and any token carrying it — find expands
// those to matched paths under find's own roots), the `+` and `;` / `\;`
// terminators, and a bare `\` left when a segment boundary split an escaped
// terminator. What remains are the child command's own literal paths, which
// still deserve judging (#1193).
func dropFindExecPlaceholders(args []string) []string {
	out := []string{}
	for _, a := range args {
		if a == "+" || a == ";" || a == `\;` || a == `\` || strings.Contains(a, "{}") {
			continue
		}
		out = append(out, a)
	}
	return out
}

// fileMutationTargets extracts the paths a file-mutation command writes: for
// a copy-like command only the destination, for everything else every
// argument.
func fileMutationTargets(_ Config, cmd string, _, matchEnd int) []string {
	segment := segmentAfter(cmd, matchEnd)
	args := argTokens(segment)
	verbStart := tokenStart(cmd, matchEnd-1)
	verb := filepath.Base(strings.Trim(cmd[verbStart:matchEnd], `"'`))
	// A verb that is the child command of a find -exec primary operates on
	// find's MATCHES: its `{}` / `+` / `\;` arguments are find's syntax, not
	// paths, and resolving them against the cwd invents targets — from a
	// shared cwd, `find /tmp/x -exec rm {} +` "resolved" to <shared>/{} and
	// blocked a write that never touches the checkout (#1193). Drop the
	// placeholders; the real write roots are find's, and the find indicator
	// judges those. A residual literal path (`-exec cp {} <shared>/dst \;`)
	// is still judged here.
	if findExecChild(cmd, verbStart) {
		args = dropFindExecPlaceholders(args)
		if len(args) == 0 {
			return []string{} // placeholders only — find's roots carry the write
		}
	}
	if !copyLikeCommands[verb] || sourceMutatingCopyFlag(segment) {
		return args
	}
	if t := targetDirFlag(segment); t != "" {
		return []string{t}
	}
	if len(args) == 0 {
		return nil // no visible destination — fail safe
	}
	return args[len(args)-1:]
}

// findPrePathOption reports whether a token is one of find's pre-path global
// options (-H/-L/-P, -D debugopts, -Olevel), which may precede the root paths
// — stopping the root scan at these would miss the real roots of
// `find -L /tmp/x -delete`.
func findPrePathOption(t string) bool {
	if t == "-H" || t == "-L" || t == "-P" {
		return true
	}
	return strings.HasPrefix(t, "-D") || strings.HasPrefix(t, "-O")
}

// findMutationTargets extracts the write roots of a find command (#1193). find
// only mutates through its -delete primary (removes what it matches) and its
// -exec/-execdir/-ok/-okdir primaries (run an arbitrary command over the
// matches), so the ROOT PATHS — everything between the verb and the first
// expression token — are the write targets. A find carrying neither is a pure
// read: return the empty (non-nil) set, "no write target, not a hit", the
// same shape as statusgenTargets' read-only modes. No visible root means find
// defaults to "." — cwdTargets, which resolves against the effective cwd and
// fails safe when that is unknown. This closes the substitution gap the
// 2026-08-16 incident exposed: a worker answered a blocked `rm` with
// `find … -delete`, which matched no indicator at all.
func findMutationTargets(c Config, cmd string, matchStart, matchEnd int) []string {
	fields := strings.Fields(segmentAfter(cmd, matchEnd))
	mutating := false
	for _, f := range fields {
		f = strings.Trim(f, `"'`)
		if f == "-delete" || strings.HasPrefix(f, "-exec") || strings.HasPrefix(f, "-ok") {
			mutating = true
			break
		}
	}
	if !mutating {
		return []string{} // read-only find — no write target, not a hit
	}
	var roots []string
	for _, f := range fields {
		t := strings.Trim(f, `"'`)
		if t == "" || findPrePathOption(t) {
			continue
		}
		if strings.HasPrefix(t, "-") || t == "(" || t == `\(` || t == "!" || t == `\!` {
			break // the expression begins — roots are exhausted
		}
		roots = append(roots, strings.TrimRight(t, "`"))
	}
	if len(roots) == 0 {
		return c.cwdTargets(cmd, matchStart)
	}
	return roots
}

var indicatorSpecs = []indicatorSpec{
	{
		name: "output redirection",
		// Prefix is (^|[\s\w]) — not just whitespace/digit — so an operator
		// preceded directly by a word char (echo c>>file, echo c>file, with
		// NO space before the operator) still matches (#751). #742/#746 fixed
		// the compound-command first-match gap; this closes a separate,
		// older space-less-redirect hole in the operator regex itself.
		re:          regexp.MustCompile(`(^|[\s\w])(>{1,2})\s*[^&\s]`),
		targetAware: true,
		extract: func(_ Config, cmd string, _, matchEnd int) []string {
			args := argTokens(segmentAfter(cmd, matchEnd-1))
			if len(args) == 0 {
				return nil
			}
			return args[:1]
		},
	},
	{
		name:        "tee",
		re:          regexp.MustCompile(`\btee\b`),
		targetAware: true,
		extract: func(_ Config, cmd string, _, matchEnd int) []string {
			return argTokens(segmentAfter(cmd, matchEnd))
		},
	},
	{
		name: "file mutation command",
		// Prefix class includes "/" and quotes so path-prefixed (/bin/rm,
		// /usr/bin/install) and quoted ("rm") invocations match by BASENAME
		// (#1026) — the same evasion mechanism as path-prefixed env. The
		// other command indicators (git, tee, sed, statusgen, bazel) use \b,
		// which already treats "/" as a boundary, so only this class needed
		// widening. Target-awareness (#1006) bounds the false-positive cost:
		// a stray in-path "rm"/"mv" match still blocks only when a resolved
		// argument lands inside the shared checkout.
		// The BACKTICK is a command boundary too (#1259 security review):
		// a backtick substitution is now kept visible in the places the shell
		// executes it, and `` `touch <sentinel>` `` / `` `rm -rf <shared>/docs` ``
		// begin a command there just as `; touch` does. Without it the verb was
		// visible but unmatched, which let the human-only claim gate be
		// self-served. The other indicators use \b and already matched.
		re:          regexp.MustCompile("(^|[\\s;&|('\"/`])(cp|mv|rm|rmdir|mkdir|touch|truncate|ln|rsync|dd|install)\\b"),
		targetAware: true,
		extract:     fileMutationTargets,
	},
	{
		// find is a file-mutation command ONLY in its -delete / -exec family
		// forms, and its write targets are its ROOT paths, so it gets its own
		// extract instead of joining the alternation above (#1193 — the
		// substitution a worker reached for when `rm` was blocked). Same
		// basename-matching prefix class as the file-mutation indicator;
		// target-awareness bounds the false-positive cost identically.
		name:        "find -delete/-exec (file mutation)",
		re:          regexp.MustCompile("(^|[\\s;&|('\"/`])find\\b"),
		targetAware: true,
		extract:     findMutationTargets,
	},
	{
		name:        "sed -i",
		re:          regexp.MustCompile(`\bsed\s+(-\S+\s+)*-i`),
		targetAware: true,
		extract: func(_ Config, cmd string, _, matchEnd int) []string {
			// sed -i SCRIPT FILE...: the script is not a path, only what
			// follows it is. If we can't see past the script, fail safe
			// (nil ⇒ caller treats the match as a hit, matching the old
			// cwd/indicator-only behavior).
			args := argTokens(segmentAfter(cmd, matchEnd))
			if len(args) <= 1 {
				return nil
			}
			return args[1:]
		},
	},
	{
		name: "mutating git subcommand",
		// The subcommand token must END AT A SHELL BOUNDARY, not at \b (#1190
		// review): \b fires between `merge` and the `-` of `merge-base`, so
		// `git merge-base --is-ancestor` was refused as a "mutating git
		// subcommand" — it writes nothing. RE2 has no lookahead, so the
		// boundary character is consumed by the match; gitMutationTargets
		// trims it back off before reading the subcommand's arguments.
		re:          regexp.MustCompile(`\bgit\s+((-C|-c|--git-dir(=| )|--work-tree(=| ))\s*\S+\s+)*(add|commit|checkout|switch|restore|clean|merge|rebase|reset|stash|cherry-pick|am|apply|mv|rm)([\s;&|)]|$)`),
		targetAware: true,
		extract:     gitMutationTargets,
	},
	{
		name:        "statusgen (writes STATUS.md)",
		re:          regexp.MustCompile(`\bstatusgen\b`),
		targetAware: true,
		extract:     statusgenTargets,
	},
	{
		// Example build-tool indicator: a generic build command that writes
		// its output tree (here, bazel-out/) relative to cwd. Adopters whose
		// build tool differs supply their own dangerous-command matcher through
		// the ASSAY_WRITEGUARD_CALLOUT extension point rather than editing this
		// compiled default.
		name:        "build tool (writes build output)",
		re:          regexp.MustCompile(`\bbazel\s+build\b`),
		targetAware: true,
		extract: func(c Config, cmd string, matchStart, _ int) []string {
			if !isInvocation(cmd, matchStart, matchStart+len("bazel")) {
				return []string{}
			}
			return c.cwdTargets(cmd, matchStart)
		},
	},
}

// checkWriteIndicators scans cmd for write-ish constructs. Every indicator
// is target-aware (#1006): it only reports a block when the indicator's own
// resolved target lands inside the shared checkout. An extract returning
// nil means no target could be identified — fail safe, treat as a hit. The
// !targetAware branch is retained for any future indicator whose write
// destination genuinely cannot be modeled.
//
// Target-aware indicators use FindAllStringIndex to inspect EVERY occurrence,
// not just the first. This closes a compound-command bypass where a safe
// first occurrence (e.g. echo ok > /tmp/x) passes the check and the guard
// never inspects a subsequent dangerous one (echo pwn >> <shared>/STATUS.md).
// Indicator regexes run against the LITERAL-MASKED command (#1190) so a
// keyword inside a quoted argument or a heredoc body — prose, not a command —
// never arms an indicator; extraction still reads the raw command, since
// masking preserves byte offsets. Relative targets resolve against the
// EFFECTIVE cwd for the match position (payload cwd + any preceding cd).
func (c Config) checkWriteIndicators(cmd, rroot, wtd string) (name string, blocked bool) {
	return c.scanWriteIndicators(cmd, func(p string) bool {
		return underDir(p, rroot) && !underDir(p, wtd)
	}, true)
}

// scanWriteIndicators is checkWriteIndicators generalized over the destination
// under test: hit reports whether a resolved write target is one the caller
// cares about, and failSafe decides what an UNIDENTIFIABLE target means — a
// hit for the write-indicator scan (a write whose destination cannot be established, from
// a command that already names the shared checkout or runs inside it, must not
// pass silently), not a hit for the narrow claim-sentinel scan, where "some
// path I cannot read" is no evidence about one specific file.
//
// A target whose HEAD expansion cannot be resolved (expandTargetHead) falls
// back to the pre-#1190 behaviour: resolve it against the effective cwd. That
// is the fail-safe half the review found missing — head admitted nine genuine
// shared-checkout writes the old guard blocked by skipping such targets
// outright (PR #1259 review, blocking 1) — and it is no wider than the old
// guard: an unknown head still only hits when the frame is inside the shared
// checkout, exactly as `<cwd>/$S` did.
func (c Config) scanWriteIndicators(cmd string, hit func(string) bool, failSafe bool) (name string, blocked bool) {
	hay := maskLiterals(cmd)
	for _, ind := range indicatorSpecs {
		allLocs := ind.re.FindAllStringIndex(hay, -1)
		if len(allLocs) == 0 {
			continue
		}
		if !ind.targetAware {
			if failSafe {
				return ind.name, true
			}
			continue
		}
		for _, loc := range allLocs {
			targets := ind.extract(c, cmd, loc[0], loc[1])
			if targets == nil {
				// No explicit target could be identified.
				if failSafe {
					return ind.name, true
				}
				continue
			}
			base := c.effectiveCwd(cmd, loc[0])
			for _, tgt := range targets {
				if t, ok := expandTargetHead(cmd, loc[0], tgt, base); ok {
					tgt = t
				}
				if hit(resolveToken(tgt, base)) {
					return ind.name, true
				}
			}
		}
		// Every occurrence of this indicator has its own target(s) resolve
		// outside the destination under test — not a hit for THIS indicator;
		// keep checking others.
	}
	return "", false
}

// maskQuoted replaces the contents of single- and double-quoted spans with
// spaces (preserving length/position, and the quote characters themselves)
// so that segment-boundary scanning isn't fooled by a quoted ";"/"&"/"|".
// It is NOT used for write-indicator detection (which must still see a
// literal `rm`/`git commit`/etc. even if arguments around it are quoted).
// Escape- and comment-aware since the #1259 security review: a `\'`
// opens nothing and a quote inside a `#` comment is not a quote. Unlike
// maskQuotedKeepShellArg this one does NOT restore an unterminated span — it
// feeds effectiveCwd, where HIDING a `cd` is the fail-safe direction, so
// revealing on doubt would be backwards here.
func maskQuoted(cmd string) string {
	b := []byte(cmd)
	var quote byte
	for i := 0; i < len(cmd); i++ {
		switch {
		case quote != 0:
			if closesQuote(cmd, i, quote) {
				quote = 0
			} else {
				b[i] = ' '
			}
		case opensComment(cmd, i):
			for i+1 < len(cmd) && cmd[i+1] != '\n' {
				i++
			}
		case opensQuote(cmd, i):
			quote = cmd[i]
		}
	}
	return string(b)
}

// segmentAfter returns cmd[after:], truncated at the next unquoted shell
// segment boundary (; & | ) or a newline) or end of string.
//
// The NEWLINE is a segment boundary here for the same reason it already is in
// segmentBefore (#1190 review): without it, a mutation verb on line 1 swallowed
// every bare word on ALL following lines as one of its targets, and a bare word
// resolved against a shared cwd lands inside the shared checkout — so
// `rm -rf /private/tmp/zz` + newline + `echo SIM-OK` was refused as a file
// mutation targeting <shared>/SIM-OK. Multi-line Bash is routine for agents;
// the same command with `;` was always allowed. A BACKSLASH-escaped newline is
// a line continuation, not a boundary — `rm -rf \` + newline + `<shared>/dist`
// is one command and must keep blocking.
func segmentAfter(cmd string, after int) string {
	if after < 0 || after > len(cmd) {
		return ""
	}
	masked := maskQuoted(cmd)
	for i := after; i < len(masked); i++ {
		switch masked[i] {
		case ';', '&', '|', ')':
			return cmd[after:i]
		case '\n':
			if !escapedNewline(masked, i) {
				return cmd[after:i]
			}
		}
	}
	return cmd[after:]
}

// escapedNewline reports whether the newline at index i is a shell line
// continuation (preceded by an odd number of backslashes).
func escapedNewline(s string, i int) bool { return oddBackslashesBefore(s, i) }

// oddBackslashesBefore reports whether the byte at index i is backslash-escaped
// — i.e. preceded by an odd number of backslashes. Inside double quotes `\$`
// is a literal dollar, so `"\$(rm -rf x)"` is prose, not a substitution.
func oddBackslashesBefore(s string, i int) bool {
	n := 0
	for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
}

// argTokens splits a command segment into whitespace-separated argument
// tokens, respecting single/double quotes (a quoted phrase is one token,
// with the quotes stripped), and skipping flags (tokens starting with "-").
func argTokens(segment string) []string {
	var tokens []string
	var b strings.Builder
	var quote byte
	flush := func() {
		if b.Len() > 0 {
			tokens = append(tokens, b.String())
			b.Reset()
		}
	}
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				b.WriteByte(c)
			}
		case c == '\'' || c == '"':
			quote = c
		case c == ' ' || c == '\t' || c == '\n':
			flush()
		default:
			b.WriteByte(c)
		}
	}
	flush()
	var out []string
	for _, t := range tokens {
		if strings.HasPrefix(t, "-") {
			continue
		}
		// A TRAILING backtick is shell syntax closing a substitution that opened
		// before the verb, not part of the path: `` `touch <sentinel>` `` yields
		// the token "<sentinel>`", which then fails the claim gate's exact-path
		// comparison and self-served the human-only exemption (#1259 security
		// review). Trimming can only shorten a target, never move it out of
		// a directory, so it is fail-closed. A LEADING backtick is left alone —
		// `` rm -rf `pwd`/dist `` is one word whose head expandTargetHead
		// resolves.
		t = strings.TrimRight(t, "`")
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}
