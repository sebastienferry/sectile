import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor,within} from "@testing-library/react";
import ProjectPrompt from "../src/components/ProjectPrompt";
import {WorkflowTask} from "../src/components/WorkflowTask";
import App from "../src/App";
import type {Run} from "../src/domain/execution";
import type {RunResult} from "../src/domain/workflow";
import * as conversation from "../src/lib/conversation";
import * as workspace from "../src/lib/workspace";
import * as api from "../src/lib/agent";
vi.mock("@tauri-apps/api/core",()=>({invoke:vi.fn()}));
vi.mock("@tauri-apps/api/event",()=>({listen:vi.fn()}));
vi.mock("../src/lib/conversation",async original=>({...await original<typeof conversation>(),createConversation:vi.fn(),readConversation:vi.fn(),controlConversation:vi.fn()}));
vi.mock("../src/lib/workspace",async original=>({...await original<typeof workspace>(),projectContext:vi.fn(),providerReadiness:vi.fn(),runResult:vi.fn(),openExternalLink:vi.fn(),declareReviewed:vi.fn()}));
vi.mock("../src/lib/agent",()=>({
 listProjects:vi.fn(),listTasks:vi.fn(),getTask:vi.fn(),executionSnapshot:vi.fn(),projectSkills:vi.fn(),getOrchestration:vi.fn(),launchTask:vi.fn(),
 stopRun:vi.fn(),runOrchestration:vi.fn(),taskTrackers:vi.fn(),taskIssueTypes:vi.fn(),createTask:vi.fn(),updateTask:vi.fn(),
}));
vi.mock("../src/components/RunConsole",()=>({RunConsole:()=> <div>Live output</div>}));
vi.mock("../src/components/RunReview",()=>({RunReview:()=> <div>Task review</div>}));

const project={configured:true,aiProvider:"claude",skills:[{id:"clarify"},{id:"specify"},{id:"implement"}],policy:{}} as unknown as workspace.ProjectContext;
const task={id:"task",key:"APP-7",title:"Clarified task",status:"to_specify",source:"jira",labels:["#clarified"]};
const clarifyRun:Run={id:"clarify-run",projectId:"p",conversation:true,taskId:"task",taskKey:"APP-7",skill:"clarify",status:"running",provider:"claude",createdAt:"2026-11-15"};
const done:RunResult={activity:{id:"clarify-run",taskId:"task",skillId:"clarify",status:"completed",summary:""},task:{status:"to_specify",labels:["#clarified"]}};
const transcript=(id:string,extra:object={})=>({id,busy:false,readOnly:false,events:[JSON.stringify({kind:"assistant",text:"Clarification written"})],...extra});

beforeEach(()=>{
 vi.resetAllMocks();
 vi.mocked(conversation.readConversation).mockImplementation(async id=>transcript(id));
 vi.mocked(conversation.controlConversation).mockResolvedValue({accepted:true});
 vi.mocked(workspace.projectContext).mockResolvedValue(project);
 vi.mocked(workspace.providerReadiness).mockResolvedValue({provider:"claude",complete:true,steps:{skills:{status:"success"},mcp:{status:"success"}}} as any);
 vi.mocked(workspace.runResult).mockResolvedValue(done);
 vi.stubGlobal("matchMedia",()=>({matches:false,addEventListener(){},removeEventListener(){}}));
});
afterEach(()=>{cleanup();vi.unstubAllGlobals();localStorage.clear();});

test("the conversation strip shows the skill status, and its panel the run state and diagnostics",async()=>{
 render(<ProjectPrompt projectId="p" runId="clarify-run" run={clarifyRun} task={task} projectContext={project} onCreated={vi.fn()}/>);
 await screen.findByText("Clarification written");
 const strip=screen.getByLabelText("Conversation status");
 const summary=await within(strip).findByText("Done · Clarified · Next: Specify");
 expect(summary.closest("summary")?.getAttribute("title")).toBe("Done · Clarified\nNext: Specify\nExecution: Running · Skill record: completed · Task stage: clarified");
 expect(summary.closest("summary")?.querySelector(".skill-status-mark.skill-done")).not.toBeNull();
 expect(strip.textContent).not.toContain("Execution: Active session · ");
 fireEvent.click(summary);
 expect(within(strip).getByText("Execution: Active session")).toBeTruthy();
 expect(within(strip).getByText("Execution: Running · Skill record: completed · Task stage: clarified")).toBeTruthy();
 expect(strip.textContent).toContain("Ready to respond");
});

