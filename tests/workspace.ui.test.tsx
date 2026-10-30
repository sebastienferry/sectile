import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
vi.mock("@tauri-apps/api/core", () => ({ invoke: vi.fn() }));
vi.mock("@tauri-apps/api/event", () => ({ listen: vi.fn() }));
vi.mock("../src/components/ProjectPrompt", () => ({ default: ({ projectId }: {projectId: string}) => <div>Free prompt for {projectId}</div> }));
import App from "../src/App";
import type { Task } from "../src/domain/board";
import { conversationFor } from "../src/lib/conversation-controller";
import * as workspace from "../src/lib/workspace";
vi.mock("../src/lib/workspace",async original=>({...await original<typeof workspace>(),projectContext:vi.fn(),openExternalLink:vi.fn()}));
import * as api from "../src/lib/agent";
vi.mock("../src/lib/agent", () => ({
  listProjects: vi.fn(), listTasks: vi.fn(), getTask: vi.fn(), executionSnapshot: vi.fn(),
  projectSkills: vi.fn(), getOrchestration: vi.fn(), launchTask: vi.fn(),
  stopRun: vi.fn(), runOrchestration: vi.fn(),
}));
vi.mock("../src/components/RunConsole", () => ({ RunConsole: () => <div>Live output</div> }));
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{}});
  vi.mocked(workspace.openExternalLink).mockResolvedValue(undefined);
  vi.stubGlobal("matchMedia", () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
  vi.mocked(api.listProjects).mockResolvedValue([{ id: "p", name: "Example" }]);
  vi.mocked(api.listTasks).mockResolvedValue([]);
  vi.mocked(api.getTask).mockResolvedValue({id:"task",key:"APP-1",title:"Task",status:"to_clarify",source:"jira",trackerStatus:"Open"});
  vi.mocked(api.projectSkills).mockResolvedValue([{ id: "implement_issue" }]);
  vi.mocked(api.getOrchestration).mockResolvedValue({ orchestratorEnabled: false, agents: [] });
  vi.mocked(api.executionSnapshot).mockResolvedValue({
    status: { connected: true, capabilities: [] },
    runs: [{ id: "r", projectId: "p", taskKey: "APP-1", skill: "implement_issue", status: "running", trace: true, createdAt: "2026-10-28" }],
  });
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); localStorage.clear(); });
test("sidebar navigation replaces the view without canceling a run", async () => {
  render(<App />);
  fireEvent.click(await screen.findByRole("button", { name: /Toutes les exécutions/ }));
  await screen.findByText("Exécutions de ce poste · 1 actives");
  fireEvent.click(screen.getByRole("button", { name: /APP-1/ }));
  await screen.findByText("Live output");
  expect(screen.queryByRole("tablist")).toBeNull();
  expect(api.executionSnapshot).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: "Tableau" }));
  await screen.findByRole("textbox", { name: "Rechercher des tickets" });
  expect(screen.queryByText("Live output")).toBeNull();
  expect(api.stopRun).not.toHaveBeenCalled();
});
test("the native macOS Settings menu opens the existing Settings view", async () => {
  Object.defineProperty(window, "__TAURI_INTERNALS__", { configurable: true, value: {} });
  vi.mocked(invoke).mockResolvedValue({});
  let openSettings: (() => void) | undefined;
  vi.mocked(listen).mockImplementation(async (_event, callback) => {
    openSettings = () => callback({ event: "sectile:open-settings", id: 1, payload: undefined });
    return () => {};
  });
  try {
    render(<App />);
    await screen.findByRole("button", { name: "Example" });
    await waitFor(() => expect(openSettings).toBeDefined());
    openSettings?.();
    expect(await screen.findByRole("heading", { name: "Configuration du poste" })).toBeTruthy();
  } finally {
    delete (window as Window & { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__;
  }
});
test("workflow run status stays on its ticket instead of duplicating the project rail row", async () => {
  vi.mocked(api.listProjects).mockResolvedValue([{ id: "p", name: "Example", path: "/work/example" }]);
  vi.mocked(api.listTasks).mockResolvedValue([{ id: "task", key: "APP-2", title: "Task with active run", status: "to_implement", source: "jira", trackerStatus: "In Progress", labels: ["#specified"] }]);
  vi.mocked(api.executionSnapshot).mockResolvedValue({
    status: { connected: true, capabilities: [] },
    runs: [{ id: "run", projectId: "p", taskId: "task", taskKey: "APP-2", skill: "implement", status: "running", createdAt: "2026-10-28" }],
  });
  vi.mocked(workspace.projectContext).mockResolvedValue({ configured: true, path: "/work/example", skills: [], policy: {}, board: { trackers: [{ trackerId: "jira-1", provider: "jira" }] } });
  render(<App />);
  await screen.findByText("Task with active run");
  expect(await screen.findByText("Implémenter · En cours")).toBeTruthy();
  const sidebar = screen.getByRole("complementary");
  expect(within(sidebar).queryByText("APP-2")).toBeNull();
  expect(within(sidebar).getByRole("button", { name: /Toutes les exécutions/ })).toBeTruthy();
});
test("ticket workflow keeps its own return action without repeated project controls",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
 vi.mocked(api.listTasks).mockResolvedValue([{id:"task",key:"APP-1",title:"A tracked ticket",status:"to_clarify",source:"jira",trackerStatus:"Open"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 render(<App/>);
 fireEvent.click(await screen.findByRole("button",{name:"Ouvrir le workflow de APP-1"}));
 expect(await screen.findByRole("heading",{name:"A tracked ticket"})).toBeTruthy();
 expect(screen.getByRole("button",{name:"← Tickets du projet"})).toBeTruthy();
 expect(screen.queryByRole("button",{name:"Tableau"})).toBeNull();
 expect(screen.queryByRole("button",{name:"Nouveau prompt"})).toBeNull();
 expect(screen.queryByRole("button",{name:"Configurer le projet"})).toBeNull();
 expect(screen.getByRole("button",{name:"Nouveau prompt dans Example"})).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"← Tickets du projet"}));
 expect(await screen.findByRole("textbox",{name:"Rechercher des tickets"})).toBeTruthy();
});
test("project menu opens the execution queue and groups ticket conversations by stage",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue([{id:"new",key:"APP-1",title:"New task",status:"to_clarify",source:"jira",trackerStatus:"Open"},{id:"reviewed",key:"APP-2",title:"Reviewed task",status:"to_close",source:"jira",trackerStatus:"Done",prUrl:"https://gitlab.example.com/team/app/-/merge_requests/7",prLinks:[{url:"https://gitlab.example.com/team/app/-/merge_requests/7",state:"merged"}]}]);
 vi.mocked(api.executionSnapshot).mockResolvedValue({status:{connected:true,capabilities:[]},runs:[
  {id:"new-run",projectId:"p",taskId:"new",taskKey:"APP-1",conversation:true,title:"Clarify this",provider:"claude",status:"running",skill:"clarify",createdAt:"2026-10-30"},
  {id:"reviewed-run",projectId:"p",taskId:"reviewed",taskKey:"APP-2",conversation:true,title:"Review this",provider:"codex",status:"completed",skill:"adjust",createdAt:"2026-10-29"},
 ]});
 render(<App/>);
 const sidebar=screen.getByRole("complementary");
 await within(sidebar).findByText("APP-1");
 expect(sidebar.querySelector(".conversation-indicator.session")).not.toBeNull();
 fireEvent.click(within(sidebar).getByLabelText("Actions pour Example"));
 fireEvent.change(within(sidebar).getByRole("combobox",{name:"Grouper les conversations de Example"}),{target:{value:"stage"}});
 expect(localStorage.getItem("sectile-tauri.conversationGrouping")).toBe("stage");
 await within(sidebar).findByText("À clarifier");
 expect(within(sidebar).getByText("Revu")).toBeTruthy();
 expect(within(sidebar).getByRole("button",{name:"Ouvrir la PR/MR de APP-2"}).getAttribute("title")).toContain("MR !7 · fusionnée");
 fireEvent.click(within(sidebar).getByRole("button",{name:"Ouvrir la PR/MR de APP-2"}));
 expect(workspace.openExternalLink).toHaveBeenCalledWith("https://gitlab.example.com/team/app/-/merge_requests/7");
 fireEvent.click(within(sidebar).getByRole("button",{name:"File des exécutions"}));
 await screen.findByText("Exécutions de ce poste · 1 actives");
});
test("conversation actions stop a live session and archive and restore a finished one",async()=>{
 vi.mocked(api.executionSnapshot).mockResolvedValue({status:{connected:true,capabilities:[]},runs:[
  {id:"live-chat",projectId:"p",conversation:true,title:"Live discussion",provider:"claude",status:"running",skill:"",createdAt:"2026-10-30"},
  {id:"done-chat",projectId:"p",conversation:true,title:"Finished discussion",provider:"codex",status:"completed",skill:"",createdAt:"2026-10-29"},
 ]});
 render(<App/>);
 const sidebar=screen.getByRole("complementary");
 await within(sidebar).findByText("Live discussion");
 fireEvent.click(within(sidebar).getByLabelText("Actions de la conversation Live discussion"));
 fireEvent.click(within(sidebar).getByRole("button",{name:"Arrêter la conversation"}));
 await waitFor(()=>expect(api.stopRun).toHaveBeenCalledWith("live-chat"));
 const finishedActions=within(sidebar).getByLabelText("Actions de la conversation Finished discussion").closest("details")!;
 fireEvent.click(within(finishedActions).getByLabelText("Actions de la conversation Finished discussion"));
 fireEvent.click(within(finishedActions).getByRole("button",{name:"Archiver la conversation"}));
 expect(within(sidebar).queryByText("Finished discussion")).toBeNull();
 expect(localStorage.getItem("sectile-tauri.archivedConversations")).toBe('["done-chat"]');
 fireEvent.click(within(sidebar).getByLabelText("Actions pour Example"));
 fireEvent.click(within(sidebar).getByRole("button",{name:"Voir les conversations archivées"}));
 expect(within(sidebar).getByText("Finished discussion")).toBeTruthy();
 fireEvent.click(within(sidebar).getByLabelText("Actions de la conversation Finished discussion"));
 fireEvent.click(within(sidebar).getByRole("button",{name:"Restaurer la conversation"}));
 expect(localStorage.getItem("sectile-tauri.archivedConversations")).toBe("[]");
});
test("launch restores the last project board instead of selecting the first project", async () => {
  vi.mocked(api.listProjects).mockResolvedValue([
    { id: "p", name: "Example" },
    { id: "q", name: "Second project" },
  ]);
  localStorage.setItem("sectile-tauri.workspace", JSON.stringify({ projectId: "q", view: "board" }));
  render(<App />);
  await screen.findByRole("heading", { name: "Second project" });
  expect(screen.getByRole("button", { name: "Second project" })).toBeTruthy();
});
test("launch restores a saved conversation when the agent still has it", async () => {
  vi.mocked(api.listProjects).mockResolvedValue([
    { id: "p", name: "Example" },
    { id: "q", name: "Second project" },
  ]);
  vi.mocked(api.executionSnapshot).mockResolvedValue({
    status: { connected: true, capabilities: [] },
    runs: [{ id: "conversation-7", projectId: "q", conversation: true, title: "Saved conversation", provider: "claude", status: "completed", createdAt: "2026-10-28" }],
  });
  localStorage.setItem("sectile-tauri.workspace", JSON.stringify({ projectId: "q", view: "prompt", runId: "conversation-7" }));
  render(<App />);
  await screen.findByText("Free prompt for q");
  expect(screen.getByRole("heading", { name: "Second project" })).toBeTruthy();
});
test("launch falls back to the project board when the saved conversation is gone", async () => {
  vi.mocked(api.executionSnapshot).mockResolvedValue({
    status: { connected: true, capabilities: [] },
    runs: [],
  });
  vi.mocked(api.listProjects).mockResolvedValue([{ id: "p", name: "Example", path: "/work/example" }]);
  vi.mocked(workspace.projectContext).mockResolvedValue({ configured: true, path: "/work/example", skills: [], policy: {}, board: { trackers: [] } });
  localStorage.setItem("sectile-tauri.workspace", JSON.stringify({ projectId: "p", view: "prompt", runId: "removed-conversation" }));
  render(<App />);
  await screen.findByRole("heading", { name: "Votre projet local est prêt" });
  expect(screen.queryByText("Free prompt for p")).toBeNull();
});
test("theme selection remains available without tabs", async () => {
  render(<App />);
  await screen.findByRole("button", { name: "Example" });
  expect(await screen.findAllByRole("button", { name: "Tableau" })).toHaveLength(1);
  fireEvent.click(screen.getByRole("button", { name: "Thème clair" }));
  expect(document.documentElement.dataset.theme).toBe("light");
  expect(screen.queryByRole("tab")).toBeNull();
});

