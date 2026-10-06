package main

// coverage_inputs.go — what a Verify row's witness DEPENDS ON (#2026 cause 2).
//
// THE DEFECT. With no dependency manifest (production today), a witness spoke
// for every path outside docs/streams/** and STATUS.md, so ANY later commit —
// a changelog fragment, a release stamp, an unrelated README — held every
// gate:model brief at `wrong-revision`, and none could auto-flip to `done`.
//
// THE FIX. Staleness is judged by whether a path the row's command CAN READ
// changed since the witness ran. deriveRowInputs reads the command text with a
// small, closed shell grammar and returns the repo paths it reads. Anything the
// grammar does not recognise — a `$` (variable or command substitution), a
// backtick, a glob, a redirection, a subshell, an assignment, a parent-relative
// or absolute path, a command outside the closed set below (git, go, awk, a
// script, ...) — makes the row UNDERIVABLE, and an underivable row keeps the
// conservative scope (every path outside the board's bookkeeping), naming why.
// So the derivation can only NARROW the scope when it has positively
// established everything the command reads; any doubt widens it back.
//
// THE CLOSED SET. A row is `&&`, `||` or `;`-joined stages; a stage is one
// command, optionally piped (`|`) into filters. Those operators decide only
// WHETHER a command runs, never what it can read, so the read set is the union
// over every command. `cd DIR` is a stage of its own, followed by `&&`, and a
// row with a `cd` must be joined by `&&` alone (after a failed `cd`, a `;` or
// `||` runs later commands in the old directory, and a `||` before it can skip
// it). Readers (the first command of a stage) and what each reads:
//
//	grep [-cqviEFoxwnhHlLsP] [-e PAT]... [PAT] FILE...   each FILE
//	sed -n '<addr>[,<addr>]p' FILE...                    each FILE
//	head|tail [-n N|-N] FILE...                          each FILE
//	wc [-lcmw] FILE...    cat FILE...    sort [-unr] FILE...
//	test -f|-e|-s|-r FILE                                FILE
//	echo|printf|true|set -e|set -o pipefail              nothing
//
// Filters (every later command of a stage) must read only standard input:
// grep, sed -n '<script>', head, tail, wc, sort, cat, uniq [-c], tr, cut —
// each with no file operand. Option letters outside the sets above (grep -r,
// -f, -d; sed's e/r/w commands; sort -o; ...) are refused, and so is any
// operand that starts with `-` (GNU tools permute options, so a later `-r`
// still counts).
//
// WHY `go` IS NOT IN THE SET. `go test`/`go build`/`go vet` read a whole
// module (and go.work, replace targets, the module cache), then RUN code that
// can open any path, walk up to the repository root, or shell out to git. Its
// read set is not in the command's text; establishing it means scanning the
// code under test, and in this repository the modules most Verify rows test
// read outside themselves. A `go` row keeps the conservative scope, so a
// change to the code under test still refuses it.
//
// WHAT ELSE A DERIVED ROW DEPENDS ON (derivedAtBase): every
// `.gitattributes` on the way down to each input (a filter or eol rule changes
// the bytes a command reads without changing the blob), and the absence of a
// symbolic link or submodule on any input's path (a link reads its target,
// which can change without the link changing), and every input TRACKED,
// byte-exactly, in both the witness's tree and the item's — the staleness diff
// can only name tracked paths, so an untracked or ignored file (a build
// output), a case or normalisation variant of a tracked name (which a
// case-insensitive checkout opens as the tracked file), or a path the command
// never read there (verifyrun --root below the repository root) would never go
// stale. A row that reads its own brief
// or a file verify and regen write (isVerifyWrittenPath) keeps the
// conservative scope: those files move with every Evidence write.
//
// WHAT IS NOT MODELLED. The environment: the tool binaries on PATH, CDPATH,
// BASH_ENV, locale. The pre-existing design already leaves out-of-tree
// environment changes to a dependency manifest (this file's sibling header,
// "WORK-INPUT DEPENDENCIES"); nothing here changes that.

import (
	"errors"
	"fmt"
	pathpkg "path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// rowInputs is what deriveRowInputs establishes: the repo-relative files and
// directories (a directory covers everything under it) the command can read.
type rowInputs struct {
	paths []string
}

// shellWord is one word of a row's command after quote removal; op is set for
// an operator token (&&, ||, ;, |) instead.
type shellWord struct {
	text string
	op   string
}

// errUnderivable wraps every reason deriveRowInputs refuses.
var errUnderivable = errors.New("underivable")

func underivable(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{errUnderivable}, a...)...)
}

