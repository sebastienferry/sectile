// Run with PLAYWRIGHT_MODULE pointing to a Playwright installation.
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const harness = `import React from 'react'; import {createRoot} from 'react-dom/client';
import {TaskCard} from '/src/components/TaskCard.tsx';
import {PullRequestStateIcon} from '/src/components/PullRequestStateIcon.tsx';
import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.ctx={settings:{density:'standard'},projects:[],activities:[],t:translations.fr,skillLabel:x=>x,skillCommand:(_id,command)=>command,isPinned:()=>false,isSkillRunning:false,parentFilter:null};
const task={id:'fixture',key:'#233',title:'PR state fixture',projectId:'p',status:'to_test',priority:'medium',source:'github',labels:['#implemented'],description:'Current PR',prUrl:'https://github.com/acme/app/pull/2'};
const root=createRoot(document.getElementById('root'));
window.render=(state,compact)=>root.render(<div style={{width:320,margin:20}}><TaskCard compact={compact} task={{...task,prLinks:[{url:'https://github.com/acme/app/pull/1',state:'merged'},{url:task.prUrl,state}]}}/><div id="history"><PullRequestStateIcon link={{url:'old',state:'merged'}}/></div></div>);
window.render('open',false);`
const server = await createServer({root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{
 name:'pr-state-fixture',enforce:'pre',
 transform(code,id){if(id.endsWith('.tsx'))return code.replace("import { useApp } from '../context/AppContext'","const useApp = () => window.ctx")},
 configureServer(s){s.middlewares.use(async(req,res,next)=>{
  if(req.url!=='/fixture')return next()
  res.setHeader('Content-Type','text/html')
  res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))
 })},
 resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},
 load(id){if(id===root+'/fixture.tsx')return harness},
}]})
await server.listen()
let browser
try {
 browser=await chromium.launch({headless:true,channel:'chrome'})
 const page=await browser.newPage({viewport:{width:900,height:650}})
 const errors=[];page.on('pageerror',error=>errors.push(error.message))
 await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)
 for(const [state,label] of [['open','PR ouverte'],['conflicting','PR en conflits'],['merged','PR fusionnée'],['closed','PR fermée sans fusion'],[undefined,'État de la PR inconnu']]){
  for(const compact of [false,true]){
   await page.evaluate(([state,compact])=>window.render(state,compact),[state,compact])
   if(compact)await page.getByRole('button',{name:'Actions',exact:true}).click()
   const link=page.locator('a[href="https://github.com/acme/app/pull/2"]')
   await link.getByRole('img',{name:label,exact:true}).waitFor()
   await page.locator('#history').getByRole('img',{name:'PR fusionnée',exact:true}).waitFor()
   assert.equal(await link.getAttribute('target'),'_blank')
   if(compact)await page.keyboard.press('Escape')
  }
 }
 assert.deepEqual(errors,[])
 console.log('PASS: all PR states in detailed cards, compact menus and independent history icons.')
}finally{await browser?.close();await server.close()}
