import { invoke } from "@tauri-apps/api/core";
export interface LocalSetup {
  enabled: boolean;
  mode?: "unconfigured" | "local" | "fleet";
  fleetServer?: string;
  backendError?: string;
  updatePreparing?: boolean;
  ready?: boolean;
  signedIn?: boolean;
  agentReady?: boolean;
  apiSetupAvailable?: boolean;
  dataDirectory?: string;
  profile?: { email: string; displayName: string; role?: string };
  trackers?: { id: string; name: string; provider: string; site: string; scope: string }[];
  credentials?: { credentials: { tracker: string; siteUrl?: string; account?: string; kind?: string; disconnected?: boolean }[] };
  projects?: { id: string; name: string; trackers?: { trackerId: string }[] }[];
}
export const localSetupStatus = () => invoke<LocalSetup>("local_setup_status");
export interface ServiceLog { path: string; text: string; missing: boolean; truncated: boolean }
export const localServiceLog = (service: "companion" | "agent") => invoke<ServiceLog>("local_service_log", { service });
export const prepareLocalUpdate = () => invoke<void>("prepare_local_update");
export const enableLocalRuntime = () => invoke<void>("enable_local_runtime");
export const attachFleetAgent = () => invoke<void>("attach_fleet_agent");
export const pairFleetAgent = (serverUrl: string, pairingCode: string) => invoke<void>("pair_fleet_agent", { serverUrl, pairingCode });
export const saveLocalProfile = (email: string, displayName: string) => invoke<void>("save_local_profile", { email, displayName });
export const saveLocalCredential = (input: { tracker: string; siteUrl: string; email: string; token: string }) => invoke<void>("save_local_credential", { input });
export const createLocalTracker = (input: { name: string; provider: string; site: string; scope: string }) => invoke("create_local_tracker", { input });
export const updateLocalTracker = (trackerId: string, input: { name: string; provider: string; site: string; scope: string }) => invoke("update_local_tracker", { trackerId, input });
export const deleteLocalTracker = (trackerId: string) => invoke("delete_local_tracker", { trackerId });
export const createLocalProject = (name: string, trackerId: string) => invoke("create_local_project", { name, trackerId });
export const attachProjectTracker = (projectId: string, trackerId: string) => invoke("attach_project_tracker", { projectId, trackerId });
export const chooseProjectFolder = (projectId: string) => invoke<string | null>("choose_project_folder", { projectId });
export interface TrackerSyncActivity { id: string; trackerId?: string; status: string; summary?: string; error?: string }
export const syncLocalTracker = (trackerId: string) => invoke<{queued:boolean;activity:TrackerSyncActivity}>("sync_local_tracker", { trackerId });
export const trackerSyncStatus = (activityId: string) => invoke<TrackerSyncActivity>("tracker_sync_status", { activityId });
export const activeTrackerSyncs = () => invoke<TrackerSyncActivity[]>("active_tracker_syncs");
export interface OAuthApp { configured: boolean; clientIdSet?: boolean; secretSet?: boolean; redirectUrl?: string; expectedRedirectUrl: string }
export const oauthApps = () => invoke<Record<string, OAuthApp>>("oauth_apps");
export const saveOAuthApp = (provider: string, clientId: string, clientSecret: string) => invoke<void>("save_oauth_app", { provider, clientId, clientSecret });
export const beginOAuth = (provider: string) => invoke<string>("begin_oauth", { provider });
export const oauthStatus = (id: string) => invoke<{ status: string }>("oauth_status", { id });
export const cancelOAuth = (id: string) => invoke<void>("cancel_oauth", { id });
