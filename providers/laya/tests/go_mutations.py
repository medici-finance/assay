"""Run bounded Go seam mutants, restoring production source after every run."""
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[3]
path = root / "tools/desk/internal/deskkit/layaadvisor.go"
original = path.read_text()
mutations = {
    "fallback": ('a.AllowCPUFallback && response.FallbackReason != ""', 'response.FallbackReason != ""'),
    "approval": ('if !a.Approved {', 'if false {'),
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
for name, (old, new) in mutations.items():
    if original.count(old) != 1:
        raise SystemExit(f"mutation {name}: matcher changed")
    try:
        path.write_text(original.replace(old, new))
        result = subprocess.run(["go", "test", "-timeout", "10s", "-count=1", "-run", "^TestLaya(Advisor|Buffer)", "./internal/deskkit"], cwd=root/"tools/desk", capture_output=True, text=True, timeout=30)
        if result.returncode == 0 or not any(marker in result.stdout for marker in ("--- FAIL: TestLayaAdvisor", "--- FAIL: TestLayaBuffer")):
            raise SystemExit(f"mutation {name} not caught: {result.stdout}{result.stderr}")
        print(f"checked-failed as expected: {name}")
    finally:
        path.write_text(original)
