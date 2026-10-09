import {Editor,rootCtx,defaultValueCtx,editorViewCtx,parserCtx,serializerCtx,editorViewOptionsCtx,commandsCtx,remarkStringifyOptionsCtx,remarkPluginsCtx} from '@milkdown/kit/core'
import {commonmark,imageSchema,paragraphSchema,listItemSchema,toggleStrongCommand,toggleEmphasisCommand,toggleInlineCodeCommand,toggleLinkCommand,linkSchema,turnIntoTextCommand,wrapInHeadingCommand,wrapInBulletListCommand,wrapInOrderedListCommand,wrapInBlockquoteCommand,createCodeBlockCommand,insertHrCommand} from '@milkdown/kit/preset/commonmark'
import {gfm,toggleStrikethroughCommand,insertTableCommand} from '@milkdown/kit/preset/gfm'
import {history} from '@milkdown/kit/plugin/history'
import {clipboard} from '@milkdown/kit/plugin/clipboard'
import {indent} from '@milkdown/kit/plugin/indent'
import {block,BlockProvider} from '@milkdown/kit/plugin/block'
import {$prose,$view,callCommand} from '@milkdown/kit/utils'
import {Plugin,PluginKey,TextSelection,NodeSelection} from '@milkdown/kit/prose/state'
import {Decoration,DecorationSet} from '@milkdown/kit/prose/view'
import {keymap} from '@milkdown/kit/prose/keymap'
import '@milkdown/kit/prose/view/style/prosemirror.css'
import '@milkdown/kit/prose/tables/style/tables.css'
import {isExternalLink} from './markdownView.mjs'

// The task description editor (#805): ProseMirror through Milkdown, so the
// document is Markdown natively. This is the only module that imports the
// editor library; the task page talks to it through the handle it returns.
//
// What the description loaded with is the baseline. The change test compares
// documents, never Markdown text: the serializer writes its own list markers,
// emphasis characters and blank lines, and a task saved without touching its
// description must not have it rewritten. While the document still equals the
// last text it was parsed from, that text is what markdown() returns, so raw
// HTML and footnotes survive a save that did not edit them.

const BLOCKS=[
 {id:'text',label:'Text',keywords:'paragraph plain',run:ctx=>callCommand(turnIntoTextCommand.key)(ctx)},
 {id:'h1',label:'Heading 1',keywords:'title h1',run:ctx=>callCommand(wrapInHeadingCommand.key,1)(ctx)},
 {id:'h2',label:'Heading 2',keywords:'subtitle h2',run:ctx=>callCommand(wrapInHeadingCommand.key,2)(ctx)},
 {id:'h3',label:'Heading 3',keywords:'h3',run:ctx=>callCommand(wrapInHeadingCommand.key,3)(ctx)},
 {id:'bullet',label:'Bullet list',keywords:'unordered ul',run:ctx=>callCommand(wrapInBulletListCommand.key)(ctx)},
 {id:'ordered',label:'Numbered list',keywords:'ordered ol',run:ctx=>callCommand(wrapInOrderedListCommand.key)(ctx)},
 {id:'task',label:'Task list',keywords:'todo checkbox',run:ctx=>callCommand(wrapInBulletListCommand.key)(ctx)&&checkListItem(ctx)},
 {id:'quote',label:'Quote',keywords:'blockquote',run:ctx=>callCommand(wrapInBlockquoteCommand.key)(ctx)},
 {id:'code',label:'Code block',keywords:'pre fence',run:ctx=>callCommand(createCodeBlockCommand.key)(ctx)},
 {id:'table',label:'Table',keywords:'grid',run:ctx=>callCommand(insertTableCommand.key,{row:3,col:3})(ctx)},
 {id:'divider',label:'Divider',keywords:'rule hr separator',run:ctx=>callCommand(insertHrCommand.key)(ctx)},
]
// A task list is a bullet list whose items carry a checked state.
function checkListItem(ctx){
 const view=ctx.get(editorViewCtx),{$from}=view.state.selection,type=listItemSchema.type(ctx)
 for(let depth=$from.depth;depth>0;depth--){
  const node=$from.node(depth)
  if(node.type===type){view.dispatch(view.state.tr.setNodeMarkup($from.before(depth),undefined,{...node.attrs,checked:false}));return true}
 }
 return false
}
export function slashMatches(query){
 const text=query.trim().toLowerCase()
 return BLOCKS.filter(item=>!text||(item.label+' '+item.keywords).toLowerCase().includes(text))
}
// The slash query of a selection: the text typed after a "/" that opens an
// otherwise empty paragraph, the cursor at its end. Null when there is none.
function slashQuery(state){
 const {selection}=state
 if(!selection.empty)return null
 const {$from}=selection,parent=$from.parent
 if(parent.type.name!=='paragraph'||$from.parentOffset!==parent.content.size)return null
 const match=/^\/([\w -]{0,24})$/.exec(parent.textContent)
 return match?{query:match[1],from:$from.start(),to:$from.pos}:null
}

