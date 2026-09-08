package main

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// ghClient reads GitHub through the `gh` CLI, inheriting the caller's auth.
// Every failure surfaces as an error, so the caller reports `unreachable`
// rather than assuming a pass.
type ghClient struct {
	defaultBranches map[string]string
}

func newGHClient() *ghClient { return &ghClient{defaultBranches: map[string]string{}} }

func (g *ghClient) DefaultBranch(repo string) (string, error) {
	if b, ok := g.defaultBranches[repo]; ok {
		return b, nil
	}
	out, err := g.api("repos/"+repo, "-q", ".default_branch")
	if err != nil {
		return "", err
	}
	b := strings.TrimSpace(out)
	if b == "" {
		return "", fmt.Errorf("empty default branch for %s", repo)
	}
	g.defaultBranches[repo] = b
	return b, nil
}

func (g *ghClient) BlobSHA(repo, path, ref string) (string, error) {
	endpoint := fmt.Sprintf("repos/%s/contents/%s?ref=%s", repo, escapePath(path), url.QueryEscape(ref))
	out, err := g.api(endpoint, "-q", ".sha")
	if err != nil {
		return "", err
	}
	sha := strings.TrimSpace(out)
	if sha == "" {
		// A directory (or anything that is not a single file blob) yields no
		// scalar .sha. Treat as not-found rather than in-sync.
		return "", fmt.Errorf("%w: %s@%s has no blob sha", ErrNotFound, path, ref)
	}
	return sha, nil
}

func (g *ghClient) TagCommit(repo, tag string) (string, error) {
	endpoint := fmt.Sprintf("repos/%s/git/ref/tags/%s", repo, escapePath(tag))
	out, err := g.api(endpoint, "-q", ".object.sha + \" \" + .object.type")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return "", fmt.Errorf("%w: tag %s has no object", ErrNotFound, tag)
	}
	sha, typ := fields[0], fields[1]
	// An annotated tag's ref points at a tag object; peel it to the commit.
	if typ == "tag" {
		out, err = g.api(fmt.Sprintf("repos/%s/git/tags/%s", repo, sha), "-q", ".object.sha")
		if err != nil {
			return "", err
		}
		sha = strings.TrimSpace(out)
	}
	if sha == "" {
		return "", fmt.Errorf("%w: tag %s peels to no commit", ErrNotFound, tag)
	}
	return sha, nil
}

func (g *ghClient) CommitDate(repo, commit string) (string, error) {
	endpoint := fmt.Sprintf("repos/%s/commits/%s", repo, url.PathEscape(commit))
	out, err := g.api(endpoint, "-q", ".commit.committer.date")
	if err != nil {
		return "", err
	}
	date := strings.TrimSpace(out)
	if date == "" {
		return "", fmt.Errorf("%w: %s has no committer date", ErrNotFound, commit)
	}
	return date, nil
}

func (g *ghClient) CommitsSince(repo, path, ref, since string, maxPages int) (int, bool, error) {
	count := 0
	for page := 1; page <= maxPages; page++ {
		endpoint := fmt.Sprintf("repos/%s/commits?path=%s&sha=%s&since=%s&per_page=100&page=%d",
			repo, url.QueryEscape(path), url.QueryEscape(ref), url.QueryEscape(since), page)
		out, err := g.api(endpoint, "-q", ".[].sha")
		if err != nil {
			return count, false, err
		}
		n := len(splitNonEmpty(out))
		count += n
		if n < 100 {
			return count, false, nil
		}
	}
	// Ran out of pages with a full last page: the count is a lower bound.
	return count, true, nil
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// escapePath escapes each path segment, keeping the slashes as separators.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

func (g *ghClient) api(endpoint string, extra ...string) (string, error) {
	args := append([]string{"api", endpoint}, extra...)
	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "HTTP 404") || strings.Contains(msg, "Not Found") {
			return "", fmt.Errorf("%w: %s", ErrNotFound, firstLine(msg))
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("gh api %s: %s", endpoint, firstLine(msg))
	}
	return stdout.String(), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
