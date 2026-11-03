package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"strings"

	"gopkg.in/yaml.v3"

	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// GitHubFiler is the App-identity WRITE path for desk__escalate / report_file.
//
// It is deliberately a separate type from adapter.Client (the read surface),
// which is built on a transport that STRUCTURALLY refuses every write. The
// agent write path is this brief (the service contract); it does not relax the read
// surface's guarantee — it stands beside it with its own client, so "the read
// surface cannot write" stays literally true.
//
// The token this holds is the service App's ([AppIdentity]); every issue and
// comment it creates is authored by that App, which is what makes the
// authority model's claim ("mutations author as the App, never a human") true
// on the wire and not only in the registry.
//
// OpenEscalations is backed by the derived INDEX, not a fresh GitHub read: the
// filing-time collapse folds against the console's own view of what is open,
// which is the same set the inbox serves. GitHub stays the record; the index
// is how the tool knows a matching escalation is already open.
type GitHubFiler struct {
	http    *http.Client
	apiBase string
	token   string
	db      *index.DB

	// recordToken and recordApp are the SEPARATE identity the cell-override
	// record PR is filed under (the service contract): the write-issues App, NOT
	// [AppIdentity] and NEVER a person's token. They are empty until
	// [GitHubFiler.SetRecordApp] wires them; empty means the record PR is
	// could-not-check (the filer degrades honestly, never falling back).
	recordToken string
	recordApp   string

	// filed remembers which (cell, table-hash) record PRs this process already
	// opened, so refiling the same table is idempotent — a POST that changes
	// nothing files no second PR (the PR-storm the brief's row 6 guards against).
	mu    sync.Mutex
	filed map[string]FiledRef
}

// NewGitHubFiler builds the write path. token is the App installation token
// for [AppIdentity]; db is the derived index the collapse lookup reads.
func NewGitHubFiler(apiBase, token string, db *index.DB) *GitHubFiler {
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	return &GitHubFiler{
		http:    &http.Client{Timeout: 30 * time.Second},
		apiBase: strings.TrimSuffix(apiBase, "/"),
		token:   token,
		db:      db,
		filed:   map[string]FiledRef{},
	}
}

// SetRecordApp wires the write-issues App identity the cell-override record PR
// files under (the service contract). It is DELIBERATELY separate from the desk
// App token the escalation writes use: the record PR is a config-repo write by
// a dedicated cross-repo issue-writing identity, and if that App is not
// configured the filer stays could-not-check rather than borrowing the read App
// or a person's token. token is the write-issues App installation token; app is
// its bot login for reporting.
func (g *GitHubFiler) SetRecordApp(token, app string) {
	g.recordToken = token
	g.recordApp = app
}

// RecordIdentity is the login the record PR is filed as, or "" when the
// write-issues App is not configured (the could-not-check state).
func (g *GitHubFiler) RecordIdentity() string {
	if g.recordToken == "" {
		return ""
	}
	return g.recordApp
}

// RecordPRRequest is one cell-override mirror-to-git filing.
type RecordPRRequest struct {
	Cell       string           // the cell whose overrides changed
	Repo       string           // owner/name of the cell's config repo (cells.yaml lives here)
	ConfigPath string           // path to cells.yaml within Repo
	Overrides  domain.Overrides // the current table to mirror into the cell's overrides: block
}

// RecordPRResult reports a record PR filing's outcome.
type RecordPRResult struct {
	// Status is a human/wire line: a filed/idempotent note, or an honest
	// could-not-check when the write-issues App is not configured.
	Status string `json:"status"`
	// Ref is the filed (or already-open) PR, nil when nothing was filed.
	Ref *FiledRef `json:"ref,omitempty"`
	// Idempotent is true when a PR for this (cell, table hash) already existed
	// and no new PR was opened.
	Idempotent bool `json:"idempotent,omitempty"`
}

// RecordFiler is the record-PR write path the override POST needs. GitHubFiler
// satisfies it; a test supplies a fake with no network. It is a SEPARATE
// interface from [Filer] (the escalation write path) so the escalation surface
// and its authority test are untouched by this addition.
type RecordFiler interface {
	// RecordIdentity is the write-issues App login, or "" when unconfigured.
	RecordIdentity() string
	// FileRecordPR mirrors a cell's override table into its cells.yaml
	// overrides: block via a draft PR under the write-issues App, idempotent per
	// (cell, table hash). When the App is unconfigured it returns a
	// could-not-check result and files NOTHING — never a fallback identity.
	FileRecordPR(ctx context.Context, req RecordPRRequest) (RecordPRResult, error)
}

