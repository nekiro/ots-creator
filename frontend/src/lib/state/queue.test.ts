import { describe, expect, it } from "vitest";
import { Category } from "../api";
import { formatIdList } from "../util/format";
import { DropQueue, type QueueItem } from "./queue.svelte";

const ITEM = Category.CategoryItem;
const OUTFIT = Category.CategoryOutfit;

function queue() {
  let items: QueueItem<number>[] = [];
  const dropQueue = new DropQueue<number>({
    from: "from B",
    hint: "",
    items: () => items,
    setItems: (next) => (items = next),
    thumb: (category, key) => `thumb/${category}/${key}`,
    put: async (_category, keys) => ({ ids: keys }),
  });
  return { dropQueue, items: () => items };
}

describe("DropQueue", () => {
  it("queues each key once per category", () => {
    const { dropQueue } = queue();
    dropQueue.enqueue(ITEM, [1, 2, 2]);
    dropQueue.enqueue(ITEM, [2, 3]);
    dropQueue.enqueue(OUTFIT, [2]);
    expect(dropQueue.queued(ITEM)).toEqual([1, 2, 3]);
    expect(dropQueue.queued(OUTFIT)).toEqual([2]);
    expect(dropQueue.counts()).toEqual(
      new Map([
        [ITEM, 3],
        [OUTFIT, 1],
      ]),
    );
  });

  it("keeps labels and thumbnails of queued items", () => {
    const { dropQueue } = queue();
    dropQueue.enqueue(ITEM, [{ key: 5, label: "sword" }, 6]);
    const [sword, plain] = dropQueue.list(ITEM);
    expect(dropQueue.label(sword)).toBe("sword");
    expect(dropQueue.label(plain)).toBe("#6");
    expect(dropQueue.thumb(plain)).toBe(`thumb/${ITEM}/6`);
  });

  it("dequeues and clears one category", () => {
    const { dropQueue } = queue();
    dropQueue.enqueue(ITEM, [1, 2, 3]);
    dropQueue.enqueue(OUTFIT, [1]);
    dropQueue.dequeue(ITEM, [2]);
    expect(dropQueue.queued(ITEM)).toEqual([1, 3]);
    dropQueue.clear(ITEM);
    expect(dropQueue.queued(ITEM)).toEqual([]);
    expect(dropQueue.queued(OUTFIT)).toEqual([1]);
  });

  it("moves an item to the front", () => {
    const { dropQueue } = queue();
    dropQueue.enqueue(ITEM, [1, 2, 3]);
    dropQueue.toFront(ITEM, 3);
    expect(dropQueue.queued(ITEM)).toEqual([3, 1, 2]);
    dropQueue.toFront(ITEM, 9);
    expect(dropQueue.queued(ITEM)).toEqual([3, 1, 2]);
  });
});

describe("formatIdList", () => {
  it("writes one id or a run of ids", () => {
    expect(formatIdList([5])).toBe("#5");
    expect(formatIdList([7, 5, 6])).toBe("#5-#7");
  });

  it("lists scattered ids, at most three", () => {
    expect(formatIdList([9, 5])).toBe("#5, #9");
    expect(formatIdList([12, 5, 9, 20, 30])).toBe("#5, #9, #12 (+2)");
  });
});
