import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {StrictMode} from "react";
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
test("reaching the chain target replaces its launch action while keeping the next manual step",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue([{...task,status:"to_close",labels:["#reviewed"],prUrl:"https://example.test/pr/1"}]);
 vi.mocked(context.projectContext).mockResolvedValue({...project,skills:[...project.skills,{id:"handoff"}]});
 show();
 expect(await screen.findByText("Objectif de la chaîne atteint : Revu.")).toBeTruthy();
 expect(screen.queryByRole("button",{name:/Lancer la chaîne/})).toBeNull();
 await waitFor(()=>expect(screen.getByRole("button",{name:"Étape suivante : Finaliser"}).hasAttribute("disabled")).toBe(false));
});
test("a chain target reached after rendering prevents dispatch on the fresh read",async()=>{
 vi.mocked(api.listTasks).mockResolvedValueOnce([task]).mockResolvedValue([{...task,status:"to_close",labels:["#reviewed"]}]);
 show();
 const chain=await screen.findByRole("button",{name:"Lancer la chaîne · jusqu’à Revu"});
 await waitFor(()=>expect(chain.hasAttribute("disabled")).toBe(false));
 fireEvent.click(chain);
 expect(await screen.findByText("Objectif de la chaîne atteint : Revu.")).toBeTruthy();
 expect(api.launchTask).not.toHaveBeenCalled();
 expect(screen.queryByRole("alert")).toBeNull();
});
test("an unconfigured project disables chain and custom skill dispatch",async()=>{
 vi.mocked(context.projectContext).mockResolvedValue({...project,configured:false});
 show();
 const chain=await screen.findByRole("button",{name:"Lancer la chaîne · jusqu’à Revu"});
 expect(chain.hasAttribute("disabled")).toBe(true);
 fireEvent.click(screen.getByText("Instructions et autre skill"));
 fireEvent.change(screen.getByLabelText("Autre skill"),{target:{value:"implement"}});
 const custom=screen.getByRole("button",{name:"Lancer la skill choisie"});
 expect(custom.hasAttribute("disabled")).toBe(true);
 fireEvent.click(chain);fireEvent.click(custom);
 expect(api.launchTask).not.toHaveBeenCalled();
});
test("switching tickets clears the prior workflow and instructions while the next ticket loads",async()=>{
 const view=show();
 await screen.findByRole("heading",{name:task.title});
 fireEvent.click(screen.getByText("Instructions et autre skill"));
 fireEvent.change(screen.getByLabelText("Instructions supplémentaires"),{target:{value:"Only for the first ticket"}});
 let finish!:(tasks:typeof task[])=>void;
 vi.mocked(api.listTasks).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));
 view.rerender(<WorkflowTask projectId="project" taskId="second" onBack={()=>{}} onSetup={()=>{}}/>);
 expect(screen.queryByRole("heading",{name:task.title})).toBeNull();
 expect(screen.queryByLabelText("Instructions supplémentaires")).toBeNull();
 expect(screen.queryByRole("button",{name:/Étape suivante/})).toBeNull();
 finish([{...task,id:"second",key:"APP-2",title:"Second ticket"}]);
 await screen.findByRole("heading",{name:"Second ticket"});
 expect((screen.getByLabelText("Instructions supplémentaires") as HTMLTextAreaElement).value).toBe("");
 expect(api.launchTask).not.toHaveBeenCalled();
});
test("workflow loads after React remounts its effect during an in-flight read",async()=>{
 let resolveFirst!:(tasks:typeof task[])=>void;
 vi.mocked(api.listTasks).mockImplementationOnce(()=>new Promise(resolve=>{resolveFirst=resolve;})).mockResolvedValue([task]);
 render(<StrictMode><WorkflowTask projectId="project" taskId="task" onBack={()=>{}} onSetup={()=>{}}/></StrictMode>);
 await waitFor(()=>expect(api.listTasks).toHaveBeenCalledTimes(2));
 resolveFirst([task]);
 expect(await screen.findByRole("button",{name:"Étape suivante : Implémenter"})).toBeTruthy();
});
test("finished tickets show their result and PR without unavailable launch controls",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue([{...task,status:"finished",labels:["#finished"],prUrl:"https://example.test/pr/1"}]);
 show();
 expect(await screen.findByText("Ce ticket est terminé.")).toBeTruthy();
 expect(screen.getByRole("button",{name:"PR/MR · état inconnu"})).toBeTruthy();
 expect(screen.queryByRole("button",{name:/Lancer la chaîne/})).toBeNull();
 expect(screen.queryByText("Instructions et autre skill")).toBeNull();
 expect(screen.queryByText(/Préparation skills\/MCP non confirmée/)).toBeNull();
});
test("unverified engine preparation stays compact and can still open setup",async()=>{
 const onSetup=vi.fn();
 vi.mocked(context.providerReadiness).mockResolvedValue({provider:"claude",complete:false,steps:{skills:{status:"unknown"},mcp:{status:"unknown"}}});
 render(<WorkflowTask projectId="project" taskId="task" onBack={()=>{}} onSetup={onSetup}/>);
 const summary=await screen.findByText("Préparation du moteur non vérifiée");
 expect(summary.closest("details")?.hasAttribute("open")).toBe(false);
 await waitFor(()=>expect(screen.getByRole("button",{name:"Étape suivante : Implémenter"}).hasAttribute("disabled")).toBe(false));
 fireEvent.click(summary);
 fireEvent.click(screen.getByRole("button",{name:"Vérifier les moteurs IA"}));
 expect(onSetup).toHaveBeenCalledOnce();
});
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
 expect(screen.queryByRole("button",{name:"Arrêter l’exécution"})).toBeNull();
});
test("active workflow runs show an interrupt action and acknowledge a requested stop",async()=>{
 monitor.snapshot.runs=[{id:"run",projectId:"project",taskId:"task",skill:"implement",status:"running",createdAt:"2026-10-28"}];
 vi.mocked(context.runResult).mockResolvedValue({});
 const view=show();
 expect(await screen.findByRole("button",{name:"Arrêter l’exécution"})).toBeTruthy();
 monitor.snapshot.runs=[{...monitor.snapshot.runs[0],cancelRequested:true}];
 view.rerender(<WorkflowTask projectId="project" taskId="task" onBack={()=>{}} onSetup={()=>{}}/>);
 const requested=await screen.findByRole("button",{name:"Arrêt demandé…"});
 expect(requested.hasAttribute("disabled")).toBe(true);
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
