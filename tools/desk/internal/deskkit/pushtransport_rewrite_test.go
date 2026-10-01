package deskkit

// pushtransport_rewrite_test.go — the push-transport gate against git's url rewrites (#884).
//
// These tests drive REAL git, not a config fixture: the defect is that the gate read the
// configured URL string while git pushes to the URL it gets after applying
// url.<base>.insteadOf / url.<base>.pushInsteadOf. A fixture that encodes this code's own
// idea of the rewrite would test the reimplementation against itself; asking git is the
// only oracle that cannot share the bug. No remote is contacted — `remote get-url` and
// `config --list` read local config only.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// rewriteRepo is a throwaway repository whose git config is isolated from the host's
// (no system or global config), so a rewrite rule on the machine running the test cannot
// leak into — or rescue — the case under test.
func rewriteRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	tmp := t.TempDir()
	global := filepath.Join(tmp, "global.gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	for _, k := range []string{"GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "GIT_DIR", "GIT_WORK_TREE"} {
		unsetForTest(t, k)
	}
	dir := filepath.Join(tmp, "repo")
	rewriteGit(t, "init", "-q", dir)
	rewriteGit(t, "-C", dir, "config", "gc.auto", "0")
	return dir
}

func rewriteGit(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// realGitGateInput wires the gate the way deskpr and deskwt do: both readers are git
// itself, run in the worktree.
func realGitGateInput(dir string) PushTransportInput {
	run := func(args ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
		return string(out), err
	}
	return PushTransportInput{
		Tool: "deskpr", Verb: "create", Dir: dir, Remote: "origin",
		ConfigZ:  func() (string, error) { return run("config", "--list", "-z") },
		PushURLs: func() (string, error) { return run("remote", "get-url", "--push", "--all", "origin") },
	}
}

const rewriteHTTPS = "https://example.com/example-org/tracker.git"

// TestPushTransportRefusesInsteadOfRewrittenToSSH: the configured push URL is https, and
// url.<base>.insteadOf rewrites it to SSH before git connects. The gate must refuse — and
// name the rule — rather than pass the https string it was handed.
//
// FAIL-FIRST: on the unfixed gate (which decides from remote.origin.url as configured) this
// returns nil — the false pass #884 reports.
func TestPushTransportRefusesInsteadOfRewrittenToSSH(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", rewriteHTTPS)
	rewriteGit(t, "-C", dir, "config", "url.git@example.com:.insteadOf", "https://example.com/")

	// Oracle check: the fixture really does make git push over SSH.
	if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "origin")); got != "git@example.com:example-org/tracker.git" {
		t.Fatalf("fixture: git resolves the push url to %q, want the SSH rewrite", got)
	}

	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal (exit %d): git will push this over SSH",
			err, ExitCodeOf(err), ExitRefused)
	}
	for _, frag := range []string{
		"SSH transport",
		"worker App",
		"url.git@example.com:.insteadOf", // the rule, in git's own spelling
		rewriteHTTPS,                     // the configured URL it rewrote
		"git@example.com:example-org/tracker.git", // what git will actually push to
		"does NOT escape it",                      // the remedy is the rule, not a pushurl
	} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("refusal is missing %q:\n%s", frag, err.Error())
		}
	}

	// insteadOf rewrites an explicit pushurl too, so an https pushurl override does not
	// clear it — the gate must not take the pushurl at face value either.
	t.Run("explicit https pushurl is still rewritten", func(t *testing.T) {
		rewriteGit(t, "-C", dir, "config", "remote.origin.pushurl", rewriteHTTPS)
		err := CheckPushTransport(realGitGateInput(dir))
		if err == nil || ExitCodeOf(err) != ExitRefused {
			t.Fatalf("err = %v (exit %d), want a refusal: insteadOf applies to pushurl values", err, ExitCodeOf(err))
		}
		if !strings.Contains(err.Error(), "remote.origin.pushurl") ||
			!strings.Contains(err.Error(), "url.git@example.com:.insteadOf") {
			t.Errorf("refusal should trace remote.origin.pushurl through the insteadOf rule:\n%s", err.Error())
		}
		// The configured value already is the https target, so the rule is the whole story:
		// no second set-url step is owed, and removing the rule alone clears it.
		if strings.Contains(err.Error(), "Removing the rule alone is not enough") ||
			strings.Contains(err.Error(), "remote set-url --push origin") {
			t.Errorf("refusal proposes a set-url step the rule removal does not need:\n%s", err.Error())
		}
		rewriteGit(t, "-C", dir, "config", "--unset-all", "url.git@example.com:.insteadOf")
		if err := CheckPushTransport(realGitGateInput(dir)); ExitCodeOf(err) == ExitRefused {
			t.Fatalf("removing the named rule did not clear the refusal:\n%v", err)
		}
	})
}

