/**
 * Fits square cells of at least minSize into width: as many columns as fit,
 * each cell grown to fill the width. The size stays a whole, even number of
 * pixels with minSize's parity, so content centered in a cell lands on whole
 * pixels and pixel art stays sharp. Less than one pixel per column is left.
 */
export function fitCells(width: number, minSize: number, gap: number): { columns: number; size: number } {
	if (!(width > 0)) return { columns: 1, size: minSize };
	const columns = Math.max(1, Math.floor((width + gap) / (minSize + gap)));
	let size = Math.floor((width - (columns - 1) * gap) / columns);
	if ((size - minSize) % 2) size--;
	return { columns, size: Math.max(minSize, size) };
}
