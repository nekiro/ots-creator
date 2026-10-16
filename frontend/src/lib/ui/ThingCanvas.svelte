<script lang="ts">
  // Self-contained animated rendering of one thing: used by the OBD viewer
  // and the animated list thumbnails. Pixels come from `get`; bump `ready`
  // when more sprites have loaded.
  import { frameGroupAt } from "../thing";
  import { Category, type Thing } from "../api";
  import { Animator, guessImproved, previewDurations } from "../render/animator";
  import { compose, type PixelSource } from "../render/compose";
  import { contentBox, union, type Box } from "../render/bounds";
  import { DEFAULT_COLORS } from "../render/outfit";
  import { clamp } from "../math";

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
    improved,
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
    /** The client stores frame durations; guessed from the thing when not given. */
    improved?: boolean;
  } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let frame = $state(0);

  const frameGroup = $derived(frameGroupAt(thing, group));
  const isOutfit = $derived(thing.category === Category.CategoryOutfit);
  const dirX = (shown: typeof frameGroup) => (isOutfit && shown ? Math.min(direction, shown.patternX - 1) : 0);
  const draw = (f: number) =>
    compose(frameGroup!, frameGroup!.sprites, size, { pos: { layer: 0, x: dirX(frameGroup), y: 0, z: 0, frame: f }, colors: isOutfit && colorize ? DEFAULT_COLORS : null }, get);

  // Fit mode crops to the visible pixels of all frames, so the animation
  // does not jump and small objects in big textures are not drawn tiny.
  const crop = $derived.by(() => {
    void ready;
    if (!fit || !frameGroup) return null;
    let box: Box | null = null;
    for (let f = 0; f < frameGroup.frames; f++) box = union(box, contentBox(draw(f)));
    return box;
  });
  const w = $derived(crop ? crop.w : (frameGroup?.width ?? 1) * size);
  const h = $derived(crop ? crop.h : (frameGroup?.height ?? 1) * size);
  const scale = $derived(fit ? Math.min(1, fit / Math.max(w, h)) : zoom);

  $effect(() => {
    if (!frameGroup || frameGroup.frames < 2 || !playing) {
      frame = 0;
      return;
    }
    const d = previewDurations(frameGroup, {
      outfit: isOutfit,
      groups: thing.frameGroups.length,
      animateAlways: !!thing.props.animateAlways,
      improved: improved ?? guessImproved(thing),
    });
    let animator: Animator;
    try {
      animator = new Animator(Number(frameGroup.mode), 0, clamp(frameGroup.startFrame, 0, frameGroup.frames - 1), d, performance.now());
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
    if (!canvas || !frameGroup) return;
    const img = draw(frame);
    const c = crop ?? { x: 0, y: 0, w: img.width, h: img.height };
    canvas.width = c.w;
    canvas.height = c.h;
    canvas.getContext("2d")!.putImageData(img, -c.x, -c.y);
  });
</script>

<canvas bind:this={canvas} class="pixel" style="width:{w * scale}px;height:{h * scale}px"></canvas>
