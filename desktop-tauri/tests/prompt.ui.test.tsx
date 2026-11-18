import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { conversationFor } from "../src/lib/conversation-controller";
import userEvent from "@testing-library/user-event";
import { stopRun } from "../src/lib/agent";
import type { ProjectPromptProps } from "../src/components/ProjectPrompt";
import ProjectPrompt from "../src/components/ProjectPrompt";
import { getComposer, press, queryComposer, typeIn, valueOf } from "./composer-editor";
import * as api from "../src/lib/conversation";
vi.mock("../src/lib/conversation", async original => ({
  ...await original<typeof api>(), createConversation: vi.fn(), readConversation: vi.fn(), controlConversation: vi.fn(),
}));
vi.mock("../src/lib/agent", async original => ({ ...await original<typeof import("../src/lib/agent")>(), stopRun: vi.fn() }));
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
  expect(screen.getByRole("heading",{name:"New prompt"})).toBeTruthy();
  expect(document.activeElement).toBe(getComposer("Free prompt"));
  typeIn(getComposer("Free prompt"), "Explain the project");
  fireEvent.click(screen.getByRole("button", { name: "Send" }));
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
 typeIn(getComposer("Free prompt"),"Prompt for project one");
 fireEvent.click(screen.getByRole("button",{name:"Send"}));
 view.rerender(<ProjectPrompt key="project-two" projectId="project-two" onCreated={opened}/>);
 expect(valueOf(getComposer("Free prompt"))).toBe("");
 resolveCreate({id:"old-project-chat"});
 await waitFor(()=>expect(api.controlConversation).toHaveBeenCalledWith("old-project-chat",{action:"message",message:"Prompt for project one"}));
 expect(opened).not.toHaveBeenCalled();
 typeIn(getComposer("Free prompt"),"Prompt for project two");
 fireEvent.click(screen.getByRole("button",{name:"Send"}));
 await waitFor(()=>expect(opened).toHaveBeenCalledWith("new-project-chat"));
 expect(api.createConversation).toHaveBeenNthCalledWith(2,"project-two","claude");
});
test("the React controller renders Go state, sends only the next message and restores read-only history", async () => {
  vi.mocked(api.readConversation).mockResolvedValue(snapshot("history-chat"));
  render(<ProjectPrompt projectId="project-two" runId="history-chat" onCreated={vi.fn()} />);
  await screen.findByText("Repository explanation");
  expect(api.controlConversation).not.toHaveBeenCalled();
  vi.mocked(api.readConversation).mockResolvedValue({ ...snapshot("history-chat"), readOnly: true });
  typeIn(getComposer(), "Continue");
  await waitFor(() => expect(screen.getByRole("button", { name: "Send" }).hasAttribute("disabled")).toBe(false));
  fireEvent.click(screen.getByRole("button", { name: "Send" }));
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("history-chat", { action: "message", message: "Continue" }));
  await waitFor(() => expect(screen.getByLabelText("Conversation status").textContent).toContain("Read-only history"));
  expect(queryComposer()).toBeNull();
  expect(screen.getByLabelText("History options")).toBeTruthy();
});
test("a free project conversation keeps only relevant status above the composer",async()=>{
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("free-chat"));
 render(<ProjectPrompt projectId="p" runId="free-chat" run={{id:"free-chat",projectId:"p",conversation:true,status:"running",createdAt:"2026-10-30"}} onCreated={vi.fn()}/>);
 await screen.findByText("Repository explanation");
 const strip=screen.getByLabelText("Conversation status");
 expect(strip.textContent).toContain("Ready to respond");
 expect(strip.textContent).toContain("Execution: Active session");
 expect(strip.textContent).not.toContain("Skill :");
 expect(strip.textContent).not.toContain("Ticket :");
 expect(strip.textContent).not.toContain("Étape :");
 expect(screen.getByRole("heading",{level:2}).closest("header")?.querySelector('[role="status"]')).toBeNull();
});
test("an agent outage keeps conversation history and draft while blocking sends until recovery",async()=>{
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("offline-chat"));
 const view=render(<ProjectPrompt projectId="p" runId="offline-chat" agentAvailable run={{id:"offline-chat",projectId:"p",conversation:true,status:"running",createdAt:"2026-10-30"}} onCreated={vi.fn()}/>);
 await screen.findByText("Repository explanation");
 typeIn(getComposer(),"Continue after recovery");
 view.rerender(<ProjectPrompt projectId="p" runId="offline-chat" agentAvailable={false} run={{id:"offline-chat",projectId:"p",conversation:true,status:"running",createdAt:"2026-10-30"}} onCreated={vi.fn()}/>);
 expect(screen.getByLabelText("Conversation status").textContent).toContain("Agent unavailable");
 expect(screen.getByLabelText("Conversation status").textContent).toContain("Execution: Status unavailable");
 expect(screen.getByText("Repository explanation")).toBeTruthy();
 expect(valueOf(getComposer())).toBe("Continue after recovery");
 expect(screen.getByRole("button",{name:"Send"}).hasAttribute("disabled")).toBe(true);
 fireEvent.submit(getComposer().closest("form")!);
 expect(api.controlConversation).not.toHaveBeenCalled();
 view.rerender(<ProjectPrompt projectId="p" runId="offline-chat" agentAvailable run={{id:"offline-chat",projectId:"p",conversation:true,status:"running",createdAt:"2026-10-30"}} onCreated={vi.fn()}/>);
 await waitFor(()=>expect(vi.mocked(api.readConversation).mock.calls.length).toBeGreaterThan(1));
 await waitFor(()=>expect(screen.getByRole("button",{name:"Send"}).hasAttribute("disabled")).toBe(false));
 expect(valueOf(getComposer())).toBe("Continue after recovery");
});
test("a linked conversation shows ticket and execution state above the composer",async()=>{
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("linked-chat"));
 const openExecution=vi.fn();
 const openQueue=vi.fn();
 const openTask=vi.fn();
 render(<ProjectPrompt projectId="p" runId="linked-chat" run={{id:"linked-chat",projectId:"p",conversation:true,taskId:"task",taskKey:"APP-42",skill:"clarify",status:"running",createdAt:"2026-10-30"}} task={{id:"task",key:"APP-42",title:"Linked task",source:"jira",trackerStatus:"In Progress",status:"to_close",prUrl:"https://github.com/team/app/pull/42",prLinks:[{url:"https://github.com/team/app/pull/42",state:"open"}]}} onOpenExecution={openExecution} onOpenQueue={openQueue} onOpenTask={openTask} onCreated={vi.fn()}/>);
 await screen.findByText("Repository explanation");
 expect(screen.getByRole("heading",{name:/APP-42.*Linked task/})).toBeTruthy();
 fireEvent.click(screen.getByRole("button",{name:"Open ticket APP-42"}));
 expect(openTask).toHaveBeenCalledOnce();
 const strip=screen.getByLabelText("Conversation status");
 expect(strip.textContent).toContain("Workflow · Reviewed");
 expect(strip.textContent).not.toContain("Skill :");
 expect(strip.textContent).not.toContain("Ticket :");
 fireEvent.click(screen.getByRole("button",{name:/Workflow · Reviewed/}));
 expect(screen.getByRole("region",{name:"Ticket workflow"}).textContent).toContain("In Progress");
 expect(screen.getByRole("button",{name:"PR #42 · open"})).toBeTruthy();
 expect(screen.getByRole("button",{name:"Stop discussion"})).toBeTruthy();
 // A ticket skill's strip shows the skill status in place of the run state (#908).
 fireEvent.click(screen.getByText("Working · Clarify"));
 fireEvent.click(screen.getByRole("button",{name:"View execution queue"}));
 expect(openQueue).toHaveBeenCalledOnce();
});
test("a ticket conversation lists every linked pull request with its own state",async()=>{
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("multi-pr-chat"));
 render(<ProjectPrompt projectId="p" runId="multi-pr-chat" run={{id:"multi-pr-chat",projectId:"p",conversation:true,taskId:"task",taskKey:"APP-42",skill:"implement",status:"completed",createdAt:"2026-10-30"}} task={{id:"task",key:"APP-42",title:"Linked task",source:"gitlab",status:"to_review",prLinks:[{url:"https://github.com/team/app/pull/42",state:"merged"},{url:"https://gitlab.example.com/team/app/-/merge_requests/7",state:"open"}]}} onCreated={vi.fn()}/>);
 await screen.findByText("Repository explanation");
 fireEvent.click(screen.getByText("PR/MR: 2 linked"));
 expect(screen.getByRole("button",{name:"PR #42 · merged"})).toBeTruthy();
 expect(screen.getByRole("button",{name:"MR !7 · open"})).toBeTruthy();
});
test("a finished discussion archives through its own menu without sending a provider turn",async()=>{
 vi.mocked(api.readConversation).mockResolvedValue({...snapshot("finished-chat"),readOnly:true,canRename:true});
 const archive=vi.fn();
 render(<ProjectPrompt projectId="p" runId="finished-chat" run={{id:"finished-chat",projectId:"p",taskId:"",skill:"",status:"completed",conversation:true,createdAt:"2026-10-30"}} onArchive={archive} onCreated={vi.fn()}/>);
 await screen.findByText("Repository explanation");
 fireEvent.click(screen.getByRole("button",{name:"Archive discussion"}));
 expect(archive).toHaveBeenCalledOnce();
 expect(api.controlConversation).not.toHaveBeenCalled();
});
test("a stopped discussion describes the saved turn as completed and replaces the composer with a compact history footer",async()=>{
 vi.mocked(api.readConversation).mockResolvedValue({...snapshot("stopped-chat"),readOnly:true,events:[
  JSON.stringify({kind:"assistant",text:"OK QA"}),
  JSON.stringify({kind:"notice",text:"Prêt pour votre prochain message"}),
  JSON.stringify({kind:"notice",text:"Conversation arrêtée"}),
 ]});
 render(<ProjectPrompt projectId="p" runId="stopped-chat" onCreated={vi.fn()}/>);
 await screen.findByText("OK QA");
 expect(screen.getByText("Response completed")).toBeTruthy();
 expect(screen.getByText("Conversation arrêtée")).toBeTruthy();
 expect(screen.queryByText("Prêt pour votre prochain message")).toBeNull();
 expect(queryComposer()).toBeNull();
 expect(screen.getByLabelText("Conversation status").textContent).toContain("Read-only history");
 expect(screen.getByRole("button",{name:"Terminal"})).toBeTruthy();
});
test("pending tools require an explicit decision; navigation does not interrupt Go", async () => {
  vi.mocked(api.readConversation).mockResolvedValue({
    ...snapshot("approval-chat"),
    approvals: [{
      id: "tool-1",
      tool: "Bash",
      input: { command: "git status" },
      suggestions: [{ type: "addRules", behavior: "allow", rules: [{ toolName: "Bash", ruleContent: "git status" }] }],
    }],
  });
  const view = render(<ProjectPrompt projectId="project-three" runId="approval-chat" onCreated={vi.fn()} />);
  await screen.findByText("Permission required · Bash");
  expect(api.controlConversation).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "Always allow" })).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Always allow" }));
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("approval-chat", { action: "approval", id: "tool-1", decision: "always" }));
  view.unmount();
  expect(api.controlConversation).not.toHaveBeenCalledWith(expect.anything(), { action: "interrupt" });
});
test("a failed read does not retry a prompt and recovery only observes", async () => {
  vi.mocked(api.readConversation).mockRejectedValueOnce(new Error("Disconnected")).mockResolvedValue(snapshot("recovery-chat"));
  render(<ProjectPrompt projectId="project-four" runId="recovery-chat" onCreated={vi.fn()} />);
  await screen.findByRole("alert");
  expect(screen.getByRole("button", { name: "Send" }).hasAttribute("disabled")).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "Refresh status" }));
  await screen.findByText("Repository explanation");
  expect(api.controlConversation).not.toHaveBeenCalled();
});
test("streamed state exposes interruption and the next turn waits for Go to become idle", async () => {
  vi.mocked(api.readConversation).mockResolvedValue({ ...snapshot("live-chat", true), partial: "Working draft" });
  render(<ProjectPrompt projectId="project-five" runId="live-chat" onCreated={vi.fn()} />);
  await screen.findByText("Working draft");
  // While Go is busy the composer queues instead of sending: an empty draft has nothing to queue.
  expect(screen.queryByRole("button", { name: "Send" })).toBeNull();
  expect(screen.getByRole("button", { name: "Queue message" }).hasAttribute("disabled")).toBe(true);
  vi.mocked(api.readConversation).mockResolvedValue(snapshot("live-chat"));
  fireEvent.click(screen.getByRole("button", { name: "Interrupt" }));
  await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("live-chat", { action: "interrupt" }));
  await waitFor(() => expect(screen.getByLabelText("Conversation status").textContent).toContain("Ready to respond"));
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
  expect(screen.getByRole("button", { name: "Refresh status" }).hasAttribute("disabled")).toBe(false);
  fireEvent.click(screen.getByRole("button", { name: "Refresh status" }));
  await waitFor(() => expect(screen.getByLabelText("Conversation status").textContent).toContain("Ready to respond"));
  expect(api.controlConversation).not.toHaveBeenCalled();
});
test("interactive answers cross IPC as answers, not as an ordinary prompt or a tool grant", async () => {
 vi.mocked(api.readConversation).mockResolvedValue({...snapshot("question-wire"),busy:true,approvals:[{id:"q-wire",tool:"AskUserQuestion",input:{questions:[{question:"Which scope?",options:[{label:"Small"}]}]}}]});
 render(<ProjectPrompt projectId="question-project" runId="question-wire" onCreated={vi.fn()} />);
 await screen.findByRole("radio",{name:"Small"});
 fireEvent.click(screen.getByRole("radio",{name:"Small"}));
 vi.mocked(api.readConversation).mockResolvedValue(snapshot("question-wire"));
 fireEvent.click(screen.getByRole("button",{name:"Send answers"}));
 await waitFor(()=>expect(api.controlConversation).toHaveBeenCalledWith("question-wire",{action:"answer",id:"q-wire",answers:{"Which scope?":"Small"}}));
 await waitFor(()=>expect(screen.queryByRole("button",{name:"Send answers"})).toBeNull());
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
 const input=getComposer("Free prompt");
 typeIn(input,"Explain the code");
 press(input,"Enter",{shiftKey:true});
 expect(api.createConversation).not.toHaveBeenCalled();
 // Shift+Enter is a new line in the message.
 expect(valueOf(input)).toBe("Explain the code\n");
 press(input,"Enter");
 await waitFor(()=>expect(api.controlConversation).toHaveBeenCalledWith("keyboard-prompt",{action:"message",message:"Explain the code\n"}));
 expect(api.createConversation).toHaveBeenCalledOnce();
});
test("prompt suggestions fill the composer without starting execution", () => {
 render(<ProjectPrompt projectId="suggestion-project" onCreated={vi.fn()} />);
 fireEvent.click(screen.getByRole("button",{name:"Prepare a plan"}));
 expect(valueOf(getComposer("Free prompt"))).toContain("without modifying files");
 expect((screen.getByLabelText("Conversation mode") as HTMLSelectElement).value).toBe("plan");
 expect(api.createConversation).not.toHaveBeenCalled();
});

