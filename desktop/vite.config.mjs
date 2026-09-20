import { defineConfig } from "vite"
import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  base: "./",
  plugins: [
    {
      name: "copy-icon",
      generateBundle() {
        this.emitFile({
          type: "asset",
          fileName: "assets/icon.svg",
          source: fs.readFileSync(path.resolve(__dirname, "assets/icon.svg")),
        })
      }
    }
  ]
})
