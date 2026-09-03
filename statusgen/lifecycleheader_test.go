package main

import (
	"os"
	"path/filepath"
	"testing"
)

// lcWriteFile writes content to <root>/<rel>, creating parent dirs.
func lcWriteFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestSpecStatusHeaderParsing pins the §8.1/§8.3 header grammar: the first-token
// classification, the ` — <prose>` tail discarded for the machine value, a
// non-set first token yielding UNCLASSIFIED (never rounded up), and the
// header-block boundary that keeps a body prose mention from being read as state.
func TestSpecStatusHeaderParsing(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name         string
		content      string
		wantHasStat  bool
		wantState    string
		wantRaw      string
		wantRoutes   bool
		wantRoutesTo string
	}{
		{
			name:         "approved with routes-to",
			content:      "# Spec\n\n**Status:** approved\n**Routes-to:** docs/streams/x/\n\n## Body\n",
			wantHasStat:  true,
			wantState:    "approved",
			wantRaw:      "approved",
			wantRoutes:   true,
			wantRoutesTo: "docs/streams/x/",
		},
		{
			name:        "draft, no routes-to",
			content:     "# Spec\n\n**Status:** draft\n\n## Body\n",
			wantHasStat: true,
			wantState:   "draft",
			wantRaw:     "draft",
		},
		{
			name:         "routed with routes-to",
			content:      "**Status:** routed\n**Routes-to:** docs/streams/y/\n",
			wantHasStat:  true,
			wantState:    "routed",
			wantRaw:      "routed",
			wantRoutes:   true,
			wantRoutesTo: "docs/streams/y/",
		},
		{
			name:        "prose tail discarded for the machine value",
			content:     "**Status:** approved — ruled 2026-08-30 by the desk\n**Routes-to:** docs/streams/z/\n",
			wantHasStat: true,
			wantState:   "approved",
			wantRaw:     "approved",
			wantRoutes:  true, wantRoutesTo: "docs/streams/z/",
		},
		{
			name:        "uppercase DRAFT is unclassified (case-sensitive, §8.1)",
			content:     "**Status:** DRAFT — published for review\n",
			wantHasStat: true,
			wantState:   "", // unclassified — MUST NOT be rounded up to draft
			wantRaw:     "DRAFT",
		},
		{
			name:        "non-state first token is unclassified",
			content:     "**Status:** design accepted 2026-08-24\n",
			wantHasStat: true,
			wantState:   "",
			wantRaw:     "design",
		},
		{
			name:        "no status header -> not opted in",
			content:     "# Spec\n\nSome prose, no header.\n\n## Body\n",
			wantHasStat: false,
		},
		{
			name:        "a **Status:** mention below the first H2 is body prose, not the header",
			content:     "# Spec\n\n## 8. Grammar\n\n**Status:** approved is the form a line takes.\n",
			wantHasStat: false, // header block ended at `## 8.`
		},
		{
			name:        "routes-to line with empty destination does not count as present",
			content:     "**Status:** approved\n**Routes-to:**\n",
			wantHasStat: true,
			wantState:   "approved",
			wantRaw:     "approved",
			wantRoutes:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := lcWriteFile(t, root, "spec/"+tc.name+".md", tc.content)
			h, err := parseSpecLifecycleHeader(root, p)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if h.HasStatus != tc.wantHasStat {
				t.Errorf("HasStatus = %v, want %v", h.HasStatus, tc.wantHasStat)
			}
			if h.State != tc.wantState {
				t.Errorf("State = %q, want %q", h.State, tc.wantState)
			}
			if tc.wantHasStat && h.StateRaw != tc.wantRaw {
				t.Errorf("StateRaw = %q, want %q", h.StateRaw, tc.wantRaw)
			}
			if h.Routes != tc.wantRoutes {
				t.Errorf("Routes = %v, want %v", h.Routes, tc.wantRoutes)
			}
			if h.RoutesTo != tc.wantRoutesTo {
				t.Errorf("RoutesTo = %q, want %q", h.RoutesTo, tc.wantRoutesTo)
			}
			if tc.wantState != "" && !h.Classified() {
				t.Errorf("Classified() = false, want true for state %q", tc.wantState)
			}
			if tc.wantHasStat && tc.wantState == "" && h.Classified() {
				t.Errorf("Classified() = true for an unclassified header (%q) — MUST NOT be rounded up", tc.wantRaw)
			}
		})
	}
}