// lexRow splits a sh-row command (markdown `\|` already unescaped) into words
// and operators. It accepts only what it can read EXACTLY as bash would: plain
// words, single-quoted text, and double-quoted text with no `$`, backtick,
// backslash or `!`. Anything else is refused, never approximated.
func lexRow(s string) ([]shellWord, error) {
	var out []shellWord
	var cur strings.Builder
	inWord := false
	flush := func() {
		if inWord {
			out = append(out, shellWord{text: cur.String()})
			cur.Reset()
			inWord = false
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			flush()
		case c == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return nil, underivable("an unterminated single quote")
			}
			q := s[i+1 : i+1+j]
			if err := quotedTextOK(q); err != nil {
				return nil, err
			}
			cur.WriteString(q)
			inWord = true
			i += j + 1
		case c == '"':
			j := strings.IndexByte(s[i+1:], '"')
			if j < 0 {
				return nil, underivable("an unterminated double quote")
			}
			q := s[i+1 : i+1+j]
			if strings.ContainsAny(q, "$`\\!") {
				return nil, underivable("an expansion or escape inside double quotes")
			}
			if err := quotedTextOK(q); err != nil {
				return nil, err
			}
			cur.WriteString(q)
			inWord = true
			i += j + 1
		case c == '&':
			if i+1 < len(s) && s[i+1] == '&' {
				flush()
				out = append(out, shellWord{op: "&&"})
				i++
				continue
			}
			return nil, underivable("a background `&`")
		case c == '|':
			flush()
			if i+1 < len(s) && s[i+1] == '|' {
				out = append(out, shellWord{op: "||"})
				i++
				continue
			}
			out = append(out, shellWord{op: "|"})
		case c == ';':
			flush()
			out = append(out, shellWord{op: ";"})
		case strings.IndexByte("$`\\<>(){}[]*?~#!\n\r", c) >= 0:
			return nil, underivable("the shell metacharacter %q", string(c))
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	flush()
	return out, nil
}

// quotedTextOK refuses quoted text whose meaning normalizeCommandText could
// change: the Command guard compares commands with runs of whitespace
// collapsed, so a quoted operand holding a tab or a double space could name a
// different file in the witness's command than in the row's.
func quotedTextOK(q string) error {
	if strings.ContainsAny(q, "\t\n\r") || strings.Contains(q, "  ") {
		return underivable("quoted text with a tab, newline or repeated space")
	}
	return nil
}

// deriveRowInputs returns the repo paths Verify row r's command can read, or
// an error (wrapping errUnderivable) saying why they cannot be established.
func deriveRowInputs(r verifyRow) (rowInputs, error) {
	if r.Shell != "" && r.Shell != rowShellSh {
		return rowInputs{}, underivable("the row runs under %s, whose grammar is not read here", r.Shell)
	}
	words, err := lexRow(unescapePipes(r.Command))
	if err != nil {
		return rowInputs{}, err
	}
	// Split into stages at &&, ||, ; (keeping the separator that FOLLOWS each).
	type stage struct {
		cmds [][]string
		next string
	}
	var stages []stage
	cur := stage{}
	var cmd []string
	endCmd := func() error {
		if len(cmd) == 0 {
			return underivable("an empty command")
		}
		cur.cmds = append(cur.cmds, cmd)
		cmd = nil
		return nil
	}
	for _, w := range words {
		switch w.op {
		case "":
			cmd = append(cmd, w.text)
		case "|":
			if err := endCmd(); err != nil {
				return rowInputs{}, err
			}
		default:
			if err := endCmd(); err != nil {
				return rowInputs{}, err
			}
			cur.next = w.op
			stages = append(stages, cur)
			cur = stage{}
		}
	}
	if err := endCmd(); err != nil {
		return rowInputs{}, err
	}
	stages = append(stages, cur)

	hasCD, onlyAnd := false, true
	for _, st := range stages {
		if st.cmds[0][0] == "cd" {
			hasCD = true
		}
		if st.next != "" && st.next != "&&" {
			onlyAnd = false
		}
	}
	if hasCD && !onlyAnd {
		// After a failed `cd`, a `;` or `||` runs the rest in the old
		// directory, and a `||` before it can skip it: the directory each
		// later command reads in is no longer fixed by the text.
		return rowInputs{}, underivable("a `cd` in a row joined by `;` or `||`")
	}
	var in rowInputs
	seen := map[string]bool{}
	cwd := ""
	for _, st := range stages {
		first := st.cmds[0]
		if first[0] == "cd" {
			if len(st.cmds) != 1 || len(first) != 2 || st.next != "&&" {
				return rowInputs{}, underivable("a `cd` that is not a stage of its own, with one directory, followed by `&&`")
			}
			d, err := rowPath(cwd, first[1])
			if err != nil {
				return rowInputs{}, err
			}
			cwd = d
			continue
		}
		for i, c := range st.cmds {
			files, err := commandReads(c, i == 0)
			if err != nil {
				return rowInputs{}, err
			}
			for _, f := range files {
				p, err := rowPath(cwd, f)
				if err != nil {
					return rowInputs{}, err
				}
				if !seen[p] {
					seen[p] = true
					in.paths = append(in.paths, p)
				}
			}
		}
	}
	if len(in.paths) == 0 {
		return rowInputs{}, underivable("the command reads no repository path")
	}
	sort.Strings(in.paths)
	return in, nil
}

// rowPath resolves operand f against cwd (both repo-relative) to a clean
// repo-relative path, refusing anything that is not a plain relative path
// inside the repository.
func rowPath(cwd, f string) (string, error) {
	if f == "" || f == "-" || strings.HasPrefix(f, "-") {
		return "", underivable("the operand %q is not a file path", f)
	}
	if strings.HasPrefix(f, "/") || strings.ContainsAny(f, "\\:*?[]") {
		return "", underivable("the operand %q is not a plain relative path", f)
	}
	for _, seg := range strings.Split(f, "/") {
		if seg == ".." {
			return "", underivable("the operand %q climbs out of its directory", f)
		}
	}
	p := pathpkg.Clean(pathpkg.Join(cwd, f))
	if p == "." || p == ".git" || strings.HasPrefix(p, ".git/") {
		return "", underivable("the operand %q names the repository root or its .git", f)
	}
	return p, nil
}

var (
	sedPrintRe = regexp.MustCompile(`^(\d+|\$|/[^/\\]*/)(,(\d+|\$|/[^/\\]*/))?p$`)
	countArgRe = regexp.MustCompile(`^\+?\d+$`)
)

// commandReads returns the file operands command c reads. reader is true for
// the first command of a stage (which must name the files it reads) and false
// for a filter after `|` (which must read standard input only).
func commandReads(c []string, reader bool) ([]string, error) {
	name, args := c[0], c[1:]
	if strings.Contains(name, "=") {
		return nil, underivable("the environment assignment %q", name)
	}
	var files []string
	switch name {
	case "grep":
		patternSet := false
		endOpts := false
		var operands []string
		for i := 0; i < len(args); i++ {
			a := args[i]
			if !endOpts && a == "--" {
				endOpts = true
				continue
			}
			if !endOpts && strings.HasPrefix(a, "-") && a != "-" {
				if strings.HasPrefix(a, "--") {
					return nil, underivable("the grep option %q", a)
				}
				for j := 1; j < len(a); j++ {
					l := a[j]
					if l == 'e' {
						patternSet = true
						if j == len(a)-1 {
							if i+1 >= len(args) {
								return nil, underivable("grep -e with no pattern")
							}
							i++
						}
						break
					}
					if strings.IndexByte("cqviEFoxwnhHlLsP", l) < 0 {
						return nil, underivable("the grep option -%c", l)
					}
				}
				continue
			}
			operands = append(operands, a)
		}
		if !patternSet {
			if len(operands) == 0 {
				return nil, underivable("grep with no pattern")
			}
			operands = operands[1:]
		}
		files = operands
	case "sed":
		if len(args) < 2 || args[0] != "-n" || !sedPrintRe.MatchString(args[1]) {
			return nil, underivable("a sed other than `sed -n '<address>[,<address>]p'`")
		}
		files = args[2:]
	case "head", "tail":
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "-n" && i+1 < len(args) && countArgRe.MatchString(args[i+1]):
				i++
			case strings.HasPrefix(a, "-n") && countArgRe.MatchString(a[2:]):
			case strings.HasPrefix(a, "-") && len(a) > 1 && countArgRe.MatchString(a[1:]):
			default:
				files = append(files, a)
			}
		}
	case "wc", "sort", "uniq", "cat":
		letters := map[string]string{"wc": "lcmw", "sort": "unr", "uniq": "c", "cat": ""}[name]
		for _, a := range args {
			if strings.HasPrefix(a, "-") && len(a) > 1 {
				for j := 1; j < len(a); j++ {
					if strings.IndexByte(letters, a[j]) < 0 {
						return nil, underivable("the %s option -%c", name, a[j])
					}
				}
				continue
			}
			files = append(files, a)
		}
		if name == "uniq" && len(files) > 0 {
			return nil, underivable("uniq with an operand (it writes its second)")
		}
	case "tr":
		var sets int
		for _, a := range args {
			if strings.HasPrefix(a, "-") && len(a) > 1 && strings.Trim(a[1:], "ds") == "" {
				continue
			}
			sets++
		}
		if sets == 0 || sets > 2 {
			return nil, underivable("a tr with %d sets", sets)
		}
		return nil, nil // tr reads standard input only; its operands are sets
	case "cut":
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "-d" || a == "-f" || a == "-c":
				if i+1 >= len(args) {
					return nil, underivable("cut %s with no value", a)
				}
				i++
			case len(a) > 2 && (strings.HasPrefix(a, "-d") || strings.HasPrefix(a, "-f") || strings.HasPrefix(a, "-c")):
			default:
				return nil, underivable("the cut operand %q", a)
			}
		}
		return nil, nil
	case "test":
		if !reader || len(args) != 2 || !map[string]bool{"-f": true, "-e": true, "-s": true, "-r": true}[args[0]] {
			return nil, underivable("a test other than `test -f|-e|-s|-r FILE`")
		}
		files = args[1:]
	case "echo", "printf", "true":
		return nil, nil // reads nothing; its operands are text
	case "set":
		if strings.Join(args, " ") != "-e" && strings.Join(args, " ") != "-o pipefail" {
			return nil, underivable("a set other than `set -e` or `set -o pipefail`")
		}
		return nil, nil
	default:
		return nil, underivable("the command %q is outside the set whose reads a command's text establishes", name)
	}
	if reader && len(files) == 0 {
		return nil, underivable("%s with no file operand reads standard input, which no file names", name)
	}
	if !reader && len(files) > 0 {
		return nil, underivable("the filter %s names a file operand", name)
	}
	return files, nil
}

