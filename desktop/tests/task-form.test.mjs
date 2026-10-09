import assert from 'node:assert/strict'
import { test } from 'node:test'
import { validateTitle, validateDescription, validatePullRequestUrl, addLink, removeLink, moveLink, currentLink, taskChanges, assigneeLookup, initialProject, leaveMessage, DESCRIPTION_LIMIT } from '../src/task-form.mjs'

test('a pull request link must be an absolute web address, without credentials, listed once',()=>{
 const links=[{url:'https://github.com/o/r/pull/1'}]
 assert.equal(validatePullRequestUrl('  ',links).reason,'Enter a pull request link.')
 assert.equal(validatePullRequestUrl('github.com/o/r/pull/2',links).reason,'Enter an absolute http or https link.')
 assert.equal(validatePullRequestUrl('ftp://example.com/pull/2',links).reason,'Enter an absolute http or https link.')
 assert.equal(validatePullRequestUrl('javascript:alert(1)',links).reason,'Enter an absolute http or https link.')
 assert.equal(validatePullRequestUrl('https://user:secret@github.com/o/r/pull/2',links).reason,'A link with credentials is refused.')
 assert.equal(validatePullRequestUrl('https://github.com/o/r/pull/1',links).reason,'This pull request is already listed.')
 assert.deepEqual(validatePullRequestUrl(' https://github.com/o/r/pull/2 ',links),{ok:true,url:'https://github.com/o/r/pull/2'})
 assert.equal(validatePullRequestUrl('http://gitlab.example/g/p/-/merge_requests/3').ok,true)
})

test('the link set is added to, reordered and shortened without losing loaded details',()=>{
 const loaded=[{url:'a',state:'merged',branch:'feat/1'},{url:'b',state:'open'}]
 const added=addLink(loaded,'c')
 assert.deepEqual(added.at(-1),{url:'c'})
 assert.equal(added[0],loaded[0],'a loaded link is kept as it is')
 assert.deepEqual(moveLink(added,0,1).map(link=>link.url),['b','a','c'])
 assert.deepEqual(moveLink(added,2,-1).map(link=>link.url),['a','c','b'])
 assert.equal(moveLink(added,0,-1),added,'the first cannot move up')
 assert.equal(moveLink(added,2,1),added,'the last cannot move down')
 assert.deepEqual(removeLink(added,1).map(link=>link.url),['a','c'])
 assert.equal(currentLink(added).url,'c')
 assert.equal(currentLink([]),null)
 assert.deepEqual(moveLink(added,0,1)[1],{url:'a',state:'merged',branch:'feat/1'})
})

test('a save sends only the fields that changed',()=>{
 const loaded={title:'Title',description:'* item',assignee:'Jane',assigneeAccountId:'42',prLinks:[{url:'a',state:'merged'},{url:'b'}]}
 const untouched={title:'Title',description:'- item',assignee:'Jane',assigneeAccountId:'42',prLinks:loaded.prLinks}
 assert.deepEqual(taskChanges(loaded,untouched),{},'a reserialized description is not a change')
 assert.deepEqual(taskChanges(loaded,{...untouched,title:'  Title  '}),{},'a title is trimmed')
 assert.deepEqual(taskChanges(loaded,{...untouched,title:'New'}),{title:'New'})
 assert.deepEqual(taskChanges(loaded,untouched,{descriptionChanged:true}),{description:'- item'})
 assert.deepEqual(taskChanges(loaded,{...untouched,assignee:'Joe',assigneeAccountId:'7',assigneeAvatar:'https://a/7.png'}),{assignee:'Joe',assigneeAccountId:'7',assigneeAvatar:'https://a/7.png'})
 assert.deepEqual(taskChanges(loaded,{...untouched,assignee:'',assigneeAccountId:''}),{assignee:'',assigneeAccountId:'',assigneeAvatar:''},'clearing unassigns')
 assert.deepEqual(taskChanges(loaded,{...untouched,prLinks:[]}),{prLinks:[]},'removing every link detaches them all')
 assert.deepEqual(taskChanges(loaded,{...untouched,prLinks:[loaded.prLinks[1],loaded.prLinks[0]]}),{prLinks:[{url:'b'},{url:'a',state:'merged'}]},'a reorder is a change')
 assert.deepEqual(taskChanges({title:'T'},{title:'T',prLinks:[]}),{},'no links before, none after')
})

test('title and description limits',()=>{
 assert.equal(validateTitle('   ').ok,false)
 assert.deepEqual(validateTitle('  Fix it '),{ok:true,title:'Fix it'})
 assert.equal(validateTitle('x'.repeat(501)).ok,false)
 assert.equal(validateDescription('x'.repeat(DESCRIPTION_LIMIT)).ok,true)
 assert.equal(validateDescription('x'.repeat(60001)).reason,'The description is too long (60,001 of 60,000 characters).')
})

test('assignee lookup is offered where the tracker can search people',()=>{
 assert.equal(assigneeLookup('jira'),true)
 assert.equal(assigneeLookup('GitLab'),true)
 assert.equal(assigneeLookup('github'),false)
 assert.equal(assigneeLookup(''),false)
})

test('a creation starts on the project it came from, else the selected, the remembered or the only one',()=>{
 const known=[{id:'a'},{id:'b'}]
 assert.equal(initialProject({openedFrom:'b',selected:'a',remembered:'a',known}),'b')
 assert.equal(initialProject({openedFrom:'gone',selected:'a',known}),'a')
 assert.equal(initialProject({remembered:'b',known}),'b')
 assert.equal(initialProject({remembered:'gone',known}),'')
 assert.equal(initialProject({known:[{id:'only'}]}),'only')
})

test('leaving names the task',()=>{
 assert.equal(leaveMessage('#12'),'Discard unsaved changes to #12?')
 assert.equal(leaveMessage(''),'Discard unsaved changes to the new task?')
})
