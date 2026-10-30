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
function mount(view: "launch" | "executions" | "orchestration" = "launch") {
  return render(
    <ExecutionProvider>
      <Supervisor
        projectId="p"
        tasks={[task]}
        onChanged={vi.fn()}
        view={view}
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
    name: "Démarrer la tâche",
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
    (screen.getByRole("button", { name: "Lancement…" }) as HTMLButtonElement)
      .disabled,
  ).toBe(true);
  await act(async () => finish());
  await screen.findByText(/Demande acceptée/);
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
  fireEvent.click(await screen.findByRole("button", { name: "Arrêter" }));
  expect(api.stopRun).toHaveBeenCalledWith("r1");
  expect(screen.queryByText(/Arrêt confirmé/)).toBeNull();
  await act(async () => reject("Exit not confirmed"));
  await screen.findByText("Exit not confirmed");
  expect(screen.queryByText(/Arrêt confirmé/)).toBeNull();
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
  expect(free.textContent).toContain("Codex · Session active");
  expect(screen.getByTitle("APP-2 · Fix search").textContent).toContain("Claude Code · clarify · Session active");
  expect(screen.queryByText("Console libre")).toBeNull();
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
    name: "Lancer cet agent",
  });
  await waitFor(() =>
    expect((button as HTMLButtonElement).disabled).toBe(false),
  );
  fireEvent.click(button);
  await screen.findByText("APP-4 : Capacity full");
  expect(api.runOrchestration).toHaveBeenCalledWith("p", "agent");
});
test("polling failure retains visible history but disables stop", async () => {
  vi.useFakeTimers();
  mount("executions");
  await act(async () => {
    await Promise.resolve();
  });
  expect(
    (screen.getByRole("button", { name: "Arrêter" }) as HTMLButtonElement)
      .disabled,
  ).toBe(false);
  vi.mocked(api.executionSnapshot).mockRejectedValue("Offline");
  await act(async () => {
    await vi.advanceTimersByTimeAsync(3000);
  });
  expect(screen.getByText("APP-1")).toBeTruthy();
  expect(
    (screen.getByRole("button", { name: "Arrêter" }) as HTMLButtonElement)
      .disabled,
  ).toBe(true);
  expect(screen.getByText(/données affichées sont anciennes/)).toBeTruthy();
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
