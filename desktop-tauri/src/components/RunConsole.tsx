import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { watchRun } from "../lib/agent";
import "@xterm/xterm/css/xterm.css";

export function RunConsole({ runId }: { runId: string }) {
  const root = useRef<HTMLDivElement>(null);
  const [retry, setRetry] = useState(0);
  const [status, setStatus] = useState("");
  useEffect(() => {
    let disposed = false;
    let stop: (() => Promise<void>) | undefined;
    const theme = () => document.documentElement.dataset.theme === "light"
      ? { background: "#ffffff", foreground: "#111315" }
      : { background: "#11151c", foreground: "#d8e0ec" };
    const terminal = new Terminal({
      disableStdin: true,
      scrollback: 2000,
      fontSize: 12,
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
    setStatus("Connexion au flux…");
    void watchRun(runId, (event) => {
      if (disposed) return;
      if (event.kind === "bytes") {
        terminal.write(new Uint8Array(event.data));
        setStatus("Flux connecté · lecture seule");
      } else if (event.kind === "error") setStatus(event.data);
      else
        setStatus("Flux fermé. L’état de l’exécution est indiqué ci-dessus.");
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
        <button onClick={() => setRetry((v) => v + 1)}>
          Reconnecter le flux
        </button>
      </div>
      <div className="terminal" ref={root} />
      <p className="muted">
        Les réponses et approbations restent dans le terminal ou l’application
        du fournisseur. Cette console ne transmet aucune saisie.
      </p>
    </section>
  );
}