// forVerifyRow is forRow plus, when no dependency manifest is supplied (the
// production shape), the row's derived inputs: a derivable row's witness
// speaks for exactly what its command reads; an underivable one keeps the
// conservative scope and says why. A supplied manifest keeps its own rules.
func (sc witnessScope) forVerifyRow(r verifyRow) witnessScope {
	out := sc.forRow(r.ID)
	if sc.manifest != nil {
		return out
	}
	in, err := deriveRowInputs(r)
	if err != nil {
		out.whyConservative = "no dependency manifest, and the row's inputs cannot be derived from its command (" + strings.TrimPrefix(err.Error(), errUnderivable.Error()+": ") + ")"
		return out
	}
	out.conservative, out.whyConservative = false, ""
	out.inputs = in.paths
	return out
}

// derivedAtBase completes a derived scope against the witness's commit base
// and the item's revision target (both full commit ids). It adds every
// `.gitattributes` on the way to each input, and falls back to the
// conservative scope (never narrower) when an input is or sits under a symbolic
// link or submodule, or covers the brief's own file or a file verify and regen
// write. why is non-empty when the trees themselves cannot be read: the caller
// then resolves could-not-check.
func (sc witnessScope) derivedAtBase(root, base, target string) (eff witnessScope, why string) {
	eff = sc
	eff.inputs = append([]string(nil), sc.inputs...)
	have := map[string]bool{}
	for _, p := range eff.inputs {
		have[p] = true
	}
	for _, p := range sc.inputs {
		for d := pathpkg.Dir(p); ; d = pathpkg.Dir(d) {
			ga := ".gitattributes"
			if d != "." {
				ga = d + "/.gitattributes"
			}
			if !have[ga] {
				have[ga] = true
				eff.inputs = append(eff.inputs, ga)
			}
			if d == "." {
				break
			}
		}
	}
	sort.Strings(eff.inputs)
	widen := func(reason string) witnessScope {
		w := sc
		w.inputs = nil
		w.conservative, w.whyConservative = true, "no dependency manifest, and "+reason
		return w
	}
	for _, p := range sc.inputs {
		if p == sc.briefRel || isVerifyWrittenPath(p) {
			return widen(fmt.Sprintf("the row reads %s, which verify and regen write with every Evidence update", p)), ""
		}
	}
	for _, rev := range []string{base, target} {
		entries, err := treeModesAt(root, rev)
		if err != nil {
			return sc, fmt.Sprintf("the tree at %s could not be listed (git ls-tree failed), so what the row reads cannot be established", rev)
		}
		// Every derived input (never the `.gitattributes` added above) must be
		// TRACKED, byte-exactly, in both trees: a blob at that path, or a
		// directory some entry sits under. The staleness diff can only ever
		// name tracked paths, so an input it cannot name could change without
		// staling the witness: an untracked or ignored file (a build output, a
		// file an earlier row writes), a case or Unicode-normalisation variant
		// of a tracked name that a case-insensitive checkout opens as the
		// tracked file, or a path the command never read there because it ran
		// in another directory (verifyrun --root below the repository root).
		which := "the witness's"
		if rev != base {
			which = "the item's"
		}
		for _, p := range sc.inputs {
			found := false
			for _, e := range entries {
				if inputCovers(p, e.path) || ((e.mode == "120000" || e.mode == "160000") && strings.HasPrefix(p, e.path+"/")) {
					found = true
					break
				}
			}
			if !found {
				return widen(fmt.Sprintf("the row reads %s, which is not tracked under that exact path in %s tree, so a change to what it opens cannot show in the trees' diff", p, which)), ""
			}
		}
		for _, e := range entries {
			for _, p := range sc.inputs {
				covered := inputCovers(p, e.path)
				if (e.mode == "120000" || e.mode == "160000") && (covered || strings.HasPrefix(p, e.path+"/")) {
					return widen(fmt.Sprintf("the row reads %s through the symbolic link or submodule %s, whose target its path does not establish", p, e.path)), ""
				}
				if covered && (e.path == sc.briefRel || isVerifyWrittenPath(e.path)) {
					return widen(fmt.Sprintf("the row reads %s, which verify and regen write with every Evidence update", e.path)), ""
				}
			}
		}
	}
	return eff, ""
}

