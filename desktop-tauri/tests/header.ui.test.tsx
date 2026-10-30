import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
vi.mock("@tauri-apps/api/core", () => ({ invoke: vi.fn() }));
vi.mock("@tauri-apps/api/event", () => ({ listen: vi.fn() }));
import App from "../src/App";
import * as workspace from "../src/lib/workspace";
vi.mock("../src/lib/workspace", async original => ({ ...await original<typeof workspace>(), projectContext: vi.fn(), openExternalLink: vi.fn() }));
import * as api from "../src/lib/agent";
vi.mock("../src/lib/agent", () => ({
  listProjects: vi.fn(), listTasks: vi.fn(), getTask: vi.fn(), executionSnapshot: vi.fn(),
  projectSkills: vi.fn(), getOrchestration: vi.fn(), launchTask: vi.fn(),
  stopRun: vi.fn(), runOrchestration: vi.fn(),
}));

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(workspace.projectContext).mockResolvedValue({ configured: true, path: "/work/example", skills: [], policy: {} });
  vi.stubGlobal("matchMedia", () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
  vi.mocked(api.listProjects).mockResolvedValue([{ id: "p", name: "Example" }]);
  vi.mocked(api.listTasks).mockResolvedValue([]);
  vi.mocked(api.projectSkills).mockResolvedValue([]);
  vi.mocked(api.getOrchestration).mockResolvedValue({ orchestratorEnabled: false, agents: [] });
  vi.mocked(api.executionSnapshot).mockResolvedValue({
    status: { connected: true, server: "http://localhost:8090", capabilities: [] },
    runs: [],
  });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  localStorage.clear();
});

test("header displays Sectile Desktop branding and draggable region", async () => {
  render(<App />);
  const title = await screen.findByText("Sectile Desktop");
  expect(title).toBeDefined();
  const header = title.closest("header");
  expect(header).toBeDefined();
  expect(header?.hasAttribute("data-tauri-drag-region")).toBe(true);
});

test("header displays connection indicator with link to server", async () => {
  render(<App />);
  const connectionLink = await screen.findByText("Connected");
  expect(connectionLink).toBeDefined();
  const connectionContainer = document.querySelector("#connection");
  expect(connectionContainer?.getAttribute("data-state")).toBe("on");
  expect(connectionContainer?.querySelector(".connection-dot")).toBeDefined();

  fireEvent.click(connectionLink);
  expect(invoke).toHaveBeenCalledWith("open_external_link", { url: "http://localhost:8090" });
});

test("header displays disconnected indicator when status is disconnected", async () => {
  vi.mocked(api.executionSnapshot).mockResolvedValue({
    status: { connected: false, capabilities: [] },
    runs: [],
  });
  render(<App />);
  const notConnected = await screen.findByText("Not connected");
  expect(notConnected).toBeDefined();
  const connectionContainer = document.querySelector("#connection");
  expect(connectionContainer?.getAttribute("data-state")).toBe("off");
});

test("header includes agent actions menu with restart and stop options", async () => {
  render(<App />);
  const moreButton = await screen.findByLabelText("Agent actions");
  expect(moreButton).toBeDefined();

  fireEvent.click(moreButton);
  expect(await screen.findByText("Restart agent")).toBeDefined();
  expect(screen.getByText("Stop agent")).toBeDefined();
});

test("header includes theme toggle controls and settings button", async () => {
  render(<App />);
  expect(await screen.findByRole("button", { name: "Thème sombre" })).toBeDefined();
  expect(screen.getByRole("button", { name: "Thème clair" })).toBeDefined();
  expect(screen.getByRole("button", { name: "Thème système" })).toBeDefined();
  expect(screen.getByRole("button", { name: "Paramètres" })).toBeDefined();
});