test("running tool commands can be canceled from the tool row or with Escape in the composer", async () => {
 vi.mocked(api.readConversation).mockResolvedValue({
  id: "tool-cancel-chat",
  busy: true,
  readOnly: false,
  events: [JSON.stringify({ kind: "tool", tool: "Bash", toolId: "bash-1", text: "npm test" })],
 });
 render(<ProjectPrompt projectId="project-cancel" runId="tool-cancel-chat" onCreated={vi.fn()} />);
 const cancelToolBtn = await screen.findByRole("button", { name: "Cancel running command" });
 fireEvent.click(cancelToolBtn);
 await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("tool-cancel-chat", { action: "interrupt" }));

 vi.mocked(api.controlConversation).mockClear();
 press(getComposer(), "Escape");
 await waitFor(() => expect(api.controlConversation).toHaveBeenCalledWith("tool-cancel-chat", { action: "interrupt" }));
});

test("turn-ready notices are hidden in active conversations to avoid duplicating Ready to respond", async () => {
 vi.mocked(api.readConversation).mockResolvedValue({
  id: "ready-dedup-chat",
  busy: false,
  readOnly: false,
  events: [
   JSON.stringify({ kind: "assistant", text: "Answer ready" }),
   JSON.stringify({ kind: "notice", text: "Ready for your next message" }),
   JSON.stringify({ kind: "notice", text: "Prêt pour votre prochain message" }),
  ],
 });
 render(<ProjectPrompt projectId="project-dedup" runId="ready-dedup-chat" onCreated={vi.fn()} />);
 await screen.findByText("Answer ready");
 expect(screen.getByLabelText("Conversation status").textContent).toContain("Ready to respond");
 expect(screen.queryByText("Ready for your next message")).toBeNull();
 expect(screen.queryByText("Prêt pour votre prochain message")).toBeNull();
 expect(screen.queryByText("Response completed")).toBeNull();
});

