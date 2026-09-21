#!/usr/bin/env python3
"""Offline contract tests: no model requests, credentials or live cell writes."""
import copy
import json
import os
from pathlib import Path
import shlex
import subprocess
import tempfile
import unittest

CELLCTL = Path(__file__).resolve().parents[1] / 'cellctl'
EXAMPLE = CELLCTL.parent / 'examples/model-policy.json'


class ModelPolicyTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='cellctl-policy-')
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.cell = self.root / 'cells/example'
        self.cfg = self.root / 'claude-config'
        self.bin = self.root / 'bin'
        for path in (self.cell / 'home/.config/assay', self.cfg, self.bin):
            path.mkdir(parents=True)
        self.policy = json.loads(EXAMPLE.read_text())
        self.path = self.cell / 'policy.json'
        self.save()
        self.repo = self.root / 'repo'
        subprocess.run(['git','init','-q','-b','main',str(self.repo)],check=True)
        (self.repo / 'seed').write_text('fixture\n')
        self.git('add','seed')
        self.git('-c','user.name=Fixture','-c','user.email=fixture@example.invalid','commit','-qm','seed')
        self.git('update-ref','refs/remotes/origin/main','HEAD')
        (self.repo/'.git/FETCH_HEAD').write_text(self.git('rev-parse','HEAD').stdout)
        self.envfile = self.cell / 'cell.env'
        self.envfile.write_text('\n'.join([
            'CELL=example','CELL_KIND=house','DESKD=0','CELL_COCKPIT=tmux',
            'CELL_REPO='+shlex.quote(str(self.repo)),
            'CELL_ROOTS='+shlex.quote('example-org/example-repo='+str(self.repo)),
            'CELL_MODEL_POLICY=policy.json',
            'DESK_MODEL_pr_review_desk=sonnet', # Policy replaces legacy pins.
            'CELL_PROVIDER=kimi', # Policy provider replaces legacy default.
            ''
        ]))
        (self.cell / 'home/.config/assay/roster.env').write_text('')
        # No HOME/CODEX_HOME changes: all fixture output paths are explicit.
        self.env = {k:v for k,v in os.environ.items() if not k.startswith(('CELL','DESK','CODEX_MODEL','TIER_MODEL','ANTHROPIC','CLAUDE','MODEL_'))}
        self.env.update(CELLS_ROOT=str(self.root/'cells'),CLAUDE_CONFIG_DIR=str(self.cfg),
                        PATH=str(self.bin)+os.pathsep+os.environ['PATH'],
                        DESK_TOOLS_BIN=str(self.bin/'tools'),ASSAY_CONFIG_HOME=str(self.root/'operator'),
                        ZAI_API_KEY='fixture-zai',KIMI_API_KEY='fixture-kimi',DRY_RUN='1',
                        POLICY_TEST_RECORD=str(self.root/'launch.json'))
        (self.bin/'tools').mkdir()
        stub='''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
if sys.argv[1:3] == ['plugin','list']:
    print('[{"id":"assay@assay","enabled":true}]')
elif sys.argv[1:2] == ['plugin']:
    pass
elif sys.argv[1:] == ['--version']:
    print('2.1.278')
else:
    Path(os.environ['POLICY_TEST_RECORD']).write_text(json.dumps({'argv':sys.argv,'env':{k:v for k,v in os.environ.items() if k.startswith(('ANTHROPIC','CLAUDE_CODE'))}}))
'''
        for name in ('claude','codex'):
            p=self.bin/name; p.write_text(stub); p.chmod(0o755)
        p=self.bin/'tmux'; p.write_text('#!/bin/sh\nexit 0\n'); p.chmod(0o755)
        # A stub fetch keeps the launch integration offline while preserving real local git.
        real_git=subprocess.check_output(['which','git'],text=True).strip()
        p=self.bin/'git'; p.write_text('#!/bin/sh\ncase " $* " in *" fetch "*) exit 0;; esac\nexec '+shlex.quote(real_git)+' "$@"\n'); p.chmod(0o755)

    def git(self,*args):
        return subprocess.run(['git','-C',str(self.repo),*args],check=True,capture_output=True,text=True)

    def save(self):
        self.path.write_text(json.dumps(self.policy))

    def run_cell(self,*args,ok=True,input=None,env=None):
        result=subprocess.run([str(CELLCTL),*args],env=env or self.env,input=input,capture_output=True,text=True)
        if ok:
            self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        else:
            self.assertNotEqual(result.returncode,0,result.stdout+result.stderr)
        return result

    def resolve(self,role='pr-review-desk',provider='',requested='',harness='',ok=True):
        r=self.run_cell('model-policy','resolve',str(self.path),provider,role,requested,str(CELLCTL),harness,ok=ok)
        return json.loads(r.stdout) if ok else r

    def test_role_resolution(self):
        r=self.resolve()
        self.assertEqual((r['model'],r['effort']),('claude-opus-4-8[1m]','high'))
        self.assertEqual(self.resolve('worker-desk')['provider'],'glm')
        self.assertEqual(self.resolve('intake-desk')['harness'],'codex')
        self.assertEqual(self.resolve('intake-desk')['effort'],'medium')

    def test_direct_and_alias_requests(self):
        self.assertEqual(self.resolve(requested='opus')['model'],'claude-opus-4-8[1m]')
        for value in ('claude-opus-5','Opus5','claude-opus-5[1m]','gateway/claude-opus-5','latest','opusplan'):
            with self.subTest(value=value): self.resolve(requested=value,ok=False)

    def test_ban_cannot_be_removed_by_omitting_deny(self):
        self.policy['deny']=[]
        self.policy['providers']['anthropic']['tiers']['strong']['model']='claude-opus-5'
        self.save(); self.resolve(ok=False)

    def test_invalid_unused_provider_still_fails(self):
        self.policy['providers']['kimi']['tiers']['fast']['model']='opus'
        self.save(); self.resolve(ok=False)

    def test_unsupported_efforts(self):
        for provider,value in [('glm','medium'),('kimi','xhigh'),('codex','max')]:
            with self.subTest(provider=provider):
                original=copy.deepcopy(self.policy)
                spec=self.policy['providers'][provider]['tiers']['mid']
                spec['effort']=value; spec['supported_efforts'].append(value)
                self.save(); self.resolve(ok=False)
                self.policy=original

    def test_unknown_role_and_provider(self):
        self.resolve(role='typo',ok=False)
        self.resolve(provider='typo',ok=False)
        self.resolve(provider='glm',harness='codex',ok=False)

    def test_explicit_override_routes_provider_and_tier(self):
        r=self.resolve(provider='kimi',requested='opus')
        self.assertEqual((r['model'],r['effort']),('k3[1m]','high'))
        r=self.resolve(harness='codex')
        self.assertEqual((r['provider'],r['harness']),('codex','codex'))

    def test_digest_changes_with_effort(self):
        before=self.resolve()['policy_sha256']
        self.policy['providers']['anthropic']['tiers']['strong']['effort']='xhigh'
        self.save(); self.assertNotEqual(before,self.resolve()['policy_sha256'])

    def test_child_mapping_allowlist_and_hooks(self):
        r=self.resolve(); settings=r['settings']; env=r['env']
        self.assertEqual(env['ANTHROPIC_DEFAULT_OPUS_MODEL'],'claude-opus-4-8[1m]')
        self.assertEqual(env['CLAUDE_CODE_SUBAGENT_MODEL'],r['model'])
        self.assertEqual(env['CLAUDE_CODE_EFFORT_LEVEL'],'high')
        self.assertFalse(any('opus-5' in x for x in settings['availableModels']))
        command=settings['hooks']['PreToolUse'][0]['hooks'][0]['command']
        for value,code in [('opus',0),('inherit',0),('claude-opus-5',2),('unknown-model',2)]:
            event={'hook_event_name':'PreToolUse','tool_name':'Agent','tool_input':{'model':value}}
            p=subprocess.run(shlex.split(command),input=json.dumps(event),capture_output=True,text=True,env=self.env)
            self.assertEqual(p.returncode,code,p.stderr)
        event={'hook_event_name':'PreModelSwitch','to_model':'claude-opus-5','requested_model':'opus'}
        p=subprocess.run(shlex.split(command),input=json.dumps(event),capture_output=True,text=True,env=self.env)
        self.assertEqual(p.returncode,2,p.stderr)

    def test_desk_show_and_up_agree_on_mixed_providers(self):
        show=self.run_cell('show','example').stdout
        up=self.run_cell('up','example','--cockpit','tmux').stdout
        for role,provider,model,effort in [('pr-review-desk','anthropic','claude-opus-4-8[1m]','high'),('worker-desk','glm','glm-5.3-flash[1m]','high'),('intake-desk','codex','gpt-5.6-terra','medium')]:
            desk=self.run_cell('desk','example',role).stdout
            for output in (show,up,desk):
                self.assertIn(model,output)
                self.assertIn('provider='+provider,output)
                self.assertIn('effort='+effort,output)

    def test_up_preflights_all_roles_before_windows(self):
        env=dict(self.env); env.pop('ZAI_API_KEY')
        r=self.run_cell('up','example','--cockpit','tmux',env=env,ok=False)
        self.assertIn('no role windows launched',r.stderr)
        self.assertNotIn('[dry-run] worker-desk:',r.stdout)
        self.assertNotIn('fixture-kimi',r.stdout+r.stderr)

    def test_policy_persistence_refuses_ignored_legacy_pin(self):
        before=self.envfile.read_bytes()
        self.run_cell('desk','example','pr-review-desk','--model','opus','--set',ok=False)
        self.assertEqual(before,self.envfile.read_bytes())

    def test_unimplemented_launch_paths_refuse(self):
        self.run_cell('up','example','--cockpit','tmux','--automate','every 5 minutes',ok=False)
        self.envfile.write_text(self.envfile.read_text().replace('CELL_KIND=house','CELL_KIND=scrubbed')+'CELL_REPO_SLUG=example-org/example-repo\n')
        self.run_cell('desk','example','worker-desk',ok=False)

    def test_competing_allowlist_refuses_before_launch(self):
        (self.cfg/'settings.json').write_text(json.dumps({'availableModels':['claude-opus-5']}))
        r=self.run_cell('desk','example','pr-review-desk',ok=False)
        self.assertIn('availableModels conflicts',r.stderr)
        self.assertFalse((self.cell/'worktrees').exists())

    def test_old_harness_refuses_before_launch(self):
        p=self.bin/'claude'; p.write_text(p.read_text().replace('2.1.278','2.1.200'))
        r=self.run_cell('desk','example','pr-review-desk',ok=False)
        self.assertIn('>=2.1.251',r.stderr)

    def test_unknown_effort_field_refuses(self):
        self.policy['roles']['pr-review-desk']['effort']='max'
        self.save(); self.resolve(ok=False)

    def test_real_exec_argv_and_env(self):
        # Reuse local worktrees; launcher fetch is stubbed, model binaries only record argv.
        env=dict(self.env); env.pop('DRY_RUN')
        for role in ('pr-review-desk','worker-desk','intake-desk'):
            wt=self.cell/'worktrees'/role; wt.parent.mkdir(exist_ok=True)
            self.git('worktree','add','-q',str(wt),'-b',role)
            self.run_cell('desk','example',role,env=env)
            record=json.loads(Path(env['POLICY_TEST_RECORD']).read_text())
            if role=='intake-desk':
                self.assertIn('model_provider="openai"',record['argv'])
                self.assertIn('model_reasoning_effort="medium"',record['argv'])
                self.assertIn('agents.default_subagent_model="gpt-5.6-terra"',record['argv'])
                self.assertIn('agents.default_subagent_reasoning_effort="medium"',record['argv'])
            else:
                self.assertIn('--effort',record['argv']); self.assertIn('high',record['argv'])
                self.assertEqual(record['env']['CLAUDE_CODE_EFFORT_LEVEL'],'high')
                settings=json.loads(record['argv'][record['argv'].index('--settings')+1])
                self.assertEqual(settings['env']['ANTHROPIC_MODEL'],record['env']['ANTHROPIC_MODEL'])
                if role=='worker-desk':
                    self.assertEqual(record['env']['ANTHROPIC_DEFAULT_OPUS_MODEL'],'glm-5.3[1m]')
                    self.assertEqual(record['env']['ANTHROPIC_MODEL'],'glm-5.3-flash[1m]')
                    self.assertIn('z.ai',record['env']['ANTHROPIC_BASE_URL'])
                    self.assertEqual(record['env']['ANTHROPIC_AUTH_TOKEN'],'fixture-zai')


if __name__=='__main__': unittest.main(verbosity=2)
