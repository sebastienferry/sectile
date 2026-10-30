import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { LocalSettings } from "../src/components/LocalSettings";
import * as api from "../src/lib/local";

vi.mock("../src/components/ProviderSettings", () => ({ ProviderSettings: () => <div>Provider settings ready</div> }));
vi.mock("../src/components/OAuthConnections", () => ({ OAuthConnections: () => <div>OAuth connections</div> }));
vi.mock("../src/lib/local", () => ({
  localSetupStatus: vi.fn(),
  localServiceLog: vi.fn(),
  enableLocalRuntime: vi.fn(),
  prepareLocalUpdate: vi.fn(),
  attachFleetAgent: vi.fn(),
  pairFleetAgent: vi.fn(),
  saveLocalProfile: vi.fn(),
  saveLocalCredential: vi.fn(),
  createLocalTracker: vi.fn(),
  updateLocalTracker: vi.fn(),
  deleteLocalTracker: vi.fn(),
  createLocalProject: vi.fn(),
  attachProjectTracker: vi.fn(),
  chooseProjectFolder: vi.fn(),
  syncLocalTracker: vi.fn(),
  trackerSyncStatus: vi.fn(),
  activeTrackerSyncs: vi.fn(),
}));

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.localServiceLog).mockResolvedValue({ path: "/test/companion.log", text: "service ready", missing: false, truncated: false });
  vi.mocked(api.activeTrackerSyncs).mockResolvedValue([]);
});

afterEach(() => {
  cleanup();
});

test("displays active mode badge for Local mode and allows expanding Fleet mode switch", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue({
    enabled: true,
    mode: "local",
    ready: true,
    signedIn: true,
    agentReady: true,
    profile: { email: "local@example.com", displayName: "Local User" },
  });

  render(<LocalSettings onChanged={vi.fn()} initialSection="services" />);

  expect(await screen.findByText("Mode actif : Compagnon Local")).toBeTruthy();
  expect(screen.getByText("Basculer vers le mode Fleet (Agent d’équipe)")).toBeTruthy();

  // Expand the Fleet switch details
  fireEvent.click(screen.getByText("Basculer vers le mode Fleet (Agent d’équipe)"));
  expect(screen.getByLabelText("Adresse du backend Fleet")).toBeTruthy();
  expect(screen.getByLabelText("Code de jumelage")).toBeTruthy();
});

test("displays active mode badge for Fleet mode and allows switching back to Local", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue({
    enabled: true,
    mode: "fleet",
    ready: true,
    signedIn: true,
    agentReady: true,
    fleetServer: "https://fleet.example.com",
    profile: { email: "fleet@example.com", displayName: "Fleet User" },
  });
  vi.mocked(api.enableLocalRuntime).mockResolvedValue(undefined);

  render(<LocalSettings onChanged={vi.fn()} initialSection="services" />);

  expect(await screen.findByText("Mode actif : Agent Fleet")).toBeTruthy();
  expect(screen.getByText("Basculer vers le mode Compagnon local")).toBeTruthy();

  fireEvent.click(screen.getByRole("button", { name: "Activer le mode Compagnon local" }));

  await waitFor(() => {
    expect(api.enableLocalRuntime).toHaveBeenCalledOnce();
  });
});

test("prompts confirmation when switching mode while active executions exist", async () => {
  vi.mocked(api.localSetupStatus).mockResolvedValue({
    enabled: true,
    mode: "fleet",
    ready: true,
    signedIn: true,
    agentReady: true,
    fleetServer: "https://fleet.example.com",
    profile: { email: "fleet@example.com", displayName: "Fleet User" },
  });
  vi.mocked(api.enableLocalRuntime).mockResolvedValue(undefined);

  const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);

  render(<LocalSettings onChanged={vi.fn()} initialSection="services" hasActiveExecutions={true} />);

  await screen.findByText("Mode actif : Agent Fleet");
  fireEvent.click(screen.getByRole("button", { name: "Activer le mode Compagnon local" }));

  expect(confirmSpy).toHaveBeenCalledWith(expect.stringContaining("Des exécutions sont actuellement en cours"));
  expect(api.enableLocalRuntime).not.toHaveBeenCalled();

  // Confirm this time
  confirmSpy.mockReturnValue(true);
  fireEvent.click(screen.getByRole("button", { name: "Activer le mode Compagnon local" }));

  await waitFor(() => {
    expect(api.enableLocalRuntime).toHaveBeenCalledOnce();
  });

  confirmSpy.mockRestore();
});
