package writepolicy

import "testing"

func fixtureInput(tool, target string) Evaluation {
	cl := Classify(Request{Tool: tool, Target: target}, LexicalContext{Platform: POSIX, Cwd: "/owned", Home: "/fixture-home", Env: map[string]string{}})
	return Evaluation{Classified: cl, ClassificationID: cl.ID(), Policy: Policy{Profile: LegacyHook,
		SharedRoot: "/shared", ProjectDir: "/owned", CwdTop: "/owned", SentinelPath: "/claim"},
		Claim: ClaimObservation{Source: NoClaim}, Callout: CalloutObservation{State: CalloutUnconfigured}}
}

// completeFixture supplies explicitly stipulated filesystem facts, never disk
// observations. Expected decisions below are independent of the classifier.
func completeFixture(t *testing.T, in Evaluation, aliases map[string]string) Outcome {
	t.Helper()
	for round := 0; round < 30; round++ {
		out := Evaluate(in)
		if len(out.Need.Paths) == 0 {
			return out
		}
		for _, req := range out.Need.Paths {
			resolved := req.Input
			if v, ok := aliases[req.Input]; ok {
				resolved = v
			}
			in.Paths = append(in.Paths, PathObservation{Input: req.Input, State: PathResolved,
				Resolved: resolved, Ancestor: req.Input, ResolvedAncestor: resolved})
		}
	}
	t.Fatal("observation requests failed to settle")
	return Outcome{}
}

func reclassify(in Evaluation, cwd string) Evaluation {
	lex := in.Classified.lex
	lex.Cwd = cwd
	in.Classified = Classify(in.Classified.request, lex)
	in.ClassificationID = in.Classified.ID()
	return in
}

func managedInput(in Evaluation) Evaluation {
	in.Policy.Profile = Managed
	b := Binding{Allocation: "allocation-a", Generation: "generation-1", Repository: "repo-a", Owner: "worker-a", Session: "session-a"}
	in.Context = ContextObservation{Expected: b, Observed: b, Validated: true, ProducerAPI: APIVersion, ObservedAt: 100, ValidUntil: 200}
	return in
}

