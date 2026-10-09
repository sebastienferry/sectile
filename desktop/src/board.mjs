import { STAGES, taskStage } from './workflow.mjs'
import { orderedTasks, DEFAULT_SORT } from './task-list-order.mjs'
import { STAGE_LABELS } from '../../shared/workflowStage.mjs'
import { epicColorHex } from '../../shared/epicColor.mjs'

// The project board (#806): one project's tasks in the six workflow columns.
// These helpers hold its rules; main.js only renders them.

// What a project reads as when the agent sends no board data (an older agent):
// no mapping, so tasks are placed from their labels and status alone, and no
// macro colour.
export const EMPTY_BOARD=Object.freeze({epicColors:false,trackerColumns:[],stageColumns:{},trackers:[]})

// The two display options, stored on this workstation once for every project
// and independent of the web board's own.
const HIDE_FINISHED_KEY='boardHideFinished',CARD_DISPLAY_KEY='boardCardDisplay'
export const CARD_DISPLAYS=['condensed','full']

function read(storage,key){try{return storage?.getItem(key)??null}catch{return null}}

// The finished column starts collapsed and cards start condensed.
export function boardOptions(storage){
 const display=read(storage,CARD_DISPLAY_KEY)
 return {
  hideFinished:read(storage,HIDE_FINISHED_KEY)!=='false',
  cardDisplay:CARD_DISPLAYS.includes(display)?display:'condensed'
 }
}

// Saves one option. A storage that refuses the write leaves the choice to the
// current session only.
export function saveBoardOption(storage,option,value){
 try{
  if(option==='hideFinished')storage?.setItem(HIDE_FINISHED_KEY,String(Boolean(value)))
  else if(option==='cardDisplay'&&CARD_DISPLAYS.includes(value))storage?.setItem(CARD_DISPLAY_KEY,value)
 }catch{}
}

// The six columns in workflow order, each with its tasks ordered as the
// tickets list orders them by default. A task whose stage reads as none of
// the six lands in New, so every task shows exactly once.
export function boardColumns(tasks,board){
 const columns=STAGES.map(stage=>({stage,tasks:[]}))
 for(const task of tasks){
  const stage=taskStage(task,board)
  ;(columns.find(column=>column.stage===stage)||columns[0]).tasks.push(task)
 }
 for(const column of columns)column.tasks=orderedTasks(column.tasks,DEFAULT_SORT)
 return columns
}

// The labels a full card shows: its own, without the workflow stage ones the
// column already says.
export function boardCardLabels(task){
 const workflow=new Set([...STAGE_LABELS,'done'])
 return (task.labels||[]).filter(label=>String(label).trim()&&!workflow.has(String(label).trim().replace(/^#+/,'').toLowerCase()))
}

// The colour bar of a card: its macro's, the web board's colour for the same
// parent key, when the project enables macro colours.
export function cardEpicColor(task,board){
 return board?.epicColors===true?epicColorHex(task.parentKey):null
}
