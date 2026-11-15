vi.mock("../src/components/ProviderSettings", () => ({ ProviderSettings: () => <div>Provider settings ready</div> }));
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { LocalSettings } from "../src/components/LocalSettings";
import * as api from "../src/lib/local";
vi.mock("../src/components/OAuthConnections", () => ({ OAuthConnections: ({serverManaged}:{serverManaged?:boolean}) => <div data-server-managed={String(serverManaged??false)}>OAuth connections</div> }));
vi.mock("../src/lib/local", () => ({ localSetupStatus: vi.fn(), localServiceLog: vi.fn(), enableLocalRuntime: vi.fn(), prepareLocalUpdate: vi.fn(), attachFleetAgent: vi.fn(), pairFleetAgent: vi.fn(), saveLocalProfile: vi.fn(), saveLocalCredential: vi.fn(), createLocalTracker: vi.fn(), updateLocalTracker: vi.fn(), deleteLocalTracker: vi.fn(), createLocalProject: vi.fn(), deleteLocalProject: vi.fn(), attachProjectTracker: vi.fn(), chooseProjectFolder: vi.fn(), syncLocalTracker: vi.fn(), trackerSyncStatus: vi.fn(), activeTrackerSyncs: vi.fn() }));
const ready = { enabled: true, ready: true, signedIn: true, agentReady: true, profile: { email: "fixture@example.invalid", displayName: "Fixture" }, trackers: [{id:"t",name:"Tracker",provider:"github",scope:"fixture/repo"}], projects: [{id:"p",name:"Project"}] };
test("an administrator can edit a tracker and explicitly confirm deletion", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({...ready,mode:"local"});
 vi.mocked(api.updateLocalTracker).mockResolvedValue({});
 vi.mocked(api.deleteLocalTracker).mockResolvedValue({});
 render(<LocalSettings onChanged={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Accounts and trackers"}));
 fireEvent.click(screen.getByRole("button",{name:"Edit"}));
 fireEvent.change(screen.getByLabelText("Tracker name to edit"),{target:{value:"Renamed"}});
 fireEvent.click(screen.getByRole("button",{name:"Save changes"}));
 await waitFor(()=>expect(api.updateLocalTracker).toHaveBeenCalledWith("t",{name:"Renamed",provider:"github",site:"",scope:"fixture/repo"}));
 fireEvent.click(screen.getByRole("button",{name:"Delete"}));
 expect(api.deleteLocalTracker).not.toHaveBeenCalled();
 fireEvent.click(screen.getByRole("button",{name:"Confirm deletion"}));
 await waitFor(()=>expect(api.deleteLocalTracker).toHaveBeenCalledWith("t"));
});
test("a Fleet member cannot edit or delete shared trackers", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({...ready,mode:"fleet",profile:{...ready.profile,role:"member"}});
 render(<LocalSettings onChanged={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Accounts and trackers"}));
 expect(screen.queryByRole("button",{name:"Edit"})).toBeNull();
 expect(screen.queryByRole("button",{name:"Delete"})).toBeNull();
});
beforeEach(() => { vi.resetAllMocks(); vi.mocked(api.localServiceLog).mockResolvedValue({path:"/test/companion.log",text:"service ready",missing:false,truncated:false}); vi.mocked(api.activeTrackerSyncs).mockResolvedValue([]); });
afterEach(() => cleanup());
test("provider detection starts only after the agent becomes available", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValueOnce({enabled:true,mode:"local",ready:true,signedIn:false,agentReady:false}).mockResolvedValue({...ready,mode:"local"});
 render(<LocalSettings onChanged={vi.fn()} />);
 await screen.findByLabelText("Email address");
 expect(screen.queryByText("Provider settings ready")).toBeNull();
 fireEvent.click(screen.getByRole("button",{name:"Local services"}));
 fireEvent.click(screen.getByRole("button",{name:"Refresh",exact:true}));
 await waitFor(()=>expect(screen.getByText("Provider settings ready")).toBeTruthy());
});
test("Local activation waits for the local server before exposing profile setup", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValueOnce({ enabled: false }).mockResolvedValue({enabled:true,ready:true,signedIn:false});
  let finish!: () => void;
  vi.mocked(api.enableLocalRuntime).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  render(<LocalSettings onChanged={vi.fn()} />);
  expect(await screen.findByText("CONNECT TO SECTILE")).toBeTruthy();
  const enable = await screen.findByRole("button",{name:"Start in Local mode"});
  fireEvent.click(enable); fireEvent.click(enable);
  expect(api.enableLocalRuntime).toHaveBeenCalledOnce();
  expect(screen.queryByLabelText("Email address")).toBeNull();
  finish();
  await screen.findByLabelText("Email address");
});
test("service diagnostics can be read even when the Local companion is not ready", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({enabled:true,mode:"local",ready:false,signedIn:false,agentReady:false});
 render(<LocalSettings onChanged={vi.fn()} />);
 expect(await screen.findByRole("heading",{name:"Local companion log"})).toBeTruthy();
 expect(await screen.findByText("service ready")).toBeTruthy();
 expect(api.localServiceLog).toHaveBeenCalledWith("companion");
 expect(screen.getByLabelText("Service log content").textContent).toBe("service ready");
 vi.mocked(api.localServiceLog).mockResolvedValue({path:"/test/companion.log",text:"latest line",missing:false,truncated:true});
 fireEvent.click(screen.getByRole("button",{name:"Refresh log"}));
 await screen.findByText("latest line");
 expect(screen.getByText("Showing the last 256 KiB of the log.")).toBeTruthy();
});
test("first-start failure exposes both Local and Fleet diagnostics", async () => {
 vi.mocked(api.localSetupStatus).mockRejectedValue(new Error("Startup failed"));
 vi.mocked(api.localServiceLog).mockImplementation(async service=>({path:`/test/${service}.log`,text:`${service} diagnostic`,missing:false,truncated:false}));
 render(<LocalSettings onChanged={vi.fn()} />);
 expect((await screen.findByRole("alert")).textContent).toContain("Startup failed");
 expect(await screen.findByText("companion diagnostic")).toBeTruthy();
 expect(await screen.findByText("agent diagnostic")).toBeTruthy();
 expect(api.localServiceLog).toHaveBeenCalledWith("companion");
 expect(api.localServiceLog).toHaveBeenCalledWith("agent");
 expect(screen.queryByRole("button",{name:"Start in Local mode"})).toBeNull();
 expect(screen.queryByRole("button",{name:"Pair this workstation with Fleet"})).toBeNull();
});
test("offline Fleet keeps its selected mode and diagnostics without offering Local setup", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({enabled:true,mode:"fleet",ready:false,signedIn:false,agentReady:false,apiSetupAvailable:true,fleetServer:"http://127.0.0.1:65534",backendError:"The Fleet backend is unreachable. Check its connection."});
 render(<LocalSettings onChanged={vi.fn()} />);
 expect(await screen.findByRole("heading",{name:"Fleet configuration"})).toBeTruthy();
 expect(await screen.findByRole("heading",{name:"Fleet agent log"})).toBeTruthy();
 expect(screen.getByRole("alert").textContent).toContain("Fleet backend is unreachable");
 expect(screen.queryByRole("heading",{name:"Local companion log"})).toBeNull();
 expect(screen.queryByRole("button",{name:"Start in Local mode"})).toBeNull();
 expect((screen.getByRole("button",{name:"Profile"}) as HTMLButtonElement).disabled).toBe(true);
});
test("Fleet setup becomes available after an agent or backend reconnects", async () => {
 vi.mocked(api.localSetupStatus)
  .mockResolvedValueOnce({enabled:true,mode:"fleet",ready:false,signedIn:false,agentReady:false,apiSetupAvailable:true,fleetServer:"https://fleet.example.invalid",backendError:"The attached Fleet agent is unreachable."})
  .mockResolvedValue({enabled:true,mode:"fleet",ready:true,signedIn:true,agentReady:true,apiSetupAvailable:true,fleetServer:"https://fleet.example.invalid",profile:{email:"fixture@example.invalid",displayName:"Fixture"}});
 render(<LocalSettings onChanged={vi.fn()} />);
 expect((await screen.findByRole("alert")).textContent).toContain("attached Fleet agent");
 expect((screen.getByRole("button",{name:"Profile"}) as HTMLButtonElement).disabled).toBe(true);
 fireEvent.click(screen.getByRole("button",{name:"Refresh"}));
 await screen.findByText("Status refreshed.");
 await waitFor(() => expect((screen.getByRole("button",{name:"Profile"}) as HTMLButtonElement).disabled).toBe(false));
 expect(screen.queryByRole("alert")).toBeNull();
 fireEvent.click(screen.getByRole("button",{name:"Profile"}));
 expect((await screen.findByLabelText("Email address") as HTMLInputElement).value).toBe("fixture@example.invalid");
 expect(api.localSetupStatus).toHaveBeenCalledTimes(2);
});
test("manual status refresh makes one retry after startup status failed", async () => {
 vi.mocked(api.localSetupStatus).mockRejectedValueOnce(new Error("First status failed")).mockResolvedValue({enabled:false,mode:"unconfigured"});
 render(<LocalSettings onChanged={vi.fn()} />);
 expect((await screen.findByRole("alert")).textContent).toContain("First status failed");
 fireEvent.click(screen.getByRole("button",{name:"Refresh"}));
 await screen.findByText("Status refreshed.");
 expect(api.localSetupStatus).toHaveBeenCalledTimes(2);
});
test("legacy Fleet attachment keeps diagnostics available without native backend setup", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValueOnce({ enabled: false, mode: "unconfigured" }).mockResolvedValue({ enabled: true, ready: true, signedIn: false, agentReady: true, mode: "fleet", apiSetupAvailable: false, fleetServer: "https://fleet.example.invalid" });
  vi.mocked(api.localServiceLog).mockResolvedValue({path:"/test/fleet/agent.log",text:"fleet ready",missing:false,truncated:false});
  vi.mocked(api.attachFleetAgent).mockResolvedValue();
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button",{name:"Attach existing Fleet agent"}));
  await waitFor(() => expect(api.attachFleetAgent).toHaveBeenCalledOnce());
  expect(await screen.findByText("YOUR FLEET WORKSPACE")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", {name:"Connection and services"}));
  expect(screen.getByRole("heading", {name:"Fleet configuration"})).toBeTruthy();
  expect(screen.getByRole("heading", {name:"Connection and services"})).toBeTruthy();
  expect(screen.getByRole("heading", {name:"Fleet Agent"})).toBeTruthy();
  expect(await screen.findByRole("heading", {name:"Fleet agent log"})).toBeTruthy();
  expect(await screen.findByText("fleet ready")).toBeTruthy();
  expect(api.localServiceLog).toHaveBeenCalledWith("agent");
  expect(screen.getByText(/Connection status and Fleet agent attached to this workstation/)).toBeTruthy();
  expect(screen.getByText("This Fleet agent is managed by its original installation.")).toBeTruthy();
  expect((screen.getByRole("button",{name:"Accounts and trackers"}) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button",{name:"Projects"}) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button",{name:"Profile"}) as HTMLButtonElement).disabled).toBe(true);
  await waitFor(() => expect(screen.queryByRole("button",{name:"Attach existing Fleet agent"})).toBeNull());
});
test("Fleet pairing sends the one-time code to native code and clears it after failure", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue({ enabled: false, mode: "unconfigured" });
  vi.mocked(api.pairFleetAgent).mockRejectedValue(new Error("Invalid pairing code"));
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.change(await screen.findByLabelText("Fleet backend URL"),{target:{value:"https://fleet.example.invalid"}});
  const code=screen.getByLabelText("Pairing code");
  fireEvent.change(code,{target:{value:"one-time-code"}});
  fireEvent.click(screen.getByRole("button",{name:"Pair this workstation with Fleet"}));
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
  fireEvent.change(await screen.findByLabelText("Fleet backend URL"),{target:{value:"https://fleet.example.invalid"}});
  fireEvent.change(screen.getByLabelText("Pairing code"),{target:{value:"one-time-code"}});
  fireEvent.click(screen.getByRole("button",{name:"Pair this workstation with Fleet"}));
  await screen.findByText("Workstation paired with Fleet backend.");
  fireEvent.click(screen.getByRole("button",{name:"Connection and services"}));
  expect(screen.getByText("The Fleet agent starts automatically on this workstation and continues in the background when you close the desktop.")).toBeTruthy();
  fireEvent.click(screen.getByRole("button",{name:"Profile"}));
  await waitFor(() => expect((screen.getByLabelText("Email address") as HTMLInputElement).value).toBe(profile.email));
  expect((screen.getByLabelText("Email address") as HTMLInputElement).readOnly).toBe(true);
  expect((screen.getByLabelText("Display name") as HTMLInputElement).value).toBe(profile.displayName);
});
test("attaching an existing Fleet agent hydrates the signed-in profile returned by the backend", async () => {
  const profile = { email: "attached-user@example.invalid", displayName: "Attached User", role: "member" };
  vi.mocked(api.localSetupStatus)
    .mockResolvedValueOnce({ enabled: false, mode: "unconfigured" })
    .mockResolvedValue({ enabled: true, ready: true, signedIn: true, agentReady: true, mode: "fleet", apiSetupAvailable: true, profile });
  vi.mocked(api.attachFleetAgent).mockResolvedValue();
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByText("I already paired an agent on this workstation"));
  fireEvent.click(screen.getByRole("button", { name: "Attach existing Fleet agent" }));
  await screen.findByText("Fleet agent attached to this desktop.");
  fireEvent.click(screen.getByRole("button", { name: "Profile" }));
  await waitFor(() => expect((screen.getByLabelText("Email address") as HTMLInputElement).value).toBe(profile.email));
  expect((screen.getByLabelText("Email address") as HTMLInputElement).readOnly).toBe(true);
  expect((screen.getByLabelText("Display name") as HTMLInputElement).value).toBe(profile.displayName);
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
  fireEvent.click(await screen.findByRole("button",{name:"Accounts and trackers"}));
  expect(screen.getByText("OAuth connections").getAttribute("data-server-managed")).toBe("true");
  expect(screen.getByText("OAuth connections").closest("details")).toBeNull();
  expect(screen.getByText("Manual token access · advanced option").closest("details")?.open).toBe(false);
  expect(screen.getByText(/Trackers are shared in Fleet/)).toBeTruthy();
  fireEvent.click(screen.getByRole("button",{name:"Projects"}));
  fireEvent.change(await screen.findByLabelText("Project name"),{target:{value:"New Fleet project"}});
  fireEvent.change(screen.getByLabelText("Tracker"),{target:{value:"t"}});
  fireEvent.click(screen.getByRole("button",{name:"Create project"}));
  await waitFor(() => expect(api.createLocalProject).toHaveBeenCalledWith("New Fleet project","t"));
});
test("a credential is sent only on submission and its input is cleared after failure", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
  vi.mocked(api.saveLocalCredential).mockRejectedValue(new Error("Connection unavailable"));
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button",{name:"Accounts and trackers"}));
  expect(screen.getByText("Personal tracker access")).toBeTruthy();
  expect(screen.getByText("Existing local OAuth connection").closest("details")?.open).toBe(false);
  const token = await screen.findByLabelText("Personal access token");
  expect(token.closest("details")).toBeNull();
  expect(screen.queryByLabelText("Site or API URL")).toBeNull();
  expect(screen.queryByLabelText("Account email")).toBeNull();
  fireEvent.change(token,{target:{value:"fixture-secret"}});
  expect(api.saveLocalCredential).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button",{name:"Save access"}));
  await screen.findByRole("alert");
  expect((token as HTMLInputElement).value).toBe("");
  expect(api.saveLocalCredential).toHaveBeenCalledWith({tracker:"github",siteUrl:"",email:"",token:"fixture-secret"});
});
test("Local personal access shows only provider-specific fields and labels an existing OAuth grant", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue({ ...ready, mode:"local", credentials:{credentials:[{tracker:"jira",kind:"oauth",account:"Fixture",siteUrl:"https://example.atlassian.net"}]} });
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button",{name:"Accounts and trackers"}));
  expect(screen.getByText(/OAuth · Fixture · https:\/\/example.atlassian.net/)).toBeTruthy();
  fireEvent.change(screen.getByLabelText("Provider"),{target:{value:"gitlab"}});
  expect((screen.getByLabelText("Site or API URL") as HTMLInputElement).value).toBe("https://gitlab.com");
  expect(screen.queryByLabelText("Account email")).toBeNull();
  fireEvent.change(screen.getByLabelText("Provider"),{target:{value:"jira"}});
  expect(screen.getByLabelText("Account email")).toBeTruthy();
  expect((screen.getByLabelText("Site or API URL") as HTMLInputElement).value).toBe("");
});
test("tracker source and personal credential keep independent provider and site drafts", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
  vi.mocked(api.createLocalTracker).mockResolvedValue({});
  render(<LocalSettings onChanged={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button", {name:"Accounts and trackers"}));
  fireEvent.change(screen.getByLabelText("Provider"), {target:{value:"jira"}});
  fireEvent.change(screen.getByLabelText("Site or API URL"), {target:{value:"https://personal.atlassian.net"}});
  fireEvent.change(screen.getByLabelText("Source provider"), {target:{value:"gitlab"}});
  fireEvent.change(screen.getByLabelText("Source URL"), {target:{value:"https://gitlab.example.invalid"}});
  expect((screen.getByLabelText("Provider") as HTMLSelectElement).value).toBe("jira");
  expect((screen.getByLabelText("Site or API URL") as HTMLInputElement).value).toBe("https://personal.atlassian.net");
  fireEvent.change(screen.getByLabelText("Tracker name"), {target:{value:"Engineering"}});
  fireEvent.change(screen.getByLabelText("Repository or project key"), {target:{value:"team/engineering"}});
  fireEvent.click(screen.getByRole("button", {name:"Add tracker"}));
  await waitFor(() => expect(api.createLocalTracker).toHaveBeenCalledWith({name:"Engineering",provider:"gitlab",site:"https://gitlab.example.invalid/api/v4",scope:"team/engineering"}));
  expect(api.saveLocalCredential).not.toHaveBeenCalled();
});
test("existing projects use the native folder picker for repository mapping", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
  vi.mocked(api.chooseProjectFolder).mockResolvedValue("/fixture/repo");
  const onChanged=vi.fn();
  render(<LocalSettings onChanged={onChanged} />);
  fireEvent.click(await screen.findByRole("button",{name:"Projects"}));
  fireEvent.change(await screen.findByLabelText("Project to map"),{target:{value:"p"}});
  fireEvent.click(screen.getByRole("button",{name:"Choose folder"}));
  await screen.findByText("Local folder mapped to project.");
  expect(api.chooseProjectFolder).toHaveBeenCalledWith("p");
  expect(onChanged).toHaveBeenCalledOnce();
  expect(api.createLocalProject).not.toHaveBeenCalled();
});

