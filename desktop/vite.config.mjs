import { defineConfig } from "vite"
import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"
import { browserAgent } from "./browser-agent.mjs"

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  base: "./",
  plugins: [
    {
      name: "browser-preview",
      apply: "serve",
      configureServer(server) {
        server.middlewares.use(browserAgent())
      },
      transformIndexHtml(html) {
        return html.replace("connect-src 'none'", "connect-src 'self' ws: wss:")
          .replace(/src="\/src\/main\.js(?:\?[^"\s]*)?"/, 'src="/src/browser-preview.mjs"')
      }
    },
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
