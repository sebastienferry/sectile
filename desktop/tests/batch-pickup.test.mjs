import {test} from 'node:test'
import assert from 'node:assert/strict'
import {batchMembers,moveBatchMember,isValidBatchWorktreeName,suggestedBatchWorktreeName,buildBatchPickupPrompt} from '../src/batch-pickup.mjs'
test('batch membership uses distinct full IDs and all project memberships',()=>{
 const tasks=[{id:'full-1',projectId:'other',projectIds:['other','p']},{id:'full-2',projectId:'p'}]
 assert.deepEqual(batchMembers(['full-2','full-1'],tasks,'p'),[tasks[1],tasks[0]])
 for(const ids of [[],['full-1'],['full-1','full-1'],['full-1','missing']])assert.throws(()=>batchMembers(ids,tasks,'p'))
 assert.throws(()=>batchMembers(['full-1','full-2'],tasks,'other'))
 assert.deepEqual(moveBatchMember(['full-1','full-2'],1,-1),['full-2','full-1'])
 assert.deepEqual(moveBatchMember(['full-1','full-2'],0,-1),['full-1','full-2'])
})
test('worktree names are trimmed directory names with an 80 character boundary',()=>{
 for(const name of [' batch-1_2 ','a','a'.repeat(80)])assert.equal(isValidBatchWorktreeName(name),true)
 for(const name of ['','../a','a/b','-a','a b','a'.repeat(81),'$(pwd)'])assert.equal(isValidBatchWorktreeName(name),false)
 assert.equal(suggestedBatchWorktreeName(['#1','#2']),'batch-1-2')
 assert.equal(suggestedBatchWorktreeName(['a'.repeat(80)]).length,80)
 assert.equal(suggestedBatchWorktreeName(['#']), 'batch')
})
test('prompt preserves work and requests one sequential combined PR in exact order',()=>{
 const prompt=buildBatchPickupPrompt(['full-2','full-1'],' batch-2-1 ')
 assert.match(prompt,/^\/pickup-issues full-2 full-1\n/)
 assert.match(prompt,/\.tasks\/worktrees\/batch-2-1/)
 assert.match(prompt,/Preserve any existing work/)
 assert.match(prompt,/sequentially in exactly the order listed/)
 assert.match(prompt,/one combined pull request/)
 assert.throws(()=>buildBatchPickupPrompt(['full-1'],'valid'))
 assert.throws(()=>buildBatchPickupPrompt(['full-1','full-2'],'../invalid'))
})
