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
