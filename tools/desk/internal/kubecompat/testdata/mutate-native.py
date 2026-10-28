#!/usr/bin/env python3
"""Fail-first native/CLI mutations against one immutable tracked offline snapshot."""
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile

repo=Path(__file__).resolve().parents[5]
cases=[
 ('native-attach','internal/kubecompat/native/client.go','SubResource("attach")','SubResource("exec")','./cmd/cellctl','TestKubeCompatCellctlLifecycle'),
 ('upgrade-tuple','internal/kubecompat/profile.go','if i != other {','if false {','./internal/kubecompat/native','TestKubeCompatUpgradeRace'),
 ('attach-redirect','internal/kubecompat/native/client.go','NewSPDYExecutorRejectRedirects','NewSPDYExecutorForTransports','./internal/kubecompat/native','TestKubeCompatAttachRedirect'),
 ('stop-proof','internal/kubecompat/lifecycle.go','term.GroupEmpty','true','./cmd/cellctl','TestKubeCompatStopUncertainty'),
 ('existing-stop','cmd/cellctl/kube_compat.go','if e := c.cadenceGuard(role); e != nil {','if e := c.cadenceGuard(role); false {','./cmd/cellctl','TestKubeCompatParity'),
 ('unmeasured-result','cmd/cellctl/kube_compat.go','rt, e := c.kubeRuntimeE(false)','rt := c.kubeRuntime(false); var e error','./cmd/cellctl','TestKubeCompatPlanInputGone')]
with tempfile.TemporaryDirectory(prefix='compat-native-mutations-') as scratch:
 root=Path(scratch);archive=root/'tracked.tar'
 with archive.open('wb') as target:
  subprocess.run(['git','-C',str(repo),'archive','HEAD'],stdout=target,check=True,timeout=20)
 if archive.stat().st_size>256*1024*1024:raise SystemExit('could-not-check: tracked snapshot exceeds 256MiB input budget')
 tree=root/'source';tree.mkdir()
 with tarfile.open(archive) as source:source.extractall(tree,filter='data')
 module=tree/'tools/desk'
 env={**os.environ,'GOWORK':'off','GOPROXY':'off','GOSUMDB':'off','GOTOOLCHAIN':'local','KUBECONFIG':'/dev/null'}
 for name,filename,before,after,package,test in cases:
  path=module/filename;original=path.read_text();assert before in original,name
  path.write_text(original.replace(before,after,1))
  try:
   red=subprocess.run(['go','test','-count=1','-timeout=30s','-v',package,'-run','^'+test+'$'],cwd=module,env=env,capture_output=True,text=True,timeout=40)
   if red.returncode==0 or '--- FAIL: '+test not in red.stdout:
    print(red.stdout[-2500:]);print(red.stderr[-2500:]);raise SystemExit('checked-failed: '+name+' escaped or did not fail its named behavior fixture')
   print('rejected: '+name+'; --- FAIL: '+test)
  finally:path.write_text(original)
print('checked-clean: native/CLI compatibility mutations discriminate')