// inputCovers reports whether a change to repo-relative path p touches input
// in: p is in, or p is under in as a directory.
func inputCovers(in, p string) bool {
	return p == in || strings.HasPrefix(p, in+"/")
}

// treeMode is one `git ls-tree -r` entry: its mode and path.
type treeMode struct{ mode, path string }

var (
	treeModesMu    sync.Mutex
	treeModesCache = map[string][]treeMode{}
)

// treeModesAt lists every entry of rev's tree with its mode (`git ls-tree -r
// -z --full-tree`), so a symbolic link (120000) or submodule (160000) on an
// input's path is seen. An error is returned, never an empty listing, when git
// cannot read the tree.
func treeModesAt(root, rev string) ([]treeMode, error) {
	key := root + "\x00" + rev
	treeModesMu.Lock()
	got, ok := treeModesCache[key]
	treeModesMu.Unlock()
	if ok {
		return got, nil
	}
	out, err := coverageGit(root, "ls-tree", "-r", "-z", "--full-tree", "--end-of-options", rev).Output()
	if err != nil {
		return nil, err
	}
	var entries []treeMode
	for _, e := range splitNUL(out) {
		meta, name, found := strings.Cut(e, "\t")
		f := strings.Fields(meta)
		if !found || len(f) != 3 {
			return nil, fmt.Errorf("unparseable ls-tree entry %q", e)
		}
		entries = append(entries, treeMode{mode: f[0], path: name})
	}
	treeModesMu.Lock()
	treeModesCache[key] = entries
	treeModesMu.Unlock()
	return entries, nil
}

