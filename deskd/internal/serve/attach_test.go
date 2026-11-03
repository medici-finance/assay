package serve_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/serve"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// countingTarget is a recording fake driver: it counts factory builds and
// Attach calls and hands back a live session that never ends.
type countingTarget struct {
	mu        sync.Mutex
	factories int
	attaches  int
}

func (c *countingTarget) factory() (terminal.Target, error) {
	c.mu.Lock()
	c.factories++
	c.mu.Unlock()
	return c, nil
}

func (c *countingTarget) Attach(context.Context, *terminal.TargetSpec, terminal.Size) (terminal.Session, error) {
	c.mu.Lock()
	c.attaches++
	c.mu.Unlock()
	return &idleSession{ch: make(chan []byte)}, nil
}

func (c *countingTarget) counts() (factories, attaches int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.factories, c.attaches
}

type idleSession struct {
	ch   chan []byte
	once sync.Once
}

func (s *idleSession) Read() <-chan []byte   { return s.ch }
func (s *idleSession) Write([]byte) error    { return nil }
func (s *idleSession) Resize(int, int) error { return nil }
func (s *idleSession) Alive() bool           { return true }
func (s *idleSession) Close() error          { s.once.Do(func() { close(s.ch) }); return nil }
func (c *countingTarget) lookup() terminal.DriverLookup {
	return func(schema.TargetKind) (func() (terminal.Target, error), bool) { return c.factory, true }
}

// The local kind's GLOBAL registration is this test binary's recording fake.
// The serve package links no real driver (Verify row 2), but its test binary
// does link the real container driver: the service contract's container rows
// (v1_terminal_container_test.go) import it, and it registers the container
// kind from init(). So the fake takes the kind nothing in this binary
// registers, and TestGetNeverTriggersAttach also asserts, kind-agnostically,
// that no desk (the container one included) ends up with a session.
// Registered once for the whole binary so -count=N does not double-register.
var (
	globalFakeOnce sync.Once
	globalFake     = &countingTarget{}
)

func registerGlobalFake(t *testing.T) {
	t.Helper()
	globalFakeOnce.Do(func() {
		if _, taken := terminal.DriverFor(schema.TargetLocal); taken {
			t.Fatal("could-not-check: another driver already registered the local kind in this test binary")
		}
		terminal.RegisterDriver(schema.TargetLocal, globalFake.factory)
	})
}

// recordingDockerOnPath puts a `docker` first on PATH that appends its argv
// to a log file and fails, and returns the log's path.
func recordingDockerOnPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "docker.log")
	script := "#!/bin/sh\necho \"$@\" >> '" + log + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// flushSignalRecorder is an httptest.ResponseRecorder that closes flushed on
// the handler's first Flush, the point at which a stream route has finished
// its pre-stream work and settled into streaming, and settled on its second,
// which a stream route makes only from inside its streaming loop.
type flushSignalRecorder struct {
	*httptest.ResponseRecorder
	mu      sync.Mutex
	flushes int
	flushed chan struct{}
	settled chan struct{}
}

func newFlushSignalRecorder() *flushSignalRecorder {
	return &flushSignalRecorder{ResponseRecorder: httptest.NewRecorder(), flushed: make(chan struct{}), settled: make(chan struct{})}
}

func (r *flushSignalRecorder) Flush() {
	r.ResponseRecorder.Flush()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushes++
	switch r.flushes {
	case 1:
		close(r.flushed)
	case 2:
		close(r.settled)
	}
}

func attachCfg() *schema.File {
	return &schema.File{Cells: []schema.Cell{{
		Name: "assay", Repos: []string{"example-org/console"}, SilenceThreshold: "48h",
		Loops: []schema.Loop{
			{Role: schema.RoleReview, App: "assay-reviewer-app", Target: &terminal.TargetSpec{Kind: schema.TargetContainer, Container: "c-review"}},
			{Role: schema.RoleVerify, App: "assay-verifier-app", Target: &terminal.TargetSpec{Kind: schema.TargetLocal, Command: []string{"/bin/sh"}}},
		},
	}}}
}

