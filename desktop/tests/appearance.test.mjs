import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { APPEARANCE_CHOICES, TERMINAL_THEMES, terminalOptions, terminalTheme } from '../src/appearance.mjs'
import { pullRequestPresentation } from '../src/pullRequests.mjs'

// WCAG relative luminance and contrast ratio, with sRGB linearisation.
const luminance=hex=>{const [r,g,b]=[1,3,5].map(i=>parseInt(hex.slice(i,i+2),16)/255).map(v=>v<=0.03928?v/12.92:((v+0.055)/1.055)**2.4);return 0.2126*r+0.7152*g+0.0722*b}
const contrast=(a,b)=>{const [x,y]=[luminance(a),luminance(b)].sort((p,q)=>q-p);return (x+0.05)/(y+0.05)}
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
 assert.deepEqual(TERMINAL_THEMES.dark,{
  background:'#11151c',foreground:'#d8e0ec',cursor:'#d8e0ec',cursorAccent:'#11151c',selectionBackground:'#39465a',
  black:'#1a212d',red:'#ff7b7b',green:'#62d3be',yellow:'#f2c46d',blue:'#6ab0ff',magenta:'#c792ea',cyan:'#79c0ff',white:'#d8e0ec',
  brightBlack:'#5b6b7e',brightRed:'#ff9292',brightGreen:'#80e6ce',brightYellow:'#ffd98a',brightBlue:'#a6c8ff',brightMagenta:'#dcb5f5',brightCyan:'#9ecbff',brightWhite:'#ffffff'
 })
})

test('the light terminal darkens what falls below WCAG AA, the dark one keeps colours as printed',()=>{
 assert.deepEqual(terminalOptions(false),{minimumContrastRatio:4.5,theme:TERMINAL_THEMES.light})
 assert.deepEqual(terminalOptions(true),{minimumContrastRatio:1,theme:TERMINAL_THEMES.dark})
})

test('the light terminal is light, and every text colour reaches WCAG AA on it',()=>{
 assert.equal(terminalTheme(false),TERMINAL_THEMES.light)
 const light=TERMINAL_THEMES.light
 assert.ok(luminance(light.background)>0.9)
 // CLI output printed in ANSI white or yellow assumes a dark background; on a
 // light one every colour has to stay readable.
 for(const key of ['foreground',...ANSI]){
  const ratio=contrast(light[key],light.background)
  assert.ok(ratio>=4.5,key+' '+ratio.toFixed(2))
 }
})

// The stylesheet keeps its colours in two token blocks, dark and light, and the
// rules only name tokens: a literal left in a rule would stay dark in light mode.
const css=readFileSync(new URL('../src/style.css',import.meta.url),'utf8').replace(/\r\n/g,'\n')
const LITERAL=/#[0-9a-fA-F]{3,8}\b|rgba?\(/g
function tokenBlocks(){
 const start=css.indexOf(':root{')
 const light=css.indexOf('@media (prefers-color-scheme:light)')
 const end=css.indexOf('\n }\n}\n',light)+'\n }\n}\n'.length
 assert.ok(start>=0&&light>start&&end>light,'token blocks not found at the top of style.css')
 const names=block=>new Set([...block.matchAll(/--([a-z0-9-]+):/g)].map(match=>match[1]))
 const values=block=>new Map([...block.matchAll(/--([a-z0-9-]+):\s*(#[0-9a-f]{6})\b/gi)].map(m=>[m[1],m[2].toLowerCase()]))
 return {dark:names(css.slice(start,light)),light:names(css.slice(light,end)),lightValues:values(css.slice(light,end)),rules:css.slice(end)}
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

// Disabled controls (opacity, exempt under WCAG 1.4.3) and non-text graphics (effort bars, context ring; WCAG 1.4.11) are left out.
test('the light conversation text reaches WCAG AA on its background',()=>{
 const {lightValues}=tokenBlocks()
 const pairs=[['text-faint','bg'],['text-faint','button-bg'],['text','bg'],['text','button-bg'],['danger','bg'],['danger','button-bg'],['text-muted','bg'],['accent-text','accent-button-bg'],['waiting','button-bg']]
 for(const [text,background] of pairs){
  assert.ok(lightValues.has(text)&&lightValues.has(background),text+' on '+background+' is not a hex token')
  const ratio=contrast(lightValues.get(text),lightValues.get(background))
  assert.ok(ratio>=4.5,text+' on '+background+' '+ratio.toFixed(2))
 }
})
