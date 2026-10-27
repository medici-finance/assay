package deskkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/medici-finance/assay/perfrec"
)

// IssueCommentBatchSize bounds lookahead, not a thread's history. Each thread
// keeps its original first:100 selection and forgeMaxEventPages limit. Five
// aliases keep a response to at most 500 selected comment nodes per page.
const IssueCommentBatchSize = 5

type issueCommentGetter interface {
	IssueContentEvents(ForgeRepo, int) (*TrustPayload, error)
}

type issueCommentBatchReader interface {
	issueCommentEvents(ForgeRepo, []int) (map[int]*TrustPayload, error)
}

// ReadIssueCommentEvents keeps the caller's wrapped reader. Unknown decorators
// and providers retain their original per-issue operation; a failed native
// batch never retries through a different operation or credential.
func ReadIssueCommentEvents(f issueCommentGetter, repo ForgeRepo, numbers []int) (map[int]*TrustPayload, error) {
	if len(numbers) == 0 || len(numbers) > IssueCommentBatchSize {
		return nil, Refused("issue comment batch is outside its bounded size")
	}
	seen := map[int]bool{}
	for _, n := range numbers {
		if n <= 0 || int64(n) > 1<<31-1 || seen[n] {
			return nil, Refused("issue comment batch has an invalid or repeated identity")
		}
		seen[n] = true
	}
	if batch, ok := f.(issueCommentBatchReader); ok && len(numbers) > 1 && CanBatchIssueComments(f) {
		return batch.issueCommentEvents(repo, numbers)
	}
	out := map[int]*TrustPayload{}
	for _, n := range numbers {
		payload, err := f.IssueContentEvents(repo, n)
		if err != nil {
			return nil, err
		}
		if payload == nil {
			return nil, Unverifiable("missing issue comment history", nil)
		}
		out[n] = payload
	}
	return out, nil
}

// CanBatchIssueComments inspects only the explicitly owned wrapper capability.
// It returns no backend/client/credential and probes no provider. An unknown
// decorator cannot inherit a batch method that skips its own read guard.
func CanBatchIssueComments(f issueCommentGetter) bool {
	switch v := f.(type) {
	case *GitHubForge:
		return true
	case *outboundForge:
		return CanBatchIssueComments(v.Forge)
	case *readOnlyForge:
		return CanBatchIssueComments(v.inner)
	default:
		return false
	}
}

func (o *outboundForge) issueCommentEvents(repo ForgeRepo, numbers []int) (map[int]*TrustPayload, error) {
	return ReadIssueCommentEvents(o.Forge, repo, numbers)
}

func (r *readOnlyForge) issueCommentEvents(repo ForgeRepo, numbers []int) (map[int]*TrustPayload, error) {
	return ReadIssueCommentEvents(r.inner, repo, numbers)
}

var issueCommentBatchQuery = func() string {
	var vars, fields strings.Builder
	for slot := 0; slot < IssueCommentBatchSize; slot++ {
		fmt.Fprintf(&vars, ",$n%d:Int!,$use%d:Boolean!,$after%d:String", slot, slot, slot)
		fmt.Fprintf(&fields, "a%d:issue(number:$n%d) @include(if:$use%d){number comments(first:100,after:$after%d){pageInfo{hasNextPage endCursor} nodes{createdAt author{login __typename ...on User{databaseId} ...on Bot{databaseId}}}}} ", slot, slot, slot, slot)
	}
	return `query($owner:String!,$name:String!` + vars.String() + `){repository(owner:$owner,name:$name){` + fields.String() + `}}`
}()

type issueCommentPage struct {
	Number   int `json:"number"`
	Comments *struct {
		PageInfo *struct {
			HasNextPage *bool  `json:"hasNextPage"`
			EndCursor   string `json:"endCursor"`
		} `json:"pageInfo"`
		Nodes *[]ghIssueCommentNode `json:"nodes"`
	} `json:"comments"`
}

type issueCommentObservation struct {
	events []ContentEvent
	more   bool
	cursor string
}

func (g *GitHubForge) issueCommentEvents(repo ForgeRepo, numbers []int) (map[int]*TrustPayload, error) {
	if !ValidRepoSlug(repo.Slug()) || g.onlyRepo != "" && !strings.EqualFold(g.onlyRepo, repo.Slug()) {
		return nil, Refused("issue comment history is outside the admitted repository")
	}
	out := map[int]*TrustPayload{}
	cursors := map[int]string{}
	for _, n := range numbers {
		out[n] = &TrustPayload{}
		cursors[n] = ""
	}
	for page := 1; page <= forgeMaxEventPages && len(cursors) > 0; page++ {
		vars := map[string]any{"owner": repo.Owner, "name": repo.Name}
		aliases := map[string]int{}
		for slot := 0; slot < IssueCommentBatchSize; slot++ {
			n, active := 0, false
			var after any
			if slot < len(numbers) {
				n = numbers[slot]
				cursor, pending := cursors[n]
				active = pending
				if cursor != "" {
					after = cursor
				}
			}
			vars[fmt.Sprintf("n%d", slot)] = n
			vars[fmt.Sprintf("use%d", slot)] = active
			vars[fmt.Sprintf("after%d", slot)] = after
			if active {
				aliases[fmt.Sprintf("a%d", slot)] = n
			}
		}
		resp, capture, err := g.requestJSON(http.MethodPost, "/graphql", map[string]any{"query": issueCommentBatchQuery, "variables": vars})
		if err != nil {
			return nil, err
		}
		pages, meta, wireFailed, decodeErr := readCommentBatch(resp.Body, aliases, repo)
		if capture != nil && capture.body != nil {
			points := perfrec.Points{Coverage: perfrec.CoverageFailed, Unit: perfrec.UnitGitHubPoints}
			if !wireFailed {
				env, _ := json.Marshal(map[string]any{"data": map[string]any{"_assayPerf": meta}})
				points = decodeRequestPoints(env, perfrec.UnitGitHubPoints)
			}
			attachRequestPoints(capture.body, points)
		}
		resp.Body.Close()
		if decodeErr != nil {
			return nil, Unverifiable("cannot read complete issue comment batch", decodeErr)
		}
		for _, n := range numbers {
			row, pending := pages[n]
			if !pending {
				continue
			}
			payload := out[n]
			payload.Events = append(payload.Events, row.events...)
			more := row.more
			cursor := row.cursor
			if !more {
				payload.Complete = true
			}
			if !more || cursor == "" {
				delete(cursors, n)
			} else {
				cursors[n] = cursor
			}
		}
	}
	return out, nil
}

