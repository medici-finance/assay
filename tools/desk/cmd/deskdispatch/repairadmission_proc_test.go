package main

// Multi-process fixture for the example-stream/18 admission rows 2 and 4.
//
// The test binary re-executes itself in two helper roles, selected by ADMIT_HELPER before
// TestMain runs any test:
//
//   - claim: a claim tool speaking the deskkit contract (acquire/show/list/release; exit 0
//     acquired, 5 refused, 6 could not be established) over deskkit's file claim store. That
//     store is a real compare-and-set: a directory flock plus an O_EXCL create, with stale
//     (past-TTL) claims reclaimed in place on the next acquire.
//   - dispatch: one dispatcher process. It runs admitAndClaim — the same admit -> claim -> release
//     sequence dispatch() runs, with dispatch's stepClaim call as the claim step — against the live claim-tool admission backend, with the
//     production opt-in read from the environment. Only reservation() is pinned (the width store
//     lives under HOME). ADMIT_CRASH stops it at a boundary so the parent kills it there.
//
// Dispatchers are separate OS processes racing on one store, and a "crash" is a real kill.

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	admitRepo        = "medici-finance/assay"
	admitLeaseKey    = "assay--" + admissionLoop + "--admission"
	admitCrashMarker = "ADMIT-HELPER-AT-BOUNDARY"
	// crashLease stops a dispatcher after it holds the admission lease, before the item claim.
	crashLease = "lease"
	// crashClaim stops a dispatcher after the item claim is placed, before the lease release.
	crashClaim = "claim"
)

// admitHelperMain runs the helper role named by ADMIT_HELPER and reports its exit code; ok=false
// means this is an ordinary test run.
func admitHelperMain() (code int, ok bool) {
	switch os.Getenv("ADMIT_HELPER") {
	case "claim":
		return claimHelper(os.Args[1:]), true
	case "dispatch":
		return dispatchHelper(), true
	}
	return 0, false
}

// ---- helper role: the claim tool ---------------------------------------------------------------

func helperClaimCfg() deskkit.ClaimConfig {
	return deskkit.ClaimConfig{ClaimsDir: os.Getenv("ADMIT_STORE"), StaleClaim: claimedClaimTTL}
}

func claimHelper(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "dispatch-claim: no verb")
		return deskkit.ExitRefused
	}
	cfg := helperClaimCfg()
	key := ""
	if len(args) > 1 && !strings.HasPrefix(args[1], "--") {
		key = args[1]
	}
	switch args[0] {
	case "acquire":
		got, err := deskkit.Acquire(cfg, deskkit.Claim{Kind: deskkit.KindDispatch, Item: key,
			Owner: os.Getenv("ADMIT_OWNER")})
		if err != nil {
			fmt.Fprintln(os.Stderr, "dispatch-claim:", err)
			return deskkit.ExitUnverifiable
		}
		if !got {
			fmt.Println("dispatch-claim: DEDUP " + key + " is held by a live claim")
			return deskkit.ExitRefused
		}
		fmt.Println("dispatch-claim: CLAIMED " + key)
		return 0
	case "show":
		line, held, err := helperHeldLine(cfg, key)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dispatch-claim:", err)
			return deskkit.ExitUnverifiable
		}
		if !held {
			fmt.Println("dispatch-claim: FREE " + key)
			return 0
		}
		fmt.Println(line)
		return 0
	case "list":
		claims, err := deskkit.List(cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dispatch-claim:", err)
			return deskkit.ExitUnverifiable
		}
		if len(claims) == 0 {
			fmt.Println("(no dispatch claims)")
		}
		for _, c := range claims {
			if line, held, err := helperHeldLine(cfg, c.Item); err == nil && held {
				fmt.Println(line)
			}
		}
		return 0
	case "release":
		if err := deskkit.Release(cfg, key); err != nil {
			fmt.Fprintln(os.Stderr, "dispatch-claim:", err)
			return deskkit.ExitUnverifiable
		}
		return 0
	}
	fmt.Fprintln(os.Stderr, "dispatch-claim: unknown verb "+args[0])
	return deskkit.ExitRefused
}

