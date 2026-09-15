#!/usr/bin/env bash
# Container delegation tests: no Docker, network, credentials, or host worktree.
set -euo pipefail
CELLCTL_UNDER_TEST="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/cellctl"
export CELLCTL_UNDER_TEST
python3 - <<'PY'
import json, os, pathlib, subprocess, tempfile, unittest

class Containers(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(prefix='cellctl-container-')
        self.addCleanup(self.tmp.cleanup)
        self.root=pathlib.Path(self.tmp.name)
        self.cells=self.root/'cells'
        self.env={**os.environ,'HOME':str(self.root/'home'),'CELLS_ROOT':str(self.cells),
            'CLAUDE_CONFIG_DIR':str(self.root/'missing-host-config'),
            'GH_TOKEN':'fixture-forge-secret','ANTHROPIC_API_KEY':'fixture-model-secret',
            'CLAUDE_CODE_OAUTH_TOKEN':'fixture-oauth-secret','SSH_AUTH_SOCK':'fixture-socket'}
        self.launcher=self.root/'launcher with spaces'
        self.launcher.write_text('''#!/usr/bin/env python3
import json,os,pathlib,sys
root=pathlib.Path(os.environ['CELL_DIR'])
(root/'calls.json').write_text(json.dumps({'args':sys.argv[1:],'env':dict(os.environ)}))
code=root/'exit-code'
sys.exit(int(code.read_text()) if code.exists() else 0)
''')
        self.launcher.chmod(0o755)
        self.cli=os.environ['CELLCTL_UNDER_TEST']
        self.call('new','sample','--kind','container','--repo','example-org/example-repo','--launcher',str(self.launcher))
        self.directory=self.cells/'sample'

    def call(self,*args,code=0,env=None):
        result=subprocess.run([self.cli,*args],env=env or self.env,capture_output=True,text=True)
        self.assertEqual(result.returncode,code,result.stdout+result.stderr)
        return result

    def recorded(self):
        return json.loads((self.directory/'calls.json').read_text())

    def test_registration_is_discoverable_and_has_no_host_custody(self):
        self.assertIn('sample',self.call('ls').stdout.splitlines())
        self.assertEqual([p.name for p in self.directory.iterdir()],['cell.env'])
        self.assertEqual((self.directory/'cell.env').stat().st_mode & 0o777,0o600)
        before=(self.directory/'cell.env').read_bytes()
        self.call('new','sample','--kind','container','--repo','example-org/example-repo','--launcher',str(self.launcher),code=3)
        self.assertEqual((self.directory/'cell.env').read_bytes(),before)

    def test_check_delegates_without_host_credentials_or_host_config(self):
        self.call('check','sample')
        record=self.recorded()
        self.assertEqual(record['args'],['check'])
        for key in ('GH_TOKEN','ANTHROPIC_API_KEY','CLAUDE_CODE_OAUTH_TOKEN','SSH_AUTH_SOCK','CLAUDE_CONFIG_DIR','ASSAY_CONFIG_HOME'):
            self.assertNotIn(key,record['env'])
        self.assertEqual(record['env']['CELL_REPO'],'example-org/example-repo')

    def test_desk_passes_model_as_one_argument_and_preserves_pin_rules(self):
        model='example model; $(touch SHOULD_NOT_EXIST)'
        self.call('desk','sample','the-desk','--model',model)
        self.assertEqual(self.recorded()['args'],['desk','the-desk','--harness','claude','--model',model])
        self.call('desk','sample','the-desk','--model','opus',code=3)
        self.assertFalse((self.root/'SHOULD_NOT_EXIST').exists())

    def test_up_is_one_coordinator_and_down_propagates_failure(self):
        self.call('up','sample')
        self.assertEqual(self.recorded()['args'],['desk','the-desk','--harness','claude','--model','fable'])
        self.call('down','sample')
        self.assertEqual(self.recorded()['args'],['down'])
        (self.directory/'exit-code').write_text('37')
        self.call('check','sample',code=37)
        self.call('up','sample',code=37)
        self.call('down','sample',code=37)

    def test_dry_run_does_not_call_launcher_or_persist_override(self):
        before=(self.directory/'cell.env').read_bytes()
        env={**self.env,'DRY_RUN':'1'}
        for args in [('check','sample'),('down','sample'),('desk','sample','the-desk','--model','sonnet','--set')]:
            self.assertIn('[dry-run]',self.call(*args,env=env).stdout)
        self.assertFalse((self.directory/'calls.json').exists())
        self.assertEqual((self.directory/'cell.env').read_bytes(),before)

    def test_explicit_harness_and_set_use_the_correct_namespace(self):
        self.call('desk','sample','the-desk','--harness','codex','--model','example-code-model','--set')
        self.assertEqual(self.recorded()['args'],['desk','the-desk','--harness','codex','--model','example-code-model'])
        self.assertIn('CODEX_MODEL_the_desk=example-code-model',(self.directory/'cell.env').read_text())

    def test_unsupported_host_options_and_disabled_roles_refuse(self):
        for args in [('desk','sample','the-desk','--provider','sample'),
                     ('desk','sample','the-desk','/host/config'),
                     ('desk','sample','worker-desk'),('up','sample','--no-attach'),
                     ('down','sample','--keep-deskd'),('deskd','sample')]:
            self.call(*args,code=3)
        self.assertFalse((self.directory/'calls.json').exists())

    def test_invalid_launcher_or_name_refuses_without_scaffolding(self):
        self.call('new','../escape','--kind','container','--repo','example-org/example-repo','--launcher',str(self.launcher),code=3)
        self.call('new','missing','--kind','container','--repo','example-org/example-repo','--launcher','relative/path',code=3)
        self.launcher.unlink()
        self.call('check','sample',code=3)
        self.assertFalse((self.cells/'missing').exists())

unittest.main()
PY
