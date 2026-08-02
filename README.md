# ArrayDeck

**A self-discovering application dashboard for Unraid.**

ArrayDeck lists every Docker container on an Unraid server and automatically
derives the best available front-end URL. It is not another manually curated
bookmark dashboard: the deployed containers and Unraid metadata are the source
of truth.

- **Containers are discovered, not entered.** Docker events update the grid
  within seconds; a periodic reconcile repairs anything missed.
- **Links are resolved, not copied into configuration.** Unraid
  `net.unraid.docker.webui` labels, dockerMan template `<WebUI>` patterns
  (`[IP]`/`[PORT:n]` tokens included), reverse-proxy labels, a curated app
  catalog and safe HTTP probes — in strict priority order.
- **Groups are inferred, not hand-maintained**, from Unraid category metadata
  with deterministic fallbacks.
- **Every decision is explainable.** The discovery inspector shows the winning
  source, each resolution step, every candidate and its evidence — and lets
  you override or dismiss without losing the discovered record.
- **All containers stay visible.** Databases and workers show
  “No web interface detected” instead of disappearing.
- **Read-only by design.** No start/stop/restart, no Docker API passthrough.

The full product and technical design lives in [`docs/`](docs/README.md).

## Quick start (Unraid)

1. Copy `deploy/unraid/arraydeck.xml` to
   `/boot/config/plugins/dockerMan/templates-user/` (or add the container
   manually with the same mappings).
2. Required mappings:

   ```text
   /mnt/user/appdata/arraydeck                       → /data          (rw)
   /boot/config/plugins/dockerMan/templates-user     → /unraid/templates (ro)
   /var/run/docker.sock                              → /var/run/docker.sock (ro)
   ```

3. Optional: create a **read-only** API key in Unraid (Settings → Management
   Access → API) and set `UNRAID_API_URL` (e.g. `http://tower.local/graphql`)
   and `UNRAID_API_KEY`. This enables hostname/LAN-address detection and the
   server card. ArrayDeck works without it using Docker metadata alone.
4. Open `http://tower.local:8417/`.

## Quick start (Compose)

```sh
cd deploy/compose
docker compose up --build
```

`docker-compose.hardened.yml` is the recommended production variant: a
filtered [docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy)
owns the socket (write methods disabled), and ArrayDeck runs unprivileged with
`DOCKER_HOST=tcp://docker-proxy:2375`.

## Configuration

Everything runtime-tunable (link mode, hostname/LAN overrides, probes, theme,
visibility) lives in the UI under **Settings** and persists in SQLite at
`/data/arraydeck.db`. Process configuration is environment-only:

| Variable | Default | Purpose |
|---|---|---|
| `ARRAYDECK_LISTEN` | `:8417` | HTTP listen address |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker engine; supports `tcp://` socket proxies |
| `UNRAID_API_URL` | – | Optional Unraid GraphQL endpoint |
| `UNRAID_API_KEY` | – | Read-only key; never sent to the browser |
| `TEMPLATES_DIR` | `/unraid/templates` | dockerMan user templates (mounted read-only) |
| `DATA_DIR` | `/data` | SQLite + icon cache location |
| `RECONCILE_INTERVAL_SECONDS` | `30` | Full re-discovery interval |
| `LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` (JSON logs) |

## Link modes

The address is a *policy*, not a frozen string: switching modes rewrites every
applicable link instantly, with no rediscovery.

- **Hostname** — the discovered/overridden server name (normally `tower.local`).
- **LAN IP** — the selected private IPv4 address.
- **Smart** — preserves explicit reverse-proxy hostnames, otherwise prefers the
  hostname. Containers with their own LAN address (macvlan/ipvlan) always use it.

## Security model

- The MVP is read-only: no endpoint can mutate containers and no generic
  Docker passthrough exists.
- A directly mounted socket still grants the process broad host power. Bind
  ArrayDeck to the LAN only or put it behind an authenticated reverse proxy,
  and prefer the hardened socket-proxy deployment.
- HTTP probes target only local addresses derived from trusted container
  metadata, use short timeouts, never follow redirects off-host and never send
  credentials.
- The Unraid API key stays server-side and is redacted from logs and errors.
- Icons are proxied and cached server-side to avoid mixed-content and CORS
  problems.

## Development

```sh
make run        # backend on :8417 against your local Docker socket
make dev-web    # Vite dev server on :5173, proxying /api
make test       # Go unit + fixture-integration tests
make embed      # single binary with the compiled UI embedded
make docker     # full image build
```

Repository layout follows `docs/IMPLEMENTATION_PLAN.md`:

```text
cmd/arraydeck            entrypoint
internal/discovery/*     docker, unraid (GraphQL), dockerMan template adapters
internal/linkresolver    WebUI token parsing, candidates, scoring, probes
internal/classifier      category rules
internal/store           SQLite (settings, overrides, decisions, icon cache)
internal/app             reconciler, host identity, view assembly
internal/api             REST + SSE + embedded UI
web/                     React + TypeScript + Vite (light-first theme)
deploy/                  compose + Unraid template
```

The REST surface is documented in `docs/ARCHITECTURE.md` §3 and served under
`/api/v1`; live updates stream over `GET /api/v1/events` (SSE).
