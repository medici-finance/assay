#!/usr/bin/env python3
"""Plan or explicitly prepare one cold Claude desk volume; never start an agent."""
import argparse
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys

LAUNCHER = Path('/usr/local/lib/assay/desk-session.py')


class Error(Exception):
    pass


def need(condition, message):
    if not condition:
        raise Error(message)


def load_launcher(path):
    need(path.is_absolute() and path.is_file() and not path.is_symlink(),
         'image-owned compatibility launcher unavailable')
    spec = importlib.util.spec_from_file_location('desk_session', path)
    module = importlib.util.module_from_spec(spec)
    previous = sys.dont_write_bytecode
    try:
        sys.dont_write_bytecode = True
        spec.loader.exec_module(module)
    finally:
        sys.dont_write_bytecode = previous
    return module


def handoff(launcher, work, expected):
    history = work / '.assay-compat'
    need(history.is_dir() and not history.is_symlink(), 'prepared history missing or linked; held')
    need(not (history / 'uncommitted').exists() and not (history / '.state-next').exists(),
         'interrupted history write; held')
    actual, launch_id = launcher.profile(work / 'compat-launch.json')
    inputs = dict(actual)
    inputs.pop('initialization_id')
    need(inputs == expected, 'prepared inputs differ from the selected installed profile')
    state = launcher.read(history / 'state.json')
    launcher.state(state, actual, launch_id)
    need(state['state'] == 'prepared', 'cold preparation is no longer unused')
    return {'descriptor': actual, 'launch_id': launch_id, 'workload_env': {
        'ASSAY_COMPAT_LAUNCH_ID': launch_id,
        'ASSAY_COMPAT_INIT_ID': state['initialization_id']}}


def run(args, *, launcher_path=LAUNCHER):
    """Reuse the image's descriptor/hash/history owner, including its prepare verb."""
    need(os.geteuid() == 999 and os.getegid() == 999,
         'prepare as workload UID/GID 999')
    work = Path(args.work)
    need(work.is_absolute() and work.is_dir() and not work.is_symlink(),
         'explicit retained work directory required')
    launcher = load_launcher(launcher_path)
    need(re.fullmatch(r'[a-z0-9][a-z0-9-]{0,62}', args.cell) and args.role in launcher.ROLES,
         'invalid cell or role')
    need(re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?', args.agent_version),
         'exact installed Claude package version required')
    need(re.fullmatch(r'claude-[a-z][a-z0-9]*-[0-9]+(?:-[0-9]+)*(?:\[1m\])?', args.model),
         'explicit versioned Claude model ID required')
    descriptor_path = work / 'compat-launch.json'
    history = work / '.assay-compat'

    def cold():
        need(not os.path.lexists(history), 'history exists; no reset or reinitialization is supported')
        need(not os.path.lexists(descriptor_path), 'launch descriptor exists; do not overwrite it')

    agent = Path(args.agent or work / '.npm-global/bin/claude')
    manifest = Path(args.version_manifest or work / '.npm-global/lib/node_modules/@anthropic-ai/claude-code/package.json')
    instructions = Path(args.instructions or '/opt/assay/plugin/skills/' + args.role + '/SKILL.md')
    workdir = Path(args.workdir or work / 'repository')
    need(all(p.is_absolute() for p in [agent, manifest, instructions, workdir]),
         'agent, package, instructions and work paths must be absolute')
    need(agent.is_file() and os.access(agent, os.X_OK) and instructions.is_file() and workdir.is_dir(),
         'preinstalled agent, role instructions or work inputs missing')
    package = launcher.read(manifest)
    need(package.get('name') == '@anthropic-ai/claude-code'
         and package.get('version') == args.agent_version, 'installed Claude package version differs')
    descriptor = {
        'schema': launcher.SCHEMA, 'cell': args.cell, 'role': args.role,
        'agent': str(agent), 'agent_sha256': launcher.digest(agent),
        'agent_version': args.agent_version, 'version_manifest': str(manifest),
        'model': args.model, 'instructions': str(instructions),
        'instructions_sha256': launcher.digest(instructions), 'workdir': str(workdir),
        'initial_prompt': 'Read /opt/assay/plugin/skills/' + args.role +
            '/SKILL.md and run that role under its existing STOP, claim and human gates.'}
    if args.readback:
        return handoff(launcher, work, descriptor)
    cold()
    if not args.prepare:
        return {'mode': 'plan', 'descriptor_inputs': descriptor,
                'next': 'Explicit --prepare creates cold history; no files were written.'}

    # Serialize helper callers on this retained volume. This is bookkeeping, not
    # a permission to initialize an existing/unknown execution. The launcher still
    # owns its independent history-exists and durable-write checks.
    fd = os.open(work / '.assay-compat-prepare.lock', os.O_WRONLY | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise Error('another preparation owns this volume') from error
        cold()
        descriptor['initialization_id'] = os.urandom(32).hex()
        out = os.open(descriptor_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(out, 'wb') as target:
            target.write(launcher.canonical(descriptor) + b'\n')
            target.flush()
            os.fsync(target.fileno())
        directory = os.open(work, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
        # Only the installed image-owned launcher may create retained history.
        # No package installer, agent executable, API client or model is invoked.
        try:
            result = subprocess.run([sys.executable, str(launcher_path), '--prepare',
                '--profile', str(descriptor_path), '--work', str(work)],
                stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30, check=False)
        except (OSError, subprocess.SubprocessError) as error:
            raise Error('cold preparation did not complete; retain descriptor/history for the custodian') from error
        need(result.returncode == 0,
             'image-owned cold preparation failed; retain descriptor/history for the custodian')
        return handoff(launcher, work, {k: v for k, v in descriptor.items() if k != 'initialization_id'})
    finally:
        os.close(fd)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument('--prepare', action='store_true', help='explicitly create cold descriptor/history; default is read-only plan')
    mode.add_argument('--readback', action='store_true', help='read validated unused preparation bindings without changing them')
    parser.add_argument('--work', default='/work')
    parser.add_argument('--cell', required=True)
    parser.add_argument('--role', required=True)
    parser.add_argument('--agent-version', required=True)
    parser.add_argument('--model', required=True)
    parser.add_argument('--agent')
    parser.add_argument('--version-manifest')
    parser.add_argument('--instructions')
    parser.add_argument('--workdir')
    args = parser.parse_args()
    try:
        result = run(args)
        print(json.dumps(result, indent=2))
    except Error as error:
        print('held: ' + str(error), file=sys.stderr)
        return 78
    except Exception:
        # Paths and inherited runtime diagnostics may be sensitive. Never dump a
        # traceback, environment, credential, input file or child stderr here.
        print('held: compatibility preparation did not complete; retain inputs and history for custodian inspection', file=sys.stderr)
        return 78
    return 0


if __name__ == '__main__':
    sys.exit(main())
