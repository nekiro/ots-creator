import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { loadJson, readJson, readPositive, readStored, saveJson, writeStored } from "./storage";

function memoryStorage(): Storage {
  const data = new Map<string, string>();
  return {
    get length() {
      return data.size;
    },
    clear: () => data.clear(),
    getItem: (key) => data.get(key) ?? null,
    key: (index) => [...data.keys()][index] ?? null,
    removeItem: (key) => void data.delete(key),
    setItem: (key, value) => void data.set(key, String(value)),
  };
}

describe("storage", () => {
  beforeEach(() => vi.stubGlobal("localStorage", memoryStorage()));
  afterEach(() => vi.unstubAllGlobals());

  it("round trips text and JSON", () => {
    writeStored("a", "text");
    expect(readStored("a")).toBe("text");
    saveJson("b", { x: 1 });
    expect(readJson("b")).toEqual({ x: 1 });
  });

  it("merges stored objects over defaults", () => {
    saveJson("prefs", { grid: true });
    expect(loadJson("prefs", { grid: false, labels: true })).toEqual({ grid: true, labels: true });
    expect(loadJson("missing", { grid: false })).toEqual({ grid: false });
  });

  it("falls back on unreadable JSON", () => {
    writeStored("bad", "{");
    expect(readJson("bad")).toBeNull();
    expect(loadJson("bad", { ok: 1 })).toEqual({ ok: 1 });
  });

  it("reads only positive numbers", () => {
    writeStored("height", "120");
    expect(readPositive("height")).toBe(120);
    writeStored("height", "0");
    expect(readPositive("height")).toBeNull();
    expect(readPositive("none")).toBeNull();
  });

  it("survives storage that throws", () => {
    vi.stubGlobal("localStorage", {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("blocked");
      },
    });
    expect(readStored("a")).toBeNull();
    expect(() => saveJson("a", 1)).not.toThrow();
    expect(loadJson("a", { v: 2 })).toEqual({ v: 2 });
  });
});
