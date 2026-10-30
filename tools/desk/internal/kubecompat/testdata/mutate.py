#!/usr/bin/env python3
"""Reproducible fail-first mutations of pure compatibility decisions, offline."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

source=Path(__file__).resolve().parent.parent
cases=[('tuple', 'profile.go', 'if i != other {', 'if false {', 'TestKubeCompatIdentity'),
       ('launch-binding','profile.go','r.LaunchID != r.LaunchDigest(name)','false','TestKubeCompatIdentity'),
       ('unknown-stop','lifecycle.go','term.GroupEmpty','true','TestKubeCompatStopHolds'),
       ('pending','lifecycle.go','s.PodPresent && !s.Pending','s.PodPresent','TestKubeCompatPending')]
for name,filename,before,after,test in cases:
 with tempfile.TemporaryDirectory(prefix='compat-mutation-') as scratch:
  root=Path(scratch)
  for file in ['profile.go','lifecycle.go','profile_test.go']:
   shutil.copyfile(source/file,root/file)
  (root/'go.mod').write_text('module compatibility-mutation\n\ngo 1.27.0\n')
  env={**os.environ,'GOWORK':'off','GOPROXY':'off','GOSUMDB':'off','GOTOOLCHAIN':'local','KUBECONFIG':'/dev/null'}
  args=['go','test','-count=1','-timeout=15s','-run','^'+test+'$','-v','.']
  clean=subprocess.run(args,cwd=root,env=env,capture_output=True,text=True,timeout=25)
  if clean.returncode or '--- PASS: '+test not in clean.stdout:
   raise SystemExit('could-not-check: mutation baseline '+name+' did not pass')
  path=root/filename;text=path.read_text()
  assert before in text,name
  path.write_text(text.replace(before,after,1))
  red=subprocess.run(args,cwd=root,env=env,capture_output=True,text=True,timeout=25)
  if red.returncode==0 or '--- FAIL: '+test not in red.stdout:
   raise SystemExit('checked-failed: mutation '+name+' escaped or failed for another reason')
  print('rejected: '+name+'; --- FAIL: '+test)
print('checked-clean: compatibility decision mutations discriminate')
