// The player of the play view: outfit, level and the world light, kept in
// local storage (shared by the play window).
import { loadJson, saveJson } from "../../state/storage";
import { PLAY } from "../constants";

const PLAYER_KEY = "map:player";

export interface PlayerSettings {
  lookType: number;
  head: number;
  body: number;
  legs: number;
  feet: number;
  /** Addon bits: 1 first, 2 second. */
  addons: number;
  /** Mount looktype, ridden while mounted is set. */
  mount: number;
  mounted: boolean;
  /** Sets the speed (TFS: 220 + 2 per level above 1). */
  level: number;
  /** World light the server sends (PLAY.dayLight by day, PLAY.nightLight by night). */
  worldLight: number;
}

export const DEFAULT_PLAYER: PlayerSettings = {
  lookType: 128,
  head: 78,
  body: 69,
  legs: 58,
  feet: 76,
  addons: 0,
  mount: 0,
  mounted: false,
  level: 1,
  worldLight: PLAY.dayLight,
};

export const playerSettings = $state<PlayerSettings>(loadJson(PLAYER_KEY, DEFAULT_PLAYER));

$effect.root(() => {
  $effect(() => saveJson(PLAYER_KEY, { ...playerSettings }));
});
