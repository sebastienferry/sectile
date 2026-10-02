import assert from 'node:assert/strict'
import { test } from 'node:test'
import { isExternalLink, markdownModel } from '../src/markdownView.mjs'

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
 const images=blocks('![A diagram](docs/diagram.png) ![remote](https://example.com/x.png)')[0].children.filter(c=>c.type==='image')
 assert.deepEqual(images.map(i=>[i.alt,i.src]),[['A diagram','docs/diagram.png'],['remote','https://example.com/x.png']])
})

test('empty and missing sources render nothing',()=>{
 assert.deepEqual(markdownModel('').children,[])
 assert.deepEqual(markdownModel(undefined).children,[])
})
