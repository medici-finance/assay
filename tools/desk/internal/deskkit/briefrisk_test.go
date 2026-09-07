package deskkit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitBriefRef(t *testing.T) {
	cases := []struct {
		in                 string
		wantStream, wantNN string
		wantOK             bool
	}{
		{"example-stream/15", "example-stream", "15", true},
		{"example-stream:15", "example-stream", "15", true},
		{"medici-finance/assay:example-stream:15", "example-stream", "15", true},
		{"cell:medici-finance/assay:example-stream:15", "example-stream", "15", true},
		{"  example-stream/15  ", "example-stream", "15", true},
		{"example-stream/15/extra", "", "", false}, // too many slash parts
		{"example-stream/", "", "", false},         // empty NN
		{"/15", "", "", false},                     // empty stream
		{"example-stream/abc", "", "", false},      // non-numeric NN
		{"example-stream", "", "", false},          // no separator
		{"", "", "", false},
	}
	for _, c := range cases {
		gotStream, gotNN, gotOK := splitBriefRef(c.in)
		if gotOK != c.wantOK || gotStream != c.wantStream || gotNN != c.wantNN {
			t.Errorf("splitBriefRef(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.in, gotStream, gotNN, gotOK, c.wantStream, c.wantNN, c.wantOK)
		}
	}
}

func TestBriefFrontmatterRisk(t *testing.T) {
	cases := []struct {
		name         string
		content      string
		want         bool
		wantHasFence bool
	}{
		{
			name:         "gate human",
			content:      "---\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\nbody",
			want:         true,
			wantHasFence: true,
		},
		{
			name:         "sensitive-data yes in flow map",
			content:      "---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}\n---\nbody",
			want:         true,
			wantHasFence: true,
		},
		{
			name:         "regulatory yes per-line",
			content:      "---\ngate: model\nregulatory: yes\n---\nbody",
			want:         true,
			wantHasFence: true,
		},
		{
			name:         "no gate no risk",
			content:      "---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\nbody",
			want:         false,
			wantHasFence: true,
		},
		{
			name:         "no frontmatter fence reports unparseable",
			content:      "gate: human\nsensitive-data: yes\n",
			want:         false,
			wantHasFence: false,
		},
		{
			name:         "sensitive-data yes only in prose is not classed",
			content:      "---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\nThis touches sensitive-data: yes areas in prose.",
			want:         false,
			wantHasFence: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, reason, hasFence := briefFrontmatterRisk(c.content)
			if got != c.want {
				t.Fatalf("briefFrontmatterRisk classed = %v (%q), want %v", got, reason, c.want)
			}
			if hasFence != c.wantHasFence {
				t.Fatalf("briefFrontmatterRisk hasFrontmatter = %v, want %v", hasFence, c.wantHasFence)
			}
			if got && reason == "" {
				t.Fatalf("classed but no reason given")
			}
		})
	}
}