test("task conversation header renders the next workflow step as a chevron icon button with tooltip", async () => {
 vi.mocked(api.readConversation).mockResolvedValue({
  id: "task-chat-next",
  busy: false,
  readOnly: false,
  events: [],
 });
 const onLaunchSkill = vi.fn();
 render(
  <ProjectPrompt
   projectId="project-next"
   runId="task-chat-next"
   run={{ id: "task-chat-next", projectId: "project-next", taskId: "t-1", taskKey: "SEC-1", conversation: true, status: "succeeded", createdAt: "2026-11-15" }}
   task={{ id: "t-1", key: "SEC-1", title: "Ticket test", status: "new", trackerStatus: "Todo" }}
   projectContext={{ configured: true, issueTracker: "local", skills: [{ id: "clarify", name: "Clarify" }] }}
   onLaunchSkill={onLaunchSkill}
   onCreated={vi.fn()}
  />,
 );
 const btn = await screen.findByRole("button", { name: "Next step: Clarify" });
 expect(btn.getAttribute("title")).toBe("Next step: Clarify");
 expect(btn.textContent).toBe("");
 expect(btn.querySelector("svg")).not.toBeNull();
 fireEvent.click(btn);
 expect(onLaunchSkill).toHaveBeenCalledWith("clarify");
});


