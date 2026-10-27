import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import wails from "@wailsio/runtime/plugins/vite";

export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  build: {
    // Bundle ships embedded in the Wails binary and loads from disk, so
    // chunk size doesn't affect load time the way it would over a network.
    chunkSizeWarningLimit: 1024,
  },
  plugins: [svelte(), wails("./bindings")],
});
