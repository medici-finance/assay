package main

// The container bracket's credit check (desktools-v2/71 Task 8, Verify row 10).
// Every case runs on a fixture repository with a github.com origin that is never
// contacted, the fixture roster from TestMain, and a fake deskread on PATH that
// serves envelopes rendered from testdata/credit-records/. No forge is used.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const (
	creditBriefRel = "docs/streams/demo/brief-01-demo.md"
	creditRepo     = "example-org/tracker"
	creditTree     = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	creditClosed   = "2026-10-22T00:05:00Z"
	creditRow      = "| 1 | `true` | pass exit=0 sandbox=container-netns | sha256:aaaaaaaaaaaa | 2026-10-22 | assay-verifier-app[bot] @ bbbbbbbbbbbb |"
	creditPreface  = "# Demo brief\n\n## Evidence\n\n"
)

// creditRecordFixture is one testdata/credit-records file. Row 11's parity test
// in deskkit reads the same files.
type creditRecordFixture struct {
	Why         string          `json:"why"`
	Expect      string          `json:"expect"`
	Repo        string          `json:"repo"`
	Number      int             `json:"number"`
	Issue       json.RawMessage `json:"issue"`
	Trust       json.RawMessage `json:"trust"`
	LabelEvents json.RawMessage `json:"labelEvents"`
}

func loadCreditRecordFixtures(t *testing.T) map[string]creditRecordFixture {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "credit-records", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no credit-record fixtures: %v", err)
	}
	out := map[string]creditRecordFixture{}
	for _, p := range paths {
		var f creditRecordFixture
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&f); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out[strings.TrimSuffix(filepath.Base(p), ".json")] = f
	}
	return out
}

// creditFakeDeskread renders every fixture into deskread envelopes (one file per
// kind and issue number) and puts a fake deskread on PATH that serves them. It
// exits non-zero when CREDIT_FAKE_FAIL is set or no envelope exists.
func creditFakeDeskread(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: the fake deskread is a #!/bin/sh script on PATH")
	}
	dir, bin := t.TempDir(), t.TempDir()
	for _, f := range loadCreditRecordFixtures(t) {
		for kind, payload := range map[string][2]any{
			"issue":        {"issue", f.Issue},
			"trust":        {"trust", f.Trust},
			"label-events": {"labelEvents", f.LabelEvents},
		} {
			item := map[string]any{"repo": f.Repo, "number": f.Number, payload[0].(string): payload[1]}
			env, err := json.Marshal(map[string]any{"schema": deskreadSchema, "kind": kind, "items": []any{item}, "partial": []any{}})
			if err != nil {
				t.Fatal(err)
			}
			putFile(t, filepath.Join(dir, fmt.Sprintf("%s-%d.json", kind, f.Number)), string(env))
		}
	}
	writeStubBin(t, bin, "deskread", `#!/bin/sh
if [ -n "$CREDIT_FAKE_FAIL" ]; then echo "deskread: the forge is unreachable" >&2; exit 4; fi
n=${3##*#}
f="$CREDIT_FAKE_DIR/$1-$n.json"
[ -f "$f" ] || { echo "deskread: no fixture $f" >&2; exit 3; }
cat "$f"
`)
	t.Setenv("CREDIT_FAKE_DIR", dir)
	t.Setenv("CREDIT_FAKE_FAIL", "")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// creditAtt is a version-1 attestation line naming issue n of repo.
func creditAtt(repo string, n int, run, source, brief, model, tier string) string {
	return fmt.Sprintf("Verification-Attestation: %s#%d run=%s source=%s brief=%s model=%s tier=%s", repo, n, run, source, brief, model, tier)
}

// creditAttV2 is a version-2 (cross-repository) attestation line naming issue n
// of the tracking repository repo.
func creditAttV2(repo string, n int, sourceRepo, trackingSource string) string {
	return fmt.Sprintf("Verification-Attestation: %s#%d run=%s source-repo=%s source=%s tracking-source=%s brief=%s model=claude-opus-4 tier=strong",
		repo, n, fixtureRun, sourceRepo, fixtureSource, trackingSource, creditBriefRel)
}

