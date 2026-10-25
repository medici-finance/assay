package index_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// TestAttentionState exercises the §1.2 precedence derivation. The attention layer Verify
// row 1: open escalation ⇒ waiting; a class with no no-op artifact derives
// silent, never idle; precedence holds.
func TestAttentionState(t *testing.T) {
	now := time.Now().UTC()
	W := 30 * time.Minute
	cfg := func(idleLeaves bool) index.AttentionConfig {
		return index.AttentionConfig{Window: W, SilenceThreshold: 48 * time.Hour, IdleLeavesArtifact: idleLeaves}
	}

	cases := []struct {
		name string
		in   index.AttentionInputs
		cfg  index.AttentionConfig
		want domain.AttentionState
	}{
		{
			name: "open escalation derives waiting",
			in:   index.AttentionInputs{OpenEscalations: 1},
			cfg:  cfg(true),
			want: domain.Waiting,
		},
		{
			name: "waiting outranks working — filed-and-continued loop",
			in: index.AttentionInputs{
				OpenEscalations: 2,
				Artifacts:       []index.Artifact{{At: now.Add(-time.Minute)}},
			},
			cfg:  cfg(true),
			want: domain.Waiting,
		},
		{
			name: "recent work artifact derives working",
			in:   index.AttentionInputs{Artifacts: []index.Artifact{{At: now.Add(-5 * time.Minute)}}},
			cfg:  cfg(true),
			want: domain.Working,
		},
		{
			name: "no-op tick within W derives idle when the class leaves one",
			in:   index.AttentionInputs{Artifacts: []index.Artifact{{At: now.Add(-5 * time.Minute), NoOp: true}}},
			cfg:  cfg(true),
			want: domain.Idle,
		},
		{
			name: "idle-honesty: a class that leaves NO no-op artifact derives silent, never idle",
			in:   index.AttentionInputs{Artifacts: []index.Artifact{{At: now.Add(-5 * time.Minute), NoOp: true}}},
			cfg:  cfg(false), // this class leaves nothing on a no-op tick
			want: domain.Silent,
		},
		{
			name: "stale artifact older than W, no escalation, derives silent",
			in:   index.AttentionInputs{Artifacts: []index.Artifact{{At: now.Add(-72 * time.Hour)}}},
			cfg:  cfg(true),
			want: domain.Silent,
		},
		{
			name: "nothing at all derives silent, not a healthier state",
			in:   index.AttentionInputs{},
			cfg:  cfg(true),
			want: domain.Silent,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := index.DeriveAttention(tc.in, tc.cfg, now)
			if got != tc.want {
				t.Fatalf("DeriveAttention = %q, want %q", got, tc.want)
			}
			if !got.Valid() {
				t.Fatalf("derived state %q is not a valid AttentionState", got)
			}
		})
	}
}

// TestFirstArtifactDeadline is the attention layer Verify row 1's second half + §1.3:
// dispatched with no subsequent artifact past N ⇒ failed-to-start, and NEVER
// working; a late first artifact clears it; N below the poller floor never
// fires; pod phase is preferred where a pod exists.
func TestFirstArtifactDeadline(t *testing.T) {
	now := time.Now().UTC()
	cfg := index.AttentionConfig{Window: 30 * time.Minute, FirstArtifactN: index.DefaultFirstArtifactN}
	dispatchedAt := now.Add(-10 * time.Minute) // > N (300s)

	t.Run("dispatched, no artifact past N, derives failed-to-start", func(t *testing.T) {
		got := index.DeriveAttention(index.AttentionInputs{
			Dispatch: &index.DispatchRecord{At: dispatchedAt},
		}, cfg, now)
		if got != domain.FailedToStart {
			t.Fatalf("got %q, want failed-to-start", got)
		}
	})

	t.Run("never working: born-dead outranks a stale would-be working read", func(t *testing.T) {
		got := index.DeriveAttention(index.AttentionInputs{
			Dispatch: &index.DispatchRecord{At: dispatchedAt},
			// an artifact BEFORE dispatch does not clear the deadline
			Artifacts: []index.Artifact{{At: dispatchedAt.Add(-time.Hour)}},
		}, cfg, now)
		if got == domain.Working {
			t.Fatalf("got working; a born-dead loop must never read working")
		}
		if got != domain.FailedToStart {
			t.Fatalf("got %q, want failed-to-start", got)
		}
	})

	t.Run("a late first artifact clears it — no sticky flag", func(t *testing.T) {
		got := index.DeriveAttention(index.AttentionInputs{
			Dispatch:  &index.DispatchRecord{At: dispatchedAt},
			Artifacts: []index.Artifact{{At: dispatchedAt.Add(2 * time.Minute)}},
		}, cfg, now)
		if got == domain.FailedToStart {
			t.Fatalf("a first artifact after dispatch must clear failed-to-start")
		}
	})

	t.Run("N below the 60s poller floor never fires", func(t *testing.T) {
		sub := index.AttentionConfig{Window: 30 * time.Minute, FirstArtifactN: 5 * time.Second}
		got := index.DeriveAttention(index.AttentionInputs{
			Dispatch: &index.DispatchRecord{At: now.Add(-30 * time.Second)},
		}, sub, now)
		if got == domain.FailedToStart {
			t.Fatalf("a sub-floor N (5s) is unmeasurable and must not fire failed-to-start")
		}
	})

	t.Run("within N, not yet failed-to-start", func(t *testing.T) {
		got := index.DeriveAttention(index.AttentionInputs{
			Dispatch: &index.DispatchRecord{At: now.Add(-30 * time.Second)},
		}, cfg, now)
		if got == domain.FailedToStart {
			t.Fatalf("dispatched 30s ago with N=300s is within deadline, not failed-to-start")
		}
	})

	t.Run("pod phase Failed is preferred and derives failed-to-start", func(t *testing.T) {
		got := index.DeriveAttention(index.AttentionInputs{
			PodPhase:  index.PodFailed,
			Artifacts: []index.Artifact{{At: now.Add(-time.Minute)}}, // recent artifact ignored
		}, cfg, now)
		if got != domain.FailedToStart {
			t.Fatalf("pod phase Failed must derive failed-to-start regardless of artifacts, got %q", got)
		}
	})
}

