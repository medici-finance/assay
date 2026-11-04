#!/usr/bin/env python3
"""Compile real owner faults in a disposable archive; never mutate the source checkout.

The caller must supply the explicitly built, source-bound custody fixture packet.
No missing Docker/profile prerequisite, setup error or SKIP can count as a kill.
"""
from pathlib import Path
import hashlib
import io
import json
import os
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[6]
MODULE = ROOT / "tools/desk"
OWNER = Path("internal/forgeservice/auth")

def command(args, cwd=ROOT):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()

def fault(name, file, before, after, test, witness):
    return (name, [(file, before, after)], test, witness)

FAULTS = [
    fault("construction-policy", "controller.go", "if CheckConfig(cfg) != nil || !clockPresent(wall)", "if !clockPresent(wall)", "Launch/policy", "malformed configuration accepted"),
    ("observed-launch", [("controller.go", "p.Launch.Match(cmd.Launch.Observation) != nil", "false", 2)], "Launch", "foreign resource observation accepted"),
    fault("issuer", "verifier.go", "claims.Issuer != v.config.Issuer || ", "", "Claims/issuer", "refusal released trusted fields"),
    fault("audience", "verifier.go", "claims.Audience != v.config.Audience || ", "", "Claims/audience", "refusal released trusted fields"),
    ("cell", [("verifier.go", "cfg.Policy.Cell != cfg.Bootstrap.Binding.Cell || ", ""), ("verifier.go", "cfg.Launch.Cell != cfg.Policy.Cell || ", "")], "Claims/constructor-cell", "foreign cell configuration accepted"),
    fault("repository-kind-admission", "verifier.go", " || claims.Scope.Scope != scope", "", "Claims/query-(repository|kind|admission)", "refusal released trusted fields"),
    fault("actor-source-composition-epochs-limits", "verifier.go", " || !same(claims.Scope, v.config.Policy)", "", "Claims/(actor|source|composition|access-epoch|config-epoch|policy-epoch|policy-class|limits)", "refusal released trusted fields"),
    ("role-intent", [("verifier.go", " || !same(claims.Scope, v.config.Policy)", ""), ("verifier.go", " || claims.LaunchDigest != launchDigest(v.config.Launch)", "")], "Claims/role", "refusal released trusted fields"),
    fault("closed-operation", "codec.go", "c.Operation != Operation || ", "", "ClosedCodec", "effect operation accepted"),
    fault("revocation", "controller.go", "r == nil || r.revoked || ", "r == nil || ", "Current/revoke", "refusal released trusted fields"),
    ("signature-key", [("verifier.go", "if !ed25519.Verify(b.PublicKey,", "if false && !ed25519.Verify(b.PublicKey,", 2)], "Claims/verification-key", "refusal released trusted fields"),
    fault("current-proof-signature", "verifier.go", 'if !ed25519.Verify(b.PublicKey, append([]byte("forge-local-current-v1\\n"), proof.Payload...), proof.Signature) {', 'if false {', "Current/forged-proof-after-revoke", "refusal released trusted fields"),
    fault("registration-needs-revoke-class", "controller.go", "return ControlReceipt{}, ErrRegistrationNeedsRevoke", "return ControlReceipt{}, ErrConflict", "RegistrationBudget/(live|expired)", "existing registration refusal class"),
    fault("registration-capacity-class", "controller.go", "return ControlReceipt{}, ErrCapacity", "return ControlReceipt{}, ErrConfig", "RegistrationBudget/lifetime-capacity", "capacity refusal class"),
    fault("current-challenge", "verifier.go", " || !bytes.Equal(p.Challenge, challenge)", "", "Current/challenge-replay", "refusal released trusted fields"),
    fault("request-start", "verifier.go", "until := began.Add(v.config.CurrentReadLease)", "until := time.Now().Add(v.config.CurrentReadLease)", "Current/delayed-current", "refusal released trusted fields"),
    fault("during-current-wall-expiry", "verifier.go", " || !now.Before(claims.ExpiresAt)", "", "Current/during-current-expiry", "refusal released trusted fields"),
    fault("final-wall-expiry", "verifier.go", " || !now.Before(r.lease.expires)", "", "Current/wall-release", "expired wall receipt released"),
    fault("producer-elapsed-expiry", "controller.go", "remaining := time.Until(r.until)", "remaining := c.config.MaxGrantLease", "Current/producer-monotonic-expiry", "refusal released trusted fields"),
    fault("final-elapsed-expiry", "verifier.go", " || !time.Now().Before(r.lease.until)", "", "Current/monotonic-release", "wall rollback renewed local release lease"),
    fault("original-cancellation", "verifier.go", " || r.lease.original.Err() != nil", "", "Current/cancel-release", "new context revived canceled receipt"),
    fault("single-release", "verifier.go", "if !r.lease.used.CompareAndSwap(false, true) {", "if false {", "Claims", "receipt reused"),
    ("channel-type", [("channels.go", "if e != nil || st.Mode()&os.ModeSocket == 0 {", "if e != nil {"), ("channels.go", "again.Mode()&os.ModeSocket == 0 || ", "")], "ChannelRestart/(regular|symlink)", "non-socket channel"),
    fault("reserved-control-capacity", "control.go", '"control.sock", c.controlSlots,', '"control.sock", c.dataSlots,', "ControlCapacity", "slow data slots blocked private revocation"),
    ("channel-live-gate", [("channels.go", "if ctx.Err() != nil || !c.liveGate() {", "if ctx.Err() != nil {"), ("channels.go", " || !c.liveGate() {", " {")], "ChannelRestart/changed-gate", "changed gate unlinked retained channel"),
    fault("channel-duplicate", "channels.go", "if c.channels[key] {", "if false {", "ChannelRestart/duplicate", "duplicate"),
    fault("private-volume-leak", "../../cellcontainer/authority_profile.go", 'mounts = []Mount{{"volume", c.Volumes.Current, "/authority-current", true}, {"volume", c.Volumes.Bootstrap, "/authority-bootstrap", true}}', 'mounts = []Mount{{"volume", c.Volumes.Current, "/authority-current", true}, {"volume", c.Volumes.Bootstrap, "/authority-bootstrap", true}, {"volume", c.Volumes.Private, "/authority-control", true}}', "Custody", "independent role escaped signer/control/other-launch containment"),
]