// creditBaseAtt is the attestation rendered from fixture record n's base binding.
func creditBaseAtt(n int) string {
	return creditAtt(creditRepo, n, fixtureRun, fixtureSource, creditBriefRel, "claude-opus-4", "strong")
}

// creditBlock is a container witness block naming att, closed by a bracket that
// matches its span. closed is the bracket's closed= time.
func creditBlock(t *testing.T, att, closed string) string {
	t.Helper()
	span := witnessHeader + "\n" + creditRow + "\n" + treeDigestPrefix + creditTree + "\n" + containerImagePrefix + fixtureImage + "\n" + att + "\n"
	a, ok := parseContainerAttestation(att)
	if !ok {
		t.Fatalf("fixture attestation is not a version-1 or version-2 line: %q", att)
	}
	return span + fmt.Sprintf("%srun=%s source=%s tree=%s witness=%s opened=%s closed=%s before=receipt after=check-verifier\n",
		admissionBracketPrefix, a.run, a.source, creditTree, digestOf(t, span), fixtureOpened, closed)
}

type creditFx struct{ root, brief string }

// newCreditRepo is a repository with a github.com origin and the brief committed
// by a roster human, without any Evidence block yet.
func newCreditRepo(t *testing.T) creditFx {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "remote", "add", "origin", "https://github.com/"+creditRepo+".git")
	fx := creditFx{root: root, brief: filepath.Join(root, filepath.FromSlash(creditBriefRel))}
	if err := os.MkdirAll(filepath.Dir(fx.brief), 0o755); err != nil {
		t.Fatal(err)
	}
	fx.commit(t, vfMailHuman, creditPreface)
	return fx
}

func (fx creditFx) commit(t *testing.T, mail, text string) {
	t.Helper()
	putFile(t, fx.brief, text)
	vfGit(t, fx.root, mail, "add", "-A")
	vfGit(t, fx.root, mail, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "evidence")
}

// landed is a repository whose Evidence block for att the given author committed.
func landed(t *testing.T, mail, att string) creditFx {
	t.Helper()
	fx := newCreditRepo(t)
	fx.commit(t, mail, creditPreface+creditBlock(t, att, creditClosed))
	return fx
}

func (fx creditFx) credit(t *testing.T, source string) (int, string) {
	t.Helper()
	res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", fx.brief, "--credit", "--source", source})
	return res.code, out
}

// rosterWith is the TestMain fixture roster with the given keys replaced.
func rosterWith(over map[string]string) map[string]string {
	vals := map[string]string{}
	for _, l := range strings.Split(fixtureRoster, "\n") {
		if k, v, ok := strings.Cut(l, "="); ok && !strings.HasPrefix(strings.TrimSpace(l), "#") {
			vals[k] = v
		}
	}
	for k, v := range over {
		vals[k] = v
	}
	return vals
}

