"""Offline cold-preparation flows; no cluster, package install or model call."""
import argparse
import fcntl
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('prepare', HERE / 'prepare.py')
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


class Preparation(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.work = Path(self.tmp.name)
        self.agent = self.work / 'agent'
        self.agent.write_text('#!/bin/sh\ntouch "' + str(self.work / 'agent-invoked') + '"\n')
        self.agent.chmod(0o700)
        self.manifest = self.work / 'package.json'
        self.manifest.write_text(json.dumps({'name': '@anthropic-ai/claude-code', 'version': '2.1.228'}))
        self.instructions = self.work / 'role.md'
        self.instructions.write_text('Synthetic role inputs; no credential or model.\n')
        (self.work / 'repository').mkdir()

    def args(self, role='worker-desk', apply=True):
        return argparse.Namespace(work=str(self.work), cell='example', role=role,
            agent_version='2.1.228', model='claude-sonnet-5-5',
            agent=str(self.agent), version_manifest=str(self.manifest),
            instructions=str(self.instructions), workdir=str(self.work / 'repository'),
            prepare=apply)

    def run_prepare(self, args):
        with patch.object(prepare.os, 'geteuid', return_value=999), \
             patch.object(prepare.os, 'getegid', return_value=999):
            return prepare.run(args, launcher_path=HERE / 'desk-session.py')

    def test_five_role_handoffs_use_actual_prepared_state(self):
        for role in ['the-desk', 'intake-desk', 'worker-desk', 'pr-review-desk', 'verify-desk']:
            with self.subTest(role=role), tempfile.TemporaryDirectory() as directory:
                args = self.args(role)
                args.work = directory
                result = self.run_prepare(args)
                state = json.loads((Path(directory) / '.assay-compat/state.json').read_text())
                descriptor = json.loads((Path(directory) / 'compat-launch.json').read_text())
                self.assertEqual(result['descriptor'], descriptor)
                self.assertEqual(result['launch_id'], state['launch_id'])
                self.assertEqual(result['workload_env'], {
                    'ASSAY_COMPAT_LAUNCH_ID': state['launch_id'],
                    'ASSAY_COMPAT_INIT_ID': state['initialization_id']})
                self.assertEqual(descriptor['role'], role)
                self.assertNotIn('launch_id', descriptor)
                self.assertEqual((state['state'], state['pod_uid'], state['agent_pid']), ('prepared', '', 0))
                self.assertEqual((Path(directory) / 'compat-launch.json').stat().st_mode & 0o777, 0o600)
                self.assertFalse((self.work / 'agent-invoked').exists())

    def test_default_plan_writes_nothing(self):
        before = sorted(self.work.iterdir())
        result = self.run_prepare(self.args(apply=False))
        self.assertEqual(sorted(self.work.iterdir()), before)
        self.assertEqual(result['mode'], 'plan')
        self.assertNotIn('launch_id', result)

    def test_existing_descriptor_and_history_are_never_replaced(self):
        for name in ['compat-launch.json', '.assay-compat']:
            with self.subTest(name=name):
                path = self.work / name
                path.write_text('unknown predecessor')
                with self.assertRaises(prepare.Error):
                    self.run_prepare(self.args())
                self.assertEqual(path.read_text(), 'unknown predecessor')
                path.unlink()  # synthetic fixture only

    def test_dangling_history_link_is_not_cold(self):
        (self.work / '.assay-compat').symlink_to(self.work / 'absent')
        with self.assertRaises(prepare.Error):
            self.run_prepare(self.args())
        self.assertTrue((self.work / '.assay-compat').is_symlink())

    def test_repeat_preparation_preserves_every_original_binding(self):
        self.run_prepare(self.args())
        files = ['compat-launch.json', '.assay-compat/state.json']
        before = {name: (self.work / name).read_bytes() for name in files}
        with self.assertRaises(prepare.Error):
            self.run_prepare(self.args())
        self.assertEqual(before, {name: (self.work / name).read_bytes() for name in files})

    def test_wrong_package_version_stops_before_descriptor(self):
        args = self.args()
        args.agent_version = '2.1.227'
        with self.assertRaises(prepare.Error):
            self.run_prepare(args)
        self.assertFalse((self.work / 'compat-launch.json').exists())

    def test_wrong_package_and_missing_inputs_stop_before_descriptor(self):
        self.manifest.write_text('{"name":"another-package","version":"2.1.228"}')
        with self.assertRaises(prepare.Error):
            self.run_prepare(self.args())
        self.assertFalse((self.work / 'compat-launch.json').exists())

    def test_missing_executable_instructions_or_work_stops_before_descriptor(self):
        for field in ['agent', 'instructions', 'workdir']:
            with self.subTest(field=field):
                args = self.args()
                setattr(args, field, str(self.work / 'absent'))
                with self.assertRaises(prepare.Error):
                    self.run_prepare(args)
                self.assertFalse((self.work / 'compat-launch.json').exists())

    def test_concurrent_cold_preparation_is_held(self):
        with (self.work / '.assay-compat-prepare.lock').open('w') as holder:
            fcntl.flock(holder.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
            with self.assertRaises(prepare.Error):
                self.run_prepare(self.args())
            self.assertFalse((self.work / 'compat-launch.json').exists())

    def test_floating_package_or_model_is_not_a_pin(self):
        for field, value in [('agent_version', 'latest'), ('agent_version', '^2.1.228'),
                             ('model', 'sonnet'), ('model', 'claude-sonnet-latest')]:
            with self.subTest(field=field, value=value):
                args = self.args()
                setattr(args, field, value)
                with self.assertRaises(prepare.Error):
                    self.run_prepare(args)
        self.assertFalse((self.work / 'compat-launch.json').exists())

    def test_preparer_identity_is_workload_identity(self):
        for uid, gid in [(0, 999), (999, 0)]:
            with self.subTest(uid=uid, gid=gid), \
                 patch.object(prepare.os, 'geteuid', return_value=uid), \
                 patch.object(prepare.os, 'getegid', return_value=gid):
                with self.assertRaises(prepare.Error):
                    prepare.run(self.args(), launcher_path=HERE / 'desk-session.py')
        self.assertFalse((self.work / 'compat-launch.json').exists())

    def test_lower_layer_failure_is_held_and_not_erased(self):
        with patch.object(prepare.subprocess, 'run', side_effect=OSError('synthetic failure')):
            with self.assertRaises(prepare.Error):
                self.run_prepare(self.args())
        self.assertTrue((self.work / 'compat-launch.json').is_file())
        with self.assertRaises(prepare.Error):
            self.run_prepare(self.args())


if __name__ == '__main__':
    unittest.main()
