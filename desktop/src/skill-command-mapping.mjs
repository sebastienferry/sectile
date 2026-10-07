// A fixed mapping keeps workflow IDs out of editable fields.
export function skillCommandMapping({validate,onChange}){
 const box=document.createElement('div');box.className='skill-command-mapping'
 let catalogue=[],entries=[]
 return {box,
  set(values={},skills=catalogue){
   catalogue=skills;entries=[];box.replaceChildren()
   const table=document.createElement('table');table.setAttribute('aria-label','Skill command mapping')
   const head=document.createElement('thead'),header=document.createElement('tr')
   for(const label of ['Step','Standard command','Custom command']){const cell=document.createElement('th');cell.scope='col';cell.textContent=label;header.append(cell)}
   head.append(header);table.append(head)
   const body=document.createElement('tbody')
   const all=[...catalogue,...Object.keys(values).filter(id=>!catalogue.some(skill=>skill.id===id)).map(id=>({id,name:id,command:''}))]
   for(const skill of all){
    const row=document.createElement('tr'),name=document.createElement('th');name.scope='row';name.textContent=skill.name||skill.id
    const standard=document.createElement('td'),code=document.createElement('code');code.textContent=skill.command||'Project command';standard.append(code)
    const custom=document.createElement('td'),input=document.createElement('input');input.type='text';input.value=values[skill.id]||'';input.placeholder=skill.command||'Standard command';input.setAttribute('aria-label','Command for skill '+skill.id)
    input.oninput=()=>{if(validate&&!validate(input.value))input.setAttribute('aria-invalid','true');else input.removeAttribute('aria-invalid');onChange?.()}
    custom.append(input);row.append(name,standard,custom);body.append(row);entries.push({id:skill.id,input})
   }
   table.append(body);box.append(table)
  },
  get(){return Object.fromEntries(entries.filter(({input})=>input.value.trim()).map(({id,input})=>[id,input.value.trim()]))},
  invalid(){return entries.some(({input})=>validate&&!validate(input.value))}
 }
}
