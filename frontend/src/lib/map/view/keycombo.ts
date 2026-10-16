const MODIFIERS = new Set(["Control", "Shift", "Alt", "Meta", "AltGraph", "CapsLock", "NumLock", "ScrollLock"]);

/**
 * The key combination of a key press, like "Ctrl+Shift+S", or null for a
 * modifier alone. Shift is left out for symbols: it changes the character
 * itself (Shift+= is "+").
 */
export function comboOf(e: Pick<KeyboardEvent, "key" | "ctrlKey" | "metaKey" | "altKey" | "shiftKey">): string | null {
  if (MODIFIERS.has(e.key) || e.key === "Dead" || e.key === "Unidentified") return null;
  let key = e.key === " " ? "Space" : e.key;
  const letter = key.length === 1 && key.toLowerCase() !== key.toUpperCase();
  if (key.length === 1) key = key.toUpperCase();
  const parts: string[] = [];
  if (e.ctrlKey || e.metaKey) parts.push("Ctrl");
  if (e.altKey) parts.push("Alt");
  if (e.shiftKey && (letter || key.length > 1)) parts.push("Shift");
  parts.push(key);
  return parts.join("+");
}