test("a trackerless local project shows prompt guidance without tracker controls",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[]}});
 render(<App />);
 await screen.findByRole("heading",{name:"Votre projet local est prêt"});
 expect(screen.getByText("Aucun tracker n’est associé. Lancez un prompt pour travailler dans votre dossier local.")).toBeTruthy();
 expect(screen.getByRole("button",{name:"Nouveau prompt"})).toBeTruthy();
 expect(screen.queryByRole("textbox",{name:"Rechercher des tickets"})).toBeNull();
 expect(screen.queryByRole("combobox",{name:"Regrouper les tickets"})).toBeNull();
 expect(screen.queryByText("Statuts du tracker · lecture seule")).toBeNull();
 expect(screen.queryByRole("button",{name:"Commencer une conversation"})).toBeNull();
});

test("a folderless project can attach its folder directly from the board", async () => {
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"",skills:[],policy:{},board:{trackers:[]}});
 vi.mocked(invoke).mockResolvedValue("/work/new-folder");
 render(<App />);
 await screen.findByRole("heading",{name:"Choisissez un dossier pour commencer"});
 expect(screen.getByText("Lancez un prompt libre dans ce projet, sans configurer de tracker.")).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"Choisir un dossier"}));
 await screen.findByRole("heading",{name:"Votre projet local est prêt"});
 expect(invoke).toHaveBeenCalledWith("choose_project_folder",{projectId:"p"});
 expect(screen.queryByRole("textbox",{name:"Rechercher des tickets"})).toBeNull();
});

