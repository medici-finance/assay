package deskkit

import (
	"testing"
)

// forge_writefile_test.go — TestWriteFileOpBothBackends (the write-verbs brief's Verify row 9).
//
// The file-write and file-read ops added by this brief run the SAME scenario names against BOTH
// backends' recorded fixtures, so a behavioural gap between GitHub and GitLab reads as a named
// failing scenario rather than as silence. Each scenario names an expectation on the mapped
// WriteFileResult / FileContent — the Changed flag, the DefaultBranchNotWritable sentinel, the
// append-only shrink refusal — and both backends must satisfy it. The per-operation wire footprint
// stays pinned by the golden corpora (read_file / write_file_* in TestForgeGithubGolden and
// TestForgeGitlabGolden); this test pins the cross-backend behavioural equivalence the mapping is
// really about.

func TestWriteFileOpBothBackends(t *testing.T) {
	scenarios := []struct {
		name    string
		setupGH func(*goldenServer)
		setupGL func(*glServer)
		run     func(f Forge) (any, error)
		check   func(t *testing.T, backend string, got any, err error)
	}{
		{
			name: "read_file_returns_content_and_id",
			setupGH: func(s *goldenServer) {
				s.contentsGet = map[string]any{"sha": "blob-gh", "content": ghB64("row one\n")}
			},
			setupGL: func(s *glServer) {
				s.repoFile = map[string]map[string]any{"EVIDENCE.md": {
					"file_name": "EVIDENCE.md", "file_path": "EVIDENCE.md",
					"content": glB64("row one\n"), "encoding": "base64",
					"ref": "feat/x", "blob_id": "blob-gl", "last_commit_id": "commit-gl",
				}}
			},
			run: func(f Forge) (any, error) {
				return f.ReadFile(forgeTestRepo, ReadFileInput{File: "EVIDENCE.md", Ref: "feat/x"})
			},
			check: func(t *testing.T, backend string, got any, err error) {
				if err != nil {
					t.Fatalf("%s: ReadFile errored: %v", backend, err)
				}
				fc := got.(*FileContent)
				if string(fc.Content) != "row one\n" {
					t.Errorf("%s: content = %q, want %q", backend, fc.Content, "row one\n")
				}
				if !fc.Exists {
					t.Errorf("%s: Exists = false, want true", backend)
				}
				if fc.SHA == "" {
					t.Errorf("%s: SHA empty — the opaque content id an update cites is missing", backend)
				}
			},
		},
		{
			name: "write_file_changes_existing_content",
			setupGH: func(s *goldenServer) {
				s.contentsGet = map[string]any{"sha": "blob-gh", "content": ghB64("row one\n")}
				s.contentsPut = map[string]any{
					"content": map[string]any{"sha": "blob-gh-2"},
					"commit": map[string]any{"author": map[string]any{"name": "assay-verifier-app[bot]"}},
				}
			},
			setupGL: func(s *glServer) {
				s.project = map[string]any{"visibility": "private", "default_branch": "main"}
				s.repoFile = map[string]map[string]any{"EVIDENCE.md": {
					"file_name": "EVIDENCE.md", "file_path": "EVIDENCE.md",
					"content": glB64("row one\n"), "encoding": "base64",
					"ref": "feat/x", "blob_id": "blob-gl", "last_commit_id": "commit-gl",
				}}
				s.updateFileResp = map[string]any{"file_path": "EVIDENCE.md", "branch": "feat/x"}
			},
			run: func(f Forge) (any, error) {
				return f.WriteFile(forgeTestRepo, WriteFileInput{
					File: "EVIDENCE.md", Branch: "feat/x", Content: []byte("row one\nrow two\n"),
					Message: "Evidence: verification row",
				})
			},
			check: func(t *testing.T, backend string, got any, err error) {
				if err != nil {
					t.Fatalf("%s: WriteFile errored: %v", backend, err)
				}
				res := got.(*WriteFileResult)
				if !res.Changed {
					t.Errorf("%s: Changed = false, want true — the content differs", backend)
				}
				if res.DefaultBranchNotWritable {
					t.Errorf("%s: DefaultBranchNotWritable set on a non-default branch write", backend)
				}
			},
		},
		{
			name: "write_file_idempotent_noop_on_identical_content",
			setupGH: func(s *goldenServer) {
				s.contentsGet = map[string]any{"sha": "blob-gh", "content": ghB64("row one\n")}
			},
			setupGL: func(s *glServer) {
				s.project = map[string]any{"visibility": "private", "default_branch": "main"}
				s.repoFile = map[string]map[string]any{"EVIDENCE.md": {
					"file_name": "EVIDENCE.md", "file_path": "EVIDENCE.md",
					"content": glB64("row one\n"), "encoding": "base64",
					"ref": "feat/x", "blob_id": "blob-gl", "last_commit_id": "commit-gl",
				}}
			},
			run: func(f Forge) (any, error) {
				return f.WriteFile(forgeTestRepo, WriteFileInput{
					File: "EVIDENCE.md", Branch: "feat/x", Content: []byte("row one\n"),
					Message: "Evidence: verification row",
				})
			},
			check: func(t *testing.T, backend string, got any, err error) {
				if err != nil {
					t.Fatalf("%s: WriteFile errored: %v", backend, err)
				}
				res := got.(*WriteFileResult)
				if res.Changed {
					t.Errorf("%s: Changed = true on byte-identical content — the idempotency read did not fold in", backend)
				}
			},
		},
		{
			name: "write_file_append_only_shrink_refused",
			setupGH: func(s *goldenServer) {
				s.contentsGet = map[string]any{"sha": "blob-gh", "content": ghB64("a\nb\nc\n")}
			},
			setupGL: func(s *glServer) {
				s.project = map[string]any{"visibility": "private", "default_branch": "main"}
				s.repoFile = map[string]map[string]any{"rows.jsonl": {
					"file_name": "rows.jsonl", "file_path": "rows.jsonl",
					"content": glB64("a\nb\nc\n"), "encoding": "base64",
					"ref": "feat/x", "blob_id": "blob-gl", "last_commit_id": "commit-gl",
				}}
			},
			run: func(f Forge) (any, error) {
				return f.WriteFile(forgeTestRepo, WriteFileInput{
					File: "rows.jsonl", Branch: "feat/x", Content: []byte("a\n"),
					Message: "shrink", AppendOnly: true,
				})
			},
			check: func(t *testing.T, backend string, got any, err error) {
				if err == nil {
					t.Fatalf("%s: append-only shrink was accepted; want a refusal", backend)
				}
				if ExitCodeOf(err) != ExitRefused {
					t.Errorf("%s: shrink refusal exit = %d, want %d (Refused)", backend, ExitCodeOf(err), ExitRefused)
				}
			},
		},
		{
			name: "write_file_default_branch_probe",
			setupGH: func(s *goldenServer) {
				// GitHub's default branch is directly writable by the App — carve-out.
				s.contentsGetStatus = 404 // absent → create
				s.contentsPut = map[string]any{
					"content": map[string]any{"sha": "blob-gh-new"},
					"commit": map[string]any{"author": map[string]any{"name": "assay-verifier-app[bot]"}},
				}
			},
			setupGL: func(s *glServer) {
				s.project = map[string]any{"visibility": "private", "default_branch": "main"}
			},
			run: func(f Forge) (any, error) {
				return f.WriteFile(forgeTestRepo, WriteFileInput{
					File: "EVIDENCE.md", Branch: "main", Content: []byte("row one\n"),
					Message: "Evidence: verification row",
				})
			},
			check: func(t *testing.T, backend string, got any, err error) {
				if err != nil {
					t.Fatalf("%s: WriteFile errored: %v", backend, err)
				}
				res := got.(*WriteFileResult)
				switch backend {
				case "github":
					// The direct-main carve-out: GitHub writes the default branch directly.
					if res.DefaultBranchNotWritable {
						t.Errorf("github: DefaultBranchNotWritable set — the direct-main carve-out was not honoured")
					}
					if !res.Changed {
						t.Errorf("github: Changed = false — the default-branch write did not land")
					}
				case "gitlab":
					// GitLab protects the default branch: the sentinel, nothing written.
					if !res.DefaultBranchNotWritable {
						t.Errorf("gitlab: DefaultBranchNotWritable NOT set on a default-branch write — the probe did not fire")
					}
					if res.Changed {
						t.Errorf("gitlab: Changed = true on the sentinel path — a write was made when none should have been")
					}
				}
			},
		},
	}

	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			t.Run("github", func(t *testing.T) {
				gs := newGoldenServer(t)
				if sc.setupGH != nil {
					sc.setupGH(gs)
				}
				got, err := sc.run(gs.forge())
				sc.check(t, "github", got, err)
			})
			t.Run("gitlab", func(t *testing.T) {
				ls := newGLServer(t)
				if sc.setupGL != nil {
					sc.setupGL(ls)
				}
				got, err := sc.run(ls.forge())
				sc.check(t, "gitlab", got, err)
			})
		})
	}
}