// maxLandingCandidates bounds witnessLanded's search. A witness further from
// the item's revision than this is could-not-check, never assumed landed.
var maxLandingCandidates = 1000

// witnessLanded is #2026 cause 1's landing check, judged by TREES, never by
// ancestry: a witness at commit w is honoured at the item's revision t only
// when some commit on t's history since their merge base(s) — the base itself
// when w is on t's history, or the squash commit when w's branch was squash-
// merged — has a tree identical to w's outside docs/streams/** and STATUS.md.
// A witness whose branch never landed as it was tested (it was squash-merged
// onto a main that had moved, or never merged) is a mismatch. why is set when
// the search cannot be completed (a shallow clone, a git failure, more than
// maxLandingCandidates commits to compare): the caller resolves could-not-check.
// A completed search is cached per (root, w, t, bound): many rows share one
// witness commit, and an unlanded one costs up to the bound's worth of diffs.
func witnessLanded(root, w, t string) (landedAt string, landed bool, why string) {
	key := root + "\x00" + w + "\x00" + t + "\x00" + strconv.Itoa(maxLandingCandidates)
	landedMu.Lock()
	got, ok := landedCache[key]
	landedMu.Unlock()
	if ok {
		return got.at, got.landed, ""
	}
	landedAt, landed, why = searchLanded(root, w, t)
	if why == "" {
		landedMu.Lock()
		landedCache[key] = landedResult{landedAt, landed}
		landedMu.Unlock()
	}
	return landedAt, landed, why
}

