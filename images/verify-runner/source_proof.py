"""Exact local Git source proof shared by verification adapters and images.

This library reads local Git objects only. It grants no admission or acceptance.
"""
import hashlib
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import zlib


FIXED = '/usr/bin:/bin:/usr/sbin:/sbin'


class Refused(Exception):
    pass


def digest(data):
    return hashlib.sha256(data).hexdigest()


def git_env():
    return {'PATH': FIXED, 'HOME': '/var/empty', 'GIT_CONFIG_GLOBAL': '/dev/null',
            'GIT_CONFIG_NOSYSTEM': '1', 'GIT_ATTR_NOSYSTEM': '1',
            'GIT_NO_REPLACE_OBJECTS': '1', 'GIT_TERMINAL_PROMPT': '0'}


def git(root, *args):
    return subprocess.check_output(['/usr/bin/git', '-C', str(root), *args], env=git_env())


def tail(stderr):
    """Last line of git's diagnostics, so a refusal names its cause."""
    lines = stderr.decode(errors='replace').strip().splitlines()
    return ' (git: ' + lines[-1][:200] + ')' if lines else ''


def attest(objects, repo, source):
    """Copy the attested commit's tree into repo's own object store, every object re-hashed.

    Git does not re-check a blob's content against its id on read, so bytes read from the
    borrowed store are trusted only once sha1('<type> <size>\\0' + raw) equals the id the
    attested commit names. One cat-file --batch process serves every object.
    Returns the attested root tree id and {path: (mode, blob id)}.
    """
    env = dict(git_env(), GIT_DIR=repo, GIT_OBJECT_DIRECTORY=objects)
    expected = {}
    with tempfile.TemporaryFile() as err:
        try:
            proc = subprocess.Popen(['/usr/bin/git', 'cat-file', '--batch'], env=env,
                                    stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=err)
        except OSError:
            raise Refused('cannot read attested source')
        def fetch(oid, kind, label):
            try:
                proc.stdin.write(oid.encode() + b'\n')
                proc.stdin.flush()
                header = proc.stdout.readline().split()
                size = int(header[2]) if len(header) == 3 and header[0] == oid.encode() else -1
                raw = proc.stdout.read(size) if size >= 0 else b''
                if size < 0 or len(raw) != size or proc.stdout.read(1) != b'\n':
                    raise ValueError
            except (OSError, ValueError):
                proc.kill()
                proc.wait()
                err.seek(0)
                raise Refused('cannot read attested object: ' + label + tail(err.read()))
            if hashlib.sha1(kind + b' ' + str(size).encode() + b'\0' + raw).hexdigest() != oid:
                raise Refused('attested object differs from its id: ' + label)
            path = Path(repo) / 'objects' / oid[:2] / oid[2:]
            path.parent.mkdir(exist_ok=True)
            path.write_bytes(zlib.compress(kind + b' ' + str(size).encode() + b'\0' + raw, 1))
            return raw
        try:
            commit = fetch(source, b'commit', 'commit ' + source)
            match = re.match(rb'tree ([0-9a-f]{40})\n', commit)
            if not match:
                raise Refused('attested commit names no tree')
            tree = match[1].decode()
            pending, blobs = [(tree, '')], set()
            while pending:
                oid, prefix = pending.pop()
                data = fetch(oid, b'tree', prefix or 'root tree')
                while data:
                    head, rest = data.split(b'\0', 1)
                    mode, name = head.split(b' ', 1)
                    entry, data = rest[:20].hex(), rest[20:]
                    rel = prefix + os.fsdecode(name)
                    mode = int(mode, 8)
                    if stat.S_ISDIR(mode):
                        pending.append((entry, rel + '/'))
                    elif stat.S_ISREG(mode) or stat.S_ISLNK(mode):
                        # Git's own canonical modes, as ls-tree printed them.
                        canon = '120000' if stat.S_ISLNK(mode) else '100755' if mode & stat.S_IXUSR else '100644'
                        if entry not in blobs:
                            fetch(entry, b'blob', rel)
                            blobs.add(entry)
                        expected[rel] = (canon, entry)
                    else:
                        raise Refused('submodules and non-blob sources are unsupported')
        except ValueError:
            raise Refused('malformed attested tree')
        finally:
            try:
                proc.stdin.close()
            except OSError:
                pass
            proc.wait()
            proc.stdout.close()
    return tree, expected


