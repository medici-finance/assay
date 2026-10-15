"""Offline planted validation failures for the release cache (regression: #138)."""
from pathlib import Path
import tempfile, subprocess, os, json
source=Path('tools/release/go_cache.py').read_text(); tests=Path('tools/release/go_cache_test.py').read_text()
cases=[('wrong-workflow',"run['path'] == WORKFLOW",'True','test_producer'),
       ('wrong-job',"j['name'] == 'release'",'True','test_producer'),
       ('wrong-artifact-id',"a['id'] == int(artifact_id)",'True','test_producer'),
       ('expired',"artifact['expired'] is False",'True','test_producer'),
       ('wrong-digest','sha256(archive) == digest','True','test_bad_archives'),
       ('path-traversal',"all(p not in ('', '.', '..') for p in parts)",'True','test_bad_archives'),
       ('producer-contract','data == Path(path).read_bytes()','True','test_source_contract')]
for name,old,new,test in cases:
 assert old in source,name
 with tempfile.TemporaryDirectory(prefix='cache-mutation-',dir=os.environ.get('TMPDIR')) as d:
  p=Path(d);(p/'go_cache.py').write_text(source.replace(old,new,1));(p/'go_cache_test.py').write_text(tests)
  r=subprocess.run(['python3','-B','-m','unittest','go_cache_test.CacheTests.'+test,'-v'],cwd=p,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=20)
  print(json.dumps({'mutation':name,'test':test,'exit':r.returncode}),flush=True)
  print(r.stdout,flush=True)
  assert r.returncode!=0,'SURVIVED: '+name
print('PASS: 7 planted guard mutations killed')
