package main

// brief — resolve an item KEY (`<stream>/<NN>`) to the three facts every dispatch
// and verify consumer currently re-derives by hand: which file the key names, what
// its frontmatter (`gate:` / `risk:` / `exec-tier:` …) says, and what the stream
// README's board row reports for its status. A positional subcommand intercepted
// before the parent flag parse (like verifyrun/conform/init), reusing the SAME
// parsers the lint uses — parseBriefFile for the frontmatter and parseBriefTable
// for the row — so the answer this prints and the answer --lint validates can
// never disagree (desk-tools/12).
//
// It is READ-ONLY: it never writes STATUS.md or any generated file, touches no
// network, and shells out to nothing. The whole verb is "print what statusgen
// already parsed for one key".
//
// Exit contract (two-state; there is no could-not-check leg — a key either
// resolves to exactly one file or it does not):
//   - 0  every key resolved to exactly one brief file.
//   - 2  at least one key was unresolvable (bad grammar, zero matching files, or
//        more than one — the numeric-prefix collision a hand-rolled glob could not
//        detect), OR a usage error. On any failure NO JSON body is printed, so a
//        consumer never reads a partial array as complete; every key's outcome is
//        reported to stderr first.
//
// A legacy brief (no `schema:` frontmatter) and a brief with no README board row
// both RESOLVE (exit 0): the former with `"schema": "legacy"` and empty
// frontmatter fields, the latter with `"row": null`. Absence is reported as
// itself — the JSON never invents a status (three-state instrument rule).

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// briefInfoExit* — the two-state contract described above.
const (
	briefInfoExitOK      = 0
	briefInfoExitResolve = 2 // unresolvable key or usage error
)

// itemKeyNumRe pins the `<NN>` half of an item key: digits with an optional
// single trailing letter (`12`, `03`, `12a`), the same shape briefNameRe captures
// from a filename.
var itemKeyNumRe = regexp.MustCompile(`^[0-9]+[a-z]?$`)

// briefInfoRow is the board-row half of a resolved key: the stream README table
// cells for this brief. nil (rendered `null`) when the brief file has no row.
type briefInfoRow struct {
	Status   string `json:"status"`
	Verified string `json:"verified"`
	Reviewed string `json:"reviewed"`
	Wave     int    `json:"wave"`
	Effort   string `json:"effort"`
}

// briefInfo is one resolved key: its file (relative to --root, so the output
// carries no machine path), its frontmatter fields, and its board row.
type briefInfo struct {
	Key         string            `json:"key"`
	File        string            `json:"file"`
	Schema      string            `json:"schema"`
	Title       string            `json:"title"`
	Wave        int               `json:"wave"`
	Effort      string            `json:"effort"`
	Gate        string            `json:"gate"`
	Risk        map[string]string `json:"risk"`
	ExecTier    string            `json:"exec_tier"`
	ExecTierWhy string            `json:"exec_tier_why"`
	Depends     []string          `json:"depends"`
	Unblocks    []string          `json:"unblocks"`
	Issues      []int             `json:"issues"`
	Row         *briefInfoRow     `json:"row"`
}

