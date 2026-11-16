package deskkit

import (
	"fmt"
	"net/http"
	"strings"
)

// ChangeCommentTimeBatchSize bounds read-ahead. Each change retains ListComments'
// first-100 comment window; this is not the walked issue-escalation history.
const ChangeCommentTimeBatchSize = 5

type ChangeCommentTimes struct {
	// Only Author and CreatedAt are selected; body/edit/minimization fields are unavailable.
	Comments []Comment
	Err      error
}

type changeCommentTimeReader interface {
	changeCommentTimes(ForgeRepo, []int) (map[int]ChangeCommentTimes, error)
}

// CanBatchChangeCommentTimes admits only owned decorators. An opaque wrapper must
// keep its own ListComments guard; an embedded optional method cannot bypass it.
func CanBatchChangeCommentTimes(f Forge) bool {
	switch v := f.(type) {
	case *GitHubForge:
		return true
	case *outboundForge:
		return CanBatchChangeCommentTimes(v.Forge)
	case *readOnlyForge:
		return CanBatchChangeCommentTimes(v.inner)
	default:
		return false
	}
}

// ReadChangeCommentTimes preserves per-change failures. A failed native batch is
// not retried under another operation or identity. No result survives the call.
func ReadChangeCommentTimes(f Forge, repo ForgeRepo, numbers []int) (map[int]ChangeCommentTimes, error) {
	if len(numbers) == 0 || len(numbers) > ChangeCommentTimeBatchSize || !ValidRepoSlug(repo.Slug()) {
		return nil, Refused("change comment-time batch is outside its bounded scope")
	}
	seen := map[int]bool{}
	for _, n := range numbers {
		if n <= 0 || int64(n) > 1<<31-1 || seen[n] {
			return nil, Refused("change comment-time batch has an invalid or repeated identity")
		}
		seen[n] = true
	}
	if CanBatchChangeCommentTimes(f) {
		return f.(changeCommentTimeReader).changeCommentTimes(repo, numbers)
	}
	out := map[int]ChangeCommentTimes{}
	for _, n := range numbers {
		comments, err := f.ListComments(repo, n)
		row := ChangeCommentTimes{Err: err, Comments: comments}
		out[n] = row
	}
	return out, nil
}

func (o *outboundForge) changeCommentTimes(repo ForgeRepo, numbers []int) (map[int]ChangeCommentTimes, error) {
	return ReadChangeCommentTimes(o.Forge, repo, numbers)
}
func (r *readOnlyForge) changeCommentTimes(repo ForgeRepo, numbers []int) (map[int]ChangeCommentTimes, error) {
	return ReadChangeCommentTimes(r.inner, repo, numbers)
}

var changeCommentTimeQuery = func() string {
	var vars, fields strings.Builder
	for i := 0; i < ChangeCommentTimeBatchSize; i++ {
		fmt.Fprintf(&vars, ",$n%d:Int!,$use%d:Boolean!", i, i)
		fmt.Fprintf(&fields, "a%d:pullRequest(number:$n%d) @include(if:$use%d){number comments(first:100){nodes{createdAt author{login __typename ...on User{databaseId} ...on Bot{databaseId}}}}} ", i, i, i)
	}
	return `query($owner:String!,$name:String!` + vars.String() + `){repository(owner:$owner,name:$name){` + fields.String() + `}}`
}()

type changeCommentTimeWire struct {
	Number   int `json:"number"`
	Comments *struct {
		Nodes *[]struct {
			CreatedAt string    `json:"createdAt"`
			Author    *gqlActor `json:"author"`
		} `json:"nodes"`
	} `json:"comments"`
}

func (g *GitHubForge) changeCommentTimes(repo ForgeRepo, numbers []int) (map[int]ChangeCommentTimes, error) {
	if g.onlyRepo != "" && !strings.EqualFold(g.onlyRepo, repo.Slug()) {
		return nil, Refused("change comment times are outside the admitted repository")
	}
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name}
	for i := 0; i < ChangeCommentTimeBatchSize; i++ {
		n := 0
		if i < len(numbers) {
			n = numbers[i]
		}
		vars[fmt.Sprintf("n%d", i)] = n
		vars[fmt.Sprintf("use%d", i)] = i < len(numbers)
	}
	var wire struct {
		Data struct {
			Repository map[string]*changeCommentTimeWire `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
			Path    []any  `json:"path"`
		} `json:"errors"`
	}
	if err := g.doJSON(http.MethodPost, "/graphql", map[string]any{"query": changeCommentTimeQuery, "variables": vars}, &wire); err != nil {
		return nil, err
	}
	out := map[int]ChangeCommentTimes{}
	aliases := map[string]bool{}
	for i := range numbers {
		aliases[fmt.Sprintf("a%d", i)] = true
	}
	for i, n := range numbers {
		alias := fmt.Sprintf("a%d", i)
		bad := false
		for _, err := range wire.Errors {
			// Root/unlocated failures cannot certify any row. Located failures affect
			// only that change, preserving the caller's row-local failure contract.
			if len(err.Path) < 2 || err.Path[0] != "repository" {
				bad = true
				break
			}
			if target, ok := err.Path[1].(string); !ok || !aliases[target] || target == alias {
				bad = true
				break
			}
		}
		row := wire.Data.Repository[alias]
		if bad || row == nil || row.Number != n || row.Comments == nil || row.Comments.Nodes == nil || len(*row.Comments.Nodes) > 100 {
			out[n] = ChangeCommentTimes{Err: Unverifiable("cannot read change comment activity for "+repo.Slug()+fmt.Sprintf("#%d", n), nil)}
			continue
		}
		result := ChangeCommentTimes{Comments: []Comment{}}
		for _, c := range *row.Comments.Nodes {
			author := Account{}
			if c.Author != nil {
				author = gqlActorAccount(c.Author)
			}
			result.Comments = append(result.Comments, Comment{Author: author, CreatedAt: c.CreatedAt})
		}
		out[n] = result
	}
	// Keep native query and result types local to this provider; callers hold no
	// credential or GraphQL document and cannot widen the selected population.
	return out, nil
}
