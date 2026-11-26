// Package forgechange exposes repository-scoped PR and MR operations backed by
// the desk-tools forge adapters, with credentials and policy supplied by the caller.
package forgechange

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type Config struct {
	Forge      string // github or gitlab
	ProjectURL string // canonical HTTPS web URL
	Repository string // owner/project
	Token      string
	HTTPClient *http.Client

	// GitLabLoopback is an optional HTTPS origin on 127.0.0.1 for a caller's
	// validating TLS proxy. The installation path still comes from ProjectURL.
	GitLabLoopback string
}

// Client binds all operations to one repository. Use one client per goroutine.
type Client struct {
	forge     deskkit.Forge
	repo      deskkit.ForgeRepo
	changeURL string
}

var (
	pathSegment = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	shaPattern  = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)
	idPattern   = regexp.MustCompile(`^[1-9][0-9]*$`)
)

func httpsURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	return u, err == nil && u.Scheme == "https" && u.Opaque == "" && u.Hostname() != "" && u.User == nil &&
		u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && u.RawPath == ""
}

func New(cfg Config) (*Client, error) {
	owner, name, ok := strings.Cut(cfg.Repository, "/")
	if !ok || !repoPattern.MatchString(cfg.Repository) || owner == "." || owner == ".." || name == "." || name == ".." {
		return nil, errors.New("expected an owner/project repository")
	}
	proj, ok := httpsURL(cfg.ProjectURL)
	if !ok || !strings.HasSuffix(proj.Path, "/"+cfg.Repository) || proj.String() != cfg.ProjectURL {
		return nil, errors.New("project URL must be HTTPS and match the repository")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("an explicit forge credential is required")
	}
	c := &Client{repo: deskkit.ForgeRepo{Owner: owner, Name: name}}
	var gitlabBase string
	apiBase := deskkit.GitHubAPIBase
	switch cfg.Forge {
	case "github":
		if proj.Host != "github.com" || proj.Path != "/"+cfg.Repository || cfg.GitLabLoopback != "" {
			return nil, errors.New("GitHub project must be on github.com")
		}
		c.changeURL = cfg.ProjectURL + "/pull/"
	case "gitlab":
		base := *proj
		base.Path = strings.TrimSuffix(proj.Path, "/"+cfg.Repository)
		if base.Path != "" {
			for _, segment := range strings.Split(strings.TrimPrefix(base.Path, "/"), "/") {
				if !pathSegment.MatchString(segment) || segment == "." || segment == ".." {
					return nil, errors.New("invalid GitLab installation path")
				}
			}
		}
		if cfg.GitLabLoopback != "" {
			local, ok := httpsURL(cfg.GitLabLoopback)
			if !ok || local.Hostname() != "127.0.0.1" || local.Port() == "" || local.Path != "" {
				return nil, errors.New("GitLab proxy must be a loopback HTTPS origin")
			}
			base.Host = local.Host
		}
		gitlabBase = base.String()
		apiBase = gitlabBase
		c.changeURL = cfg.ProjectURL + "/-/merge_requests/"
	default:
		return nil, errors.New("forge must be github or gitlab")
	}
	forge, err := deskkit.NewStandaloneChangeForge(deskkit.ForgeKind(cfg.Forge), c.repo, cfg.Token, gitlabBase, boundHTTP(cfg.HTTPClient, apiBase))
	if err != nil {
		return nil, err
	}
	c.forge = forge
	return c, nil
}

type originTransport struct {
	host string
	next http.RoundTripper
}

func (t originTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != t.host {
		return nil, errors.New("forge request left its configured HTTPS origin")
	}
	resp, err := t.next.RoundTrip(req)
	if err == nil && resp != nil && resp.StatusCode >= 300 && resp.StatusCode < 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("forge returned a redirect (HTTP %d)", resp.StatusCode)
	}
	return resp, err
}

