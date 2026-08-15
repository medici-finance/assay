package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// signoff_test.go — the three-state sign-off reader, and the proof it can fail.
//
// Two properties are under test and they pull in opposite directions, which is
// the point:
//
//   - the reader must never manufacture a grant (no could-not-read or unsigned
//     block may come back Granted), and
//   - the reader must never manufacture a REFUSAL either — a register that a
//     human really did sign must come back Granted, or the tool goes inert and
//     tells operators a decision was not made.
//
// A check that only ever refuses passes the first property and is worthless. So
// every refusal case below is paired with a control that grants, and the
// steering-write section ends on a mutant that PASSES — the same authority
// question, against a register where the authority exists.

// liveShapeSignOff is the house style of the real register, and the exact shape
// a line-oriented reader is blind to: prose on the key's line, artifact URL on
// the line below. All five rulings in the live register are written this way.
func liveShapeSignOff(id, url string) string {
	return "## " + id + " Some ruling\n\n" +
		"**Statement.** The rule this ruling states.\n\n" +
		"**Sign-off:** a human, 2026-08-13 — approved:\n" +
		url + "\n"
}

const someArtifact = "https://github.com/an-org/a-repo/pull/1#issuecomment-1234567890"

func TestSignOffReadsTheBlockNotTheLine(t *testing.T) {
	// The regression. Before the block read, every ruling in a fully signed
	// register came back UNSIGNED — a positive, wrong claim that no human had
	// ruled.
	got := ReadSignOffFromText(liveShapeSignOff("R-1", someArtifact), "R-1")
	if got.State != SignOffGranted {
		t.Fatalf("live register shape: state=%s detail=%s", got.State, got.Detail)
	}
	if got.URL != someArtifact {
		t.Fatalf("url = %q, want %q", got.URL, someArtifact)
	}

	t.Run("prose wrapped across several lines is still one block", func(t *testing.T) {
		text := "## R-1 lanes\n\n**Sign-off:** a human, 2026-08-13 — lanes B and C approved (lane A\n" +
			"was already granted elsewhere):\n" + someArtifact + "\n\n## R-2 next\n"
		got := ReadSignOffFromText(text, "R-1")
		if got.State != SignOffGranted || got.URL != someArtifact {
			t.Fatalf("state=%s url=%q detail=%s", got.State, got.URL, got.Detail)
		}
	})

	t.Run("the URL may sit on the key's own line", func(t *testing.T) {
		text := "## R-1 lanes\n\n**Sign-off:** " + someArtifact + "\n\n## R-2 next\n"
		if got := ReadSignOffFromText(text, "R-1"); got.State != SignOffGranted {
			t.Fatalf("state=%s detail=%s", got.State, got.Detail)
		}
	})
}

func TestSignOffUnsignedIsAPositiveDetermination(t *testing.T) {
	// A register that was read, whose block is there and carries no artifact.
	// This one IS a finding about the human, and must stay distinct from
	// could-not-read: it is the state an operator resolves by getting a
	// signature, and the other is the state they resolve by fixing their
	// invocation.
	for _, blank := range []string{"", "_(empty)_", "(none yet)"} {
		text := "## R-1 lanes\n\n**Sign-off:** " + blank + "\n\n## R-2 next\n\n**Sign-off:** " +
			someArtifact + "\n"
		got := ReadSignOffFromText(text, "R-1")
		if got.State != SignOffUnsigned {
			t.Fatalf("blank %q: state=%s detail=%s", blank, got.State, got.Detail)
		}
		if got.URL != "" {
			t.Fatalf("blank %q: an unsigned ruling returned a URL %q", blank, got.URL)
		}
	}
}

func TestSignOffCouldNotReadIsNeverReportedAsUnsigned(t *testing.T) {
	cases := []struct {
		name string
		text string
		id   string
	}{
		{
			name: "no such ruling section",
			text: liveShapeSignOff("R-1", someArtifact),
			id:   "R-9",
		},
		{
			name: "section exists but carries no sign-off line at all",
			text: "## R-1 lanes\n\n**Statement.** A rule with no acceptance line.\n\n## R-2 next\n",
			id:   "R-1",
		},
		{
			name: "two sign-off lines in one section",
			text: "## R-1 lanes\n\n**Sign-off:** " + someArtifact + "\n\n" +
				"**Sign-off:** https://github.com/an-org/a-repo/pull/2#issuecomment-2222222222\n",
			id: "R-1",
		},
		{
			name: "two URLs in one block",
			text: "## R-1 lanes\n\n**Sign-off:** approved by both:\n" + someArtifact + "\n" +
				"https://github.com/an-org/a-repo/pull/2#issuecomment-2222222222\n",
			id: "R-1",
		},
		{
			// The lesson, as its own case. A block that ends promising an
			// artifact and supplies none is a read this parser could not
			// finish. Calling it "unsigned" would accuse a human of not
			// deciding, on the strength of the reader's own blindness.
			name: "block ends promising an artifact and none follows",
			text: "## R-1 lanes\n\n**Sign-off:** a human, 2026-08-13 — approved:\n\n## R-2 next\n",
			id:   "R-1",
		},
		{
			name: "a truncated scheme is not a decision",
			text: "## R-1 lanes\n\n**Sign-off:** a human — see https:\n\n## R-2 next\n",
			id:   "R-1",
		},
		{
			name: "no ruling id named",
			text: liveShapeSignOff("R-1", someArtifact),
			id:   "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReadSignOffFromText(tc.text, tc.id)
			if got.State != SignOffCouldNotRead {
				t.Fatalf("state = %s, want could-not-read (detail: %s)", got.State, got.Detail)
			}
			if got.URL != "" {
				t.Fatalf("could-not-read returned a URL %q", got.URL)
			}
			if strings.TrimSpace(got.Detail) == "" {
				t.Fatal("could-not-read with no reason an operator can act on")
			}
		})
	}
}

