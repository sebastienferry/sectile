// Search the settings already loaded for this workstation or project. Controls
// stay in their original panels, retaining their values and save handlers.
export function installSettingsSearch({tabs,content,panels,categories,selectCategory}){
 const input=document.createElement('input');input.type='search';input.placeholder='Search settings…';input.setAttribute('aria-label','Search settings');input.className='settings-search'
 tabs.querySelector('.configuration-back').after(input)
 const notice=document.createElement('p');notice.className='settings-search-status';notice.setAttribute('role','status');notice.hidden=true;content.prepend(notice)
 const title=content.querySelector('.configuration-panel-title')
 const normalize=value=>value.normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLowerCase()
 const original=new Map(),headings=new Map()
 let previous=null
 const observer=new MutationObserver(()=>{if(input.value.trim())apply()})
 const observe=()=>observer.observe(content,{childList:true,subtree:true,characterData:true})
 function restore(){
  for(const [row,hidden] of original)if(row.isConnected)row.hidden=hidden
  original.clear()
  for(const heading of headings.values())heading.hidden=true
  notice.hidden=true
 }
 function apply(){
  observer.disconnect()
  const words=normalize(input.value.trim()).split(/\s+/).filter(Boolean)
  if(!words.length){restore();if(previous)selectCategory(previous);previous=null;observe();return}
  if(!previous)previous=tabs.querySelector('[data-category][aria-selected="true"]')?.dataset.category
  const matches=text=>words.every(word=>normalize(text).includes(word))
  let count=0
  for(const category of categories){
   const panel=panels[category.id];if(!panel)continue
   let heading=headings.get(panel)
   if(!heading){heading=document.createElement('h3');heading.className='settings-search-heading';heading.textContent=category.label;panel.prepend(heading);headings.set(panel,heading)}
   const rows=[...panel.querySelectorAll('.setting-row')]
   const categoryMatch=matches(category.label)
   const matching=rows.filter(row=>matches(row.textContent+' '+[...row.querySelectorAll('[aria-label]')].map(el=>el.getAttribute('aria-label')).join(' ')))
   const panelMatch=categoryMatch||matching.length>0||matches(panel.textContent)
   for(const row of rows){if(!original.has(row))original.set(row,row.hidden);row.hidden=original.get(row)||(!categoryMatch&&matching.length>0&&!matching.includes(row))}
   panel.hidden=!panelMatch;heading.hidden=!panelMatch
   if(panelMatch)count++
  }
  title.textContent='Search results';notice.hidden=false;notice.textContent=count?count+' settings section'+(count===1?'':'s')+' found':'No settings found'
  observe()
 }
 input.addEventListener('input',apply)
 tabs.addEventListener('click',event=>{if(event.target.closest('[role="tab"]')&&input.value){input.value='';apply()}},true)
 observe()
 return ()=>observer.disconnect()
}
