# ArrayDeck architecture

## 1. Recommended stack

### Backend

- Go service
- Official Docker Engine client or direct Engine API adapter
- GraphQL client for the Unraid API
- SQLite via an embedded driver
- Server-sent events for live browser updates
- Embedded compiled front-end assets

### Front end

- React + TypeScript
- Vite build
- Lightweight query/cache layer
- CSS variables and component primitives rather than a heavy dashboard framework

### Packaging

One ArrayDeck image serves the API and static UI. A hardened deployment may add a Docker socket proxy sidecar, but the application remains a single image.

## 2. Runtime components

### Source adapters

Each source adapter returns normalized records and declares its availability.

#### Docker adapter

Responsibilities:

- List all containers
- Inspect labels, image, state, health and port mappings
- Inspect network attachments and IP addresses
- Subscribe to Docker events
- Identify the ArrayDeck container so it can be hidden by default without special-casing its name

#### Unraid API adapter

Responsibilities:

- Discover server name and network addresses when available
- Read Docker/container status fields exposed by the current GraphQL schema
- Read future array, disk, VM and notification data
- Report API capabilities through schema introspection rather than assuming a fixed Unraid version

The adapter should use a read-only API key and degrade gracefully when unavailable.

#### dockerMan template adapter

Mount `/boot/config/plugins/dockerMan/templates-user` read-only and parse XML files.

Responsibilities:

- Match templates to containers by name and repository
- Read `<WebUI>`, `<Icon>`, `<Category>`, `<Project>` and related metadata
- Provide the exact user-configured WebUI pattern as a fallback to Docker labels
- Never write to the flash device

### Normalizer

Merges source records into a `ContainerRecord` keyed by Docker container ID. Every field keeps provenance:

```text
value: "http://[IP]:[PORT:8080]/"
source: "docker-label:net.unraid.docker.webui"
observed_at: timestamp
```

### Link resolver

Produces zero or more `EndpointCandidate` records and selects a winner using deterministic priority and confidence rules. Address rendering is kept separate from endpoint discovery so changing from hostname to IP does not require rescanning.

### Classifier

Maps an application to one or more categories. It should be rule-based and inspectable:

1. Unraid category
2. Explicit application labels
3. Curated image catalog
4. Repository/name tokens
5. Port/service fingerprints
6. `Other`

### Store

SQLite is the durable state layer. Suggested path inside the container: `/data/arraydeck.db`; host mapping: `/mnt/user/appdata/arraydeck:/data`.

Suggested tables:

```sql
settings(key TEXT PRIMARY KEY, value_json TEXT NOT NULL, updated_at INTEGER NOT NULL)
container_overrides(container_key TEXT PRIMARY KEY, value_json TEXT NOT NULL, updated_at INTEGER NOT NULL)
endpoint_decisions(container_key TEXT PRIMARY KEY, value_json TEXT NOT NULL, updated_at INTEGER NOT NULL)
icon_cache(cache_key TEXT PRIMARY KEY, mime TEXT, body BLOB, etag TEXT, updated_at INTEGER NOT NULL)
discovery_runs(id INTEGER PRIMARY KEY, started_at INTEGER, finished_at INTEGER, stats_json TEXT)
```

Avoid mirroring the entire Docker inventory into relational tables unless history becomes a product feature.

## 3. API shape

Suggested internal HTTP API:

```text
GET  /api/v1/status
GET  /api/v1/containers
GET  /api/v1/containers/{id}
GET  /api/v1/containers/{id}/discovery
POST /api/v1/containers/{id}/override
DELETE /api/v1/containers/{id}/override
GET  /api/v1/settings
PUT  /api/v1/settings
POST /api/v1/discovery/reconcile
GET  /api/v1/events                 # SSE
GET  /api/v1/icons/{cacheKey}
```

`GET /containers` should return normalized endpoints, not a URL already frozen to one link mode:

```json
{
  "id": "...",
  "name": "qBittorrent",
  "state": "running",
  "category": "Downloaders",
  "endpoint": {
    "scheme": "http",
    "addressPolicy": "unraid-host",
    "explicitHost": null,
    "hostPort": 8080,
    "path": "/",
    "confidence": 0.98,
    "source": "docker-label"
  }
}
```

The UI renders the address according to its selected mode.

## 4. Host identity and dynamic IP

Use this order:

1. Explicit administrator override.
2. Unraid API network/interface data, selecting an active private IPv4 address on the preferred interface.
3. Hostname parsed from the configured Unraid API endpoint.
4. Current browser origin hostname as a safe runtime fallback.

Expose all discovered candidates in Settings. Never silently switch to a different subnet after a network change; surface the change and select the best active address deterministically.

Recommended settings model:

```json
{
  "serverHostname": "tower.local",
  "lanAddress": "192.168.0.253",
  "preferredInterface": "br0",
  "linkMode": "hostname"
}
```

## 5. Event model

- Subscribe to Docker create/start/stop/die/destroy/health-status events.
- Debounce bursts for 250–500ms.
- Re-inspect only affected containers.
- Run a full reconciliation every 30 seconds to repair missed events.
- Broadcast compact patches over SSE.
- Use the Docker event stream for fast state changes even when the Unraid API has a longer cache window.

## 6. Security model

### Direct socket mode

Simplest deployment:

```text
/var/run/docker.sock:/var/run/docker.sock
```

A read-only filesystem mount does not make Docker socket access harmless. Code with access to the Docker API can often perform privileged actions. Treat this mode as trusted-code access to the host.

Mitigations:

- Keep the application read-only by design.
- Do not expose generic Docker API passthrough endpoints.
- Run the web process as a non-root user where possible.
- Bind ArrayDeck only to the LAN or place it behind an authenticated reverse proxy.
- Pin image versions and publish an SBOM.

### Hardened socket-proxy mode

Recommended production option:

- A small proxy owns the Docker socket.
- Expose only container list/inspect, network inspect, image inspect and events endpoints.
- Disable all write methods.
- Put the proxy on an internal Docker network not exposed to the LAN.

### Unraid API mode

Use a read-only key with the narrowest available Docker, system and network read permissions. Store it as a Docker secret or environment variable and never return it to the browser.

## 7. Deployment modes

### Compatibility mode

- ArrayDeck container
- Docker socket mounted directly
- Templates directory mounted read-only
- Optional Unraid API credentials

### Hardened mode

- ArrayDeck container
- Docker socket proxy sidecar
- Templates directory mounted read-only
- Read-only Unraid API key
- Optional reverse proxy authentication

## 8. Future expansion seams

Create module interfaces now, even if only the Applications module ships:

```go
type Module interface {
    Name() string
    RegisterRoutes(router Router)
    Snapshot(ctx context.Context) (any, error)
}
```

Likely future modules:

- Array and parity status
- Disk temperatures and SMART summaries
- Cache pool utilization
- VM inventory
- Docker update availability
- Unraid notifications
- Tailscale/remote access state
- Historical uptime and health events
