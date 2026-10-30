import { afterEach, expect, test, vi } from "vitest";
import { act, cleanup, render } from "@testing-library/react";
import { RunConsole } from "../src/components/RunConsole";
import { watchRun, type Output } from "../src/lib/agent";
const terminal = vi.hoisted(() => ({
  loadAddon: vi.fn(),
  open: vi.fn(),
  write: vi.fn(),
  dispose: vi.fn(),
  options: { theme: {} },
}));
vi.mock("@xterm/xterm", () => ({
  Terminal: class {
    options = terminal.options;
    loadAddon = terminal.loadAddon;
    open = terminal.open;
    write = terminal.write;
    dispose = terminal.dispose;
  },
}));
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class {
    fit = vi.fn();
  },
}));
vi.mock("../src/lib/agent", () => ({ watchRun: vi.fn() }));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});
function observer() {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
}
test("a subscription resolving after unmount is still canceled", async () => {
  observer();
  let resolve!: (close: () => Promise<void>) => void;
  vi.mocked(watchRun).mockImplementation(
    () =>
      new Promise((done) => {
        resolve = done;
      }),
  );
  const view = render(<RunConsole runId="r1" />);
  view.unmount();
  const close = vi.fn().mockResolvedValue(undefined);
  await act(async () => resolve(close));
  expect(close).toHaveBeenCalledOnce();
  expect(terminal.dispose).toHaveBeenCalledOnce();
});
test("console passes byte output to the terminal and closes when changing runs", async () => {
  observer();
  const close = vi.fn().mockResolvedValue(undefined);
  let receive!: (event: Output) => void;
  vi.mocked(watchRun).mockImplementation(async (_, callback) => {
    receive = callback;
    return close;
  });
  const view = render(<RunConsole runId="r1" />);
  await act(async () => {
    await Promise.resolve();
  });
  act(() => receive({ kind: "bytes", data: [65, 66, 67] }));
  expect(terminal.write).toHaveBeenCalledWith(new Uint8Array([65, 66, 67]));
  view.rerender(<RunConsole runId="r2" />);
  await act(async () => {
    await Promise.resolve();
  });
  expect(close).toHaveBeenCalledOnce();
  expect(watchRun).toHaveBeenLastCalledWith("r2", expect.any(Function));
});

test("changing appearance updates the terminal without reconnecting", async () => {
  observer();
  vi.mocked(watchRun).mockResolvedValue(vi.fn().mockResolvedValue(undefined));
  render(<RunConsole runId="r1" />);
  await act(async () => { document.documentElement.dataset.theme = "light"; });
  expect(terminal.options.theme).toEqual({ background: "#ffffff", foreground: "#111315" });
  expect(watchRun).toHaveBeenCalledOnce();
  delete document.documentElement.dataset.theme;
});
