import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { invoke } from "@tauri-apps/api/core";
vi.mock("@tauri-apps/api/core", () => ({ invoke: vi.fn() }));
vi.mock("../src/components/ProjectPrompt", () => ({ default: ({ projectId }: {projectId: string}) => <div>Free prompt for {projectId}</div> }));
import App from "../src/App";
import { conversationFor } from "../src/lib/conversation-controller";
import * as workspace from "../src/lib/workspace";
vi.mock("../src/lib/workspace",async original=>({...await original<typeof workspace>(),projectContext:vi.fn()}));
import * as api from "../src/lib/agent";
vi.mock("../src/lib/agent", () => ({
  listProjects: vi.fn(), listTasks: vi.fn(), executionSnapshot: vi.fn(),
  projectSkills: vi.fn(), getOrchestration: vi.fn(), launchTask: vi.fn(),
  stopRun: vi.fn(), runOrchestration: vi.fn(),
}));
vi.mock("../src/components/RunConsole", () => ({ RunConsole: () => <div>Live output</div> }));
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(workspace.projectContext).mockResolvedValue({configured:true,path:"/work/example",skills:[],policy:{}});
  vi.stubGlobal("matchMedia", () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
  vi.mocked(api.listProjects).mockResolvedValue([{ id: "p", name: "Example" }]);
  vi.mocked(api.listTasks).mockResolvedValue([]);
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
  fireEvent.click(await screen.findByRole("button", { name: /APP-1/ }));
  await screen.findByText("Live output");
  expect(screen.queryByRole("tablist")).toBeNull();
  expect(api.executionSnapshot).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: "Tableau" }));
  await screen.findByRole("textbox", { name: "Rechercher des tickets" });
  expect(screen.queryByText("Live output")).toBeNull();
  expect(api.stopRun).not.toHaveBeenCalled();
});
test("theme selection remains available without tabs", async () => {
  render(<App />);
  await screen.findByRole("button", { name: "Example" });
  expect(screen.queryByRole("button", { name: "Tableau" })).toBeNull();
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
 expect(screen.queryByRole("button",{name:"Choisir un dossier"})).toBeNull();
});

test("project settings change the existing folder and update the toolbar and next prompt",async()=>{
 vi.mocked(api.listProjects).mockResolvedValue([{id:"p",name:"Example",path:"/work/old"}]);
 vi.mocked(invoke).mockImplementation(async command=>command==="conversation_profiles"?{profiles:[],defaultId:"",inherited:true}:command==="choose_project_folder"?"/work/new":null);
 render(<App />);
 fireEvent.click(await screen.findByRole("button",{name:"Configurer le projet"}));
 await screen.findByRole("region",{name:"Dossier du projet"});
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
