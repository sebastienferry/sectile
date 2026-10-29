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
 fireEvent.click(screen.getByRole("button",{name:"Détecter / vérifier les connexions"}));
 await screen.findAllByText("CLI détectée · connecté");
});
