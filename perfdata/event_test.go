package perfdata_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfdata"
	"github.com/medici-finance/assay/perfdata/local"
)

var update = flag.Bool("update", false, "rewrite golden files")

// mapper is an in-memory AliasMapper: a stable alias per key.
type mapper map[string]string

func (m mapper) Alias(k string) (string, error) {
	if a, ok := m[k]; ok {
		return a, nil
	}
	a := perfdata.NewAlias()
	m[k] = a
	return a, nil
}

const testSource = "src-aaaaaaaaaaaaaaaaaaaaaaaaaa"

var testBuild = perfdata.Build{Version: "v1.2.3", OS: "linux", Arch: "amd64"}
var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func ms(v int64) perfdata.Measurement { return perfdata.Measured(v) }

func toolPayload(tool string, wall, own int64) json.RawMessage {
	tc := perfdata.ToolCompletion{Tool: tool, Verb: "lint", Outcome: perfdata.OutcomeOK, StartedAt: perfdata.FormatTime(t0),
		Wall: ms(wall), CPU: perfdata.CPU{Own: ms(own), Descendant: perfdata.Unavailable(), Inclusive: perfdata.Unavailable()}}
	b, _ := json.Marshal(tc)
	return b
}

// seal builds one valid event (or returns the refusal).
func seal(t *testing.T, k perfdata.Kind, p perfdata.Profile, ctx *perfdata.Context, payload json.RawMessage, seq uint64) (perfdata.Event, []byte, error) {
	t.Helper()
	return perfdata.Seal(perfdata.Draft{Kind: k, Payload: payload}, perfdata.Identity{Source: testSource, Epoch: 1, Seq: seq},
		p, "r1", testBuild, ctx, t0)
}

// rawEvent re-encodes an event with one top-level field replaced, bypassing Seal.
func rawEvent(t *testing.T, e perfdata.Event, mutate func(m map[string]any)) []byte {
	t.Helper()
	b, _ := json.Marshal(e)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	mutate(m)
	out, _ := json.Marshal(m)
	return out
}

func wantRefusal(t *testing.T, err error, class perfdata.RefusalClass, planted ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want a %s refusal, got acceptance", class)
	}
	r := perfdata.AsRefusal(err)
	if r.Class != class {
		t.Fatalf("want class %s, got %s (%v)", class, r.Class, err)
	}
	for _, p := range planted {
		if strings.Contains(err.Error(), p) {
			t.Fatalf("refusal echoes refused input %q: %v", p, err)
		}
	}
}