type landedResult struct {
	at     string
	landed bool
}

var (
	landedMu    sync.Mutex
	landedCache = map[string]landedResult{}
)

// searchLanded is witnessLanded's uncached search.
func searchLanded(root, w, t string) (landedAt string, landed bool, why string) {
	if why := shallowCloneWhy(root); why != "" {
		return "", false, why
	}
	out, err := coverageGit(root, "merge-base", "--all", "--end-of-options", w, t).Output()
	if err != nil {
		return "", false, "the witness's commit and the item's revision could not be related (git merge-base --all failed)"
	}
	bases := strings.Fields(string(out))
	if len(bases) == 0 {
		return "", false, "git merge-base --all named no common ancestor"
	}
	// The merge bases and t need no search: compare them first, so a witness on
	// the item's own history (its merge base is the witness itself) lands
	// however many commits followed it. Only the search for a squash commit
	// between them is bounded.
	if at, ok, why := landedAmong(root, w, append(append([]string(nil), bases...), t)); ok || why != "" {
		return at, ok, why
	}
	revArgs := []string{"rev-list", "--max-count=" + strconv.Itoa(maxLandingCandidates+1), "--end-of-options", t}
	for _, b := range bases {
		revArgs = append(revArgs, "^"+b)
	}
	out, err = coverageGit(root, revArgs...).Output()
	if err != nil {
		return "", false, "the commits between the witness and the item's revision could not be listed (git rev-list failed)"
	}
	more := strings.Fields(string(out))
	if len(more) > maxLandingCandidates {
		return "", false, fmt.Sprintf("more than %d commits lie between the witness's merge base and the item's revision, past the bound this search compares", maxLandingCandidates)
	}
	return landedAmong(root, w, more)
}

// landedAmong reports the first of candidates whose tree is identical to w's
// outside docs/streams/** and STATUS.md; why is set when git cannot compare.
func landedAmong(root, w string, candidates []string) (landedAt string, landed bool, why string) {
	for _, c := range candidates {
		d, err := coverageGit(root, "diff", "--name-only", "--no-renames", "-z", "--end-of-options", w, c, "--").Output()
		if err != nil {
			return "", false, "the witness's tree could not be compared with " + c + " (git diff failed)"
		}
		same := true
		for _, p := range splitNUL(d) {
			if !isBoardBookkeepingPath(p) {
				same = false
				break
			}
		}
		if same {
			return c, true, ""
		}
	}
	return "", false, ""
}

// shallowCloneWhy is non-empty when root is a shallow clone (or that cannot be
// read): its history is cut, so neither the merge base nor the landing search
// can be trusted, and the claim is could-not-check.
func shallowCloneWhy(root string) string {
	out, err := coverageGit(root, "rev-parse", "--is-shallow-repository").Output()
	if err != nil {
		return "whether this clone is shallow could not be read (git rev-parse failed)"
	}
	if strings.TrimSpace(string(out)) != "false" {
		return "this clone is shallow, so the history between the witness and the item's revision cannot all be read; fetch the full history"
	}
	return ""
}
