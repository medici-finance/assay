package main

// labels.go — ONE label table (tables.go fleetLabels), two forge backends. `deskfleet labels`
// drives either backend; `deskfleet provision --project` drives the GitLab one. Creation is
// idempotent on both: a label that already exists is a named no-op, never an error.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// labelBackend creates one label on one forge. exists=true means the forge reported the
// label already present (a no-op).
type labelBackend interface {
	forge() string
	create(l fleetLabel) (exists bool, err error)
}

// gitlabLabels: POST /projects/:id/labels. GitLab answers a duplicate with 409 on some
// versions and 400 "already exists" / "has already been taken" on others — both are success.
type gitlabLabels struct {
	c           *apiClient
	projectPath string // "/projects/<id-or-encoded-path>"
}

func (g *gitlabLabels) forge() string { return "gitlab" }

func (g *gitlabLabels) create(l fleetLabel) (bool, error) {
	resp, err := g.c.do("POST", g.projectPath+"/labels",
		map[string]any{"name": l.Name, "color": "#" + l.Color, "description": l.Description})
	if err != nil {
		return false, err
	}
	switch {
	case resp.Status == 200 || resp.Status == 201:
		return false, nil
	case resp.Status == 409:
		return true, nil
	case resp.Status == 400:
		body := string(resp.Body)
		if strings.Contains(body, "already exists") || strings.Contains(body, "has already been taken") {
			return true, nil
		}
	}
	return false, errors.New(statusText(resp))
}

// githubLabels: POST /repos/:owner/:repo/labels. GitHub answers a duplicate with 422 and an
// `already_exists` error code.
type githubLabels struct {
	c    *apiClient
	repo string // owner/name
}

func (g *githubLabels) forge() string { return "github" }

func (g *githubLabels) create(l fleetLabel) (bool, error) {
	owner, name, _ := strings.Cut(g.repo, "/")
	resp, err := g.c.do("POST", "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+"/labels",
		map[string]any{"name": l.Name, "color": l.Color, "description": l.Description})
	if err != nil {
		return false, err
	}
	if resp.Status == 201 {
		return false, nil
	}
	if resp.Status == 422 {
		var e struct {
			Errors []struct {
				Code string `json:"code"`
			} `json:"errors"`
		}
		if json.Unmarshal(resp.Body, &e) == nil {
			for _, x := range e.Errors {
				if x.Code == "already_exists" {
					return true, nil
				}
			}
		}
	}
	return false, errors.New(statusText(resp))
}

// ensureLabels creates every fleetLabels row through b. A failure is reported through fail
// and does not stop the rows after it.
func ensureLabels(b labelBackend, out io.Writer, fail func(string, ...any)) {
	for _, l := range fleetLabels {
		exists, err := b.create(l)
		switch {
		case err != nil:
			fail("label '%s' not created on %s (%v) — the desk write that reaches for it (deskflip queue swap / "+
				"deskfile --raised-by) degrades silently until fixed", l.Name, b.forge(), err)
		case exists:
			fmt.Fprintf(out, "label: '%s' already exists (no-op)\n", l.Name)
		default:
			fmt.Fprintf(out, "label: created '%s' (#%s)\n", l.Name, l.Color)
		}
	}
}

// labelsOpts are one `labels` run's settings, typed by the command tree.
type labelsOpts struct {
	forge, repo, project string
	tokenFile            string
	dryRun               bool
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
	return nil
}

// runLabels is the `labels` handler: the command tree has parsed and validated o.
func runLabels(o labelsOpts, e *env) int {
	target := o.target
	if o.dryRun {
		for _, l := range fleetLabels {
			fmt.Fprintf(e.stdout, "[dry-run] would create %s label '%s' (#%s) on %s (idempotent)\n", o.forge, l.Name, l.Color, target)
		}
		return exitOK
	}

	var b labelBackend
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
		b = &githubLabels{c: newGitHubClient(base, e.http, tok), repo: target}
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
		b = &gitlabLabels{c: newGitLabClient(base, e.http, tok), projectPath: "/projects/" + url.PathEscape(target)}
	}
	// Name the target BEFORE the first request, as provision does: a network-reaching mode
	// says where it is about to write before it writes there.
	fmt.Fprintf(e.stdout, "target: %s %s %s\n", base, o.forge, target)

	failed := 0
	ensureLabels(b, e.stdout, func(format string, a ...any) {
		failed++
		fmt.Fprintf(e.stderr, "error: "+format+"\n", a...)
	})
	if failed > 0 {
		fmt.Fprintf(e.stderr, "deskfleet labels: %d of %d label(s) could not be created\n", failed, len(fleetLabels))
		return exitFailed
	}
	return exitOK
}
