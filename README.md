# OTS Creator

Object editor for Open Tibia clients, from classic `Tibia.dat` / `Tibia.spr` to Tibia 12+
protobuf assets. It looks and feels like the Tibia client, and it has a built-in market to
share objects and sprites with other users. Built with [Wails v3](https://v3.wails.io) (Go)
and Svelte 5.

![OTS Creator](.github/assets/promo.gif)

## Features

**Clients**

- Open and compile `Tibia.dat` / `Tibia.spr` for every client version. The version is
  detected from the file signatures, and OTFI files are supported. Extended sprites,
  transparency, improved animations and frame groups are all handled.
- Tibia 12+ assets: `catalog-content.json`, `appearances.dat` (protobuf) and LZMA sprite
  sheets, including object names and descriptions. Files are written back byte for byte
  when nothing changed.
- Create new clients, and convert between versions and between dat/spr and assets
  ("compile as"). Flags the target version cannot store are reported as warnings.

**Editing**

- Object list with categories (items, outfits, effects, missiles), search by id or name,
  and filters by flag.
- Live preview: animation, directions, idle and walking groups, addons, mounts, outfit
  colors, zoom and pixel grid.
- Properties editor for layout, patterns, frame durations and every flag, with a flag
  filter.
- Sprite list per object or for the whole client, with multi-select, replace, import and
  export.
- Undo/redo, copy and paste of whole objects or only their properties, bulk edit of
  several objects, and duplicate and remove.

**Import and export**

- OBD (ObjectBuilder) import of versions 1 to 3 and export as version 3, with an OBD
  viewer that works without an open client.
- Sprite sheets: export one frame group or the whole outfit. On import the layout is
  detected from the image (directions, mask layer, mounts, addons, frames).
- Drop images or OBD files anywhere, onto the preview or onto an object in the list, or
  paste a PNG with Ctrl+V.
- Slicer to cut images into sprites.

**Tools**

- Optimize sprites (duplicates, empty and unused ones), bulk frame durations, and split
  or merge outfit frame groups.

**Market**

- Browse objects and sprite packs shared by other users, with categories, tags, search,
  a live preview and one-click import.
- Share an object, sprites or an `.obd` / image file from disk. No account is needed: you
  pick a nickname, Tibia-style tags and a license. You can later delete your own entries.

**Updates**

- Built-in updater from GitHub releases.

## Development

Requirements: Go 1.26+, Node 22+, `wails3` CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`).

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
