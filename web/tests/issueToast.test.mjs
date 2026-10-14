import assert from 'node:assert/strict'
import { test } from 'node:test'
import { issueToastLink } from '../src/lib/issueToast.ts'
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