// recordPRKey is the idempotency key: same cell + same table content ⇒ same key
// ⇒ at most one PR.
func recordPRKey(cell string, ov domain.Overrides) string { return cell + "\x00" + ov.Hash() }

// FileRecordPR mirrors a cell's override table to its cells.yaml via a draft PR
// authored by the write-issues App (the service contract). It is idempotent per
// (cell, table hash): a repeated filing of the same table returns the existing
// PR and opens no second one. When the write-issues App is not configured it
// returns could-not-check and touches no forge — the honest degrade the brief
// requires, with NO fallback to the read App or a human credential.
func (g *GitHubFiler) FileRecordPR(ctx context.Context, req RecordPRRequest) (RecordPRResult, error) {
	if g.recordToken == "" {
		return RecordPRResult{Status: "could-not-check (write-issues App not configured)"}, nil
	}
	key := recordPRKey(req.Cell, req.Overrides)
	g.mu.Lock()
	if ref, ok := g.filed[key]; ok {
		g.mu.Unlock()
		r := ref
		return RecordPRResult{Status: "record PR already open for this table (idempotent per table hash)", Ref: &r, Idempotent: true}, nil
	}
	g.mu.Unlock()

	owner, name, ok := strings.Cut(req.Repo, "/")
	if !ok {
		return RecordPRResult{}, fmt.Errorf("record PR: repo %q is not owner/name", req.Repo)
	}
	branch := fmt.Sprintf("overrides/%s/%d", req.Cell, time.Now().UTC().Unix())

	// 1. Resolve the base branch and its head sha.
	baseBranch, err := g.defaultBranch(ctx, owner, name)
	if err != nil {
		return RecordPRResult{}, fmt.Errorf("record PR: resolving base branch: %w", err)
	}
	baseSHA, err := g.refSHA(ctx, owner, name, "heads/"+baseBranch)
	if err != nil {
		return RecordPRResult{}, fmt.Errorf("record PR: resolving base sha: %w", err)
	}

	// 2. Create the record branch off the base head.
	if err := g.createRef(ctx, owner, name, "refs/heads/"+branch, baseSHA); err != nil {
		return RecordPRResult{}, fmt.Errorf("record PR: creating branch %q: %w", branch, err)
	}

	// 3. Read cells.yaml on the base, edit the cell's overrides: block, commit
	//    it onto the record branch.
	content, fileSHA, err := g.fileContent(ctx, owner, name, req.ConfigPath, baseBranch)
	if err != nil {
		return RecordPRResult{}, fmt.Errorf("record PR: reading %s: %w", req.ConfigPath, err)
	}
	edited, err := setCellOverrides(content, req.Cell, req.Overrides)
	if err != nil {
		return RecordPRResult{}, fmt.Errorf("record PR: editing overrides block: %w", err)
	}
	commitMsg := fmt.Sprintf("overrides(%s): %s (record PR)", req.Cell, strings.Join(domain.SortedStrings(req.Overrides.SetKeys()), ", "))
	if err := g.putFile(ctx, owner, name, req.ConfigPath, branch, edited, fileSHA, commitMsg); err != nil {
		return RecordPRResult{}, fmt.Errorf("record PR: committing %s: %w", req.ConfigPath, err)
	}

	// 4. Open the DRAFT PR.
	ref, err := g.createPR(ctx, owner, name, commitMsg, branch, baseBranch,
		fmt.Sprintf("Record PR mirroring cell `%s`'s operator overrides to `%s` (the service contract). "+
			"The deskd-local override table is authoritative until this merges; on the next config reload deskd adopts this block.", req.Cell, req.ConfigPath))
	if err != nil {
		return RecordPRResult{}, fmt.Errorf("record PR: opening draft PR: %w", err)
	}

	g.mu.Lock()
	g.filed[key] = ref
	g.mu.Unlock()
	r := ref
	return RecordPRResult{Status: "record PR filed (draft) under the write-issues App", Ref: &r}, nil
}

