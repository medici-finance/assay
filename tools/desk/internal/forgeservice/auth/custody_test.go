package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/tools/desk/internal/cellcontainer"
	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
	"github.com/medici-finance/assay/tools/desk/internal/forgeservice/auth"
	"github.com/medici-finance/assay/tools/desk/internal/forgeservice/auth/testfixture"
)

// custodyFailure is the fatal message for a failed custody step. It keeps the
// step's fixed text first (the mutation driver matches on it), then the error
// value and, once a producer exists, the producer's own state. An intermittent
// failure can then be read from one log line.
func custodyFailure(step string, err error, producer string) string {
	msg := step
	if err != nil {
		msg += ": " + err.Error()
	}
	if producer != "" {
		msg += " [producer: " + producer + "]"
	}
	return msg
}

// regression: #6717
func TestCustodyFailureNamesStepAndCause(t *testing.T) {
	cause := errors.New("example cause")
	if got, want := custodyFailure("operator initialize failed", cause, `fixture-stage="gate-acquired"; read-back version=0 ready=false`), `operator initialize failed: example cause [producer: fixture-stage="gate-acquired"; read-back version=0 ready=false]`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := custodyFailure("closed component creation failed producer", cause, ""), "closed component creation failed producer: example cause"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	witness := "independent role escaped signer/control/other-launch containment"
	if got := custodyFailure(witness, nil, "state"); !strings.HasPrefix(got, witness) {
		t.Fatalf("witness text no longer leads the message: %q", got)
	}
}

// errorDroppingFatals lists each t.Fatal that is the first statement under an
// "!= nil {" check yet does not report through custodyFailure, so drops the error.
func errorDroppingFatals(src string) []string {
	var bad []string
	lines := strings.Split(src, "\n")
	for i := 1; i < len(lines); i++ {
		prev, cur := strings.TrimSpace(lines[i-1]), strings.TrimSpace(lines[i])
		if strings.HasSuffix(prev, "!= nil {") && strings.HasPrefix(cur, "t.Fatal") && !strings.Contains(cur, "custodyFailure(") {
			bad = append(bad, fmt.Sprintf("line %d: %s", i+1, cur))
		}
	}
	return bad
}

// regression: #6717
func TestCustodyFatalsCarryTheError(t *testing.T) {
	src, e := os.ReadFile("custody_test.go")
	if e != nil {
		t.Fatal(custodyFailure("custody test source unreadable", e, ""))
	}
	if bad := errorDroppingFatals(string(src)); len(bad) != 0 {
		t.Fatalf("custody fatals drop the step's error; report through custodyFailure:\n%s", strings.Join(bad, "\n"))
	}
	// Positive control: one planted error-dropping fatal must be flagged.
	plant := "\tif _, e = exampleStep(); e != nil {\n\t\tt.Fatal(\"example step failed\")\n\t}\n"
	if got := errorDroppingFatals(plant); len(got) != 1 {
		t.Fatalf("guard no longer flags a planted error-dropping fatal: %q", got)
	}
}

