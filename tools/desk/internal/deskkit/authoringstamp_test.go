package deskkit

import (
	"errors"
	"strings"
	"testing"
)

// The shared authoring-stamp condition (#6855), decided directly. The two flip verbs' wired
// cases live with each verb; these pin the classes neither of them reaches.

const gateRun = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

var gateRepo = ForgeRepo{Owner: "example-org", Name: "one"}

func gateBody(repo string, run string) string {
	return "Summary.\n\n" + WorkerAttestationLinePrefix + " " + repo + "#41 run=" + run + "\n"
}

func gateRecord(model, tier string) WorkerRecordReader {
	return func(_ ForgeRepo, n int) (WorkerReceipt, error) {
		return WorkerReceipt{Issue: n, Binding: WorkerBinding{Version: 1, Run: gateRun, Repo: gateRepo.Slug(),
			Kit: "worker", Model: model, Tier: tier}}, nil
	}
}

func authoredBy(model, tier, who string) StampTimeline {
	return stampTimelineAppliedBy(AuthoredModelPrefix+model, AuthoredTierPrefix+tier, who)
}

func TestAuthoringStampGateClasses(t *testing.T) {
	plantRoster(t, stampAuthorityFixtureRoster)
	const authority = "opex"
	ok := gateRecord("example-model-1", "strong")
	cases := []struct {
		name     string
		body     string
		tl       StampTimeline
		read     WorkerRecordReader
		wantExit int // 0 = condition holds
		want     string
	}{
		{"stamped", gateBody(gateRepo.Slug(), gateRun), authoredBy("example-model-1", "strong", authority), ok,
			0, "matches worker attestation"},
		{"no line", "Summary only.\n", StampTimeline{}, nil, 0, "not an attested worker change"},
		{"unstamped", gateBody(gateRepo.Slug(), gateRun), StampTimeline{}, ok, ExitRefused, "UNSTAMPED"},
		{"mismatched tier", gateBody(gateRepo.Slug(), gateRun), authoredBy("example-model-1", "any", authority),
			ok, ExitRefused, "MISMATCHED"},
		{"self-applied", gateBody(gateRepo.Slug(), gateRun), authoredBy("example-model-1", "strong", "someone"),
			ok, ExitRefused, "not a stamp authority"},
		{"foreign record", gateBody("example-org/other", gateRun), StampTimeline{}, ok, ExitRefused,
			"cannot be landed or checked from this repository"},
		{"run differs", gateBody(gateRepo.Slug(), strings.Repeat("f", 64)), StampTimeline{}, ok, ExitRefused,
			"names run"},
		{"record refused", gateBody(gateRepo.Slug(), gateRun), StampTimeline{},
			func(ForgeRepo, int) (WorkerReceipt, error) { return WorkerReceipt{}, Refused("record was edited") },
			ExitRefused, "did not validate: record was edited"},
		{"record unreadable", gateBody(gateRepo.Slug(), gateRun), StampTimeline{},
			func(ForgeRepo, int) (WorkerReceipt, error) {
				return WorkerReceipt{}, Unverifiable("read failed", errors.New("503"))
			}, ExitUnverifiable, "could-not-check is never a standing stamp"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg, err := AuthoringStampGate(gateRepo, 7, c.body, c.tl, c.read)
			if c.wantExit == 0 {
				if err != nil || !strings.Contains(msg, c.want) {
					t.Fatalf("want the condition to hold (%q), got msg=%q err=%v", c.want, msg, err)
				}
				return
			}
			if err == nil || ExitCodeOf(err) != c.wantExit || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want exit %d naming %q, got msg=%q err=%v", c.wantExit, c.want, msg, err)
			}
			if !strings.Contains(err.Error(), "condition "+AuthoringStampCondition) {
				t.Errorf("the refusal must name the condition: %v", err)
			}
		})
	}
}

// The fix line carries only record values, in the flag order `deskdispatch --stamp-only` takes.
func TestStampOnlyFixLine(t *testing.T) {
	r := WorkerReceipt{Issue: 41, Binding: WorkerBinding{Repo: "example-org/one", Kit: "worker",
		Model: "example-model-1", Tier: "strong"}}
	want := "deskdispatch --stamp-only --repo example-org/one --pr 7 --attestation 41 " +
		"--model example-model-1 --tier strong --kit worker"
	if got := StampOnlyFixLine(r, 7); got != want {
		t.Fatalf("fix line = %q, want %q", got, want)
	}
}

func TestWorkerStampGap(t *testing.T) {
	body := gateBody(gateRepo.Slug(), gateRun)
	stamped := []string{AuthoredModelPrefix + "m", AuthoredTierPrefix + "strong"}
	for _, c := range []struct {
		name   string
		body   string
		labels []string
		want   bool
	}{
		{"no line", "plain\n", nil, false},
		{"unstamped", body, []string{DispatchedModelPrefix + "m", DispatchedTierPrefix + "strong"}, true},
		{"half pair", body, stamped[:1], true},
		{"stamped", body, stamped, false},
		{"malformed line", WorkerAttestationLinePrefix + " nonsense\n", stamped, true},
	} {
		if _, got := WorkerStampGap(c.body, c.labels); got != c.want {
			t.Errorf("%s: gap = %t, want %t", c.name, got, c.want)
		}
	}
}
