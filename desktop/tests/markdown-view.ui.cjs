const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

const hostile=`# Guide

Some *emphasis* and a table:

| Name | Value |
|:-----|------:|
| one  | 1     |

- [x] done
- [ ] todo

<script>window.hostile=true</script>
<img src=x onerror="window.hostile=true">
<iframe src="https://example.com"></iframe>

[web](https://example.com/) [mail](mailto:someone@example.com) [plan](../plan.md) [anchor](#usage) [script](javascript:window.hostile=true)

![diagram](docs/x.png) ![remote](https://example.com/x.png)
`

test('Changes renders Markdown files by default, safely, with the raw diff chosen per execution',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-markdown-ui-'))
 let capabilities=['git-diff','markdown-documents']
 const runs=['a','b'].map(id=>({id,taskId:id,taskKey:'#'+id,projectId:'project',skill:'implement',status:'completed',directory:'/tmp/'+id,sessionId:id}))
 const text=(path,status,document,extra={})=>({path,status,kind:'text',additions:1,deletions:0,patch:'@@ -0,0 +1 @@\n+raw '+path,document,...extra})
 const files={
  a:[
   text('docs/guide.md','modified',{side:'new',content:hostile}),
   text('README.MARKDOWN','added',{side:'new',content:'# Upper case readme'}),
   text('big.md','added',{side:'new',omittedReason:'File exceeds the 512 KiB rendering limit.'}),
   {path:'bin.md',status:'added',kind:'binary',additions:null,deletions:null,patch:'',omittedReason:'Binary contents are not displayed.'},
   text('gone.md','deleted',{side:'old',content:'# Old title'}),
   text('main.go','modified'),
   text('page.mdx','added'),
  ],
  b:[text('other.md','added',{side:'new',content:'# Other execution'})],
 }
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/project'}]));return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end(JSON.stringify(runs.map(r=>({id:r.taskId,key:r.taskKey,title:'Task '+r.taskId,labels:['#specified']}))));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/git-diff?')){
   const id=new URL(req.url,'http://localhost').searchParams.get('id')
   const listed=files[id].map(f=>{const copy={...f};if(!capabilities.includes('markdown-documents'))delete copy.document;return copy})
   res.end(JSON.stringify({runId:id,taskId:id,directory:'/tmp/'+id,branch:'feat/'+id,baseRef:'refs/remotes/origin/main',mergeBase:'1234567',generatedAt:'2026-10-01T12:00:00Z',complete:true,countsPartial:false,isClean:false,filesChanged:listed.length,additions:0,deletions:0,warnings:[],files:listed}));return
  }
  res.writeHead(404).end()
 })
 const ws=new WebSocketServer({server});ws.on('connection',socket=>socket.send('console\r\n'))
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 const launch=async()=>{
  const app=await electron.launch({args:[path.resolve(__dirname,'..')],env});const page=await app.firstWindow();page.setDefaultTimeout(8000)
  await app.evaluate(({shell})=>{globalThis.opened=[];shell.openExternal=async url=>{globalThis.opened.push(url)}})
  await page.locator('.run').filter({hasText:'Task a'}).click()
  await page.getByRole('button',{name:'Changes',exact:true}).click();await expect(page.locator('.diff-status')).toHaveText('Changes loaded.')
  return {app,page}
 }
 let app
 try{
  let page
  ;({app,page}=await launch())
  const toggle=page.locator('.diff-render-toggle'),note=page.locator('.diff-render-note'),view=page.locator('.diff-rendered'),patch=page.locator('.diff-patch')
  const pick=name=>page.locator('.diff-files button').filter({hasText:name}).click()
  // A Markdown file opens rendered, with the toggle pressed; it turns back to the raw diff.
  await pick('docs/guide.md · modified')
  await expect(toggle).toBeVisible();await expect(toggle).toHaveAttribute('aria-pressed','true');await expect(toggle).toBeEnabled()
  await expect(patch).toBeHidden();await expect(view).toBeVisible()
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-pressed','false');await expect(patch).toContainText('+raw docs/guide.md');await expect(view).toBeHidden()
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-pressed','true');await expect(patch).toBeHidden();await expect(view).toBeVisible()
  await expect(view.locator('h1')).toHaveText('Guide');await expect(view.locator('em')).toHaveText('emphasis')
  await expect(view.locator('table th')).toHaveText(['Name','Value']);await expect(view.locator('td').nth(1)).toHaveCSS('text-align','right')
  assert.deepEqual(await view.locator('input[type=checkbox]').evaluateAll(boxes=>boxes.map(b=>[b.checked,b.disabled])),[[true,true],[false,true]])
  await expect(page.locator('.diff-file-info')).toContainText('docs/guide.md · modified · text')
  // Raw HTML is text, nothing loads, nothing carries an href.
  await expect(view).toContainText('<script>window.hostile=true</script>');await expect(view).toContainText('<iframe src="https://example.com"></iframe>')
  assert.equal(await page.locator('#changes img,#changes script,#changes iframe,#changes object,#changes embed').count(),0)
  assert.equal(await view.locator('[href],[src]').count(),0)
  await expect(view.locator('.md-image')).toHaveText(['[diagram] (docs/x.png)','[remote] (https://example.com/x.png)'])
  // Web and mail links open outside; every other link is inert and keeps the window.
  const url=page.url()
  await view.getByRole('link',{name:'web'}).click()
  await view.getByRole('link',{name:'mail'}).focus();await page.keyboard.press('Enter')
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual(['https://example.com/','mailto:someone@example.com'])
  for(const label of ['plan','anchor','script'])await view.locator('.md-link-inert').filter({hasText:label}).click()
  await expect(view.locator('.md-link-target')).toHaveText([' (../plan.md)',' (#usage)',' (javascript:window.hostile=true)'])
  await page.waitForTimeout(150)
  assert.deepEqual(await app.evaluate(()=>globalThis.opened),['https://example.com/','mailto:someone@example.com'])
  assert.equal(page.url(),url);assert.equal(await page.evaluate(()=>window.hostile),undefined)
  // Opening a link that is not web or mail is refused by the main process too.
  await assert.rejects(page.evaluate(()=>window.localAgent.openLink('file:///etc/passwd')))
  await assert.rejects(page.evaluate(()=>window.localAgent.openLink('https://user:secret@example.com/')))
  // The choice follows the reader across Markdown files, and skips other files.
  await pick('main.go · modified');await expect(toggle).toBeHidden();await expect(patch).toContainText('+raw main.go');await expect(view).toBeHidden()
  await pick('page.mdx · added');await expect(toggle).toBeHidden();await expect(patch).toBeVisible()
  await pick('bin.md · added');await expect(toggle).toBeHidden()
  await pick('README.MARKDOWN · added');await expect(toggle).toHaveAttribute('aria-pressed','true');await expect(view.locator('h1')).toHaveText('Upper case readme')
  await pick('big.md · added')
  await expect(toggle).toBeDisabled();await expect(toggle).toHaveAttribute('aria-pressed','false');await expect(note).toHaveText('File exceeds the 512 KiB rendering limit.');await expect(patch).toContainText('+raw big.md');await expect(view).toBeHidden()
  await pick('gone.md · deleted')
  await expect(toggle).toHaveAttribute('aria-pressed','true');await expect(note).toHaveText('Old version: this file is deleted.');await expect(view.locator('h1')).toHaveText('Old title')
  await page.getByRole('button',{name:'Refresh',exact:true}).click();await expect(page.locator('.diff-status')).toHaveText('Changes loaded.')
  await expect(view.locator('h1')).toHaveText('Old title');await expect(toggle).toHaveAttribute('aria-pressed','true')
  // The raw diff chosen on one execution is forgotten when another is selected.
  await toggle.click();await expect(toggle).toHaveAttribute('aria-pressed','false')
  await page.locator('.run').filter({hasText:'Task b'}).click();await expect(page.locator('.diff-context')).toContainText('feat/b')
  await expect(view.locator('h1')).toHaveText('Other execution');await expect(toggle).toHaveAttribute('aria-pressed','true')
  await toggle.click();await expect(toggle).toHaveAttribute('aria-pressed','false');await expect(patch).toContainText('+raw other.md');await expect(view).toBeHidden()
  await toggle.click();await expect(view.locator('h1')).toHaveText('Other execution')
  await view.focus();await expect(view).toBeFocused()
  // An agent without Markdown contents keeps the raw diff and says why.
  capabilities=['git-diff'];await page.getByRole('button',{name:'Refresh',exact:true}).click();await expect(page.locator('.diff-status')).toHaveText('Changes loaded.')
  await expect(toggle).toBeDisabled();await expect(note).toHaveText('Update and restart the local agent to render Markdown.');await expect(patch).toContainText('+raw other.md')
  capabilities=['git-diff','markdown-documents']
  await app.close();app=null
  // The choice is not persisted: a new Desktop session starts rendered.
  ;({app,page}=await launch())
  await page.locator('.diff-files button').filter({hasText:'docs/guide.md · modified'}).click()
  await expect(page.locator('.diff-render-toggle')).toHaveAttribute('aria-pressed','true');await expect(page.locator('.diff-rendered h1')).toHaveText('Guide')
 }finally{if(app)await app.close();ws.close();await new Promise(resolve=>server.close(resolve))}
})
