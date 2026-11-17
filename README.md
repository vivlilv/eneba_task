# Eneba Marketplace Demo

A deliberately narrow marketplace slice for the Eneba internship assignment. The current lane is homepage → product/offers → cart → sign-up/sign-in → checkout → simulated payment → Order and digital-key retrieval. The homepage's copy and product blocks come from PostgreSQL; no real payment is processed and every seeded key is synthetic.

## Stack

- React + TypeScript + Vite (`web/`)
- Go REST API (`api/`)
- PostgreSQL, with Docker Compose for local development (`compose.yaml`)

## Run locally

Start the local database:

```sh
docker compose up -d db
```

In one terminal, start the API:

```sh
cd api
go run ./cmd/server
```

The API applies embedded SQL migrations and seeds fictional demo products, sellers, offers, and non-redeemable keys on startup. It listens on `http://localhost:8080`.

In another terminal, start the frontend:

```sh
cd web
npm ci
npm run dev
```

Open the Vite URL (normally `http://localhost:5173`). Vite proxies `/api` requests to the Go API.

Local defaults are for development only. To load overrides from `.env.example`, copy it to `.env` and export it in your shell (`set -a; source .env; set +a`). Do not reuse the demo database password or demo keys in production.

## Current boundary

The homepage is the only product discovery surface. A browse-all catalog, search, filtering, sorting, seller management, and real payment are intentionally outside this slice. The public free-tier deployment target is not selected yet.
