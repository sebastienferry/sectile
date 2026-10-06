import assert from 'node:assert/strict'
import { test } from 'node:test'
import { CARD_LINES, RESULT_LINES, editHunk, shortPath, toolCard } from '../src/tool-cards.mjs'

const tool=(name,input,extra={})=>({kind:'tool',text:name,tool:name,input,...extra})
const signs=hunk=>hunk.map(line=>line.sign+line.text)

test('an edit is a diff that keeps the shared lines as context',()=>{
 assert.deepEqual(signs(editHunk('a\nold\nz','a\nnew\nnewer\nz')),[' a','-old','+new','+newer',' z'])
 const card=toolCard(tool('Edit',{file_path:'/repo/src/app.js',old_string:'a\nold',new_string:'a\nnew'}),'/repo')
 assert.equal(card.target,'src/app.js')
 assert.equal(card.note,'+1 −1')
 assert.deepEqual(signs(card.body.hunks[0]),[' a','-old','+new'])
})

test('a replace-all edit and a multi-edit say so',()=>{
 assert.match(toolCard(tool('Edit',{file_path:'f',old_string:'x',new_string:'y',replace_all:true})).note,/every occurrence/)
 const multi=toolCard(tool('MultiEdit',{file_path:'f',edits:[{old_string:'a',new_string:'b'},null,{old_string:'c',new_string:'d'}]}))
 assert.equal(multi.body.hunks.length,2)
 assert.equal(multi.note,'+2 −2 · 2 edits')
})

test('a written file shows as added lines, bounded',()=>{
 const card=toolCard(tool('Write',{file_path:'/repo/big.txt',content:Array.from({length:CARD_LINES+5},(_,i)=>'l'+i).join('\n')}),'/repo')
 assert.equal(card.target,'big.txt')
 assert.equal(card.body.hunks[0].length,CARD_LINES)
 assert.equal(card.body.more,5)
 assert.ok(card.body.hunks[0].every(line=>line.sign==='+'))
})

test('a command shows its description on the line and the command in the body',()=>{
 const card=toolCard(tool('Bash',{command:'go test ./...',description:'Run the tests'}))
 assert.equal(card.target,'Run the tests')
 assert.deepEqual(card.body,{type:'code',text:'$ go test ./...'})
 assert.equal(toolCard(tool('Bash',{command:'ls\npwd'})).target,'ls')
})

test('reads and searches are one line',()=>{
 const read=toolCard(tool('Read',{file_path:'/repo/a.go',offset:10,limit:5}),'/repo')
 assert.equal(read.body,null);assert.equal(read.target,'a.go');assert.equal(read.note,'lines 10-14')
 const grep=toolCard(tool('Grep',{pattern:'TODO',path:'/repo/internal',glob:'*.go'}),'/repo')
 assert.equal(grep.target,'TODO');assert.equal(grep.note,'internal · *.go');assert.equal(grep.body,null)
})

test('a todo list is a checklist, open by default',()=>{
 const card=toolCard(tool('TodoWrite',{todos:[{content:'Read',status:'completed'},{content:'Write',status:'in_progress'},{content:'Ship',status:'bogus'},'junk']}))
 assert.equal(card.open,true)
 assert.equal(card.target,'1 of 3 done')
 assert.deepEqual(card.body.items.map(item=>item.status),['completed','in_progress','pending'])
})

test('an unknown tool or a call without arguments falls back to its detail',()=>{
 const mcp=toolCard(tool('get_task',{taskId:'#42'},{tool:'mcp__sectile__get_task',detail:'#42'}))
 assert.equal(mcp.name,'get_task');assert.equal(mcp.target,'#42');assert.match(mcp.body.text,/"taskId": "#42"/)
 const old=toolCard({kind:'tool',text:'Read',detail:'<script>'})
 assert.deepEqual(old.body,{type:'code',text:'<script>'})
 assert.equal(toolCard({kind:'tool',text:'Edit',tool:'Edit',input:'not an object',detail:'f'}).target,'f')
})

test('a path outside the directory stays whole',()=>{
 assert.equal(shortPath('/other/file','/repo'),'/other/file')
 assert.equal(shortPath('/repo/file','/repo/'),'file')
 assert.equal(shortPath(42,'/repo'),'')
})

test('a result is kept with its call, bounded, and an error marks the card',()=>{
 const bash=tool('Bash',{command:'ls'},{toolId:'t1'})
 const card=toolCard(bash,'',{result:{kind:'tool_result',toolId:'t1',text:Array.from({length:RESULT_LINES+3},(_,i)=>'f'+i).join('\n'),truncated:true}})
 assert.equal(card.result.lines.length,RESULT_LINES);assert.equal(card.result.more,3);assert.equal(card.result.truncated,true)
 assert.equal(card.failed,false);assert.equal(card.pending,false)
 const failed=toolCard(tool('Read',{file_path:'/x'}),'',{result:{text:'No such file',error:true}})
 assert.equal(failed.failed,true);assert.deepEqual(failed.result.lines,['No such file'])
})

test('an edit keeps its confirmation quiet unless it failed',()=>{
 const edit=tool('Edit',{file_path:'f',old_string:'a',new_string:'b'})
 assert.equal(toolCard(edit,'',{result:{text:'The file f has been updated.'}}).result,undefined)
 assert.deepEqual(toolCard(edit,'',{result:{text:'String not found',error:true}}).result.lines,['String not found'])
})

test('a call not answered yet is pending only while Claude works',()=>{
 const call=tool('Bash',{command:'sleep 1'},{toolId:'t1'})
 assert.equal(toolCard(call,'',{pending:true}).pending,true)
 assert.equal(toolCard(call,'',{pending:true,result:{text:''}}).pending,false)
 assert.equal(toolCard(call).pending,false)
})
