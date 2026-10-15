package deskkit

import "testing"

// regression: S-2266-7 — the Contents API serves a symbolic link to a file as that file
// (type "file" under the target's path), any other link as itself with no content, and a
// submodule as type "submodule"; ReadFile must say which, so a caller that needs a plain
// file can refuse the rest.
func TestGitHubReadFileKind(t *testing.T) {
	const file = "docs/streams/sdlc/brief-18.md"
	cases := []struct {
		name string
		resp map[string]any
		want string
	}{
		{"regular file", map[string]any{"type": "file", "path": file, "sha": "b1", "content": ghB64("x\n")}, FileKindRegular},
		{"link to a file", map[string]any{"type": "file", "path": "docs/streams/sdlc/other.md", "sha": "b2", "content": ghB64("y\n")}, FileKindSymlink},
		{"link to nothing", map[string]any{"type": "symlink", "path": file, "sha": "b3", "target": "../gone.md"}, FileKindSymlink},
		{"submodule", map[string]any{"type": "submodule", "path": file, "sha": "b4", "submodule_git_url": "https://example.invalid/x.git"}, FileKindSubmodule},
		{"no type served", map[string]any{"sha": "b5", "content": ghB64("z\n")}, ""},
		{"file with no path", map[string]any{"type": "file", "sha": "b6", "content": ghB64("z\n")}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newGoldenServer(t)
			s.contentsGet = c.resp
			fc, err := s.forge().ReadFile(forgeTestRepo, ReadFileInput{File: file, Ref: "main"})
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if fc.Kind != c.want {
				t.Fatalf("Kind = %q, want %q", fc.Kind, c.want)
			}
		})
	}
}
