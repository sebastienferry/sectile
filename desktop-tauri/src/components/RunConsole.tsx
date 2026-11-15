import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { detachRunTerminal, watchRun } from "../lib/agent";
import "@xterm/xterm/css/xterm.css";

export function RunConsole({ runId, sessionId }: { runId: string; sessionId?: string }) {
  const root = useRef<HTMLDivElement>(null);
  const [retry, setRetry] = useState(0);
  const [status, setStatus] = useState("");
  const [detaching, setDetaching] = useState(false);
  useEffect(() => {
    let disposed = false;
    let stop: (() => Promise<void>) | undefined;
    const theme = () => document.documentElement.dataset.theme === "light"
      ? { background: "#ffffff", foreground: "#111315" }
      : { background: "#11151c", foreground: "#d8e0ec" };
    const terminal = new Terminal({
      disableStdin: true,
      scrollback: 2000,
      fontSize: 13,
      theme: theme(),
    });
    const appearance = new MutationObserver(() => {
      terminal.options.theme = theme();
    });
    appearance.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    const fit = new FitAddon();
    terminal.loadAddon(fit);
    terminal.open(root.current!);
    fit.fit();
    const observer = new ResizeObserver(() => {
      if (!disposed) fit.fit();
    });
    observer.observe(root.current!);
    setStatus("Connecting to stream…");
    void watchRun(runId, (event) => {
      if (disposed) return;
      if (event.kind === "bytes") {
        terminal.write(new Uint8Array(event.data));
        setStatus("Stream connected · read-only");
      } else if (event.kind === "error") setStatus(event.data);
      else
        setStatus("Stream closed. Execution status is shown above.");
    })
      .then((close) => {
        if (disposed) void close().catch(() => {});
        else stop = close;
      })
      .catch((error) => {
        if (!disposed) setStatus(String(error));
      });
    return () => {
      disposed = true;
      observer.disconnect();
      appearance.disconnect();
      terminal.dispose();
      if (stop) void stop().catch(() => {});
    };
  }, [runId, retry]);
  return (
    <section className="console-panel">
      <div className="console-toolbar">
        <span role="status">{status}</span>
        <div className="toolbar-actions">
          {sessionId && (
            <button
              type="button"
              disabled={detaching}
              onClick={() => {
                setDetaching(true);
                void detachRunTerminal(runId)
                  .catch(() => setStatus("Unable to open external terminal."))
                  .finally(() => setDetaching(false));
              }}
            >
              Open in terminal
            </button>
          )}
          <button onClick={() => setRetry((v) => v + 1)}>
            Reconnect stream
          </button>
        </div>
      </div>
      <div className="terminal" ref={root} />
      <p className="muted">
        Answers and approvals stay in the provider's terminal or app. This
        console does not forward input.
      </p>
    </section>
  );
}
