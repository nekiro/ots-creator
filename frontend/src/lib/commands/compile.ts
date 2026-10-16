// Messages after opening and compiling a client, and the sprite
// optimization offered before a compile.
import { SpriteService, formatSize, type CompileStats, type State } from "../api";
import { choose } from "../confirm.svelte";
import { app, run, toast } from "../state/app.svelte";
import { savedNote } from "./shared";

/** " Removed 3 unused sheets (1.2 MB freed)." after an asset compilation. */
export function sheetsNote(stats: CompileStats): string {
  if (!stats.sheetsRemoved) return "";
  const freed = stats.freed > 0 ? ` (${formatSize(stats.freed)} freed)` : "";
  return ` Removed ${stats.sheetsRemoved.toLocaleString()} unused sheet${stats.sheetsRemoved === 1 ? "" : "s"}${freed}.`;
}

/** " Updated items.otb (2 moved, 1 reloaded)." after a compile; problems
 * with the items.otb get their own toast (the client was compiled anyway). */
export function otbNote(stats: CompileStats): string {
  const otb = stats.otb;
  if (!otb) return "";
  if (otb.error) {
    toast(`items.otb was not updated: ${otb.error}`, "error");
    return "";
  }
  if (otb.missing) toast(`${otb.missing} server item(s) in items.otb point to client items that no longer exist.`);
  if (!otb.written) return "";
  const parts = [otb.remapped && `${otb.remapped} moved`, otb.updated && `${otb.updated} reloaded`, otb.added && `${otb.added} new`].filter(Boolean);
  return ` Updated items.otb${parts.length ? ` (${parts.join(", ")})` : ""}.`;
}

/** After opening a client: the remembered items.otb is optional, so a
 * failure only shows a toast. */
export function openedNote(state: State | undefined) {
  if (state?.otbError) toast(`items.otb was not loaded: ${state.otbError}`, "error");
}

/**
 * Before a compile: when sprites can be optimized, asks whether to do it
 * first. False when the user cancelled the compile.
 */
export async function offerOptimize(): Promise<boolean> {
  const options = { duplicates: true, empty: true, unused: true };
  const plan = await run("Checking sprites", () => SpriteService.Analyze(options), { modal: true });
  if (!plan || plan.before === plan.after) return true;
  const parts = [
    plan.duplicates && `${plan.duplicates.toLocaleString()} duplicate`,
    plan.empty && `${plan.empty.toLocaleString()} empty`,
    plan.unused && `${plan.unused.toLocaleString()} unused`,
  ].filter(Boolean);
  const choice = await choose({
    title: "Optimize sprites",
    message: `${(plan.before - plan.after).toLocaleString()} sprites can be removed (${parts.join(", ")}): ${plan.before.toLocaleString()} → ${plan.after.toLocaleString()}${savedNote(plan.bytes)}.
Optimize them before compiling? Undo reverts it.`,
    ok: "Optimize",
    alt: "Compile as is",
  });
  if (choice === null) return false;
  if (choice === "alt") return true;
  const done = await run("Optimizing sprites", () => SpriteService.Optimize(options), { modal: true });
  if (!done) return false;
  app.selectedSprites = [];
  toast(`Optimized sprites: ${done.before.toLocaleString()} → ${done.after.toLocaleString()}${savedNote(done.bytes)}.`, "success");
  return true;
}