// TestForgeLocalAuthorityCustody requires the explicit owned source-bound image
// packet. No default engine, registry, image, credential or live endpoint exists.
// A SKIP is UNQUALIFIED; only an actual named PASS qualifies this fixture.
func TestForgeLocalAuthorityCustody(t *testing.T) {
	path := os.Getenv("AUTH57_CUSTODY_INPUT")
	if path == "" {
		t.Skip("UNQUALIFIED: explicit owned authority fixture packet required")
	}
	var input struct{ DockerBinary, DockerConfig, Host, Engine, Image, Platform, Owner, Cell string }
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(custodyFailure("fixture input unavailable", e, ""))
	}
	if e = json.Unmarshal(b, &input); e != nil {
		t.Fatal(custodyFailure("fixture input invalid", e, ""))
	}
	// This fixture is explicitly local and cannot be redirected to a remote engine.
	if !strings.HasPrefix(input.Host, "unix:///") || !strings.HasPrefix(input.Owner, "auth57-") || !strings.HasPrefix(input.Cell, "auth57-") {
		t.Fatal("fixture locality/ownership mismatch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	volumes := cellcontainer.AuthorityVolumes{Private: input.Owner + "-control", Current: input.Owner + "-current", Bootstrap: input.Owner + "-bootstrap", Grants: []cellcontainer.AuthorityGrantVolume{{LaunchID: "caller-a", Volume: input.Owner + "-grant-a", Slot: 0}, {LaunchID: "caller-b", Volume: input.Owner + "-grant-b", Slot: 1}}}
	limits := cellcontainer.AuthorityRuntimeLimits{Pids: 32, Memory: 64 * 1024 * 1024, NanoCPUs: 500000000, TmpfsBytes: 8 * 1024 * 1024, CommandTimeout: 20 * time.Second, StopGrace: 2 * time.Second}
	type instance struct {
		runtime *cellcontainer.AuthorityRuntime
		handle  *cellcontainer.AuthorityHandle
	}
	all := []instance{}
	var producer *cellcontainer.AuthorityRuntime
	prepared := false
	fatal := func(step string, e error) {
		t.Helper()
		t.Fatal(custodyFailure(step, e, ""))
	}
	// producerState reads the producer's fixture stage and the operator
	// read-back for a failure message. It is diagnostic only and bounded.
	producerState := func(h *cellcontainer.AuthorityHandle, l *auth.Launcher) string {
		diag, c := context.WithTimeout(ctx, 10*time.Second)
		defer c()
		state := "probe unavailable"
		raw, _ := json.Marshal(testfixture.ProbeCall{OtherSlot: 1})
		if out, e := producer.Probe(diag, h, raw); e != nil {
			state = "probe failed: " + e.Error()
		} else {
			var r testfixture.ProbeResult
			if json.Unmarshal(out, &r) != nil {
				state = "probe invalid"
			} else {
				state = fmt.Sprintf("fixture-stage=%q control-socket=%t gate=%t", r.FixtureStage, r.ControlSocketPresent, r.GatePresent)
			}
		}
		if l == nil {
			return state
		}
		r, e := l.ReadControl(diag)
		if e != nil {
			return state + "; read-back failed: " + e.Error()
		}
		return state + fmt.Sprintf("; read-back version=%d ready=%t", r.Version, r.Ready)
	}
	fatalProducer := func(step string, e error, h *cellcontainer.AuthorityHandle, l *auth.Launcher) {
		t.Helper()
		t.Fatal(custodyFailure(step, e, producerState(h, l)))
	}
	t.Cleanup(func() {
		cleanup, c := context.WithTimeout(context.Background(), time.Minute)
		defer c()
		for i := len(all) - 1; i >= 0; i-- {
			if all[i].handle != nil && all[i].runtime.CloseOwned(cleanup, all[i].handle) != nil {
				t.Error("exact owned container cleanup failed")
			}
		}
		if prepared && producer.RemoveOwnedVolumes(cleanup) != nil {
			t.Error("exact owned volume cleanup failed")
		}
	})
	create := func(kind celllaunch.AuthorityKind, id string) instance {
		t.Helper()
		role := "worker"
		if kind == celllaunch.AuthorityProducer {
			role = ""
		}
		cfg := cellcontainer.AuthorityConfig{Schema: cellcontainer.AuthorityProfileSchema, Host: input.Host, Engine: input.Engine, DockerBinary: input.DockerBinary, DockerConfig: input.DockerConfig, Image: input.Image, Platform: input.Platform, Owner: input.Owner, Cell: input.Cell, LaunchID: id, Role: role, Kind: kind, Volumes: volumes, Limits: limits}
		p, e := cellcontainer.NewAuthorityPlan(cfg)
		if e != nil {
			fatal(fmt.Sprint("closed plan invalid ", kind), e)
		}
		r, e := cellcontainer.NewAuthorityRuntime(p)
		if e != nil {
			fatal(fmt.Sprint("closed runtime invalid ", kind), e)
		}
		if kind == celllaunch.AuthorityProducer {
			producer = r
			prepared = true
			if e = r.PrepareVolumes(ctx); e != nil {
				fatal("owned volume preparation failed", e)
			}
		}
		h, e := r.Create(ctx)
		all = append(all, instance{r, h})
		if e != nil {
			fatal(fmt.Sprint("closed component creation failed ", kind), e)
		}
		return instance{r, h}
	}
	p := create(celllaunch.AuthorityProducer, "producer")
	a := create(celllaunch.AuthorityCaller, "caller-a")
	bCaller := create(celllaunch.AuthorityCaller, "caller-b")
	q := create(celllaunch.AuthorityQuery, "query")
	sibling := create(celllaunch.AuthoritySibling, "sibling")
	scope := func(id string) auth.GrantScope {
		l := refresh.Limits{CallerFillsPerWindow: 2, AdmissionFillsPerWindow: 4, FillWindow: time.Minute, MinFreshness: time.Millisecond, MaxFreshness: time.Hour, ActiveFills: 1, QueuedCallers: 2, AdmissionFills: 2, AdmissionWaiters: 4, FailureEntries: 4, WaitTimeout: time.Second, FillTimeout: time.Second, Cooldown: time.Second, FailureTTL: time.Minute, Refresh: mirror.RefreshPolicy{FullInventoryInterval: time.Hour}}
		return auth.GrantScope{Scope: mirror.Scope{Repository: mirror.RepositoryID{Provider: mirror.GitHub, Host: "github.com", Path: "fixture/repository"}, Kind: mirror.Issues, AdmissionID: "fixture-admission"}, Caller: refresh.Caller{ID: id, AdmissionID: "fixture-admission", AccessEpoch: "access-a", ConfigEpoch: "config-a", PolicyClass: "read-class", PolicyEpoch: "policy-a", Limits: l}, Actor: "fixture-" + id, Role: "worker", Cell: input.Cell, SourceID: "fixture-source", CompositionID: "fixture-composition"}
	}
	cfg := auth.ProducerConfig{Schema: auth.Schema, Profile: auth.Profile, Issuer: "fixture-issuer", Audience: "fixture-audience", MaxGrantLease: time.Minute, ClockSkew: time.Second, CurrentReadLease: 100 * time.Millisecond, MaxRevocationBound: 100 * time.Millisecond, IOTimeout: time.Second, MaxFrameBytes: 64 * 1024, MaxGrants: 8, MaxConnections: 4}
	launcher, e := auth.NewLauncher(ctx, p.runtime, p.handle, cfg, []auth.LauncherRoute{{Runtime: a.runtime, Handle: a.handle, Scope: scope("caller-a")}, {Runtime: bCaller.runtime, Handle: bCaller.handle, Scope: scope("caller-b")}})
	if e != nil {
		fatalProducer("fresh protected launcher composition failed", e, p.handle, nil)
	}
	run, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- launcher.Run(run) }()
	t.Cleanup(func() {
		stop()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("producer exec did not stop")
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, e = launcher.ReadControl(ctx); e == nil {
			break
		}
		if time.Now().After(deadline) {
			fatalProducer("actual producer private channel unavailable", e, p.handle, nil)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, e = launcher.Initialize(ctx); e != nil {
		fatalProducer("operator initialize failed", e, p.handle, launcher)
	}
	if _, e = launcher.Register(ctx, "caller-a", time.Minute); e != nil {
		fatalProducer("fresh caller-a registration failed", e, p.handle, launcher)
	}
	if _, e = launcher.Register(ctx, "caller-b", time.Minute); e != nil {
		fatalProducer("fresh caller-b registration failed", e, p.handle, launcher)
	}
	grant := func(x instance) auth.SignedGrant {
		t.Helper()
		raw, e := x.runtime.ReadGrant(ctx, x.handle)
		if e != nil {
			fatal("predetermined grant channel unavailable", e)
		}
		g, e := auth.DecodeGrant(raw, cfg.MaxFrameBytes)
		if e != nil {
			fatal("grant envelope invalid", e)
		}
		return g
	}
	ga, gb := grant(a), grant(bCaller)
	verify := func(id string, g auth.SignedGrant, sc mirror.Scope, delay time.Duration) testfixture.VerifyResult {
		t.Helper()
		raw, _ := json.Marshal(testfixture.VerifyCall{LaunchID: id, Grant: g, Scope: sc, Delay: delay})
		out, e := q.runtime.Verify(ctx, q.handle, raw)
		if e != nil {
			fatal("independent query helper failed", e)
		}
		var r testfixture.VerifyResult
		if e = json.Unmarshal(out, &r); e != nil {
			fatal("query result invalid", e)
		}
		return r
	}
	allowed := verify("caller-a", ga, scope("caller-a").Scope, 0)
	if !allowed.Allowed || allowed.Actor != "fixture-caller-a" || allowed.Caller != "caller-a" || allowed.Role != "worker" {
		t.Fatal("actual producer/current/query positive failed")
	}
	held := func(r testfixture.VerifyResult) {
		t.Helper()
		if r.Allowed || r.Actor != "" || r.Role != "" || r.Caller != "" {
			t.Fatal("refusal released trusted identity")
		}
	}
	held(verify("caller-a", gb, scope("caller-a").Scope, 0))
	bad := auth.SignedGrant{Payload: ga.Payload, Signature: append([]byte(nil), ga.Signature...)}
	bad.Signature[0] ^= 1
	held(verify("caller-a", bad, scope("caller-a").Scope, 0))
	wrong := scope("caller-a").Scope
	wrong.Repository.Path = "fixture/other"
	held(verify("caller-a", ga, wrong, 0))
	held(verify("caller-a", ga, scope("caller-a").Scope, 150*time.Millisecond))
	probe := func(x instance) testfixture.ProbeResult {
		t.Helper()
		raw, _ := json.Marshal(testfixture.ProbeCall{ProducerPID: p.handle.HostPID(), OtherSlot: 1, OtherLaunchID: "caller-b"})
		out, e := x.runtime.Probe(ctx, x.handle, raw)
		if e != nil {
			fatal("containment witness unavailable", e)
		}
		var r testfixture.ProbeResult
		if e = json.Unmarshal(out, &r); e != nil {
			fatal("containment witness invalid", e)
		}
		return r
	}
	for _, x := range []instance{q, a, bCaller, sibling} {
		r := probe(x)
		if !r.PrivateAbsent || !r.ControlDenied || !r.EngineAbsent || !r.ProducerMemoryDenied || !r.OtherGrantDenied || !r.CurrentControlDenied || !r.GrantControlDenied {
			t.Fatal("independent role escaped signer/control/other-launch containment")
		}
	}
	qr := probe(q)
	if !qr.CurrentReadOnly || !qr.BootstrapReadOnly || !qr.GrantAbsent {
		t.Fatal("query mount view differs from closed profile")
	}
	ar := probe(a)
	if !ar.GrantReadOnly || !ar.CurrentAbsent || !ar.BootstrapAbsent {
		t.Fatal("caller mount view differs from closed profile")
	}
	sr := probe(sibling)
	if !sr.CurrentAbsent || !sr.BootstrapAbsent || !sr.GrantAbsent {
		t.Fatal("sibling received an authority channel")
	}
	var claims auth.GrantClaims
	if e = json.Unmarshal(ga.Payload, &claims); e != nil {
		fatal("fixture grant payload invalid", e)
	}
	if _, e = launcher.Revoke(ctx, claims.GrantID); e != nil {
		fatalProducer("protected revocation failed", e, p.handle, launcher)
	}
	held(verify("caller-a", ga, scope("caller-a").Scope, 0))
	if _, e = launcher.Rotate(ctx); e != nil {
		fatalProducer("protected rotation failed", e, p.handle, launcher)
	}
	held(verify("caller-b", gb, scope("caller-b").Scope, 0))
	if _, e = launcher.Register(ctx, "caller-a", time.Minute); e != nil {
		fatalProducer("post-rotation fresh registration failed", e, p.handle, launcher)
	}
	if !verify("caller-a", grant(a), scope("caller-a").Scope, 0).Allowed {
		t.Fatal("post-rotation current proof failed")
	}
	fresh := grant(a)
	var priorClaims auth.GrantClaims
	if e = json.Unmarshal(fresh.Payload, &priorClaims); e != nil {
		fatal("prior actual grant invalid", e)
	}
	if _, e = launcher.Stop(ctx); e != nil {
		fatalProducer("protected stop failed", e, p.handle, launcher)
	}
	held(verify("caller-a", fresh, scope("caller-a").Scope, 0))
	// A real producer process/container restart keeps the exact owned channel
	// volumes. Records and sockets on those volumes must not restore authority.
	stop()
	if e = p.runtime.CloseOwned(ctx, p.handle); e != nil {
		fatal("actual old producer removal failed", e)
	}
	all[0].handle = nil
	restarted, e := p.runtime.Create(ctx)
	all = append(all, instance{p.runtime, restarted})
	if e != nil {
		fatal("actual producer restart failed", e)
	}
	launcher2, e := auth.NewLauncher(ctx, p.runtime, restarted, cfg, []auth.LauncherRoute{{Runtime: a.runtime, Handle: a.handle, Scope: scope("caller-a")}, {Runtime: bCaller.runtime, Handle: bCaller.handle, Scope: scope("caller-b")}})
	if e != nil {
		fatalProducer("restarted protected launcher failed", e, restarted, nil)
	}
	run2, stop2 := context.WithCancel(ctx)
	done2 := make(chan error, 1)
	go func() { done2 <- launcher2.Run(run2) }()
	t.Cleanup(func() {
		stop2()
		select {
		case <-done2:
		case <-time.After(3 * time.Second):
			t.Error("restarted producer exec did not stop")
		}
	})
	deadline = time.Now().Add(10 * time.Second)
	for {
		r, e := launcher2.ReadControl(ctx)
		if e == nil {
			if r.Version != 0 || r.Ready {
				t.Fatal("retained records restored producer authority")
			}
			break
		}
		if time.Now().After(deadline) {
			fatalProducer("retained-volume restart private channel unavailable", e, restarted, nil)
		}
		time.Sleep(20 * time.Millisecond)
	}
	held(verify("caller-a", fresh, scope("caller-a").Scope, 0))
	if _, e = launcher2.Initialize(ctx); e != nil {
		fatalProducer("fresh restart incarnation initialization failed", e, restarted, launcher2)
	}
	held(verify("caller-a", fresh, scope("caller-a").Scope, 0))
	if _, e = launcher2.Register(ctx, "caller-a", time.Minute); e != nil {
		fatalProducer("fresh restart registration failed", e, restarted, launcher2)
	}
	restartedGrant := grant(a)
	var nextClaims auth.GrantClaims
	if json.Unmarshal(restartedGrant.Payload, &nextClaims) != nil || nextClaims.KeyRef == priorClaims.KeyRef || nextClaims.Instance == priorClaims.Instance || nextClaims.Incarnation == priorClaims.Incarnation || nextClaims.Binding.ContainerID == priorClaims.Binding.ContainerID {
		t.Fatal("actual restart reused identity/key/container binding")
	}
	if !verify("caller-a", restartedGrant, scope("caller-a").Scope, 0).Allowed {
		t.Fatal("fresh restart positive failed")
	}
	t.Log("ACTUAL independent producer/query/two-caller/sibling custody and current-proof matrix PASS; offline fixture only")
}