func TestSignOffUnreadableFileIsCouldNotRead(t *testing.T) {
	got := ReadSignOff(filepath.Join(t.TempDir(), "absent.md"), "R-1")
	if got.State != SignOffCouldNotRead {
		t.Fatalf("missing register: state = %s, want could-not-read", got.State)
	}
	// Control: the same reader against a register that is there.
	p := filepath.Join(t.TempDir(), "rulings.md")
	if err := os.WriteFile(p, []byte(liveShapeSignOff("R-1", someArtifact)), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ReadSignOff(p, "R-1"); got.State != SignOffGranted {
		t.Fatalf("readable signed register: state = %s, detail = %s", got.State, got.Detail)
	}
}

func TestSignOffDoesNotBorrowASiblingRulingsSignature(t *testing.T) {
	text := "## R-1 lanes\n\n**Sign-off:** _(empty)_\n\n## R-2 taxonomy\n\n**Sign-off:** " +
		someArtifact + "\n"
	if got := ReadSignOffFromText(text, "R-1"); got.State != SignOffUnsigned {
		t.Fatalf("R-1 borrowed R-2's signature: state=%s url=%q", got.State, got.URL)
	}
	if got := ReadSignOffFromText(text, "R-2"); got.State != SignOffGranted {
		t.Fatalf("R-2 is signed but read as %s", got.State)
	}

	t.Run("an id is matched as a whole token", func(t *testing.T) {
		text := "## R-12 a later ruling\n\n**Sign-off:** " + someArtifact + "\n"
		if got := ReadSignOffFromText(text, "R-1"); got.State == SignOffGranted {
			t.Fatal("R-1 was granted by R-12's signature")
		}
	})

	t.Run("a bold key ends the block", func(t *testing.T) {
		// An "**Amends.**" paragraph that QUOTES a URL is not the
		// authorization; swallowing it would grant on borrowed text.
		text := "## R-1 lanes\n\n**Sign-off:** _(empty)_\n" +
			"**Amends.** the standing rule recorded at " + someArtifact + "\n"
		if got := ReadSignOffFromText(text, "R-1"); got.State != SignOffUnsigned {
			t.Fatalf("state=%s url=%q — a neighbouring paragraph signed the ruling", got.State, got.URL)
		}
	})
}

// ---------------------------------------------------------------------------
// The steering-write control (desk-console-2/04).
//
// desk-console-2/04 asks for two writes that reorder and inject work. No ruling
// in the register grants either one, so the brief ships no write. What it does
// ship is the determination such a write would have to pass, with the proof
// that the determination discriminates rather than always refusing.

// steerRulingID is the id a steering/priority write grant would have to carry.
// Nothing in the register uses it; that is the finding, not an oversight.
const steerRulingID = "R-STEER"

// steerWriteAuthorized is the whole gate, and it is one line on purpose: a
// steering write proceeds on SignOffGranted and on nothing else. Both other
// states refuse, and they refuse for different reasons the operator is told.
func steerWriteAuthorized(so SignOff) bool { return so.State == SignOffGranted }

func TestUnauthorizedSteeringWriteIsRefused(t *testing.T) {
	// A register in the live one's exact shape, fully signed — for other
	// rulings. Asked the steering question, it must refuse, and refuse as
	// could-not-read rather than as a claim about what a human decided: nobody
	// has been asked, so nobody has declined.
	register := liveShapeSignOff("R-1", someArtifact) + "\n" +
		liveShapeSignOff("R-5", someArtifact) + "\n"

	so := ReadSignOffFromText(register, steerRulingID)
	if steerWriteAuthorized(so) {
		t.Fatalf("a steering write was authorized by a register that never mentions %s", steerRulingID)
	}
	if so.State != SignOffCouldNotRead {
		t.Fatalf("state = %s, want could-not-read: an absent ruling is not a declined one", so.State)
	}

	t.Run("a present but unsigned steering ruling still refuses", func(t *testing.T) {
		r := register + "## " + steerRulingID + " Steering write\n\n**Sign-off:**\n"
		so := ReadSignOffFromText(r, steerRulingID)
		if steerWriteAuthorized(so) {
			t.Fatal("an unsigned steering ruling authorized a write")
		}
		if so.State != SignOffUnsigned {
			t.Fatalf("state = %s, want unsigned", so.State)
		}
	})

	t.Run("a neighbouring signed ruling does not authorize it", func(t *testing.T) {
		if so := ReadSignOffFromText(register, "R-1"); !steerWriteAuthorized(so) {
			t.Fatal("control failed: R-1 is signed in this fixture")
		}
		if so := ReadSignOffFromText(register, steerRulingID); steerWriteAuthorized(so) {
			t.Fatal("R-1's signature authorized a steering write")
		}
	})

	// PROOF THE CHECK CAN FAIL. Same gate, same code path, against a register
	// where the authority exists: it grants. A gate that refused here would be
	// a gate that refuses everything, and would prove nothing about the three
	// refusals above.
	t.Run("positive control: a signed steering ruling IS authorized", func(t *testing.T) {
		r := register + liveShapeSignOff(steerRulingID, someArtifact)
		so := ReadSignOffFromText(r, steerRulingID)
		if !steerWriteAuthorized(so) {
			t.Fatalf("the gate refuses even a signed grant — it discriminates nothing "+
				"(state=%s detail=%s)", so.State, so.Detail)
		}
		if so.URL != someArtifact {
			t.Fatalf("url = %q, want the signed artifact", so.URL)
		}
	})
}
