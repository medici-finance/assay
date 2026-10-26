"""Run bounded Go seam mutants, restoring production source after every run.

An unmutated control run must pass first. The per-run test timeout outlasts the
fixture's 10s hang, so the timeout mutant ends in a named test failure rather than a
harness timeout panic.
"""
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[3]
path = root / "tools/desk/internal/deskkit/layaadvisor.go"
wired = root / "tools/desk/internal/deskkit/zz_laya_wired_mutant.go"
original = path.read_text()
env_line = 'cmd.Env = []string{"HF_HUB_OFFLINE=1", "TRANSFORMERS_OFFLINE=1", "KUBECONFIG=/dev/null", "CUDA_VISIBLE_DEVICES="}'
mutations = {
    "approval": ('if !a.Approved {', 'if false {'),
    "cpu-only": ('if a.Backend != layaBackend {', 'if false {'),
    "normalise": ('if a.Backend != layaBackend {', 'if !bytes.EqualFold(bytes.TrimSpace([]byte(a.Backend)), []byte(layaBackend)) {'),
    "requested": ('if p.RequestedBackend != layaBackend {', 'if false {'),
    "requested-field": ('if p.RequestedBackend != layaBackend {', 'if p.RequestedBackend != a.Backend {'),
    "actual": ('if p.ActualBackend != layaBackend {', 'if false {'),
    "actual-field": ('if p.ActualBackend != layaBackend {', 'if p.ActualBackend != a.Backend {'),
    "fallback-reason": ('if response.FallbackReason != "" {', 'if false {'),
    "gpu-hidden": (env_line, env_line.replace(', "CUDA_VISIBLE_DEVICES="', '')),
    "env-inherit": (env_line, env_line.replace('[]string{', 'append(cmd.Environ(), ').replace('="}', '=")')),
    "timeout": ('exec.CommandContext(ctx, a.Command[0], a.Command[1:]...)', 'exec.Command(a.Command[0], a.Command[1:]...)'),
    "pipes": ('case <-ctx.Done():', 'case <-(chan struct{})(nil):'),
    "expired": ('func layaDeadline(ctx context.Context) error {', 'func layaDeadline(ctx context.Context) error { return nil;'),
    "abstention": ('!p.Abstained || len(p.LabelProbabilities)', 'len(p.LabelProbabilities)'),
    "output": ('if len(p) > b.limit-b.Len() {', 'if false {'),
    "second-copy": ('func (b *boundedBuffer) Len()', 'func (b *boundedBuffer) ReadFrom(r io.Reader) (int64, error) { return b.buffer.ReadFrom(r) }\nfunc (b *boundedBuffer) Len()'),
    "roundtrip": ('return p, nil', 'return fail, nil'),
    "malformed": ('return fail, fmt.Errorf("laya: malformed response: %w", err)', 'return fail, nil'),
    "bytes": ('return fail, Refused("laya: input byte limit")', 'return fail, nil'),
}
# Not a source edit: a production file that names the advisor must redden the unwired check.
extra = {"wiring": "package deskkit\n\nvar _ = LayaAdvisor{}\n"}


def go_test():
    return subprocess.run(["go", "test", "-timeout", "30s", "-count=1", "-run", "^TestLaya(Advisor|Buffer)", "./internal/deskkit"], cwd=root/"tools/desk", capture_output=True, text=True, timeout=60)


def caught(name, result):
    if result.returncode == 0 or not any(marker in result.stdout for marker in ("--- FAIL: TestLayaAdvisor", "--- FAIL: TestLayaBuffer")):
        raise SystemExit(f"mutation {name} not caught: {result.stdout}{result.stderr}")
    print(f"checked-failed as expected: {name}")


if wired.exists():
    raise SystemExit(f"stale mutant file present: {wired}")
control = go_test()
if control.returncode != 0:
    raise SystemExit(f"control fails unmutated: {control.stdout}{control.stderr}")
print("control passed unmutated")
for name, (old, new) in mutations.items():
    if original.count(old) != 1:
        raise SystemExit(f"mutation {name}: matcher changed")
    try:
        path.write_text(original.replace(old, new))
        caught(name, go_test())
    finally:
        path.write_text(original)
for name, source in extra.items():
    try:
        wired.write_text(source)
        caught(name, go_test())
    finally:
        wired.unlink(missing_ok=True)
total = len(mutations) + len(extra)
print(f"{total} of {total} mutants caught")
