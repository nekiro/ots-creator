import { describe, expect, it } from "vitest";
import { basename, dirname } from "./path";

describe("path", () => {
  it("splits Windows paths", () => {
    expect(basename("C:\\a\\Tibia.dat")).toBe("Tibia.dat");
    expect(dirname("C:\\a\\Tibia.dat")).toBe("C:\\a");
  });

  it("splits slash paths", () => {
    expect(basename("/a/b/Tibia.dat")).toBe("Tibia.dat");
    expect(dirname("/a/b/Tibia.dat")).toBe("/a/b");
  });
});
