# Golden Gate

Reverse proxy in Go to debug and visualize requests/responses to external APIs, with a server-rendered dashboard.

## Features
- One proxy per service declared in `configs/service.json`.
- Persistent request/response log in SQLite (file-based, survives redeploy when mounted on a volume).
- Per-service dashboard with cards, click-through to an explore page with time-range filtering.
- Automatic gzip decompression so JSON responses render readable.
- Body truncation at a configurable size, sensitive header redaction in stored logs.

## Requirements
- Go 1.23+
- [templ](https://templ.guide/) v0.3.924 (`go install github.com/a-h/templ/cmd/templ@v0.3.924`)
- Docker (optional)

## Local install & run

```sh
go mod download
go install github.com/a-h/templ/cmd/templ@v0.3.924
make run
```

Dashboard at <http://localhost:8080/dashboard>.

## Configuration

### `configs/service.json`
Define proxied services:

```json
{
  "buda_api": {
    "base_prefix": "/buda",
    "target": "https://www.buda.com/api/v2"
  }
}
```

A request to `http://localhost:8080/buda/markets` is forwarded to `https://www.buda.com/api/v2/markets`.

### Environment variables

| Var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DB_DRIVER` | `sqlite` | Database driver (only `sqlite` is wired up) |
| `DB_PATH` | `./data/golden_gate.db` | SQLite file path. In Docker, use `/data/golden_gate.db` with a mounted volume. |
| `CONFIG_PATH` | `./configs/service.json` | Path to the service.json. In Docker it lives on the volume (`/data/service.json`); edits are hot-reloaded. |
| `MAX_BODY_BYTES` | `1048576` | Max bytes persisted per request/response body. Larger bodies are truncated and flagged. |
| `TIME_ZONE` | `America/Santiago` | Display timezone for the dashboard (storage stays UTC) |
| `ENVIRONMENT` | `development` | Reserved for future use |

### Redacted headers
The following headers have their values replaced with `[REDACTED]` in stored logs (still forwarded untouched to the upstream):

- `Authorization`
- `Cookie`, `Set-Cookie`
- `Proxy-Authorization`
- `X-Api-Key`

## Dashboard

- `GET /dashboard` — cards, one per configured service plus an "huérfano" card for any historical data whose service is no longer in `service.json`. Click a card to drill in.
- `GET /dashboard/services/{name}?from=…&to=…&page=…` — explore page. Defaults to the last 24h in `TIME_ZONE`. Paginates 50 requests per page.

## Hot reload of services

The service.json is watched at runtime. Add, remove or edit entries while the
server is running and the dispatcher swaps the routing table within ~300 ms
without dropping in-flight requests. On invalid configs (missing target,
duplicate prefix, prefix colliding with `/dashboard`) the previous snapshot is
kept and the error is logged — the app never serves a broken table.

In production with the Coolify volume mounted, edit
`/mnt/hd1/golden-gate/service.json` on the host (or `/data/service.json` from
inside the container) and the change applies on the next debounce tick.
Bootstrap: if `CONFIG_PATH` does not exist on first boot, Golden Gate writes
its embedded default (the one shipped in the binary) to that location.

## Docker

```sh
docker build -t golden-gate .
docker run -p 8080:8080 \
  -v $(pwd)/configs:/app/configs \
  -v $(pwd)/data:/data \
  golden-gate
```

The image exposes port `8080`, declares `/data` as a volume and pre-sets `DB_PATH=/data/golden_gate.db`.

## Coolify deploy

Golden Gate is designed to run on Coolify with the SQLite file persisted to the host. Use this layout:

1. **Volume mount** — host path `/mnt/hd1/golden-gate/` ↔ container path `/data`.
2. **Env vars** — leave `DB_PATH=/data/golden_gate.db` (already set in the Dockerfile). Optionally override `MAX_BODY_BYTES`, `TIME_ZONE`.
3. **Build** — the Dockerfile is self-contained: `templ generate` runs at build time, migrations are embedded in the binary so no runtime filesystem dependency.

After a redeploy, the SQLite file remains under `/mnt/hd1/golden-gate/golden_gate.db` on the host and historical logs survive.

To purge logs manually:

```sh
sqlite3 /mnt/hd1/golden-gate/golden_gate.db "DELETE FROM request_logs WHERE created_at < datetime('now', '-30 days');"
```

## Generate views

```sh
make generate
```

Regenerates the `*_templ.go` files in `internal/dashboard/views/`.
