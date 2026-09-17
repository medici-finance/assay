package main

import (
	"strings"
	"testing"
)

// actingidentity_test.go — forge-neutral/18 row 14.
//
// forge-neutral/07 gave statusgen two acting-identity controls: the Evidence-actor check
// (evidenceactor.go), which decides whose commit backs a Verify row, and the verifyrun witness
// (verifyrun.go's executingRunner), which records who ran the rows. Both are read purely from
// git (blame/commit identity) and the roster config — NEITHER reaches a forge CLI, and
// forge-neutral/18 does not touch either file. This is a single named regression proof, per
// row 14's exact command, that moving the OTHER reads (issue lists, the attribution walk, the
// brief-parse memo, --changed-only, the history-load memo) left these two byte-identical: same
// verifier binding, same accept/reject verdicts, same runner-derivation shapes.
//
// It is deliberately a thin re-assertion of golden values already covered in depth by
// evidenceactor_test.go and verifyrun_test.go — this test's job is to be the ONE place row 14's
// command name resolves to, not to duplicate their coverage.
func TestActingIdentityUnchanged(t *testing.T) {
	t.Run("evidence-actor policy binds the fixture roster's verifier unchanged", func(t *testing.T) {
		p := evidenceActorPolicyFromRoster()
		if p.Unavailable != "" {
			t.Fatalf("policy unavailable: %q", p.Unavailable)
		}
		if p.Verifier.Login != "assay-verifier-app" || p.Verifier.ID != 300000005 {
			t.Fatalf("verifier ref = %+v, want assay-verifier-app:300000005 (forge-neutral/07's binding, unmoved)", p.Verifier)
		}
		if !p.idPinned() {
			t.Fatal("the fixture binding carries an id; matching must stay id-pinned")
		}
	})

	t.Run("evidence-actor classify verdicts are unchanged", func(t *testing.T) {
		p := evidenceActorPolicy{
			Verifier: actorRef{Login: "assay-verifier-app", ID: 300000005},
			Humans:   []actorRef{{Login: "ada", ID: 100001}},
		}
		cases := []struct {
			name, email string
			want        actorVerdict
		}{
			{fixtureVerifierName, fixtureVerifierEmail, actorVerifier},
			{fixtureHumanName, fixtureHumanEmail, actorHuman},
			{fixtureWorkerName, fixtureWorkerEmail, actorRejected},
			{fixtureVerifierName, "999+assay-verifier-app[bot]@users.noreply.github.com", actorImpostor},
		}
		for _, c := range cases {
			got, reason := p.classify(c.name, c.email)
			if got != c.want {
				t.Errorf("classify(%q, %q) = %v, want %v (reason: %s)", c.name, c.email, got, c.want, reason)
			}
		}
	})

	t.Run("executingRunner prefers the Actions identity, verbatim", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("GITHUB_ACTOR", "assay-reviewer-app[bot]")
		got, _, ok := executingRunner(t.TempDir())
		if !ok || got != "assay-reviewer-app[bot]" {
			t.Fatalf("runner = (%q, %v), want the App slug verbatim", got, ok)
		}
	})

	t.Run("executingRunner falls back to the git identity as human:<login>", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("GITHUB_ACTOR", "")
		root := t.TempDir()
		gitInit(t, root, "Alex Rivera", "alex@example.com")
		got, _, ok := executingRunner(root)
		if !ok || got != "human:alex" {
			t.Fatalf("runner = (%q, %v), want human:alex", got, ok)
		}
		if !strings.HasPrefix(got, "human:") {
			t.Error("a human commit identity must render as human:<login>")
		}
	})

	t.Run("executingRunner fails closed with no derivable identity", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("GITHUB_ACTOR", "")
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("GIT_CONFIG_GLOBAL", t.TempDir()+"/absent")
		t.Setenv("GIT_CONFIG_SYSTEM", t.TempDir()+"/absent")
		root := t.TempDir()
		runGit(t, root, "init")
		if got, _, ok := executingRunner(root); ok {
			t.Fatalf("runner = %q, want a refusal — this control must never attribute a witness to nobody", got)
		}
	})
}
