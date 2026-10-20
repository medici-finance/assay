package deskkit

import (
	"crypto/sha1" // Git's object format, not an authentication primitive.
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// The file is selected UNDER the returned branch commit, rather than through a
// second branch-expression lookup. Its bytes and blob revision therefore describe the same immutable tree.
// Schema: https://docs.github.com/en/graphql/reference/commits
const githubEvidenceSnapshotQuery = `query EvidenceLandingSnapshot($owner:String!,$name:String!,$number:Int!,$branch:String!,$file:String!){
 repository(owner:$owner,name:$name){
  visibility
  ref(qualifiedName:$branch){target{__typename oid ... on Commit{
   file(path:$file){mode object{__typename oid ... on Blob{byteSize isBinary text}}}
  }}}
  issue(number:$number){
   number title body state url author{login __typename ... on User{databaseId} ... on Bot{databaseId}}
   lastEditedAt
   comments(first:100){pageInfo{hasNextPage} nodes{createdAt lastEditedAt author{login __typename ... on User{databaseId} ... on Bot{databaseId}}}}
   labels(first:100){pageInfo{hasNextPage} nodes{name}}
   timelineItems(first:100,itemTypes:[LABELED_EVENT,UNLABELED_EVENT]){
    pageInfo{hasNextPage} nodes{__typename
     ... on LabeledEvent{createdAt actor{login __typename} label{name}}
     ... on UnlabeledEvent{createdAt actor{login __typename} label{name}}
    }
   }
  }
 }
}`

type githubEvidenceSnapshot struct {
	issue      *Issue
	trust      TrustPayload
	events     []LabelEvent
	moreEvents bool
	visibility string
	file       *FileContent
}

type githubEvidenceConnection[T any] struct {
	PageInfo *struct{ HasNextPage *bool } `json:"pageInfo"`
	Nodes    []T                          `json:"nodes"`
}

type githubEvidenceIssue struct {
	Number                  int
	Title, Body, State, URL string
	Author                  *gqlActor
	Comments                *githubEvidenceConnection[gqlComment]
	Labels                  *githubEvidenceConnection[struct{ Name string }]
	TimelineItems           *githubEvidenceConnection[struct {
		Typename  string `json:"__typename"`
		CreatedAt string
		Actor     *gqlActor
		Label     *struct{ Name string }
	}]
}

func (g *GitHubForge) evidenceSnapshot(repo ForgeRepo, number int, file, branch string) (*githubEvidenceSnapshot, error) {
	var env struct {
		Data struct {
			Repository *struct {
				Visibility string
				Issue      json.RawMessage
				Ref        *struct {
					Target *struct {
						Typename string `json:"__typename"`
						OID      string
						File     json.RawMessage
					}
				}
			}
		}
		Errors []struct {
			Type string
			Path []any
		}
	}
	in := map[string]any{"query": githubEvidenceSnapshotQuery, "variables": map[string]any{
		"owner": repo.Owner, "name": repo.Name, "number": number,
		"branch": "refs/heads/" + strings.TrimPrefix(branch, "refs/heads/"), "file": file,
	}}
	if err := g.doJSON(http.MethodPost, "/graphql", in, &env); err != nil {
		return nil, err
	}
	bad := func(reason string) (*githubEvidenceSnapshot, error) {
		return nil, Unverifiable("cannot read evidence publication snapshot: "+reason, nil)
	}
	if len(env.Errors) > 0 {
		// Commit.file reports an absent entry as null plus NOT_FOUND. Only
		// that field's negative lookup may accompany an otherwise complete
		// snapshot; arbitrary or mixed errors still make it unreadable.
		// Never decode or print the forge's free-form error message.
		if len(env.Errors) != 1 || env.Errors[0].Type != "NOT_FOUND" {
			return bad("GraphQL returned errors")
		}
		p := env.Errors[0].Path
		if len(p) != 4 || p[0] != "repository" || p[1] != "ref" || p[2] != "target" || p[3] != "file" {
			return bad("GraphQL NOT_FOUND outside destination file")
		}
	}
	r := env.Data.Repository
	if r == nil || r.Ref == nil || r.Ref.Target == nil || r.Ref.Target.Typename != "Commit" || len(r.Ref.Target.OID) != 40 {
		return bad("missing repository or branch commit")
	}
	if r.Visibility != "PUBLIC" && r.Visibility != "PRIVATE" && r.Visibility != "INTERNAL" {
		return bad("missing or unknown live repository visibility")
	}
	var wi githubEvidenceIssue
	var ti gqlItem
	if err := json.Unmarshal(r.Issue, &wi); err != nil {
		return bad("missing or malformed attestation issue")
	}
	if err := json.Unmarshal(r.Issue, &ti); err != nil {
		return bad("malformed attestation history")
	}
	if wi.Number != number || wi.Labels == nil || wi.Labels.PageInfo == nil || wi.Labels.PageInfo.HasNextPage == nil || wi.Labels.Nodes == nil || wi.TimelineItems == nil || wi.TimelineItems.PageInfo == nil || wi.TimelineItems.PageInfo.HasNextPage == nil || wi.TimelineItems.Nodes == nil || wi.Comments == nil || wi.Comments.PageInfo == nil || wi.Comments.PageInfo.HasNextPage == nil || wi.Comments.Nodes == nil || !ti.bodyHistoryKnown {
		return bad("incomplete attestation issue, labels or history")
	}
	// Same trust reducer as IssueTrustEvents, including edited-body and
	// incomplete-comment-history refusals in the admission consumer.
	var te gqlEnvelope
	te.Data.Repository.Issue = &ti
	trust, err := trustFromEnvelope(te, false)
	if err != nil {
		return bad("malformed trust history")
	}
	s := &githubEvidenceSnapshot{trust: trust,
		visibility: strings.ToLower(r.Visibility), moreEvents: *wi.TimelineItems.PageInfo.HasNextPage,
		issue: &Issue{Number: wi.Number, Title: wi.Title, Body: wi.Body, State: strings.ToLower(wi.State), URL: wi.URL, Author: gqlActorAccount(wi.Author)}}
	for _, l := range wi.Labels.Nodes {
		s.issue.Labels = append(s.issue.Labels, l.Name)
	}
	if *wi.Labels.PageInfo.HasNextPage {
		// A large current label set needs the existing unbounded issue read.
		issue, err := g.GetIssueTyped(repo, number, TargetIssue)
		if err != nil {
			return nil, err
		}
		s.issue.Labels = issue.Labels
	}
	for _, ev := range wi.TimelineItems.Nodes {
		if ev.Typename != "LabeledEvent" && ev.Typename != "UnlabeledEvent" {
			return bad("unrecognised label event")
		}
		if ev.Label == nil || ev.Label.Name == "" {
			return bad("unreadable label event")
		}
		s.events = append(s.events, LabelEvent{Name: ev.Label.Name, AppliedBy: gqlActorAccount(ev.Actor).Login, CreatedAt: ev.CreatedAt, Removed: ev.Typename == "UnlabeledEvent"})
	}
	rawFile := r.Ref.Target.File
	if len(rawFile) == 0 {
		return bad("missing destination lookup")
	}
	if len(env.Errors) > 0 && string(rawFile) != "null" {
		return bad("GraphQL NOT_FOUND without an absent destination")
	}
	if string(rawFile) != "null" {
		var f struct {
			Mode   int
			Object *struct {
				Typename string `json:"__typename"`
				OID      string
				ByteSize *int
				IsBinary *bool
				Text     *string
			}
		}
		if err := json.Unmarshal(rawFile, &f); err != nil {
			return bad("malformed destination lookup")
		}
		b := f.Object
		// Evidence must be a regular text blob. Refuse symlinks, binaries and
		// partial/converted text instead of mistaking them for absent files.
		if f.Mode != 0100644 || b == nil || b.Typename != "Blob" || b.IsBinary == nil || *b.IsBinary || b.Text == nil || b.ByteSize == nil || *b.ByteSize != len(*b.Text) {
			return bad("destination is not a complete regular text blob")
		}
		content := []byte(*b.Text)
		h := sha1.New()
		fmt.Fprintf(h, "blob %d\x00", len(content))
		h.Write(content)
		if fmt.Sprintf("%x", h.Sum(nil)) != b.OID {
			return bad("destination bytes do not match the returned Git blob")
		}
		// The checks above prove a regular-mode, complete text blob whose bytes hash to its
		// own object id, so the read is a plain file (S-2266-8's FileKindRegular).
		s.file = &FileContent{Content: content, SHA: b.OID, Exists: true, Kind: FileKindRegular}
	}
	return s, nil
}
