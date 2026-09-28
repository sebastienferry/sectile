// The editors the Execution defaults panel offers (#535). The stored value is
// the plain `editorCommand` string: a preset stores its command, None stores
// nothing, and any other command is a custom one.

export const EDITORS=[
 {id:'',label:'None'},
 {id:'code',label:'VS Code'},
 {id:'cursor',label:'Cursor'},
 {id:'zed',label:'Zed'},
 {id:'subl',label:'Sublime Text'},
 {id:'custom',label:'Custom command…'}
]

const PRESETS=EDITORS.filter(editor=>editor.id&&editor.id!=='custom')

// editorChoice is what the picker shows for a stored command: the preset it
// names exactly, else the custom entry holding the command as it was written.
export function editorChoice(value){
 const command=String(value??'').trim()
 if(!command)return {select:'',custom:''}
 if(PRESETS.some(editor=>editor.id===command))return {select:command,custom:''}
 return {select:'custom',custom:command}
}

// editorLabel names the editor in "Open in <label>": the preset's name, else
// the program a custom command starts, without its path or arguments.
export function editorLabel(value){
 const command=String(value??'').trim()
 const preset=PRESETS.find(editor=>editor.id===command)
 if(preset)return preset.label
 const program=command.split(/\s+/)[0]||''
 return program.split(/[\\/]/).pop()||program
}