func TestBracketCredit(t *testing.T) {
	creditFakeDeskread(t)

	// The base case. The block matches its own bracket, so without --credit the
	// verb says verdict=match and exits 0; with it, credit=credited and exit 0.
	base := landed(t, vfMailVerifier, creditBaseAtt(41))
	if res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", base.brief}); res.code != verifyrunExitPass || !strings.Contains(out, " verdict=match ") || strings.Contains(out, "credit=") {
		t.Fatalf("base without --credit: exit %d output %q", res.code, out)
	}
	if code, out := base.credit(t, fixtureSource); code != verifyrunExitPass || !strings.Contains(out, " verdict=match ") || !strings.HasSuffix(strings.TrimSpace(strings.Split(out, "\n")[0]), " credit=credited") {
		t.Fatalf("base: exit %d output %q, want credit=credited exit 0", code, out)
	}

	type outcome struct {
		code    int
		verdict string
		why     string
	}
	notCredited := func(why string) outcome { return outcome{verifyrunExitFail, creditNotCredited, why} }
	couldNot := func(why string) outcome { return outcome{verifyrunExitCouldNot, creditCouldNotCheck, why} }
	check := func(t *testing.T, name string, fx creditFx, source string, want outcome) {
		t.Helper()
		// Every negative is a block whose bracket matches: --credit is what
		// tells it apart.
		if res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", fx.brief}); !strings.Contains(out, " verdict=match ") {
			t.Fatalf("%s: the fixture's bracket does not match (exit %d): %q", name, res.code, out)
		}
		code, out := fx.credit(t, source)
		if code != want.code || !strings.Contains(out, " credit="+want.verdict+"\n") || !strings.Contains(out, want.why) {
			t.Fatalf("%s: exit %d output %q, want exit %d credit=%s naming %q", name, code, out, want.code, want.verdict, want.why)
		}
	}

	// (a) provenance: every line from the header through the bracket must be
	// the bound verifier's, on the pinned id.
	t.Run("provenance", func(t *testing.T) {
		other := landed(t, fixtureWorkerEmail, creditBaseAtt(41))
		if res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", other.brief}); res.code != verifyrunExitPass {
			t.Fatalf("a non-verifier commit without --credit: exit %d output %q, want 0 (a matching bracket)", res.code, out)
		}
		check(t, "non-verifier commit", other, fixtureSource, notCredited("was not committed by the bound verifier"))
		check(t, "roster human", landed(t, vfMailHuman, creditBaseAtt(41)), fixtureSource, notCredited("was not committed by the bound verifier"))

		// Only the attestation line is the verifier's.
		onlyAtt := newCreditRepo(t)
		final := creditPreface + creditBlock(t, creditBaseAtt(41), creditClosed)
		onlyAtt.commit(t, vfMailHuman, strings.Replace(final, "model=claude-opus-4", "model=claude-sonnet-4", 1))
		onlyAtt.commit(t, vfMailVerifier, final)
		check(t, "only the attestation line is the verifier's", onlyAtt, fixtureSource, notCredited("was not committed by the bound verifier"))

		// Only the bracket line is the verifier's.
		onlyBracket := newCreditRepo(t)
		onlyBracket.commit(t, vfMailHuman, creditPreface+creditBlock(t, creditBaseAtt(41), "2026-10-22T00:06:00Z"))
		onlyBracket.commit(t, vfMailVerifier, final)
		check(t, "only the bracket line is the verifier's", onlyBracket, fixtureSource, notCredited("was not committed by the bound verifier"))

		// One span line rewritten later by another identity: a stamp on the
		// Runner cell, outside the digest, so the bracket still matches.
		rewritten := landed(t, vfMailVerifier, creditBaseAtt(41))
		rewritten.commit(t, fixtureWorkerEmail, strings.Replace(getFile(t, rewritten.brief), " @ bbbbbbbbbbbb |", " @ bbbbbbbbbbbb (on-behalf-of human:ada) |", 1))
		check(t, "one span line rewritten by another identity", rewritten, fixtureSource, notCredited("L7 was not committed by the bound verifier"))

		// Working-tree-only: a line changed but not committed, and a file
		// never committed.
		dirty := landed(t, vfMailVerifier, creditBaseAtt(41))
		putFile(t, dirty.brief, strings.Replace(getFile(t, dirty.brief), "\n"+containerImagePrefix+fixtureImage+"\n", "\n"+containerImagePrefix+fixtureImage+"  \n", 1))
		check(t, "an uncommitted line", dirty, fixtureSource, notCredited("an uncommitted line has no author"))
		wtOnly := newCreditRepo(t)
		putFile(t, wtOnly.brief, creditPreface+creditBlock(t, creditBaseAtt(41), creditClosed))
		check(t, "a bracket and span only in the working tree", wtOnly, fixtureSource, notCredited("an uncommitted line has no author"))
		untracked := newCreditRepo(t)
		runGit(t, untracked.root, "mv", creditBriefRel, "docs/streams/demo/brief-00-moved.md")
		vfGit(t, untracked.root, vfMailVerifier, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "move")
		putFile(t, untracked.brief, creditPreface+creditBlock(t, creditBaseAtt(41), creditClosed))
		check(t, "a file not in HEAD", untracked, fixtureSource, notCredited("is not in HEAD"))

		// A verifier entry with no numeric id is login-only: nothing can be
		// credited to it, and that is could-not-check, not credited.
		scanWithRoster(t, rosterWith(map[string]string{
			scanEnvTrustedBotSlugs: "desk=assay-desk-app:300000001,verifier=assay-verifier-app",
		}))
		check(t, "a verifier entry with no id", base, fixtureSource, couldNot("not id-pinned"))
	})

	// (b) record: the attestation is the line rendered from an admissible,
	// dispatcher-authored record in the brief's own repository.
	t.Run("record", func(t *testing.T) {
		for _, c := range []struct {
			n         int
			att, want string
		}{
			{42, creditBaseAtt(42), "differs from the line rendered from example-org/tracker#42 in run"},
			{43, creditBaseAtt(43), "in source"},
			{44, creditBaseAtt(44), "in brief"},
			{45, creditBaseAtt(45), "in model"},
			{46, creditBaseAtt(46), "in tier"},
			{47, creditBaseAtt(47), "not the dispatcher"},
			{48, creditBaseAtt(48), "has title"},
			{49, creditBaseAtt(49), "unknown field"},
			{50, creditBaseAtt(50), "not the exact rendering"},
			{51, creditBaseAtt(51), "body was edited"},
			{52, creditBaseAtt(52), "history is incomplete"},
			{53, creditBaseAtt(53), "bodyHistoryKnown is false or absent"},
			{59, creditBaseAtt(59), "bodyHistoryKnown is false or absent"},
			{54, creditBaseAtt(54), "not closed"},
			{55, creditBaseAtt(55), "no dispatcher model stamp"},
			{56, creditBaseAtt(56), "dispatcher stamp names model=claude-sonnet-4"},
			{57, creditBaseAtt(57), "tier=any"},
			{58, creditBaseAtt(58), "not the dispatcher"},
		} {
			check(t, fmt.Sprintf("record #%d", c.n), landed(t, vfMailVerifier, c.att), fixtureSource, notCredited(c.want))
		}
		check(t, "a record in another repository", landed(t, vfMailVerifier, creditAtt("example-org/agents", 41, fixtureRun, fixtureSource, creditBriefRel, "claude-opus-4", "strong")),
			fixtureSource, notCredited("is not in the brief's own repository example-org/tracker"))
		scanWithRoster(t, rosterWith(map[string]string{
			scanEnvTrustedBotSlugs: "desk=assay-verifier-app:300000005,verifier=assay-verifier-app:300000005",
		}))
		check(t, "a one-actor roster", base, fixtureSource, notCredited("to one actor"))
	})

	// (b) per bracket: two brackets naming one record are each compared with
	// that record, so a credited first bracket never carries the second.
	t.Run("record shared by two brackets", func(t *testing.T) {
		other := strings.Replace(creditBaseAtt(41), "model=claude-opus-4", "model=claude-sonnet-4", 1)
		for _, c := range []struct {
			name        string
			first, last string
			want        [2]string
		}{
			{"matching then differing", creditBaseAtt(41), other, [2]string{creditCredited, creditNotCredited}},
			{"differing then matching", other, creditBaseAtt(41), [2]string{creditNotCredited, creditCredited}},
		} {
			fx := newCreditRepo(t)
			fx.commit(t, vfMailVerifier, creditPreface+creditBlock(t, c.first, creditClosed)+"\n"+creditBlock(t, c.last, creditClosed))
			code, out := fx.credit(t, fixtureSource)
			var lines []string
			for _, l := range strings.Split(out, "\n") {
				if strings.HasPrefix(l, "bracket=") {
					lines = append(lines, l)
				}
			}
			if code != verifyrunExitFail || len(lines) != 2 {
				t.Fatalf("%s: exit %d output %q, want exit 1 and two bracket lines", c.name, code, out)
			}
			for i, l := range lines {
				if !strings.Contains(l, " verdict=match ") || !strings.HasSuffix(l, " credit="+c.want[i]) {
					t.Fatalf("%s: bracket %d reads %q, want verdict=match and credit=%s", c.name, i+1, l, c.want[i])
				}
			}
			if !strings.Contains(out, "differs from the line rendered from example-org/tracker#41 in model") {
				t.Fatalf("%s: output %q does not name the model difference", c.name, out)
			}
		}
	})

	// (c) binding: the attestation names this brief and --source.
	t.Run("binding", func(t *testing.T) {
		copied := landed(t, vfMailVerifier, creditAtt(creditRepo, 44, fixtureRun, fixtureSource, "docs/streams/demo/brief-02-other.md", "claude-opus-4", "strong"))
		check(t, "a block copied from another brief", copied, fixtureSource, notCredited("binding: the attestation names brief=docs/streams/demo/brief-02-other.md"))
		check(t, "another source", base, strings.Repeat("c", 40), notCredited("binding: the attestation names source="+fixtureSource))
	})

	// What the check cannot read is could-not-check, exit 2, never credited.
	t.Run("could-not-check", func(t *testing.T) {
		shallow := t.TempDir()
		runGit(t, shallow, "clone", "-q", "--depth", "1", "file://"+base.root, ".")
		runGit(t, shallow, "remote", "set-url", "origin", "https://github.com/"+creditRepo+".git")
		check(t, "a shallow clone", creditFx{root: shallow, brief: filepath.Join(shallow, filepath.FromSlash(creditBriefRel))}, fixtureSource, couldNot("shallow clone"))

		t.Setenv("CREDIT_FAKE_FAIL", "1")
		check(t, "a failing deskread", base, fixtureSource, couldNot("the forge is unreachable"))
		t.Setenv("CREDIT_FAKE_FAIL", "")

		scanWithNoRoster(t)
		check(t, "no roster", base, fixtureSource, couldNot("roster"))
	})

	// --credit and --source go together, with --bracket-digest only.
	t.Run("usage", func(t *testing.T) {
		for _, args := range [][]string{
			{"--bracket-digest", "--witness", base.brief, "--credit"},
			{"--bracket-digest", "--witness", base.brief, "--source", fixtureSource},
			{"--bracket-digest", "--witness", base.brief, "--credit", "--source", "B" + fixtureSource[1:]},
			{"--credit", "--source", fixtureSource, "--witness", base.brief},
		} {
			if res, out := captureVerifyrunOut(t, args); res.code != verifyrunExitUsageError || !strings.Contains(out, "--source") {
				t.Fatalf("%q: exit %d output %q, want a usage error", args, res.code, out)
			}
		}
	})
}