// helperHeldLine renders one claim in the claim tool's HELD shape; its age comes from the claim
// file's mtime, the same clock the store's stale reclaim reads.
func helperHeldLine(cfg deskkit.ClaimConfig, key string) (string, bool, error) {
	path := deskkit.ClaimPath(cfg, key)
	fi, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	owner := "-"
	if claims, lerr := deskkit.List(cfg); lerr == nil {
		for _, c := range claims {
			if c.Item == key && c.Owner != "" {
				owner = c.Owner
			}
		}
	}
	age := int(time.Since(fi.ModTime()) / time.Minute)
	return fmt.Sprintf("dispatch-claim: HELD %s — dispatch-claim %s owner=%s state=claimed branch=- at=%s age=%dm",
		key, key, owner, fi.ModTime().UTC().Format(time.RFC3339), age), true, nil
}

// ---- helper role: one dispatcher ---------------------------------------------------------------

// pinnedWidthBackend is the live claim-tool backend with only reservation() pinned.
type pinnedWidthBackend struct {
	*claimToolAdmissionBackend
	width   int
	reserve map[string]int
}

func (b *pinnedWidthBackend) reservation() (int, map[string]int, error) {
	return b.width, b.reserve, nil
}

func dispatchHelper() int {
	width, err := strconv.Atoi(os.Getenv("ADMIT_WIDTH"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "dispatch helper: ADMIT_WIDTH:", err)
		return deskkit.ExitUnverifiable
	}
	reserve := map[string]int{}
	for _, kv := range strings.Split(os.Getenv("ADMIT_RESERVE"), ",") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			n, _ := strconv.Atoi(v)
			reserve[k] = n
		}
	}
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "dispatch helper:", err)
		return deskkit.ExitUnverifiable
	}
	item, key := os.Getenv("ADMIT_ITEM"), os.Getenv("ADMIT_KEY")
	crash := os.Getenv("ADMIT_CRASH")
	pause, _ := time.ParseDuration(os.Getenv("ADMIT_PAUSE"))

	if os.Getenv("ADMIT_BARRIER") == "1" {
		var b [1]byte
		_, _ = os.Stdin.Read(b[:]) // start line: the parent releases every contender at once
	}

	newAdmissionBackend = func(o dispatchOpts, plan dispatchPlan, repo string, auth claimAuth) admissionBackend {
		return &pinnedWidthBackend{
			claimToolAdmissionBackend: &claimToolAdmissionBackend{o: o, plan: plan, repo: repo, auth: auth, now: time.Now()},
			width:                     width, reserve: reserve,
		}
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		if len(args) >= 2 {
			atLease := crash == crashLease && args[0] == "acquire" && args[1] == key
			atClaim := crash == crashClaim && args[0] == "release" && args[1] == admitLeaseKey
			if atLease || atClaim {
				fmt.Println(admitCrashMarker)
				_ = os.Stdout.Sync()
				time.Sleep(10 * time.Minute) // the parent kills this process here
			}
			if pause > 0 && args[0] == "acquire" && args[1] == key {
				time.Sleep(pause) // widen the count -> claim window a missing lease would expose
			}
		}
		return exec.Command(name, args...)
	}

	auth := claimAuth{env: withEnv(os.Environ(), "ADMIT_HELPER=claim"), source: "test claim helper"}
	o := dispatchOpts{root: os.Getenv("ADMIT_ROOT"), item: item, quiet: true}
	plan := dispatchPlan{claimTool: self, claimToolIsScript: true, claimKey: key}
	claim := func() error { // dispatch's own stepClaim call, verbatim
		return stepClaim(o, admitRepo, plan.claimTool, plan.claimToolIsScript, auth, plan.claimKey)
	}
	if err := admitAndClaim(o, plan, admitRepo, auth, claim); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return deskkit.ExitCodeOf(err)
	}
	return 0
}

// withEnv returns env with each KEY=value in set replacing any existing KEY.
func withEnv(env []string, set ...string) []string {
	drop := map[string]bool{}
	for _, kv := range set {
		k, _, _ := strings.Cut(kv, "=")
		drop[k] = true
	}
	out := make([]string, 0, len(env)+len(set))
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if !drop[k] {
			out = append(out, kv)
		}
	}
	return append(out, set...)
}

