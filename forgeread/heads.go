package forgeread

import (
	"encoding/json"
	"fmt"
	"strings"
)

// HeadsEnvelope is one complete git branch advertisement. Complete is explicit:
// missing, partial and failed reads must never become known-empty claims.
type HeadsEnvelope struct {
	Schema   int      `json:"schema"`
	Repo     string   `json:"repo"`
	Complete bool     `json:"complete"`
	Heads    []string `json:"heads"`
}

func ParseHeadsEnvelope(data []byte, repo string) ([]string, error) {
	var env HeadsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("cannot parse branch advertisement: %w", err)
	}
	if env.Schema != EnvelopeSchema || env.Repo != repo || !env.Complete || env.Heads == nil {
		return nil, fmt.Errorf("branch advertisement is incomplete or does not match the requested repository/schema")
	}
	seen := map[string]bool{}
	for _, head := range env.Heads {
		if !strings.HasPrefix(head, "refs/heads/") || len(head) == len("refs/heads/") || seen[head] {
			return nil, fmt.Errorf("invalid or repeated branch in advertisement")
		}
		seen[head] = true
	}
	return env.Heads, nil
}
