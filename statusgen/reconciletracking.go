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
// evidence is the brief's whole raw Evidence section: once it records a
// cross-repository line anywhere, a run with no literal binding refuses
// rather than taking the source-revision read.
func briefVersionForPass(path, evidence string, p flipPass) (int, error) {
	var binding *containerAttestation
	legacy := false
	// p.Run is the selected, already comment-normalized run. Reuse the literal
	// marker scanner without normalizing comments a second time or reading an
	// older run's attestation.
	for _, m := range protectedMarkersNamed(normalizedProtectedEvidence(p.Run), strings.TrimSpace(attestationPrefix)) {
		a, ok := parseContainerAttestation(m.text)
		cross := false
		for _, field := range strings.Fields(m.text) {
			cross = cross || strings.HasPrefix(field, "source-repo=") || strings.HasPrefix(field, "tracking-source=")
		}
		if !cross {
			// The old source-revision reader ignored optional v1 metadata,
			// including unstructured historical notes. Keep that acceptance
			// set; only a structured v1 line conflicts with a selected v2 run.
			legacy = legacy || (m.literal && ok && a.version == 1)
			continue
		}
		if !m.literal {
			return 0, fmt.Errorf("cross-repository attestation must be literal run metadata")
		}
		if !ok || a.version != 2 {
			return 0, fmt.Errorf("malformed cross-repository run attestation%s", missingCrossFields(m.text))
		}
		if !hex64Re.MatchString(a.run) || !hex40Re.MatchString(a.source) || !hex40Re.MatchString(a.trackingSource) {
			return 0, fmt.Errorf("cross-repository attestation needs full run, source and tracking-source identifiers")
		}
		if !flipSHAAgree(p.SHA, a.source) {
			return 0, fmt.Errorf("attested source %s does not bind the run's source %s", a.source, p.SHA)
		}
		// The whole parsed line is compared, run=, model= and tier= included:
		// two brackets from separate invocations in one run refuse even when
		// they bind one tracking-source. That fails closed by intent.
		if binding != nil && *binding != a {
			return 0, fmt.Errorf("conflicting cross-repository attestations in the selected run")
		}
		candidate := a
		binding = &candidate
	}
	if binding == nil {
		if unboundCrossAttestation(evidence) {
			return 0, fmt.Errorf("a cross-repository attestation in Evidence is not bound by literal selected-run metadata")
		}
		version, err := briefVersionAt(path, p.SHA)
		if err != nil {
			// The source revision is read in this repository. A run made in
			// another repository has no such commit here; say what would let
			// it be read rather than leave a bare git failure.
			return 0, fmt.Errorf("%w; a run made in another repository is read only at the tracking-source= its selected run's literal Verification-Attestation line names, and this run binds none", err)
		}
		return version, nil
	}
	if legacy {
		return 0, fmt.Errorf("selected run mixes same-repository and cross-repository attestations")
	}
	dir := filepath.Dir(path)
	repo, err := attestationTrackingRepo(dir)
	if err != nil {
		return 0, err
	}
	if !strings.EqualFold(repo, binding.repo) {
		return 0, fmt.Errorf("attestation tracking repository %s is not the brief's repository", binding.repo)
	}
	prefix, ok := (coverageRepo{root: dir}).RootPrefix()
	if !ok {
		return 0, fmt.Errorf("cannot resolve the tracking brief's repository-relative path")
	}
	rel := prefix + filepath.Base(path)
	if binding.brief != rel {
		return 0, fmt.Errorf("attestation names brief %s, not %s", binding.brief, rel)
	}
	version, err := briefVersionAt(path, binding.trackingSource)
	if err != nil {
		return 0, fmt.Errorf("tracking-source %s: %w", binding.trackingSource, err)
	}
	return version, nil
}

// missingCrossFields names the cross-repository fields a malformed attestation
// line lacks, so the refusal says what the record must carry. A line without
// tracking-source= has no commit in this repository to read the brief at: its
// source= belongs to the deliverable repository and is never read here.
func missingCrossFields(line string) string {
	has := map[string]bool{}
	for _, field := range strings.Fields(line) {
		if k, _, ok := strings.Cut(field, "="); ok {
			has[k] = true
		}
	}
	switch {
	case !has["tracking-source"]:
		return ": it names no tracking-source=, the commit in this repository to read the brief version at (its source= is a deliverable-repository commit and is never read here)"
	case !has["source-repo"]:
		return ": it names no source-repo=, the deliverable repository its source= belongs to"
	}
	return ""
}

// unboundCrossAttestation reports whether any raw Evidence line, in any
// position (fenced, quoted, indented, commented, inline-code-prefixed or in an
// earlier run), is an attestation line carrying a v2 field. Only called when
// the literal scan bound nothing, so every such line is one it did not bind.
// The match is deliberately looser than the parser: a deliverable-revision
// read is correct only for a brief with no cross-repository record at all.
func unboundCrossAttestation(evidence string) bool {
	for _, line := range strings.Split(strings.ToLower(evidence), "\n") {
		if strings.Contains(line, "verification-attestation:") &&
			(strings.Contains(line, "source-repo=") || strings.Contains(line, "tracking-source=")) {
			return true
		}
	}
	return false
}
