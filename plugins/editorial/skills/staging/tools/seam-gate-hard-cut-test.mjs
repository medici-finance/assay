// Exercise real browser measurements, including failure controls.
import {mkdtempSync,writeFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';import {join} from 'node:path';import {fileURLToPath} from 'node:url';import {spawnSync} from 'node:child_process';
const root=mkdtempSync(join(tmpdir(),'seam-hard-cut-'));const tool=fileURLToPath(new URL('./seam-gate.mjs',import.meta.url));
const cases=[['hard-cut', 'normal', true],['hard-cut','overlap',false],['hard-cut','gap',false],['hard-cut','missing',false],['hard-cut','no-reason',false],['cut','normal',false],['unknown','normal',false],['match-cut','normal',true],['hard-cut','exact-overlap',false],['hard-cut','exact-gap',false],['hard-cut','opening',false]];
try{for(const [type,mode,pass]of cases){
 // The opening fixture agrees with its ledger: outgoing never visible, incoming visible from t=0.
 const switchAt=mode==='opening'?0:1;
 const html=`<style>.s{position:absolute;left:100px;top:100px;width:300px;height:200px;background:blue}</style><div class="s" id="a"></div><div class="s" id="b"></div><script>window.__playerReady=true;window.__renderReady=true;window.__player={pause(){},seek(t){document.getElementById('a').style.visibility=(t<${switchAt}${mode==='overlap'?'||t<1.2':mode==='exact-overlap'?'||t===1':''})?'visible':'hidden';document.getElementById('b').style.visibility=(t>=${mode==='gap'?1.2:switchAt}${mode==='exact-gap'?'&&t!==1':''})?'visible':'hidden'}};__player.seek(0);</script>`;
 const seam={id:mode,cut:mode==='opening'?0:1,type,reason:mode==='no-reason'?'':'editorial change',exit:{selector:'#a',axis:'x',dir:1},entry:{selector:mode==='missing'?'#absent':'#b',axis:'x',dir:1}};if(type==='match-cut')seam.carrier={out:'#a',in:'#b'};
 writeFileSync(join(root,'film.html'),html);writeFileSync(join(root,'seams.json'),JSON.stringify({seams:[seam]}));
 const r=spawnSync(process.execPath,[tool,'verify','--ledger',join(root,'seams.json'),'--url','http://127.0.0.1:1','--comp-url','file://'+join(root,'film.html'),'--json'],{encoding:'utf8'});
 const rows=JSON.parse(r.stdout)[0].rows;
 const openingPlanOnly=mode!=='opening'||(rows.filter(row=>row.status==='FAIL').length===1&&rows.some(row=>row.check==='hard-cut-plan'&&row.status==='FAIL'&&row.detail.includes('positive cut time')));
 if((r.status===0)!==pass||!openingPlanOnly){console.error(type,mode,r.stdout,r.stderr);process.exitCode=1;break}console.log(`PASS ${type}/${mode}: ${pass?'accepted':'rejected'}`);
}}finally{rmSync(root,{recursive:true,force:true})}
