vi.mock("../src/components/ProviderSettings", () => ({ ProviderSettings: () => null }));
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { LocalSettings } from "../src/components/LocalSettings";
import * as api from "../src/lib/local";
vi.mock("../src/components/OAuthConnections", () => ({ OAuthConnections: () => <div>OAuth connections</div> }));
vi.mock("../src/lib/local", () => ({ localSetupStatus: vi.fn(), enableLocalRuntime: vi.fn(), attachFleetAgent: vi.fn(), saveLocalProfile: vi.fn(), saveLocalCredential: vi.fn(), createLocalTracker: vi.fn(), createLocalProject: vi.fn(), mapLocalProject: vi.fn(), syncLocalTracker: vi.fn() }));
const ready = { enabled: true, ready: true, signedIn: true, agentReady: true, profile: { email: "fixture@example.invalid", displayName: "Fixture" }, trackers: [{id:"t",name:"Tracker",provider:"github",scope:"fixture/repo"}], projects: [{id:"p",name:"Project"}] };
beforeEach(() => { vi.resetAllMocks(); });
afterEach(() => cleanup());
test("Local activation waits for the local server before exposing profile setup", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValueOnce({ enabled: false }).mockResolvedValue({enabled:true,ready:true,signedIn:false});
  let finish!: () => void;
  vi.mocked(api.enableLocalRuntime).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  render(<LocalSettings onChanged={vi.fn()} />);
  const enable = await screen.findByRole("button",{name:"Démarrer en mode Local"});
  fireEvent.click(enable); fireEvent.click(enable);
  expect(api.enableLocalRuntime).toHaveBeenCalledOnce();
  expect(screen.queryByLabelText("Adresse e-mail")).toBeNull();
  finish();
  await screen.findByLabelText("Adresse e-mail");
});
test("Fleet is attached explicitly and hides Local-only setup", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValueOnce({ enabled: false, mode: "unconfigured" }).mockResolvedValue({ enabled: true, ready: true, signedIn: true, agentReady: true, mode: "fleet", fleetServer: "https://fleet.example.invalid" });
  vi.mocked(api.attachFleetAgent).mockResolvedValue();
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button",{name:"Associer l’agent Fleet existant"}));
  await waitFor(() => expect(api.attachFleetAgent).toHaveBeenCalledOnce());
  expect(screen.queryByLabelText("Adresse e-mail")).toBeNull();
  await waitFor(() => expect(screen.queryByRole("button",{name:"Associer l’agent Fleet existant"})).toBeNull());
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
test("repository mapping retries independently of project creation", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
  vi.mocked(api.mapLocalProject).mockRejectedValue(new Error("Invalid repository"));
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button",{name:"Projets"}));
  fireEvent.change(await screen.findByLabelText("Projet à associer"),{target:{value:"p"}});
  fireEvent.change(screen.getByLabelText("Chemin absolu du dépôt Git"),{target:{value:"/fixture/repo"}});
  fireEvent.click(screen.getByRole("button",{name:"Associer le dépôt"}));
  await waitFor(()=>expect(api.mapLocalProject).toHaveBeenCalledWith("p","/fixture/repo"));
  await screen.findByRole("alert");
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
test("the projects section offers the local folder flow without a tracker", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 const open=vi.fn();render(<LocalSettings onChanged={vi.fn()} onOpenFolder={open} />);
 fireEvent.click(await screen.findByRole("button",{name:"Projets"}));
 fireEvent.click(screen.getByRole("button",{name:"Choisir un dossier local"}));
 expect(open).toHaveBeenCalledOnce();
 expect(api.createLocalProject).not.toHaveBeenCalled();
});
