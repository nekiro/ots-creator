<script lang="ts">
  // Warnings from loading the active map, grouped by what went wrong
  // ("unknown attribute 0") with the places it happened; a place goes there.
  import { goTo, mapCommands } from "../../../lib/map/view/commands";
  import { map } from "../../../lib/map/state.svelte";
  import Dialog from "../../../lib/ui/Dialog.svelte";

  type Place = { text: string; pos: { x: number; y: number; z: number } | null };
  const list = $derived(map.activeId !== null ? (map.loadWarnings[map.activeId] ?? []) : []);

  // "item 939 at 31934,31045,7: unknown attribute 0" -> problem "unknown
  // attribute 0", place "item 939 at 31934,31045,7".
  // Only the first 20 places of a problem are listed; "N more not listed:
  // problem" counts the rest.
  const hidden = $derived.by(() => {
    const out = new Map<string, number>();
    for (const w of list) {
      const m = w.match(/^(\d+) more not listed: (.*)$/);
      if (m) out.set(m[2], (out.get(m[2]) ?? 0) + Number(m[1]));
    }
    return out;
  });
  const total = $derived(list.length + [...hidden.values()].reduce((a, b) => a + b - 1, 0));

  const groups = $derived.by(() => {
    const out = new Map<string, Place[]>();
    for (const w of list) {
      if (/^\d+ more not listed: /.test(w)) continue;
      const i = w.lastIndexOf(": ");
      const problem = i > 0 ? w.slice(i + 2) : w;
      const where = i > 0 ? w.slice(0, i) : "";
      const m = where.match(/(\d+),\s*(\d+),\s*(\d+)/);
      const g = out.get(problem) ?? [];
      g.push({ text: where, pos: m ? { x: +m[1], y: +m[2], z: +m[3] } : null });
      out.set(problem, g);
    }
    for (const k of hidden.keys()) if (!out.has(k)) out.set(k, []);
    const count = (k: string, l: Place[]) => l.length + (hidden.get(k) ?? 0);
    return [...out].sort((a, b) => count(b[0], b[1]) - count(a[0], a[1]));
  });
  let open = $state<Record<string, boolean>>({});
  // Places drawn per problem; thousands of rows at once would stall.
  const PAGE = 300;
  let shown = $state<Record<string, number>>({});

  function go(p: Place) {
    if (!p.pos) return;
    goTo(p.pos);
    mapCommands.inspect(p.pos);
  }

  function copyAll() {
    void navigator.clipboard.writeText(list.join("\n"));
  }
</script>

<Dialog title="Load warnings · {map.active?.name ?? ''}" width={620} onclose={() => (map.dialog = null)}>
  {#if !list.length}
    <p class="t-label">This map loaded without warnings.</p>
  {:else}
    <p class="t-label hint">{total.toLocaleString()} warning{total > 1 ? "s" : ""}. The map opened; the listed things were skipped or kept as they were. Click a place to go there.</p>
    <div class="t-panel body">
      {#each groups as [problem, places] (problem)}
        <button class="m-row grp" onclick={() => (open[problem] = !open[problem])}>
          <span class="chev">{open[problem] ? "▾" : "▸"}</span>
          <span class="m-name">{problem}</span>
          <span class="m-meta">{(places.length + (hidden.get(problem) ?? 0)).toLocaleString()}</span>
        </button>
        {#if open[problem]}
          {#each places.slice(0, shown[problem] ?? PAGE) as p}
            <button class="m-row place" disabled={!p.pos} title={p.pos ? "Go there" : ""} onclick={() => go(p)}>
              <span class="m-name">{p.text || problem}</span>
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
