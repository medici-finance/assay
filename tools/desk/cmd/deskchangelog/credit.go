package main

// The release-credit identity classifier, ported from tools/changelog/credit-identity.sh
// (now an adapter onto `deskchangelog credit-identity`).
//
// A release credits the people who contributed a notable change. This decides, for one
// pull-request author login, whether the author is credited or skipped, and why. It prints
// exactly one verdict on stdout and always exits 0 — a credit must never be able to fail a
// release:
//
//   credit       an external contributor: credit them
//   skip:roster  a trusted maintainer, a mapped human login, or a trusted bot slug
//   skip:bot     a bot by shape (`<x>[bot]`, `app/<x>`), roster or not
//   skip:opt-out the pull-request body carries the opt-out marker line
//   skip:unknown no login, or no roster configured at all (fail closed: credit nobody)
//
// Comparison is ASCII case-insensitive and ignores ASCII whitespace. Roster lists separate
// entries with commas or newlines.

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

const optOutMarker = "<!-- changelog-credit: no -->"

const creditUsage = "usage: credit-identity.sh classify <login> [<body-file>]"

// normLogin is `tr '[:upper:]' '[:lower:]' | tr -d '[:space:]'` over ASCII.
func normLogin(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r':
			continue
		case c >= 'A' && c <= 'Z':
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

// rosterEntries splits a roster list on commas and newlines.
func rosterEntries(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' })
}

// optedOut reports whether body (a regular file, symlinks followed) carries the marker as a
// whole line once every carriage return is deleted.
func optedOut(body string) bool {
	if body == "" {
		return false
	}
	if st, err := os.Stat(body); err != nil || !st.Mode().IsRegular() {
		return false
	}
	b, err := os.ReadFile(body)
	if err != nil {
		return false
	}
	for _, ln := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
		if ln == optOutMarker {
			return true
		}
	}
	return false
}

func classifyLogin(login, body string, v *cli.Values) string {
	l := normLogin(login)
	if l == "" {
		return "skip:unknown"
	}
	logins, slugs, humans := v.String("trusted-logins"), v.String("trusted-bot-slugs"), v.String("human-login-map")
	if logins == "" && slugs == "" && humans == "" {
		return "skip:unknown"
	}
	if optedOut(body) {
		return "skip:opt-out"
	}
	for _, e := range rosterEntries(logins) {
		name, _, _ := strings.Cut(e, ":")
		if n := normLogin(name); n != "" && n == l {
			return "skip:roster"
		}
	}
	for _, e := range rosterEntries(humans) {
		i := strings.LastIndex(e, ":")
		if i < 0 {
			continue
		}
		if n := normLogin(e[i+1:]); n != "" && n == l {
			return "skip:roster"
		}
	}
	if strings.HasSuffix(l, "[bot]") || strings.HasPrefix(l, "app/") {
		return "skip:bot"
	}
	for _, e := range rosterEntries(slugs) {
		if _, after, found := strings.Cut(e, "="); found {
			e = after
		}
		slug, _, _ := strings.Cut(e, ":")
		if n := normLogin(slug); n != "" && n == l {
			return "skip:roster"
		}
	}
	return "credit"
}

// creditBindings are the classifier's inputs: the roster, from the environment only.
func creditBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "trusted-logins", Kind: cli.String, Env: []string{"ASSAY_TRUSTED_LOGINS"},
			Usage: "trusted maintainer logins"},
		{Key: "trusted-bot-slugs", Kind: cli.String, Env: []string{"ASSAY_TRUSTED_BOT_SLUGS"},
			Usage: "trusted bot slugs"},
		{Key: "human-login-map", Kind: cli.String, Env: []string{"ASSAY_HUMAN_LOGIN_MAP"},
			Usage: "name:login map of the operator's people"},
	}
}

func creditIdentityCommand(stdout, stderr io.Writer) *cobra.Command {
	usage := func() error {
		fmt.Fprintln(stderr, creditUsage)
		fmt.Fprintln(stdout, "skip:unknown")
		return nil
	}
	parent := &cobra.Command{
		Use:   "credit-identity",
		Short: "classify a pull-request author for release credit",
		Long: `Classifies a pull-request author login for release credit. Run
'deskchangelog credit-identity classify --help'. Anything but 'classify' prints the
usage line and the fail-closed verdict 'skip:unknown', exit 0.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(*cobra.Command, []string) error { return usage() },
	}
	classify := &cobra.Command{
		Use:   "classify <login> [<body-file>]",
		Short: "print credit, skip:roster, skip:bot, skip:opt-out or skip:unknown",
		Long: `Prints one verdict for <login>: 'credit' for an external contributor;
'skip:roster' for a trusted login, a mapped human login or a trusted bot slug;
'skip:bot' for a bot-shaped login ('<x>[bot]', 'app/<x>'); 'skip:opt-out' when
<body-file> carries the line '<!-- changelog-credit: no -->'; 'skip:unknown' when
the login is empty or no roster is configured (fail closed).

Environment: ASSAY_TRUSTED_LOGINS (login[:id] entries), ASSAY_TRUSTED_BOT_SLUGS
([role=]slug[:id] entries), ASSAY_HUMAN_LOGIN_MAP (name:login entries); entries
separated by commas or newlines.

Exit codes: 0 always, after parsing · 2 an unknown flag.`,
		Args: cobra.ArbitraryArgs,
	}
	set := cli.Declare(classify, creditBindings()...)
	classify.RunE = func(_ *cobra.Command, a []string) error {
		if len(a) == 0 {
			return usage()
		}
		v, err := resolve(set)
		if err != nil {
			return err
		}
		body := ""
		if len(a) > 1 {
			body = a[1]
		}
		fmt.Fprintln(stdout, classifyLogin(a[0], body, v))
		return nil
	}
	parent.AddCommand(classify)
	return parent
}
