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
 ("executor-routing", "verifieradmission.go", "filepath.Join(receipt.BriefRoot, b.Brief), nil", "brief, nil", ".", "^TestCrossRepoReceiptRouting$", source / "statusgen"),
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
