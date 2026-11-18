import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import ProjectPrompt from "../src/components/ProjectPrompt";
import { getComposer, press, typeIn, valueOf } from "./composer-editor";
import { conversationFor } from "../src/lib/conversation-controller";
import * as folders from "../src/lib/folders";
import * as chat from "../src/lib/conversation";
vi.mock("../src/lib/folders",()=>({projectFolders:vi.fn(),attachFolder:vi.fn()}));
vi.mock("../src/lib/workspace",()=>({projectContext:vi.fn(async()=>({policy:{},skills:[]})),projectRepositories:vi.fn(async()=>[])}));
vi.mock("../src/lib/profiles",()=>({conversationProfiles:vi.fn(async()=>({profiles:[],defaultId:"",inherited:true}))}));
vi.mock("../src/lib/conversation",async original=>({...await original<typeof chat>(),providerModels:vi.fn(),createConversation:vi.fn(),readConversation:vi.fn(),controlConversation:vi.fn()}));
beforeEach(()=>{vi.resetAllMocks();vi.mocked(folders.projectFolders).mockResolvedValue([]);vi.mocked(chat.providerModels).mockResolvedValue([]);vi.mocked(chat.createConversation).mockResolvedValue({id:"prompt-after-folder-confirmed"});vi.mocked(chat.readConversation).mockResolvedValue({id:"prompt-after-folder-confirmed",busy:false,readOnly:false});vi.mocked(chat.controlConversation).mockResolvedValue({});});afterEach(cleanup);
test("a first prompt waits until native attachment is confirmed",async()=>{
 let resolve!:(value:folders.FolderAttachment|null)=>void;
 vi.mocked(folders.attachFolder).mockReturnValue(new Promise(done=>resolve=done));
 render(<ProjectPrompt projectId="project" directory="/working" onCreated={()=>{}}/>);
 typeIn(getComposer("Free prompt"),"Use the added context");
 fireEvent.click(screen.getByRole("button",{name:/^(Add folder|Add a folder|Ajouter un dossier)$/i}));
 expect(screen.getByRole("button",{name:"Send"}).hasAttribute("disabled")).toBe(true);
 fireEvent.submit(getComposer("Free prompt").closest("form")!);
 expect(chat.createConversation).not.toHaveBeenCalled();
 resolve({path:"/context",appliesAt:"next-launch"});
 await waitFor(()=>expect(screen.getByRole("button",{name:"Send"}).hasAttribute("disabled")).toBe(false));
 fireEvent.click(screen.getByRole("button",{name:"Send"}));
 await waitFor(()=>expect(chat.createConversation).toHaveBeenCalledWith("project","claude"));
});

test.each([false, true])("a conversation preserves its draft until attachment is confirmed (busy=%s)", async busy => {
  const id = `attachment-conversation-${busy}`;
  vi.mocked(chat.readConversation).mockResolvedValue({ id, busy, readOnly: false });
  let resolve!: (value: folders.FolderAttachment | null) => void;
  vi.mocked(folders.attachFolder).mockReturnValue(new Promise(done => { resolve = done; }));
  render(<ProjectPrompt projectId="project" runId={id} directory="/working" onCreated={() => {}} />);
  await waitFor(() => expect(conversationFor(id).getSnapshot().loaded).toBe(true));
  const box = getComposer();
  typeIn(box, "Use the new folder");
  fireEvent.click(screen.getByText("Folders ·").closest("summary")!);
  fireEvent.click(screen.getByRole("button", { name: "Add folder" }));
  const button = screen.getByRole("button", { name: "Send" });
  expect(button.hasAttribute("disabled")).toBe(true);
  fireEvent.click(button);
  press(box, "Enter");
  expect(valueOf(box)).toBe("Use the new folder");
  expect(conversationFor(id).getSnapshot().queue.items).toEqual([]);
  expect(chat.controlConversation).not.toHaveBeenCalled();
  resolve({ path: "/context", appliesAt: "next-turn" });
  await waitFor(() => expect(button.hasAttribute("disabled")).toBe(false));
  fireEvent.click(button);
  expect(valueOf(box)).toBe("");
  if (busy) {
    expect(conversationFor(id).getSnapshot().queue.items.map(item => item.text)).toEqual(["Use the new folder"]);
    expect(chat.controlConversation).not.toHaveBeenCalled();
  } else {
    expect(chat.controlConversation).toHaveBeenCalledWith(id, { action: "message", message: "Use the new folder" });
  }
});
