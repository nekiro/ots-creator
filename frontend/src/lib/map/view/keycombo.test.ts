import { describe, expect, it } from "vitest";
import { comboOf } from "./keycombo";

const press = (key: string, mods: Partial<Record<"ctrlKey" | "metaKey" | "altKey" | "shiftKey", boolean>> = {}) =>
  comboOf({ key, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, ...mods });

describe("comboOf", () => {
  it("names letters in upper case with modifiers in a fixed order", () => {
    expect(press("s")).toBe("S");
    expect(press("S", { ctrlKey: true, shiftKey: true })).toBe("Ctrl+Shift+S");
    expect(press("k", { metaKey: true })).toBe("Ctrl+K");
    expect(press("b", { altKey: true, ctrlKey: true })).toBe("Ctrl+Alt+B");
  });

  it("leaves Shift out of symbols", () => {
    expect(press("+", { shiftKey: true })).toBe("+");
    expect(press("!", { shiftKey: true })).toBe("!");
    expect(press("0", { ctrlKey: true })).toBe("Ctrl+0");
  });

  it("keeps Shift for named keys", () => {
    expect(press("PageUp", { shiftKey: true })).toBe("Shift+PageUp");
    expect(press("Delete")).toBe("Delete");
    expect(press(" ")).toBe("Space");
  });

  it("ignores modifiers alone", () => {
    expect(press("Control", { ctrlKey: true })).toBeNull();
    expect(press("Shift", { shiftKey: true })).toBeNull();
    expect(press("Dead")).toBeNull();
  });
});
