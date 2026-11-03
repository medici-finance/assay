#!/usr/bin/env python3
"""Actual canonical refresh-production controls; explicit owned container required.
Each fault must produce a named runtime failure; source bytes restore exactly.
A compound fault removes cooperating checks for the single broken execution mode.
"""
import os
import pathlib
import signal
import subprocess
import sys

module = pathlib.Path(__file__).resolve().parents[2]
if not os.environ.get('ASSAY_TEST_POSTGRES_DSN'):
    raise SystemExit('explicit disposable-container DSN required')
top = pathlib.Path(subprocess.check_output(['git','rev-parse','--show-toplevel'],cwd=module,text=True).strip())
branch = subprocess.check_output(['git','branch','--show-current'],cwd=module,text=True).strip()
if not (top/'.git').is_file() or not branch.startswith('codex/'):
    raise SystemExit('owned linked feature worktree required')
paths=['store/refresh.go','store/refresh_attempt.go','store/migrations/00003_refresh.sql']
subprocess.run(['git','diff','--exit-code','HEAD','--',*paths],cwd=module,check=True,stdout=subprocess.DEVNULL)
original={p:(module/p).read_bytes() for p in paths}
env=dict(os.environ,GOWORK='off',GOPROXY='off',GOTOOLCHAIN='local')

def run(test):
    target=top/"tools/desk" if test.startswith("TestMirrorPersisted") else module
    package="./internal/deskkit" if target!=module else "./store"
    return subprocess.run(['go','test','-count=1','-timeout=60s','-run','^'+test+'$',package],cwd=target,env=env,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=75)

def change(state,path,old,new):
    if state[path].count(old)!=1:
        raise RuntimeError('nonunique mutation anchor: '+path)
    state[path]=state[path].replace(old,new)

def mutate(name):
    state={p:b.decode() for p,b in original.items()}
    code='store/refresh.go';attempt='store/refresh_attempt.go';sql='store/migrations/00003_refresh.sql'
    if name=='canonical-store':
        change(state,code,'if s != g.store || !t.Owner {','if !t.Owner {')
    elif name=='current-generation':
        change(state,code,' || string(f.generation) != t.Generation','')
    elif name=='common-transaction':
        change(state,code,'bound := &MirrorStore{database: g.store.database, limits: g.store.limits, tx: tx}','bound := &MirrorStore{database: g.store.database, limits: g.store.limits}')
        change(state,code,'header, err := bound.lockMirrorHeader(ctx, tx, f.scope, k.Scope)','header, err := readMirrorHeader(ctx, tx, f.scope, k.Scope, bound.limits.MaxBytes, false)')
        change(state,sql,' OR (NEW.snapshot_version<>OLD.snapshot_version AND current_xid IS DISTINCT FROM pg_current_xact_id())','')
    elif name=='source-debit':
        change(state,attempt,'UPDATE public.refresh_budgets SET http_used=http_used+1','UPDATE public.refresh_budgets SET http_used=http_used')
        change(state,sql,'source_id=NEW.source_id AND known AND NOT unknown_hold AND write_xid=pg_current_xact_id()','source_id=NEW.source_id AND known AND NOT unknown_hold')
    elif name=='retry-publication':
        change(state,sql,'retry_at=greatest(retry_at,NEW.retry_at)','retry_at=retry_at')
    elif name=='unknown-publication':
        change(state,sql,'unknown_hold=unknown_hold OR NEW.unknown_hold','unknown_hold=unknown_hold')
    elif name=='aggregate-reconciliation':
        change(state,code,'held := receipt.Held != "" || count != observed || unknown > 0','held := receipt.Held != ""')
        change(state,code,'measuredHTTP == 0 || c.Update.CompleteScan && incomplete > 0','measuredHTTP == 0')
        change(state,code,'receipt.HTTP.Value != measuredHTTP','receipt.HTTP.Value != count')
    elif name=='data-bound':
        change(state,code,'if c.Update != nil && c.Failure == "" && (c.Update.CompleteScan || len(c.Update.Upsert) > 0) {','if false {')
    elif name=='decoded-prefix':
        change(state,attempt,'func validAttemptObservation(o mirror.AttemptObservation) bool {','func validAttemptObservation(o mirror.AttemptObservation) bool { if !o.DecodedBytes.Known && o.DecodedBytes.Value>0 { return false }')
    elif name=='zero-entry':
        change(state,attempt,'if o.HTTP.Value == 0 {','if o.HTTP.Value==0 {return false}; if false {')
    elif name=='postcommit-zero-closure':
        change(state,attempt,'_ = g.ObserveAttempt(cleanup, reservation, mirror.AttemptObservation{HTTP: mirror.WorkCount{Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: time.Now().UTC()})','_ = cleanup')
    elif name=='shared-retry-policy':
        change(state,code,'l.Refresh.FailureBackoff == 0','l.Refresh.FailureBackoff >= 0')
    elif name=='superseded-ticket-release':
        change(state,code,'UPDATE public.refresh_waiters SET released=true WHERE ticket_id=$1 AND admission=$2 AND generation=$3 AND NOT released','UPDATE public.refresh_waiters SET released=false WHERE ticket_id=$1 AND admission=$2 AND generation=$3 AND NOT released')
    elif name=='review-owner-isolation':
        change(state,code,'} else if reviews || !f.reviewOnlyPartial {','} else {')
    elif name=='joined-store-apply':
        change(state,code,'_, err = refresh.ApplyCompletion(ctx, bound, a, c)','c.Update=nil; _, err = refresh.ApplyCompletion(ctx, bound, a, c)')
    elif name=='joined-caller-ceiling':
        change(state,attempt,'UPDATE public.refresh_callers SET http_used=http_used+1 WHERE admission=$1 AND caller_id=$2 AND known AND http_used<http_limit','UPDATE public.refresh_callers SET http_used=http_used+1 WHERE admission=$1 AND caller_id=$2 AND known')
        anchor='''CREATE TABLE public.refresh_callers ('''
        begin=state[sql].index(anchor);end=state[sql].index('CREATE TABLE public.refresh_fills',begin)
        section=state[sql][begin:end]
        old='CHECK(http_limit>0 AND http_used<=http_limit)'
        if section.count(old)!=1: raise RuntimeError('nonunique caller ceiling')
        state[sql]=state[sql][:begin]+section.replace(old,'CHECK(http_limit>0)')+state[sql][end:]
    elif name=='requirement-class':
        change(state,code,'if reviews {\n\t\t\tcolumn = "review_retry_at"','if false {\n\t\t\tcolumn = "review_retry_at"')
    elif name=='outcome-version':
        change(state,code,'if checked && version > 0 && o.SnapshotVersion == strconv.FormatInt(version, 10) && version == f.version {','if checked && version >= 0 {')
    elif name=='sql-expiry':
        change(state,sql,'IF OLD.expires_at<=clock_timestamp() OR NEW.completion_xid IS DISTINCT FROM pg_current_xact_id() THEN','IF NEW.completion_xid IS DISTINCT FROM pg_current_xact_id() THEN')
    elif name=='sql-reservation':
        change(state,sql,'CREATE TRIGGER refresh_attempt_insert_guard BEFORE INSERT ON public.refresh_attempts FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_attempt_insert();','-- lower reservation guard removed by planted fault')
    else: raise RuntimeError('unknown fault')
    for p,text in state.items(): (module/p).write_text(text)

