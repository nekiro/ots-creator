# OTMAP map format, version 1

OTMAP is the text map format of OTS Creator. A map is a world file and one file per 32x32
sector, all UTF-8 text. It holds the same data as OTBM with its house, spawn, zone and
attributes files, in a form that `git diff` can show and `git merge` can combine.

The editor opens both formats and saves a map in the format it was opened in. "Save as" picks
the format by the extension of the file name: `.otbm` or `.otmap`.

## Why not OTBM

| | OTBM | OTMAP v1 |
|---|---|---|
| Files | one binary map plus XML and JSON side files | one world file plus text sector files |
| One edited tile in git | the whole map file changes | one line of one sector file changes |
| Edits in different areas | always a merge conflict | merged by git |
| Review | not readable | the diff shows the position and the items |
| Server attributes (quest numbers...) | numeric codes, sizes in a side JSON | named keys declared in the world file |

Measured on a real 8.0 map (8 million tiles, 75 MB OTBM), on all cores:

| | OTBM | OTMAP |
|---|---|---|
| Open | 1.1 s | 0.36 s |
| Save | – | 1.2 s, 0.1 s when nothing changed |
| Size on disk | 75 MB | 182 MB in 2 155 sector files, largest 403 KB |
| Git repository after the first commit | 19.4 MiB | 23.5 MiB |

The text is about 2.4 times larger on disk, but git compresses it to about the size of the
OTBM. After that, every commit stores only the changed sectors instead of a new copy of the
whole map.

## Directory layout

```
world/
  map.otmap              world file: metadata, attributes, towns, houses, zones, waypoints
  sectors/
    1004_972.sector      sector x = floor(x / 32), sector y = floor(y / 32)
    1004_973.sector
    ...
```

- To open a map, open its world file. Any name with the `.otmap` extension works.
- The sector files are in the `sectors` folder next to the world file. Keep one map per
  folder.
- A sector file holds all 16 floors of its 32x32 area.
- The file name is the only source of the sector coordinate. Sector files do not repeat it.
- A sector with no records has no file. Saving deletes the `*.sector` files of sectors that
  became empty and keeps all other files in the folder.
- Saving does not rewrite a file whose content did not change.

## Lexical rules

These rules apply to the world file and to sector files.

- One record per line. Every line, the last one too, ends with LF.
- Canonical files contain no empty lines, no comments, no trailing spaces, and one space
  between tokens. Readers also accept CRLF, empty lines, and runs of spaces or tabs between
  tokens. The next save writes the canonical form.
- `uint` is an unsigned decimal integer. `int` can start with `-`.
- `position` is `X,Y,Z` with no spaces.
- `name` is `[a-z][a-z0-9_]*`.
- `string` is enclosed in double quotes and holds bytes, like OTBM strings. Valid UTF-8 is
  written as it is. Escapes: `\\`, `\"`, `\n`, and `\xHH` for one byte. Writers use `\xHH`
  for control characters and for bytes that are not valid UTF-8 (Latin-1 text of old
  servers), so every byte string reads back unchanged.

## World file

### Grammar

```
world-file   = header LF *(record LF)
header       = "otmap 1"

record       = items / otb / size / description / spawnformat / attribute
             / town / house / zone / waypoint
items        = "items" SP ("server" / "client")
otb          = "otb" SP uint SP uint                     ; items.otb major, minor
size         = "size" SP uint SP uint                    ; width, height
description  = "description" SP string
spawnformat  = "spawnformat" SP ("single" / "separate")
attribute    = "attribute" SP uint SP name SP attr-type [SP string]
attr-type    = "u8" / "u16" / "u32" / "u64" / "i32" / "string"
town         = "town" SP uint SP string SP "temple=" position
house        = "house" SP uint SP string SP "town=" uint SP "exit=" position
               *(SP house-attr)
house-attr   = "rent=" int / "guildhall" / "beds=" uint / "clientid=" uint
zone         = "zone" SP uint SP string
waypoint     = "waypoint" SP string SP position
```

### Records

| Record | Meaning |
|---|---|
| `items server` | Item IDs are server IDs (items.otb), as in OTBM. Required. |
| `items client` | Item IDs are client IDs (servers without items.otb). |
| `otb` | Version of items.otb. |
| `size` | Map width and height. Without it, the reader uses the bounds of the tiles. |
| `description` | Map description. Several records are joined with a newline. |
| `spawnformat` | `single`: monsters and NPCs in one spawn file. `separate`: two files. Default `single`. |
| `attribute` | A server item attribute: OTBM code, key in sector files, type, optional display name. |
| `town` | Town ID, name, temple position. |
| `house` | House ID, name, town, exit position, rent, guildhall, beds, client ID. House tiles come from `house=N` on tiles, so the size is not stored. |
| `zone` | Zone ID and name. Zone tiles come from `zone=` on tiles. |
| `waypoint` | Waypoint name and position. |

