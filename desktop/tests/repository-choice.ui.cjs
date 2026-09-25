const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// A launch the agent parked until its ticket is pinned to a repository (#456)
// is resumed from the console pane: choosing a repository pins the ticket and
// the agent starts the same execution.
test('a run waiting for its repository is resumed by choosing one',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-repository-choice-'))
 const run={id:'run-1',projectId:'p',taskId:'t1',taskKey:'#12',skill:'implement',directory:'',status:'waiting'}
 const pins=[]
 const server=http.createServer((req,res)=>{
  assert.equal(req.headers.authorization,'Bearer test-secret');res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:['repositories']}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Project',path:'/tmp/repo',configured:true}]));return}
  if(req.url==='/desktop/project?id=p'){res.end(JSON.stringify({configured:true,monoRepo:false,parallelism:1,server:{aiProvider:'claude',skills:[]}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify([run]));return}
  if(req.url==='/desktop/repositories?projectId=p'){
   res.end(JSON.stringify([{url:'git@github.com:o/a.git',identity:'github.com/o/a',path:'/tmp/repo',code:true},{url:'git@github.com:o/b.git',identity:'github.com/o/b',path:'/tmp/b',code:false}]));return
  }
  if(req.url==='/desktop/repositories'&&req.method==='POST'){
   let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{pins.push(JSON.parse(raw));run.status='preparing';res.writeHead(204).end()});return
  }
  if(req.url.startsWith('/desktop/tasks')){res.end('[]');return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  await page.locator('.run').first().click()
  const choice=page.getByRole('group',{name:'Repository of this task'})
  await choice.waitFor()
  await page.waitForFunction(()=>document.querySelectorAll('#repository-choice option').length===2)
  await choice.getByLabel('Repository').selectOption('github.com/o/b')
  await choice.getByRole('button',{name:'Start in this repository'}).click()
  await page.getByText('Repository pinned',{exact:false}).waitFor()
  assert.deepEqual(pins,[{projectId:'p',repository:'github.com/o/b',taskId:'t1'}])
  // Once the agent resumes the run, the choice has nothing left to decide.
  await page.waitForFunction(()=>document.querySelector('#repository-choice')===null)
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
