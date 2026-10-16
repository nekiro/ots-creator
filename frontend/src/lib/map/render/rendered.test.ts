import { describe, expect, it, vi } from "vitest";
import { rendered, whenRendered } from "./rendered";

describe("whenRendered", () => {
  it("resolves on the map's complete frame, or after the timeout", async () => {
    vi.useFakeTimers();
    let firstResolved = false;
    let secondResolved = false;
    void whenRendered(1).then(() => (firstResolved = true));
    void whenRendered(2, 1000).then(() => (secondResolved = true));
    rendered(3);
    rendered(1);
    await Promise.resolve();
    expect([firstResolved, secondResolved]).toEqual([true, false]);
    await vi.advanceTimersByTimeAsync(1000);
    expect(secondResolved).toBe(true);
    vi.useRealTimers();
  });
});
