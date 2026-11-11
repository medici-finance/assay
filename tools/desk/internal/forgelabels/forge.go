package forgelabels

// forge.go — the two forge backends (GitHub, GitLab) over one Transport, and the ensure /
// check the operator path runs through them.
//
// ENSURE is create-each, idempotent: a label the forge reports as already present is a named
// no-op. On GitLab that includes a label inherited from an ancestor group — a project-label
// create whose title a group already defines is answered 400 "already exists at group
// level", which reads as present, so an inherited group label is never shadowed by a
// project copy.
//
// CHECK is read-only. It lists the target's labels — on GitLab with include_ancestor_groups,
// so an inherited group label counts as present — and names each canonical label missing. A
// failed or truncated read is an error, the could-not-check third state; it is never
// reported as "nothing missing".
//
// GitLab names with a single colon (`to:worker`, `raised-by:desk`) are PLAIN labels. Only a
// double colon makes a scoped label; nothing here sends, or matches, one.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Response is one forge answer: the HTTP status and the (bounded) body.
type Response struct {
	Status int
	Body   []byte
}

// Transport sends one request to a forge API base and returns its answer. A transport
// failure is an error; any HTTP status, including non-2xx, is a Response for the backend to
// judge. The credential is the transport's business and never crosses this interface.
type Transport interface {
	Do(method, path string, body any) (Response, error)
}

// Backend creates and lists labels on one GitHub repo or GitLab project.
type Backend interface {
	// Forge is "github" or "gitlab".
	Forge() string
	// Target is the repo (owner/name) or project (path or id) the backend writes to.
	Target() string
	// Create creates one label. exists=true means the forge reported it already present.
	Create(l Label) (exists bool, err error)
	// List reads every label name the target can use, inherited ones included.
	List() ([]string, error)
}

// StatusError is a forge's non-2xx answer: the status and, where the forge sent one, its
// short message. It never carries a request header.
type StatusError struct {
	Status  int
	Message string
}

func (e *StatusError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}

// Denied reports whether err is the forge refusing the credential (401 / 403): the credential
// cannot do this, as opposed to the forge failing. A denied ensure is UNFINISHED — the step
// needs a credential that can write labels — and is reported with the command to finish it.
func Denied(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && (se.Status == 401 || se.Status == 403)
}

func statusError(resp Response) error {
	return &StatusError{Status: resp.Status, Message: forgeMessage(resp.Body)}
}

// forgeMessage extracts the forge's short human-readable reason from an error body, or "".
func forgeMessage(body []byte) string {
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return ""
	}
	for _, k := range []string{"message", "error"} {
		if v, ok := m[k]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if len(s) > 300 {
				s = s[:300] + "…"
			}
			return s
		}
	}
	return ""
}

// listPerPage / listMaxPages bound a label read. A target with more labels than the bound
// is reported could-not-check (truncated), never as a partial list a missing label could
// hide behind.
const (
	listPerPage  = 100
	listMaxPages = 50
)

// GitLab returns the GitLab backend for project (a numeric id or a full path; the path is
// escaped here).
func GitLab(t Transport, project string) Backend {
	return &gitlabLabels{c: t, project: project, projectPath: "/projects/" + url.PathEscape(project)}
}

// GitHub returns the GitHub backend for repo (owner/name).
func GitHub(t Transport, repo string) Backend {
	return &githubLabels{c: t, repo: repo}
}

// gitlabLabels: POST /projects/:id/labels. GitLab answers a duplicate with 409 on some
// versions and 400 "already exists" / "has already been taken" on others — both are success.
// A title an ancestor group already defines is 400 "already exists at group level": the
// inherited label is present.
type gitlabLabels struct {
	c           Transport
	project     string
	projectPath string // "/projects/<id-or-encoded-path>"
}

func (g *gitlabLabels) Forge() string  { return "gitlab" }
func (g *gitlabLabels) Target() string { return g.project }

