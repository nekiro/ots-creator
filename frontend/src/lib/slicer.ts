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
    const srcY = y0 + y;
    if (srcY < 0 || srcY >= img.height) continue;
    for (let x = 0; x < size; x++) {
      const srcX = x0 + x;
      if (srcX < 0 || srcX >= img.width) continue;
      const srcOffset = (srcY * img.width + srcX) * 4;
      const dstOffset = (y * size + x) * 4;
      const [r, g, b, a] = [img.data[srcOffset], img.data[srcOffset + 1], img.data[srcOffset + 2], img.data[srcOffset + 3]];
      if (a === 0 || (magenta && r === 255 && g === 0 && b === 255 && a === 255)) continue;
      out[dstOffset] = r;
      out[dstOffset + 1] = g;
      out[dstOffset + 2] = b;
      out[dstOffset + 3] = a;
    }
  }
  return out;
}

export function isEmpty(pixels: Uint8ClampedArray): boolean {
  for (let i = 3; i < pixels.length; i += 4) if (pixels[i] !== 0) return false;
  return true;
}
