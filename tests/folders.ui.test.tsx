import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {PromptFolders,AttachedFolderSettings} from "../src/components/FolderContext";
import * as api from "../src/lib/folders";
vi.mock("../src/lib/folders",()=>({projectFolders:vi.fn(),attachFolder:vi.fn(),detachFolder:vi.fn()}));
vi.mock("../src/lib/workspace",()=>({projectContext:vi.fn(async()=>({policy:{},skills:[]})),projectRepositories:vi.fn(async()=>[])}));
beforeEach(()=>{vi.resetAllMocks();vi.mocked(api.projectFolders).mockResolvedValue([]);vi.mocked(api.attachFolder).mockResolvedValue(null);});afterEach(cleanup);
test("attachments added in a conversation are project-wide and apply at the next message",async()=>{
 vi.mocked(api.attachFolder).mockResolvedValue({path:"/notes",appliesAt:"next-turn"});
 vi.mocked(api.projectFolders).mockResolvedValueOnce([]).mockResolvedValue([{path:"/notes",kind:"folder"}]);
 render(<PromptFolders projectId="project" runId="old-chat" directory="/old-working-directory"/>);
 expect(screen.getByText(/Dossiers du projet ·/)).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"Attacher au projet"}));
 await screen.findByText("Dossier ajouté au projet · disponible dès le prochain message.");
 expect(api.attachFolder).toHaveBeenCalledWith("project","old-chat");
 expect(screen.getByTitle("/old-working-directory")).toBeTruthy();
 fireEvent.click(screen.getByText(/Dossiers du projet ·/));
 expect(screen.getByText("/notes")).toBeTruthy();
 expect(screen.getByText("Les dossiers attachés sont partagés par toutes les conversations de ce projet. Leur retrait se fait dans sa configuration.")).toBeTruthy();
 expect(screen.queryByRole("button",{name:/Détacher/})).toBeNull();
});
test("new prompt attachments do not create a run, and cancellation leaves context unchanged",async()=>{
 render(<PromptFolders projectId="project" directory="/project"/>);
 fireEvent.click(screen.getByRole("button",{name:"Attacher au projet"}));
 await waitFor(()=>expect(screen.getByRole("button",{name:"Attacher au projet"}).hasAttribute("disabled")).toBe(false));
 expect(api.attachFolder).toHaveBeenCalledWith("project",undefined);
 expect(screen.queryByRole("status")).toBeNull();
 expect(screen.getByTitle("/project")).toBeTruthy();
});
test("read-only histories cannot attach and project configuration detaches without deleting files",async()=>{
 const view=render(<PromptFolders projectId="project" runId="history" directory="/old" readOnly/>);
 expect(screen.getByRole("button",{name:"Attacher au projet"}).hasAttribute("disabled")).toBe(true);
 view.unmount();vi.mocked(api.projectFolders).mockResolvedValueOnce([{path:"/notes",kind:"missing"}]).mockResolvedValue([]);vi.mocked(api.detachFolder).mockResolvedValue({});
 render(<AttachedFolderSettings projectId="project"/>);
 expect(await screen.findByText("Dossier introuvable")).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"Détacher /notes"}));
 await waitFor(()=>expect(api.detachFolder).toHaveBeenCalledWith("project","/notes"));
});