// Decode the envelope incrementally and retain only one alias's wire page at a
// time. There is no new aggregate byte limit that could reject a set of valid
// legacy threads. At most five original bounded histories are retained; unknown
// fields are skipped without decoding the whole batch into a RawMessage.
func readCommentBatch(body io.Reader, aliases map[string]int, repo ForgeRepo) (map[int]*issueCommentObservation, json.RawMessage, bool, error) {
	dec := json.NewDecoder(body)
	rows := map[int]*issueCommentObservation{}
	var meta json.RawMessage
	var businessErr error
	dataSeen, repoSeen, errorsSeen, hasErrors, pointShapeFailed := false, false, false, false, false
	refuse := func(err error) {
		if businessErr == nil {
			businessErr = err
		}
	}
	// Shape/type refusals are still complete JSON observations. Keep consuming
	// them so cost after the refused alias remains the provider's real scalar.
	shape := func(err error, pointEnvelope bool) error {
		var bad commentShapeError
		if errors.As(err, &bad) {
			refuse(err)
			if pointEnvelope && !bad.null {
				pointShapeFailed = true
			}
			return nil
		}
		return err
	}
	err := commentObject(dec, func(key string) error {
		switch key {
		case "data":
			if dataSeen {
				refuse(fmt.Errorf("repeated comment data"))
			}
			dataSeen = true
			err := commentObject(dec, func(key string) error {
				if key == "_assayPerf" {
					return dec.Decode(&meta)
				}
				if key != "repository" {
					return skipCommentValue(dec)
				}
				if repoSeen {
					refuse(fmt.Errorf("repeated comment repository"))
					return skipCommentValue(dec)
				}
				repoSeen = true
				err := commentObject(dec, func(alias string) error {
					n, expected := aliases[alias]
					if !expected || rows[n] != nil {
						refuse(fmt.Errorf("unexpected or repeated comment alias"))
						return skipCommentValue(dec)
					}
					var row *issueCommentPage
					if err := dec.Decode(&row); err != nil {
						var bad *json.UnmarshalTypeError
						if errors.As(err, &bad) {
							refuse(err)
							return nil
						}
						return err
					}
					if row == nil || row.Number != n || row.Comments == nil || row.Comments.PageInfo == nil || row.Comments.PageInfo.HasNextPage == nil || row.Comments.Nodes == nil {
						refuse(fmt.Errorf("missing comment identity or page provenance for issue %d", n))
						return nil
					}
					events, err := appendIssueEvents(nil, *row.Comments.Nodes, repo, n)
					if err != nil {
						refuse(err)
						return nil
					}
					rows[n] = &issueCommentObservation{events: events, more: *row.Comments.PageInfo.HasNextPage, cursor: row.Comments.PageInfo.EndCursor}
					return nil
				})
				return shape(err, false)
			})
			return shape(err, true)
		case "errors":
			if errorsSeen {
				refuse(fmt.Errorf("repeated comment errors"))
			}
			errorsSeen = true
			token, err := dec.Token()
			if err != nil || token == nil {
				return err
			}
			if token != json.Delim('[') {
				refuse(fmt.Errorf("malformed comment errors"))
				return skipCommentOpened(dec, token)
			}
			for dec.More() {
				hasErrors = true
				if err := skipCommentValue(dec); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		default:
			return skipCommentValue(dec)
		}
	})
	err = shape(err, true)
	if err != nil {
		return nil, meta, true, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, meta, true, fmt.Errorf("comment response did not end cleanly: %v", err)
	}
	if !dataSeen || !repoSeen || hasErrors || len(rows) != len(aliases) {
		refuse(fmt.Errorf("comment observation is incomplete or carries GraphQL errors"))
	}
	if businessErr != nil {
		return nil, meta, pointShapeFailed, businessErr
	}
	return rows, meta, pointShapeFailed, nil
}

type commentShapeError struct{ null bool }

func (commentShapeError) Error() string { return "expected comment response object" }

func commentObject(dec *json.Decoder, field func(string) error) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	if token != json.Delim('{') {
		if err := skipCommentOpened(dec, token); err != nil {
			return err
		}
		return commentShapeError{null: token == nil}
	}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return fmt.Errorf("invalid comment response key")
		}
		if err := field(key); err != nil {
			return err
		}
	}
	_, err = dec.Token()
	return err
}

func skipCommentValue(dec *json.Decoder) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	return skipCommentOpened(dec, token)
}

func skipCommentOpened(dec *json.Decoder, token json.Token) error {
	var err error
	if token == json.Delim('{') {
		for dec.More() {
			if _, err := dec.Token(); err != nil {
				return err
			}
			if err := skipCommentValue(dec); err != nil {
				return err
			}
		}
		_, err = dec.Token()
	} else if token == json.Delim('[') {
		for dec.More() {
			if err := skipCommentValue(dec); err != nil {
				return err
			}
		}
		_, err = dec.Token()
	}
	return err
}