func boundHTTP(client *http.Client, base string) *http.Client {
	c := deskkit.BoundedHTTPClient()
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
	c.Transport = originTransport{host: u.Host, next: rt}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}

// OpenChanges returns change numbers and whether the provider scan was capped.
func (c *Client) OpenChanges() ([]int, bool, error) {
	numbers, capped, err := deskkit.StandaloneOpenChangeNumbers(c.forge, c.repo)
	if err != nil {
		return nil, false, operationError("list open changes", err)
	}
	for _, number := range numbers {
		if number <= 0 {
			return nil, false, errors.New("forge returned an invalid open change number")
		}
	}
	return numbers, capped, nil
}

type Change struct {
	State        string `json:"state"`
	HeadSHA      string `json:"headSha"`
	BaseRef      string `json:"baseRef"`
	URL          string `json:"url"`
	ChangedFiles int    `json:"changedFiles"`
}

func (c *Client) Change(number int) (Change, error) {
	if number < 1 {
		return Change{}, errors.New("change number must be positive")
	}
	pr, err := c.forge.GetPullRequest(c.repo, number)
	if err != nil {
		return Change{}, operationError("read change", err)
	}
	if pr == nil || pr.Number != number || pr.URL != fmt.Sprintf("%s%d", c.changeURL, number) ||
		!shaPattern.MatchString(pr.HeadSHA) {
		return Change{}, errors.New("forge returned a mismatched or incomplete change")
	}
	return Change{State: strings.ToLower(pr.State), HeadSHA: pr.HeadSHA, BaseRef: pr.BaseRef, URL: pr.URL, ChangedFiles: pr.ChangedFiles}, nil
}

type File struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previousFilename,omitempty"`
	Status           string `json:"status"`
	Patch            string `json:"patch,omitempty"`
	PatchAbsent      bool   `json:"patchAbsent"`
}

type Files struct {
	Complete     bool   `json:"complete"`
	ChangedFiles int    `json:"changedFiles"`
	Files        []File `json:"files"`
}

func (c *Client) ChangedFiles(number int) (Files, error) {
	pr, err := c.Change(number)
	if err != nil {
		return Files{}, err
	}
	files, err := c.forge.ListChangedFiles(c.repo, number)
	if err != nil {
		return Files{}, operationError("list changed files", err)
	}
	out := Files{Complete: len(files) == pr.ChangedFiles, ChangedFiles: pr.ChangedFiles, Files: make([]File, 0, len(files))}
	for _, file := range files {
		out.Files = append(out.Files, File{file.Filename, file.PreviousFilename, file.Status, file.Patch, file.PatchAbsent})
	}
	return out, nil
}

type Comment struct {
	AuthorID   int64  `json:"authorId"`
	AuthorType string `json:"authorType,omitempty"`
	DatabaseID int64  `json:"databaseId"`
	URL        string `json:"url"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	Body       string `json:"body"`
}

func (c *Client) Comments(number int) ([]Comment, error) {
	if number < 1 {
		return nil, errors.New("change number must be positive")
	}
	rows, err := c.forge.ListCommentsTyped(c.repo, number, deskkit.TargetChange)
	if err != nil {
		return nil, operationError("list change comments", err)
	}
	out := make([]Comment, 0, len(rows))
	for _, row := range rows {
		out = append(out, Comment{row.Author.ID, row.Author.Type, row.DatabaseID, row.URL, row.CreatedAt, row.UpdatedAt, row.Body})
	}
	return out, nil
}

func operationError(operation string, err error) error {
	var api *deskkit.ForgeAPIError
	if errors.As(err, &api) {
		return fmt.Errorf("%s failed (HTTP %d)", operation, api.Status)
	}
	var transport *url.Error
	if errors.As(err, &transport) {
		return fmt.Errorf("%s failed: %w", operation, transport.Err)
	}
	return fmt.Errorf("%s could not be verified", operation)
}
