package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// IssueStateBatchSize bounds one positive-state observation. It does not change
// listing/history page caps or replace a positive read with listing absence.
const IssueStateBatchSize = 20

// IssueState is the narrow observation needed to retire a local placeholder.
type IssueState struct {
	Number int
	State  string
	Title  string
}

// IssueStateAccess supplies only this fixed read, under an already admitted
// repository. It exposes neither a credential nor an arbitrary GraphQL query.
type IssueStateAccess interface {
	IssueStateBatch(context.Context, string, []int) ([]byte, error)
}

var issueStateDocument = func() string {
	var variables, fields strings.Builder
	for slot := 0; slot < IssueStateBatchSize; slot++ {
		fmt.Fprintf(&variables, ",$n%d:Int!,$use%d:Boolean!", slot, slot)
		fmt.Fprintf(&fields, "a%d:issue(number:$n%d) @include(if:$use%d){number state title} ", slot, slot, slot)
	}
	return "query($owner:String!,$name:String!" + variables.String() + "){repository(owner:$owner,name:$name){" + fields.String() + "}}"
}()

// IssueStateQuery returns one fixed document. Repository and issue identities
// remain variables; no provider values or caller selections become query text.
func IssueStateQuery(numbers []int) (string, error) {
	if len(numbers) == 0 || len(numbers) > IssueStateBatchSize {
		return "", fmt.Errorf("positive issue-state batch needs 1..%d numbers", IssueStateBatchSize)
	}
	seen := map[int]bool{}
	for _, n := range numbers {
		if n <= 0 || int64(n) > 1<<31-1 || seen[n] {
			return "", fmt.Errorf("positive issue-state batch has an invalid or repeated number")
		}
		seen[n] = true
	}
	return issueStateDocument, nil
}

// IssueStateVariables names only the bounded issue-number slots. Repository
// variables are supplied by the repository-scoped authenticated composition.
func IssueStateVariables(numbers []int) (map[string]any, error) {
	if _, err := IssueStateQuery(numbers); err != nil {
		return nil, err
	}
	vars := map[string]any{}
	for slot := 0; slot < IssueStateBatchSize; slot++ {
		n := 0
		active := slot < len(numbers)
		if active {
			n = numbers[slot]
		}
		vars[fmt.Sprintf("n%d", slot)] = n
		vars[fmt.Sprintf("use%d", slot)] = active
	}
	return vars, nil
}

// PositiveIssueStates reads every explicitly requested issue. Missing/null data,
// any GraphQL error, identity drift and unknown state refuse the entire result.
// A partial observation is never evidence that a placeholder may be retired.
func PositiveIssueStates(ctx context.Context, access IssueStateAccess, repo string, numbers []int) ([]IssueState, error) {
	// Validate the whole identity set before any read.
	seen := map[int]bool{}
	for _, n := range numbers {
		if n <= 0 || int64(n) > 1<<31-1 || seen[n] {
			return nil, fmt.Errorf("positive issue-state set has an invalid or repeated number")
		}
		seen[n] = true
	}
	out := make([]IssueState, 0, len(numbers))
	for start := 0; start < len(numbers); start += IssueStateBatchSize {
		end := min(start+IssueStateBatchSize, len(numbers))
		batch := append([]int(nil), numbers[start:end]...)
		raw, err := access.IssueStateBatch(ctx, repo, batch)
		if err != nil {
			return nil, err
		}
		var env struct {
			Data *struct {
				Repository map[string]json.RawMessage `json:"repository"`
			} `json:"data"`
			Errors []json.RawMessage `json:"errors"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("cannot decode positive issue-state observation: %w", err)
		}
		if len(env.Errors) != 0 || env.Data == nil || env.Data.Repository == nil || len(env.Data.Repository) != len(batch) {
			return nil, fmt.Errorf("positive issue-state observation is incomplete or carries GraphQL errors")
		}
		for slot, n := range batch {
			var row *struct {
				Number int     `json:"number"`
				State  string  `json:"state"`
				Title  *string `json:"title"`
			}
			if err := json.Unmarshal(env.Data.Repository[fmt.Sprintf("a%d", slot)], &row); err != nil || row == nil || row.Number != n || row.Title == nil {
				return nil, fmt.Errorf("positive issue-state observation for issue %d is missing or has a different identity", n)
			}
			state := strings.ToLower(row.State)
			if state != "open" && state != "closed" {
				return nil, fmt.Errorf("positive issue-state observation for issue %d has an unknown state", n)
			}
			out = append(out, IssueState{Number: n, State: state, Title: *row.Title})
		}
	}
	return out, nil
}
