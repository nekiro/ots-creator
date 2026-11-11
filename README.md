<div align="center">

<img src=".github/assets/icon.png" width="128" height="128" alt="OTS Creator icon">

# OTS Creator

**Object and map editor for Open Tibia clients, from `Tibia.dat` / `Tibia.spr` to Tibia 12+ assets.**

[![Latest release](https://img.shields.io/github/v/release/nekiro/ots-creator?style=flat-square&color=e8c46a)](https://github.com/nekiro/ots-creator/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/nekiro/ots-creator/total?style=flat-square&color=d4930a)](https://github.com/nekiro/ots-creator/releases)
![Platforms](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-555?style=flat-square)
[![License](https://img.shields.io/badge/license-Apache%202.0%20%2B%20Commons%20Clause-555?style=flat-square)](LICENSE)

[Download](https://github.com/nekiro/ots-creator/releases/latest) ·
[Features](#features) ·
[Map editor](#map-editor) ·
[AI agents](#ai-agents-mcp) ·
[OTOBJ format](#otobj-object-format) ·
[OTMAP format](#otmap-map-format) ·
[Development](#development)

</div>

It looks and feels like the Tibia client and has a built-in market to share objects and
sprites. Built with [Wails v3](https://v3.wails.io) (Go) and Svelte 5.

![OTS Creator](.github/assets/promo.gif)

## Install

Download the latest build from [Releases](https://github.com/nekiro/ots-creator/releases).
Later versions install from the built-in updater.

- **Windows:** `otscreator-amd64-installer.exe`, or the portable `otscreator-windows-amd64.exe`.
- **macOS 12+:** `otscreator-darwin-arm64.zip` (Apple Silicon) or `otscreator-darwin-amd64.zip`
  (Intel). Move `otscreator.app` to Applications. The app is not notarized, so the first
  start is blocked: click "Open Anyway" in System Settings > Privacy & Security, or run
  `xattr -dr com.apple.quarantine /Applications/otscreator.app`.
- **Linux** (x86_64 and arm64, GTK 4 and WebKitGTK 6.0: Ubuntu 24.04+, Debian 13+, Fedora
  40+): `otscreator-linux-amd64.AppImage` / `otscreator-linux-arm64.AppImage`, made
  executable with `chmod +x`. The built-in updater replaces the AppImage file. The `.deb`
  and `.rpm` packages install the app system-wide instead; update them by installing the
  package of the next version.

## Features

### Clients

- **Every `Tibia.dat` / `Tibia.spr` version**, detected from the file signatures, with OTFI
  files, extended sprites, transparency, improved animations and frame groups.
- **Tibia 12+ assets:** `catalog-content.json`, protobuf `appearances.dat` and LZMA sprite
  sheets, with object names and descriptions. Unchanged files are written back byte for byte.
- **New clients and conversion** between versions and between dat/spr and assets
  ("Compile as"), with a warning for every flag the target cannot store.
- **Optional `items.otb` sync** for servers that map server ids to client ids with it (TFS,
  OTServ). Pick the file in Open client or in File > Server items; it is remembered per
  client. Compiling also updates it: server items follow moved client items, edited items
  get their client data reloaded (flags, light, minimap color, sprite hash…) and new client
  items get new server ids. Server ids and server-only data are kept, and an unchanged file
  is not rewritten.

### Object editing

- **Object list** with items, outfits, effects and missiles, search by id or name, and flag
  filters.
- **Live preview** of animations, directions, idle and walking groups, addons, mounts and
  outfit colors, with zoom and a pixel grid.
- **Properties** for layout, patterns, frame durations and every flag.
- **Sprites** per object or for the whole client: multi-select, replace, import, export, and
  drawing with a pixel grid, zoom and pan.
- Undo/redo, copy and paste of whole objects or only their properties, bulk edit,
  duplicate and remove.

### Import and export

- **Objects** as `.otobj` (the native format, see [below](#otobj-object-format)) or OBD from
  ObjectBuilder (import of versions 1 to 3, export as version 3). The export format is a
  setting. The object viewer opens both without a client.
- **Export everything** at once: all objects (one folder per category) or all sprites
  (single files or sheets).
- **Sprite sheets** of one frame group or a whole outfit. On import the layout (directions,
  mask layer, mounts, addons, frames) is detected from the image.
- **Drop** images, `.otobj` or `.obd` files anywhere, onto the preview or onto an object, or
  paste a PNG with Ctrl+V. A slicer cuts images into sprites.

### Compare and merge clients

- **Side by side** by category. Sprites are compared by pixels, not ids, and differences
  that only come from the client formats are ignored.
- **Every difference** shows both versions, animated on hover, and what changed (flags,
  size, patterns, frames, animation, sprites). An empty placeholder against a real object
  counts as new; a filter hides ids that are empty in the only client that has them.
- **Copy in both directions**, keeping ids or appending, or merge all of B into A. Objects
  of B can be queued (double click) and dropped into slots of A one by one, or all at once
  into free ids. Sprites with the same pixels under the same id are reused.
- Every copy is one undo step, copies that kept their ids can be reverted one by one, and
  the second client can be compiled from the same window.

### Tools

- Optimize sprites (duplicate, empty and unused ones), set frame durations in bulk, and
  split or merge outfit frame groups.
- Reorder objects: drag them to other ids, or cut and click a slot. They swap places, or
  are inserted and the objects between shift. Item ids stored in items (market, NPC
  currency) follow the moved items.

### Market

- Browse objects and sprite packs from other users: categories, tags, search, live preview
  and one-click import.
- Publish an object, sprites or an `.otobj` / `.obd` / image file. No account: pick a
  nickname, Tibia-style tags and a license, and delete your own entries later.

### Performance

- Heavy work (comparing, searching, optimizing, decoding Tibia 12+ sheets, exporting) uses
  every CPU core, so clients with hundreds of thousands of sprites stay fast.
- Long operations run in the background with progress in the status bar, and you can keep
  editing during an export. The export window can be hidden and reopened. **Stop** asks
  first and keeps the files written so far.
- Closing the app asks first while an export runs or when changes are not applied or
  compiled yet.

## Map editor

Switch between **Objects** and **Map** at the top left. Maps are drawn with the sprites of
the open client; object changes show after **Reload from client**.

- **Files:** OTBM versions 1 to 4 open and are saved as version 2 (what servers load), with
  houses, spawns (one file, or separate monster and NPC files), zones, towns and
  waypoints. The text format OTMAP (see [below](#otmap-map-format)) holds the same data
  in files that git can diff and merge. A map is saved in the format it was opened in;
  **Save as** picks the format by the extension (`.otbm` or `.otmap`). Several maps open
  in tabs.
- **Item ids:** server ids (translated with `items.otb`) or client ids, detected
  automatically. Names and types come from `items.xml`.
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

### Server item attributes

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

## AI agents (MCP)

AI agents such as Claude Code can build maps through an
[MCP](https://modelcontextprotocol.io) server. The agent starts OTS Creator by itself as a
command (**Settings > AI agents > Copy command** gives the line), for example:

```
claude mcp add ots-creator -- "C:\Users\you\AppData\Local\Programs\OTS Creator\otscreator.exe" mcp
```

This works with the editor closed: the agent opens the client and maps without a window
and saves them with `save_map`. To watch the agent work, turn on the MCP server in
**Settings > AI agents** (port 47800 by default; only programs on this computer can
connect): while the editor runs, the agent's edits go to it, show live and are undone like
your own. Agents that connect over HTTP can use `http://127.0.0.1:47800/mcp` directly.

Its tools cover everything a map needs:

- **Files:** open a client, set the server data, new, open, save, close and import maps,
  map properties, undo and redo.
- **Catalog:** brushes, tilesets, items (by name, id or flags) and creatures.
- **Drawing:** any brush at points or over a rectangle, batches of brushes as one undo
  step, and `paint_grid`, which paints an area from a text plan (one character per tile).
- **Looking:** `render_area` returns an image of an area drawn with the client's sprites,
  with a grid and coordinates; also a minimap image, a text listing of tiles, single tiles,
  find and statistics.
- **Details:** items and their attributes (action and unique ids, text, teleports,
  containers), tile flags, houses, towns, waypoints, zones, creatures and spawns.
- **Selection:** select, copy, cut, paste, delete, move, make a brush from it, and the
  map tools (borders, randomize, cleanup, replace items).

## OTOBJ object format

`.otobj` is a ZIP with a `manifest.json` and one PNG sprite sheet per frame group. Full
specification: [docs/otobj.md](docs/otobj.md).

```
manifest.json        category, name, flags by name, frame groups, animation, NPC trade
sprites/idle.png     sprite sheet in the ObjectBuilder layout
sprites/walking.png
```

Why not OBD:

- **Readable:** plain JSON you can read in a text editor and diff in git. OBD is one
  LZMA-compressed binary block.
- **Version independent:** flags are stored by name (`pickupable`, `hasLight`), not as the
  flag bytes of one dat version. On import the client keeps what it can store and warns
  about the rest.
- **Editable sprites:** normal PNG sheets in the ObjectBuilder layout.
- **Complete:** also stores names, descriptions, NPC trade, Tibia 12+ flags and 64 px sprites.
- **Extensible:** readers ignore unknown keys and files, so new fields need no new version.

Measured on the current CipSoft client (one file at a time, one core):

| | OBD | OTOBJ |
|---|---|---|
| Reading | 1x | 2 to 4x faster |
| Writing 1 000 items | 3.6 s | 0.26 s |
| Typical outfit (citizen) | 84 KB | 80 KB |
| Outfit with many colors | 134 KB | 386 KB |
| All 1 978 outfits | 72 MB | 111 MB |

Files can be up to 3 times larger, because LZMA compresses raw pixels better than PNG.
Sheets with at most 256 colors use a palette, which brings most outfits to the size of OBD.
OBD stays fully supported for ObjectBuilder, and the market keeps storing objects as OBD.

## OTMAP map format

OTMAP is a map as text: a world file and one file per 32×32 sector, one line per tile.
Full specification: [docs/otmap.md](docs/otmap.md).

```
map.otmap                  item ids, size, server attributes, towns, houses, zones, waypoints
sectors/1011_1007.sector   tiles, spawn areas and creatures of x 32352-32383, y 32224-32255
```

```
32353 32224 7 [pz] 4515 1209(aid=1001)
32354 32224 7 4515 1387(dest=32360,32230,8)
32358 32224 7 4515 1739(uid=5000 chest_quest_number=7){3031:100 2853{3046}}
spawn 32362 32228 7 radius=3
monster 32362 32228 7 "Rat" spawntime=60
```

Why not OTBM:

- **Diffable:** an edited tile is one changed line, with its world position. OTBM is one
  binary file that changes as a whole.
- **Mergeable:** edits to different tiles merge in git without conflicts.
- **One place:** houses, spawns, zones and server attributes are in the same folder and
  syntax, not in XML and JSON side files.
- **Canonical:** opening and saving writes the same bytes, so a save without changes gives
  an empty diff. Unchanged sector files are not rewritten.
- **Lossless:** OTBM → OTMAP → OTBM keeps every tile, item, attribute, house, town, zone,
  waypoint and spawn.

Measured on an 8.0 map with 8 million tiles (all cores):

| | OTBM | OTMAP |
|---|---|---|
| Open | 1.1 s | 0.36 s |
| Save | – | 1.2 s, 0.1 s without changes |
| On disk | 75 MB | 182 MB in 2 155 files |
| Git repository after the first commit | 19.4 MiB | 23.5 MiB |

Keep LF line endings in the repository that holds the map (`.gitattributes`):

```
*.otmap  text eol=lf
*.sector text eol=lf
```

## Development

Requirements: Go 1.26+, Node 22+, `wails3` CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`).
On Linux also a C compiler and the GTK 4 / WebKitGTK 6.0 headers (Debian/Ubuntu:
`sudo apt install build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev`).

```sh
wails3 task dev        # run with hot reload
wails3 task build      # production build into bin/
wails3 task test       # Go + frontend unit tests
```

## License

Apache License 2.0 with the Commons Clause (see `LICENSE` and `NOTICE`): you may use,
modify and share OTS Creator, but you may not sell it. Derived works must keep the
copyright notices and credits.

Copyright (c) 2026 Marcin Jałocha (nekiro)
