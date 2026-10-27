export class LruCache<Key, Value> {
  private entries = new Map<Key, Value>();

  constructor(readonly capacity: number) {}

  get size(): number {
    return this.entries.size;
  }

  get(key: Key): Value | undefined {
    const value = this.entries.get(key);
    if (value === undefined) return undefined;
    this.entries.delete(key);
    this.entries.set(key, value);
    return value;
  }

  peek(key: Key): Value | undefined {
    return this.entries.get(key);
  }

  set(key: Key, value: Value): void {
    this.entries.delete(key);
    this.entries.set(key, value);
    while (this.entries.size > this.capacity) this.entries.delete(this.entries.keys().next().value!);
  }

  delete(key: Key): void {
    this.entries.delete(key);
  }
}
