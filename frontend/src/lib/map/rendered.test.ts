import { describe, expect, it, vi } from "vitest";
import { rendered, whenRendered } from "./rendered";

describe("whenRendered", () => {
  it("resolves on the map's complete frame, or after the timeout", async () => {
    vi.useFakeTimers();
    let a = false;
    let b = false;
    void whenRendered(1).then(() => (a = true));
    void whenRendered(2, 1000).then(() => (b = true));
    rendered(3);
    rendered(1);
    await Promise.resolve();
    expect([a, b]).toEqual([true, false]);
    await vi.advanceTimersByTimeAsync(1000);
    expect(b).toBe(true);
    vi.useRealTimers();
  });
});
