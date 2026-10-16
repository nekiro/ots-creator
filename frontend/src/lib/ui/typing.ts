// Shortcuts stay out of the way while the user types in a field.

/** True when the event comes from a text field, select or editable element. */
export function isTyping(event: Event): boolean {
  const target = event.target;
  return target instanceof Element && target.closest("input, textarea, select, [contenteditable]") !== null;
}