const workflowProps = (id:string):ProjectPromptProps => ({
 projectId:"p",runId:id,onCreated:vi.fn(),onLaunchSkill:vi.fn(),
 run:{id,projectId:"p",taskId:"ticket",conversation:true,status:"completed",createdAt:"2026-11-18"},
 task:{id:"ticket",key:"#925",title:"Workflow actions",status:"new"},
 projectContext:{configured:true,skills:[{id:"clarify",name:"Clarify"},{id:"implement",name:"Implement"}]},
});

test("closed history shares next-step behavior with the header and retains archive and start over",async()=>{
 const id="closed-actions";
 vi.mocked(api.readConversation).mockResolvedValue({...snapshot(id),readOnly:true});
 const props={...workflowProps(id),archived:true,onStartOver:vi.fn(),onArchive:vi.fn()};
 render(<ProjectPrompt {...props}/>);
 await screen.findByText("Repository explanation");
 const strip=screen.getByLabelText("Conversation status");
 const button=within(strip).getByRole("button",{name:"Next step: Clarify"});
 const workflow=within(strip).getByRole("button",{name:/Workflow/});
 expect(workflow.nextElementSibling).toBe(button);
 expect(button.title).toBe("Next step: Clarify");
 const header=screen.getByRole("heading",{level:2}).closest("header")!;
 fireEvent.click(within(header).getByRole("button",{name:"Next step: Clarify"}));
 expect(props.onLaunchSkill).toHaveBeenCalledExactlyOnceWith("clarify");
 props.onLaunchSkill.mockClear();
 const user=userEvent.setup();
 button.focus();
 await user.keyboard("{Enter}");
 expect(props.onLaunchSkill).toHaveBeenCalledExactlyOnceWith("clarify");
 props.onLaunchSkill.mockClear();
 await user.keyboard(" ");
 expect(props.onLaunchSkill).toHaveBeenCalledExactlyOnceWith("clarify");
 expect(screen.queryByRole("region",{name:"Ticket workflow"})).toBeNull();
 expect(api.controlConversation).not.toHaveBeenCalled();
 expect(screen.queryByRole("button",{name:"Stop discussion"})).toBeNull();
 expect(within(header).getByRole("button",{name:"Archive discussion"})).toBeTruthy();
 expect(within(header).getByRole("button",{name:"Start over"})).toBeTruthy();
});

