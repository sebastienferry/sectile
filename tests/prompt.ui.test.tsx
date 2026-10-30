import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import ProjectPrompt from "../src/components/ProjectPrompt";
import * as api from "../src/lib/conversation";
vi.mock("../src/lib/conversation", async original => ({
  ...await original<typeof api>(), createConversation: vi.fn(), readConversation: vi.fn(), controlConversation: vi.fn(),
}));
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.controlConversation).mockResolvedValue({ accepted: true });
});
afterEach(() => cleanup());
const snapshot = (id: string, busy = false) => ({ id, busy, readOnly: false, events: [JSON.stringify({ kind: "assistant", text: "Repository explanation" })] });
test("a free prompt creates a project-scoped Go conversation and crosses IPC once", async () => {
  vi.mocked(api.createConversation).mockResolvedValue({ id: "new-project-chat" });
  vi.mocked(api.readConversation).mockResolvedValue(snapshot("new-project-chat"));
  const created = vi.fn();
  render(<ProjectPrompt projectId="project-one" onCreated={created} />);
  fireEvent.change(screen.getByRole("textbox", { name: "Prompt libre" }), { target: { value: "Explain the project" } });
  fireEvent.click(screen.getByRole("button", { name: "Envoyer" }));
  await waitFor(() => expect(created).toHaveBeenCalledWith("new-project-chat"));
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("new-project-chat", { action: "message", message: "Explain the project" }));
  expect(api.createConversation).toHaveBeenCalledWith("project-one", "claude");
  expect(api.controlConversation).toHaveBeenCalledTimes(1);
});
test("switching projects clears the draft and does not reopen an older project after creation",async()=>{
 let resolveCreate!:(value:{id:string})=>void;
 vi.mocked(api.createConversation).mockReturnValueOnce(new Promise(resolve=>{resolveCreate=resolve;})).mockResolvedValue({id:"new-project-chat"});
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("new-project-chat"));
 const opened=vi.fn();
 const view=render(<ProjectPrompt key="project-one" projectId="project-one" onCreated={opened}/>);
 fireEvent.change(screen.getByRole("textbox",{name:"Prompt libre"}),{target:{value:"Prompt for project one"}});
 fireEvent.click(screen.getByRole("button",{name:"Envoyer"}));
 view.rerender(<ProjectPrompt key="project-two" projectId="project-two" onCreated={opened}/>);
 expect((screen.getByRole("textbox",{name:"Prompt libre"}) as HTMLTextAreaElement).value).toBe("");
 resolveCreate({id:"old-project-chat"});
 await waitFor(()=>expect(api.controlConversation).toHaveBeenCalledWith("old-project-chat",{action:"message",message:"Prompt for project one"}));
 expect(opened).not.toHaveBeenCalled();
 fireEvent.change(screen.getByRole("textbox",{name:"Prompt libre"}),{target:{value:"Prompt for project two"}});
 fireEvent.click(screen.getByRole("button",{name:"Envoyer"}));
 await waitFor(()=>expect(opened).toHaveBeenCalledWith("new-project-chat"));
 expect(api.createConversation).toHaveBeenNthCalledWith(2,"project-two","claude");
});
test("the React controller renders Go state, sends only the next message and restores read-only history", async () => {
  vi.mocked(api.readConversation).mockResolvedValue(snapshot("history-chat"));
  render(<ProjectPrompt projectId="project-two" runId="history-chat" onCreated={vi.fn()} />);
  await screen.findByText("Repository explanation");
  expect(api.controlConversation).not.toHaveBeenCalled();
  fireEvent.change(screen.getByRole("textbox", { name: "Message suivant" }), { target: { value: "Continue" } });
  await waitFor(() => expect(screen.getByRole("button", { name: "Envoyer" }).hasAttribute("disabled")).toBe(false));
  fireEvent.click(screen.getByRole("button", { name: "Envoyer" }));
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("history-chat", { action: "message", message: "Continue" }));
  vi.mocked(api.readConversation).mockResolvedValue({ ...snapshot("history-chat"), readOnly: true });
  await waitFor(() => expect(screen.getByRole("button", { name: "Actualiser le suivi" }).hasAttribute("disabled")).toBe(false));
  fireEvent.click(screen.getByRole("button", { name: "Actualiser le suivi" }));
  await screen.findByText("Lecture seule");
  expect(screen.getByRole("textbox", { name: "Message suivant" }).hasAttribute("disabled")).toBe(true);
});
test("a free project conversation keeps only relevant status above the composer",async()=>{
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("free-chat"));
 render(<ProjectPrompt projectId="p" runId="free-chat" run={{id:"free-chat",projectId:"p",conversation:true,status:"running",createdAt:"2026-10-30"}} onCreated={vi.fn()}/>);
 await screen.findByText("Repository explanation");
 const strip=screen.getByLabelText("Suivi de la conversation");
 expect(strip.textContent).toContain("Prêt à répondre");
 expect(strip.textContent).toContain("Exécution : Session active");
 expect(strip.textContent).not.toContain("Skill :");
 expect(strip.textContent).not.toContain("Ticket :");
 expect(strip.textContent).not.toContain("Étape :");
});
test("a linked conversation shows ticket and execution state above the composer",async()=>{
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("linked-chat"));
 const openExecution=vi.fn();
 const openQueue=vi.fn();
 render(<ProjectPrompt projectId="p" runId="linked-chat" run={{id:"linked-chat",projectId:"p",conversation:true,taskId:"task",taskKey:"APP-42",skill:"clarify",status:"running",createdAt:"2026-10-30"}} task={{id:"task",key:"APP-42",title:"Linked task",source:"jira",trackerStatus:"In Progress",status:"to_close",prUrl:"https://github.com/team/app/pull/42",prLinks:[{url:"https://github.com/team/app/pull/42",state:"open"}]}} onOpenExecution={openExecution} onOpenQueue={openQueue} onCreated={vi.fn()}/>);
 await screen.findByText("Repository explanation");
 const strip=screen.getByLabelText("Suivi de la conversation");
 expect(strip.textContent).toContain("Skill : Clarifier");
 expect(strip.textContent).toContain("APP-42 · In Progress");
 expect(strip.textContent).toContain("Étape : Revu");
 expect(strip.textContent).toContain("PR/MR : PR #42 · ouverte");
 expect(screen.getByRole("button",{name:"PR #42 · ouverte"})).toBeTruthy();
 fireEvent.click(screen.getByLabelText("Actions de la discussion"));
 expect(screen.getByRole("button",{name:"Voir l’exécution"})).toBeTruthy();
 expect(screen.getByRole("button",{name:"Arrêter la discussion"})).toBeTruthy();
 fireEvent.click(screen.getByText("Exécution : Session active"));
 fireEvent.click(screen.getByRole("button",{name:"Voir cette exécution"}));
 expect(openExecution).toHaveBeenCalledOnce();
 fireEvent.click(screen.getByRole("button",{name:"Voir la file des exécutions"}));
 expect(openQueue).toHaveBeenCalledOnce();
});
test("a ticket conversation lists every linked pull request with its own state",async()=>{
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("multi-pr-chat"));
 render(<ProjectPrompt projectId="p" runId="multi-pr-chat" run={{id:"multi-pr-chat",projectId:"p",conversation:true,taskId:"task",taskKey:"APP-42",skill:"implement",status:"completed",createdAt:"2026-10-30"}} task={{id:"task",key:"APP-42",title:"Linked task",source:"gitlab",status:"to_review",prLinks:[{url:"https://github.com/team/app/pull/42",state:"merged"},{url:"https://gitlab.example.com/team/app/-/merge_requests/7",state:"open"}]}} onCreated={vi.fn()}/>);
 await screen.findByText("Repository explanation");
 fireEvent.click(screen.getByText("PR/MR : 2 liées"));
 expect(screen.getByRole("button",{name:"PR #42 · fusionnée"})).toBeTruthy();
 expect(screen.getByRole("button",{name:"MR !7 · ouverte"})).toBeTruthy();
});
test("a finished discussion archives through its own menu without sending a provider turn",async()=>{
 vi.mocked(api.readConversation).mockResolvedValue({...snapshot("finished-chat"),readOnly:true,canRename:true});
 const archive=vi.fn();
 render(<ProjectPrompt projectId="p" runId="finished-chat" run={{id:"finished-chat",projectId:"p",taskId:"",skill:"",status:"completed",conversation:true,createdAt:"2026-10-30"}} onArchive={archive} onCreated={vi.fn()}/>);
 await screen.findByText("Repository explanation");
 fireEvent.click(screen.getByLabelText("Actions de la discussion"));
 fireEvent.click(screen.getByRole("button",{name:"Archiver la discussion"}));
 expect(archive).toHaveBeenCalledOnce();
 expect(api.controlConversation).not.toHaveBeenCalled();
});
test("pending tools require an explicit decision; navigation does not interrupt Go", async () => {
  vi.mocked(api.readConversation).mockResolvedValue({ ...snapshot("approval-chat"), approvals: [{ id: "tool-1", tool: "Bash", input: { command: "git status" } }] });
  const view = render(<ProjectPrompt projectId="project-three" runId="approval-chat" onCreated={vi.fn()} />);
  await screen.findByText("Autorisation requise · Bash");
  expect(api.controlConversation).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Autoriser une fois" }));
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("approval-chat", { action: "approval", id: "tool-1", decision: "allow" }));
  view.unmount();
  expect(api.controlConversation).not.toHaveBeenCalledWith(expect.anything(), { action: "interrupt" });
});
test("a failed read does not retry a prompt and recovery only observes", async () => {
  vi.mocked(api.readConversation).mockRejectedValueOnce(new Error("Disconnected")).mockResolvedValue(snapshot("recovery-chat"));
  render(<ProjectPrompt projectId="project-four" runId="recovery-chat" onCreated={vi.fn()} />);
  await screen.findByRole("alert");
  expect(screen.getByRole("button", { name: "Envoyer" }).hasAttribute("disabled")).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "Actualiser le suivi" }));
  await screen.findByText("Repository explanation");
  expect(api.controlConversation).not.toHaveBeenCalled();
});
test("streamed state exposes interruption and the next turn waits for Go to become idle", async () => {
  vi.mocked(api.readConversation).mockResolvedValue({ ...snapshot("live-chat", true), partial: "Working draft" });
  render(<ProjectPrompt projectId="project-five" runId="live-chat" onCreated={vi.fn()} />);
  await screen.findByText("Working draft");
  expect(screen.getByRole("button", { name: "Envoyer" }).hasAttribute("disabled")).toBe(true);
  vi.mocked(api.readConversation).mockResolvedValue(snapshot("live-chat"));
  fireEvent.click(screen.getByRole("button", { name: "Interrompre" }));
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("live-chat", { action: "interrupt" }));
  await screen.findByText("Prêt");
  expect(screen.queryByText("Working draft")).toBeNull();
});
test("a lost send acknowledgement is never replayed and observation recovers", async () => {
  const { ConversationController } = await import("../src/lib/conversation-controller");
  const controller = new ConversationController("lost-ack");
  vi.mocked(api.controlConversation).mockRejectedValueOnce(new Error("Acknowledgement lost"));
  await controller.send("Only once", true);
  expect(controller.getSnapshot().error).toContain("Acknowledgement lost");
  expect(api.controlConversation).toHaveBeenCalledTimes(1);
  vi.mocked(api.readConversation).mockResolvedValue(snapshot("lost-ack"));
  await controller.refresh();
  expect(controller.getSnapshot().error).toBe("");
  expect(api.controlConversation).toHaveBeenCalledTimes(1);
});
test("concurrent sends are admitted once and a pre-mutation read cannot restore stale state", async () => {
  const { ConversationController } = await import("../src/lib/conversation-controller");
  const controller = new ConversationController("read-race");
  let resolveRead!: (value: api.ConversationSnapshot) => void;
  let resolveSend!: (value: unknown) => void;
  vi.mocked(api.readConversation).mockReturnValueOnce(new Promise(resolve => { resolveRead = resolve; }));
  vi.mocked(api.controlConversation).mockReturnValueOnce(new Promise(resolve => { resolveSend = resolve; }));
  const read = controller.refresh();
  const send = controller.send("Send once", true);
  expect(await controller.send("Duplicate", true)).toBe(false);
  resolveRead({ ...snapshot("read-race"), readOnly: true });
  await read;
  expect(controller.getSnapshot().loaded).toBe(false);
  vi.mocked(api.readConversation).mockResolvedValue(snapshot("read-race"));
  resolveSend({ accepted: true });
  await send;
  await waitFor(() => expect(controller.getSnapshot().loaded).toBe(true));
  expect(controller.getSnapshot().snapshot.readOnly).toBe(false);
  expect(api.controlConversation).toHaveBeenCalledTimes(1);
});
test("a failed read during execution leaves observation recovery available", async () => {
  vi.mocked(api.readConversation).mockResolvedValueOnce({ ...snapshot("busy-error", true), partial: "Work in progress" }).mockRejectedValueOnce(new Error("Offline")).mockResolvedValue(snapshot("busy-error"));
  render(<ProjectPrompt projectId="project-six" runId="busy-error" onCreated={vi.fn()} />);
  await screen.findByText("Work in progress");
  await screen.findByRole("alert");
  expect(screen.getByRole("button", { name: "Actualiser le suivi" }).hasAttribute("disabled")).toBe(false);
  fireEvent.click(screen.getByRole("button", { name: "Actualiser le suivi" }));
  await screen.findByText("Prêt");
  expect(api.controlConversation).not.toHaveBeenCalled();
});
test("interactive answers cross IPC as answers, not as an ordinary prompt or a tool grant", async () => {
 vi.mocked(api.readConversation).mockResolvedValue({...snapshot("question-wire"),busy:true,approvals:[{id:"q-wire",tool:"AskUserQuestion",input:{questions:[{question:"Which scope?",options:[{label:"Small"}]}]}}]});
 render(<ProjectPrompt projectId="question-project" runId="question-wire" onCreated={vi.fn()} />);
 await screen.findByRole("radio",{name:"Small"});
 fireEvent.click(screen.getByRole("radio",{name:"Small"}));
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("question-wire"));
 fireEvent.click(screen.getByRole("button",{name:"Envoyer les réponses"}));
 await waitFor(()=>expect(api.controlConversation).toHaveBeenCalledWith("question-wire",{action:"answer",id:"q-wire",answers:{"Which scope?":"Small"}}));
 await waitFor(()=>expect(screen.queryByRole("button",{name:"Envoyer les réponses"})).toBeNull());
 expect(api.controlConversation).toHaveBeenCalledTimes(1);
});

