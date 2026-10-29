import fixture from "../../tests/fixtures/ux-baseline.json";

type InvokeArgs = Record<string, unknown>;
type TauriWindow = Window & {
  __TAURI_INTERNALS__?: { invoke?: (command: string, args?: InvokeArgs) => Promise<unknown> };
  isTauri?: boolean;
};

const projects = [fixture.project];
const runs = fixture.runs;
const conversations: Record<string, unknown> = fixture.conversations;

function boardTasks() {
  if (new URLSearchParams(window.location.search).get("fixture") !== "large-board") return fixture.tasks;
  return Array.from({length:1988},(_,index)=>{
    const template=fixture.tasks[index%fixture.tasks.length];
    return {...template,id:`task-${2605+index}`,key:`PE-${2605+index}`,title:`${template.title} ${index+1}`,
      trackerStatus:index<1400?"Open":index<1800?"In Progress":"Done"};
  });
}

export function installUXFixture() {
  const target = window as TauriWindow;
  target.isTauri = true;
  target.__TAURI_INTERNALS__ = {
    invoke: async (command, args = {}) => {
      switch (command) {
        case "list_projects":
          return projects;
        case "list_tasks":
          return boardTasks();
        case "execution_snapshot":
          return {
            status: { connected: true, runtimeMode: "local", capabilities: ["orchestration-agents"] },
            runs,
          };
        case "project_context":
          return fixture.projectContext;
        case "project_skills":
          return fixture.projectContext.skills;
        case "read_conversation":
          return conversations[String(args.runId)] ?? Promise.reject(new Error("Unknown fixture conversation"));
        case "local_setup_status":
          return fixture.localSetup;
        case "conversation_profiles":
          return { profiles: [{ id: "profile-default", name: "Default Codex", provider: "codex", model: "gpt-5.4-codex", mode: "execute" }], defaultId: "profile-default" };
        case "provider_status":
          return { installed: true, path: `/fixture/bin/${String(args.provider)}`, authentication: "connected" };
        case "provider_models":
          return String(args.provider) === "codex" ? ["gpt-5.4-codex", "gpt-5.3-codex"] : ["claude-sonnet-4-5", "claude-opus-4-1"];
        case "run_result":
          return fixture.runResult;
        case "run_diff":
          return fixture.diff;
        case "provider_readiness":
          return fixture.providerReadiness;
        case "orchestration":
          return fixture.orchestration;
        case "project_repositories":
          return [];
        case "oauth_apps":
          return { github: { configured: true }, gitlab: { configured: false }, jira: { configured: true, redirectUrl: "http://127.0.0.1:52963/auth/jira/callback", expectedRedirectUrl: "http://127.0.0.1:52963/auth/jira/callback" } };
        case "watch_run": {
          const channel = args.output as { onmessage?: (event: unknown) => void } | undefined;
          queueMicrotask(() => channel?.onmessage?.({ kind: "bytes", data: Array.from(new TextEncoder().encode("Fixture console output\\r\\n")) }));
          return null;
        }
        case "unwatch_run":
        case "control_conversation":
        case "rename_conversation":
        case "open_external_link":
        case "editor_preference":
        case "open_run_editor":
          return null;
        default:
          throw new Error(`UX fixture blocks unsupported command: ${command}`);
      }
    },
  };
  console.info("Sectile UX fixture enabled. All desktop commands are isolated and read-only.");
}
