import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { invoke } from "@tauri-apps/api/core";
import { ProviderSettings } from "../src/components/ProviderSettings";
vi.mock("@tauri-apps/api/core", () => ({ invoke: vi.fn() }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });
test("missing CLI disables login and opening login does not claim success", async () => {
 vi.mocked(invoke).mockImplementation(async (command, args) => command === "provider_login" ? { opened: true } : { installed: (args as {provider:string}).provider === "codex", authentication: "disconnected" });
 render(<ProviderSettings />);
 await screen.findByText("CLI non détectée");
 expect(screen.getByRole("button",{ name: "Se connecter à Claude Code" }).hasAttribute("disabled")).toBe(true);
 fireEvent.click(screen.getByRole("button",{ name: "Se connecter à Codex" }));
 await waitFor(() => expect(invoke).toHaveBeenCalledWith("provider_login",{provider:"codex"}));
 await screen.findByText(/Terminez la connexion/);
 expect(screen.queryByText("CLI détectée · connecté")).toBeNull();
});
test("refresh reads the CLI result before displaying connected", async () => {
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"unknown"});
 render(<ProviderSettings />);
 await screen.findAllByText("CLI détectée · état de connexion indéterminé");
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"connected"});
 fireEvent.click(screen.getByRole("button",{name:"Vérifier les connexions"}));
 await screen.findAllByText("CLI détectée · connecté");
});
test("one failed CLI check does not hide the other provider's state", async () => {
 vi.mocked(invoke).mockImplementation(async (_command, args) => (args as {provider:string}).provider === "claude" ? Promise.reject(new Error("CLI inaccessible")) : {installed:true,authentication:"connected"});
 render(<ProviderSettings />);
 expect(await screen.findByText("État de la CLI indisponible")).toBeTruthy();
 expect(screen.getByText("Claude Code : Error: CLI inaccessible")).toBeTruthy();
 expect(screen.getByText("CLI détectée · connecté")).toBeTruthy();
 expect(screen.getByRole("button",{name:"Se connecter à Claude Code"}).hasAttribute("disabled")).toBe(true);
 expect(screen.getByRole("button",{name:"Reconnecter Codex"}).hasAttribute("disabled")).toBe(false);
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"connected"});
 fireEvent.click(screen.getByRole("button",{name:"Vérifier les connexions"}));
 await screen.findAllByText("CLI détectée · connecté");
 await waitFor(() => expect(screen.queryByText("Claude Code : Error: CLI inaccessible")).toBeNull());
});
test("Claude Code and Codex use their desktop brand marks", async () => {
 vi.mocked(invoke).mockResolvedValue({installed:true,authentication:"connected"});
 const view = render(<ProviderSettings />);
 await screen.findAllByText("CLI détectée · connecté");
 const cards = view.container.querySelectorAll(".provider-card");
 expect(cards).toHaveLength(2);
 expect(cards[0].querySelector(".provider-symbol path")?.getAttribute("d")?.startsWith("M4.709 15.955")).toBe(true);
 expect(cards[1].querySelector(".provider-symbol path")?.getAttribute("d")?.startsWith("M9.205 8.658")).toBe(true);
});
