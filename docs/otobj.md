# OTOBJ object format, version 1

`.otobj` is the native single-object format of OTS Creator. It holds one client object (an
item, outfit, effect or missile) with its properties, layout, animation and sprites, in a
form that people can read and edit and that does not depend on a client version.

OBD (ObjectBuilder) stays supported for import and export. `.otobj` is the default for
exports because of the reasons below.

## Why not OBD

| | OBD v3 | OTOBJ v1 |
|---|---|---|
| Container | one LZMA stream of binary data | ZIP with JSON and PNG files |
| Properties | dat flag bytes of one client version | flag names (`pickupable`, `hasLight`) |
| Sprites | raw ARGB pixels per sprite | PNG sprite sheets, open in any image editor |
| Tibia 12+ data (names, NPC trade, new flags, 64 px sprites) | no | yes |
| Readable in a text editor or a git diff | no | manifest yes, sheets as images |
| New fields | a new file version that older tools reject | new keys that older readers ignore |

Measured on the current CipSoft client (one core, one file at a time):

| Sample | OBD | OTOBJ | Read OBD -> OTOBJ |
|---|---|---|---|
| citizen (outfit 128, 1 728 sprites) | 84 KB | 80 KB | 34 -> 16 ms |
| biggest outfit (3 072 sprites) | 134 KB | 386 KB | 52 -> 23 ms |
| items 3000-3999 (1 000 files) | 1.4 MB | 2.7 MB | 132 -> 79 ms |
| all outfits (1 978 files) | 72 MB | 111 MB | 30.4 -> 7.2 s |

- Reading is 2 to 4 times faster: decoding PNG is cheaper than LZMA.
- Writing is about as fast for big objects and much faster for many small ones (1 000 items:
  3.6 s as OBD, 0.26 s as OTOBJ), because every OBD file starts a new LZMA encoder.
- Files are 1 to 3 times larger. LZMA on raw pixels compresses better than PNG. Sheets with
  at most 256 colors use a palette, which brings typical outfits to the size of OBD. The
  difference is a few kilobytes per object, and in exchange the sprites can be edited
  directly.

## Container

A ZIP archive (no encryption, no ZIP64 needed). Entries use Deflate or Store; writers store
PNG files without compression because PNG is already compressed.

| Entry | Required | Content |
|---|---|---|
| `manifest.json` | yes | the object, UTF-8 JSON (below) |
| `sprites/<name>.png` | one per frame group | sprite sheet of the group |

Readers ignore entries they do not know, so later versions can add files (for example
previews) without breaking older readers.

## manifest.json

```json
{
  "format": "otobj",
  "version": 1,
  "generator": "OTS Creator 1.4.0",
  "category": "outfit",
  "spriteSize": 32,
  "source": { "client": 1098, "id": 128 },
  "name": "",
  "description": "",
  "flags": { "hasLight": true, "lightLevel": 3, "lightColor": 215, "animateAlways": true },
  "npcSales": [],
  "frameGroups": [
    {
      "type": "idle",
      "width": 2, "height": 2, "exactSize": 64,
      "layers": 2, "patternX": 4, "patternY": 3, "patternZ": 2, "frames": 1,
      "sheet": "sprites/idle.png"
    },
    {
      "type": "walking",
      "width": 2, "height": 2, "exactSize": 64,
      "layers": 2, "patternX": 4, "patternY": 3, "patternZ": 2, "frames": 8,
      "animation": {
        "mode": "async", "loopCount": 0, "startFrame": 0,
        "durations": [{ "min": 100, "max": 100 }]
      },
      "sheet": "sprites/walking.png"
    }
  ]
}
```

| Key | Type | Meaning |
|---|---|---|
| `format` | string | always `"otobj"` |
| `version` | integer | format version; this document describes 1 |
| `generator` | string | optional, the tool that wrote the file |
| `category` | string | `item`, `outfit`, `effect` or `missile` |
| `spriteSize` | integer | edge of one sprite in pixels: 32 or 64 |
| `source` | object | optional and informative: `client` version and object `id` it came from |
| `name`, `description` | string | optional; stored by Tibia 12+ clients |
| `flags` | object | properties that are set (below) |
| `npcSales` | array | optional NPC trade entries (below) |
| `frameGroups` | array | 1 group, or 2 for outfits (idle, then walking) |

### flags

Keys are the property names of OTS Creator (the same as in its JSON and in the flag
filter). Only properties that differ from their default (`false`, `0`, `""`) are written.
A payload belongs to the flag that enables it, for example `lightLevel` and `lightColor`
to `hasLight`, or `market` (an object with `name`, `category`, `tradeAs`, `showAs`,
`restrictProfession`, `restrictLevel`) to `isMarket`.

Readers ignore keys they do not know. When the client an object is imported into cannot
store a flag, the flag is dropped with a warning, as when compiling to another version.

### npcSales

Each entry: `name`, `location` (strings), `salePrice`, `buyPrice` (what the player pays and
what the NPC pays; 0 = no trade that way), `currencyObjectId` (0 = gold) and
`currencyQuestFlag` (name of a quest currency).

### frameGroups

| Key | Meaning |
|---|---|
| `type` | `idle` or `walking` (walking only as the second group of an outfit) |
| `width`, `height` | size of a texture in sprites, 1 to 8 |
| `exactSize` | exact size in pixels (client value) |
| `layers` | 1, or 2 for outfits with a color mask |
| `patternX`, `patternY`, `patternZ` | patterns (outfits: directions, addons, mount) |
| `frames` | animation frames, 1 to 255 |
| `animation` | only when `frames` > 1: `mode` (`async` or `sync`), `loopCount` (0 = forever, -1 = ping-pong), `startFrame` (-1 = random) and one `durations` entry per frame in milliseconds |
| `sheet` | name of the PNG entry with the sprites |

## Sprite sheets

The layout is the one of ObjectBuilder sheets, so sheets can be exchanged with it:

- Columns: `patternZ x patternX x layers` textures, rows: `frames x patternY` textures.
- A texture is `width x height` sprites. Sprite (0, 0) of a texture is its bottom-right
  sprite, as in the client.
- The sheet is exactly `columns x width x spriteSize` by `rows x height x spriteSize`
  pixels.
- Empty pixels are fully transparent. Magenta has no special meaning.

Writers use an 8-bit palette (with transparency) when the sheet has at most 256 colors,
and RGBA otherwise. Readers accept any PNG color type.

## Reading rules

- Reject the file when `format` is not `"otobj"`, when `version` is higher than the
  reader supports, when an entry named by `sheet` is missing or when a sheet does not have
  the size its layout needs.
- Ignore unknown keys and entries.
- Limit sizes: at most 4 096 sprites per frame group, and at most 64 MiB per decompressed
  entry.

## Versioning

`version` changes only when an older reader would read a file wrongly. Adding keys,
flags or entries does not change it.