// creditTrackingSource is the tracking-repository commit the version-2 fixture
// records (60, 61) name.
var creditTrackingSource = strings.Repeat("d", 40)

// regression: #4626
//
// TestBracketCreditCrossRepo: a version-2 (cross-repository) span is credited
// only when both its repositories pass the repository checks: the tracking
// repository is the brief's own, and the source repository is in the configured
// repository set. Records 60 and 61 differ only in their source repository.
func TestBracketCreditCrossRepo(t *testing.T) {
	creditFakeDeskread(t)
	check := func(t *testing.T, name string, att string, wantCode int, wantCredit, wantWhy string) {
		t.Helper()
		fx := landed(t, vfMailVerifier, att)
		if res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", fx.brief}); res.code != verifyrunExitPass || !strings.Contains(out, " verdict=match ") {
			t.Fatalf("%s: the fixture's bracket does not match (exit %d): %q", name, res.code, out)
		}
		code, out := fx.credit(t, fixtureSource)
		if code != wantCode || !strings.Contains(out, " credit="+wantCredit+"\n") || !strings.Contains(out, wantWhy) {
			t.Fatalf("%s: exit %d output %q, want exit %d credit=%s naming %q", name, code, out, wantCode, wantCredit, wantWhy)
		}
	}
	admitted := creditAttV2(creditRepo, 60, "example-org/agents", creditTrackingSource)
	unlisted := creditAttV2(creditRepo, 61, "example-org/unlisted", creditTrackingSource)

	check(t, "a configured source repository", admitted, verifyrunExitPass, creditCredited, "")
	check(t, "an unconfigured source repository", unlisted, verifyrunExitFail, creditNotCredited,
		"source repository example-org/unlisted is not in the configured repository set")

	// The source-repository check reads the roster, not the forge: with every
	// read failing, the unconfigured span is still not-credited, never
	// could-not-check.
	t.Setenv("CREDIT_FAKE_FAIL", "1")
	check(t, "an unconfigured source repository, forge unreachable", unlisted, verifyrunExitFail, creditNotCredited,
		"is not in the configured repository set")
	t.Setenv("CREDIT_FAKE_FAIL", "")

	// The tracking repository must be the brief's own, as for version 1.
	check(t, "a record in another tracking repository", creditAttV2("example-org/agents", 60, creditRepo, creditTrackingSource),
		verifyrunExitFail, creditNotCredited, "is not in the brief's own repository example-org/tracker")

	// The span's version-2 fields are compared with the record's.
	check(t, "another source repository than the record's", creditAttV2(creditRepo, 60, "example-org/examples", creditTrackingSource),
		verifyrunExitFail, creditNotCredited, "differs from the line rendered from example-org/tracker#60 in source-repo")
	check(t, "another tracking source than the record's", creditAttV2(creditRepo, 60, "example-org/agents", strings.Repeat("e", 40)),
		verifyrunExitFail, creditNotCredited, "differs from the line rendered from example-org/tracker#60 in tracking-source")

	// The set is the roster's: an owner/* entry admits the same span.
	allowed := rosterWith(nil)[scanEnvAllowedRepos]
	scanWithRoster(t, rosterWith(map[string]string{scanEnvAllowedRepos: allowed + ",example-org/*"}))
	check(t, "a source repository matched by an owner pattern", unlisted, verifyrunExitPass, creditCredited, "")
}