// TestPushTransportRefusesPushInsteadOfRewrittenToSSH: url.<base>.pushInsteadOf rewrites
// only the PUSH url — fetch stays https, so `remote -v`'s first line and a fetch both look
// clean while every push leaves over SSH. The gate must refuse and name the rule.
//
// FAIL-FIRST: on the unfixed gate this returns nil.
func TestPushTransportRefusesPushInsteadOfRewrittenToSSH(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", rewriteHTTPS)
	rewriteGit(t, "-C", dir, "config", "url.ssh://git@example.com/.pushInsteadOf", "https://example.com/")

	if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "origin")); got != rewriteHTTPS {
		t.Fatalf("fixture: fetch url = %q, want it untouched by a push-only rule", got)
	}
	if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "origin")); got != "ssh://git@example.com/example-org/tracker.git" {
		t.Fatalf("fixture: git resolves the push url to %q, want the SSH rewrite", got)
	}

	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal (exit %d): git will push this over SSH",
			err, ExitCodeOf(err), ExitRefused)
	}
	for _, frag := range []string{
		"SSH transport",
		"url.ssh://git@example.com/.pushInsteadOf",
		"ssh://git@example.com/example-org/tracker.git",
		"remote set-url --push origin " + rewriteHTTPS, // an explicit pushurl clears a push-only rule
		"never pushInsteadOf-rewritten",                // and the refusal says why
	} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("refusal is missing %q:\n%s", frag, err.Error())
		}
	}

	// The remedy the refusal names really clears it: git never pushInsteadOf-rewrites an
	// explicit pushurl, and the gate must agree rather than over-refuse.
	t.Run("the named remedy clears it", func(t *testing.T) {
		rewriteGit(t, "-C", dir, "remote", "set-url", "--push", "origin", rewriteHTTPS)
		if err := CheckPushTransport(realGitGateInput(dir)); ExitCodeOf(err) == ExitRefused {
			t.Fatalf("an explicit https pushurl is not pushInsteadOf-rewritten, yet the gate refused:\n%v", err)
		}
	})
}

// TestPushTransportSSHAliasRemedy: an scp-like SSH url that an insteadOf rule rewrites into
// ANOTHER SSH url (the port-443 ssh alias shape). The rule matches only the SSH spelling, so
// an explicit https push url escapes it — and removing the rule would leave the configured
// url, still SSH. The remedy must be `remote set-url --push <https twin of the configured
// url>`, and applying it must clear the refusal.
//
// FAIL-FIRST: on c23d37ea4 (remedy() chose by rule kind alone) this refusal told the operator
// to remove or narrow the rule and that `set-url --push` would NOT clear it — both false here.
func TestPushTransportSSHAliasRemedy(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", "git@example.com:example-org/tracker.git")
	rewriteGit(t, "-C", dir, "config", "url.ssh://git@ssh.example.com:443/.insteadOf", "git@example.com:")

	const alias = "ssh://git@ssh.example.com:443/example-org/tracker.git"
	if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "origin")); got != alias {
		t.Fatalf("fixture: git resolves the push url to %q, want the ssh-alias rewrite %q", got, alias)
	}

	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal: git pushes this over SSH", err, ExitCodeOf(err))
	}
	msg := err.Error()
	for _, frag := range []string{
		"url.ssh://git@ssh.example.com:443/.insteadOf", // the rule, still attributed
		"remote set-url --push origin " + rewriteHTTPS, // the https twin of the CONFIGURED url
		"the rule matches only the configured url",     // why a pushurl escapes an insteadOf rule
	} {
		if !strings.Contains(msg, frag) {
			t.Errorf("refusal is missing %q:\n%s", frag, msg)
		}
	}
	for _, bad := range []string{"does NOT escape it", "remove or narrow", "https://ssh.example.com/"} {
		if strings.Contains(msg, bad) {
			t.Errorf("refusal carries %q — advice that is false for an SSH-to-SSH rewrite:\n%s", bad, msg)
		}
	}

	// The remedy the refusal names really clears it.
	rewriteGit(t, "-C", dir, "remote", "set-url", "--push", "origin", rewriteHTTPS)
	if err := CheckPushTransport(realGitGateInput(dir)); ExitCodeOf(err) == ExitRefused {
		t.Fatalf("the named remedy did not clear the refusal:\n%v", err)
	}
}

