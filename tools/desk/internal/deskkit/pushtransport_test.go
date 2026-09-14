package deskkit

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// configZ renders a `git config --list -z` payload from ordered key/value pairs, so a test
// states the config it means rather than a hand-escaped blob. Odd-length input is a test
// bug and panics.
func configZ(kv ...string) string {
	if len(kv)%2 != 0 {
		panic("configZ: want key/value pairs")
	}
	var b strings.Builder
	for i := 0; i < len(kv); i += 2 {
		b.WriteString(kv[i])
		b.WriteString("\n")
		b.WriteString(kv[i+1])
		b.WriteString("\x00")
	}
	return b.String()
}

func gateInput(t *testing.T, cfg string) PushTransportInput {
	t.Helper()
	return PushTransportInput{
		Tool: "deskpr", Verb: "create", Dir: "/w", Remote: "origin",
		ConfigZ: func() (string, error) { return cfg, nil },
	}
}

// TestPushGateRefusesSshUnderBot is the gate's reason for existing: a bot
// session whose resolved PUSH url is an SSH one is REFUSED, in every spelling git accepts.
//
// FAIL-FIRST: see pushtransport-mutations.json — the CONTROL mutant neuters the
// isSSHTransport branch in CheckPushTransport, and every case below goes red.
func TestPushGateRefusesSshUnderBot(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")

	cases := []struct {
		name string
		cfg  string
		want string // a fragment the remedy line must carry
	}{
		{
			name: "ssh:// scheme on remote.origin.url",
			cfg:  configZ("remote.origin.url", "ssh://git@example.com/example-org/tracker.git"),
			want: "https://example.com/example-org/tracker.git",
		},
		{
			name: "scp-like git@host:path on remote.origin.url",
			cfg:  configZ("remote.origin.url", "git@example.com:example-org/tracker.git"),
			want: "https://example.com/example-org/tracker.git",
		},
		{
			name: "ssh:// with an explicit port — the port is dropped from the https remedy",
			cfg:  configZ("remote.origin.url", "ssh://git@example.com:443/example-org/tracker.git"),
			want: "https://example.com/example-org/tracker.git",
		},
		{
			name: "git+ssh:// scheme",
			cfg:  configZ("remote.origin.url", "git+ssh://git@example.com/example-org/tracker.git"),
			want: "https://example.com/example-org/tracker.git",
		},
		{
			// The PUSH url is the one that matters: an https fetch url with an ssh pushurl
			// override is exactly the shape a reader who only looked at `remote -v`'s first
			// line would call clean.
			name: "https url but an ssh pushurl override",
			cfg: configZ(
				"remote.origin.url", "https://example.com/example-org/tracker.git",
				"remote.origin.pushurl", "git@example.com:example-org/tracker.git"),
			want: "remote.origin.pushurl",
		},
		{
			// A push fans out to EVERY pushurl, so a clean first value is not a pass.
			name: "two pushurls, only the second is ssh",
			cfg: configZ(
				"remote.origin.pushurl", "https://example.com/example-org/tracker.git",
				"remote.origin.pushurl", "ssh://git@example.com/example-org/tracker.git"),
			want: "ssh://git@example.com/example-org/tracker.git",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var errb bytes.Buffer
			in := gateInput(t, tc.cfg)
			in.Stderr = &errb
			err := CheckPushTransport(in)
			if err == nil {
				t.Fatalf("CheckPushTransport = nil, want a refusal")
			}
			if got := ExitCodeOf(err); got != ExitRefused {
				t.Fatalf("exit code = %d, want %d (refused)", got, ExitRefused)
			}
			msg := err.Error()
			for _, frag := range []string{"SSH transport", "worker App", tc.want, "remote set-url --push origin"} {
				if !strings.Contains(msg, frag) {
					t.Errorf("refusal does not name %q:\n%s", frag, msg)
				}
			}
		})
	}
}

// TestPushGateAllowsNonSSH pins the other half of the contract: the gate is about
// the PUSH transport and nothing else. Fetch-side SSH, https, file:// and a local path all
// pass.
func TestPushGateAllowsNonSSH(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")

	cases := []struct {
		name string
		cfg  string
	}{
		{
			// The load-bearing one: SSH is still a legitimate FETCH transport. Only the
			// pushurl override decides.
			name: "ssh fetch url with an https pushurl override",
			cfg: configZ(
				"remote.origin.url", "git@example.com:example-org/tracker.git",
				"remote.origin.pushurl", "https://example.com/example-org/tracker.git",
				"credential.helper", "!f(){ echo password=x; }; f"),
		},
		{
			name: "plain https",
			cfg: configZ(
				"remote.origin.url", "https://example.com/example-org/tracker.git",
				"credential.helper", "!f(){ echo password=x; }; f"),
		},
		{
			name: "file:// (every offline fixture)",
			cfg:  configZ("remote.origin.url", "file:///tmp/origin.git"),
		},
		{
			name: "a bare local path",
			cfg:  configZ("remote.origin.url", "/tmp/origin.git"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var errb bytes.Buffer
			in := gateInput(t, tc.cfg)
			in.Stderr = &errb
			if err := CheckPushTransport(in); err != nil {
				t.Fatalf("CheckPushTransport = %v, want nil", err)
			}
			if strings.Contains(errb.String(), "NOTICE") {
				t.Errorf("unexpected NOTICE:\n%s", errb.String())
			}
		})
	}
}

