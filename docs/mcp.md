# AI agents (MCP)

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

Its tools cover everything a map needs, and the objects of the client:

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
- **Objects:** look at the objects and sprites of the client, create them from PNG sprite
  sheets, change, duplicate, move and remove them, and save the client.
- **Server items:** `set_items_xml` attaches the `items.xml` and sets its attribute map,
  `get_thing` shows the entry of an item and `update_item_xml` edits it.

See [server-items.md](server-items.md) for the `items.xml` and its attribute map.
