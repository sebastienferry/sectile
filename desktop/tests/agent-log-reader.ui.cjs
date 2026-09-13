const {test}=require('node:test')
const assert=require('node:assert/strict')
const fs=require('node:fs/promises'),os=require('node:os'),path=require('node:path')
const {readAgentLog,MAX_LOG_BYTES}=require('../electron/agent-log.cjs')

async function fixture(t){
 const root=await fs.mkdtemp(path.join(os.tmpdir(),'sectile-agent-log-'))
 t.after(()=>fs.rm(root,{recursive:true,force:true}))
 return path.join(root,'agent.log')
}

test('agent log snapshots handle missing, empty, appended, truncated and replaced files',async t=>{
 const file=await fixture(t)
 assert.deepEqual(await readAgentLog(file),{path:file,text:'',missing:true,truncated:false})
 await fs.writeFile(file,'')
 assert.deepEqual(await readAgentLog(file),{path:file,text:'',missing:false,truncated:false})
 await fs.appendFile(file,'First\n')
 assert.equal((await readAgentLog(file)).text,'First\n')
 await fs.appendFile(file,'Second\n')
 assert.equal((await readAgentLog(file)).text,'First\nSecond\n')
 await fs.truncate(file,0)
 assert.equal((await readAgentLog(file)).text,'')
 await fs.rename(file,file+'.old')
 await fs.writeFile(file,'Replacement\n')
 assert.equal((await readAgentLog(file)).text,'Replacement\n')
})

test('agent log reads only the recent bounded tail and preserves literal contents',async t=>{
 const file=await fixture(t)
 const tail='x'.repeat(MAX_LOG_BYTES-40)+'<script>literal</script>\x1b[31m\n'
 await fs.writeFile(file,'earlier'.repeat(MAX_LOG_BYTES)+tail)
 const snapshot=await readAgentLog(file)
 assert.equal(snapshot.truncated,true)
 assert.equal(Buffer.byteLength(snapshot.text),MAX_LOG_BYTES)
 assert.ok(snapshot.text.endsWith(tail))
 await fs.writeFile(file,'x'.repeat(MAX_LOG_BYTES))
 assert.equal((await readAgentLog(file)).truncated,false)
})

test('agent log tail skips partial UTF-8 prefixes',async t=>{
 const file=await fixture(t)
 for(const character of ['é','€','😀']){
  for(let cut=1;cut<Buffer.byteLength(character);cut++){
   const tail='z'.repeat(MAX_LOG_BYTES-(Buffer.byteLength(character)-cut))
   await fs.writeFile(file,character+tail)
   const snapshot=await readAgentLog(file)
   assert.equal(snapshot.text,tail)
   assert.equal(snapshot.truncated,true)
  }
 }
})

test('agent log rejects directories and symlinks without reading another file',async t=>{
 const file=await fixture(t)
 await fs.mkdir(file)
 await assert.rejects(readAgentLog(file),/not a regular file/)
 await fs.rmdir(file)
 await fs.writeFile(file+'.private','Do not read')
 await fs.symlink(file+'.private',file)
 await assert.rejects(readAgentLog(file))
})
