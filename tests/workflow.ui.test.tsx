import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {WorkflowTask} from "../src/components/WorkflowTask";
import * as api from "../src/lib/agent";
import * as context from "../src/lib/workspace";
vi.mock("../src/lib/agent",()=>({listTasks:vi.fn(),launchTask:vi.fn(),executionSnapshot:vi.fn(),stopRun:vi.fn()}));
vi.mock("../src/lib/workspace",()=>({projectContext:vi.fn(),providerReadiness:vi.fn(),runResult:vi.fn(),declareReviewed:vi.fn(),openExternalLink:vi.fn()}));
vi.mock("../src/components/RunConsole",()=>({RunConsole:()=> <div>Task output</div>}));
vi.mock("../src/components/RunReview",()=>({RunReview:()=> <div>Task review</div>}));
const monitor={snapshot:{status:{connected:true},runs:[] as any[]},fresh:true,refresh:vi.fn(async()=>{})};
vi.mock("../src/components/ExecutionMonitor",()=>({useExecutionMonitor:()=>monitor}));
const task={id:"task",key:"APP-1",title:"Implement feature",status:"to_implement",source:"github",labels:["#specified"]};
const project={configured:true,aiProvider:"claude",skills:[{id:"implement"},{id:"specify"},{id:"pickup"}],policy:{fullChainStopStage:"reviewed"}};
beforeEach(()=>{vi.resetAllMocks();monitor.snapshot.runs=[];monitor.fresh=true;vi.mocked(api.listTasks).mockResolvedValue([task]);vi.mocked(context.projectContext).mockResolvedValue(project);vi.mocked(context.providerReadiness).mockResolvedValue({provider:"claude",complete:true,steps:{skills:{status:"success"},mcp:{status:"success"}}});vi.mocked(api.executionSnapshot).mockResolvedValue({status:{connected:true},runs:[]});vi.mocked(api.launchTask).mockResolvedValue({});});
afterEach(cleanup);
const show=()=>render(<WorkflowTask projectId="project" taskId="task" onBack={()=>{}} onSetup={()=>{}}/>);
test("next-step launch rereads workflow state and preserves the configured chain stop in the UI",async()=>{
 show();const next=await screen.findByRole("button",{name:"Étape suivante : Implémenter"});
 await waitFor(()=>expect(next.hasAttribute("disabled")).toBe(false));
 expect(screen.getByRole("button",{name:"Lancer la chaîne · jusqu’à Revu"})).toBeTruthy();
 fireEvent.click(next);
 await waitFor(()=>expect(api.launchTask).toHaveBeenCalledWith("project",{taskId:"task",skillId:"implement",prompt:"",mode:""}));
 expect(vi.mocked(api.listTasks).mock.invocationCallOrder[1]).toBeLessThan(vi.mocked(api.launchTask).mock.invocationCallOrder[0]);expect(vi.mocked(context.projectContext).mock.invocationCallOrder[1]).toBeLessThan(vi.mocked(api.launchTask).mock.invocationCallOrder[0]);
 expect(await screen.findByText(/Lancement accepté/)).toBeTruthy();
 fireEvent.click(next);expect(api.launchTask).toHaveBeenCalledTimes(1);
});
test("a concurrent execution found at click prevents dispatch and preserves instructions",async()=>{
 vi.mocked(api.executionSnapshot).mockResolvedValue({status:{connected:true},runs:[{id:"active",taskId:"task",projectId:"project",status:"running",skill:"implement",createdAt:"2026-10-28"}]});
 show();fireEvent.click(await screen.findByText("Instructions et autre skill"));fireEvent.change(screen.getByLabelText("Instructions supplémentaires"),{target:{value:"Keep this instruction"}});
 const next=await screen.findByRole("button",{name:"Étape suivante : Implémenter"});await waitFor(()=>expect(next.hasAttribute("disabled")).toBe(false));fireEvent.click(next);
 expect(await screen.findByRole("alert")).toBeTruthy();expect(api.launchTask).not.toHaveBeenCalled();expect((screen.getByLabelText("Instructions supplémentaires") as HTMLTextAreaElement).value).toBe("Keep this instruction");
});
test("a changed expected next skill is refreshed without launching the stale action",async()=>{
 vi.mocked(context.projectContext).mockResolvedValueOnce(project).mockResolvedValue({...project,policy:{},skills:[{id:"clarify"}]});
 vi.mocked(api.listTasks).mockResolvedValueOnce([task]).mockResolvedValue([{...task,status:"to_clarify",labels:["#new"]}]);
 show();const next=await screen.findByRole("button",{name:"Étape suivante : Implémenter"});await waitFor(()=>expect(next.hasAttribute("disabled")).toBe(false));fireEvent.click(next);
 await screen.findByRole("alert");expect(api.launchTask).not.toHaveBeenCalled();
});
test("completed process output is not presented as a confirmed skill result",async()=>{
 monitor.snapshot.runs=[{id:"run",projectId:"project",taskId:"task",skill:"implement",status:"completed",createdAt:"2026-10-28",trace:true}];vi.mocked(context.runResult).mockResolvedValue({});
 show();expect(await screen.findByText("Processus terminé · résultat de skill non confirmé")).toBeTruthy();expect(screen.getByText("Task output")).toBeTruthy();
});

test("a known MCP mismatch blocks a workflow launch instead of using another companion",async()=>{
 vi.mocked(context.providerReadiness).mockResolvedValue({provider:"claude",complete:false,steps:{skills:{status:"success"},mcp:{status:"pending",configuration:{present:true,valid:false}}}});
 show();
 expect(await screen.findByText(/L’entrée MCP existante ne correspond pas/)).toBeTruthy();
 expect(screen.getByRole("button",{name:"Étape suivante : Implémenter"}).hasAttribute("disabled")).toBe(true);
 expect(api.launchTask).not.toHaveBeenCalled();
});

test("a transport failure remains visible when workflow metadata refreshes",{timeout:8000},async()=>{
 vi.mocked(api.launchTask).mockRejectedValue(new Error("Connection lost"));
 monitor.snapshot.runs=[{id:"previous",projectId:"project",taskId:"task",skill:"specify",status:"completed",createdAt:"2026-10-28"}];vi.mocked(context.runResult).mockResolvedValue({});
 show();
 const next=await screen.findByRole("button",{name:"Étape suivante : Implémenter"});await waitFor(()=>expect(next.hasAttribute("disabled")).toBe(false));fireEvent.click(next);
 expect(await screen.findByRole("alert")).toBeTruthy();
 // A periodic metadata refresh must not turn an ambiguous send into permission to resend.
 await waitFor(()=>expect(vi.mocked(api.listTasks).mock.calls.length).toBeGreaterThanOrEqual(3),{timeout:6000});
 expect(screen.getByRole("alert").textContent).toContain("Lancement non confirmé");
 expect(next.hasAttribute("disabled")).toBe(true);expect(api.launchTask).toHaveBeenCalledTimes(1);
});
