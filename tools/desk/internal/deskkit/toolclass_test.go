package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ClassForTool is the security boundary the roster split introduces, and a
// correctness review found it with NO test at all: the mutation
// "ClassForTool ignores ciEligible", so every acting desk tool reads its roster
// from the environment under GITHUB_ACTIONS=true — survived the entire suite.
//
// The split-by-action-class ruling is what this pins. An ACTING tool
// (posts, flips, dispatches, admits to a queue) must load file-only, in CI as well
// as locally, because a locally-run acting tool is often driven by an agent reading
// untrusted text and an env override is one injected sentence away. Only report-class
// tools — which classify and print and grant nothing — may take the CI transport.
//
// Two instruments, deliberately. TestClassForToolHonoursEligibility pins the
// FUNCTION, which is where a one-token regression lands; TestActingToolsIgnoreAHostileEnvUnderCI
// pins the CONSEQUENCE end-to-end through the real loader, so a future refactor that
// keeps ClassForTool honest but reaches the environment by some other route is also
// caught. A test of the function alone would have been satisfied by a narrower mutation.

func TestClassForToolHonoursEligibility(t *testing.T) {
	cases := []struct {
		why        string
		ciEligible bool
		inCI       bool
		want       ToolClass
	}{
		{
			why:        "an ACTING tool locally: file-only, the restrictive default",
			ciEligible: false, inCI: false, want: ClassWrite,
		},
		{
			// The mutation's exact target. This is the assertion whose absence let the
			// mutation through: the other three rows are all satisfied by a
			// ClassForTool that ignores ciEligible entirely.
			why:        "an ACTING tool inside a runner: STILL file-only — ciEligible=false is unconditional",
			ciEligible: false, inCI: true, want: ClassWrite,
		},
		{
			why:        "a REPORT-class tool locally: no runner, so no CI transport",
			ciEligible: true, inCI: false, want: ClassWrite,
		},
		{
			why:        "a REPORT-class tool inside a runner: the one combination that reads the environment",
			ciEligible: true, inCI: true, want: ClassCI,
		},
	}
	for _, c := range cases {
		t.Run(c.why, func(t *testing.T) {
			if c.inCI {
				t.Setenv("GITHUB_ACTIONS", "true")
			} else {
				t.Setenv("GITHUB_ACTIONS", "")
			}
			if got := ClassForTool(c.ciEligible); got != c.want {
				t.Fatalf("ClassForTool(ciEligible=%t) with GITHUB_ACTIONS in-CI=%t = %v, want %v — %s",
					c.ciEligible, c.inCI, got, c.want, c.why)
			}
		})
	}
}

// TestClassForToolIsTheOnlyCIRoute pins the third combination's negative: InCI()
// alone must not be sufficient. Stated separately because the table above reads as
// four independent rows and this is the invariant they exist to express.
func TestClassForToolIsTheOnlyCIRoute(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	if !InCI() {
		t.Fatal("InCI() = false with GITHUB_ACTIONS=true — the positive control is broken, " +
			"so the assertion below would prove nothing")
	}
	if got := ClassForTool(false); got == ClassCI {
		t.Fatal("ClassForTool(false) = ClassCI inside a runner — being in CI is NOT sufficient " +
			"to take the environment transport; the tool must additionally be report-class. " +
			"Every acting desk main passes false here, so this would hand the whole fleet's " +
			"roster to whatever the environment says (the mutation this pins)")
	}
}

// TestActingToolsIgnoreAHostileEnvUnderCI is the end-to-end half: the roster is set
// in the ENVIRONMENT to an attacker identity, there is no config-home file, and the
// process is inside a runner. An acting tool must report UNCONFIGURED and refuse —
// it must not learn the attacker's login from the environment.
//
// This is the scenario run against statusgen --scan-issues
// at the binary level; here it is the deskkit loader, which is what the other eleven
// acting mains go through.
func TestActingToolsIgnoreAHostileEnvUnderCI(t *testing.T) {
	home := t.TempDir() // empty config home: nothing to load from the file transport
	t.Setenv("HOME", home)
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv(EnvBlessLogin, "attacker:999")
	t.Setenv(EnvTrustedLogins, "attacker:999")
	t.Setenv(EnvTrustedBotSlugs, "reviewer=attacker-app:998")
	t.Setenv(EnvAllowedRepos, "attacker/anything:no-ci:private")
	t.Cleanup(ReloadConfig)

	cfg := LoadConfig(ClassForTool(false))
	if cfg.Configured() {
		t.Fatalf("an acting tool loaded a CONFIGURED roster from the environment inside a runner. "+
			"Source reported: %q. The split-by-action-class ruling makes the environment a refused "+
			"route for every tool that acts", cfg.Source)
	}

	// The refusal must not be a technicality that still leaked the identity.
	SetToolClass(ClassForTool(false))
	t.Cleanup(func() { SetToolClass(ClassWrite) })
	ReloadConfig()
	if TrustedAuthor("attacker") {
		t.Fatal("TrustedAuthor(\"attacker\") = true — the hostile environment roster reached a " +
			"trust decision on an acting tool's load path")
	}
	if IsBlessAuthority("attacker") {
		t.Fatal("IsBlessAuthority(\"attacker\") = true — the environment named the blessing " +
			"authority for an acting tool")
	}

	// Positive control: the same loader, same hostile environment, but report-class —
	// this one IS allowed to read the environment. Without it, the assertions above
	// would also pass against a loader that reads nothing at all from anywhere.
	if got := LoadConfig(ClassCI); !got.Configured() {
		t.Fatalf("the report-class load did not pick the environment up (%v) — the negative "+
			"assertions above prove nothing unless this transport demonstrably works", got.Problems)
	}
}

// TestEveryActingMainPassesFalse is the source-text companion to the behavioural
// tests above: it is not enough for ClassForTool to be correct if a main declares
// itself report-class. The echo-coverage guard (echocoverage_test.go) already
// asserts that every roster-reading main CALLS ClassForTool; this asserts which
// ARGUMENT the acting ones pass.
//
// Report-class mains are listed explicitly, so promoting a tool into that set is a
// decision someone writes down rather than a one-token edit. There are none in
// cmd/ today: statusgen is the only report-class consumer and it lives in the other
// module, with its own scanClassForMode.
func TestEveryActingMainPassesFalse(t *testing.T) {
	reportClassMains := map[string]string{} // none in tools/desk today

	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("cannot enumerate %s: %v", cmdDir, err)
	}
	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		main := filepath.Join(cmdDir, e.Name(), "main.go")
		src, err := os.ReadFile(main)
		if err != nil {
			continue // not every command keeps its entrypoint in main.go
		}
		text := string(src)
		if !strings.Contains(text, "ClassForTool(") {
			continue
		}
		checked++
		if why, ok := reportClassMains[e.Name()]; ok {
			if !strings.Contains(text, "ClassForTool(true)") {
				t.Errorf("%s is listed as report-class (%s) but does not call ClassForTool(true)", main, why)
			}
			continue
		}
		if !strings.Contains(text, "ClassForTool(false)") {
			t.Errorf("%s calls ClassForTool but not with false. Every ACTING tool must be "+
				"file-only in CI as well as locally; if this tool really only classifies and "+
				"prints, add it to reportClassMains with the reason", main)
		}
	}
	if checked == 0 {
		t.Fatal("no main.go under " + cmdDir + " calls ClassForTool — this guard enumerated " +
			"nothing and would pass over a tree with the call deleted everywhere")
	}
}