// regression: #3752
func TestWritePolicyCases(t *testing.T) {
	cases := []struct {
		name, tool, target string
		decision           Decision
		reason             ReasonCode
	}{
		{"shared-file", "Write", "/shared/new", Deny, SharedDenied},
		{"owned-file", "Write", "/owned/new", Allow, NoObjection},
		{"sentinel-file", "Edit", "/claim", Deny, SentinelDenied},
		{"carveout", "Write", "/shared/.claude/worktrees/w/new", Allow, NoObjection},
		{"carveout-traversal", "Write", "/shared/.claude/worktrees/../../new", Deny, SharedDenied},
		{"shared-shell", "Bash", "echo x > /shared/new", Deny, SharedDenied},
		{"spaceless-redirect-shared", "Bash", "echo x>/shared/new", Deny, SharedDenied},
		{"spaceless-append-shared", "Bash", "echo x>>/shared/new", Deny, SharedDenied},
		{"spaceless-redirect-owned", "Bash", "echo x>/owned/new", Allow, NoObjection},
		{"owned-shell", "Bash", "echo x > /owned/new", Allow, NoObjection},
		{"literal", "Bash", "printf '%s' 'rm /shared/new'", Allow, NoObjection},
		{"heredoc-literal", "Bash", "cat <<'END'\nrm /shared/new\nEND", Allow, NoObjection},
		{"substitution", "Bash", "echo $(touch /claim)", Deny, SentinelDenied},
		{"read-mention", "Bash", "cat /shared/new && echo x > /owned/new", Allow, NoObjection},
		{"compound", "Bash", "echo x > /owned/new; echo y >> /shared/new", Deny, SharedDenied},
		{"cd-shared", "Bash", "cd /shared && cat README.md", Deny, SharedDenied},
		{"find-delete", "Bash", "find /shared -delete", Deny, SharedDenied},
		{"copy-source", "Bash", "cp /shared/new /owned/copy", Allow, NoObjection},
		{"copy-destination", "Bash", "cp /owned/new /shared/copy", Deny, SharedDenied},
		{"legacy-unknown-outside", "Bash", "rm $UNKNOWN/file", Allow, NoObjection},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := fixtureInput(tc.tool, tc.target)
			got := completeFixture(t, in, nil)
			if got.Decision != tc.decision || got.Reason != tc.reason || got.Revision != APIVersion {
				t.Fatalf("got %+v; want %s/%s", got, tc.decision, tc.reason)
			}
			if again := completeFixture(t, in, nil); again.Decision != got.Decision || again.Reason != got.Reason || again.Detail != got.Detail {
				t.Fatal("same observations changed the answer")
			}
		})
	}
	t.Run("symlink-and-new-ancestor", func(t *testing.T) {
		in := fixtureInput("Write", "/alias/new")
		if got := completeFixture(t, in, map[string]string{"/alias/new": "/shared/new"}); got.Decision != Deny {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("chained-cwd-uses-observed-intermediate", func(t *testing.T) {
		in := fixtureInput("Bash", "cd /alias && cd child && echo /shared > file")
		// The first cwd resolves outside; its child resolves into shared. The
		// observation key proves the second cwd depends on the observed first.
		aliases := map[string]string{"/alias": "/owned/real", "/owned/real/child": "/shared/sub"}
		if got := completeFixture(t, in, aliases); got.Decision != Deny {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("sentinel-before-exemption", func(t *testing.T) {
		in := reclassify(fixtureInput("Write", "/claim"), "/shared")
		in.Policy.ProjectDir = "/shared"
		in.Policy.CwdTop = "/shared"
		in.Claim = ClaimObservation{Source: EnvironmentClaim, Token: "yes"}
		if got := completeFixture(t, in, nil); got.Reason != SentinelDenied {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("claim-expiry-scope-and-alias", func(t *testing.T) {
		for _, tc := range []struct {
			name, body string
			now, mtime int64
			want       ReasonCode
		}{
			{"inclusive", "/alias", 200, 100, Exempt}, {"expired", "", 201, 100, SharedDenied},
			{"future-legacy", "", 100, 200, Exempt}, {"comment-only", " # comment\n", 100, 100, Exempt},
			{"foreign-scope", "/elsewhere", 100, 100, SharedDenied},
		} {
			t.Run(tc.name, func(t *testing.T) {
				in := reclassify(fixtureInput("Write", "/shared/new"), "/shared")
				in.Policy.ProjectDir = "/shared"
				in.Policy.CwdTop = "/shared"
				in.Claim = ClaimObservation{Source: SentinelClaim, Readable: true, Body: tc.body, ObservedAt: tc.now, ModifiedAt: tc.mtime, TTL: 100}
				if got := completeFixture(t, in, map[string]string{"/alias": "/shared"}); got.Reason != tc.want {
					t.Fatalf("%+v", got)
				}
			})
		}
	})
	t.Run("callout-only-adds-refusals", func(t *testing.T) {
		in := fixtureInput("Bash", "echo x > /shared/new")
		in.Callout.State = CalloutAllow
		if got := completeFixture(t, in, nil); got.Decision != Deny || got.Reason != SharedDenied || got.Need.Callout {
			t.Fatalf("%+v", got)
		}
		in = reclassify(fixtureInput("Bash", "frobnicate"), "/shared")
		in.Callout.State = CalloutPending
		if got := completeFixture(t, in, nil); got.Decision != Unknown || !got.Need.Callout {
			t.Fatalf("%+v", got)
		}
		for _, s := range []CalloutState{CalloutRefuse, CalloutFailure} {
			in.Callout.State = s
			if got := completeFixture(t, in, nil); got.Reason != CalloutDenied {
				t.Fatalf("%+v", got)
			}
		}
	})
	t.Run("managed-never-inherits-legacy-fallback", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			change func(*Evaluation)
		}{
			{"missing-binding", func(in *Evaluation) { in.Context = ContextObservation{} }},
			{"foreign-generation", func(in *Evaluation) { in.Context.Observed.Generation = "generation-2" }},
			{"foreign-session", func(in *Evaluation) { in.Context.Observed.Session = "other" }},
			{"stale", func(in *Evaluation) { in.Context.ValidUntil = 99 }},
			{"missing-cwd", func(in *Evaluation) { *in = reclassify(*in, "") }},
			{"missing-git", func(in *Evaluation) { in.Policy.CwdTop = "" }},
			{"empty-target", func(in *Evaluation) {
				in.Classified = Classify(Request{Tool: "Write"}, in.Classified.lex)
				in.ClassificationID = in.Classified.ID()
			}},
			{"wrong-producer", func(in *Evaluation) { in.Context.ProducerAPI = "other" }},
			{"agent-exemption", func(in *Evaluation) { in.Claim.Source = AgentClaim }},
			{"legacy-env-exemption", func(in *Evaluation) { in.Claim = ClaimObservation{Source: EnvironmentClaim, Token: "yes"} }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				in := managedInput(fixtureInput("Write", "/owned/new"))
				tc.change(&in)
				if got := completeFixture(t, in, nil); got.Decision != Unknown || got.Reason != ManagedHold {
					t.Fatalf("%+v", got)
				}
			})
		}
		in := managedInput(fixtureInput("Bash", "rm $UNKNOWN/file"))
		if got := completeFixture(t, in, nil); got.Decision != Unknown || got.Reason != ManagedHold {
			t.Fatalf("unknown outside shared inherited allow: %+v", got)
		}
	})
	t.Run("claim-body-observation-order", func(t *testing.T) {
		in := reclassify(fixtureInput("Write", "/shared/new"), "/shared")
		in.Policy.ProjectDir, in.Policy.CwdTop = "/shared", "/shared"
		in.Claim = ClaimObservation{Source: SentinelClaim, BodyPending: true, ObservedAt: 201, ModifiedAt: 100, TTL: 100}
		if got := completeFixture(t, in, nil); got.Need.ClaimBody || got.Reason != SharedDenied {
			t.Fatalf("expired claim read body: %+v", got)
		}
		in.Claim.ObservedAt = 200
		if got := completeFixture(t, in, nil); !got.Need.ClaimBody || got.Decision != Unknown {
			t.Fatalf("live unread body: %+v", got)
		}
		in.Claim.BodyPending, in.Claim.Readable = false, true
		if got := completeFixture(t, in, nil); got.Reason != Exempt {
			t.Fatalf("empty body scope: %+v", got)
		}
		in.Claim.Directory, in.Claim.BodyPending = true, true
		if got := completeFixture(t, in, nil); got.Need.ClaimBody || got.Reason != SharedDenied {
			t.Fatalf("directory body read: %+v", got)
		}
	})
	t.Run("observation-validation", func(t *testing.T) {
		in := fixtureInput("Write", "/shared/new")
		if got := Evaluate(in); got.Decision != Unknown || len(got.Need.Paths) == 0 {
			t.Fatalf("missing observation allowed: %+v", got)
		}
		in.ClassificationID = "foreign"
		if got := Evaluate(in); got.Reason != InvalidObservation {
			t.Fatalf("%+v", got)
		}
		in.ClassificationID = in.Classified.ID()
		bad := PathObservation{Input: "/shared/new", State: PathResolved, Resolved: "/owned/new", Ancestor: "/shared", ResolvedAncestor: "/shared", Suffix: "new"}
		in.Paths = []PathObservation{bad}
		if got := Evaluate(in); got.Reason != InvalidObservation {
			t.Fatalf("inconsistent pieces: %+v", got)
		}
		bad = PathObservation{Input: "/shared/new", State: PathFailed}
		in.Paths = []PathObservation{bad, bad}
		if got := Evaluate(in); got.Reason != InvalidObservation {
			t.Fatalf("duplicate: %+v", got)
		}
	})
	t.Run("windows-lexical-files", func(t *testing.T) {
		for _, tc := range []struct {
			target string
			want   Decision
		}{
			{`C:\shared\new`, Deny}, {`C:\owned\new`, Allow}, {`C:\shared\.claude\worktrees\w\new`, Allow},
			{`C:\shared\.claude\worktrees\..\..\new`, Deny},
		} {
			cl := Classify(Request{Tool: "Write", Target: tc.target}, LexicalContext{Platform: Windows, Cwd: `C:\owned`, Home: `C:\fixture-home`})
			in := Evaluation{Classified: cl, ClassificationID: cl.ID(), Claim: ClaimObservation{Source: NoClaim}, Policy: Policy{Profile: LegacyHook, SharedRoot: `C:\shared`, ProjectDir: `C:\owned`}, Callout: CalloutObservation{State: CalloutUnconfigured}}
			if got := completeFixture(t, in, nil); got.Decision != tc.want {
				t.Fatalf("%s: %+v", tc.target, got)
			}
		}
	})
}

// regression: #3752
func TestClassificationOwnsItsSnapshot(t *testing.T) {
	env := map[string]string{"D": "/shared"}
	cl := Classify(Request{Tool: "Bash", Target: "echo x > $D/new"}, LexicalContext{Platform: POSIX, Cwd: "/shared", Env: env})
	env["D"] = "/owned"
	tokens := cl.PathTokens()
	if len(tokens) > 0 {
		tokens[0] = "/owned"
	}
	in := fixtureInput("Bash", "")
	in.Classified = cl
	in.ClassificationID = cl.ID()
	if got := completeFixture(t, in, nil); got.Decision != Deny {
		t.Fatalf("caller changed the snapshot: %+v", got)
	}
}
