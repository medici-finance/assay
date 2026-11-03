package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// fakeFiler is a Filer with no network: it records what desk__escalate asks it
// to write and returns a configurable set of open escalations for the
// filing-time-collapse lookup. This is the only way to exercise the write path
// offline (the live gh path is Verify row 3, could-not-check by design).
type fakeFiler struct {
	open     []OpenEscalation
	filed    []NewEscalation
	comments []reRaise
	num      int
}

type reRaise struct {
	Ref  FiledRef
	Note string
}

func (f *fakeFiler) Identity() string { return AppIdentity }

func (f *fakeFiler) OpenEscalations(context.Context, string) ([]OpenEscalation, error) {
	return f.open, nil
}

func (f *fakeFiler) FileEscalation(_ context.Context, e NewEscalation) (FiledRef, error) {
	f.num++
	f.filed = append(f.filed, e)
	return FiledRef{Repo: e.Repo, Number: f.num, URL: "https://github.com/" + e.Repo + "/issues/" + itoa(f.num)}, nil
}

func (f *fakeFiler) CommentReRaise(_ context.Context, ref FiledRef, note string) error {
	f.comments = append(f.comments, reRaise{Ref: ref, Note: note})
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// testConfig is a one-cell cells.yaml whose review loop authenticates as
// assay-review-app, so a caller login of assay-review-app[bot] joins to the
// {assay, review} automation.
const testConfig = `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [glm-5.2]
        app: assay-review-app
`

func testServer(t *testing.T, filer Filer, flood FloodConfig) *Server {
	t.Helper()
	cfg, err := schema.Parse([]byte(testConfig))
	if err != nil {
		t.Fatalf("parse test config: %v", err)
	}
	fixed := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	return NewServer(Options{
		Config: cfg,
		Dir:    index.NewDirectory(cfg),
		Filer:  filer,
		Now:    func() time.Time { return fixed },
		Flood:  flood,
	})
}

const reviewCaller = "assay-review-app[bot]"

func mustArgs(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return raw
}

// TestOptionIDs is Verify row 4: duplicate and malformed ids are rejected AT
// FILING; missing ids fall back to option-1..n (spec §1.1.1).
func TestOptionIDs(t *testing.T) {
	t.Run("missing ids fall back to option-1..n", func(t *testing.T) {
		opts, err := normalizeOptions([]OptionInput{
			{Label: "Ship the partial index", Consequence: "inbox live today"},
			{Label: "Hold until the fix lands", Consequence: "nothing ships this week"},
		})
		if err != nil {
			t.Fatalf("valid option-less-id set was rejected: %v", err)
		}
		if len(opts) != 2 || opts[0].ID != "option-1" || opts[1].ID != "option-2" {
			t.Fatalf("missing ids did not fall back to option-1..n: %+v", opts)
		}
	})

	t.Run("duplicate ids rejected", func(t *testing.T) {
		_, err := normalizeOptions([]OptionInput{
			{ID: "ship", Label: "A", Consequence: "c1"},
			{ID: "ship", Label: "B", Consequence: "c2"},
		})
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("duplicate ids must be rejected at filing; got err=%v", err)
		}
	})

	t.Run("malformed id rejected", func(t *testing.T) {
		for _, bad := range []string{"Ship", "a", "has space", "toolongtoolongtoolongtoolongtoolongX", "-lead"} {
			_, err := normalizeOptions([]OptionInput{{ID: bad, Label: "L", Consequence: "c"}})
			if err == nil || !strings.Contains(err.Error(), "malformed") {
				t.Fatalf("malformed id %q must be rejected; got err=%v", bad, err)
			}
		}
	})

	t.Run("valid slug kept", func(t *testing.T) {
		opts, err := normalizeOptions([]OptionInput{{ID: "ship-partial", Label: "L", Consequence: "c"}})
		if err != nil || len(opts) != 1 || opts[0].ID != "ship-partial" {
			t.Fatalf("valid slug rejected or mangled: opts=%+v err=%v", opts, err)
		}
	})
}

