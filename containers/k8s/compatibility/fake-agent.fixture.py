#!/usr/local/bin/python3
import os,signal,sys,termios,tty,json,time
from pathlib import Path
work=Path('/work')
def event(kind,**kw):
 with (work/'events.jsonl').open('a') as f: f.write(json.dumps(dict(kind=kind,pid=os.getpid(),ppid=os.getppid(),sid=os.getsid(0),**kw))+'\n')
def end(sig,frame):
 event('term',signal=sig)
 raise SystemExit(0)
signal.signal(signal.SIGTERM,end)
try:
 fd=os.open('/dev/tty',os.O_RDWR); os.close(fd); controlling=True; error=''
except OSError as e: controlling=False; error=str(e)
event('agent',cell=os.environ.get('ASSAY_CELL'),role=os.environ.get('ASSAY_CELL_ROLE'),loop=os.environ.get('DESK_LOOP'),session=os.environ.get('DESK_SESSION'),argv=sys.argv[1:],isatty=os.isatty(0),controlling_tty=controlling,error=error)
mode=(work/'mode').read_text().strip()
if mode in ('detached','stubborn'):
 pid=os.fork()
 if pid==0:
  os.setsid()
  pid=os.fork()
  if pid: os._exit(0)
  if mode=='stubborn': signal.signal(signal.SIGTERM,signal.SIG_IGN)
  time.sleep(.15); event('detached')
  while True: time.sleep(.1)
 os.waitpid(pid,0)
if os.isatty(0): tty.setraw(0)
print('FAKE_READY',flush=True)
while True:
 b=os.read(0,1)
 if not b: time.sleep(.1); continue
 event('byte',hex=b.hex())
 print('FAKE_BYTE '+b.hex(),flush=True)
