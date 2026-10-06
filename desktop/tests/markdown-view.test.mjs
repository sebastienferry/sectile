import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { isExternalLink, markdownModel, renderMarkdown, resolveImageTarget } from '../src/markdownView.mjs'

const testdata=name=>readFileSync(new URL('../../internal/runner/testdata/'+name,import.meta.url),'utf8')

const blocks=source=>markdownModel(source).children
const find=(tree,type)=>{
 for(const child of tree.children||[]){if(child.type===type)return child;const found=find(child,type);if(found)return found}
}
const text=tree=>tree.text??(tree.children||[]).map(text).join('')

test('headings, emphasis, quotes and nested lists keep their structure',()=>{
 const [heading,paragraph,quote,list]=blocks('## Usage\n\nSome *em*, **strong** and `code`.\n\n> quoted\n\n- one\n  1. nested\n  2. second\n- two\n')
 assert.deepEqual([heading.type,heading.level,text(heading)],['heading',2,'Usage'])
 assert.deepEqual(paragraph.children.map(c=>c.type),['text','em','text','strong','text','code','text'])
 assert.equal(quote.type,'blockquote');assert.equal(text(quote),'quoted')
 assert.equal(list.type,'list');assert.equal(list.ordered,undefined);assert.equal(list.children.length,2)
 const nested=find(list.children[0],'list')
 assert.deepEqual([nested.ordered,nested.start,nested.children.length],[true,1,2])
 assert.equal(blocks('3. third\n')[0].start,3)
})

test('tables keep their alignment',()=>{
 const [table]=blocks('| Left | Center | Right | None |\n|:-----|:------:|------:|------|\n| a | b | c | d |\n')
 assert.equal(table.type,'table')
 const header=table.children[0].children[0].children,body=table.children[1].children[0].children
 assert.deepEqual(header.map(c=>[c.header,c.align]),[[true,'left'],[true,'center'],[true,'right'],[true,undefined]])
 assert.deepEqual(body.map(text),['a','b','c','d']);assert.equal(body[0].header,false)
})

test('task lists, strikethrough and fenced code',()=>{
 const [list,paragraph,fence,math]=blocks('- [ ] todo\n- [x] done\n- [X] upper\n- plain [ ] item\n\n~~gone~~\n\n```mermaid\ngraph TD\n```\n\n```math\na^2\n```\n')
 assert.deepEqual(list.children.map(c=>[c.type,c.checked,text(c)]),[['task',false,'todo'],['task',true,'done'],['task',true,'upper'],['item',undefined,'plain [ ] item']])
 assert.equal(paragraph.children[0].type,'strike')
 assert.deepEqual([fence.type,fence.info,fence.text],['codeBlock','mermaid','graph TD\n'])
 assert.deepEqual([math.type,math.text],['codeBlock','a^2\n'])
})

test('raw HTML stays text, block and inline',()=>{
 const source='<script>alert(1)</script>\n\n<iframe src="https://example.com"></iframe>\n\nInline <b>bold</b> and <img src=x onerror=alert(1)>\n'
 const tree=markdownModel(source)
 assert.equal(text(tree),'<script>alert(1)</script><iframe src="https://example.com"></iframe>Inline <b>bold</b> and <img src=x onerror=alert(1)>')
 assert.equal(JSON.stringify(tree).includes('"html'),false)
})

test('only absolute web and mail links are external',()=>{
 const cases={'https://example.com':true,'http://example.com/a?b#c':true,'mailto:someone@example.com':true,'HTTPS://EXAMPLE.COM':true,'https://user:secret@example.com':false,'../plan.md':false,'#usage':false,'docs/guide.md':false,'javascript:alert(1)':false,'JavaScript:alert(1)':false,'file:///etc/passwd':false,'data:text/html,x':false,'vbscript:x':false,'':false}
 for(const [href,external] of Object.entries(cases)){
  assert.equal(isExternalLink(href),external,href)
  const link=find(markdownModel(`[label](<${href}>)`),'link')
  assert.equal(link?.external,external,'parsed '+href)
 }
 assert.equal(find(markdownModel('<https://example.com>'),'link').external,true)
})

