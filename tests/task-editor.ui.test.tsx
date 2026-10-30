import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { TaskEditor } from "../src/components/TaskEditor";
import * as api from "../src/lib/agent";

vi.mock("../src/lib/agent", async original => ({ ...await original<typeof api>(), taskTrackers: vi.fn(), taskIssueTypes: vi.fn(), getTask: vi.fn(), createTask: vi.fn(), updateTask: vi.fn() }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });

test("creating a ticket sends its selected tracker and fields to Go once", async () => {
  vi.mocked(api.taskTrackers).mockResolvedValue({defaultTrackerId:"be",trackers:[{id:"be",provider:"gitlab",name:"Backend"},{id:"fe",provider:"github",name:"Frontend"}]});
  vi.mocked(api.taskIssueTypes).mockResolvedValue({supported:true,issueTypes:["Bug", "Task"]});
  vi.mocked(api.createTask).mockResolvedValue({id:"remote-1",key:"FE-1",title:"Fix layout",status:"to_clarify"});
  const saved=vi.fn();
  render(<TaskEditor projectId="p" projectName="Project" available onClose={vi.fn()} onSaved={saved}/>);
  await screen.findByRole("option",{name:"Frontend"});
  fireEvent.change(screen.getByLabelText("Tracker"),{target:{value:"fe"}});
  await screen.findByRole("option",{name:"Bug"});
  fireEvent.change(screen.getByLabelText("Type"),{target:{value:"Bug"}});
  fireEvent.change(screen.getByLabelText("Priorité"),{target:{value:"high"}});
  fireEvent.change(screen.getByLabelText("Titre"),{target:{value:" Fix layout "}});
  fireEvent.change(screen.getByLabelText(/Description/),{target:{value:"Expected behavior"}});
  fireEvent.click(screen.getByRole("button",{name:"Créer le ticket"}));
  await waitFor(()=>expect(api.createTask).toHaveBeenCalledExactlyOnceWith("p",{title:"Fix layout",description:"Expected behavior",priority:"high",issueType:"Bug",trackerId:"fe"}));
  expect(saved).toHaveBeenCalledWith(expect.objectContaining({id:"remote-1",title:"Fix layout"}));
});

test("editing a ticket reloads server state and sends only changed fields", async () => {
  vi.mocked(api.taskTrackers).mockResolvedValue({defaultTrackerId:"be",trackers:[{id:"be",provider:"gitlab",name:"Backend"}]});
  vi.mocked(api.taskIssueTypes).mockResolvedValue({supported:true,issueTypes:["Task"]});
  vi.mocked(api.getTask).mockResolvedValue({id:"t",key:"BE-2",title:"Original",description:"Keep",priority:"medium",issueType:"Task",trackerId:"be",status:"to_clarify"});
  vi.mocked(api.updateTask).mockResolvedValue({id:"t",title:"Changed",status:"to_clarify"});
  const saved=vi.fn();
  render(<TaskEditor projectId="p" projectName="Project" taskId="t" available onClose={vi.fn()} onSaved={saved}/>);
  await screen.findByDisplayValue("Original");
  expect((screen.getByRole("button",{name:"Enregistrer"}) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.change(screen.getByLabelText("Titre"),{target:{value:"Changed"}});
  fireEvent.click(screen.getByRole("button",{name:"Enregistrer"}));
  await waitFor(()=>expect(api.updateTask).toHaveBeenCalledExactlyOnceWith("p","t",{title:"Changed"}));
  await waitFor(()=>expect(api.getTask).toHaveBeenCalledTimes(2));
  expect(saved).toHaveBeenCalledWith(expect.objectContaining({id:"t"}));
});

test("a project without a remote tracker cannot create a local ticket copy", async () => {
  vi.mocked(api.taskTrackers).mockResolvedValue({trackers:[]});
  render(<TaskEditor projectId="p" projectName="Local" available onClose={vi.fn()} onSaved={vi.fn()}/>);
  await screen.findByText(/n’a pas de tracker connecté/);
  fireEvent.change(screen.getByLabelText("Titre"),{target:{value:"Untracked"}});
  expect((screen.getByRole("button",{name:"Créer le ticket"}) as HTMLButtonElement).disabled).toBe(true);
  expect(api.createTask).not.toHaveBeenCalled();
});

test("editing PR/MR links keeps existing metadata and sends their new order", async () => {
  vi.mocked(api.taskTrackers).mockResolvedValue({defaultTrackerId:"be",trackers:[{id:"be",provider:"gitlab",name:"Backend"}]});
  vi.mocked(api.taskIssueTypes).mockResolvedValue({supported:true,issueTypes:[]});
  const loaded = {id:"t",key:"BE-2",title:"Review",priority:"medium",trackerId:"be",status:"reviewed",prLinks:[{url:"https://gitlab.example/g/p/-/merge_requests/1",state:"merged",branch:"feat/old"},{url:"https://gitlab.example/g/p/-/merge_requests/2",state:"open"}]};
  vi.mocked(api.getTask).mockResolvedValue(loaded);
  vi.mocked(api.updateTask).mockResolvedValue(loaded);
  render(<TaskEditor projectId="p" projectName="Project" taskId="t" available onClose={vi.fn()} onSaved={vi.fn()}/>);
  await screen.findByDisplayValue("Review");
  fireEvent.click(screen.getByText("Liens PR/MR · 2"));
  fireEvent.click(screen.getByRole("button",{name:"Descendre https://gitlab.example/g/p/-/merge_requests/1"}));
  fireEvent.click(screen.getByRole("button",{name:"Enregistrer"}));
  await waitFor(() => expect(api.updateTask).toHaveBeenCalledExactlyOnceWith("p","t",{prLinks:[loaded.prLinks[1],loaded.prLinks[0]]}));
});

test("invalid and duplicate PR/MR links are rejected without changing the ticket", async () => {
  vi.mocked(api.taskTrackers).mockResolvedValue({defaultTrackerId:"be",trackers:[{id:"be",provider:"gitlab",name:"Backend"}]});
  vi.mocked(api.taskIssueTypes).mockResolvedValue({supported:true,issueTypes:[]});
  vi.mocked(api.getTask).mockResolvedValue({id:"t",title:"Review",priority:"medium",trackerId:"be",status:"reviewed",prLinks:[{url:"https://gitlab.example/g/p/-/merge_requests/1"}]});
  render(<TaskEditor projectId="p" projectName="Project" taskId="t" available onClose={vi.fn()} onSaved={vi.fn()}/>);
  await screen.findByDisplayValue("Review");
  fireEvent.click(screen.getByText("Liens PR/MR · 1"));
  fireEvent.change(screen.getByRole("textbox",{name:"URL PR/MR"}),{target:{value:"https://token@gitlab.example/g/p/-/merge_requests/2"}});
  fireEvent.click(screen.getByRole("button",{name:"Ajouter un lien"}));
  expect(screen.getByRole("alert").textContent).toMatch(/invalide/);
  expect(api.updateTask).not.toHaveBeenCalled();
});
