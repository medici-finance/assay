#!/usr/bin/env python3
# Issue #2798 review controls. Run only from a disposable/owned worktree.
# Assert a green pristine baseline, require one anchor and an assertion kill per
# control, and restore the original source after every probe, including failure.
from pathlib import Path
import subprocess,os,sys
module=Path(__file__).resolve().parents[3]
p=module/"internal/deskkit/declaredscope.go"
entries=[
 ("outside-context", 'if marker && (!context || raw != line) {', 'if false {', '^TestScope(ReviewedCorpusRefusesPartial|ContinuationPositions)$'),
 ("field-position", '(raw == line && scopeFieldKey(line))', '(strings.HasSuffix(strings.Fields(line)[0], ":") && !strings.HasPrefix(line, "-"))', '^TestScope(ReviewedCorpusRefusesPartial|ContinuationPositions)$'),
 ("unsupported-marker", 'if marker && !strings.HasPrefix(line, "files:") {', 'if false {', '^TestScopeContinuationPositions$'),
 ("hyphen-tail", '[]string{" — "}', '[]string{" — ", " – ", " - "}', '^TestScopeContinuationPositions$'),
 ("action-annotation", 'return nil, Refused("refused: unclosed Context files: action annotation")', 'break', '^TestScopeContinuationPositions$'),
 ("loose-continuation", 'else if strings.HasPrefix(line, "- ") {', 'else if false {', '^TestScopeLooseContinuation$'),
 ("resolution-work", 'if n > maxScopeWork-work {', 'if false {', '^TestScopeResolutionWorkBound$'),
]
selected=set(sys.argv[1:])
known={entry[0] for entry in entries}
if selected-known:raise SystemExit("unknown control: "+", ".join(sorted(selected-known)))
entries=[entry for entry in entries if not selected or entry[0] in selected]
baseline=subprocess.run(["go","test","./internal/deskkit","-run","^TestScope(ReviewedCorpusRefusesPartial|ContinuationPositions|ResolutionWorkBound|LooseContinuation)$","-count=1","-timeout=60s"],cwd=module,env=dict(os.environ,GOWORK="off",KUBECONFIG="/dev/null"),capture_output=True,text=True)
if baseline.returncode:raise SystemExit("pristine baseline failed\n"+baseline.stdout+baseline.stderr)
print("pristine baseline: green",flush=True)
original=p.read_text()
for name,old,new,tests in entries:
 if original.count(old)!=1:raise SystemExit(name+": anchor absent/ambiguous")
 try:
  p.write_text(original.replace(old,new))
  result=subprocess.run(["go","test","./internal/deskkit","-run",tests,"-count=1","-timeout=60s"],cwd=module,env=dict(os.environ,GOWORK="off",KUBECONFIG="/dev/null"),capture_output=True,text=True)
  output=result.stdout+result.stderr
  if result.returncode==0 or "--- FAIL:" not in output or "build failed" in output:raise SystemExit(name+": no assertion kill\n"+output)
  print(name+": killed",flush=True)
  print("\n".join(l for l in output.splitlines() if "--- FAIL:" in l or "_test.go:" in l),flush=True)
 finally:p.write_text(original)