// TestPushGateInertWithoutBot: a human at a terminal, no $DESK_LOOP, an
// SSH remote — that is what an SSH remote is FOR, and the gate must not touch it.
func TestPushGateInertWithoutBot(t *testing.T) {
	t.Setenv("DESK_LOOP", "")
	var errb bytes.Buffer
	in := gateInput(t, configZ("remote.origin.url", "git@example.com:example-org/tracker.git"))
	in.Stderr = &errb
	if err := CheckPushTransport(in); err != nil {
		t.Fatalf("CheckPushTransport with $DESK_LOOP unset = %v, want nil (inert)", err)
	}
	if errb.Len() != 0 {
		t.Errorf("expected silence with no loop identity, got:\n%s", errb.String())
	}
}

// TestPushGateBadLoopNotices: a $DESK_LOOP nothing recognises means the
// bot question is could-not-check. It must SAY the gate did not run — the three-state rule's
// "never rounded up to a pass", and the difference between looked-and-found-nothing and
// never-looked.
func TestPushGateBadLoopNotices(t *testing.T) {
	t.Setenv("DESK_LOOP", "not-a-loop-name")
	var errb bytes.Buffer
	in := gateInput(t, configZ("remote.origin.url", "git@example.com:example-org/tracker.git"))
	in.Stderr = &errb
	if err := CheckPushTransport(in); err != nil {
		t.Fatalf("CheckPushTransport = %v, want nil (could-not-check is not a refusal here)", err)
	}
	got := errb.String()
	for _, frag := range []string{"NOTICE", "COULD NOT BE ESTABLISHED", "did NOT run", "never a pass"} {
		if !strings.Contains(got, frag) {
			t.Errorf("could-not-check NOTICE does not carry %q:\n%s", frag, got)
		}
	}
}

// TestPushGateHttpsNoAppHelper: https is the sanctioned transport,
// but https answered by a machine keychain is the same ambient identity one layer along.
// The evidence is weaker than an SSH url, so this is a NOTICE — it must never become a
// refusal, and it must never be silent.
func TestPushGateHttpsNoAppHelper(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")

	cases := []struct {
		name  string
		cfg   string
		shown string
	}{
		{
			name:  "no credential helper at all",
			cfg:   configZ("remote.origin.url", "https://example.com/example-org/tracker.git"),
			shown: "none configured",
		},
		{
			name: "only a machine-keychain helper",
			cfg: configZ(
				"remote.origin.url", "https://example.com/example-org/tracker.git",
				"credential.helper", "osxkeychain"),
			shown: "osxkeychain",
		},
		{
			// `credential.helper=` RESETS the list. It is not a helper, and must not be
			// rendered as one.
			name: "an explicit empty reset and nothing after it",
			cfg: configZ(
				"remote.origin.url", "https://example.com/example-org/tracker.git",
				"credential.helper", ""),
			shown: "credential.helper: none configured",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var errb bytes.Buffer
			in := gateInput(t, tc.cfg)
			in.Stderr = &errb
			if err := CheckPushTransport(in); err != nil {
				t.Fatalf("CheckPushTransport = %v, want nil (NOTICE, never a refusal)", err)
			}
			got := errb.String()
			if !strings.Contains(got, "NOTICE") || !strings.Contains(got, tc.shown) {
				t.Fatalf("expected a NOTICE naming %q, got:\n%s", tc.shown, got)
			}
			if !strings.Contains(got, "could-not-check, not a pass") {
				t.Errorf("NOTICE does not report itself as could-not-check:\n%s", got)
			}
		})
	}
}

// TestPushGateUrlScopedHelper: the house pattern is an inline
// helper, often url-scoped. A real App helper must not be nagged at.
func TestPushGateUrlScopedHelper(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	var errb bytes.Buffer
	in := gateInput(t, configZ(
		"remote.origin.url", "https://example.com/example-org/tracker.git",
		"credential.https://example.com.helper", "!f(){ echo password=x; }; f"))
	in.Stderr = &errb
	if err := CheckPushTransport(in); err != nil {
		t.Fatalf("CheckPushTransport = %v, want nil", err)
	}
	if strings.Contains(errb.String(), "NOTICE") {
		t.Errorf("a url-scoped App helper was noticed at:\n%s", errb.String())
	}
}

