package deskkit

import "testing"

// regression: #4885
func TestQueueVisibilityScalars(t *testing.T) {
	for _, tc := range []struct {
		name string
		wire any
		want string
	}{
		{"public", "PUBLIC", "public"},
		{"private", "PRIVATE", "private"},
		{"internal", "INTERNAL", "internal"},
		{"missing", nil, ""},
		{"empty", "", ""},
		{"future-enum", "FUTURE_VISIBILITY", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newGoldenServer(t)
			repo := map[string]any{"pullRequests": map[string]any{"nodes": []any{}}}
			if tc.wire != nil {
				repo["visibility"] = tc.wire
			}
			s.graphql = map[string]any{"data": map[string]any{"repository": repo}}
			q, err := s.forge().ReviewQueueSnapshot(forgeTestRepo)
			if err != nil || q.Visibility != tc.want || len(s.requests) != 1 {
				t.Fatalf("visibility: queue=%+v error=%v requests=%d want=%q", q, err, len(s.requests), tc.want)
			}
		})
	}
}

// regression: #4885
func TestQueueVisibilityPartialError(t *testing.T) {
	s := newGoldenServer(t)
	s.graphql = map[string]any{
		"data": map[string]any{"repository": map[string]any{
			"visibility": "PRIVATE", "pullRequests": map[string]any{"nodes": []any{}},
		}},
		"errors": []any{map[string]any{"message": "fixture partial response"}},
	}
	q, err := s.forge().ReviewQueueSnapshot(forgeTestRepo)
	if err == nil || q != nil {
		t.Fatalf("partial snapshot became usable repository facts: %+v %v", q, err)
	}
}
