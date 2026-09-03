package verifier

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Fixture is the deterministic scripted-verdict reference adapter: it answers
// Verify from a fixed table of verdicts keyed by MatchKey, so the whole sweep
// lane runs offline with no live coding agent. It is the AgentVerifier analogue
// of adapters.GithubLabels — a reference implementation shipped in the binary,
// with the live agent CLI left to configuration.
//
// It records how many times Verify was invoked (Calls). The standing-lane
// incrementality guarantee — a rerun over an unchanged tree sends ZERO
// already-adjudicated suspects to the verifier — is asserted directly against
// that counter.
type Fixture struct {
	mu       sync.Mutex
	scripted map[string]Verdict
	calls    int
}

// NewFixture builds a Fixture from an in-memory verdict table keyed by
// MatchKey (category|file).
func NewFixture(scripted map[string]Verdict) *Fixture {
	cp := make(map[string]Verdict, len(scripted))
	for k, v := range scripted {
		cp[k] = v
	}
	return &Fixture{scripted: cp}
}

// LoadFixture reads a scripted-verdict table from a JSON file: an object mapping
// MatchKey → verdict. This is the on-disk form a config's fixture verifier
// selection points at.
func LoadFixture(path string) (*Fixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("verifier: reading scripted verdicts %q: %w", path, err)
	}
	var scripted map[string]Verdict
	if err := json.Unmarshal(raw, &scripted); err != nil {
		return nil, fmt.Errorf("verifier: parsing scripted verdicts %q: %w", path, err)
	}
	return NewFixture(scripted), nil
}

// Verify looks up the scripted verdict for the suspect by MatchKey and stamps
// its fingerprint from the suspect. A suspect with no scripted entry returns an
// error, which the emitter records as could-not-verify — a missing script is
// never silently treated as a clean confirm or a drop. The context pack is
// intentionally unused: the reference adapter's job is determinism, not
// analysis.
func (f *Fixture) Verify(s Suspect, _ ContextPack) (Verdict, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	v, ok := f.scripted[MatchKey(s)]
	if !ok {
		return Verdict{}, fmt.Errorf("verifier: no scripted verdict for suspect %s (%s)", MatchKey(s), s.Fingerprint)
	}
	v.Fingerprint = s.Fingerprint
	return v, nil
}

// Calls reports how many times Verify has been invoked on this Fixture.
func (f *Fixture) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}
