package deskkit

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
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

// ghEntry is the GraphQL answer for the file's tree entry at the ref: its mode (decimal, as
// GitHub serves it) and object id, reached through a commit (wrapped in a tag object when tag
// is true). A nil mode answers with no entry and the forge's NOT_FOUND error.
func ghEntry(mode any, oid string, tag bool) map[string]any {
	if mode == nil {
		return map[string]any{
			"data":   map[string]any{"repository": map[string]any{"object": map[string]any{"__typename": "Commit", "file": nil}}},
			"errors": []any{map[string]any{"type": "NOT_FOUND", "path": []any{"repository", "object", "file"}}},
		}
	}
	commit := map[string]any{"__typename": "Commit", "file": map[string]any{"mode": mode, "oid": oid}}
	obj := commit
	if tag {
		obj = map[string]any{"__typename": "Tag", "target": commit}
	}
	return map[string]any{"data": map[string]any{"repository": map[string]any{"object": obj}}}
}

// regression: #3149 — the blob-hash proof cannot tell a regular file from a symbolic link
// whose target holds exactly the link's own text: the link's blob is its target path, the
// target's bytes equal that text, so the served bytes hash to the served sha. Only the tree
// entry's mode separates them. A regular file is reported only when the entry at the ref
// has a regular-file mode and is the same blob the Contents API served.
func TestGitHubReadFileTreeMode(t *testing.T) {
	const file = "docs/streams/sdlc/brief-18.md"
	const text = "brief-18.md" // a link to itself would hold this, and so would its "target"
	served := map[string]any{"type": "file", "path": file, "encoding": "base64", "sha": blobID(text, false), "content": ghB64(text)}
	oid := blobID(text, false)
	cases := []struct {
		name  string
		ref   string
		entry any
		want  string
	}{
		{"regular mode", "main", ghEntry(0o100644, oid, false), FileKindRegular},
		{"executable mode", "main", ghEntry(0o100755, oid, false), FileKindRegular},
		{"regular mode through a tag", "v1.0.0", ghEntry(0o100644, oid, true), FileKindRegular},
		{"default branch", "", ghEntry(0o100644, oid, false), FileKindRegular},
		{"self-identical link", "main", ghEntry(0o120000, oid, false), FileKindSymlink},
		{"submodule entry", "main", ghEntry(0o160000, oid, false), FileKindSubmodule},
		{"entry is another blob", "main", ghEntry(0o100644, blobID("moved", false), false), FileKindUnproven},
		{"no entry at the ref", "main", ghEntry(nil, "", false), FileKindUnproven},
		{"unknown mode", "main", ghEntry(0o40000, oid, false), FileKindUnproven},
		{"mode read refused", "main", "500", FileKindUnproven},
		{"empty answer", "main", map[string]any{"data": map[string]any{"repository": nil}}, FileKindUnproven},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newGoldenServer(t)
			s.contentsGet = served
			if c.entry == "500" {
				s.forceStatus["/graphql"] = http.StatusInternalServerError
			} else {
				s.fileEntry = c.entry.(map[string]any)
			}
			fc, err := s.forge().ReadFile(forgeTestRepo, ReadFileInput{File: file, Ref: c.ref})
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if fc.Kind != c.want {
				t.Fatalf("Kind = %q, want %q", fc.Kind, c.want)
			}
			if string(fc.Content) != text {
				t.Fatalf("Content = %q, want the served bytes", fc.Content)
			}
			// The entry is looked up at the same ref the Contents API read, and for the same path.
			var q struct{ Variables map[string]any }
			for _, r := range s.requests {
				if r.Method == http.MethodPost && r.Path == "/graphql" {
					_ = json.Unmarshal(r.Body, &q)
				}
			}
			wantRev := c.ref
			if wantRev == "" {
				wantRev = "HEAD"
			}
			if q.Variables["rev"] != wantRev || q.Variables["file"] != file {
				t.Fatalf("entry lookup variables = %v, want rev %q file %q", q.Variables, wantRev, file)
			}
		})
	}
}

// regression: #3149 — no Contents-API answer alone yields a regular file: whatever the
// response, the regular kind comes only from the tree-entry check. A read that skips that
// check reports could-not-check, never "file".
func TestGhContentsKindNeverRegular(t *testing.T) {
	const file = "a/b.md"
	for _, body := range []string{"", "x\n", "b.md"} {
		w := ghContentsWire{Type: "file", Path: file, Encoding: "base64", SHA: blobID(body, false)}
		if k := ghContentsKind(w, file, []byte(body)); k == FileKindRegular {
			t.Fatalf("ghContentsKind(%q) = %q; a regular kind needs the tree entry's mode", body, k)
		}
	}
}

// regression: #3149 — WriteFile's pre-write read goes through the same kind proof, so a
// branch entry that is a self-identical link is refused with no PUT, while the same bytes
// held as a regular file still write.
func TestGitHubWriteFileRefusesLink(t *testing.T) {
	const file = "docs/streams/sdlc/rows.jsonl"
	const text = "rows.jsonl"
	put := map[string]any{"content": map[string]any{"sha": "new"}, "commit": map[string]any{"sha": "c1", "author": map[string]any{"name": "bot"}}}
	in := WriteFileInput{File: file, Branch: "main", Content: []byte(text + "\nmore\n"), Message: "m"}
	for _, c := range []struct {
		name   string
		mode   int
		writes int
	}{{"self-identical link", 0o120000, 0}, {"control: regular file", 0o100644, 1}} {
		t.Run(c.name, func(t *testing.T) {
			s := newGoldenServer(t)
			s.contentsGet, s.contentsPut = ghServedFile(file, text), put
			s.fileEntry = ghEntry(c.mode, blobID(text, false), false)
			_, err := s.forge().WriteFile(forgeTestRepo, in)
			n := 0
			for _, r := range s.requests {
				if r.Method == http.MethodPut {
					n++
				}
			}
			if n != c.writes || (c.writes == 0) != (err != nil && IsUnverifiable(err)) {
				t.Fatalf("WriteFile = %v with %d PUT(s); want %d", err, n, c.writes)
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