func TestEventPrivacy(t *testing.T) {
	inv := perfdata.NewAlias()
	toolCtx := &perfdata.Context{Invocation: inv}

	t.Run("control: a clean tool event seals and decodes", func(t *testing.T) {
		e, b, err := seal(t, perfdata.KindTool, perfdata.ProfileTool, toolCtx, toolPayload("statusgen", 10, 5), 1)
		if err != nil {
			t.Fatal(err)
		}
		back, err := perfdata.DecodeEvent(b)
		if err != nil || back.ID != e.ID {
			t.Fatalf("round trip: %v", err)
		}
	})

	t.Run("free text is refused and never echoed", func(t *testing.T) {
		const planted = "Fix the flaky login test for customer ACME"
		_, _, err := seal(t, perfdata.KindTool, perfdata.ProfileTool, toolCtx, toolPayload(planted, 10, 5), 1)
		wantRefusal(t, err, perfdata.RefusePrivacy, planted, "ACME")
		ph, _ := json.Marshal(perfdata.Phase{Phase: perfdata.PhaseName("reading " + planted), StartedAt: perfdata.FormatTime(t0), Wall: ms(1), CPUOwn: ms(1)})
		_, _, err = seal(t, perfdata.KindPhase, perfdata.ProfileTool, toolCtx, ph, 1)
		wantRefusal(t, err, perfdata.RefusePrivacy, planted)
		call, _ := json.Marshal(perfdata.ServiceCall{Operation: "GET /repos/acme/secret", Duration: perfdata.NewDistribution()})
		_, _, err = seal(t, perfdata.KindCall, perfdata.ProfileTool, toolCtx, call, 1)
		wantRefusal(t, err, perfdata.RefusePrivacy, "acme")
		var u perfdata.Usage
		u.Mode, u.Model = perfdata.UsageDelta, "My Private Model Name"
		ub, _ := json.Marshal(u)
		_, _, err = seal(t, perfdata.KindUsage, perfdata.ProfileWorkflow, wfCtx(), ub, 1)
		wantRefusal(t, err, perfdata.RefusePrivacy, "Private")
	})

	t.Run("raw locators are refused", func(t *testing.T) {
		for _, loc := range []string{"medici-finance/assay#123", "session-1234", "https://example.test/x", "/home/user/repo"} {
			c := wfCtx()
			c.Session = loc
			_, _, err := seal(t, perfdata.KindTool, perfdata.ProfileWorkflow, c, toolPayload("statusgen", 1, 1), 1)
			wantRefusal(t, err, perfdata.RefusePrivacy, loc)
			c = wfCtx()
			c.Work = []perfdata.WorkRef{{Kind: perfdata.WorkBrief, Ref: loc}}
			_, _, err = seal(t, perfdata.KindTool, perfdata.ProfileWorkflow, c, toolPayload("statusgen", 1, 1), 1)
			wantRefusal(t, err, perfdata.RefusePrivacy, loc)
		}
		e, _, _ := seal(t, perfdata.KindTool, perfdata.ProfileTool, toolCtx, toolPayload("statusgen", 1, 1), 1)
		b := rawEvent(t, e, func(m map[string]any) { m["source"] = "laptop-of-someone" })
		_, err := perfdata.DecodeEvent(b)
		wantRefusal(t, err, perfdata.RefusePrivacy, "someone")
	})

	t.Run("unknown fields are refused without naming them", func(t *testing.T) {
		e, _, _ := seal(t, perfdata.KindTool, perfdata.ProfileTool, toolCtx, toolPayload("statusgen", 1, 1), 1)
		b := rawEvent(t, e, func(m map[string]any) { m["noteAboutCustomer"] = "x" })
		_, err := perfdata.DecodeEvent(b)
		wantRefusal(t, err, perfdata.RefuseSchema, "noteAboutCustomer")
		b = rawEvent(t, e, func(m map[string]any) { m["payload"].(map[string]any)["prompt"] = "secret prompt" })
		_, err = perfdata.DecodeEvent(b)
		wantRefusal(t, err, perfdata.RefuseSchema, "prompt", "secret")
		_, err = perfdata.DecodeEvent([]byte(`{"schema":"performance-event-v2"}`))
		wantRefusal(t, err, perfdata.RefuseUnsupported)
	})

	t.Run("tool profile cannot escalate to workflow identity", func(t *testing.T) {
		for name, c := range map[string]*perfdata.Context{
			"session":  {Invocation: inv, Session: perfdata.NewAlias()},
			"agent":    {Invocation: inv, Agent: perfdata.NewAlias()},
			"parent":   {Invocation: inv, ParentInvocation: perfdata.NewAlias()},
			"work":     {Invocation: inv, Work: []perfdata.WorkRef{{Kind: perfdata.WorkBrief, Ref: perfdata.NewAlias()}}},
			"deskRole": {Invocation: inv, DeskRole: perfdata.DeskWorker},
			"refresh":  {Invocation: inv, SharedRefresh: perfdata.NewAlias()},
		} {
			_, _, err := seal(t, perfdata.KindTool, perfdata.ProfileTool, c, toolPayload("statusgen", 1, 1), 1)
			if err == nil {
				t.Fatalf("%s: tool profile accepted linked context", name)
			}
			wantRefusal(t, err, perfdata.RefusePrivacy)
		}
		// The same check holds on decode of a stored record, not only on Seal.
		e, _, err := seal(t, perfdata.KindTool, perfdata.ProfileWorkflow, wfCtx(), toolPayload("statusgen", 1, 1), 1)
		if err != nil {
			t.Fatal(err)
		}
		b := rawEvent(t, e, func(m map[string]any) { m["profile"] = "tool" })
		_, err = perfdata.DecodeEvent(b)
		wantRefusal(t, err, perfdata.RefusePrivacy)
		ub, _ := json.Marshal(perfdata.Usage{Mode: perfdata.UsageDelta, Model: "m1"})
		_, _, err = seal(t, perfdata.KindUsage, perfdata.ProfileTool, toolCtx, ub, 1)
		wantRefusal(t, err, perfdata.RefusePrivacy)
		_, err = perfdata.Translate(perfdata.ProfileTool, &perfdata.RawContext{Invocation: "i", Session: "session-raw-77"}, mapper{})
		wantRefusal(t, err, perfdata.RefusePrivacy, "session-raw-77")
	})

	t.Run("aliases are stable per key and separate across kinds", func(t *testing.T) {
		m := mapper{}
		raw := &perfdata.RawContext{Invocation: "same", Session: "same", Agent: "same", DeskRole: "worker-desk", Role: "implement",
			Work: []perfdata.RawWork{{Kind: "brief", Repo: "org/repo", Brief: "s/01", Primary: true}, {Kind: "issue", Instance: "github.com", Repo: "org/repo", Number: 7}}}
		a, err := perfdata.Translate(perfdata.ProfileWorkflow, raw, m)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := perfdata.Translate(perfdata.ProfileWorkflow, raw, m)
		if a.Invocation != b.Invocation || a.Work[0].Ref != b.Work[0].Ref {
			t.Fatal("alias not stable within one mapper")
		}
		if a.Invocation == a.Session || a.Session == a.Agent {
			t.Fatal("one text in two identity kinds must map to two aliases")
		}
		if other, _ := perfdata.Translate(perfdata.ProfileWorkflow, raw, mapper{}); other.Invocation == a.Invocation {
			t.Fatal("a new epoch mapper must not reproduce aliases")
		}
		enc, _ := json.Marshal(a)
		for _, leak := range []string{"same", "org/repo", "s/01", "github.com"} {
			if bytes.Contains(enc, []byte(leak)) {
				t.Fatalf("aliased context carries raw %q", leak)
			}
		}
		odd, _ := perfdata.Translate(perfdata.ProfileWorkflow, &perfdata.RawContext{Invocation: "x", DeskRole: "Ian's desk", Role: "boss"}, m)
		if odd.DeskRole != perfdata.DeskUnavailable || odd.Role != perfdata.RoleUnavailable {
			t.Fatalf("unknown role input must become unavailable, got %q/%q", odd.DeskRole, odd.Role)
		}
	})

	t.Run("golden events round-trip byte-identically", func(t *testing.T) {
		path := filepath.Join("testdata", "events", "valid.jsonl")
		if *update {
			writeGolden(t, path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, line := range bytes.Split(bytes.TrimSuffix(b, []byte("\n")), []byte("\n")) {
			e, err := perfdata.DecodeEvent(line)
			if err != nil {
				t.Fatalf("golden line %d: %v", n+1, err)
			}
			again, err := e.Encode()
			if err != nil || !bytes.Equal(again, line) {
				t.Fatalf("golden line %d does not re-encode byte-identically", n+1)
			}
			n++
		}
		if n < 9 {
			t.Fatalf("golden set covers %d kinds, want every v1 kind", n)
		}
		// Reordered keys are a different byte form and are refused.
		var m map[string]json.RawMessage
		first := bytes.SplitN(b, []byte("\n"), 2)[0]
		_ = json.Unmarshal(first, &m)
		reordered, _ := json.Marshal(m) // map encoding sorts keys
		if !bytes.Equal(reordered, first) {
			if _, err := perfdata.DecodeEvent(reordered); err == nil {
				t.Fatal("non-canonical key order accepted")
			}
		}
	})

	t.Run("stored files never contain raw locators", func(t *testing.T) {
		dir := t.TempDir()
		pol := testPolicy()
		st, err := local.Open(dir, perfdata.Consent{Armed: true, Profile: perfdata.ProfileWorkflow}, pol, local.Options{Build: testBuild})
		if err != nil {
			t.Fatal(err)
		}
		planted := []string{"inv-raw-4242", "session-raw-4242", "agent-raw-4242", "medici-finance/assay", "streamx/brief-77", "uuid-4242", "gitlab.example.test"}
		raw := &perfdata.RawContext{Invocation: planted[0], Session: planted[1], Agent: planted[2], DeskRole: "worker-desk", Role: "implement",
			Work: []perfdata.RawWork{{Kind: "brief", Repo: planted[3], Brief: planted[4], BriefUUID: planted[5]}, {Kind: "issue", Instance: planted[6], Repo: planted[3], Number: 4242}}}
		if _, err := st.Write([]perfdata.Draft{{Kind: perfdata.KindTool, Ctx: raw, Payload: toolPayload("statusgen", 3, 1)}}, false); err != nil {
			t.Fatal(err)
		}
		st.Close()
		files := 0
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			files++
			b, _ := os.ReadFile(p)
			for _, s := range planted {
				if bytes.Contains(b, []byte(s)) {
					t.Errorf("%s contains raw locator %q", filepath.Base(p), s)
				}
			}
			if info.Mode().Perm()&0o077 != 0 && filepath.Base(p) != "lock" {
				t.Errorf("%s is readable beyond the owner: %v", filepath.Base(p), info.Mode().Perm())
			}
			return nil
		})
		if files < 4 {
			t.Fatalf("expected a populated store, found %d files", files)
		}
	})
}

