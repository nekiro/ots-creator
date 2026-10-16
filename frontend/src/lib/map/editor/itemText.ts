// Item texts (signs, books) shortened and wrapped for map labels.

/** An item text as label lines: wrapped at words to width characters, at
 * most maxLines (the last ends with "…" when the text goes on). */
export function wrapText(text: string, width = 40, maxLines = 12): string[] {
  const out: string[] = [];
  for (const paragraph of text.split(/\r?\n/)) {
    let line = "";
    for (const word of paragraph.trim().split(/\s+/)) {
      if (!word) continue;
      for (let rest = word; rest; ) {
        // A word longer than a line is split.
        const piece = rest.slice(0, width);
        rest = rest.slice(piece.length);
        if (line && line.length + 1 + piece.length > width) {
          out.push(line);
          line = piece;
        } else line = line ? `${line} ${piece}` : piece;
      }
    }
    if (line) out.push(line);
  }
  if (out.length <= maxLines) return out;
  const lines = out.slice(0, maxLines);
  lines[maxLines - 1] = lines[maxLines - 1].replace(/[\s,.;:\-–]+$/, "") + "…";
  return lines;
}

/** Start of an item text short enough for a map label: its first line,
 * cut at a word. */
export function labelText(text: string, max = 18): string {
  const line = text.trim().split(/\r?\n/, 1)[0].trim();
  if (line.length <= max) return line;
  const cut = line.slice(0, max);
  const space = cut.lastIndexOf(" ");
  return (space > max / 2 ? cut.slice(0, space) : cut).replace(/[\s,.;:\-–]+$/, "") + "…";
}
