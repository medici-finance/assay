package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

type acceptanceBarrier struct {
	mu       sync.Mutex
	warnings int
	blocked  chan struct{}
	release  chan struct{}
}

func (w *acceptanceBarrier) Write(p []byte) (int, error) {
	w.mu.Lock()
	stop := false
	if strings.Contains(string(p), "running UNPINNED") {
		w.warnings++
		stop = w.warnings == 2
	}
	w.mu.Unlock()
	if stop {
		close(w.blocked)
		<-w.release
	}
	return len(p), nil
}

// regression: #5023
func TestAcceptanceIsolation(t *testing.T) {
	f, _ := setupFake(t)
	targetA, targetB := "docs/streams/x/a.md", "docs/streams/x/b.md"
	rootA := rootWithFile(t, targetA, "# A\n")
	rootB := rootWithFile(t, targetB, "# B\n")
	f.setFile(targetA, "old A")
	f.setFile(targetB, "old B")
	bin := acceptanceStub(t)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	barrier := &acceptanceBarrier{blocked: make(chan struct{}), release: make(chan struct{})}
	oldErr, oldOut := stderr, stdout
	stderr, stdout = barrier, &syncWriter{}
	oldDiff, oldLint := lintDiffFn, statusgenLintFn
	t.Cleanup(func() { stderr, stdout = oldErr, oldOut; lintDiffFn, statusgenLintFn = oldDiff, oldLint })
	entered := make(chan struct{})
	var once sync.Once
	lintDiffFn = lintDiffAt
	statusgenLintFn = func(root string, options childOptions) ([]string, error) {
		if root == rootA {
			once.Do(func() { close(entered); <-barrier.blocked })
		}
		return statusgenLintAt(root, options)
	}
	doneA, doneB := make(chan int, 1), make(chan int, 1)
	go func() {
		doneA <- run([]string{"example-org/tracker", "main", "--root", rootA, "--evidence-file", targetA})
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first invocation did not enter its child check")
	}
	go func() {
		doneB <- run([]string{"example-org/tracker", "main", "--root", rootB, "--evidence-file", targetB, "--acceptance-forge"})
	}()
	select {
	case <-barrier.blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("second invocation did not reach post-parse boundary")
	}
	codeA := <-doneA
	close(barrier.release)
	codeB := <-doneB
	if codeA != 0 || codeB != 0 {
		t.Fatalf("invocation results A=%d B=%d", codeA, codeB)
	}
	for _, c := range []struct {
		root string
		want int
	}{{rootA, 0}, {rootB, 2}} {
		argv, err := os.ReadFile(filepath.Join(c.root, "child-argv"))
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(argv), "--acceptance-forge\n"); got != c.want {
			t.Errorf("child opt-in count=%d want=%d", got, c.want)
		}
	}
}

func acceptanceStub(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	source := filepath.Join(bin, "child.go")
	code := `package main
import("os";"path/filepath";"strings")
func main(){
 root:="";for i,a:=range os.Args {if a=="--root"&&i+1<len(os.Args){root=os.Args[i+1]}}
 if root==""{os.Exit(96)}
 f,e:=os.OpenFile(filepath.Join(root,"child-argv"),os.O_CREATE|os.O_WRONLY|os.O_APPEND,0600)
 if e!=nil{os.Exit(97)}
 f.WriteString(strings.Join(os.Args[1:],"\n")+"\n");f.Close()
}`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	name := "statusgen"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if out, err := goquiet.Cmd(exec.Command("go", "build", "-o", filepath.Join(bin, name), source)).CombinedOutput(); err != nil {
		t.Fatalf("build child fixture: %v %s", err, out)
	}
	return bin
}
