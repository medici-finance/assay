// Package testfixture composes the actual authority owner in a source-bound,
// disposable container helper. It supplies no signing or current-state mock.
package testfixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/tools/desk/internal/cellcontainer"
	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
	"github.com/medici-finance/assay/tools/desk/internal/forgeservice/auth"
)

type wall struct{}

func (wall) Now() time.Time { return time.Now() }

type VerifyCall struct {
	LaunchID string
	Grant    auth.SignedGrant
	Scope    mirror.Scope
	Delay    time.Duration
}
type VerifyResult struct {
	Allowed             bool
	Actor, Role, Caller string
}
type ProbeCall struct {
	ProducerPID   int64
	OtherSlot     int
	OtherLaunchID string
}
type ProbeResult struct {
	PrivateAbsent, ControlDenied, EngineAbsent, ProducerMemoryDenied bool
	CurrentReadOnly, BootstrapReadOnly, GrantReadOnly                bool
	OtherGrantDenied, CurrentControlDenied, GrantControlDenied       bool
	BootstrapAbsent, CurrentAbsent, GrantAbsent                      bool
	ControlSocketPresent, CurrentSocketPresent, GatePresent          bool
	FixtureStage                                                     string
}

func read(in io.Reader, dst any) error {
	b, e := io.ReadAll(io.LimitReader(in, celllaunch.MaxRecordBytes+1))
	if e != nil || len(b) > celllaunch.MaxRecordBytes {
		return auth.ErrHeld
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil || d.Decode(new(any)) != io.EOF {
		return auth.ErrHeld
	}
	want, e := json.Marshal(dst)
	if e != nil || !bytes.Equal(want, b) {
		return auth.ErrHeld
	}
	return nil
}
func write(out io.Writer, v any) error {
	b, e := json.Marshal(v)
	if e != nil || len(b) > celllaunch.MaxRecordBytes {
		return auth.ErrHeld
	}
	_, e = out.Write(b)
	return e
}

// Run has a closed fixture command vocabulary. No command accepts a shell,
// filesystem path, arbitrary signing payload or engine operation from stdin.
func Run(ctx context.Context, op string, in io.Reader, out io.Writer) error {
	switch op {
	case "standby":
		<-ctx.Done()
		return nil
	case "producer":
		return producer(ctx, in)
	case "control":
		b, e := io.ReadAll(io.LimitReader(in, celllaunch.MaxRecordBytes+1))
		if e != nil {
			return auth.ErrHeld
		}
		c, e := auth.DecodeControlCall(b)
		if e != nil {
			return e
		}
		r, e := auth.RequestControl(ctx, "/authority-private/control.sock", c.MaxFrameBytes, c.IOTimeout, c.Command)
		if e != nil {
			return e
		}
		return write(out, r)
	case "read-grant":
		g, e := auth.ReadGrant(ctx, "/authority-grant/grant.sock", celllaunch.MaxRecordBytes, time.Second)
		if e != nil {
			return e
		}
		b, e := auth.EncodeGrant(g, celllaunch.MaxRecordBytes)
		if e != nil {
			return e
		}
		_, e = out.Write(b)
		return e
	case "verify":
		return verify(ctx, in, out)
	case "probe":
		var p ProbeCall
		if read(in, &p) != nil {
			return auth.ErrHeld
		}
		return write(out, probe(ctx, p))
	default:
		return auth.ErrHeld
	}
}

func producer(ctx context.Context, in io.Reader) error {
	b, e := io.ReadAll(io.LimitReader(in, celllaunch.MaxRecordBytes+1))
	if e != nil {
		return auth.ErrHeld
	}
	s, e := auth.DecodeProducerStart(b)
	if e != nil {
		return e
	}
	var roots []*os.Root
	defer func() {
		for _, r := range roots {
			r.Close()
		}
	}()
	open := func(p string) (*os.Root, error) {
		r, e := os.OpenRoot(p)
		if e == nil {
			roots = append(roots, r)
		}
		return r, e
	}
	private, e := open("/authority-private")
	if e != nil {
		return e
	}
	current, e := open("/authority-current")
	if e != nil {
		return e
	}
	bootstrap, e := open("/authority-bootstrap")
	if e != nil {
		return e
	}
	grants := map[string]*os.Root{}
	for _, g := range s.Grants {
		r, e := open(cellcontainer.AuthorityGrantTarget(g.Slot))
		if e != nil {
			return e
		}
		grants[g.LaunchID] = r
	}
	stage := func(value string) {
		f, e := private.OpenFile("fixture-stage", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if e == nil {
			f.Write([]byte(value))
			f.Close()
		}
	}
	stage("starting")
	c, e := auth.NewController(s.Config, auth.ProducerResources{Private: private, Current: current, Bootstrap: bootstrap, Grants: grants, Binding: s.Binding}, wall{})
	if e != nil {
		stage("constructor-held")
		return e
	}
	stage("gate-acquired")
	defer c.Close()
	// A real producer-only witness distinguishes mount leakage from an empty
	// image directory. It contains no key or identity and is never authority.
	f, e := private.OpenFile("producer-present", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	if _, e = f.Write([]byte("fixture")); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	run, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 2+len(grants))
	var wg sync.WaitGroup
	serve := func(fn func(context.Context) error) { wg.Add(1); go func() { defer wg.Done(); done <- fn(run) }() }
	serve(c.ServeControl)
	serve(func(ctx context.Context) error { return c.ServeCurrent(ctx, auth.CurrentChannel{}) })
	for id := range grants {
		serve(func(ctx context.Context) error { return c.ServeGrant(ctx, auth.GrantChannel{LaunchID: id}) })
	}
	var result error
	select {
	case <-ctx.Done():
	case result = <-done:
	}
	cancel()
	wg.Wait()
	if result != nil {
		stage("serve-held")
	} else {
		stage("stopped")
	}
	return result
}

func verify(ctx context.Context, in io.Reader, out io.Writer) error {
	var q VerifyCall
	if read(in, &q) != nil || q.Delay < 0 || q.Delay > time.Second {
		return auth.ErrHeld
	}
	held := func() error { return write(out, VerifyResult{}) }
	root, e := os.OpenRoot("/authority-bootstrap")
	if e != nil {
		return held()
	}
	defer root.Close()
	cfg, e := auth.LoadVerifierConfig(root, q.LaunchID, celllaunch.MaxRecordBytes)
	if e != nil {
		return held()
	}
	current, e := auth.NewCurrentClient(auth.CurrentConfig{Path: "/authority-current/current.sock", Bootstrap: cfg.Bootstrap, MaxFrameBytes: cfg.MaxFrameBytes, IOTimeout: time.Second}, wall{})
	if e != nil {
		return held()
	}
	v, e := auth.NewVerifier(cfg, current, wall{})
	if e != nil {
		return held()
	}
	r, e := v.Authorize(ctx, q.Grant, q.Scope)
	if e != nil {
		return held()
	}
	if q.Delay > 0 {
		timer := time.NewTimer(q.Delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return held()
		case <-timer.C:
		}
	}
	result := VerifyResult{Allowed: true, Actor: r.Actor(), Role: r.Role(), Caller: r.Caller().ID}
	// Prepare the detached output before the final release check.
	b, e := json.Marshal(result)
	if e != nil || v.CheckRelease(ctx, r) != nil {
		return held()
	}
	_, e = out.Write(b)
	return e
}

func absent(path string) bool { _, e := os.Lstat(path); return errors.Is(e, os.ErrNotExist) }
func readonly(path string) bool {
	f, e := os.OpenFile(filepath.Join(path, "fixture-write"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e == nil {
		f.Close()
		os.Remove(filepath.Join(path, "fixture-write"))
		return false
	}
	return errors.Is(e, syscall.EROFS)
}
func deniedSocket(ctx context.Context, path string) bool {
	r, e := auth.RequestControl(ctx, path, celllaunch.MaxRecordBytes, 250*time.Millisecond, nil)
	return e != nil && r.Version == 0 && r.TupleDigest == ""
}
func probe(ctx context.Context, p ProbeCall) ProbeResult {
	r := ProbeResult{PrivateAbsent: absent("/authority-private/producer-present"), ControlDenied: deniedSocket(ctx, "/authority-private/control.sock"), EngineAbsent: absent("/var/run/docker.sock") && absent("/run/docker.sock"), CurrentReadOnly: readonly("/authority-current"), BootstrapReadOnly: readonly("/authority-bootstrap"), GrantReadOnly: readonly("/authority-grant"), OtherGrantDenied: true, CurrentControlDenied: deniedSocket(ctx, "/authority-current/current.sock"), GrantControlDenied: deniedSocket(ctx, "/authority-grant/grant.sock"), BootstrapAbsent: absent("/authority-bootstrap/bootstrap.json"), CurrentAbsent: absent("/authority-current/current.sock"), GrantAbsent: absent("/authority-grant/grant.sock")}
	if p.OtherSlot >= 0 && p.OtherSlot < cellcontainer.AuthorityGrantSlots {
		conn, e := (&net.Dialer{Timeout: 250 * time.Millisecond}).DialContext(ctx, "unix", filepath.Join(cellcontainer.AuthorityGrantTarget(p.OtherSlot), "grant.sock"))
		r.OtherGrantDenied = e != nil
		if e == nil {
			conn.Close()
		}
	}
	if p.ProducerPID > 0 {
		f, e := os.Open("/proc/" + strconv.FormatInt(p.ProducerPID, 10) + "/mem")
		r.ProducerMemoryDenied = e != nil
		if e == nil {
			f.Close()
		}
	}
	socket := func(path string) bool { st, e := os.Lstat(path); return e == nil && st.Mode()&os.ModeSocket != 0 }
	r.ControlSocketPresent = socket("/authority-private/control.sock")
	r.CurrentSocketPresent = socket("/authority-current/current.sock")
	r.GatePresent = !absent("/authority-private/live.lock")
	if b, e := os.ReadFile("/authority-private/fixture-stage"); e == nil && len(b) < 32 {
		r.FixtureStage = string(b)
	}
	return r
}

func Main() int {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	op := "standby"
	if len(os.Args) == 2 {
		op = os.Args[1]
	} else if len(os.Args) > 2 {
		return 1
	}
	if Run(ctx, op, os.Stdin, os.Stdout) != nil {
		return 1
	}
	return 0
}
