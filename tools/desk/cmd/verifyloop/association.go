package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// Planning has a pass-local reader. Dispatch deliberately keeps the original
// fresh Git checks (including their config scope); no scan reader is retained.
type verifierAssociationReader interface {
	Head(string) (string, string, error)
	Snapshot(string) (string, string, error)
	Deliverables(string, string, string) ([]string, error)
}

type freshVerifierAssociation struct{}

func (freshVerifierAssociation) Head(root string) (string, string, error) {
	return deskkit.VerifierRepositoryHead(root)
}
func (freshVerifierAssociation) Snapshot(root string) (string, string, error) {
	return deskkit.VerifierRepositorySnapshot(root)
}
func (freshVerifierAssociation) Deliverables(root, revision, brief string) ([]string, error) {
	return deskkit.VerifierDeliverables(root, revision, brief)
}

// associateVerifierItem adds the source side of a declared cross-repository
// item without changing its tracking identity. Plain legacy same-repo items
// keep their existing representation. Dispatch rechecks this association.
func associateVerifierItem(it loopengine.Item, root string) (loopengine.Item, error) {
	// A fresh observation is mandatory outside the scanner, especially dispatch.
	return associateVerifierItemIn(it, root, freshVerifierAssociation{})
}

func associateVerifierItemIn(it loopengine.Item, root string, pass verifierAssociationReader) (loopengine.Item, error) {
	originalPayload := it.Payload
	it.Payload = map[string]string{}
	for k, v := range originalPayload {
		it.Payload[k] = v
	}
	for _, key := range []string{"source_repo", "source_root", "tracking_sha"} {
		delete(it.Payload, key)
	}
	raw, err := os.ReadFile(filepath.Join(root, it.BriefPath))
	if err != nil {
		return it, err
	}
	text := string(raw)
	if !strings.Contains(text, "deliverable_repo:") && !strings.Contains(text, "homed-in:") && len(deskkit.BriefFilesTags(text)) == 0 {
		return it, nil
	}
	// Declarations are read from the commit the scan read (HEAD), so whether the
	// work is cross-repository at all is decided before, and independently of,
	// the fetched-main gate. A same-repository brief (an alias tag or homed-in
	// naming this repository) keeps its legacy representation even while the
	// checkout lags fetched main; only a distinct deliverable needs the snapshot.
	repo, head, err := pass.Head(root)
	if err != nil {
		return it, err
	}
	repos, err := pass.Deliverables(root, head, it.BriefPath)
	if err != nil {
		return it, err
	}
	if len(repos) == 0 || (len(repos) == 1 && strings.EqualFold(repos[0], repo)) {
		return it, nil
	}
	if len(repos) != 1 {
		return it, deskkit.Unverifiable("verifier queue requires one unambiguous declared deliverable", nil)
	}
	_, trackingSHA, err := pass.Snapshot(root)
	if err != nil {
		return it, err
	}
	sourceRoot := deskkit.RootForRepo(repos[0])
	if sourceRoot == "" {
		return it, deskkit.Unverifiable("verifier deliverable root is not configured", nil)
	}
	sourceRepo, sourceSHA, err := pass.Snapshot(sourceRoot)
	if err != nil {
		return it, err
	}
	if !strings.EqualFold(sourceRepo, repos[0]) {
		return it, deskkit.Refused("configured verifier deliverable root has the wrong origin")
	}
	payload := map[string]string{}
	for k, v := range it.Payload {
		payload[k] = v
	}
	payload["repo"], payload["root"], payload["tracking_sha"] = repo, root, trackingSHA
	payload["source_repo"], payload["source_root"] = sourceRepo, sourceRoot
	it.Payload, it.TargetSHA = payload, sourceSHA
	return it, nil
}

func checkQueuedAssociation(it loopengine.Item) error {
	if it.Payload["source_repo"] == "" {
		return nil
	}
	got, err := associateVerifierItem(it, it.Payload["root"])
	if err != nil {
		return err
	}
	if got.TargetSHA != it.TargetSHA {
		return deskkit.Refused("queued verifier deliverable revision changed")
	}
	for _, key := range []string{"repo", "root", "tracking_sha", "source_repo", "source_root"} {
		if got.Payload[key] == "" || got.Payload[key] != it.Payload[key] {
			return deskkit.Refused("queued verifier association changed: " + key)
		}
	}
	return nil
}

// The full attested brief is local prompt context. Native ACP's source-only
// filesystem fence stays unchanged; no private file is copied into the source.
func admittedVerifierPrompt(it loopengine.Item, tier loopengine.Tier, receipt deskkit.VerifierReceipt) (string, error) {
	if receipt.Binding.Version != 2 {
		return renderDispatchPrompt(it, tier), nil
	}
	raw, err := os.ReadFile(filepath.Join(receipt.BriefRoot, receipt.Binding.Brief))
	if err != nil {
		return "", err
	}
	local := it
	local.Payload = map[string]string{}
	for k, v := range it.Payload {
		local.Payload[k] = v
	}
	delete(local.Payload, "root")
	local.Payload["verify_table"] = extractVerify(string(raw))
	prompt := renderDispatchPrompt(local, tier)
	prompt = strings.Replace(prompt, "Create and work in your OWN temporary worktree under /private/tmp off the target SHA.", "Work in the already admitted deliverable home; do not allocate a replacement run.", 1)
	return prompt + fmt.Sprintf("\nDeliverable repository: %s\nTracking revision: %s\nThe admitted tracking brief follows as local context; keep it out of the deliverable repository and public records. Run commands in the admitted deliverable home.\n\n%s\n", receipt.Binding.SourceRepo, receipt.Binding.TrackingSource, verifierPromptPlan(string(raw))), nil
}

var verifierPromptEvidence = regexp.MustCompile(`(?im)^\s*#{1,6}\s+[^a-z0-9]*(?:[0-9]+[^a-z0-9]+)*evidence(?:[^a-z0-9]|$)`)

func verifierPromptPlan(text string) string {
	if at := verifierPromptEvidence.FindStringIndex(text); at != nil {
		return text[:at[0]]
	}
	return text
}

// itemWorldSpec reads what one item's run needs from its brief: the declared
// sibling checkouts and the pinned instruments its runner-executed Verify rows
// call. It reads the brief already materialized under root and nothing else.
func itemWorldSpec(it loopengine.Item, root string) (deskkit.WorldSpec, error) {
	spec := deskkit.WorldSpec{Root: root}
	if strings.TrimSpace(it.BriefPath) == "" {
		return spec, fmt.Errorf("item %s names no brief to read prerequisites from", it.ID)
	}
	raw, err := os.ReadFile(filepath.Join(root, it.BriefPath))
	if err != nil {
		return spec, fmt.Errorf("cannot read the brief's prerequisites: %w", err)
	}
	var commands []string
	for _, row := range parseVerifyRows(string(raw)) {
		if row.runnerExecuted() {
			commands = append(commands, row.Command)
		}
	}
	spec.Siblings = deskkit.DeclaredSiblings(string(raw))
	spec.Instruments = deskkit.WorldInstruments(root, commands)
	return spec, nil
}
