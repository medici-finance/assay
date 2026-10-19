#!/usr/bin/env python3
"""Positive controls for #1363: each broken input must redden its check."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

root = Path(__file__).resolve().parents[4]
scratch = Path(tempfile.mkdtemp(prefix="worker-kit-mutations-", dir=os.environ.get("TMPDIR")))
arms = "docs/streams/desk-supervision/08-arms"
scripts = "docs/streams/desk-supervision/verify.d"
brief = "docs/streams/desk-supervision/brief-08-objectives-over-transitions.md"
# Bounded tracked input only; never copy build output or the whole checkout.
archive = subprocess.Popen(["git", "-C", str(root), "archive", "HEAD", arms], stdout=subprocess.PIPE)
subprocess.run(["tar", "-x", "-C", str(scratch)], stdin=archive.stdout, check=True, timeout=20)
archive.stdout.close()
assert archive.wait(timeout=20) == 0
run = next((scratch / arms / "with-overlay").glob("*/run.json"))
record = json.loads(run.read_text())
record["check"] = "fail"
run.write_text(json.dumps(record))
result = subprocess.run(["bash", str(root / scripts / "08-reduction.sh"), str(scratch / arms)],
                        cwd=root, capture_output=True, text=True, timeout=90)
assert result.returncode == 1 and "task-check safety floor differs" in result.stderr, result.stderr
print("rejected planted failed task result")
# A second consumer must not be silently omitted by a check of only known sites.
(scratch / scripts).mkdir(parents=True)
shutil.copy2(root / scripts / "08-consumers.py", scratch / scripts / "08-consumers.py")
text = (root / brief).read_text().replace("\nversion: 1", '\n  - "tools/second-consumer.md: fixed-here"\nversion: 1', 1)
(scratch / brief).write_text(text)
result = subprocess.run(["python3", str(scratch / scripts / "08-consumers.py")],
                        capture_output=True, text=True, timeout=20)
assert result.returncode == 1 and "exactly three" in result.stderr, result.stderr
print("rejected planted additional consumer route")
print("2 planted defects rejected")