// regression: #4626
//
// TestAllowedRepoMirror holds allowedRepo to deskkit IsAllowedRepo's answers:
// an explicit entry or an owner/* pattern, exact-case, and false for anything
// that is not one owner/name slug or when nothing is configured.
func TestAllowedRepoMirror(t *testing.T) {
	cfg := scanConfig{Repos: map[string]string{"example-org/tracker": "ci:private", "other-org/*": "ci:unknown"}}
	for repo, want := range map[string]bool{
		"example-org/tracker": true,
		"example-org/agents":  false,
		"Example-org/tracker": false,
		"other-org/anything":  true,
		"Other-org/anything":  false,
		"other-org/*":         false,
		"other-org/a/b":       false,
		"other-org":           false,
		"/tracker":            false,
		"":                    false,
	} {
		if got := cfg.allowedRepo(repo); got != want {
			t.Errorf("allowedRepo(%q) = %v, want %v", repo, got, want)
		}
	}
	if (scanConfig{}).allowedRepo("example-org/tracker") {
		t.Error("an unconfigured set admitted a repository")
	}
}

// TestCreditRecordFixtures is statusgen's half of row 11: the record reader
// admits or refuses each shared fixture as its `expect` says. deskkit's
// TestVerifierAttestationCreditParity reads the same files.
func TestCreditRecordFixtures(t *testing.T) {
	creditFakeDeskread(t)
	fixtures := loadCreditRecordFixtures(t)
	names := make([]string, 0, len(fixtures))
	for n := range fixtures {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		f := fixtures[name]
		// The line to credit is the one rendered from the fixture's own
		// binding when it decodes, so an admitted record is judged on its
		// admission alone.
		att := creditBaseAtt(f.Number)
		var iss struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal(f.Issue, &iss); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, raw, ok := strings.Cut(iss.Body, "\n\n"); ok {
			var b creditBinding
			if json.Unmarshal([]byte(raw), &b) == nil && b.Run != "" {
				att = creditRenderedLine(b, f.Number)
			}
		}
		a, ok := parseContainerAttestation(att)
		if !ok {
			t.Fatalf("%s: %q is not an attestation line", name, att)
		}
		cc := &creditContext{desk: "assay-desk-app", repo: creditRepo, rec: creditOK(), records: map[string]creditRecord{}}
		v := cc.record(a, att)
		admitted := v.verdict == creditCredited
		if admitted != (f.Expect == "admit") {
			t.Errorf("%s (%s): credit=%s %s, want %s", name, f.Why, v.verdict, v.why, f.Expect)
		}
	}
}
