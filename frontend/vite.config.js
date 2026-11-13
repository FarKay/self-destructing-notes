import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],

  build: {
    // the Go program embeds the "web" folder, so build straight into it
    outDir: "../web",
    emptyOutDir: true,
  },

  server: {
    // while developing, send API calls to the Go server
    proxy: {
      "/notes": "http://localhost:4000",
    },
  },
});
