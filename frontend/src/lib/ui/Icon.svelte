<script lang="ts" module>
  // 16x16 pixel-style icons drawn with simple paths.
  const PATHS: Record<string, string> = {
    open: "M2 4h4l1 1h7v8H2z M2 6h12",
    save: "M3 2h8l2 2v10H3z M5 2v4h6V2 M5 9h6v5H5z",
    new: "M4 2h6l3 3v9H4z M10 2v3h3 M8.5 7v5 M6 9.5h5",
    undo: "M5 5L2 8l3 3 M2 8h8a3 3 0 0 1 0 6H8",
    redo: "M11 5l3 3-3 3 M14 8H6a3 3 0 0 0 0 6h2",
    plus: "M8 3v10 M3 8h10",
    refresh: "M13 8a5 5 0 1 1-1.5-3.6 M13 2v3h-3",
    market: "M2 6l1.5-4h9L14 6z M2 6a2 2 0 0 0 4 0a2 2 0 0 0 4 0a2 2 0 0 0 4 0 M3 8v6h10V8 M6.5 14v-3.5h3V14",
    copy: "M5 5h8v8H5z M3 11V3h8",
    trash: "M3 4h10 M6 4V2h4v2 M4 4l1 10h6l1-10",
    import: "M8 2v8 M5 7l3 3 3-3 M3 12v2h10v-2",
    export: "M8 10V2 M5 5l3-3 3 3 M3 12v2h10v-2",
    play: "M5 3l8 5-8 5z",
    pause: "M4 3h3v10H4z M9 3h3v10H9z",
    prev: "M11 3L5 8l6 5z M4 3v10",
    next: "M5 3l6 5-6 5z M12 3v10",
    grid: "M2 2h12v12H2z M6 2v12 M10 2v12 M2 6h12 M2 10h12",
    pixels: "M2 2h12v12H2z M4.5 2v12 M7 2v12 M9.5 2v12 M12 2v12 M2 4.5h12 M2 7h12 M2 9.5h12 M2 12h12",
    zoomin: "M7 3a4 4 0 1 1 0 8a4 4 0 1 1 0-8 M10 10l4 4 M5 7h4 M7 5v4",
    zoomout: "M7 3a4 4 0 1 1 0 8a4 4 0 1 1 0-8 M10 10l4 4 M5 7h4",
    check: "M3 8l3 3 7-7",
    close: "M4 4l8 8 M12 4l-8 8",
    search: "M7 3a4 4 0 1 1 0 8a4 4 0 1 1 0-8 M10 10l4 4",
    film: "M2 3h12v10H2z M5 3v10 M11 3v10 M2 6h3 M2 10h3 M11 6h3 M11 10h3",
    sheet: "M2 2h12v12H2z M2 8h12 M8 2v12",
    image: "M2 3h12v10H2z M2 11l4-4 3 3 2-2 3 3 M10 6a1 1 0 1 1 0 .1",
    arrowN: "M8 3l4 5H4z M8 8v5",
    arrowE: "M13 8l-5 4V4z M8 8H3",
    arrowS: "M8 13l4-5H4z M8 8V3",
    arrowW: "M3 8l5 4V4z M8 8h5",
    replace: "M3 6h9 M9 3l3 3-3 3 M13 10H4 M7 7l-3 3 3 3",
    info: "M8 2a6 6 0 1 1 0 12A6 6 0 1 1 8 2 M8 7v4 M8 5v.5",
  };
</script>

<script lang="ts">
  let { name, size = 14 }: { name: string; size?: number } = $props();
</script>

<svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  <path d={PATHS[name] ?? ""} />
</svg>
