// A mapping table to configure AI model overrides per workflow skill.
export function skillModelMapping({validate,onChange}){
 const box=document.createElement('div');box.className='skill-model-mapping'
 let catalogue=[],entries=[]
 const datalistId='skill-model-options-'+Math.random().toString(36).slice(2,8)

 const baseRow=document.createElement('div')
 baseRow.className='skill-model-base-row'
 const baseLabel=document.createElement('label')
 baseLabel.textContent='Engine default model:'
 const baseInput=document.createElement('input')
 baseInput.type='text'
 baseInput.setAttribute('list',datalistId)
 baseInput.setAttribute('aria-label','Engine default model')
 baseInput.placeholder='Provider default'
 baseRow.append(baseLabel,baseInput)

 const tableContainer=document.createElement('div')
 tableContainer.className='skill-model-table-container'

 function updateFallbacks(){
  const fallback=baseInput.value.trim()||'Provider default'
  for(const {code,input} of entries){
   code.textContent=fallback
   input.placeholder='Inherit ('+fallback+')'
  }
 }

 baseInput.oninput=()=>{
  if(validate&&!validate(baseInput.value))baseInput.setAttribute('aria-invalid','true')
  else baseInput.removeAttribute('aria-invalid')
  updateFallbacks()
  onChange?.()
 }

 return {
  box,
  set(values={},skills=catalogue,{engineModel='',suggestedModels=[],disabled=false}={}){
   catalogue=skills;entries=[];box.replaceChildren()

   const datalist=document.createElement('datalist')
   datalist.id=datalistId
   for(const m of suggestedModels){
    if(!m)continue
    const opt=document.createElement('option')
    opt.value=m
    datalist.append(opt)
   }
   box.append(datalist)

   baseInput.value=engineModel||''
   baseInput.disabled=!!disabled
   baseInput.removeAttribute('aria-invalid')
   box.append(baseRow)

   tableContainer.replaceChildren()
   const table=document.createElement('table');table.setAttribute('aria-label','Skill model mapping')
   const head=document.createElement('thead'),header=document.createElement('tr')
   for(const label of ['Step','Engine model','Skill model']){
    const cell=document.createElement('th');cell.scope='col';cell.textContent=label;header.append(cell)
   }
   head.append(header);table.append(head)

   const body=document.createElement('tbody')
   const fallbackText=engineModel?engineModel:'Provider default'
   const all=[...catalogue,...Object.keys(values).filter(id=>!catalogue.some(skill=>skill.id===id)).map(id=>({id,name:id}))]

   for(const skill of all){
    const row=document.createElement('tr')
    const name=document.createElement('th');name.scope='row';name.textContent=skill.name||skill.id
    const standard=document.createElement('td'),code=document.createElement('code')
    code.textContent=fallbackText
    standard.append(code)

    const custom=document.createElement('td'),input=document.createElement('input')
    input.type='text'
    input.setAttribute('list',datalistId)
    input.value=values[skill.id]||''
    input.placeholder='Inherit ('+fallbackText+')'
    input.setAttribute('aria-label','Model for skill '+skill.id)
    input.disabled=!!disabled
    input.oninput=()=>{
     if(validate&&!validate(input.value))input.setAttribute('aria-invalid','true')
     else input.removeAttribute('aria-invalid')
     onChange?.()
    }
    custom.append(input)
    row.append(name,standard,custom)
    body.append(row)
    entries.push({id:skill.id,code,input})
   }
   table.append(body)
   tableContainer.append(table)
   box.append(tableContainer)
  },
  getEngineModel(){
   return baseInput.value.trim()
  },
  get(){
   return Object.fromEntries(entries.filter(({input})=>input.value.trim()).map(({id,input})=>[id,input.value.trim()]))
  },
  invalid(){
   if(validate&&!validate(baseInput.value))return true
   return entries.some(({input})=>validate&&!validate(input.value))
  }
 }
}
