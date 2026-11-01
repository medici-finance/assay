import os,subprocess,json,hashlib,time,pty,select,errno,tempfile,shutil,sys,uuid,importlib.util
from pathlib import Path
source=Path(__file__).resolve().parent
scratch=tempfile.TemporaryDirectory(prefix='compat-pid1-fixtures-')
root=Path(scratch.name)
shutil.copyfile(source/'desk-session.py',root/'desk-session.py')
shutil.copyfile(source/'fake-agent.fixture.py',root/'fake-agent.py')
os.chmod(root/'fake-agent.py',0o755)
mutation=os.environ.get('ASSAY_COMPAT_TEST_MUTATION','')
if mutation:
 path=root/'desk-session.py';text=path.read_text()
 recipes={'same-pod':("prior['state'] == 'prepared' or prior['state'] == 'clean' and prior['pod_uid'] != pod_uid",'True'),
          'corrupt-history':('    state(prior, launch, launch_id)','    pass  # planted bypass of lower history validation')}
 assert mutation in recipes,'unknown fixture mutation'
 before,after=recipes[mutation];assert before in text;path.write_text(text.replace(before,after,1))
host=os.environ.get('ASSAY_TEST_DOCKER_HOST','')
if not host.startswith('unix:///'):
 print('could-not-check: set ASSAY_TEST_DOCKER_HOST to an explicit local Unix Docker socket',file=sys.stderr)
 sys.exit(2)
D=['docker','--host',host]
run_id=uuid.uuid4().hex[:12]
image='python:3.13-slim'; names=[]; results=[]
# Inspect only the explicit local daemon; --pull=never and --network=none below
# prevent registry/model contact. This is a test environment, not candidate proof.
inspection=subprocess.run(D+['image','inspect',image,'--format','{{.Id}}'],capture_output=True,text=True,timeout=10)
if inspection.returncode:
 print('could-not-check: cached Python fixture image unavailable',file=sys.stderr);sys.exit(2)
print('fixture image='+inspection.stdout.strip())
(root/'version.json').write_text('{"name":"@anthropic-ai/claude-code","version":"qa-synthetic"}')
(root/'role.md').write_text('Offline synthetic QA agent; no credentials or model.\n')
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def events(w):
 try:return [json.loads(l) for l in (w/'events.jsonl').read_text().splitlines()]
 except FileNotFoundError:return []
def wait_for(fn,timeout=5):
 until=time.monotonic()+timeout
 while time.monotonic()<until:
  if fn():return True
  time.sleep(.05)
 return False
