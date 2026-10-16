const MODIFIERS = new Set(["Control", "Shift", "Alt", "Meta", "AltGraph", "CapsLock", "NumLock", "ScrollLock"]);

/**
 * The key combination of a key press, like "Ctrl+Shift+S", or null for a
 * modifier alone. Shift is left out for symbols: it changes the character
 * itself (Shift+= is "+").
 */
export function comboOf(event: Pick<KeyboardEvent, "key" | "ctrlKey" | "metaKey" | "altKey" | "shiftKey">): string | null {
  if (MODIFIERS.has(event.key) || event.key === "Dead" || event.key === "Unidentified") return null;
  let key = event.key === " " ? "Space" : event.key;
  const letter = key.length === 1 && key.toLowerCase() !== key.toUpperCase();
  if (key.length === 1) key = key.toUpperCase();
  const parts: string[] = [];
  if (event.ctrlKey || event.metaKey) parts.push("Ctrl");
  if (event.altKey) parts.push("Alt");
  if (event.shiftKey && (letter || key.length > 1)) parts.push("Shift");
  parts.push(key);
  return parts.join("+");
}
