// File paths as text; both Windows and slash separators count.

/** The last part of a path: "C:\a\Tibia.dat" gives "Tibia.dat". */
export function basename(path: string): string {
  return path.replace(/^.*[\\/]/, "");
}

/** The path without its last part: "C:\a\Tibia.dat" gives "C:\a". */
export function dirname(path: string): string {
  return path.replace(/[\\/][^\\/]*$/, "");
}
