import {test} from 'node:test'
import assert from 'node:assert/strict'
import {TICKET_COLUMNS,ticketColumns,rowProjectID,rowInfo,viewProjectIDs} from '../src/view-tickets.mjs'

test('a project pane keeps its columns and its single project',()=>{
 const view={projectID:'p',info:{configured:true}}
 assert.equal(ticketColumns(null),TICKET_COLUMNS)
 assert.equal(rowProjectID(view,{projectId:'other'}),'p')
 assert.equal(rowInfo(view,{projectId:'other'}),view.info)
})

test('a view names each row\'s project after the key',()=>{
 assert.deepEqual(ticketColumns({id:'v'}).map(([field])=>field),['state','key','project','title','stage','priority','pr','actions'])
})

test('a view row reads its own project, and a view folder makes it launchable',()=>{
 const a={configured:true,server:{skills:[{id:'clarify'}]}},b={configured:false,server:{skills:[{id:'specify'}]}}
 const view={boardView:{id:'v',directory:''},infos:new Map([['a',a],['b',b]])}
 assert.equal(rowProjectID(view,{projectId:'b'}),'b')
 assert.equal(rowInfo(view,{projectId:'a'}),a)
 assert.equal(rowInfo(view,{projectId:'b'}).configured,false)
 assert.deepEqual(rowInfo(view,{projectId:'gone'}),{configured:false,server:{skills:[]}})
 view.boardView.directory='/work/view'
 assert.deepEqual(rowInfo(view,{projectId:'b'}),{...b,configured:true})
 assert.equal(b.configured,false,'the project info itself is left alone')
})

test('a view reads each of its projects once',()=>{
 assert.deepEqual(viewProjectIDs([{projectId:'a'},{projectId:'b'},{projectId:'a'},{}]),['a','b'])
})
