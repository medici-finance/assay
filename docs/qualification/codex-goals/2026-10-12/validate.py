"""Validate retained evidence only. No process launch or network access."""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
manifest = json.loads((ROOT / 'manifest.json').read_text())
sha = lambda data: hashlib.sha256(data).hexdigest()

def read(name):
    return [json.loads(line) for line in (ROOT / name).read_text().splitlines()]

def clients(rows, method):
    return [r for r in rows if r.get('direction') == 'client' and r['method'] == method]

def events(rows, method):
    return [r for r in rows if isinstance(r.get('payload'), dict)
            and r['payload'].get('method') == method]

def response(rows, ident):
    return next(r['payload'] for r in rows if r.get('direction') == 'server'
                and r['payload'].get('id') == ident)

def record(rows, tag):
    return next(r['payload'] for r in rows if r.get('harnessRecord') == tag)

def check_reasoning(value):
    if isinstance(value, dict):
        if value.get('type') == 'reasoning':
            assert value == {'type': 'reasoning', 'omitted': 'native reasoning payload'}
        for nested in value.values():
            check_reasoning(nested)
    elif isinstance(value, list):
        for nested in value:
            check_reasoning(nested)

for source in manifest['sources']:
    data = (ROOT / source['artifact']).read_bytes()
    assert sha(data) == source['artifactSha256']
    rows = read(source['artifact'])
    assert [r['sourceLine'] for r in rows] == list(range(1, source['sourceLines'] + 1))
    counts = {}
    for row in rows:
        if 'omitted' in row:
            counts[row['omitted']] = counts.get(row['omitted'], 0) + 1
        check_reasoning(row)
    assert counts == source['omittedLines']
for driver in manifest['drivers']:
    assert sha((ROOT / driver['artifact']).read_bytes()) == driver['sha256']
bootstrap = (ROOT / 'generated-bootstrap.txt').read_text()
assert sha(bootstrap.encode()) == manifest['bootstrapSha256']

for name in ['original-bootstrap.jsonl', 'bootstrap-tools-rpc.jsonl']:
    rows = read(name)
    starts = clients(rows, 'turn/start')
    assert len(starts) == 1 and starts[0]['params']['input'][0]['text'] == bootstrap
    began, ended = events(rows, 'turn/started'), events(rows, 'turn/completed')
    assert len(began) == len(ended) == 2
    ids = [r['payload']['params']['turn']['id'] for r in began]
    assert len(set(ids)) == 2
    assert ids == [r['payload']['params']['turn']['id'] for r in ended]
    assert ended[0]['sourceLine'] < began[1]['sourceLine']
    steer = clients(rows, 'turn/steer')
    assert len(steer) == 1 and steer[0]['params']['expectedTurnId'] == ids[1]
    assert began[1]['sourceLine'] < steer[0]['sourceLine']
    observed = record(rows, 'OBSERVED')
    assert observed['files']['counter.txt'].strip() == '1'
    assert observed['files']['steering.txt'].strip() == 'STEERING_ACCEPTED'

rows = read('bootstrap-tools-rpc.jsonl')
provenance = record(rows, 'PROVENANCE')
assert provenance['version'] == 'codex-cli 0.162.1'
assert provenance['status'] == ''
assert record(rows, 'SOURCE_AFTER') == {'revision': provenance['revision'], 'status': ''}
assert record(rows, 'FINAL_READBACK') == {'goal': None}
assert record(rows, 'OBSERVED')['goal']['goal']['status'] == 'paused'
assert record(rows, 'OBSERVED')['goal']['goal']['tokenBudget'] is None
# Establishment came from native tools: no client set before the two turns end.
assert min(r['sourceLine'] for r in clients(rows, 'thread/goal/set')) > events(rows, 'turn/completed')[-1]['sourceLine']
thread_id = clients(rows, 'turn/start')[0]['params']['threadId']

native = [r['native']['payload'] for r in read('bootstrap-tools-native.jsonl') if 'native' in r]
calls = [r for r in native if r['type'] == 'custom_tool_call']
outputs = {r['call_id']: r for r in native if r['type'] == 'custom_tool_call_output'}
assert len(calls) == len(outputs) == 5
assert 'tools.get_goal({})' in calls[0]['input']
create = calls[1]
assert 'tools.create_goal({objective:' in create['input']
assert 'token_budget' not in create['input'] and 'tools.get_goal({})' in create['input']
assert 'cat .agents/skills/worker-desk/SKILL.md' in calls[2]['input']
assert 'counter.txt' in calls[3]['input']
assert 'tools.update_goal({status:"paused"})' in calls[4]['input']
assert 'tools.get_goal({})' in calls[4]['input']

def tool_values(call):
    return [json.loads(v['text']) for v in outputs[call['call_id']]['output']
            if v.get('type') == 'input_text' and v['text'].startswith('{')]
assert tool_values(calls[0])[0]['goal'] is None
active = tool_values(create)
assert len(active) == 2
for value in active:
    assert value['goal']['status'] == 'active' and value['goal']['threadId'] == thread_id
    assert value['goal']['objective'] in bootstrap
    assert value['remainingTokens'] is None
assert 'Local qualification worker desk only.' in tool_values(calls[2])[0]['output']
assert tool_values(calls[3])[0]['output'] == 'count=1\n'
for value in tool_values(calls[4])[1:]:
    assert value['goal']['status'] == 'paused' and value['goal']['threadId'] == thread_id

for name in ['goals-disabled.jsonl', 'existing-paused.jsonl']:
    rows = read(name)
    assert clients(rows, 'turn/start')[0]['params']['input'][0]['text'] == bootstrap
    observed = record(rows, 'OBSERVED')
    assert not observed['roleInvoked'] and observed['before'] == observed['after']
    if name == 'existing-paused.jsonl':
        assert observed['after']['goal']['status'] == 'paused'
        assert record(rows, 'FINAL_READBACK') == {'goal': None}
    else:
        assert observed['after']['error']['code'] == -32600
assert record(read('engine-control.jsonl'), 'OBSERVED')['completedTurns'] == 2
print('PASS: six retained transcripts; hashes, full line accounting, actual bootstrap, native tool admission/readback, automatic turns, steering, controls and cleanup')