// TestPushTransportRemedyRuleSSH: the configured url is itself SSH (no rewrite produced it),
// but an insteadOf rule rewrites the https url the remedy would propose back into SSH. A
// `set-url --push` remedy would leave the refusal standing, so the refusal must name that
// rule instead — the same per-target decision as the attributed branches.
func TestPushTransportRemedyRuleSSH(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", "ssh://git@example.com/example-org/tracker.git")
	rewriteGit(t, "-C", dir, "config", "url.git@example.com:.insteadOf", "https://example.com/")

	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal", err, ExitCodeOf(err))
	}
	msg := err.Error()
	for _, frag := range []string{"remove or narrow the rule url.git@example.com:.insteadOf", "does NOT escape it"} {
		if !strings.Contains(msg, frag) {
			t.Errorf("refusal is missing %q:\n%s", frag, msg)
		}
	}
	// The configured url is itself SSH, so removing the rule is not enough on its own either:
	// the set-url step must come AFTER the rule, as the second of two steps, never as the
	// whole remedy.
	rule := strings.Index(msg, "remove or narrow")
	setURL := strings.Index(msg, "remote set-url --push origin "+rewriteHTTPS)
	if setURL < 0 || setURL < rule {
		t.Errorf("refusal should name the rule first and then `remote set-url --push origin %s`:\n%s", rewriteHTTPS, msg)
	}

	// Oracle: the set-url step alone really would not have cleared it...
	rewriteGit(t, "-C", dir, "remote", "set-url", "--push", "origin", rewriteHTTPS)
	if err := CheckPushTransport(realGitGateInput(dir)); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("oracle: an https pushurl the rule matches should still be refused, got %v", err)
	}
	// ...and both steps together do.
	rewriteGit(t, "-C", dir, "config", "--unset-all", "url.git@example.com:.insteadOf")
	if err := CheckPushTransport(realGitGateInput(dir)); ExitCodeOf(err) == ExitRefused {
		t.Fatalf("the two named steps did not clear the refusal:\n%v", err)
	}
}

// TestPushTransportPushAndInsteadOfRemedy: a pushInsteadOf AND an insteadOf rule both match
// the configured https url. git pushes to the pushInsteadOf alias; the https push url the
// remedy proposes is itself insteadOf-rewritten back to SSH. Removing the insteadOf rule
// alone leaves the pushInsteadOf alias in force, so the refusal must name BOTH steps rather
// than send the operator round the gate twice.
//
// FAIL-FIRST: on a940e5617 the refusal named only the insteadOf rule.
func TestPushTransportPushAndInsteadOfRemedy(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", rewriteHTTPS)
	rewriteGit(t, "-C", dir, "config", "url.git@push.example:.pushInsteadOf", "https://example.com/")
	rewriteGit(t, "-C", dir, "config", "url.git@example.com:.insteadOf", "https://example.com/")

	if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "origin")); got != "git@push.example:example-org/tracker.git" {
		t.Fatalf("fixture: git resolves the push url to %q, want the pushInsteadOf alias", got)
	}
	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal", err, ExitCodeOf(err))
	}
	msg := err.Error()
	for _, frag := range []string{
		"is rewritten by the rule url.git@push.example:.pushInsteadOf",
		"remove or narrow the rule url.git@example.com:.insteadOf",
		"Removing the rule alone is not enough",
	} {
		if !strings.Contains(msg, frag) {
			t.Errorf("refusal is missing %q:\n%s", frag, msg)
		}
	}
	rule := strings.Index(msg, "remove or narrow")
	setURL := strings.Index(msg, "remote set-url --push origin "+rewriteHTTPS)
	if setURL < 0 || setURL < rule {
		t.Errorf("refusal should name the rule first and then `remote set-url --push origin %s`:\n%s", rewriteHTTPS, msg)
	}

	// Oracle: the rule removal alone leaves the pushInsteadOf alias in force...
	rewriteGit(t, "-C", dir, "config", "--unset-all", "url.git@example.com:.insteadOf")
	if err := CheckPushTransport(realGitGateInput(dir)); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("oracle: with the pushInsteadOf alias still set the push is still SSH, got %v", err)
	}
	// ...and the second named step clears it.
	rewriteGit(t, "-C", dir, "remote", "set-url", "--push", "origin", rewriteHTTPS)
	if err := CheckPushTransport(realGitGateInput(dir)); ExitCodeOf(err) == ExitRefused {
		t.Fatalf("the two named steps did not clear the refusal:\n%v", err)
	}
}

