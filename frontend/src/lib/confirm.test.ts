import { describe, expect, it } from "vitest";
import { answer, ask, confirmState } from "./confirm.svelte";

describe("ask", () => {
  it("resolves with the answer and clears the question", async () => {
    const p = ask({ title: "T", message: "M", ok: "Yes" });
    expect(confirmState.question?.title).toBe("T");
    answer(true);
    expect(await p).toBe(true);
    expect(confirmState.question).toBeNull();
  });

  it("cancels a pending question when a new one arrives", async () => {
    const first = ask({ title: "A", message: "", ok: "Ok" });
    const second = ask({ title: "B", message: "", ok: "Ok" });
    expect(await first).toBe(false);
    answer(true);
    expect(await second).toBe(true);
  });
});