test("cancelling the native folder picker leaves the project unchanged", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
  vi.mocked(api.chooseProjectFolder).mockResolvedValue(null);
  const onChanged=vi.fn();
  render(<LocalSettings onChanged={onChanged} />);
  fireEvent.click(await screen.findByRole("button",{name:"Projects"}));
  fireEvent.change(await screen.findByLabelText("Project to map"),{target:{value:"p"}});
  fireEvent.click(screen.getByRole("button",{name:"Choose folder"}));
  await waitFor(()=>expect(api.chooseProjectFolder).toHaveBeenCalledWith("p"));
  await waitFor(()=>expect(screen.queryByRole("status")).toBeNull());
  expect(api.localSetupStatus).toHaveBeenCalledOnce();
  expect(onChanged).not.toHaveBeenCalled();
  expect(api.createLocalProject).not.toHaveBeenCalled();
});

test("settings navigation preserves a tracker draft and never saves on navigation", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 render(<LocalSettings onChanged={vi.fn()} />);
 fireEvent.click(await screen.findByRole("button",{name:"Accounts and trackers"}));
 fireEvent.change(screen.getByLabelText("Tracker name"),{target:{value:"Draft tracker"}});
 fireEvent.click(screen.getByRole("button",{name:"AI engines"}));
 expect(screen.queryByRole("button",{name:"Add tracker"})).toBeNull();
 fireEvent.click(screen.getByRole("button",{name:"Accounts and trackers"}));
 expect((screen.getByLabelText("Tracker name") as HTMLInputElement).value).toBe("Draft tracker");
 expect(api.createLocalTracker).not.toHaveBeenCalled();
});
test("AI engine settings use one page heading without repeating the section heading", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({...ready,mode:"local"});
 render(<LocalSettings onChanged={vi.fn()} />);
 fireEvent.click(await screen.findByRole("button",{name:"AI engines"}));
 expect(screen.getAllByRole("heading",{name:"AI engines"})).toHaveLength(1);
 expect(screen.getByText("Connect Claude Code and Codex to work in your projects.")).toBeTruthy();
});
test("tracker sync distinguishes queued work from a completed read",async()=>{
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 vi.mocked(api.syncLocalTracker).mockResolvedValue({queued:true,activity:{id:"sync-1",status:"queued"}});
 let complete!:(value:api.TrackerSyncActivity)=>void;
 vi.mocked(api.trackerSyncStatus).mockReturnValue(new Promise(resolve=>{complete=resolve;}));
 render(<LocalSettings onChanged={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Accounts and trackers"}));
 fireEvent.click(screen.getByRole("button",{name:"Sync"}));
 expect(await screen.findByText("Syncing…")).toBeTruthy();
 expect(screen.getByRole("button",{name:"Sync"}).hasAttribute("disabled")).toBe(true);
 complete({id:"sync-1",status:"completed",summary:"2 GitHub issues synchronized successfully"});
 expect(await screen.findByText("2 tickets synced.")).toBeTruthy();
 await waitFor(()=>expect(screen.getByRole("button",{name:"Sync"}).hasAttribute("disabled")).toBe(false));
 expect(api.trackerSyncStatus).toHaveBeenCalledWith("sync-1");
});
test("an automatic tracker sync is followed instead of queued a second time",async()=>{
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 const running={id:"auto-sync",trackerId:"t",status:"running"};
 vi.mocked(api.activeTrackerSyncs).mockResolvedValue([running]);
 let complete!:(value:api.TrackerSyncActivity)=>void;
 vi.mocked(api.trackerSyncStatus).mockReturnValue(new Promise(resolve=>{complete=resolve;}));
 render(<LocalSettings onChanged={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Accounts and trackers"}));
 expect(await screen.findByText("Syncing…")).toBeTruthy();
 expect(screen.getByRole("button",{name:"Sync"}).hasAttribute("disabled")).toBe(true);
 expect(api.syncLocalTracker).not.toHaveBeenCalled();
 complete({id:"auto-sync",trackerId:"t",status:"completed",summary:"106 GitLab issues synchronized successfully"});
 expect(await screen.findByText("106 tickets synced.")).toBeTruthy();
 await waitFor(()=>expect(screen.getByRole("button",{name:"Sync"}).hasAttribute("disabled")).toBe(false));
});
test("manual sync checks for an automatic activity that appeared after the last poll",async()=>{
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 vi.mocked(api.activeTrackerSyncs)
  .mockResolvedValueOnce([])
  .mockResolvedValue([{id:"auto-sync",trackerId:"t",status:"queued"}]);
 vi.mocked(api.trackerSyncStatus).mockResolvedValue({id:"auto-sync",status:"running"});
 render(<LocalSettings onChanged={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Accounts and trackers"}));
 await waitFor(()=>expect(api.activeTrackerSyncs).toHaveBeenCalledTimes(1));
 fireEvent.click(screen.getByRole("button",{name:"Sync"}));
 expect(await screen.findByText("Syncing…")).toBeTruthy();
 expect(api.syncLocalTracker).not.toHaveBeenCalled();
 expect(api.trackerSyncStatus).toHaveBeenCalledWith("auto-sync");
});
test("tracker sync reports a backend failure instead of claiming tickets arrived",async()=>{
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 vi.mocked(api.syncLocalTracker).mockResolvedValue({queued:true,activity:{id:"sync-2",status:"queued"}});
 vi.mocked(api.trackerSyncStatus).mockResolvedValue({id:"sync-2",status:"failed",error:"Permission denied"});
 render(<LocalSettings onChanged={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Accounts and trackers"}));
 fireEvent.click(screen.getByRole("button",{name:"Sync"}));
 expect(await screen.findByRole("alert")).toHaveProperty("textContent","Sync failed: Permission denied");
 await waitFor(()=>expect(screen.getByRole("button",{name:"Sync"}).hasAttribute("disabled")).toBe(false));
});
test("the projects section offers the local folder flow without a tracker", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 const open=vi.fn();render(<LocalSettings onChanged={vi.fn()} onOpenFolder={open} />);
 fireEvent.click(await screen.findByRole("button",{name:"Projects"}));
 fireEvent.click(screen.getByRole("button",{name:"Choose a local folder"}));
 expect(open).toHaveBeenCalledOnce();
 expect(api.createLocalProject).not.toHaveBeenCalled();
});
test("an existing folder project can gain a tracker without creating another project", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue(ready);
 vi.mocked(api.attachProjectTracker).mockResolvedValue({});
 const changed = vi.fn();
 render(<LocalSettings onChanged={changed} />);
 fireEvent.click(await screen.findByRole("button", {name:"Projects"}));
 fireEvent.change(screen.getByLabelText("Existing project"), {target:{value:"p"}});
 fireEvent.change(screen.getByLabelText("Tracker to add"), {target:{value:"t"}});
 fireEvent.click(screen.getByRole("button", {name:"Attach tracker"}));
 await waitFor(() => expect(api.attachProjectTracker).toHaveBeenCalledWith("p", "t"));
 await waitFor(() => expect(changed).toHaveBeenCalled());
 expect(api.createLocalProject).not.toHaveBeenCalled();
});


test("Local update preparation does not refresh or relaunch after acknowledgement", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({...ready,mode:"local"});
 let finish!:()=>void;
 vi.mocked(api.prepareLocalUpdate).mockImplementation(()=>new Promise(resolve=>{finish=resolve;}));
 const changed=vi.fn();
 render(<LocalSettings onChanged={changed}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Local services",exact:true}));
 fireEvent.click(screen.getByText("Update application"));
 const stop=screen.getByRole("button",{name:"Stop and quit to update"});
 fireEvent.click(stop);fireEvent.click(stop);
 expect(api.prepareLocalUpdate).toHaveBeenCalledOnce();
 expect(stop.hasAttribute("disabled")).toBe(true);
 finish();
 await screen.findByText("Services stopped. Closing desktop for update…");
 expect(api.localSetupStatus).toHaveBeenCalledOnce();
 expect(api.enableLocalRuntime).not.toHaveBeenCalled();
 expect(changed).not.toHaveBeenCalled();
});

test("Local update refusal leaves explicit retry and service restart available", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({...ready,mode:"local"});
 vi.mocked(api.prepareLocalUpdate).mockRejectedValue("Finish active executions.");
 render(<LocalSettings onChanged={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Local services",exact:true}));
 fireEvent.click(screen.getByText("Update application"));
 const stop=screen.getByRole("button",{name:"Stop and quit to update"});
 fireEvent.click(stop);
 await screen.findByText("Finish active executions.");
 expect(stop.hasAttribute("disabled")).toBe(false);
 expect(screen.getByRole("button",{name:"Check / start local companion"}).hasAttribute("disabled")).toBe(false);
 expect(api.localSetupStatus).toHaveBeenCalledOnce();
});

test("Fleet does not expose the Local update stop action", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({...ready,mode:"fleet",fleetServer:"https://fleet.example.invalid"});
 render(<LocalSettings onChanged={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Connection and services",exact:true}));
 expect(screen.queryByRole("button",{name:"Stop and quit to update",hidden:true})).toBeNull();
});


test("reopening Settings during update preparation retains retry and explicit restart", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({enabled:true,mode:"local",ready:false,agentReady:false,updatePreparing:true});
 render(<LocalSettings onChanged={vi.fn()}/>);
 await screen.findByRole("button",{name:"Check / start local companion"});
 fireEvent.click(screen.getByText("Update application"));
 expect(screen.getByRole("button",{name:"Stop and quit to update"})).toBeTruthy();
 expect(api.enableLocalRuntime).not.toHaveBeenCalled();
});

test("connections section provides subtabs for Trackers and PAT access and defaults GitHub source to API URL", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({ ...ready, mode: "local" });
 render(<LocalSettings onChanged={vi.fn()} />);
 fireEvent.click(await screen.findByRole("button", { name: "Accounts and trackers" }));

 const trackersTab = screen.getByRole("tab", { name: "Trackers" });
 const patTab = screen.getByRole("tab", { name: "PAT access" });
 expect(trackersTab.getAttribute("aria-selected")).toBe("true");
 expect(patTab.getAttribute("aria-selected")).toBe("false");

 const sourceSiteInput = screen.getByLabelText("Source URL") as HTMLInputElement;
 expect(sourceSiteInput.value).toBe("https://api.github.com");

 fireEvent.change(screen.getByLabelText("Source provider"), { target: { value: "gitlab" } });
 expect(sourceSiteInput.value).toBe("https://gitlab.com");

 fireEvent.change(screen.getByLabelText("Source provider"), { target: { value: "github" } });
 expect(sourceSiteInput.value).toBe("https://api.github.com");

 fireEvent.click(patTab);
 expect(trackersTab.getAttribute("aria-selected")).toBe("false");
 expect(patTab.getAttribute("aria-selected")).toBe("true");
});

test("appearance settings allow configuring color theme and font size scale", async () => {
 vi.mocked(api.localSetupStatus).mockResolvedValue({ ...ready, mode: "local" });
 render(<LocalSettings onChanged={vi.fn()} />);
 fireEvent.click(await screen.findByRole("button", { name: "Appearance" }));

 const themeSelect = screen.getByLabelText("Color theme") as HTMLSelectElement;
 const fontSizeSelect = screen.getByLabelText("Font size") as HTMLSelectElement;
 expect(fontSizeSelect.value).toBe("default");

 fireEvent.change(themeSelect, { target: { value: "light" } });
 expect(document.documentElement.dataset.theme).toBe("light");

 fireEvent.change(fontSizeSelect, { target: { value: "large" } });
 expect(document.documentElement.dataset.fontSize).toBe("large");
});