// TestPushTransportMultiPushURLRemedy: two pushurls, an https mirror and an SSH one. The
// plain `remote set-url --push origin <https>` fails on a multi-valued pushurl ("has
// multiple values"), so the remedy line must name the value it replaces — and running the
// line exactly as printed must clear the refusal while keeping the https mirror.
func TestPushTransportMultiPushURLRemedy(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not on PATH")
	}
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	const mirror = "https://mirror.example/example-org/tracker.git"
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", rewriteHTTPS)
	rewriteGit(t, "-C", dir, "config", "remote.origin.pushurl", mirror)
	rewriteGit(t, "-C", dir, "config", "--add", "remote.origin.pushurl", "git@example.com:example-org/tracker.git")

	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal", err, ExitCodeOf(err))
	}
	var line string
	for _, l := range strings.Split(err.Error(), "\n") {
		if strings.HasPrefix(l, "  git -C ") {
			line = strings.TrimSpace(l)
		}
	}
	if line == "" {
		t.Fatalf("refusal carries no runnable remedy line:\n%v", err)
	}
	if out, rerr := exec.Command("sh", "-c", line).CombinedOutput(); rerr != nil {
		t.Fatalf("the remedy line as printed does not run: %v\n%s\nline: %s", rerr, out, line)
	}
	if err := CheckPushTransport(realGitGateInput(dir)); ExitCodeOf(err) == ExitRefused {
		t.Fatalf("the remedy line ran but the refusal stands:\n%v", err)
	}
	got := strings.Fields(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "--all", "origin"))
	if strings.Join(got, " ") != mirror+" "+rewriteHTTPS {
		t.Errorf("push urls after the remedy = %q, want the mirror kept and the SSH value replaced", got)
	}
}

// TestPushTransportNoURLRealGit: a remote with no url does not resolve to nothing with real
// git — `remote get-url --push --all` prints the remote's bare NAME. The gate must still call
// that could-not-check (exit 6), as the stub-driven "remote has no url" case does.
//
// FAIL-FIRST: on a940e5617 both shapes returned nil (exit 0).
func TestPushTransportNoURLRealGit(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	for _, tc := range []struct {
		name string
		set  [][]string
	}{
		{"fetch refspec only", [][]string{{"remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"}}},
		{"url set empty", [][]string{{"remote.origin.url", ""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := rewriteRepo(t)
			for _, kv := range tc.set {
				rewriteGit(t, "-C", dir, "config", kv[0], kv[1])
			}
			if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "--all", "origin")); got != "origin" {
				t.Fatalf("fixture: git resolves the push url to %q, want the bare remote name", got)
			}
			err := CheckPushTransport(realGitGateInput(dir))
			if err == nil || ExitCodeOf(err) != ExitUnverifiable {
				t.Fatalf("err = %v (exit %d), want exit %d: the remote has no url", err, ExitCodeOf(err), ExitUnverifiable)
			}
			if !strings.Contains(err.Error(), "no url or pushurl configured") {
				t.Errorf("could-not-check should say the remote has no url:\n%s", err.Error())
			}
		})
	}

	// Control: a remote whose url really is a local path spelled like its name is configured,
	// so it is judged (a local path is not SSH), never mistaken for the no-url shape.
	t.Run("url configured as the name", func(t *testing.T) {
		dir := rewriteRepo(t)
		rewriteGit(t, "-C", dir, "config", "remote.origin.url", "origin")
		if err := CheckPushTransport(realGitGateInput(dir)); err != nil {
			t.Fatalf("err = %v (exit %d), want nil: the url is configured", err, ExitCodeOf(err))
		}
	})
}

