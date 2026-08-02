# ArrayDeck product specification

## 1. Problem

Existing homelab dashboards typically require users to manually add every application link, icon and group. The dashboard therefore becomes a second configuration database that drifts from the actual Unraid deployment.

ArrayDeck treats Unraid and Docker as the source of truth. It automatically lists all containers, identifies which ones expose a browser interface, resolves the correct address and port, and groups applications using existing metadata.

## 2. Primary user outcome

A user installs one dashboard container and immediately sees:

1. Every running and stopped Docker container.
2. A working front-end link where one can be determined.
3. A clear “No web interface detected” state where no front end exists.
4. A global choice between `tower.local`, the server’s native LAN IP, and a smart address mode.
5. Automatically inferred categories and icons.

No application-by-application setup is required for a normal Unraid deployment.

## 3. Product principles

### Deployment is the source of truth

The visible inventory is rebuilt from live container state. SQLite does not own the list of applications.

### Automation must be explainable

Each selected URL records its source, transformations and confidence. The user can open a discovery trace instead of guessing why a link is wrong.

### Overrides are exceptions

A user may override a URL, icon, name, visibility or category, but the underlying discovered record remains visible and can be restored.

### All containers are first-class

Databases, workers and utility containers remain listed even when they do not expose HTTP. The dashboard is also a lightweight inventory.

### Read-only by default

The MVP does not manage containers. This keeps the permission model simple and reduces risk.

## 4. Core workflows

### First run

1. User deploys ArrayDeck.
2. ArrayDeck connects to Docker and optionally the Unraid GraphQL API.
3. The app scans containers, labels, port mappings, networks and Unraid templates.
4. The browser opens directly to a populated application grid.
5. A setup banner appears only when a required discovery source is unavailable.

### Open an application

1. User chooses a global link mode: Hostname, LAN IP or Smart.
2. Card URLs are rendered from the same normalized endpoint using the selected address policy.
3. Clicking the card opens the application in a new tab.

### Diagnose a bad link

1. User opens the card details.
2. The discovery trace shows the winning source, candidate URLs, mapped-port substitution and probe result.
3. User can set a narrow override or dismiss an incorrect candidate.

### Add a new Docker container

1. Container is deployed in Unraid.
2. Docker emits an event or the next reconciliation cycle detects it.
3. The new card appears without editing ArrayDeck.

## 5. Functional requirements

### Inventory

- List running, stopped, paused, restarting and unhealthy containers.
- Display name, image, state, health, network mode, relevant ports and icon.
- Refresh automatically from Docker events with periodic reconciliation.
- Never hide a container solely because no web URL was found.

### Link modes

- **Hostname:** use the discovered Unraid hostname, normally `tower.local`.
- **LAN IP:** use the selected private LAN address discovered from Unraid network information.
- **Smart:** preserve explicit reverse-proxy hostnames; otherwise use the preferred local address.
- Remember the selected mode per browser and optionally globally.
- Allow a hostname and LAN IP override when automatic detection is wrong.

### URL discovery

- Prefer explicit Unraid/Docker WebUI metadata.
- Resolve `[IP]` and `[PORT:n]` tokens against actual networking and port mappings.
- Detect reverse-proxy routes from common labels.
- Probe plausible published HTTP ports only after metadata-based methods fail.
- Keep all candidate URLs with a source and confidence score.
- Avoid treating common non-HTTP service ports as web interfaces.

### Automatic grouping

- Prefer Unraid template category metadata.
- Fall back to known image/app classification, OCI labels and port fingerprints.
- Offer “Grouped”, “Flat”, “By state” and “By network” views.
- Permit optional overrides without requiring any group configuration.

### Search and filtering

- Search by container name, image, category, port, host and URL.
- Filter by state, web UI availability, network, category and confidence.

### Persistence

SQLite stores only:

- User preferences
- Manual overrides
- Favorites and hidden-state choices
- Cached icons and metadata
- Discovery decisions and short history
- Optional probe results

The live container list is reconstructed at startup.

## 6. Non-functional requirements

- Startup to populated dashboard: target under 3 seconds for 100 containers on a local server.
- Reconciliation interval: configurable, default 30 seconds.
- Docker event updates visible in the UI within 2 seconds.
- No third-party cloud dependency.
- Usable without internet access after the image is pulled.
- Responsive down to 360px width.
- Keyboard navigable with visible focus states.
- Minimum contrast target: WCAG AA.
- App should remain usable if the Unraid API is unavailable but Docker metadata is available.

## 7. MVP scope

### Included

- Docker inventory
- Dynamic front-end links
- Hostname/LAN IP/Smart modes
- Automatic grouping
- Search/filter
- Discovery inspector
- SQLite persistence
- Desktop and mobile UI
- Unraid Docker template and Compose deployment

### Deferred

- Start/stop/restart/update actions
- Container logs and terminal
- Full array/disk dashboard
- VM controls
- Authentication beyond trusted-LAN/reverse-proxy deployment guidance
- Multi-server support
- Cloud sync

## 8. Acceptance criteria

1. A freshly deployed ArrayDeck instance lists all Docker containers without user-entered links.
2. A standard Unraid-managed container with `net.unraid.docker.webui` opens the correct mapped host port.
3. Switching between Hostname and LAN IP rewrites applicable links without rediscovery.
4. A container on a custom network with its own LAN IP uses the correct endpoint policy.
5. A host-networked application resolves against the Unraid host.
6. A database container is listed with “No web interface detected.”
7. Adding or removing a container updates the dashboard automatically.
8. A low-confidence inferred link is visibly marked and has a trace.
9. Preferences and overrides survive container restarts through `/mnt/user/appdata/arraydeck`.
10. The dashboard works when the Unraid API is disabled, with reduced server metadata and an explicit status message.
