import { mount } from "svelte";
import "./theme.css";
import App from "./App.svelte";
import { initPixelScale } from "./lib/pixelscale";
import { initScrollButtons } from "./lib/ui/scrollbuttons";

void initPixelScale();
initScrollButtons();

// No browser context menu (Inspect, Reload...) anywhere: the app opens its
// own menus from oncontextmenu handlers, which still run.
window.addEventListener("contextmenu", (e) => e.preventDefault());

mount(App, { target: document.getElementById("app")! });
