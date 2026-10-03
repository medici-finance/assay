package main

// gitbatch.go — one git object reader per root, per run (forge-neutral/18 Task 4, Verify row 12).
//
// THE COST THIS REMOVES. A `--lint` used to start one `git show <rev>:<path>` process per read,
// and several checks read the same object independently: on this repository that was 122 `show`
// processes for 66 distinct objects, and 10 `merge-base` processes for two distinct questions.
// Process start, not work, was the price. Here every `<rev>:<path>` read goes through ONE
// long-lived `git cat-file --batch` per root, and a repeated read — a blob or a merge-base —
// is answered from the run's own memo.
//
// THE CACHE MUST NOT OUTLIVE ITS SUBJECT. The session is opened by run() and closed when run()
// returns, so no answer crosses a run boundary — a later run, in the same process, against a
// tree whose HEAD has since moved, re-reads. Inside a run the tree is not mutated, and the
// objects read are named by revision, so a memoised answer is the answer git would give again.
//
// SAME ANSWER, OR THE OLD PATH. A read the batch cannot answer EXACTLY the way `git show`
// would — an object that is not a blob (`git show` renders a tree as a listing, cat-file as raw
// bytes), an ambiguous name, a spec cat-file's line protocol cannot carry, a batch process that
// died — falls back to running `git show` itself, unchanged. Outside a session (a caller that is
// not under run(), or a unit test calling a check directly) every read is the old exec, so
// nothing that does not opt into a session changes behaviour.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// gitReadResult is one memoised answer: the bytes `Output()` would have returned and its error.
type gitReadResult struct {
	out []byte
	err error
}

// errGitObjectMissing is what a session returns for `<rev>:<path>` naming no object — the case
// in which `git show` exits 128. Every caller migrated onto gitShowObject tests only err != nil
// (none renders the error text), so the sentinel changes no output.
var errGitObjectMissing = errors.New("git object missing")

// gitReadSession is one run's git read memo plus its batch readers.
type gitReadSession struct {
	mu         sync.Mutex
	batches    map[string]*catFileBatch // per root; nil entry = could not start, use exec
	objects    map[string]gitReadResult // root + "\x00" + spec
	mergeBases map[string]gitReadResult // root + "\x00" + a + "\x00" + b
}

// activeGitReads is the session run() opened, or nil outside one.
var activeGitReads *gitReadSession

// beginGitReadSession opens a session for one run and returns its closer, which also restores
// whatever session (normally none) was active before.
func beginGitReadSession() (end func()) {
	prev := activeGitReads
	s := &gitReadSession{
		batches:    map[string]*catFileBatch{},
		objects:    map[string]gitReadResult{},
		mergeBases: map[string]gitReadResult{},
	}
	activeGitReads = s
	return func() {
		s.close()
		activeGitReads = prev
	}
}

func (s *gitReadSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.batches {
		if b != nil {
			b.close()
		}
	}
	s.batches = map[string]*catFileBatch{}
}

// gitShowObject is `git -C root show <rev>:<path>` — the bytes of that blob — answered through
// the run's batch reader when a session is open, and by exec otherwise.
func gitShowObject(root, rev, path string) ([]byte, error) {
	spec := rev + ":" + path
	s := activeGitReads
	if s == nil {
		return exec.Command("git", "-C", root, "show", spec).Output()
	}
	key := root + "\x00" + spec
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.objects[key]; ok {
		return cloneBytes(r.out), r.err
	}
	r := s.readObjectLocked(root, spec)
	s.objects[key] = r
	return cloneBytes(r.out), r.err
}

