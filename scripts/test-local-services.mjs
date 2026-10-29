// Exercise the Go contracts used by native onboarding with disposable data.
// No real tracker credentials or repositories are used.
// Opt-in SECTILE_TEST_REAL_CODEX / SECTILE_TEST_REAL_CLAUDE use signed-in CLIs.
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, mkdir, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'node:http';
import { randomUUID } from 'node:crypto';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const binaries = resolve(process.env.SECTILE_TEST_BINARIES_DIR || join(root,'src-tauri/binaries'));
const data = await mkdtemp(join(tmpdir(), 'sectile-onboarding-'));
const children = [];
const api = createServer((req, res) => { res.setHeader('Content-Type','application/json'); res.end(JSON.stringify({ id: 1, login: 'fixture', name: 'Fixture', username: 'fixture' })); });
await new Promise(resolve => api.listen(0, '127.0.0.1', resolve));
const trackerSite = `http://127.0.0.1:${api.address().port}`;
const runtimeBinary = join(binaries, `sectile-server${process.platform==='win32'?'.exe':''}`);
const discovery = spawnSync(runtimeBinary,['--local-runtime-info'],{encoding:'utf8',windowsHide:true});
assert.equal(discovery.status,0,'Build the Go services before testing runtime discovery');
const runtimeInfo = JSON.parse(discovery.stdout);
const port = runtimeInfo.port;
const url = runtimeInfo.origin;
const reservation = createServer();
await new Promise((resolve, reject) => {
  reservation.once('error', () => reject(new Error('Stop the desktop local services before running the isolated services test: its fixed port is occupied.')));
  reservation.listen(port,'127.0.0.1',resolve);
});
await new Promise(resolve => reservation.close(resolve));
const key = randomUUID();
await mkdir(join(data,'server')); await mkdir(join(data,'agent')); await mkdir(join(data,'repo'));
const realCodex = process.env.SECTILE_TEST_REAL_CODEX === "1";
const realClaude = process.env.SECTILE_TEST_REAL_CLAUDE === "1";
const env = Object.fromEntries(['PATH','SYSTEMROOT','WINDIR','TMP','TEMP'].filter(k => process.env[k]).map(k=>[k,process.env[k]]));
env.HOME = data; env.USERPROFILE = data;
function start(name, args, extra) {
  const child = spawn(join(binaries, `sectile-${name}${process.platform==='win32'?'.exe':''}`), args, {cwd:data,env:{...env,...extra},stdio:'ignore',windowsHide:true});
  children.push(child); return child;
}
async function until(action) { let last; for(let i=0;i<150;i++){ try{return await action();}catch(e){last=e; await new Promise(r=>setTimeout(r,200));}} throw last; }
let token = '';
async function request(route, method='GET', body, headers={}) {
  const response = await fetch(url+route,{method,headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json',...headers},body:body===undefined?undefined:JSON.stringify(body),signal:AbortSignal.timeout(15000)});
  if(!response.ok) throw new Error(`${method} ${route}: HTTP ${response.status}`);
  return response.status===204?null:response.json();
}
try {
  const serverEnv = {PORT:String(port),SECTILE_SERVER_HOST:'127.0.0.1',SECTILE_SERVER_DATA_DIR:join(data,'server'),DB_PATH:join(data,'server/tasks.db'),SECTILE_LOCAL_RUNTIME_KEY:key};
  const server = start('server',[],serverEnv);
  await until(()=>request('/api/local-runtime','GET',undefined,{Authorization:`Bearer ${key}`}));
  assert.equal((await fetch(url+'/api/local-runtime')).status,404);
  const signIn = await fetch(url+'/auth/local',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({email:'fixture@example.invalid'})});
  assert.equal(signIn.status,200);
  const cookie = signIn.headers.get('set-cookie').split(';')[0];
  const code = await request('/api/pairing-codes','POST',{}, {Cookie:cookie});
  const pair = await request('/api/v1/agent/pair','POST',{code:code.code,label:'Disposable integration fixture'});
  token=pair.token;
  const profile = await request('/api/me','PATCH',{displayName:'Fixture User'});
  assert.equal(profile.displayName,'Fixture User'); assert.equal(profile.role,'admin');
  await request('/api/me/tracker-credentials','PUT',{tracker:'gitlab',siteUrl:trackerSite,email:'fixture@example.invalid',token:'disposable-fixture-token'});
  // Test the native-to-browser handoff without contacting a real provider.
  await request('/api/admin/github-oauth','PUT',{clientId:'fixture-client',clientSecret:'fixture-oauth-secret',redirectUrl:url+'/auth/github/callback'});
  const appInfo = await request('/api/admin/github-oauth');
  assert.equal(appInfo.configured,true);
  assert.ok(!JSON.stringify(appInfo).includes('fixture-oauth-secret'));
  const flow = await request('/api/desktop-oauth','POST',{provider:'github'});
  const browserStart = await fetch(url+flow.startPath,{redirect:'manual'});
  assert.equal(browserStart.status,302);
  assert.equal(new URL(browserStart.headers.get('location')).hostname,'github.com');
  assert.ok(browserStart.headers.get('set-cookie').startsWith('sectile_desktop_github='));
  assert.equal((await fetch(url+flow.startPath,{redirect:'manual'})).status,410);
  assert.equal((await request('/api/desktop-oauth?id='+flow.id)).status,'waiting');
  await request('/api/desktop-oauth?id='+flow.id,'DELETE');
  assert.equal((await request('/api/desktop-oauth?id='+flow.id)).status,'cancelled');
  const credentials = await request('/api/me/tracker-credentials');
  assert.equal(credentials.credentials[0].tracker,'gitlab');
  assert.ok(!JSON.stringify(credentials).includes('disposable-fixture-token'));
  const tracker = await request('/api/admin/trackers','POST',{name:'Fixture tracker',provider:'gitlab',site:trackerSite,scope:'fixture/repository',autoSyncEnabled:false});
  const project = await request('/api/projects','POST',{name:'Fixture project',trackers:[{trackerId:tracker.id}],defaultTrackerId:tracker.id});
  assert.ok(project.id);
  assert.equal(spawnSync('git',['init',join(data,'repo')],{stdio:'ignore',windowsHide:true}).status,0);
  start('agent',['--url',url,'--project','all','--desktop-info',join(data,'agent-connection.json')],{TOKEN:token,SECTILE_PAIRED_DEVICE_ID:pair.deviceId,SECTILE_DESKTOP_DATA_DIR:join(data,'agent'),SECTILE_DISABLE_MCP_REFRESH:'1',...((realCodex || realClaude) ? {HOME:process.env.HOME,USER:process.env.USER,USERPROFILE:process.env.USERPROFILE ?? process.env.HOME,...(process.env.CODEX_HOME ? {CODEX_HOME:process.env.CODEX_HOME} : {})} : {})});
  const connection = await until(async()=>JSON.parse(await readFile(join(data,'agent-connection.json'),'utf8')));
  async function agent(route,method='GET',body) {
    const response = await fetch(connection.url+route,{method,headers:{Authorization:`Bearer ${connection.token}`,'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body),signal:AbortSignal.timeout(15000)});
    assert.ok(response.ok,`Agent ${route}: HTTP ${response.status}`);
    return response.status===204?null:response.json();
  }
  await until(async()=>{const status=await agent('/desktop/status'); assert.equal(status.server,url); assert.equal(status.connected,true);});
  await agent('/desktop/projects','POST',{projectId:project.id,path:join(data,'repo')});
  const folderProject = await request('/api/projects','POST',{name:'Folder-only project',trackers:[],folderOnly:true});
  assert.ok(folderProject.id);
  assert.equal(folderProject.trackers?.length ?? 0, 0);
  const plainFolder = join(data,'plain-folder');
  await mkdir(plainFolder,{recursive:true});
  await agent('/desktop/projects','POST',{projectId:folderProject.id,path:plainFolder,folderOnly:true});
  const beforeAttachment = await request('/api/projects/'+folderProject.id);
  const attached = await request('/api/projects/'+folderProject.id,'PUT',{
    trackers:[...beforeAttachment.trackers.map(item=>({trackerId:item.trackerId})),{trackerId:tracker.id}],
    defaultTrackerId:tracker.id,
  });
  assert.equal(attached.defaultTrackerId,tracker.id);
  assert.deepEqual(attached.trackers.map(item=>item.trackerId),[...beforeAttachment.trackers.map(item=>item.trackerId),tracker.id]);
  assert.equal((await agent('/desktop/projects')).find(item=>item.id===folderProject.id)?.path,plainFolder);
  const conversation = await agent('/desktop/consoles','POST',{projectId:folderProject.id,provider:'claude',view:'conversation'});
  assert.ok(conversation.id);
  assert.equal(conversation.directory,plainFolder);
  if (realCodex) {
    const chat = await agent('/desktop/consoles','POST',{projectId:folderProject.id,provider:'codex',view:'conversation'});
    const route = '/desktop/conversation?id='+chat.id;
    await agent(route,'POST',{message:'This is an isolated interruption test. Only in the current folder: use a shell tool to execute exactly: printf STARTED > started-marker.txt; sleep 90; printf FAILED > delayed-marker.txt . Wait for this command to finish. Do not run other commands or inspect other folders.'});
    await until(async()=>{
      const current=await agent(route);
      assert.ok(current.busy);
      assert.ok((current.events??[]).some(line=>{const event=JSON.parse(line);return event.kind==='tool' && JSON.stringify(event).includes('sleep 90')}));
    });
    await until(async()=>assert.equal(await readFile(join(plainFolder,'started-marker.txt'),'utf8'),'STARTED'));
    await agent(route,'POST',{interrupt:true});
    await until(async()=>assert.equal((await agent(route)).busy,false));
    console.log('Real Codex interrupted; waiting 95 seconds to detect a surviving tool.');
    await new Promise(resolve=>setTimeout(resolve,95000));
    await assert.rejects(readFile(join(plainFolder,'delayed-marker.txt')), error=>error.code==='ENOENT');
    console.log('PASS: delayed marker absent after 95 seconds.');
    await agent(route,'POST',{message:'Resume this interruption test. Use a shell tool to verify delayed-marker.txt does not exist in this folder. If absent, reply exactly RESUME_OK. Do not write any files or inspect other folders.'});
    await until(async()=>{
      const current=await agent(route);
      assert.equal(current.busy,false);
      assert.ok((current.events??[]).some(line=>{const event=JSON.parse(line);return event.kind==='assistant' && event.text.includes('RESUME_OK')}));
    });
    await agent('/desktop/stop?id='+chat.id,'POST');
    console.log('PASS: the same Codex conversation resumed and confirmed RESUME_OK.');
  }
  if (realClaude) {
    const chat = await agent('/desktop/consoles','POST',{projectId:folderProject.id,provider:'claude',view:'conversation'});
    const route = '/desktop/conversation?id='+chat.id;
    const first = 'printf SECTILE_CLAUDE_OK > probe.txt; cat probe.txt';
    const delayed = 'printf STARTED > started-marker.txt; sleep 90; printf FAILED > delayed-marker.txt';
    const resumed = 'test ! -e delayed-marker.txt && cat probe.txt';
    async function currentWithApprovals(expectedCommand) {
      const current = await agent(route);
      for (const approval of current.approvals??[]) {
        assert.equal(approval.tool,'Bash','Only the test shell command may be approved');
        assert.equal(approval.input?.command?.trim(),expectedCommand,'Unexpected shell command requested');
        await agent(route,'POST',{approval:{id:approval.id,decision:'allow'}});
      }
      return current;
    }
    await agent(route,'POST',{message:'Isolated desktop test. Work only in the current folder. Execute exactly this single shell tool command, with no other commands: '+first+'. Then reply exactly SECTILE_CLAUDE_OK.'});
    await until(async()=>{
      const current=await currentWithApprovals(first);
      assert.equal(current.busy,false);
      assert.ok((current.events??[]).some(line=>{const event=JSON.parse(line);return event.kind==='assistant' && event.text.includes('SECTILE_CLAUDE_OK')}), JSON.stringify({busy:current.busy,events:(current.events??[]).map(line=>JSON.parse(line)).map(e=>({kind:e.kind,text:e.text?.slice(0,800)}))}));
    });
    assert.equal(await readFile(join(plainFolder,'probe.txt'),'utf8'),'SECTILE_CLAUDE_OK');
    const firstEvents=(await agent(route)).events.map(JSON.parse);
    assert.ok(firstEvents.some(e=>e.kind==='tool'));
    assert.ok(firstEvents.some(e=>e.kind==='tool_result' && e.text.includes('SECTILE_CLAUDE_OK')));
    console.log('PASS: real Claude response, shell call and tool result.');
    await agent(route,'POST',{message:'Interruption test. Execute exactly this single shell command in the current folder, without any other commands: '+delayed+'. Wait for it to finish before replying.'});
    await until(async()=>{
      await currentWithApprovals(delayed);
      assert.equal(await readFile(join(plainFolder,'started-marker.txt'),'utf8'),'STARTED');
    });
    await agent(route,'POST',{interrupt:true});
    await until(async()=>assert.equal((await agent(route)).busy,false));
    console.log('Real Claude interrupted after the shell start marker; waiting 95 seconds.');
    await new Promise(resolve=>setTimeout(resolve,95000));
    await assert.rejects(readFile(join(plainFolder,'delayed-marker.txt')), error=>error.code==='ENOENT');
    console.log('PASS: Claude delayed marker absent after 95 seconds.');
    await agent(route,'POST',{message:'Resume the same conversation. Execute exactly this shell command in the current folder: '+resumed+'. If the output is SECTILE_CLAUDE_OK and the delayed marker is absent, reply exactly RESUME_OK. Do not write files.'});
    await until(async()=>{
      const current=await currentWithApprovals(resumed);
      assert.equal(current.busy,false);
      assert.ok((current.events??[]).some(line=>{const event=JSON.parse(line);return event.kind==='assistant' && event.text.includes('RESUME_OK')}));
    });
    await agent('/desktop/stop?id='+chat.id,'POST');
    console.log('PASS: the same Claude conversation resumed and confirmed RESUME_OK.');
  }
  const projects = await agent('/desktop/projects');
  assert.ok(projects.some(p=>p.id===project.id && p.configured));
  const status=await agent('/desktop/status'); assert.equal(status.server,url);
  server.kill('SIGTERM');
  await new Promise(resolve=>server.once('exit',resolve));
  start('server',[],serverEnv);
  await until(()=>request('/api/local-runtime','GET',undefined,{Authorization:`Bearer ${key}`}));
  assert.equal((await request('/api/me')).displayName,'Fixture User');
  await until(async()=>assert.equal((await agent('/desktop/status')).connected,true));
  assert.ok((await request('/api/projects')).some(p=>p.id===project.id));
  const resumedProject = await request('/api/projects/'+folderProject.id);
  assert.equal(resumedProject.defaultTrackerId,tracker.id);
  assert.deepEqual(resumedProject.trackers.map(item=>item.trackerId),attached.trackers.map(item=>item.trackerId));
  assert.equal((await agent('/desktop/projects')).find(item=>item.id===folderProject.id)?.path,plainFolder);
  console.log('Passed: isolated startup, authenticated discovery, profile, pairing, credentials, tracker, existing project association, folder retention and server restart.');
} finally {
  for(const child of children.reverse()) {
    if(child.exitCode===null) { child.kill('SIGTERM'); await Promise.race([new Promise(r=>child.once('exit',r)),new Promise(r=>setTimeout(r,3000))]); if(child.exitCode===null) child.kill('SIGKILL'); }
  }
  await new Promise(resolve=>api.close(resolve));
  await rm(data,{recursive:true,force:true});
}
