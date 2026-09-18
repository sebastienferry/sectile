import { test } from 'node:test'
import assert from 'node:assert/strict'
import { orderedTasks, nextSort, compareIdentity, DEFAULT_SORT } from '../src/task-list-order.mjs'

const keys=tasks=>tasks.map(task=>task.key||task.id)

test('default order is priority descending then natural identity ascending',()=>{
 const tasks=[
  {id:'t9',key:'#9',title:'Nine',priority:'medium',status:'to_clarify'},
  {id:'t100',key:'#100',title:'Hundred',priority:'medium',status:'to_clarify'},
  {id:'t12',key:'#12',title:'Twelve',priority:'urgent',status:'to_clarify'},
  {id:'t7',key:'#7',title:'Seven',status:'to_clarify'},
  {id:'t3',key:'#3',title:'Three',priority:'low',status:'to_clarify'},
  {id:'t5',key:'#5',title:'Five',priority:'HIGH',status:'to_clarify'}
 ]
 assert.deepEqual(keys(orderedTasks(tasks)),['#12','#5','#9','#100','#3','#7'])
 assert.deepEqual(keys(orderedTasks(tasks,DEFAULT_SORT)),['#12','#5','#9','#100','#3','#7'])
})

test('identity compares naturally across trackers and falls back to the id',()=>{
 const tasks=[
  {id:'b',key:'PROJ-10',title:'',priority:'low'},
  {id:'a',key:'PROJ-9',title:'',priority:'low'},
  {id:'local-10',title:'',priority:'low'},
  {id:'local-2',title:'',priority:'low'}
 ]
 assert.deepEqual(orderedTasks(tasks).map(task=>task.key||task.id),['local-2','local-10','PROJ-9','PROJ-10'])
 assert.equal(compareIdentity({id:'x',key:'#2'},{id:'y',key:'#2'})<0,true)
 assert.equal(compareIdentity({id:'x',key:'#2'},{id:'x',key:'#2'}),0)
})

test('unknown or missing priority sorts last in the default order and first when ascending',()=>{
 const tasks=[
  {id:'1',key:'#1',priority:'weird'},
  {id:'2',key:'#2',priority:'low'},
  {id:'3',key:'#3'}
 ]
 assert.deepEqual(keys(orderedTasks(tasks)),['#2','#1','#3'])
 assert.deepEqual(keys(orderedTasks(tasks,{field:'priority',ascending:true})),['#1','#3','#2'])
})

test('each sortable column orders in both directions and keeps identity ascending on ties',()=>{
 const tasks=[
  {id:'1',key:'#10',title:'beta',priority:'low',status:'to_implement'},
  {id:'2',key:'#2',title:'Alpha',priority:'high',status:'to_clarify'},
  {id:'3',key:'#3',title:'beta',priority:'medium',labels:['#reviewed']},
  {id:'4',key:'#4',title:'gamma',priority:'urgent',status:'to_specify'}
 ]
 assert.deepEqual(keys(orderedTasks(tasks,{field:'key',ascending:true})),['#2','#3','#4','#10'])
 assert.deepEqual(keys(orderedTasks(tasks,{field:'key',ascending:false})),['#10','#4','#3','#2'])
 assert.deepEqual(keys(orderedTasks(tasks,{field:'title',ascending:true})),['#2','#3','#10','#4'])
 assert.deepEqual(keys(orderedTasks(tasks,{field:'title',ascending:false})),['#4','#3','#10','#2'])
 assert.deepEqual(keys(orderedTasks(tasks,{field:'stage',ascending:true})),['#2','#4','#10','#3'])
 assert.deepEqual(keys(orderedTasks(tasks,{field:'stage',ascending:false})),['#3','#10','#4','#2'])
 assert.deepEqual(keys(orderedTasks(tasks,{field:'priority',ascending:true})),['#10','#3','#2','#4'])
})

test('unknown stages sort after the workflow stages',()=>{
 const tasks=[
  {id:'1',key:'#1',status:'mystery'},
  {id:'2',key:'#2',status:'to_close'},
  {id:'3',key:'#3',status:'to_clarify'}
 ]
 assert.deepEqual(keys(orderedTasks(tasks,{field:'stage',ascending:true})),['#3','#2','#1'])
})

test('ordering does not mutate the input',()=>{
 const tasks=[{id:'2',key:'#2',priority:'low'},{id:'1',key:'#1',priority:'urgent'}]
 const copy=[...tasks]
 orderedTasks(tasks)
 assert.deepEqual(tasks,copy)
})

test('nextSort starts ascending except for priority and flips the same column',()=>{
 assert.deepEqual(nextSort(DEFAULT_SORT,'title'),{field:'title',ascending:true})
 assert.deepEqual(nextSort({field:'title',ascending:true},'title'),{field:'title',ascending:false})
 assert.deepEqual(nextSort({field:'title',ascending:false},'title'),{field:'title',ascending:true})
 assert.deepEqual(nextSort({field:'key',ascending:true},'priority'),{field:'priority',ascending:false})
 assert.deepEqual(nextSort(DEFAULT_SORT,'priority'),{field:'priority',ascending:true})
 assert.deepEqual(nextSort(null,'stage'),{field:'stage',ascending:true})
})