test("a local board does not flash remote controls while its project context loads",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
 let resolveContext!: (value:workspace.ProjectContext)=>void;
 vi.mocked(workspace.projectContext).mockReturnValue(new Promise(resolve=>{resolveContext=resolve;}));
 render(<App />);
 await screen.findByText("Chargement de la configuration du projet…");
 expect(screen.queryByRole("textbox",{name:"Rechercher des tickets"})).toBeNull();
 expect(screen.queryByRole("heading",{name:"Aucun ticket distant"})).toBeNull();
 resolveContext({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"local",provider:"local"}]}});
 await screen.findByRole("heading",{name:"Votre projet local est prêt"});
 expect(screen.queryByRole("textbox",{name:"Rechercher des tickets"})).toBeNull();
});

test("a failed board context read explains the problem and can be retried",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
 vi.mocked(workspace.projectContext)
  .mockRejectedValueOnce(new Error("Context unavailable"))
  .mockResolvedValueOnce({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[]}});
 render(<App />);
 expect((await screen.findByRole("alert")).textContent).toContain("Context unavailable");
 expect(screen.queryByRole("heading",{name:"Aucun ticket distant"})).toBeNull();
 fireEvent.click(screen.getByRole("button",{name:"Réessayer"}));
 await screen.findByRole("heading",{name:"Votre projet local est prêt"});
 expect(screen.queryByRole("alert")).toBeNull();
});

