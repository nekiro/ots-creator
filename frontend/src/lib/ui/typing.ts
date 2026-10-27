export function isTyping(event: Event): boolean {
  const target = event.target;
  return target instanceof Element && target.closest("input, textarea, select, [contenteditable]") !== null;
}
