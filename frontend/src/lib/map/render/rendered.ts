// Lets a map load wait until the map canvas has drawn the view completely
// (every chunk, sprite and appearance in place), so it does not open on a
// black or half drawn map.

const waiters = new Map<number, Set<() => void>>();

/** Resolves once the map has been drawn completely, or after timeout ms. */
export function whenRendered(id: number, timeout = 20000): Promise<void> {
  return new Promise((resolve) => {
    let set = waiters.get(id);
    if (!set) waiters.set(id, (set = new Set()));
    const done = () => {
      clearTimeout(timer);
      set.delete(done);
      resolve();
    };
    const timer = setTimeout(done, timeout);
    set.add(done);
  });
}

/** Called by the map canvas after a frame with nothing left to load. */
export function rendered(id: number): void {
  const set = waiters.get(id);
  if (!set?.size) return;
  for (const done of [...set]) done();
}