// TestPushTransportLongestRule: two insteadOf rules whose prefixes overlap. Git applies the
// LONGEST matching prefix, so the refusal must name that rule — not whichever sorts first.
func TestPushTransportLongestRule(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", rewriteHTTPS)
	// The SHORT prefix's base sorts first, so a first-match attribution would name it.
	rewriteGit(t, "-C", dir, "config", "url.ssh://git@a-short.example/.insteadOf", "https://example.com/")
	rewriteGit(t, "-C", dir, "config", "url.ssh://git@z-long.example/.insteadOf", "https://example.com/example-org/")

	if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "origin")); got != "ssh://git@z-long.example/tracker.git" {
		t.Fatalf("fixture: git resolves the push url to %q, want the longest-prefix rewrite", got)
	}
	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal", err, ExitCodeOf(err))
	}
	if !strings.Contains(err.Error(), "is rewritten by the rule url.ssh://git@z-long.example/.insteadOf") {
		t.Errorf("refusal should name the longest-prefix rule:\n%s", err.Error())
	}
}

// TestPushTransportPushurlNoAlias: with an explicit pushurl git never applies pushInsteadOf,
// only insteadOf. A pushInsteadOf rule that ALSO matches the pushurl must not steal the
// attribution from the insteadOf rule git really applied.
func TestPushTransportPushurlNoAlias(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", rewriteHTTPS)
	rewriteGit(t, "-C", dir, "config", "remote.origin.pushurl", rewriteHTTPS)
	rewriteGit(t, "-C", dir, "config", "url.ssh://git@alias.example/.pushInsteadOf", "https://example.com/")
	rewriteGit(t, "-C", dir, "config", "url.git@example.com:.insteadOf", "https://example.com/")

	if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "origin")); got != "git@example.com:example-org/tracker.git" {
		t.Fatalf("fixture: git resolves the push url to %q, want the insteadOf rewrite of the pushurl", got)
	}
	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal", err, ExitCodeOf(err))
	}
	if !strings.Contains(err.Error(), "remote.origin.pushurl = "+rewriteHTTPS+" is rewritten by the rule url.git@example.com:.insteadOf") {
		t.Errorf("refusal should trace the pushurl through the insteadOf rule:\n%s", err.Error())
	}
}

// TestPushTransportAliasSkip: with no pushurl, once ANY url has a pushInsteadOf alias git
// pushes to the aliases only, and insteadOf is never applied on top of a url that has one.
// Here the first url's alias is https while its insteadOf rewrite happens to spell the SSH
// alias of the SECOND url; the refusal must trace the second url's pushInsteadOf rule — the
// rewrite git really used — not the first url's insteadOf rewrite, which git never pushes to.
func TestPushTransportAliasSkip(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", "https://example.com/r.git")
	rewriteGit(t, "-C", dir, "config", "--add", "remote.origin.url", "https://other.example/r.git")
	rewriteGit(t, "-C", dir, "config", "url.https://pushhost.example/.pushInsteadOf", "https://example.com/")
	rewriteGit(t, "-C", dir, "config", "url.git@ssh.example:.pushInsteadOf", "https://other.example/")
	rewriteGit(t, "-C", dir, "config", "url.git@ssh.example:.insteadOf", "https://example.com/")

	got := strings.Fields(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "--all", "origin"))
	if strings.Join(got, " ") != "https://pushhost.example/r.git git@ssh.example:r.git" {
		t.Fatalf("fixture: git resolves the push urls to %q", got)
	}
	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal", err, ExitCodeOf(err))
	}
	want := "remote.origin.url = https://other.example/r.git is rewritten by the rule url.git@ssh.example:.pushInsteadOf"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("refusal should trace the second url through its pushInsteadOf rule (%q):\n%s", want, err.Error())
	}
}