const placeholderKey=new PluginKey('md-placeholder')
function placeholderPlugin(text){
 return $prose(()=>new Plugin({key:placeholderKey,props:{decorations:state=>{
  const doc=state.doc
  if(doc.childCount!==1||doc.firstChild.type.name!=='paragraph'||doc.firstChild.content.size)return null
  return DecorationSet.create(doc,[Decoration.node(0,doc.firstChild.nodeSize,{class:'md-placeholder','data-placeholder':text})])
 }}}))
}

// An image is never fetched: only a data: image is drawn, any other source is
// a placeholder that names it. The node, and so its Markdown, is untouched.
const imageView=$view(imageSchema.node,()=>node=>{
 const src=String(node.attrs.src||''),alt=String(node.attrs.alt||'')
 if(/^data:image\//i.test(src)){
  const img=document.createElement('img');img.src=src;img.alt=alt;img.className='md-image-data'
  return {dom:img,ignoreMutation:()=>true}
 }
 const dom=document.createElement('span');dom.className='md-image-placeholder';dom.title=src
 dom.textContent='🖼 '+(alt||'image')+(src?' ('+src+')':'')
 return {dom,ignoreMutation:()=>true}
})

// remark reads a link or an image written without a title as a null title,
// which the schema refuses, and the node is then dropped from the document.
// An empty title is what the schema expects, and it is written back as none.
function untitled(){
 const visit=node=>{
  if((node.type==='link'||node.type==='image')&&node.title==null)node.title=''
  for(const child of node.children||[])visit(child)
 }
 return tree=>visit(tree)
}

const DROPPED='script,style,iframe,object,embed,link,meta,base,form'
function sanitizePastedHTML(html){
 const parsed=new DOMParser().parseFromString(html,'text/html')
 for(const element of parsed.querySelectorAll(DROPPED))element.remove()
 return parsed.body.innerHTML
}

