package main

import (
	"encoding/base64"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

// The board push classification (#2910). pushcred.sh is the one place a failed
// generated-board push is told apart: expired, revoked, scope, auth, nonff,
// other. These tests run the REAL run: texts of every staged board writer, and
// the regen script of the scaffolded GitLab CI file, against a throwaway origin
// with a `git` shim that fails `push` with a canned forge answer.

const (
	// pushTokenSentinel is the GitLab push token's value in the rig. It must
	// never reach output, a log, or git's argv.
	pushTokenSentinel = "PUSH-TOKEN-SENTINEL-2910"
	// urlSecretSentinel rides in the URL of a canned git error, the way a
	// credential embedded in a remote URL would; masking must hide it.
	urlSecretSentinel = "URL-SECRET-SENTINEL-2910"
)

// pushShim is `git` on the step's PATH. Everything but `push` goes to the real
// git. `push` is logged (argv, dry-run or real) and then answered by mode:
// SHIM_DRYRUN for a dry run (unset: the same as SHIM_PUSH), SHIM_PUSH for a
// real one. `ok` hands the push to the real git. When the push carries `-c`
// pairs (the inline credential helper), the shim asks git what password they
// yield and records it, so a test can prove the helper works without the
// value ever being an argument.
const pushShim = `#!/usr/bin/env bash
set -u
sub= ; skip=0
for a in "$@"; do
  if [ "$skip" = 1 ]; then skip=0; continue; fi
  case $a in -c|-C) skip=1 ;; -*) ;; *) sub=$a; break ;; esac
done
if [ "$sub" != push ]; then exec "$REAL_GIT" "$@"; fi
printf '%s\n' "$*" >>"$STUB_LOG/push-argv"
dry=0
for a in "$@"; do if [ "$a" = --dry-run ]; then dry=1; fi; done
if [ "$dry" = 1 ]; then
  echo . >>"$STUB_LOG/dryruns"; mode=${SHIM_DRYRUN:-$SHIM_PUSH}
else
  echo . >>"$STUB_LOG/pushes"; mode=$SHIM_PUSH
fi
cfg=(); prev=
for a in "$@"; do
  if [ "$prev" = -c ]; then cfg+=(-c "$a"); fi
  prev=$a
done
if [ "${#cfg[@]}" -gt 0 ]; then
  printf 'protocol=https\nhost=gitlab.test\n\n' |
    GIT_TERMINAL_PROMPT=0 "$REAL_GIT" "${cfg[@]}" credential fill 2>/dev/null |
    sed -n 's/^password=//p' >"$STUB_LOG/helper-password"
fi
url="https://x-access-token:${URL_SECRET}@forge.test/o/r.git/"
case $mode in
  ok) exec "$REAL_GIT" "$@" ;;
  expired) printf 'remote: Token has expired.\nfatal: Authentication failed for %s\n' "$url" >&2 ;;
  revoked) printf 'remote: Token was revoked. You have to re-authorize from the user.\nfatal: Authentication failed for %s\n' "$url" >&2 ;;
  scope) printf 'remote: Permission to o/r.git denied to board-writer[bot].\nfatal: unable to access %s: The requested URL returned error: 403\n' "$url" >&2 ;;
  auth) printf 'remote: Invalid username or password.\nfatal: Authentication failed for %s\n' "$url" >&2 ;;
  glauth) printf 'remote: HTTP Basic: Access denied. If a password was provided for Git authentication, the password was incorrect or you are required to use a token instead of a password. If a token was provided, it was either incorrect, expired, or improperly scoped.\nfatal: Authentication failed for %s\n' "$url" >&2 ;;
  other) printf 'remote: error: GH013: Repository rule violations found for refs/heads/main.\nTo %s\n ! [remote rejected] HEAD -> main (push declined due to repository rule violations)\n' "$url" >&2 ;;
  nonff) printf 'To %s\n ! [rejected]        HEAD -> main (fetch first)\nerror: failed to push some refs to %s\nhint: Updates were rejected because the remote contains work that you do not have locally.\n' "$url" "$url" >&2 ;;
  *) echo "shim: unknown mode $mode" >&2 ;;
esac
exit 1
`

// genStub stands in for statusgen and qualgen: every call is logged, and each
// regeneration writes a fresh board so a retry always has something to commit.
const genStub = `#!/usr/bin/env bash
if [ "$(basename "$0")" = statusgen ]; then
  case " $* " in
    *" --acceptance-forge "*)
      [ "${GH_TOKEN:-}" = READ-TOKEN-SENTINEL ] && [ "${ASSAY_HUMAN_LOGIN_MAP:-}" = alex:ada ] || {
        echo "acceptance caller lacks read token or human map" >&2; exit 98;
      } ;;
  esac
fi
echo "$(basename "$0") $*" >>"$STUB_LOG/gen-calls"
n=$(wc -l <"$STUB_LOG/gen-calls" | tr -d ' ')
case " $* " in
  *" --auto-flip-model "*) echo "flip $n" >>docs/streams/s/README.md ;;
  *" --lint "*) : ;;
  *" report "*) mkdir -p docs/quality; echo "quality $n" >docs/quality/QUALITY.md ;;
  *) echo "board $n" >STATUS.md ;;
esac
`

// curlStub answers the GitLab token self-introspection. SHIM_SELF is a
// comma-separated list of answers, one per call (the last repeats):
// <code>:<body-name>. The header it was handed on stdin is logged apart from
// its argv, so a test can tell the token travelled as stdin, not an argument.
const curlStub = `#!/usr/bin/env bash
printf '%s\n' "$*" >>"$STUB_LOG/curl-argv"
cat >>"$STUB_LOG/curl-stdin"
out=
while [ $# -gt 0 ]; do
  case $1 in -o) out=$2; shift ;; esac
  shift
done
n=$(( $(cat "$STUB_LOG/curl-n" 2>/dev/null || echo 0) + 1 ))
echo "$n" >"$STUB_LOG/curl-n"
IFS=, read -r -a answers <<<"$SHIM_SELF"
i=$(( n - 1 )); if [ "$i" -ge "${#answers[@]}" ]; then i=$(( ${#answers[@]} - 1 )); fi
code=${answers[$i]%%:*}; body=${answers[$i]#*:}
case $body in
  expired) b='{"error":"invalid_token","error_description":"Token is expired. You can either do re-authorization or token refresh."}' ;;
  revoked) b='{"error":"invalid_token","error_description":"Token was revoked. You have to re-authorize from the user."}' ;;
  live) b='{"id":7,"name":"board","revoked":false,"active":true,"scopes":["write_repository"],"expires_at":"2999-01-01"}' ;;
  pastdate) b='{"id":7,"name":"board","revoked":false,"active":true,"scopes":["write_repository"],"expires_at":"2001-01-01"}' ;;
  readonly) b='{"id":7,"name":"board","revoked":false,"active":true,"scopes":["read_repository"],"expires_at":"2999-01-01"}' ;;
  scope) b='{"error":"insufficient_scope"}' ;;
  *) b='{}' ;;
esac
if [ -n "$out" ]; then printf '%s' "$b" >"$out"; fi
printf '%s' "$code"
`

type pushRig struct {
	t                         *testing.T
	dir, origin, ws, bin, log string
	env                       []string
}

func requirePushTools(t *testing.T) {
	t.Helper()
	requireJobTools(t)
	requireTool(t, "awk")
}

func newPushRig(t *testing.T) *pushRig {
	t.Helper()
	requirePushTools(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	r := &pushRig{t: t, dir: t.TempDir()}
	r.origin = filepath.Join(r.dir, "origin.git")
	r.ws = filepath.Join(r.dir, "ws")
	r.bin = filepath.Join(r.dir, "bin")
	r.log = filepath.Join(r.dir, "log")
	for _, d := range []string{r.bin, r.log} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"git": pushShim, "statusgen": genStub, "qualgen": genStub, "curl": curlStub} {
		if err := os.WriteFile(filepath.Join(r.bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitCfg := filepath.Join(r.dir, "gitconfig")
	// The GitLab regen pushes to its https remote; the runner's git resolves
	// it to the rig's origin.
	//
	// No automatic housekeeping (#3390). commit, fetch and the receiving end of
	// a push each start `git maintenance run --auto --detach`; the detached
	// child takes a lock under .git/objects and can still be writing there when
	// t.TempDir's cleanup removes the rig, which fails the subtest with
	// "directory not empty". Every git the rig runs reads this file (r.env), so
	// switching it off here stops the fork for the rig and every step script.
	// TestPushcredNoAutoMaintenance pins it.
	cfg := "[url \"file://" + r.origin + "\"]\n\tinsteadOf = https://gitlab.test/g/p.git\n" +
		"[maintenance]\n\tauto = false\n\tautoDetach = false\n" +
		"[gc]\n\tauto = 0\n\tautoDetach = false\n"
	if err := os.WriteFile(gitCfg, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	r.env = append(scrubbedEnv(),
		"GIT_CONFIG_GLOBAL="+gitCfg, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=job", "GIT_AUTHOR_EMAIL=job@example.invalid",
		"GIT_COMMITTER_NAME=job", "GIT_COMMITTER_EMAIL=job@example.invalid",
		"STUB_LOG="+r.log, "TMPDIR="+r.dir, "HOME="+r.dir,
		"REAL_GIT="+realGit, "URL_SECRET="+urlSecretSentinel,
	)
	seed := filepath.Join(r.dir, "seed")
	r.git("", "init", "-q", "--bare", "-b", "main", r.origin)
	r.git("", "init", "-q", "-b", "main", seed)
	src, err := os.ReadFile("pushcred.sh")
	if err != nil {
		t.Fatal(err)
	}
	for p, body := range map[string]string{
		"statusgen/pushcred.sh":    string(src),
		"docs/streams/s/README.md": "# s\n",
	} {
		full := filepath.Join(seed, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r.git(seed, "add", "-A")
	r.git(seed, "commit", "-q", "-m", "seed")
	r.git(seed, "push", "-q", r.origin, "main")
	r.git("", "clone", "-q", r.origin, r.ws)
	return r
}

func (r *pushRig) git(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// run executes a script the way the runner would (bash -e) in the workspace,
// with the shims first on PATH.
func (r *pushRig) run(script string, env ...string) (int, string) {
	r.t.Helper()
	cmd := exec.Command("bash", "-e", scriptFile(r.t, script))
	cmd.Dir = r.ws
	cmd.Env = append(append(append([]string{}, r.env...),
		"PATH="+r.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"RUNNER_TEMP="+r.bin, "GITHUB_ACTIONS=true"), env...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		r.t.Fatal(err)
	}
	return code, string(out)
}

// count is the number of lines in a stub log (0 when never written).
func (r *pushRig) count(name string) int {
	b, err := os.ReadFile(filepath.Join(r.log, name))
	if err != nil {
		return 0
	}
	return strings.Count(string(b), "\n")
}

func (r *pushRig) logText(name string) string {
	b, _ := os.ReadFile(filepath.Join(r.log, name))
	return string(b)
}

// boardTarget is one staged step that pushes a generated board to main.
type boardTarget struct {
	file, job, step string
	// pre writes what the step's earlier steps would have left in the tree.
	pre func(ws string) error
	// nonffPushes is the step's retry budget; raceText is its race wording.
	nonffPushes int
	raceText    []string
}

func plantFile(rel, body string) func(string) error {
	return func(ws string) error {
		p := filepath.Join(ws, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte(body), 0o644)
	}
}

func boardTargets() []boardTarget {
	var ts []boardTarget
	for _, dir := range []string{"../ci/staged-workflows", "../tools/ci-load/activation"} {
		sg := filepath.Join(dir, "assay-statusgen.yml")
		ts = append(ts,
			boardTarget{sg, "regen", "Commit STATUS.md if it changed", plantFile("STATUS.md", "board 0\n"),
				3, []string{"push rejected: main moved", "moving faster than the regen can re-sync"}},
			boardTarget{sg, "model-autoflip", "Auto-flip gate:model briefs from the App approval at the merged head", nil,
				5, []string{"lost a race with a concurrent push"}},
			boardTarget{filepath.Join(dir, "assay-qualgen.yml"), "regen", "Commit QUALITY.md if it changed",
				plantFile("docs/quality/QUALITY.md", "quality 0\n"),
				3, []string{"push rejected: main moved", "moving faster than the regen can re-sync"}},
		)
	}
	return ts
}

func (bt boardTarget) name() string {
	return strings.TrimPrefix(bt.file, "../") + ":" + bt.job
}

func (bt boardTarget) run(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(bt.file)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range parseWorkflow(t, raw, bt.file).Jobs[bt.job].Steps {
		if st.Name == bt.step {
			// Resolve the actual step env with the same read/write sentinels as
			// the reconcile rig. Unknown expressions still fail closed.
			var prefix string
			for _, kv := range stepEnv(t, st, nil) {
				k, v, _ := strings.Cut(kv, "=")
				prefix += "export " + k + "='" + strings.ReplaceAll(v, "'", "'\\''") + "'\n"
			}
			return prefix + st.Run
		}
	}
	t.Fatalf("%s: no step named %q", bt.name(), bt.step)
	return ""
}

// classWant is the line each non-race class must print.
var classWant = map[string]string{
	"expired": "push credential has EXPIRED",
	"revoked": "push credential was REVOKED",
	"scope":   "lacks push permission",
	"auth":    "authentication failed",
	"other":   "neither a credential failure nor a moved main",
}

var raceWording = []string{"main moved", "moving faster", "lost a race", "Re-syncing"}

func assertNoRace(t *testing.T, where, out string) {
	t.Helper()
	for _, w := range raceWording {
		if strings.Contains(out, w) {
			t.Errorf("%s: a non-race failure printed race wording %q:\n%s", where, w, out)
		}
	}
}

func assertNoSecret(t *testing.T, where, out string) {
	t.Helper()
	for _, s := range []string{pushTokenSentinel, urlSecretSentinel} {
		if strings.Contains(out, s) {
			t.Errorf("%s: a credential value reached the output:\n%s", where, out)
		}
	}
}

// TestBoardPushAuthFailsOnce: the credential dies after the preflight (it
// passes the dry run, then the real push is refused). Every staged board
// writer must fail ONCE, with the class's own line, spending no retry and
// never saying main moved; git's output is shown masked.
//
// regression: #2910
func TestBoardPushAuthFailsOnce(t *testing.T) {
	for _, bt := range boardTargets() {
		for _, class := range []string{"expired", "revoked", "scope", "auth", "other"} {
			t.Run(bt.name()+"/"+class, func(t *testing.T) {
				r := newPushRig(t)
				if bt.pre != nil {
					if err := bt.pre(r.ws); err != nil {
						t.Fatal(err)
					}
				}
				code, out := r.run(bt.run(t), "SHIM_DRYRUN=ok", "SHIM_PUSH="+class)
				if code == 0 {
					t.Errorf("a refused push exited 0:\n%s", out)
				}
				if n := r.count("pushes"); n != 1 {
					t.Errorf("a %s failure pushed %d times, want exactly 1 (no retry spent):\n%s", class, n, out)
				}
				if !strings.Contains(out, classWant[class]) {
					t.Errorf("output lacks the %s line %q:\n%s", class, classWant[class], out)
				}
				assertNoRace(t, bt.name(), out)
				assertNoSecret(t, bt.name(), out)
			})
		}
	}
}

// TestPushcredNoAutoMaintenance: no git the push rig runs may start automatic
// maintenance, which detaches and can outlive the subtest, racing t.TempDir's
// cleanup. The step path is traced end to end: a commit, a fetch, and a real
// push (whose receive-pack runs in the rig's origin).
//
// regression: #3390
func TestPushcredNoAutoMaintenance(t *testing.T) {
	r := newPushRig(t)
	trace := filepath.Join(r.dir, "git-trace")
	script := "git commit -q --allow-empty -m probe\n" +
		"git fetch -q origin main\n" +
		"git push -q origin HEAD:main\n"
	code, out := r.run(script, "SHIM_PUSH=ok", "GIT_TRACE="+trace)
	if code != 0 {
		t.Fatalf("the traced git steps failed:\n%s", out)
	}
	b, err := os.ReadFile(trace)
	if err != nil {
		t.Fatalf("no git trace was written: %v", err)
	}
	got := string(b)
	for _, want := range []string{"built-in: git commit", "built-in: git fetch", "receive-pack"} {
		if !strings.Contains(got, want) {
			t.Fatalf("positive control: the trace lacks %q, so it proves nothing:\n%s", want, got)
		}
	}
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "maintenance run") || strings.Contains(l, "gc --auto") {
			t.Errorf("a rig git started automatic housekeeping: %s", strings.TrimSpace(l))
		}
	}
}

// TestBoardPushPreflightStops: a dead credential is caught BEFORE the board
// commit is written — the dry-run push fails, the step stops with the class's
// line, makes no commit and no real push, and regenerates nothing.
//
// regression: #2910
func TestBoardPushPreflightStops(t *testing.T) {
	for _, bt := range boardTargets() {
		for _, class := range []string{"expired", "revoked", "scope", "auth"} {
			t.Run(bt.name()+"/"+class, func(t *testing.T) {
				r := newPushRig(t)
				if bt.pre != nil {
					if err := bt.pre(r.ws); err != nil {
						t.Fatal(err)
					}
				}
				head := r.git(r.ws, "rev-parse", "HEAD")
				code, out := r.run(bt.run(t), "SHIM_PUSH="+class)
				if code == 0 {
					t.Errorf("a dead credential exited 0:\n%s", out)
				}
				if n := r.count("pushes"); n != 0 {
					t.Errorf("preflight let %d real push(es) through:\n%s", n, out)
				}
				if got := r.git(r.ws, "rev-parse", "HEAD"); got != head {
					t.Errorf("a board commit was written before the credential was checked (HEAD %s → %s)", head, got)
				}
				if n := r.count("gen-calls"); n != 0 {
					t.Errorf("the generator ran %d time(s) behind a dead credential:\n%s", n, r.logText("gen-calls"))
				}
				if !strings.Contains(out, classWant[class]) {
					t.Errorf("output lacks the %s line %q:\n%s", class, classWant[class], out)
				}
				assertNoRace(t, bt.name(), out)
				assertNoSecret(t, bt.name(), out)
			})
		}
	}
}

// TestBoardPushRaceRetries: a genuine race (non-fast-forward) is the ONE class
// that retries and says main moved — and the credential is preflighted again
// before every retry, so a dry run precedes each real push.
//
// regression: #2910
func TestBoardPushRaceRetries(t *testing.T) {
	for _, bt := range boardTargets() {
		t.Run(bt.name(), func(t *testing.T) {
			r := newPushRig(t)
			if bt.pre != nil {
				if err := bt.pre(r.ws); err != nil {
					t.Fatal(err)
				}
			}
			_, out := r.run(bt.run(t), "SHIM_DRYRUN=ok", "SHIM_PUSH=nonff")
			if n := r.count("pushes"); n != bt.nonffPushes {
				t.Errorf("a race pushed %d times, want the full budget %d:\n%s", n, bt.nonffPushes, out)
			}
			if n := r.count("dryruns"); n != bt.nonffPushes {
				t.Errorf("%d credential preflight(s) for %d push attempt(s); want one before the commit and one before each retry:\n%s", n, bt.nonffPushes, out)
			}
			for _, w := range bt.raceText {
				if !strings.Contains(out, w) {
					t.Errorf("a race lacks its wording %q:\n%s", w, out)
				}
			}
			assertNoSecret(t, bt.name(), out)
		})
	}
}

// TestBoardPushHappyPath: the preflight changes nothing for a live
// credential — the board lands on origin/main in one push.
func TestBoardPushHappyPath(t *testing.T) {
	for _, bt := range boardTargets() {
		t.Run(bt.name(), func(t *testing.T) {
			r := newPushRig(t)
			if bt.pre != nil {
				if err := bt.pre(r.ws); err != nil {
					t.Fatal(err)
				}
			}
			before := r.git(r.origin, "rev-parse", "main")
			code, out := r.run(bt.run(t), "SHIM_PUSH=ok")
			if code != 0 {
				t.Fatalf("a live credential failed the step:\n%s", out)
			}
			if r.git(r.origin, "rev-parse", "main") == before {
				t.Errorf("origin/main did not advance:\n%s", out)
			}
			if n := r.count("pushes"); n != 1 {
				t.Errorf("pushed %d times, want 1:\n%s", n, out)
			}
		})
	}
}

// gitlabRegenScript is the scaffolded GitLab regen job's script, every anchor
// expanded, minus the release-binary install (the rig's statusgen stub stands
// in for it), joined the way the runner joins script items: one shell.
func gitlabRegenScript(t *testing.T, ci string) string {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(ci), &doc); err != nil {
		t.Fatalf("parsing the GitLab template: %v", err)
	}
	job, ok := doc["statusgen-regen"].(map[string]any)
	if !ok {
		t.Fatal("the GitLab template has no statusgen-regen job")
	}
	var items []string
	var flatten func(v any)
	flatten = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				flatten(e)
			}
		case string:
			items = append(items, x)
		default:
			t.Fatalf("unexpected script item %T", v)
		}
	}
	flatten(job["script"])
	var keep []string
	for _, it := range items {
		if strings.Contains(it, ".assay-versions") {
			continue
		}
		keep = append(keep, it)
	}
	return strings.Join(keep, "\n")
}

func gitlabEnv(self string, extra ...string) []string {
	return append([]string{
		"CI_SERVER_HOST=gitlab.test", "CI_PROJECT_PATH=g/p", "CI_DEFAULT_BRANCH=main",
		"CI_API_V4_URL=https://gitlab.test/api/v4", "STATUSGEN_PUSH_TOKEN=" + pushTokenSentinel,
		"GITHUB_ACTIONS=", "SHIM_SELF=" + self,
	}, extra...)
}

// assertTokenOffArgv: the push token never appears in git's or curl's argv.
func assertTokenOffArgv(t *testing.T, r *pushRig) {
	t.Helper()
	for _, l := range []string{"push-argv", "curl-argv"} {
		if strings.Contains(r.logText(l), pushTokenSentinel) {
			t.Errorf("the push token rode in %s:\n%s", l, r.logText(l))
		}
	}
}

// TestGitlabRegenExpiredToken: the scaffolded GitLab regen job, run with an
// EXPIRED push token (GitLab answers the token's self-introspection with 401
// "Token is expired"). It must stop before regenerating, with the expired
// line, push nothing, and never put the token in output or argv.
//
// regression: #2910
func TestGitlabRegenExpiredToken(t *testing.T) {
	r := newPushRig(t)
	code, out := r.run(gitlabRegenScript(t, initGitlabCI), gitlabEnv("401:expired", "SHIM_PUSH=glauth")...)
	if code == 0 {
		t.Errorf("an expired token exited 0:\n%s", out)
	}
	if !strings.Contains(out, classWant["expired"]) {
		t.Errorf("output lacks the expired line:\n%s", out)
	}
	if n := r.count("push-argv"); n != 0 {
		t.Errorf("an expired token still reached %d push(es):\n%s", n, r.logText("push-argv"))
	}
	if n := r.count("gen-calls"); n != 0 {
		t.Errorf("statusgen regenerated behind an expired token:\n%s", out)
	}
	if !strings.Contains(r.logText("curl-stdin"), "PRIVATE-TOKEN: "+pushTokenSentinel) {
		t.Errorf("the introspection did not carry the token as a header on stdin:\n%s", r.logText("curl-argv"))
	}
	assertNoRace(t, "gitlab", out)
	assertNoSecret(t, "gitlab", out)
	assertTokenOffArgv(t, r)
}

// TestGitlabRegenClassifies: the GitLab regen job's push, past the preflight,
// for each answer the forge can give. GitLab's push refusal is one message for
// invalid, expired and under-scoped tokens, so a refused push is refined by
// self-introspection; a race is reported as a race, once (no retry loop here).
//
// regression: #2910
func TestGitlabRegenClassifies(t *testing.T) {
	cases := []struct {
		name, self, dry, push string
		wantCode              int
		want                  string
		pushes                int
	}{
		// The token expired between the preflight and the push.
		{"expires-mid-job", "200:live,401:expired", "ok", "glauth", 1, classWant["expired"], 1},
		{"revoked-mid-job", "200:live,401:revoked", "ok", "glauth", 1, classWant["revoked"], 1},
		// Introspection is could-not-check (the token cannot read the API);
		// the dry run is the check, and it is refused.
		{"dryrun-refused", "403:scope", "", "glauth", 1, classWant["auth"], 0},
		{"expiry-date-past", "200:pastdate", "", "ok", 1, classWant["expired"], 0},
		{"read-only-scope", "200:readonly", "", "ok", 1, classWant["scope"], 0},
		{"race", "200:live", "ok", "nonff", 1, "push rejected: main moved", 1},
		{"live", "200:live", "", "ok", 0, "credential preflight passed", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newPushRig(t)
			env := gitlabEnv(c.self, "SHIM_PUSH="+c.push)
			if c.dry != "" {
				env = append(env, "SHIM_DRYRUN="+c.dry)
			}
			code, out := r.run(gitlabRegenScript(t, initGitlabCI), env...)
			if (code == 0) != (c.wantCode == 0) {
				t.Errorf("exit %d, want %d:\n%s", code, c.wantCode, out)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("output lacks %q:\n%s", c.want, out)
			}
			if n := r.count("pushes"); n != c.pushes {
				t.Errorf("%d real push(es), want %d:\n%s", n, c.pushes, out)
			}
			if c.name != "race" {
				assertNoRace(t, c.name, out)
			}
			assertNoSecret(t, c.name, out)
			assertTokenOffArgv(t, r)
			if c.name == "live" {
				if got := strings.TrimSpace(r.logText("helper-password")); got != pushTokenSentinel {
					t.Errorf("the inline credential helper yielded %q, want the push token", got)
				}
				if !strings.Contains(r.git(r.origin, "log", "-1", "--format=%s", "main"), "[skip-status-regen]") {
					t.Errorf("the regenerated board did not land on main:\n%s", out)
				}
			}
		})
	}
}

// TestPushcredClassUnit pins the classifier on the forge answers it reads, in
// bash and in the system sh (the GitLab job's shell may be either).
//
// regression: #2910
func TestPushcredClassUnit(t *testing.T) {
	requirePushTools(t)
	cases := map[string]string{
		"remote: Token has expired.\nfatal: Authentication failed for 'https://h/p.git/'":                                                      "expired",
		`{"error":"invalid_token","error_description":"Token is expired."}`:                                                                    "expired",
		"remote: Token was revoked. You have to re-authorize from the user.":                                                                   "revoked",
		"remote: Permission to o/r.git denied to x[bot].\nfatal: unable to access 'https://h/o/r.git/': The requested URL returned error: 403": "scope",
		"remote: GitLab: You are not allowed to push code to protected branches on this project.":                                              "scope",
		"remote: Invalid username or password.\nfatal: Authentication failed for 'https://h/o/r.git/'":                                         "auth",
		// GitLab's one refusal says "expired" in passing; on its own it is NOT
		// the expired class — only introspection can tell.
		"remote: HTTP Basic: Access denied. If a token was provided, it was either incorrect, expired, or improperly scoped.": "auth",
		"fatal: could not read Username for 'https://h': terminal prompts disabled":                                           "auth",
		" ! [rejected]        HEAD -> main (fetch first)\nhint: Updates were rejected because the remote contains work":       "nonff",
		" ! [rejected]        HEAD -> main (non-fast-forward)":                                                                "nonff",
		" ! [remote rejected] main -> main (cannot lock ref 'refs/heads/main': is at 1 but expected 2)":                       "nonff",
		"remote: error: GH013: Repository rule violations found for refs/heads/main.":                                         "other",
		" ! [remote rejected] HEAD -> main (pre-receive hook declined)":                                                       "other",
		"fatal: unable to access 'https://h/': Could not resolve host: h":                                                     "other",
	}
	src, err := filepath.Abs("pushcred.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, sh := range []string{"bash", "sh"} {
		for in, want := range cases {
			p := filepath.Join(t.TempDir(), "log")
			if err := os.WriteFile(p, []byte(in+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(sh, "-c", `. "$1"; pushcred_class "$2"`, sh, src, p).CombinedOutput()
			if err != nil {
				t.Fatalf("%s: %v\n%s", sh, err, out)
			}
			if got := strings.TrimSpace(string(out)); got != want {
				t.Errorf("%s: class(%q) = %q, want %q", sh, in, got, want)
			}
		}
		// Only the race names a moved main.
		for _, c := range []string{"expired", "revoked", "scope", "auth", "other", "nonff"} {
			out, err := exec.Command(sh, "-c", `. "$1"; pushcred_message "$2"`, sh, src, c).CombinedOutput()
			if err != nil {
				t.Fatalf("%s: %v\n%s", sh, err, out)
			}
			if moved := strings.Contains(string(out), "main moved"); moved != (c == "nonff") {
				t.Errorf("%s: message(%s) = %q: only nonff may say main moved", sh, c, out)
			}
		}
	}
}

// TestPushcredMaskShapes plants one sentinel of each token shape the mask
// claims to cover. The shapes are assembled at run time so no token-shaped
// literal sits in the source.
//
// regression: #2910
func TestPushcredMaskShapes(t *testing.T) {
	requirePushTools(t)
	src, err := filepath.Abs("pushcred.sh")
	if err != nil {
		t.Fatal(err)
	}
	fine := "github" + "_pat_" + strings.Repeat("A", 11) + "_" + strings.Repeat("b", 59)
	dotted := "gl" + "pat-" + strings.Repeat("C", 27) + ".01." + "SUFFIXSENTINEL"
	shapes := map[string]string{
		"classic":     "gh" + "s_" + strings.Repeat("D", 36),
		"fine":        fine,
		"gl-classic":  "gl" + "pat-" + strings.Repeat("E", 20),
		"gl-dotted":   dotted,
		"gl-deploy":   "gl" + "dt-" + strings.Repeat("F", 20),
		"url-userinf": "https://x-access-token:" + urlSecretSentinel + "@forge.test/o/r.git",
	}
	for _, sh := range []string{"bash", "sh"} {
		for name, tok := range shapes {
			in := "fatal: Authentication failed with " + tok + " for refs/heads/main\n"
			cmd := exec.Command(sh, "-c", `. "$1"; pushcred_mask`, sh, src)
			cmd.Stdin = strings.NewReader(in)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s %s: %v\n%s", sh, name, err, out)
			}
			got := string(out)
			for _, part := range []string{tok, "SUFFIXSENTINEL", urlSecretSentinel, strings.Repeat("b", 59)} {
				if strings.Contains(tok, part) && strings.Contains(got, part) {
					t.Errorf("%s: %s shape survived masking: %q", sh, name, got)
				}
			}
			if !strings.Contains(got, "***") || !strings.Contains(got, "refs/heads/main") {
				t.Errorf("%s: %s: want the token masked and the rest kept, got %q", sh, name, got)
			}
		}
	}
}

// modeShim is `git` for TestPushcredRawLogMode: on push it records the mode of
// every file in TMPDIR (the raw log is open for writing at that moment), then
// fails with a credential-bearing line.
const modeShim = `#!/bin/sh
for f in "$TMPDIR"/*; do
  [ -f "$f" ] && ls -l "$f" | cut -c1-10 >>"$MODE_LOG"
done
echo "fatal: Authentication failed for https://u:${URL_SECRET}@forge.test/o/r.git/" >&2
exit 1
`

// TestPushcredRawLogMode pins that the unmasked push output and the masked
// log are owner-only under a permissive umask: the raw file is a mktemp file
// and the masked log is written under umask 077.
//
// regression: #2910
func TestPushcredRawLogMode(t *testing.T) {
	requirePushTools(t)
	src, err := filepath.Abs("pushcred.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, sh := range []string{"bash", "sh"} {
		bin, tmp, logs := t.TempDir(), t.TempDir(), t.TempDir()
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte(modeShim), 0o755); err != nil {
			t.Fatal(err)
		}
		modeLog := filepath.Join(logs, "modes")
		caller := filepath.Join(tmp, "caller.log")
		for _, preset := range []string{"", caller} {
			cmd := exec.Command(sh, "-c", `umask 022; . "$1"; PUSHCRED_LOG=$2; pushcred_push origin HEAD:main 2>/dev/null; echo "rc=$?"`, sh, src, preset)
			cmd.Env = append(scrubbedEnv(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"TMPDIR="+tmp, "MODE_LOG="+modeLog, "URL_SECRET="+urlSecretSentinel)
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "rc=1") {
				t.Fatalf("%s: pushcred_push: %v\n%s", sh, err, out)
			}
		}
		b, err := os.ReadFile(modeLog)
		if err != nil || len(b) == 0 {
			t.Fatalf("%s: the shim saw no file in TMPDIR during the push (%v)", sh, err)
		}
		for _, m := range strings.Fields(string(b)) {
			if m != "-rw-------" {
				t.Errorf("%s: a push log was %s while git wrote to it under umask 022, want -rw-------", sh, m)
			}
		}
		st, err := os.Stat(caller)
		if err != nil {
			t.Fatalf("%s: caller-named PUSHCRED_LOG not written: %v", sh, err)
		}
		if st.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s: caller-named PUSHCRED_LOG has mode %v, want owner-only", sh, st.Mode().Perm())
		}
		if c, _ := os.ReadFile(caller); strings.Contains(string(c), urlSecretSentinel) {
			t.Errorf("%s: masked log carries the URL secret: %q", sh, c)
		}
		_ = os.Remove(caller)
	}
}