test("next-step eligibility tracks agent availability and task-wide activity without clearing the draft",async()=>{
 const id="actions-lifecycle";
 vi.mocked(api.readConversation).mockResolvedValue(snapshot(id));
 const props=workflowProps(id);
 const view=render(<ProjectPrompt {...props}/>);
 await screen.findByText("Repository explanation");
 typeIn(getComposer(),"Keep this draft");
 expect(screen.getAllByRole("button",{name:"Next step: Clarify"})).toHaveLength(1);
 expect(within(screen.getByLabelText("Conversation status")).queryByRole("button",{name:/Next step/})).toBeNull();
 view.rerender(<ProjectPrompt {...props} taskRuns={[{...props.run!,id:"other-active",status:"running"}]}/>);
 expect(screen.queryByRole("button",{name:/Next step/})).toBeNull();
 expect(valueOf(getComposer())).toBe("Keep this draft");
 view.rerender(<ProjectPrompt {...props} agentAvailable={false}/>);
 expect(screen.getByRole("button",{name:"Next step: Clarify"}).hasAttribute("disabled")).toBe(true);
 expect(valueOf(getComposer())).toBe("Keep this draft");
 vi.mocked(api.readConversation).mockResolvedValue({...snapshot(id),readOnly:true});
 await act(()=>conversationFor(id).refresh());
 await waitFor(()=>expect(queryComposer()).toBeNull());
 expect(screen.getAllByRole("button",{name:"Next step: Clarify"})).toHaveLength(2);
 for(const button of screen.getAllByRole("button",{name:"Next step: Clarify"}))expect(button.hasAttribute("disabled")).toBe(true);
 view.rerender(<ProjectPrompt {...props} run={{...props.run!,status:"running"}}/>);
 expect(screen.queryByRole("button",{name:/Next step/})).toBeNull();
 expect(within(screen.getByLabelText("Conversation status")).getByRole("button",{name:"Stop discussion"})).toBeTruthy();
});