`attribute` replaces `<map>-attributes.json`. The JSON entry
`{"code": 25, "name": "Door quest number", "type": "u16"}` becomes
`attribute 25 door_quest_number u16 "Door quest number"`, and sector files write
`door_quest_number=12`. The key is the name in lowercase with `_` between words, or
`attr_<code>` when that is empty, taken, or a standard key. The display name is written only
when it differs from the key. Codes 0-24 and 128 are standard OTBM attributes and cannot be
declared.

Saving an OTMAP map as OTBM writes `<map>-attributes.json` when the map declares attributes
and the file does not exist yet.

### Order

`otmap 1`, `items`, `otb`, `size`, `description`, `spawnformat`, then `attribute` by code,
`town` by ID, `house` by ID, `zone` by ID, `waypoint` by name, then position.

### Example

```
otmap 1
items server
otb 3 20
size 34000 34000
description "Rookgaard and the mainland"
attribute 25 door_quest_number u16 "Door quest number"
attribute 26 door_quest_value i32 "Door quest value"
attribute 28 chest_quest_number u16 "Chest quest number"
town 1 "Thais" temple=32369,32241,7
house 1 "Spiritkeep" town=1 exit=32265,32316,7 rent=19210 guildhall beds=23
zone 1 "Arena"
waypoint "thais-temple" 32369,32241,7
```

## Sector file

### Grammar

```
sector-file   = header LF *(record LF)
header        = "otmap-sector 1"
record        = tile / area / creature

tile          = x SP y SP z [SP tile-attrs] SP ground *(SP item)
ground        = "-" / item                    ; "-" = tile without ground
tile-attrs    = "[" tile-attr *(SP tile-attr) "]"
tile-attr     = "pz" / "nopvp" / "nologout" / "pvpzone" / "refresh"
              / "flags=" uint                 ; flag bits without a name
              / "house=" uint
              / "zone=" uint *("," uint)

item          = id [":" subtype] [item-attrs] [contents]
item-attrs    = "(" item-attr *(SP item-attr) ")"
item-attr     = std-attr / declared-attr / map-attr
std-attr      = "aid=" uint / "uid=" uint / "depot=" uint / "door=" uint
              / "dest=" position / "text=" string / "desc=" string
              / "writer=" string / "written=" uint / "duration=" uint
              / "decay=" uint / "sleeper=" uint / "sleepstart=" uint
declared-attr = name "=" (int / string)       ; key from an "attribute" record
map-attr      = "@" (name / string) "=" map-value
map-value     = int / double / float / "true" / "false" / string
double        = number with "." or an exponent, or "nan", "inf", "-inf"
float         = double "f"
contents      = "{" item *(SP item) "}"

area          = ("spawn" / "npcspawn") SP x SP y SP z SP "radius=" uint
creature      = ("monster" / "npc") SP x SP y SP z SP string *(SP creature-attr)
creature-attr = "spawntime=" int / "dir=" ("n" / "e" / "s" / "w") / "weight=" uint

x, y          = uint                          ; world coordinates, 0-65535
z             = uint                          ; floor, 0-15
id            = uint                          ; item ID, 1-65535
subtype       = uint                          ; 0-65535
```

Coordinates are world coordinates, not offsets in the sector, so a diff line shows the real
position.

### Tiles

`x y z [tile attributes] ground items...`

- The ground is always the first item token. `-` means the tile has no ground.
- Items follow in stack order, bottom first.
- Editor-only (meta) items are not written, as in OTBM.
- A tile with no ground, no items, no flags, no house and no zones has no tile record.

| Tile attribute | OTBM |
|---|---|
| `pz` | flag 1 |
| `nopvp` | flag 4 |
| `nologout` | flag 8 |
| `pvpzone` | flag 16 |
| `refresh` | flag 32 |
| `flags=N` | all other flag bits, as a number |
| `house=N` | house tile of house N |
| `zone=A,B` | zone IDs of the tile, in their order |

### Items

`id[:subtype](attributes){contents}`

- `subtype` is the count, charges or fluid type. It is omitted when 0. It is 16 bits, so
  servers with 16-bit counts lose nothing.
- `contents` lists the items inside a container, in slot order. Contents nest up to 64
  levels. An empty container has no `{}`.

