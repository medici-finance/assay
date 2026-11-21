package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// acceptanceSource is the immutable source portion of an adjudication record.
// It describes the inputs to a decision and carries no authority on its own.
type acceptanceSource struct {
	Repository string              `json:"repository"`
	Revision   string              `json:"revision"`
	BriefPath  string              `json:"briefPath"`
	BriefBlob  string              `json:"briefBlob"`
	ReadmeBlob string              `json:"readmeBlob"`
	Refusals   []acceptanceRefusal `json:"refusals"`
}

type acceptanceRefusal struct {
	Line   int    `json:"evidenceLine"`
	Digest string `json:"sha256"`
	Reason string `json:"reason"`
}

// readAcceptanceSource binds the full close scan to a committed brief and its
// stream README. Callers must resolve the canonical brief before calling it.
func readAcceptanceSource(root, briefPath string) (acceptanceSource, error) {
	return readAcceptanceSourceAt(root, briefPath, "")
}

// readAcceptanceSourceAt rechecks an already bound revision against the current
// checkout. Unrelated descendant commits do not stale an unchanged brief and
// stream README. A replaced or off-branch source is refused.
func readAcceptanceSourceAt(root, briefPath, revision string) (acceptanceSource, error) {
	repo, err := attestationTrackingRepo(root)
	if err != nil {
		return acceptanceSource{}, err
	}
	reader := ordinaryGitCore(root)
	if reader.err != nil || reader.native != "" {
		return acceptanceSource{}, fmt.Errorf("acceptance source: could not open the canonical Git reader")
	}
	current, err := reader.repo.HeadID()
	if err != nil {
		return acceptanceSource{}, fmt.Errorf("acceptance source: read HEAD: %w", err)
	}
	rev := revision
	if rev == "" {
		rev = current
	}
	if !isObjectID(rev) {
		return acceptanceSource{}, fmt.Errorf("acceptance source: bound revision is not a full object id")
	}
	// This is the adjudication record's provenance, not a witness's
	// applicability. The exact source bytes and credited run are checked apart.
	landed, err := reader.repo.IsAncestor(rev, current)
	if err != nil || !landed {
		return acceptanceSource{}, fmt.Errorf("acceptance source: bound revision is not a committed ancestor of HEAD")
	}
	rel, inside := relWithin(root, briefPath)
	if !inside {
		return acceptanceSource{}, fmt.Errorf("acceptance source: brief is outside the repository")
	}
	rel = filepath.ToSlash(rel)
	readme := filepath.ToSlash(filepath.Join(filepath.Dir(rel), "README.md"))
	briefBlob, raw, err := readAcceptanceBlob(root, rev, rel)
	if err != nil {
		return acceptanceSource{}, err
	}
	readmeBlob, _, err := readAcceptanceBlob(root, rev, readme)
	if err != nil {
		return acceptanceSource{}, err
	}

	_, evidence := briefSectionsOf(string(raw))
	lines := strings.Split(evidence, "\n")
	refusals := []acceptanceRefusal{}
	for _, signal := range scanHeldRefusals(evidence, false) {
		span := evidence
		if signal.line > 0 {
			span = lines[signal.line-1]
		}
		refusals = append(refusals, acceptanceRefusal{
			Line:   signal.line,
			Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(span))),
			Reason: signal.reason,
		})
	}
	return acceptanceSource{Repository: repo, Revision: rev, BriefPath: rel,
		BriefBlob: briefBlob, ReadmeBlob: readmeBlob, Refusals: refusals}, nil
}

func readAcceptanceBlob(root, rev, rel string) (string, []byte, error) {
	reader := ordinaryGitCore(root)
	if reader.err != nil || reader.native != "" {
		return "", nil, fmt.Errorf("acceptance source: could not open the canonical Git reader")
	}
	blob, err := reader.repo.TreeishID(rev, reader.prefix+rel)
	if err != nil {
		return "", nil, fmt.Errorf("acceptance source: resolve %s at %s: %w", rel, rev, err)
	}
	content, err := reader.repo.FileAt(rev, reader.prefix+rel)
	if err != nil {
		return "", nil, fmt.Errorf("acceptance source: read %s at %s: %w", rel, rev, err)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if resolved, inside := relWithin(root, path); !inside || resolved != rel {
		return "", nil, fmt.Errorf("acceptance source: %s does not resolve to its canonical path", rel)
	}
	working, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("acceptance source: read %s: %w", rel, err)
	}
	committed := []byte(content)
	if !bytes.Equal(working, committed) {
		return "", nil, fmt.Errorf("acceptance source: %s differs from the committed blob", rel)
	}
	return blob, committed, nil
}
