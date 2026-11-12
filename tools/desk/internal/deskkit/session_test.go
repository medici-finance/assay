package deskkit

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// regression: #6147
// TestResolveSessionFlagWins pins the order: explicit flag → $DESK_SESSION →
// $CLAUDE_SESSION_ID → "".
func TestResolveSessionFlagWins(t *testing.T) {
	cases := []struct{ flag, desk, claude, want string }{
		{"b", "a", "c", "b"},
		{"  b ", "a", "c", "b"},
		{"", "a", "c", "a"},
		{"  ", " a ", "c", "a"},
		{"", "", "c", "c"},
		{"", "", "", ""},
	}
	for _, tc := range cases {
		t.Setenv("DESK_SESSION", tc.desk)
		t.Setenv("CLAUDE_SESSION_ID", tc.claude)
		if got := ResolveSessionName(tc.flag); got != tc.want {
			t.Errorf("flag=%q DESK_SESSION=%q CLAUDE_SESSION_ID=%q: got %q, want %q",
				tc.flag, tc.desk, tc.claude, got, tc.want)
		}
	}
}

// sessionEnvRead matches the NAME of a session environment variable as a quoted literal,
// so it sees a direct Getenv/LookupEnv read, a loop over both names, and a name handed to
// any helper alike. A matcher keyed to the Getenv call shape alone misses the loop form.
var sessionEnvRead = regexp.MustCompile(`"(DESK_SESSION|CLAUDE_SESSION_ID)"`)

// sessionEnvReaders lists the non-test Go files under the desk commands that name a session
// variable; each must be on this list with the reason it cannot put the environment ahead
// of an explicit --session. A file that resolves a session name for a command with a
// --session flag belongs on neither side of this list: it calls ResolveSessionName.
var sessionEnvReaders = map[string]string{
	"writeguard/guard.go":          "presence check (is this a desk session at all?), never resolves a name",
	"deskclaim/claim.go":           "resolveOwner reads --owner first, then the environment",
	"deskclaim-ref/args.go":        "defaultOwner is the fallback after --owner, which it consults first",
	"deskdispatch/dispatch.go":     "dispatchSessionSuffix takes no flag; it names the dispatcher's own session",
	"cellctl/codex_environment.go": "launcher: forwards the variable to a child, resolves no name",
	"cellctl/comms.go":             "launcher: sets the variable for a child, resolves no name",
	"cellctl/cobra.go":             "declares --session with the variable as its default; the flag layer applies the flag first",
	"cellctl/env.go":               "launcher: allow-list and value table for the child environment, resolves no name",
	"cellctl/envkeys.go":           "launcher: documents the variable in the environment-key table",
	"cellctl/helper_custody.go":    "launcher: sets the variable for a child, resolves no name",
	"cellctl/launch.go":            "launcher: sets the variable for a child, resolves no name",
	"cellctl/scratch_launch.go":    "reads the launched session's own value for scratch ownership; no --session flag is in play",
}

// regression: #6147
// TestSessionEnvReadsUseResolver closes the defect class: a desk command that reads
// $DESK_SESSION / $CLAUDE_SESSION_ID itself can put the environment ahead of an explicit
// --session again. Session-name resolution goes through ResolveSessionName; a new direct
// reader fails here until it is routed through the resolver or allow-listed with a reason.
func TestSessionEnvReadsUseResolver(t *testing.T) {
	// Positive control: the matcher flags a planted env-first resolver in both shapes the
	// tree uses, the direct read and the loop over both names.
	for _, planted := range []string{
		`if s := os.Getenv("DESK_SESSION"); s != "" {`,
		`for _, env := range []string{"DESK_SESSION", "CLAUDE_SESSION_ID"} {`,
	} {
		if !sessionEnvRead.MatchString(planted) {
			t.Fatalf("matcher no longer recognises a planted session read: %s", planted)
		}
	}
	root := filepath.Join("..", "..", "cmd")
	var offenders []string
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if !sessionEnvRead.Match(data) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		seen[rel] = true
		if _, ok := sessionEnvReaders[rel]; !ok {
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	// An allow-list entry that no longer names a session variable is stale: drop it, so the
	// list never grants a file an exemption it has no use for.
	for rel := range sessionEnvReaders {
		if !seen[rel] {
			offenders = append(offenders, "stale allow-list entry: "+rel)
		}
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Fatalf("these desk commands name a session env var outside the allow-list; resolve the "+
			"session with deskkit.ResolveSessionName so an explicit --session wins, or "+
			"allow-list the file with a reason: %v", offenders)
	}
}
