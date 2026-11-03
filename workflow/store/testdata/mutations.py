#!/usr/bin/env python3
"""Plant production-path faults in an owned clean worktree; restore exact bytes.
Requires the explicit disposable-container ASSAY_TEST_POSTGRES_DSN. No downloads,
server startup, ambient daemon discovery or accepted-verification claims.
"""
import os
import pathlib
import signal
import subprocess
import sys

module = pathlib.Path(__file__).resolve().parents[2]
if not (module / "go.mod").exists():
    raise SystemExit("run the committed script from workflow/store/testdata")
if not os.environ.get("ASSAY_TEST_POSTGRES_DSN"):
    raise SystemExit("explicit disposable-container DSN is required")
top = pathlib.Path(subprocess.check_output(["git", "rev-parse", "--show-toplevel"], cwd=module, text=True).strip())
if not (top / ".git").is_file():
    raise SystemExit("an owned linked worktree is required")
branch = subprocess.check_output(["git", "branch", "--show-current"], cwd=module, text=True).strip()
if not branch or branch in {"main", "master"}:
    raise SystemExit("an owned feature branch is required")
paths = ["store/config.go", "store/migrate.go", "store/ledger.go", "internal/pgtest/fixture.go"]
subprocess.run(["git", "diff", "--exit-code", "HEAD", "--", *paths], cwd=module, check=True, stdout=subprocess.DEVNULL)
original = {p: (module / p).read_bytes() for p in paths}
env = dict(os.environ, GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local")

def run(test):
    return subprocess.run(["go", "test", "-count=1", "-timeout=35s", "-run", "^" + test + "$", "./store"], cwd=module, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, timeout=45)

def replace(state, path, old, new):
    if state[path].count(old) != 1:
        raise RuntimeError("mutation anchor is not unique: " + path)
    state[path] = state[path].replace(old, new)

def mutate(which):
    state = {p: b.decode() for p,b in original.items()}
    if which == "checksum":
        replace(state,"store/ledger.go","checksum != expected", "(checksum != expected && false)")
    elif which == "version":
        replace(state,"store/ledger.go","!ok || !applied || checksum != expected", "!applied || (ok && checksum != expected)")
        replace(state,"store/ledger.go","r.Version != int64(len(result)-1-i)", "r.Version != int64(len(result)-1-i) && false")
    elif which == "ambient":
        replace(state,"store/config.go",'strings.HasPrefix(key, "PG") && value != ""', 'strings.HasPrefix(key, "PG") && value != "" && false')
    elif which == "schema":
        replace(state,"store/ledger.go","!slices.Equal(actual, expected)", "!slices.Equal(actual, expected) && false")
    elif which == "fixture-owner":
        replace(state,"internal/pgtest/fixture.go",'marker != f.marker && (f.ready || marker != "")', 'marker != f.marker && false')
    elif which == "locking":
        replace(state,"store/migrate.go","goose.WithSessionLocker(locker)","goose.WithDisableGlobalRegistry(true)")
        replace(state,"store/migrate.go",'bootstrap, _ := fs.ReadFile(source, "ledger/create.sql")','_ = locker\n\tbootstrap, _ := fs.ReadFile(source, "ledger/create.sql")')
    elif which == "premature-ack":
        replace(state,"store/migrate.go",'"context"','"context"\n "database/sql"')
        replace(state,"store/migrate.go","provider.Up(ctx)","prematureUp(ctx,db,source,manifest)")
        replace(state,"store/migrate.go",'if _, err = prematureUp(ctx,db,source,manifest);','_ = provider\n if _, err = prematureUp(ctx,db,source,manifest);')
        state["store/migrate.go"] += '''
func prematureUp(ctx context.Context,db *sql.DB,source fs.FS,manifest map[int64]string)([]*goose.MigrationResult,error){
 bootstrap,_:=fs.ReadFile(source,"ledger/create.sql")
 if _,err:=db.ExecContext(ctx,strings.Replace(string(bootstrap),"CREATE TABLE ","CREATE TABLE IF NOT EXISTS ",1));err!=nil{return nil,err}
 for v:=int64(0);v<int64(len(manifest));v++ {if _,err:=db.ExecContext(ctx,"INSERT INTO public.coordination_schema(version_id,checksum) VALUES($1,$2) ON CONFLICT DO NOTHING",v,manifest[v]);err!=nil{return nil,err}}
 p,err:=goose.NewProvider(goose.DialectPostgres,db,source,goose.WithDisableVersioning(true),goose.WithDisableGlobalRegistry(true));if err!=nil{return nil,err};return p.Up(ctx)
}
'''
    elif which == "nontransactional":
        replace(state,"store/migrate.go",'"context"','"context"\n "testing/fstest"')
        replace(state,"store/migrate.go",'manifest, err := loadManifest(source)','source=withoutTransactions(source)\n manifest, err := loadManifest(source)')
        replace(state,"store/migrate.go",'case "StatementBegin", "StatementEnd":','case "StatementBegin", "StatementEnd", "NO TRANSACTION":')
        replace(state,"store/ledger.go",'if _, ok := db.(*sql.Tx); !ok {','if _, ok := db.(*sql.Tx); !ok && false {')
        state["store/migrate.go"] += '''
func withoutTransactions(source fs.FS) fs.FS {
 result:=fstest.MapFS{};fs.WalkDir(source,".",func(name string,d fs.DirEntry,e error)error{if e!=nil{return e};if d.IsDir(){return nil};b,e:=fs.ReadFile(source,name);if e!=nil{return e};if strings.HasSuffix(name,".sql")&&!strings.Contains(name,"/"){b=append(b,[]byte("\\n-- +goose NO TRANSACTION\\n")...)};result[name]=&fstest.MapFile{Data:b};return nil});return result
}
'''
    else:
        raise RuntimeError("unknown mutation")
    for path,text in state.items():
        (module/path).write_text(text)

signal.signal(signal.SIGTERM, lambda *_: sys.exit(143))
controls = [("premature-ack","TestPGFoundationMigrations"),("nontransactional","TestPGFoundationMigrations"),("locking","TestPGFoundationConcurrent"),("checksum","TestPGFoundationCompatibility"),("version","TestPGFoundationCompatibility"),("schema","TestPGFoundationSchema"),("ambient","TestPGFoundationNoFallback"),("fixture-owner","TestPGFoundationFixture")]
try:
    # A failing baseline cannot qualify any mutant.
    baseline = subprocess.run(["go","test","-count=1","-timeout=45s","-run","^TestPGFoundation","./store"],cwd=module,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=55)
    if baseline.returncode:
        print(baseline.stdout); raise SystemExit("baseline failed")
    for name,test in controls:
        for path,b in original.items(): (module/path).write_bytes(b)
        mutate(name)
        result = run(test)
        if not result.returncode:
            raise SystemExit("SURVIVED: " + name)
        if "[build failed]" in result.stdout or "build failed" in result.stdout or "[setup failed]" in result.stdout or "undefined:" in result.stdout:
            print(result.stdout);raise SystemExit("INVALID (compile/setup failure): " + name)
        if "--- FAIL: " + test not in result.stdout:
            print(result.stdout);raise SystemExit("INVALID (named failure absent): " + name)
        print("KILLED: " + name + " -> " + test,flush=True)
    print("PASS: all8 production-path fault controls killed; original bytes restored",flush=True)
finally:
    for path,b in original.items(): (module/path).write_bytes(b)
