// Edits made by AI agents through the MCP server (internal/mcpserver): the
// editor shows them as if they were its own.
import { Events } from "@wailsio/runtime";
import { applyChange, loadMapData, map } from "./state.svelte";
import { goTo, mapCommands } from "./view/commands";

/** Follows the agents' edits; returns the function that stops. */
export function followAgents(): () => void {
  // event.data is an mcpserver.Event.
  return Events.On("mcp:map", (event) => {
    const agent = event.data;
    if (!agent) return;
    switch (agent.kind) {
      case "change":
        applyChange(agent.id, agent.change);
        break;
      case "opened":
        map.mode = "map";
        void mapCommands.created(agent.id);
        break;
      case "closed":
        map.maps = map.maps.filter((open) => open.id !== agent.id);
        if (map.activeId === agent.id) {
          map.activeId = map.maps[0]?.id ?? null;
          map.inspect = null;
          map.tile = null;
          map.focus = null;
        }
        break;
      case "data":
        void loadMapData(true);
        break;
      case "show":
        if (!agent.pos) break;
        map.mode = "map";
        map.activeId = agent.id;
        goTo(agent.pos, agent.id);
        break;
    }
  });
}
