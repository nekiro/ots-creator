import { describe, expect, it } from "vitest";
import { PREVIEW_DROP, classifyDrop, thingDropId } from "./classifyDrop";

const open = { open: true, focused: true };
const closed = { open: false, focused: false };

describe("classifyDrop", () => {
  it("takes one image on the preview as a sheet of the focused object", () => {
    expect(classifyDrop(["C:/a/sheet.png"], PREVIEW_DROP, open)).toEqual({ kind: "previewSheet", path: "C:/a/sheet.png" });
  });

  it("imports an image dropped on the preview as a sprite when nothing is focused", () => {
    expect(classifyDrop(["sheet.png"], PREVIEW_DROP, { open: true, focused: false })).toEqual({ kind: "importImages", paths: ["sheet.png"] });
  });

  it("takes one image on an object as that object's sheet", () => {
    expect(classifyDrop(["sheet.JPG"], thingDropId(1234), open)).toEqual({ kind: "thingSheet", path: "sheet.JPG", id: 1234 });
  });

  it("imports several images as sprites", () => {
    expect(classifyDrop(["a.png", "b.gif", "notes.txt"], PREVIEW_DROP, open)).toEqual({ kind: "importImages", paths: ["a.png", "b.gif"] });
  });

  it("imports object files, before images", () => {
    expect(classifyDrop(["a.obd", "b.otobj", "c.png"], "", open)).toEqual({ kind: "importObjects", paths: ["a.obd", "b.otobj"] });
  });

  it("views objects and images without an open client", () => {
    expect(classifyDrop(["a.png", "b.obd"], "", closed)).toEqual({ kind: "view", path: "b.obd" });
    expect(classifyDrop(["a.png"], PREVIEW_DROP, closed)).toEqual({ kind: "view", path: "a.png" });
  });

  it("opens client files and folders", () => {
    expect(classifyDrop(["C:/tibia/Tibia.dat"], "", open)).toEqual({ kind: "openClient", path: "C:/tibia/Tibia.dat" });
    expect(classifyDrop(["C:/tibia/assets/catalog-content.json"], "", closed)).toEqual({ kind: "openClient", path: "C:/tibia/assets/catalog-content.json" });
    expect(classifyDrop(["C:/clients/860"], "", closed)).toEqual({ kind: "openClient", path: "C:/clients/860" });
  });

  it("rejects other files", () => {
    expect(classifyDrop(["notes.txt"], "", open)).toEqual({ kind: "unknown" });
  });
});