| Item attribute | Type | OTBM attribute |
|---|---|---|
| `aid` | u16 | action ID (4) |
| `uid` | u16 | unique ID (5) |
| `depot` | u16 | depot ID (10) |
| `door` | u8 | house door ID (14) |
| `dest` | position | teleport destination (8) |
| `text` | string | text (6) |
| `desc` | string | description (7) |
| `writer` | string | written by (19) |
| `written` | u32 | written date (18) |
| `duration` | u32 | duration (16) |
| `decay` | u8 | decaying state (17) |
| `sleeper` | u32 | sleeper GUID (20) |
| `sleepstart` | u32 | sleep start (21) |
| declared key | declared type | code of the `attribute` record |
| `@key` | int, double, float, bool, string | attribute map (128) |

`@key` writes the key as a name when it is one, otherwise as a string: `@rarity=2`,
`@"Custom Key"="x"`. Numbers: `5` is a 32-bit integer, `5.0` a double, `5.0f` a float.

### Spawn areas and creatures

`spawn` and `npcspawn` give the center and radius of a spawn area. `monster` and `npc` give
a creature at its own position. Servers group them the way their spawn files do:

- An area covers the tiles with `|dx| <= radius`, `|dy| <= radius` on its floor.
- A monster belongs to the first `spawn` area that covers it. Areas are ordered by
  `(z, y, x)` of their center over the whole map.
- An NPC belongs to the first `npcspawn` area that covers it. With `spawnformat single`, it
  uses the `spawn` areas too.
- An area and its creatures can be in different sector files.
- A creature that no area covers stays in OTMAP. Saving as OTBM cannot write it and reports
  it.

What the radius means is up to the server. CipSoft-style servers respawn monsters on a free
tile within the radius of the center, and NPCs use it as their walking range.

| Creature attribute | Default | Meaning |
|---|---|---|
| `spawntime=N` | 0 | Respawn time in seconds |
| `dir=D` | `n` | Look direction |
| `weight=N` | 0 | Spawn weight (separate spawn files) |

### Order

1. Header.
2. Tiles by `(z, y, x)`.
3. Areas by `(z, y, x)`, `spawn` before `npcspawn`.
4. Creatures by `(z, y, x)`. Creatures on one tile keep their order.

### Example

`sectors/1011_1007.sector`:

```
otmap-sector 1
32352 32224 7 [pz] 4515
32353 32224 7 [pz] 4515 1209(aid=1001)
32354 32224 7 4515 1387(dest=32360,32230,8)
32355 32224 7 4515 3031:50
32356 32224 7 [house=12] 4515 1213(door=1)
32357 32224 7 4515(aid=2000)
32358 32224 7 4515 1739(uid=5000 chest_quest_number=7){3031:100 2853{3046}}
32359 32224 7 4515 1430(text="Welcome to \"Thais\".\nBe nice." writer="Sam" written=1700000000)
32360 32224 7 4515 1223(door_quest_number=12 door_quest_value=1 @rarity=2)
32352 32226 8 [nologout] -
npcspawn 32355 32226 7 radius=1
spawn 32362 32228 7 radius=3
npc 32355 32226 7 "Sam" dir=e
monster 32362 32228 7 "Rat" spawntime=60
monster 32364 32229 7 "Cave Rat" spawntime=90 dir=s
```

## Reading rules

Readers report errors as `file:line:column: message`, for example
`sectors/0_0.sector:3:1: tile 1,1,7 written twice`.

Errors (the map does not open):

- A missing or unknown header, an unsupported version, or no `items` record.
- A syntax error, a value out of range, or `z` above 15.
- A record outside the sector of its file name.
- Two tiles at one position, or two areas of one kind at one position.
- An attribute that is neither standard, declared, nor `@`.
- A town, house, zone or attribute defined twice.

Warnings (the map opens, the next save writes it right):

- Records out of order. The reader does not need the order.
- A house tile of a house that the world file does not define. The house gets a default
  record, as with OTBM.
- A zone, or the town of a house, that the world file does not define.
- A waypoint name used twice.
- `items server` or `items client` that does not match the loaded item data.

A server that loads OTMAP adds its own checks: unknown item IDs, a ground item that is not
first, contents in an item that is not a container, unknown creature names, duplicate
unique IDs.

## Canonical form

Open and save writes identical bytes. Writers:

- write LF and single spaces;
- write records in the order above;
- write tile attributes in the order `pz nopvp nologout pvpzone refresh flags house zone`;
- write item attributes in the order of the item attribute table, then declared attributes
  by code, then `@` attributes by key;
- write creature attributes in the order `spawntime dir weight`;
- omit default values: `subtype` 0, `spawntime` 0, `dir` n, `weight` 0, `rent` 0, `beds` 0,
  `clientid` 0, `spawnformat single`;
- write doubles and floats in the shortest form that reads back to the same value, with a
  `.` or an exponent.

## Repository settings

Add to the `.gitattributes` of a repository that holds OTMAP maps:

```
*.otmap  text eol=lf
*.sector text eol=lf
```

Without it, Git on Windows checks the files out with CRLF.
