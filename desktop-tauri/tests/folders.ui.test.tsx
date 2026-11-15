import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import { invoke } from "@tauri-apps/api/core";
import {PromptFolders,AttachedFolderSettings} from "../src/components/FolderContext";
import * as api from "../src/lib/folders";
vi.mock("@tauri-apps/api/core",()=>({invoke:vi.fn()}));
vi.mock("../src/lib/folders",()=>({projectFolders:vi.fn(),attachFolder:vi.fn(),detachFolder:vi.fn()}));
vi.mock("../src/lib/workspace",()=>({projectContext:vi.fn(async()=>({policy:{},skills:[]})),projectRepositories:vi.fn(async()=>[])}));
beforeEach(()=>{vi.resetAllMocks();vi.mocked(api.projectFolders).mockResolvedValue([]);vi.mocked(api.attachFolder).mockResolvedValue(null);vi.mocked(invoke).mockResolvedValue(null);});afterEach(cleanup);
test("attachments added in a conversation are project-wide and apply at the next message",async()=>{
 vi.mocked(api.attachFolder).mockResolvedValue({path:"/notes",appliesAt:"next-turn"});
 vi.mocked(api.projectFolders).mockResolvedValueOnce([]).mockResolvedValue([{path:"/notes",kind:"folder"}]);
 render(<PromptFolders projectId="project" runId="old-chat" directory="/old-working-directory"/>);
 expect(screen.getByText(/Folders ·/)).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"Add folder"}));
 await screen.findByText("Folder added to project · available on next message.");
 expect(api.attachFolder).toHaveBeenCalledWith("project","old-chat");
 expect(screen.getByTitle("/old-working-directory")).toBeTruthy();
 fireEvent.click(screen.getByText(/Folders ·/));
 expect(screen.getByText("/notes")).toBeTruthy();
 expect(screen.getByText("Attached folders are shared across all conversations in this project. Remove them in project settings.")).toBeTruthy();
 expect(screen.queryByRole("button",{name:/Detach/})).toBeNull();
});
test("new prompt attachments do not create a run, and cancellation leaves context unchanged",async()=>{
 render(<PromptFolders projectId="project" directory="/project"/>);
 fireEvent.click(screen.getByRole("button",{name:"Add folder"}));
 await waitFor(()=>expect(screen.getByRole("button",{name:"Add folder"}).hasAttribute("disabled")).toBe(false));
 expect(api.attachFolder).toHaveBeenCalledWith("project",undefined);
 expect(screen.queryByRole("status")).toBeNull();
 expect(screen.getByTitle("/project")).toBeTruthy();
});
test("read-only histories cannot attach and project configuration detaches without deleting files",async()=>{
 const view=render(<PromptFolders projectId="project" runId="history" directory="/old" readOnly/>);
 expect(screen.queryByRole("button",{name:"Add folder"})).toBeNull();
 expect(screen.getAllByText("/old").length).toBeGreaterThan(0);
 view.unmount();vi.mocked(api.projectFolders).mockResolvedValueOnce([{path:"/notes",kind:"missing"}]).mockResolvedValue([]);vi.mocked(api.detachFolder).mockResolvedValue({});
 render(<AttachedFolderSettings projectId="project"/>);
 expect(await screen.findByText("Folder not found")).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"Detach /notes"}));
 await waitFor(()=>expect(api.detachFolder).toHaveBeenCalledWith("project","/notes"));
});
test("clicking a folder in the menu copies its path and clicking the Finder button opens it",async()=>{
 const writeText=vi.fn().mockResolvedValue(undefined);
 Object.defineProperty(navigator,"clipboard",{value:{writeText},configurable:true});
 render(<PromptFolders compact projectId="project" directory="/workspace/sectile"/>);
 fireEvent.click(screen.getByRole("button",{name:"Copy path /workspace/sectile"}));
 await waitFor(()=>expect(writeText).toHaveBeenCalledWith("/workspace/sectile"));
 fireEvent.click(screen.getByRole("button",{name:"Open /workspace/sectile in Finder"}));
 await waitFor(()=>expect(invoke).toHaveBeenCalledWith("open_folder_at",{path:"/workspace/sectile"}));
});
test("compact composer folder picker updates the folder count directly without rendering inline status text",async()=>{
 vi.mocked(api.attachFolder).mockResolvedValue({path:"/extra",appliesAt:"next-launch"});
 vi.mocked(api.projectFolders).mockResolvedValueOnce([]).mockResolvedValue([{path:"/extra",kind:"folder"}]);
 render(<PromptFolders compact projectId="project" directory="/workspace/sectile"/>);
 fireEvent.click(screen.getByRole("button",{name:"Add folder"}));
 await screen.findByText("/extra");
 expect(screen.queryByRole("status")).toBeNull();
});
