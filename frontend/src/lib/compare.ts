import { CATEGORY_LABELS, type Category, type DiffEntry } from "./api";

export interface MergeStep {
  category: Category;
  ids: number[];
}

export function mergeIds(entries: DiffEntry[] | null | undefined, hideEmpty: boolean): number[] {
  return (entries ?? []).filter((entry) => entry.status !== "onlyA" && !(hideEmpty && entry.empty)).map((entry) => entry.id);
}

export function mergeSummary(plan: MergeStep[]): string {
  return plan.map((step) => `${step.ids.length} ${CATEGORY_LABELS[step.category].toLowerCase()}`).join(", ");
}