// runBriefInfo is the `statusgen brief` entry point; it returns the process exit
// code. Flags may appear before or after the positional keys
// (`brief desk-tools/12 --root .. --json`), so args are scanned by hand rather
// than handed to flag.Parse, which stops at the first non-flag token.
func runBriefInfo(args []string, stdout, stderr io.Writer) int {
	root := "."
	asText := false
	var keys []string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--root" || a == "-root":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "brief: --root needs a value")
				return briefInfoExitResolve
			}
			root = args[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case strings.HasPrefix(a, "-root="):
			root = a[len("-root="):]
		case a == "--json" || a == "-json":
			asText = false
		case a == "--text" || a == "-text":
			asText = true
		case a == "-h" || a == "--help":
			fmt.Fprintln(stderr, "usage: statusgen brief [--root DIR] [--json|--text] <stream>/<NN> [<stream>/<NN> ...]")
			return briefInfoExitResolve
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "brief: unknown flag %q\n", a)
			return briefInfoExitResolve
		default:
			keys = append(keys, a)
		}
	}

	if len(keys) == 0 {
		fmt.Fprintln(stderr, "brief: need at least one <stream>/<NN> key")
		return briefInfoExitResolve
	}

	// Resolve every key first, collecting successes and failures, so that a single
	// unresolvable key reports EVERY key's outcome (never a silent partial array).
	infos := make([]*briefInfo, len(keys))
	failed := false
	for i, key := range keys {
		info, err := resolveBriefKey(root, key)
		if err != nil {
			failed = true
			fmt.Fprintf(stderr, "brief: %s\n", err)
			continue
		}
		infos[i] = info
		fmt.Fprintf(stderr, "brief: resolved %s -> %s\n", key, info.File)
	}
	if failed {
		// No JSON body on failure: a consumer must never read a partial result as
		// a complete array.
		return briefInfoExitResolve
	}

	if asText {
		for i, info := range infos {
			if i > 0 {
				fmt.Fprintln(stdout)
			}
			renderBriefInfoText(stdout, info)
		}
		return briefInfoExitOK
	}

	// A single key emits a single object; multiple keys emit an array in argument
	// order (so a consumer that always passes one key gets the object shape).
	var payload any = infos
	if len(infos) == 1 {
		payload = infos[0]
	}
	enc, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "brief: rendering JSON: %v\n", err)
		return briefInfoExitResolve
	}
	fmt.Fprintln(stdout, string(enc))
	return briefInfoExitOK
}

// resolveBriefKey turns one `<stream>/<NN>` key into a fully-assembled briefInfo,
// or an error naming exactly why it could not be resolved.
func resolveBriefKey(root, key string) (*briefInfo, error) {
	stream, num, ok := splitItemKey(key)
	if !ok {
		return nil, fmt.Errorf("invalid key %q: want <stream>/<NN>", key)
	}

	streamDir := filepath.Join(root, "docs", "streams", stream)
	entries, err := os.ReadDir(streamDir)
	if err != nil {
		// A missing stream directory is the same three-state answer as a missing
		// file: the key names no brief here.
		return nil, fmt.Errorf("no brief file for %s", key)
	}

	// Match on the EXACT parsed number, never a prefix glob: key `foo/1` must not
	// match `brief-12-*`. Exactly one file must match — zero or many is a
	// resolution failure the consumer must see, not paper over.
	var matches []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := briefNameRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if m[1] == num {
			matches = append(matches, e.Name())
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no brief file for %s", key)
	case 1:
		// fall through
	default:
		return nil, fmt.Errorf("multiple brief files for %s: %s", key, strings.Join(matches, ", "))
	}

	name := matches[0]
	path := filepath.Join(streamDir, name)

	info := &briefInfo{
		Key:      key,
		File:     filepath.ToSlash(filepath.Join("docs", "streams", stream, name)),
		Depends:  []string{},
		Unblocks: []string{},
		Issues:   []int{},
		Risk:     map[string]string{},
	}

	bf, okFM, err := parseBriefFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", key, err)
	}
	if !okFM || bf == nil {
		// Legacy brief (no `schema:` frontmatter): resolving it is not an error;
		// it reports as `schema: legacy` with empty frontmatter fields.
		info.Schema = "legacy"
	} else {
		info.Schema = bf.Schema
		info.Title = bf.Title
		info.Wave = bf.Wave
		info.Effort = bf.Effort
		info.Gate = bf.Gate
		info.ExecTier = bf.ExecTier
		info.ExecTierWhy = bf.ExecTierWhy
		if len(bf.Depends) > 0 {
			info.Depends = bf.Depends
		}
		if len(bf.Unblocks) > 0 {
			info.Unblocks = bf.Unblocks
		}
		if len(bf.Issues) > 0 {
			info.Issues = bf.Issues
		}
		if len(bf.Risk) > 0 {
			info.Risk = bf.Risk
		}
	}

	row, err := briefRowFor(filepath.Join(streamDir, "README.md"), num)
	if err != nil {
		return nil, fmt.Errorf("%s: reading board row: %v", key, err)
	}
	info.Row = row // nil when the brief has no README row (reported as `"row": null`)

	return info, nil
}

