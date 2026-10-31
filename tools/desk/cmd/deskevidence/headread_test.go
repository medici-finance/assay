package main

import (
	"github.com/medici-finance/assay/gitquiet/goquiet"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #5023
func TestBoardHeadWithoutProcess(t *testing.T) {
	f, _ := setupFake(t)
	root := gitRoot(t, "fixture", "fixture@example.invalid")
	headShim(t, "all")
	_, err := decisionGateBranchBoard(f, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, root, "main", "docs/streams/x/README.md")
	if err != nil {
		t.Fatalf("canonical HEAD read spawned git: %v", err)
	}
}

// regression: #5023
func TestHeadReadProfiles(t *testing.T) {
	for _, profile := range []string{"normal", "nested", "shallow", "replacement", "environment", "blob", "missing"} {
		t.Run(profile, func(t *testing.T) {
			root := gitRoot(t, "", "")
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			native := false
			switch profile {
			case "nested":
				// The selected reader preserves native filesystem discovery on
				// Windows when a cwd-to-root boundary identity is unavailable.
				native = runtime.GOOS == "windows"
				root = filepath.Join(root, "nested")
				if err := os.Mkdir(root, 0o700); err != nil {
					t.Fatal(err)
				}
			case "shallow":
				native = true
				if err := os.WriteFile(filepath.Join(root, ".git", "shallow"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "replacement":
				native = true
				cmd := exec.Command(realGit, "-C", root, "update-ref", "refs/replace/"+strings.Repeat("a", 40), "HEAD")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("replace fixture: %v %s", err, out)
				}
			case "environment":
				native = true
				t.Setenv("GIT_CONFIG_COUNT", "0")
			case "blob":
				native = true
				cmd := exec.Command(realGit, "-C", root, "hash-object", "-w", "--stdin")
				cmd.Stdin = strings.NewReader("blob fixture")
				out, err := cmd.Output()
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), out, 0o600); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte(strings.Repeat("e", 40)+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, wantErr := nativeLandingHead(root)
			marker := headShim(t, "")
			got, gotErr := landingHead(root)
			if got != want || (gotErr != nil) != (wantErr != nil) {
				t.Fatalf("native parity: got %q/%v want %q/%v", got, gotErr, want, wantErr)
			}
			calls, err := os.ReadFile(marker)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			wantCalls := 0
			if native {
				wantCalls = 1
			}
			if len(calls) != wantCalls {
				t.Fatalf("native calls=%d want %d; errors must not select another reader", len(calls), wantCalls)
			}
		})
	}
}

// regression: #5023
func TestBaseHeadWithoutProcess(t *testing.T) {
	root := gitRoot(t, "", "")
	headShim(t, "rev-parse")
	if _, err := replacedBaseCommit(root, evidenceBrief, []byte("replacement\n"), true, nil); err != nil {
		t.Fatalf("base HEAD read spawned git: %v", err)
	}
}

// TestHeadMatched measures only the actual local HEAD operation. It makes no
// whole-tool, lock-wait, CPU or remote-request performance claim.
func TestHeadMatched(t *testing.T) {
	if os.Getenv("ASSAY_HEAD_COMPARE") != "1" {
		t.Skip("explicit fixture comparison")
	}
	root := gitRoot(t, "", "")
	want, err := nativeLandingHead(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := landingHead(root); err != nil || got != want {
		t.Fatalf("warm-up parity: %q %v", got, err)
	}
	for pair := 0; pair < 20; pair++ {
		order := []string{"native", "library"}
		if pair%2 == 1 {
			order[0], order[1] = order[1], order[0]
		}
		for _, kind := range order {
			read := landingHead
			if kind == "native" {
				read = nativeLandingHead
			}
			start := time.Now()
			got, err := read(root)
			elapsed := time.Since(start)
			if err != nil || got != want {
				t.Fatalf("sample parity: %q %v", got, err)
			}
			t.Logf("HEAD_SAMPLE pair=%d kind=%s elapsed_ns=%d output=%s", pair, kind, elapsed.Nanoseconds(), got)
		}
	}
}

// headShim is a compiled fixture so native-profile controls execute on every
// supported OS without relying on a POSIX shell or executable-script suffixes.
func headShim(t *testing.T, deny string) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	source := filepath.Join(bin, "shim.go")
	code := `package main
import("os";"os/exec")
func main(){
 f,e:=os.OpenFile(os.Getenv("HEAD_MARKER"),os.O_CREATE|os.O_WRONLY|os.O_APPEND,0600)
 if e!=nil {os.Exit(97)}
 f.WriteString("x");f.Close()
 deny:=os.Getenv("HEAD_SHIM_DENY")
 if deny=="all" {os.Exit(91)}
 for _,arg:=range os.Args[1:] {if deny!=""&&arg==deny {os.Exit(91)}}
 c:=exec.Command(os.Getenv("HEAD_REAL_GIT"),os.Args[1:]...)
 c.Stdin=os.Stdin;c.Stdout=os.Stdout;c.Stderr=os.Stderr
 if e:=c.Run();e!=nil {if x,ok:=e.(*exec.ExitError);ok {os.Exit(x.ExitCode())};os.Exit(98)}
}`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	name := "git"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(bin, name), source)
	if out, err := goquiet.Cmd(cmd).CombinedOutput(); err != nil {
		t.Fatalf("build native-profile fixture: %v %s", err, out)
	}
	marker := filepath.Join(bin, "calls")
	t.Setenv("HEAD_MARKER", marker)
	t.Setenv("HEAD_REAL_GIT", realGit)
	t.Setenv("HEAD_SHIM_DENY", deny)
	t.Setenv("PATH", bin)
	return marker
}
