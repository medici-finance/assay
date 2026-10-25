package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type issueStateFixture struct {
	calls  [][]int
	answer func([]int) ([]byte, error)
}

func (f *issueStateFixture) IssueStateBatch(_ context.Context, _ string, ns []int) ([]byte, error) {
	f.calls = append(f.calls, append([]int(nil), ns...))
	return f.answer(ns)
}

func issueStatesBody(ns []int) []byte {
	rows := map[string]any{}
	for slot, n := range ns {
		rows[fmt.Sprintf("a%d", slot)] = map[string]any{"number": n, "state": "CLOSED", "title": fmt.Sprintf("title-%d", n)}
	}
	b, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": rows}})
	return b
}

// regression: #4360
// regression: class issue-positive-state-batch
func TestPositiveIssueStatesBoundAndIdentity(t *testing.T) {
	for _, count := range []int{0, 1, 20, 21, 95} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			f := &issueStateFixture{answer: func(ns []int) ([]byte, error) { return issueStatesBody(ns), nil }}
			var ns []int
			for n := 1; n <= count; n++ {
				ns = append(ns, n)
			}
			got, err := PositiveIssueStates(context.Background(), f, "fixture/repo", ns)
			if err != nil || len(got) != count || len(f.calls) != (count+19)/20 {
				t.Fatal(got, err, f.calls)
			}
			for i, row := range got {
				if row.Number != i+1 || row.State != "closed" || row.Title != fmt.Sprintf("title-%d", i+1) {
					t.Fatal(row)
				}
			}
			for _, page := range f.calls {
				if len(page) > 20 {
					t.Fatal(page)
				}
				q, err := IssueStateQuery(page)
				if err != nil || strings.Contains(q, "mutation") {
					t.Fatal(q, err)
				}
			}
		})
	}
}

// regression: #4360
// regression: class issue-positive-state-batch
func TestPositiveIssueStatesRefusesPartialOrChanged(t *testing.T) {
	for name, body := range map[string]string{
		"null-data":       `{"data":null}`,
		"null-repo":       `{"data":{"repository":null}}`,
		"missing-alias":   `{"data":{"repository":{}}}`,
		"null-issue":      `{"data":{"repository":{"a0":null}}}`,
		"wrong-number":    `{"data":{"repository":{"a0":{"number":8,"state":"CLOSED","title":"wrong"}}}}`,
		"unknown-state":   `{"data":{"repository":{"a0":{"number":7,"state":"DELETED","title":"unknown"}}}}`,
		"missing-title":   `{"data":{"repository":{"a0":{"number":7,"state":"CLOSED"}}}}`,
		"extra-alias":     `{"data":{"repository":{"a0":{"number":7,"state":"CLOSED","title":"a"},"a1":{"number":8,"state":"CLOSED","title":"b"}}}}`,
		"data-and-errors": `{"data":{"repository":{"a0":{"number":7,"state":"CLOSED","title":"valid"}}},"errors":[{"message":"permission denied"}]}`,
		"truncated":       `{"data":`,
	} {
		t.Run(name, func(t *testing.T) {
			f := &issueStateFixture{answer: func([]int) ([]byte, error) { return []byte(body), nil }}
			got, err := PositiveIssueStates(context.Background(), f, "fixture/repo", []int{7})
			if err == nil || got != nil || len(f.calls) != 1 {
				t.Fatal(got, err, f.calls)
			}
		})
	}
	f := &issueStateFixture{answer: func([]int) ([]byte, error) { return nil, errors.New("permission denied") }}
	if out, err := PositiveIssueStates(context.Background(), f, "fixture/repo", []int{1, 2}); err == nil || out != nil || len(f.calls) != 1 {
		t.Fatal(out, err, f.calls)
	}
	for _, ns := range [][]int{{0}, {-1}, {1, 1}} {
		f.calls = nil
		if out, err := PositiveIssueStates(context.Background(), f, "fixture/repo", ns); err == nil || out != nil || len(f.calls) != 0 {
			t.Fatal(ns, out, err, f.calls)
		}
	}
}
