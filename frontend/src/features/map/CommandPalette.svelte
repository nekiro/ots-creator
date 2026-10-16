<script lang="ts">
  // Ctrl+K: type a position, a place, a brush, an item, a creature or a
  // command, and press Enter.
  import { tick } from "svelte";
  import { parsePos, fmtPos, type CreatureType, type PaletteBrush } from "../../lib/map/api";
  import { goTo, mapCommands } from "../../lib/map/commands";
  import { mapMenus } from "../../lib/map/menus";
  import { map, selectBrush } from "../../lib/map/state.svelte";
  import { goHouse, houses, towns } from "../../lib/map/world";
  import { backdrop, pop } from "../../lib/ui/motion";
  import ItemIcon from "./ItemIcon.svelte";

  interface Entry {
    group: string;
    label: string;
    detail?: string;
    keys?: string;
    icon?: { id?: number; outfit?: number };
    run: () => unknown;
  }

  let text = $state(map.command ?? "");
  let active = $state(0);
  let input: HTMLInputElement;
  let list: HTMLDivElement;

  $effect(() => {
    void tick().then(() => input?.focus());
  });

  function close() {
    map.command = null;
  }

  const commands = $derived.by<Entry[]>(() => {
    const out: Entry[] = [];
    for (const menu of mapMenus()) {
      for (const e of menu.items) {
        if (e === "-" || e.disabled?.()) continue;
        out.push({ group: "Commands", label: e.label.replace(/…$/, ""), detail: menu.label, keys: e.keys, run: e.action });
      }
    }
    out.push({ group: "Commands", label: "Editor settings", detail: "Settings", run: () => (map.dialog = "settings") });
    return out;
  });

  const q = $derived(text.trim().toLowerCase());
  const entries = $derived.by<Entry[]>(() => {
    const out: Entry[] = [];
    const has = map.activeId !== null;
    const p = has ? parsePos(text) : null;
    if (p) out.push({ group: "Go to", label: `Go to ${fmtPos(p)}`, run: () => (goTo(p), mapCommands.inspect(p)) });
    if (!q) return [...out, ...commands.slice(0, 40)];
    const match = (s: string) => s.toLowerCase().includes(q);
    if (has) {
      for (const t of towns()) if (match(t.name)) out.push({ group: "Places", label: t.name, detail: "town", run: () => (mapCommands.focus({ kind: "town", id: t.id }), t.temple.x && goTo(t.temple)) });
      for (const h of houses()) if (match(h.name)) out.push({ group: "Places", label: h.name, detail: "house", run: () => (mapCommands.focus({ kind: "house", id: h.id }), goHouse(h)) });
      for (const w of map.active?.waypoints ?? []) if (w && match(w.name)) out.push({ group: "Places", label: w.name, detail: "waypoint", run: () => (mapCommands.focus({ kind: "waypoint", name: w.name }), goTo(w.pos)) });
    }
    const brushes = (map.palette?.brushes ?? []).filter((b): b is PaletteBrush => !!b && match(b.name)).slice(0, 30);
    for (const b of brushes) out.push({ group: "Brushes", label: b.name, detail: b.kind, icon: { id: b.look }, run: () => selectBrush({ kind: b.kind === "decoration" ? "wall" : b.kind, name: b.name, look: b.look, label: b.name }) });
    for (const c of (map.palette?.creatures ?? []).filter((c): c is CreatureType => !!c && match(c.name)).slice(0, 20)) {
      out.push({ group: "Creatures", label: c.name, detail: c.npc ? "NPC" : "monster", icon: { outfit: c.outfit.lookItem ? 0 : c.outfit.lookType, id: c.outfit.lookItem }, run: () => selectBrush({ kind: "creature", creature: c.name, npc: !!c.npc, label: c.name }) });
    }
    for (const id of map.items?.search(q, 30) ?? []) {
      const name = map.items!.name(id);
      out.push({ group: "Items", label: name, detail: String(id), icon: { id }, run: () => selectBrush({ kind: "raw", item: id, look: id, label: name }) });
    }
    for (const c of commands) if (match(c.label) || match(c.detail ?? "")) out.push(c);
    return out.slice(0, 120);
  });

  $effect(() => {
    void entries;
    active = 0;
  });

  function run(e: Entry | undefined) {
    if (!e) return;
    close();
    void Promise.resolve(e.run());
  }

  function onkey(event: KeyboardEvent) {
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      close();
    } else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const n = entries.length;
      if (!n) return;
      active = (active + (event.key === "ArrowDown" ? 1 : n - 1)) % n;
      void tick().then(() => list?.querySelector(".on")?.scrollIntoView({ block: "nearest" }));
    } else if (event.key === "Enter") {
      event.preventDefault();
      run(entries[active]);
    }
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="overlay" transition:backdrop|global onmousedown={(event) => event.target === event.currentTarget && close()}>
  <div class="box t-window" transition:pop|global role="dialog" aria-label="Command palette">
    <input class="t-input big" bind:this={input} bind:value={text} placeholder="Position, place, brush, item, creature or command…" onkeydown={onkey} spellcheck="false" />
    <div class="list" bind:this={list}>
      {#each entries as e, i}
        {#if i === 0 || entries[i - 1].group !== e.group}<div class="m-sect">{e.group}</div>{/if}
        <button class="m-row" class:on={i === active} onmousemove={() => (active = i)} onclick={() => run(e)}>
          {#if e.icon}<span class="t-slot sm"><ItemIcon id={e.icon.id ?? 0} outfit={e.icon.outfit ?? 0} size={24} /></span>{/if}
          <span class="m-name">{e.label}</span>
          {#if e.detail}<span class="m-meta">{e.detail}</span>{/if}
          {#if e.keys}<kbd>{e.keys}</kbd>{/if}
        </button>
      {:else}
        <div class="m-empty">Nothing matches. Positions like 1000 1000 7 or {"{x = 1000, y = 1000, z = 7}"} work too.</div>
      {/each}
    </div>
    <div class="foot t-label"><kbd>↑</kbd><kbd>↓</kbd> choose · <kbd>Enter</kbd> run · <kbd>Esc</kbd> close</div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 60;
    background: rgba(0, 0, 0, 0.35);
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 12vh;
  }
  .box {
    width: min(560px, 92vw);
    max-height: 64vh;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 8px;
  }
  .big {
    height: 26px;
    font-size: 14px;
  }
  .list {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }
  .sm {
    width: 26px;
    height: 26px;
  }
  .foot {
    font-size: 11px;
    display: flex;
    gap: 3px;
    align-items: center;
  }
</style>