test("local journal failure is visible without replaying a prompt", async () => {
 vi.mocked(api.readConversation).mockResolvedValue({...snapshot("journal-failure"),persistenceError:"La sauvegarde locale est indisponible."});
 render(<ProjectPrompt projectId="journal-project" runId="journal-failure" onCreated={vi.fn()} />);
 expect(await screen.findByRole("alert")).toHaveProperty("textContent","La sauvegarde locale est indisponible.");
 expect(api.controlConversation).not.toHaveBeenCalled();
});

test("Enter sends a prompt once while Shift Enter keeps composing", async () => {
 vi.mocked(api.createConversation).mockResolvedValue({id:"keyboard-prompt"});
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("keyboard-prompt"));
 render(<ProjectPrompt projectId="keyboard-project" onCreated={vi.fn()} />);
 const input=screen.getByRole("textbox",{name:"Prompt libre"});
 fireEvent.change(input,{target:{value:"Explain the code"}});
 fireEvent.keyDown(input,{key:"Enter",shiftKey:true});
 expect(api.createConversation).not.toHaveBeenCalled();
 fireEvent.keyDown(input,{key:"Enter"});
 await waitFor(()=>expect(api.controlConversation).toHaveBeenCalledWith("keyboard-prompt",{action:"message",message:"Explain the code"}));
 expect(api.createConversation).toHaveBeenCalledOnce();
});
test("prompt suggestions fill the composer without starting execution", () => {
 render(<ProjectPrompt projectId="suggestion-project" onCreated={vi.fn()} />);
 fireEvent.click(screen.getByRole("button",{name:"Préparer un plan"}));
 expect((screen.getByRole("textbox",{name:"Prompt libre"}) as HTMLTextAreaElement).value).toContain("sans modifier de fichiers");
 expect((screen.getByLabelText("Mode de conversation") as HTMLSelectElement).value).toBe("plan");
 expect(api.createConversation).not.toHaveBeenCalled();
});