def case(label,mode='normal'):
 w=root/label;w.mkdir()
 (w/'mode').write_text(mode)
 p={'cell':'example','initial_prompt':'Read /opt/assay/plugin/skills/worker-desk/SKILL.md and run that role under its existing STOP, claim and human gates.','schema':'cell-kube-compat-v1','role':'worker-desk','initialization_id':'a'*64,'agent':'/qa/fake-agent.py','agent_sha256':sha(root/'fake-agent.py'),'agent_version':'qa-synthetic','version_manifest':'/qa/version.json','model':'synthetic-no-model','instructions':'/qa/role.md','instructions_sha256':sha(root/'role.md'),'workdir':'/work'}
 (w/'profile.json').write_text(json.dumps(p))
 lid=hashlib.sha256(json.dumps(p,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()).hexdigest()
 base=D+['run','--pull=never','--network=none','--mount',f'type=bind,src={root},dst=/qa,readonly','--mount',f'type=bind,src={w},dst=/work']
 prep=subprocess.run(base+['--rm',image,'python','/qa/desk-session.py','--prepare','--profile','/work/profile.json','--work','/work'],capture_output=True,text=True,timeout=10)
 assert prep.returncode==0,prep.stderr
 return w,lid,base

def launch(label,w,lid,base,pod='pod-a',terminal=False):
 name='compat-pid1-fixture-'+run_id+'-'+label; names.append(name)
 cmd=base+['--name',name,'-i']+(['-t'] if terminal else [])+['-e','ASSAY_DESK=worker-desk','-e','ASSAY_COMPAT_INIT_ID='+'a'*64,'-e','ASSAY_COMPAT_LAUNCH_ID='+lid,'-e','ASSAY_POD_UID='+pod,image,'python','/qa/desk-session.py','--profile','/work/profile.json','--work','/work']
 if terminal:
  m,s=pty.openpty(); p=subprocess.Popen(cmd,stdin=s,stdout=s,stderr=s);os.close(s)
  return name,p,m
 return name,subprocess.Popen(cmd,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE),None

def frame(lid,pod='pod-a'):return b'\x1dASSAY_STOP '+pod.encode()+b' '+lid.encode()+b'\n'
def finish(name,p,m=None):
 if m is None:
  out,err=p.communicate(timeout=15)
 else:
  p.wait(timeout=15);out=bytearray();err=b''
  while select.select([m],[],[],.1)[0]:
   try:b=os.read(m,65536)
   except OSError as e:
    if e.errno==errno.EIO:break
    raise
   if not b:break
   out.extend(b)
  os.close(m)
  out=bytes(out)
 (root/(name+'.stdout')).write_bytes(out);(root/(name+'.stderr')).write_bytes(err)
 return p.returncode,out.decode(errors='replace'),err.decode(errors='replace')
try:
 w,lid,base=case('positive');name,p,m=launch('positive',w,lid,base)
 assert wait_for(lambda:any(e['kind']=='agent' for e in events(w)))
 ready=subprocess.run(D+['exec','-e','ASSAY_POD_UID=pod-a',name,'python','/qa/desk-session.py','--ready','--profile','/work/profile.json','--work','/work'],capture_output=True,text=True,timeout=5)
 assert ready.returncode==0,(ready.stdout,ready.stderr)
 expected=['--model','synthetic-no-model','--append-system-prompt-file','/qa/role.md','Read /opt/assay/plugin/skills/worker-desk/SKILL.md and run that role under its existing STOP, claim and human gates.']
 assert next(e['argv'] for e in events(w) if e['kind']=='agent')==expected
 stamp=next(e for e in events(w) if e['kind']=='agent');assert stamp['cell']=='example' and stamp['role']=='worker-desk' and stamp['loop']=='worker-desk' and stamp['session']=='example-worker-desk-pod-a',stamp
 p.stdin.write(b'q');p.stdin.flush(); immediate=wait_for(lambda:any(e.get('hex')=='71' for e in events(w)),1)
 p.stdin.write(frame(lid));p.stdin.flush(); rc,out,err=finish(name,p)
 assert rc==0 and json.loads((w/'.assay-compat/state.json').read_text())['state']=='clean',(rc,out,err)
 results.append(dict(test='positive',exit=rc,byte_without_newline=immediate,state=json.loads((w/'.assay-compat/state.json').read_text()),events=events(w),stdout=out,stderr=err))
 before=len(events(w));name,p,m=launch('samepod',w,lid,base)
 rc,out,err=finish(name,p);assert rc==78 and len(events(w))==before,(rc,out,err);results.append(dict(test='samePod-history-hold',exit=rc,agent_event_count_unchanged=len(events(w))==before,stderr=err))
 name,p,m=launch('newpod',w,lid,base,pod='pod-b');assert wait_for(lambda:len(events(w))>before)
 p.stdin.write(frame(lid,'pod-b'));p.stdin.flush();rc,out,err=finish(name,p);assert rc==0 and json.loads((w/'.assay-compat/state.json').read_text())['state']=='clean' and len([e for e in events(w) if e['kind']=='agent'])==2,(rc,out,err);results.append(dict(test='clean-newPod-restart',exit=rc,state=json.loads((w/'.assay-compat/state.json').read_text()),stderr=err))
 w,lid,base=case('dirty');s=w/'.assay-compat/state.json';v=json.loads(s.read_text());v.update(state='intent',pod_uid='prior-pod');s.write_text(json.dumps(v))
 name,p,m=launch('dirty',w,lid,base);rc,out,err=finish(name,p);assert rc==78 and not events(w),(rc,out,err);results.append(dict(test='dirty-history-hold',exit=rc,events=events(w),state=json.loads(s.read_text()),stderr=err))
 w,lid,base=case('detached','detached');name,p,m=launch('detached',w,lid,base)
 assert wait_for(lambda:any(e['kind']=='detached' for e in events(w)))
 p.stdin.write(frame(lid));p.stdin.flush();rc,out,err=finish(name,p);assert rc==0 and json.loads((w/'.assay-compat/state.json').read_text())['state']=='clean' and any(e['kind']=='term' and e['pid']==next(x['pid'] for x in events(w) if x['kind']=='detached') for e in events(w)),(rc,out,err,events(w));results.append(dict(test='setsid-reparented-cleanup',exit=rc,state=json.loads((w/'.assay-compat/state.json').read_text()),events=events(w),stdout=out,stderr=err))
 w,lid,base=case('tty');name,p,m=launch('tty',w,lid,base,terminal=True)
 assert wait_for(lambda:any(e['kind']=='agent' for e in events(w)))
 os.write(m,b'q');immediate=wait_for(lambda:any(e.get('hex')=='71' for e in events(w)),1)
 os.write(m,b'\n');after_newline=wait_for(lambda:any(e.get('hex')=='71' for e in events(w)),1)
 os.write(m,frame(lid));rc,out,err=finish(name,p,m)
 assert rc==0 and json.loads((w/'.assay-compat/state.json').read_text())['state']=='clean',(rc,out,err)
 assert immediate and any(e.get('controlling_tty') for e in events(w)),events(w)
 results.append(dict(test='real-terminal',exit=rc,byte_without_newline=immediate,byte_after_newline=after_newline,state=json.loads((w/'.assay-compat/state.json').read_text()),events=events(w),stdout=out,stderr=err))

 for label,changes in [('wrong-types',{'state':'clean','pod_uid':[],'agent_pid':'garbage'}),('invalid-pid',{'state':'clean','pod_uid':'prior','agent_pid':99})]:
  w,lid,base=case(label);path=w/'.assay-compat/state.json';v=json.loads(path.read_text());v.update(changes);path.write_text(json.dumps(v))
  name,p,m=launch(label,w,lid,base);rc,out,err=finish(name,p)
  assert rc==78 and not events(w),(label,rc,out,err)
 for label in ['erased-history','corrupt-history']:
  w,lid,base=case(label);path=w/'.assay-compat/state.json'
  if label=='erased-history':path.unlink()
  else:path.write_text('{broken')
  name,p,m=launch(label,w,lid,base);rc,out,err=finish(name,p)
  assert rc==78 and not events(w),(label,rc,out,err)
 w,lid,base=case('stubborn','stubborn');name,p,m=launch('stubborn',w,lid,base)
 assert wait_for(lambda:any(e['kind']=='detached' for e in events(w)))
 p.stdin.write(frame(lid));p.stdin.flush();rc,out,err=finish(name,p)
 assert rc==78 and json.loads((w/'.assay-compat/state.json').read_text())['state']!='clean',(rc,out,err)
 w,lid,base=case('uncontrolled-term');name,p,m=launch('uncontrolled-term',w,lid,base)
 assert wait_for(lambda:any(e['kind']=='agent' for e in events(w)))
 subprocess.run(D+['kill','--signal','TERM',name],check=True,capture_output=True)
 rc,out,err=finish(name,p)
 assert rc==78 and json.loads((w/'.assay-compat/state.json').read_text())['state']!='clean',(rc,out,err)
 for label,content in [('failed-exec','#!/not-a-real-interpreter\n'),('instant-exit','#!/bin/sh\nexit 0\n')]:
  agent=root/(label+'-agent');agent.write_text(content);agent.chmod(0o755)
  w,lid,base=case(label);profile_path=w/'profile.json';v=json.loads(profile_path.read_text());v['agent']='/qa/'+agent.name;v['agent_sha256']=sha(agent);profile_path.write_text(json.dumps(v));lid=hashlib.sha256(json.dumps(v,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()).hexdigest()
  state_path=w/'.assay-compat/state.json';v=json.loads(state_path.read_text());v['launch_id']=lid;state_path.write_text(json.dumps(v))
  name,p,m=launch(label,w,lid,base);rc,out,err=finish(name,p)
  assert rc==78 and json.loads(state_path.read_text())['state']!='clean' and not events(w),(rc,out,err)
 (root/'zombie-probe.py').write_text("""import os,json,time,subprocess,sys
from pathlib import Path
pid=os.fork()
if pid==0:os._exit(0)
for _ in range(100):
 text=Path('/proc',str(pid),'stat').read_text();status=text[text.rfind(')')+2:].split()[0]
 if status=='Z':break
 time.sleep(.01)
assert status=='Z'
path=Path('/work/.assay-compat/state.json');value=json.loads(path.read_text());value.update(state='running',pod_uid='pod-a',agent_pid=pid);path.write_text(json.dumps(value))
p=subprocess.run([sys.executable,'/qa/desk-session.py','--ready','--profile','/work/profile.json','--work','/work'],capture_output=True,text=True)
os.waitpid(pid,0)
assert p.returncode==78 and 'dead or zombie' in p.stderr,(p.returncode,p.stdout,p.stderr)
""")
 w,lid,base=case('zombie-readiness')
 probe=subprocess.run(base+['--rm','-e','ASSAY_POD_UID=pod-a',image,'python','/qa/zombie-probe.py'],capture_output=True,text=True,timeout=10)
 assert probe.returncode==0,(probe.stdout,probe.stderr)
 (root/'fsync-probe.py').write_text("""import os,json,importlib.util
from pathlib import Path
s=importlib.util.spec_from_file_location('launcher','/qa/desk-session.py');m=importlib.util.module_from_spec(s);s.loader.exec_module(m)
path=Path('/work/.assay-compat/state.json');value=json.loads(path.read_text());value.update(state='clean',pod_uid='prior-pod',agent_pid=0)
real=m.os.fsync;count=0
def fail(fd):
 global count
 count+=1
 if count==4:raise OSError('synthetic directory fsync after rename failure')
 return real(fd)
m.os.fsync=fail
try:m.durable(path,value)
except OSError:pass
else:raise AssertionError('fault did not fire')
assert (path.parent/'uncommitted').exists() and json.loads(path.read_text())['state']=='clean'
""")
 w,lid,base=case('fsync-after-rename')
 probe=subprocess.run(base+['--rm',image,'python','/qa/fsync-probe.py'],capture_output=True,text=True,timeout=10)
 assert probe.returncode==0,(probe.stdout,probe.stderr)
 name,p,m=launch('fsync-after-rename',w,lid,base);rc,out,err=finish(name,p)
 assert rc==78 and not events(w) and 'interrupted retained write' in err,(rc,out,err)
 print('checked-clean: compatibility launch and execution-group stop')
finally:
 for name in names:
  subprocess.run(D+['rm','-f',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
 scratch.cleanup()
