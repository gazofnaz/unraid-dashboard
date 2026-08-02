# ArrayDeck implementation plan

## Phase 0 — repository and quality gates

Deliverables:

- Monorepo structure
- Backend and front-end build
- Multi-stage Dockerfile
- Lint, unit test and image build workflows
- Version endpoint and structured logging
- SQLite migration framework

Suggested layout:

```text
/cmd/arraydeck/main.go
/internal/api
/internal/config
/internal/discovery/docker
/internal/discovery/unraid
/internal/discovery/templates
/internal/linkresolver
/internal/classifier
/internal/store
/internal/events
/web/src
/web/public
/deploy/unraid/arraydeck.xml
/deploy/compose/docker-compose.yml
```

## Phase 1 — Docker inventory

- Connect to Docker Engine
- List all containers including stopped containers
- Normalize name, image, state, health, labels, ports and networks
- Subscribe to Docker events
- Add `/api/v1/containers` and `/api/v1/events`
- Build the flat Containers screen

Exit test: adding, stopping and deleting a test container updates the UI without restarting ArrayDeck.

## Phase 2 — Unraid-native discovery

- Parse `net.unraid.docker.webui`, icon and managed labels
- Parse dockerMan user templates read-only
- Match templates safely
- Implement `[IP]` and `[PORT:n]` parsing
- Add Hostname and LAN IP address policies
- Add Unraid GraphQL connection and capability introspection

Exit test: common Unraid-managed apps open through their actual published host ports.

## Phase 3 — resolver, classification and inspector

- Candidate generation and scoring
- Reverse-proxy label adapters
- Safe HTTP probe worker
- Known application catalog format
- Automatic category classifier
- Discovery trace API and inspector UI
- Low-confidence review queue

Exit test: every selected endpoint can be explained from stored source facts.

## Phase 4 — polished applications dashboard

- Implement grouped card view
- Search and filters
- Hostname/LAN IP/Smart segmented control
- Icon proxy/cache
- Responsive mobile layout
- Empty/error/loading states
- Keyboard navigation

Exit test: visual and interaction behavior matches the supplied mockups at desktop and mobile breakpoints.

## Phase 5 — persistence and overrides

- Settings table
- URL/name/icon/category/visibility overrides
- Favorites and collapsed groups
- Backup-friendly database location
- Reset-to-discovered action

Exit test: all preferences survive a container recreation while live inventory still follows Docker.

## Phase 6 — deployment hardening

- Non-root image
- Health check
- Read-only root filesystem support where practical
- Direct socket and socket-proxy examples
- Unraid XML template
- Security documentation
- SBOM and vulnerability scan

## Test matrix

### Network modes

- Default bridge with one published port
- User-defined bridge
- Host network
- Custom LAN IP on macvlan/ipvlan
- Namespace sharing through another container
- IPv4 wildcard binding
- Loopback-only binding
- IPv6 binding

### Metadata

- Correct Unraid WebUI label
- Incorrect/missing mapping in WebUI label
- dockerMan template only
- Reverse-proxy label only
- Compose container with no Unraid labels
- Multiple plausible HTTP ports
- No HTTP service
- External explicit hostname

### Lifecycle

- Start/stop/restart
- Container rename
- Image update/recreate with new ID
- Container removal
- Docker daemon restart
- Unraid array stop/start
- Unraid API unavailable/recovered
- Template file updated

### Scale

- 10 containers
- 100 containers
- 300 containers
- Slow/unresponsive endpoints during probes

## Definition of done for v0.1

- Fresh install populates without manual links.
- All containers are visible.
- Hostname and native IP modes work.
- At least three discovery sources are implemented: Unraid WebUI label, dockerMan XML and published-port probe.
- Event-driven refresh works.
- Discovery inspector exists.
- SQLite persists settings and overrides under appdata.
- Compose and Unraid XML deployment are documented and tested.
