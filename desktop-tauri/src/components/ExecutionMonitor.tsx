import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import type { Snapshot } from "../domain/execution";
import { executionSnapshot } from "../lib/agent";
const Context = createContext<ReturnType<typeof useMonitor> | null>(null);
function useMonitor() {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [fresh, setFresh] = useState(false);
  const [pollError, setPollError] = useState("");
  const [lastRead, setLastRead] = useState("");
  const refreshRef = useRef<() => Promise<void>>(async () => {});
  useEffect(() => {
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    let staleTimer: ReturnType<typeof setTimeout>;
    let inFlight: Promise<void> | null = null;
    const poll = (): Promise<void> => {
      if (inFlight) return inFlight;
      clearTimeout(timer);
      inFlight = (async () => {
        try {
          const next = await executionSnapshot();
          if (disposed) return;
          clearTimeout(staleTimer);
          staleTimer = setTimeout(() => {
            if (!disposed) {
              setFresh(false);
              setPollError(
                "Les dernières données datent de plus de 10 secondes",
              );
            }
          }, 10000);
          setSnapshot(next);
          setFresh(true);
          setPollError("");
          setLastRead(new Date().toLocaleTimeString("fr-FR"));
        } catch (error) {
          if (!disposed) {
            setFresh(false);
            setPollError(String(error));
          }
        } finally {
          inFlight = null;
          if (!disposed) timer = setTimeout(() => void poll(), 3000);
        }
      })();
      return inFlight;
    };
    refreshRef.current = poll;
    void poll();
    return () => {
      disposed = true;
      clearTimeout(timer);
      clearTimeout(staleTimer);
    };
  }, []);

  return {
    snapshot,
    fresh,
    pollError,
    lastRead,
    refresh: () => refreshRef.current(),
  };
}
export function ExecutionProvider({ children }: { children: ReactNode }) {
  const monitor = useMonitor();
  return <Context.Provider value={monitor}>{children}</Context.Provider>;
}
export function useExecutionMonitor() {
  const value = useContext(Context);
  if (!value) throw new Error("ExecutionProvider is required");
  return value;
}
