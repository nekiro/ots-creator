<script lang="ts">
  // Self-contained animated rendering of one thing: used by the OBD viewer
  // and the animated list thumbnails. Pixels come from `get`; bump `ready`
  // when more sprites have loaded.
  import { Category, type Thing } from "../api";
  import { Animator } from "../render/animator";
  import { compose, type PixelSource } from "../render/compose";
  import { contentBox, union, type Box } from "../render/bounds";
  import { DEFAULT_COLORS } from "../render/outfit";

  let {
    thing,
    get,
    size = 32,
    ready = 0,
    group = 0,
    direction = 2,
    zoom = 1,
    fit = 0,
    playing = true,
    colorize = true,
  }: {
    thing: Thing;
    get: PixelSource;
    size?: number;
    ready?: number;
    group?: number;
    /** Outfit direction (pattern X), south by default. */
    direction?: number;
    zoom?: number;
    /** When set, the visible pixels are cropped and scaled down to fit this many pixels. */
    fit?: number;
    playing?: boolean;
    /** Paint outfits with the default colors; off shows the white template like list thumbnails. */
    colorize?: boolean;
  } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let frame = $state(0);

  const g = $derived(thing.frameGroups[Math.min(group, thing.frameGroups.length - 1)] ?? null);
  const isOutfit = $derived(thing.category === Category.CategoryOutfit);
  const dirX = (gr: typeof g) => (isOutfit && gr ? Math.min(direction, gr.patternX - 1) : 0);
  const draw = (f: number) =>
    compose(g!, g!.sprites, size, { pos: { layer: 0, x: dirX(g), y: 0, z: 0, frame: f }, colors: isOutfit && colorize ? DEFAULT_COLORS : null }, get);

  // Fit mode crops to the visible pixels of all frames, so the animation
  // does not jump and small objects in big textures are not drawn tiny.
  const crop = $derived.by(() => {
    void ready;
    if (!fit || !g) return null;
    let box: Box | null = null;
    for (let f = 0; f < g.frames; f++) box = union(box, contentBox(draw(f)));
    return box;
  });
  const w = $derived(crop ? crop.w : (g?.width ?? 1) * size);
  const h = $derived(crop ? crop.h : (g?.height ?? 1) * size);
  const scale = $derived(fit ? Math.min(1, fit / Math.max(w, h)) : zoom);

  $effect(() => {
    if (!g || g.frames < 2 || !playing) {
      frame = 0;
      return;
    }
    const d = g.durations.length === g.frames ? g.durations : Array.from({ length: g.frames }, () => ({ min: 100, max: 100 }));
    let animator: Animator;
    try {
      animator = new Animator(Number(g.mode), 0, Math.min(Math.max(g.startFrame, 0), g.frames - 1), d, performance.now());
    } catch {
      return;
    }
    let raf = 0;
    const tick = (t: number) => {
      frame = animator.update(t);
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  });

  $effect(() => {
    void ready;
    if (!canvas || !g) return;
    const img = draw(frame);
    const c = crop ?? { x: 0, y: 0, w: img.width, h: img.height };
    canvas.width = c.w;
    canvas.height = c.h;
    canvas.getContext("2d")!.putImageData(img, -c.x, -c.y);
  });
</script>

<canvas bind:this={canvas} class="pixel" style="width:{w * scale}px;height:{h * scale}px"></canvas>
