import { describe, expect, it } from "vitest";
import { labelText, wrapText } from "./itemText";

describe("wrapText", () => {
  it("keeps a short text on one line", () => {
    expect(wrapText("Welcome to Thais")).toEqual(["Welcome to Thais"]);
  });

  it("wraps at the last word that fits the width", () => {
    expect(wrapText("one two three four", 9)).toEqual(["one two", "three", "four"]);
  });

  it("keeps the text's own line breaks as paragraphs", () => {
    expect(wrapText("first line\r\nsecond line")).toEqual(["first line", "second line"]);
  });

  it("drops empty paragraphs and repeated spaces", () => {
    expect(wrapText("a   b\n\n\nc")).toEqual(["a b", "c"]);
  });

  it("splits a word longer than a line", () => {
    expect(wrapText("abcdefghij", 4)).toEqual(["abcd", "efgh", "ij"]);
  });

  it("ends the last kept line with an ellipsis when lines are cut", () => {
    expect(wrapText("a\nb\nc\nd", 40, 2)).toEqual(["a", "b…"]);
  });

  it("drops trailing punctuation before the ellipsis", () => {
    expect(wrapText("stop here, \nmore", 40, 1)).toEqual(["stop here…"]);
  });
});

describe("labelText", () => {
  it("keeps a first line that fits", () => {
    expect(labelText("  Depot  \nsecond line")).toBe("Depot");
  });

  it("cuts a long line at the last word in its second half", () => {
    expect(labelText("Welcome to the city of Thais")).toBe("Welcome to the…");
  });

  it("cuts inside a word when no space is in the second half", () => {
    expect(labelText("Supercalifragilistic word", 10)).toBe("Supercalif…");
  });

  it("drops trailing punctuation before the ellipsis", () => {
    expect(labelText("Hello, there friend", 7)).toBe("Hello…");
  });
});
