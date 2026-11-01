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
    ["",["free"]],["À clarifier",["new"]],["Revu",["reviewed"]],["Étape inconnue",["unknown"]],
  ]);
});

test("conversation navigation keeps remote statuses and recent ordering",()=>{
  const runs=[run("first","a"),run("second","b"),run("free","")];
  const tasks=[{id:"a",title:"First",status:"to_clarify",source:"jira",trackerStatus:"Open"},{id:"b",title:"Second",status:"to_close",source:"jira",trackerStatus:"Done"}];
  assert.deepEqual(conversationGroups(runs,tasks,"tracker").map(group=>group.label),["","Open","Done"]);
  assert.deepEqual(conversationGroups(runs,tasks,"recent")[0].runs.map(item=>item.id),["first","second","free"]);
});

test("conversation navigation sorts by creation date and by ticket ID", () => {
  const older: Run = { id: "old", projectId: "p", taskId: "t1", taskKey: "#10", skill: "", status: "completed", createdAt: "2026-10-01T10:00:00Z", conversation: true };
  const newer: Run = { id: "new", projectId: "p", taskId: "t2", taskKey: "#2", skill: "", status: "running", createdAt: "2026-10-15T10:00:00Z", conversation: true };
  const newest: Run = { id: "latest", projectId: "p", taskId: "t3", taskKey: "#100", skill: "", status: "running", createdAt: "2026-10-20T10:00:00Z", conversation: true };

  const byCreated = conversationGroups([older, newest, newer], [], "created");
  assert.equal(byCreated[0].label, "");
  assert.deepEqual(byCreated[0].runs.map(r => r.id), ["latest", "new", "old"]);

  const byId = conversationGroups([newest, older, newer], [], "id");
  assert.equal(byId[0].label, "");
  assert.deepEqual(byId[0].runs.map(r => r.id), ["new", "old", "latest"]);
});
