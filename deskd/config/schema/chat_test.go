package schema_test

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
)

// cellWithChat renders a minimal valid cell carrying the given chat block.
func cellWithChat(chatBlock string) string {
	return `cells:
  - name: assay
    repos: [example-org/console]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [glm-5.3]
        app: assay-reviewer-app
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
` + chatBlock
}

// TestChatAbsenceIsOff pins the default. A cell with no chat block is valid and
// Ask Assay is OFF — no desk data leaves, and the pane renders could-not-check.
func TestChatAbsenceIsOff(t *testing.T) {
	f, err := schema.Parse([]byte(cellWithChat("")))
	if err != nil {
		t.Fatalf("a cell with no chat block must be valid: %v", err)
	}
	if f.Cells[0].ChatEnabled() {
		t.Fatal("a cell with no chat block reports Ask Assay as enabled — absence must be OFF")
	}
	if f.Cells[0].Chat != nil {
		t.Fatal("an absent chat block was defaulted in")
	}
}

// TestChatConfigRefusesModelOutsideLadder is the §13.8 gate: no metered backend
// can be configured in, because there is no metered model name validation will
// accept.
func TestChatConfigRefusesModelOutsideLadder(t *testing.T) {
	offLadder := []string{
		"gpt-4o", "gpt-4.1", "claude-opus-4-8", "claude-sonnet-5",
		"gemini-2.5-pro", "glm-5.3", "deepseek-reasoner", "o3",
	}
	for _, m := range offLadder {
		block := "    chat:\n      models: [" + m + "]\n      key_env: MODEL_BACKEND_KEY\n"
		_, err := schema.Parse([]byte(cellWithChat(block)))
		if err == nil {
			t.Fatalf("the validator accepted the off-ladder model %q — a metered backend can be configured in", m)
		}
		if !strings.Contains(err.Error(), "ladder") {
			t.Fatalf("%q: the refusal does not teach the rule: %v", m, err)
		}
	}
}

// TestChatConfigAcceptsTheLadder is the positive control: a check that refuses
// everything is not a gate, it is a wall.
func TestChatConfigAcceptsTheLadder(t *testing.T) {
	for _, models := range []string{
		"[deepseek-chat]",
		"[glm-5.2]",
		"[deepseek-chat, glm-5.2]",
	} {
		block := "    chat:\n      models: " + models + "\n      key_env: MODEL_BACKEND_KEY\n"
		f, err := schema.Parse([]byte(cellWithChat(block)))
		if err != nil {
			t.Fatalf("the validator refused the on-ladder selection %s: %v", models, err)
		}
		if !f.Cells[0].ChatEnabled() {
			t.Fatalf("%s: a configured chat block does not report as enabled", models)
		}
	}
}

// TestChatConfigRefusesLadderInversion: a cell may DECLINE a rung, never
// promote the expensive model ahead of the cheap one.
func TestChatConfigRefusesLadderInversion(t *testing.T) {
	block := "    chat:\n      models: [glm-5.2, deepseek-chat]\n      key_env: MODEL_BACKEND_KEY\n"
	_, err := schema.Parse([]byte(cellWithChat(block)))
	if err == nil {
		t.Fatal("the validator accepted an inverted ladder — the expensive rung would be tried first")
	}
	if !strings.Contains(err.Error(), "ladder order") {
		t.Fatalf("the refusal does not name the rule: %v", err)
	}
}

// TestChatConfigCannotCarryAKey is the custody property in config form: the
// file NAMES an environment variable, and a pasted key value fails on its own
// characters before anyone has to notice what it is.
func TestChatConfigCannotCarryAKey(t *testing.T) {
	t.Run("there is no field to put a key in", func(t *testing.T) {
		block := "    chat:\n      models: [deepseek-chat]\n      key_env: MODEL_BACKEND_KEY\n" +
			"      api_key: sk-a-real-looking-secret\n"
		_, err := schema.Parse([]byte(cellWithChat(block)))
		if err == nil {
			t.Fatal("the schema accepted an api_key field — a config file that can express a secret will eventually hold one")
		}
	})

	t.Run("a key value pasted into key_env is refused", func(t *testing.T) {
		for _, pasted := range []string{
			"sk-abc123def456ghi789",
			"glm-5.2-key-9f8e7d6c",
			"Bearer abc.def.ghi",
			"", "ab",
		} {
			block := "    chat:\n      models: [deepseek-chat]\n      key_env: \"" + pasted + "\"\n"
			if _, err := schema.Parse([]byte(cellWithChat(block))); err == nil {
				t.Fatalf("key_env accepted %q, which is not an environment variable name", pasted)
			}
		}
	})

	t.Run("a real variable name is accepted", func(t *testing.T) {
		block := "    chat:\n      models: [deepseek-chat]\n      key_env: ASSAY_MODEL_BACKEND_KEY\n"
		if _, err := schema.Parse([]byte(cellWithChat(block))); err != nil {
			t.Fatalf("a valid key_env was refused: %v", err)
		}
	})
}

// TestChatConfigRefusesPlaintextEndpoint: the subscription key must not travel
// in clear.
func TestChatConfigRefusesPlaintextEndpoint(t *testing.T) {
	block := "    chat:\n      models: [deepseek-chat]\n      key_env: MODEL_BACKEND_KEY\n" +
		"      endpoint: http://api.example.com/v1\n"
	if _, err := schema.Parse([]byte(cellWithChat(block))); err == nil {
		t.Fatal("the validator accepted a plaintext model endpoint")
	}
	ok := "    chat:\n      models: [deepseek-chat]\n      key_env: MODEL_BACKEND_KEY\n" +
		"      endpoint: https://api.example.com/v1\n"
	if _, err := schema.Parse([]byte(cellWithChat(ok))); err != nil {
		t.Fatalf("the validator refused an https endpoint: %v", err)
	}
}

// TestChatEmptyBlockIsAnErrorNotOff: `chat: {}` is present-but-empty, which is
// a config error. OFF is expressed by omitting the key entirely — the same
// distinction write_app draws, for the same reason.
func TestChatEmptyBlockIsAnErrorNotOff(t *testing.T) {
	block := "    chat:\n      models: []\n      key_env: MODEL_BACKEND_KEY\n"
	if _, err := schema.Parse([]byte(cellWithChat(block))); err == nil {
		t.Fatal("a chat block with no models validated as OFF — an ambiguous OFF on a sensitive-data path is the bug")
	}
}

// TestChatModelsCopyCannotWidenTheLadder pins that ChatModels returns a copy.
func TestChatModelsCopyCannotWidenTheLadder(t *testing.T) {
	got := schema.ChatModels()
	got = append(got, "gpt-4o")
	got[0] = "gpt-4o"
	if n := len(schema.ChatModels()); n != len(schema.ChatModelLadder) {
		t.Fatalf("ChatModels now returns %d entries — the backing array was reached", n)
	}
	for i, m := range schema.ChatModels() {
		if m != schema.ChatModelLadder[i] {
			t.Fatalf("the ladder was mutated through the slice ChatModels returned: %v", schema.ChatModels())
		}
	}
}
