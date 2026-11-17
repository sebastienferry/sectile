import { invoke } from "@tauri-apps/api/core";

/** A file or folder the agent lists for an `@` mention (#917). */
export interface MentionFile { root: string; rootLabel: string; path: string; absolute: string; kind: "file" | "folder"; modified?: string }

/** The project's files and folders matching query; none when the agent cannot list them (older agent). */
export async function mentionFiles(projectId: string, runId: string | undefined, query: string): Promise<MentionFile[]> {
  try {
    const files = await invoke<MentionFile[]>("mention_files", { projectId, runId: runId ?? null, query });
    return Array.isArray(files) ? files : [];
  } catch {
    return [];
  }
}
