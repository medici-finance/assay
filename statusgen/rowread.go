package main

// `statusgen rowread` — the read-only forge lane for a Verify row (#4700).
//
// An admitted verifyrun row carries no forge credential: admittedRowEnv
// strips GH_TOKEN, GITHUB_TOKEN, GH_ENTERPRISE_TOKEN and GITHUB_ENTERPRISE_TOKEN
// before the row's shell starts, so a row that pipes `gh api` reads nothing.
// The ruling on #4700 settles how such a row reads forge state anyway: through
// a read-only verb that mints the credential and holds it inside its own
// process, so nothing the row can see — env, argv, a file, its output — ever
// carries it.
//
// This file is that verb's row-facing surface, and it is deliberately thin. It
// adds no minter and no forge client: each read goes through
// deskreadReader.readItem (forgeread.go), the same seam the trust gate and the
// un-block lane already use, which runs the desk-tools `deskread` verb. deskread
// resolves its role from DESK_LOOP and mints under that role's App custody,
// in-process; it never reads an ambient login. What this file adds is:
//
//   - a CLOSED allowlist of read kinds. Anything else — a write verb, another
//     read kind, an unknown flag (`--ci-workflow-token` included) — is refused
//     before deskread starts.
//   - target validation, so a row cannot smuggle a flag into deskread's argv:
//     the target is rebuilt from a parsed owner/name and a positive number.
//   - a fail-closed answer. Any issue deskread did not read exits
//     could-not-check (6) and prints nothing, not even the matches of issues
//     read before it, because an empty or partial answer is exactly what the
//     row's "is the marker present?" test would misread.
//   - an output that carries no comment body: one JSON line per matching
//     comment naming the issue, the author and the time.

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

const (
	rowreadExitOK       = 0
	rowreadExitUsage    = 5
	rowreadExitCouldNot = 6
)

// rowreadKinds is the closed set of read kinds a row may ask for. It is a
// subset of deskread's own read kinds, and only kinds whose answer is a
// per-issue read with a parsed, fail-closed shape in readItem belong here.
var rowreadKinds = map[string]bool{"comments": true}

// rowreadTarget is the only target shape accepted: owner/name#N. Owner and
// name use the forge's own character set, so neither can begin with a dash.
var rowreadTarget = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]*)/([A-Za-z0-9._][A-Za-z0-9._-]*)#([0-9]+)$`)

type rowreadIssue struct {
	repo   string
	number int
}

type rowreadIssues []rowreadIssue

func (r *rowreadIssues) String() string { return fmt.Sprint(len(*r)) }

func (r *rowreadIssues) Set(v string) error {
	m := rowreadTarget.FindStringSubmatch(v)
	if m == nil {
		return fmt.Errorf("want owner/name#N, got %q", v)
	}
	n, err := strconv.Atoi(m[3])
	if err != nil || n <= 0 {
		return fmt.Errorf("want a positive issue number, got %q", v)
	}
	*r = append(*r, rowreadIssue{repo: m[1] + "/" + m[2], number: n})
	return nil
}

// rowreadMatch is one output line. It names a comment; it never carries one.
type rowreadMatch struct {
	Repo        string `json:"repo"`
	Number      int    `json:"number"`
	AuthorLogin string `json:"authorLogin"`
	CreatedAt   string `json:"createdAt"`
}

func runRowread(args []string, stdout, stderr io.Writer) int {
	usage := "usage: statusgen rowread comments --issue owner/name#N [--issue ...] [--body-contains TEXT]"
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return rowreadExitUsage
	}
	kind := args[0]
	if !rowreadKinds[kind] {
		fmt.Fprintf(stderr, "statusgen rowread: refusing %q: not an allowed read kind (allowed: comments). "+
			"This lane reads; it never writes, and nothing ran\n", kind)
		return rowreadExitUsage
	}
	fs := flag.NewFlagSet("rowread "+kind, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var issues rowreadIssues
	fs.Var(&issues, "issue", "an issue to read, owner/name#N (repeatable)")
	contains := fs.String("body-contains", "", "print only comments whose body contains this text")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(stderr, usage)
		return rowreadExitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "statusgen rowread: refusing unexpected argument %q\n%s\n", fs.Arg(0), usage)
		return rowreadExitUsage
	}
	if len(issues) == 0 {
		fmt.Fprintf(stderr, "statusgen rowread: name at least one --issue\n%s\n", usage)
		return rowreadExitUsage
	}
	// Every match is held until every issue has been read. A later unread
	// issue must leave stdout empty, not holding the earlier issues' lines:
	// a row that tests "any output?" without pipefail would read a partial
	// answer as a pass.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	reader := newDeskreadReader()
	for _, is := range issues {
		cs, err := reader.IssueComments(is.repo, is.number)
		if err != nil {
			fmt.Fprintf(stderr, "statusgen rowread: %s#%d: could-not-check: %v\n", is.repo, is.number, err)
			return rowreadExitCouldNot
		}
		for _, c := range cs {
			if *contains != "" && !strings.Contains(c.Body, *contains) {
				continue
			}
			if err := enc.Encode(rowreadMatch{Repo: is.repo, Number: is.number, AuthorLogin: c.User.Login, CreatedAt: c.CreatedAt}); err != nil {
				fmt.Fprintln(stderr, "statusgen rowread: encoding output:", err)
				return rowreadExitCouldNot
			}
		}
	}
	if _, err := stdout.Write(buf.Bytes()); err != nil {
		fmt.Fprintln(stderr, "statusgen rowread: writing output:", err)
		return rowreadExitCouldNot
	}
	return rowreadExitOK
}
