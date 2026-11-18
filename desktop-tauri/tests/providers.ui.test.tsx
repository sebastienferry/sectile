import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { invoke } from "@tauri-apps/api/core";
import { ProviderSettings } from "../src/components/ProviderSettings";
import { HarnessSettings } from "../src/components/HarnessSettings";
vi.mock("@tauri-apps/api/core", () => ({ invoke: vi.fn() }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });

test("missing CLI disables login and opening login does not claim success", async () => {
 vi.mocked(invoke).mockImplementation(async (command, args) => command === "provider_login" ? { opened: true } : { installed: (args as {provider:string}).provider === "codex", authentication: "disconnected" });
 render(<ProviderSettings />);
 fireEvent.click(screen.getByRole("tab", { name: "Claude" }));
 await screen.findByText("CLI not detected");
 expect(screen.getByRole("button",{ name: "Sign in to Claude Code" }).hasAttribute("disabled")).toBe(true);
 fireEvent.click(screen.getByRole("tab", { name: "Codex" }));
 fireEvent.click(screen.getByRole("button",{ name: "Sign in to Codex" }));
 await waitFor(() => expect(invoke).toHaveBeenCalledWith("provider_login",{provider:"codex"}));
 await screen.findByText(/Finish signing in/);
 expect(screen.queryByText("CLI detected · connected")).toBeNull();
});

test("refresh reads the CLI result before displaying connected", async () => {
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"unknown"});
 render(<ProviderSettings />);
 fireEvent.click(screen.getByRole("tab", { name: "Claude" }));
 await screen.findByText("CLI detected · connection status unknown");
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"connected"});
 fireEvent.click(screen.getByRole("tab", { name: "Overview" }));
 fireEvent.click(screen.getByRole("button",{name:"Detect AI Engines"}));
 fireEvent.click(screen.getByRole("tab", { name: "Claude" }));
 await screen.findByText("CLI detected · connected");
});

test("one failed CLI check does not hide the other provider's state", async () => {
 vi.mocked(invoke).mockImplementation(async (_command, args) => (args as {provider:string}).provider === "claude" ? Promise.reject(new Error("CLI inaccessible")) : {installed:true,authentication:"connected"});
 render(<ProviderSettings />);
 fireEvent.click(screen.getByRole("tab", { name: "Claude" }));
 expect(await screen.findByText("CLI status unavailable")).toBeTruthy();
 expect(screen.getByText("Claude Code: Error: CLI inaccessible")).toBeTruthy();
 expect(screen.getByRole("button",{name:"Sign in to Claude Code"}).hasAttribute("disabled")).toBe(true);
 fireEvent.click(screen.getByRole("tab", { name: "Codex" }));
 expect(screen.getByText("CLI detected · connected")).toBeTruthy();
 expect(screen.getByRole("button",{name:"Reconnect Codex"}).hasAttribute("disabled")).toBe(false);
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"connected"});
 fireEvent.click(screen.getByRole("tab", { name: "Overview" }));
 fireEvent.click(screen.getByRole("button",{name:"Detect AI Engines"}));
 fireEvent.click(screen.getByRole("tab", { name: "Claude" }));
 await screen.findByText("CLI detected · connected");
 await waitFor(() => expect(screen.queryByText("Claude Code: Error: CLI inaccessible")).toBeNull());
});

test("Claude Code and Codex use their desktop brand marks", async () => {
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"connected"});
 const view = render(<ProviderSettings />);
 fireEvent.click(screen.getByRole("tab", { name: "Claude" }));
 await screen.findByText("CLI detected · connected");
 expect(view.container.querySelector(".provider-card .provider-symbol path")?.getAttribute("d")?.startsWith("M4.709 15.955")).toBe(true);
 fireEvent.click(screen.getByRole("tab", { name: "Codex" }));
 await screen.findByText("CLI detected · connected");
 expect(view.container.querySelector(".provider-card .provider-symbol path")?.getAttribute("d")?.startsWith("M9.205 8.658")).toBe(true);
});

test("detected CLI version and models are displayed per provider", async () => {
 vi.mocked(invoke).mockImplementation(async (_command, args) => (args as {provider:string}).provider === "claude"
  ? { installed: true, path: "/usr/local/bin/claude", version: "2.1.109 (Claude Code)", models: ["claude-opus-4-6", "claude-sonnet-4-6"], authentication: "connected" }
  : { installed: true, path: "/usr/local/bin/codex", version: "codex-cli 0.120.0", models: ["gpt-5.4", "gpt-5.3-codex"], authentication: "connected" });
 render(<ProviderSettings />);
 fireEvent.click(screen.getByRole("tab", { name: "Claude" }));
 expect(await screen.findByText("2.1.109 (Claude Code)")).toBeTruthy();
 expect(screen.getByText("claude-opus-4-6")).toBeTruthy();
 fireEvent.click(screen.getByRole("tab", { name: "Codex" }));
 expect(await screen.findByText("codex-cli 0.120.0")).toBeTruthy();
 expect(screen.getByText("gpt-5.4")).toBeTruthy();
});

test("Harness section and AI engine tabs expose global mode and Claude sandbox controls", async () => {
 vi.mocked(invoke).mockImplementation(async (command) => {
  if (command === "provider_status") return { installed: true, authentication: "connected" };
  if (command === "provider_readiness") {
   return { provider: "claude", permissions: { mode: "auto" } };
  }
  if (command === "get_workstation_sandbox") {
   return { claudeSandbox: { enabled: null }, projects: [] };
  }
  if (command === "get_workstation_harness") {
   return { permissionMode: "", autoApprove: false, networkAccess: false, writablePaths: [] };
  }
  return {};
 });

 render(<HarnessSettings />);
 const modeSelect = await screen.findByLabelText("Global permission mode");
 fireEvent.change(modeSelect, { target: { value: "auto" } });
 const networkCheckbox = await screen.findByRole("checkbox", { name: /Allow network/i });
 fireEvent.click(networkCheckbox);
 fireEvent.click(screen.getByRole("button", { name: "+ ~/.cargo" }));
 fireEvent.click(screen.getByRole("button", { name: "Save global harness" }));
 await waitFor(() =>
  expect(invoke).toHaveBeenCalledWith("save_workstation_harness", {
   input: { permissionMode: "auto", autoApprove: true, networkAccess: true, writablePaths: ["~/.cargo"] },
  }),
 );
 cleanup();

 render(<ProviderSettings projects={[{ id: "p1", name: "Sectile" }]} />);
 expect(screen.queryByText("Claude Code")).toBeNull();
 fireEvent.click(screen.getByRole("tab", { name: "Claude" }));
 await screen.findByRole("button", { name: "Autonomy in sandbox" });
 fireEvent.click(screen.getByRole("button", { name: "Autonomy in sandbox" }));
 fireEvent.click(screen.getByRole("button", { name: "Save Claude sandbox" }));
 await waitFor(() =>
  expect(invoke).toHaveBeenCalledWith("save_workstation_sandbox", expect.objectContaining({
   input: expect.objectContaining({
    claudeSandbox: expect.objectContaining({
     enabled: true,
     autoAllowBashIfSandboxed: true,
     allowUnsandboxedCommands: false,
    }),
   }),
  })),
 );
});
