const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

test('Changes preserves console state and rejects obsolete responses',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-diff-ui-'))
 let attachments=0,detaches=0,input='',mode='normal',supported=true,pending
 const runs=['a','b'].map(id=>({id,taskId:id,taskKey:'#'+id,projectId:'project',skill:'implement',status:id==='a'?'running':'completed',directory:'/tmp/'+id,sessionId:id}))
 const payload=id=>({runId:id,taskId:id,directory:'/tmp/'+id,branch:'feat/'+id,baseRef:'refs/remotes/origin/main',mergeBase:'1234567',generatedAt:'2026-09-13T12:00:00Z',complete:true,countsPartial:false,isClean:false,filesChanged:2,additions:1,deletions:0,warnings:[],files:[{path:'<img src=x onerror=alert(1)>.txt',status:'added',kind:'text',additions:1,deletions:0,patch:'@@ -0,0 +1 @@\n+<script>window.hostile=true</script>'},{path:'binary.dat',status:'added',kind:'binary',additions:null,deletions:null,patch:'',omittedReason:'Binary contents are not displayed.'}]})
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities:supported?['git-diff']:[]}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/project'}]));return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end(JSON.stringify(runs.map(r=>({id:r.taskId,key:r.taskKey,title:'Task '+r.taskId,labels:['#specified']}))));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/git-diff?')){
   const id=new URL(req.url,'http://localhost').searchParams.get('id')
   if(mode==='delay'&&id==='a'){pending=()=>res.end(JSON.stringify(payload(id)));return}
   if(mode==='error'){res.writeHead(409).end(JSON.stringify({error:{code:'checkout_changed',message:'Checkout changed. Refresh to retry.'}}));return}
   const data=payload(id)
   if(mode==='empty'){data.files=[];data.filesChanged=0;data.additions=0;data.isClean=true}
   if(mode==='partial'){data.complete=false;data.countsPartial=true;data.warnings=[{message:'Only a prefix is shown.'}]}
   res.end(JSON.stringify(data));return
  }
  res.writeHead(404).end()
 })
 const ws=new WebSocketServer({server});ws.on('connection',socket=>{attachments++;socket.send('console remains alive\r\n');socket.on('close',()=>detaches++);socket.on('message',raw=>{const msg=JSON.parse(raw);if(msg.type==='input')input+=msg.data})})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env});const page=await app.firstWindow();page.setDefaultTimeout(8000)
  await page.locator('.run').filter({hasText:'Task a'}).click();await expect.poll(()=>attachments).toBeGreaterThan(0)
  const initialAttachments=attachments,initialDetaches=detaches
  await page.getByRole('button',{name:'Changes',exact:true}).click();await expect(page.locator('.diff-status')).toHaveText('Changes loaded.')
  await expect(page.locator('.diff-patch')).toContainText('<script>window.hostile=true</script>')
  assert.equal(await page.locator('#changes script,#changes img').count(),0);assert.equal(await page.evaluate(()=>window.hostile),undefined)
  await page.getByRole('button',{name:'binary.dat · added',exact:true}).click();await expect(page.locator('.diff-file-info')).toContainText('Binary contents are not displayed.')
  await page.getByRole('button',{name:'Refresh',exact:true}).focus();await page.keyboard.press('Enter');await expect(page.locator('.diff-status')).toHaveText('Changes loaded.')
  await expect(page.getByRole('button',{name:'binary.dat · added',exact:true})).toHaveAttribute('aria-pressed','true')
  assert.equal(input,'');assert.equal(attachments,initialAttachments);assert.equal(detaches,initialDetaches)
  mode='error';await page.getByRole('button',{name:'Refresh',exact:true}).click();await expect(page.locator('.diff-error')).toContainText('Checkout changed');assert.equal(await page.locator('.diff-files button').count(),0)
  mode='empty';await page.getByRole('button',{name:'Refresh',exact:true}).click();await expect(page.locator('.diff-summary')).toHaveText('No changes compared with baseline')
  mode='partial';await page.getByRole('button',{name:'Refresh',exact:true}).click();await expect(page.locator('.diff-summary')).toContainText('partial totals')
  await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].setSize(700,650));await page.screenshot({path:path.join(root,'changes-narrow.png')})
  assert.equal(await page.locator('#changes').evaluate(e=>e.scrollWidth<=e.clientWidth),true)
  mode='delay';await page.getByRole('button',{name:'Refresh',exact:true}).click();await expect.poll(()=>Boolean(pending)).toBe(true)
  await page.locator('.run').filter({hasText:'Task b'}).click();await expect(page.locator('.diff-context')).toContainText('feat/b');pending();await page.waitForTimeout(150);await expect(page.locator('.diff-context')).toContainText('feat/b')
  supported=false;await page.getByRole('button',{name:'Refresh',exact:true}).click();await expect(page.locator('.diff-error')).toContainText('Update and restart the local agent')
  supported=true;mode='normal';await page.getByRole('button',{name:'Refresh',exact:true}).click();await expect(page.locator('.diff-status')).toHaveText('Changes loaded.')
  const beforeConsole=attachments;await page.getByRole('button',{name:'Console',exact:true}).click();await expect(page.locator('#terminal')).toBeVisible();assert.equal(attachments,beforeConsole)
  assert.ok(await page.locator('.xterm-helper-textarea').evaluate(e=>e===document.activeElement))
  console.log('Changes screenshot: '+path.join(root,'changes-narrow.png'))
 }finally{if(app)await app.close();ws.close();await new Promise(resolve=>server.close(resolve))}
})
