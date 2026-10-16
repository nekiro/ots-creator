// A map that keeps only its most recently used entries.

export class LruCache<Key, Value> {
  private entries = new Map<Key, Value>();

  constructor(readonly capacity: number) {}

  get size(): number {
    return this.entries.size;
  }

  /** The value, now the most recently used. */
  get(key: Key): Value | undefined {
    const value = this.entries.get(key);
    if (value === undefined) return undefined;
    this.entries.delete(key);
    this.entries.set(key, value);
    return value;
  }

  /** The value without counting it as used. */
  peek(key: Key): Value | undefined {
    return this.entries.get(key);
  }

  /** Stores the value as the most recently used; the oldest go past the capacity. */
  set(key: Key, value: Value): void {
    this.entries.delete(key);
    this.entries.set(key, value);
    while (this.entries.size > this.capacity) this.entries.delete(this.entries.keys().next().value!);
  }

  delete(key: Key): void {
    this.entries.delete(key);
  }
}
