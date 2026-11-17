import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { invoke } from "@tauri-apps/api/core";
import ProjectPrompt from "../src/components/ProjectPrompt";
import { ContextTray, type ContextTrayItem } from "../src/components/ContextTray";
import * as folders from "../src/lib/folders";
vi.mock("@tauri-apps/api/core", () => ({ invoke: vi.fn() }));
vi.mock("../src/lib/folders", () => ({ projectFolders: vi.fn(), attachFolder: vi.fn(), detachFolder: vi.fn() }));

// jsdom lays nothing out: a pill's row is read from its index, `perRow` pills per row.
let perRow = 2;
const observers: ResizeObserverCallback[] = [];
beforeEach(() => {
  vi.resetAllMocks();
  perRow = 2;
  observers.length = 0;
  vi.mocked(invoke).mockResolvedValue(null);
  Object.defineProperty(HTMLElement.prototype, "offsetTop", {
    configurable: true,
    get(this: HTMLElement) {
      if (!this.classList.contains("context-tray-pill")) return 0;
      return Math.floor(Array.from(this.parentElement!.children).indexOf(this) / perRow) * 24;
    },
  });
  vi.stubGlobal("ResizeObserver", class { constructor(callback: ResizeObserverCallback) { observers.push(callback); } observe() {} unobserve() {} disconnect() {} });
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); delete (HTMLElement.prototype as { offsetTop?: number }).offsetTop; });

const items = (...names: string[]): ContextTrayItem[] => names.map(name => ({ key: `/${name}`, label: name, title: `Folder · /${name}`, icon: "folder" }));
const hiddenPills = () => screen.getAllByRole("button", { hidden: true }).filter(button => button.getAttribute("aria-hidden") === "true").map(button => button.textContent);

test("pills past the first row hide behind +N more until Show less", () => {
  const view = render(<ContextTray items={items("a", "b", "c", "d", "e")} />);
  const toggle = screen.getByRole("button", { name: "+3 more" });
  expect(hiddenPills()).toEqual(["c", "d", "e"]);
  fireEvent.click(toggle);
  expect(screen.getByRole("button", { name: "Show less" }).getAttribute("aria-expanded")).toBe("true");
  expect(hiddenPills()).toEqual([]);
  // A different set of items starts collapsed again.
  view.rerender(<ContextTray items={items("a", "b", "c")} />);
  expect(screen.getByRole("button", { name: "+1 more" })).toBeTruthy();
});

test("a resize that fits every pill drops the toggle", () => {
  render(<ContextTray items={items("a", "b", "c")} />);
  expect(screen.getByRole("button", { name: "+1 more" })).toBeTruthy();
  perRow = 10;
  act(() => observers.forEach(callback => callback([], {} as ResizeObserver)));
  expect(screen.queryByRole("button", { name: /more/ })).toBeNull();
});

test("the new prompt composer shows its folders as pills that copy their path, not the working folder alone", async () => {
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  vi.mocked(folders.projectFolders).mockResolvedValue([]);
  const view = render(<ProjectPrompt projectId="p" directory="/work/app" onCreated={vi.fn()} />);
  await act(async () => {});
  expect(screen.queryByRole("group", { name: "Message context" })).toBeNull();
  view.unmount();

  vi.mocked(folders.projectFolders).mockResolvedValue([{ path: "/notes/design", kind: "folder" }]);
  render(<ProjectPrompt projectId="p" directory="/work/app" onCreated={vi.fn()} />);
  const tray = await screen.findByRole("group", { name: "Message context" });
  expect(tray.closest(".composer-box")).toBeTruthy();
  expect(within(tray).getAllByRole("button").map(button => button.textContent)).toEqual(["app", "design"]);
  fireEvent.click(within(tray).getByRole("button", { name: "Attached to project · /notes/design · click to copy" }));
  expect(writeText).toHaveBeenCalledWith("/notes/design");
});