func (g *gitlabLabels) Create(l Label) (bool, error) {
	resp, err := g.c.Do("POST", g.projectPath+"/labels",
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
	return false, statusError(resp)
}

// List reads GET /projects/:id/labels with include_ancestor_groups=true, so a label the
// project inherits from its group (or any ancestor) is listed — it counts as present.
func (g *gitlabLabels) List() ([]string, error) {
	return listPages(func(page int) (Response, error) {
		return g.c.Do("GET", g.projectPath+"/labels?include_ancestor_groups=true&per_page="+
			strconv.Itoa(listPerPage)+"&page="+strconv.Itoa(page), nil)
	})
}

// githubLabels: POST /repos/:owner/:repo/labels. GitHub answers a duplicate with 422 and an
// `already_exists` error code.
type githubLabels struct {
	c    Transport
	repo string // owner/name
}

func (g *githubLabels) Forge() string  { return "github" }
func (g *githubLabels) Target() string { return g.repo }

func (g *githubLabels) reposPath() string {
	owner, name, _ := strings.Cut(g.repo, "/")
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}

func (g *githubLabels) Create(l Label) (bool, error) {
	resp, err := g.c.Do("POST", g.reposPath()+"/labels",
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
	return false, statusError(resp)
}

func (g *githubLabels) List() ([]string, error) {
	return listPages(func(page int) (Response, error) {
		return g.c.Do("GET", g.reposPath()+"/labels?per_page="+strconv.Itoa(listPerPage)+
			"&page="+strconv.Itoa(page), nil)
	})
}

// listPages walks a paginated label list to exhaustion: a page shorter than listPerPage is
// the last. Any failed page fails the whole read, and running past listMaxPages is a
// truncation — both errors, never a partial answer.
func listPages(get func(page int) (Response, error)) ([]string, error) {
	var names []string
	for page := 1; page <= listMaxPages; page++ {
		resp, err := get(page)
		if err != nil {
			return nil, err
		}
		if resp.Status != 200 {
			return nil, statusError(resp)
		}
		var chunk []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(resp.Body, &chunk); err != nil {
			return nil, fmt.Errorf("label list page %d: unreadable response: %v", page, err)
		}
		for _, l := range chunk {
			names = append(names, l.Name)
		}
		if len(chunk) < listPerPage {
			return names, nil
		}
	}
	return nil, fmt.Errorf("label list truncated at %d pages of %d — cannot tell which labels are missing",
		listMaxPages, listPerPage)
}

// Result is one label's ensure outcome. Err == nil && !Exists means created.
type Result struct {
	Label  Label
	Exists bool
	Err    error
}

// Ensure creates every label in ls through b, in order. A failure does not stop the labels
// after it: each row's outcome is returned.
func Ensure(b Backend, ls []Label) []Result {
	out := make([]Result, 0, len(ls))
	for _, l := range ls {
		exists, err := b.Create(l)
		out = append(out, Result{Label: l, Exists: exists, Err: err})
	}
	return out
}

// Failed counts the failed rows of rs, and reports whether any failure was the forge
// refusing the credential (the step is unfinished: it needs a credential that can write).
func Failed(rs []Result) (n int, denied bool) {
	for _, r := range rs {
		if r.Err != nil {
			n++
			denied = denied || Denied(r.Err)
		}
	}
	return n, denied
}

// Check reads b's labels and returns the names of ls the target does not have, in ls's
// order. Matching is case-insensitive, as both forges treat label names for uniqueness (and
// as deskfile's existence probe matches). An error is could-not-check: the caller must never
// read it as "nothing missing".
func Check(b Backend, ls []Label) (missing []string, err error) {
	names, err := b.List()
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, n := range names {
		have[strings.ToLower(strings.TrimSpace(n))] = true
	}
	for _, l := range ls {
		if !have[strings.ToLower(l.Name)] {
			missing = append(missing, l.Name)
		}
	}
	return missing, nil
}

// CredentialPlaceholder is what a printed finish command names in place of a credential:
// the command takes a FILE, and this package never prints a path it was not given.
const CredentialPlaceholder = "<FILE holding an operator credential that can write labels>"

// RolesPlaceholder is what a printed command names in place of --roles when the caller had
// no bound role to derive the set from: the roster's role= names, which the operator fills in
// once the roster exists. The command never falls back to a guessed list silently.
const RolesPlaceholder = "<the roster's role= names, comma-separated>"

// Command is the exact `deskfleet labels` command that ensures the set for roles on one
// target — what an unfinished step prints so the operator can finish it. apiBase is only
// rendered for GitLab, whose base has no default. check=true renders the read-only form. No
// roles renders RolesPlaceholder in --roles' place.
func Command(forge, target string, roles []string, apiBase string, check bool) string {
	var b strings.Builder
	if forge == "gitlab" {
		b.WriteString("GITLAB_API_BASE=" + shellWord(apiBase) + " ")
	}
	b.WriteString("deskfleet labels --forge " + forge)
	if forge == "gitlab" {
		b.WriteString(" --project " + shellWord(target))
	} else {
		b.WriteString(" --repo " + shellWord(target))
	}
	if rs := NormalizeRoles(roles); len(rs) > 0 {
		b.WriteString(" --roles " + strings.Join(rs, ","))
	} else {
		b.WriteString(" --roles " + RolesPlaceholder)
	}
	if check {
		b.WriteString(" --check")
	}
	b.WriteString(" --token-file " + CredentialPlaceholder)
	return b.String()
}

// shellWord single-quotes s unless it is made only of characters a POSIX shell reads
// literally.
func shellWord(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool { return !shellLiteral(r) }) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellLiteral reports whether r is an ASCII letter or digit, or one of - _ . / : @ % + =.
func shellLiteral(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune("-_./:@%+=", r)
}
