vi.mock("../src/components/ProviderSettings", () => ({ ProviderSettings: () => null }));
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { LocalSettings } from "../src/components/LocalSettings";
import * as api from "../src/lib/local";
vi.mock("../src/components/OAuthConnections", () => ({ OAuthConnections: ({serverManaged}:{serverManaged?:boolean}) => <div data-server-managed={String(serverManaged??false)}>OAuth connections</div> }));
vi.mock("../src/lib/local", () => ({ localSetupStatus: vi.fn(), enableLocalRuntime: vi.fn(), attachFleetAgent: vi.fn(), pairFleetAgent: vi.fn(), saveLocalProfile: vi.fn(), saveLocalCredential: vi.fn(), createLocalTracker: vi.fn(), createLocalProject: vi.fn(), chooseProjectFolder: vi.fn(), syncLocalTracker: vi.fn(), trackerSyncStatus: vi.fn() }));
const ready = { enabled: true, ready: true, signedIn: true, agentReady: true, profile: { email: "fixture@example.invalid", displayName: "Fixture" }, trackers: [{id:"t",name:"Tracker",provider:"github",scope:"fixture/repo"}], projects: [{id:"p",name:"Project"}] };
beforeEach(() => { vi.resetAllMocks(); });
afterEach(() => cleanup());
test("Local activation waits for the local server before exposing profile setup", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValueOnce({ enabled: false }).mockResolvedValue({enabled:true,ready:true,signedIn:false});
  let finish!: () => void;
  vi.mocked(api.enableLocalRuntime).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  render(<LocalSettings onChanged={vi.fn()} />);
  expect(await screen.findByText("CONNEXION À SECTILE")).toBeTruthy();
  const enable = await screen.findByRole("button",{name:"Démarrer en mode Local"});
  fireEvent.click(enable); fireEvent.click(enable);
  expect(api.enableLocalRuntime).toHaveBeenCalledOnce();
  expect(screen.queryByLabelText("Adresse e-mail")).toBeNull();
  finish();
  await screen.findByLabelText("Adresse e-mail");
});
test("Fleet attachment enables backend setup and hides Local-only controls", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValueOnce({ enabled: false, mode: "unconfigured" }).mockResolvedValue({ enabled: true, ready: true, signedIn: true, agentReady: true, mode: "fleet", fleetServer: "https://fleet.example.invalid" });
  vi.mocked(api.attachFleetAgent).mockResolvedValue();
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button",{name:"Associer l’agent Fleet existant"}));
  await waitFor(() => expect(api.attachFleetAgent).toHaveBeenCalledOnce());
  expect(await screen.findByText("VOTRE ESPACE FLEET")).toBeTruthy();
  expect(screen.getByRole("heading", {name:"Configuration Fleet"})).toBeTruthy();
  expect(screen.getByRole("heading", {name:"Connexion et services"})).toBeTruthy();
  expect(screen.getByRole("heading", {name:"Agent Fleet"})).toBeTruthy();
  expect(screen.getByText(/État de la connexion et de l’agent Fleet associé/)).toBeTruthy();
  fireEvent.click(screen.getByRole("button", {name:"Comptes et trackers"}));
  expect(await screen.findByLabelText("Adresse e-mail")).toBeTruthy();
  expect((screen.getByRole("button",{name:"Comptes et trackers"}) as HTMLButtonElement).disabled).toBe(false);
  expect((screen.getByRole("button",{name:"Projets"}) as HTMLButtonElement).disabled).toBe(false);
  fireEvent.click(screen.getByRole("button",{name:"Profil"}));
  expect(await screen.findByRole("heading",{name:"Profil Fleet"})).toBeTruthy();
  expect(screen.getByText(/Ce profil identifie vos actions avec votre compte sur le backend Fleet/)).toBeTruthy();
  await waitFor(() => expect(screen.queryByRole("button",{name:"Associer l’agent Fleet existant"})).toBeNull());
});
test("Fleet pairing sends the one-time code to native code and clears it after failure", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue({ enabled: false, mode: "unconfigured" });
  vi.mocked(api.pairFleetAgent).mockRejectedValue(new Error("Invalid pairing code"));
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.change(await screen.findByLabelText("Adresse du backend Fleet"),{target:{value:"https://fleet.example.invalid"}});
  const code=screen.getByLabelText("Code de jumelage");
  fireEvent.change(code,{target:{value:"one-time-code"}});
  fireEvent.click(screen.getByRole("button",{name:"Associer ce poste à Fleet"}));
  await waitFor(() => expect(api.pairFleetAgent).toHaveBeenCalledWith("https://fleet.example.invalid","one-time-code"));
  await screen.findByRole("alert");
  await waitFor(() => expect((code as HTMLInputElement).value).toBe(""));
});
test("Fleet pairing hydrates the signed-in profile returned by the selected backend", async () => {
  const profile = { email: "fleet-user@example.invalid", displayName: "Fleet User", role: "member" };
  vi.mocked(api.localSetupStatus)
    .mockResolvedValueOnce({ enabled: false, mode: "unconfigured" })
    .mockResolvedValue({ enabled: true, ready: true, signedIn: true, agentReady: true, mode: "fleet", apiSetupAvailable: true, profile });
  vi.mocked(api.pairFleetAgent).mockResolvedValue();
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.change(await screen.findByLabelText("Adresse du backend Fleet"),{target:{value:"https://fleet.example.invalid"}});
  fireEvent.change(screen.getByLabelText("Code de jumelage"),{target:{value:"one-time-code"}});
  fireEvent.click(screen.getByRole("button",{name:"Associer ce poste à Fleet"}));
  await screen.findByText("Poste associé au backend Fleet.");
  fireEvent.click(screen.getByRole("button",{name:"Profil"}));
  await waitFor(() => expect((screen.getByLabelText("Adresse e-mail") as HTMLInputElement).value).toBe(profile.email));
  expect((screen.getByLabelText("Adresse e-mail") as HTMLInputElement).readOnly).toBe(true);
  expect((screen.getByLabelText("Nom affiché") as HTMLInputElement).value).toBe(profile.displayName);
});
test("attaching an existing Fleet agent hydrates the signed-in profile returned by the backend", async () => {
  const profile = { email: "attached-user@example.invalid", displayName: "Attached User", role: "member" };
  vi.mocked(api.localSetupStatus)
    .mockResolvedValueOnce({ enabled: false, mode: "unconfigured" })
    .mockResolvedValue({ enabled: true, ready: true, signedIn: true, agentReady: true, mode: "fleet", apiSetupAvailable: true, profile });
  vi.mocked(api.attachFleetAgent).mockResolvedValue();
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByText("J’ai déjà associé un agent sur ce poste"));
  fireEvent.click(screen.getByRole("button", { name: "Associer l’agent Fleet existant" }));
  await screen.findByText("Agent Fleet associé à ce desktop.");
  fireEvent.click(screen.getByRole("button", { name: "Profil" }));
  await waitFor(() => expect((screen.getByLabelText("Adresse e-mail") as HTMLInputElement).value).toBe(profile.email));
  expect((screen.getByLabelText("Adresse e-mail") as HTMLInputElement).readOnly).toBe(true);
  expect((screen.getByLabelText("Nom affiché") as HTMLInputElement).value).toBe(profile.displayName);
});
test("paired Fleet enables native setup while keeping shared tracker creation admin-only", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue({
    enabled: true, ready: true, signedIn: true, agentReady: true, apiSetupAvailable: true,
    mode: "fleet", fleetServer: "https://fleet.example.invalid",
    profile: {email:"fixture@example.invalid",displayName:"Fixture",role:"member"},
    trackers: [{id:"t",name:"Tracker",provider:"github",scope:"fixture/repo"}],
    projects: [{id:"p",name:"Project"}],
  });
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button",{name:"Comptes et trackers"}));
  expect(screen.getByText("OAuth connections").getAttribute("data-server-managed")).toBe("true");
  expect(screen.getByText(/Les trackers sont partagés dans Fleet/)).toBeTruthy();
  fireEvent.click(screen.getByRole("button",{name:"Projets"}));
  fireEvent.change(await screen.findByLabelText("Nom du projet"),{target:{value:"New Fleet project"}});
  fireEvent.change(screen.getByLabelText("Tracker"),{target:{value:"t"}});
  fireEvent.click(screen.getByRole("button",{name:"Créer le projet"}));
  await waitFor(() => expect(api.createLocalProject).toHaveBeenCalledWith("New Fleet project","t"));
});
test("a credential is sent only on submission and its input is cleared after failure", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
  vi.mocked(api.saveLocalCredential).mockRejectedValue(new Error("Connection unavailable"));
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button",{name:"Comptes et trackers"}));
  fireEvent.click(await screen.findByText("Accès manuel par jeton · option avancée"));
  const token = await screen.findByLabelText("Jeton personnel");
  fireEvent.change(token,{target:{value:"fixture-secret"}});
  expect(api.saveLocalCredential).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button",{name:"Enregistrer l’accès"}));
  await screen.findByRole("alert");
  expect((token as HTMLInputElement).value).toBe("");
  expect(api.saveLocalCredential).toHaveBeenCalledWith({tracker:"github",siteUrl:"",email:"",token:"fixture-secret"});
});
test("existing projects use the native folder picker for repository mapping", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
  vi.mocked(api.chooseProjectFolder).mockResolvedValue("/fixture/repo");
  const onChanged=vi.fn();
  render(<LocalSettings onChanged={onChanged} />);
  fireEvent.click(await screen.findByRole("button",{name:"Projets"}));
  fireEvent.change(await screen.findByLabelText("Projet à associer"),{target:{value:"p"}});
  fireEvent.click(screen.getByRole("button",{name:"Choisir un dossier"}));
  await screen.findByText("Dossier local associé au projet.");
  expect(api.chooseProjectFolder).toHaveBeenCalledWith("p");
  expect(onChanged).toHaveBeenCalledOnce();
  expect(api.createLocalProject).not.toHaveBeenCalled();
});

