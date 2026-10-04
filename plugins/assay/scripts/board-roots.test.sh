#!/usr/bin/env bash
# Board-root derivations must use the configured inventory, not compiled topology.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
python3 - "$ROOT" <<'PY'
from pathlib import Path
import os
import re
import subprocess
import sys
import tempfile

# Covers both inline tables and shell derivations continued on the next line.
command = re.compile(r"deskroster\s+repos\s+--scope\s+topology\b")
def stale_roots(root):
    hits = []
    for path in sorted(root.rglob("*.md")):
        text = path.read_text(encoding="utf-8")
        for match in command.finditer(text):
            block = text[match.end():].split("\n\n", 1)[0]
            if "root=" in block:
                line = text.count("\n", 0, match.start()) + 1
                hits.append(f"{path.relative_to(root)}:{line}")
    return hits

root = Path(sys.argv[1]) / "plugins" / "assay"
if not (root / "skills").is_dir():
    raise SystemExit("could-not-check: plugin skill corpus is absent")
# Positive control: a planted second consumer must be named, not silently ignored.
with tempfile.TemporaryDirectory() as directory:
    fixture = Path(directory)
    second = fixture / "new-consumer.md"
    second.write_text("DECLARED=$(deskroster repos --scope topology \\\n | awk '/root=/')\n", encoding="utf-8")
    assert stale_roots(fixture) == ["new-consumer.md:1"], "second consumer was not caught"
    second.write_text("DECLARED=$(deskroster repos --scope roots \\\n | awk '/root=/')\n", encoding="utf-8")
    assert stale_roots(fixture) == [], "configured-root consumer was rejected"
hits = stale_roots(root)
if hits:
    raise SystemExit("compiled topology used to derive board roots: " + ", ".join(hits))

# Execute the runbook's actual extraction, including its failure propagation.
runbook = (root / "skills/worker-desk/references/dispatch-runbook.md").read_text(encoding="utf-8")
extract = runbook.split("```bash\n", 1)[1].split("# observed:", 1)[0]
with tempfile.TemporaryDirectory() as directory:
    stub = Path(directory) / "deskroster"
    stub.write_text("#!/usr/bin/env bash\n"
                    "[[ \"$*\" = 'repos --scope roots' ]] || exit 5\n"
                    "[[ ${FAIL_ROOTS:-0} = 0 ]] || exit 5\n"
                    "printf 'example-org/demo\\troot=./custom-demo\\n'\n", encoding="utf-8")
    stub.chmod(0o700)
    env = dict(os.environ, PATH=directory + os.pathsep + os.environ["PATH"], FAIL_ROOTS="0")
    invocation = ["bash", "--noprofile", "--norc", "-c", extract + '\nprintf "%s\\n" "$DECLARED"']
    result = subprocess.run(invocation, env=env, capture_output=True, text=True)
    assert result.returncode == 0 and result.stdout == "./custom-demo\n", result
    env["FAIL_ROOTS"] = "1"
    result = subprocess.run(invocation, env=env, capture_output=True, text=True)
    assert result.returncode == 6 and result.stdout == "", result
    assert "could-not-check" in result.stderr, result
print("board-roots.test.sh: OK (configured inventory; planted second consumer caught)")
PY