controls=[
 ('canonical-store','TestPGRefreshGateAtomicCompletion'),
 ('current-generation','TestPGRefreshGateProcesses'),
 ('common-transaction','TestPGRefreshGateAtomicCompletion'),
 ('source-debit','TestPGRefreshAttemptAuthority'),
 ('retry-publication','TestPGRefreshAttemptAuthority'),
 ('unknown-publication','TestPGRefreshAttemptAuthority'),
 ('aggregate-reconciliation','TestPGRefreshAttemptAuthority'),
 ('data-bound','TestPGRefreshAttemptAuthority'),
 ('decoded-prefix','TestPGRefreshAttemptAuthority'),
 ('zero-entry','TestPGRefreshAttemptAuthority'),
 ('postcommit-zero-closure','TestPGRefreshCapacityCancellation'),
 ('requirement-class','TestPGRefreshRequirementBackoff'),
 ('outcome-version','TestPGRefreshRequirementBackoff'),
 ('review-owner-isolation','TestPGRefreshRequirementBackoff'),
 ('sql-expiry','TestPGRefreshSchemaBoundary'),
 ('sql-reservation','TestPGRefreshSchemaBoundary'),
 ('joined-store-apply','TestMirrorPersistedReadThroughJoin/cohort-github'),
 ('joined-caller-ceiling','TestMirrorPersistedReadThroughJoin/lower-native-budget'),
 ('superseded-ticket-release','TestPGRefreshGateAtomicCompletion'),
 ('shared-retry-policy','TestPGRefreshSchemaBoundary'),
]
composites={'common-transaction','source-debit','aggregate-reconciliation','joined-caller-ceiling'}
# Resume qualification after a corrected planted fault; prior controls are kept
# with their exact source fingerprints, never silently reported by this suffix.
if len(sys.argv)>2: raise SystemExit('expected optional starting control name')
if len(sys.argv)==2:
    names=[name for name,_ in controls]
    if sys.argv[1] not in names: raise SystemExit('unknown starting control')
    controls=controls[names.index(sys.argv[1]):]


signal.signal(signal.SIGTERM,lambda *_:sys.exit(143))
try:
    baseline=run('TestPGRefresh(GateAtomicCompletion|GateProcesses|AttemptAuthority|RequirementBackoff|CapacityCancellation|SchemaBoundary)')
    if baseline.returncode: print(baseline.stdout);raise SystemExit('baseline failed')
    for name,test in controls:
        for p,b in original.items(): (module/p).write_bytes(b)
        mutate(name);result=run(test)
        if not result.returncode:raise SystemExit('SURVIVED: '+name)
        if any(x in result.stdout for x in ['[build failed]','[setup failed]','undefined:','explicit container fixture is unavailable','module lookup disabled']):
            print(result.stdout);raise SystemExit('INVALID compile/setup: '+name)
        if '--- FAIL: '+test not in result.stdout:
            print(result.stdout);raise SystemExit('INVALID named failure absent: '+name)
        print('KILLED: '+name+(' [COMPOSITE guard bundle]' if name in composites else '')+' -> '+test,flush=True)
    print('PASS: selected '+str(len(controls))+' actual production faults killed ('+str(sum(name in composites for name,_ in controls))+' composite guard bundles); exact source restored',flush=True)
finally:
    for p,b in original.items(): (module/p).write_bytes(b)
