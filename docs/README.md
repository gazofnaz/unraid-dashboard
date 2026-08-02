# ArrayDeck design package

**Working title:** ArrayDeck  
**Product:** A self-discovering application dashboard for Unraid  
**Status:** Build-ready product and technical design

ArrayDeck lists every Docker container on an Unraid server and automatically derives the best available front-end URL. It is not another manually curated bookmark dashboard: the deployed containers and Unraid metadata are the source of truth.

## Package contents

- `PRODUCT_SPEC.md` — product scope, behavior and acceptance criteria
- `ARCHITECTURE.md` — recommended implementation and security model
- `DISCOVERY_ENGINE.md` — deterministic URL and category discovery rules
- `UI_SPEC.md` — screens, components and responsive behavior
- `IMPLEMENTATION_PLAN.md` — phased engineering plan and test matrix
- `CODEX_PROMPT.md` — a ready-to-paste build brief for Codex
- `deployment/docker-compose.yml` — reference deployment
- `deployment/arraydeck.xml` — starter Unraid Docker template
- `mockups/01-dashboard-desktop.png` — primary desktop concept
- `mockups/02-discovery-inspector.png` — explainable discovery screen
- `mockups/03-mobile-dashboard.png` — mobile layout
- `mockups/04-architecture.png` — system architecture
- `prototype/*.svg` — editable vector originals for the mockups

## Product position

ArrayDeck borrows the polished application-grid feel of Homarr, Dashy, Homer and Heimdall, but changes the data model:

- Containers are discovered, not entered.
- Web links are resolved, not copied into configuration.
- Groups are inferred, not hand-maintained.
- Containers without a web interface are still visible.
- Manual edits are optional overrides and never become the primary inventory.

## Recommended MVP

Build one Docker image containing a small Go backend and a compiled React front end. Persist only preferences, cached metadata and overrides in SQLite at `/data/arraydeck.db`. Read container inventory from Docker, enrich it with Unraid API data and dockerMan template metadata, then stream status changes to the browser.

The first release should be read-only. Opening links, searching, filtering and inspecting discovery decisions are in scope. Start/stop/restart controls can be added later behind explicit permissions.
