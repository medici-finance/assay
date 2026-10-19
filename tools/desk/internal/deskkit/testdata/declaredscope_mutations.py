#!/usr/bin/env python3
# Issue #2798 review controls. Run only from a disposable/owned worktree.
# Assert a green pristine baseline, require one anchor and an assertion kill per
# control, and restore the original source after every probe, including failure.
from pathlib import Path
import subprocess,os,sys
module=Path(__file__).resolve().parents[3]
p=module/"internal/deskkit/declaredscope.go"
entries=[
 ("outside-context", 'if marker && (!context || !columnZero) {', 'if false {', '^TestScope(ReviewedCorpusRefusesPartial|ContinuationPositions)$'),
 ("field-position", 'else if line == "" || (columnZero && scopeFieldKey(line)) {', 'else if line == "" || (strings.HasSuffix(strings.Fields(line)[0], ":") && !strings.HasPrefix(line, "-")) {', '^TestScope(ReviewedCorpusRefusesPartial|ContinuationPositions)$'),
 ("unsupported-marker", 'if marker && !strings.HasPrefix(line, "files:") {', 'if false {', '^TestScopeContinuationPositions$'),
 ("hyphen-tail", '[]string{" — "}', '[]string{" — ", " – ", " - "}', '^TestScopeContinuationPositions$'),
 ("action-annotation", 'return nil, Refused("refused: unclosed Context files: action annotation")', 'break', '^TestScopeContinuationPositions$'),
 ("loose-continuation", 'else if scopeLooseContinuation(raw, line) {', 'else if false {', '^TestScopeLooseContinuation$'),
 ("resolution-work", 'if n > maxScopeWork-work {', 'if false {', '^TestScopeResolutionWorkBound$'),
 ("annotated-field", r"(?: \([^)]*\))?", "", '^TestScopeAcceptedFieldBoundaries$'),
 ("prose-marker", 'outsideFieldMarker := scopeOutsideFilesFieldPattern.MatchString(strings.TrimLeft(line, "*_"))', 'outsideFieldMarker := scopeBroadFilesMarker(markerLine)', '^TestScopeAcceptedFieldBoundaries$'),
 ("formatted-marker", 'markerLine := scopeMarkerText(line)', 'markerLine := strings.ToLower(strings.TrimLeft(line, "*_"))', '^TestScopeFormattedMarkersRefuse$'),
 ("loose-markers", 'scopeListPrefix(line) != 0 || strings.HasPrefix(line, "`")', 'strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "`")', '^TestScopeLooseMarkdownContinuationsRefuse$'),
 ("unclosed-fence", 'if fence != "" {\n\t\treturn nil, Refused("refused: unclosed Markdown fence; declared scope cannot be established")', 'if false {\n\t\treturn nil, Refused("refused: unclosed Markdown fence; declared scope cannot be established")', '^TestScopeFenceIntegrity$'),
 ("fence-width", 'fence = line[:length]', 'fence = line[:length-length+3]', '^TestScope(AcceptedFieldBoundaries|FenceIntegrity)$'),
 ("unknown-field", '"layout", "out-of-repo files":', '"layout", "out-of-repo files", "see":', '^TestScopeLooseMarkdownContinuationsRefuse$'),
 ("fence-opener-indent", 'if !fenceAtColumn || (line[0]', 'if false || (line[0]', '^TestScope(FenceIntegrity|FenceOpeners|MalformedFenceReceiptRefuses)$'),
 ("fence-opener-info", "(line[0] == '`' && strings.ContainsRune(line[length:], '`'))", 'false', '^TestScope(FenceIntegrity|MalformedFenceReceiptRefuses|InlineSpanBeforeContext)$'),
 ("fence-closer-indent", 'if fenceAtColumn && strings.HasPrefix(line, fence)', 'if strings.HasPrefix(line, fence)', '^TestScope(FenceIntegrity|FenceOpeners)$'),
 ("fence-context-indent", 'if context && !columnZero {\n\t\t\t\treturn nil, Refused("refused: indented Context fence may belong to a container")', 'if false {\n\t\t\t\treturn nil, Refused("refused: indented Context fence may belong to a container")', '^TestScopeFenceIntegrity$'),
 ("fence-declaration", 'if context && (block || endedByBlank) {', 'if false {', '^TestScopeFenceIntegrity$'),
 ("fence-hidden-heading", 'else if strings.HasPrefix(line, "## ") {', 'else if false {', '^TestScopeFenceIntegrity$'),
 ("fence-hidden-marker", 'else if scopeFilesFieldPattern.MatchString(scopeMarkerText(line)) || scopeBroadFilesMarker(scopeMarkerText(line)) {', 'else if false {', '^TestScopeFenceIntegrity$'),
 ("section-indent", 'return nil, Refused("refused: indented section heading in Context cannot end declared scope")', '// planted indented section boundary', '^TestScopeIndentedSectionRefuses$'),
 ("section-html", 'return nil, Refused("refused: HTML may hide a section boundary in declared files scope")', '// planted hidden HTML boundary', '^TestScopeHTMLSectionBoundaryRefuses$'),
]
selected=set(sys.argv[1:])
known={entry[0] for entry in entries}
if selected-known:raise SystemExit("unknown control: "+", ".join(sorted(selected-known)))
entries=[entry for entry in entries if not selected or entry[0] in selected]
baseline=subprocess.run(["go","test","./internal/deskkit","-run","^Test(Scope|Declared|HistoricalScope)","-count=1","-timeout=60s"],cwd=module,env=dict(os.environ,GOWORK="off",KUBECONFIG="/dev/null"),capture_output=True,text=True)
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
