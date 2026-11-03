#!/usr/bin/env python3
"""One compatibility execution in a private Linux PID namespace; no scheduler."""
import argparse
import errno
import fcntl
import hashlib
import json
import os
from pathlib import Path
import select
import signal
import re
import sys
import termios
import tty
import time

SCHEMA = 'cell-kube-compat-v1'
ROLES = {'the-desk', 'intake-desk', 'worker-desk', 'pr-review-desk', 'verify-desk'}

class Held(Exception):
    pass

def need(condition, message):
    if not condition:
        raise Held(message)

def strict(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            need(key not in result, 'duplicate state/profile member')
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=pairs)

def read(path):
    need(not path.is_symlink() and path.is_file(), 'required retained history/profile missing')
    raw = path.read_bytes()
    need(len(raw) <= 65536, 'retained history/profile exceeds bound')
    return strict(raw)

def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False).replace('\u2028', '\\u2028').replace('\u2029', '\\u2029').encode('utf-8')

def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as source:
        while True:
            block = source.read(65536)
            if not block:
                break
            h.update(block)
    return h.hexdigest()

def durable(path, value):
    # Persist an interrupted-write marker before mutating history. It remains on
    # every error, including rename succeeded / directory fsync failed. Clearing
    # it needs no further fallible sync: after a crash it may conservatively
    # reappear (held), but it cannot make uncommitted clean history admissible.
    marker = path.parent / 'uncommitted'
    guard = os.open(marker, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        os.write(guard, b'pending retained state transaction\n')
        os.fsync(guard)
    finally:
        os.close(guard)
    directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(directory)
        tmp = path.with_name('.state-next')
        fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        data = canonical(value)
        with os.fdopen(fd, 'wb') as target:
            target.write(data)
            target.flush()
            os.fsync(target.fileno())
        os.replace(tmp, path)
        os.fsync(directory)
        marker.unlink()  # last fallible operation; errors leave the guard present
    finally:
        try:
            os.close(directory)
        except OSError:
            pass  # the read-only directory descriptor adds no durability obligation


def profile(path):
    value = read(path)
    required = {'schema', 'cell', 'role', 'initialization_id', 'agent', 'agent_sha256',
                'agent_version', 'version_manifest', 'model', 'instructions',
                'instructions_sha256', 'workdir', 'initial_prompt'}
    need(set(value) == required and value['schema'] == SCHEMA and value['role'] in ROLES,
         'unknown launch profile')
    need(isinstance(value['cell'],str) and re.fullmatch(r'[a-z0-9-]+',value['cell']), 'invalid launch cell')
    need(value['initial_prompt'] == 'Read /opt/assay/plugin/skills/' + value['role'] + '/SKILL.md and run that role under its existing STOP, claim and human gates.', 'pinned initial role prompt differs')
    for key in ['initialization_id', 'agent_sha256', 'instructions_sha256']:
        need(isinstance(value[key], str) and len(value[key]) == 64 and
             all(c in '0123456789abcdef' for c in value[key]), 'invalid launch digest')
    for key in ['agent', 'version_manifest', 'instructions', 'workdir']:
        need(Path(value[key]).is_absolute(), 'launch paths must be explicit')
    need(value['agent_version'] and value['model'] and Path(value['workdir']).is_dir(),
         'pinned agent/model/work inputs unavailable')
    return value, hashlib.sha256(canonical(value)).hexdigest()

def state(value, launch, launch_id):
    need(set(value) == {'schema', 'cell', 'role', 'initialization_id', 'launch_id', 'pod_uid', 'state', 'agent_pid'}
         and value['schema'] == SCHEMA and value['cell']==launch['cell'] and value['role'] == launch['role']
         and value['initialization_id'] == launch['initialization_id']
         and value['launch_id'] == launch_id and value['state'] in {'prepared', 'intent', 'running', 'clean'},
         'retained history corrupt or foreign')
    need(isinstance(value['pod_uid'], str) and
         (value['pod_uid'] == '' or re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,127}', value['pod_uid'])),
         'invalid retained Pod UID')
    need(type(value['agent_pid']) is int and
         ((value['state'] == 'running' and value['agent_pid'] > 1) or
          (value['state'] != 'running' and value['agent_pid'] == 0)), 'invalid retained agent PID')
    need((value['state'] == 'prepared') == (value['pod_uid'] == ''), 'invalid retained lineage')

def processes():
    # Private PID namespace is mandatory. Scanning all other namespace processes
    # closes detached/reparented/setsid descendants; process-group kill alone does not.
    own = os.getpid()
    result = []
    for name in os.listdir('/proc'):
        if name.isdigit() and int(name) != own:
            try:
                text = Path('/proc', name, 'stat').read_text()
            except FileNotFoundError:
                continue
            status = text[text.rfind(')') + 2:].split()[0]
            if status != 'Z':
                result.append(int(name))
    return result

def reap():
    while True:
        try:
            pid, _ = os.waitpid(-1, os.WNOHANG)
            if pid == 0:
                return
        except ChildProcessError:
            return

def quiesce():
    # Repeatedly enumerate to close fork races. No clean record after the bound,
    # failed signal or surviving task. SIGKILL/grace expiry deliberately stays dirty.
    until = time.monotonic() + 8
    while time.monotonic() < until:
        live = processes()
        if not live:
            reap()
            need(not processes(), 'execution descendants survived stop')
            return
        for pid in live:
            try:
                os.kill(pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
        reap()
        time.sleep(0.05)
    raise Held('execution descendants did not acknowledge clean stop')

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--prepare', action='store_true')
    parser.add_argument('--ready', action='store_true')
    parser.add_argument('--profile', default='/work/compat-launch.json')
    parser.add_argument('--work', default='/work')
    args = parser.parse_args()
    work = Path(args.work)
    launch, launch_id = profile(Path(args.profile))
    history = work / '.assay-compat'
    if args.prepare:
        # Explicit operator-owned preparation; it never repairs/resets existing state.
        need(not history.exists() and not history.is_symlink(), 'history already exists; no reset is supported')
        history.mkdir(mode=0o700)
        durable(history / 'state.json', {'schema': SCHEMA, 'cell':launch['cell'], 'role': launch['role'],
                'initialization_id': launch['initialization_id'], 'launch_id': launch_id,
                'pod_uid': '', 'state': 'prepared', 'agent_pid': 0})
        print('checked-clean: cold compatibility history prepared; no agent invoked')
        return
    if args.ready:
        need(not (history/'uncommitted').exists() and not (history/'.state-next').exists(), 'interrupted retained write; held')
        current = read(history / 'state.json')
        state(current, launch, launch_id)
        need(current['state'] == 'running' and current['pod_uid'] == os.environ.get('ASSAY_POD_UID')
             and current['agent_pid'] > 1, 'agent is not running')
        text=Path('/proc',str(current['agent_pid']),'stat').read_text()
        need(text[text.rfind(')')+2:].split()[0] not in {'Z','X'}, 'agent is dead or zombie')
        os.kill(current['agent_pid'], 0)
        return
    need(os.getpid() == 1 and Path('/proc/1/stat').is_file(), 'launcher requires private Linux PID 1')
    need(os.environ.get('ASSAY_DESK') == launch['role'] and
         os.environ.get('ASSAY_COMPAT_LAUNCH_ID') == launch_id and
         os.environ.get('ASSAY_COMPAT_INIT_ID') == launch['initialization_id'],
         'image/profile launch identity differs')
    pod_uid = os.environ.get('ASSAY_POD_UID', '')
    need(pod_uid and len(pod_uid) <= 128 and not any(c.isspace() for c in pod_uid), 'explicit Pod UID required')
    need(history.is_dir() and not history.is_symlink(), 'cold preparation/history absent; held')
    lock_fd = os.open(history / 'lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    fcntl.flock(lock_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    state_path = history / 'state.json'
    prior = read(state_path)
    state(prior, launch, launch_id)
    need(prior['state'] == 'prepared' or prior['state'] == 'clean' and prior['pod_uid'] != pod_uid,
         'same-Pod restart or unconfirmed predecessor; held')
    need(not (history / '.state-next').exists() and not (history/'uncommitted').exists(), 'interrupted retained write; held')
    agent = Path(launch['agent'])
    need(agent.is_file() and os.access(agent, os.X_OK) and digest(agent) == launch['agent_sha256'],
         'pinned agent executable unavailable')
    version = read(Path(launch['version_manifest']))
    need(version.get('name') == '@anthropic-ai/claude-code' and version.get('version') == launch['agent_version'],
         'pinned Claude-compatible package version differs')
    instructions = Path(launch['instructions'])
    need(instructions.is_file() and digest(instructions) == launch['instructions_sha256'],
         'pinned role instructions differ')
    # A fresh private container PID namespace has no surviving old execution.
    # A kubelet readiness probe may overlap preflight; it is not old work.
    need(os.environ.get('ASSAY_CELL',launch['cell'])==launch['cell'], 'foreign inherited cell marker')
    # Existing role tools keep their STOP/claim/session owners. These markers do
    # not grant authority; they make the selected cell/role explicit to the child.
    os.environ['ASSAY_CELL']=launch['cell']
    os.environ['ASSAY_CELL_ROLE']=launch['role']
    os.environ['DESK_LOOP']=launch['role']
    os.environ['DESK_SESSION']=launch['cell']+'-'+launch['role']+'-'+pod_uid
    intent = dict(prior, pod_uid=pod_uid, state='intent')
    durable(state_path, intent)
    stop = False
    interrupted = False
    def request_stop(signum, frame):
        nonlocal interrupted
        # SIGTERM from Kubernetes may be an unconfirmed deletion, not a controlled
        # custodian stop. It quiesces best-effort but never records clean history.
        interrupted = True
    signal.signal(signal.SIGTERM, request_stop)
    signal.signal(signal.SIGINT, request_stop)
    signal.signal(signal.SIGHUP, lambda *_: None)  # terminal detach is presentation only
    # The child uses a separate PTY. Container stdin remains a stable supervisor
    # channel, so closing an attach never sends hangup or EOF to the agent.
    import pty
    if os.isatty(0):
        tty.setraw(0)  # one keystroke reaches the child without an outer line buffer
    exec_read, exec_write = os.pipe2(os.O_CLOEXEC)
    child_pid, master = pty.fork()  # setsid + controlling slave + foreground group
    if child_pid == 0:
        os.close(exec_read)
        try:
            os.chdir(launch['workdir'])
            os.execv(str(agent), [str(agent), '--model', launch['model'],
                                 '--append-system-prompt-file', str(instructions), launch['initial_prompt']])
        except OSError:
            os.write(exec_write, b'exec-failed')
            os._exit(78)
    os.close(exec_write)
    ready, _, _ = select.select([exec_read], [], [], 2)
    need(ready and os.read(exec_read, 64) == b'', 'pinned agent exec was not acknowledged')
    os.close(exec_read)
    text=Path('/proc',str(child_pid),'stat').read_text()
    need(text[text.rfind(')')+2:].split()[0] not in {'Z','X'}, 'agent exited before launch acknowledgement')
    durable(state_path, dict(intent, state='running', agent_pid=child_pid))
    def resize(*_):
        try:
            size = fcntl.ioctl(0, termios.TIOCGWINSZ, bytes(8))
            fcntl.ioctl(master, termios.TIOCSWINSZ, size)
            os.killpg(child_pid, signal.SIGWINCH)
        except OSError:
            pass
    signal.signal(signal.SIGWINCH, resize)
    resize()
    os.set_blocking(master, False)
    os.set_blocking(1, False)
    stdin_open = True
    to_child = bytearray()
    to_terminal = bytearray()
    pending = bytearray()
    prefix = b'\x1dASSAY_STOP '
    expected = prefix + pod_uid.encode() + b' ' + launch_id.encode() + b'\n'
    while not stop and not interrupted:
        writable = ([master] if to_child else []) + ([1] if to_terminal else [])
        ready, writes, _ = select.select(([0] if stdin_open and len(to_child) < 65536 else []) + [master], writable, [], 0.1)
        for fd, queue in [(master, to_child), (1, to_terminal)]:
            if fd in writes:
                try:
                    count = os.write(fd, queue[:65536])
                    del queue[:count]
                except (BlockingIOError, BrokenPipeError):
                    pass
        if master in ready:
            try:
                data = os.read(master, 65536)
            except OSError as err:
                if err.errno != errno.EIO:
                    raise
                data = b''
            if not data:
                break  # natural exit is dirty, including successful child exit
            to_terminal.extend(data)
            # Presentation cannot block supervision/stop. The container runtime
            # owns transcript retention; detached slow displays keep a bounded tail.
            if len(to_terminal) > 1048576:
                del to_terminal[:-1048576]
        if 0 in ready:
            data = os.read(0, 65536)
            if not data:
                stdin_open = False
                continue
            pending.extend(data)
            # Only a complete exact control frame is consumed by the supervisor.
            # An unexpected control frame is a held stop request, never child input.
            while pending:
                if pending[0] == 0x1d:
                    if b'\n' not in pending:
                        need(len(pending) <= 256, 'invalid control frame')
                        break
                    end = pending.index(10) + 1
                    frame = bytes(pending[:end]); del pending[:end]
                    need(frame == expected, 'stop frame belongs to another execution')
                    stop = True
                    break
                index = pending.find(0x1d)
                count = len(pending) if index < 0 else index
                to_child.extend(pending[:count]); del pending[:count]
        try:
            exited, _ = os.waitpid(child_pid, os.WNOHANG)
        except ChildProcessError:
            exited = child_pid
        if exited:
            break
    quiesce()
    if not stop or interrupted:
        raise Held('uncontrolled child/container exit; retained launch intent stays dirty')
    message = {'schema': SCHEMA, 'pod_uid': pod_uid, 'launch_id': launch_id,
               'clean': True, 'group_empty': True}
    # The native API must report this message on the exact terminated container ID.
    # Failure to write/fsync it cannot become successful stop acknowledgement.
    target = Path('/dev/termination-log')
    with target.open('wb') as stream:
        stream.write(canonical(message)); stream.flush(); os.fsync(stream.fileno())
    clean = dict(intent, state='clean', agent_pid=0)
    durable(state_path, clean)
    try:
        os.write(1, b'checked-clean: compatibility launch and execution-group stop\n')
    except OSError:
        pass

if __name__ == '__main__':
    try:
        main()
    except (Held, OSError, ValueError, KeyError, TypeError) as error:
        print('held: compatibility execution requires custodian reconciliation (' + str(error) + ')', file=sys.stderr)
        sys.exit(78)
