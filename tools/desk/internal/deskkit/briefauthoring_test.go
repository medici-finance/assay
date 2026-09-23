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

		// A delivery: docs/streams AND code. One non-docs path makes it a delivery.
		{"docs plus code is a delivery", "example-port/11", append(append([]ChangedFile{}, authoring...),
			cfModified("tools/desk/cmd/example/main.go")), false},
		// A delivery of a brief whose deliverable is a doc under docs/streams: it touches only
		// docs/streams, but it never ADDS the brief's own file.
		{"docs-only delivery that does not add the brief", "example-port/02", []ChangedFile{
			cfModified("docs/streams/example-port/README.md"),
			cfModified("docs/streams/example-port/brief-02-portability-audit.md"),
			cfAdded("docs/streams/example-port/portability-audit.md"),
		}, false},
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
			ChangedFile{Filename: "docs/streams/example-port/notes.go", PreviousFilename: "tools/desk/notes.go", Status: "renamed"}), false},
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
