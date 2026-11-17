import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SidebarHelp from "../src/components/SidebarHelpDialog";

beforeEach(() => {
  // jsdom does not implement native modal focus containment; verify it in a browser.
  vi.spyOn(HTMLDialogElement.prototype, "showModal").mockImplementation(function (this: HTMLDialogElement) { this.setAttribute("open", ""); });
  vi.spyOn(HTMLDialogElement.prototype, "close").mockImplementation(function (this: HTMLDialogElement) { this.removeAttribute("open"); });
});
afterEach(() => cleanup());

test("keyboard activation exposes every contextual symbol and focuses Close", async () => {
  const user = userEvent.setup();
  render(<SidebarHelp />);
  await user.tab();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Sidebar status help" }));
  await user.keyboard("{Enter}");
  const dialog = screen.getByRole("dialog", { name: "Sidebar status help" });
  expect(document.activeElement).toBe(within(dialog).getByRole("button", { name: "Close" }));
  expect(dialog.querySelectorAll("li").length).toBe(11);
  for (const kind of ["working", "waiting", "done", "verify", "failed", "stopping", "stopped"]) {
    expect(dialog.querySelector(`.skill-status-mark.skill-${kind}`)?.getAttribute("aria-hidden")).toBe("true");
  }
  for (const kind of ["live", "session", "needs-input", "idle"]) {
    expect(dialog.querySelector(`.conversation-indicator.${kind}`)?.getAttribute("aria-hidden")).toBe("true");
  }
  expect(within(dialog).getByText(/whole ticket is not necessarily finished/)).toBeTruthy();
  expect(within(dialog).getByText(/pulse alone does not prove work/)).toBeTruthy();
  expect(within(dialog).getByText(/question or approval in the console/)).toBeTruthy();
  expect(within(dialog).getByText(/Check the result and ticket/)).toBeTruthy();
  await user.click(within(dialog).getByRole("button", { name: "Close" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Sidebar status help" }));
  await user.keyboard(" ");
  expect(screen.getByRole("dialog")).toBeTruthy();
});

test("native Escape cancellation closes help and restores the trigger", () => {
  render(<SidebarHelp />);
  const trigger = screen.getByRole("button", { name: "Sidebar status help" });
  fireEvent.click(trigger);
  fireEvent(screen.getByRole("dialog"), new Event("cancel", { cancelable: true }));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
});

test("sidebar removal closes the modal and focuses the remaining Settings control", async () => {
  const view = render(<><button id="settings">Settings</button><SidebarHelp /></>);
  fireEvent.click(screen.getByRole("button", { name: "Sidebar status help" }));
  view.rerender(<button id="settings">Settings</button>);
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Settings" }));
});
