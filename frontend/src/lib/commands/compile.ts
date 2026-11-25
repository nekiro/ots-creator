import { SpriteService, type CompileStats, type State } from "../api";
import { choose } from "../confirm.svelte";
import { app, run, toast } from "../state/app.svelte";
import { formatSize, plural } from "../util/format";
import { savedNote } from "./shared";

export function sheetsNote(stats: CompileStats): string {
	if (!stats.sheetsRemoved) return "";
	const freed = stats.freed > 0 ? ` (${formatSize(stats.freed)} freed)` : "";
	return ` Removed ${plural(stats.sheetsRemoved, "unused sheet")}${freed}.`;
}

export function otbNote(stats: CompileStats): string {
	const otb = stats.otb;
	if (!otb) return "";
	if (otb.error) {
		toast(`items.otb was not updated: ${otb.error}`, "error");
		return "";
	}
	if (otb.missing) toast(`${otb.missing} server item(s) in items.otb point to client items that no longer exist.`);
	if (!otb.written) return "";
	const parts = [
		otb.remapped && `${otb.remapped} moved`,
		otb.updated && `${otb.updated} reloaded`,
		otb.added && `${otb.added} new`,
	].filter(Boolean);
	return ` Updated items.otb${parts.length ? ` (${parts.join(", ")})` : ""}.`;
}

export function xmlNote(stats: CompileStats): string {
	const xml = stats.xml;
	if (!xml) return "";
	if (xml.error) {
		toast(`items.xml was not updated: ${xml.error}`, "error");
		return "";
	}
	if (xml.skipped)
		toast(
			`${xml.skipped} edited items.xml ${xml.skipped === 1 ? "entry" : "entries"} not written: the items.otb gives the item no server id.`,
		);
	if (!xml.written) return "";
	return ` Updated items.xml${xmlParts(xml)}.`;
}

/** What compiling changes in the items.xml, as " (2 edited, 1 new)"; "" for nothing. */
export function xmlParts(xml: NonNullable<CompileStats["xml"]>): string {
	const parts = [
		xml.edited && `${xml.edited} edited`,
		xml.added && `${xml.added} new`,
		xml.updated && `${xml.updated} with changed attributes`,
		xml.moved && `${xml.moved} moved`,
	].filter(Boolean);
	return parts.length ? ` (${parts.join(", ")})` : "";
}

export function openedNote(state: State | undefined) {
	if (state?.otbError) toast(`items.otb was not loaded: ${state.otbError}`, "error");
	if (state?.xmlError) toast(`items.xml was not loaded: ${state.xmlError}`, "error");
}

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
	toast(
		`Optimized sprites: ${done.before.toLocaleString()} → ${done.after.toLocaleString()}${savedNote(done.bytes)}.`,
		"success",
	);
	return true;
}