test("switching projects does not show another project's tickets or search filter",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"},{id:"q",name:"Second project",path:"/work/second"}]);
 let resolveSecond!: (items:Task[])=>void;
 vi.mocked(api.listTasks).mockImplementation(id=>id==="p"
  ? Promise.resolve([{id:"old",key:"APP-1",title:"Old project ticket",source:"jira",trackerStatus:"Open",status:"to_clarify"}])
  : new Promise(resolve=>{resolveSecond=resolve;}));
 vi.mocked(workspace.projectContext).mockImplementation(id=>Promise.resolve({configured:true,path:id==="p"?"/work/example":"/work/second",skills:[],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}}));
 render(<App />);
 await screen.findByText("Old project ticket");
 fireEvent.change(screen.getByRole("textbox",{name:"Rechercher des tickets"}),{target:{value:"Old"}});
 fireEvent.click(screen.getByRole("button",{name:"Second project"}));
 expect(screen.queryByText("Old project ticket")).toBeNull();
 expect((await screen.findByRole("textbox",{name:"Rechercher des tickets"}) as HTMLInputElement).value).toBe("");
 resolveSecond([{id:"new",key:"APP-2",title:"Second project ticket",source:"jira",trackerStatus:"Open",status:"to_clarify"}]);
 await screen.findByText("Second project ticket");
});

test("a failed local ticket refresh does not expose remote tracker controls",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
 vi.mocked(api.listTasks).mockResolvedValueOnce([]).mockRejectedValueOnce(new Error("Ticket read unavailable"));
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[]}});
 render(<App />);
 await screen.findByRole("heading",{name:"Votre projet local est prêt"});
 fireEvent.click(screen.getByRole("button",{name:"Actualiser le tableau"}));
 expect((await screen.findByRole("alert")).textContent).toContain("Ticket read unavailable");
 expect(screen.queryByRole("textbox",{name:"Rechercher des tickets"})).toBeNull();
});

test("the synthetic local tracker does not show remote tracker controls",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"local",provider:"local"}]}});
 render(<App />);
 await screen.findByRole("heading",{name:"Votre projet local est prêt"});
 expect(screen.queryByRole("textbox",{name:"Rechercher des tickets"})).toBeNull();
 expect(screen.queryByRole("combobox",{name:"Regrouper les tickets"})).toBeNull();
 expect(screen.queryByText("Statuts du tracker · lecture seule")).toBeNull();
});

test("an older agent identifies a single local tracker through project settings",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",issueTracker:"local",skills:[],policy:{},board:{trackers:[{trackerId:"local"}]}});
 render(<App />);
 await screen.findByRole("heading",{name:"Votre projet local est prêt"});
 expect(screen.queryByRole("textbox",{name:"Rechercher des tickets"})).toBeNull();
});