export async function createMarkdownEditor(root,{markdown='',openLink=async()=>{},onChange=()=>{},placeholder=''}={}){
 const wrapper=document.createElement('div');wrapper.className='md-editor'
 const surface=document.createElement('div');surface.className='md-surface'
 const source=document.createElement('textarea');source.className='md-source';source.hidden=true;source.spellcheck=false
 source.setAttribute('aria-label','Description (Markdown source)')
 wrapper.append(surface,source);root.append(wrapper)
 let ctxRef=null
 const notify=()=>onChange()
 // The slash menu and the inline toolbar are plain DOM, kept beside the
 // document rather than in it, and driven from the editor's state.
 const slash=document.createElement('div');slash.className='md-slash';slash.setAttribute('role','listbox');slash.setAttribute('aria-label','Insert a block');slash.hidden=true
 const toolbar=document.createElement('div');toolbar.className='md-toolbar';toolbar.setAttribute('role','toolbar');toolbar.setAttribute('aria-label','Format');toolbar.hidden=true
 const handle=document.createElement('div');handle.className='md-block-handle'
 wrapper.append(slash,toolbar)
 let slashState={open:false,dismissed:null,active:0,items:[]}
 const applySlash=item=>{
  const view=ctxRef.get(editorViewCtx),found=slashQuery(view.state)
  if(found)view.dispatch(view.state.tr.delete(found.from,found.to))
  item.run(ctxRef);closeSlash();view.focus()
 }
 const closeSlash=()=>{slashState.open=false;slash.hidden=true}
 const drawSlash=view=>{
  slash.replaceChildren()
  if(!slashState.items.length){const empty=document.createElement('div');empty.className='md-slash-empty';empty.textContent='No matching block';slash.append(empty)}
  for(const [index,item] of slashState.items.entries()){
   const option=document.createElement('div');option.className='md-slash-option';option.setAttribute('role','option');option.textContent=item.label
   option.setAttribute('aria-selected',String(index===slashState.active))
   option.onmousedown=event=>{event.preventDefault();applySlash(item)}
   slash.append(option)
  }
  const coords=view.coordsAtPos(view.state.selection.from),box=wrapper.getBoundingClientRect()
  slash.style.left=Math.max(0,coords.left-box.left)+'px';slash.style.top=(coords.bottom-box.top+4)+'px'
 }
 const slashPlugin=$prose(()=>new Plugin({
  view:()=>({update:view=>{
   const found=slashQuery(view.state)
   if(!found||!view.hasFocus()){closeSlash();if(!found)slashState.dismissed=null;return}
   if(slashState.dismissed!==null&&slashState.dismissed===found.from)return
   const items=slashMatches(found.query)
   if(!slashState.open||items.length!==slashState.items.length)slashState.active=0
   slashState={...slashState,open:true,dismissed:null,items};slash.hidden=false;drawSlash(view)
  }}),
  props:{handleKeyDown:(view,event)=>{
   if(!slashState.open)return false
   const count=slashState.items.length
   if(event.key==='ArrowDown'||event.key==='ArrowUp'){if(count){slashState.active=(slashState.active+(event.key==='ArrowDown'?1:count-1))%count;drawSlash(view)}return true}
   if(event.key==='Enter'){const item=slashState.items[slashState.active];if(item){applySlash(item);return true}return false}
   if(event.key==='Escape'){slashState.dismissed=slashQuery(view.state)?.from??null;closeSlash();return true}
   return false
  }}
 }))
 // The inline toolbar: the marks of a selection, and the link editor that
 // Mod-K opens in its place.
 const linkInput=document.createElement('input');linkInput.className='md-link-input';linkInput.placeholder='Paste a link';linkInput.setAttribute('aria-label','Link address')
 const linkForm=document.createElement('form');linkForm.className='md-link-form';linkForm.hidden=true;linkForm.append(linkInput)
 const marks=[
  ['Bold','B',toggleStrongCommand],['Italic','I',toggleEmphasisCommand],['Strikethrough','S',toggleStrikethroughCommand],['Inline code','</>',toggleInlineCodeCommand],
 ]
 const buttons=document.createElement('div');buttons.className='md-toolbar-buttons'
 for(const [label,text,command] of marks){
  const button=document.createElement('button');button.type='button';button.textContent=text;button.title=label;button.setAttribute('aria-label',label)
  button.onmousedown=event=>{event.preventDefault();ctxRef.get(commandsCtx).call(command.key)}
  buttons.append(button)
 }
 const linkButton=document.createElement('button');linkButton.type='button';linkButton.textContent='Link';linkButton.title='Link';linkButton.setAttribute('aria-label','Link')
 linkButton.onmousedown=event=>{event.preventDefault();openLinkEditor()}
 buttons.append(linkButton);toolbar.append(buttons,linkForm)
 let linkRange=null
 const placeToolbar=view=>{
  const {from,to}=view.state.selection,start=view.coordsAtPos(from),end=view.coordsAtPos(to),box=wrapper.getBoundingClientRect()
  toolbar.style.left=Math.max(0,Math.min(start.left,end.left)-box.left)+'px';toolbar.style.top=Math.max(0,start.top-box.top-40)+'px'
 }
 function openLinkEditor(){
  const view=ctxRef.get(editorViewCtx),{from,to,empty}=view.state.selection
  if(empty)return false
  const type=linkSchema.type(ctxRef)
  let current=''
  view.state.doc.nodesBetween(from,to,node=>{const mark=type.isInSet(node.marks);if(mark&&!current)current=mark.attrs.href})
  linkRange={from,to};linkInput.value=current;buttons.hidden=true;linkForm.hidden=false;toolbar.hidden=false;placeToolbar(view)
  linkInput.focus();linkInput.select();return true
 }
 const closeLinkEditor=(focus=true)=>{
  linkRange=null;linkForm.hidden=true;buttons.hidden=false
  if(focus)ctxRef.get(editorViewCtx).focus()
 }
 linkForm.onsubmit=event=>{
  event.preventDefault()
  const view=ctxRef.get(editorViewCtx),range=linkRange,href=linkInput.value.trim()
  if(!range){closeLinkEditor();return}
  view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc,range.from,range.to)))
  const type=linkSchema.type(ctxRef)
  if(!href)view.dispatch(view.state.tr.removeMark(range.from,range.to,type))
  else{
   let linked=false
   view.state.doc.nodesBetween(range.from,range.to,node=>{if(type.isInSet(node.marks))linked=true})
   if(linked)view.dispatch(view.state.tr.removeMark(range.from,range.to,type).addMark(range.from,range.to,type.create({href})))
   else ctxRef.get(commandsCtx).call(toggleLinkCommand.key,{href})
  }
  closeLinkEditor()
 }
 linkInput.onkeydown=event=>{if(event.key==='Escape'){event.preventDefault();event.stopPropagation();closeLinkEditor()}}
 linkInput.onblur=event=>{if(!toolbar.contains(event.relatedTarget))closeLinkEditor(false)}
 const toolbarPlugin=$prose(()=>new Plugin({view:()=>({update:view=>{
  if(linkRange)return
  const {selection}=view.state
  const show=!selection.empty&&!(selection instanceof NodeSelection)&&view.hasFocus()&&!selection.$from.parent.type.spec.code
  toolbar.hidden=!show
  if(show)placeToolbar(view)
 }})}))
 const shortcuts=$prose(ctx=>keymap({
  'Mod-Shift-x':()=>ctx.get(commandsCtx).call(toggleStrikethroughCommand.key),
  'Mod-k':()=>openLinkEditor(),
 }))
 // Links never navigate the window: Cmd/Ctrl+click opens a web or mail link
 // through the opener the page passes, and any other link not at all.
 const linkClicks=$prose(()=>new Plugin({props:{
  handleDOMEvents:{click:(view,event)=>{
   const anchor=event.target.closest?.('a[href]')
   if(!anchor||!view.dom.contains(anchor))return false
   event.preventDefault()
   const href=anchor.getAttribute('href')
   if((event.metaKey||event.ctrlKey)&&isExternalLink(href))openLink(href)
   return true
  }},
  transformPastedHTML:sanitizePastedHTML,
 }}))
 const changes=$prose(()=>new Plugin({view:()=>({update:(view,previous)=>{if(!view.state.doc.eq(previous.doc))notify()}})}))
 const editor=await Editor.make()
  .config(ctx=>{
   ctx.set(rootCtx,surface)
   ctx.set(defaultValueCtx,markdown)
   ctx.update(remarkPluginsCtx,plugins=>[...plugins,{plugin:untitled,options:{}}])
   // Lists are written with "-", as GitHub, GitLab and the web board write them.
   ctx.update(remarkStringifyOptionsCtx,options=>({...options,bullet:'-'}))
   ctx.update(editorViewOptionsCtx,options=>({...options,attributes:{class:'md-document','aria-label':'Description','aria-multiline':'true',role:'textbox',spellcheck:'true'}}))
  })
  .use(commonmark).use(gfm).use(history).use(clipboard).use(indent).use(block)
  .use(imageView).use(placeholderPlugin(placeholder)).use(slashPlugin).use(toolbarPlugin).use(shortcuts).use(linkClicks).use(changes)
  .create()
 ctxRef=editor.ctx
 const view=()=>editor.ctx.get(editorViewCtx)
 // Focus leaving the document hides its menus, unless it went into them.
 view().dom.addEventListener('focusout',event=>{if(!wrapper.contains(event.relatedTarget)){closeSlash();if(!linkRange)toolbar.hidden=true}})
 // The block handle: a grip to drag the hovered block, and a + that opens a
 // block below it with the slash menu ready.
 const add=document.createElement('button');add.type='button';add.className='md-block-add';add.textContent='+';add.title='Insert a block below';add.setAttribute('aria-label','Insert a block below')
 const grip=document.createElement('span');grip.className='md-block-grip';grip.textContent='⋮⋮';grip.title='Drag to move';grip.setAttribute('aria-hidden','true')
 handle.append(add,grip)
 const provider=new BlockProvider({ctx:editor.ctx,content:handle,root:wrapper,getOffset:()=>8,getPlacement:()=>'left'})
 add.onclick=()=>{
  const active=provider.active,current=view()
  if(!active)return
  const after=active.$pos.pos+active.node.nodeSize,paragraph=paragraphSchema.type(editor.ctx).create(null,current.state.schema.text('/'))
  const tr=current.state.tr.insert(after,paragraph)
  current.dispatch(tr.setSelection(TextSelection.create(tr.doc,after+2)));current.focus()
 }
 provider.update()
 const parse=text=>editor.ctx.get(parserCtx)(text)
 const serialize=doc=>editor.ctx.get(serializerCtx)(doc)
 // The text a document was last parsed from, and that document. Until the
 // document moves away from it, that text is the Markdown to send.
 let anchor={text:markdown,doc:view().state.doc}
 let baseline=view().state.doc
 let sourceOn=false,enteredSource='',changedBeforeEntering=false,stateBeforeSource=null
 const replaceDocument=doc=>{const current=view();current.dispatch(current.state.tr.replaceWith(0,current.state.doc.content.size,doc.content))}
 const currentMarkdown=()=>{const doc=view().state.doc;return doc.eq(anchor.doc)?anchor.text:serialize(doc)}
 source.addEventListener('input',notify)
 const grow=()=>{source.style.height='auto';source.style.height=source.scrollHeight+'px'}
 source.addEventListener('input',grow)
 return {
  markdown:()=>sourceOn?source.value:currentMarkdown(),
  changed:()=>sourceOn?source.value!==enteredSource||changedBeforeEntering:!view().state.doc.eq(baseline),
  sourceMode:()=>sourceOn,
  setSourceMode(on){
   if(on===sourceOn)return
   const current=view()
   if(on){
    changedBeforeEntering=!current.state.doc.eq(baseline);stateBeforeSource=current.state
    enteredSource=currentMarkdown();source.value=enteredSource
    surface.hidden=true;source.hidden=false;sourceOn=true;closeSlash();toolbar.hidden=true;provider.hide();grow();source.focus()
    return
   }
   sourceOn=false;source.hidden=true;surface.hidden=false
   // An untouched source gives the document back as it was, history included.
   if(stateBeforeSource&&source.value===enteredSource)current.updateState(stateBeforeSource)
   else{replaceDocument(parse(source.value));anchor={text:source.value,doc:view().state.doc}}
   stateBeforeSource=null;notify();current.focus()
  },
  // rebase makes what the editor holds the new baseline, once it is saved.
  rebase(){
   if(sourceOn){
    baseline=parse(source.value);anchor={text:source.value,doc:baseline}
    enteredSource=source.value;changedBeforeEntering=false;stateBeforeSource=null
    return
   }
   anchor={text:currentMarkdown(),doc:view().state.doc};baseline=anchor.doc
  },
  // reset loads another description, as the server stored it.
  reset(text){
   replaceDocument(parse(text));baseline=view().state.doc;anchor={text,doc:baseline}
   if(sourceOn){source.value=text;enteredSource=text;changedBeforeEntering=false;stateBeforeSource=null;grow()}
  },
  focus:()=>sourceOn?source.focus():view().focus(),
  destroy(){provider.destroy();editor.destroy();wrapper.remove()},
 }
}
