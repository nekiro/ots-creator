<script lang="ts">
  // Ctrl+K: type a position, a place, a brush, an item, a creature or a
  // command, and press Enter.
  import { tick } from "svelte";
  import { parsePos, fmtPos, type CreatureType, type PaletteBrush } from "../../lib/map/data/api";
  import { goTo, mapCommands } from "../../lib/map/view/commands";
  import { mapMenus } from "../../lib/map/view/menus";
  import { map, selectBrush } from "../../lib/map/state.svelte";
  import { goHouse, houses, towns } from "../../lib/map/editor/world";
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
      for (const menuItem of menu.items) {
        if (menuItem === "-" || menuItem.disabled?.()) continue;
        out.push({ group: "Commands", label: menuItem.label.replace(/…$/, ""), detail: menu.label, keys: menuItem.keys, run: menuItem.action });
      }
    }
    return out;
  });

  const query = $derived(text.trim().toLowerCase());
  const entries = $derived.by<Entry[]>(() => {
    const out: Entry[] = [];
    const hasMap = map.activeId !== null;
    const position = hasMap ? parsePos(text) : null;
    if (position) out.push({ group: "Go to", label: `Go to ${fmtPos(position)}`, run: () => (goTo(position), mapCommands.inspect(position)) });
    if (!query) return [...out, ...commands.slice(0, 40)];
    const match = (label: string) => label.toLowerCase().includes(query);
    if (hasMap) {
      for (const town of towns()) if (match(town.name)) out.push({ group: "Places", label: town.name, detail: "town", run: () => (mapCommands.focus({ kind: "town", id: town.id }), town.temple.x && goTo(town.temple)) });
      for (const house of houses()) if (match(house.name)) out.push({ group: "Places", label: house.name, detail: "house", run: () => (mapCommands.focus({ kind: "house", id: house.id }), goHouse(house)) });
      for (const waypoint of map.active?.waypoints ?? []) if (waypoint && match(waypoint.name)) out.push({ group: "Places", label: waypoint.name, detail: "waypoint", run: () => (mapCommands.focus({ kind: "waypoint", name: waypoint.name }), goTo(waypoint.pos)) });
    }
    const brushes = (map.palette?.brushes ?? []).filter((brush): brush is PaletteBrush => !!brush && match(brush.name)).slice(0, 30);
    for (const brush of brushes) out.push({ group: "Brushes", label: brush.name, detail: brush.kind, icon: { id: brush.look }, run: () => selectBrush({ kind: brush.kind === "decoration" ? "wall" : brush.kind, name: brush.name, look: brush.look, label: brush.name }) });
    for (const creature of (map.palette?.creatures ?? []).filter((creature): creature is CreatureType => !!creature && match(creature.name)).slice(0, 20)) {
      out.push({ group: "Creatures", label: creature.name, detail: creature.npc ? "NPC" : "monster", icon: { outfit: creature.outfit.lookItem ? 0 : creature.outfit.lookType, id: creature.outfit.lookItem }, run: () => selectBrush({ kind: "creature", creature: creature.name, npc: !!creature.npc, label: creature.name }) });
    }
    for (const id of map.items?.search(query, 30) ?? []) {
      const name = map.items!.name(id);
      out.push({ group: "Items", label: name, detail: String(id), icon: { id }, run: () => selectBrush({ kind: "raw", item: id, look: id, label: name }) });
    }
    for (const command of commands) if (match(command.label) || match(command.detail ?? "")) out.push(command);
    return out.slice(0, 120);
  });

  $effect(() => {
    void entries;
    active = 0;
  });

  function run(entry: Entry | undefined) {
    if (!entry) return;
    close();
    void Promise.resolve(entry.run());
  }

  function onkey(event: KeyboardEvent) {
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      close();
    } else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const count = entries.length;
      if (!count) return;
      active = (active + (event.key === "ArrowDown" ? 1 : count - 1)) % count;
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
      {#each entries as entry, i}
        {#if i === 0 || entries[i - 1].group !== entry.group}<div class="m-sect">{entry.group}</div>{/if}
        <button class="m-row" class:on={i === active} onmousemove={() => (active = i)} onclick={() => run(entry)}>
          {#if entry.icon}<span class="t-slot sm"><ItemIcon id={entry.icon.id ?? 0} outfit={entry.icon.outfit ?? 0} size={24} /></span>{/if}
          <span class="m-name">{entry.label}</span>
          {#if entry.detail}<span class="m-meta">{entry.detail}</span>{/if}
          {#if entry.keys}<kbd>{entry.keys}</kbd>{/if}
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
