package deskkit

import (
	"strings"
	"testing"
)

// forge_hardening_kinds_test.go — the per-forge PARTITION of op 40's kind vocabulary (the
// forge-gitlab GitLab-hardening-reads brief).
//
// The vocabulary is one closed set; each kind is served by exactly one forge and refused BY
// NAME by the other. These tests pin the partition from the enum side (every kind has a
// forge, the two halves are disjoint and exhaustive, the preflight document is chosen from the
// resolved forge) — the goldens `hardening_read_github_kind_refused` (GitLab) and
// `hardening_read_gitlab_kind_refused` (GitHub) pin the zero-request refusal from the wire
// side.

func TestHardeningKindsPartitionByForge(t *testing.T) {
	all := HardeningReadKinds()
	if len(all) == 0 {
		t.Fatal("HardeningReadKinds is empty — a partition over nothing proves nothing")
	}
	gh := hardeningReadKindsFor(ForgeGitHub)
	gl := hardeningReadKindsFor(ForgeGitLab)
	if len(gh) == 0 || len(gl) == 0 {
		t.Fatalf("both halves must be non-empty: github=%v gitlab=%v", gh, gl)
	}
	seen := map[string]ForgeKind{}
	for _, k := range gh {
		seen[k] = ForgeGitHub
	}
	for _, k := range gl {
		if prev, dup := seen[k]; dup {
			t.Errorf("kind %q is served by both %s and gitlab — the halves must be disjoint", k, prev)
		}
		seen[k] = ForgeGitLab
	}
	for _, k := range all {
		f, ok := seen[k]
		if !ok {
			t.Errorf("kind %q is in the vocabulary but served by NO forge — a validated kind neither backend answers", k)
			continue
		}
		if got := HardeningReadKindForge(HardeningReadKind(k)); got != f {
			t.Errorf("HardeningReadKindForge(%q) = %q, want %q", k, got, f)
		}
	}
	if len(seen) != len(all) {
		t.Errorf("partition covers %d kinds, vocabulary has %d", len(seen), len(all))
	}
	// The five GitLab kinds the brief names, by name — so a renamed constant cannot quietly
	// drop one out of the GitLab half.
	for _, want := range []string{"project", "protected-branches", "protected-tags", "push-rules", "approvals"} {
		if seen[want] != ForgeGitLab {
			t.Errorf("kind %q is not in the gitlab half (%v)", want, gl)
		}
	}
	if HardeningReadKindForge(HardeningReadKind("not-a-real-kind")) != "" {
		t.Error("an unknown kind must report no forge")
	}
}

// The preflight document is read off the RESOLUTION ResolveForge hands back, never off a
// ForgeKind a caller supplies — the package's exported surface carries no forge selector.
func TestHardeningRepoDocumentKindPerForge(t *testing.T) {
	if k, err := (ForgeResolution{Kind: ForgeGitHub}).HardeningRepoDocumentKind(); err != nil || k != HardeningReadRepo {
		t.Fatalf("github preflight kind = %q, %v; want %q", k, err, HardeningReadRepo)
	}
	if k, err := (ForgeResolution{Kind: ForgeGitLab}).HardeningRepoDocumentKind(); err != nil || k != HardeningReadProject {
		t.Fatalf("gitlab preflight kind = %q, %v; want %q", k, err, HardeningReadProject)
	}
	k, err := (ForgeResolution{Kind: ForgeKind("bitkeeper")}).HardeningRepoDocumentKind()
	if err == nil || k != "" {
		t.Fatalf("an unknown forge must refuse, got kind %q err %v", k, err)
	}
	if !strings.Contains(err.Error(), "could-not-check") {
		t.Fatalf("the refusal must say could-not-check, got %q", err.Error())
	}
}

// Each backend refuses the OTHER forge's kinds by name, before any request exists, and the
// refusal names the forge that does serve the kind — so a checklist author who put a GitHub
// row in a GitLab checklist reads the answer, not a 404.
func TestHardeningKindRefusedByTheOtherForge(t *testing.T) {
	t.Run("gitlab_refuses_every_github_kind", func(t *testing.T) {
		s := newGLServer(t)
		f := s.forge()
		for _, k := range hardeningReadKindsFor(ForgeGitHub) {
			raw, err := f.RepoHardeningRead(glRepo, HardeningReadKind(k))
			if err == nil || raw != nil {
				t.Fatalf("gitlab answered github kind %q: raw=%s err=%v", k, raw, err)
			}
			if !strings.Contains(err.Error(), "could-not-check") || !strings.Contains(err.Error(), "gitlab serves no hardening read of kind") ||
				!strings.Contains(err.Error(), "it is a github kind") {
				t.Fatalf("refusal for %q must name gitlab, the kind and github: %q", k, err.Error())
			}
		}
		if n := len(s.requests); n != 0 {
			t.Fatalf("refusing by name must emit ZERO requests, got %d: %+v", n, s.requests)
		}
	})
	t.Run("github_refuses_every_gitlab_kind", func(t *testing.T) {
		s := newGoldenServer(t)
		f := s.forge()
		for _, k := range hardeningReadKindsFor(ForgeGitLab) {
			raw, err := f.RepoHardeningRead(forgeTestRepo, HardeningReadKind(k))
			if err == nil || raw != nil {
				t.Fatalf("github answered gitlab kind %q: raw=%s err=%v", k, raw, err)
			}
			if !strings.Contains(err.Error(), "could-not-check") || !strings.Contains(err.Error(), "github serves no hardening read of kind") ||
				!strings.Contains(err.Error(), "it is a gitlab kind") {
				t.Fatalf("refusal for %q must name github, the kind and gitlab: %q", k, err.Error())
			}
		}
		if n := len(s.requests); n != 0 {
			t.Fatalf("refusing by name must emit ZERO requests, got %d: %+v", n, s.requests)
		}
	})
}
