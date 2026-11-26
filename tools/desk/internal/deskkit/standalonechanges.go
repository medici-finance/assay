package deskkit

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type StandaloneChangeConfig struct {
	ProjectURL string // canonical HTTPS web URL
	Repository string // owner/project
	Token      string
	HTTPClient *http.Client

	// PublicationNotice receives a diagnostic when part of the publication
	// policy could not be checked. A nil handler writes the diagnostic to stderr.
	PublicationNotice func(string)

	// GitLabInstance is the preconfigured GitLab installation URL. The project
	// must match this instance and installation path before any network access.
	GitLabInstance string

	// GitLabLoopback is an optional HTTPS origin on 127.0.0.1 for a caller's
	// validating TLS proxy. The installation path still comes from ProjectURL.
	GitLabLoopback string
}

// StandaloneChanges binds all operations to one repository. Use one client per goroutine.
type StandaloneChanges struct {
	forge     Forge
	repo      ForgeRepo
	changeURL string
	notice    func(string)
}

var (
	standalonePathSegment = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	standaloneRepoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	standaloneSHAPattern  = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)
)

func standaloneHTTPSURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	return u, err == nil && u.Scheme == "https" && u.Opaque == "" && u.Hostname() != "" && u.User == nil &&
		u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && u.RawPath == ""
}

func NewStandaloneChanges(cfg StandaloneChangeConfig) (*StandaloneChanges, error) {
	owner, name, ok := strings.Cut(cfg.Repository, "/")
	if !ok || !standaloneRepoPattern.MatchString(cfg.Repository) || owner == "." || owner == ".." || name == "." || name == ".." {
		return nil, errors.New("expected an owner/project repository")
	}
	proj, ok := standaloneHTTPSURL(cfg.ProjectURL)
	if !ok || !strings.HasSuffix(proj.Path, "/"+cfg.Repository) || proj.String() != cfg.ProjectURL {
		return nil, errors.New("project URL must be HTTPS and match the repository")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("an explicit forge credential is required")
	}
	notice := cfg.PublicationNotice
	if notice == nil {
		notice = func(message string) { _, _ = fmt.Fprintln(os.Stderr, message) }
	}
	c := &StandaloneChanges{repo: ForgeRepo{Owner: owner, Name: name}, notice: notice}
	var gitlabBase string
	apiBase := GitHubAPIBase
	kind := ForgeGitHub
	switch {
	case strings.EqualFold(proj.Hostname(), "github.com"):
		if proj.Host != "github.com" || proj.Path != "/"+cfg.Repository || cfg.GitLabInstance != "" || cfg.GitLabLoopback != "" {
			return nil, errors.New("GitHub project must be on github.com")
		}
		c.changeURL = cfg.ProjectURL + "/pull/"
	case cfg.GitLabInstance != "":
		kind = ForgeGitLab
		base := *proj
		base.Path = strings.TrimSuffix(proj.Path, "/"+cfg.Repository)
		if base.String() != cfg.GitLabInstance {
			return nil, errors.New("GitLab project URL must match the configured installation")
		}
		if base.Path != "" {
			for _, segment := range strings.Split(strings.TrimPrefix(base.Path, "/"), "/") {
				if !standalonePathSegment.MatchString(segment) || segment == "." || segment == ".." {
					return nil, errors.New("invalid GitLab installation path")
				}
			}
		}
		if cfg.GitLabLoopback != "" {
			local, ok := standaloneHTTPSURL(cfg.GitLabLoopback)
			if !ok || local.Hostname() != "127.0.0.1" || local.Port() == "" || local.Path != "" {
				return nil, errors.New("GitLab proxy must be a loopback HTTPS origin")
			}
			base.Host = local.Host
		}
		gitlabBase = base.String()
		apiBase = gitlabBase
		c.changeURL = cfg.ProjectURL + "/-/merge_requests/"
	default:
		return nil, errors.New("project must identify github.com or the configured GitLab installation")
	}
	forge, err := newStandaloneChangeBackend(kind, c.repo, cfg.Token, gitlabBase, standaloneBoundHTTP(cfg.HTTPClient, apiBase))
	if err != nil {
		return nil, err
	}
	c.forge = forge
	return c, nil
}

type standaloneOriginTransport struct {
	host string
	next http.RoundTripper
}

func (t standaloneOriginTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != t.host {
		return nil, errors.New("forge request left its configured HTTPS origin")
	}
	resp, err := t.next.RoundTrip(req)
	if err == nil && resp != nil && resp.StatusCode >= 300 && resp.StatusCode < 400 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("forge returned a redirect (HTTP %d)", resp.StatusCode)
	}
	return resp, err
}