// ---- parent side ---------------------------------------------------------------------------------

// admitFixture is one scheduling scope: a claim store, a root carrying the repair sidecar, and the
// loop's width and reserve.
type admitFixture struct {
	t       *testing.T
	store   string
	root    string
	width   int
	reserve string // "rework=1"
}

func newAdmitFixture(t *testing.T, width int, reserve string) *admitFixture {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Skipf("cannot re-execute the test binary: %v", err)
	}
	if _, err := os.Stat(self); err != nil {
		t.Skipf("test binary not on disk: %v", err)
	}
	t.Setenv(deskkit.RootsEnv, "") // in-process restarts read THIS root's sidecar, as the children do
	return &admitFixture{t: t, store: t.TempDir(), root: t.TempDir(), width: width, reserve: reserve}
}

func (f *admitFixture) cfg() deskkit.ClaimConfig {
	return deskkit.ClaimConfig{ClaimsDir: f.store, StaleClaim: claimedClaimTTL}
}

// place records a live claim held by some other worker.
func (f *admitFixture) place(key string) {
	f.t.Helper()
	if ok, err := deskkit.Acquire(f.cfg(), deskkit.Claim{Kind: deskkit.KindDispatch, Item: key, Owner: "other"}); err != nil || !ok {
		f.t.Fatalf("place %s: ok=%v err=%v", key, ok, err)
	}
}

// age backdates a claim past the claim tool's TTL, as wall time would for a dead holder.
func (f *admitFixture) age(key string) {
	f.t.Helper()
	old := time.Now().Add(-(claimedClaimTTL + 10*time.Minute))
	if err := os.Chtimes(deskkit.ClaimPath(f.cfg(), key), old, old); err != nil {
		f.t.Fatalf("age %s: %v", key, err)
	}
}

func (f *admitFixture) held(key string) bool {
	_, err := os.Stat(deskkit.ClaimPath(f.cfg(), key))
	return err == nil
}

// items lists every item claim in the store (the lease excluded).
func (f *admitFixture) items() []string {
	f.t.Helper()
	claims, err := deskkit.List(f.cfg())
	if err != nil {
		f.t.Fatal(err)
	}
	var out []string
	for _, c := range claims {
		if c.Item != admitLeaseKey {
			out = append(out, c.Item)
		}
	}
	return out
}

func (f *admitFixture) cmd(item, crash string, pause time.Duration, barrier bool) *exec.Cmd {
	self, _ := os.Executable()
	c := exec.Command(self)
	set := []string{
		"ADMIT_HELPER=dispatch", "ADMIT_STORE=" + f.store, "ADMIT_ROOT=" + f.root,
		"ADMIT_ITEM=" + item, "ADMIT_KEY=" + claimKeyFor(item, admitRepo),
		"ADMIT_OWNER=dispatcher-" + strconv.Itoa(os.Getpid()) + "-" + item,
		"ADMIT_WIDTH=" + strconv.Itoa(f.width), "ADMIT_RESERVE=" + f.reserve,
		"ADMIT_CRASH=" + crash, "ADMIT_PAUSE=" + pause.String(),
		deskkit.EnvRepairAdmission + "=on", deskkit.RootsEnv + "=",
	}
	if barrier {
		set = append(set, "ADMIT_BARRIER=1")
	}
	c.Env = withEnv(os.Environ(), set...)
	return c
}

// run runs one dispatcher to completion and returns its exit code and stderr.
func (f *admitFixture) run(item string) (int, string) {
	f.t.Helper()
	c := f.cmd(item, "", 0, false)
	var stderr strings.Builder
	c.Stderr = &stderr
	return exitOf(f.t, c.Run()), stderr.String()
}

