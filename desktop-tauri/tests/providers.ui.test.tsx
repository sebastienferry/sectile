import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { invoke } from "@tauri-apps/api/core";
import { ProviderSettings } from "../src/components/ProviderSettings";
vi.mock("@tauri-apps/api/core", () => ({ invoke: vi.fn() }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });
test("missing CLI disables login and opening login does not claim success", async () => {
 vi.mocked(invoke).mockImplementation(async (command, args) => command === "provider_login" ? { opened: true } : { installed: (args as {provider:string}).provider === "codex", authentication: "disconnected" });
 render(<ProviderSettings />);
 await screen.findByText("CLI not detected");
 expect(screen.getByRole("button",{ name: "Sign in to Claude Code" }).hasAttribute("disabled")).toBe(true);
 fireEvent.click(screen.getByRole("button",{ name: "Sign in to Codex" }));
 await waitFor(() => expect(invoke).toHaveBeenCalledWith("provider_login",{provider:"codex"}));
 await screen.findByText(/Finish signing in/);
 expect(screen.queryByText("CLI detected · connected")).toBeNull();
});
test("refresh reads the CLI result before displaying connected", async () => {
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"unknown"});
 render(<ProviderSettings />);
 await screen.findAllByText("CLI detected · connection status unknown");
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"connected"});
 fireEvent.click(screen.getByRole("button",{name:"Check connections"}));
 await screen.findAllByText("CLI detected · connected");
});
test("one failed CLI check does not hide the other provider's state", async () => {
 vi.mocked(invoke).mockImplementation(async (_command, args) => (args as {provider:string}).provider === "claude" ? Promise.reject(new Error("CLI inaccessible")) : {installed:true,authentication:"connected"});
 render(<ProviderSettings />);
 expect(await screen.findByText("CLI status unavailable")).toBeTruthy();
 expect(screen.getByText("Claude Code: Error: CLI inaccessible")).toBeTruthy();
 expect(screen.getByText("CLI detected · connected")).toBeTruthy();
 expect(screen.getByRole("button",{name:"Sign in to Claude Code"}).hasAttribute("disabled")).toBe(true);
 expect(screen.getByRole("button",{name:"Reconnect Codex"}).hasAttribute("disabled")).toBe(false);
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"connected"});
 fireEvent.click(screen.getByRole("button",{name:"Check connections"}));
 await screen.findAllByText("CLI detected · connected");
 await waitFor(() => expect(screen.queryByText("Claude Code: Error: CLI inaccessible")).toBeNull());
});
test("Claude Code and Codex use their desktop brand marks", async () => {
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"connected"});
 const view = render(<ProviderSettings />);
 await screen.findAllByText("CLI detected · connected");
 const cards = view.container.querySelectorAll(".provider-card");
 expect(cards).toHaveLength(2);
 expect(cards[0].querySelector(".provider-symbol path")?.getAttribute("d")?.startsWith("M4.709 15.955")).toBe(true);
 expect(cards[1].querySelector(".provider-symbol path")?.getAttribute("d")?.startsWith("M9.205 8.658")).toBe(true);
});
test("detected CLI version and models are displayed per provider", async () => {
 vi.mocked(invoke).mockImplementation(async (_command, args) => (args as {provider:string}).provider === "claude"
  ? { installed: true, path: "/usr/local/bin/claude", version: "2.1.109 (Claude Code)", models: ["claude-opus-4-6", "claude-sonnet-4-6"], authentication: "connected" }
  : { installed: true, path: "/usr/local/bin/codex", version: "codex-cli 0.120.0", models: ["gpt-5.4", "gpt-5.3-codex"], authentication: "connected" });
 render(<ProviderSettings />);
 expect(await screen.findByText("2.1.109 (Claude Code)")).toBeTruthy();
 expect(screen.getByText("codex-cli 0.120.0")).toBeTruthy();
 expect(screen.getByText("claude-opus-4-6")).toBeTruthy();
 expect(screen.getByText("gpt-5.4")).toBeTruthy();
});
