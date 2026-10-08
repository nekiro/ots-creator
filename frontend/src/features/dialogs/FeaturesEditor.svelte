<script lang="ts">
  import type { Features } from "../../lib/api";
  import Select from "../../lib/ui/Select.svelte";

  // Features forced by a version cannot be turned off (see client.ApplyVersionDefaults).
  let { features = $bindable(), version, sizeLocked = false }: { features: Features; version: number; sizeLocked?: boolean } = $props();

  const forced = $derived({
    extended: version >= 960,
    improvedAnimations: version >= 1050,
    frameGroups: version >= 1057,
  });

  $effect(() => {
    if (forced.extended && !features.extended) features.extended = true;
    if (forced.improvedAnimations && !features.improvedAnimations) features.improvedAnimations = true;
    if (forced.frameGroups && !features.frameGroups) features.frameGroups = true;
  });

  const items: { key: "extended" | "transparency" | "improvedAnimations" | "frameGroups"; label: string; hint: string }[] = [
    { key: "extended", label: "Extended", hint: "32-bit sprite ids (more than 65535 sprites)" },
    { key: "transparency", label: "Transparency", hint: "Sprites keep an alpha channel" },
    { key: "improvedAnimations", label: "Improved animations", hint: "Per-frame durations, loops and start frame" },
    { key: "frameGroups", label: "Frame groups", hint: "Separate idle and walking animations for outfits" },
  ];
</script>

<div class="features">
  {#each items as it}
    <label class="row" title={it.hint}>
      <input class="t-check" type="checkbox" bind:checked={features[it.key]} disabled={it.key !== "transparency" && forced[it.key]} />
      <span>{it.label}</span>
      {#if it.key !== "transparency" && forced[it.key]}<span class="t-label">(required)</span>{/if}
    </label>
  {/each}
  <label class="row">
    <span class="t-label">Sprite size</span>
    <Select value={features.spriteSize} disabled={sizeLocked} options={[32, 64, 128, 256].map((s) => ({ value: s, label: `${s}×${s}` }))} onchange={(v) => (features.spriteSize = v)} />
  </label>
</div>

<style>
  .features {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px 12px;
  }
</style>