def render(repo, tree, out):
    """Git's checkout of the attested tree under its own .gitattributes only, into out.

    repo is an empty template-free repository holding only re-hashed attested objects, so
    the home's config, filter drivers, info/attributes and working-tree .gitattributes never
    take part, and no object outside the attested tree can be read.
    """
    env = dict(git_env(), GIT_DIR=repo, GIT_WORK_TREE=out,
               GIT_INDEX_FILE=os.path.join(repo, 'render.index'))
    try:
        os.mkdir(out, 0o700)
        for argv in (['read-tree', tree],
                     ['--attr-source=' + tree + '^{tree}', 'checkout-index', '-a', '-f']):
            proc = subprocess.run(['/usr/bin/git', *argv], env=env, cwd=out,
                                  stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
            if proc.returncode:
                raise Refused('cannot render attested source' + tail(proc.stderr))
    except (OSError, subprocess.SubprocessError) as exc:
        raise Refused('cannot render attested source: ' + str(exc))


def prove_clone(root, source):
    """Exact bytes git renders from each re-hashed attested blob under the attested commit's
    own .gitattributes, plus modes; never git diff/index normalization claims."""
    if git(root, 'rev-parse', 'HEAD').decode().strip() != source:
        raise Refused('execution source differs from admitted commit')
    objects = git(root, 'rev-parse', '--path-format=absolute', '--git-path',
                  'objects').decode().rstrip('\n')
    with tempfile.TemporaryDirectory(prefix='attested-attr-') as scratch:
        repo = os.path.join(scratch, 'attested.git')
        subprocess.check_output(['/usr/bin/git', 'init', '-q', '--bare', '--template=', repo],
                                env=git_env())
        tree, expected = attest(objects, repo, source)
        rendered = os.path.join(scratch, 'rendered')
        render(repo, tree, rendered)
        compare(root, expected, Path(rendered))


def rendered_bytes(rendered, rel):
    path = rendered / rel
    try:
        if not stat.S_ISREG(path.lstat().st_mode):
            raise Refused('cannot render attested source: ' + rel)
        return path.read_bytes()
    except OSError:
        raise Refused('cannot render attested source: ' + rel)


def compare(root, expected, rendered):
    seen = set()
    def walk_error(error):
        raise Refused('cannot inspect execution directory: ' + str(error))
    for parent, dirs, files in os.walk(root, followlinks=False, onerror=walk_error):
        if Path(parent) == root:
            dirs[:] = [d for d in dirs if d != '.git']
            files = [f for f in files if f != '.git']
        for d in list(dirs):
            if (Path(parent) / d).is_symlink():
                files.append(d)
                dirs.remove(d)
        for name in files:
            path = Path(parent) / name
            rel = path.relative_to(root).as_posix()
            if rel not in expected:
                raise Refused('unattested execution file: ' + rel)
            mode, oid = expected[rel]
            info = path.lstat()
            if stat.S_ISLNK(info.st_mode):
                actual_mode, data = '120000', os.fsencode(os.readlink(path))
            elif stat.S_ISREG(info.st_mode):
                actual_mode = '100755' if info.st_mode & stat.S_IXUSR else '100644'
                data = path.read_bytes()
            else:
                raise Refused('unsupported execution file: ' + rel)
            # Git never converts a symlink target; a regular file is git's own rendering.
            if mode == '120000':
                want = hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest() == oid
            else:
                want = data == rendered_bytes(rendered, rel)
            if mode != actual_mode or not want:
                raise Refused('execution bytes/mode differ from source: ' + rel)
            seen.add(rel)
    if seen != set(expected):
        raise Refused('execution source has missing files')


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check-clone', nargs=2, metavar=('ROOT', 'SHA'), required=True)
    args = parser.parse_args()
    root, sha = args.check_clone
    if not re.fullmatch(r'[0-9a-f]{40}', sha):
        raise Refused('source must be a full lowercase commit identifier')
    prove_clone(Path(root).resolve(strict=True), sha)


if __name__ == '__main__':
    import sys
    try:
        main()
    except (Refused, OSError, ValueError, subprocess.SubprocessError) as exc:
        print('source-proof: refused: ' + str(exc), file=sys.stderr)
        sys.exit(1)
