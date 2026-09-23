package deskkit

import "testing"

// The stream slugs below are synthetic (example-*).

func cfAdded(p string) ChangedFile    { return ChangedFile{Filename: p, Status: "added"} }
func cfModified(p string) ChangedFile { return ChangedFile{Filename: p, Status: "modified"} }

func TestBriefAuthoringOnly(t *testing.T) {
	// The shape of a real briefs-authoring PR: a changelog fragment, the stream board README, and
	// the new brief files, nothing else.
	authoring := []ChangedFile{
		cfAdded("changelog/example-port-11-14-briefs.md"),
		cfModified("docs/streams/example-port/README.md"),
		cfAdded("docs/streams/example-port/brief-11-portable-pollers.md"),
		cfAdded("docs/streams/example-port/brief-12-deposix-prose.md"),
	}
	cases := []struct {
		name    string
		briefID string
		files   []ChangedFile
		want    bool
	}{
		{"authoring PR adds the brief it names", "example-port/11", authoring, true},
		{"the same PR authored a sibling brief too", "example-port/12", authoring, true},
		{"colon-form brief id", "example-repo:example-port:11", authoring, true},
		{"mixed-case brief id", "Example-Port/11", authoring, true},
		{"authoring that also touches another stream's board and brief", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfModified("docs/streams/example-other/README.md"), cfModified("docs/streams/example-other/brief-04-x.md")), true},

		// A delivery: docs/streams AND code. One non-docs path makes it a delivery.
		{"docs plus code is a delivery", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfModified("tools/desk/cmd/example/main.go")), false},
		// A delivery of a brief whose deliverable is a doc under docs/streams: it touches only
		// docs/streams, does not add the brief's own file, and adds a document that is not a brief.
		{"docs-only delivery that does not add the brief", "example-port/02", []ChangedFile{
			cfModified("docs/streams/example-port/README.md"),
			cfModified("docs/streams/example-port/brief-02-portability-audit.md"),
			cfAdded("docs/streams/example-port/portability-audit.md"),
		}, false},
		// Authored AND delivered in one PR, where the deliverable is a doc under docs/streams: it
		// adds the brief's own file, but it also adds a stream document that is neither the board
		// README nor a brief, so it is a delivery.
		{"author-and-deliver of a docs-only brief", "example-port/02", []ChangedFile{
			cfAdded("changelog/example-port-02.md"),
			cfModified("docs/streams/example-port/README.md"),
			cfAdded("docs/streams/example-port/brief-02-portability-audit.md"),
			cfAdded("docs/streams/example-port/portability-audit.md"),
		}, false},
		{"author-and-deliver of a decision record", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfAdded("docs/streams/decisions/DR-example.md")), false},
		{"a stream file nested below the stream dir", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfAdded("docs/streams/example-port/brief-11-x/notes.md")), false},
		{"a top-level docs/streams file", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfModified("docs/streams/README.md")), false},
		{"a brief-named file with no number", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfAdded("docs/streams/example-port/brief-draft-notes.md")), false},
		// It adds a DIFFERENT brief's file, so it did not author this one.
		{"adds only a different brief", "example-port/13", authoring, false},
		{"brief-110 is not brief-11", "example-port/11", []ChangedFile{
			cfAdded("docs/streams/example-port/brief-110-other.md"),
		}, false},
		{"brief file under another stream", "example-port/11", []ChangedFile{
			cfAdded("docs/streams/example-other/brief-11-x.md"),
		}, false},
		{"brief file only modified, not added", "example-port/11", []ChangedFile{
			cfModified("docs/streams/example-port/brief-11-portable-pollers.md"),
		}, false},
		// Renames are judged on BOTH halves: moving code into docs/streams still touches code.
		{"rename of a code file into docs/streams", "example-port/11", append(append([]ChangedFile{}, authoring...),
			ChangedFile{Filename: "docs/streams/example-port/brief-13-notes.md", PreviousFilename: "tools/desk/notes.go", Status: "renamed"}), false},
		{"changelog README is not a fragment", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfModified("changelog/README.md")), false},
		{"nested changelog path is not a fragment", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfAdded("changelog/sub/x.md")), false},
		{"non-markdown changelog file", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfAdded("changelog/x.sh")), false},
		{"dot-dot escape out of docs/streams", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfModified("docs/streams/../../tools/x.go")), false},
		{"CHANGELOG.md at the root is not a fragment", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfModified("CHANGELOG.md")), false},
		{"empty file list", "example-port/11", nil, false},
		{"unparseable brief id", "not-a-brief", authoring, false},
	}
	for _, c := range cases {
		if got := BriefAuthoringOnly(c.briefID, c.files); got != c.want {
			t.Errorf("%s: BriefAuthoringOnly(%q) = %v, want %v", c.name, c.briefID, got, c.want)
		}
	}
}
