import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { APPEARANCE_CHOICES, TERMINAL_THEMES, terminalTheme } from '../src/appearance.mjs'
import { pullRequestPresentation } from '../src/pullRequests.mjs'

const ANSI=['black','red','green','yellow','blue','magenta','cyan','white','brightBlack','brightRed','brightGreen','brightYellow','brightBlue','brightMagenta','brightCyan','brightWhite']

test('the settings offer System, Dark and Light, System first',()=>{
 assert.deepEqual(APPEARANCE_CHOICES.map(choice=>choice.value),['system','dark','light'])
})

test('each terminal theme carries a complete palette',()=>{
 for(const [name,theme] of Object.entries(TERMINAL_THEMES)){
  for(const key of ['background','foreground','cursor','cursorAccent','selectionBackground',...ANSI]){
   assert.match(theme[key]||'',/^#[0-9a-f]{6}$/,name+'.'+key)
  }
 }
})

test('the dark terminal keeps the colours the console always had',()=>{
 assert.equal(terminalTheme(true),TERMINAL_THEMES.dark)
 assert.equal(TERMINAL_THEMES.dark.background,'#11151c')
 assert.equal(TERMINAL_THEMES.dark.foreground,'#d8e0ec')
})

test('the light terminal is light, and its text stays dark',()=>{
 assert.equal(terminalTheme(false),TERMINAL_THEMES.light)
 const luminance=hex=>{
  const [r,g,b]=[1,3,5].map(i=>parseInt(hex.slice(i,i+2),16)/255)
  return 0.2126*r+0.7152*g+0.0722*b
 }
 assert.ok(luminance(TERMINAL_THEMES.light.background)>0.9)
 // CLI output printed in ANSI white or yellow assumes a dark background; on a
 // light one those have to stay readable.
 for(const key of ['foreground','white','yellow','brightWhite','brightYellow']){
  assert.ok(luminance(TERMINAL_THEMES.light[key])<0.6,key)
 }
})

// The stylesheet keeps its colours in two token blocks, dark and light, and the
// rules only name tokens: a literal left in a rule would stay dark in light mode.
const css=readFileSync(new URL('../src/style.css',import.meta.url),'utf8')
const LITERAL=/#[0-9a-fA-F]{3,8}\b|rgba?\(/g
function tokenBlocks(){
 const start=css.indexOf(':root{')
 const light=css.indexOf('@media (prefers-color-scheme:light)')
 const end=css.indexOf('\n }\n}\n',light)+'\n }\n}\n'.length
 assert.ok(start>=0&&light>start&&end>light,'token blocks not found at the top of style.css')
 const names=block=>new Set([...block.matchAll(/--([a-z0-9-]+):/g)].map(match=>match[1]))
 return {dark:names(css.slice(start,light)),light:names(css.slice(light,end)),rules:css.slice(end)}
}

test('no stylesheet rule carries a colour literal',()=>{
 const {rules}=tokenBlocks()
 const declarations=[...rules.replace(/\/\*[\s\S]*?\*\//g,'').matchAll(/\{([^{}]*)\}/g)].map(match=>match[1])
 assert.deepEqual(declarations.flatMap(body=>body.match(LITERAL)||[]),[])
})

test('every token a rule uses is defined, and light redefines every dark token',()=>{
 const {dark,light,rules}=tokenBlocks()
 const used=new Set([...rules.matchAll(/var\(--([a-z0-9-]+)/g)].map(match=>match[1]))
 assert.deepEqual([...used].filter(name=>!dark.has(name)),[])
 assert.deepEqual([...dark].filter(name=>!light.has(name)),[])
 assert.deepEqual([...light].filter(name=>!dark.has(name)),[])
})

test('the pull request colours come from tokens both modes define',()=>{
 const {dark,light}=tokenBlocks()
 for(const state of ['open','merged','closed','conflicting','unknown']){
  const token=pullRequestPresentation({state}).color.match(/^var\(--([a-z-]+)\)$/)?.[1]
  assert.ok(token&&dark.has(token)&&light.has(token),state)
 }
})
