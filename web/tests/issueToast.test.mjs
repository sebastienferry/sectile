import assert from 'node:assert/strict'
import { test } from 'node:test'
import { cappedLinks, isTask, issueRefLink, issueToastLink, withTicketLinks } from '../src/lib/issueToast.ts'
import { toastDuration } from '../src/lib/toastTimer.ts'
const labels = { internal: 'Open', external: 'Tracker' }
test('issue actions retain the returned object and optional tracker URL', () => {
  const task = {id:'a',key:'#1',title:'Updated',externalUrl:'https://tracker/1'}
  let selected
  const link = issueToastLink(task, value => selected=value, labels)
  link.onOpen()
  assert.equal(selected, task)
  assert.equal(link.externalUrl,task.externalUrl)
  assert.equal(issueToastLink({...task,externalUrl:''},()=>{},labels).externalUrl,undefined)
})
test('tracker-only actions never offer internal navigation or fabricate a destination', () => {
  const task={id:'',key:'J-1',externalUrl:'https://tracker/J-1'}
  assert.deepEqual(issueToastLink(task,()=>assert.fail('internal navigation'),labels),{label:'Tracker J-1',externalUrl:task.externalUrl})
  assert.equal(issueToastLink({...task,externalUrl:''},()=>{},labels),undefined)
})
test('plural and external-only actions retain linked timing and explicit precedence', () => {
  assert.equal(toastDuration({links:[{externalUrl:'https://tracker/1'}]}),8000)
  assert.equal(toastDuration({links:[]}),3500)
  assert.equal(toastDuration({links:[{}],duration:123}),123)
})
test('a ticket known by reference reads it by id on click and needs both id and key', () => {
  let opened
  const link = issueRefLink({id:'t-9',key:'J-9'}, (id,key)=>opened=[id,key], labels)
  assert.equal(link.label,'Open J-9')
  assert.equal(link.externalUrl,undefined)
  link.onOpen()
  assert.deepEqual(opened,['t-9','J-9'])
  assert.equal(issueRefLink({key:'J-9'},()=>assert.fail('no id'),labels),undefined)
  assert.equal(issueRefLink({id:'t-9'},()=>assert.fail('no key'),labels),undefined)
  assert.equal(isTask({id:'t-9',key:'J-9'}),false)
  assert.equal(isTask({id:'t-9',key:'J-9',title:'T'}),true)
})
test('several tickets keep five links in order and count the rest', () => {
  const links = n => Array.from({length:n},(_,i)=>({label:'Open #'+i,onOpen(){}}))
  assert.deepEqual(cappedLinks(links(3)).links.map(l=>l.label),['Open #0','Open #1','Open #2'])
  assert.equal(cappedLinks(links(3)).more,0)
  assert.equal(cappedLinks(links(5)).links.length,5)
  assert.equal(cappedLinks(links(5)).more,0)
  const seven = cappedLinks(links(7))
  assert.deepEqual(seven.links.map(l=>l.label),['Open #0','Open #1','Open #2','Open #3','Open #4'])
  assert.equal(seven.more,2)
  const [a,b] = links(2)
  assert.deepEqual(cappedLinks([undefined,a,undefined,b]),{links:[a,b],more:2},'an unknown ticket is counted, never linked')
  assert.deepEqual(cappedLinks([undefined,undefined]),{links:[],more:2})
  assert.deepEqual(cappedLinks([undefined]),{links:[],more:0},'one ticket reads as a single-ticket toast')
  assert.deepEqual(cappedLinks([a]),{links:[a],more:0})
})
test('ticket links follow the link a toast already offers', () => {
  const offer = {label:'Add token',onOpen(){}}
  const ticket = {label:'Open #1',onOpen(){}}
  const other = {label:'Open #2',onOpen(){}}
  const base = {type:'error',title:'Refused'}
  assert.deepEqual(withTicketLinks({...base,link:offer},ticket),{...base,links:[offer,ticket]})
  assert.deepEqual(withTicketLinks({...base,links:[offer]},{links:[ticket,other],more:3}),{...base,links:[offer,ticket,other],moreLinks:3})
  assert.deepEqual(withTicketLinks(base,{links:[ticket],more:0}),{...base,links:[ticket]})
  assert.equal('moreLinks' in withTicketLinks(base,{links:[ticket],more:0}),false)
  assert.deepEqual(withTicketLinks(base,{links:[],more:2}),{...base,links:[],moreLinks:2})
  const untouched = {...base,link:offer}
  assert.equal(withTicketLinks(untouched,undefined),untouched)
  assert.equal(withTicketLinks(untouched,{links:[],more:0}),untouched)
})
