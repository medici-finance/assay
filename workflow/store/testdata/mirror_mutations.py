#!/usr/bin/env python3
"""Actual mirror-production faults; explicit owned Docker fixture required.
No source copies or mocked SQL. Every control must fail its named test; exact
production bytes are restored even on interruption. Compile/setup failures do not count.
"""
import os
import pathlib
import signal
import subprocess
import sys

module = pathlib.Path(__file__).resolve().parents[2]
if not os.environ.get("ASSAY_TEST_POSTGRES_DSN"):
    raise SystemExit("explicit disposable-container DSN required")
top = pathlib.Path(subprocess.check_output(["git","rev-parse","--show-toplevel"],cwd=module,text=True).strip())
branch = subprocess.check_output(["git","branch","--show-current"],cwd=module,text=True).strip()
if not (top/".git").is_file() or not branch.startswith("codex/"):
    raise SystemExit("owned linked feature worktree required")
paths = ["store/mirror.go","store/mirror_codec.go","store/migrations/00002_mirror.sql"]
subprocess.run(["git","diff","--exit-code","HEAD","--",*paths],cwd=module,check=True,stdout=subprocess.DEVNULL)
original = {p:(module/p).read_bytes() for p in paths}
env = dict(os.environ,GOWORK="off",GOPROXY="off",GOTOOLCHAIN="local")

def run(test):
    return subprocess.run(["go","test","-count=1","-timeout=45s","-run","^"+test+"$","./store"],cwd=module,env=env,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=55)

def change(state,path,old,new):
    if state[path].count(old)!=1:
        raise RuntimeError("nonunique mutation anchor: "+path)
    state[path]=state[path].replace(old,new)

def mutate(name):
    state={p:b.decode() for p,b in original.items()}
    code="store/mirror.go"; sql="store/migrations/00002_mirror.sql"
    if name=="authorization":
        change(state,code,'if a == nil || a.ID() == "" || a.ID() != scope.AdmissionID {','if false {')
        change(state,code,'if err := a.Authorize(ctx, scope.Repository); err != nil || a.ID() != scope.AdmissionID {','if false {')
    elif name=="post-io-authorization":
        anchor='''defer func() {
		if current := admitMirror(ctx, a, scope); current != nil {'''
        change(state,code,anchor,'''defer func() {
		if current := admitMirror(ctx, a, scope); current != nil && false {''')
    elif name=="precommit-authorization":
        change(state,code,'''if err = admitMirror(ctx, a, u.Scope); err != nil {
		return mirror.Snapshot{}, err
	}
	if err = tx.Commit();''','''if err = admitMirror(ctx, a, u.Scope); err != nil && false {
		return mirror.Snapshot{}, err
	}
	if err = tx.Commit();''')
    elif name=="cas":
        change(state,code,'WHERE scope_key=$1 AND version=$2 RETURNING version','WHERE scope_key=$1 AND ($2::bigint>=0) RETURNING version')
    elif name=="admission-partition":
        change(state,code,'string(scope.Kind), scope.AdmissionID}','string(scope.Kind), ""}')
    elif name=="partial-absence":
        change(state,code,'if len(u.NotInInventory) > 0 && (!u.CompleteScan || !u.FullInventory) {','if false {')
    elif name=="complete-proof":
        change(state,sql,"IF NEW.state='complete' AND NOT NEW.complete_scan THEN", "IF false THEN")
        change(state,sql,"IF coalesce(NEW.last_complete,zero_stamp)>coalesce(OLD.last_complete,zero_stamp) AND (NOT NEW.complete_scan OR NEW.state<>'complete') THEN","IF false THEN")
    elif name=="inventory-proof":
        change(state,sql,"IF coalesce(NEW.last_inventory,zero_stamp)>coalesce(OLD.last_inventory,zero_stamp) AND (NOT NEW.complete_scan OR NOT NEW.full_inventory OR NEW.state<>'complete') THEN","IF false THEN")
    elif name=="observed-proof":
        change(state,sql,"IF coalesce(NEW.observed_up_to,zero_stamp)>coalesce(OLD.observed_up_to,zero_stamp) AND (NOT NEW.complete_scan OR NEW.state<>'complete') THEN","IF false THEN")
    elif name=="monotonicity":
        anchor="IF coalesce(NEW.last_attempted,zero_stamp)<coalesce(OLD.last_attempted,zero_stamp) OR coalesce(NEW.last_complete,zero_stamp)<coalesce(OLD.last_complete,zero_stamp) OR coalesce(NEW.last_inventory,zero_stamp)<coalesce(OLD.last_inventory,zero_stamp) OR coalesce(NEW.observed_up_to,zero_stamp)<coalesce(OLD.observed_up_to,zero_stamp) THEN"
        change(state,sql,anchor,"IF false THEN")
    elif name=="hold-recovery":
        change(state,sql,"IF OLD.reason='permission' AND NOT coalesce(","IF false AND NOT coalesce(")
    elif name=="null-recovery-proof":
        change(state,sql,"NOT coalesce((NEW.complete_scan AND NEW.full_inventory AND NEW.state='complete' AND NEW.last_complete>OLD.last_attempted AND NEW.last_inventory>OLD.last_attempted),false)","NOT (NEW.complete_scan AND NEW.full_inventory AND NEW.state='complete' AND NEW.last_complete>OLD.last_attempted AND NEW.last_inventory>OLD.last_attempted)")
    elif name=="new-absence":
        change(state,sql,"IF TG_OP='INSERT' AND NOT NEW.in_inventory THEN","IF false THEN")
    elif name=="rebuild-completeness":
        change(state,sql,"IF NEW.version<>0 OR NEW.state<>'never-read' OR NEW.reason<>'' OR NEW.last_attempted IS NOT NULL OR NEW.last_complete IS NOT NULL OR NEW.last_inventory IS NOT NULL OR NEW.observed_up_to IS NOT NULL OR NEW.validator_since IS NOT NULL OR NEW.complete_scan OR NEW.full_inventory OR NEW.retention_required THEN","IF false THEN")
    elif name=="codec-frame":
        change(state,"store/mirror_codec.go","!errors.Is(err, io.EOF)","err == io.EOF && false")
    elif name=="derived-history":
        change(state,code,"return h.snapshot, nil","if len(h.snapshot.Records)>0 { h.snapshot.Coverage.RetainedFrom=h.snapshot.Records[0].UpdatedAt }; return h.snapshot, nil")
    elif name=="split-transaction":
        # One composite faulty execution mode: commit the header early and bypass
        # the matching same-transaction row guard. The injected second-row failure
        # must expose the erroneous acknowledged header on reopen.
        change(state,code,"for _, r := range u.Upsert {\n\t\tpayload := payloads[r.ID]", "if err=tx.Commit(); err!=nil{return mirror.Snapshot{},err}; tx,err=s.database.db.BeginTx(ctx,nil); if err!=nil{return mirror.Snapshot{},err}; defer tx.Rollback()\n\tfor _, r := range u.Upsert {\n\t\tpayload := payloads[r.ID]")
        change(state,sql,"IF s.write_xid<>pg_current_xact_id() OR s.retention_required OR s.version=0 THEN","IF s.retention_required OR s.version=0 THEN")
    else:
        raise RuntimeError("unknown fault")
    for p,text in state.items(): (module/p).write_text(text)

