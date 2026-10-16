import { mount } from "svelte";
import "./theme.css";
import App from "./App.svelte";
import PlayWindow from "./features/map/PlayWindow.svelte";
import { initPixelScale } from "./lib/pixelscale";
import { initScrollButtons } from "./lib/ui/scrollbuttons";

// The play window (WindowService.OpenPlayWindow) shows only the play view.
const playWindow = new URLSearchParams(location.search).has("play");

if (!playWindow) void initPixelScale();
initScrollButtons();

// No browser context menu (Inspect, Reload...) anywhere: the app opens its
// own menus from oncontextmenu handlers, which still run.
window.addEventListener("contextmenu", (event) => event.preventDefault());

mount(playWindow ? PlayWindow : App, { target: document.getElementById("app")! });
