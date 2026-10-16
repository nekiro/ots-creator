// Pure parts of comparing two clients.
import { CATEGORY_LABELS, type Category, type DiffEntry } from "./api";

export interface MergeStep {
  category: Category;
  ids: number[];
}

/** Ids B would copy into A: everything A lacks or has differently (empty padding left out when hideEmpty). */
export function mergeIds(entries: DiffEntry[] | null | undefined, hideEmpty: boolean): number[] {
  return (entries ?? []).filter((entry) => entry.status !== "onlyA" && !(hideEmpty && entry.empty)).map((entry) => entry.id);
}

/** "12 items, 3 outfits" for a merge plan. */
export function mergeSummary(plan: MergeStep[]): string {
  return plan.map((step) => `${step.ids.length} ${CATEGORY_LABELS[step.category].toLowerCase()}`).join(", ");
}
