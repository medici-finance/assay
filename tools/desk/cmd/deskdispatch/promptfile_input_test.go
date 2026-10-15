package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The caller's --prompt-file is the caller's own file. It is hashed before and after a real
// dispatch and must not change; the assembled prompt lands at a NEW sibling whose path is the
// only line printed on stdout.
//
// regression: #2749
func TestPromptFileInputUntouched(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "home"))

	dir := t.TempDir()
	promptFile := filepath.Join(dir, "task.md")
	original := []byte("my own task file\n")
	if err := os.WriteFile(promptFile, original, 0o600); err != nil {
		t.Fatal(err)
	}
	before := sha256.Sum256(original)

	out, errOut, rc := ddRunCapture(t, []string{"example-stream--07", "--root", root, "--kit", "worker",
		"--tier", "strong", "--quiet", "--prompt-file", promptFile})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d:\n%s", rc, errOut)
	}

	after, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(after) != before || !bytes.Equal(after, original) {
		t.Fatalf("the caller's --prompt-file was modified (%d bytes now, %d before)", len(after), len(original))
	}

	want := filepath.Join(dir, "task.emitted.md")
	if got := strings.TrimSpace(out); got != want {
		t.Errorf("stdout = %q, want exactly the emitted path %q", got, want)
	}
	emitted, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("the emitted prompt is not at the printed path: %v", err)
	}
	if !strings.Contains(string(emitted), "every file operation stays under it") {
		t.Errorf("the emitted file does not hold the assembled prompt:\n%.200s", emitted)
	}
	if fi, _ := os.Stat(want); fi == nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("emitted file mode = %v, want 0600", fi)
	}
}

// emittedPathFor never aliases its input, whatever the spelling of the flag.
//
// regression: #2749
func TestEmittedNeverAliasesInput(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"p.md", "p", "p.emitted", "p.emitted.md", "p.packet.md", "a.b.c", ".md"} {
		in := filepath.Join(dir, name)
		got := emittedPathFor(in)
		if got == in {
			t.Errorf("emittedPathFor(%q) == its input", in)
		}
		if got == strings.TrimSuffix(in, filepath.Ext(in))+packetSuffix {
			t.Errorf("emittedPathFor(%q) collides with the packet path %q", in, got)
		}
		if !strings.HasSuffix(got, emittedSuffix) {
			t.Errorf("emittedPathFor(%q) = %q, missing %s", in, got, emittedSuffix)
		}
	}
}