func wfCtx() *perfdata.Context {
	return &perfdata.Context{Invocation: perfdata.NewAlias(), Session: perfdata.NewAlias(), DeskRole: perfdata.DeskWorker, Role: perfdata.RoleImplement}
}

func testPolicy() perfdata.Policy {
	p, err := perfdata.DecodePolicy([]byte(`{"schema":"performance-policy-v1","revision":"r1","profiles":["tool","workflow"],"tools":["statusgen","deskboard","deskdispatch"],"ageCapDays":7,"byteCap":16777216}`))
	if err != nil {
		panic(err)
	}
	return p
}

// fixedAlias is a deterministic alias for golden data only.
func fixedAlias(c byte) string { return "pa-" + strings.Repeat(string(c), 26) }

func writeGolden(t *testing.T, path string) {
	t.Helper()
	ctx := &perfdata.Context{Invocation: fixedAlias('b'), Session: fixedAlias('c'), Attempt: fixedAlias('d'),
		DeskRole: perfdata.DeskWorker, Role: perfdata.RoleImplement, Work: []perfdata.WorkRef{{Kind: perfdata.WorkBrief, Ref: fixedAlias('e'), Primary: true}}}
	mk := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	five := int64(5)
	points := &perfdata.Points{Unit: "github-points", MeasuredSum: &five, MeasuredSamples: 1, UnknownSamples: 1}
	payloads := []struct {
		k perfdata.Kind
		v any
	}{
		{perfdata.KindTool, json.RawMessage(toolPayload("statusgen", 120, 40))},
		{perfdata.KindPhase, perfdata.Phase{Phase: "collect", StartedAt: perfdata.FormatTime(t0), Wall: ms(30), CPUOwn: perfdata.Unavailable()}},
		{perfdata.KindAttempt, perfdata.Attempt{State: perfdata.AttemptStarted}},
		{perfdata.KindUsage, perfdata.Usage{Mode: perfdata.UsageCumulative, Model: "model-a", InputTokens: &five,
			Cost: &perfdata.Cost{MicroUnits: 12, Currency: "USD", PricingRevision: "p1", Basis: perfdata.BasisEstimated, Method: "list-price"}}},
		{perfdata.KindHTTP, perfdata.HTTPAttempts{Forge: "github", Class: "graphql", Origin: "direct", Scope: perfdata.ScopeOwn, Attempts: 2, Duration: perfdata.DistributionOf(12, 40), Points: points}},
		{perfdata.KindCall, perfdata.ServiceCall{Operation: "board-read", LogicalCalls: 1, TransportAttempts: 1, Outcomes: perfdata.CallOutcomes{OK: 1}, Duration: perfdata.DistributionOf(8)}},
		{perfdata.KindHandler, perfdata.Handler{Operation: "board-read", Requests: 1, Cache: perfdata.CacheOutcomes{Hit: 1}, Upstream: perfdata.UpstreamObserved, Duration: perfdata.DistributionOf(2)}},
		{perfdata.KindHealth, perfdata.Health{WriteLatency: perfdata.DistributionOf(0, 1)}},
		{perfdata.KindCorrection, perfdata.Correction{Corrects: perfdata.EventID(testSource, 1, 1), Digest: strings.Repeat("0", 64), Reason: "retracted"}},
	}
	var buf bytes.Buffer
	for i, p := range payloads {
		c := ctx
		if p.k == perfdata.KindHealth || p.k == perfdata.KindCorrection {
			c = nil
		}
		_, line, err := seal(t, p.k, perfdata.ProfileWorkflow, c, mk(p.v), uint64(i+1))
		if err != nil {
			t.Fatalf("golden %s: %v", p.k, err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
