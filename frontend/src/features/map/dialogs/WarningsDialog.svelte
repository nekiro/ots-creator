<script lang="ts">
  import { goTo, mapCommands } from "../../../lib/map/view/commands";
  import { map } from "../../../lib/map/state.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";
  import { plural } from "../../../lib/util/format";

  type Place = { text: string; pos: { x: number; y: number; z: number } | null };
  const list = $derived(map.activeId !== null ? (map.loadWarnings[map.activeId] ?? []) : []);

  // "item 939 at 31934,31045,7: unknown attribute 0" -> problem "unknown
  // attribute 0", place "item 939 at 31934,31045,7".
  // Only the first 20 places of a problem are listed; "N more not listed:
  // problem" counts the rest.
  const hidden = $derived.by(() => {
    const out = new Map<string, number>();
    for (const warning of list) {
      const match = warning.match(/^(\d+) more not listed: (.*)$/);
      if (match) out.set(match[2], (out.get(match[2]) ?? 0) + Number(match[1]));
    }
    return out;
  });
  const total = $derived(list.length + [...hidden.values()].reduce((sum, more) => sum + more - 1, 0));

  const groups = $derived.by(() => {
    const out = new Map<string, Place[]>();
    for (const warning of list) {
      if (/^\d+ more not listed: /.test(warning)) continue;
      const separator = warning.lastIndexOf(": ");
      const problem = separator > 0 ? warning.slice(separator + 2) : warning;
      const where = separator > 0 ? warning.slice(0, separator) : "";
      const match = where.match(/(\d+),\s*(\d+),\s*(\d+)/);
      const places = out.get(problem) ?? [];
      places.push({ text: where, pos: match ? { x: +match[1], y: +match[2], z: +match[3] } : null });
      out.set(problem, places);
    }
    for (const problem of hidden.keys()) if (!out.has(problem)) out.set(problem, []);
    const count = (problem: string, places: Place[]) => places.length + (hidden.get(problem) ?? 0);
    return [...out].sort((first, second) => count(second[0], second[1]) - count(first[0], first[1]));
  });
  let open = $state<Record<string, boolean>>({});
  // Places drawn per problem; thousands of rows at once would stall.
  const PAGE = 300;
  let shown = $state<Record<string, number>>({});

  function go(place: Place) {
    if (!place.pos) return;
    goTo(place.pos);
    mapCommands.inspect(place.pos);
  }

  function copyAll() {
    void navigator.clipboard.writeText(list.join("\n"));
  }
</script>

<Dialog title="Load warnings · {map.active?.name ?? ''}" width={620} onclose={() => (map.dialog = null)}>
  {#if !list.length}
    <p class="t-label">This map loaded without warnings.</p>
  {:else}
    <p class="t-label hint">{plural(total, "warning")}. The map opened; the listed things were skipped or kept as they were. Click a place to go there.</p>
    <div class="t-panel body">
      {#each groups as [problem, places] (problem)}
        <button class="m-row grp" onclick={() => (open[problem] = !open[problem])}>
          <span class="chev">{open[problem] ? "▾" : "▸"}</span>
          <span class="m-name">{problem}</span>
          <span class="m-meta">{(places.length + (hidden.get(problem) ?? 0)).toLocaleString()}</span>
        </button>
        {#if open[problem]}
          {#each places.slice(0, shown[problem] ?? PAGE) as place}
            <button class="m-row place" disabled={!place.pos} title={place.pos ? "Go there" : ""} onclick={() => go(place)}>
              <span class="m-name">{place.text || problem}</span>
            </button>
          {/each}
          {#if places.length > (shown[problem] ?? PAGE)}
            <button class="m-row place showmore" onclick={() => (shown[problem] = (shown[problem] ?? PAGE) + PAGE)}>
              Show {Math.min(PAGE, places.length - (shown[problem] ?? PAGE)).toLocaleString()} more of {(places.length - (shown[problem] ?? PAGE)).toLocaleString()}
            </button>
          {/if}
          {#if hidden.get(problem)}
            <div class="m-row place more">…and {hidden.get(problem)!.toLocaleString()} more places not listed</div>
          {/if}
        {/if}
      {/each}
    </div>
  {/if}
  {#snippet footer()}
    {#if list.length}<button class="t-btn" onclick={copyAll}>Copy all</button><span class="grow"></span>{/if}
    <button class="t-btn" onclick={() => (map.dialog = null)}>Close</button>
  {/snippet}
</Dialog>

<style>
  .hint {
    font-size: 12px;
    margin: 0 0 6px;
  }
  .body {
    height: 360px;
    overflow-y: auto;
    padding: 2px;
  }
  .chev {
    width: 14px;
    flex: none;
    color: var(--text-dim);
  }
  .place {
    padding-left: 22px;
    color: var(--text-dim);
  }
  .place:hover:not(:disabled) {
    color: var(--text-bright);
  }
  .showmore {
    color: var(--gold);
  }
  .more {
    cursor: default;
    font-style: italic;
  }
  .grow {
    flex: 1;
  }
</style>