controls=[("authorization","TestPGMirrorConformance"),("post-io-authorization","TestPGMirrorScopeIsolation"),("precommit-authorization","TestPGMirrorScopeIsolation"),("admission-partition","TestPGMirrorScopeIsolation"),("partial-absence","TestPGMirrorConformance"),("cas","TestPGMirrorConcurrentCAS"),("complete-proof","TestPGMirrorAtomicCoverage"),("inventory-proof","TestPGMirrorAtomicCoverage"),("observed-proof","TestPGMirrorAtomicCoverage"),("monotonicity","TestPGMirrorAtomicCoverage"),("hold-recovery","TestPGMirrorConformance"),("null-recovery-proof","TestPGMirrorAtomicCoverage"),("new-absence","TestPGMirrorAtomicCoverage"),("rebuild-completeness","TestPGMirrorRestartRebuild"),("codec-frame","TestPGMirrorPresenceHistory"),("derived-history","TestPGMirrorPresenceHistory"),("split-transaction","TestPGMirrorAtomicCoverage")]
signal.signal(signal.SIGTERM,lambda *_:sys.exit(143))
try:
    baseline=run("TestPGMirror(Conformance|AtomicCoverage|ConcurrentCAS|RestartRebuild|ScopeIsolation|PresenceHistory|SchemaBoundary)")
    if baseline.returncode:
        print(baseline.stdout);raise SystemExit("baseline failed")
    for name,test in controls:
        for p,b in original.items(): (module/p).write_bytes(b)
        mutate(name); result=run(test)
        if not result.returncode: raise SystemExit("SURVIVED: "+name)
        if any(x in result.stdout for x in ["[build failed]","build failed","[setup failed]","undefined:","explicit container fixture is unavailable"]):
            print(result.stdout);raise SystemExit("INVALID compile/setup: "+name)
        if "--- FAIL: "+test not in result.stdout:
            print(result.stdout);raise SystemExit("INVALID named failure absent: "+name)
        print("KILLED: "+name+" -> "+test,flush=True)
    print("PASS: all17 production-path faults killed; exact source restored",flush=True)
finally:
    for p,b in original.items(): (module/p).write_bytes(b)
