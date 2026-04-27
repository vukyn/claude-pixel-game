# editor-web

React + Vite + TS + Tailwind + shadcn/ui FE for the behavior visual editor.

Pairs with the Go Fiber editor server at `cmd/editor` (port 8080 by default). Vite dev server proxies `/api/*` to the editor server.

## Run

```bash
# from repo root — terminal A
make editor       # go run ./cmd/editor on :8080

# terminal B
make web          # vite dev on :5173
# or
cd tools/editor-web && npm run dev
```

Open http://localhost:5173.

## Test

```bash
cd tools/editor-web
npm run test       # vitest unit (mapping, validation, layout, store, api client)
npm run e2e        # playwright (requires both servers running)
npm run build      # production bundle to dist/
```

## Layout

```
src/
  api/          typed fetch wrappers + zod schemas
  state/        zustand editor store
  bt/           JSON ↔ graph mapping, validation, dagre layout, custom React Flow nodes
  components/   TopBar, StatesPanel, BTCanvas, Inspector, TuningDrawer
  components/ui shadcn/ui primitives (button, badge, tabs, slider, ...)
  lib/          cn() helper
```

## Stack

| Layer | Lib |
|---|---|
| Framework | React 19 + Vite 8 + TypeScript |
| Styling | Tailwind v3 + shadcn/ui (radix-nova style) |
| Graph canvas | React Flow v11 + dagre auto-layout |
| State | Zustand |
| Forms | shadcn Field + radix primitives |
| JSON viewer | @uiw/react-json-view |
| Test | Vitest + @testing-library/react + Playwright |

## Conventions

- Semantic Tailwind tokens only (`bg-card`, `text-muted-foreground`, ...). No raw colors except in BT node components (preserve graph color coding).
- Path alias `@/*` → `./src/*`
- Forms via `<Field>` + `<FieldLabel htmlFor>` + control with matching `id`
- shadcn upgrades: `npx shadcn@latest add --diff <component>` then merge per file