func standaloneBoundHTTP(client *http.Client, base string) *http.Client {
	c := BoundedHTTPClient()
	if client != nil {
		c.Transport = client.Transport
		if client.Timeout > 0 {
			c.Timeout = client.Timeout
		}
	}
	rt := c.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	u, _ := url.Parse(base)
	c.Transport = standaloneOriginTransport{host: u.Host, next: rt}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}

// OpenChanges returns change numbers and whether the provider scan was capped.
func (c *StandaloneChanges) OpenChanges() ([]int, bool, error) {
	numbers, capped, err := standaloneOpenChangeNumbers(c.forge, c.repo)
	if err != nil {
		return nil, false, standaloneOperationError("list open changes", err)
	}
	for _, number := range numbers {
		if number <= 0 {
			return nil, false, errors.New("forge returned an invalid open change number")
		}
	}
	return numbers, capped, nil
}

type StandaloneChange struct {
	State        string `json:"state"`
	HeadSHA      string `json:"headSha"`
	BaseRef      string `json:"baseRef"`
	URL          string `json:"url"`
	ChangedFiles int    `json:"changedFiles"`
}

func (c *StandaloneChanges) Change(number int) (StandaloneChange, error) {
	if number < 1 {
		return StandaloneChange{}, errors.New("change number must be positive")
	}
	pr, err := c.forge.GetPullRequest(c.repo, number)
	if err != nil {
		return StandaloneChange{}, standaloneOperationError("read change", err)
	}
	if pr == nil || pr.Number != number || pr.URL != fmt.Sprintf("%s%d", c.changeURL, number) ||
		!standaloneSHAPattern.MatchString(pr.HeadSHA) {
		return StandaloneChange{}, errors.New("forge returned a mismatched or incomplete change")
	}
	return StandaloneChange{State: strings.ToLower(pr.State), HeadSHA: pr.HeadSHA, BaseRef: pr.BaseRef, URL: pr.URL, ChangedFiles: pr.ChangedFiles}, nil
}

type StandaloneChangeFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previousFilename,omitempty"`
	Status           string `json:"status"`
	Patch            string `json:"patch,omitempty"`
	PatchAbsent      bool   `json:"patchAbsent"`
}

type StandaloneChangeFiles struct {
	HeadSHA      string                 `json:"headSha"`
	Complete     bool                   `json:"complete"`
	ChangedFiles int                    `json:"changedFiles"`
	Files        []StandaloneChangeFile `json:"files"`
}

// ChangedFiles brackets the file list with change reads and returns the
// observed head. Callers pairing it with an earlier observation must compare SHAs.
func (c *StandaloneChanges) ChangedFiles(number int) (StandaloneChangeFiles, error) {
	pr, err := c.Change(number)
	if err != nil {
		return StandaloneChangeFiles{}, err
	}
	files, err := c.forge.ListChangedFiles(c.repo, number)
	if err != nil {
		return StandaloneChangeFiles{}, standaloneOperationError("list changed files", err)
	}
	after, err := c.Change(number)
	if err != nil {
		return StandaloneChangeFiles{}, err
	}
	if after != pr {
		return StandaloneChangeFiles{}, errors.New("change moved during the file read")
	}
	out := StandaloneChangeFiles{HeadSHA: pr.HeadSHA, Complete: len(files) == pr.ChangedFiles, ChangedFiles: pr.ChangedFiles, Files: make([]StandaloneChangeFile, 0, len(files))}
	for _, file := range files {
		out.Files = append(out.Files, StandaloneChangeFile(file))
	}
	return out, nil
}

type StandaloneChangeComment struct {
	AuthorID   int64  `json:"authorId"`
	AuthorType string `json:"authorType,omitempty"`
	DatabaseID int64  `json:"databaseId"`
	URL        string `json:"url"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	Body       string `json:"body"`
}

func (c *StandaloneChanges) Comments(number int) ([]StandaloneChangeComment, error) {
	if number < 1 {
		return nil, errors.New("change number must be positive")
	}
	rows, err := c.forge.ListCommentsTyped(c.repo, number, TargetChange)
	if err != nil {
		return nil, standaloneOperationError("list change comments", err)
	}
	out := make([]StandaloneChangeComment, 0, len(rows))
	for _, row := range rows {
		out = append(out, StandaloneChangeComment{row.Author.ID, row.Author.Type, row.DatabaseID, row.URL, row.CreatedAt, row.UpdatedAt, row.Body})
	}
	return out, nil
}

func standaloneOperationError(operation string, err error) error {
	var api *ForgeAPIError
	if errors.As(err, &api) {
		return fmt.Errorf("%s failed (HTTP %d)", operation, api.Status)
	}
	var transport *url.Error
	if errors.As(err, &transport) {
		return fmt.Errorf("%s failed: %w", operation, transport.Err)
	}
	return fmt.Errorf("%s could not be verified", operation)
}
