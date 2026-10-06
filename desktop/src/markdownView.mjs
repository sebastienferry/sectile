import MarkdownIt from 'markdown-it'

// A rendered Markdown document of the Changes panel (#575). Parsing yields a
// plain tree that node --test can check; the DOM is then built from it with
// createElement and textContent only, so raw HTML stays text, no frame is ever
// created and no element carries an href to navigate with. An image is created
// only from bytes the caller supplies as a data: URL, never from a URL of the
// document (#683).
const md=new MarkdownIt('commonmark',{html:false,linkify:false}).enable(['table','strikethrough'])
// Every link becomes a link token: the renderer decides what may be followed.
md.validateLink=()=>true

const EXTERNAL=['http:','https:','mailto:']
export function isExternalLink(href){
 try{const url=new URL(href);return EXTERNAL.includes(url.protocol)&&!url.username&&!url.password}catch{return false}
}

// resolveImageTarget turns an image target of the document at documentPath
// into a repository path: {path}, {reason} when a repository image is refused,
// or null when the target is not a repository image. It follows the steps of
// resolveImageTarget in internal/runner/markdown_images.go, and
// internal/runner/testdata/markdown_image_targets.json keeps both in step.
export function resolveImageTarget(documentPath,src){
 const target=String(src??'')
 if(!target||/^[A-Za-z][A-Za-z0-9+.-]*:/.test(target))return null
 let decoded
 // A "%" that starts no escape is literal, as markdown-it encodes it.
 try{decoded=decodeURIComponent(target.split(/[?#]/,1)[0].replace(/%(?![0-9A-Fa-f]{2})/g,'%25'))}catch{return null}
 if(!decoded)return null
 const parts=decoded.startsWith('/')?[]:String(documentPath).split('/').slice(0,-1)
 for(const segment of decoded.split('/')){
  if(!segment||segment==='.')continue
  if(segment!=='..'){parts.push(segment);continue}
  if(!parts.length)return {reason:'This image path leaves the repository.'}
  parts.pop()
 }
 if(!parts.length)return {reason:'This image path leaves the repository.'}
 const path=parts.join('/')
 return /[\n\0]/.test(path)?{path,reason:'This path cannot be rendered.'}:{path}
}

const BLOCKS={paragraph:'paragraph',heading:'heading',blockquote:'blockquote',bullet_list:'list',ordered_list:'list',list_item:'item',table:'table',thead:'thead',tbody:'tbody',tr:'row',th:'cell',td:'cell',em:'em',strong:'strong',s:'strike',link:'link'}

function node(token){
 const type=BLOCKS[token.type.replace(/_open$/,'')],result={type,children:[]}
 if(type==='heading')result.level=Number(token.tag.slice(1))
 if(token.type==='ordered_list_open'){result.ordered=true;result.start=Number(token.attrGet('start')||1)}
 if(type==='paragraph'&&token.hidden)result.tight=true
 if(type==='cell'){result.header=token.tag==='th';const align=/text-align:(left|center|right)/.exec(token.attrGet('style')||'');if(align)result.align=align[1]}
 if(type==='link'){result.href=token.attrGet('href')||'';result.external=isExternalLink(result.href)}
 return result
}

function leaf(token){
 switch(token.type){
  case 'text':case 'html_inline':return {type:'text',text:token.content}
  case 'softbreak':return {type:'text',text:'\n'}
  case 'hardbreak':return {type:'break'}
  case 'code_inline':return {type:'code',text:token.content}
  case 'image':{const title=token.attrGet('title');return {type:'image',src:token.attrGet('src')||'',alt:token.content,...title?{title}:{}}}
  case 'fence':case 'code_block':return {type:'codeBlock',text:token.content,info:token.info.trim()}
  case 'html_block':return {type:'paragraph',children:[{type:'text',text:token.content}]}
  case 'hr':return {type:'rule'}
 }
 return null
}

function build(tokens,root){
 const stack=[root]
 for(const token of tokens){
  const parent=stack[stack.length-1]
  if(token.type==='inline'){build(token.children||[],parent);continue}
  if(token.nesting===1){const child=node(token);parent.children.push(child);stack.push(child);continue}
  if(token.nesting===-1){stack.pop();continue}
  const child=leaf(token)
  if(child)parent.children.push(child)
 }
 return root
}

// GFM task list items: "[ ] " or "[x] " opening the item's first paragraph.
function tasks(tree){
 for(const child of tree.children||[]){
  tasks(child)
  if(child.type!=='item')continue
  const first=child.children[0]?.children?.[0],marker=first?.type==='text'&&/^\[([ xX])\][ \t]+/.exec(first.text)
  if(!marker||child.children[0].type!=='paragraph')continue
  first.text=first.text.slice(marker[0].length)
  child.type='task';child.checked=marker[1]!==' '
 }
 return tree
}

export function markdownModel(source){
 return tasks(build(md.parse(String(source??''),{}),{type:'document',children:[]}))
}

function element(document,tag,className,text){
 const result=document.createElement(tag)
 if(className)result.className=className
 if(text!=null)result.textContent=text
 return result
}

function link(document,item,openLink){
 if(item.external){
  const anchor=element(document,'a','md-link')
  anchor.setAttribute('role','link');anchor.tabIndex=0;anchor.title=item.href
  const open=event=>{event.preventDefault();Promise.resolve(openLink?.(item.href)).catch(()=>{})}
  anchor.addEventListener('click',open)
  anchor.addEventListener('keydown',event=>{if(event.key==='Enter')open(event)})
  return anchor
 }
 const inert=element(document,'span','md-link-inert')
 inert.title=item.href
 return inert
}

const IMAGE_TYPES=['image/png','image/jpeg','image/gif','image/webp','image/svg+xml']

// picture shows the image the caller's image option supplies for item, or
// the alt text and target, with the reason it is not shown as a tooltip.
function picture(document,item,image){
 const fallback=reason=>{const span=element(document,'span','md-image',`[${item.alt}] (${item.src})`);if(reason)span.title=reason;return span}
 const found=image?.(item)
 if(!found||found.reason||!IMAGE_TYPES.includes(found.mimeType)||typeof found.data!=='string'||!/^[A-Za-z0-9+/]*={0,2}$/.test(found.data))return fallback(found?.reason)
 const img=element(document,'img','md-picture')
 img.alt=item.alt;img.decoding='async'
 if(item.title)img.title=item.title
 img.addEventListener('error',()=>img.replaceWith(fallback('This image could not be displayed.')),{once:true})
 img.src=`data:${found.mimeType};base64,${found.data}`
 return img
}

const TAGS={blockquote:'blockquote',table:'table',thead:'thead',tbody:'tbody',row:'tr',em:'em',strong:'strong',strike:'s'}

function append(document,parent,items,options){
 for(const item of items){
  let target
  switch(item.type){
   case 'text':parent.append(document.createTextNode(item.text));continue
   case 'break':parent.append(element(document,'br'));continue
   case 'code':parent.append(element(document,'code',null,item.text));continue
   case 'rule':parent.append(element(document,'hr'));continue
   case 'image':parent.append(picture(document,item,options.image));continue
   case 'codeBlock':{const pre=element(document,'pre');pre.append(element(document,'code',null,item.text));parent.append(pre);continue}
   case 'paragraph':if(item.tight){append(document,parent,item.children,options);continue}target=element(document,'p');break
   case 'heading':target=element(document,'h'+item.level);break
   case 'list':target=element(document,item.ordered?'ol':'ul');if(item.ordered&&item.start!==1)target.start=item.start;break
   case 'item':target=element(document,'li');break
   case 'task':{
    target=element(document,'li','md-task')
    const box=element(document,'input');box.type='checkbox';box.checked=item.checked;box.disabled=true
    target.append(box,document.createTextNode(' '))
    break
   }
   case 'cell':target=element(document,item.header?'th':'td');if(item.align)target.style.textAlign=item.align;break
   case 'link':target=link(document,item,options.openLink);break
   default:if(!TAGS[item.type])continue;target=element(document,TAGS[item.type])
  }
  append(document,target,item.children,options)
  parent.append(target)
  if(item.type==='link'&&!item.external)parent.append(element(document,'span','md-link-target',` (${item.href})`))
 }
 return parent
}

// image(item), optional, returns what to show for an image item: null for
// its alt text, {reason} for its alt text with the reason as a tooltip, or
// {mimeType,data} with the Base64 bytes. Without it, every image is its alt
// text, as in the conversation view.
export function renderMarkdown(model,{document=globalThis.document,openLink,image}={}){
 return append(document,document.createDocumentFragment(),model.children,{openLink,image})
}
