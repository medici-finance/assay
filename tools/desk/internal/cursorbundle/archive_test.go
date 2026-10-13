package cursorbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"testing"
)

func archiveFixture(t *testing.T, entries []struct {
	name, body string
	kind       byte
}) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: e.kind}
		if e.kind == tar.TypeSymlink {
			h.Size = 0
			h.Linkname = "outside"
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// regression: #2701
func TestArchiveClosedManifest(t *testing.T) {
	type entry = struct {
		name, body string
		kind       byte
	}
	plugin := `{"version":"1.2.3"}`
	manifest := archiveManifest{Schema: 1, Version: "v1.2.3", Files: map[string]string{".claude-plugin/plugin.json": digest([]byte(plugin))}}
	m, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	good := []entry{{"manifest.json", string(m), tar.TypeReg}, {".claude-plugin/plugin.json", plugin, tar.TypeReg}}
	raw := archiveFixture(t, good)
	if _, err := VerifyArchive(raw, "v1.2.3", digest(raw)); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]entry{
		"unlisted":      append(append([]entry{}, good...), entry{"extra.txt", "unlisted", tar.TypeReg}),
		"duplicate":     append(append([]entry{}, good...), good[1]),
		"case-alias":    append(append([]entry{}, good...), entry{".claude-plugin/PLUGIN.json", plugin, tar.TypeReg}),
		"escape":        append(append([]entry{}, good...), entry{"../outside", "bad", tar.TypeReg}),
		"symlink":       append(append([]entry{}, good...), entry{"link", "", tar.TypeSymlink}),
		"altered-entry": {{"manifest.json", string(m), tar.TypeReg}, {".claude-plugin/plugin.json", `{"version":"9.9.9"}`, tar.TypeReg}},
		"missing":       {good[0]},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			raw := archiveFixture(t, entries)
			if _, err := VerifyArchive(raw, "v1.2.3", digest(raw)); err == nil {
				t.Fatal("malformed archive admitted despite valid outer digest")
			}
		})
	}
	verified, err := VerifyArchive(raw, "v1.2.3", digest(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := verified.Materialize(t.TempDir()); err == nil {
		t.Fatal("incomplete placement source accepted")
	}
}