// TestPushGateCouldNotCheck: a config read that fails, and a remote
// with no url at all, are exit 6. Neither may read as "no SSH found, carry on".
func TestPushGateCouldNotCheck(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")

	// The message matters, not only the exit code: a failed READ and a remote with no url
	// are different diagnoses with the same exit code, and a gate that swallows the read
	// error falls through to the second one — same rc, wrong story, and the operator is sent
	// to configure a remote that is configured. (Caught by the mutation map's
	// "round a failed git-config read up to a pass" entry, which is INVISIBLE to an
	// exit-code-only assertion.)
	t.Run("config read fails", func(t *testing.T) {
		in := gateInput(t, "")
		in.ConfigZ = func() (string, error) { return "", errors.New("git exploded") }
		err := CheckPushTransport(in)
		if err == nil || ExitCodeOf(err) != ExitUnverifiable {
			t.Fatalf("err = %v (exit %d), want exit %d", err, ExitCodeOf(err), ExitUnverifiable)
		}
		if !strings.Contains(err.Error(), "cannot read git config") {
			t.Fatalf("a failed config READ must report itself as one, not fall through to another "+
				"could-not-check with the same exit code:\n%s", err.Error())
		}
	})

	t.Run("remote has no url", func(t *testing.T) {
		err := CheckPushTransport(gateInput(t, configZ("user.name", "someone")))
		if err == nil || ExitCodeOf(err) != ExitUnverifiable {
			t.Fatalf("err = %v (exit %d), want exit %d", err, ExitCodeOf(err), ExitUnverifiable)
		}
	})

	t.Run("no reader wired", func(t *testing.T) {
		in := gateInput(t, "")
		in.ConfigZ = nil
		err := CheckPushTransport(in)
		if err == nil || ExitCodeOf(err) != ExitUnverifiable {
			t.Fatalf("err = %v (exit %d), want exit %d", err, ExitCodeOf(err), ExitUnverifiable)
		}
	})
}

// TestPushGateNamesTheActingRole: the refusal must name the App this session acts
// as, not a generic "a bot" — the reader's next question is always "which identity was I
// about to push under".
func TestPushGateNamesTheActingRole(t *testing.T) {
	t.Setenv("DESK_LOOP", "verify-desk")
	err := CheckPushTransport(gateInput(t, configZ("remote.origin.url", "git@example.com:example-org/tracker.git")))
	if err == nil {
		t.Fatal("want a refusal")
	}
	if !strings.Contains(err.Error(), "verifier App") {
		t.Errorf("refusal does not name the verifier App:\n%s", err.Error())
	}
}

// TestIsSSHTransport pins the classifier's edges directly — including the Windows drive
// path, which carries a colon before any slash and would otherwise read as scp-like.
func TestIsSSHTransport(t *testing.T) {
	cases := []struct {
		url string
		ssh bool
	}{
		{"ssh://git@example.com/o/r.git", true},
		{"SSH://git@example.com/o/r.git", true},
		{"git+ssh://example.com/o/r", true},
		{"git@example.com:o/r.git", true},
		{"example.com:o/r.git", true},
		{"[fd00::1]:o/r.git", true},
		{"https://example.com/o/r.git", false},
		{"http://example.com/o/r.git", false},
		{"git://example.com/o/r.git", false},
		{"file:///tmp/o.git", false},
		{"/tmp/o.git", false},
		{"../sibling.git", false},
		{"C:/src/repo", false},
		{`C:\src\repo`, false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isSSHTransport(tc.url); got != tc.ssh {
			t.Errorf("isSSHTransport(%q) = %v, want %v", tc.url, got, tc.ssh)
		}
	}
}

// TestHTTPSEquivalent pins the remedy-line rewrite, which is the part of the refusal a
// reader actually pastes.
func TestHTTPSEquivalent(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ssh://git@example.com/o/r.git", "https://example.com/o/r.git"},
		{"ssh://git@example.com:443/o/r.git", "https://example.com/o/r.git"},
		{"git@example.com:o/r.git", "https://example.com/o/r.git"},
		{"ssh://git@example.com/~user/r.git", "https://example.com/user/r.git"},
		{"ssh://git@[fd00::1]:22/o/r.git", "https://[fd00::1]/o/r.git"},
		{"ssh://example.com", ""},
	}
	for _, tc := range cases {
		if got := httpsEquivalent(tc.in); got != tc.want {
			t.Errorf("httpsEquivalent(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestParseConfigZMultilineHelper is why the reader asks git for `-z`: a credential helper
// whose value is an inline shell function spans lines, and the line-oriented `--list` splits
// it into garbage — which would then read as an unrecognised (i.e. App) helper by accident.
func TestParseConfigZMultilineHelper(t *testing.T) {
	cfg := parseConfigZ(configZ(
		"credential.helper", "!f() {\n  echo password=x\n}; f",
		"remote.origin.url", "https://example.com/o/r.git"))
	if got := cfg["credential.helper"]; len(got) != 1 || !strings.Contains(got[0], "echo password=x") {
		t.Fatalf("credential.helper = %q, want the whole multi-line value intact", got)
	}
	if got := cfg["remote.origin.url"]; len(got) != 1 || got[0] != "https://example.com/o/r.git" {
		t.Fatalf("remote.origin.url = %q, want the url unaffected by the multi-line neighbour", got)
	}
}
