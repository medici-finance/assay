package deskkit

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

// blobID is the git object id of a blob holding body, in the given hash (sha1 or sha256).
func blobID(body string, wide bool) string {
	hdr := fmt.Sprintf("blob %d\x00%s", len(body), body)
	if wide {
		s := sha256.Sum256([]byte(hdr))
		return hex.EncodeToString(s[:])
	}
	s := sha1.Sum([]byte(hdr))
	return hex.EncodeToString(s[:])
}

// regression: S-2266-7 C-2266-6 S-2266-8 — the Contents API is not a record of the entry's
// own bytes. A symbolic link to a file is served as type "file" under the REQUESTED path
// with the link's own blob sha and the target's content; a large file is served with
// encoding "none" and an empty body. ReadFile must say "regular file" only when the served
// bytes hash to the served sha, so a caller that needs a plain record can refuse the rest.
func TestGitHubReadFileKind(t *testing.T) {
	const file = "docs/streams/sdlc/brief-18.md"
	const linkText = "other.md" // what a link's own blob holds: its target path
	cases := []struct {
		name string
		resp map[string]any
		want string
	}{
		{"regular file", map[string]any{"type": "file", "path": file, "encoding": "base64", "sha": blobID("x\n", false), "content": ghB64("x\n")}, FileKindRegular},
		{"regular empty file", map[string]any{"type": "file", "path": file, "encoding": "base64", "sha": blobID("", false), "content": ""}, FileKindRegular},
		{"regular file in a sha256 repository", map[string]any{"type": "file", "path": file, "encoding": "base64", "sha": blobID("x\n", true), "content": ghB64("x\n")}, FileKindRegular},
		// The live shape of a link to a file: the requested path, the link's blob, the target's bytes.
		{"link to a file, live shape", map[string]any{"type": "file", "path": file, "encoding": "base64", "sha": blobID(linkText, false), "content": ghB64("y\n")}, FileKindUnproven},
		{"link to a file, under the target's path", map[string]any{"type": "file", "path": "docs/streams/sdlc/other.md", "encoding": "base64", "sha": "b2", "content": ghB64("y\n")}, FileKindSymlink},
		// A file the endpoint serves without its content reads as an empty regular file unless the hash is checked.
		{"content omitted for size", map[string]any{"type": "file", "path": file, "encoding": "none", "sha": blobID("a large file", false), "content": ""}, FileKindUnproven},
		{"empty body for a non-empty blob", map[string]any{"type": "file", "path": file, "encoding": "base64", "sha": blobID("a large file", false), "content": ""}, FileKindUnproven},
		{"content not base64-encoded", map[string]any{"type": "file", "path": file, "encoding": "utf-8", "sha": blobID("x\n", false), "content": ghB64("x\n")}, FileKindUnproven},
		{"served sha is not an object id", map[string]any{"type": "file", "path": file, "encoding": "base64", "sha": "b1", "content": ghB64("x\n")}, FileKindUnproven},
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

// regression: S-2266-8 — WriteFile's own pre-write read is the base its idempotency and
// append-only shrink checks compare against. A file the Contents API serves without its
// content (over the size limit: encoding "none", an empty body) read as an empty prior
// file, so a whole-file write that drops rows passed the shrink guard against nothing. The
// write must stop as could-not-check, with no PUT, whenever the branch's entry is not a
// plain file whose served bytes are its own blob; the matched control (the same file
// served in full) writes, and an append-only shrink against it is still refused.
func TestGitHubWriteFileRefusesUnprovenPrior(t *testing.T) {
	const file = "docs/streams/sdlc/verify-outcomes.jsonl"
	const prior = "{\"row\":1}\n{\"row\":2}\n"
	put := map[string]any{"content": map[string]any{"sha": "new"}, "commit": map[string]any{"sha": "c1", "author": map[string]any{"name": "bot"}}}
	in := WriteFileInput{File: file, Branch: "main", Content: []byte("{\"row\":3}\n"), Message: "m", AppendOnly: true}
	puts := func(s *goldenServer) int {
		n := 0
		for _, r := range s.requests {
			if r.Method == "PUT" {
				n++
			}
		}
		return n
	}
	refused := []struct {
		name string
		resp map[string]any
	}{
		{"content omitted for size", map[string]any{"type": "file", "path": file, "encoding": "none", "sha": blobID(prior, false), "content": ""}},
		{"link to a file, live shape", map[string]any{"type": "file", "path": file, "encoding": "base64", "sha": blobID("other.jsonl", false), "content": ghB64(prior)}},
		{"link to a file, under the target's path", map[string]any{"type": "file", "path": "docs/streams/sdlc/other.jsonl", "encoding": "base64", "sha": blobID(prior, false), "content": ghB64(prior)}},
		{"link to nothing", map[string]any{"type": "symlink", "path": file, "sha": blobID("gone", false)}},
		{"submodule", map[string]any{"type": "submodule", "path": file, "sha": blobID("x", false)}},
		{"no type served", map[string]any{"sha": blobID(prior, false), "content": ghB64(prior)}},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			s := newGoldenServer(t)
			s.contentsGet, s.contentsPut = c.resp, put
			res, err := s.forge().WriteFile(forgeTestRepo, in)
			if err == nil || !IsUnverifiable(err) {
				t.Fatalf("WriteFile = %+v, %v; want could-not-check", res, err)
			}
			if n := puts(s); n != 0 {
				t.Fatalf("refused write still sent %d PUT(s)", n)
			}
		})
	}
	t.Run("control: the same file served in full", func(t *testing.T) {
		s := newGoldenServer(t)
		s.contentsGet = map[string]any{"type": "file", "path": file, "encoding": "base64", "sha": blobID(prior, false), "content": ghB64(prior)}
		s.contentsPut = put
		grow := in
		grow.Content = []byte(prior + "{\"row\":3}\n")
		if _, err := s.forge().WriteFile(forgeTestRepo, grow); err != nil || puts(s) != 1 {
			t.Fatalf("WriteFile = %v with %d PUT(s); want one write", err, puts(s))
		}
		s2 := newGoldenServer(t)
		s2.contentsGet, s2.contentsPut = s.contentsGet, put
		if _, err := s2.forge().WriteFile(forgeTestRepo, in); err == nil || !IsRefused(err) || puts(s2) != 0 {
			t.Fatalf("shrink against the real prior = %v with %d PUT(s); want refused, no write", err, puts(s2))
		}
	})
}
