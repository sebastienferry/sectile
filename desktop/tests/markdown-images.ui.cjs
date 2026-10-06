const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

const base64=text=>Buffer.from(text).toString('base64')
// A 1×1 PNG, a wide and a small SVG.
const dot='iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=='
const wide=base64('<svg xmlns="http://www.w3.org/2000/svg" width="3000" height="30"><rect width="3000" height="30" fill="teal"/></svg>')
const small=base64('<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20"><script>window.hostile=true</script><image href="https://example.com/x.png" width="40" height="20"/></svg>')

const guide=`# Guide

![Dot](images/dot.png "A single dot") ![Wide](wide.svg) ![Small](/web/small.svg)

![Missing](missing.png) ![Broken](broken.png) ![Out](../../outside.png) ![Remote](https://example.com/remote.png)

[![Badge](images/dot.png)](https://example.com/badge)
`

test('Changes shows the repository images a rendered document references',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-markdown-images-ui-'))
 let capabilities=['git-diff','markdown-documents','markdown-images']
 const runs=[{id:'a',taskId:'a',taskKey:'#a',projectId:'project',skill:'implement',status:'completed',directory:'/tmp/a',sessionId:'a'}]
 const text=(path,document)=>({path,status:'modified',kind:'text',additions:1,deletions:0,patch:'@@ -0,0 +1 @@\n+raw '+path,document})
 const files=[
  text('docs/big.md',{side:'new',content:'![Big](big.png)',images:[{path:'docs/big.png',image:'new:docs/big.png'}]}),
  text('docs/guide.md',{side:'new',content:guide,images:[
   {path:'docs/images/dot.png',image:'new:docs/images/dot.png'},
   {path:'docs/wide.svg',image:'new:docs/wide.svg'},
   {path:'web/small.svg',image:'new:web/small.svg'},
   {path:'docs/missing.png',omittedReason:'Image not found in the inspected state.'},
   {path:'docs/broken.png',image:'new:docs/broken.png'},
  ]}),
 ]
 const images={
  'new:docs/images/dot.png':{mimeType:'image/png',data:dot},
  'new:docs/wide.svg':{mimeType:'image/svg+xml',data:wide},
  'new:web/small.svg':{mimeType:'image/svg+xml',data:small},
  'new:docs/broken.png':{mimeType:'image/png',data:base64('not a png')},
  // About 10 MiB of Base64, the size of a full image budget.
  'new:docs/big.png':{mimeType:'image/png',data:Buffer.alloc(7.5*(1<<20),7).toString('base64')},
 }
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/project'}]));return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end(JSON.stringify(runs.map(r=>({id:r.taskId,key:r.taskKey,title:'Task '+r.taskId,labels:['#specified']}))));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/git-diff?')){
   const result={runId:'a',taskId:'a',directory:'/tmp/a',branch:'feat/a',baseRef:'refs/remotes/origin/main',mergeBase:'1234567',generatedAt:'2026-10-06T12:00:00Z',complete:true,countsPartial:false,isClean:false,filesChanged:files.length,additions:0,deletions:0,warnings:[],files}
   // An older agent sends documents without images.
   if(capabilities.includes('markdown-images'))result.images=images
   else result.files=files.map(f=>({...f,document:{side:f.document.side,content:f.document.content}}))
   res.end(JSON.stringify(result));return
  }
  res.writeHead(404).end()
 })
 const ws=new WebSocketServer({server});ws.on('connection',socket=>socket.send('console\r\n'))
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env});const page=await app.firstWindow();page.setDefaultTimeout(8000)
  const requests=[];page.on('request',request=>{if(!request.url().startsWith('file:'))requests.push(request.url())})
  await app.evaluate(({shell})=>{globalThis.opened=[];shell.openExternal=async url=>{globalThis.opened.push(url)}})
  assert.equal(await page.evaluate(()=>document.querySelector('meta[http-equiv="Content-Security-Policy"]').content),"default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'none'; img-src 'self' data:")
  await page.locator('.run').filter({hasText:'Task a'}).click()
  await page.getByRole('button',{name:'Changes',exact:true}).click();await expect(page.locator('.diff-status')).toHaveText('Changes loaded.')
  // The first file carries about 10 MiB of image data and still renders; its
  // bytes are not an image, so it falls back once the window fails to decode it.
  const view=page.locator('.diff-rendered')
  await expect(view.locator('.md-image')).toHaveText('[Big] (big.png)')
  await expect(view.locator('.md-image')).toHaveAttribute('title','This image could not be displayed.')
  await page.getByRole('combobox',{name:'Changed file'}).selectOption({label:'docs/guide.md · modified'})
  const picture=name=>view.getByRole('img',{name,exact:true})
  await expect(picture('Dot').first()).toHaveAttribute('title','A single dot')
  await expect.poll(()=>picture('Dot').first().evaluate(img=>img.complete&&img.naturalWidth)).toBe(1)
  assert.equal(await picture('Dot').first().evaluate(img=>img.width),1)
  // A wide image is scaled down to the view, a small one keeps its size.
  await expect.poll(()=>picture('Wide').evaluate(img=>img.complete&&img.naturalWidth)).toBe(3000)
  const sizes=await view.evaluate(view=>{const img=view.querySelector('img[alt="Wide"]');return {image:img.getBoundingClientRect().width,view:view.clientWidth}})
  assert.ok(sizes.image<=sizes.view&&sizes.image<3000,JSON.stringify(sizes))
  await expect.poll(()=>picture('Small').evaluate(img=>img.complete&&img.getBoundingClientRect().width)).toBe(40)
  // Images that are not shown keep their alt text, with the reason if any.
  const fallback=view.locator('.md-image')
  await expect(fallback).toHaveText(['[Missing] (missing.png)','[Broken] (broken.png)','[Out] (../../outside.png)','[Remote] (https://example.com/remote.png)'])
  assert.deepEqual(await fallback.evaluateAll(spans=>spans.map(span=>span.getAttribute('title'))),['Image not found in the inspected state.','This image could not be displayed.','This image path leaves the repository.',null])
  // An image is a link only when the Markdown wraps it in one.
  await view.getByRole('link').filter({has:page.getByRole('img',{name:'Badge',exact:true})}).click()
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual(['https://example.com/badge'])
  assert.equal(await picture('Dot').first().evaluate(img=>img.closest('a,[role=link]')),null)
  // Nothing is fetched and the SVG ran nothing.
  await page.waitForTimeout(150)
  assert.deepEqual(requests,[]);assert.equal(await page.evaluate(()=>window.hostile),undefined)
  // An agent without the capability: every image is its alt text, with no reason.
  capabilities=['git-diff','markdown-documents'];await page.getByRole('button',{name:'Refresh',exact:true}).click();await expect(page.locator('.diff-status')).toHaveText('Changes loaded.')
  await expect(view.locator('h1')).toHaveText('Guide')
  assert.equal(await view.locator('img').count(),0)
  await expect(fallback).toHaveCount(8)
  assert.deepEqual(await fallback.evaluateAll(spans=>spans.filter(span=>span.hasAttribute('title')).length),0)
 }finally{if(app)await app.close();ws.close();await new Promise(resolve=>server.close(resolve))}
})
