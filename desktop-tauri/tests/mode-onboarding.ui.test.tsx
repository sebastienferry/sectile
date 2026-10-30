import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ModeOnboarding } from "../src/components/ModeOnboarding";
import * as api from "../src/lib/local";

vi.mock("../src/lib/local", () => ({
  enableLocalRuntime: vi.fn(),
  pairFleetAgent: vi.fn(),
  attachFleetAgent: vi.fn(),
}));

beforeEach(() => {
  vi.resetAllMocks();
});

afterEach(() => {
  cleanup();
});

test("renders onboarding view with Local Companion and Agent Fleet cards", () => {
  render(<ModeOnboarding onConfigured={vi.fn()} />);

  expect(screen.getByRole("heading", { name: "Bienvenue dans Sectile Desktop" })).toBeTruthy();
  expect(screen.getByRole("heading", { name: "Compagnon Local" })).toBeTruthy();
  expect(screen.getByRole("heading", { name: "Agent Fleet" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Démarrer en mode Compagnon" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Configurer le mode Fleet" })).toBeTruthy();
});

test("selecting Local Companion initializes local runtime and calls onConfigured", async () => {
  const onConfigured = vi.fn();
  vi.mocked(api.enableLocalRuntime).mockResolvedValue(undefined);

  render(<ModeOnboarding onConfigured={onConfigured} />);

  fireEvent.click(screen.getByRole("button", { name: "Démarrer en mode Compagnon" }));
  expect(screen.getByText("Démarrage du compagnon local…")).toBeTruthy();

  await waitFor(() => {
    expect(api.enableLocalRuntime).toHaveBeenCalledOnce();
    expect(onConfigured).toHaveBeenCalledOnce();
  });
});

test("selecting Agent Fleet reveals pairing form and pairs with Fleet server", async () => {
  const onConfigured = vi.fn();
  vi.mocked(api.pairFleetAgent).mockResolvedValue(undefined);

  render(<ModeOnboarding onConfigured={onConfigured} />);

  fireEvent.click(screen.getByRole("button", { name: "Configurer le mode Fleet" }));

  const urlInput = screen.getByLabelText("Adresse du serveur Fleet");
  const codeInput = screen.getByLabelText("Code de jumelage");

  fireEvent.change(urlInput, { target: { value: "https://sectile.example.com" } });
  fireEvent.change(codeInput, { target: { value: "pair123" } });

  fireEvent.click(screen.getByRole("button", { name: "Associer à Fleet" }));

  await waitFor(() => {
    expect(api.pairFleetAgent).toHaveBeenCalledWith("https://sectile.example.com", "pair123");
    expect(onConfigured).toHaveBeenCalledOnce();
  });
});

test("attaching an existing Fleet agent calls attachFleetAgent", async () => {
  const onConfigured = vi.fn();
  vi.mocked(api.attachFleetAgent).mockResolvedValue(undefined);

  render(<ModeOnboarding onConfigured={onConfigured} />);

  fireEvent.click(screen.getByRole("button", { name: "Configurer le mode Fleet" }));
  fireEvent.click(screen.getByRole("button", { name: "Associer l’agent Fleet existant" }));

  await waitFor(() => {
    expect(api.attachFleetAgent).toHaveBeenCalledOnce();
    expect(onConfigured).toHaveBeenCalledOnce();
  });
});

test("displays error message when runtime enablement fails", async () => {
  vi.mocked(api.enableLocalRuntime).mockRejectedValue(new Error("Local startup failed"));

  render(<ModeOnboarding onConfigured={vi.fn()} />);

  fireEvent.click(screen.getByRole("button", { name: "Démarrer en mode Compagnon" }));

  expect(await screen.findByRole("alert")).toBeTruthy();
  expect(screen.getByRole("alert").textContent).toContain("Local startup failed");
});
