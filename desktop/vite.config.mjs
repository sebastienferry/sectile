import { defineConfig } from "vite"
import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  base: "./",
  plugins: [
    {
      name: "changelog-raw",
      enforce: "pre",
      resolveId(id) {
        if (id.includes("CHANGELOG.md")) {
          return "\0changelog.raw"
        }
      },
      load(id) {
        if (id === "\0changelog.raw") {
          const changelog = fs.readFileSync(path.resolve(__dirname, "../CHANGELOG.md"), "utf-8")
          return `export default ${JSON.stringify(changelog)}`
        }
      }
    },
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