// TestGetNeverTriggersAttach is Verify row 7 — the single control between "a
// request" and "an exec". With the supervisor NOT started and a recording
// fake driver registered for the desk's kind, a GET to EVERY KindRead and
// KindEvent route in contract.Routes (a stream route included, the day it
// exists) builds no driver and attaches nothing.
func TestGetNeverTriggersAttach(t *testing.T) {
	registerGlobalFake(t)
	f0, a0 := globalFake.counts()
	// The container kind resolves to the REAL container driver in this test
	// binary, and that driver's only way to a container is the docker CLI. A
	// recording `docker` first on PATH counts every invocation (and attaches
	// nothing), so a read that tried to attach the container desk is seen too.
	dockerLog := recordingDockerOnPath(t)

	db, err := index.Open(filepath.Join(t.TempDir(), "idx.db"))
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	cfg := attachCfg()
	reg := terminal.NewRegistry(cfg)
	bus := serve.NewEventBus()
	h := serve.NewV1(db, cfg, nil, nil, nil, reg, bus, nil)
	// A heartbeat every 10 ms, so the terminal stream flushes again from
	// inside its loop soon after its first flush (see the walk below).
	defer serve.SetTerminalStreamTimings(10*time.Millisecond, 0, 0)()

	walked := 0
	for _, r := range contract.Routes {
		if r.Method != contract.GET || (r.Kind != contract.KindRead && r.Kind != contract.KindEvent) {
			continue
		}
		paths := []string{r.Path}
		switch {
		case strings.Contains(r.Path, "/desks/{id}"):
			// Both desks: the container one and the local one the fake
			// records for.
			paths = []string{strings.Replace(r.Path, "{id}", "assay/review", 1), strings.Replace(r.Path, "{id}", "assay/verify", 1)}
		case strings.Contains(r.Path, "/cells/{id}"):
			paths = []string{strings.Replace(r.Path, "{id}", "assay", 1)}
		default:
			paths = []string{strings.Replace(r.Path, "{id}", "IT1", 1)}
		}
		for _, path := range paths {
			// No deadline on the request context (F-cor-6). The container
			// half of this walk is observed ONLY by the recording `docker`,
			// and the driver runs it with exec.CommandContext on the request
			// context: a context that has ended either kills the recorder
			// before it logs or stops it from starting at all, and the walk
			// then reads clean. So the context ends only on signals the
			// handler gives itself. A route that returns has finished all of
			// its work. A stream route is followed through two flushes: the
			// first ends its pre-stream work, and the second comes from inside
			// its streaming loop (a heartbeat on the terminal stream, a
			// published event on /events; no desk here has scrollback, so the
			// terminal stream's only pre-loop flush is its opening
			// terminal.state event). So whatever it runs before the
			// loop, both before and after the first flush (an attach attempt
			// and its docker call included), has finished before the context
			// is cancelled. That closes the post-first-flush shape the round-8
			// review named: a lazy attach placed after the stream's first
			// flush now runs its docker call on a live context.
			ctx, cancel := context.WithCancel(context.Background())
			req := httptest.NewRequest(http.MethodGet, path+"?cell=assay", nil).WithContext(ctx)
			rec := newFlushSignalRecorder()
			done := make(chan struct{})
			go func() { defer close(done); h.ServeHTTP(rec, req) }()
			select {
			case <-done:
			case <-rec.flushed:
				bus.Publish(contract.EventIndexHealth, map[string]string{"walk": path})
				select {
				case <-done:
				case <-rec.settled:
				case <-time.After(5 * time.Second):
					cancel()
					t.Fatalf("GET %s flushed once and then neither returned nor flushed from its stream loop within 5s", path)
				}
			case <-time.After(5 * time.Second):
				cancel()
				t.Fatalf("GET %s neither returned nor flushed a stream within 5s", path)
			}
			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatalf("GET %s did not return after its context ended", path)
			}
		}
		walked++
	}
	if walked < 3 {
		t.Fatalf("walked %d GET routes, want every read and event route (the contract lists more)", walked)
	}
	f1, a1 := globalFake.counts()
	if f1 != f0 || a1 != a0 {
		t.Fatalf("GETs built the driver %d time(s) and attached %d time(s), want 0 and 0 — a read must never cause an exec",
			f1-f0, a1-a0)
	}
	if b, err := os.ReadFile(dockerLog); err == nil && len(b) > 0 {
		t.Fatalf("GETs ran the docker CLI — a read must never cause an exec:\n%s", b)
	}
	// Kind-agnostic: no desk the walk named holds a session afterwards,
	// whichever driver its kind resolves to in this binary.
	for _, id := range []string{"assay/review", "assay/verify"} {
		th, ok := reg.Resolve(id)
		if !ok {
			// Not vacuous: the desks the GETs named are configured rows.
			t.Fatalf("the desk %s the walk targeted is not configured", id)
		}
		if reg.Alive(id) || th.Connected || th.Attach.Status != terminal.AttachOff {
			t.Fatalf("after the GET walk %s reads alive=%v connected=%v attach=%s — a read must never cause an exec",
				id, reg.Alive(id), th.Connected, th.Attach.Status)
		}
	}
}

