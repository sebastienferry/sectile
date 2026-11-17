import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import ProjectPrompt from "../src/components/ProjectPrompt";
import * as api from "../src/lib/conversation";
vi.mock("../src/lib/conversation", async original => ({
  ...await original<typeof api>(), createConversation: vi.fn(), readConversation: vi.fn(), controlConversation: vi.fn(),
}));
beforeEach(() => { vi.resetAllMocks(); });
afterEach(cleanup);

const conversation = (id: string, context?: { used: number; window: number }) => ({ id, busy: false, readOnly: false, provider: "claude", model: "opus", events: [JSON.stringify({ kind: "assistant", text: "Ready" })], ...(context ? { context } : {}) });
async function open(id: string, context?: { used: number; window: number }) {
  vi.mocked(api.readConversation).mockResolvedValue(conversation(id, context));
  render(<ProjectPrompt projectId="p" runId={id} onCreated={vi.fn()} />);
  await screen.findByText("Ready");
}

test("the gauge stays hidden until the window is known", async () => {
  await open("no-window", { used: 1200, window: 0 });
  expect(screen.queryByRole("progressbar")).toBeNull();
  cleanup();
  await open("no-context");
  expect(screen.queryByRole("progressbar")).toBeNull();
});

test("the gauge shows the share of the window beside the model selector", async () => {
  await open("used", { used: 36000, window: 200000 });
  const meter = screen.getByRole("progressbar", { name: "Context usage: 18% · 36,000 / 200,000" });
  expect(meter.getAttribute("aria-valuenow")).toBe("18");
  expect(meter.getAttribute("title")).toBe("18% · 36,000 / 200,000");
  expect(meter.textContent).toBe("18%");
  expect(meter.classList.contains("warning")).toBe(false);
  expect(meter.closest(".composer-footer")).toBeTruthy();
  const fill = meter.querySelector(".context-meter-fill")!;
  const length = Number(fill.getAttribute("stroke-dasharray"));
  expect(Number(fill.getAttribute("stroke-dashoffset"))).toBeCloseTo(length * 0.82);
});

test("above 80% the gauge warns and suggests /compact", async () => {
  await open("full", { used: 170000, window: 200000 });
  const meter = screen.getByRole("progressbar");
  expect(meter.classList.contains("warning")).toBe(true);
  expect(meter.getAttribute("aria-label")).toBe("Context usage: 85% · 170,000 / 200,000 · Consider /compact");
});
