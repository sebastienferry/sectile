const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('project open tasks load immediately, search safely and launch the selected ticket',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-open-tasks-'))
 const requests=[],launches=[],pending=[]
 let failRead=false,failLaunch=false,configured=true,empty=false
 const tasks=[
  {id:'a1',key:'#1',title:'First open task',status:'to_clarify'},
  {id:'a2',key:'#2',title:'Second open task',status:'to_implement'},
  {id:'a3',key:'#3',title:'Finished by status',status:'finished'},
  {id:'a4',key:'#4',title:'Finished by label',labels:['#finished']},
  {id:'a5',key:'#5',title:'Done task',status:'done'}
 ]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost'),project=url.searchParams.get('projectId')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify(['A','B'].map(name=>({id:'project-'+name.toLowerCase(),name:'Project '+name,path:'/tmp/'+name}))));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured,server:{skills:[{id:'clarify'},{id:'pickup'},{id:'specify'}]}}));return}
  if(url.pathname==='/desktop/tasks'){
   if(req.method==='POST'){
    let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
     if(failLaunch){res.writeHead(500);res.end(JSON.stringify({error:'Launch rejected'}));return}
     launches.push({project,...JSON.parse(raw)});res.end(JSON.stringify({status:'queued'}))
    });return
   }
   const query=url.searchParams.get('q');requests.push({project,query,launchable:url.searchParams.get('launchable')})
   if(query==='slow'){pending.push(res);return}
   if(failRead){res.writeHead(503);res.end(JSON.stringify({error:'Offline'}));return}
   const result=empty?[]:project==='project-b'?[{id:'b1',key:'#1',title:'Other project task',status:'to_clarify'}]:tasks
   res.end(JSON.stringify(result.filter(task=>!query||(task.key+' '+task.title).includes(query))));return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const open=name=>page.getByRole('button',{name:'Open tasks in Project '+name,exact:true})
  const cards=page.locator('.server-task'),query=page.getByRole('textbox',{name:'Search server tasks'})
  const search=page.getByRole('button',{name:'Search',exact:true})
  const close=()=>page.getByRole('button',{name:'Close',exact:true}).click()
  const heading=page.getByRole('button',{name:'▾ Project A',exact:true})
  await heading.waitFor()
  await page.mouse.move(700,400)
  await expect(open('A')).toHaveCSS('opacity','0')
  await heading.hover()
  await expect(open('A')).toHaveCSS('opacity','1')
  await heading.click()
  await page.getByRole('button',{name:'▸ Project A',exact:true}).focus();await page.keyboard.press('Tab')
  await expect(open('A')).toBeFocused();await expect(open('A')).toHaveCSS('opacity','1')
  await page.keyboard.press('Enter')
  await expect(cards).toHaveCount(2)
  assert.deepEqual(requests.at(-1),{project:'project-a',query:'',launchable:'true'})
  assert.equal(await query.inputValue(),'')
  assert.equal(await query.evaluate(el=>document.activeElement===el),true)
  assert.equal(await page.getByRole('combobox',{name:'Skill for #1',exact:true}).inputValue(),'pickup')
  assert.equal(launches.length,0)
  await query.fill('Second');await search.click();await expect(cards).toHaveCount(1)
  await expect(cards).toContainText('#2 · Second open task')
  await query.fill('missing');await search.click();await page.getByText('No matching open tasks',{exact:true}).waitFor()
  await query.fill('');await search.click();await expect(cards).toHaveCount(2)
  await page.screenshot({path:path.join(root,'project-open-tasks.png')})
  console.log('Open tasks screenshot: '+path.join(root,'project-open-tasks.png'))
  // Hold an older request, complete a newer one, then reject the old request.
  await query.fill('slow');await search.click();await expect.poll(()=>pending.length).toBe(1)
  await page.getByText('Loading open tasks…',{exact:true}).waitFor()
  await query.fill('Second');await search.click();await expect(cards).toHaveCount(1)
  pending.shift().writeHead(503).end(JSON.stringify({error:'Stale failure'}))
  await page.waitForTimeout(150)
  await expect(cards).toContainText('Second open task')
  await expect(page.getByRole('alert').filter({hasText:'Stale failure'})).toHaveCount(0)
  // An old successful response cannot populate another project's dialog.
  await query.fill('slow');await search.click();await expect.poll(()=>pending.length).toBe(1)
  await close();await open('B').click();await expect(cards).toContainText('Other project task')
  pending.shift().end(JSON.stringify(tasks))
  await page.waitForTimeout(150);await expect(cards).toHaveCount(1);await expect(cards).toContainText('Other project task')
  assert.equal(requests.at(-1).project,'project-b')
  failLaunch=true;await page.getByRole('button',{name:'Launch',exact:true}).click()
  await page.getByText('Launch rejected',{exact:false}).waitFor()
  assert.equal(launches.length,0)
  failLaunch=false
  await page.getByRole('combobox',{name:'Skill for #1',exact:true}).selectOption('specify')
  await page.getByRole('button',{name:'Launch',exact:true}).click()
  await expect(page.locator('#project-dialog')).not.toBeVisible()
  assert.deepEqual(launches,[{project:'project-b',taskID:'b1',skillID:'specify',prompt:''}])
  // A discussion is launchable with no instructions at all.
  await open('B').click();await expect(cards).toContainText('Other project task')
  const instructions=page.getByRole('textbox',{name:'Custom instructions',exact:true})
  await page.getByRole('combobox',{name:'Skill for #1',exact:true}).selectOption('discuss')
  await expect(instructions).toBeHidden()
  await page.getByRole('button',{name:'Launch',exact:true}).click()
  await expect(page.locator('#project-dialog')).not.toBeVisible()
  assert.deepEqual(launches.at(-1),{project:'project-b',taskID:'b1',skillID:'discuss',prompt:''})
  failRead=true;await open('A').click();await page.getByRole('alert').filter({hasText:'Could not load open tasks'}).waitFor()
  failRead=false;await search.click();await expect(cards).toHaveCount(2)
  empty=true;await search.click();await page.getByText('No open tasks in this project',{exact:true}).waitFor()
  empty=false;configured=false;await search.click();await expect(cards).toHaveCount(2)
  await page.getByText('Configure a local repository before launching tasks.',{exact:true}).waitFor()
  for(const button of await page.getByRole('button',{name:'Launch',exact:true}).all())assert.equal(await button.isDisabled(),true)
  await page.keyboard.press('Escape');await expect(page.locator('#project-dialog')).not.toBeVisible()
  await page.getByRole('button',{name:'▸ Project A',exact:true}).waitFor()
  // The new icon fits alongside the existing project controls at minimum width.
  await page.getByRole('separator',{name:'Resize sidebar'}).focus()
  for(let i=0;i<4;i++)await page.keyboard.press('ArrowLeft')
  const row=page.locator('.project-row').first(),bounds=await row.boundingBox()
  for(const button of await row.getByRole('button').all()){
   const control=await button.boundingBox()
   assert.ok(control.x>=bounds.x&&control.x+control.width<=bounds.x+bounds.width+1)
  }
  assert.equal(launches.length,2)
 }finally{
  for(const response of pending)response.end('[]')
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
