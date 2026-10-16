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
  import { clamp } from "../util/math";

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
  const patternXOf = (shown: typeof frameGroup) => (isOutfit && shown ? Math.min(direction, shown.patternX - 1) : 0);
  const draw = (frameIndex: number) =>
    compose(frameGroup!, frameGroup!.sprites, size, { pos: { layer: 0, x: patternXOf(frameGroup), y: 0, z: 0, frame: frameIndex }, colors: isOutfit && colorize ? DEFAULT_COLORS : null }, get);

  // Fit mode crops to the visible pixels of all frames, so the animation
  // does not jump and small objects in big textures are not drawn tiny.
  const crop = $derived.by(() => {
    void ready;
    if (!fit || !frameGroup) return null;
    let box: Box | null = null;
    for (let frameIndex = 0; frameIndex < frameGroup.frames; frameIndex++) box = union(box, contentBox(draw(frameIndex)));
    return box;
  });
  const width = $derived(crop ? crop.w : (frameGroup?.width ?? 1) * size);
  const height = $derived(crop ? crop.h : (frameGroup?.height ?? 1) * size);
  const scale = $derived(fit ? Math.min(1, fit / Math.max(width, height)) : zoom);

  $effect(() => {
    if (!frameGroup || frameGroup.frames < 2 || !playing) {
      frame = 0;
      return;
    }
    const durations = previewDurations(frameGroup, {
      outfit: isOutfit,
      groups: thing.frameGroups.length,
      animateAlways: !!thing.props.animateAlways,
      improved: improved ?? guessImproved(thing),
    });
    let animator: Animator;
    try {
      animator = new Animator(Number(frameGroup.mode), 0, clamp(frameGroup.startFrame, 0, frameGroup.frames - 1), durations, performance.now());
    } catch {
      return;
    }
    let animationFrame = 0;
    const tick = (time: number) => {
      frame = animator.update(time);
      animationFrame = requestAnimationFrame(tick);
    };
    animationFrame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(animationFrame);
  });

  $effect(() => {
    void ready;
    if (!canvas || !frameGroup) return;
    const image = draw(frame);
    const area = crop ?? { x: 0, y: 0, w: image.width, h: image.height };
    canvas.width = area.w;
    canvas.height = area.h;
    canvas.getContext("2d")!.putImageData(image, -area.x, -area.y);
  });
</script>

<canvas bind:this={canvas} class="pixel" style="width:{width * scale}px;height:{height * scale}px"></canvas>