// TestDerivationInputs is the §11 authority suite (the attention layer Verify rows 4 & 5).
// It parses attention.go and FAILS on any addition to the derivation's input
// set: a new import beyond {time, internal/domain}, or a new field on
// AttentionInputs / AttentionConfig. Adding an env-var read means importing
// "os" (caught here); adding a request field means a new struct field (caught
// here). The input sources enumerate to EXACTLY {index rows, cell config,
// clock}.
func TestDerivationInputs(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "attention.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse attention.go: %v", err)
	}

	// 1. Imports must be a subset of the sanctioned set. Anything else — os,
	// net/http, io, bufio, the adapter — is a new input channel.
	allowedImports := map[string]bool{
		`"time"`: true,
		`"github.com/medici-finance/assay/deskd/internal/domain"`: true,
	}
	for _, imp := range file.Imports {
		if !allowedImports[imp.Path.Value] {
			t.Fatalf("attention.go imports %s — the derivation's input set is closed to "+
				"{index rows, cell config, clock}; a new import is a new input source (spec §1.2, §11)", imp.Path.Value)
		}
	}

	// 2. The two input structs enumerate the closed set of sources. Their
	// fields must match EXACTLY — an addition is a new source.
	wantInputs := map[string]bool{"OpenEscalations": true, "Artifacts": true, "Dispatch": true, "PodPhase": true}
	wantConfig := map[string]bool{"Window": true, "FirstArtifactN": true, "SilenceThreshold": true, "IdleLeavesArtifact": true}
	fieldsByType := structFields(file)
	assertExactFields(t, "AttentionInputs", fieldsByType["AttentionInputs"], wantInputs)
	assertExactFields(t, "AttentionConfig", fieldsByType["AttentionConfig"], wantConfig)

	// 3. DeriveAttention's signature is exactly (AttentionInputs,
	// AttentionConfig, time.Time) — no fourth parameter can smuggle in a
	// source, and the return is a domain.AttentionState.
	sig := deriveSignature(t, file)
	wantParams := []string{"AttentionInputs", "AttentionConfig", "time.Time"}
	if len(sig) != len(wantParams) {
		t.Fatalf("DeriveAttention takes %d params, want exactly %d %v", len(sig), len(wantParams), wantParams)
	}
	for i, p := range wantParams {
		if sig[i] != p {
			t.Fatalf("DeriveAttention param %d is %q, want %q — the input set is closed", i, sig[i], p)
		}
	}
}

func structFields(file *ast.File) map[string][]string {
	out := map[string][]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		var names []string
		for _, f := range st.Fields.List {
			for _, nm := range f.Names {
				names = append(names, nm.Name)
			}
		}
		out[ts.Name.Name] = names
		return true
	})
	return out
}

func assertExactFields(t *testing.T, typeName string, got []string, want map[string]bool) {
	t.Helper()
	seen := map[string]bool{}
	for _, f := range got {
		if !want[f] {
			t.Fatalf("%s has an unexpected field %q — a new field is a new input source; "+
				"the derivation may read only (index rows, cell config, clock) (spec §1.2, §11)", typeName, f)
		}
		seen[f] = true
	}
	for f := range want {
		if !seen[f] {
			t.Fatalf("%s is missing the expected input field %q", typeName, f)
		}
	}
}

func deriveSignature(t *testing.T, file *ast.File) []string {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "DeriveAttention" || fn.Recv != nil {
			continue
		}
		var params []string
		for _, p := range fn.Type.Params.List {
			typ := exprString(p.Type)
			// one entry per name sharing a type (there are none here, but be safe)
			n := len(p.Names)
			if n == 0 {
				n = 1
			}
			for range n {
				params = append(params, typ)
			}
		}
		return params
	}
	t.Fatalf("DeriveAttention func decl not found in attention.go")
	return nil
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(v.X)
	}
	return ""
}
