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
