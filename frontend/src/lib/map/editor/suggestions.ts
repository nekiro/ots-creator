// Suggestions of the map (see MapService.Suggest) as brushes, ranked again
// by what the mapper picks this session: a pick raises the suggestion, the
// suggestions shown before it and passed over sink a little.
import { MapService, type Suggestion, type SuggestBrush } from "../data/api";
import { map, type BrushSel } from "../state.svelte";

const PICK_BONUS = 0.5;
const SKIP_PENALTY = 0.15;

const picks = new Map<string, number>();
const skips = new Map<string, number>();

export function suggestionKey(suggestion: Suggestion): string {
  return suggestion.brush ? `${suggestion.kind}:${suggestion.brush}` : `item:${suggestion.item}`;
}

/** The brush a suggestion selects: its palette brush, else the raw item. */
export function selectionOf(suggestion: Suggestion): BrushSel {
  if (suggestion.brush) {
    const kind = suggestion.kind === "decoration" ? "wall" : suggestion.kind!;
    return { kind, name: suggestion.brush, look: suggestion.item, label: suggestion.brush };
  }
  return { kind: "raw", item: suggestion.item, look: suggestion.item, label: map.items?.name(suggestion.item) ?? `item ${suggestion.item}` };
}

export function suggestionTitle(suggestion: Suggestion): string {
  const name = suggestion.brush ? `${suggestion.brush} (brush)` : (map.items?.name(suggestion.item) ?? "");
  return `${name} · ${suggestion.reason}`;
}

/** The current brush as the context of the "with" suggestions. */
export function brushContext(): SuggestBrush {
  const brush = map.brush;
  if (!brush) return { kind: "", name: "", item: 0 };
  return { kind: brush.kind, name: brush.name ?? "", item: brush.item ?? 0 };
}

export function rerank(list: Suggestion[]): Suggestion[] {
  const weight = (suggestion: Suggestion) => {
    const key = suggestionKey(suggestion);
    return (suggestion.score * (1 + PICK_BONUS * (picks.get(key) ?? 0))) / (1 + SKIP_PENALTY * (skips.get(key) ?? 0));
  };
  return list
    .map((suggestion, order) => ({ suggestion, order, weight: weight(suggestion) }))
    .sort((left, right) => right.weight - left.weight || left.order - right.order)
    .map((entry) => entry.suggestion);
}

export function notePick(list: Suggestion[], picked: Suggestion): void {
  const pickedKey = suggestionKey(picked);
  for (const suggestion of list) {
    const key = suggestionKey(suggestion);
    if (key === pickedKey) break;
    skips.set(key, (skips.get(key) ?? 0) + 1);
  }
  picks.set(pickedKey, (picks.get(pickedKey) ?? 0) + 1);
}

/** Makes a brush that scatters the suggestions, weighted by their scores. */
export async function mixBrush(mapId: number, list: Suggestion[]): Promise<BrushSel> {
  const top = Math.max(...list.map((suggestion) => suggestion.score), 0) || 1;
  const name = await MapService.MakeMix(
    mapId,
    list.map((suggestion) => ({ item: suggestion.item, weight: Math.max(1, Math.round((suggestion.score / top) * 10)) })),
  );
  const names = list.map((suggestion) => suggestion.brush ?? map.items?.name(suggestion.item) ?? suggestion.item).join(", ");
  return { kind: "doodad", name, look: list[0].item, label: `Mix: ${names}` };
}
