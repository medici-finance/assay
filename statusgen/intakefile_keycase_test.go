package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// ----- issue #931: parseIntakeFile matches frontmatter keys case-insensitively -----
//
// yaml.v3 matches struct tags case-sensitively, so `Disposition: accepted`
// used to leave Disposition empty and the entry silently defaulted to the
// untriaged "new" — the per-entry twin of the legacy-path gap fixed for
// parseIntakeLegacy (#915). The fold covers every intakeEntry field, never
// rewrites a value, and refuses an owned key that repeats in differing case.

func TestParseIntakeFileKeyCase(t *testing.T) {
	cases := []struct {
		name     string
		fm       string
		wantDisp string
		wantID   string
		wantWhy  string
		wantErr  string // substring of the error; "" means no error
	}{
		{
			name:     "canonical lowercase key",
			fm:       "id: I-1\ndate: \"2026-07-08\"\ntitle: T\ndisposition: accepted\n",
			wantDisp: "accepted",
			wantID:   "I-1",
		},
		{
			name:     "TitleCase key",
			fm:       "id: I-1\ndate: \"2026-07-08\"\ntitle: T\nDisposition: accepted\n",
			wantDisp: "accepted",
			wantID:   "I-1",
		},
		{
			name:     "upper key",
			fm:       "id: I-1\ndate: \"2026-07-08\"\ntitle: T\nDISPOSITION: rejected\n",
			wantDisp: "rejected",
			wantID:   "I-1",
		},
		{
			name:     "mixed key",
			fm:       "id: I-1\ndate: \"2026-07-08\"\ntitle: T\nDisPoSiTiOn: scoped → example-stream\n",
			wantDisp: "scoped → example-stream",
			wantID:   "I-1",
		},
		{
			name:     "quoted key, spaced colon, value untouched",
			fm:       "id: I-1\ndate: \"2026-07-08\"\ntitle: T\n\"Disposition\" :   \"  Accepted  \"\n",
			wantDisp: "  Accepted  ",
			wantID:   "I-1",
		},
		{
			name:     "missing key defaults to new",
			fm:       "id: I-1\ndate: \"2026-07-08\"\ntitle: T\n",
			wantDisp: "new",
			wantID:   "I-1",
		},
		{
			name:     "all owned fields fold",
			fm:       "ID: I-2\nDATE: \"2026-07-08\"\nTitle: T\nDisposition: watching\nScoped-To: example-stream\nWHY: because\nDecision-Issue: \"42\"\n",
			wantDisp: "watching",
			wantID:   "I-2",
			wantWhy:  "because",
		},
		{
			name:    "dup disposition differing case errors",
			fm:      "id: I-1\ndate: \"2026-07-08\"\ntitle: T\ndisposition: accepted\nDisposition: rejected\n",
			wantErr: `"Disposition" (line 5) repeats "disposition" (line 4)`,
		},
		{
			name:    "dup id differing case errors",
			fm:      "id: I-1\nID: I-9\ndate: \"2026-07-08\"\ntitle: T\ndisposition: accepted\n",
			wantErr: `"ID" (line 2) repeats "id" (line 1)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte("---\n" + tc.fm + "---\n\nBody.\n")
			e, err := parseIntakeFile(raw)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (entry %+v)", tc.wantErr, e)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseIntakeFile: %v", err)
			}
			if e.Disposition != tc.wantDisp {
				t.Errorf("Disposition = %q, want %q", e.Disposition, tc.wantDisp)
			}
			if e.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", e.ID, tc.wantID)
			}
			if tc.wantWhy != "" {
				if e.Why != tc.wantWhy {
					t.Errorf("Why = %q, want %q", e.Why, tc.wantWhy)
				}
				if e.Date != "2026-07-08" || e.Title != "T" || e.ScopedTo != "example-stream" || e.DecisionIssue != "42" {
					t.Errorf("folded fields: Date=%q Title=%q ScopedTo=%q DecisionIssue=%q", e.Date, e.Title, e.ScopedTo, e.DecisionIssue)
				}
			}
			if e.Body != "Body." {
				t.Errorf("Body = %q, want %q", e.Body, "Body.")
			}
		})
	}
}

// TestParseIntakeDirDupKeyNamesFile pins that the ambiguous-key
// refusal reaches the register reader as an error that names the file, so the
// board fails closed on the one entry instead of silently counting it.
func TestParseIntakeDirDupKeyNamesFile(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	writeTemp(t, idir, "2026-07-08-dup.md",
		"---\nid: I-dup\ndate: \"2026-07-08\"\ntitle: Dup\ndisposition: accepted\nDISPOSITION: rejected\n---\n\nBody.")

	_, err := parseIntakeDir(root)
	if err == nil {
		t.Fatal("expected parseIntakeDir to refuse a differing-case duplicate key, got nil")
	}
	for _, want := range []string{"2026-07-08-dup.md", `"DISPOSITION" (line 5)`, `"disposition" (line 4)`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err.Error(), want)
		}
	}
}

// TestParseIntakeDirTitleCaseCounts pins the user-visible symptom
// from #931 end to end: a title-cased key no longer leaves the entry untriaged.
func TestParseIntakeDirTitleCaseCounts(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	writeTemp(t, idir, "2026-07-08-tc.md",
		"---\nid: I-tc\ndate: \"2026-07-08\"\ntitle: TC\nDisposition: accepted\n---\n\nBody.")

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Disposition != "accepted" {
		t.Errorf("Disposition = %q, want %q (title-cased key must not fall back to new)", entries[0].Disposition, "accepted")
	}
}
