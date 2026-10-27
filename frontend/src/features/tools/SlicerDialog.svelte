<script lang="ts">
  import { SpriteService, encodeBytes } from "../../lib/api";
  import { toCss } from "../../lib/pixelscale";
  import { cutCell, fitGrid, isEmpty, type Grid, type Pixels } from "../../lib/slicer";
  import { app, run, toast } from "../../lib/state/app.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import Icon from "../../lib/ui/Icon.svelte";
  import NumberField from "../../lib/ui/NumberField.svelte";
  import PanStage from "../../lib/ui/PanStage.svelte";

  const size = app.project?.info.features.spriteSize ?? 32;

  let fileInput: HTMLInputElement;
  let canvas = $state<HTMLCanvasElement>();
  let img = $state<Pixels | null>(null);
  let bitmap = $state<ImageBitmap | null>(null);
  let name = $state("");
  let grid = $state<Grid>({ offsetX: 0, offsetY: 0, columns: 0, rows: 0 });
  let zoom = $state(2);
  let magenta = $state(true);
  let skipEmpty = $state(true);
  // Selected cells as "column,row"; empty means all cells.
  let picked = $state<Set<string>>(new Set());

  const cells = $derived(grid.columns * grid.rows);
  const cellKeyOf = (column: number, row: number) => `${column},${row}`;

  async function load(file: File | undefined) {
    if (!file) return;
    try {
      const decoded = await createImageBitmap(file);
      const offscreen = new OffscreenCanvas(decoded.width, decoded.height);
      const ctx = offscreen.getContext("2d")!;
      ctx.drawImage(decoded, 0, 0);
      const data = ctx.getImageData(0, 0, decoded.width, decoded.height);
      bitmap?.close();
      bitmap = decoded;
      img = { width: data.width, height: data.height, data: data.data };
      name = file.name;
      grid = fitGrid(data.width, data.height, size);
      picked = new Set();
    } catch {
      toast(`Cannot read ${file.name}.`, "error");
    }
  }

  function setOffset(axis: "offsetX" | "offsetY", value: number) {
    if (!img) return;
    const next = { ...grid, [axis]: value };
    const fit = fitGrid(img.width, img.height, size, next.offsetX, next.offsetY);
    grid = { ...next, columns: Math.min(grid.columns, fit.columns) || fit.columns, rows: Math.min(grid.rows, fit.rows) || fit.rows };
    picked = new Set();
  }

  $effect(() => {
    if (!canvas || !img || !bitmap) return;
    const canvasWidth = img.width * zoom;
    const canvasHeight = img.height * zoom;
    canvas.width = canvasWidth;
    canvas.height = canvasHeight;
    const ctx = canvas.getContext("2d")!;
    ctx.imageSmoothingEnabled = false;
    ctx.clearRect(0, 0, canvasWidth, canvasHeight);
    ctx.drawImage(bitmap, 0, 0, canvasWidth, canvasHeight);
    const cell = size * zoom;
    const originX = grid.offsetX * zoom;
    const originY = grid.offsetY * zoom;
    ctx.fillStyle = "rgba(80, 140, 255, 0.35)";
    for (const cellKey of picked) {
      const [column, row] = cellKey.split(",").map(Number);
      ctx.fillRect(originX + column * cell, originY + row * cell, cell, cell);
    }
    ctx.strokeStyle = "rgba(255, 255, 255, 0.55)";
    ctx.lineWidth = 1;
    ctx.beginPath();
    for (let column = 0; column <= grid.columns; column++) {
      ctx.moveTo(originX + column * cell + 0.5, originY);
      ctx.lineTo(originX + column * cell + 0.5, originY + grid.rows * cell);
    }
    for (let row = 0; row <= grid.rows; row++) {
      ctx.moveTo(originX, originY + row * cell + 0.5);
      ctx.lineTo(originX + grid.columns * cell, originY + row * cell + 0.5);
    }
    ctx.stroke();
  });

  function cellAt(event: MouseEvent): [number, number] | null {
    const rect = (event.currentTarget as HTMLCanvasElement).getBoundingClientRect();
    const x = toCss(event.clientX - rect.left) / zoom - grid.offsetX;
    const y = toCss(event.clientY - rect.top) / zoom - grid.offsetY;
    const column = Math.floor(x / size);
    const row = Math.floor(y / size);
    return column >= 0 && row >= 0 && column < grid.columns && row < grid.rows ? [column, row] : null;
  }

  function toggle(event: MouseEvent) {
    const cell = cellAt(event);
    if (!cell) return;
    const cellKey = cellKeyOf(...cell);
    const next = new Set(picked);
    if (next.has(cellKey)) next.delete(cellKey);
    else next.add(cellKey);
    picked = next;
  }

  function slices(): Uint8ClampedArray[] {
    if (!img) return [];
    const out: Uint8ClampedArray[] = [];
    for (let row = 0; row < grid.rows; row++)
      for (let column = 0; column < grid.columns; column++) {
        if (picked.size && !picked.has(cellKeyOf(column, row))) continue;
        const pixels = cutCell(img, grid, column, row, size, magenta);
        if (!skipEmpty || !isEmpty(pixels)) out.push(pixels);
      }
    return out;
  }

  async function importSprites() {
    const list = slices();
    if (!list.length) {
      toast("Every chosen cell is empty.");
      return;
    }
    const ids = await run("Importing", () => SpriteService.AddPixels(list.map(encodeBytes)));
    if (ids?.length) {
      toast(`Added ${ids.length} sprite(s): #${ids[0]}${ids.length > 1 ? `-#${ids.at(-1)}` : ""}`, "success");
      app.selectedSprites = [ids[0]];
      app.dialog = null;
    }
  }
</script>

<Dialog title="Slicer" width={760} onclose={() => (app.dialog = null)}>
  <div class="layout">
    <div class="stage t-panel">
      {#if img}
        <PanStage onzoom={(delta) => (zoom = Math.min(6, Math.max(1, zoom + delta)))} resetKey={name}>
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
          <canvas bind:this={canvas} onclick={toggle} class="pixel"></canvas>
        </PanStage>
      {:else}
        <button class="pick" onclick={() => fileInput.click()}>
          <Icon name="image" size={28} />
          <span>Choose an image to cut into {size}×{size} sprites</span>
        </button>
      {/if}
    </div>
    <div class="side col">
      <button class="t-btn" onclick={() => fileInput.click()}>Open image…</button>
      {#if img}<span class="t-label name" title={name}>{name} · {img.width}×{img.height}</span>{/if}
      <h4>Grid</h4>
      <NumberField label="Offset X" value={grid.offsetX} min={0} max={size - 1} width={44} disabled={!img} onchange={(value) => setOffset("offsetX", value)} />
      <NumberField label="Offset Y" value={grid.offsetY} min={0} max={size - 1} width={44} disabled={!img} onchange={(value) => setOffset("offsetY", value)} />
      <NumberField label="Columns" value={grid.columns} min={1} max={img ? fitGrid(img.width, img.height, size, grid.offsetX).columns : 1} width={44} disabled={!img} onchange={(value) => ((grid.columns = value), (picked = new Set()))} />
      <NumberField label="Rows" value={grid.rows} min={1} max={img ? fitGrid(img.width, img.height, size, 0, grid.offsetY).rows : 1} width={44} disabled={!img} onchange={(value) => ((grid.rows = value), (picked = new Set()))} />
      <NumberField label="Zoom" value={zoom} min={1} max={6} width={44} disabled={!img} onchange={(value) => (zoom = value)} />
      <h4>Import</h4>
      <label class="row"><input class="t-check" type="checkbox" bind:checked={magenta} />Magenta is transparent</label>
      <label class="row"><input class="t-check" type="checkbox" bind:checked={skipEmpty} />Skip empty cells</label>
      <span class="t-label">{picked.size ? `${picked.size} of ${cells} cells picked` : `All ${cells} cells (click to pick)`}</span>
      <button class="t-btn" disabled={!picked.size} onclick={() => (picked = new Set())}>Clear selection</button>
    </div>
  </div>
  <input bind:this={fileInput} type="file" accept="image/png,image/bmp,image/gif,image/jpeg" hidden onchange={(event) => load(event.currentTarget.files?.[0])} />
  {#snippet footer()}
    <span class="grow"></span>
    <button class="t-btn primary" disabled={!img || !cells || !app.open || !!app.busy} onclick={importSprites}>Import sprites</button>
    <button class="t-btn" onclick={() => (app.dialog = null)}>Cancel</button>
  {/snippet}
</Dialog>

<style>
  .layout {
    display: flex;
    gap: 10px;
    height: 440px;
  }
  .stage {
    position: relative;
    flex: 1;
    min-width: 0;
    overflow: hidden;
    padding: 6px;
    display: flex;
    background-image: url("../../assets/ui/ditherpattern.png");
  }
  canvas {
    margin: auto;
    cursor: crosshair;
    flex: none;
  }
  .pick {
    margin: auto;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    background: none;
    border: 0;
    color: var(--text-dim);
    font: inherit;
    cursor: pointer;
  }
  .pick:hover {
    color: var(--text-bright);
  }
  .side {
    width: 190px;
    flex: none;
    gap: 5px;
    align-items: stretch;
  }
  .side :global(.nf) {
    justify-content: space-between;
  }
  h4 {
    margin: 6px 0 0;
    color: var(--text-bright);
    font-size: var(--fs);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
