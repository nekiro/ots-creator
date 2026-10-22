import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MapService, type ItemTable } from "./data/api";
import { loadMapData, map } from "./state.svelte";

type View = Awaited<ReturnType<typeof MapService.ViewData>>;
type Status = Awaited<ReturnType<typeof MapService.Status>>;

function view(mode: "otb" | "client"): View {
  return {
    data: { ready: true, mode, client: "test", config: {} },
    items: {
      ids: [mode === "otb" ? 1000 : 100], clients: [100], flags: [0], tops: [0],
      minimap: [0], names: ["Ground"], kinds: [0], speeds: [150],
      mode, maxId: mode === "otb" ? 1000 : 100, flagbits: {},
    } satisfies ItemTable,
    palette: { tilesets: [], brushes: [], creatures: [], doorTypes: [] },
  } as unknown as View;
}

beforeEach(() => {
  map.activeId = 1;
  map.data = null;
  map.items = null;
  map.palette = null;
  map.brush = null;
  map.pasting = false;
  vi.spyOn(MapService, "Status").mockResolvedValue({
    data: { ready: true, mode: "otb" },
    maps: [{ id: 1, itemMode: "otb" }, { id: 2, itemMode: "client" }],
    recent: [], keys: {}, settings: {},
  } as unknown as Status);
});
afterEach(() => vi.restoreAllMocks());

describe("map tab item modes", () => {
  it("uses the selected map's client IDs instead of the shared server defaults", async () => {
    map.activeId = 2;
    const data = vi.spyOn(MapService, "ViewData").mockResolvedValue(view("client"));
    await loadMapData(true);
    expect(data).toHaveBeenCalledWith(2, true);
    expect(map.data?.mode).toBe("client");
    expect(map.items?.client[100]).toBe(100);
    expect(map.items?.has(1000)).toBe(false);
  });

  it("resets server-ID brush and paste state after conversion", async () => {
    const data = vi.spyOn(MapService, "ViewData").mockResolvedValue(view("otb"));
    await loadMapData(true);
    map.brush = { kind: "raw", item: 1000, label: "Ground" };
    map.pasting = true;
    data.mockResolvedValue(view("client"));
    await loadMapData();
    expect(map.items?.mode).toBe("client");
    expect(map.brush).toBe(null);
    expect(map.pasting).toBe(false);
  });

  it("ignores an older tab response that arrives after a new tab loads", async () => {
    let finish!: (result: View) => void;
    const pending = new Promise<View>((resolve) => (finish = resolve));
    const data = vi.spyOn(MapService, "ViewData").mockImplementation((id) =>
      (id === 1 ? pending : Promise.resolve(view("client"))) as ReturnType<typeof MapService.ViewData>,
    );
    const oldLoad = loadMapData(true);
    await vi.waitFor(() => expect(data).toHaveBeenCalledWith(1, true));
    map.activeId = 2;
    await loadMapData(true);
    finish(view("otb"));
    await oldLoad;
    expect(map.activeId).toBe(2);
    expect(map.data?.mode).toBe("client");
    expect(map.items?.mode).toBe("client");
  });
});
