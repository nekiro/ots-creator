# Map editor

Switch between **Objects** and **Map** at the top left. Maps are drawn with the sprites of
the open client; object changes show after **Reload from client**.

- **Files:** OTBM versions 1 to 4 open and are saved as version 2 (what servers load), with
  houses, spawns (one file, or separate monster and NPC files), zones, towns and
  waypoints. The text format [OTMAP](otmap.md) holds the same data
  in files that git can diff and merge. A map is saved in the format it was opened in;
  **Save as** picks the format by the extension (`.otbm` or `.otmap`). Several maps open
  in tabs.
- **Item ids:** each map uses server ids (translated with the `items.otb` attached to the
  client, the same file compiling keeps in sync) or client ids. Opening or creating a map
  shows which one and why (the `.otmap` file says it, the earlier choice for the map, an
  `items.otb` attached to the client or found next to the map); picking an `items.otb`
  there attaches it to the client. Names and types come from `items.xml`.
- **Brushes:** grounds with automatic borders, walls with doors and windows, doodads,
  carpets, tables, houses, creatures and spawns, waypoints, zones, PZ / no-PvP / no-logout
  flags and an eraser. Size, square or circle shape, Shift+drag fills a rectangle, and any
  item can be placed as is.
- **Brush files** use the editor's own JSON format and are edited in the app. Brushes,
  tilesets and creatures of Remere's Map Editor are imported from its data folder, and a
  selection becomes a doodad brush.
- **Layout:** a **Library** of everything to draw with (terrain, decoration, items,
  creatures, one search), a **World** outline of towns, houses, waypoints and areas, and an
  inspector for whatever you click (tile, house, town, or the map and its statistics).
  Changes apply as you type.
- **Ctrl+K** goes to anything: a position (pasted in any format), a town, house, waypoint,
  brush, item, creature or command.
- **Selection** over one or several floors, with a floating bar (copy, move between floors,
  redo borders, find, make a brush) and an undo history you can jump back in.
- **Find and tools:** a find panel with replace and remove; map tools (unknown items,
  corpses, empty spawns, spawn times, unseen tiles, borders) on the selection or the whole map; import
  of another map at an offset; minimap export.
- **Big maps:** the backend builds the drawing data of each 32×32 chunk once and the GPU
  keeps it (WebGL2), so panning and zooming only move the camera and only animated sprites
  update per frame. Plain tiles are stored packed, so maps with millions of tiles stay
  small in memory.
- **Map together:** several people edit one map at the same time. See
  [map-together.md](map-together.md).

## Server item attributes

Some servers add their own item attributes to OTBM (for example quest numbers on chests
and doors). The editor cannot skip an attribute it does not know, because OTBM does not
store attribute sizes: it warns and drops that attribute and the ones after it on the item.

To keep them, put `<map>-attributes.json` next to the map (`world-attributes.json` for
`world.otbm`) and list each attribute with its code and type:

```json
[
  { "code": 25, "name": "Door quest number", "type": "u16" },
  { "code": 26, "name": "Door quest value", "type": "i32" },
  { "code": 28, "name": "Chest quest number", "type": "u16" }
]
```

Types: `u8`, `u16`, `u32`, `i32`, `u64` (little-endian, as the server writes them) and
`string` (uint16 length, then the text). The file is read when the map opens; listed
attributes are kept unchanged and written back when the map is saved. Codes of standard
OTBM attributes (1 to 24 and 128) cannot be redefined. In OTBM, `name` is only for you.

An OTMAP map declares these attributes in its world file instead, and its sector files
write them by a key made from `name` (`door_quest_number=12`). Saving an OTMAP map as OTBM
writes the attributes file when it does not exist yet.
