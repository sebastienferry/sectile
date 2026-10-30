import assert from "node:assert/strict";
import test from "node:test";
import { conversationGroups } from "../src/domain/conversation-navigation.ts";
import type { Run } from "../src/domain/execution.ts";

const run = (id:string, taskId:string):Run => ({id,projectId:"p",taskId,taskKey:taskId,skill:"",status:"running",createdAt:"2026-10-30",conversation:true});

test("conversation navigation groups linked tickets by workflow stage without inventing missing stages",()=>{
  const groups=conversationGroups(
    [run("free",""),run("reviewed","r"),run("unknown","u"),run("new","n")],
    [{id:"r",title:"Reviewed",status:"to_close",source:"jira",trackerStatus:"Done"},{id:"u",title:"Unknown",status:"external",source:"jira",trackerStatus:"Open"},{id:"n",title:"New",status:"to_clarify",source:"jira",trackerStatus:"Open"}],
    "stage",
  );
  assert.deepEqual(groups.map(group=>[group.label,group.runs.map(item=>item.id)]),[
    ["À clarifier",["new"]],["Revu",["reviewed"]],["Prompts libres",["free"]],["Étape inconnue",["unknown"]],
  ]);
});

test("conversation navigation keeps remote statuses and recent ordering",()=>{
  const runs=[run("first","a"),run("second","b"),run("free","")];
  const tasks=[{id:"a",title:"First",status:"to_clarify",source:"jira",trackerStatus:"Open"},{id:"b",title:"Second",status:"to_close",source:"jira",trackerStatus:"Done"}];
  assert.deepEqual(conversationGroups(runs,tasks,"tracker").map(group=>group.label),["Open","Done","Prompts libres"]);
  assert.deepEqual(conversationGroups(runs,tasks,"recent")[0].runs.map(item=>item.id),["first","second","free"]);
});