test("a failed skill shows its short reason in the strip and the full reason in the panel",async()=>{
 vi.mocked(workspace.runResult).mockResolvedValue({...done,activity:{...done.activity!,status:"failed",error:"git push rejected\nhint: fetch first"}});
 render(<ProjectPrompt projectId="p" runId="clarify-run" run={clarifyRun} task={task} projectContext={project} onCreated={vi.fn()}/>);
 const strip=screen.getByLabelText("Conversation status");
 expect(await within(strip).findByText("Could not finish · git push rejected")).toBeTruthy();
 expect(strip.querySelector(".conversation-execution-reason")?.textContent).toBe("git push rejected\nhint: fetch first");
 expect(strip.querySelector("details.skill-status-failed")).not.toBeNull();
});

test("a pending question in a ticket conversation reads as needing an answer",async()=>{
 vi.mocked(workspace.runResult).mockResolvedValue({activity:{id:"clarify-run",taskId:"task",skillId:"clarify",status:"running"},task:{status:"to_clarify"}});
 vi.mocked(conversation.readConversation).mockImplementation(async id=>transcript(id,{approvals:[{id:"q1",kind:"question",questions:[{question:"Which scope?",options:[]}]}]}));
 render(<ProjectPrompt projectId="p" runId="clarify-run" run={clarifyRun} task={task} projectContext={project} onCreated={vi.fn()}/>);
 const strip=screen.getByLabelText("Conversation status");
 expect(await within(strip).findByText("Needs your answer · Answer in the console")).toBeTruthy();
});

test("the end of a conversation turn reads the skill result again",async()=>{
 let busy=true;
 vi.mocked(conversation.readConversation).mockImplementation(async id=>transcript(id,{busy}));
 vi.mocked(workspace.runResult).mockResolvedValue({activity:{id:"clarify-run",taskId:"task",skillId:"clarify",status:"running"},task:{status:"to_clarify"}});
 render(<ProjectPrompt projectId="p" runId="clarify-run" run={clarifyRun} task={task} projectContext={project} onCreated={vi.fn()}/>);
 const strip=screen.getByLabelText("Conversation status");
 expect(await within(strip).findByText("Working · Clarify")).toBeTruthy();
 expect(workspace.runResult).toHaveBeenCalledTimes(1);
 vi.mocked(workspace.runResult).mockResolvedValue(done);
 busy=false;
 expect(await within(strip).findByText("Done · Clarified · Next: Specify",{},{timeout:3000})).toBeTruthy();
 expect(workspace.runResult).toHaveBeenCalledTimes(2);
});

test("a free conversation keeps its run state and never reads a skill result",async()=>{
 const free:Run={id:"free-chat",projectId:"p",conversation:true,taskId:"",skill:"",status:"running",createdAt:"2026-11-15"};
 render(<ProjectPrompt projectId="p" runId="free-chat" run={free} onCreated={vi.fn()}/>);
 await screen.findByText("Clarification written");
 const strip=screen.getByLabelText("Conversation status");
 expect(strip.textContent).toContain("Execution: Active session");
 expect(strip.querySelector(".conversation-execution-details")).toBeNull();
 const discuss:Run={...clarifyRun,id:"discuss-chat",skill:"discuss"};
 cleanup();
 render(<ProjectPrompt projectId="p" runId="discuss-chat" run={discuss} task={task} projectContext={project} onCreated={vi.fn()}/>);
 await screen.findByText("Clarification written");
 expect(screen.getByLabelText("Conversation status").textContent).toContain("Execution: Active session");
 expect(workspace.runResult).not.toHaveBeenCalled();
});