test("cancelling the native folder picker leaves the project unchanged", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
  vi.mocked(api.chooseProjectFolder).mockResolvedValue(null);
  const onChanged=vi.fn();
  render(<LocalSettings onChanged={onChanged} />);
  fireEvent.click(await screen.findByRole("button",{name:"Projets"}));
  fireEvent.change(await screen.findByLabelText("Projet à associer"),{target:{value:"p"}});
  fireEvent.click(screen.getByRole("button",{name:"Choisir un dossier"}));
  await waitFor(()=>expect(api.chooseProjectFolder).toHaveBeenCalledWith("p"));
  await waitFor(()=>expect(screen.queryByRole("status")).toBeNull());
  expect(api.localSetupStatus).toHaveBeenCalledOnce();
  expect(onChanged).not.toHaveBeenCalled();
  expect(api.createLocalProject).not.toHaveBeenCalled();
});

test("settings navigation preserves a tracker draft and never saves on navigation", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 render(<LocalSettings onChanged={vi.fn()} />);
 fireEvent.click(await screen.findByRole("button",{name:"Comptes et trackers"}));
 fireEvent.change(screen.getByLabelText("Nom du tracker"),{target:{value:"Draft tracker"}});
 fireEvent.click(screen.getByRole("button",{name:"Moteurs IA"}));
 expect(screen.queryByRole("button",{name:"Ajouter le tracker"})).toBeNull();
 fireEvent.click(screen.getByRole("button",{name:"Comptes et trackers"}));
 expect((screen.getByLabelText("Nom du tracker") as HTMLInputElement).value).toBe("Draft tracker");
 expect(api.createLocalTracker).not.toHaveBeenCalled();
});
test("tracker sync distinguishes queued work from a completed read",async()=>{
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 vi.mocked(api.syncLocalTracker).mockResolvedValue({queued:true,activity:{id:"sync-1",status:"queued"}});
 let complete!:(value:api.TrackerSyncActivity)=>void;
 vi.mocked(api.trackerSyncStatus).mockReturnValue(new Promise(resolve=>{complete=resolve;}));
 render(<LocalSettings onChanged={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Comptes et trackers"}));
 fireEvent.click(screen.getByRole("button",{name:"Synchroniser"}));
 expect(await screen.findByText("Synchronisation en cours…")).toBeTruthy();
 expect(screen.getByRole("button",{name:"Synchroniser"}).hasAttribute("disabled")).toBe(true);
 complete({id:"sync-1",status:"completed",summary:"2 GitHub issues synchronized successfully"});
 expect(await screen.findByText("2 tickets synchronisés.")).toBeTruthy();
 await waitFor(()=>expect(screen.getByRole("button",{name:"Synchroniser"}).hasAttribute("disabled")).toBe(false));
 expect(api.trackerSyncStatus).toHaveBeenCalledWith("sync-1");
});
test("tracker sync reports a backend failure instead of claiming tickets arrived",async()=>{
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 vi.mocked(api.syncLocalTracker).mockResolvedValue({queued:true,activity:{id:"sync-2",status:"queued"}});
 vi.mocked(api.trackerSyncStatus).mockResolvedValue({id:"sync-2",status:"failed",error:"Permission denied"});
 render(<LocalSettings onChanged={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Comptes et trackers"}));
 fireEvent.click(screen.getByRole("button",{name:"Synchroniser"}));
 expect(await screen.findByRole("alert")).toHaveProperty("textContent","Synchronisation échouée : Permission denied");
 await waitFor(()=>expect(screen.getByRole("button",{name:"Synchroniser"}).hasAttribute("disabled")).toBe(false));
});
test("the projects section offers the local folder flow without a tracker", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 const open=vi.fn();render(<LocalSettings onChanged={vi.fn()} onOpenFolder={open} />);
 fireEvent.click(await screen.findByRole("button",{name:"Projets"}));
 fireEvent.click(screen.getByRole("button",{name:"Choisir un dossier local"}));
 expect(open).toHaveBeenCalledOnce();
 expect(api.createLocalProject).not.toHaveBeenCalled();
});