// TestTerminalCapabilityFlow is Verify row 101's flow half (report F-2): an
// unsupported host is decided before any attempt and reaches the reader as an
// actionable row — registry → supervisor → GET /v1/desks → the wire type the
// generated client is built from. A supported host serving the same config is
// independently usable: the capability is the HOST's, so a Windows client
// reading a Linux deskd sees it attached.
func TestTerminalCapabilityFlow(t *testing.T) {
	t.Run("windows-host-unsupported", func(t *testing.T) {
		cfg := attachCfg()
		reg := terminal.NewRegistry(cfg)
		fake := &countingTarget{}
		sup := terminal.NewSupervisor(reg, nil, terminal.SupervisorOptions{Host: terminal.Host{OS: "windows", Arch: "amd64"}, Drivers: fake.lookup()})
		sup.Start(context.Background())
		defer sup.Stop()

		rows := getDesks(t, cfg, reg)
		for _, row := range rows {
			if row.Attach.Status != terminal.AttachFailed || row.Attach.Reason != terminal.ReasonUnsupported {
				t.Errorf("%s attach = %s/%s, want failed/unsupported", row.ID, row.Attach.Status, row.Attach.Reason)
			}
			c := row.Attach.Capability
			if c.HostOS != "windows" || c.Support != terminal.SupportUnsupported || c.Reason != terminal.CapReasonNoPTYBackend || c.Profile != terminal.ProfileTrustedHost || c.Backend == "" {
				t.Errorf("%s capability = %+v, want windows / unsupported / no-pty-backend / trusted-host with a named backend", row.ID, c)
			}
			if row.State != string(terminal.StateCouldNotCheck) || row.Attach.NextAttemptAt != nil {
				t.Errorf("%s state=%s next=%v, want could-not-check and no retry", row.ID, row.State, row.Attach.NextAttemptAt)
			}
		}
		time.Sleep(100 * time.Millisecond)
		if f, a := fake.counts(); f != 0 || a != 0 {
			t.Fatalf("unsupported host built the driver %d time(s) and attached %d time(s), want 0 — never retry, never spawn", f, a)
		}
	})
	t.Run("linux-host-supported", func(t *testing.T) {
		cfg := attachCfg()
		reg := terminal.NewRegistry(cfg)
		fake := &countingTarget{}
		sup := terminal.NewSupervisor(reg, nil, terminal.SupervisorOptions{Host: terminal.Host{OS: "linux", Arch: "amd64"}, Drivers: fake.lookup()})
		sup.Start(context.Background())
		defer sup.Stop()
		deadline := time.Now().Add(5 * time.Second)
		for {
			rows := getDesks(t, cfg, reg)
			ok := true
			for _, row := range rows {
				ok = ok && row.Attach.Status == terminal.AttachAttached && row.Attach.Capability.Support == terminal.SupportSupported
			}
			if ok {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("a supported host never read attached/supported: %+v", rows)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	t.Run("generated-client-carries-it", func(t *testing.T) {
		for _, f := range []string{"../contract/openapi.yaml", "../../cockpit/src/client/v1.d.ts"} {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("could-not-check: %v", err)
			}
			for _, want := range []string{"AttachStatus", "AttachCapability", "next_attempt_at", "host_os", "support"} {
				if !strings.Contains(string(b), want) {
					t.Errorf("%s lacks %q — regenerate (go generate ./internal/contract/ && npm run gen-client)", f, want)
				}
			}
		}
	})
}

func getDesks(t *testing.T, cfg *schema.File, reg *terminal.Registry) []contract.DeskRow {
	t.Helper()
	h := serve.NewV1(nil, cfg, nil, nil, nil, reg, nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/desks?cell=assay", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/desks = %d: %s", rec.Code, rec.Body.String())
	}
	var resp contract.DesksResponse
	if err := decode(t, rec, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Desks) != 2 {
		t.Fatalf("rows = %d, want 2", len(resp.Desks))
	}
	return resp.Desks
}