test("tracker-backed projects retain board search, grouping and ticket status",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
 vi.mocked(api.listTasks).mockResolvedValue([{id:"t",key:"APP-2",title:"Remote issue",source:"jira",trackerStatus:"To Do",status:"to_clarify"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 render(<App />);
 await screen.findByRole("textbox",{name:"Rechercher des tickets"});
 await screen.findByText("Remote issue");
 expect(screen.getByRole("combobox",{name:"Regrouper les tickets"})).toBeTruthy();
 expect(screen.getByText("Statuts du tracker · lecture seule")).toBeTruthy();
 expect(screen.getByText("Remote issue")).toBeTruthy();
 expect(screen.queryByRole("button",{name:"Effacer la sélection"})).toBeNull();
 expect(screen.queryByRole("button",{name:"Préparer un lot"})).toBeNull();
 expect(screen.getByRole("region",{name:"Colonnes de tickets, défilement horizontal"}).getAttribute("tabindex")).toBe("0");
});
test("workflow columns open by default and the tracker grouping choice persists",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue([{id:"t",key:"APP-2",title:"Remote issue",source:"jira",trackerStatus:"To Do",status:"to_clarify"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 const view=render(<App/>);
 await screen.findByText("Remote issue");
 expect(document.querySelector('.column[data-stage="new"]')).not.toBeNull();
 expect(document.querySelector('.column[data-stage="clarified"]')).not.toBeNull();
 const grouping=screen.getByRole("combobox",{name:"Regrouper les tickets"}) as HTMLSelectElement;
 expect(grouping.value).toBe("stage");
 fireEvent.change(grouping,{target:{value:"tracker"}});
 expect(localStorage.getItem("sectile-tauri.boardGrouping")).toBe("tracker");
 expect(document.querySelector('.column[data-stage]')).toBeNull();
 view.unmount();
 render(<App/>);
 await screen.findByText("Remote issue");
 expect((screen.getByRole("combobox",{name:"Regrouper les tickets"}) as HTMLSelectElement).value).toBe("tracker");
});
test("the backlog list keeps epic context and selection while switching views",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue([{id:"t1",key:"APP-1",title:"First issue",source:"jira",trackerStatus:"To Do",status:"to_clarify",parentKey:"EPIC-1",priority:"high"},{id:"t2",key:"APP-2",title:"Second issue",source:"jira",trackerStatus:"To Do",status:"to_clarify",parentKey:"EPIC-1",priority:"medium"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 render(<App/>);
 await screen.findByText("First issue");
 fireEvent.click(screen.getByRole("button",{name:"Liste"}));
 const list=screen.getByRole("region",{name:"Liste des tickets"});
 expect(within(list).getAllByText("EPIC-1")).toHaveLength(2);
 fireEvent.click(within(list).getByRole("checkbox",{name:"Sélectionner APP-1"}));
 expect(screen.getByText("1 sélectionné")).toBeTruthy();
 fireEvent.click(within(screen.getByRole("group",{name:"Vue des tickets"})).getByRole("button",{name:"Tableau"}));
 expect((screen.getByRole("checkbox",{name:"Sélectionner APP-1"}) as HTMLInputElement).checked).toBe(true);
 expect(localStorage.getItem("sectile-tauri.ticketView")).toBe("board");
});
test("the board uses Sectile epic colors only when the project enables them",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue([{id:"t1",key:"APP-1",title:"Epic child",source:"jira",trackerStatus:"To Do",status:"to_clarify",parentKey:"PE-1078"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{epicColors:true,trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 render(<App/>);
 await screen.findByText("Epic child");
 const card=document.querySelector(".board .task") as HTMLElement;
 expect(card.classList.contains("has-epic-color")).toBe(true);
 expect(card.style.getPropertyValue("--epic")).toBe("#f59e0b");
 expect(screen.getByRole("checkbox",{name:"Sélectionner APP-1"})).toBeTruthy();
 expect(screen.getByRole("combobox",{name:"Actions pour APP-1"})).toBeTruthy();
});
test("a selected ticket batch requires confirmation and uses the Go pickup skill",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue([{id:"t1",key:"APP-1",title:"First issue",source:"jira",trackerStatus:"To Do",status:"to_clarify"},{id:"t2",key:"APP-2",title:"Second issue",source:"jira",trackerStatus:"To Do",status:"to_clarify"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[{id:"pickup_issues"}],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 vi.mocked(api.launchTask).mockResolvedValue({});
 render(<App/>);
 await screen.findByText("First issue");
 fireEvent.click(screen.getByRole("checkbox",{name:"Sélectionner APP-1"}));
 fireEvent.click(screen.getByRole("checkbox",{name:"Sélectionner APP-2"}));
 expect(screen.getByRole("button",{name:"Effacer la sélection"})).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"Préparer un lot"}));
 expect(api.launchTask).not.toHaveBeenCalled();
 fireEvent.click(screen.getByRole("button",{name:"Lancer le lot"}));
 await waitFor(()=>expect(api.launchTask).toHaveBeenCalledWith("p",{taskId:"t1",skillId:"pickup_issues",prompt:"",mode:"autonomous",batchTaskIds:["t1","t2"]}));
});
test("the board skill menu launches the selected skill only after refreshing its ticket",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue([{id:"t1",key:"APP-1",title:"First issue",source:"jira",trackerStatus:"To Do",status:"to_clarify"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[{id:"clarify"}],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 vi.mocked(api.executionSnapshot).mockResolvedValue({status:{connected:true,capabilities:[]},runs:[]});
 vi.mocked(api.launchTask).mockResolvedValue({});
 render(<App/>);
 await screen.findByText("First issue");
 const menu=screen.getByRole("combobox",{name:"Actions pour APP-1"});
 await waitFor(()=>expect((within(menu).getByRole("option",{name:"Lancer : Clarifier"}) as HTMLOptionElement).disabled).toBe(false));
 fireEvent.change(menu,{target:{value:"clarify"}});
 await waitFor(()=>expect(api.launchTask).toHaveBeenCalledWith("p",{taskId:"t1",skillId:"clarify",prompt:"",mode:""}));
});
test("board density defaults to compact and remembers the detailed choice",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue([{id:"t",key:"APP-2",title:"A long remote issue title",source:"jira",trackerStatus:"To Do",status:"to_clarify"}]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 const view=render(<App />);
 await screen.findByText("A long remote issue title");
 const density=screen.getByRole("button",{name:"Cartes compactes"});
 expect(density.getAttribute("aria-pressed")).toBe("true");
 expect(document.querySelector(".board .task.compact")).not.toBeNull();
 fireEvent.click(density);
 expect(density.getAttribute("aria-pressed")).toBe("false");
 expect(document.querySelector(".board .task.compact")).toBeNull();
 expect(localStorage.getItem("sectile-tauri.boardCards")).toBe("full");
 view.unmount();
 render(<App />);
 await screen.findByText("A long remote issue title");
 expect(screen.getByRole("button",{name:"Cartes compactes"}).getAttribute("aria-pressed")).toBe("false");
});

test("a large tracker board renders tickets progressively while search reaches the full result",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue(Array.from({length:1988},(_,index)=>({id:`task-${index+1}`,key:`APP-${index+1}`,title:`Ticket ${index+1}`,source:"jira",trackerStatus:"To Do",status:"to_clarify"})));
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 render(<App />);
 await screen.findByText("Ticket 1");
 fireEvent.change(screen.getByRole("combobox",{name:"Regrouper les tickets"}),{target:{value:"tracker"}});
 expect(screen.getByText("1988 tickets")).toBeTruthy();
 expect(document.querySelectorAll(".board .task")).toHaveLength(40);
 fireEvent.click(screen.getByRole("button",{name:"Afficher les tickets suivants dans To Do"}));
 expect(document.querySelectorAll(".board .task")).toHaveLength(80);
 fireEvent.change(screen.getByRole("textbox",{name:"Rechercher des tickets"}),{target:{value:"APP-1988"}});
 await screen.findByText("Ticket 1988");
 expect(document.querySelectorAll(".board .task")).toHaveLength(1);
 expect(screen.queryByRole("button",{name:"Afficher les tickets suivants dans To Do"})).toBeNull();
});
test("active workflow tickets remain in the first visible board batch",async()=>{
 vi.mocked(api.listTasks).mockResolvedValue(Array.from({length:100},(_,index)=>({id:`task-${index+1}`,key:`APP-${index+1}`,title:`Ticket ${index+1}`,source:"jira",trackerStatus:"To Do",status:"to_implement"})));
 vi.mocked(api.executionSnapshot).mockResolvedValue({status:{connected:true,capabilities:[]},runs:[{id:"active",projectId:"p",taskId:"task-100",taskKey:"APP-100",skill:"implement",status:"running",createdAt:"2026-10-29"}]});
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 render(<App />);
 await screen.findByText("Ticket 100");
 expect(document.querySelectorAll(".board .task")).toHaveLength(40);
 expect(screen.queryByText("Ticket 40")).toBeNull();
 expect(screen.getByText("Implémenter · En cours")).toBeTruthy();
});

test("an empty remote tracker board names the missing tickets and keeps its local prompt path",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
 vi.mocked(api.listTasks).mockResolvedValue([]);
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 render(<App />);
 await screen.findByRole("heading",{name:"Aucun ticket distant"});
 expect(screen.getByText("Le tracker ne remonte aucun ticket. Vous pouvez lancer un prompt dans le dossier local associé.")).toBeTruthy();
 expect(screen.getByRole("textbox",{name:"Rechercher des tickets"})).toBeTruthy();
 expect(screen.getByRole("combobox",{name:"Regrouper les tickets"})).toBeTruthy();
 expect(screen.getByRole("button",{name:"Nouveau prompt"})).toBeTruthy();
});

test("a failed ticket refresh identifies the retained board data as stale",async()=>{
 vi.mocked(api.listTasks)
  .mockResolvedValueOnce([{id:"t",key:"APP-2",title:"Previous ticket read",source:"jira",trackerStatus:"To Do",status:"to_clarify"}])
  .mockRejectedValueOnce(new Error("Tracker temporarily unavailable"));
 vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{},board:{trackers:[{trackerId:"jira-1",provider:"jira"}]}});
 render(<App />);
 await screen.findByText("Previous ticket read");
 fireEvent.click(screen.getByRole("button",{name:"Actualiser le tableau"}));
 expect((await screen.findByRole("alert")).textContent).toContain("Tracker temporarily unavailable");
 expect(await screen.findByText("La dernière actualisation a échoué. Les tickets affichés proviennent de la lecture précédente.")).toBeTruthy();
 expect(screen.getByText("Previous ticket read")).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"Nouveau prompt"}));
 await screen.findByRole("heading",{name:"Où voulez-vous travailler ?"});
 expect(screen.queryByRole("alert")).toBeNull();
});

test("opening a folder navigates straight to its free prompt; cancel leaves the board visible", async () => {
  vi.mocked(invoke).mockResolvedValueOnce(null).mockResolvedValueOnce({id:"p",name:"Example",path:"/work/example"});
  render(<App />);
  await screen.findByRole("textbox", {name:"Rechercher des tickets"});
  fireEvent.click(screen.getByRole("button",{name:"Ouvrir un dossier"}));
  await screen.findByRole("textbox", {name:"Rechercher des tickets"});
  await waitFor(() => expect(screen.getByRole("button",{name:"Ouvrir un dossier"}).hasAttribute("disabled")).toBe(false));
  vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
  fireEvent.click(screen.getByRole("button",{name:"Ouvrir un dossier"}));
  await screen.findByText("Free prompt for p");
  expect(invoke).toHaveBeenCalledWith("open_folder_project");
});

test("settings remain reachable with the sidebar collapsed", async () => {
 vi.mocked(invoke).mockResolvedValue({enabled:true,ready:true,signedIn:true});
 render(<App />);
 await screen.findByRole("button",{name:"Example"});
 fireEvent.click(screen.getByRole("button",{name:"Afficher ou masquer les projets"}));
 fireEvent.click(screen.getByRole("button",{name:"Paramètres"}));
 await screen.findByRole("navigation",{name:"Sections des paramètres"});
 expect(screen.queryByRole("tablist")).toBeNull();
});

test("keyboard navigation opens a prompt and settings while the sidebar width persists", async () => {
 vi.mocked(api.listProjects).mockResolvedValue([{ id: "p", name: "Example", path: "/work/example" }]);
 vi.mocked(invoke).mockImplementation(async command => command === "local_service_log"
  ? { path: "/test/companion.log", text: "", missing: true, truncated: false }
  : { enabled: true, ready: true, signedIn: true, agentReady: true, mode: "local", profile: { email: "fixture@example.invalid", displayName: "Fixture" }, trackers: [], projects: [] });
 render(<App />);
 await screen.findByRole("button", { name: "Example" });
 const modifier = /Mac/i.test(navigator.platform || navigator.userAgent) ? { metaKey: true } : { ctrlKey: true };
 const divider = screen.getByRole("separator", { name: "Redimensionner la barre latérale" });
 fireEvent.keyDown(divider, { key: "ArrowRight" });
 expect(divider.getAttribute("aria-valuenow")).toBe("288");
 expect(localStorage.getItem("sectile-tauri.sidebarWidth")).toBe("288");
 fireEvent.keyDown(window, { key: "b", ...modifier });
 expect(screen.queryByRole("complementary")).toBeNull();
 fireEvent.keyDown(window, { key: "b", ...modifier });
 expect(screen.getByRole("complementary")).toBeTruthy();
 fireEvent.keyDown(window, { key: "n", ...modifier });
 await screen.findByText("Free prompt for p");
 fireEvent.keyDown(window, { key: ",", ...modifier });
 await screen.findByRole("navigation", { name: "Sections des paramètres" });
});
test("narrow windows cap the visible sidebar without discarding the saved width",async()=>{
 localStorage.setItem("sectile-tauri.sidebarWidth","368");
 vi.stubGlobal("innerWidth",800);
 render(<App/>);
 await screen.findByRole("button",{name:"Example"});
 const sidebar=screen.getByRole("complementary");
 const divider=screen.getByRole("separator",{name:"Redimensionner la barre latérale"});
 expect(sidebar.style.width).toBe("256px");
 expect(divider.getAttribute("aria-valuenow")).toBe("256");
 expect(divider.getAttribute("aria-valuemax")).toBe("256");
 expect(localStorage.getItem("sectile-tauri.sidebarWidth")).toBe("368");
 vi.stubGlobal("innerWidth",1280);
 fireEvent(window,new Event("resize"));
 expect(sidebar.style.width).toBe("368px");
 expect(divider.getAttribute("aria-valuemax")).toBe("480");
});

test("settings use one navigation rail and return to the previous workspace", async () => {
 vi.mocked(invoke).mockImplementation(async command => command === "local_service_log"
  ? {path:"/test/companion.log",text:"",missing:true,truncated:false}
  : null);
 render(<App />);
 await screen.findByRole("button", { name: "Example" });
 fireEvent.click(screen.getByRole("button", { name: "Paramètres" }));
 await screen.findByRole("navigation", { name: "Sections des paramètres" });
 expect(screen.queryByRole("button", { name: "Example" })).toBeNull();
 fireEvent.click(screen.getByRole("button", { name: "Retourner à l’espace de travail" }));
 await screen.findByRole("textbox", { name: "Rechercher des tickets" });
 expect(screen.getByRole("button", { name: "Example" })).toBeTruthy();
});

test("an unmapped project asks for its folder before mounting the prompt and keeps the same project", async () => {
 vi.mocked(invoke).mockResolvedValue("/work/example");
 render(<App />);
 fireEvent.click(await screen.findByRole("button",{name:"Nouveau prompt dans Example"}));
 await screen.findByRole("heading",{name:"Où voulez-vous travailler ?"});
 expect(screen.queryByText("Free prompt for p")).toBeNull();
 fireEvent.click(screen.getByRole("button",{name:"Choisir un dossier"}));
 await screen.findByText("Free prompt for p");
 expect(invoke).toHaveBeenCalledWith("choose_project_folder",{projectId:"p"});
 expect(invoke).not.toHaveBeenCalledWith("open_folder_project");
 expect(screen.getByTitle("/work/example").textContent).toBe("/work/example");
});

test("canceling the project folder picker keeps setup available and errors can be retried", async () => {
 vi.mocked(invoke).mockResolvedValueOnce(null).mockRejectedValueOnce(new Error("Agent HTTP 409: technical details")).mockResolvedValueOnce("/work/retry");
 render(<App />);
 fireEvent.click(await screen.findByRole("button",{name:"Nouveau prompt dans Example"}));
 fireEvent.click(await screen.findByRole("button",{name:"Choisir un dossier"}));
 await waitFor(()=>expect(screen.getByRole("button",{name:"Choisir un dossier"}).hasAttribute("disabled")).toBe(false));
 expect(screen.queryByRole("alert")).toBeNull();
 expect(screen.queryByText("Free prompt for p")).toBeNull();
 fireEvent.click(screen.getByRole("button",{name:"Choisir un dossier"}));
 const error=await screen.findByRole("alert");
 expect(error.textContent).toContain("Impossible d’associer ce dossier");
 expect(error.textContent).not.toContain("409");
 fireEvent.click(screen.getByRole("button",{name:"Choisir un dossier"}));
 await screen.findByText("Free prompt for p");
 expect(invoke).toHaveBeenCalledTimes(3);
});

test("mapped projects open their prompt directly",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/example"}]);
 render(<App />);
 fireEvent.click(await screen.findByRole("button",{name:"Nouveau prompt dans Example"}));
 await screen.findByText("Free prompt for p");
 expect(screen.queryByRole("button",{name:"Choisir un dossier"})).toBeNull();
 expect(screen.queryByRole("button",{name:"Nouveau prompt"})).toBeNull();
 expect(invoke).not.toHaveBeenCalled();
});

test("existing conversations stay readable without a current project folder",async()=>{
 vi.mocked(api.executionSnapshot).mockResolvedValue({status:{connected:true,capabilities:[]},runs:[{id:"saved",projectId:"p",conversation:true,title:"Saved conversation",provider:"claude",status:"running",createdAt:"2026-10-28"}]});
 vi.mocked(invoke).mockResolvedValue({id:"saved",busy:false,readOnly:false,title:"Saved conversation"});
 await conversationFor("saved").refresh();
 render(<App />);
 fireEvent.click(await screen.findByRole("button",{name:/Saved conversation/}));
 expect(screen.getByText(/Claude Code · Prête/)).toBeTruthy();
 await screen.findByText("Free prompt for p");
 expect(screen.queryByRole("button",{name:"Nouveau prompt"})).toBeNull();
 expect(screen.queryByRole("button",{name:"Configurer le projet"})).toBeNull();
 expect(screen.queryByRole("button",{name:"Tableau"})).toBeNull();
 expect(screen.getByRole("button",{name:"Nouveau prompt dans Example"})).toBeTruthy();
 expect(screen.queryByRole("button",{name:"Choisir un dossier"})).toBeNull();
});

test("project settings change the existing folder and update the toolbar and next prompt",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/old"}]);
 vi.mocked(invoke).mockImplementation(async command=>command==="conversation_profiles"?{profiles:[],defaultId:"",inherited:true}:command==="choose_project_folder"?"/work/new":null);
 render(<App />);
 const settingsButton=await screen.findByRole("button",{name:"Configurer le projet"});
 expect(settingsButton.classList.contains("icon-button")).toBe(true);
 fireEvent.click(settingsButton);
 await screen.findByRole("region",{name:"Dossier du projet"});
 expect(screen.queryByRole("button",{name:"Configurer le projet"})).toBeNull();
 expect(screen.getByText("/work/old",{selector:"code"})).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"Changer de dossier"}));
 await screen.findByText("/work/new",{selector:"code"});
 expect(screen.getByText("/work/new",{selector:".workspace-toolbar p"})).toBeTruthy();
 expect(invoke).toHaveBeenCalledWith("choose_project_folder",{projectId:"p"});
 fireEvent.click(screen.getByRole("button",{name:"Nouveau prompt"}));
 await screen.findByText("Free prompt for p");
 expect(api.stopRun).not.toHaveBeenCalled();
 expect(invoke).not.toHaveBeenCalledWith("open_folder_project");
});

