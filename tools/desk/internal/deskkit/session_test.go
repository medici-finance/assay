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

// sessionEnvRead matches a direct read of a session environment variable.
var sessionEnvRead = regexp.MustCompile(`(Getenv|LookupEnv)\("(DESK_SESSION|CLAUDE_SESSION_ID)"\)`)

// sessionEnvReaders lists the non-test Go files under the desk commands that read a
// session variable directly; each such file must be on the allow-list below.
var sessionEnvReaders = map[string]string{
	// Presence check only ("is this a desk session at all?"), never resolves a name.
	"writeguard/guard.go": "presence check",
	// resolveOwner: --owner flag first, then $DESK_SESSION, then SessionTag.
	"deskclaim/claim.go": "flag-first owner attribution",
}

// regression: #6147
// TestSessionEnvReadsUseResolver closes the defect class: a desk command that reads
// $DESK_SESSION / $CLAUDE_SESSION_ID itself can put the environment ahead of an explicit
// --session again. Session-name resolution goes through ResolveSessionName; a new direct
// reader fails here until it is routed through the resolver or allow-listed with a reason.
func TestSessionEnvReadsUseResolver(t *testing.T) {
	// Positive control: the matcher flags a planted env-first resolver.
	if !sessionEnvRead.MatchString(`if s := os.Getenv("DESK_SESSION"); s != "" {`) {
		t.Fatal("matcher no longer recognises a direct DESK_SESSION read")
	}
	root := filepath.Join("..", "..", "cmd")
	var offenders []string
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
		if _, ok := sessionEnvReaders[filepath.ToSlash(rel)]; !ok {
			offenders = append(offenders, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Fatalf("these desk commands read a session env var directly; resolve the session "+
			"with deskkit.ResolveSessionName so an explicit --session wins: %v", offenders)
	}
}