// setCellOverrides re-serialises cells.yaml with the named cell's overrides:
// block set to ov, editing the yaml NODE TREE so every other cell, key, comment
// and ordering in the file is preserved (a full struct round-trip would drop
// comments and emit zero fields). It is defined here beside the filer rather
// than in config/schema to keep schema free of a mutation path.
func setCellOverrides(content []byte, cell string, ov domain.Overrides) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("cells.yaml did not parse: %w", err)
	}
	if len(root.Content) == 0 {
		return nil, fmt.Errorf("cells.yaml is empty")
	}
	doc := root.Content[0] // the document's mapping node
	cellsSeq := mappingValue(doc, "cells")
	if cellsSeq == nil || cellsSeq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("cells.yaml has no cells: sequence")
	}
	for _, cellNode := range cellsSeq.Content {
		nameNode := mappingValue(cellNode, "name")
		if nameNode == nil || nameNode.Value != cell {
			continue
		}
		var ovNode yaml.Node
		if err := ovNode.Encode(ov); err != nil {
			return nil, fmt.Errorf("encoding overrides node: %w", err)
		}
		setMappingValue(cellNode, "overrides", &ovNode)
		out, err := yaml.Marshal(&root)
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	return nil, fmt.Errorf("cell %q not found in cells.yaml", cell)
}

// mappingValue returns the value node for key in a mapping node, or nil.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// setMappingValue sets key's value node in a mapping, replacing it in place when
// present or appending the key/value pair when absent.
func setMappingValue(m *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = value
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		value)
}

// ── the thin GitHub REST helpers the record PR needs ────────────────────────

func (g *GitHubFiler) defaultBranch(ctx context.Context, owner, name string) (string, error) {
	raw, err := g.get(ctx, fmt.Sprintf("%s/repos/%s/%s", g.apiBase, owner, name))
	if err != nil {
		return "", err
	}
	var resp struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	if resp.DefaultBranch == "" {
		return "main", nil
	}
	return resp.DefaultBranch, nil
}

func (g *GitHubFiler) refSHA(ctx context.Context, owner, name, ref string) (string, error) {
	raw, err := g.get(ctx, fmt.Sprintf("%s/repos/%s/%s/git/ref/%s", g.apiBase, owner, name, ref))
	if err != nil {
		return "", err
	}
	var resp struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	return resp.Object.SHA, nil
}

func (g *GitHubFiler) createRef(ctx context.Context, owner, name, ref, sha string) error {
	_, err := g.postRecord(ctx, fmt.Sprintf("%s/repos/%s/%s/git/refs", g.apiBase, owner, name),
		map[string]any{"ref": ref, "sha": sha})
	return err
}

func (g *GitHubFiler) fileContent(ctx context.Context, owner, name, path, ref string) (content []byte, sha string, err error) {
	raw, err := g.get(ctx, fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", g.apiBase, owner, name, path, ref))
	if err != nil {
		return nil, "", err
	}
	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		SHA      string `json:"sha"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", err
	}
	decoded, derr := base64.StdEncoding.DecodeString(strings.ReplaceAll(resp.Content, "\n", ""))
	if derr != nil {
		return nil, "", fmt.Errorf("contents payload did not base64-decode: %w", derr)
	}
	return decoded, resp.SHA, nil
}

func (g *GitHubFiler) putFile(ctx context.Context, owner, name, path, branch string, content []byte, sha, msg string) error {
	_, err := g.putRecord(ctx, fmt.Sprintf("%s/repos/%s/%s/contents/%s", g.apiBase, owner, name, path),
		map[string]any{
			"message": msg,
			"content": base64.StdEncoding.EncodeToString(content),
			"branch":  branch,
			"sha":     sha,
		})
	return err
}

func (g *GitHubFiler) createPR(ctx context.Context, owner, name, title, head, base, body string) (FiledRef, error) {
	raw, err := g.postRecord(ctx, fmt.Sprintf("%s/repos/%s/%s/pulls", g.apiBase, owner, name),
		map[string]any{"title": title, "head": head, "base": base, "body": body, "draft": true})
	if err != nil {
		return FiledRef{}, err
	}
	var resp struct {
		Number int    `json:"number"`
		URL    string `json:"html_url"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return FiledRef{}, err
	}
	return FiledRef{Repo: owner + "/" + name, Number: resp.Number, URL: resp.URL}, nil
}

// get / postRecord / putRecord are the record-PR transport, authenticating with
// the WRITE-ISSUES token (recordToken), never the desk App token.
func (g *GitHubFiler) get(ctx context.Context, url string) ([]byte, error) {
	return g.doRecord(ctx, http.MethodGet, url, nil)
}
func (g *GitHubFiler) postRecord(ctx context.Context, url string, payload any) ([]byte, error) {
	return g.doRecord(ctx, http.MethodPost, url, payload)
}
func (g *GitHubFiler) putRecord(ctx context.Context, url string, payload any) ([]byte, error) {
	return g.doRecord(ctx, http.MethodPut, url, payload)
}