test("project settings keep the old folder on cancellation or failure and permit a retry",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/old"}]);
 const choose=vi.fn().mockResolvedValueOnce(null).mockRejectedValueOnce(new Error("HTTP 409")).mockResolvedValueOnce("/work/new");
 vi.mocked(invoke).mockImplementation(async command=>command==="conversation_profiles"?{profiles:[],defaultId:"",inherited:true}:command==="choose_project_folder"?choose():null);
 render(<App />);
 fireEvent.click(await screen.findByRole("button",{name:"Configurer le projet"}));
 fireEvent.click(await screen.findByRole("button",{name:"Changer de dossier"}));
 await waitFor(()=>expect(screen.getByRole("button",{name:"Changer de dossier"}).hasAttribute("disabled")).toBe(false));
 expect(screen.getByText("/work/old",{selector:"code"})).toBeTruthy();
 expect(screen.queryByRole("alert")).toBeNull();
 fireEvent.click(screen.getByRole("button",{name:"Changer de dossier"}));
 expect((await screen.findByRole("alert")).textContent).not.toContain("409");
 expect(screen.getByText("/work/old",{selector:"code"})).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"Changer de dossier"}));
 await screen.findByText("/work/new",{selector:"code"});
});

test("project settings can also associate the first folder",async()=>{
 vi.mocked(invoke).mockImplementation(async command=>command==="conversation_profiles"?{profiles:[],defaultId:"",inherited:true}:command==="choose_project_folder"?"/work/first":null);
 render(<App />);
 fireEvent.click(await screen.findByRole("button",{name:"Configurer le projet"}));
 await screen.findByText("Aucun dossier associé à ce projet.");
 fireEvent.click(screen.getByRole("button",{name:"Choisir un dossier"}));
 await screen.findByRole("button",{name:"Changer de dossier"});
 expect(screen.getByText("/work/first",{selector:"code"})).toBeTruthy();
});

