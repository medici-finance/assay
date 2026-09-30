package deskkit

// outboundforge.go — the checking Forge decorator: every text-carrying write method runs
// OutboundCheck (outbound.go) and delegates ONLY when it passes.
//
// ResolveForge — the one construction site of a backend (TestForgeSingleConstructionSite) —
// returns this decorator, and forgeban's backend-type rule forbids any cmd package from
// naming a backend type to build one or to type-assert back to one, so a verb holds a
// checked Forge or none. TestOutboundForgeWrapsEveryWriteMethod walks the Forge interface by
// reflection and fails when a method has no recorded classification here, so a write added
// to the interface later cannot slip past the check by being forgotten: the embedded inner
// Forge would otherwise delegate it silently.
//
// The per-method classification (the completeness test holds the same table):
//
//   text-checked  FileIssue, PostComment, PostCommentTyped, EditComment, CreateDraftChange,
//                 EditChange, PostReview, ApplyLabels, WriteFile.
//   no author text (pass straight through)  CloseIssue, CloseIssueTyped, ReopenIssue,
//                 MarkReadyForReview, SetMergeHold, DeleteRef — plus OpenMergeHold,
//                 RunWorkflow and ApproveGate, whose only text is composed by the backend
//                 from fixed strings or is a workflow input that never renders as prose.
//   reads         everything else.

import (
	"bytes"
	"strings"
)

// outboundForge wraps a backend. Embedding the inner Forge delegates every method this
// file does not override.
type outboundForge struct {
	Forge
	role string
}

// OutboundChecked returns inner wrapped by the outbound-write check, writing under role's
// custody. Wrapping an already-checked Forge returns it unchanged, so the check never runs
// twice for one write; a nil inner stays nil.
func OutboundChecked(inner Forge, role string) Forge {
	if inner == nil {
		return nil
	}
	if _, ok := inner.(*outboundForge); ok {
		return inner
	}
	return &outboundForge{Forge: inner, role: role}
}

// IsOutboundChecked reports whether f is the checking decorator.
func IsOutboundChecked(f Forge) bool {
	_, ok := f.(*outboundForge)
	return ok
}

// backendOf returns the backend behind the checking decorator (f itself when it is not
// wrapped). Package-internal: only deskkit may see a backend type.
func backendOf(f Forge) Forge {
	if w, ok := f.(*outboundForge); ok {
		return w.Forge
	}
	return f
}

// ForgeAccountFetcher returns the account-liveness fetcher and the roster identities for the
// backend behind f, or ok=false when f is served by a backend liveness has no implementation
// for. It exists so no cmd package has to type-assert a Forge back to a backend type (the
// forgeban rule): the unwrap happens here, inside the package that owns the backends.
func ForgeAccountFetcher(f Forge, cfg Config) (fetcher AccountFetcher, identities []RosterIdentity, ok bool) {
	switch b := backendOf(f).(type) {
	case *GitHubForge:
		return &HTTPAccountFetcher{Token: b.Token, BaseURL: b.BaseURL, Client: b.Client}, RosterIdentities(cfg), true
	case *GitLabForge:
		return &HTTPGitLabAccountFetcher{Token: b.Token, BaseURL: b.BaseURL, Client: b.Client}, GitLabRosterIdentities(cfg), true
	}
	return nil, nil, false
}

func (o *outboundForge) check(repo ForgeRepo, kind string, fields ...OutboundField) error {
	return OutboundCheck(OutboundWrite{Role: o.role, Repo: repo.Slug(), Kind: kind, Fields: fields})
}

func (o *outboundForge) FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error) {
	if err := o.check(repo, OutboundKindIssue,
		OutboundField{"title", in.Title}, OutboundField{"body", in.Body}); err != nil {
		return nil, err
	}
	return o.Forge.FileIssue(repo, in)
}

func (o *outboundForge) PostComment(repo ForgeRepo, number int, body string) (*CommentRef, error) {
	if err := o.check(repo, OutboundKindComment, OutboundField{"body", body}); err != nil {
		return nil, err
	}
	return o.Forge.PostComment(repo, number, body)
}

func (o *outboundForge) PostCommentTyped(repo ForgeRepo, number int, kind TargetKind, body string) (*CommentRef, error) {
	if err := o.check(repo, OutboundKindComment, OutboundField{"body", body}); err != nil {
		return nil, err
	}
	return o.Forge.PostCommentTyped(repo, number, kind, body)
}

