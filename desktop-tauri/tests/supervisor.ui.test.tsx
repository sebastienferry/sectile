import { afterEach, beforeEach, expect, test, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { ExecutionProvider } from "../src/components/ExecutionMonitor";
import { Supervisor } from "../src/components/Supervisor";
import * as api from "../src/lib/agent";
vi.mock("../src/lib/agent", () => ({
  executionSnapshot: vi.fn(),
  projectSkills: vi.fn(),
  getOrchestration: vi.fn(),
  launchTask: vi.fn(),
  stopRun: vi.fn(),
  runOrchestration: vi.fn(),
}));
vi.mock("../src/components/RunConsole", () => ({
  RunConsole: ({ runId }: { runId: string }) => <div>Console {runId}</div>,
}));
const run = {
  id: "r1",
  projectId: "p",
  taskId: "t1",
  taskKey: "APP-1",
  skill: "implement_issue",
  status: "running",
  createdAt: "2026-10-28T12:00:00Z",
};
const task = {
  id: "t2",
  title: "Fix search",
  key: "APP-2",
  source: "github",
  status: "to_implement",
};
const snapshot = {
  status: { connected: true, capabilities: ["orchestration-agents"] },
  runs: [run],
};
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.executionSnapshot).mockResolvedValue(snapshot);
  vi.mocked(api.projectSkills).mockResolvedValue([{ id: "implement_issue" }]);
  vi.mocked(api.getOrchestration).mockResolvedValue({
    orchestratorEnabled: true,
    orchestratorMaxConcurrent: 2,
    agents: [
      {
        id: "agent",
        name: "Builder",
        file: "AGENTS.md",
        guidelines: "Implement small tickets.",
      },
    ],
  });
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});
function mount(
  view: "launch" | "executions" | "orchestration" = "launch",
  archivedConversationIds: string[] = [],
) {
  return render(
    <ExecutionProvider>
      <Supervisor
        projectId="p"
        tasks={[task]}
        onChanged={vi.fn()}
        view={view}
        archivedConversationIds={archivedConversationIds}
      />
    </ExecutionProvider>,
  );
}
test("launch sends the chosen task and blocks duplicate submissions while pending", async () => {
  let finish!: () => void;
  vi.mocked(api.launchTask).mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = () => resolve({});
      }),
  );
  mount();
  await screen.findByText("APP-1");
  fireEvent.change(screen.getByLabelText("Ticket"), {
    target: { value: "t2" },
  });
  const button = screen.getByRole("button", {
    name: "Start task",
  }) as HTMLButtonElement;
  await waitFor(() => expect(button.disabled).toBe(false));
  fireEvent.click(button);
  fireEvent.click(button);
  expect(api.launchTask).toHaveBeenCalledTimes(1);
  expect(api.launchTask).toHaveBeenCalledWith("p", {
    taskId: "t2",
    skillId: "implement_issue",
    prompt: "",
    mode: "",
  });
  expect(
    (screen.getByRole("button", { name: "Launching…" }) as HTMLButtonElement)
      .disabled,
  ).toBe(true);
  await act(async () => finish());
  await screen.findByText(/Request accepted/);
});
test("stop waits for agent acknowledgement and displays refusal", async () => {
  let reject!: (reason: string) => void;
  vi.mocked(api.stopRun).mockImplementation(
    () =>
      new Promise((_, fail) => {
        reject = fail;
      }),
  );
  mount("executions");
  fireEvent.click(await screen.findByRole("button", { name: "Stop" }));
  expect(api.stopRun).toHaveBeenCalledWith("r1");
  expect(screen.queryByText(/Stop confirmed/)).toBeNull();
  await act(async () => reject("Exit not confirmed"));
  await screen.findByText("Exit not confirmed");
  expect(screen.queryByText(/Stop confirmed/)).toBeNull();
});
test("the execution queue names free and ticket conversations by their work", async () => {
  vi.mocked(api.executionSnapshot).mockResolvedValue({
    status: { connected: true, capabilities: [] },
    runs: [
      { ...run, id: "free", taskId: "", taskKey: "", conversation: true, title: "Inspect API errors", provider: "codex", skill: "" },
      { ...run, id: "ticket", taskId: "t2", taskKey: "APP-2", conversation: true, title: "$clarify-issue", provider: "claude", skill: "clarify" },
    ],
  });
  mount("executions");
  const free = await screen.findByTitle("Inspect API errors");
  expect(free.textContent).toContain("Codex · Active session");
  expect(screen.getByTitle("APP-2 · Fix search").textContent).toContain("Claude Code · clarify · Active session");
  expect(screen.queryByText("Free console")).toBeNull();
});
test("the execution queue sorts by descending date and filters by text and archived state", async () => {
  vi.mocked(api.executionSnapshot).mockResolvedValue({
    status: { connected: true, capabilities: [] },
    runs: [
      { ...run, id: "older-active", taskId: "", taskKey: "", conversation: true, title: "Older active chat", status: "running", createdAt: "2026-10-25T10:00:00Z" },
      { ...run, id: "newer-archived", taskId: "", taskKey: "", conversation: true, title: "Newer archived chat", status: "completed", createdAt: "2026-10-30T10:00:00Z" },
    ],
  });
  mount("executions", ["newer-archived"]);
  await screen.findByTitle("Newer archived chat");
  const items = Array.from(document.querySelectorAll(".execution-list .run-select strong")).map(node => node.textContent);
  expect(items).toEqual(["Newer archived chat", "Older active chat"]);

  fireEvent.click(screen.getByRole("button", { name: "Archived" }));
  expect(screen.getByTitle("Newer archived chat")).toBeTruthy();
  expect(screen.queryByTitle("Older active chat")).toBeNull();

  fireEvent.click(screen.getByRole("button", { name: "Unarchived" }));
  expect(screen.queryByTitle("Newer archived chat")).toBeNull();
  expect(screen.getByTitle("Older active chat")).toBeTruthy();

  fireEvent.click(
    screen
      .getByRole("group", { name: "Filter by archive status" })
      .querySelectorAll("button")[0],
  );
  fireEvent.change(screen.getByRole("textbox", { name: "Filter executions" }), { target: { value: "Newer" } });
  expect(screen.getByTitle("Newer archived chat")).toBeTruthy();
  expect(screen.queryByTitle("Older active chat")).toBeNull();
});
test("orchestration submits only the selected saved agent and reports skips", async () => {
  vi.mocked(api.runOrchestration).mockResolvedValue({
    evaluatedCount: 2,
    activeRunsAfter: 2,
    maxConcurrent: 2,
    launched: [{ id: "t", key: "APP-3", title: "Work" }],
    skipped: [{ id: "s", key: "APP-4", reason: "Capacity full" }],
  });
  mount("orchestration");
  const button = await screen.findByRole("button", {
    name: "Launch this agent",
  });
  await waitFor(() =>
    expect((button as HTMLButtonElement).disabled).toBe(false),
  );
  fireEvent.click(button);
  await screen.findByText("APP-4: Capacity full");
  expect(api.runOrchestration).toHaveBeenCalledWith("p", "agent");
});
test("polling failure retains visible history but disables stop", async () => {
  vi.useFakeTimers();
  mount("executions");
  await act(async () => {
    await Promise.resolve();
  });
  expect(
    (screen.getByRole("button", { name: "Stop" }) as HTMLButtonElement)
      .disabled,
  ).toBe(false);
  vi.mocked(api.executionSnapshot).mockRejectedValue("Offline");
  await act(async () => {
    await vi.advanceTimersByTimeAsync(3000);
  });
  expect(screen.getByText("APP-1")).toBeTruthy();
  expect(
    (screen.getByRole("button", { name: "Stop" }) as HTMLButtonElement)
      .disabled,
  ).toBe(true);
  expect(screen.getByText(/Displayed data is stale/i)).toBeTruthy();
});
test("unmount stops polling", async () => {
  vi.useFakeTimers();
  const view = mount();
  await act(async () => {
    await Promise.resolve();
  });
  const count = vi.mocked(api.executionSnapshot).mock.calls.length;
  view.unmount();
  await act(async () => {
    await vi.advanceTimersByTimeAsync(10000);
  });
  expect(api.executionSnapshot).toHaveBeenCalledTimes(count);
});
test("status filter narrows executions to active runs", async () => {
  vi.mocked(api.executionSnapshot).mockResolvedValue({
    status: { connected: true, capabilities: [] },
    runs: [
      run,
      {
        id: "r2",
        projectId: "p",
        taskId: "t2",
        taskKey: "APP-2",
        skill: "implement_issue",
        status: "completed",
        createdAt: "2026-10-28T11:00:00Z",
      },
      {
        id: "r3",
        projectId: "p",
        taskId: "t3",
        taskKey: "APP-3",
        skill: "implement_issue",
        status: "failed",
        createdAt: "2026-10-28T10:00:00Z",
      },
    ],
  });
  mount("executions");
  await screen.findByText("APP-1");
  expect(screen.getByText("APP-2")).toBeTruthy();
  expect(screen.getByText("APP-3")).toBeTruthy();

  const statusGroup = screen.getByRole("group", { name: "Filter by status" });
  fireEvent.click(statusGroup.querySelectorAll("button")[1]);
  expect(screen.getByText("APP-1")).toBeTruthy();
  expect(screen.queryByText("APP-2")).toBeNull();
  expect(screen.queryByText("APP-3")).toBeNull();
});
test("completed headless executions expose archive and restore actions", async () => {
  const onToggleArchive = vi.fn();
  vi.mocked(api.executionSnapshot).mockResolvedValue({
    status: { connected: true, capabilities: [] },
    runs: [
      run,
      {
        id: "r2",
        projectId: "p",
        taskId: "t2",
        taskKey: "APP-2",
        skill: "pickup",
        status: "completed",
        headless: true,
        createdAt: "2026-10-28T11:00:00Z",
      },
    ],
  });
  render(
    <ExecutionProvider>
      <Supervisor
        projectId="p"
        tasks={[task]}
        onChanged={vi.fn()}
        view="executions"
        archivedConversationIds={[]}
        onToggleArchive={onToggleArchive}
      />
    </ExecutionProvider>,
  );
  await screen.findByText("APP-2");
  const archiveButtons = screen.getAllByRole("button", { name: "Archive execution" });
  expect(archiveButtons).toHaveLength(1);
  fireEvent.click(archiveButtons[0]);
  expect(onToggleArchive).toHaveBeenCalledWith("r2", true);
});
