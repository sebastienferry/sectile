const {test}=require('node:test')
const assert=require('node:assert/strict')
const helper=import('../src/log-text.mjs')

test('diagnostics remove reported colors, cursor controls and OSC spinner titles',async()=>{
 const {logText}=await helper
 const message='⚠ Heads up, you have less than 25% of your weekly limit left. Run /status for a breakdown.'
 const input='\x1b[39;49m\x1b[K\x1b[38;5;3;49m'+message+'\x1b[0m\x1b[r\x1b[11;3H\x1b[0 q\x1b[?25h\x1b[?2026l\x1b]0;⠧ fretzee-studio\x07\x1b]0;fretzee-studio\x07'
 assert.equal(logText(input),message)
})

test('diagnostics preserve literal markup and Unicode while stripping terminal strings',async()=>{
 const {logText}=await helper
 assert.equal(logText('é € 😀\t<img src=x>\r\nnext\rlast\x00\x07\x08'),'é € 😀\t<img src=x>\nnext\nlast')
 assert.equal(logText('\x1b]8;;https://example.test\x1b\\link\x1b]8;;\x1b\\'),'link')
 for(const start of ['\x1bP','\x1bX','\x1b^','\x1b_','\x90','\x98','\x9e','\x9f'])assert.equal(logText('a'+start+'payload\x1b\\b'),'ab')
 assert.equal(logText('\x9d0;title\x9c\x9b31mred\x9b0m\x1b(B\x1b7'),'red')
})

test('diagnostics suppress incomplete trailing controls without guessing at plain text',async()=>{
 const {logText}=await helper
 for(const tail of ['\x1b','\x1b[38;5;','\x1b]0;title','\x1b]0;title\x1b','\x1b(','\x1bPdata'])assert.equal(logText('message'+tail),'message')
 assert.equal(logText('[39;49m ordinary text'),'[39;49m ordinary text')
})
