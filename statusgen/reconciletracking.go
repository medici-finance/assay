package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// briefVersionForPass selects the brief's repository coordinate AFTER the
// caller's closure and verifier-provenance checks. It supplies no execution
// credit or authority. A v2 attestation separates deliverable source from the
// tracking snapshot that actually contains the brief. Legacy runs keep their
// original source-revision read; neither path falls back to the current file.
func briefVersionForPass(path string, p flipPass) (int, error) {
	var binding *containerAttestation
	legacy := false
	// p.Run is the selected, already comment-normalized run. Reuse the literal
	// marker scanner without normalizing comments a second time or reading an
	// older run's attestation.
	for _, m := range protectedMarkersNamed(normalizedProtectedEvidence(p.Run), strings.TrimSpace(attestationPrefix)) {
		if !m.literal {
			return 0, fmt.Errorf("attestation must be literal run metadata")
		}
		a, ok := parseContainerAttestation(m.text)
		if !ok {
			return 0, fmt.Errorf("malformed run attestation")
		}
		if a.version == 1 {
			legacy = true
			continue
		}
		if !hex64Re.MatchString(a.run) || !hex40Re.MatchString(a.source) || !hex40Re.MatchString(a.trackingSource) {
			return 0, fmt.Errorf("cross-repository attestation needs full run, source and tracking-source identifiers")
		}
		if !flipSHAAgree(p.SHA, a.source) {
			return 0, fmt.Errorf("attested source %s does not bind the run's source %s", a.source, p.SHA)
		}
		if binding != nil && *binding != a {
			return 0, fmt.Errorf("conflicting cross-repository attestations in the selected run")
		}
		candidate := a
		binding = &candidate
	}
	if binding == nil {
		return briefVersionAt(path, p.SHA)
	}
	if legacy {
		return 0, fmt.Errorf("selected run mixes same-repository and cross-repository attestations")
	}
	dir := filepath.Dir(path)
	origin, err := coverageGit(dir, "remote", "get-url", "origin").Output()
	if err != nil || remoteHost(strings.TrimSpace(string(origin))) != "github.com" {
		return 0, fmt.Errorf("cannot bind the tracking repository to a github.com origin")
	}
	if !strings.EqualFold(originOwnerRepo(strings.TrimSpace(string(origin))), binding.repo) {
		return 0, fmt.Errorf("attestation tracking repository %s is not the brief's repository", binding.repo)
	}
	prefix, err := coverageGit(dir, "rev-parse", "--show-prefix").Output()
	if err != nil {
		return 0, fmt.Errorf("cannot resolve the tracking brief's repository-relative path: %w", err)
	}
	rel := strings.TrimSpace(string(prefix)) + filepath.Base(path)
	if binding.brief != rel {
		return 0, fmt.Errorf("attestation names brief %s, not %s", binding.brief, rel)
	}
	version, err := briefVersionAt(path, binding.trackingSource)
	if err != nil {
		return 0, fmt.Errorf("tracking-source %s: %w", binding.trackingSource, err)
	}
	return version, nil
}
