// Cuts an image into sprite-sized cells for the slicer tool.

export interface Pixels {
  width: number;
  height: number;
  data: Uint8ClampedArray;
}

export interface Grid {
  offsetX: number;
  offsetY: number;
  columns: number;
  rows: number;
}

/** Largest grid of whole cells that fits the image after the offset. */
export function fitGrid(width: number, height: number, size: number, offsetX = 0, offsetY = 0): Grid {
  return {
    offsetX,
    offsetY,
    columns: Math.max(0, Math.floor((width - offsetX) / size)),
    rows: Math.max(0, Math.floor((height - offsetY) / size)),
  };
}

/**
 * Copies one cell (column, row) into RGBA sprite bytes. Pixels outside the
 * image are transparent; opaque magenta becomes transparent when asked, and
 * transparent pixels are zeroed so identical sprites compare equal.
 */
export function cutCell(img: Pixels, grid: Grid, column: number, row: number, size: number, magenta: boolean): Uint8ClampedArray {
  const out = new Uint8ClampedArray(size * size * 4);
  const x0 = grid.offsetX + column * size;
  const y0 = grid.offsetY + row * size;
  for (let y = 0; y < size; y++) {
    const sy = y0 + y;
    if (sy < 0 || sy >= img.height) continue;
    for (let x = 0; x < size; x++) {
      const sx = x0 + x;
      if (sx < 0 || sx >= img.width) continue;
      const s = (sy * img.width + sx) * 4;
      const d = (y * size + x) * 4;
      const [r, g, b, a] = [img.data[s], img.data[s + 1], img.data[s + 2], img.data[s + 3]];
      if (a === 0 || (magenta && r === 255 && g === 0 && b === 255 && a === 255)) continue;
      out[d] = r;
      out[d + 1] = g;
      out[d + 2] = b;
      out[d + 3] = a;
    }
  }
  return out;
}

export function isEmpty(px: Uint8ClampedArray): boolean {
  for (let i = 3; i < px.length; i += 4) if (px[i] !== 0) return false;
  return true;
}
