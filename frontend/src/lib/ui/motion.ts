// Gentle open/close transitions for popups. Use them with |global: the
// popups are shown by an {#if} in the parent, where local transitions do
// not play.
import { cubicOut } from "svelte/easing";
import type { TransitionConfig } from "svelte/transition";

const reduced = () => typeof matchMedia !== "undefined" && matchMedia("(prefers-reduced-motion: reduce)").matches;

/** A window: fades in, grows slightly and settles down from a few pixels above. */
export function pop(_node: Element, { duration = 180, y = -8 }: { duration?: number; y?: number } = {}): TransitionConfig {
  if (reduced()) return { duration: 0 };
  return {
    duration,
    easing: cubicOut,
    css: (progress, remaining) => `opacity:${progress};transform:translateY(${remaining * y}px) scale(${0.96 + 0.04 * progress})`,
  };
}

/** The dimmed backdrop behind a window. */
export function backdrop(_node: Element, { duration = 160 }: { duration?: number } = {}): TransitionConfig {
  if (reduced()) return { duration: 0 };
  return { duration, easing: cubicOut, css: (progress) => `opacity:${progress}` };
}