// runSerialized runs a dispatcher and retries it while another dispatcher holds the lease (exit
// 6, "holds the serialization lease"), the retry-next-tick the gate asks for. first, when non-nil,
// is an already-started first attempt whose stderr is firstErr. It returns the final exit, its
// stderr, and how many attempts met a held lease.
func (f *admitFixture) runSerialized(item string, first *exec.Cmd, firstErr *strings.Builder) (int, string, int) {
	contended := 0
	for attempt := 0; attempt < 200; attempt++ {
		var code int
		var said string
		if first != nil {
			code, said = exitOf(f.t, first.Wait()), firstErr.String()
			first = nil
		} else {
			code, said = f.run(item)
		}
		if code == deskkit.ExitUnverifiable && strings.Contains(said, "holds the serialization lease") {
			contended++
			time.Sleep(25 * time.Millisecond)
			continue
		}
		return code, said, contended
	}
	// Called from race's goroutines too, so report through the result, never t.Fatal.
	return -1, "dispatcher for " + item + " never got past the serialization lease", contended
}

// crash starts a dispatcher, waits until it reaches the boundary, and kills it there.
func (f *admitFixture) crash(item, boundary string) {
	f.t.Helper()
	c := f.cmd(item, boundary, 0, false)
	out, err := c.StdoutPipe()
	if err != nil {
		f.t.Fatal(err)
	}
	var stderr strings.Builder
	c.Stderr = &stderr
	if err := c.Start(); err != nil {
		f.t.Fatal(err)
	}
	reached := make(chan bool, 1)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if strings.TrimSpace(sc.Text()) == admitCrashMarker {
				reached <- true
				return
			}
		}
		reached <- false
	}()
	select {
	case ok := <-reached:
		if !ok {
			_ = c.Wait()
			f.t.Fatalf("dispatcher for %s exited before the %s boundary: %s", item, boundary, stderr.String())
		}
	case <-time.After(60 * time.Second):
		_ = c.Process.Kill()
		_ = c.Wait()
		f.t.Fatalf("dispatcher for %s never reached the %s boundary", item, boundary)
	}
	if err := c.Process.Kill(); err != nil {
		f.t.Fatalf("kill dispatcher: %v", err)
	}
	_ = c.Wait()
}

func exitOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	t.Errorf("dispatcher did not run: %v", err) // goroutine-safe; -1 fails the caller's exit check
	return -1
}

// race releases two dispatchers for two different items at the same instant; each holds its
// lease across a widened count -> claim window. It returns both exits and the contention seen.
func (f *admitFixture) race(a, b string) (codes [2]int, errs [2]string, contended int) {
	f.t.Helper()
	items := [2]string{a, b}
	var cmds [2]*exec.Cmd
	var bufs [2]strings.Builder
	var gates [2]interface{ Close() error }
	for i, it := range items {
		cmds[i] = f.cmd(it, "", 300*time.Millisecond, true)
		cmds[i].Stderr = &bufs[i]
		in, err := cmds[i].StdinPipe()
		if err != nil {
			f.t.Fatal(err)
		}
		gates[i] = in
		if err := cmds[i].Start(); err != nil {
			f.t.Fatal(err)
		}
	}
	for _, g := range gates {
		_ = g.Close() // EOF on stdin: the start line for both
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := range items {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code, stderr, n := f.runSerialized(items[i], cmds[i], &bufs[i])
			mu.Lock()
			codes[i], errs[i], contended = code, stderr, contended+n
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return codes, errs, contended
}

// sidecarObligations re-reads the sidecar from disk — a restarted process's view.
func sidecarObligations(t *testing.T, root, id string) deskkit.RepairObligation {
	t.Helper()
	be := &claimToolAdmissionBackend{o: dispatchOpts{root: root, item: id}, now: time.Now()}
	all, err := be.reconciledObligations()
	if err != nil {
		t.Fatalf("reconcile after restart: %v", err)
	}
	var found []deskkit.RepairObligation
	for _, o := range all {
		if o.ID == id {
			found = append(found, o)
		}
	}
	if len(found) != 1 {
		t.Fatalf("restart: want exactly one obligation %s, got %d", id, len(found))
	}
	return found[0]
}

// sidecarLines counts the raw lines (the append-only log, not the fold).
func sidecarLines(t *testing.T, root string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, repairSidecarRel))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "\n")
}
