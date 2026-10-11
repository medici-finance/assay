package loopengine

import (
	"reflect"
	"testing"
)

// TestFrontmatterList pins the exported reader to the grammar the write-scope derivation
// reads: both list forms, and "not stated" kept apart from "stated, and empty".
func TestFrontmatterList(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    []string
		wantOK  bool
	}{
		{"inline", "---\nid: a/01\ndepends: [\"a/02\", b/03]\n---\n# T\n", []string{"a/02", "b/03"}, true},
		{"block", "---\ndepends:\n  - a/02\n  - b/03\ngate: model\n---\n# T\n", []string{"a/02", "b/03"}, true},
		{"stated and empty", "---\ndepends: []\n---\n# T\n", nil, true},
		{"not stated", "---\nid: a/01\n---\n# T\n", nil, false},
		{"no frontmatter", "# T\n\ndepends: [a/02]\n", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := FrontmatterList(tc.content, "depends")
			if ok != tc.wantOK || (len(got)+len(tc.want) > 0 && !reflect.DeepEqual(got, tc.want)) {
				t.Errorf("FrontmatterList = %q, %v; want %q, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