test.each(["task","association","callback","configuration","skill","finished","unknown"])("closed history omits an unavailable next step: %s",async missing=>{
 const id=`missing-action-${missing}`;
 vi.mocked(api.readConversation).mockResolvedValue({...snapshot(id),readOnly:true});
 const props=workflowProps(id);
 if(missing==="task")props.task=undefined;
 if(missing==="association")props.run={...props.run!,taskId:undefined};
 if(missing==="callback")props.onLaunchSkill=undefined;
 if(missing==="configuration")props.projectContext=undefined;
 if(missing==="skill")props.projectContext={configured:true,skills:[]};
 if(missing==="finished"||missing==="unknown")props.task={...props.task!,status:missing};
 render(<ProjectPrompt {...props}/>);
 await screen.findByText("Repository explanation");
 expect(screen.queryByRole("button",{name:/Next step/})).toBeNull();
});

test("closed history retains PR creation recovery instead of launching review",async()=>{
 const id="recover-pr-action";
 vi.mocked(api.readConversation).mockResolvedValue({...snapshot(id),readOnly:true});
 const props=workflowProps(id);props.task={...props.task!,status:"implemented"};
 render(<ProjectPrompt {...props}/>);
 await screen.findByText("Repository explanation");
 fireEvent.click(within(screen.getByLabelText("Conversation status")).getByRole("button",{name:"Next step: Create / find PR"}));
 expect(props.onLaunchSkill).toHaveBeenCalledExactlyOnceWith("implement");
});

