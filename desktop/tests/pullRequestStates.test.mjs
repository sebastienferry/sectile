import assert from 'node:assert/strict'
import { test } from 'node:test'
import { currentPullRequest, pullRequestPresentation, renderPullRequestIndicator } from '../src/pullRequests.mjs'

test('desktop resolves current PR and preserves unknown legacy state',()=>{
 const links=[{url:'old',state:'merged'},{url:'new',state:'conflicting'}]
 assert.equal(currentPullRequest({prLinks:links,prUrl:'old',status:'done'}),links[1])
 assert.equal(currentPullRequest({prUrl:'old',status:'done'}).state,undefined)
 assert.equal(currentPullRequest({}),undefined)
})
test('state indicators have distinct shapes and accessible descriptions',()=>{
 const states=['open','conflicting','merged','closed']
 const presentations=states.map(state=>pullRequestPresentation({state}))
 assert.equal(new Set(presentations.map(p=>p.icon)).size,4)
 assert.equal(new Set(presentations.map(p=>p.label)).size,4)
 const element={style:{},setAttribute(key,value){this[key]=value}}
 renderPullRequestIndicator(element,{url:'https://github.com/a/b/pull/1',state:'merged'},'PR #1')
 assert.match(element['aria-label'],/Merged/)
 assert.match(element.title,/https:\/\/github.com/)
 renderPullRequestIndicator(element,{url:'new',state:'closed'},'PR #2')
 assert.match(element['aria-label'],/Closed without merge/)
 assert.equal(element.style.color,pullRequestPresentation({state:'closed'}).color)
})
test('untrusted state never becomes SVG markup',()=>{
 assert.equal(pullRequestPresentation({state:'<script>'}).state,'unknown')
 assert.equal(pullRequestPresentation({state:'constructor'}).state,'unknown')
})
test('each repository the task changed shows its current pull request, the primary one first',async()=>{
 const {repositoryPullRequests,pullRequestRepository}=await import('../src/pullRequests.mjs')
 assert.equal(pullRequestRepository('https://gitlab.com/g/Deploy/-/merge_requests/7'),'gitlab.com/g/deploy')
 assert.equal(pullRequestRepository('https://example.org/o/app/pull/1'),'')
 const task={prLinks:[
  {url:'https://github.com/o/app/pull/1',state:'merged'},
  {url:'https://gitlab.com/g/deploy/-/merge_requests/7',missingToken:'gitlab'},
  {url:'https://github.com/o/app/pull/2',state:'open'},
 ]}
 assert.deepEqual(repositoryPullRequests(task).map(link=>[link.repository,link.url]),[
  ['github.com/o/app','https://github.com/o/app/pull/2'],
  ['gitlab.com/g/deploy','https://gitlab.com/g/deploy/-/merge_requests/7'],
 ])
 assert.deepEqual(repositoryPullRequests({prUrl:'https://forge/pr'}).map(link=>link.url),['https://forge/pr'])
 assert.deepEqual(repositoryPullRequests({}),[])
 assert.equal(pullRequestPresentation(task.prLinks[1]).label,'State unknown: no GitLab token')
})
test('the pull request menu lists every repository, primary first, only from two',async()=>{
 const {pullRequestMenuEntries,prLabel,repositoryName}=await import('../src/pullRequests.mjs')
 const app={url:'https://github.com/o/app/pull/79',repository:'github.com/o/app',state:'open'}
 const deploy={url:'https://gitlab.com/g/deploy/-/merge_requests/7',repository:'gitlab.com/g/deploy',missingToken:'gitlab'}
 assert.deepEqual(pullRequestMenuEntries([]),[])
 assert.deepEqual(pullRequestMenuEntries(undefined),[])
 assert.deepEqual(pullRequestMenuEntries([app]),[])
 assert.deepEqual(pullRequestMenuEntries([app,deploy]),[
  {url:app.url,repository:'github.com/o/app',name:'app',label:'PR #79',link:app},
  {url:deploy.url,repository:'gitlab.com/g/deploy',name:'deploy',label:'MR !7',link:deploy},
 ])
 const [,odd]=pullRequestMenuEntries([app,{url:'https://example.org/somewhere',repository:''}])
 assert.equal(odd.label,'PR / MR')
 assert.equal(odd.name,'PR / MR')
 assert.equal(prLabel('not a url'),'PR / MR')
 assert.equal(repositoryName(undefined),'PR / MR')
})
