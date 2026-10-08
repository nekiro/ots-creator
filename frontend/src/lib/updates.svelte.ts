// Self-update through the Wails updater (app.Updater with the GitHub
// provider, see main.go). Progress comes from the standard updater events.
import { Events, Updater } from "@wailsio/runtime";
import { UpdateService, errorMessage, type UpdateInfo } from "./api";
import { app, toast } from "./state.svelte";

export const REPO_URL = "https://github.com/nekiro/ots-creator";

export type UpdateStage = "idle" | "downloading" | "verifying" | "installing" | "restarting";

export const updates = $state<{
  version: string;
  info: UpdateInfo | null;
  checking: boolean;
  stage: UpdateStage;
  progress: { written: number; total: number } | null;
}>({ version: "", info: null, checking: false, stage: "idle", progress: null });

/** Loads the running version and, when autoCheck is on, checks for updates in the background. */
export async function initUpdates(autoCheck: boolean): Promise<void> {
  try {
    updates.version = await UpdateService.Version();
  } catch {
    return; // not running inside Wails
  }
  Events.On(Updater.Events.DownloadStarted, () => (updates.stage = "downloading"));
  Events.On(Updater.Events.DownloadProgress, (e: { data: { written: number; total: number } }) => (updates.progress = e.data));
  Events.On(Updater.Events.Verifying, () => (updates.stage = "verifying"));
  Events.On(Updater.Events.Installing, () => (updates.stage = "installing"));
  Events.On(Updater.Events.UpdateReady, () => (updates.stage = "restarting"));
  if (updates.version === "dev" || !autoCheck) return;
  setTimeout(() => void checkForUpdates(false), 3000);
}

/** Checks GitHub. A manual check also reports "up to date" and errors. */
export async function checkForUpdates(manual: boolean): Promise<void> {
  if (updates.checking) return;
  updates.checking = true;
  try {
    const info = await UpdateService.Check();
    updates.info = info;
    if (info.available) {
      if (!app.dialog) app.dialog = "update";
    } else if (manual) {
      toast(`OTS Creator ${updates.version} is up to date.`, "success");
    }
  } catch (e) {
    if (manual) toast(`Update check failed: ${errorMessage(e)}`, "error");
  } finally {
    updates.checking = false;
  }
}

/** Downloads, verifies and swaps in the new version, then restarts. */
export async function installUpdate(): Promise<void> {
  updates.stage = "downloading";
  updates.progress = null;
  try {
    await UpdateService.Install();
  } catch (e) {
    updates.stage = "idle";
    toast(`Update failed: ${errorMessage(e)}`, "error");
  }
}
