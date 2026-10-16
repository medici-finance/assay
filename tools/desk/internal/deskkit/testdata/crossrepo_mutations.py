#!/usr/bin/env python3
"""Offline #2485 regression mutations; run only in an owned checkout.

Each edit must land exactly once, compile, and cause a test assertion failure.
Source is restored even on failure. No two mutation runs may share this tree.
Optional positional names select entries; no arguments runs every entry.
"""
from pathlib import Path
import os
import subprocess
import sys

module = Path(__file__).resolve().parents[3]
source = module.parents[1]
# name, file, old, new, package, exact test selection, module directory
entries = [
 ("checkout-filter-before-execution", "internal/deskkit/verifiertrackingcheckout.go", 'return Refused("tracking source requires a Git filter before admission")', "continue", "./internal/deskkit", "^TestCrossRepoCheckoutPrograms$/filter", module),
 ("checkout-hook-before-execution", "internal/deskkit/verifiertrackingcheckout.go", 'return Refused("tracking checkout has an executable post-checkout hook")', "// planted unchecked checkout hook", "./internal/deskkit", "^TestCrossRepoCheckoutPrograms$/hook", module),
 ("original-refusal", "internal/deskkit/verifierattestation.go", "return prepareVerifierCrossRepo(root, repo, brief, model, tier)", 'return Refused("verifier brief must be inside its source worktree")', "./cmd/deskdispatch", "^TestCrossRepoDispatchExecution$", module),
 ("source-closure", "internal/deskkit/verifiercrossrepo.go", "if err := verifierSourceClosure(home, b.Source, checkout, nil); err != nil {", "if err := verifierSourceClosure(home, b.Source, checkout, nil); false && err != nil {", "./internal/deskkit", "^TestVerifierCrossRepoTampering$/source_bytes", module),
 ("declaration", "internal/deskkit/verifierassociation.go", 'return Refused("immutable tracking brief does not declare the bound deliverable repository")', "return nil", "./internal/deskkit", "^TestCrossRepoPrepareRefusals$/wrong_declaration", module),
 ("visibility", "internal/deskkit/verifiercrossrepo.go", 'return Refused("verifier tracking destination cannot publish the deliverable repository binding")', "return nil", "./internal/deskkit", "^TestCrossRepoVisibility$", module),
 ("recovery-diagnostic", "internal/deskkit/verifierattestation.go", "missing original pre-work verifier record: run nonce, repository/source, brief/plan, home and model provenance are unavailable; cannot recover or execute this run", "create a new verifier run", "./internal/deskkit", "^TestCrossRepoLegacyRecovery$", module),
 ("recovery-duplicate", "internal/deskkit/verifierattestation.go", "if r.Issue == 0 && r.Attempted {", "if false && r.Issue == 0 && r.Attempted {", "./internal/deskkit", "^TestVerifierCrossRepoRoundTrip$", module),
 ("wake-source-dirty", "internal/deskkit/verifywake.go", "verifierSourceClosure(root, head, checkout, nil) != nil", "(false && verifierSourceClosure(root, head, checkout, nil) != nil)", "./internal/deskkit", "^TestCrossRepoWakeManifest$", module),
 ("source-input", "internal/deskkit/verifyoutcomes.go", "inputs[InputKeyRepoSourcePrefix+in.SourceRepo] = in.Source", "inputs[InputKeyRepoSourcePrefix+in.SourceRepo] = strings.Repeat(\"f\", 40)", "./cmd/deskevidence", "^TestCrossRepoEvidenceOutcome$", module),
 ("second-one-root-consumer", "internal/deskkit/verifiercrossrepo.go", "return r.BriefRoot", "return sourceRoot", "./cmd/deskevidence", "^TestCrossRepoEvidenceOutcome$", module),
 ("delivery-join", "cmd/deskevidence/outcomebinding.go", 'return deskkit.Refused("delivery PR is not in the admitted deliverable source")', "return nil", "./cmd/deskevidence", "^TestCrossRepoDeliveryJoin$", module),
 ("outcome-env", "cmd/deskevidence/receiptvalidate.go", "cmd.Env = deskkit.VerifierReadEnv()", "// planted ambient history reader", "./cmd/deskevidence", "^TestCrossRepoOutcomeGitEnv$", module),
 ("queue-revision", "cmd/verifyloop/association.go", "if got.TargetSHA != it.TargetSHA {", "if false && got.TargetSHA != it.TargetSHA {", "./cmd/verifyloop", "^TestCrossRepoQueueAssociation$/target", module),
 ("adapter-binding", "cmd/verifyloop/attestation.go", 'err = fmt.Errorf("verifier admission differs from queued tracking/source association")', "err = nil", "./cmd/verifyloop", "^TestCrossRepoAdapterLanding$/tracking", module),
 ("prompt-history", "cmd/verifyloop/association.go", "return text[:at[0]]", "return text", "./cmd/verifyloop", "^TestCrossRepoPromptCutsEvidence$", module),
 ("prompt-symlink", "cmd/deskdispatch/verifierattestation.go", 'return deskkit.Refused("cross-repository verifier prompt must be a regular local file")', "return nil", "./cmd/deskdispatch", "^TestCrossRepoPromptDestination$", module),
 ("prompt-hardlink", "cmd/deskdispatch/verifierattestation.go", "return os.Rename(name, file)", "return os.WriteFile(file, content, 0600)", "./cmd/deskdispatch", "^TestCrossRepoPromptDestination$", module),
 ("executor-routing", "verifieradmission.go", "filepath.Join(receipt.BriefRoot, b.Brief), receipt.SourceHome, nil", "brief, receipt.SourceHome, nil", ".", "^TestCrossRepoReceiptRouting$", source / "statusgen"),
 ("claim-root-origin", "cmd/deskdispatch/verifierattestation.go", "|| !strings.EqualFold(actual, dl.trackingRepo) ||", "|| (false && !strings.EqualFold(actual, dl.trackingRepo)) ||", "./cmd/deskdispatch", "^TestCrossRepoClaimRootOrigin$", module),
 ("prepare-distinct", "internal/deskkit/verifiercrossrepo.go", "!IsAllowedRepo(trackingRepo) || strings.EqualFold(repo, trackingRepo)", "!IsAllowedRepo(trackingRepo)", "./internal/deskkit", "^TestCrossRepoSameRepositoryRefusals$/prepare", module),
 ("check-distinct", "internal/deskkit/verifiercrossrepo.go", "if strings.EqualFold(b.Repo, b.SourceRepo) {", "if false && strings.EqualFold(b.Repo, b.SourceRepo) {", "./internal/deskkit", "^TestCrossRepoSameRepositoryRefusals$/check", module),
 ("receipt-home-emitted", "internal/deskkit/verifierattestation.go", "r.BriefRoot, r.SourceHome = r.TrackingHome, r.Home", "r.BriefRoot = r.TrackingHome", "./cmd/deskdispatch", "^TestCrossRepoDispatchExecution$", module),
 ("receipt-home-required", "verifieradmission.go", "|| !filepath.IsAbs(receipt.SourceHome)", "", ".", "^TestCrossRepoReceiptRouting$", source / "statusgen"),
 ("executor-root-default", "../../statusgen/verifyrun.go", 'case admittedHome != "":\n\t\t\treturn admittedHome', 'case false:\n\t\t\treturn admittedHome', "./cmd/deskdispatch", "^TestCrossRepoDispatchExecution$", module),
 ("executor-root-explicit", "../../statusgen/verifyrun.go", 'if why := admittedRowRootRefusal(*rootDir, admissionRoot, admittedHome); why != "" {', 'if why := admittedRowRootRefusal(*rootDir, admissionRoot, admittedHome); false && why != "" {', "./cmd/deskdispatch", "^TestCrossRepoDispatchExecution$", module),
 ("executor-root-container", "../../statusgen/verifyrun.go", "\t\tif admittedHome != \"\" {\n\t\t\tfmt.Fprintf(stderr, \"statusgen verifyrun: refusing --in-container", "\t\tif false {\n\t\t\tfmt.Fprintf(stderr, \"statusgen verifyrun: refusing --in-container", "./cmd/deskdispatch", "^TestCrossRepoDispatchExecution$", module),
 ("executor-root-helper", "verifieradmission.go", 'explicitRoot != "" && !sameDirectory(explicitRoot, home)', "false", ".", "^TestAdmittedRowRootRefusal$", source / "statusgen"),
 ('check-attached-branch', 'internal/deskkit/verifiercrossrepo.go', 'if head != b.Source || branch != "HEAD" {', 'if _ = branch; head != b.Source {', './internal/deskkit', '^TestCrossRepoCheckGuardReasons$/attached_branch', module),
 ('check-containment', 'internal/deskkit/verifiercrossrepo.go', '|| verifierContains(home, tracking) || verifierContains(tracking, home) {', '{', './internal/deskkit', '^TestCrossRepoCheckGuardReasons$/tracking_home_nested', module),
 ('check-tracking-digest', 'internal/deskkit/verifiercrossrepo.go', '|| verifierDigest([]byte(tracking)) != b.TrackingHomeSHA256 ||', '||', './internal/deskkit', '^TestCrossRepoCheckGuardReasons$/tracking_home_digest', module),
 ('check-tracking-origin', 'internal/deskkit/verifiercrossrepo.go', 'if err := verifierRepoIdentity(tracking, b.Repo); err != nil {', 'if err := verifierRepoIdentity(tracking, b.Repo); false && err != nil {', './internal/deskkit', '^TestCrossRepoCheckGuardReasons$/tracking_origin', module),
 ('check-source-origin', 'internal/deskkit/verifiercrossrepo.go', 'if err := verifierRepoIdentity(home, b.SourceRepo); err != nil {', 'if err := verifierRepoIdentity(home, b.SourceRepo); false && err != nil {', './internal/deskkit', '^TestCrossRepoCheckGuardReasons$/deliverable_origin', module),
 ('check-visibility', 'internal/deskkit/verifiercrossrepo.go', 'if err := verifierVisibility(b.Repo, b.SourceRepo); err != nil {', 'if err := verifierVisibility(b.Repo, b.SourceRepo); false && err != nil {', './internal/deskkit', '^TestCrossRepoCheckGuardReasons$/visibility', module),
 ('check-provenance', 'internal/deskkit/verifiercrossrepo.go', 'if b.SourceRepo == "" || b.TrackingSource == "" || b.TrackingHomeSHA256 == "" || r.TrackingHome == "" || b.TrackingCheckout == "" {', 'if b.SourceRepo == "" {', './internal/deskkit', '^TestCrossRepoCheckGuardReasons$/missing', module),
 ('prepare-rerun', 'internal/deskkit/verifiercrossrepo.go', 'if _, err := os.Stat(record); err == nil {', 'if _, err := os.Stat(record); false && err == nil {', './internal/deskkit', '^TestCrossRepoPrepareGuardReasons$/run_exists', module),
 ('prepare-authority', 'internal/deskkit/verifiercrossrepo.go', 'if !verifierAuthority(desk) {', 'if false && !verifierAuthority(desk) {', './internal/deskkit', '^TestCrossRepoPrepareGuardReasons$/authority', module),
 ('tracking-submodule', 'internal/deskkit/verifiertrackingcheckout.go', 'if strings.HasPrefix(entry, "160000 ") {', 'if false && strings.HasPrefix(entry, "160000 ") {', './internal/deskkit', '^TestCrossRepoTrackingSubmodule$', module),
 ('association-schema', 'internal/deskkit/verifierassociation.go', '|| graph.Schema != "graph-repos-v1" {', '{', './internal/deskkit', '^TestCrossRepoDeclarationReasons$/registry_schema', module),
 ('association-conflict', 'internal/deskkit/verifierassociation.go', 'if fm.Homed != "" && !strings.EqualFold(fm.Homed, repo) {', 'if false && fm.Homed != "" && !strings.EqualFold(fm.Homed, repo) {', './internal/deskkit', '^TestCrossRepoDeclarationReasons$/conflicting', module),
 ('association-unregistered', 'internal/deskkit/verifierassociation.go', 'return nil, Refused("verifier homed-in repository is not registered")', 'return nil, nil', './internal/deskkit', '^TestCrossRepoDeclarationReasons$/unregistered', module),
 ('snapshot-main', 'internal/deskkit/verifierassociation.go', 'if main != revision {', 'if false && main != revision {', './cmd/verifyloop', '^TestCrossRepoAssociationLaggingCheckout$/distinct', module),
 ('association-same-repo', 'cmd/verifyloop/association.go', 'if len(repos) == 0 || (len(repos) == 1 && strings.EqualFold(repos[0], repo)) {', 'if len(repos) == 0 {', './cmd/verifyloop', '^TestCrossRepoAssociationLaggingCheckout$/same', module),
 ('association-ambiguous', 'cmd/verifyloop/association.go', 'if len(repos) != 1 {', 'if false && len(repos) != 1 {', './cmd/verifyloop', '^TestCrossRepoAssociationRefusals$/ambiguous', module),
 ('association-root-origin', 'cmd/verifyloop/association.go', 'if !strings.EqualFold(sourceRepo, repos[0]) {', 'if false && !strings.EqualFold(sourceRepo, repos[0]) {', './cmd/verifyloop', '^TestCrossRepoAssociationRefusals$/deliverable_root_origin', module),
 ('packet-tracking-revision', 'cmd/deskdispatch/packet_verifier.go', 'if in.plan.verifierBriefRevision != "" && head != in.plan.verifierBriefRevision {', 'if false && in.plan.verifierBriefRevision != "" && head != in.plan.verifierBriefRevision {', './cmd/deskdispatch', '^TestCrossRepoDispatchExecution$', module),
 ('checkout-canonical', 'internal/deskkit/verifiertrackingcheckout.go', '_, root, err := verifierHome(root)', 'var err error', './internal/deskkit', '^TestCrossRepoLinkedPaths$/tracking_checkout_path', module),
 ('wake-root-canonical', 'internal/deskkit/verifywake.go', 'if _, root, err = verifierHome(root); err != nil || verifierRepoIdentity(root, repo) != nil {', 'if err = nil; verifierRepoIdentity(root, repo) != nil {', './internal/deskkit', '^TestCrossRepoLinkedPaths$/wake_root', module),
]
selected = set(sys.argv[1:])
known = {e[0] for e in entries}
if selected - known:
    raise SystemExit("unknown mutation: " + ", ".join(sorted(selected - known)))
for name, rel, old, new, package, tests, cwd in entries:
    if selected and name not in selected:
        continue
    path = cwd / rel
    original = path.read_text()
    if original.count(old) != 1:
        raise SystemExit(f"{name}: anchor absent/ambiguous in {rel}")
    try:
        path.write_text(original.replace(old, new))
        env = dict(os.environ, GOWORK="off", KUBECONFIG="/dev/null")
        result = subprocess.run(["go", "test", package, "-run", tests, "-count=1", "-timeout=90s"], cwd=cwd, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if result.returncode == 0 or "--- FAIL:" not in result.stdout or "build failed" in result.stdout:
            raise SystemExit(f"{name}: no assertion failure\n{result.stdout}")
        print(f"{name}: killed", flush=True)
        print("\n".join(line for line in result.stdout.splitlines() if "--- FAIL:" in line or "_test.go:" in line), flush=True)
    finally:
        path.write_text(original)
