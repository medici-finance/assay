package main

// labels.go — ONE label table (internal/forgelabels), two forge backends. `deskfleet labels`
// drives either backend; `deskfleet provision --project` drives the GitLab one. Creation is
// idempotent on both: a label that already exists — on GitLab, one inherited from an
// ancestor group too — is a named no-op, never an error. `labels --check` is the read-only
// form: it names each label of the set the target lacks and creates nothing.

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/forgelabels"
)

// fleetTransport carries forgelabels' requests over this verb's apiClient: the credential
// stays in the client's header, redirects stay refused, and the base URL stays validated.
type fleetTransport struct{ c *apiClient }

func (t fleetTransport) Do(method, path string, body any) (forgelabels.Response, error) {
	resp, err := t.c.do(method, path, body)
	if err != nil {
		return forgelabels.Response{}, err
	}
	return forgelabels.Response{Status: resp.Status, Body: resp.Body}, nil
}

// ensureLabels creates every row of ls through b. A failure is reported through fail and does
// not stop the rows after it.
func ensureLabels(b forgelabels.Backend, ls []fleetLabel, out io.Writer, fail func(string, ...any)) {
	for _, r := range forgelabels.Ensure(b, ls) {
		switch {
		case r.Err != nil:
			fail("label '%s' not created on %s (%v) — the desk write that reaches for it (deskflip queue swap / "+
				"deskfile --raised-by / --to) degrades silently until fixed", r.Label.Name, b.Forge(), r.Err)
		case r.Exists:
			fmt.Fprintf(out, "label: '%s' already exists (no-op)\n", r.Label.Name)
		default:
			fmt.Fprintf(out, "label: created '%s' (#%s)\n", r.Label.Name, r.Label.Color)
		}
	}
}

// labelsOpts are one `labels` run's settings, typed by the command tree.
type labelsOpts struct {
	forge, repo, project string
	tokenFile            string
	dryRun               bool
	// check is the read-only form: list the target's labels and name the missing ones.
	check bool
	// rolesRaw is --roles as given ("" = not given); roles is it parsed (validateLabels).
	rolesRaw string
	roles    []string
	// apiBase is GITLAB_API_BASE as the verb resolved it (environment only; --forge gitlab).
	apiBase string
	// target is the validated repo (github) or project (gitlab); validateLabels sets it.
	target string
}

// validateLabels is the domain check on the typed options, in the order and with the
// messages the Go flag parser's checks had. It reads nothing and touches nothing.
func validateLabels(o *labelsOpts) error {
	switch o.forge {
	case "github":
		parts := strings.Split(o.repo, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || o.project != "" {
			return errors.New("--forge github takes --repo <owner/name> (and no --project)")
		}
		o.target = o.repo
	case "gitlab":
		if o.project == "" || o.repo != "" {
			return errors.New("--forge gitlab takes --project <path-or-id> (and no --repo)")
		}
		o.target = o.project
	default:
		return errors.New("--forge must be github or gitlab")
	}
	if o.check && o.dryRun {
		return errors.New("--check and --dry-run are exclusive: --check already writes nothing (it reads the target's labels)")
	}
	if o.rolesRaw != "" {
		roles, err := forgelabels.ParseRoles(o.rolesRaw)
		if err != nil {
			return err
		}
		o.roles = roles
	}
	return nil
}

// labelSet is the set one run works on: the --roles set when given, else the documented-role
// fallback, announced on stderr so a run that silently omits a bound role cannot pass for a
// roster-derived one.
func labelSet(o labelsOpts, stderr io.Writer) []fleetLabel {
	if len(o.roles) > 0 {
		return forgelabels.Canonical(o.roles)
	}
	fmt.Fprintf(stderr, "NOTICE: --roles not given — using the documented desk roles (%s); pass the roster's "+
		"role= names with --roles when the fleet binds other roles, so every `to:<role>` deskfile --to stamps exists\n",
		strings.Join(forgelabels.DocumentedRoles(), ","))
	return fleetLabels
}

// runLabels is the `labels` handler: the command tree has parsed and validated o.
func runLabels(o labelsOpts, e *env) int {
	target := o.target
	set := labelSet(o, e.stderr)
	if o.dryRun {
		for _, l := range set {
			fmt.Fprintf(e.stdout, "[dry-run] would create %s label '%s' (#%s) on %s (idempotent)\n", o.forge, l.Name, l.Color, target)
		}
		return exitOK
	}

	var b forgelabels.Backend
	var base string
	switch o.forge {
	case "github":
		base = strings.TrimRight(e.githubAPIBase, "/")
		if err := validateAPIBase("GitHub API base", base); err != nil {
			fmt.Fprintf(e.stderr, "refused: %v\n", err)
			return exitRefused
		}
		tok, err := readCredentialFile("--token-file", o.tokenFile, operatorNamedFile)
		if err != nil {
			fmt.Fprintf(e.stderr, "refused: %v\n", err)
			return exitRefused
		}
		b = forgelabels.GitHub(fleetTransport{newGitHubClient(base, e.http, tok)}, target)
	case "gitlab":
		var err error
		base, err = gitlabAPIBase(o.apiBase)
		if err != nil {
			fmt.Fprintf(e.stderr, "refused: %v\n", err)
			return exitRefused
		}
		tok, err := readCredentialFile("--token-file", o.tokenFile, operatorNamedFile)
		if err != nil {
			fmt.Fprintf(e.stderr, "refused: %v\n", err)
			return exitRefused
		}
		b = forgelabels.GitLab(fleetTransport{newGitLabClient(base, e.http, tok)}, target)
	}
	// Name the target BEFORE the first request, as provision does: a network-reaching mode
	// says where it is about to write before it writes there.
	fmt.Fprintf(e.stdout, "target: %s %s %s\n", base, o.forge, target)

	if o.check {
		return checkLabels(b, set, e)
	}

	failed := 0
	ensureLabels(b, set, e.stdout, func(format string, a ...any) {
		failed++
		fmt.Fprintf(e.stderr, "error: "+format+"\n", a...)
	})
	if failed > 0 {
		fmt.Fprintf(e.stderr, "deskfleet labels: %d of %d label(s) could not be created\n", failed, len(set))
		return exitFailed
	}
	return exitOK
}

// checkLabels is `labels --check`: read-only, three states. Every label present is exit 0;
// a missing label is named on its own line and the run exits 1; a read that fails is
// could-not-check (exit 6), never reported as nothing missing.
func checkLabels(b forgelabels.Backend, set []fleetLabel, e *env) int {
	missing, err := forgelabels.Check(b, set)
	if err != nil {
		fmt.Fprintf(e.stderr, "could-not-check: the labels on %s %s could not be read (%v) — no label is reported present or missing\n",
			b.Forge(), b.Target(), err)
		return exitUnverifiable
	}
	for _, m := range missing {
		fmt.Fprintf(e.stdout, "label: MISSING '%s'\n", m)
	}
	if len(missing) > 0 {
		fmt.Fprintf(e.stderr, "deskfleet labels --check: %d of %d label(s) missing on %s\n", len(missing), len(set), b.Target())
		return exitFailed
	}
	fmt.Fprintf(e.stdout, "label: all %d present\n", len(set))
	return exitOK
}