// readObjectLocked answers one spec through the root's batch reader, falling back to exec for
// anything the batch cannot answer exactly as `git show` would.
func (s *gitReadSession) readObjectLocked(root, spec string) gitReadResult {
	if batchableSpec(spec) {
		b, started := s.batches[root]
		if !started {
			b, _ = startCatFileBatch(root) // nil on failure: every read for this root execs
			s.batches[root] = b
		}
		if b != nil {
			data, kind, err := b.read(spec)
			switch {
			case err != nil:
				// The batch process is unusable from here on; retire it and use exec.
				b.close()
				s.batches[root] = nil
			case kind == "blob":
				return gitReadResult{out: data}
			case kind == "missing":
				return gitReadResult{err: fmt.Errorf("%w: %s", errGitObjectMissing, spec)}
			}
			// Any other type (tree, commit, tag) or an ambiguous name: `git show`
			// renders those differently from raw bytes, so ask it.
		}
	}
	out, err := exec.Command("git", "-C", root, "show", spec).Output()
	return gitReadResult{out: out, err: err}
}

// batchableSpec reports whether cat-file's one-name-per-line protocol carries spec verbatim. A
// `./`-relative path is excluded too: `git show` resolves it against -C's directory, and the
// batch is not relied on to do the same.
func batchableSpec(spec string) bool {
	if spec == "" || strings.ContainsAny(spec, "\n\r") || strings.TrimSpace(spec) != spec {
		return false
	}
	if i := strings.Index(spec, ":"); i >= 0 && strings.HasPrefix(spec[i+1:], "./") {
		return false
	}
	return true
}

// gitMergeBaseOut is `git -C root merge-base <a> <b>` with `.Output()` semantics, memoised per
// run on the exact argument order (with several best common ancestors git's pick may depend on
// it, so `a b` and `b a` are never conflated).
func gitMergeBaseOut(root, a, b string) ([]byte, error) {
	s := activeGitReads
	if s == nil {
		return exec.Command("git", "-C", root, "merge-base", a, b).Output()
	}
	key := root + "\x00" + a + "\x00" + b
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.mergeBases[key]; ok {
		return cloneBytes(r.out), r.err
	}
	out, err := exec.Command("git", "-C", root, "merge-base", a, b).Output()
	s.mergeBases[key] = gitReadResult{out: out, err: err}
	return cloneBytes(out), err
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	return append([]byte(nil), b...)
}

// --- the cat-file --batch reader -------------------------------------------------------------

type catFileBatch struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

func startCatFileBatch(root string) (*catFileBatch, error) {
	cmd := exec.Command("git", "-C", root, "cat-file", "--batch")
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &catFileBatch{cmd: cmd, in: in, out: bufio.NewReader(out)}, nil
}

// read asks for one object. kind is the object type for a found object, "missing" for a name
// that resolves to nothing, or "ambiguous". A non-nil err means the protocol broke and the
// reader must not be used again.
func (c *catFileBatch) read(spec string) (data []byte, kind string, err error) {
	if _, err := io.WriteString(c.in, spec+"\n"); err != nil {
		return nil, "", err
	}
	header, err := c.out.ReadString('\n')
	if err != nil {
		return nil, "", err
	}
	header = strings.TrimSuffix(header, "\n")
	if rest, ok := strings.CutPrefix(header, spec+" "); ok && (rest == "missing" || rest == "ambiguous") {
		return nil, rest, nil
	}
	f := strings.Fields(header)
	if len(f) != 3 {
		return nil, "", fmt.Errorf("cat-file --batch: unexpected header %q", header)
	}
	size, err := strconv.Atoi(f[2])
	if err != nil || size < 0 {
		return nil, "", fmt.Errorf("cat-file --batch: bad size in %q", header)
	}
	buf := make([]byte, size+1) // content plus cat-file's trailing LF
	if _, err := io.ReadFull(c.out, buf); err != nil {
		return nil, "", err
	}
	if buf[size] != '\n' {
		return nil, "", fmt.Errorf("cat-file --batch: missing record terminator after %q", header)
	}
	return buf[:size], f[1], nil
}

func (c *catFileBatch) close() {
	_ = c.in.Close()
	_ = c.cmd.Wait()
}
