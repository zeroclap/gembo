# Gembo frontend

Next.js 16.2.1 / React 19.2.4 dashboard using Tailwind v4, shadcn/ui, TanStack Query/Table, and zustand. Use the committed npm lockfile; Node/npm version pinning is pending D02.

Run from `frontend/`:

```bash
npm ci
npm run dev
```

The UI serves port 3000. Start the Go API separately on port 8080 with local Redis/PostgreSQL and a synthetic application user. HTTP requests under `/api/*` are rewritten to the backend with `/api` removed. The WS hook connects directly to `/v1/arbitrage/ws`; `NEXT_PUBLIC_WS_HOST` overrides its default browser hostname and port 8080. Browser auth/subscription fixes remain planned work, so startup alone does not establish streaming correctness.

`npm run lint` runs ESLint. `npm run build` creates the production build; `npm run start` serves that build. There is no test script or frontend container definition yet.

Before writing frontend code, read [AGENTS.md](AGENTS.md) and the relevant installed Next.js guide. See the [repository guide](../docs/development.md) for processes and configuration, and the [delivery plan](../docs/delivery/README.md) for CLI-first implementation order and UI acceptance requirements.
