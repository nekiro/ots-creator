# Server items

A server keeps two files next to the client: `items.otb` (servers that give items their own
server ids) and `items.xml` (names and attributes). Both can be attached to a client. Both
are optional and remembered per client.

## items.otb

For servers that map server ids to client ids with it (TFS, OTServ). Pick the file in Open
client or in **File > Server items (items.otb)**.

Compiling also updates it: server items follow moved client items, edited items get their
client data reloaded (flags, light, minimap color, sprite hash…) and new client items get
new server ids. Server ids and server-only data are kept, and an unchanged file is not
rewritten.

Maps on server ids read their ids with the same file.

## items.xml

Attach the `items.xml` of the server in **File > Server items (items.xml)**; it is
remembered per client, and one found next to the client or its `items.otb` is offered.

- **Ids:** with an `items.otb` attached the ids of the file are server ids, translated with
  it. Without one they are client ids.
- **Entries:** the properties of an item get a **Server (items.xml)** section with its name,
  article, plural and attributes (nested ones too). Edits go through Apply and undo like
  the rest. The object list shows and searches the names of the file.
- **Compiling** writes the edited entries and adds an entry for every new item. Everything
  else in the file keeps its bytes: comments, formatting, charset. A `fromid`/`toid` range
  is only split when an item inside it changes. On client ids the entries also follow items
  moved to other ids.
- The file is read again when it changes on disk, so it can be edited in a text editor
  meanwhile.

## Attribute map

A server on client ids has no `items.otb` to learn the properties of the client items
from, so it reads them from `items.xml`. The attribute map tells which property becomes
which attribute. Set it in the same dialog; it is stored as `items.attributes.json` next
to `items.xml`, so it can be shared with the server:

```json
{
  "attributes": [
    { "client": "pickupable", "key": "pickupable" },
    { "client": "groundSpeed", "key": "groundspeed" },
    { "client": "unpassable", "key": "blocksolid", "aliases": ["blocking"] },
    { "client": "unmoveable", "key": "moveable", "not": true }
  ]
}
```

- `client` is a property of the client item (the name it has in OTOBJ files), `key` the
  attribute.
- A flag writes `value` (`1` when left out) while it is set and no attribute while it is
  off; a number writes itself.
- `not` turns a flag around: the attribute is written while the flag is off (`moveable` of
  the client's `unmoveable`).
- `aliases` are other keys the server reads as `key`. An entry that has one keeps it
  instead of getting `key` next to it.

With a map, on client ids:

- Changing a mapped property of an item changes its attribute in the Server section at
  once, and compiling writes it. New items get all their mapped attributes.
- An entry that differs from the client says where. Nothing is changed by itself, since a
  server can differ on purpose; **Fill from client** makes the entry what the client says.
- An item without an entry gets one with **Create from client**.