// pushRetryWording marks a run: block that retries a push to main as a race.
var pushRetryWording = regexp.MustCompile(`main moved|lost a race|moving faster than`)

// pushRetryAllowlist names staged files whose push loops predate the class
// and are tracked as their own follow-up, each with the reason.
var pushRetryAllowlist = map[string]string{
	"release.yml":           "changelog-roll push loop: follow-up, see the PR for #2910",
	"verify-gate-close.yml": "verify-gate close push loop: follow-up, see the PR for #2910",
}

// unclassifiedRetries returns the run: blocks of one workflow that retry a
// push as a race without classifying the failure through pushcred.
func unclassifiedRetries(t *testing.T, raw []byte, name string) []string {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	var bad []string
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				if k.Value == "run" && v.Kind == yaml.ScalarNode &&
					pushRetryWording.MatchString(v.Value) && !strings.Contains(v.Value, "pushcred_classify") {
					bad = append(bad, name+":"+strconv.Itoa(v.Line))
				}
				walk(v)
			}
			return
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)
	return bad
}

// TestPushRetryCallsPushcred is the class guard: every staged or activation
// run: block that retries a push to main as a race must classify the failure
// through pushcred first, so a new board writer cannot reintroduce "every
// failure is a race". A planted block proves the guard can fail.
//
// regression: #2910
func TestPushRetryCallsPushcred(t *testing.T) {
	planted := []byte("jobs:\n  j:\n    steps:\n      - run: |\n          git push || echo \"push rejected: main moved. retrying\"\n")
	if len(unclassifiedRetries(t, planted, "planted.yml")) != 1 {
		t.Fatal("positive control: the guard did not flag a planted unclassified retry loop")
	}
	var files []string
	for _, dir := range []string{"../ci/staged-workflows", "../tools/ci-load/activation"} {
		m, err := filepath.Glob(filepath.Join(dir, "*.yml"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Fatal("no staged workflows found")
	}
	used := map[string]bool{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		bad := unclassifiedRetries(t, raw, f)
		if _, ok := pushRetryAllowlist[filepath.Base(f)]; ok && strings.Contains(f, "staged-workflows") {
			if len(bad) > 0 {
				used[filepath.Base(f)] = true
			}
			continue
		}
		for _, b := range bad {
			t.Errorf("%s: a push retry loop reports every failure as a race; classify it with statusgen/pushcred.sh (pushcred_classify) first", b)
		}
	}
	for f := range pushRetryAllowlist {
		if !used[f] {
			t.Errorf("allowlist entry %s no longer matches an unclassified loop; remove it", f)
		}
	}
	// These tests run on the board workflow's PR lint job, or nowhere: the
	// repo-wide CI job builds and vets statusgen without testing it.
	const sel = `-run '^(TestBoardPush|TestGitlabRegen|TestPushcred|TestPushRetryCallsPushcred)' .`
	ran := false
	for _, st := range stagedWorkflow(t).Jobs["lint"].Steps {
		ran = ran || strings.Contains(st.Run, sel)
	}
	if !ran {
		t.Errorf("no lint job step of the staged board workflow runs go test %s", sel)
	}
}

// The http extraheader class (#5004). A runner's checkout can leave the CI job
// token in git config as an Authorization extraheader. git sends a configured
// header instead of asking any credential helper, so a board push that only
// ADDS a helper still authenticates as the job token, and the forge answers
// 403. These tests serve a real smart-HTTP remote (git http-backend behind
// net/http/cgi, loopback only) that records which credential each request
// carried and answers the way a forge does.

const (
	// jobHeaderSentinel is the job token inside the planted extraheader. Its
	// raw form and its base64 form must never reach output.
	jobHeaderSentinel = "JOB-HEADER-SENTINEL-5004"
	// boardTokenSentinel is the board push token, held in boardTokenVar.
	boardTokenSentinel = "BOARD-TOKEN-SENTINEL-5004"
	boardTokenVar      = "BOARD_PUSH_TOKEN"
)

func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

var (
	jobAuth   = basicAuth("gitlab-ci-token", jobHeaderSentinel)
	boardAuth = basicAuth("oauth2", boardTokenSentinel)
	// jobHeader is the extraheader value a runner's checkout would leave.
	jobHeader = "Authorization: " + jobAuth
)

// hdrRig is one workspace with a commit to push, and the forge it pushes to.
type hdrRig struct {
	t                  *testing.T
	dir, ws, url, gcfg string
	srvRepo            string
	env                []string
	mu                 sync.Mutex
	seen               []string // per request: none | job | board | other
}

// newHdrRig starts the forge: no credential gets a Basic challenge, the board
// credential is served by git http-backend, anything else is refused 403 with
// GitLab's wording for a job token that may not push.
func newHdrRig(t *testing.T) *hdrRig {
	t.Helper()
	requirePushTools(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	r := &hdrRig{t: t, dir: t.TempDir()}
	srvRoot := filepath.Join(r.dir, "srv")
	r.srvRepo = filepath.Join(srvRoot, "board.git")
	r.ws = filepath.Join(r.dir, "ws")
	// One quiet global config for the server and, by default, the client
	// (no auto maintenance: #3390). The global-scope case adds to it.
	r.gcfg = filepath.Join(r.dir, "gitconfig")
	if err := os.WriteFile(r.gcfg, []byte("[maintenance]\n\tauto = false\n\tautoDetach = false\n[gc]\n\tauto = 0\n\tautoDetach = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srvCfg := filepath.Join(r.dir, "srv-gitconfig")
	if err := os.WriteFile(srvCfg, []byte("[maintenance]\n\tauto = false\n[gc]\n\tauto = 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var base []string
	for _, kv := range scrubbedEnv() {
		k, _, _ := strings.Cut(kv, "=")
		switch k {
		case "GIT_ASKPASS", "SSH_ASKPASS", "http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY", "all_proxy", "ALL_PROXY":
			continue
		}
		base = append(base, kv)
	}
	r.env = append(base,
		"GIT_CONFIG_GLOBAL="+r.gcfg, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=job", "GIT_AUTHOR_EMAIL=job@example.invalid",
		"GIT_COMMITTER_NAME=job", "GIT_COMMITTER_EMAIL=job@example.invalid",
		"HOME="+r.dir, "TMPDIR="+r.dir, "no_proxy=*", "NO_PROXY=*",
	)
	backend := &cgi.Handler{
		Path: realGit, Args: []string{"http-backend"}, Root: "/git", Dir: srvRoot,
		Env: []string{"GIT_PROJECT_ROOT=" + srvRoot, "GIT_HTTP_EXPORT_ALL=1", "REMOTE_USER=board",
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + srvCfg, "HOME=" + r.dir},
		InheritEnv: []string{"PATH"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		who := "other"
		switch req.Header.Get("Authorization") {
		case "":
			who = "none"
		case jobAuth:
			who = "job"
		case boardAuth:
			who = "board"
		}
		r.mu.Lock()
		r.seen = append(r.seen, who)
		r.mu.Unlock()
		switch who {
		case "none":
			w.Header().Set("WWW-Authenticate", `Basic realm="forge"`)
			http.Error(w, "HTTP Basic: Access denied", http.StatusUnauthorized)
		case "board":
			backend.ServeHTTP(w, req)
		default:
			http.Error(w, "You are not allowed to push code to this project.", http.StatusForbidden)
		}
	}))
	t.Cleanup(srv.Close)
	r.url = srv.URL + "/git/board.git"
	r.git("", "init", "-q", "--bare", "-b", "main", r.srvRepo)
	r.git("", "clone", "-q", r.srvRepo, r.ws)
	if err := os.WriteFile(filepath.Join(r.ws, "STATUS.md"), []byte("board 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.git(r.ws, "add", "STATUS.md")
	r.git(r.ws, "commit", "-q", "-m", "board")
	r.git(r.ws, "remote", "set-url", "origin", r.url)
	return r
}

func (r *hdrRig) git(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// plant puts the job-token header where a runner could leave it. scope is
// local (url-scoped key in the checkout), plain (unscoped key in the
// checkout, the actions/checkout shape), global, env (GIT_CONFIG_COUNT) or
// both (scoped and unscoped in the checkout). It returns extra env for the
// push.
func (r *hdrRig) plant(scope string) []string {
	r.t.Helper()
	scoped := "http." + strings.TrimSuffix(r.url, "board.git") + ".extraheader"
	switch scope {
	case "local":
		r.git(r.ws, "config", scoped, jobHeader)
	case "plain":
		r.git(r.ws, "config", "http.extraheader", jobHeader)
	case "both":
		r.git(r.ws, "config", "http.extraheader", jobHeader)
		r.git(r.ws, "config", scoped, jobHeader)
	case "global":
		r.git("", "config", "--file", r.gcfg, scoped, jobHeader)
	case "env":
		return []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraheader", "GIT_CONFIG_VALUE_0=" + jobHeader}
	default:
		r.t.Fatalf("unknown scope %q", scope)
	}
	return nil
}

// push runs pushcred_push from the sourced script under sh, with extra env.
func (r *hdrRig) push(sh, pre string, env ...string) (int, string) {
	r.t.Helper()
	src, err := filepath.Abs("pushcred.sh")
	if err != nil {
		r.t.Fatal(err)
	}
	cmd := exec.Command(sh, "-c", pre+`. "$1"; pushcred_push "$2" HEAD:main; echo "rc=$?"`, sh, src, r.url)
	cmd.Dir = r.ws
	cmd.Env = append(append([]string{}, r.env...), env...)
	out, _ := cmd.CombinedOutput()
	m := regexp.MustCompile(`(?m)^rc=(\d+)$`).FindStringSubmatch(string(out))
	if m == nil {
		r.t.Fatalf("%s: pushcred_push printed no rc:\n%s", sh, out)
	}
	rc, _ := strconv.Atoi(m[1])
	return rc, string(out)
}

func (r *hdrRig) seenSet() map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := map[string]bool{}
	for _, w := range r.seen {
		s[w] = true
	}
	return s
}

// remoteMain is the forge's main, or "" when nothing reached it.
func (r *hdrRig) remoteMain() string {
	cmd := exec.Command("git", "--git-dir", r.srvRepo, "rev-parse", "-q", "--verify", "refs/heads/main")
	cmd.Env = r.env
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func readOr(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// assertNoHeaderValue: no form of either credential reaches the output.
func assertNoHeaderValue(t *testing.T, where, out string) {
	t.Helper()
	for _, s := range []string{jobHeaderSentinel, jobAuth, strings.TrimPrefix(jobAuth, "Basic "),
		boardTokenSentinel, strings.TrimPrefix(boardAuth, "Basic ")} {
		if strings.Contains(out, s) {
			t.Errorf("%s: a credential value reached the output:\n%s", where, out)
			return
		}
	}
}

// TestPushcredHeaderInstrument: the rig reproduces the defect. A credential
// helper added on top of the planted header (the shape pushcred_push used
// before #5004) authenticates as the header, the forge refuses it 403, and
// nothing reaches the remote. This is the instrument control for the tests
// below: if git ever stopped preferring the header, it would go red here.
//
// regression: #5004
func TestPushcredHeaderInstrument(t *testing.T) {
	for _, scope := range []string{"local", "plain", "global", "env"} {
		t.Run(scope, func(t *testing.T) {
			r := newHdrRig(t)
			env := r.plant(scope)
			cmd := exec.Command("git", "-c", "credential.helper=",
				"-c", `credential.helper=!f() { test "$1" = get || return 0; echo username=oauth2; printf 'password=%s\n' "$`+boardTokenVar+`"; }; f`,
				"push", r.url, "HEAD:main")
			cmd.Dir = r.ws
			cmd.Env = append(append(append([]string{}, r.env...), "GIT_TERMINAL_PROMPT=0", boardTokenVar+"="+boardTokenSentinel), env...)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("helper over a planted header pushed; the rig no longer reproduces #5004:\n%s", out)
			}
			seen := r.seenSet()
			if !seen["job"] || seen["board"] {
				t.Errorf("forge saw %v, want the job header and never the board credential", r.seen)
			}
			if r.remoteMain() != "" {
				t.Error("the refused push reached the remote")
			}
		})
	}
}

// TestPushcredHeaderCleared: with the job-token header planted in any scope,
// pushcred_push sends the board credential, never the header, and the push
// lands. No config file is changed: the header is reset for the one push only.
//
// regression: #5004
func TestPushcredHeaderCleared(t *testing.T) {
	for _, sh := range []string{"bash", "sh"} {
		for _, scope := range []string{"local", "plain", "both", "global", "env"} {
			t.Run(sh+"/"+scope, func(t *testing.T) {
				r := newHdrRig(t)
				env := r.plant(scope)
				local := filepath.Join(r.ws, ".git", "config")
				beforeL, beforeG := readOr(t, local), readOr(t, r.gcfg)
				rc, out := r.push(sh, "PUSHCRED_TOKEN_VAR="+boardTokenVar+"; ",
					append(env, boardTokenVar+"="+boardTokenSentinel)...)
				if rc != 0 {
					t.Fatalf("pushcred_push rc=%d with a %s extraheader, want 0:\n%s", rc, scope, out)
				}
				if seen := r.seenSet(); seen["job"] || !seen["board"] {
					t.Errorf("forge saw %v, want the board credential and never the job header", r.seen)
				}
				if got, want := r.remoteMain(), r.git(r.ws, "rev-parse", "HEAD"); got != want {
					t.Errorf("remote main = %q, want the pushed %q", got, want)
				}
				if readOr(t, local) != beforeL || readOr(t, r.gcfg) != beforeG {
					t.Error("pushcred_push edited a git config file; the reset must be for the one push only")
				}
				if scope != "env" && r.git(r.ws, "config", "--name-only", "--get-regexp", `^http\..*extraheader$`) == "" {
					t.Error("the planted header is gone from config; later git in the job loses its credential")
				}
			})
		}
	}
}

// TestPushcredHeaderNoLeak: a sentinel planted in the header never reaches
// stdout or stderr, xtrace on or off, and neither does the board token.
//
// regression: #5004
func TestPushcredHeaderNoLeak(t *testing.T) {
	for _, sh := range []string{"bash", "sh"} {
		for _, pre := range []string{"", "set -x; "} {
			mode := "quiet"
			if pre != "" {
				mode = "xtrace"
			}
			t.Run(sh+"/"+mode, func(t *testing.T) {
				r := newHdrRig(t)
				env := r.plant("both")
				rc, out := r.push(sh, pre+"PUSHCRED_TOKEN_VAR="+boardTokenVar+"; ",
					append(env, boardTokenVar+"="+boardTokenSentinel)...)
				if rc != 0 {
					t.Fatalf("pushcred_push rc=%d, want 0:\n%s", rc, out)
				}
				if !strings.Contains(out, "http.extraheader") && pre != "" {
					t.Errorf("xtrace shows no reset by name; the test is not exercising the reset:\n%s", out)
				}
				assertNoHeaderValue(t, sh+" "+pre, out)
			})
		}
	}
}

// TestPushcredNoHeaderControl: positive control. With no extraheader the
// push is unchanged: the forge challenges, the helper answers with the board
// credential, the push lands, and no reset is announced.
func TestPushcredNoHeaderControl(t *testing.T) {
	for _, sh := range []string{"bash", "sh"} {
		t.Run(sh, func(t *testing.T) {
			r := newHdrRig(t)
			rc, out := r.push(sh, "PUSHCRED_TOKEN_VAR="+boardTokenVar+"; ", boardTokenVar+"="+boardTokenSentinel)
			if rc != 0 {
				t.Fatalf("pushcred_push rc=%d, want 0:\n%s", rc, out)
			}
			if seen := r.seenSet(); !seen["none"] || !seen["board"] || seen["job"] || seen["other"] {
				t.Errorf("forge saw %v, want a challenge then the board credential", r.seen)
			}
			if strings.Contains(out, "extraheader") {
				t.Errorf("no header was planted, yet the output names a reset:\n%s", out)
			}
			assertNoHeaderValue(t, sh, out)
		})
	}
}

// TestPushcredHeaderKeptNoVar: without PUSHCRED_TOKEN_VAR the header IS the
// credential (a GitHub checkout persists it), so it is left alone.
func TestPushcredHeaderKeptNoVar(t *testing.T) {
	r := newHdrRig(t)
	r.git(r.ws, "config", "http.extraheader", "Authorization: "+boardAuth)
	rc, out := r.push("sh", "")
	if rc != 0 {
		t.Fatalf("pushcred_push rc=%d without a token variable, want 0 through the header:\n%s", rc, out)
	}
	if seen := r.seenSet(); !seen["board"] || seen["none"] {
		t.Errorf("forge saw %v, want the header's credential on every request", r.seen)
	}
}

// listFailShim is `git` when `git config` cannot list names: it exits 129
// (an unknown option), and records each push argv.
const listFailShim = `#!/bin/sh
for a in "$@"; do
  case $a in
    config) exit 129 ;;
    push) printf '%s\n' "$*" >>"$STUB_LOG/push-argv"; exit 0 ;;
  esac
done
exit 0
`

// TestPushcredHeaderListFails: when git cannot list the header names, the
// push still runs and says so as a could-not-check, never as a silent pass.
//
// regression: #5004
func TestPushcredHeaderListFails(t *testing.T) {
	requirePushTools(t)
	src, err := filepath.Abs("pushcred.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, sh := range []string{"bash", "sh"} {
		bin, logs := t.TempDir(), t.TempDir()
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte(listFailShim), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(sh, "-c", `. "$1"; PUSHCRED_TOKEN_VAR=`+boardTokenVar+`; pushcred_push origin HEAD:main; echo "rc=$?"`, sh, src)
		cmd.Env = append(scrubbedEnv(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"TMPDIR="+logs, "STUB_LOG="+logs)
		out, _ := cmd.CombinedOutput()
		if !strings.Contains(string(out), "rc=0") {
			t.Errorf("%s: push did not run to rc=0 after a failed listing:\n%s", sh, out)
		}
		if !strings.Contains(string(out), "could-not-check http extraheaders") {
			t.Errorf("%s: a failed listing was silent, want a could-not-check notice:\n%s", sh, out)
		}
		if b, _ := os.ReadFile(filepath.Join(logs, "push-argv")); !strings.Contains(string(b), "push origin HEAD:main") {
			t.Errorf("%s: push argv = %q", sh, b)
		}
	}
}