// splitItemKey parses `<stream>/<NN>` into its two halves. ok is false for any
// shape that is not exactly one stream segment and one numeric suffix.
func splitItemKey(key string) (stream, num string, ok bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	stream, num = parts[0], parts[1]
	if stream == "" || !itemKeyNumRe.MatchString(num) {
		return "", "", false
	}
	return stream, num, true
}

// briefRowFor reads a stream README and returns the board row whose `#` equals
// num, or nil when there is no such row. A missing README is treated as "no row"
// (nil, nil) — a brief file with no row is a lint finding elsewhere, not a
// resolution failure; a README that exists but cannot be parsed is a real error.
func briefRowFor(readmePath, num string) (*briefInfoRow, error) {
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	_, body, err := splitFrontmatter(string(raw))
	if err != nil {
		return nil, err
	}
	briefs, err := parseBriefTable(body)
	if err != nil {
		return nil, err
	}
	for _, b := range briefs {
		if b.Num == num {
			return &briefInfoRow{
				Status:   b.Status,
				Verified: b.Verified,
				Reviewed: b.Reviewed,
				Wave:     b.Wave,
				Effort:   b.Effort,
			}, nil
		}
	}
	return nil, nil
}

// renderBriefInfoText prints one briefInfo as `key: value` lines carrying every
// field the JSON carries, so `--text` and `--json` never diverge on content.
func renderBriefInfoText(w io.Writer, info *briefInfo) {
	fmt.Fprintf(w, "key: %s\n", info.Key)
	fmt.Fprintf(w, "file: %s\n", info.File)
	fmt.Fprintf(w, "schema: %s\n", info.Schema)
	fmt.Fprintf(w, "title: %s\n", info.Title)
	fmt.Fprintf(w, "wave: %d\n", info.Wave)
	fmt.Fprintf(w, "effort: %s\n", info.Effort)
	fmt.Fprintf(w, "gate: %s\n", info.Gate)
	riskKeys := make([]string, 0, len(info.Risk))
	for k := range info.Risk {
		riskKeys = append(riskKeys, k)
	}
	sort.Strings(riskKeys)
	riskPairs := make([]string, 0, len(riskKeys))
	for _, k := range riskKeys {
		riskPairs = append(riskPairs, k+"="+info.Risk[k])
	}
	fmt.Fprintf(w, "risk: %s\n", strings.Join(riskPairs, " "))
	fmt.Fprintf(w, "exec_tier: %s\n", info.ExecTier)
	fmt.Fprintf(w, "exec_tier_why: %s\n", info.ExecTierWhy)
	fmt.Fprintf(w, "depends: %s\n", strings.Join(info.Depends, " "))
	fmt.Fprintf(w, "unblocks: %s\n", strings.Join(info.Unblocks, " "))
	issueStrs := make([]string, 0, len(info.Issues))
	for _, n := range info.Issues {
		issueStrs = append(issueStrs, fmt.Sprintf("%d", n))
	}
	fmt.Fprintf(w, "issues: %s\n", strings.Join(issueStrs, " "))
	if info.Row == nil {
		fmt.Fprintln(w, "row: null")
	} else {
		fmt.Fprintf(w, "row.status: %s\n", info.Row.Status)
		fmt.Fprintf(w, "row.verified: %s\n", info.Row.Verified)
		fmt.Fprintf(w, "row.reviewed: %s\n", info.Row.Reviewed)
		fmt.Fprintf(w, "row.wave: %d\n", info.Row.Wave)
		fmt.Fprintf(w, "row.effort: %s\n", info.Row.Effort)
	}
}