test.each([false,true])("active free conversations keep Stop in the strip when busy is %s",async busy=>{
 const id=`stop-strip-${busy}`;
 vi.mocked(api.readConversation).mockResolvedValue(snapshot(id,busy));
 let resolveStop!:()=>void;
 vi.mocked(stopRun).mockImplementation(()=>new Promise<void>(resolve=>{resolveStop=resolve;}));
 const props={...workflowProps(id),run:{...workflowProps(id).run!,taskId:undefined,status:"running"},onStopped:vi.fn(),onArchive:vi.fn()};
 const view=render(<ProjectPrompt {...props}/>);
 await screen.findByText("Repository explanation");
 const strip=screen.getByLabelText("Conversation status");
 const stop=within(strip).getByRole("button",{name:"Stop discussion"});
 expect(screen.getAllByRole("button",{name:"Stop discussion"})).toHaveLength(1);
 expect(screen.queryByRole("button",{name:"Archive discussion"})).toBeNull();
 view.rerender(<ProjectPrompt {...props} agentAvailable={false}/>);
 expect(stop.hasAttribute("disabled")).toBe(true);
 view.rerender(<ProjectPrompt {...props} run={{...props.run,cancelRequested:true}}/>);
 expect(stop.hasAttribute("disabled")).toBe(true);
 view.rerender(<ProjectPrompt {...props}/>);
 fireEvent.click(stop);fireEvent.click(stop);
 expect(stop.hasAttribute("disabled")).toBe(true);
 expect(stop.title).toBe("Stopping…");
 expect(stopRun).toHaveBeenCalledExactlyOnceWith(id);
 const reads=vi.mocked(api.readConversation).mock.calls.length;
 resolveStop();
 await waitFor(()=>expect(props.onStopped).toHaveBeenCalledOnce());
 expect(vi.mocked(api.readConversation).mock.calls.length).toBeGreaterThan(reads);
 expect(api.controlConversation).not.toHaveBeenCalled();
});

test("Stop failure retains feedback and allows a guarded retry without changing the draft",async()=>{
 const id="stop-strip-retry";
 vi.mocked(api.readConversation).mockResolvedValue(snapshot(id));
 vi.mocked(stopRun).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce(undefined);
 const props=workflowProps(id);props.run={...props.run!,status:"running"};
 render(<ProjectPrompt {...props}/>);
 await screen.findByText("Repository explanation");
 typeIn(getComposer(),"Retained after stop failure");
 fireEvent.click(screen.getByRole("button",{name:"Stop discussion"}));
 await screen.findByText(/Stop not confirmed/);
 expect(valueOf(getComposer())).toBe("Retained after stop failure");
 fireEvent.click(screen.getByRole("button",{name:"Stop discussion"}));
 await waitFor(()=>expect(stopRun).toHaveBeenCalledTimes(2));
});
