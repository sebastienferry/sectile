const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('project settings allow remote and SDD framework editing, local setup, and omit workstation execution/AI fields',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-project-settings-test-'))
 const projectUpdates=[],deployedActions=[],savedLocal=[]
 const project={
  configured:true,
  path:'/test/repo',
  specPath:'',
  specDefault:'/test/repo',
  specKind:'git',
  aiProvider:'agy',
  anyRepository:false,
  clonesPath:'',
  clonesDefault:'/test',
  useWorktrees:true,
  specFrameworkStatuses:{
   openspec:{initialized:false,framework:'openspec'},
   speckit:{initialized:true,framework:'speckit'}
  },
  fields:{
   useWorktrees:{value:true,inherited:true,source:'default'},
   parallelism:{value:5,inherited:5,source:'default'},
   terminal:{value:'',inherited:'',source:'default'}
  },
  server:{
   projectId:'p',
   projectName:'Test project',
   gitRemoteUrl:'https://github.com/example/initial.git',
   specFramework:'openspec',
   aiProvider:'agy',
   skills:[]
  }
 }
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/project'){
   if(req.method==='PUT'){
    let body=''
    req.on('data',chunk=>body+=chunk)
    req.on('end',()=>{
     const parsed=JSON.parse(body)
     projectUpdates.push(parsed)
     if(parsed.gitRemoteUrl!==undefined)project.server.gitRemoteUrl=parsed.gitRemoteUrl
     if(parsed.specFramework!==undefined)project.server.specFramework=parsed.specFramework
     res.end(JSON.stringify(project.server))
    })
    return
   }
   if(req.method==='POST'&&url.searchParams.get('action')==='framework'){
    deployedActions.push({id:url.searchParams.get('id'),action:'framework'})
    res.end(JSON.stringify({message:'OpenSpec framework installed'}))
    return
   }
   res.end(JSON.stringify(project))
   return
  }
  if(url.pathname==='/desktop/projects'){
   if(req.method==='POST'){
    let body=''
    req.on('data',chunk=>body+=chunk)
    req.on('end',()=>{
     savedLocal.push(JSON.parse(body))
     res.end('{}')
    })
    return
   }
   res.end(JSON.stringify([{id:'p',name:'Test project',path:'/test/repo'}]))
   return
  }
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test',capabilities:['repositories','attached-folders']}));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/settings'){res.end('{"aiProvider":"agy"}');return}
  if(url.pathname==='/desktop/repositories'){res.end(JSON.stringify([{url:'git@github.com:o/a.git',identity:'github.com/o/a',path:'/test/repo',code:true}]));return}
  if(url.pathname==='/desktop/folders'){res.end('[]');return}
  res.end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'}
 delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow()
  page.setDefaultTimeout(10000)

  // Open Project Settings
  await page.getByRole('button',{name:'Actions for Test project',exact:true}).click()
  await page.getByRole('menuitem',{name:'Project settings…',exact:true}).click()

  // 1. General Tab: Git Remote and SDD Framework editing
  const remoteInput=page.getByRole('textbox',{name:'Git remote URL',exact:true})
  await expect(remoteInput).toHaveValue('https://github.com/example/initial.git')

  const sddSelect=page.getByRole('combobox',{name:'SDD framework',exact:true})
  await expect(sddSelect).toHaveValue('openspec')

  // Default engine is removed from project settings
  await expect(page.getByRole('combobox',{name:'Default engine',exact:true})).toHaveCount(0)

  // 2. Setup project locally trigger, status badge, confirmation & warning
  const setupBtn=page.getByRole('button',{name:'Setup project locally',exact:true})
  await expect(setupBtn).toBeVisible()
  const statusBadge=page.locator('.sdd-status-badge')
  await expect(statusBadge).toBeVisible()
  await expect(statusBadge).toHaveText('Not installed')

  // Clicking Setup when not installed requests confirmation explaining what is installed where
  await setupBtn.click()
  const confirmBox=page.locator('.sdd-confirm-box')
  await expect(confirmBox).toBeVisible()
  await expect(confirmBox).toHaveClass(/info/)
  await expect(confirmBox).toContainText('Install and scaffold OpenSpec (openspec) in /test/repo?')
  await page.getByRole('button',{name:'Confirm & Install',exact:true}).click()

  await expect(page.locator('.setup-status')).toHaveText('OpenSpec framework installed')
  await expect(statusBadge).toHaveText('Installed')
  assert.equal(deployedActions.length,1)
  assert.equal(deployedActions[0].action,'framework')

  // Edit Git Remote and switch SDD Framework to speckit (which is already installed)
  await remoteInput.fill('https://github.com/example/updated.git')
  await sddSelect.selectOption('speckit')
  await expect(statusBadge).toHaveText('Installed')

  // Clicking Setup when already installed displays a warning
  await setupBtn.click()
  await expect(confirmBox).toBeVisible()
  await expect(confirmBox).toHaveClass(/warning/)
  await expect(confirmBox).toContainText('⚠️ Spec Kit is already installed in /test/repo. Re-running setup will re-scaffold files and may overwrite existing configuration.')
  await page.getByRole('button',{name:'Cancel',exact:true}).click()
  await expect(confirmBox).toBeHidden()

  // 3. General Tab includes Execution controls (Worktrees & Specifications); Project Execution and Claude settings tabs are absent
  await expect(page.getByRole('tab',{name:'Execution',exact:true})).toHaveCount(0)
  await expect(page.getByRole('tab',{name:'Claude settings',exact:true})).toHaveCount(0)
  await expect(page.getByRole('group',{name:'Worktrees',exact:true})).toBeVisible()
  await expect(page.getByRole('group',{name:'Specifications',exact:true})).toBeVisible()
  await expect(page.getByRole('slider',{name:'Parallel executions',exact:true})).toHaveCount(0)
  await expect(page.getByRole('combobox',{name:'Terminal emulator',exact:true})).toHaveCount(0)
  await page.screenshot({path:'/Users/sferry/.gemini/antigravity/brain/ea0941cb-2db0-4b18-872c-9ba1dbfb6b58/project-settings-sdd-inline.png'})

  // 4. Folders Tab: Attached folders and Any repository coherent presentation
  await page.getByRole('tab',{name:'Folders',exact:true}).click()
  await expect(page.getByRole('list',{name:'Attached folders',exact:true})).toBeVisible()
  await expect(page.getByRole('button',{name:'Add folder…',exact:true})).toBeVisible()
  const anyRepoCheckbox=page.getByRole('checkbox',{name:'Let tasks change any repository',exact:true})
  await expect(anyRepoCheckbox).toBeVisible()

  // 5. Save configuration
  await page.getByRole('button',{name:'Save local configuration',exact:true}).click()
  await expect.poll(()=>projectUpdates.length).toBe(1)
  assert.equal(projectUpdates[0].gitRemoteUrl,'https://github.com/example/updated.git')
  assert.equal(projectUpdates[0].specFramework,'speckit')
  await expect.poll(()=>savedLocal.length).toBe(1)
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