const monitor={snapshot:{status:{connected:true},runs:[] as Run[]},fresh:true,refresh:vi.fn(async()=>{})};
vi.mock("../src/components/ExecutionMonitor",async original=>({...await original<object>(),useExecutionMonitor:()=>monitor}));

test("the workflow panel shows the same status as the strip, from a single result read",async()=>{
 monitor.snapshot.runs=[clarifyRun];
 vi.mocked(api.listTasks).mockResolvedValue([task]);
 vi.mocked(api.executionSnapshot).mockResolvedValue({status:{connected:true},runs:[clarifyRun]});
 render(<WorkflowTask projectId="p" taskId="task" onBack={()=>{}} onSetup={()=>{}}/>);
 const line=await screen.findByText("Done · Clarified · Next: Specify",{selector:".workflow-run-status"});
 expect(line.getAttribute("title")).toContain("Task stage: clarified");
 const strip=await screen.findByLabelText("Conversation status");
 expect(await within(strip).findByText("Done · Clarified · Next: Specify")).toBeTruthy();
 expect(workspace.runResult).toHaveBeenCalledTimes(1);
 expect(screen.queryByText(/Skill completed|skill result unconfirmed|Skill result pending/)).toBeNull();
});

test("the sidebar dot and its accessible name follow the skill status; free rows keep theirs",async()=>{
 const failedRun:Run={...clarifyRun,id:"implement-run",taskId:"other",taskKey:"APP-8",skill:"implement",status:"failed",createdAt:"2026-11-14"};
 const freeRun:Run={id:"free-chat",projectId:"p",conversation:true,title:"Free discussion",provider:"claude",taskId:"",skill:"",status:"completed",createdAt:"2026-11-13"};
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example"}]);
 vi.mocked(api.listTasks).mockResolvedValue([task,{id:"other",key:"APP-8",title:"Broken task",status:"to_implement",source:"jira",labels:["#specified"]}]);
 vi.mocked(api.getOrchestration).mockResolvedValue({orchestratorEnabled:false,agents:[]});
 vi.mocked(api.projectSkills).mockResolvedValue([]);
 vi.mocked(api.executionSnapshot).mockResolvedValue({status:{connected:true,capabilities:[]},runs:[clarifyRun,failedRun,freeRun]});
 monitor.snapshot.runs=[clarifyRun,failedRun,freeRun];
 vi.mocked(workspace.runResult).mockImplementation(async runId=>runId==="clarify-run"?done:{activity:{id:"implement-run",taskId:"other",skillId:"implement",status:"failed",error:"tests failed"},task:{status:"to_implement",labels:["#specified"]}});
 render(<App/>);
 const sidebar=screen.getByRole("complementary");
 await within(sidebar).findByText("APP-7");
 await waitFor(()=>expect(sidebar.querySelector(".conversation-indicator.skill-done")).not.toBeNull());
 await waitFor(()=>expect(sidebar.querySelector(".conversation-indicator.skill-failed")).not.toBeNull());
 const doneRow=sidebar.querySelector(".conversation-indicator.skill-done")!.closest("button")!;
 expect(doneRow.getAttribute("title")?.endsWith(" · Done · Clarified")).toBe(true);
 expect(doneRow.getAttribute("aria-label")).toBe(doneRow.getAttribute("title"));
 const failedRow=sidebar.querySelector(".conversation-indicator.skill-failed")!.closest("button")!;
 expect(failedRow.getAttribute("title")?.endsWith(" · Could not finish")).toBe(true);
 const freeRow=within(sidebar).getByText("Free discussion").closest("button")!;
 expect(freeRow.querySelector(".conversation-indicator.idle")).not.toBeNull();
 expect(freeRow.hasAttribute("aria-label")).toBe(false);
 expect(freeRow.getAttribute("title")?.endsWith(" · Completed")).toBe(true);
 expect(vi.mocked(workspace.runResult).mock.calls.map(call=>call[0]).sort()).toEqual(["clarify-run","implement-run"]);
});