// TestEscalateBodyV2 backstops Verify row 3 offline: the emitted body carries
// the v2 marker and one Consequence line per option, and ROUND-TRIPS through
// the adapter parser — the "no side channel, emits-what-it-parses" property
// (spec §1.4) checked rather than asserted.
func TestEscalateBodyV2(t *testing.T) {
	opts := []domain.Option{
		{ID: "ship-partial", Label: "Ship the partial index", Consequence: "inbox live today; roadmap lags a week"},
		{ID: "wait-for-fix", Label: "Hold until #NNN lands", Consequence: "nothing ships this week; no rework risk"},
	}
	body := FormatBody("The index is ready but the roadmap pane is not.", opts)

	if !strings.Contains(body, "<!-- assay:options v2 -->") {
		t.Fatalf("body missing the v2 marker:\n%s", body)
	}
	if got := strings.Count(body, "Consequence:"); got != 2 {
		t.Fatalf("want one Consequence line per option (2), got %d:\n%s", got, body)
	}

	parsed := adapter.ParseOptions(body)
	if parsed.Encoding != adapter.EncodingTypedV2 {
		t.Fatalf("emitted body did not parse back as typed-v2: encoding=%s problems=%v", parsed.Encoding, parsed.Problems)
	}
	if len(parsed.Options) != 2 || parsed.Options[0].ID != "ship-partial" || parsed.Options[1].ID != "wait-for-fix" {
		t.Fatalf("round-trip lost or mangled options: %+v", parsed.Options)
	}
	// The rev the adapter re-derives must equal the one we would emit — the
	// two sides agree on the fingerprint or the answer-race guard is broken.
	if parsed.Rev != domain.OptionsRev(opts) {
		t.Fatalf("OptionsRev disagreement across the wire: adapter=%s emit=%s", parsed.Rev, domain.OptionsRev(opts))
	}
}

// TestEscalateFilesAndAttributes drives desk__escalate end to end through the
// fake filer: a fresh filing opens exactly one issue, authored by the App, and
// derives agent-raised regardless of any provenance the caller might send.
func TestEscalateFilesAndAttributes(t *testing.T) {
	f := &fakeFiler{}
	s := testServer(t, f, DefaultFloodConfig)

	args := mustArgs(t, map[string]any{
		"cell":       "assay",
		"kind":       "needs-decision",
		"title":      "Ship the partial index?",
		"situation":  "The index is ready; the roadmap pane is not.",
		"provenance": "policy-raised", // MUST be ignored — provenance is by caller identity
		"options": []map[string]any{
			{"id": "ship-partial", "label": "Ship", "consequence": "inbox live today"},
			{"id": "wait-for-fix", "label": "Hold", "consequence": "no rework risk"},
		},
	})
	out, err := s.Dispatch(context.Background(), "desk__escalate", Call{Args: args, CallerLogin: reviewCaller})
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
	res := out.(EscalateResult)
	if res.Outcome != "filed" {
		t.Fatalf("outcome = %q, want filed", res.Outcome)
	}
	if res.Provenance != string(domain.AgentRaised) {
		t.Fatalf("provenance = %q, want agent-raised (caller's policy-raised claim must be ignored)", res.Provenance)
	}
	if !res.Attributed {
		t.Fatalf("review caller should attribute to the {assay, review} loop")
	}
	if len(f.filed) != 1 {
		t.Fatalf("want exactly one issue filed, got %d", len(f.filed))
	}
	if f.Identity() != AppIdentity {
		t.Fatalf("filer authored as %q, want the App", f.Identity())
	}
}

// TestEscalateNoWritePath asserts the honest could-not-check when deskd was
// started without a write token — never a silent no-op (mirrors Verify row 3's
// offline reading).
func TestEscalateNoWritePath(t *testing.T) {
	cfg, _ := schema.Parse([]byte(testConfig))
	s := NewServer(Options{Config: cfg, Dir: index.NewDirectory(cfg)}) // no Filer
	_, err := s.Dispatch(context.Background(), "desk__escalate",
		Call{Args: mustArgs(t, map[string]any{"cell": "assay", "kind": "ask", "title": "t", "situation": "s"}), CallerLogin: reviewCaller})
	if err == nil || !strings.Contains(err.Error(), "no write path") {
		t.Fatalf("escalate with no filer must error could-not-check, got %v", err)
	}
}
