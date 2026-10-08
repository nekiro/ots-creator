import { mount } from "svelte";
import "./theme.css";
import App from "./App.svelte";
import { initPixelScale } from "./lib/pixelscale";

void initPixelScale();

mount(App, { target: document.getElementById("app")! });