def main():
    packet = os.environ.get("AUTH57_CUSTODY_INPUT", "")
    if not packet or not Path(packet).is_file():
        raise SystemExit("UNQUALIFIED: explicit source-bound owned custody packet required")
    if not (ROOT / ".git").is_file() or command(["git", "status", "--porcelain"]):
        raise SystemExit("UNQUALIFIED: clean owned linked source worktree required")
    head = command(["git", "rev-parse", "HEAD"])
    archive = subprocess.check_output(["git", "archive", head], cwd=ROOT)
    out = Path(tempfile.mkdtemp(prefix="authority-controls-")).resolve()
    print("Private receipts: " + str(out), flush=True)
    source = out / "source"
    source.mkdir()
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        tar.extractall(source, filter="data")
    module = source / "tools/desk"
    env = dict(os.environ, GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local", KUBECONFIG="/dev/null")
    receipts = []
    def run(test, label):
        selector = "^TestForgeLocalAuthority" + test + "$"
        result = subprocess.run(["go", "test", "-count=1", "-timeout=5m", "-v", "-run", selector, "./internal/forgeservice/auth"], cwd=module, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=330)
        text = result.stdout.decode(errors="replace")
        (out / (label + ".log")).write_text(text)
        return result.returncode, text
    for name, edits, test, witness in FAULTS:
        code, baseline = run(test, name + "-baseline")
        parent = "TestForgeLocalAuthority" + test.split("/")[0]
        if code != 0 or "--- PASS: " + parent + " " not in baseline or "--- SKIP:" in baseline:
            raise SystemExit("UNQUALIFIED baseline: " + name)
        originals = {}
        try:
            for edit in edits:
                file, before, after, *counts = edit
                expected = counts[0] if counts else 1
                path = (module / OWNER / file).resolve()
                if not path.is_relative_to(module):
                    raise RuntimeError("fault escaped fixed module")
                originals.setdefault(path, path.read_bytes())
                text = path.read_text()
                if text.count(before) != expected:
                    raise RuntimeError("fault anchor drift: " + name)
                path.write_text(text.replace(before, after))
            code, mutated = run(test, name + "-mutant")
            if code == 0 or "[build failed]" in mutated or "--- SKIP:" in mutated or "--- FAIL: " + parent not in mutated or witness not in mutated:
                raise RuntimeError("NOT CAUGHT by independent compiling witness: " + name)
        finally:
            for path, data in originals.items():
                path.write_bytes(data)
                if path.read_bytes() != data:
                    raise RuntimeError("exact restoration failed")
        code, restored = run(test, name + "-restored")
        if code != 0 or "--- PASS: " + parent + " " not in restored or "--- SKIP:" in restored:
            raise SystemExit("restored baseline failed: " + name)
        receipts.append({"control": name, "caught": True, "restored": True, "originals": {str(p.relative_to(module)): hashlib.sha256(b).hexdigest() for p, b in originals.items()}})
        print("CAUGHT and restored: " + name, flush=True)
    (out / "RECEIPT.json").write_text(json.dumps({"source": head, "archive_sha256": hashlib.sha256(archive).hexdigest(), "controls": receipts}, indent=2) + "\n")
    print("PASS: local authority production controls killed; exact bytes restored")
    print("Private receipts: " + str(out))

if __name__ == "__main__":
    main()