test('images become their alt text and target, local or remote',()=>{
 const images=blocks('![A diagram](docs/diagram.png "The flow") ![remote](https://example.com/x.png)')[0].children.filter(c=>c.type==='image')
 assert.deepEqual(images.map(i=>[i.alt,i.src,i.title]),[['A diagram','docs/diagram.png','The flow'],['remote','https://example.com/x.png',undefined]])
})

// The agent finds the same images in the same document
// (internal/runner/markdown_images_test.go): a drift would leave an image
// the agent did not read.
test('markdown-it finds the images the agent reads',()=>{
 const images=[]
 const walk=tree=>{for(const child of tree.children||[]){if(child.type==='image')images.push(child.src);walk(child)}}
 walk(markdownModel(testdata('markdown_image_document.md')))
 const agent=['a.png','../logo.png','collapsed.png','cell.png','list.png','badge.svg','inline-html.png','block-html.png','my_shot.png','a&b.png','my shot.png','struck.png']
 assert.deepEqual(images.map(src=>resolveImageTarget('x.md',src)?.path),agent.map(target=>resolveImageTarget('x.md',target)?.path))
})

test('image targets resolve as the agent resolves them',()=>{
 const rows=JSON.parse(testdata('markdown_image_targets.json'))
 assert.ok(rows.length>=20)
 for(const row of rows){
  const resolved=resolveImageTarget(row.document,row.target)
  assert.deepEqual([resolved?.path??'',resolved?.reason??''],[row.path,row.reason],row.document+' + '+JSON.stringify(row.target))
  // markdown-it hands the renderer an encoded target: it resolves the same.
  if(row.target&&!/^[a-z][a-z0-9+.-]*:/i.test(row.target)&&!/[\n\0]/.test(row.path)&&!/%(?![0-9A-Fa-f]{2})/.test(row.target)){
   const parsed=find(markdownModel(`![x](<${row.target}>)`),'image')
   if(parsed)assert.deepEqual(resolveImageTarget(row.document,parsed.src),resolved,'parsed '+row.target)
  }
 }
})

// A minimal DOM: enough for renderMarkdown to build images and fallbacks.
function fakeDocument(){
 const node=props=>({children:[],listeners:{},append(...items){this.children.push(...items)},addEventListener(type,listener){this.listeners[type]=listener},replaceWith(other){this.replacedBy=other},...props})
 return {createElement:tag=>node({tag}),createTextNode:text=>node({tag:'#text',textContent:text}),createDocumentFragment:()=>node({tag:'#fragment'})}
}
const rendered=(source,options)=>renderMarkdown(markdownModel(source),{document:fakeDocument(),...options}).children[0].children[0]

test('an image is shown only from data the caller supplies',()=>{
 const source='![Flow](flow.png "Request flow")'
 const fallback=rendered(source)
 assert.deepEqual([fallback.tag,fallback.className,fallback.textContent,fallback.title],['span','md-image','[Flow] (flow.png)',undefined])
 assert.equal(rendered(source,{image:()=>null}).title,undefined)
 assert.equal(rendered(source,{image:()=>({reason:'Image not found in the inspected state.'})}).title,'Image not found in the inspected state.')
 const shown=rendered(source,{image:item=>{assert.equal(item.src,'flow.png');return {mimeType:'image/png',data:'iVBORw0KGgo='}}})
 assert.deepEqual([shown.tag,shown.className,shown.alt,shown.title,shown.src],['img','md-picture','Flow','Request flow','data:image/png;base64,iVBORw0KGgo='])
 shown.listeners.error()
 assert.deepEqual([shown.replacedBy.className,shown.replacedBy.title],['md-image','This image could not be displayed.'])
 for(const refused of [{mimeType:'text/html',data:'PGI+'},{mimeType:'image/png',data:'a"b'},{mimeType:'image/png'}]){
  const item=rendered(source,{image:()=>refused})
  assert.deepEqual([item.tag,item.title],['span',undefined],JSON.stringify(refused))
 }
})

test('empty and missing sources render nothing',()=>{
 assert.deepEqual(markdownModel('').children,[])
 assert.deepEqual(markdownModel(undefined).children,[])
})