test("project settings attach a tracker directly to the selected project", async () => {
 let linked = false;
 const attach = vi.fn().mockImplementation(async () => {linked = true;return {};});
 vi.mocked(invoke).mockImplementation(async command => {
   if (command === "conversation_profiles") return {profiles:[],defaultId:"",inherited:true};
   if (command === "local_setup_status") return {enabled:true,ready:true,signedIn:true,projects:[{id:"p",name:"Example",trackers:linked?[{trackerId:"t"}]:[]}],trackers:[{id:"t",name:"Fixture tracker",provider:"github",scope:"fixture/repo"}]};
   if (command === "attach_project_tracker") return attach();
   return null;
 });
 render(<App />);
 fireEvent.click(await screen.findByRole("button",{name:"Configurer le projet"}));
 await screen.findByRole("region",{name:"Source de tickets du projet"});
 fireEvent.change(await screen.findByLabelText("Tracker à ajouter"),{target:{value:"t"}});
 fireEvent.click(screen.getByRole("button",{name:"Associer le tracker"}));
 await waitFor(()=>expect(invoke).toHaveBeenCalledWith("attach_project_tracker",{projectId:"p",trackerId:"t"}));
 expect(attach).toHaveBeenCalledOnce();
 expect(await screen.findByText("Trackers associés : Fixture tracker.")).toBeTruthy();
 expect(screen.getByText("Toutes les sources disponibles sont déjà associées à ce projet.")).toBeTruthy();
 expect(screen.queryByRole("button",{name:"Associer le tracker"})).toBeNull();
 expect(invoke).not.toHaveBeenCalledWith("create_local_project",expect.anything());
});
