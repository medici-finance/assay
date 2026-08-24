package main

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	fdiff "github.com/go-git/go-git/v5/plumbing/format/diff"
)

// --- fakes implementing the go-git format/diff interfaces, so the three-state
// diff logic can be exercised deterministically without a git binary. ---

type fakeFile struct{ path string }

func (f fakeFile) Hash() plumbing.Hash     { return plumbing.ZeroHash }
func (f fakeFile) Mode() filemode.FileMode { return filemode.Regular }
func (f fakeFile) Path() string            { return f.path }

type fakeChunk struct {
	content string
	op      fdiff.Operation
}

func (c fakeChunk) Content() string       { return c.content }
func (c fakeChunk) Type() fdiff.Operation { return c.op }

type fakePatch struct {
	from, to fdiff.File
	binary   bool
	chunks   []fdiff.Chunk
}

func (p fakePatch) IsBinary() bool                  { return p.binary }
func (p fakePatch) Files() (fdiff.File, fdiff.File) { return p.from, p.to }
func (p fakePatch) Chunks() []fdiff.Chunk           { return p.chunks }

// TestDiffThreeStateDistinguishesZeroFromUnmeasured is Verify row 5: a text file
// changed by zero lines emits measured-zero; a binary/unreadable blob emits
// could-not-measure with a reason. The two are NEVER conflated — that is the
// whole point of the three-state instrument.
func TestDiffThreeStateDistinguishesZeroFromUnmeasured(t *testing.T) {
	const sha = "deadbeef"

	t.Run("text file changed by real lines is measured", func(t *testing.T) {
		fp := fakePatch{
			from: fakeFile{"a.txt"}, to: fakeFile{"a.txt"},
			chunks: []fdiff.Chunk{
				fakeChunk{"unchanged\n", fdiff.Equal},
				fakeChunk{"new line\n", fdiff.Add},
			},
		}
		fd := fileDiffFromPatch(sha, fp)
		if fd.Lines.State != StateMeasured {
			t.Fatalf("expected measured, got %q", fd.Lines.State)
		}
		if got := len(fd.Lines.Value); got != 1 {
			t.Fatalf("expected one hunk, got %d", got)
		}
		// The add line must be present and carry no premature classification.
		var sawAdd bool
		for _, lc := range fd.Lines.Value[0].Lines {
			if lc.Op == OpAdd {
				sawAdd = true
			}
			if lc.Class != "" {
				t.Errorf("skeleton must not classify lines; got class %q", lc.Class)
			}
		}
		if !sawAdd {
			t.Error("expected an add line change")
		}
	})

	t.Run("text file changed by zero lines is measured-zero", func(t *testing.T) {
		// A mode-only / rename-only change: non-binary, no add/del chunks.
		fp := fakePatch{
			from: fakeFile{"mode.txt"}, to: fakeFile{"mode.txt"},
			chunks: []fdiff.Chunk{
				fakeChunk{"identical\n", fdiff.Equal},
			},
		}
		fd := fileDiffFromPatch(sha, fp)
		if fd.Lines.State != StateMeasuredZero {
			t.Fatalf("expected measured-zero for a zero-line text change, got %q", fd.Lines.State)
		}
		if fd.Lines.Reason != "" {
			t.Errorf("measured-zero must carry no reason, got %q", fd.Lines.Reason)
		}
	})

	t.Run("binary blob is could-not-measure with a reason", func(t *testing.T) {
		fp := fakePatch{
			from: fakeFile{"logo.png"}, to: fakeFile{"logo.png"},
			binary: true,
		}
		fd := fileDiffFromPatch(sha, fp)
		if fd.Lines.State != StateCouldNotMeasure {
			t.Fatalf("expected could-not-measure for a binary blob, got %q", fd.Lines.State)
		}
		if fd.Lines.Reason == "" {
			t.Error("could-not-measure must name a reason")
		}
		if !fd.Binary {
			t.Error("binary flag should be set")
		}
	})

	t.Run("zero-line and binary are never the same state", func(t *testing.T) {
		zero := fileDiffFromPatch(sha, fakePatch{from: fakeFile{"z"}, to: fakeFile{"z"}})
		bin := fileDiffFromPatch(sha, fakePatch{from: fakeFile{"b"}, to: fakeFile{"b"}, binary: true})
		if zero.Lines.State == bin.Lines.State {
			t.Fatalf("a zero-line change and a binary blob must NOT share a state (%q)", zero.Lines.State)
		}
	})
}

// TestFileDiffKeyStableAndKind pins the reference key and kind derivation the
// commit table depends on.
func TestFileDiffKeyStableAndKind(t *testing.T) {
	add := fileDiffFromPatch("sha1", fakePatch{to: fakeFile{"new.go"}, chunks: []fdiff.Chunk{fakeChunk{"x\n", fdiff.Add}}})
	if add.Kind != ChangeAdded {
		t.Errorf("a from-nil change should be added, got %q", add.Kind)
	}
	if add.Key() != "sha1:new.go" {
		t.Errorf("unexpected key %q", add.Key())
	}
	del := fileDiffFromPatch("sha1", fakePatch{from: fakeFile{"old.go"}, chunks: []fdiff.Chunk{fakeChunk{"x\n", fdiff.Delete}}})
	if del.Kind != ChangeDeleted {
		t.Errorf("a to-nil change should be deleted, got %q", del.Kind)
	}
	if del.Key() != "sha1:old.go" {
		t.Errorf("a delete should key on the old path, got %q", del.Key())
	}
}