func (o *outboundForge) EditComment(repo ForgeRepo, commentID, body string) error {
	if err := o.check(repo, OutboundKindComment, OutboundField{"body", body}); err != nil {
		return err
	}
	return o.Forge.EditComment(repo, commentID, body)
}

func (o *outboundForge) CreateDraftChange(repo ForgeRepo, in DraftChangeInput) (*PullRef, error) {
	if err := o.check(repo, OutboundKindChange,
		OutboundField{"title", in.Title}, OutboundField{"body", in.Body},
		OutboundField{"head", in.Head}); err != nil {
		return nil, err
	}
	return o.Forge.CreateDraftChange(repo, in)
}

func (o *outboundForge) EditChange(repo ForgeRepo, number int, in EditChangeInput) error {
	if err := o.check(repo, OutboundKindChange,
		OutboundField{"title", in.Title}, OutboundField{"body", in.Body}); err != nil {
		return err
	}
	return o.Forge.EditChange(repo, number, in)
}

func (o *outboundForge) PostReview(repo ForgeRepo, number int, in ReviewInput) error {
	if err := o.check(repo, OutboundKindReview, OutboundField{"body", in.Body}); err != nil {
		return err
	}
	return o.Forge.PostReview(repo, number, in)
}

// ApplyLabels checks every label ADDED — its name and its description. A removal publishes
// no new text.
func (o *outboundForge) ApplyLabels(repo ForgeRepo, number int, change LabelChange) (*LabelOutcome, error) {
	var fields []OutboundField
	for _, l := range change.Add {
		fields = append(fields, OutboundField{"name", l.Name})
		if l.Description != "" {
			fields = append(fields, OutboundField{"description", l.Description})
		}
	}
	if err := o.check(repo, OutboundKindLabel, fields...); err != nil {
		return nil, err
	}
	return o.Forge.ApplyLabels(repo, number, change)
}

// WriteFile checks the file's path, the commit message, and the lines the write ADDS: the
// new content's lines that were not in the file at the branch it lands on (or at
// StartBranch when that branch does not exist yet). A removed line cannot introduce a
// disclosure, and re-refusing lines already on the branch would strand every append to a
// file that predates the check. When the current content cannot be read the WHOLE new
// content is checked — could-not-read never narrows the scan.
func (o *outboundForge) WriteFile(repo ForgeRepo, in WriteFileInput) (*WriteFileResult, error) {
	added := string(in.Content)
	if prior, ok := o.priorContent(repo, in); ok {
		added = addedLinesAgainst(prior, in.Content)
	}
	if err := o.check(repo, OutboundKindFile,
		OutboundField{"path", in.File}, OutboundField{in.File, added},
		OutboundField{"commit", in.Message}); err != nil {
		return nil, err
	}
	return o.Forge.WriteFile(repo, in)
}

// priorContent reads the file as it stands where the write lands. ok=false means "check
// everything" (read error, or no ref to read).
func (o *outboundForge) priorContent(repo ForgeRepo, in WriteFileInput) ([]byte, bool) {
	for _, ref := range []string{in.Branch, in.StartBranch} {
		if ref == "" {
			continue
		}
		fc, err := o.Forge.ReadFile(repo, ReadFileInput{File: in.File, Ref: ref})
		if err != nil {
			if IsForgeNotFound(err) {
				continue
			}
			return nil, false
		}
		if fc == nil {
			return nil, false
		}
		if !fc.Exists {
			continue
		}
		return fc.Content, true
	}
	// No ref holds the file: every line is new.
	return nil, true
}

// addedLinesAgainst returns next with every line that ALSO occurs in prior blanked out,
// keeping line numbers aligned with next so a refusal's :line points into the new file.
// Membership is a SET, not a count: a line the file already publishes discloses nothing new
// wherever else in the file it is repeated (an Evidence row re-quoting the Verify-row command
// it proves is the everyday case; deskevidence's own pre-flight reads the same way).
func addedLinesAgainst(prior, next []byte) string {
	have := map[string]bool{}
	for _, l := range bytes.Split(prior, []byte("\n")) {
		have[string(l)] = true
	}
	lines := strings.Split(string(next), "\n")
	for i, l := range lines {
		if have[l] {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}