func (g *GitHubFiler) doRecord(ctx context.Context, method, url string, payload any) ([]byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// The record PR authenticates as the WRITE-ISSUES App, never the desk App.
	req.Header.Set("Authorization", "Bearer "+g.recordToken)
	resp, err := g.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(raw))
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return nil, fmt.Errorf("%s %s → HTTP %d: %s", method, url, resp.StatusCode, snippet)
	}
	return raw, nil
}

// compile-time assertion: GitHubFiler is a RecordFiler.
var _ RecordFiler = (*GitHubFiler)(nil)

// Identity is the App this filer writes as.
func (g *GitHubFiler) Identity() string { return AppIdentity }

// OpenEscalations reads the cell's open escalations from the derived index and
// projects each into a collapse candidate carrying the key and guard the index
// already derived.
func (g *GitHubFiler) OpenEscalations(_ context.Context, cell string) ([]OpenEscalation, error) {
	if g.db == nil {
		return nil, fmt.Errorf("no index to read open escalations from")
	}
	rows, err := g.db.Inbox(cell, false)
	if err != nil {
		return nil, err
	}
	var out []OpenEscalation
	for _, r := range rows {
		ref := parsePlatformRef(r)
		out = append(out, OpenEscalation{
			Ref:         ref,
			CollapseKey: r.Key().String(),
			MergeGuard:  r.MergeGuard(),
			Repeat:      r.Repeat,
		})
	}
	return out, nil
}

// FileEscalation opens a new issue via the REST API, authored by the App.
func (g *GitHubFiler) FileEscalation(ctx context.Context, e NewEscalation) (FiledRef, error) {
	owner, name, ok := strings.Cut(e.Repo, "/")
	if !ok {
		return FiledRef{}, fmt.Errorf("repo %q is not owner/name", e.Repo)
	}
	payload := map[string]any{"title": e.Title, "body": e.Body, "labels": e.Labels}
	raw, err := g.post(ctx, fmt.Sprintf("%s/repos/%s/%s/issues", g.apiBase, owner, name), payload)
	if err != nil {
		return FiledRef{}, err
	}
	var resp struct {
		Number int    `json:"number"`
		URL    string `json:"html_url"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return FiledRef{}, fmt.Errorf("issue-create response did not decode: %w", err)
	}
	return FiledRef{Repo: e.Repo, Number: resp.Number, URL: resp.URL}, nil
}

// CommentReRaise records a repeat on an open escalation (spec §1.1.3).
func (g *GitHubFiler) CommentReRaise(ctx context.Context, ref FiledRef, note string) error {
	owner, name, ok := strings.Cut(ref.Repo, "/")
	if !ok {
		return fmt.Errorf("repo %q is not owner/name", ref.Repo)
	}
	_, err := g.post(ctx,
		fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", g.apiBase, owner, name, ref.Number),
		map[string]any{"body": note})
	return err
}

func (g *GitHubFiler) post(ctx context.Context, url string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(raw))
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return nil, fmt.Errorf("POST %s → HTTP %d: %s", url, resp.StatusCode, snippet)
	}
	return raw, nil
}

// parsePlatformRef recovers the owner/name#number ref from a served row's
// platform deep-link. The link is presentation data attached at the serving
// edge (spec §1.5); the row carries no platform number in the domain, so the
// filer reads it back from the link it will comment against.
func parsePlatformRef(r index.Row) FiledRef {
	url := r.Links["platform"]
	ref := FiledRef{URL: url}
	// https://github.com/<owner>/<name>/issues/<number>
	const marker = "/issues/"
	if i := strings.LastIndex(url, marker); i >= 0 {
		numPart := url[i+len(marker):]
		if j := strings.IndexAny(numPart, "/?#"); j >= 0 {
			numPart = numPart[:j]
		}
		fmt.Sscanf(numPart, "%d", &ref.Number)
		// owner/name are the two path segments before /issues/
		segs := strings.Split(strings.TrimPrefix(url[:i], "https://"), "/")
		if len(segs) >= 3 {
			ref.Repo = segs[len(segs)-2] + "/" + segs[len(segs)-1]
		}
	}
	return ref
}

// compile-time assertion: GitHubFiler is a Filer.
var _ Filer = (*GitHubFiler)(nil)
