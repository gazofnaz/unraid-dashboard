# Codex build brief: ArrayDeck v0.1

Build a production-quality MVP named **ArrayDeck**, a self-discovering Docker application dashboard designed for Unraid.

Read these files before writing code:

1. `PRODUCT_SPEC.md`
2. `ARCHITECTURE.md`
3. `DISCOVERY_ENGINE.md`
4. `UI_SPEC.md`
5. `IMPLEMENTATION_PLAN.md`

Use the supplied PNG mockups as the visual target. The product must not require users to manually enter each application link or group.

## Required architecture

- Go backend serving a compiled React + TypeScript front end from one Docker image.
- SQLite at `/data/arraydeck.db`.
- Docker Engine adapter supporting container list, inspect and events.
- Optional Unraid GraphQL adapter using a read-only API key.
- Read-only dockerMan template parser for `/unraid/templates`.
- Server-sent events for live status changes.
- REST API under `/api/v1`.

## Required deployment mappings

```text
/mnt/user/appdata/arraydeck:/data
/boot/config/plugins/dockerMan/templates-user:/unraid/templates:ro
/var/run/docker.sock:/var/run/docker.sock
```

Support a hardened `DOCKER_HOST=tcp://docker-proxy:2375` deployment as an alternative to a direct socket.

## Required product behavior

1. List all Docker containers, including stopped containers.
2. Detect explicit Unraid WebUI metadata from `net.unraid.docker.webui`.
3. Resolve `[IP]` and `[PORT:n]` tokens using actual network mode and port mappings.
4. Fall back to matching dockerMan XML `<WebUI>` metadata.
5. Fall back to reverse-proxy labels and safe HTTP probing.
6. Show containers with no web interface instead of hiding them.
7. Provide Hostname, LAN IP and Smart address modes.
8. Automatically group apps using Unraid categories and deterministic fallbacks.
9. Provide a discovery inspector explaining the selected URL.
10. Persist only settings, overrides, icon cache and discovery decisions in SQLite.

## Initial API contract

Implement:

```text
GET    /api/v1/status
GET    /api/v1/containers
GET    /api/v1/containers/{id}
GET    /api/v1/containers/{id}/discovery
GET    /api/v1/settings
PUT    /api/v1/settings
POST   /api/v1/containers/{id}/override
DELETE /api/v1/containers/{id}/override
POST   /api/v1/discovery/reconcile
GET    /api/v1/events
GET    /healthz
```

## Data model

Represent discovered endpoints in normalized form. Do not store only a final URL string.

```go
type Endpoint struct {
    Scheme         string
    AddressPolicy  string // unraid-host, container-lan-ip, explicit-host
    ExplicitHost   string
    ContainerPort  int
    HostPort       int
    Path           string
    Confidence     float64
    Source         string
}
```

Every candidate must keep evidence and a human-readable explanation.

## UI requirements

- Match `mockups/01-dashboard-desktop.png` and `mockups/03-mobile-dashboard.png` closely.
- Implement `mockups/02-discovery-inspector.png` as a functional screen.
- Do not use a generic admin template.
- Use CSS variables for the supplied palette.
- Support keyboard and touch.
- Use generated initials when no icon is available.
- Do not block page rendering on probes or external icon downloads.

## Safety requirements

- Read-only MVP: do not expose Docker mutations.
- Do not expose a generic Docker API proxy.
- Restrict probes to trusted local candidates produced by the resolver.
- Do not send cookies, credentials or authorization headers in probes.
- Never expose the Unraid API key to the browser.
- Log secrets only as redacted values.

## Testing requirements

Write unit tests for:

- WebUI token parsing
- Port-binding selection
- Bridge, host and custom-LAN network policies
- Template matching
- Candidate scoring
- Non-HTTP port exclusions
- Override precedence
- Category normalization

Write integration tests using fixture Docker inspect JSON and fixture Unraid XML files. Include at least Plex, qBittorrent, Home Assistant, PostgreSQL and a reverse-proxied Compose app.

## Build order

Complete the work in vertical slices:

1. Docker inventory API + simple UI
2. Unraid WebUI label resolution
3. Template parser
4. Address mode switching
5. Automatic grouping
6. Discovery inspector
7. Persistence and overrides
8. Hardened deployment docs

At the end of each slice, keep the repository runnable with `docker compose up --build`.
