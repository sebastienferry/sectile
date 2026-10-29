import test from "node:test";
import assert from "node:assert/strict";
import {nextSkill,skillResultLabel,taskStage} from "../src/domain/workflow.ts";
const task={id:"t",title:"Ticket",status:"to_clarify",labels:[]};
const project={configured:true,skills:["clarify","specify","implement","adjust","handoff","pickup"].map(id=>({id})),policy:{prCreationStage:"implemented"},board:{trackerColumns:[{name:"Review",statuses:["Code review"]}],stageColumns:{implemented:["Review"]},trackers:[{trackerId:"other",trackerColumns:[{name:"Doing",statuses:["WIP"]}],stageColumns:{specified:["Doing"]}}]}};
test("workflow stage preserves finished status, labels and tracker-specific mapping precedence",()=>{
 assert.equal(taskStage({...task,status:"finished",labels:["#new"]},project.board),"finished");
 assert.equal(taskStage({...task,trackerStatus:"Code review",labels:["#clarified"]},project.board),"clarified");
 assert.equal(taskStage({...task,trackerStatus:"Code review"},project.board),"implemented");
 assert.equal(taskStage({...task,trackerStatus:"WIP",trackerId:"other"},project.board),"specified");
 assert.equal(taskStage({...task,status:"unknown"},project.board),undefined);
});
test("next skill respects PR creation policy, PR presence, availability and project setup",()=>{
 assert.equal(nextSkill(task,project).skillId,"clarify");
 assert.equal(nextSkill({...task,status:"to_test"},project).skillId,"implement");
 assert.equal(nextSkill({...task,status:"to_test"},{...project,policy:{prCreationStage:"specified"}}).skillId,"specify");
 assert.equal(nextSkill({...task,status:"to_test",prUrl:"https://example.test/pr/1"},project).skillId,"adjust");
 assert.equal(nextSkill({...task,status:"to_close"},project).skillId,"handoff");
 assert.equal(nextSkill(task,{...project,configured:false}).skillId,undefined);
 assert.equal(nextSkill(task,{...project,skills:[]}).skillId,undefined);
 assert.equal(nextSkill({...task,status:"finished"},project).skillId,undefined);
});
test("process exit never substitutes for server-confirmed skill completion or stage validation",()=>{
 const run={id:"r",taskId:"t",skill:"implement",status:"completed"};
 assert.match(skillResultLabel(run),/non confirmé/);
 assert.match(skillResultLabel(run,{activity:{id:"different",taskId:"t",skillId:"implement",status:"completed"}}),/non confirmé/);
 assert.match(skillResultLabel(run,{activity:{id:"r",taskId:"t",skillId:"implement",status:"completed"},task:{status:"to_implement"}}),/validation/);
 assert.match(skillResultLabel(run,{activity:{id:"r",taskId:"t",skillId:"implement",status:"completed"},task:{status:"to_test"}}),/résultat confirmé/);
});
