package schema

import (
	"fmt"
	"regexp"
	"strings"
)

// ── chat: the Ask Assay model backend (the service contract, design §13.8) ──
//
// This block is the CONFIG half of the model policy ruled the custody design: the
// console may talk to a model endpoint only through a SUBSCRIPTION key, and
// only to a model on the closed ladder below. No metered spend, and no
// interactive OAuth in a service.
//
// It is written the same way the `per_user_identity` gate above is written,
// and for the same reason: the wrong thing must not be SPELLABLE. There is no
// `endpoint: <anything>` free field that could point at a metered vendor, no
// `api_key:` field that could hold a secret in a committed file, and no model
// name outside [ChatModelLadder] that validation will accept. A validator that
// only rejected after understanding would leave the metered backend one typo
// away; this one has nowhere to type it.
//
// ABSENCE IS OFF. A cell with no `chat:` block has no Ask Assay backend, and
// the pane renders could-not-check rather than a fabricated answer. Turning it
// on is a human's config edit on a `gate: human` path, not something a brief
// performs.

// ChatModelLadder is the CLOSED, ORDERED model ladder the Ask Assay chat may
// use — cheapest rung first (design §13.8). It is an array rather than a slice
// so that no package, this one included, can append to it: the only way to add
// a rung is to edit this line, in a reviewed diff, on a `gate: human` surface.
//
// Both members are served by an existing SUBSCRIPTION key. That is the whole
// selection criterion, and it is why a name that is not here is refused rather
// than merely undocumented: an unlisted model is, by construction, one this
// house has no subscription for — so configuring it would either fail at the
// endpoint or bill by the token.
var ChatModelLadder = [2]string{"deepseek-chat", "glm-5.2"}

// ChatModels returns the ladder as a fresh slice. It copies, so a caller that
// sorts, filters or appends to the result cannot reach [ChatModelLadder].
func ChatModels() []string {
	out := make([]string, len(ChatModelLadder))
	copy(out, ChatModelLadder[:])
	return out
}

// chatLadderRank reports a model's position on the ladder, and whether it is on
// the ladder at all.
func chatLadderRank(model string) (int, bool) {
	for i, m := range ChatModelLadder {
		if m == model {
			return i, true
		}
	}
	return 0, false
}

// keyEnvRE is the shape of an environment VARIABLE NAME. It admits upper-case
// letters, digits and underscores only, which is exactly the set that cannot
// spell a credential: every subscription key this house holds carries lower
// case, a hyphen, or a dot. So a `key_env:` line that someone pasted a SECRET
// into fails validation on its characters, before anyone has to notice what
// the value is.
var keyEnvRE = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,63}$`)

// ChatConfig is a cell's Ask Assay model-backend configuration.
//
// Note what is NOT here: there is no field that can hold a key. The console
// reads the credential from the process environment at request time, from the
// variable [ChatConfig.KeyEnv] NAMES — the deployment mounts it as a Secret.
// A config file that cannot express a secret cannot leak one, which is a
// stronger property than a review convention that it should not.
type ChatConfig struct {
	// Models is this cell's rung selection, cheapest first. Every entry must
	// be on [ChatModelLadder], entries must be unique, and they must appear in
	// LADDER ORDER — a cell may decline a rung, never reorder the ladder so
	// that the expensive model is tried first.
	Models []string `yaml:"models"`

	// KeyEnv NAMES the environment variable holding the subscription key. It
	// is the variable's name, never its value; see [keyEnvRE].
	KeyEnv string `yaml:"key_env"`

	// Endpoint is the OPTIONAL base URL of the model backend. It must be
	// https — a plaintext endpoint would put the subscription key on the wire
	// in clear, and there is no http scheme to spell here.
	Endpoint string `yaml:"endpoint"`
}

// validate enforces the §13.8 model policy. Each refusal names the rule so an
// operator reading cellsvalidate's stderr learns why, rather than only that
// they were refused.
func (cc *ChatConfig) validate() error {
	if len(cc.Models) == 0 {
		return fmt.Errorf("chat: models is required and must name at least one rung of the ladder %v — "+
			"a chat block with no model is not an OFF switch; OMIT the whole `chat:` key to leave Ask Assay off", ChatModels())
	}
	seen := map[string]bool{}
	last := -1
	for i, m := range cc.Models {
		if strings.TrimSpace(m) != m || m == "" {
			return fmt.Errorf("chat: models[%d] %q is blank or padded", i, m)
		}
		rank, ok := chatLadderRank(m)
		if !ok {
			return fmt.Errorf("chat: models[%d] %q is not on the closed model ladder %v (design §13.8, ruled the custody design: "+
				"subscription keys only, no metered spend). A model that is not on the ladder is one this house holds no "+
				"subscription for, so configuring it would bill by the token", i, m, ChatModels())
		}
		if seen[m] {
			return fmt.Errorf("chat: models[%d] %q is listed twice", i, m)
		}
		seen[m] = true
		if rank <= last {
			return fmt.Errorf("chat: models[%d] %q is out of ladder order — the ladder is cheapest-first %v and a cell may "+
				"DECLINE a rung, never promote an expensive one ahead of a cheap one", i, m, ChatModels())
		}
		last = rank
	}
	if cc.KeyEnv == "" {
		return fmt.Errorf("chat: key_env is required — it NAMES the environment variable the subscription key is mounted " +
			"into (a Secret), so that no key value ever lives in this file")
	}
	if !keyEnvRE.MatchString(cc.KeyEnv) {
		return fmt.Errorf("chat: key_env %q is not an environment VARIABLE NAME (upper-case letters, digits and "+
			"underscores, 3-64 chars). If what you pasted is a key VALUE, this file is the wrong place for it: mount it "+
			"as a Secret and name the variable here", cc.KeyEnv)
	}
	if cc.Endpoint != "" && !strings.HasPrefix(cc.Endpoint, "https://") {
		return fmt.Errorf("chat: endpoint %q is not https — a plaintext endpoint puts the subscription key on the wire "+
			"in clear", cc.Endpoint)
	}
	return nil
}
