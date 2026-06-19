# vue_server — servion HTTP server serving an embedded Vue app

A servion HTTP server that serves a Vue single-page app embedded into the binary
via `go-bindata`. It shows the `assets` server option plus property-driven
configuration (`http-server.bind-address`, `http-server.options`).

## Run

```bash
go run . run
# serves the embedded Vue app on http://0.0.0.0:8000
```

`run` is the command registered by `servion.RunCommand`; servion binds, serves and
gracefully shuts the HTTP server down on Ctrl+C.

## Override the bind address (and any property) from the command line

The bind address has an in-code default (`http-server.bind-address = 0.0.0.0:8000`).
Override it for a single run with cligo's global `-D` / `--property` flag — no config
file or code change needed:

```bash
# bind somewhere else just for this run
go run . -D http-server.bind-address=127.0.0.1:9123 run

# repeatable; works for any property
go run . -D http-server.bind-address=127.0.0.1:9123 -D http-server.options=assets run
```

`-D`/`--property` sits at the top of the resolution order
(**`-D` → env vars → config file → in-code defaults**), so it wins over the default
set in `main.go`. Run `go run . --help` to see it listed under global options.

You can also override the same property from a config file:

```bash
go run . --config app.yaml run     # .properties / .yaml / .json / .toml
```

## Rebuilding the embedded web app

The committed `bindata.go` already embeds the built assets, so the server compiles
and runs as-is. To change the UI, edit `webapp/` and regenerate:

```bash
cd webapp
npm install
npm run build          # outputs to ../assets (see webapp/vite.config.ts)
cd ..
make bindata           # regenerates bindata.go from assets/ (needs go-bindata)
```

`make build` runs `bindata` then `go build` with version/build ldflags.

## Creating the web app from scratch

```bash
node -v
npm create vue@latest webapp
cd webapp
npm run build
```

Configure `vite.config.ts` so assets resolve relative to the binary and build into
`../assets`:

```ts
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  base: './',                // correct asset paths for embedded/static serving
  build: { outDir: '../assets' },
});
```

And a hash-history router in `webapp/src/router/index.js`:

```js
import { createRouter, createWebHashHistory } from 'vue-router';
import App from '../App.vue';

export default createRouter({
  history: createWebHashHistory(),
  routes: [{ path: '/', component: App }],
});
```