// plantBrief writes a brief file under root/docs/streams/<stream>/ and returns the root.
func plantBrief(t *testing.T, stream, nn, content string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", stream)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-"+nn+"-thing.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBriefRiskFromBody(t *testing.T) {
	const repo = "medici-finance/assay"
	humanGated := "---\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}\n---\nbody"

	t.Run("slash form resolves and risk-classes", func(t *testing.T) {
		root := plantBrief(t, "example-stream", "15", humanGated)
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "Some description.\n\nBrief: example-stream/15\n")
		if br.OwningBrief != "example-stream/15" {
			t.Fatalf("OwningBrief = %q, want example-stream/15", br.OwningBrief)
		}
		if !br.Resolved {
			t.Fatalf("Resolved = false, want true")
		}
		if !br.RiskClassed {
			t.Fatalf("RiskClassed = false, want true (%q)", br.Reason)
		}
	})

	t.Run("colon form resolves and risk-classes", func(t *testing.T) {
		root := plantBrief(t, "example-stream", "15", humanGated)
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "Brief: example-stream:15\n")
		if br.OwningBrief != "example-stream/15" {
			t.Fatalf("OwningBrief = %q, want example-stream/15", br.OwningBrief)
		}
		if !br.RiskClassed {
			t.Fatalf("RiskClassed = false, want true")
		}
	})

	t.Run("non-risk brief resolves but does not classify", func(t *testing.T) {
		content := "---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\nbody"
		root := plantBrief(t, "example-stream", "15", content)
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "Brief: example-stream/15\n")
		if br.OwningBrief != "example-stream/15" || !br.Resolved {
			t.Fatalf("want resolved owning brief; got %+v", br)
		}
		if br.RiskClassed {
			t.Fatalf("RiskClassed = true, want false for a non-risk brief")
		}
	})

	t.Run("no trailer yields empty owning brief", func(t *testing.T) {
		root := plantBrief(t, "example-stream", "15", humanGated)
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "A body with no link trailer.\n")
		if br.OwningBrief != "" || br.RiskClassed {
			t.Fatalf("want empty non-classed result; got %+v", br)
		}
	})

	t.Run("issue trailer is not a brief", func(t *testing.T) {
		root := plantBrief(t, "example-stream", "15", humanGated)
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "Issue: #559\n")
		if br.OwningBrief != "" || br.RiskClassed {
			t.Fatalf("want empty non-classed result for an Issue: trailer; got %+v", br)
		}
	})

	// ---- fail-safe: a DECLARED brief that cannot be resolved/read/parsed is UNVERIFIABLE,
	// so it risk-classes (fail closed). Each error path is its own case; against the pre-fix
	// code each returned RiskClassed=false (fail-open, it would flip).

	t.Run("present trailer + glob no-match is unverifiable-risk", func(t *testing.T) {
		// Root configured, but no brief file for the named NN.
		root := plantBrief(t, "example-stream", "15", humanGated)
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "Brief: example-stream/99\n")
		if br.OwningBrief != "example-stream/99" {
			t.Fatalf("OwningBrief = %q, want example-stream/99", br.OwningBrief)
		}
		if !br.RiskClassed || !br.Unverifiable || br.Resolved {
			t.Fatalf("a declared-but-unresolvable brief must be risk-classed+unverifiable; got %+v", br)
		}
	})

	t.Run("present trailer + RootForRepo empty is unverifiable-risk", func(t *testing.T) {
		// A malformed DESK_ROOTS makes ConfiguredRoots refuse, so RootForRepo yields "" for the
		// target repo — the "no configured root" path.
		t.Setenv(RootsEnv, "this-is-not-a-valid-roots-spec")
		if RootForRepo(repo) != "" {
			t.Skip("target repo unexpectedly has a configured root")
		}
		br := BriefRiskFromBody(repo, "Brief: example-stream/15\n")
		if br.OwningBrief != "example-stream/15" || !br.RiskClassed || !br.Unverifiable {
			t.Fatalf("no configured root for a declared brief must be unverifiable-risk; got %+v", br)
		}
	})

	t.Run("present trailer + unreadable file is unverifiable-risk", func(t *testing.T) {
		// The brief path resolves to a DIRECTORY, so ReadFile errors.
		root := t.TempDir()
		dir := filepath.Join(root, "docs", "streams", "example-stream")
		if err := os.MkdirAll(filepath.Join(dir, "brief-15-thing.md"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "Brief: example-stream/15\n")
		if !br.RiskClassed || !br.Unverifiable || br.Resolved {
			t.Fatalf("an unreadable brief file must be unverifiable-risk; got %+v", br)
		}
	})

	t.Run("present trailer + splitBriefRef failure is unverifiable-risk", func(t *testing.T) {
		root := plantBrief(t, "example-stream", "15", humanGated)
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "Brief: not-a-valid-ref\n")
		if !br.RiskClassed || !br.Unverifiable {
			t.Fatalf("a malformed Brief: value must be unverifiable-risk; got %+v", br)
		}
	})

	t.Run("present trailer + unparseable frontmatter is unverifiable-risk", func(t *testing.T) {
		// Brief file resolves and reads, but carries no `---` frontmatter fence.
		root := plantBrief(t, "example-stream", "15", "no frontmatter here\ngate: human\n")
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "Brief: example-stream/15\n")
		if br.OwningBrief != "example-stream/15" || !br.RiskClassed || !br.Unverifiable {
			t.Fatalf("a brief with no parseable frontmatter must be unverifiable-risk; got %+v", br)
		}
	})

	t.Run("duplicate Brief trailer is unverifiable-risk", func(t *testing.T) {
		root := plantBrief(t, "example-stream", "15", humanGated)
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "Brief: example-stream/15\nBrief: example-stream/16\n")
		if !br.RiskClassed || !br.Unverifiable {
			t.Fatalf("a malformed (duplicate Brief) trailer set must be unverifiable-risk; got %+v", br)
		}
	})

	t.Run("duplicate Issue trailer (no brief) makes no risk claim", func(t *testing.T) {
		// A malformed trailer set that implicates NO Brief: is the trailer-absent case.
		root := plantBrief(t, "example-stream", "15", humanGated)
		t.Setenv(RootsEnv, repo+"="+root)
		br := BriefRiskFromBody(repo, "Issue: #1\nIssue: #2\n")
		if br.RiskClassed || br.OwningBrief != "" {
			t.Fatalf("a duplicate Issue: trailer declares no brief and must make no risk claim; got %+v", br)
		}
	})
}
